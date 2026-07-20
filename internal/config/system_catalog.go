package config

// SystemPackageKind classifies official system package IDs (ADR-0001 Stage 3 catalog).
type SystemPackageKind string

const (
	// KindRecommendedProduct is expanded when packages.recommended_system is true.
	// These gate optional product managers (docker/VM/photos/…), not HTTP extension mounts.
	KindRecommendedProduct SystemPackageKind = "recommended_product"
	// KindHTTPExtension is an official HTTP extension under internal/extensions/*
	// enabled only via packages.enabled (or deprecated modules.extensions).
	KindHTTPExtension SystemPackageKind = "http_extension"
)

// SystemPackageEntry is one official system package ID in the Stage-3 catalog.
type SystemPackageEntry struct {
	ID          string
	Kind        SystemPackageKind
	Description string
	// InternalPath is a documentation hint (not a load path).
	InternalPath string
}

// SystemPackageCatalog is the single source of truth for official system package IDs.
// Physical directories are not relocated in Stage 3; this is the product catalog only.
var SystemPackageCatalog = []SystemPackageEntry{
	// Recommended product surface (optional managers when recommended_system=true).
	{ID: "docker", Kind: KindRecommendedProduct, Description: "Docker container management", InternalPath: "internal/docker"},
	{ID: "vm", Kind: KindRecommendedProduct, Description: "Virtual machine management", InternalPath: "internal/vm"},
	{ID: "photos", Kind: KindRecommendedProduct, Description: "Photo library", InternalPath: "internal/photos"},
	{ID: "ai", Kind: KindRecommendedProduct, Description: "Local AI services", InternalPath: "internal/ai"},
	{ID: "backup", Kind: KindRecommendedProduct, Description: "Backup management", InternalPath: "internal/backup"},
	{ID: "cloudsync", Kind: KindRecommendedProduct, Description: "Cloud sync", InternalPath: "internal/cloudsync"},
	{ID: "downloader", Kind: KindRecommendedProduct, Description: "Download manager", InternalPath: "internal/downloader"},
	{ID: "cluster", Kind: KindRecommendedProduct, Description: "Cluster services", InternalPath: "internal/cluster"},

	// Official HTTP extensions (Package Runtime catalog / packages.enabled).
	{ID: "activeprotect", Kind: KindHTTPExtension, Description: "Active protect", InternalPath: "internal/extensions/activeprotect"},
	{ID: "agentworkflow", Kind: KindHTTPExtension, Description: "Agent workflow", InternalPath: "internal/extensions/agentworkflow"},
	{ID: "aiguardrails", Kind: KindHTTPExtension, Description: "AI guardrails", InternalPath: "internal/extensions/aiguardrails"},
	{ID: "compliancescan", Kind: KindHTTPExtension, Description: "Compliance scan", InternalPath: "internal/extensions/compliancescan"},
	{ID: "deployorch", Kind: KindHTTPExtension, Description: "Deploy orchestrator", InternalPath: "internal/extensions/deployorch"},
	{ID: "netdiag", Kind: KindHTTPExtension, Description: "Network diagnostics", InternalPath: "internal/extensions/netdiag"},
	{ID: "voicehub", Kind: KindHTTPExtension, Description: "Voice hub", InternalPath: "internal/extensions/voicehub"},
}

// RecommendedSystemPackageIDs returns catalog IDs of kind recommended_product (sorted stable by catalog order).
func RecommendedSystemPackageIDs() []string {
	var out []string
	for _, e := range SystemPackageCatalog {
		if e.Kind == KindRecommendedProduct {
			out = append(out, e.ID)
		}
	}
	return out
}

// HTTPExtensionPackageIDs returns catalog IDs of kind http_extension (catalog order).
func HTTPExtensionPackageIDs() []string {
	var out []string
	for _, e := range SystemPackageCatalog {
		if e.Kind == KindHTTPExtension {
			out = append(out, e.ID)
		}
	}
	return out
}

// LookupSystemPackage returns the catalog entry for id (case-insensitive), or false.
func LookupSystemPackage(id string) (SystemPackageEntry, bool) {
	want := normalizePackageNames([]string{id})
	if len(want) == 0 {
		return SystemPackageEntry{}, false
	}
	for _, e := range SystemPackageCatalog {
		if e.ID == want[0] {
			return e, true
		}
	}
	return SystemPackageEntry{}, false
}

// IsCatalogedSystemPackage reports whether id is in SystemPackageCatalog.
func IsCatalogedSystemPackage(id string) bool {
	_, ok := LookupSystemPackage(id)
	return ok
}
