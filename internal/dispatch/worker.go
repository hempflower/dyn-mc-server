package dispatch

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/Tnze/go-mc/bot"
	"github.com/hempflower/dmcs/internal/dns"
	"github.com/hempflower/dmcs/internal/vm"
	"github.com/spf13/viper"
)

type DispatchWorker struct {
	dnsServer     *dns.DnsServer
	serverMap     map[string]*MCServerInfo
	options       *DispatchWorkerOptions
	fakeMcServers map[int]*FakeMcServer
	quit          chan struct{}
}

type DispatchWorkerOptions struct {
	DnsTtl     int
	RootDomain string
	StartPort  int
	EndPort    int
	PublicIp   string
}

type MCServerInfo struct {
	Name      string
	SubDomain string
	Port      int
	VM        vm.VmProvider

	fakeMcServer *FakeMcServer
	lastActivity time.Time
	quit         chan struct{}
}

func GetMinecraftSrvRecord(name string) string {
	return fmt.Sprintf("_minecraft._tcp.%s", name)
}

func NewDispatchWorker(options *DispatchWorkerOptions) *DispatchWorker {
	return &DispatchWorker{
		dnsServer: dns.NewDnsServer(options.DnsTtl, options.RootDomain),
		serverMap: make(map[string]*MCServerInfo),
		options:   options,
	}
}

func (w *DispatchWorker) GetDispatchServer() string {
	return fmt.Sprintf("dispatch.%s", w.options.RootDomain)
}

func (w *DispatchWorker) GetMcServerDomain(name string) string {
	return fmt.Sprintf("%s.%s", name, w.options.RootDomain)
}

func (w *DispatchWorker) StartServer(name string) error {
	info := w.serverMap[name]
	if info == nil {
		return errors.New("server not found")
	}

	status, err := info.VM.GetStatus()
	if err != nil {
		log.Printf("Failed to get status of server %s: %s", info.Name, err)
		return err
	}

	if status.Status == vm.VM_STATUS_STOPPED {
		err := info.VM.Start()
		if err != nil {
			log.Printf("Failed to start server %s: %s", info.Name, err)
			return err
		}
		startTime := time.Now()
		for {
			// wait for server to start
			status1, err := info.VM.GetStatus()
			if err != nil {
				log.Printf("Failed to get status of server %s: %s", info.Name, err)
				return err
			}

			if status1.Status == vm.VM_STATUS_RUNNING {
				break
			}
			log.Println("Waiting for server to start")
			if time.Since(startTime) > 1*time.Minute {
				return errors.New("server failed to start")
			}
		}

		log.Println("Server started")
	}

	// 更新 dns
	w.dnsServer.AddServiceRecord(
		GetMinecraftSrvRecord(info.SubDomain),
		fmt.Sprintf("%s:%d", w.GetMcServerDomain(info.SubDomain), info.Port),
	)
	w.dnsServer.AddARecord(info.SubDomain, status.Ip)

	info.lastActivity = time.Now()

	return nil
}

func (w *DispatchWorker) StopServer(name string) error {
	info := w.serverMap[name]
	if info == nil {
		return errors.New("server not found")
	}

	status, err := info.VM.GetStatus()
	if err != nil {
		log.Printf("Failed to get status of server %s: %s", info.Name, err)
		return err
	}

	if status.Status == vm.VM_STATUS_RUNNING {
		err := info.VM.Stop()
		if err != nil {
			log.Printf("Failed to stop server %s: %s", info.Name, err)
			return err
		}
	}

	// 更新 dns
	// 切换到 fake mc server
	w.dnsServer.AddServiceRecord(
		GetMinecraftSrvRecord(info.SubDomain),
		fmt.Sprintf("%s:%d", w.GetDispatchServer(), info.fakeMcServer.port),
	)
	w.dnsServer.RemoveARecord(info.SubDomain)

	return nil
}

func (w *DispatchWorker) AddServer(info *MCServerInfo) {
	info.lastActivity = time.Now()

	usedPorts := make(map[int]struct{})
	for _, server := range w.fakeMcServers {
		usedPorts[server.port] = struct{}{}
	}

	var port int
	for i := w.options.StartPort; i <= w.options.EndPort; i++ {
		if _, ok := usedPorts[i]; !ok {
			port = i
			break
		}
	}

	log.Printf("Allocated port %d for server %s", port, info.Name)

	// create a fake mc server
	info.fakeMcServer = NewFakeMcServer(info.Name,
		viper.GetString("messages.motd"),
		viper.GetString("messages.kick"),
		port)

	info.quit = make(chan struct{})

	// update dns
	w.dnsServer.AddServiceRecord(GetMinecraftSrvRecord(info.SubDomain),
		fmt.Sprintf("%s:%d", w.GetDispatchServer(), port))
	w.dnsServer.RemoveARecord(info.Name)

	w.serverMap[info.Name] = info

	lastLogin := time.Now()

	info.fakeMcServer.OnLogin(func() {
		// 每 10 s 处理一次登录
		if time.Since(lastLogin) < 10*time.Second {
			return
		}
		lastLogin = time.Now()

		go w.StartServer(info.Name)

	})

	go func() {
		<-info.quit
		info.fakeMcServer.Stop()
	}()
	go func() {
		info.fakeMcServer.Start()
	}()
}

func (w *DispatchWorker) checkServers() {
	log.Println("Checking servers")
	for name, info := range w.serverMap {
		// 判断 vm 是否还在运行
		status, err := info.VM.GetStatus()
		if err != nil {
			log.Printf("Failed to get status of server %s: %s", name, err)
			continue
		}

		if status.Status == vm.VM_STATUS_RUNNING {
			if time.Since(info.lastActivity) > 5*time.Minute {
				log.Printf("Server %s has been inactive for 10 minutes, stopping", name)
				w.StopServer(name)
			}
			// ping mc 服务器，更新最后活动时间
			_, _, err := bot.PingAndListTimeout(fmt.Sprintf("%s:%d", status.Ip, info.Port), 5*time.Second)
			if err != nil {
				log.Printf("Failed to ping server %s: %s", name, err)
				continue
			}

			info.lastActivity = time.Now()
		}
	}
}

func (w *DispatchWorker) background() {
	ticker := time.NewTicker(1 * time.Minute)
	for {
		select {
		case <-w.quit:
			return
		case <-ticker.C:
			w.checkServers()
		}
	}
}

func (w *DispatchWorker) RemoveServer(name string) {
	info := w.serverMap[name]
	if info == nil {
		return
	}
	close(info.quit)
	// 停止 vm
	info.VM.Stop()
	// 删除 dns 记录
	w.dnsServer.RemoveServiceRecord(GetMinecraftSrvRecord(info.SubDomain))
	w.dnsServer.RemoveARecord(info.Name)
	delete(w.serverMap, name)

}

func (w *DispatchWorker) Start() error {
	log.Println("Starting dispatch worker")
	go w.background()
	// add A record for dispatch server
	w.dnsServer.AddARecord(
		"dispatch",
		w.options.PublicIp)

	err := w.dnsServer.Start()
	if err != nil {
		return err
	}

	return nil
}

func (w *DispatchWorker) Stop() {

	w.dnsServer.RemoveARecord(w.GetDispatchServer())

	w.dnsServer.Stop()
	// stop all servers
	for _, info := range w.serverMap {
		w.RemoveServer(info.Name)
	}
}
