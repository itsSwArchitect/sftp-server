package models

import (
	"os"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// Session represents an active SFTP session
type Session struct {
	ID         string       `json:"id"`
	SSHClient  *ssh.Client  `json:"-"`
	SFTPClient *sftp.Client `json:"-"`
	CreatedAt  time.Time    `json:"created_at"`
	LastAccess time.Time    `json:"last_access"`
	HomeDir    string       `json:"home_dir"`
	Username   string       `json:"username"`
	Host       string       `json:"host"`
	Port       int          `json:"port"`
	IsActive   bool         `json:"is_active"`
}

// LoginHistory represents a login history entry
type LoginHistory struct {
	Host     string    `json:"host"`
	Port     int       `json:"port"`
	Username string    `json:"username"`
	LastUsed time.Time `json:"last_used"`
	Success  bool      `json:"success"`
}

// FileInfo represents file information for display
type FileInfo struct {
	Name    string      `json:"name"`
	Size    int64       `json:"size"`
	Mode    os.FileMode `json:"mode"`
	ModTime time.Time   `json:"mod_time"`
	IsDir   bool        `json:"is_dir"`
	Path    string      `json:"path"`
}

// PageData represents data passed to templates
type PageData struct {
	Connected       bool           `json:"connected"`
	Error           string         `json:"error"`
	Success         string         `json:"success"`
	Path            string         `json:"path"`
	Files           []FileInfo     `json:"files"`
	Breadcrumbs     []Breadcrumb   `json:"breadcrumbs"`
	View            string         `json:"view"`
	ShowHidden      bool           `json:"show_hidden"`
	Filter          string         `json:"filter"`
	TotalFiles      int            `json:"total_files"`
	FilteredFiles   int            `json:"filtered_files"`
	ShowBulkActions bool           `json:"show_bulk_actions"`
	LoginHistory    []LoginHistory `json:"login_history"`
	Theme           string         `json:"theme"`
	SessionInfo     *Session       `json:"session_info"`
}

// Breadcrumb represents a breadcrumb navigation item
type Breadcrumb struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Host     string `json:"host" form:"host"`
	Port     int    `json:"port" form:"port"`
	Username string `json:"username" form:"username"`
	Password string `json:"password" form:"password"`
}

// FileListRequest represents a file list request
type FileListRequest struct {
	Path       string `json:"path" form:"path"`
	View       string `json:"view" form:"view"`
	ShowHidden bool   `json:"show_hidden" form:"show_hidden"`
	Filter     string `json:"filter" form:"filter"`
	Page       int    `json:"page" form:"page"`
	Limit      int    `json:"limit" form:"limit"`
}

// FileOperationRequest represents a file operation request
type FileOperationRequest struct {
	Paths       []string `json:"paths" form:"paths"`
	Destination string   `json:"destination" form:"destination"`
	Operation   string   `json:"operation" form:"operation"`
}

// UploadRequest represents a file upload request
type UploadRequest struct {
	Path      string `json:"path" form:"path"`
	Overwrite bool   `json:"overwrite" form:"overwrite"`
}

// APIResponse represents a standard API response
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// SessionStats represents session statistics
type SessionStats struct {
	ActiveSessions int           `json:"active_sessions"`
	TotalSessions  int           `json:"total_sessions"`
	Uptime         time.Duration `json:"uptime"`
	MemoryUsage    int64         `json:"memory_usage"`
}

// SystemInfo represents system information
type SystemInfo struct {
	Version   string       `json:"version"`
	GoVersion string       `json:"go_version"`
	Platform  string       `json:"platform"`
	StartTime time.Time    `json:"start_time"`
	Sessions  SessionStats `json:"sessions"`
}

// IsExpired checks if the session has expired
func (s *Session) IsExpired(timeout time.Duration) bool {
	return time.Since(s.LastAccess) > timeout
}

// UpdateAccess updates the last access time
func (s *Session) UpdateAccess() {
	s.LastAccess = time.Now()
}

// Close closes the session connections
func (s *Session) Close() error {
	s.IsActive = false

	var err error
	if s.SFTPClient != nil {
		if closeErr := s.SFTPClient.Close(); closeErr != nil {
			err = closeErr
		}
		s.SFTPClient = nil
	}

	if s.SSHClient != nil {
		if closeErr := s.SSHClient.Close(); closeErr != nil {
			err = closeErr
		}
		s.SSHClient = nil
	}

	return err
}

// Validate validates the login request
func (r *LoginRequest) Validate() error {
	if r.Host == "" {
		return ErrInvalidHost
	}
	if r.Port <= 0 || r.Port > 65535 {
		return ErrInvalidPort
	}
	if r.Username == "" {
		return ErrInvalidUsername
	}
	if r.Password == "" {
		return ErrInvalidPassword
	}
	return nil
}

// Validate validates the file list request
func (r *FileListRequest) Validate() error {
	if r.Path == "" {
		r.Path = "/"
	}
	if r.View == "" {
		r.View = "list"
	}
	if r.Page < 1 {
		r.Page = 1
	}
	if r.Limit < 1 {
		r.Limit = 100
	}
	return nil
}

// Custom errors
var (
	ErrInvalidHost     = NewValidationError("host is required")
	ErrInvalidPort     = NewValidationError("port must be between 1 and 65535")
	ErrInvalidUsername = NewValidationError("username is required")
	ErrInvalidPassword = NewValidationError("password is required")
	ErrSessionExpired  = NewSessionError("session has expired")
	ErrSessionNotFound = NewSessionError("session not found")
	ErrUnauthorized    = NewAuthError("unauthorized access")
)

// Error types
type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

func NewValidationError(message string) ValidationError {
	return ValidationError{Message: message}
}

type SessionError struct {
	Message string
}

func (e SessionError) Error() string {
	return e.Message
}

func NewSessionError(message string) SessionError {
	return SessionError{Message: message}
}

type AuthError struct {
	Message string
}

func (e AuthError) Error() string {
	return e.Message
}

func NewAuthError(message string) AuthError {
	return AuthError{Message: message}
}
