// Package geoipfirewall provides GeoIP-based firewall for NAS-OS.
// Features:
//   - Country/region-based IP blocking
//   - Automatic GeoIP database management
//   - Real-time connection filtering
//   - Threat intelligence integration
//   - Allow/deny rule management with priority
//   - Connection logging and statistics
package geoipfirewall

import (
	"net"
	"sync"
	"time"
)

// Manager is the central GeoIP firewall manager.
type Manager struct {
	mu            sync.RWMutex
	config        Config
	rules         map[string]*Rule
	geoDB         *GeoDatabase
	stats         *Stats
	blockedConns  []BlockedConnection
	logger        Logger
	stopCh        chan struct{}
}

// Logger interface for firewall logging.
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Debug(msg string, keysAndValues ...interface{})
}

// Config holds GeoIP firewall configuration.
type Config struct {
	Enabled           bool     `json:"enabled"`
	DefaultAction     Action   `json:"defaultAction"`     // allow or deny
	GeoDBPath         string   `json:"geoDbPath"`         // Path to GeoIP database
	GeoDBUpdateURL    string   `json:"geoDbUpdateUrl"`    // URL for auto-updates
	UpdateInterval    int      `json:"updateInterval"`    // Hours between updates
	BlockedCountries  []string `json:"blockedCountries"`  // ISO country codes
	AllowedCountries  []string `json:"allowedCountries"`  // Whitelist (overrides blocks)
	ThreatFeedURL     string   `json:"threatFeedUrl"`     // Threat intelligence feed
	ThreatFeedEnabled bool     `json:"threatFeedEnabled"`
	MaxLogEntries     int      `json:"maxLogEntries"`
	RateLimitPerSec   int      `json:"rateLimitPerSec"`   // Rate limiting
	EnableIPv6        bool     `json:"enableIpv6"`
}

// Action represents a firewall action.
type Action string

const (
	ActionAllow Action = "allow"
	ActionDeny  Action = "deny"
	ActionLog   Action = "log"
)

// Rule represents a GeoIP firewall rule.
type Rule struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Action      Action    `json:"action"`
	Countries   []string  `json:"countries"`   // ISO 3166-1 alpha-2 codes
	Regions     []string  `json:"regions"`     // Continent codes
	IPRanges    []string  `json:"ipRanges"`    // CIDR notation
	Priority    int       `json:"priority"`    // Higher = checked first
	Enabled     bool      `json:"enabled"`
	LogAction   bool      `json:"logAction"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	HitCount    int64     `json:"hitCount"`
}

// GeoDatabase holds GeoIP lookup data.
type GeoDatabase struct {
	mu        sync.RWMutex
	entries   map[string]*GeoEntry // IP prefix -> geo info
	countries map[string]*CountryInfo
	version   string
	updatedAt time.Time
	source    string
}

// GeoEntry represents a GeoIP database entry.
type GeoEntry struct {
	IPRange     net.IPNet `json:"ipRange"`
	CountryCode string    `json:"countryCode"`
	CountryName string    `json:"countryName"`
	Region      string    `json:"region"`      // Continent
	City        string    `json:"city"`
	ASN         int       `json:"asn"`
	ASOrg       string    `json:"asOrg"`
	IsProxy     bool      `json:"isProxy"`
	IsHosting   bool      `json:"isHosting"`
	ThreatLevel int       `json:"threatLevel"` // 0-10
}

// CountryInfo holds country-level information.
type CountryInfo struct {
	Code      string   `json:"code"`
	Name      string   `json:"name"`
	Region    string   `json:"region"`
	IsBlocked bool     `json:"isBlocked"`
	IsAllowed bool     `json:"isAllowed"`
	IPCount   int64    `json:"ipCount"`
}

// BlockedConnection represents a blocked connection attempt.
type BlockedConnection struct {
	Timestamp   time.Time `json:"timestamp"`
	RemoteIP    string    `json:"remoteIp"`
	RemotePort  int       `json:"remotePort"`
	LocalPort   int       `json:"localPort"`
	Protocol    string    `json:"protocol"`
	CountryCode string    `json:"countryCode"`
	CountryName string    `json:"countryName"`
	RuleID      string    `json:"ruleId"`
	RuleName    string    `json:"ruleName"`
	Action      Action    `json:"action"`
	ASN         int       `json:"asn"`
	ASOrg       string    `json:"asOrg"`
	ThreatLevel int       `json:"threatLevel"`
}

// Stats holds firewall statistics.
type Stats struct {
	mu                sync.RWMutex
	TotalConnections  int64            `json:"totalConnections"`
	BlockedCount      int64            `json:"blockedCount"`
	AllowedCount      int64            `json:"allowedCount"`
	CountryStats      map[string]int64 `json:"countryStats"`
	TopBlocked        []CountryCount   `json:"topBlocked"`
	LastUpdated       time.Time        `json:"lastUpdated"`
	GeoDBVersion      string           `json:"geoDbVersion"`
	GeoDBLastUpdate   time.Time        `json:"geoDbLastUpdate"`
	RulesCount        int              `json:"rulesCount"`
	ThreatFeedSize    int              `json:"threatFeedSize"`
}

// CountryCount represents a country with its connection count.
type CountryCount struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

// ThreatEntry represents a threat intelligence entry.
type ThreatEntry struct {
	IP          string    `json:"ip"`
	ThreatType  string    `json:"threatType"`
	Severity    int       `json:"severity"` // 1-10
	Description string    `json:"description"`
	Source      string    `json:"source"`
	FirstSeen   time.Time `json:"firstSeen"`
	LastSeen    time.Time `json:"lastSeen"`
}

// CheckResult represents the result of a connection check.
type CheckResult struct {
	Allowed     bool      `json:"allowed"`
	Action      Action    `json:"action"`
	RuleID      string    `json:"ruleId,omitempty"`
	RuleName    string    `json:"ruleName,omitempty"`
	CountryCode string    `json:"countryCode,omitempty"`
	CountryName string    `json:"countryName,omitempty"`
	ThreatLevel int       `json:"threatLevel"`
	Reason      string    `json:"reason"`
}

// DefaultConfig returns a default GeoIP firewall configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:          true,
		DefaultAction:    ActionAllow,
		GeoDBPath:        "/var/lib/nas-os/geoip/GeoLite2-Country.mmdb",
		GeoDBUpdateURL:   "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb",
		UpdateInterval:   168, // Weekly
		BlockedCountries: []string{},
		AllowedCountries: []string{},
		MaxLogEntries:    10000,
		RateLimitPerSec:  1000,
		EnableIPv6:       true,
	}
}
