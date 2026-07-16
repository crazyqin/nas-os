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
		"activeprotect":     arch.ModuleTierExtension,
		"agentworkflow":     arch.ModuleTierExtension,
		"benchmarkpro":      arch.ModuleTierLab,
		"blockdedup2":       arch.ModuleTierLab,
		"brandinsight":      arch.ModuleTierLab,
		"cinemarec":         arch.ModuleTierLab,
		"cloudsync2":        arch.ModuleTierLab,
		"containerpro":      arch.ModuleTierLab,
		"compliancescan":    arch.ModuleTierExtension,
		"datasovereignty2":  arch.ModuleTierLab,
		"deployorch":        arch.ModuleTierExtension,
		"digitalassetvault": arch.ModuleTierLab,
		"familyactivityhub": arch.ModuleTierLab,
		"filetimemachine2":  arch.ModuleTierLab,
		"guidedalert":       arch.ModuleTierLab,
		"guidedalerts":      arch.ModuleTierLab,
		"iotedgegateway":    arch.ModuleTierLab,
		"netdiag":           arch.ModuleTierExtension,
		"resmonpro":         arch.ModuleTierLab,
		"snapviz":           arch.ModuleTierLab,
		"smartalert":        arch.ModuleTierLab,
		"smarthomehubpro":   arch.ModuleTierLab,
		"smartthermal2":     arch.ModuleTierLab,
		"tcodash":           arch.ModuleTierLab,
		"themepro":          arch.ModuleTierLab,
		"updatedirector":    arch.ModuleTierLab,
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
		"/home/mrafter/nas-os/internal/lab/containerpro":          "containerpro should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/cinemarec":             "cinemarec should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/releasemanager":        "releasemanager should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/resmonpro":             "resmonpro should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/aimediatag":            "aimediatag should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/costbenchmark":         "costbenchmark should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/datasovereignty2":      "datasovereignty2 should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/digitalassetvault":     "digitalassetvault should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/draid2":                "draid2 should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/featurematrix":         "featurematrix should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/familyactivityhub":     "familyactivityhub should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/filetimemachine2":      "filetimemachine2 should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/forensics2":            "forensics2 should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/iotedgegateway":        "iotedgegateway should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/netshield":             "netshield should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/posterwallpro":         "posterwallpro should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/safeaccess":            "safeaccess should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/snapviz":               "snapviz should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/smartthermal2":         "smartthermal2 should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/smarthomehubpro":       "smarthomehubpro should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/storagecostpredict":    "storagecostpredict should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/tcodash":               "tcodash should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/themepro":              "themepro should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/truecloudbk":           "truecloudbk should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/updatedirector":        "updatedirector should live under internal/lab",
		"/home/mrafter/nas-os/internal/extensions/activeprotect":  "activeprotect should live under internal/extensions",
		"/home/mrafter/nas-os/internal/extensions/agentworkflow":  "agentworkflow should live under internal/extensions",
		"/home/mrafter/nas-os/internal/extensions/aiguardrails":   "aiguardrails should live under internal/extensions",
		"/home/mrafter/nas-os/internal/extensions/compliancescan": "compliancescan should live under internal/extensions",
		"/home/mrafter/nas-os/internal/extensions/deployorch":     "deployorch should live under internal/extensions",
		"/home/mrafter/nas-os/internal/extensions/netdiag":        "netdiag should live under internal/extensions",
		"/home/mrafter/nas-os/internal/extensions/voicehub":       "voicehub should live under internal/extensions",
		"/home/mrafter/nas-os/internal/lab/aiagentorch":             "aiagentorch should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/aianalyzer":             "aianalyzer should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/aiassistant":             "aiassistant should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/aichatbot":             "aichatbot should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/aicodeassist":             "aicodeassist should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/aiconsole":             "aiconsole should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/aiconttag":             "aiconttag should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/aidatadedup":             "aidatadedup should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/aidatamasking":             "aidatamasking should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/aidefrag":             "aidefrag should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/aideidentification":             "aideidentification should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/aifilearchive":             "aifilearchive should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/ailoganalyzer":             "ailoganalyzer should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/aiops":             "aiops should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/aiplatform":             "aiplatform should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/airplay":             "airplay should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/aisysadmin":             "aisysadmin should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/aitaskagent":             "aitaskagent should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/aitokenmeter":             "aitokenmeter should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/aitraffic":             "aitraffic should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/aitranscription":             "aitranscription should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/aivideounderstand":             "aivideounderstand should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/aiworkflow":             "aiworkflow should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/aiwriter":             "aiwriter should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/billing":             "billing should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/budget":             "budget should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/cloudbackup":             "cloudbackup should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/cloudbackupsync":             "cloudbackupsync should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/cloudbillfc":             "cloudbillfc should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/cloudbilling":             "cloudbilling should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/cloudconnect":             "cloudconnect should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/cloudfederation":             "cloudfederation should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/cloudfuse":             "cloudfuse should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/cloudmount":             "cloudmount should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/cloudportal":             "cloudportal should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/cloudsyncmgr":             "cloudsyncmgr should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/cloudui":             "cloudui should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/cost":             "cost should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/costanalyzer":             "costanalyzer should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/costdashboard":             "costdashboard should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/costforecast":             "costforecast should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/costgovernance":             "costgovernance should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/costpredict":             "costpredict should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/quantumcrypto":             "quantumcrypto should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/quantumsafe":             "quantumsafe should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/quantumsafevault":             "quantumsafevault should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/quantumsecurecomm":             "quantumsecurecomm should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/tcocalc":             "tcocalc should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/tcodashboard":             "tcodashboard should live under internal/lab",
		"/home/mrafter/nas-os/internal/lab/tcoshield":             "tcoshield should live under internal/lab",
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
		"/home/mrafter/nas-os/internal/containerpro":       "containerpro should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cinemarec":          "cinemarec should not remain at internal top level",
		"/home/mrafter/nas-os/internal/compliancescan":     "compliancescan should not remain at internal top level",
		"/home/mrafter/nas-os/internal/costbenchmark":      "costbenchmark should not remain at internal top level",
		"/home/mrafter/nas-os/internal/datasovereignty2":   "datasovereignty2 should not remain at internal top level",
		"/home/mrafter/nas-os/internal/digitalassetvault":  "digitalassetvault should not remain at internal top level",
		"/home/mrafter/nas-os/internal/draid2":             "draid2 should not remain at internal top level",
		"/home/mrafter/nas-os/internal/featurematrix":      "featurematrix should not remain at internal top level",
		"/home/mrafter/nas-os/internal/familyactivityhub":  "familyactivityhub should not remain at internal top level",
		"/home/mrafter/nas-os/internal/filetimemachine2":   "filetimemachine2 should not remain at internal top level",
		"/home/mrafter/nas-os/internal/forensics2":         "forensics2 should not remain at internal top level",
		"/home/mrafter/nas-os/internal/guidedalert":        "guidedalert should not remain at internal top level",
		"/home/mrafter/nas-os/internal/guidedalerts":       "guidedalerts should not remain at internal top level",
		"/home/mrafter/nas-os/internal/iotedgegateway":     "iotedgegateway should not remain at internal top level",
		"/home/mrafter/nas-os/internal/deployorch":         "deployorch should not remain at internal top level",
		"/home/mrafter/nas-os/internal/netdiag":            "netdiag should not remain at internal top level",
		"/home/mrafter/nas-os/internal/netshield":          "netshield should not remain at internal top level",
		"/home/mrafter/nas-os/internal/posterwallpro":      "posterwallpro should not remain at internal top level",
		"/home/mrafter/nas-os/internal/releasemanager":     "releasemanager should not remain at internal top level",
		"/home/mrafter/nas-os/internal/resmonpro":          "resmonpro should not remain at internal top level",
		"/home/mrafter/nas-os/internal/safeaccess":         "safeaccess should not remain at internal top level",
		"/home/mrafter/nas-os/internal/snapviz":            "snapviz should not remain at internal top level",
		"/home/mrafter/nas-os/internal/smartalert":         "smartalert should not remain at internal top level",
		"/home/mrafter/nas-os/internal/smartthermal2":      "smartthermal2 should not remain at internal top level",
		"/home/mrafter/nas-os/internal/smarthomehubpro":    "smarthomehubpro should not remain at internal top level",
		"/home/mrafter/nas-os/internal/storagecostpredict": "storagecostpredict should not remain at internal top level",
		"/home/mrafter/nas-os/internal/tcodash":            "tcodash should not remain at internal top level",
		"/home/mrafter/nas-os/internal/themepro":           "themepro should not remain at internal top level",
		"/home/mrafter/nas-os/internal/truecloudbk":        "truecloudbk should not remain at internal top level",
		"/home/mrafter/nas-os/internal/updatedirector":     "updatedirector should not remain at internal top level",
		"/home/mrafter/nas-os/internal/voicehub":           "voicehub should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aiagentorch":   "aiagentorch should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aianalyzer":   "aianalyzer should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aiassistant":   "aiassistant should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aichatbot":   "aichatbot should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aicodeassist":   "aicodeassist should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aiconsole":   "aiconsole should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aiconttag":   "aiconttag should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aidatadedup":   "aidatadedup should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aidatamasking":   "aidatamasking should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aidefrag":   "aidefrag should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aideidentification":   "aideidentification should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aifilearchive":   "aifilearchive should not remain at internal top level",
		"/home/mrafter/nas-os/internal/ailoganalyzer":   "ailoganalyzer should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aiops":   "aiops should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aiplatform":   "aiplatform should not remain at internal top level",
		"/home/mrafter/nas-os/internal/airplay":   "airplay should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aisysadmin":   "aisysadmin should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aitaskagent":   "aitaskagent should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aitokenmeter":   "aitokenmeter should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aitraffic":   "aitraffic should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aitranscription":   "aitranscription should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aivideounderstand":   "aivideounderstand should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aiworkflow":   "aiworkflow should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aiwriter":   "aiwriter should not remain at internal top level",
		"/home/mrafter/nas-os/internal/billing":   "billing should not remain at internal top level",
		"/home/mrafter/nas-os/internal/budget":   "budget should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cloudbackup":   "cloudbackup should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cloudbackupsync":   "cloudbackupsync should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cloudbillfc":   "cloudbillfc should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cloudbilling":   "cloudbilling should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cloudconnect":   "cloudconnect should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cloudfederation":   "cloudfederation should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cloudfuse":   "cloudfuse should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cloudmount":   "cloudmount should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cloudportal":   "cloudportal should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cloudsyncmgr":   "cloudsyncmgr should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cloudui":   "cloudui should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cost":   "cost should not remain at internal top level",
		"/home/mrafter/nas-os/internal/costanalyzer":   "costanalyzer should not remain at internal top level",
		"/home/mrafter/nas-os/internal/costdashboard":   "costdashboard should not remain at internal top level",
		"/home/mrafter/nas-os/internal/costforecast":   "costforecast should not remain at internal top level",
		"/home/mrafter/nas-os/internal/costgovernance":   "costgovernance should not remain at internal top level",
		"/home/mrafter/nas-os/internal/costpredict":   "costpredict should not remain at internal top level",
		"/home/mrafter/nas-os/internal/quantumcrypto":   "quantumcrypto should not remain at internal top level",
		"/home/mrafter/nas-os/internal/quantumsafe":   "quantumsafe should not remain at internal top level",
		"/home/mrafter/nas-os/internal/quantumsafevault":   "quantumsafevault should not remain at internal top level",
		"/home/mrafter/nas-os/internal/quantumsecurecomm":   "quantumsecurecomm should not remain at internal top level",
		"/home/mrafter/nas-os/internal/tcocalc":   "tcocalc should not remain at internal top level",
		"/home/mrafter/nas-os/internal/tcodashboard":   "tcodashboard should not remain at internal top level",
		"/home/mrafter/nas-os/internal/tcoshield":   "tcoshield should not remain at internal top level",
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
