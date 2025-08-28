package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the model distribution server",
	Long:  `Start a simple HTTP server to distribute Ollama models to client machines`,
	Run:   runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
	
	serveCmd.Flags().IntP("port", "p", 8080, "Port to serve models on")
	serveCmd.Flags().StringP("models-dir", "d", "", "Directory containing Ollama models (default: ~/.ollama/models)")
	serveCmd.Flags().StringP("bind", "b", "0.0.0.0", "IP address to bind to")
	
	viper.BindPFlag("serve.port", serveCmd.Flags().Lookup("port"))
	viper.BindPFlag("serve.models-dir", serveCmd.Flags().Lookup("models-dir"))
	viper.BindPFlag("serve.bind", serveCmd.Flags().Lookup("bind"))
}

type ModelInfo struct {
	Name         string    `json:"name"`
	Tag          string    `json:"tag"`
	Size         int64     `json:"size"`
	Modified     time.Time `json:"modified"`
	DownloadURL  string    `json:"download_url"`
	ManifestURL  string    `json:"manifest_url"`
}

type ServerInfo struct {
	ServerVersion string      `json:"server_version"`
	ModelsDir     string      `json:"models_dir"`
	Models        []ModelInfo `json:"models"`
	TotalSize     int64       `json:"total_size_bytes"`
	TotalModels   int         `json:"total_models"`
}

func runServe(cmd *cobra.Command, args []string) {
	port := viper.GetInt("serve.port")
	bind := viper.GetString("serve.bind")
	modelsDir := viper.GetString("serve.models-dir")
	
	if modelsDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatal("Could not determine home directory:", err)
		}
		modelsDir = filepath.Join(homeDir, ".ollama", "models")
	}
	
	// Check if models directory exists
	if _, err := os.Stat(modelsDir); os.IsNotExist(err) {
		log.Fatalf("Models directory does not exist: %s", modelsDir)
	}
	
	server := &ModelServer{
		modelsDir: modelsDir,
		bind:      bind,
		port:      port,
	}
	
	server.start()
}

type ModelServer struct {
	modelsDir string
	bind      string
	port      int
}

func (s *ModelServer) start() {
	mux := http.NewServeMux()
	
	// API endpoints
	mux.HandleFunc("/api/models", s.handleModelsAPI)
	mux.HandleFunc("/api/info", s.handleServerInfo)
	
	// Model download endpoints
	mux.HandleFunc("/models/", s.handleModelDownload)
	mux.HandleFunc("/manifests/", s.handleManifestDownload)
	mux.HandleFunc("/blobs/", s.handleBlobDownload)
	
	// Client scripts
	mux.HandleFunc("/install.ps1", s.handlePowerShellScript)
	mux.HandleFunc("/install.sh", s.handleBashScript)
	
	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	
	// Root handler with instructions
	mux.HandleFunc("/", s.handleRoot)
	
	addr := fmt.Sprintf("%s:%d", s.bind, s.port)
	
	log.Printf("=== Ollama Model Distribution Server ===")
	log.Printf("Models Directory: %s", s.modelsDir)
	log.Printf("Server listening on: http://%s", addr)
	log.Printf("")
	log.Printf("📋 Available endpoints:")
	log.Printf("  GET  /api/models     - List available models")
	log.Printf("  GET  /api/info       - Server information")
	log.Printf("  GET  /install.ps1    - PowerShell client script")
	log.Printf("  GET  /install.sh     - Bash client script")
	log.Printf("  GET  /health         - Health check")
	log.Printf("")
	log.Printf("🚀 Ready to serve models!")
	log.Printf("")
	
	// Get server IP for client instructions
	if serverIPs := getServerIPs(); len(serverIPs) > 0 {
		primaryIP := serverIPs[0]
		log.Printf("📝 Client Usage:")
		log.Printf("  Windows: powershell -c \"irm http://%s:%d/install.ps1 | iex\"", primaryIP, s.port)
		log.Printf("  Linux:   curl -fsSL http://%s:%d/install.sh | bash", primaryIP, s.port)
		log.Printf("  macOS:   curl -fsSL http://%s:%d/install.sh | bash", primaryIP, s.port)
		log.Printf("")
	}
	
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}

func (s *ModelServer) getAvailableModels() ([]ModelInfo, error) {
	var models []ModelInfo
	var totalSize int64
	
	manifestsDir := filepath.Join(s.modelsDir, "manifests")
	
	err := filepath.Walk(manifestsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		
		if !info.IsDir() && !strings.Contains(info.Name(), ".") {
			// Extract model name and tag from path
			relPath, _ := filepath.Rel(manifestsDir, path)
			parts := strings.Split(relPath, string(filepath.Separator))
			
			if len(parts) >= 2 {
				registry := parts[0]
				modelParts := parts[1:]
				
				// Skip registry.ollama.ai prefix
				if registry == "registry.ollama.ai" && len(parts) >= 3 {
					modelParts = parts[2:]
				}
				
				if len(modelParts) >= 2 {
					name := strings.Join(modelParts[:len(modelParts)-1], "/")
					tag := modelParts[len(modelParts)-1]
					
					model := ModelInfo{
						Name:        name,
						Tag:         tag,
						Size:        s.getModelSize(name, tag),
						Modified:    info.ModTime(),
						DownloadURL: fmt.Sprintf("/models/%s:%s", name, tag),
						ManifestURL: fmt.Sprintf("/manifests/%s:%s", name, tag),
					}
					
					models = append(models, model)
					totalSize += model.Size
				}
			}
		}
		
		return nil
	})
	
	return models, err
}

func (s *ModelServer) getModelSize(name, tag string) int64 {
	// Calculate total size by reading manifest and summing blob sizes
	manifestPath := s.getManifestPath(name, tag)
	
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return 0
	}
	
	var manifest map[string]interface{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return 0
	}
	
	var totalSize int64
	if layers, ok := manifest["layers"].([]interface{}); ok {
		for _, layer := range layers {
			if layerMap, ok := layer.(map[string]interface{}); ok {
				if size, ok := layerMap["size"].(float64); ok {
					totalSize += int64(size)
				}
			}
		}
	}
	
	return totalSize
}

func (s *ModelServer) getManifestPath(name, tag string) string {
	return filepath.Join(s.modelsDir, "manifests", "registry.ollama.ai", "library", name, tag)
}

func (s *ModelServer) handleModelsAPI(w http.ResponseWriter, r *http.Request) {
	models, err := s.getAvailableModels()
	if err != nil {
		http.Error(w, "Failed to list models", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models)
	
	log.Printf("📋 [%s] Listed %d models", getClientIP(r), len(models))
}

func (s *ModelServer) handleServerInfo(w http.ResponseWriter, r *http.Request) {
	models, _ := s.getAvailableModels()
	
	var totalSize int64
	for _, model := range models {
		totalSize += model.Size
	}
	
	info := ServerInfo{
		ServerVersion: "1.0.0",
		ModelsDir:     s.modelsDir,
		Models:        models,
		TotalSize:     totalSize,
		TotalModels:   len(models),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
	
	log.Printf("ℹ️  [%s] Server info requested", getClientIP(r))
}

func (s *ModelServer) handleModelDownload(w http.ResponseWriter, r *http.Request) {
	// Extract model name:tag from URL
	path := strings.TrimPrefix(r.URL.Path, "/models/")
	
	log.Printf("📦 [%s] Model download requested: %s", getClientIP(r), path)
	
	// Create a tar/zip containing all model files
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.tar\"", strings.ReplaceAll(path, ":", "_")))
	
	// For now, return a simple response
	fmt.Fprintf(w, "Model download for %s would be implemented here\n", path)
}

func (s *ModelServer) handleManifestDownload(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/manifests/")
	parts := strings.Split(path, ":")
	
	if len(parts) != 2 {
		http.Error(w, "Invalid manifest path", http.StatusBadRequest)
		return
	}
	
	name, tag := parts[0], parts[1]
	manifestPath := s.getManifestPath(name, tag)
	
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		http.Error(w, "Manifest not found", http.StatusNotFound)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	http.ServeFile(w, r, manifestPath)
	
	log.Printf("📄 [%s] Manifest served: %s:%s", getClientIP(r), name, tag)
}

func (s *ModelServer) handleBlobDownload(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/blobs/")
	
	// Convert colon to hyphen for file system compatibility
	// Blobs are stored as sha256-abc123... but requested as sha256:abc123...
	blobFileName := strings.ReplaceAll(path, ":", "-")
	blobPath := filepath.Join(s.modelsDir, "blobs", blobFileName)
	
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		http.Error(w, "Blob not found", http.StatusNotFound)
		return
	}
	
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, blobPath)
	
	log.Printf("🗃️  [%s] Blob served: %s", getClientIP(r), path[:12]+"...")
}

func (s *ModelServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	
	models, _ := s.getAvailableModels()
	
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Ollama Model Distribution Server</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .header { color: #333; }
        .models { margin: 20px 0; }
        .model { padding: 10px; border: 1px solid #ddd; margin: 5px 0; border-radius: 5px; }
        .usage { background: #f5f5f5; padding: 15px; border-radius: 5px; margin: 20px 0; }
        code { background: #eee; padding: 2px 5px; border-radius: 3px; }
    </style>
</head>
<body>
    <h1 class="header">🚀 Ollama Model Distribution Server</h1>
    
    <div class="usage">
        <h3>📝 Client Usage:</h3>
        <p><strong>Windows PowerShell:</strong></p>
        <code>powershell -c "irm http://` + r.Host + `/install.ps1 | iex"</code>
        
        <p><strong>Linux/macOS:</strong></p>
        <code>curl -fsSL http://` + r.Host + `/install.sh | bash</code>
    </div>
    
    <h3>📦 Available Models (` + fmt.Sprintf("%d", len(models)) + `):</h3>
    <div class="models">`
	
	for _, model := range models {
		html += fmt.Sprintf(`
        <div class="model">
            <strong>%s:%s</strong> - %.2f GB
            <br><small>Modified: %s</small>
        </div>`, model.Name, model.Tag, float64(model.Size)/(1024*1024*1024), model.Modified.Format("2006-01-02 15:04:05"))
	}
	
	html += `
    </div>
    
    <h3>🔗 API Endpoints:</h3>
    <ul>
        <li><a href="/api/models">GET /api/models</a> - List available models (JSON)</li>
        <li><a href="/api/info">GET /api/info</a> - Server information (JSON)</li>
        <li><a href="/install.ps1">GET /install.ps1</a> - PowerShell client script</li>
        <li><a href="/install.sh">GET /install.sh</a> - Bash client script</li>
    </ul>
</body>
</html>`
	
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func (s *ModelServer) handlePowerShellScript(w http.ResponseWriter, r *http.Request) {
	script, err := os.ReadFile("scripts/install.ps1")
	if err != nil {
		// Fallback to embedded script if file not found
		http.Error(w, "PowerShell script not found", http.StatusNotFound)
		return
	}
	
	// Inject server URL into the script as a comment for auto-detection
	serverURL := fmt.Sprintf("http://%s", r.Host)
	injectedScript := fmt.Sprintf("# AUTO_DETECTED_SERVER=%s\n%s", serverURL, string(script))
	
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"install.ps1\"")
	w.Write([]byte(injectedScript))
	
	log.Printf("📥 [%s] PowerShell script downloaded", getClientIP(r))
}

func (s *ModelServer) handleBashScript(w http.ResponseWriter, r *http.Request) {
	script, err := os.ReadFile("scripts/install.sh")
	if err != nil {
		// Fallback to embedded script if file not found
		http.Error(w, "Bash script not found", http.StatusNotFound)
		return
	}
	
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"install.sh\"")
	w.Write(script)
	
	log.Printf("📥 [%s] Bash script downloaded", getClientIP(r))
}

func getServerIPs() []string {
	var ips []string
	
	// Get all network interfaces
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return []string{"localhost"}
	}
	
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ips = append(ips, ipnet.IP.String())
			}
		}
	}
	
	if len(ips) == 0 {
		ips = append(ips, "localhost")
	}
	
	return ips
}

func getClientIP(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.Header.Get("X-Real-IP")
	}
	if ip == "" {
		ip = r.RemoteAddr
	}
	return ip
}
