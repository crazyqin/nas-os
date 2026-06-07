// Package security provides password history storage.
package security

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// PasswordHistoryStore stores password history for users.
type PasswordHistoryStore struct {
	mu       sync.RWMutex
	history  map[string][]passwordRecord // userID -> history
	maxCount int
}

type passwordRecord struct {
	Hash      string    `json:"hash"`
	CreatedAt time.Time `json:"created_at"`
}

// NewPasswordHistoryStore creates a new password history store.
func NewPasswordHistoryStore(maxCount int) *PasswordHistoryStore {
	return &PasswordHistoryStore{
		history:  make(map[string][]passwordRecord),
		maxCount: maxCount,
	}
}

// Add adds a password to user's history.
func (s *PasswordHistoryStore) Add(userID string, password string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	hash := hashPassword(password)
	record := passwordRecord{
		Hash:      hash,
		CreatedAt: time.Now(),
	}

	history := s.history[userID]
	history = append(history, record)

	// Trim to max count
	if len(history) > s.maxCount {
		history = history[len(history)-s.maxCount:]
	}

	s.history[userID] = history
}

// IsInHistory checks if password is in user's history.
func (s *PasswordHistoryStore) IsInHistory(userID string, password string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hash := hashPassword(password)
	history := s.history[userID]

	for _, record := range history {
		if record.Hash == hash {
			return true
		}
	}

	return false
}

// GetAge returns the age of current password in days.
func (s *PasswordHistoryStore) GetAge(userID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	history := s.history[userID]
	if len(history) == 0 {
		return 0
	}

	latest := history[len(history)-1]
	return int(time.Since(latest.CreatedAt).Hours() / 24)
}

// GetHistoryCount returns number of passwords in history.
func (s *PasswordHistoryStore) GetHistoryCount(userID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.history[userID])
}

// Clear clears password history for a user.
func (s *PasswordHistoryStore) Clear(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.history, userID)
}

// hashPassword hashes password for storage.
func hashPassword(password string) string {
	h := sha256.Sum256([]byte(password))
	return hex.EncodeToString(h[:])
}
