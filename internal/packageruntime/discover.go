package packageruntime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"nas-os/pkg/hostapi"
)

// DiskManifest is the on-disk package layout (manifest.json) for community/local packages.
// Packages are discovered without being in the official SystemPackageCatalog.
type DiskManifest struct {
	ID           string   `json:"id"`
	Name         string   `json:"name,omitempty"`
	Version      string   `json:"version,omitempty"`
	Description  string   `json:"description,omitempty"`
	Trust        string   `json:"trust,omitempty"` // "community" | "local" (default local)
	Capabilities []string `json:"capabilities,omitempty"`
	// HostAPI is the minimum host SDK version declared by the package (e.g. "1.0.0").
	HostAPI string `json:"host_api,omitempty"`
	// Entry selects how the host materializes the package. Supported:
	//   "host-sdk" (default) — in-process Host SDK lifecycle adapter (no .so required).
	Entry string `json:"entry,omitempty"`
	// Path is filled by discovery (absolute package directory).
	Path string `json:"-"`
}

// DiscoverDir scans root for subdirectories containing manifest.json.
// Missing or empty root returns nil, nil (not an error) so default installs stay quiet.
func DiscoverDir(root string) ([]DiskManifest, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, nil
	}
	st, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("community dir: %w", err)
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("community dir is not a directory: %s", root)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read community dir: %w", err)
	}

	var out []DiskManifest
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		manPath := filepath.Join(dir, "manifest.json")
		data, err := os.ReadFile(manPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read %s: %w", manPath, err)
		}
		var m DiskManifest
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("parse %s: %w", manPath, err)
		}
		m.Path = dir
		if err := ValidateDiskManifest(m); err != nil {
			return nil, fmt.Errorf("%s: %w", manPath, err)
		}
		out = append(out, m)
	}
	return out, nil
}

// ValidateDiskManifest enforces community/local trust rules (fail closed).
func ValidateDiskManifest(m DiskManifest) error {
	id := strings.ToLower(strings.TrimSpace(m.ID))
	if id == "" {
		return fmt.Errorf("manifest id is empty")
	}
	trust := hostapi.Trust(strings.ToLower(strings.TrimSpace(m.Trust)))
	if trust == "" {
		trust = hostapi.TrustLocal
	}
	switch trust {
	case hostapi.TrustCommunity, hostapi.TrustLocal:
		// ok
	case hostapi.TrustSystem, hostapi.TrustPlatform:
		return fmt.Errorf("trust %q not allowed for on-disk third-party packages", trust)
	default:
		return fmt.Errorf("unknown trust %q", trust)
	}
	for _, raw := range m.Capabilities {
		cap := hostapi.Capability(strings.ToLower(strings.TrimSpace(raw)))
		if cap == "" {
			continue
		}
		if !cap.AllowsCommunity() {
			return fmt.Errorf("capability %q not allowed for community/local packages", cap)
		}
	}
	entry := strings.ToLower(strings.TrimSpace(m.Entry))
	if entry == "" {
		entry = "host-sdk"
	}
	if entry != "host-sdk" {
		return fmt.Errorf("unsupported entry %q (supported: host-sdk)", entry)
	}
	return nil
}

// ManifestToMeta converts a validated disk manifest to hostapi.Meta.
func ManifestToMeta(m DiskManifest) hostapi.Meta {
	trust := hostapi.Trust(strings.ToLower(strings.TrimSpace(m.Trust)))
	if trust == "" {
		trust = hostapi.TrustLocal
	}
	var caps []hostapi.Capability
	for _, raw := range m.Capabilities {
		c := hostapi.Capability(strings.ToLower(strings.TrimSpace(raw)))
		if c == "" {
			continue
		}
		caps = append(caps, c)
	}
	if len(caps) == 0 {
		caps = []hostapi.Capability{hostapi.CapHostSDK}
	}
	desc := m.Description
	if desc == "" {
		desc = m.Name
	}
	return hostapi.Meta{
		ID:           strings.ToLower(strings.TrimSpace(m.ID)),
		Trust:        trust,
		Description:  desc,
		Version:      m.Version,
		Capabilities: caps,
	}
}
