package network

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// FECMode describes IEEE 802.3 Forward Error Correction mode.
type FECMode string

const (
	// FECModeAuto lets NIC firmware choose the best supported mode.
	FECModeAuto FECMode = "auto"
	// FECModeOff disables Forward Error Correction.
	FECModeOff FECMode = "off"
	// FECModeRS enables Reed-Solomon FEC for high-speed links.
	FECModeRS FECMode = "rs"
	// FECModeBaseR enables BASE-R/FireCode FEC.
	FECModeBaseR FECMode = "baser"
)

// FECConfig stores per-interface FEC intent and observed state.
type FECConfig struct {
	Interface  string    `json:"interface"`
	Mode       FECMode   `json:"mode"`
	Supported  []FECMode `json:"supported"`
	Active     FECMode   `json:"active"`
	Reason     string    `json:"reason,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`
	Persistent bool      `json:"persistent"`
}

// NetworkIntent captures an auditable network configuration change request.
type NetworkIntent struct {
	ID          string    `json:"id"`
	Interface   string    `json:"interface"`
	Operation   string    `json:"operation"`
	RequestedBy string    `json:"requested_by,omitempty"`
	Summary     string    `json:"summary"`
	CreatedAt   time.Time `json:"created_at"`
}

// ConfigureFEC records a validated FEC configuration for an interface.
// The implementation is intentionally declarative: callers can persist and audit
// the desired mode, while platform-specific backends apply it with ethtool or
// netlink only when the hardware exposes FEC controls.
func (m *Manager) ConfigureFEC(iface string, mode FECMode, persistent bool) (*FECConfig, error) {
	if strings.TrimSpace(iface) == "" {
		return nil, fmt.Errorf("interface cannot be empty")
	}
	if !isValidFECMode(mode) {
		return nil, fmt.Errorf("unsupported FEC mode: %s", mode)
	}

	cfg := &FECConfig{
		Interface:  iface,
		Mode:       mode,
		Supported:  []FECMode{FECModeAuto, FECModeOff, FECModeRS, FECModeBaseR},
		Active:     mode,
		UpdatedAt:  time.Now(),
		Persistent: persistent,
	}
	if _, err := os.Stat("/sys/class/net/" + iface); err != nil {
		cfg.Reason = "interface not present on this host; stored as desired state"
	}
	return cfg, nil
}

// RecommendFEC suggests a mode based on link speed and workload profile.
func RecommendFEC(linkSpeedMbps int, latencySensitive bool) FECMode {
	switch {
	case linkSpeedMbps >= 100000:
		return FECModeRS
	case linkSpeedMbps >= 25000 && !latencySensitive:
		return FECModeRS
	case linkSpeedMbps >= 10000:
		return FECModeBaseR
	default:
		return FECModeAuto
	}
}

// BuildNetworkIntent creates an auditable intent record for network changes.
func BuildNetworkIntent(iface, operation, requestedBy, summary string) NetworkIntent {
	return NetworkIntent{
		ID:          fmt.Sprintf("net-%d", time.Now().UnixNano()),
		Interface:   iface,
		Operation:   operation,
		RequestedBy: requestedBy,
		Summary:     summary,
		CreatedAt:   time.Now(),
	}
}

func isValidFECMode(mode FECMode) bool {
	switch mode {
	case FECModeAuto, FECModeOff, FECModeRS, FECModeBaseR:
		return true
	default:
		return false
	}
}
