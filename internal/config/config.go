package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the SFTP web client
type Config struct {
	Server   ServerConfig   `json:"server"`
	Security SecurityConfig `json:"security"`
	Session  SessionConfig  `json:"session"`
	UI       UIConfig       `json:"ui"`
	Logging  LoggingConfig  `json:"logging"`
}

// ServerConfig contains HTTP server configuration
type ServerConfig struct {
	Host         string        `json:"host"`
	Port         int           `json:"port"`
	ReadTimeout  time.Duration `json:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout"`
	IdleTimeout  time.Duration `json:"idle_timeout"`
	TLSEnabled   bool          `json:"tls_enabled"`
	CertFile     string        `json:"cert_file"`
	KeyFile      string        `json:"key_file"`
}

// SecurityConfig contains security-related settings
type SecurityConfig struct {
	MaxLoginAttempts    int           `json:"max_login_attempts"`
	LoginTimeout        time.Duration `json:"login_timeout"`
	SessionCookieName   string        `json:"session_cookie_name"`
	SessionCookieSecure bool          `json:"session_cookie_secure"`
	CSRFEnabled         bool          `json:"csrf_enabled"`
	CORSEnabled         bool          `json:"cors_enabled"`
	AllowedOrigins      []string      `json:"allowed_origins"`
}

// SessionConfig contains session management settings
type SessionConfig struct {
	Timeout         time.Duration `json:"timeout"`
	CleanupInterval time.Duration `json:"cleanup_interval"`
	MaxSessions     int           `json:"max_sessions"`
	SaveHistory     bool          `json:"save_history"`
	HistoryFile     string        `json:"history_file"`
	MaxHistory      int           `json:"max_history"`
}

// UIConfig contains user interface settings
type UIConfig struct {
	DefaultTheme   string `json:"default_theme"`
	MaxFileSize    int64  `json:"max_file_size"`
	MaxPreviewSize int64  `json:"max_preview_size"`
	DefaultView    string `json:"default_view"`
	ItemsPerPage   int    `json:"items_per_page"`
	EnableBatchOps bool   `json:"enable_batch_operations"`
	EnablePreview  bool   `json:"enable_preview"`
	EnableUpload   bool   `json:"enable_upload"`
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level      string `json:"level"`
	Format     string `json:"format"`
	Output     string `json:"output"`
	MaxSize    int    `json:"max_size_mb"`
	MaxAge     int    `json:"max_age_days"`
	MaxBackups int    `json:"max_backups"`
	Compress   bool   `json:"compress"`
}

// Load loads configuration from file and environment variables
func Load(configPath string) (*Config, error) {
	config := DefaultConfig()

	// Load from config file if it exists
	if configPath != "" {
		if err := loadFromFile(config, configPath); err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
	}

	// Override with environment variables
	loadFromEnv(config)

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:         "localhost",
			Port:         8088,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
			TLSEnabled:   false,
		},
		Security: SecurityConfig{
			MaxLoginAttempts:    5,
			LoginTimeout:        24 * time.Hour,
			SessionCookieName:   "sftp_session",
			SessionCookieSecure: false,
			CSRFEnabled:         true,
			CORSEnabled:         false,
			AllowedOrigins:      []string{"http://localhost:8088"},
		},
		Session: SessionConfig{
			Timeout:         30 * time.Minute,
			CleanupInterval: 5 * time.Minute,
			MaxSessions:     100,
			SaveHistory:     true,
			HistoryFile:     "login_history.json",
			MaxHistory:      50,
		},
		UI: UIConfig{
			DefaultTheme:   "light",
			MaxFileSize:    100 * 1024 * 1024, // 100MB
			MaxPreviewSize: 1024 * 1024,       // 1MB
			DefaultView:    "list",
			ItemsPerPage:   100,
			EnableBatchOps: true,
			EnablePreview:  true,
			EnableUpload:   true,
		},
		Logging: LoggingConfig{
			Level:      "info",
			Format:     "json",
			Output:     "stdout",
			MaxSize:    10,
			MaxAge:     7,
			MaxBackups: 3,
			Compress:   true,
		},
	}
}

// loadFromFile loads configuration from a JSON file
func loadFromFile(config *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Config file is optional
		}
		return err
	}

	return json.Unmarshal(data, config)
}

// loadFromEnv loads configuration from environment variables
func loadFromEnv(config *Config) {
	// Server config
	if host := os.Getenv("SFTP_HOST"); host != "" {
		config.Server.Host = host
	}
	if port := os.Getenv("SFTP_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.Server.Port = p
		}
	}
	if tlsEnabled := os.Getenv("SFTP_TLS_ENABLED"); tlsEnabled == "true" {
		config.Server.TLSEnabled = true
	}
	if certFile := os.Getenv("SFTP_CERT_FILE"); certFile != "" {
		config.Server.CertFile = certFile
	}
	if keyFile := os.Getenv("SFTP_KEY_FILE"); keyFile != "" {
		config.Server.KeyFile = keyFile
	}

	// Security config
	if maxAttempts := os.Getenv("SFTP_MAX_LOGIN_ATTEMPTS"); maxAttempts != "" {
		if m, err := strconv.Atoi(maxAttempts); err == nil {
			config.Security.MaxLoginAttempts = m
		}
	}
	if secure := os.Getenv("SFTP_COOKIE_SECURE"); secure == "true" {
		config.Security.SessionCookieSecure = true
	}

	// Session config
	if timeout := os.Getenv("SFTP_SESSION_TIMEOUT"); timeout != "" {
		if t, err := time.ParseDuration(timeout); err == nil {
			config.Session.Timeout = t
		}
	}
	if maxSessions := os.Getenv("SFTP_MAX_SESSIONS"); maxSessions != "" {
		if m, err := strconv.Atoi(maxSessions); err == nil {
			config.Session.MaxSessions = m
		}
	}

	// UI config
	if theme := os.Getenv("SFTP_DEFAULT_THEME"); theme != "" {
		config.UI.DefaultTheme = theme
	}
	if view := os.Getenv("SFTP_DEFAULT_VIEW"); view != "" {
		config.UI.DefaultView = view
	}

	// Logging config
	if level := os.Getenv("SFTP_LOG_LEVEL"); level != "" {
		config.Logging.Level = level
	}
	if format := os.Getenv("SFTP_LOG_FORMAT"); format != "" {
		config.Logging.Format = format
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate server config
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.Server.TLSEnabled {
		if c.Server.CertFile == "" || c.Server.KeyFile == "" {
			return fmt.Errorf("TLS enabled but cert_file or key_file not specified")
		}
	}

	// Validate security config
	if c.Security.MaxLoginAttempts < 1 {
		return fmt.Errorf("max_login_attempts must be at least 1")
	}

	// Validate session config
	if c.Session.Timeout < time.Minute {
		return fmt.Errorf("session timeout must be at least 1 minute")
	}

	if c.Session.MaxSessions < 1 {
		return fmt.Errorf("max_sessions must be at least 1")
	}

	// Validate UI config
	if c.UI.DefaultView != "list" && c.UI.DefaultView != "grid" && c.UI.DefaultView != "detailed" {
		return fmt.Errorf("invalid default_view: %s", c.UI.DefaultView)
	}

	if c.UI.DefaultTheme != "light" && c.UI.DefaultTheme != "dark" {
		return fmt.Errorf("invalid default_theme: %s", c.UI.DefaultTheme)
	}

	// Validate logging config
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.Logging.Level] {
		return fmt.Errorf("invalid log level: %s", c.Logging.Level)
	}

	return nil
}

// GetAddr returns the server address
func (c *Config) GetAddr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// Save saves the configuration to a file
func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
