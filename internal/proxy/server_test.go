package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jjasghar/ollama-lancache/internal/cache"
)

func TestNewServer(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "proxy-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	modelCache, err := cache.New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	server := NewServer("127.0.0.1", 8080, modelCache)

	if server.addr != "127.0.0.1" {
		t.Errorf("Expected addr 127.0.0.1, got %s", server.addr)
	}

	if server.port != 8080 {
		t.Errorf("Expected port 8080, got %d", server.port)
	}

	if server.cache != modelCache {
		t.Error("Cache reference not set correctly")
	}

	if server.upstreamURL.String() != "https://registry.ollama.ai" {
		t.Errorf("Expected upstream URL https://registry.ollama.ai, got %s", server.upstreamURL.String())
	}
}

func TestManifestPatternMatching(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
		matches  []string
	}{
		{
			path:     "/v2/library/llama3/manifests/8b",
			expected: true,
			matches:  []string{"/v2/library/llama3/manifests/8b", "library", "llama3", "8b"},
		},
		{
			path:     "/v2/custom/my-model/manifests/latest",
			expected: true,
			matches:  []string{"/v2/custom/my-model/manifests/latest", "custom", "my-model", "latest"},
		},
		{
			path:     "/v2/library/llama3/blobs/sha256:abc123",
			expected: false,
			matches:  nil,
		},
		{
			path:     "/invalid/path",
			expected: false,
			matches:  nil,
		},
	}

	for _, test := range tests {
		matches := manifestPattern.FindStringSubmatch(test.path)
		if test.expected && matches == nil {
			t.Errorf("Expected path %s to match manifest pattern", test.path)
		} else if !test.expected && matches != nil {
			t.Errorf("Expected path %s to NOT match manifest pattern", test.path)
		} else if test.expected && matches != nil {
			if len(matches) != len(test.matches) {
				t.Errorf("Expected %d matches for %s, got %d", len(test.matches), test.path, len(matches))
			}
			for i, expected := range test.matches {
				if i < len(matches) && matches[i] != expected {
					t.Errorf("Match %d for path %s: expected %s, got %s", i, test.path, expected, matches[i])
				}
			}
		}
	}
}

func TestBlobPatternMatching(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
		matches  []string
	}{
		{
			path:     "/v2/library/llama3/blobs/sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			expected: true,
			matches:  []string{"/v2/library/llama3/blobs/sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", "library", "llama3", "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"},
		},
		{
			path:     "/v2/library/llama3/manifests/8b",
			expected: false,
			matches:  nil,
		},
		{
			path:     "/v2/library/llama3/blobs/invalid-digest",
			expected: false,
			matches:  nil,
		},
	}

	for _, test := range tests {
		matches := blobPattern.FindStringSubmatch(test.path)
		if test.expected && matches == nil {
			t.Errorf("Expected path %s to match blob pattern", test.path)
		} else if !test.expected && matches != nil {
			t.Errorf("Expected path %s to NOT match blob pattern", test.path)
		} else if test.expected && matches != nil {
			if len(matches) != len(test.matches) {
				t.Errorf("Expected %d matches for %s, got %d", len(test.matches), test.path, len(matches))
			}
			for i, expected := range test.matches {
				if i < len(matches) && matches[i] != expected {
					t.Errorf("Match %d for path %s: expected %s, got %s", i, test.path, expected, matches[i])
				}
			}
		}
	}
}

func TestHealthEndpoint(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "proxy-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	modelCache, err := cache.New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	server := NewServer("127.0.0.1", 8080, modelCache)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Expected content type application/json, got %s", resp.Header.Get("Content-Type"))
	}

	var healthResp map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
		t.Fatalf("Failed to decode health response: %v", err)
	}

	if healthResp["status"] != "healthy" {
		t.Errorf("Expected status healthy, got %s", healthResp["status"])
	}

	if healthResp["time"] == "" {
		t.Error("Expected time field to be present")
	}
}

func TestCacheStatsEndpoint(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "proxy-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	modelCache, err := cache.New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	server := NewServer("127.0.0.1", 8080, modelCache)

	req := httptest.NewRequest("GET", "/cache/stats", nil)
	w := httptest.NewRecorder()

	server.handleCacheStats(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Expected content type application/json, got %s", resp.Header.Get("Content-Type"))
	}

	var stats map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		t.Fatalf("Failed to decode stats response: %v", err)
	}

	// Check expected fields
	expectedFields := []string{"blob_count", "manifest_count", "total_blob_size_bytes", "cache_directory"}
	for _, field := range expectedFields {
		if _, exists := stats[field]; !exists {
			t.Errorf("Expected field %s to be present in stats", field)
		}
	}
}

func TestManifestCaching(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "proxy-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	modelCache, err := cache.New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Create test manifest
	testManifest := &cache.Manifest{
		SchemaVersion: 2,
		MediaType:     "application/vnd.docker.distribution.manifest.v2+json",
		Config: cache.Layer{
			MediaType: "application/vnd.docker.container.image.v1+json",
			Digest:    "sha256:config123",
			Size:      559,
		},
		Layers: []cache.Layer{
			{
				MediaType: "application/vnd.ollama.image.model",
				Digest:    "sha256:layer123",
				Size:      4590894944,
			},
		},
	}

	// Store manifest in cache
	registry := "registry.ollama.ai"
	namespace := "library"
	model := "test-model"
	tag := "latest"

	if err := modelCache.StoreManifest(registry, namespace, model, tag, testManifest); err != nil {
		t.Fatalf("Failed to store test manifest: %v", err)
	}

	server := NewServer("127.0.0.1", 8080, modelCache)

	// Test cached manifest request
	req := httptest.NewRequest("GET", "/v2/library/test-model/manifests/latest", nil)
	w := httptest.NewRecorder()

	server.handleManifestRequest(w, req, namespace, model, tag)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	if resp.Header.Get("Content-Type") != "application/vnd.docker.distribution.manifest.v2+json" {
		t.Errorf("Expected manifest content type, got %s", resp.Header.Get("Content-Type"))
	}

	var responseManifest cache.Manifest
	if err := json.NewDecoder(resp.Body).Decode(&responseManifest); err != nil {
		t.Fatalf("Failed to decode manifest response: %v", err)
	}

	if responseManifest.SchemaVersion != testManifest.SchemaVersion {
		t.Errorf("Schema version mismatch: expected %d, got %d", testManifest.SchemaVersion, responseManifest.SchemaVersion)
	}

	if responseManifest.Config.Digest != testManifest.Config.Digest {
		t.Errorf("Config digest mismatch: expected %s, got %s", testManifest.Config.Digest, responseManifest.Config.Digest)
	}
}

func TestBlobCaching(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "proxy-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	modelCache, err := cache.New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Create test blob
	testData := []byte("This is test blob data for caching")
	digest := "sha256:e7051bb20511931d66c0dcddc9c234c9a918396dcc549accb2c334d6cdb018cc"

	// Store blob in cache
	reader := bytes.NewReader(testData)
	if err := modelCache.StoreBlob(digest, reader); err != nil {
		t.Fatalf("Failed to store test blob: %v", err)
	}

	server := NewServer("127.0.0.1", 8080, modelCache)

	// Test cached blob request
	req := httptest.NewRequest("GET", "/v2/library/test-model/blobs/"+digest, nil)
	w := httptest.NewRecorder()

	server.handleBlobRequest(w, req, "library", "test-model", digest)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	if resp.Header.Get("Content-Type") != "application/octet-stream" {
		t.Errorf("Expected blob content type, got %s", resp.Header.Get("Content-Type"))
	}

	if resp.Header.Get("Docker-Content-Digest") != digest {
		t.Errorf("Expected digest header %s, got %s", digest, resp.Header.Get("Docker-Content-Digest"))
	}

	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if !bytes.Equal(responseData, testData) {
		t.Errorf("Response data doesn't match. Expected %s, got %s", testData, responseData)
	}
}

func TestProxyToUpstream(t *testing.T) {
	// Create a mock upstream server
	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "upstream response"}`))
	}))
	defer mockUpstream.Close()

	tmpDir, err := os.MkdirTemp("", "proxy-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	modelCache, err := cache.New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	server := NewServer("127.0.0.1", 8080, modelCache)
	// Override upstream URL to point to our mock server
	server.upstreamURL, _ = server.upstreamURL.Parse(mockUpstream.URL)

	req := httptest.NewRequest("GET", "/v2/unknown/path", nil)
	w := httptest.NewRecorder()

	server.proxyToUpstream(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if !strings.Contains(string(body), "upstream response") {
		t.Errorf("Expected upstream response in body, got %s", string(body))
	}
}

func TestServerStartStop(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "proxy-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	modelCache, err := cache.New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Use a random available port
	server := NewServer("127.0.0.1", 0, modelCache) // Port 0 will select an available port

	ctx, cancel := context.WithCancel(context.Background())

	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- server.Start(ctx)
	}()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Stop the server
	cancel()

	// Wait for server to stop or timeout
	select {
	case err := <-errChan:
		if err != nil && err.Error() != "http: Server closed" && err != context.Canceled {
			t.Errorf("Unexpected server error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Server did not stop within timeout")
	}
}

// Integration test with manifest fetching simulation
func TestFetchAndCacheManifest(t *testing.T) {
	// Create a mock upstream server that returns a test manifest
	testManifest := cache.Manifest{
		SchemaVersion: 2,
		MediaType:     "application/vnd.docker.distribution.manifest.v2+json",
		Config: cache.Layer{
			MediaType: "application/vnd.docker.container.image.v1+json",
			Digest:    "sha256:upstream-config",
			Size:      559,
		},
		Layers: []cache.Layer{
			{
				MediaType: "application/vnd.ollama.image.model",
				Digest:    "sha256:upstream-layer",
				Size:      1234567890,
			},
		},
	}

	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(testManifest)
	}))
	defer mockUpstream.Close()

	tmpDir, err := os.MkdirTemp("", "proxy-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	modelCache, err := cache.New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	server := NewServer("127.0.0.1", 8080, modelCache)
	server.upstreamURL, _ = server.upstreamURL.Parse(mockUpstream.URL)

	req := httptest.NewRequest("GET", "/v2/library/test-model/manifests/latest", nil)
	w := httptest.NewRecorder()

	server.fetchAndCacheManifest(w, req, "registry.ollama.ai", "library", "test-model", "latest")

	// Verify response
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Verify manifest was cached
	if !modelCache.HasManifest("registry.ollama.ai", "library", "test-model", "latest") {
		t.Error("Manifest should be cached after fetch")
	}

	// Verify cached manifest content
	cachedManifest, err := modelCache.GetManifest("registry.ollama.ai", "library", "test-model", "latest")
	if err != nil {
		t.Fatalf("Failed to get cached manifest: %v", err)
	}

	if cachedManifest.Config.Digest != testManifest.Config.Digest {
		t.Errorf("Cached manifest config digest mismatch: expected %s, got %s", testManifest.Config.Digest, cachedManifest.Config.Digest)
	}
}

// Benchmark tests
func BenchmarkManifestServing(b *testing.B) {
	tmpDir, _ := os.MkdirTemp("", "proxy-bench")
	defer os.RemoveAll(tmpDir)

	modelCache, _ := cache.New(tmpDir)

	testManifest := &cache.Manifest{
		SchemaVersion: 2,
		MediaType:     "application/vnd.docker.distribution.manifest.v2+json",
	}

	modelCache.StoreManifest("registry.ollama.ai", "library", "test", "latest", testManifest)

	server := NewServer("127.0.0.1", 8080, modelCache)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/v2/library/test/manifests/latest", nil)
		w := httptest.NewRecorder()
		server.handleManifestRequest(w, req, "library", "test", "latest")
	}
}

func BenchmarkBlobServing(b *testing.B) {
	tmpDir, _ := os.MkdirTemp("", "proxy-bench")
	defer os.RemoveAll(tmpDir)

	modelCache, _ := cache.New(tmpDir)

	testData := bytes.Repeat([]byte("benchmark data "), 1000) // ~15KB
	digest := "sha256:e258d248fda94c63753607f7c4494ee0fcbe92f1a76bfdac795c9d84101eb317"

	reader := bytes.NewReader(testData)
	modelCache.StoreBlob(digest, reader)

	server := NewServer("127.0.0.1", 8080, modelCache)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", fmt.Sprintf("/v2/library/test/blobs/%s", digest), nil)
		w := httptest.NewRecorder()
		server.handleBlobRequest(w, req, "library", "test", digest)
	}
}
