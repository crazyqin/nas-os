package application

import (
	"testing"

	"nas-os/internal/arch"
)

func TestModuleCatalogSnapshotCoreModules(t *testing.T) {
	catalog := ModuleCatalogSnapshot()
	for _, name := range []string{moduleIdentity, moduleStorage, moduleNetwork, moduleSharing, moduleSystem} {
		if got := catalog[name]; got != arch.ModuleTierCore {
			t.Fatalf("catalog[%s] = %q, want %q", name, got, arch.ModuleTierCore)
		}
	}
}

func TestModuleCatalogSnapshotDemotesPseudoCoreModules(t *testing.T) {
	catalog := ModuleCatalogSnapshot()
	cases := map[string]arch.ModuleTier{
		"activeprotect":    arch.ModuleTierExtension,
		"agentworkflow":    arch.ModuleTierExtension,
		"brandinsight":     arch.ModuleTierLab,
		"datasovereignty2": arch.ModuleTierLab,
		"updatedirector":   arch.ModuleTierLab,
	}
	for name, want := range cases {
		if got := catalog[name]; got != want {
			t.Fatalf("catalog[%s] = %q, want %q", name, got, want)
		}
	}
}
