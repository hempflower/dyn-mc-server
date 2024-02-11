package main

import (
	"log"
	"os"
	"os/signal"

	"github.com/hempflower/dmcs/internal/dispatch"
	"github.com/hempflower/dmcs/internal/vm"
	"github.com/spf13/viper"
)

func main() {
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	viper.SetConfigType("json")
	viper.ReadInConfig()

	log.Println("Starting dispatch worker")

	options := &dispatch.DispatchWorkerOptions{
		DnsTtl:     viper.GetInt("dns.ttl"),
		RootDomain: viper.GetString("dns.rootDomain"),
		StartPort:  viper.GetInt("dispatch.startPort"),
		EndPort:    viper.GetInt("dispatch.endPort"),
		PublicIp:   viper.GetString("dispatch.ip"),
	}

	var servers []struct {
		Name      string `json:"name"`
		SubDomain string `json:"subDomain"`
		Port      int    `json:"port"`
		Vm        struct {
			Type    string                 `json:"type"`
			Options map[string]interface{} `json:"options"`
		} `json:"vm"`
	}
	viper.UnmarshalKey("servers", &servers)
	worker := dispatch.NewDispatchWorker(options)
	for _, server := range servers {
		vmProvider, err := vm.NewVmProvider(server.Vm.Type, server.Vm.Options)
		if err != nil {
			log.Fatalf("Failed to create vm provider: %s", err)
		}

		worker.AddServer(&dispatch.MCServerInfo{
			Name:      server.Name,
			SubDomain: server.SubDomain,
			Port:      server.Port,
			VM:        vmProvider,
		})
	}
	go func() {
		worker.Start()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	worker.Stop()
	log.Println("Dispatch worker stopped")
}
