package application

import (
	"os"
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

func TestModuleTierForDefaultsUnknownModulesToExtension(t *testing.T) {
	if got := ModuleTierFor("totally-new-module"); got != arch.ModuleTierExtension {
		t.Fatalf("ModuleTierFor(unknown) = %q, want %q", got, arch.ModuleTierExtension)
	}
}

func TestModuleCatalogSnapshotReturnsCopy(t *testing.T) {
	catalog := ModuleCatalogSnapshot()
	catalog[moduleIdentity] = arch.ModuleTierLab
	if got := ModuleTierFor(moduleIdentity); got != arch.ModuleTierCore {
		t.Fatalf("ModuleTierFor(%s) mutated to %q, want %q", moduleIdentity, got, arch.ModuleTierCore)
	}
}


func TestTieredModulesLiveUnderTieredNamespaces(t *testing.T) {
	cases := map[string]string{
		"/home/mrafter/nas-os/internal/lab/brandinsight":    "brandinsight should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/releasemanager":  "releasemanager should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/aimediatag":      "aimediatag should live under internal/lab",
		"/home/mrafter/nas-os/internal/extensions/activeprotect": "activeprotect should live under internal/extensions",
	}
	for path, msg := range cases {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s: %v", msg, err)
		}
	}
}
