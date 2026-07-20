package config

import (
	"sort"
	"strings"
)

// RecommendedSystemPackages is the official recommended product package ID set
// expanded when packages.recommended_system is true (or deprecated modules.optional).
// Derived from SystemPackageCatalog (ADR-0001 Stage 3 single source of truth).
//
// HTTP catalog extensions remain opt-in via packages.enabled so recommended_system
// does not auto-mount HTTP extensions.
var RecommendedSystemPackages = RecommendedSystemPackageIDs()

// PackagesConfig is the preferred ADR-0001 configuration surface.
// Prefer packages.* over deprecated modules.optional / modules.extensions.
type PackagesConfig struct {
	// RecommendedSystem enables the official recommended system package set.
	// Prefer this over deprecated modules.optional.
	RecommendedSystem bool `yaml:"recommended_system"`
	// Enabled lists system package IDs (HTTP extensions and/or named products).
	// Prefer this over deprecated modules.extensions.
	Enabled []string `yaml:"enabled"`
}

// PackageResolution is the unified enablement result after dual-read merge.
type PackageResolution struct {
	// RecommendedSystem is true when packages.recommended_system or modules.optional.
	// Production gates non-Core product managers on this flag.
	RecommendedSystem bool
	// Enabled is the sorted unique union of packages.enabled, modules.extensions,
	// and RecommendedSystemPackages when RecommendedSystem is true.
	Enabled []string
	// DualSource is true when both modules.* and packages.* contributed enablement.
	DualSource bool
	// ModulesDeprecated is true when deprecated modules.optional/extensions contributed.
	ModulesDeprecated bool
	// Warnings holds dual-source and deprecation notes (may be empty).
	Warnings []string
}

// ResolvePackages merges packages.* (preferred) with deprecated modules.* dual-read.
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
		ModulesDeprecated: modulesContributed,
	}
	if modulesContributed {
		res.Warnings = append(res.Warnings,
			"modules.optional / modules.extensions are deprecated (ADR-0001 Stage 3); prefer packages.recommended_system / packages.enabled")
	}
	if res.DualSource {
		res.Warnings = append(res.Warnings,
			"both modules.* and packages.* set; enabled packages use union (compatibility dual-read)")
	}

	// Named enablement: union of both lists.
	named := unionSorted(modExt, pkgEn)

	// When recommended/optional is on, expand official set then union with named.
	if res.RecommendedSystem {
		// Prefer live catalog-derived list; fall back if catalog empty (should not happen).
		rec := RecommendedSystemPackages
		if len(rec) == 0 {
			rec = RecommendedSystemPackageIDs()
		}
		res.Enabled = unionSorted(rec, named)
	} else {
		res.Enabled = named
	}
	return res
}

// OptionalProductsEnabled reports whether non-Core product managers should run.
// Prefer this over reading Modules.Optional in isolation.
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
