package geoipfirewall

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// NewManager creates a new GeoIP firewall manager.
func NewManager(config Config, logger Logger) (*Manager, error) {
	if config.MaxLogEntries <= 0 {
		config.MaxLogEntries = 10000
	}

	m := &Manager{
		config:       config,
		rules:        make(map[string]*Rule),
		geoDB:        &GeoDatabase{entries: make(map[string]*GeoEntry), countries: make(map[string]*CountryInfo)},
		stats:        &Stats{CountryStats: make(map[string]int64)},
		blockedConns: make([]BlockedConnection, 0),
		logger:       logger,
		stopCh:       make(chan struct{}),
	}

	// Load rules
	if err := m.loadRules(); err != nil {
		logger.Warn("failed to load firewall rules", "error", err)
	}

	// Load GeoIP database
	if err := m.loadGeoDB(); err != nil {
		logger.Warn("GeoIP database not loaded, geo-blocking disabled", "error", err)
	}

	// Start auto-update goroutine
	if config.UpdateInterval > 0 {
		go m.autoUpdateLoop()
	}

	return m, nil
}

// CheckIP checks if an IP address should be allowed or blocked.
func (m *Manager) CheckIP(ipStr string) (*CheckResult, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ipStr)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Update total connections
	m.stats.mu.Lock()
	m.stats.TotalConnections++
	m.stats.mu.Unlock()

	// Look up geo info
	geo := m.geoDB.Lookup(ip)

	result := &CheckResult{
		Allowed: true,
		Action:  m.config.DefaultAction,
	}

	if geo != nil {
		result.CountryCode = geo.CountryCode
		result.CountryName = geo.CountryName
		result.ThreatLevel = geo.ThreatLevel
	}

	// Check rules by priority (highest first)
	sortedRules := m.getSortedRules()
	for _, rule := range sortedRules {
		if !rule.Enabled {
			continue
		}
		if m.ruleMatches(rule, ip, geo) {
			result.Action = rule.Action
			result.Allowed = rule.Action == ActionAllow
			result.RuleID = rule.ID
			result.RuleName = rule.Name

			// Update rule hit count
			rule.HitCount++

			// Log if needed
			if rule.LogAction {
				m.logConnection(ipStr, 0, 0, "tcp", geo, rule)
			}

			break
		}
	}

	// Update stats
	m.stats.mu.Lock()
	if result.Allowed {
		m.stats.AllowedCount++
		result.Reason = "allowed by default"
	} else {
		m.stats.BlockedCount++
		result.Reason = fmt.Sprintf("blocked by rule: %s", result.RuleName)
		if geo != nil {
			m.stats.CountryStats[geo.CountryCode]++
		}
	}
	m.stats.LastUpdated = time.Now()
	m.stats.mu.Unlock()

	return result, nil
}

// AddRule adds a new firewall rule.
func (m *Manager) AddRule(rule *Rule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		rule.ID = generateID("rule")
	}
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	m.rules[rule.ID] = rule

	if err := m.saveRules(); err != nil {
		return fmt.Errorf("save rules: %w", err)
	}

	m.logger.Info("firewall rule added", "ruleId", rule.ID, "name", rule.Name, "action", rule.Action)
	return nil
}

// UpdateRule updates an existing firewall rule.
func (m *Manager) UpdateRule(rule *Rule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rules[rule.ID]; !exists {
		return fmt.Errorf("rule %s not found", rule.ID)
	}

	rule.UpdatedAt = time.Now()
	m.rules[rule.ID] = rule

	return m.saveRules()
}

// DeleteRule deletes a firewall rule.
func (m *Manager) DeleteRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rules[id]; !exists {
		return fmt.Errorf("rule %s not found", id)
	}

	delete(m.rules, id)
	return m.saveRules()
}

// GetRule returns a rule by ID.
func (m *Manager) GetRule(id string) (*Rule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, exists := m.rules[id]
	if !exists {
		return nil, fmt.Errorf("rule %s not found", id)
	}
	return rule, nil
}

// ListRules returns all rules.
func (m *Manager) ListRules() []*Rule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*Rule, 0, len(m.rules))
	for _, r := range m.rules {
		rules = append(rules, r)
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority > rules[j].Priority
	})
	return rules
}

// BlockCountry adds a country to the block list.
func (m *Manager) BlockCountry(countryCode string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	code := strings.ToUpper(countryCode)
	for _, c := range m.config.BlockedCountries {
		if c == code {
			return nil // Already blocked
		}
	}
	m.config.BlockedCountries = append(m.config.BlockedCountries, code)
	m.logger.Info("country blocked", "code", code)
	return m.saveConfig()
}

// UnblockCountry removes a country from the block list.
func (m *Manager) UnblockCountry(countryCode string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	code := strings.ToUpper(countryCode)
	for i, c := range m.config.BlockedCountries {
		if c == code {
			m.config.BlockedCountries = append(m.config.BlockedCountries[:i], m.config.BlockedCountries[i+1:]...)
			m.logger.Info("country unblocked", "code", code)
			return m.saveConfig()
		}
	}
	return nil
}

// GetStats returns firewall statistics.
func (m *Manager) GetStats() map[string]interface{} {
	m.stats.mu.RLock()
	defer m.stats.mu.RUnlock()

	m.mu.RLock()
	ruleCount := len(m.rules)
	geoVersion := m.geoDB.version
	geoUpdated := m.geoDB.updatedAt
	m.mu.RUnlock()

	topBlocked := make([]CountryCount, 0)
	for code, count := range m.stats.CountryStats {
		name := code
		if info, ok := m.geoDB.countries[code]; ok {
			name = info.Name
		}
		topBlocked = append(topBlocked, CountryCount{Code: code, Name: name, Count: count})
	}
	sort.Slice(topBlocked, func(i, j int) bool {
		return topBlocked[i].Count > topBlocked[j].Count
	})
	if len(topBlocked) > 10 {
		topBlocked = topBlocked[:10]
	}

	return map[string]interface{}{
		"totalConnections": m.stats.TotalConnections,
		"blockedCount":     m.stats.BlockedCount,
		"allowedCount":     m.stats.AllowedCount,
		"topBlocked":       topBlocked,
		"rulesCount":       ruleCount,
		"geoDbVersion":     geoVersion,
		"geoDbLastUpdate":  geoUpdated,
		"blockedCountries": m.config.BlockedCountries,
		"enabled":          m.config.Enabled,
	}
}

// GetBlockedConnections returns recent blocked connections.
func (m *Manager) GetBlockedConnections(limit int) []BlockedConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.blockedConns) {
		limit = len(m.blockedConns)
	}

	// Return most recent first
	start := len(m.blockedConns) - limit
	if start < 0 {
		start = 0
	}
	result := make([]BlockedConnection, limit)
	for i, j := 0, start; j < len(m.blockedConns); i, j = i+1, j+1 {
		result[i] = m.blockedConns[j]
	}
	return result
}

// UpdateGeoDB forces a GeoIP database update.
func (m *Manager) UpdateGeoDB() error {
	m.logger.Info("GeoIP database update requested")
	// In a real implementation, this would download from the configured URL
	return nil
}

// Stop stops the firewall manager.
func (m *Manager) Stop() {
	close(m.stopCh)
	m.logger.Info("GeoIP firewall stopped")
}

// ========== Internal Methods ==========

func (m *Manager) ruleMatches(rule *Rule, ip net.IP, geo *GeoEntry) bool {
	// Check country match
	if geo != nil && len(rule.Countries) > 0 {
		for _, c := range rule.Countries {
			if strings.EqualFold(c, geo.CountryCode) {
				return true
			}
		}
	}

	// Check region match
	if geo != nil && len(rule.Regions) > 0 {
		for _, r := range rule.Regions {
			if strings.EqualFold(r, geo.Region) {
				return true
			}
		}
	}

	// Check IP range match
	for _, cidr := range rule.IPRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

func (m *Manager) getSortedRules() []*Rule {
	rules := make([]*Rule, 0, len(m.rules))
	for _, r := range m.rules {
		rules = append(rules, r)
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority > rules[j].Priority
	})
	return rules
}

func (m *Manager) logConnection(ip string, remotePort, localPort int, protocol string, geo *GeoEntry, rule *Rule) {
	conn := BlockedConnection{
		Timestamp:  time.Now(),
		RemoteIP:   ip,
		RemotePort: remotePort,
		LocalPort:  localPort,
		Protocol:   protocol,
		Action:     rule.Action,
		RuleID:     rule.ID,
		RuleName:   rule.Name,
	}
	if geo != nil {
		conn.CountryCode = geo.CountryCode
		conn.CountryName = geo.CountryName
		conn.ASN = geo.ASN
		conn.ASOrg = geo.ASOrg
		conn.ThreatLevel = geo.ThreatLevel
	}

	m.blockedConns = append(m.blockedConns, conn)

	// Trim if too many entries
	if len(m.blockedConns) > m.config.MaxLogEntries {
		m.blockedConns = m.blockedConns[len(m.blockedConns)-m.config.MaxLogEntries:]
	}
}

func (m *Manager) autoUpdateLoop() {
	interval := time.Duration(m.config.UpdateInterval) * time.Hour
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := m.UpdateGeoDB(); err != nil {
				m.logger.Error("GeoIP database update failed", "error", err)
			}
		case <-m.stopCh:
			return
		}
	}
}

// ========== Persistence ==========

func (m *Manager) loadRules() error {
	path := filepath.Join(filepath.Dir(m.config.GeoDBPath), "rules.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var rules []*Rule
	if err := json.Unmarshal(data, &rules); err != nil {
		return err
	}

	for _, r := range rules {
		m.rules[r.ID] = r
	}
	return nil
}

func (m *Manager) saveRules() error {
	dir := filepath.Dir(m.config.GeoDBPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}

	// Build sorted list directly (caller already holds mu)
	rules := make([]*Rule, 0, len(m.rules))
	for _, r := range m.rules {
		rules = append(rules, r)
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority > rules[j].Priority
	})

	path := filepath.Join(dir, "rules.json")
	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0640)
}

func (m *Manager) saveConfig() error {
	dir := filepath.Dir(m.config.GeoDBPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}

	path := filepath.Join(dir, "config.json")
	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0640)
}

func (m *Manager) loadGeoDB() error {
	// In a real implementation, this would load a MaxMind GeoLite2 database
	// For now, we set up the structure and log that the database is not available
	m.geoDB.version = "placeholder"
	m.geoDB.updatedAt = time.Now()
	return nil
}

// Lookup performs a GeoIP lookup for an IP address.
func (g *GeoDatabase) Lookup(ip net.IP) *GeoEntry {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.entries == nil {
		return nil
	}

	// Simple prefix-based lookup
	ipStr := ip.String()
	for prefix, entry := range g.entries {
		if strings.HasPrefix(ipStr, prefix) {
			return entry
		}
	}
	return nil
}

// GetCountries returns all known countries with their status.
func (m *Manager) GetCountries() []CountryInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	countries := make([]CountryInfo, 0, len(m.geoDB.countries))
	for _, info := range m.geoDB.countries {
		ci := *info
		ci.IsBlocked = contains(m.config.BlockedCountries, ci.Code)
		ci.IsAllowed = contains(m.config.AllowedCountries, ci.Code)
		countries = append(countries, ci)
	}
	sort.Slice(countries, func(i, j int) bool {
		return countries[i].Code < countries[j].Code
	})
	return countries
}

// LookupIP performs a GeoIP lookup for a given IP string.
func (m *Manager) LookupIP(ipStr string) (*GeoEntry, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ipStr)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	geo := m.geoDB.Lookup(ip)
	if geo == nil {
		return &GeoEntry{
			CountryCode: "--",
			CountryName: "Unknown",
			Region:      "Unknown",
		}, nil
	}
	return geo, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}

func generateID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}
