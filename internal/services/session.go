package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"sftp-gui/internal/config"
	"sftp-gui/internal/models"
)

// SessionService manages SFTP sessions
type SessionService struct {
	sessions map[string]*models.Session
	mutex    sync.RWMutex
	config   *config.Config
}

// NewSessionService creates a new session service
func NewSessionService(cfg *config.Config) *SessionService {
	service := &SessionService{
		sessions: make(map[string]*models.Session),
		config:   cfg,
	}

	// Start cleanup goroutine
	go service.cleanupExpiredSessions()

	return service
}

// CreateSession creates a new SFTP session
func (s *SessionService) CreateSession(req *models.LoginRequest) (*models.Session, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Check session limits
	s.mutex.RLock()
	if len(s.sessions) >= s.config.Session.MaxSessions {
		s.mutex.RUnlock()
		return nil, fmt.Errorf("maximum number of sessions reached")
	}
	s.mutex.RUnlock()

	// Create SSH client config
	sshConfig := &ssh.ClientConfig{
		User: req.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(req.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Note: In production, use proper host key verification
		Timeout:         30 * time.Second,
	}

	// Connect to SSH server
	addr := fmt.Sprintf("%s:%d", req.Host, req.Port)
	sshClient, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SSH server: %w", err)
	}

	// Create SFTP client
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return nil, fmt.Errorf("failed to create SFTP client: %w", err)
	}

	// Get home directory
	homeDir, err := sftpClient.Getwd()
	if err != nil {
		homeDir = "/"
	}

	// Generate session ID
	sessionID, err := s.generateSessionID()
	if err != nil {
		sftpClient.Close()
		sshClient.Close()
		return nil, fmt.Errorf("failed to generate session ID: %w", err)
	}

	// Create session
	session := &models.Session{
		ID:         sessionID,
		SSHClient:  sshClient,
		SFTPClient: sftpClient,
		CreatedAt:  time.Now(),
		LastAccess: time.Now(),
		HomeDir:    homeDir,
		Username:   req.Username,
		Host:       req.Host,
		Port:       req.Port,
		IsActive:   true,
	}

	// Store session
	s.mutex.Lock()
	s.sessions[sessionID] = session
	s.mutex.Unlock()

	return session, nil
}

// GetSession retrieves a session by ID
func (s *SessionService) GetSession(sessionID string) (*models.Session, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return nil, models.ErrSessionNotFound
	}

	if session.IsExpired(s.config.Session.Timeout) {
		return nil, models.ErrSessionExpired
	}

	session.UpdateAccess()
	return session, nil
}

// DeleteSession removes a session
func (s *SessionService) DeleteSession(sessionID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return models.ErrSessionNotFound
	}

	// Close connections
	if err := session.Close(); err != nil {
		// Log error but continue with deletion
		fmt.Printf("Error closing session connections: %v\n", err)
	}

	delete(s.sessions, sessionID)
	return nil
}

// ListSessions returns all active sessions
func (s *SessionService) ListSessions() []*models.Session {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	sessions := make([]*models.Session, 0, len(s.sessions))
	for _, session := range s.sessions {
		if !session.IsExpired(s.config.Session.Timeout) {
			sessions = append(sessions, session)
		}
	}

	return sessions
}

// GetStats returns session statistics
func (s *SessionService) GetStats() models.SessionStats {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	activeSessions := 0
	for _, session := range s.sessions {
		if !session.IsExpired(s.config.Session.Timeout) && session.IsActive {
			activeSessions++
		}
	}

	return models.SessionStats{
		ActiveSessions: activeSessions,
		TotalSessions:  len(s.sessions),
	}
}

// CleanupExpiredSessions removes expired sessions
func (s *SessionService) CleanupExpiredSessions() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	var expiredSessions []string
	for id, session := range s.sessions {
		if session.IsExpired(s.config.Session.Timeout) {
			expiredSessions = append(expiredSessions, id)
		}
	}

	for _, id := range expiredSessions {
		session := s.sessions[id]
		if err := session.Close(); err != nil {
			fmt.Printf("Error closing expired session %s: %v\n", id, err)
		}
		delete(s.sessions, id)
	}

	return len(expiredSessions)
}

// generateSessionID generates a random session ID
func (s *SessionService) generateSessionID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// cleanupExpiredSessions runs periodically to clean up expired sessions
func (s *SessionService) cleanupExpiredSessions() {
	ticker := time.NewTicker(s.config.Session.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		count := s.CleanupExpiredSessions()
		if count > 0 {
			fmt.Printf("Cleaned up %d expired sessions\n", count)
		}
	}
}
