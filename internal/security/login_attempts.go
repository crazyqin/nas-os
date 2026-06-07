// Package security provides login attempts tracking.
package security

import (
	"sync"
	"time"
)

// LoginAttemptsTracker tracks login attempts for lockout.
type LoginAttemptsTracker struct {
	mu              sync.RWMutex
	attempts        map[string]*attemptRecord
	maxAttempts     int
	lockoutDuration time.Duration
}

type attemptRecord struct {
	Count       int       `json:"count"`
	LockedAt    time.Time `json:"locked_at"`
	LastAttempt time.Time `json:"last_attempt"`
}

// NewLoginAttemptsTracker creates a new attempts tracker.
func NewLoginAttemptsTracker(maxAttempts int, lockoutMinutes int) *LoginAttemptsTracker {
	return &LoginAttemptsTracker{
		attempts:        make(map[string]*attemptRecord),
		maxAttempts:     maxAttempts,
		lockoutDuration: time.Duration(lockoutMinutes) * time.Minute,
	}
}

// AddAttempt adds a failed login attempt.
func (t *LoginAttemptsTracker) AddAttempt(userID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	record := t.attempts[userID]
	if record == nil {
		record = &attemptRecord{}
		t.attempts[userID] = record
	}

	record.Count++
	record.LastAttempt = time.Now()
}

// Reset resets attempts for successful login.
func (t *LoginAttemptsTracker) Reset(userID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.attempts, userID)
}

// Lock locks a user account.
func (t *LoginAttemptsTracker) Lock(userID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	record := t.attempts[userID]
	if record == nil {
		record = &attemptRecord{}
		t.attempts[userID] = record
	}

	record.LockedAt = time.Now()
}

// GetStatus returns current attempts and lock status.
func (t *LoginAttemptsTracker) GetStatus(userID string) (int, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	record := t.attempts[userID]
	if record == nil {
		return 0, false
	}

	// Check if lockout expired
	if !record.LockedAt.IsZero() {
		if time.Since(record.LockedAt) > t.lockoutDuration {
			return 0, false // lockout expired
		}
		return record.Count, true
	}

	return record.Count, false
}

// IsLocked checks if user is locked.
func (t *LoginAttemptsTracker) IsLocked(userID string) bool {
	_, locked := t.GetStatus(userID)
	return locked
}

// GetRemainingTime returns remaining lockout time in minutes.
func (t *LoginAttemptsTracker) GetRemainingTime(userID string) int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	record := t.attempts[userID]
	if record == nil || record.LockedAt.IsZero() {
		return 0
	}

	remaining := t.lockoutDuration - time.Since(record.LockedAt)
	if remaining <= 0 {
		return 0
	}

	return int(remaining.Minutes())
}

// ClearAll clears all attempts records.
func (t *LoginAttemptsTracker) ClearAll() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.attempts = make(map[string]*attemptRecord)
}

// GetStats returns lockout statistics.
func (t *LoginAttemptsTracker) GetStats() LockoutStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	stats := LockoutStats{
		TotalUsers:  len(t.attempts),
		LockedUsers: 0,
		HighRisk:    0,
	}

	for _, record := range t.attempts {
		if !record.LockedAt.IsZero() && time.Since(record.LockedAt) < t.lockoutDuration {
			stats.LockedUsers++
		}
		if record.Count >= t.maxAttempts-1 {
			stats.HighRisk++
		}
	}

	return stats
}

// LockoutStats represents lockout statistics.
type LockoutStats struct {
	TotalUsers  int `json:"total_users"`
	LockedUsers int `json:"locked_users"`
	HighRisk    int `json:"high_risk"` // near lockout threshold
}
