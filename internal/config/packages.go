package config

import (
	"sort"
	"strings"
)

// RecommendedSystemPackages is the official system package ID set expanded when
// modules.optional or packages.recommended_system is true (ADR-0001 Stage 1).
// Non-empty by design so recommended/optional resolution is observable.
// IDs name the optional product surface gated today (docker/VM/photos/…);
// HTTP catalog extensions (voicehub, …) remain opt-in via modules.extensions
// or packages.enabled so optional=true does not change extension mounting.
var RecommendedSystemPackages = []string{
	"docker",
	"vm",
	"photos",
	"ai",
	"backup",
	"cloudsync",
	"downloader",
	"cluster",
}

// PackagesConfig is the ADR Stage-1 packages surface (dual-read with modules.*).
type PackagesConfig struct {
	// RecommendedSystem enables the official recommended system package set.
	// Equivalent to modules.optional when only modules.* is set.
	RecommendedSystem bool `yaml:"recommended_system"`
	// Enabled lists system package IDs to enable (unioned with modules.extensions).
	Enabled []string `yaml:"enabled"`
}

// PackageResolution is the unified enablement result after dual-read merge.
type PackageResolution struct {
	// RecommendedSystem is true when modules.optional or packages.recommended_system.
	// Production gates non-Core product managers on this flag.
	RecommendedSystem bool
	// Enabled is the sorted unique union of modules.extensions, packages.enabled,
	// and RecommendedSystemPackages when RecommendedSystem is true.
	Enabled []string
	// DualSource is true when both modules.* and packages.* contributed enablement.
	DualSource bool
	// Warnings holds human-readable dual-source notes (may be empty).
	Warnings []string
}

// ResolvePackages merges modules.* and packages.* per ADR-0001 dual-read rules.
// Pure function of config fields — no I/O.
func (c *Config) ResolvePackages() PackageResolution {
	if c == nil {
		return PackageResolution{}
	}

	modOptional := c.Modules.Optional
	modExt := normalizePackageNames(c.Modules.Extensions)
	pkgRec := c.Packages.RecommendedSystem
	pkgEn := normalizePackageNames(c.Packages.Enabled)

	modulesContributed := modOptional || len(modExt) > 0
	packagesContributed := pkgRec || len(pkgEn) > 0

	res := PackageResolution{
		RecommendedSystem: modOptional || pkgRec,
		DualSource:        modulesContributed && packagesContributed,
	}
	if res.DualSource {
		res.Warnings = append(res.Warnings,
			"both modules.* and packages.* set; enabled packages use union (ADR-0001 Stage 1)")
	}

	// Named enablement: union of both lists.
	named := unionSorted(modExt, pkgEn)

	// When recommended/optional is on, expand official set then union with named.
	if res.RecommendedSystem {
		res.Enabled = unionSorted(RecommendedSystemPackages, named)
	} else {
		res.Enabled = named
	}
	return res
}

// OptionalProductsEnabled reports whether non-Core product managers should run.
// Prefer this over reading Modules.Optional in isolation (ADR Stage 1).
func (c *Config) OptionalProductsEnabled() bool {
	return c.ResolvePackages().RecommendedSystem
}

// EnabledPackageNames returns the resolved package ID list (copy).
func (c *Config) EnabledPackageNames() []string {
	en := c.ResolvePackages().Enabled
	if len(en) == 0 {
		return nil
	}
	out := make([]string, len(en))
	copy(out, en)
	return out
}

// EnabledNamedPackages returns enabled package names that appear in known,
// excluding pure recommended-surface IDs that are not in known (for extension mount).
// If known is empty, returns all Enabled names.
func (c *Config) EnabledNamedPackages(known []string) []string {
	en := c.ResolvePackages().Enabled
	if len(en) == 0 {
		return nil
	}
	if len(known) == 0 {
		out := make([]string, len(en))
		copy(out, en)
		return out
	}
	allow := make(map[string]struct{}, len(known))
	for _, k := range known {
		k = strings.ToLower(strings.TrimSpace(k))
		if k != "" {
			allow[k] = struct{}{}
		}
	}
	var out []string
	for _, name := range en {
		if _, ok := allow[name]; ok {
			out = append(out, name)
		}
	}
	return out
}

func normalizePackageNames(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	var out []string
	for _, raw := range in {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func unionSorted(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	var out []string
	for _, list := range [][]string{a, b} {
		for _, name := range list {
			name = strings.ToLower(strings.TrimSpace(name))
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}
