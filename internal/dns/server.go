package dns

import (
	"log"
	"net"
	"strconv"

	"github.com/miekg/dns"
)

type DnsServer struct {
	ns         *dns.Server
	ttl        int
	rootDomain string
	srvRecords map[string]string
	aRecords   map[string]string
}

func NewDnsServer(ttl int, rootDomain string) *DnsServer {
	// if rootDomain dose not end with a dot, add it
	if rootDomain[len(rootDomain)-1] != '.' {
		rootDomain += "."
	}

	return &DnsServer{
		ttl:        ttl,
		rootDomain: rootDomain,
		srvRecords: make(map[string]string),
		aRecords:   make(map[string]string),
	}
}

func (s *DnsServer) dnsHandler(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = false

	// only handle SRV query
	if len(r.Question) != 1 {
		m.Rcode = dns.RcodeNameError
		w.WriteMsg(m)
		return
	}

	// get the service name
	recordName := r.Question[0].Name

	if len(recordName) <= len(s.rootDomain) {
		m.Rcode = dns.RcodeNameError
		w.WriteMsg(m)
		return
	}

	if recordName[len(recordName)-1] != '.' {
		recordName += "."
	}

	// remove the root domain
	recordName = recordName[:len(recordName)-len(s.rootDomain)-1]

	if r.Question[0].Qtype == dns.TypeSRV {
		// get the service record
		serviceRecord, ok := s.srvRecords[recordName]
		if !ok {
			m.Rcode = dns.RcodeNameError
			w.WriteMsg(m)
			return
		}

		// parse the service record
		host, port, err := net.SplitHostPort(serviceRecord)
		if err != nil {
			m.Rcode = dns.RcodeServerFailure
			w.WriteMsg(m)
			return
		}

		// add dot to the host
		if host[len(host)-1] != '.' {
			host += "."
		}

		log.Printf("host: %s, port: %s\n", host, port)
		intport, _ := strconv.Atoi(port)
		// create SRV record
		srv := &dns.SRV{
			Hdr:      dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeSRV, Class: dns.ClassINET, Ttl: uint32(s.ttl)},
			Priority: 10,
			Weight:   10,
			Port:     uint16(intport),
			Target:   host,
		}

		log.Printf("SRV record for %s: %s\n", recordName, serviceRecord)

		m.Answer = append(m.Answer, srv)
	} else if r.Question[0].Qtype == dns.TypeA {
		// get the A record
		ip, ok := s.aRecords[recordName]
		if !ok {
			m.Rcode = dns.RcodeNameError
		} else {
			// response with A record
			m.Answer = append(m.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: uint32(s.ttl)},
				A:   net.ParseIP(ip),
			})
			log.Println(m.Answer)
			log.Printf("A record for %s: %s\n", recordName, ip)
		}
	} else {
		m.Rcode = dns.RcodeNameError
	}

	err := w.WriteMsg(m)

	if err != nil {
		log.Printf("Failed to write DNS response: %s\n", err.Error())
	}

	log.Printf("Received DNS query for %s\n", recordName)

}

func (s *DnsServer) Start() error {
	s.ns = &dns.Server{
		Addr:      ":53",
		Net:       "udp",
		Handler:   dns.HandlerFunc(s.dnsHandler),
		UDPSize:   65536,
		ReusePort: true,
	}

	log.Println("Starting DNS server")
	err := s.ns.ListenAndServe()
	if err != nil {
		log.Fatalf("Failed to start DNS server: %s\n", err.Error())
		return err
	}

	return nil
}

func (s *DnsServer) Stop() {
	s.ns.Shutdown()
}

// AddServiceRecord adds a service record to the DNS server. reconds is in the format of "host:port"
func (s *DnsServer) AddServiceRecord(serviceName string, record string) {
	log.Printf("Adding service record for %s: %s\n", serviceName, record)
	s.srvRecords[serviceName] = record
}

// RemoveServiceRecord removes a service record from the DNS server
func (s *DnsServer) RemoveServiceRecord(serviceName string) {
	delete(s.srvRecords, serviceName)
}

// AddARecord adds an A record to the DNS server
func (s *DnsServer) AddARecord(name string, ip string) {
	log.Printf("Adding A record for %s: %s\n", name, ip)
	s.aRecords[name] = ip
}

// RemoveARecord removes an A record from the DNS server
func (s *DnsServer) RemoveARecord(name string) {
	delete(s.aRecords, name)
}
