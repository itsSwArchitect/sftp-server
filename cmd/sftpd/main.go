package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"sftp-gui/internal/config"
	"sftp-gui/internal/handlers"
	"sftp-gui/internal/middleware"
	"sftp-gui/internal/models"
	"sftp-gui/internal/services"
)

var (
	// Build information
	version   = "1.0.0"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	// Parse command line flags
	var (
		configPath  = flag.String("config", "", "Path to configuration file")
		showVersion = flag.Bool("version", false, "Show version information")
		showHelp    = flag.Bool("help", false, "Show help information")
		host        = flag.String("h", "", "Server host address")
		port        = flag.Int("p", 0, "Server port")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("SFTP Web Client %s\n", version)
		fmt.Printf("Build time: %s\n", buildTime)
		fmt.Printf("Git commit: %s\n", gitCommit)
		fmt.Printf("Go version: %s\n", runtime.Version())
		fmt.Printf("Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		return
	}

	if *showHelp {
		printHelp()
		return
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Override config with command line flags
	if *host != "" {
		cfg.Server.Host = *host
	}
	if *port != 0 {
		cfg.Server.Port = *port
	}

	// Create services
	sessionService := services.NewSessionService(cfg)
	fileService := services.NewFileService(sessionService)
	loginHistoryService := services.NewLoginHistoryService(cfg)

	// Load templates
	templates, err := loadTemplates()
	if err != nil {
		log.Fatalf("Failed to load templates: %v", err)
	}

	// Create handlers
	handler := handlers.New(sessionService, fileService, loginHistoryService, cfg, templates)

	// Create middleware
	mw := middleware.New(sessionService, cfg)

	// Setup routes
	mux := setupRoutes(handler, mw, cfg)

	// Create HTTP server
	server := &http.Server{
		Addr:         cfg.GetAddr(),
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("🚀 SFTP Web Client v%s starting on %s", version, cfg.GetAddr())
		log.Printf("📁 Open http://%s in your browser", cfg.GetAddr())

		var err error
		if cfg.Server.TLSEnabled {
			log.Printf("🔒 TLS enabled")
			err = server.ListenAndServeTLS(cfg.Server.CertFile, cfg.Server.KeyFile)
		} else {
			err = server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down server...")

	// Create a deadline for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("✅ Server stopped")
}

// setupRoutes configures all HTTP routes
func setupRoutes(h *handlers.Handler, mw *middleware.Middleware, cfg *config.Config) *http.ServeMux {
	mux := http.NewServeMux()

	// Static files (if static directory exists)
	staticDir := filepath.Join("web", "static")
	if _, err := os.Stat(staticDir); err == nil {
		mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))
	}

	// Public routes (no authentication required)
	publicMux := http.NewServeMux()
	publicMux.HandleFunc("/", h.Home)
	publicMux.HandleFunc("/connect", h.Login)
	publicMux.HandleFunc("/health", healthCheck)
	publicMux.HandleFunc("/version", versionHandler)

	// Protected routes (authentication required)
	protectedMux := http.NewServeMux()
	protectedMux.HandleFunc("/disconnect", h.Logout)
	protectedMux.HandleFunc("/download", h.Download)
	protectedMux.HandleFunc("/download-multiple", h.DownloadMultiple)
	protectedMux.HandleFunc("/upload", h.Upload)
	protectedMux.HandleFunc("/preview", h.Preview)
	protectedMux.HandleFunc("/delete", h.Delete)

	// Apply middleware to public routes
	publicHandler := mw.SecurityHeaders(
		mw.CORS(
			mw.Logger(
				mw.Recovery(publicMux))))

	// Apply middleware to protected routes
	protectedHandler := mw.SecurityHeaders(
		mw.CORS(
			mw.SessionAuth(
				mw.Logger(
					mw.Recovery(protectedMux)))))

	// Mount handlers
	mux.Handle("/", publicHandler)
	mux.Handle("/disconnect", protectedHandler)
	mux.Handle("/download", protectedHandler)
	mux.Handle("/download-multiple", protectedHandler)
	mux.Handle("/upload", protectedHandler)
	mux.Handle("/preview", protectedHandler)
	mux.Handle("/delete", protectedHandler)

	return mux
}

// loadTemplates loads HTML templates from embedded filesystem
func loadTemplates() (*template.Template, error) {
	// Template functions
	funcMap := template.FuncMap{
		"formatSize": formatFileSize,
		"fileIcon":   getFileIcon,
		"fileType":   getFileType,
		"cleanPath":  cleanPath,
		"dir":        filepath.Dir,
		"canPreview": canPreviewFile,
	}

	// Load templates
	tmpl := template.New("").Funcs(funcMap)

	// Walk through template files
	err := walkTemplates(tmpl, "web/templates")
	if err != nil {
		return nil, err
	}

	return tmpl, nil
}

// walkTemplates recursively loads templates from filesystem
func walkTemplates(tmpl *template.Template, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())

		if entry.IsDir() {
			if err := walkTemplates(tmpl, path); err != nil {
				return err
			}
		} else if strings.HasSuffix(entry.Name(), ".html") || strings.HasSuffix(entry.Name(), ".tmpl") {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			_, err = tmpl.New(entry.Name()).Parse(string(content))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Template helper functions
func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}

	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"KB", "MB", "GB", "TB", "PB", "EB"}
	return fmt.Sprintf("%.1f %s", float64(size)/float64(div), units[exp])
}

func getFileIcon(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

	iconMap := map[string]string{
		".txt": "📄",
		".doc": "📄", ".docx": "📄",
		".pdf": "📕",
		".xls": "📊", ".xlsx": "📊",
		".ppt": "📊", ".pptx": "📊",
		".jpg": "🖼️", ".jpeg": "🖼️", ".png": "🖼️", ".gif": "🖼️", ".bmp": "🖼️", ".svg": "🖼️",
		".mp4": "🎥", ".avi": "🎥", ".mkv": "🎥", ".mov": "🎥",
		".mp3": "🎵", ".wav": "🎵", ".flac": "🎵",
		".zip": "📦", ".tar": "📦", ".gz": "📦", ".rar": "📦", ".7z": "📦",
		".js": "💻", ".html": "💻", ".css": "💻", ".py": "💻", ".go": "💻", ".java": "💻",
		".json": "⚙️", ".xml": "⚙️", ".yaml": "⚙️", ".yml": "⚙️",
	}

	if icon, exists := iconMap[ext]; exists {
		return icon
	}

	return "📄"
}

func getFileType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

	typeMap := map[string]string{
		".txt": "Text",
		".doc": "Word Document", ".docx": "Word Document",
		".pdf": "PDF",
		".xls": "Excel", ".xlsx": "Excel",
		".ppt": "PowerPoint", ".pptx": "PowerPoint",
		".jpg": "Image", ".jpeg": "Image", ".png": "Image", ".gif": "Image", ".bmp": "Image", ".svg": "Image",
		".mp4": "Video", ".avi": "Video", ".mkv": "Video", ".mov": "Video",
		".mp3": "Audio", ".wav": "Audio", ".flac": "Audio",
		".zip": "Archive", ".tar": "Archive", ".gz": "Archive", ".rar": "Archive", ".7z": "Archive",
		".js": "JavaScript", ".html": "HTML", ".css": "CSS", ".py": "Python", ".go": "Go", ".java": "Java",
		".json": "JSON", ".xml": "XML", ".yaml": "YAML", ".yml": "YAML",
	}

	if fileType, exists := typeMap[ext]; exists {
		return fileType
	}

	return "File"
}

func canPreviewFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))

	previewableExts := map[string]bool{
		".txt": true, ".md": true, ".json": true, ".xml": true, ".yaml": true, ".yml": true,
		".js": true, ".html": true, ".css": true, ".py": true, ".go": true, ".java": true,
		".c": true, ".cpp": true, ".h": true, ".hpp": true, ".sh": true, ".bash": true,
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".bmp": true, ".svg": true,
		".pdf": false, // Could be previewable with special handling
	}

	return previewableExts[ext]
}

func cleanPath(basePath, relativePath string) string {
	if relativePath == "" {
		return basePath
	}
	return filepath.Join(basePath, relativePath)
}

// HTTP handlers
func healthCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   version,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func versionHandler(w http.ResponseWriter, r *http.Request) {
	info := models.SystemInfo{
		Version:   version,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		StartTime: time.Now(), // This should be the actual start time
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func printHelp() {
	fmt.Printf(`SFTP Web Client %s

A modern web-based SFTP client with a clean interface.

USAGE:
    sftpd [OPTIONS]

OPTIONS:
    -config <path>    Path to configuration file
    -version          Show version information
    -help             Show this help message

ENVIRONMENT VARIABLES:
    SFTP_HOST         Server host (default: localhost)
    SFTP_PORT         Server port (default: 8088)
    SFTP_TLS_ENABLED  Enable TLS (default: false)
    SFTP_CERT_FILE    TLS certificate file
    SFTP_KEY_FILE     TLS private key file
    SFTP_LOG_LEVEL    Log level (debug, info, warn, error)

EXAMPLES:
    # Start with default settings
    sftpd

    # Start with custom config file
    sftpd -config /path/to/config.json

    # Start with environment variables
    SFTP_PORT=8080 SFTP_LOG_LEVEL=debug sftpd

For more information, visit: https://github.com/yourorg/sftp-web-client
`, version)
}
