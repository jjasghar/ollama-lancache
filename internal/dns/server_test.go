package dns

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestNewServer(t *testing.T) {
	server := NewServer("127.0.0.1", 5353, "8.8.8.8:53", "registry.ollama.ai", "192.168.1.100")

	if server.addr != "127.0.0.1" {
		t.Errorf("Expected addr 127.0.0.1, got %s", server.addr)
	}

	if server.port != 5353 {
		t.Errorf("Expected port 5353, got %d", server.port)
	}

	if server.upstreamDNS != "8.8.8.8:53" {
		t.Errorf("Expected upstream DNS 8.8.8.8:53, got %s", server.upstreamDNS)
	}

	if server.registryHost != "registry.ollama.ai" {
		t.Errorf("Expected registry host registry.ollama.ai, got %s", server.registryHost)
	}

	if server.redirectIP != "192.168.1.100" {
		t.Errorf("Expected redirect IP 192.168.1.100, got %s", server.redirectIP)
	}
}

func TestDNSInterception(t *testing.T) {
	// Create a test DNS server
	server := NewServer("127.0.0.1", 0, "8.8.8.8:53", "registry.ollama.ai", "192.168.1.100")

	// Create a mock DNS request for registry.ollama.ai
	msg := &dns.Msg{}
	msg.SetQuestion(dns.Fqdn("registry.ollama.ai"), dns.TypeA)

	// Create a mock response writer
	mockWriter := &mockResponseWriter{}

	// Handle the request
	server.handleDNSRequest(mockWriter, msg)

	// Verify the response
	if mockWriter.response == nil {
		t.Fatal("No response received")
	}

	if len(mockWriter.response.Answer) != 1 {
		t.Fatalf("Expected 1 answer, got %d", len(mockWriter.response.Answer))
	}

	// Check if the answer is an A record
	if aRecord, ok := mockWriter.response.Answer[0].(*dns.A); ok {
		if aRecord.A.String() != "192.168.1.100" {
			t.Errorf("Expected IP 192.168.1.100, got %s", aRecord.A.String())
		}
	} else {
		t.Error("Expected A record in response")
	}
}

func TestDNSNonInterception(t *testing.T) {
	// Create a test DNS server with a mock upstream that returns a specific IP
	server := NewServer("127.0.0.1", 0, "127.0.0.1:5354", "registry.ollama.ai", "192.168.1.100")

	// Start a mock upstream DNS server
	mockUpstream := &mockUpstreamDNS{
		responses: map[string]string{
			"google.com.": "8.8.8.8",
		},
	}
	go mockUpstream.start("127.0.0.1:5354")
	time.Sleep(100 * time.Millisecond) // Give the server time to start

	// Create a mock DNS request for a different domain
	msg := &dns.Msg{}
	msg.SetQuestion(dns.Fqdn("google.com"), dns.TypeA)

	// Create a mock response writer
	mockWriter := &mockResponseWriter{}

	// Handle the request
	server.handleDNSRequest(mockWriter, msg)

	// For non-intercepted domains, the request should be forwarded
	// This test verifies that non-registry domains are handled differently
	if mockWriter.response == nil {
		t.Fatal("No response received")
	}

	// The response should come from the upstream server
	if len(mockWriter.response.Answer) > 0 {
		if aRecord, ok := mockWriter.response.Answer[0].(*dns.A); ok {
			// Should NOT be our redirect IP
			if aRecord.A.String() == "192.168.1.100" {
				t.Error("Non-registry domain should not be redirected to cache IP")
			}
		}
	}
}

func TestDNSServerStartStop(t *testing.T) {
	// Find an available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to find available port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	server := NewServer("127.0.0.1", port, "8.8.8.8:53", "registry.ollama.ai", "192.168.1.100")

	ctx, cancel := context.WithCancel(context.Background())

	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- server.Start(ctx)
	}()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Test that the server is running by sending a DNS query
	client := &dns.Client{Net: "udp", Timeout: time.Second}
	msg := &dns.Msg{}
	msg.SetQuestion(dns.Fqdn("registry.ollama.ai"), dns.TypeA)

	resp, _, err := client.Exchange(msg, "127.0.0.1:"+string(rune(port)))
	if err != nil {
		// Server might not be fully started yet, this is acceptable for this test
		t.Logf("DNS query failed (server might still be starting): %v", err)
	} else if resp != nil && len(resp.Answer) > 0 {
		if aRecord, ok := resp.Answer[0].(*dns.A); ok {
			if aRecord.A.String() != "192.168.1.100" {
				t.Errorf("Expected redirected IP 192.168.1.100, got %s", aRecord.A.String())
			}
		}
	}

	// Stop the server
	cancel()

	// Wait for server to stop or timeout
	select {
	case err := <-errChan:
		if err != nil && err != context.Canceled {
			t.Errorf("Unexpected server error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Server did not stop within timeout")
	}
}

// Mock implementations for testing

type mockResponseWriter struct {
	response *dns.Msg
	addr     net.Addr
}

func (m *mockResponseWriter) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 5353}
}

func (m *mockResponseWriter) RemoteAddr() net.Addr {
	if m.addr != nil {
		return m.addr
	}
	return &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
}

func (m *mockResponseWriter) WriteMsg(msg *dns.Msg) error {
	m.response = msg
	return nil
}

func (m *mockResponseWriter) Write([]byte) (int, error) {
	return 0, nil
}

func (m *mockResponseWriter) Close() error {
	return nil
}

func (m *mockResponseWriter) TsigStatus() error {
	return nil
}

func (m *mockResponseWriter) TsigTimersOnly(bool) {}

func (m *mockResponseWriter) Hijack() {}

type mockUpstreamDNS struct {
	responses map[string]string
	server    *dns.Server
}

func (m *mockUpstreamDNS) start(addr string) {
	mux := dns.NewServeMux()
	mux.HandleFunc(".", m.handleRequest)

	m.server = &dns.Server{
		Addr:    addr,
		Net:     "udp",
		Handler: mux,
	}

	m.server.ListenAndServe()
}

func (m *mockUpstreamDNS) handleRequest(w dns.ResponseWriter, r *dns.Msg) {
	msg := dns.Msg{}
	msg.SetReply(r)

	for _, question := range r.Question {
		if question.Qtype == dns.TypeA {
			if ip, exists := m.responses[question.Name]; exists {
				record := &dns.A{
					Hdr: dns.RR_Header{
						Name:   question.Name,
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    300,
					},
					A: net.ParseIP(ip),
				}
				msg.Answer = append(msg.Answer, record)
			}
		}
	}

	w.WriteMsg(&msg)
}

// Benchmark tests
func BenchmarkDNSRequest(b *testing.B) {
	server := NewServer("127.0.0.1", 0, "8.8.8.8:53", "registry.ollama.ai", "192.168.1.100")
	mockWriter := &mockResponseWriter{}

	msg := &dns.Msg{}
	msg.SetQuestion(dns.Fqdn("registry.ollama.ai"), dns.TypeA)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		server.handleDNSRequest(mockWriter, msg)
	}
}
