package dns

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/miekg/dns"
)

// Server represents a DNS server that intercepts queries for Ollama registry
type Server struct {
	addr         string
	port         int
	upstreamDNS  string
	registryHost string
	redirectIP   string
	server       *dns.Server
}

// NewServer creates a new DNS server instance
func NewServer(addr string, port int, upstreamDNS, registryHost, redirectIP string) *Server {
	return &Server{
		addr:         addr,
		port:         port,
		upstreamDNS:  upstreamDNS,
		registryHost: registryHost,
		redirectIP:   redirectIP,
	}
}

// Start starts the DNS server
func (s *Server) Start(ctx context.Context) error {
	mux := dns.NewServeMux()
	mux.HandleFunc(".", s.handleDNSRequest)

	s.server = &dns.Server{
		Addr:    fmt.Sprintf("%s:%d", s.addr, s.port),
		Net:     "udp",
		Handler: mux,
	}

	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- s.server.ListenAndServe()
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		log.Println("DNS server shutting down...")
		return s.server.Shutdown()
	case err := <-errChan:
		return fmt.Errorf("DNS server failed: %w", err)
	}
}

// handleDNSRequest handles incoming DNS requests
func (s *Server) handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	msg := dns.Msg{}
	msg.SetReply(r)
	msg.Authoritative = true

	for _, question := range r.Question {
		switch question.Qtype {
		case dns.TypeA:
			domain := strings.TrimSuffix(question.Name, ".")
			
			// Check if this is the Ollama registry we want to intercept
			if domain == s.registryHost {
				log.Printf("Intercepting DNS query for %s, redirecting to %s", domain, s.redirectIP)
				
				// Parse the redirect IP
				ip := net.ParseIP(s.redirectIP)
				if ip == nil {
					log.Printf("Invalid redirect IP: %s", s.redirectIP)
					s.forwardToUpstream(w, r)
					return
				}
				
				// Create A record pointing to our cache server
				record := &dns.A{
					Hdr: dns.RR_Header{
						Name:   question.Name,
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    300, // 5 minutes TTL
					},
					A: ip.To4(),
				}
				msg.Answer = append(msg.Answer, record)
			} else {
				// Forward other queries to upstream DNS
				s.forwardToUpstream(w, r)
				return
			}
			
		default:
			// Forward non-A record queries to upstream DNS
			s.forwardToUpstream(w, r)
			return
		}
	}

	if err := w.WriteMsg(&msg); err != nil {
		log.Printf("Failed to write DNS response: %v", err)
	}
}

// forwardToUpstream forwards DNS queries to the upstream DNS server
func (s *Server) forwardToUpstream(w dns.ResponseWriter, r *dns.Msg) {
	client := &dns.Client{
		Net: "udp",
	}

	resp, _, err := client.Exchange(r, s.upstreamDNS)
	if err != nil {
		log.Printf("Failed to forward DNS query to upstream %s: %v", s.upstreamDNS, err)
		
		// Send a SERVFAIL response
		msg := &dns.Msg{}
		msg.SetReply(r)
		msg.SetRcode(r, dns.RcodeServerFailure)
		w.WriteMsg(msg)
		return
	}

	if err := w.WriteMsg(resp); err != nil {
		log.Printf("Failed to write forwarded DNS response: %v", err)
	}
}
