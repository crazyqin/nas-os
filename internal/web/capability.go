package web

import (
	"fmt"
	"strings"

	"nas-os/internal/config"
)

// ValidateBinaryCapabilities returns an error when config requests products or
// HTTP extensions that are not compiled into this binary (Core vs nasd_full).
// Call from application.New so misconfigured Core deploys fail closed at boot.
func ValidateBinaryCapabilities(cfg *config.Config) error {
	if cfg == nil {
		return nil
	}
	missing := CapabilityGaps(cfg)
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf(
		"this nasd binary cannot provide: %s — rebuild with -tags nasd_full (make build-full) or clear packages.enabled / packages.recommended_system / modules.optional / app-center-enabled.json",
		strings.Join(missing, ", "),
	)
}

// CapabilityGaps lists package IDs requested by config but not linked in this build.
func CapabilityGaps(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	var missing []string
	seen := make(map[string]struct{})
	add := func(id string) {
		id = strings.ToLower(strings.TrimSpace(id))
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		missing = append(missing, id)
	}

	if !ProductsLinked() {
		if cfg.OptionalProductsEnabled() || cfg.Modules.Optional {
			add("recommended_system/modules.optional")
		}
		for _, id := range cfg.BootProductIDs() {
			add(id)
		}
	}
	if !ExtensionsLinked() {
		for _, id := range cfg.EnabledPackageNames() {
			if e, ok := config.LookupSystemPackage(id); ok && e.Kind == config.KindHTTPExtension {
				add(id)
			}
		}
		// App-center SSOT may list extensions
		for _, id := range config.LoadAppCenterEnabledIDs(cfg.DataPath("app-center-enabled.json")) {
			if e, ok := config.LookupSystemPackage(id); ok && e.Kind == config.KindHTTPExtension {
				add(id)
			}
		}
	}
	return missing
}

// packageLinked reports whether this binary can host the given catalog package.
func packageLinked(id string) bool {
	id = strings.ToLower(strings.TrimSpace(id))
	e, ok := config.LookupSystemPackage(id)
	if !ok {
		// Community packages only need Host SDK runtime (always present).
		return true
	}
	switch e.Kind {
	case config.KindRecommendedProduct:
		return ProductsLinked()
	case config.KindHTTPExtension:
		return ExtensionsLinked()
	default:
		return true
	}
}
