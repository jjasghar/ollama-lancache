package proxy

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jjasghar/ollama-lancache/internal/cache"
)

// Server represents an HTTP proxy server for caching Ollama models
type Server struct {
	addr        string
	port        int
	cache       *cache.Cache
	server      *http.Server
	client      *http.Client
	upstreamURL *url.URL
}

var (
	// Regex patterns for Ollama API endpoints
	manifestPattern = regexp.MustCompile(`^/v2/([^/]+)/([^/]+)/manifests/(.+)$`)
	blobPattern     = regexp.MustCompile(`^/v2/([^/]+)/([^/]+)/blobs/(sha256:[a-f0-9]{64})$`)
)

// NewServer creates a new HTTP proxy server instance
func NewServer(addr string, port int, modelCache *cache.Cache) *Server {
	upstreamURL, _ := url.Parse("https://registry.ollama.ai")
	
	return &Server{
		addr:  addr,
		port:  port,
		cache: modelCache,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		upstreamURL: upstreamURL,
	}
}

// Start starts the HTTP proxy server
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	
	// Handle all requests
	mux.HandleFunc("/", s.handleRequest)
	
	// Health check endpoint
	mux.HandleFunc("/health", s.handleHealth)
	
	// Cache stats endpoint
	mux.HandleFunc("/cache/stats", s.handleCacheStats)
	
	// Direct blob serving endpoint (for redirects)
	mux.HandleFunc("/blobs/", s.handleDirectBlob)

	s.server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", s.addr, s.port),
		Handler: mux,
		ErrorLog: log.New(log.Writer(), "", log.LstdFlags),
	}
	
	// Note: Using HTTP for better compatibility with self-signed certificates

	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if s.port == 443 {
			log.Printf("HTTPS proxy listening on %s (with self-signed cert for registry.ollama.ai)", s.server.Addr)
			
			// Generate self-signed certificate for registry.ollama.ai
			cert, err := s.generateSelfSignedCert()
			if err != nil {
				errChan <- fmt.Errorf("failed to generate certificate: %w", err)
				return
			}
			
			s.server.TLSConfig = &tls.Config{
				Certificates: []tls.Certificate{cert},
				MinVersion:   tls.VersionTLS12,
				MaxVersion:   tls.VersionTLS13,
				CipherSuites: []uint16{
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				},
				PreferServerCipherSuites: true,
				InsecureSkipVerify:      false, // We want to serve a cert, even if self-signed
			}
			
			errChan <- s.server.ListenAndServeTLS("", "")
		} else {
			log.Printf("HTTP proxy listening on %s", s.server.Addr)
			errChan <- s.server.ListenAndServe()
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		log.Println("HTTP proxy shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(shutdownCtx)
	case err := <-errChan:
		if err == http.ErrServerClosed {
			return nil
		}
		return fmt.Errorf("HTTP proxy failed: %w", err)
	}
}

// handleRequest handles all incoming HTTP requests
func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.Header.Get("X-Real-IP")
	}
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}

	// Enhanced logging for debugging client issues
	log.Printf("🌐 [%s] %s %s %s User-Agent: %s", clientIP, r.Method, r.Host, r.URL.Path, r.Header.Get("User-Agent"))
	log.Printf("🔍 [%s] Full URL: %s, Content-Length: %s, Accept: %s", 
		clientIP, r.URL.String(), r.Header.Get("Content-Length"), r.Header.Get("Accept"))

	// Handle Ollama API endpoints
	if r.URL.Path == "/api/pull" {
		log.Printf("🔥 [%s] Ollama pull request detected", clientIP)
		s.handleOllamaPull(w, r)
		return
	}
	
	if r.URL.Path == "/api/tags" {
		log.Printf("📋 [%s] Ollama tags request detected", clientIP)
		s.handleOllamaTags(w, r)
		return
	}
	
	if r.URL.Path == "/api/generate" {
		log.Printf("🚫 [%s] Ollama generate request blocked - this is a cache server", clientIP)
		s.handleUnsupportedOperation(w, r, "generate")
		return
	}
	
	if r.URL.Path == "/api/chat" {
		log.Printf("🚫 [%s] Ollama chat request blocked - this is a cache server", clientIP)
		s.handleUnsupportedOperation(w, r, "chat")
		return
	}
	
	// Handle other Ollama API endpoints that aren't pull/tags
	if strings.HasPrefix(r.URL.Path, "/api/") {
		log.Printf("🚫 [%s] Other Ollama API request blocked: %s", clientIP, r.URL.Path)
		s.handleUnsupportedOperation(w, r, strings.TrimPrefix(r.URL.Path, "/api/"))
		return
	}

	// Handle Docker Registry v2 API root endpoint
	if r.URL.Path == "/v2/" {
		s.handleRegistryRoot(w, r)
		return
	}

	// Check if this is a manifest request
	if matches := manifestPattern.FindStringSubmatch(r.URL.Path); matches != nil {
		s.handleManifestRequest(w, r, matches[1], matches[2], matches[3])
		return
	}

	// Check if this is a blob request
	if matches := blobPattern.FindStringSubmatch(r.URL.Path); matches != nil {
		s.handleBlobRequest(w, r, matches[1], matches[2], matches[3])
		return
	}

	// For all other requests, proxy to upstream
	s.proxyToUpstream(w, r)
}

// handleManifestRequest handles manifest requests
func (s *Server) handleManifestRequest(w http.ResponseWriter, r *http.Request, namespace, model, tag string) {
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.Header.Get("X-Real-IP")
	}
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}
	
	registry := "registry.ollama.ai"
	
	// Try to serve from cache first
	if (r.Method == "GET" || r.Method == "HEAD") && s.cache.HasManifest(registry, namespace, model, tag) {
		log.Printf("💾 CACHE HIT [%s] Serving manifest: %s/%s:%s", clientIP, namespace, model, tag)
		
		manifest, err := s.cache.GetManifest(registry, namespace, model, tag)
		if err != nil {
			log.Printf("Error reading cached manifest: %v", err)
			s.proxyToUpstream(w, r)
			return
		}
		
		w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
		w.Header().Set("Docker-Content-Digest", calculateManifestDigest(manifest))
		
		if r.Method == "HEAD" {
			// For HEAD requests, just send headers without body
			w.WriteHeader(http.StatusOK)
			return
		}
		
		// For GET requests, send the manifest content
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(manifest); err != nil {
			log.Printf("Error encoding manifest: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	// If not in cache or not a GET request, fetch from upstream and cache it
	if r.Method == "GET" {
		s.fetchAndCacheManifest(w, r, registry, namespace, model, tag)
	} else {
		s.proxyToUpstream(w, r)
	}
}

// handleBlobRequest handles blob requests
func (s *Server) handleBlobRequest(w http.ResponseWriter, r *http.Request, namespace, model, digest string) {
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.Header.Get("X-Real-IP")
	}
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}
	// Try to serve from cache first
	if s.cache.HasBlob(digest) {
						if r.Method == "HEAD" {
			log.Printf("💾 CACHE HIT [%s] HEAD request for blob: %s", clientIP, digest)
			
			// HEAD requests should also be served directly like the real registry
			s.serveBlobDirectFromV2(w, r, digest, clientIP)
			return
		}
		
						if r.Method == "GET" {
			userAgent := r.Header.Get("User-Agent")

			// Log all headers for debugging
			log.Printf("🔍 [%s] Headers: User-Agent=%s, Accept=%s, Range=%s", 
				clientIP, userAgent, r.Header.Get("Accept"), r.Header.Get("Range"))
			
			// The real registry.ollama.ai serves blobs DIRECTLY with HTTP/2 200 and Content-Length
			// NO REDIRECTS - serve directly from /v2/ endpoint like the real registry
			log.Printf("💾 CACHE HIT [%s] Serving blob DIRECTLY from /v2/ like real registry: %s", clientIP, digest)
			
			s.serveBlobDirectFromV2(w, r, digest, clientIP)
			return
		}
	}

	// If not in cache or not a GET request, fetch from upstream and cache it
	if r.Method == "GET" {
		s.fetchAndCacheBlob(w, r, digest)
	} else {
		s.proxyToUpstream(w, r)
	}
}

// fetchAndCacheManifest fetches a manifest from upstream and caches it
func (s *Server) fetchAndCacheManifest(w http.ResponseWriter, r *http.Request, registry, namespace, model, tag string) {
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.Header.Get("X-Real-IP")
	}
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}
	
	log.Printf("🌍 CACHE MISS [%s] Fetching manifest from upstream: %s/%s:%s", clientIP, namespace, model, tag)
	
	// Create upstream request
	upstreamURL := *s.upstreamURL
	upstreamURL.Path = r.URL.Path
	upstreamURL.RawQuery = r.URL.RawQuery
	
	upstreamReq, err := http.NewRequest(r.Method, upstreamURL.String(), nil)
	if err != nil {
		log.Printf("Error creating upstream request: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	
	// Copy headers
	for key, values := range r.Header {
		for _, value := range values {
			upstreamReq.Header.Add(key, value)
		}
	}
	
	// Make upstream request
	resp, err := s.client.Do(upstreamReq)
	if err != nil {
		log.Printf("Error fetching from upstream: %v", err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	
	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	
	// If successful, parse and cache the manifest
	if resp.StatusCode == http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Error reading upstream response: %v", err)
			return
		}
		
		var manifest cache.Manifest
		if err := json.Unmarshal(body, &manifest); err != nil {
			log.Printf("Error parsing manifest: %v", err)
			w.Write(body) // Still send the response even if we can't cache it
			return
		}
		
		// Cache the manifest
		if err := s.cache.StoreManifest(registry, namespace, model, tag, &manifest); err != nil {
			log.Printf("❌ [%s] Error caching manifest: %v", clientIP, err)
		} else {
			log.Printf("✅ [%s] Cached manifest: %s/%s:%s", clientIP, namespace, model, tag)
		}
		
		// Send the response
		w.Write(body)
	} else {
		// Just copy the error response
		io.Copy(w, resp.Body)
	}
}

// fetchAndCacheBlob fetches a blob from upstream and caches it
func (s *Server) fetchAndCacheBlob(w http.ResponseWriter, r *http.Request, digest string) {
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.Header.Get("X-Real-IP")
	}
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}
	
	log.Printf("🌍 CACHE MISS [%s] Fetching blob from upstream: %s", clientIP, digest)
	
	// Create upstream request
	upstreamURL := *s.upstreamURL
	upstreamURL.Path = r.URL.Path
	upstreamURL.RawQuery = r.URL.RawQuery
	
	upstreamReq, err := http.NewRequest(r.Method, upstreamURL.String(), nil)
	if err != nil {
		log.Printf("Error creating upstream request: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	
	// Copy headers
	for key, values := range r.Header {
		for _, value := range values {
			upstreamReq.Header.Add(key, value)
		}
	}
	
	// Make upstream request
	resp, err := s.client.Do(upstreamReq)
	if err != nil {
		log.Printf("Error fetching from upstream: %v", err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	
	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	
	// If successful, stream and cache the blob
	if resp.StatusCode == http.StatusOK {
		// Create a pipe to tee the response to both the client and cache
		pr, pw := io.Pipe()
		
		// Start caching in a goroutine
		go func() {
			defer pw.Close()
			if err := s.cache.StoreBlob(digest, pr); err != nil {
				log.Printf("❌ [%s] Error caching blob %s: %v", clientIP, digest, err)
			} else {
				log.Printf("✅ [%s] Cached blob: %s", clientIP, digest)
			}
		}()
		
		// Tee the response to both the client and the pipe
		teeReader := io.TeeReader(resp.Body, pw)
		if _, err := io.Copy(w, teeReader); err != nil {
			log.Printf("Error streaming blob: %v", err)
		}
		
		pw.Close() // Ensure the pipe is closed
	} else {
		// Just copy the error response
		io.Copy(w, resp.Body)
	}
}

// proxyToUpstream proxies requests to the upstream server
func (s *Server) proxyToUpstream(w http.ResponseWriter, r *http.Request) {
	upstreamURL := *s.upstreamURL
	upstreamURL.Path = r.URL.Path
	upstreamURL.RawQuery = r.URL.RawQuery
	
	upstreamReq, err := http.NewRequest(r.Method, upstreamURL.String(), r.Body)
	if err != nil {
		log.Printf("Error creating upstream request: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	
	// Copy headers
	for key, values := range r.Header {
		for _, value := range values {
			upstreamReq.Header.Add(key, value)
		}
	}
	
	resp, err := s.client.Do(upstreamReq)
	if err != nil {
		log.Printf("Error proxying to upstream: %v", err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	
	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	
	// Copy response body
	io.Copy(w, resp.Body)
}

// handleRegistryRoot handles Docker Registry v2 API root endpoint
func (s *Server) handleRegistryRoot(w http.ResponseWriter, r *http.Request) {
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.Header.Get("X-Real-IP")
	}
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}

	log.Printf("🐳 [%s] Docker Registry v2 API root request", clientIP)

	// Return standard Docker Registry v2 API response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	w.WriteHeader(http.StatusOK)
	
	// Empty JSON response indicates v2 API support
	w.Write([]byte("{}"))
	
	log.Printf("✅ [%s] Responded to registry v2 API check", clientIP)
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.Header.Get("X-Real-IP")
	}
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}
	
	log.Printf("✅ [%s] Health check request", clientIP)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// serveBlobRegistry serves a blob using Ollama-compatible chunked strategy
func (s *Server) serveBlobRegistry(w http.ResponseWriter, r *http.Request, digest, clientIP string) {
	// Handle OPTIONS requests for CORS
	if r.Method == "OPTIONS" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Docker-Content-Digest")
		w.WriteHeader(http.StatusOK)
		return
	}

	reader, err := s.cache.GetBlob(digest)
	if err != nil {
		log.Printf("❌ [%s] Error reading blob: %v", clientIP, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer reader.Close()

	// Get the blob size
	blobSize := s.getBlobSize(digest)
	
	// Handle range requests properly first
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" {
		log.Printf("📏 [%s] Processing range request: %s", clientIP, rangeHeader)
		s.serveBlobDirect(w, r, digest, clientIP)
		return
	}
	
	// For Ollama's non-range requests, use a special streaming strategy
	userAgent := r.Header.Get("User-Agent")
	if strings.Contains(userAgent, "ollama") {
		log.Printf("🎯 [%s] Ollama client detected - using adaptive chunked strategy", clientIP)
		s.serveBlobOllamaStrategy(w, r, reader, digest, blobSize, clientIP)
		return
	}
	
	// For other clients, serve normally
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Docker-Content-Digest", digest)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", blobSize))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	
	w.WriteHeader(http.StatusOK)
	
	log.Printf("📏 [%s] Serving complete blob to non-Ollama client: %d bytes", clientIP, blobSize)

	// Stream the complete file
	written, err := io.Copy(w, reader)
	if err != nil {
		log.Printf("❌ [%s] Error streaming blob after %d bytes: %v", clientIP, written, err)
		return
	}

	log.Printf("✅ [%s] Successfully served blob: %s (%d bytes)", clientIP, digest[:12]+"...", written)
}

// serveBlobOllamaStrategy implements the strategy that Ollama 0.11.7 expects
func (s *Server) serveBlobOllamaStrategy(w http.ResponseWriter, r *http.Request, reader io.ReadCloser, digest string, blobSize int64, clientIP string) {
	// Set headers that indicate we support resumable downloads
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Docker-Content-Digest", digest)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	
	// The key insight: DO NOT set Content-Length for initial download
	// This forces Ollama to use chunked/connection-based downloading
	log.Printf("🎯 [%s] Using Ollama-compatible strategy: no Content-Length, chunked encoding", clientIP)
	
	// Use chunked transfer encoding (HTTP/1.1 default when no Content-Length)
	w.Header().Set("Transfer-Encoding", "chunked")
	
	w.WriteHeader(http.StatusOK)
	
	// Create a limited buffer for controlled sending
	const chunkSize = 64 * 1024 // 64KB chunks
	buffer := make([]byte, chunkSize)
	
	var totalWritten int64
	chunkCount := 0
	
	for {
		// Read a chunk from the file
		n, err := reader.Read(buffer)
		if err != nil && err != io.EOF {
			log.Printf("❌ [%s] Error reading blob chunk: %v", clientIP, err)
			return
		}
		
		if n == 0 {
			break // End of file
		}
		
		// Write the chunk
		written, writeErr := w.Write(buffer[:n])
		if writeErr != nil {
			log.Printf("❌ [%s] Error writing chunk %d after %d bytes: %v", clientIP, chunkCount, totalWritten, writeErr)
			return
		}
		
		totalWritten += int64(written)
		chunkCount++
		
		// Force flush after each chunk
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		
		// Log progress periodically
		if chunkCount%10 == 0 || err == io.EOF {
			log.Printf("📊 [%s] Sent chunk %d: %d/%d bytes (%.1f%%)", 
				clientIP, chunkCount, totalWritten, blobSize, 
				float64(totalWritten)/float64(blobSize)*100)
		}
		
		if err == io.EOF {
			break
		}
		
		// Small delay to prevent overwhelming the client
		time.Sleep(1 * time.Millisecond)
	}
	
	log.Printf("✅ [%s] Successfully served blob via Ollama strategy: %s (%d bytes in %d chunks)", 
		clientIP, digest[:12]+"...", totalWritten, chunkCount)
}

// serveBlobDirectFromV2 serves a blob directly from /v2/ endpoint like the real registry.ollama.ai
func (s *Server) serveBlobDirectFromV2(w http.ResponseWriter, r *http.Request, digest, clientIP string) {
	reader, err := s.cache.GetBlob(digest)
	if err != nil {
		log.Printf("❌ [%s] Error reading blob: %v", clientIP, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer reader.Close()

	// Get the blob size
	blobSize := s.getBlobSize(digest)
	
	// Set headers exactly like the real registry.ollama.ai (HTTP/2 200, Content-Length)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Docker-Content-Digest", digest)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", blobSize))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	
	// Handle range requests
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" {
		log.Printf("📏 [%s] Processing range request from /v2/: %s", clientIP, rangeHeader)
		s.serveBlobDirect(w, r, digest, clientIP)
		return
	}
	
	w.WriteHeader(http.StatusOK)
	
	log.Printf("📏 [%s] Serving complete blob directly from /v2/: %d bytes", clientIP, blobSize)

	// Stream the complete file using io.Copy like a real registry
	written, err := io.Copy(w, reader)
	if err != nil {
		log.Printf("❌ [%s] Error streaming blob from /v2/ after %d bytes: %v", clientIP, written, err)
		return
	}

	log.Printf("✅ [%s] Successfully served blob from /v2/: %s (%d bytes)", clientIP, digest[:12]+"...", written)
}

// serveBlobDirectOptimized serves a blob directly optimized for Ollama clients
func (s *Server) serveBlobDirectOptimized(w http.ResponseWriter, r *http.Request, digest, clientIP string) {
	// Get blob size first
	blobSize := s.getBlobSize(digest)
	if blobSize <= 0 {
		log.Printf("❌ [%s] Could not determine blob size: %s", clientIP, digest)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	reader, err := s.cache.GetBlob(digest)
	if err != nil {
		log.Printf("❌ [%s] Error reading blob: %v", clientIP, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer reader.Close()

	// Set headers for complete blob transfer
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Docker-Content-Digest", digest)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", blobSize))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Connection", "keep-alive")
	
	// No range request, serve complete file
	w.WriteHeader(http.StatusOK)
	
	log.Printf("📏 [%s] Serving complete blob: %d bytes", clientIP, blobSize)

	// Ollama expects small initial chunk, then uses Range requests
	// Send just enough to let Ollama know the blob exists, then let it use Range requests
	buffer := make([]byte, 64*1024) // 64KB initial chunk
	n, err := reader.Read(buffer)
	if n > 0 {
		bytesWritten, writeErr := w.Write(buffer[:n])
		if writeErr != nil {
			log.Printf("❌ [%s] Error writing to client: %v", clientIP, writeErr)
			return
		}
		written := int64(bytesWritten)
		
		// Flush to ensure data is sent immediately
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		
		log.Printf("✅ [%s] Sent initial chunk (%d bytes) - Ollama should now use Range requests", clientIP, written)
	} else if err != nil {
		log.Printf("❌ [%s] Error reading initial blob chunk: %v", clientIP, err)
	}
}

// serveBlobDirect serves a blob directly from cache with HTTP range support
func (s *Server) serveBlobDirect(w http.ResponseWriter, r *http.Request, digest, clientIP string) {
	// Get blob size first
	blobSize := s.getBlobSize(digest)
	if blobSize <= 0 {
		log.Printf("❌ [%s] Could not determine blob size: %s", clientIP, digest)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Handle conditional requests
	etag := fmt.Sprintf(`"%s"`, digest)
	
	// Check If-None-Match for caching
	if inm := r.Header.Get("If-None-Match"); inm != "" {
		if inm == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	
	// Parse Range header if present
	rangeHeader := r.Header.Get("Range")
	var start, end int64 = 0, blobSize - 1
	var partial bool = false

	if rangeHeader != "" && rangeHeader != "=" {
		// Check If-Range header - if present, only honor range if ETag matches
		if ifRange := r.Header.Get("If-Range"); ifRange != "" {
			if ifRange != etag {
				// If-Range doesn't match, serve full content
				log.Printf("📏 [%s] If-Range header doesn't match, serving full blob: %d bytes", clientIP, blobSize)
			} else {
				partial = true
			}
		} else {
			partial = true
		}
		
		if partial {
			// Parse range header: "bytes=start-end"
			if strings.HasPrefix(rangeHeader, "bytes=") {
				rangeSpec := strings.TrimPrefix(rangeHeader, "bytes=")
				parts := strings.Split(rangeSpec, "-")
				if len(parts) == 2 {
					if parts[0] != "" {
						if s, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
							start = s
						}
					}
					if parts[1] != "" {
						if e, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
							end = e
						}
					}
				}
			}
			log.Printf("📏 [%s] Range request: '%s' (bytes %d-%d of %d)", clientIP, rangeHeader, start, end, blobSize)
		}
	} else {
		log.Printf("📏 [%s] No range header (serving full blob: %d bytes)", clientIP, blobSize)
	}

	// Validate range
	if start < 0 || end >= blobSize || start > end {
		log.Printf("❌ [%s] Invalid range: %d-%d for blob size %d", clientIP, start, end, blobSize)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", blobSize))
		http.Error(w, "Requested Range Not Satisfiable", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	reader, err := s.cache.GetBlob(digest)
	if err != nil {
		log.Printf("❌ [%s] Error reading blob: %v", clientIP, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer reader.Close()

	// Set headers
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Docker-Content-Digest", digest)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Connection", "keep-alive")
	
	// Add resumable download support headers
	w.Header().Set("ETag", etag)
	w.Header().Set("Last-Modified", "Wed, 01 Jan 2020 00:00:00 GMT")

	contentLength := end - start + 1
	w.Header().Set("Content-Length", fmt.Sprintf("%d", contentLength))

	if partial {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, blobSize))
		w.WriteHeader(http.StatusPartialContent)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	// For HEAD requests, we're done after setting headers
	if r.Method == "HEAD" {
		if partial {
			log.Printf("📋 [%s] HEAD response for range %d-%d: 206 Partial Content", clientIP, start, end)
		} else {
			log.Printf("📋 [%s] HEAD response: 200 OK, Accept-Ranges=bytes", clientIP)
		}
		return
	}

	// Skip to start position if needed
	if start > 0 {
		if _, err := io.CopyN(io.Discard, reader, start); err != nil {
			log.Printf("❌ [%s] Error seeking to start position %d: %v", clientIP, start, err)
			return
		}
	}

	// Stream the requested range
	written := int64(0)
	buffer := make([]byte, 64*1024) // 64KB buffer
	remaining := contentLength

	for remaining > 0 {
		toRead := int64(len(buffer))
		if remaining < toRead {
			toRead = remaining
		}

		n, err := reader.Read(buffer[:toRead])
		if n > 0 {
			if _, writeErr := w.Write(buffer[:n]); writeErr != nil {
				if strings.Contains(writeErr.Error(), "broken pipe") || strings.Contains(writeErr.Error(), "connection reset") {
					log.Printf("ℹ️  [%s] Client disconnected after %d bytes of range %d-%d", clientIP, written, start, end)
				} else {
					log.Printf("❌ [%s] Error writing to client after %d bytes: %v", clientIP, written, writeErr)
				}
				return
			}
			written += int64(n)
			remaining -= int64(n)

			// Flush every 1MB
			if written%1048576 == 0 {
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			} else {
				log.Printf("❌ [%s] Error reading blob after %d bytes: %v", clientIP, written, err)
				return
			}
		}
	}

	if partial {
		log.Printf("✅ [%s] Successfully served range %d-%d (%d bytes) of %s", clientIP, start, end, written, digest[:12]+"...")
	} else {
		log.Printf("✅ [%s] Successfully served complete blob: %s (%d bytes)", clientIP, digest[:12]+"...", written)
	}
}

// handleDirectBlob handles direct blob serving (for redirected requests)
func (s *Server) handleDirectBlob(w http.ResponseWriter, r *http.Request) {
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.Header.Get("X-Real-IP")
	}
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}

	// Extract digest from URL: /blobs/sha256:abc123...
	path := strings.TrimPrefix(r.URL.Path, "/blobs/")
	digest := path

	log.Printf("📦 [%s] Direct blob %s request: %s", clientIP, r.Method, digest)

	if !s.cache.HasBlob(digest) {
		log.Printf("❌ [%s] Blob not found in cache: %s", clientIP, digest)
		http.NotFound(w, r)
		return
	}

	// Use the registry-compatible blob serving function
	s.serveBlobRegistry(w, r, digest, clientIP)
}

// handleCacheStats handles cache statistics requests
func (s *Server) handleCacheStats(w http.ResponseWriter, r *http.Request) {
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.Header.Get("X-Real-IP")
	}
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}
	
	log.Printf("📊 [%s] Cache stats request", clientIP)
	stats, err := s.cache.GetCacheStats()
	if err != nil {
		log.Printf("Error getting cache stats: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleUnsupportedOperation returns an error for operations not supported by the cache
func (s *Server) handleUnsupportedOperation(w http.ResponseWriter, r *http.Request, operation string) {
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.Header.Get("X-Real-IP")
	}
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}

	log.Printf("🚫 [%s] Blocked unsupported operation: %s", clientIP, operation)

	errorMessage := map[string]interface{}{
		"error": "This is an Ollama model cache server. Please don't use 'ollama run' or 'ollama generate' - we are trying to cache the model on your local laptop. Please use 'ollama pull' to download models to your local machine, then unset OLLAMA_HOST and run models locally with a standard Ollama installation.\n\nTo unset OLLAMA_HOST:\n• Windows (PowerShell): Remove-Variable OLLAMA_HOST -ErrorAction SilentlyContinue; $env:OLLAMA_HOST = \"\"\n• Windows (CMD): set OLLAMA_HOST=\n• Mac/Linux: unset OLLAMA_HOST",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	if err := json.NewEncoder(w).Encode(errorMessage); err != nil {
		log.Printf("❌ [%s] Error encoding unsupported operation response: %v", clientIP, err)
	}

	log.Printf("📋 [%s] Sent cache-only usage instructions", clientIP)
}

// handleOllamaTags handles Ollama's /api/tags requests (for 'ollama list')
func (s *Server) handleOllamaTags(w http.ResponseWriter, r *http.Request) {
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.Header.Get("X-Real-IP")
	}
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}

	log.Printf("📋 [%s] Retrieving cached models list", clientIP)

	// Get list of cached models from our cache
	cachedModels := s.getCachedModelsInfo()
	
	// Create Ollama-compatible response
	response := map[string]interface{}{
		"models": cachedModels,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("❌ [%s] Error encoding tags response: %v", clientIP, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ [%s] Returned %d cached models", clientIP, len(cachedModels))
}

// getCachedModelsInfo returns detailed info about cached models in Ollama format
func (s *Server) getCachedModelsInfo() []map[string]interface{} {
	manifestsDir := s.cache.GetManifestsDir()
	
	var models []map[string]interface{}
	
	// Walk through registry.ollama.ai/library/ subdirectories
	registryDir := filepath.Join(manifestsDir, "registry.ollama.ai", "library")
	entries, err := os.ReadDir(registryDir)
	if err != nil {
		log.Printf("Error reading library directory %s: %v", registryDir, err)
		return models
	}

	for _, modelEntry := range entries {
		if !modelEntry.IsDir() {
			continue
		}
		
		modelName := modelEntry.Name()
		modelDir := filepath.Join(registryDir, modelName)
		
		// Check for tags in this model directory
		tagEntries, err := os.ReadDir(modelDir)
		if err != nil {
			log.Printf("Error reading model directory %s: %v", modelDir, err)
			continue
		}
		
		for _, tagEntry := range tagEntries {
			if tagEntry.IsDir() {
				continue
			}
			
			tagName := tagEntry.Name()
			fullModelName := modelName + ":" + tagName
			
			modelInfo := map[string]interface{}{
				"name":       fullModelName,
				"model":      fullModelName,
				"modified_at": time.Now().Format("2006-01-02T15:04:05.999999999-07:00"),
				"size":       s.getModelSize(fullModelName),
				"digest":     "sha256:cached-model", // Placeholder - could compute real digest
			}
			models = append(models, modelInfo)
		}
	}

	return models
}

// getModelSize estimates the size of a cached model
func (s *Server) getModelSize(modelName string) int64 {
	// This is a simplified implementation
	// In a real implementation, you'd sum up all the blob sizes for this model
	return 4000000000 // 4GB placeholder
}

// handleOllamaPull handles Ollama's /api/pull requests
func (s *Server) handleOllamaPull(w http.ResponseWriter, r *http.Request) {
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.Header.Get("X-Real-IP")
	}
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}
	
	// Parse the request body to get model name
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("❌ [%s] Error reading pull request body: %v", clientIP, err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	
	// Log the raw body for debugging
	log.Printf("🔍 [%s] Raw request body: %s", clientIP, string(body))
	
	var pullRequest struct {
		Model  string `json:"model"`
		Name   string `json:"name"`   // Alternative field name
		Stream bool   `json:"stream"` // Ollama might send this
	}
	
	if err := json.Unmarshal(body, &pullRequest); err != nil {
		log.Printf("❌ [%s] Error parsing pull request JSON: %v", clientIP, err)
		// Still proxy even if we can't parse - let upstream handle it
		s.proxyOllamaRequest(w, r, body)
		return
	}
	
	// Try both model fields
	modelName := pullRequest.Model
	if modelName == "" {
		modelName = pullRequest.Name
	}
	
	log.Printf("📥 [%s] Ollama pull request for model: '%s' (stream: %v)", clientIP, modelName, pullRequest.Stream)
	
	// Check if we have this model cached locally
	if modelName != "" && s.hasModelCached(modelName) {
		log.Printf("💾 CACHE HIT [%s] Streaming cached model to client: %s", clientIP, modelName)
		s.streamCachedModelToClient(w, r, modelName, pullRequest.Stream)
		return
	}
	
	log.Printf("🌍 CACHE MISS [%s] Model '%s' not in cache, downloading from upstream", clientIP, modelName)
	
	// For cache misses, proxy to upstream Ollama and cache the results
	s.downloadAndCacheModel(w, r, modelName, body)
}

// proxyOllamaRequest proxies Ollama API requests to upstream
func (s *Server) proxyOllamaRequest(w http.ResponseWriter, r *http.Request, body []byte) {
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.Header.Get("X-Real-IP")
	}
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}
	
	// Create upstream request to registry.ollama.ai
	upstreamURL := "https://registry.ollama.ai" + r.URL.Path
	if r.URL.RawQuery != "" {
		upstreamURL += "?" + r.URL.RawQuery
	}
	
	upstreamReq, err := http.NewRequest(r.Method, upstreamURL, strings.NewReader(string(body)))
	if err != nil {
		log.Printf("❌ [%s] Error creating upstream request: %v", clientIP, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	
	// Copy headers
	for key, values := range r.Header {
		for _, value := range values {
			upstreamReq.Header.Add(key, value)
		}
	}
	
	log.Printf("🌍 [%s] Proxying Ollama API request to upstream", clientIP)
	
	resp, err := s.client.Do(upstreamReq)
	if err != nil {
		log.Printf("❌ [%s] Error proxying Ollama request: %v", clientIP, err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	
	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	
	// Stream the response back
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("❌ [%s] Error streaming Ollama response: %v", clientIP, err)
	} else {
		log.Printf("✅ [%s] Successfully proxied Ollama API request", clientIP)
	}
}

// generateSelfSignedCert generates a self-signed certificate for HTTPS
func (s *Server) generateSelfSignedCert() (tls.Certificate, error) {
	// Generate a private key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Ollama LanCache"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{""},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour), // Valid for 1 year
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv4(0, 0, 0, 0)},
		DNSNames:     []string{"localhost", "registry.ollama.ai"},
	}

	// Add the server's IP addresses to the certificate
	interfaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range interfaces {
			if iface.Flags&net.FlagLoopback == 0 && iface.Flags&net.FlagUp != 0 {
				addrs, err := iface.Addrs()
				if err == nil {
					for _, addr := range addrs {
						if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
							if ipNet.IP.To4() != nil {
								template.IPAddresses = append(template.IPAddresses, ipNet.IP)
							}
						}
					}
				}
			}
		}
	}

	// Create the certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}

	// Encode certificate and key
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	// Create TLS certificate
	return tls.X509KeyPair(certPEM, keyPEM)
}

// hasModelCached checks if a model is available in the local cache
func (s *Server) hasModelCached(modelName string) bool {
	// Parse model name (e.g., "granite3.3:8b" -> namespace="library", model="granite3.3", tag="8b")
	parts := strings.Split(modelName, ":")
	if len(parts) != 2 {
		return false
	}
	
	modelParts := strings.Split(parts[0], "/")
	var namespace, model string
	
	if len(modelParts) == 1 {
		namespace = "library"
		model = modelParts[0]
	} else if len(modelParts) == 2 {
		namespace = modelParts[0]
		model = modelParts[1]
	} else {
		return false
	}
	
	tag := parts[1]
	registry := "registry.ollama.ai"
	
	return s.cache.HasManifest(registry, namespace, model, tag)
}

// streamCachedModelToClient streams a cached model to the client for local download
func (s *Server) streamCachedModelToClient(w http.ResponseWriter, r *http.Request, modelName string, stream bool) {
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.Header.Get("X-Real-IP")
	}
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}

	log.Printf("📤 [%s] Cache HIT: Model '%s' available, client will download blobs via registry API", clientIP, modelName)

	// For cached models, return a successful response that will trigger
	// Ollama client to download blobs via the Docker registry endpoints
	// The client will then make separate requests for manifests and blobs
	// which our cache will serve from local storage
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Send a basic success response - the real download will happen
	// via subsequent manifest/blob requests that we'll serve from cache
	response := map[string]interface{}{
		"status": "success",
	}

	respBytes, err := json.Marshal(response)
	if err != nil {
		log.Printf("❌ [%s] Error marshaling response: %v", clientIP, err)
		return
	}

	// Write newline-delimited JSON (NDJSON)
	if _, err := w.Write(respBytes); err != nil {
		log.Printf("❌ [%s] Error writing response: %v", clientIP, err)
		return
	}
	if _, err := w.Write([]byte("\n")); err != nil {
		log.Printf("❌ [%s] Error writing newline: %v", clientIP, err)
		return
	}

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	log.Printf("✅ [%s] Sent cache hit response for: %s (client will download via registry API)", clientIP, modelName)
}

// downloadAndCacheModel downloads from upstream and caches while streaming to client
func (s *Server) downloadAndCacheModel(w http.ResponseWriter, r *http.Request, modelName string, body []byte) {
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.Header.Get("X-Real-IP")
	}
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}

	log.Printf("⬇️ [%s] Downloading model '%s' from upstream Ollama", clientIP, modelName)

	// For now, return an error explaining that cache misses need to be populated
	// In a full implementation, this would connect to an upstream Ollama server
	errorMessage := map[string]interface{}{
		"error": fmt.Sprintf("Model '%s' not found in cache. Cache misses are not yet implemented. Please ask your administrator to add this model to the cache by running 'ollama pull %s' on the cache server.", modelName, modelName),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	if err := json.NewEncoder(w).Encode(errorMessage); err != nil {
		log.Printf("❌ [%s] Error encoding cache miss response: %v", clientIP, err)
	}

	log.Printf("📋 [%s] Sent cache miss message for model: %s", clientIP, modelName)
}

// serveCachedModel serves a model from the local cache using Ollama API format
func (s *Server) serveCachedModel(w http.ResponseWriter, r *http.Request, modelName string, stream bool) {
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.Header.Get("X-Real-IP")
	}
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}
	
	// For cached models, we need to send the exact response format Ollama expects
	// Since this is complex and model-specific, let's proxy to upstream but intercept the blobs
	log.Printf("💾 [%s] Model '%s' is cached, but using hybrid approach (proxy + cache blobs)", clientIP, modelName)
	
	// Create a new request with the original body to proxy to upstream
	// But the blob downloads will be served from cache via the Docker registry endpoints
	body := fmt.Sprintf(`{"model":"","username":"","password":"","name":"%s"}`, modelName)
	
	upstreamURL := "https://registry.ollama.ai" + r.URL.Path
	if r.URL.RawQuery != "" {
		upstreamURL += "?" + r.URL.RawQuery
	}
	
	upstreamReq, err := http.NewRequest(r.Method, upstreamURL, strings.NewReader(body))
	if err != nil {
		log.Printf("❌ [%s] Error creating upstream request: %v", clientIP, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	
	// Copy headers
	for key, values := range r.Header {
		for _, value := range values {
			upstreamReq.Header.Add(key, value)
		}
	}
	
	log.Printf("🔄 [%s] Hybrid mode: Getting manifest from upstream, blobs from cache", clientIP)
	
	resp, err := s.client.Do(upstreamReq)
	if err != nil {
		log.Printf("❌ [%s] Error proxying cached model request: %v", clientIP, err)
		log.Printf("🎯 [%s] Upstream error for cached model, trying cache-only mode", clientIP)
		s.serveCacheOnlyModel(w, r, modelName)
		return
	}
	defer resp.Body.Close()
	
	// Check if upstream returned not found but we have it cached
	if resp.StatusCode == 404 {
		log.Printf("🎯 [%s] Model '%s' not found upstream but we have it cached, serving cache-only", clientIP, modelName)
		s.serveCacheOnlyModel(w, r, modelName)
		return
	}
	
	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	
	// Stream the response back (Ollama will make separate requests for blobs which we'll cache-serve)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("❌ [%s] Error streaming cached model response: %v", clientIP, err)
	} else {
		log.Printf("✅ [%s] Successfully handled cached model request: %s", clientIP, modelName)
	}
}

// serveCacheOnlyModel serves a model entirely from cache when upstream is unavailable
func (s *Server) serveCacheOnlyModel(w http.ResponseWriter, r *http.Request, modelName string) {
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.Header.Get("X-Real-IP")
	}
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}
	
	log.Printf("🏠 [%s] Model '%s' is cached, but client needs local copy", clientIP, modelName)
	
	// The cache has the model, but the client needs to download it locally
	// We can't actually serve the full model data through the /api/pull endpoint
	// because that would require implementing the full Ollama download protocol
	
	errorMessage := map[string]interface{}{
		"error": fmt.Sprintf("Model '%s' is available in cache, but you need a local copy. This cache serves existing installs, not fresh downloads. To get this model locally:\n\n1. Temporarily unset OLLAMA_HOST:\n   • Windows (PowerShell): $env:OLLAMA_HOST = \"\"\n   • Windows (CMD): set OLLAMA_HOST=\n   • Mac/Linux: unset OLLAMA_HOST\n\n2. Download the model: ollama pull %s\n\n3. The model will now be available locally", modelName, modelName),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	if err := json.NewEncoder(w).Encode(errorMessage); err != nil {
		log.Printf("❌ [%s] Error encoding cache-only response: %v", clientIP, err)
	}

	log.Printf("📋 [%s] Directed client to download model locally: %s", clientIP, modelName)
}

// getBlobSize returns the size of a blob in the cache
func (s *Server) getBlobSize(digest string) int64 {
	// Use the cache's blob directory structure
	manifestsDir := s.cache.GetManifestsDir()
	blobsDir := strings.Replace(manifestsDir, "/manifests", "/blobs", 1)
	filename := strings.Replace(digest, ":", "-", 1)
	blobPath := blobsDir + "/" + filename
	
	if stat, err := os.Stat(blobPath); err == nil {
		return stat.Size()
	}
	return 0 // Unknown size
}

// calculateManifestDigest calculates the digest for a manifest (simplified)
func calculateManifestDigest(manifest *cache.Manifest) string {
	// This is a simplified implementation
	// In practice, you'd want to calculate the actual SHA256 of the canonical JSON
	return "sha256:placeholder"
}
