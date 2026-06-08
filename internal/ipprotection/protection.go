package ipprotection

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// IPEntry represents an IP address entry
type IPEntry struct {
	IP        string    `json:"ip"`
	Type      string    `json:"type"` // whitelist, blacklist, auto_banned
	Reason    string    `json:"reason,omitempty"`
	FailCount int       `json:"fail_count"`
	LastFail  time.Time `json:"last_fail"`
	BannedAt  time.Time `json:"banned_at,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ProtectionConfig represents IP protection configuration
type ProtectionConfig struct {
	Enabled          bool `json:"enabled"`
	MaxFailAttempts  int  `json:"max_fail_attempts"`
	BanDurationHours int  `json:"ban_duration_hours"`
	AutoBanEnabled   bool `json:"auto_ban_enabled"`
}

// Protection handles IP-based access protection
type Protection struct {
	mu       sync.RWMutex
	config   ProtectionConfig
	entries  map[string]*IPEntry
	failures map[string]int
}

// NewProtection creates a new IP protection handler
func NewProtection() *Protection {
	return &Protection{
		config: ProtectionConfig{
			Enabled:          true,
			MaxFailAttempts:  5,
			BanDurationHours: 24,
			AutoBanEnabled:   true,
		},
		entries:  make(map[string]*IPEntry),
		failures: make(map[string]int),
	}
}

// AddToWhitelist adds an IP to whitelist
func (p *Protection) AddToWhitelist(ip string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if net.ParseIP(ip) == nil {
		return fmt.Errorf("invalid IP address: %s", ip)
	}

	p.entries[ip] = &IPEntry{
		IP:        ip,
		Type:      "whitelist",
		CreatedAt: time.Now(),
	}
	return nil
}

// AddToBlacklist adds an IP to blacklist
func (p *Protection) AddToBlacklist(ip, reason string, durationHours int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if net.ParseIP(ip) == nil {
		return fmt.Errorf("invalid IP address: %s", ip)
	}

	now := time.Now()
	p.entries[ip] = &IPEntry{
		IP:        ip,
		Type:      "blacklist",
		Reason:    reason,
		BannedAt:  now,
		ExpiresAt: now.Add(time.Duration(durationHours) * time.Hour),
		CreatedAt: now,
	}
	return nil
}

// RemoveEntry removes an IP entry
func (p *Protection) RemoveEntry(ip string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.entries[ip]; !exists {
		return fmt.Errorf("IP not found: %s", ip)
	}
	delete(p.entries, ip)
	delete(p.failures, ip)
	return nil
}

// RecordFailure records a failed attempt for an IP
func (p *Protection) RecordFailure(ip string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.config.AutoBanEnabled {
		return
	}

	// Check if IP is whitelisted
	if entry, exists := p.entries[ip]; exists && entry.Type == "whitelist" {
		return
	}

	p.failures[ip]++
	now := time.Now()

	// Update existing entry or create new one
	if entry, exists := p.entries[ip]; exists {
		entry.FailCount = p.failures[ip]
		entry.LastFail = now
	} else {
		p.entries[ip] = &IPEntry{
			IP:        ip,
			Type:      "auto_banned",
			FailCount: p.failures[ip],
			LastFail:  now,
			CreatedAt: now,
		}
	}

	// Auto-ban if threshold exceeded
	if p.failures[ip] >= p.config.MaxFailAttempts {
		p.entries[ip].Type = "auto_banned"
		p.entries[ip].BannedAt = now
		p.entries[ip].ExpiresAt = now.Add(time.Duration(p.config.BanDurationHours) * time.Hour)
		p.entries[ip].Reason = fmt.Sprintf("Auto-banned after %d failed attempts", p.failures[ip])
	}
}

// IsAllowed checks if an IP is allowed to access
func (p *Protection) IsAllowed(ip string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.config.Enabled {
		return true
	}

	entry, exists := p.entries[ip]
	if !exists {
		return true
	}

	switch entry.Type {
	case "whitelist":
		return true
	case "blacklist":
		if !entry.ExpiresAt.IsZero() && time.Now().After(entry.ExpiresAt) {
			// Ban expired, allow access
			return true
		}
		return false
	case "auto_banned":
		if !entry.ExpiresAt.IsZero() && time.Now().After(entry.ExpiresAt) {
			// Ban expired, reset failures
			delete(p.failures, ip)
			delete(p.entries, ip)
			return true
		}
		return false
	default:
		return true
	}
}

// GetEntries returns all IP entries
func (p *Protection) GetEntries() []*IPEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()

	entries := make([]*IPEntry, 0, len(p.entries))
	for _, entry := range p.entries {
		entries = append(entries, entry)
	}
	return entries
}

// GetConfig returns the current configuration
func (p *Protection) GetConfig() ProtectionConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config
}

// UpdateConfig updates the protection configuration
func (p *Protection) UpdateConfig(config ProtectionConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config = config
}

// CleanupExpired removes expired entries
func (p *Protection) CleanupExpired() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	for ip, entry := range p.entries {
		if entry.Type != "whitelist" && !entry.ExpiresAt.IsZero() && now.After(entry.ExpiresAt) {
			delete(p.entries, ip)
			delete(p.failures, ip)
		}
	}
}

// RegisterRoutes registers IP protection HTTP routes
func (p *Protection) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/ip/entries", p.handleEntries)
	mux.HandleFunc("/api/ip/whitelist/add", p.handleAddWhitelist)
	mux.HandleFunc("/api/ip/blacklist/add", p.handleAddBlacklist)
	mux.HandleFunc("/api/ip/remove", p.handleRemove)
	mux.HandleFunc("/api/ip/config", p.handleConfig)
	mux.HandleFunc("/api/ip/config/update", p.handleUpdateConfig)
	mux.HandleFunc("/api/ip/cleanup", p.handleCleanup)
}

func (p *Protection) handleEntries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	entries := p.GetEntries()
	json.NewEncoder(w).Encode(entries)
}

func (p *Protection) handleAddWhitelist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		IP string `json:"ip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := p.AddToWhitelist(req.IP); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (p *Protection) handleAddBlacklist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		IP            string `json:"ip"`
		Reason        string `json:"reason"`
		DurationHours int    `json:"duration_hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.DurationHours <= 0 {
		req.DurationHours = 24
	}

	if err := p.AddToBlacklist(req.IP, req.Reason, req.DurationHours); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (p *Protection) handleRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		IP string `json:"ip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := p.RemoveEntry(req.IP); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (p *Protection) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	config := p.GetConfig()
	json.NewEncoder(w).Encode(config)
}

func (p *Protection) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var config ProtectionConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	p.UpdateConfig(config)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (p *Protection) handleCleanup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	p.CleanupExpired()
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
