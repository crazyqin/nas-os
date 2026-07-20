// Package hostapi is the ADR-0001 Stage-2 Host SDK: the stable surface that
// packages (system / community / local) may depend on.
//
// Versioning: bump APIVersion on incompatible changes. Prefer additive fields
// and new interfaces over breaking existing method signatures.
//
// Packages MUST NOT import nas-os/internal/* business packages; only hostapi
// (and stdlib / approved shared libs) for community trust.
package hostapi

import (
	"context"
	"net/http"
)

// APIVersion is the Host SDK major.minor contract version.
const APIVersion = "1.0.0"

// Trust is the package trust class (orthogonal to enablement lists).
type Trust string

const (
	// TrustPlatform is compiled into Core (not a loadable package).
	TrustPlatform Trust = "platform"
	// TrustSystem is an official monorepo package (extensions / optional products).
	TrustSystem Trust = "system"
	// TrustCommunity is a third-party signed package (sandbox + hostapi only).
	TrustCommunity Trust = "community"
	// TrustLocal is a developer-installed package (strictest / dev).
	TrustLocal Trust = "local"
)

// Rank returns a relative privilege rank (higher = more trusted).
func (t Trust) Rank() int {
	switch t {
	case TrustPlatform:
		return 40
	case TrustSystem:
		return 30
	case TrustCommunity:
		return 20
	case TrustLocal:
		return 10
	default:
		return 0
	}
}

// Capability is a declared package privilege request (host-evaluated).
type Capability string

const (
	// CapHTTPAdmin requests unrestricted admin HTTP route mount (system-only).
	CapHTTPAdmin Capability = "http.admin"
	// CapHostSDK is the baseline community/local surface (Host interface only).
	CapHostSDK Capability = "host.sdk"
)

// Meta describes a package for catalog and status APIs.
type Meta struct {
	ID           string       `json:"id"`
	Trust        Trust        `json:"trust"`
	Description  string       `json:"description,omitempty"`
	Version      string       `json:"version,omitempty"`
	Capabilities []Capability `json:"capabilities,omitempty"`
}

// AllowsCommunity reports whether the capability may be granted to community/local trust.
func (c Capability) AllowsCommunity() bool {
	switch c {
	case CapHostSDK, "":
		return true
	case CapHTTPAdmin:
		return false
	default:
		// Unknown capabilities are denied for community/local (fail closed).
		return false
	}
}

// Package is the lifecycle contract for a loadable package.
// Implementations must not start background work in Init; Start owns that.
type Package interface {
	Meta() Meta
	Init(ctx context.Context, host Host) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Health(ctx context.Context) error
}

// HTTPMounter is implemented by packages that attach admin HTTP routes.
// Registration is only invoked by the runtime for TrustSystem+ packages
// (Stage 2: community route mount remains gated).
type HTTPMounter interface {
	// MountHTTP registers routes under the host-provided admin API base.
	// method is uppercase (GET, POST, …); path is relative (e.g. "/voicehub/status").
	MountHTTP(register func(method, path string, h http.Handler)) error
}

// Host is the platform surface available to packages during Init/Start.
// Community packages should treat this as the only allowed host dependency.
type Host interface {
	// APIVersion returns hostapi.APIVersion of the running host.
	APIVersion() string
	// DataDir is the process data root (absolute).
	DataDir() string
	// ConfigDir is the process config root (absolute).
	ConfigDir() string
	// DataPath joins DataDir with elem.
	DataPath(elem ...string) string
	// ConfigPath joins ConfigDir with elem.
	ConfigPath(elem ...string) string
	// Logf writes a host-owned log line (packages should not open their own log files).
	Logf(format string, args ...any)
	// Allows reports whether the host permits the given trust class to load.
	Allows(trust Trust) bool
}

// StaticHost is a minimal Host for tests and simple embeddings.
type StaticHost struct {
	Data   string
	Config string
	Log    func(format string, args ...any)
	// MinTrust when non-empty rejects packages with lower Rank.
	MinTrust Trust
}

// APIVersion implements Host.
func (h *StaticHost) APIVersion() string { return APIVersion }

// DataDir implements Host.
func (h *StaticHost) DataDir() string { return h.Data }

// ConfigDir implements Host.
func (h *StaticHost) ConfigDir() string { return h.Config }

// DataPath implements Host.
func (h *StaticHost) DataPath(elem ...string) string {
	return join(h.Data, elem...)
}

// ConfigPath implements Host.
func (h *StaticHost) ConfigPath(elem ...string) string {
	return join(h.Config, elem...)
}

// Logf implements Host.
func (h *StaticHost) Logf(format string, args ...any) {
	if h.Log != nil {
		h.Log(format, args...)
	}
}

// Allows implements Host.
func (h *StaticHost) Allows(trust Trust) bool {
	if h.MinTrust == "" {
		return trust == TrustSystem || trust == TrustCommunity || trust == TrustLocal || trust == TrustPlatform
	}
	return trust.Rank() >= h.MinTrust.Rank()
}

func join(base string, elem ...string) string {
	if base == "" {
		return ""
	}
	out := base
	for _, e := range elem {
		if e == "" {
			continue
		}
		if out != "" && out[len(out)-1] != '/' {
			out += "/"
		}
		out += e
	}
	return out
}
