package services

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"sftp-gui/internal/config"
	"sftp-gui/internal/models"
)

// LoginHistoryService manages login history
type LoginHistoryService struct {
	history []models.LoginHistory
	mutex   sync.RWMutex
	config  *config.Config
}

// NewLoginHistoryService creates a new login history service
func NewLoginHistoryService(cfg *config.Config) *LoginHistoryService {
	service := &LoginHistoryService{
		history: make([]models.LoginHistory, 0),
		config:  cfg,
	}

	// Load existing history
	if cfg.Session.SaveHistory {
		service.loadHistory()
	}

	return service
}

// AddLogin adds a login attempt to history
func (l *LoginHistoryService) AddLogin(host string, port int, username string, success bool) {
	if !l.config.Session.SaveHistory {
		return
	}

	l.mutex.Lock()
	defer l.mutex.Unlock()

	// Check if this combination already exists
	for i, entry := range l.history {
		if entry.Host == host && entry.Port == port && entry.Username == username {
			// Update existing entry
			l.history[i].LastUsed = time.Now()
			l.history[i].Success = success

			// Move to front (most recent)
			entry := l.history[i]
			l.history = append(l.history[:i], l.history[i+1:]...)
			l.history = append([]models.LoginHistory{entry}, l.history...)

			l.saveHistory()
			return
		}
	}

	// Add new entry
	newEntry := models.LoginHistory{
		Host:     host,
		Port:     port,
		Username: username,
		LastUsed: time.Now(),
		Success:  success,
	}

	l.history = append([]models.LoginHistory{newEntry}, l.history...)

	// Limit history size
	if len(l.history) > l.config.Session.MaxHistory {
		l.history = l.history[:l.config.Session.MaxHistory]
	}

	l.saveHistory()
}

// GetHistory returns the login history
func (l *LoginHistoryService) GetHistory() []models.LoginHistory {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	// Return a copy to prevent external modification
	history := make([]models.LoginHistory, len(l.history))
	copy(history, l.history)

	return history
}

// GetSuccessfulHistory returns only successful login attempts
func (l *LoginHistoryService) GetSuccessfulHistory() []models.LoginHistory {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	var successful []models.LoginHistory
	for _, entry := range l.history {
		if entry.Success {
			successful = append(successful, entry)
		}
	}

	return successful
}

// ClearHistory clears all login history
func (l *LoginHistoryService) ClearHistory() error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	l.history = make([]models.LoginHistory, 0)
	return l.saveHistory()
}

// RemoveEntry removes a specific entry from history
func (l *LoginHistoryService) RemoveEntry(host string, port int, username string) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	for i, entry := range l.history {
		if entry.Host == host && entry.Port == port && entry.Username == username {
			l.history = append(l.history[:i], l.history[i+1:]...)
			l.saveHistory()
			return
		}
	}
}

// loadHistory loads history from file
func (l *LoginHistoryService) loadHistory() error {
	if l.config.Session.HistoryFile == "" {
		return nil
	}

	data, err := os.ReadFile(l.config.Session.HistoryFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist yet, that's ok
		}
		return fmt.Errorf("failed to read history file: %w", err)
	}

	var history []models.LoginHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return fmt.Errorf("failed to parse history file: %w", err)
	}

	// Sort by last used (most recent first)
	sort.Slice(history, func(i, j int) bool {
		return history[i].LastUsed.After(history[j].LastUsed)
	})

	// Limit to max history size
	if len(history) > l.config.Session.MaxHistory {
		history = history[:l.config.Session.MaxHistory]
	}

	l.history = history
	return nil
}

// saveHistory saves history to file
func (l *LoginHistoryService) saveHistory() error {
	if l.config.Session.HistoryFile == "" {
		return nil
	}

	data, err := json.MarshalIndent(l.history, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
	}

	if err := os.WriteFile(l.config.Session.HistoryFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write history file: %w", err)
	}

	return nil
}
