package cmd

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jjasghar/ollama-lancache/internal/cache"
	"github.com/jjasghar/ollama-lancache/internal/dns"
	"github.com/jjasghar/ollama-lancache/internal/proxy"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the Ollama LanCache server",
	Long:  `Start both the DNS server and HTTP proxy server to provide caching for Ollama models.`,
	RunE:  runServer,
}

func init() {
	rootCmd.AddCommand(serverCmd)
}

// getLocalIPs returns a list of local IP addresses (excluding loopback)
func getLocalIPs() []string {
	var ips []string
	interfaces, err := net.Interfaces()
	if err != nil {
		return ips
	}

	for _, iface := range interfaces {
		// Skip loopback and inactive interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
				if ipNet.IP.To4() != nil { // IPv4
					ips = append(ips, ipNet.IP.String())
				}
			}
		}
	}
	return ips
}

// getCachedModels scans the manifests directory to find cached models
func getCachedModels(cacheDir string) []string {
	var models []string
	manifestsDir := filepath.Join(cacheDir, "manifests")
	
	// Walk through the manifests directory structure: registry/namespace/model/tag
	err := filepath.Walk(manifestsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}
		
		// Skip directories
		if info.IsDir() {
			return nil
		}
		
		// Parse the path to extract model info
		// Expected structure: manifestsDir/registry/namespace/model/tag
		relPath, err := filepath.Rel(manifestsDir, path)
		if err != nil {
			return nil
		}
		
		parts := strings.Split(filepath.ToSlash(relPath), "/")
		if len(parts) >= 4 {
			// registry := parts[0]  // e.g., registry.ollama.ai
			namespace := parts[1]    // e.g., library
			model := parts[2]        // e.g., llama3
			tag := parts[3]          // e.g., 8b
			
			// Format as namespace/model:tag (skip registry for cleaner display)
			if namespace == "library" {
				models = append(models, fmt.Sprintf("%s:%s", model, tag))
			} else {
				models = append(models, fmt.Sprintf("%s/%s:%s", namespace, model, tag))
			}
		}
		
		return nil
	})
	
	if err != nil {
		return models
	}
	
	// Remove duplicates and sort
	seen := make(map[string]bool)
	var uniqueModels []string
	for _, model := range models {
		if !seen[model] {
			seen[model] = true
			uniqueModels = append(uniqueModels, model)
		}
	}
	
	return uniqueModels
}

func runServer(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize cache directory first
	cacheDir := viper.GetString("cache-dir")
	if cacheDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		cacheDir = home + "/.ollama/models"
	}

	modelCache, err := cache.New(cacheDir)
	if err != nil {
		return fmt.Errorf("failed to initialize cache: %w", err)
	}

	// Display startup banner with IP addresses
	localIPs := getLocalIPs()
	log.Printf("=== Ollama LanCache Server Starting ===")
	log.Printf("Cache Directory: %s", cacheDir)
	
	// Show cached models
	stats, err := modelCache.GetCacheStats()
	if err == nil {
		log.Printf("Cache Status: %d manifests, %d blobs, %d MB total", 
			stats["manifest_count"], stats["blob_count"], stats["total_blob_size_mb"])
		
		// List cached models
		cachedModels := getCachedModels(cacheDir)
		if len(cachedModels) > 0 {
			log.Printf("Cached Models:")
			for _, model := range cachedModels {
				log.Printf("  - %s", model)
			}
		} else {
			log.Printf("Cached Models: None (cache is empty)")
		}
		log.Printf("")
	}
	
	if len(localIPs) > 0 {
		log.Printf("Server IP Address(es): %s", strings.Join(localIPs, ", "))
		log.Printf("Primary IP for clients: %s", localIPs[0])
		log.Printf("")
		log.Printf("Client Configuration Commands:")
		log.Printf("  Windows: echo %s registry.ollama.ai >> C:\\Windows\\System32\\drivers\\etc\\hosts", localIPs[0])
		log.Printf("  Linux/macOS: echo \"%s registry.ollama.ai\" | sudo tee -a /etc/hosts", localIPs[0])
		log.Printf("")
	} else {
		log.Printf("Server IP Address: Unable to detect (check network interfaces)")
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 2)

	// Start DNS server if enabled
	if viper.GetBool("dns-enabled") {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			dnsServer := dns.NewServer(
				viper.GetString("listen-addr"),
				viper.GetInt("dns-port"),
				viper.GetString("upstream-dns"),
				viper.GetString("ollama-registry"),
				viper.GetString("listen-addr"),
			)
			
			log.Printf("Starting DNS server on %s:%d", viper.GetString("listen-addr"), viper.GetInt("dns-port"))
			if err := dnsServer.Start(ctx); err != nil {
				errChan <- fmt.Errorf("DNS server error: %w", err)
			}
		}()
	}

	// Start HTTP proxy if enabled
	if viper.GetBool("http-enabled") {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			proxyServer := proxy.NewServer(
				viper.GetString("listen-addr"),
				viper.GetInt("http-port"),
				modelCache,
			)
			
			log.Printf("Starting HTTP proxy on %s:%d", viper.GetString("listen-addr"), viper.GetInt("http-port"))
			if err := proxyServer.Start(ctx); err != nil {
				errChan <- fmt.Errorf("HTTP proxy error: %w", err)
			}
		}()
	}

	// Give services time to start, then show ready message
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(500 * time.Millisecond):
			if len(localIPs) > 0 {
				log.Printf("🚀 Server Ready! Clients can now connect to %s", localIPs[0])
				log.Printf("💡 Test with: curl http://%s:%d/health", localIPs[0], viper.GetInt("http-port"))
			}
		}
	}()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		log.Printf("Received signal %s, shutting down...", sig)
		cancel()
	case err := <-errChan:
		log.Printf("Server error: %v", err)
		cancel()
		return err
	}

	wg.Wait()
	log.Println("Server shutdown complete")
	return nil
}
