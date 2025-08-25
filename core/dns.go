package core

import (
	"log"
	"net"
	"strings"

	"github.com/miekg/dns"
)

func (d *DNSServer) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true
	for _, q := range r.Question {
		name := strings.TrimSuffix(strings.ToLower(q.Name), ".")
		switch q.Qtype {
		case dns.TypeA:
			if d.isLocal(name) {
				m.Answer = append(m.Answer, &dns.A{Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 5}, A: net.ParseIP("127.0.0.1")})
			} else {
				d.forward(name, q.Qtype, m)
			}
		case dns.TypeAAAA:
			if d.isLocal(name) {
				m.Answer = append(m.Answer, &dns.AAAA{Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 5}, AAAA: net.ParseIP("::1")})
			} else {
				d.forward(name, q.Qtype, m)
			}
		default:
			// minimal: forward others
			d.forward(name, q.Qtype, m)
		}
	}
	_ = w.WriteMsg(m)
}

func (d *DNSServer) forward(name string, qtype uint16, m *dns.Msg) {
	if d.forwardTo == "" {
		d.forwardTo = "1.1.1.1:53"
	}
	c := new(dns.Client)
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(name), qtype)
	if resp, _, err := c.Exchange(msg, d.forwardTo); err == nil && resp != nil {
		m.Answer = append(m.Answer, resp.Answer...)
	}
}

func ServeDNS(bind string, isLocal func(string) bool, forwardTo string) {
	s := &DNSServer{isLocal: isLocal, forwardTo: forwardTo}
	dns.HandleFunc(".", s.ServeDNS)
	udpServer := &dns.Server{Addr: bind, Net: "udp"}
	cpServer := &dns.Server{Addr: bind, Net: "tcp"}
	go func() { log.Printf("DNS(udp) listening on %s", bind); log.Fatal(udpServer.ListenAndServe()) }()
	go func() { log.Printf("DNS(tcp) listening on %s", bind); log.Fatal(cpServer.ListenAndServe()) }()
}
