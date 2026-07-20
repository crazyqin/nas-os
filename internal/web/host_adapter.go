package web

import (
	"log"
	"path/filepath"

	"nas-os/internal/config"
	"nas-os/pkg/hostapi"
)

// configHost adapts config.Config to hostapi.Host for the package runtime.
type configHost struct {
	cfg *config.Config
}

func newConfigHost(cfg *config.Config) *configHost {
	return &configHost{cfg: cfg}
}

func (h *configHost) APIVersion() string { return hostapi.APIVersion }

func (h *configHost) DataDir() string {
	if h == nil || h.cfg == nil {
		return ""
	}
	return h.cfg.Paths.DataDir
}

func (h *configHost) ConfigDir() string {
	if h == nil || h.cfg == nil {
		return ""
	}
	return h.cfg.Paths.ConfigDir
}

func (h *configHost) DataPath(elem ...string) string {
	if h == nil || h.cfg == nil {
		return filepath.Join(append([]string{""}, elem...)...)
	}
	return h.cfg.DataPath(elem...)
}

func (h *configHost) ConfigPath(elem ...string) string {
	if h == nil || h.cfg == nil {
		return filepath.Join(append([]string{""}, elem...)...)
	}
	return h.cfg.ConfigPath(elem...)
}

func (h *configHost) Logf(format string, args ...any) {
	log.Printf("[hostapi] "+format, args...)
}

// Allows: Stage 2 host accepts system packages for HTTP catalog; community/local
// may load lifecycle only when explicitly allowed later. Default allows all
// non-empty trust classes used by the runtime (trust gate still applied for
// MinTrust-style hosts in tests).
func (h *configHost) Allows(trust hostapi.Trust) bool {
	switch trust {
	case hostapi.TrustSystem, hostapi.TrustCommunity, hostapi.TrustLocal, hostapi.TrustPlatform:
		return true
	default:
		return false
	}
}
