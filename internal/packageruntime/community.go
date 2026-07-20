package packageruntime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"nas-os/pkg/hostapi"
)

// RegisterDiscovered registers on-disk third-party packages into the runtime catalog.
// Does not enable/start them; callers Enable explicitly. Official SystemPackageCatalog
// IDs are not required. Returns registered IDs (normalized).
func (r *Runtime) RegisterDiscovered(manifests []DiskManifest) (registered []string, err error) {
	if r == nil {
		return nil, fmt.Errorf("runtime is nil")
	}
	for _, m := range manifests {
		if err := ValidateDiskManifest(m); err != nil {
			return registered, fmt.Errorf("manifest %s: %w", m.ID, err)
		}
		meta := ManifestToMeta(m)
		// Refuse to overwrite system trust entries with third-party packages.
		r.mu.RLock()
		existing, exists := r.catalog[meta.ID]
		r.mu.RUnlock()
		if exists && existing.Meta.Trust == hostapi.TrustSystem {
			return registered, fmt.Errorf("package %q already registered as system; cannot replace with community/local", meta.ID)
		}

		pkgDir := m.Path
		md := meta
		if err := r.Register(md, func(h hostapi.Host) (hostapi.Package, error) {
			return NewHostSDKPackage(md, pkgDir), nil
		}); err != nil {
			return registered, err
		}
		registered = append(registered, meta.ID)
	}
	return registered, nil
}

// HostSDKPackage is the default third-party package: Host SDK lifecycle only.
// It never mounts admin HTTP routes (does not implement HTTPMounter).
type HostSDKPackage struct {
	meta   hostapi.Meta
	pkgDir string
	host   hostapi.Host
	started bool
}

// NewHostSDKPackage builds a community/local package bound to a disk path.
func NewHostSDKPackage(meta hostapi.Meta, pkgDir string) *HostSDKPackage {
	return &HostSDKPackage{meta: meta, pkgDir: pkgDir}
}

// Meta implements hostapi.Package.
func (p *HostSDKPackage) Meta() hostapi.Meta { return p.meta }

// Init validates host API and capabilities via the real Host.
func (p *HostSDKPackage) Init(ctx context.Context, host hostapi.Host) error {
	if host == nil {
		return fmt.Errorf("host is nil")
	}
	p.host = host
	if host.APIVersion() == "" {
		return fmt.Errorf("host API version empty")
	}
	// Community packages only receive Host SDK surface.
	for _, cap := range p.meta.Capabilities {
		if !cap.AllowsCommunity() {
			return fmt.Errorf("capability %q refused for trust %s", cap, p.meta.Trust)
		}
	}
	if p.meta.Trust.Rank() >= hostapi.TrustSystem.Rank() {
		return fmt.Errorf("host-sdk package cannot claim trust %s", p.meta.Trust)
	}
	if !host.Allows(p.meta.Trust) {
		return fmt.Errorf("host does not allow trust %s", p.meta.Trust)
	}
	host.Logf("community package init id=%s trust=%s host_api=%s dir=%s",
		p.meta.ID, p.meta.Trust, host.APIVersion(), p.pkgDir)
	return nil
}

// Start records a marker under the host data dir (proves Host path works).
func (p *HostSDKPackage) Start(ctx context.Context) error {
	if p.host == nil {
		return fmt.Errorf("not initialized")
	}
	markerDir := p.host.DataPath("community-packages", sanitizeID(p.meta.ID))
	if err := os.MkdirAll(markerDir, 0o750); err != nil {
		return fmt.Errorf("marker dir: %w", err)
	}
	marker := filepath.Join(markerDir, "started")
	content := fmt.Sprintf("id=%s\nversion=%s\nhost_api=%s\n", p.meta.ID, p.meta.Version, p.host.APIVersion())
	if err := os.WriteFile(marker, []byte(content), 0o640); err != nil {
		return fmt.Errorf("write marker: %w", err)
	}
	p.started = true
	p.host.Logf("community package started id=%s marker=%s", p.meta.ID, marker)
	return nil
}

// Stop removes the started marker.
func (p *HostSDKPackage) Stop(ctx context.Context) error {
	if p.host == nil {
		return nil
	}
	marker := p.host.DataPath("community-packages", sanitizeID(p.meta.ID), "started")
	_ = os.Remove(marker)
	p.started = false
	p.host.Logf("community package stopped id=%s", p.meta.ID)
	return nil
}

// Health reports started state.
func (p *HostSDKPackage) Health(ctx context.Context) error {
	if !p.started {
		return fmt.Errorf("not started")
	}
	return nil
}

// Started reports whether Start completed (tests).
func (p *HostSDKPackage) Started() bool { return p.started }

func sanitizeID(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	var b strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}
