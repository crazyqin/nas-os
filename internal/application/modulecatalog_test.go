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
		"benchmarkpro":     arch.ModuleTierLab,
		"blockdedup2":      arch.ModuleTierLab,
		"brandinsight":     arch.ModuleTierLab,
		"cinemarec":        arch.ModuleTierLab,
		"cloudsync2":       arch.ModuleTierLab,
		"compliancescan":   arch.ModuleTierExtension,
		"datasovereignty2": arch.ModuleTierLab,
		"deployorch":       arch.ModuleTierExtension,
		"filetimemachine2": arch.ModuleTierLab,
		"netdiag":          arch.ModuleTierExtension,
		"resmonpro":        arch.ModuleTierLab,
		"snapviz":          arch.ModuleTierLab,
		"smartthermal2":    arch.ModuleTierLab,
		"tcodash":          arch.ModuleTierLab,
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
		"/home/mrafter/nas-os/internal/lab/benchmarkpro":          "benchmarkpro should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/blockdedup2":           "blockdedup2 should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/brandinsight":          "brandinsight should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/cloudsync2":            "cloudsync2 should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/cinemarec":             "cinemarec should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/releasemanager":        "releasemanager should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/resmonpro":             "resmonpro should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/aimediatag":            "aimediatag should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/costbenchmark":         "costbenchmark should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/datasovereignty2":      "datasovereignty2 should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/draid2":                "draid2 should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/featurematrix":         "featurematrix should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/filetimemachine2":      "filetimemachine2 should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/forensics2":            "forensics2 should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/netshield":             "netshield should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/posterwallpro":         "posterwallpro should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/safeaccess":            "safeaccess should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/snapviz":               "snapviz should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/smartthermal2":         "smartthermal2 should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/storagecostpredict":    "storagecostpredict should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/tcodash":               "tcodash should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/truecloudbk":           "truecloudbk should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/updatedirector":        "updatedirector should live under internal/lab",
		"/home/mrafter/nas-os/internal/extensions/activeprotect":  "activeprotect should live under internal/extensions",
		"/home/mrafter/nas-os/internal/extensions/agentworkflow":  "agentworkflow should live under internal/extensions",
		"/home/mrafter/nas-os/internal/extensions/aiguardrails":   "aiguardrails should live under internal/extensions",
		"/home/mrafter/nas-os/internal/extensions/compliancescan": "compliancescan should live under internal/extensions",
		"/home/mrafter/nas-os/internal/extensions/deployorch":     "deployorch should live under internal/extensions",
		"/home/mrafter/nas-os/internal/extensions/netdiag":        "netdiag should live under internal/extensions",
		"/home/mrafter/nas-os/internal/extensions/voicehub":       "voicehub should live under internal/extensions",
	}
	for path, msg := range cases {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s: %v", msg, err)
		}
	}
}

func TestDeprecatedTopLevelTieredModulePathsAreGone(t *testing.T) {
	paths := map[string]string{
		"/home/mrafter/nas-os/internal/activeprotect":      "activeprotect should not remain at internal top level",
		"/home/mrafter/nas-os/internal/agentworkflow":      "agentworkflow should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aiguardrails":       "aiguardrails should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aimediatag":         "aimediatag should not remain at internal top level",
		"/home/mrafter/nas-os/internal/benchmarkpro":       "benchmarkpro should not remain at internal top level",
		"/home/mrafter/nas-os/internal/blockdedup2":        "blockdedup2 should not remain at internal top level",
		"/home/mrafter/nas-os/internal/brandinsight":       "brandinsight should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cloudsync2":         "cloudsync2 should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cinemarec":          "cinemarec should not remain at internal top level",
		"/home/mrafter/nas-os/internal/compliancescan":     "compliancescan should not remain at internal top level",
		"/home/mrafter/nas-os/internal/costbenchmark":      "costbenchmark should not remain at internal top level",
		"/home/mrafter/nas-os/internal/datasovereignty2":   "datasovereignty2 should not remain at internal top level",
		"/home/mrafter/nas-os/internal/draid2":             "draid2 should not remain at internal top level",
		"/home/mrafter/nas-os/internal/featurematrix":      "featurematrix should not remain at internal top level",
		"/home/mrafter/nas-os/internal/filetimemachine2":   "filetimemachine2 should not remain at internal top level",
		"/home/mrafter/nas-os/internal/forensics2":         "forensics2 should not remain at internal top level",
		"/home/mrafter/nas-os/internal/deployorch":         "deployorch should not remain at internal top level",
		"/home/mrafter/nas-os/internal/netdiag":            "netdiag should not remain at internal top level",
		"/home/mrafter/nas-os/internal/netshield":          "netshield should not remain at internal top level",
		"/home/mrafter/nas-os/internal/posterwallpro":      "posterwallpro should not remain at internal top level",
		"/home/mrafter/nas-os/internal/releasemanager":     "releasemanager should not remain at internal top level",
		"/home/mrafter/nas-os/internal/resmonpro":          "resmonpro should not remain at internal top level",
		"/home/mrafter/nas-os/internal/safeaccess":         "safeaccess should not remain at internal top level",
		"/home/mrafter/nas-os/internal/snapviz":            "snapviz should not remain at internal top level",
		"/home/mrafter/nas-os/internal/smartthermal2":      "smartthermal2 should not remain at internal top level",
		"/home/mrafter/nas-os/internal/storagecostpredict": "storagecostpredict should not remain at internal top level",
		"/home/mrafter/nas-os/internal/tcodash":            "tcodash should not remain at internal top level",
		"/home/mrafter/nas-os/internal/truecloudbk":        "truecloudbk should not remain at internal top level",
		"/home/mrafter/nas-os/internal/updatedirector":     "updatedirector should not remain at internal top level",
		"/home/mrafter/nas-os/internal/voicehub":           "voicehub should not remain at internal top level",
	}
	for path, msg := range paths {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			if err == nil {
				t.Fatal(msg)
			}
			t.Fatalf("%s: unexpected stat error: %v", msg, err)
		}
	}
}
