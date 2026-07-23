package application

import (
	"path/filepath"
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
		"acme":              arch.ModuleTierLab,
		"activebackup":      arch.ModuleTierLab,
		"alerting":          arch.ModuleTierLab,
		"selfheal":          arch.ModuleTierLab,
		"ztna":              arch.ModuleTierLab,
		"reports":           arch.ModuleTierLab,
		"smartpricing":      arch.ModuleTierLab,
		"filemanager":       arch.ModuleTierLab,
		"filetimemachine":   arch.ModuleTierLab,
		"storagetiering":    arch.ModuleTierLab,
		"nvmeof":            arch.ModuleTierLab,
		"hybridpool":        arch.ModuleTierLab,
		"connect":           arch.ModuleTierLab,
		"firewall":          arch.ModuleTierLab,
		"ldap":              arch.ModuleTierLab,
		"fido2":             arch.ModuleTierLab,
		"media":             arch.ModuleTierLab,
		"downloadstation":   arch.ModuleTierLab,
		"collaboration":     arch.ModuleTierLab,
		"automation":        arch.ModuleTierLab,
		"dockercompose":     arch.ModuleTierLab,
		"appstore":          arch.ModuleTierLab,
		"ssdcache":          arch.ModuleTierLab,
		"nasdiscovery":      arch.ModuleTierLab,
	}
	for name, want := range cases {
		if got := catalog[name]; got != want {
			t.Fatalf("catalog[%s] = %q, want %q", name, got, want)
		}
	}
}

func TestModuleTierForDefaultsUnknownModulesToLab(t *testing.T) {
	if got := ModuleTierFor("totally-new-module"); got != arch.ModuleTierLab {
		t.Fatalf("ModuleTierFor(unknown) = %q, want %q (unknown must not look like product Extension)", got, arch.ModuleTierLab)
	}
}

func TestModuleCatalogSnapshotReturnsCopy(t *testing.T) {
	catalog := ModuleCatalogSnapshot()
	catalog[moduleIdentity] = arch.ModuleTierLab
	if got := ModuleTierFor(moduleIdentity); got != arch.ModuleTierCore {
		t.Fatalf("ModuleTierFor(%s) mutated to %q, want %q", moduleIdentity, got, arch.ModuleTierCore)
	}
}



// labRoot returns the on-disk lab tree under internal/lab (in-repo greenhouse).
// Empty only if the tree is missing (should not happen in a full checkout).
func labRoot(t *testing.T) string {
	t.Helper()
	root := repoRoot(t)
	lab := filepath.Join(root, "internal", "lab")
	// Nested module / real packages present
	if st, err := os.Stat(lab); err == nil && st.IsDir() {
		if entries, err := os.ReadDir(lab); err == nil && len(entries) > 3 {
			return lab
		}
	}
	return ""
}


func TestTieredModulesLiveUnderTieredNamespaces(t *testing.T) {
	lab := labRoot(t)
	if lab == "" {
		t.Skip("internal/lab tree missing in this checkout")
	}
	// Sample of demoted packages that must live under the lab tree (not internal top-level).
	names := []string{
		"benchmarkpro",
		"blockdedup2",
		"brandinsight",
		"cloudsync2",
		"containerpro",
		"cinemarec",
		"releasemanager",
		"resmonpro",
		"aimediatag",
		"costbenchmark",
		"datasovereignty2",
		"digitalassetvault",
		"draid2",
		"featurematrix",
		"familyactivityhub",
		"filetimemachine2",
		"forensics2",
		"iotedgegateway",
		"netshield",
		"posterwallpro",
		"safeaccess",
		"snapviz",
		"smartthermal2",
		"smarthomehubpro",
		"storagecostpredict",
		"tcodash",
		"themepro",
		"truecloudbk",
		"updatedirector",
		"aiagentorch",
		"aianalyzer",
		"aiassistant",
		"aichatbot",
		"aicodeassist",
		"aiconsole",
		"aiconttag",
		"aidatadedup",
		"aidatamasking",
		"aidefrag",
		"aideidentification",
	}
	for _, name := range names {
		path := filepath.Join(lab, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s should live under lab tree %s: %v", name, lab, err)
		}
		// Must not reappear at internal top level
		top := filepath.Join(repoRoot(t), "internal", name)
		if st, err := os.Stat(top); err == nil && st.IsDir() {
			t.Fatalf("%s must not reappear at internal top level", name)
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
		"/home/mrafter/nas-os/internal/aiagentorch":        "aiagentorch should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aianalyzer":         "aianalyzer should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aiassistant":        "aiassistant should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aichatbot":          "aichatbot should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aicodeassist":       "aicodeassist should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aiconsole":          "aiconsole should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aiconttag":          "aiconttag should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aidatadedup":        "aidatadedup should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aidatamasking":      "aidatamasking should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aidefrag":           "aidefrag should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aideidentification": "aideidentification should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aifilearchive":      "aifilearchive should not remain at internal top level",
		"/home/mrafter/nas-os/internal/ailoganalyzer":      "ailoganalyzer should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aiops":              "aiops should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aiplatform":         "aiplatform should not remain at internal top level",
		"/home/mrafter/nas-os/internal/airplay":            "airplay should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aisysadmin":         "aisysadmin should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aitaskagent":        "aitaskagent should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aitokenmeter":       "aitokenmeter should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aitraffic":          "aitraffic should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aitranscription":    "aitranscription should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aivideounderstand":  "aivideounderstand should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aiworkflow":         "aiworkflow should not remain at internal top level",
		"/home/mrafter/nas-os/internal/aiwriter":           "aiwriter should not remain at internal top level",
		"/home/mrafter/nas-os/internal/billing":            "billing should not remain at internal top level",
		"/home/mrafter/nas-os/internal/budget":             "budget should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cloudbackup":        "cloudbackup should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cloudbackupsync":    "cloudbackupsync should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cloudbillfc":        "cloudbillfc should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cloudbilling":       "cloudbilling should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cloudconnect":       "cloudconnect should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cloudfederation":    "cloudfederation should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cloudfuse":          "cloudfuse should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cloudmount":         "cloudmount should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cloudportal":        "cloudportal should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cloudsyncmgr":       "cloudsyncmgr should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cloudui":            "cloudui should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cost":               "cost should not remain at internal top level",
		"/home/mrafter/nas-os/internal/costanalyzer":       "costanalyzer should not remain at internal top level",
		"/home/mrafter/nas-os/internal/costdashboard":      "costdashboard should not remain at internal top level",
		"/home/mrafter/nas-os/internal/costforecast":       "costforecast should not remain at internal top level",
		"/home/mrafter/nas-os/internal/costgovernance":     "costgovernance should not remain at internal top level",
		"/home/mrafter/nas-os/internal/costpredict":        "costpredict should not remain at internal top level",
		"/home/mrafter/nas-os/internal/quantumcrypto":      "quantumcrypto should not remain at internal top level",
		"/home/mrafter/nas-os/internal/quantumsafe":        "quantumsafe should not remain at internal top level",
		"/home/mrafter/nas-os/internal/quantumsafevault":   "quantumsafevault should not remain at internal top level",
		"/home/mrafter/nas-os/internal/quantumsecurecomm":  "quantumsecurecomm should not remain at internal top level",
		"/home/mrafter/nas-os/internal/tcocalc":            "tcocalc should not remain at internal top level",
		"/home/mrafter/nas-os/internal/tcodashboard":       "tcodashboard should not remain at internal top level",
		"/home/mrafter/nas-os/internal/tcoshield":          "tcoshield should not remain at internal top level",
		// v3.22.0 demotion wave: old top-level paths must stay gone
		"/home/mrafter/nas-os/internal/acme":                "acme should not remain at internal top level",
		"/home/mrafter/nas-os/internal/acmemanager":         "acmemanager should not remain at internal top level",
		"/home/mrafter/nas-os/internal/activedirectory":     "activedirectory should not remain at internal top level",
		"/home/mrafter/nas-os/internal/activeinsight":       "activeinsight should not remain at internal top level",
		"/home/mrafter/nas-os/internal/activityfeed":        "activityfeed should not remain at internal top level",
		"/home/mrafter/nas-os/internal/adaptivetwofa":       "adaptivetwofa should not remain at internal top level",
		"/home/mrafter/nas-os/internal/adminprivilege":      "adminprivilege should not remain at internal top level",
		"/home/mrafter/nas-os/internal/album":               "album should not remain at internal top level",
		"/home/mrafter/nas-os/internal/alerting":            "alerting should not remain at internal top level",
		"/home/mrafter/nas-os/internal/analytics":           "analytics should not remain at internal top level",
		"/home/mrafter/nas-os/internal/antivirus":           "antivirus should not remain at internal top level",
		"/home/mrafter/nas-os/internal/apilevelmeter":       "apilevelmeter should not remain at internal top level",
		"/home/mrafter/nas-os/internal/apimigrator":         "apimigrator should not remain at internal top level",
		"/home/mrafter/nas-os/internal/apiproxy":            "apiproxy should not remain at internal top level",
		"/home/mrafter/nas-os/internal/apisignverify":       "apisignverify should not remain at internal top level",
		"/home/mrafter/nas-os/internal/app":                 "app should not remain at internal top level",
		"/home/mrafter/nas-os/internal/appreview":           "appreview should not remain at internal top level",
		"/home/mrafter/nas-os/internal/appstore":            "appstore should not remain at internal top level",
		"/home/mrafter/nas-os/internal/armadapter":          "armadapter should not remain at internal top level",
		"/home/mrafter/nas-os/internal/assetmgr":            "assetmgr should not remain at internal top level",
		"/home/mrafter/nas-os/internal/assetvaluator":       "assetvaluator should not remain at internal top level",
		"/home/mrafter/nas-os/internal/audit":               "audit should not remain at internal top level",
		"/home/mrafter/nas-os/internal/automation":          "automation should not remain at internal top level",
		"/home/mrafter/nas-os/internal/batchrename":         "batchrename should not remain at internal top level",
		"/home/mrafter/nas-os/internal/bitrotheal":          "bitrotheal should not remain at internal top level",
		"/home/mrafter/nas-os/internal/bluetoothprovision":  "bluetoothprovision should not remain at internal top level",
		"/home/mrafter/nas-os/internal/bootrepair":          "bootrepair should not remain at internal top level",
		"/home/mrafter/nas-os/internal/branding":            "branding should not remain at internal top level",
		"/home/mrafter/nas-os/internal/brtprefetch":         "brtprefetch should not remain at internal top level",
		"/home/mrafter/nas-os/internal/btrfs":               "btrfs should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cachewarm":           "cachewarm should not remain at internal top level",
		"/home/mrafter/nas-os/internal/calendar":            "calendar should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cdp":                 "cdp should not remain at internal top level",
		"/home/mrafter/nas-os/internal/chat":                "chat should not remain at internal top level",
		"/home/mrafter/nas-os/internal/clientthumb":         "clientthumb should not remain at internal top level",
		"/home/mrafter/nas-os/internal/clipboard":           "clipboard should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cms":                 "cms should not remain at internal top level",
		"/home/mrafter/nas-os/internal/collaboration":       "collaboration should not remain at internal top level",
		"/home/mrafter/nas-os/internal/complreport":         "complreport should not remain at internal top level",
		"/home/mrafter/nas-os/internal/composeinclude":      "composeinclude should not remain at internal top level",
		"/home/mrafter/nas-os/internal/composevisual":       "composevisual should not remain at internal top level",
		"/home/mrafter/nas-os/internal/configdrift":         "configdrift should not remain at internal top level",
		"/home/mrafter/nas-os/internal/connect":             "connect should not remain at internal top level",
		"/home/mrafter/nas-os/internal/consensus":           "consensus should not remain at internal top level",
		"/home/mrafter/nas-os/internal/contacts":            "contacts should not remain at internal top level",
		"/home/mrafter/nas-os/internal/containwatch":        "containwatch should not remain at internal top level",
		"/home/mrafter/nas-os/internal/contentai":           "contentai should not remain at internal top level",
		"/home/mrafter/nas-os/internal/contentseo":          "contentseo should not remain at internal top level",
		"/home/mrafter/nas-os/internal/contentworkflow":     "contentworkflow should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cron":                "cron should not remain at internal top level",
		"/home/mrafter/nas-os/internal/crossdevclipboard":   "crossdevclipboard should not remain at internal top level",
		"/home/mrafter/nas-os/internal/crossnasreplication": "crossnasreplication should not remain at internal top level",
		"/home/mrafter/nas-os/internal/crossplatformsync":   "crossplatformsync should not remain at internal top level",
		"/home/mrafter/nas-os/internal/customdash":          "customdash should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cxlmempool":          "cxlmempool should not remain at internal top level",
		"/home/mrafter/nas-os/internal/cyberposture":        "cyberposture should not remain at internal top level",
		"/home/mrafter/nas-os/internal/dam":                 "dam should not remain at internal top level",
		"/home/mrafter/nas-os/internal/datagovernance":      "datagovernance should not remain at internal top level",
		"/home/mrafter/nas-os/internal/datamigration":       "datamigration should not remain at internal top level",
		"/home/mrafter/nas-os/internal/datasettier":         "datasettier should not remain at internal top level",
		"/home/mrafter/nas-os/internal/dedupadvisor":        "dedupadvisor should not remain at internal top level",
		"/home/mrafter/nas-os/internal/desktopmanager":      "desktopmanager should not remain at internal top level",
		"/home/mrafter/nas-os/internal/diagnostics":         "diagnostics should not remain at internal top level",
		"/home/mrafter/nas-os/internal/digitalsignage":      "digitalsignage should not remain at internal top level",
		"/home/mrafter/nas-os/internal/digitaltwin":         "digitaltwin should not remain at internal top level",
		"/home/mrafter/nas-os/internal/directplay":          "directplay should not remain at internal top level",
		"/home/mrafter/nas-os/internal/diskhealth":          "diskhealth should not remain at internal top level",
		"/home/mrafter/nas-os/internal/distScheduler":       "distScheduler should not remain at internal top level",
		"/home/mrafter/nas-os/internal/distributedtask":     "distributedtask should not remain at internal top level",
		"/home/mrafter/nas-os/internal/dlpengine":           "dlpengine should not remain at internal top level",
		"/home/mrafter/nas-os/internal/dockercompose":       "dockercompose should not remain at internal top level",
		"/home/mrafter/nas-os/internal/dockermanager":       "dockermanager should not remain at internal top level",
		"/home/mrafter/nas-os/internal/docmanager":          "docmanager should not remain at internal top level",
		"/home/mrafter/nas-os/internal/docprocessor":        "docprocessor should not remain at internal top level",
		"/home/mrafter/nas-os/internal/docworkspace":        "docworkspace should not remain at internal top level",
		"/home/mrafter/nas-os/internal/domainsync":          "domainsync should not remain at internal top level",
		"/home/mrafter/nas-os/internal/downloadstation":     "downloadstation should not remain at internal top level",
		"/home/mrafter/nas-os/internal/dpdkaccel":           "dpdkaccel should not remain at internal top level",
		"/home/mrafter/nas-os/internal/drive":               "drive should not remain at internal top level",
		"/home/mrafter/nas-os/internal/driveinsight":        "driveinsight should not remain at internal top level",
		"/home/mrafter/nas-os/internal/drivemigration":      "drivemigration should not remain at internal top level",
		"/home/mrafter/nas-os/internal/edgegateway":         "edgegateway should not remain at internal top level",
		"/home/mrafter/nas-os/internal/enclosure":           "enclosure should not remain at internal top level",
		"/home/mrafter/nas-os/internal/encryption":          "encryption should not remain at internal top level",
		"/home/mrafter/nas-os/internal/esignature":          "esignature should not remain at internal top level",
		"/home/mrafter/nas-os/internal/fancurve":            "fancurve should not remain at internal top level",
		"/home/mrafter/nas-os/internal/fastdedup":           "fastdedup should not remain at internal top level",
		"/home/mrafter/nas-os/internal/fecconfig":           "fecconfig should not remain at internal top level",
		"/home/mrafter/nas-os/internal/federatednas":        "federatednas should not remain at internal top level",
		"/home/mrafter/nas-os/internal/fido2":               "fido2 should not remain at internal top level",
		"/home/mrafter/nas-os/internal/fileactivity":        "fileactivity should not remain at internal top level",
		"/home/mrafter/nas-os/internal/fileactivitywatcher": "fileactivitywatcher should not remain at internal top level",
		"/home/mrafter/nas-os/internal/filecache":           "filecache should not remain at internal top level",
		"/home/mrafter/nas-os/internal/filededup":           "filededup should not remain at internal top level",
		"/home/mrafter/nas-os/internal/fileinsights":        "fileinsights should not remain at internal top level",
		"/home/mrafter/nas-os/internal/fileintegrity":       "fileintegrity should not remain at internal top level",
		"/home/mrafter/nas-os/internal/filelifecycle":       "filelifecycle should not remain at internal top level",
		"/home/mrafter/nas-os/internal/filelock":            "filelock should not remain at internal top level",
		"/home/mrafter/nas-os/internal/filemanager":         "filemanager should not remain at internal top level",
		"/home/mrafter/nas-os/internal/filepreview":         "filepreview should not remain at internal top level",
		"/home/mrafter/nas-os/internal/filerequest":         "filerequest should not remain at internal top level",
		"/home/mrafter/nas-os/internal/filesharing":         "filesharing should not remain at internal top level",
		"/home/mrafter/nas-os/internal/filesyncclient":      "filesyncclient should not remain at internal top level",
		"/home/mrafter/nas-os/internal/filesynchub":         "filesynchub should not remain at internal top level",
		"/home/mrafter/nas-os/internal/filetagger":          "filetagger should not remain at internal top level",
		"/home/mrafter/nas-os/internal/filetimeline":        "filetimeline should not remain at internal top level",
		"/home/mrafter/nas-os/internal/fileversion":         "fileversion should not remain at internal top level",
		"/home/mrafter/nas-os/internal/filewatcher":         "filewatcher should not remain at internal top level",
		"/home/mrafter/nas-os/internal/fips":                "fips should not remain at internal top level",
		"/home/mrafter/nas-os/internal/fipsvault":           "fipsvault should not remain at internal top level",
		"/home/mrafter/nas-os/internal/firewall":            "firewall should not remain at internal top level",
		"/home/mrafter/nas-os/internal/freeipa":             "freeipa should not remain at internal top level",
		"/home/mrafter/nas-os/internal/fworchestrator":      "fworchestrator should not remain at internal top level",
		"/home/mrafter/nas-os/internal/gateway":             "gateway should not remain at internal top level",
		"/home/mrafter/nas-os/internal/gdprscanner":         "gdprscanner should not remain at internal top level",
		"/home/mrafter/nas-os/internal/geoipfirewall":       "geoipfirewall should not remain at internal top level",
		"/home/mrafter/nas-os/internal/hdddedup":            "hdddedup should not remain at internal top level",
		"/home/mrafter/nas-os/internal/hwcompat":            "hwcompat should not remain at internal top level",
		"/home/mrafter/nas-os/internal/hybridpool":          "hybridpool should not remain at internal top level",
		"/home/mrafter/nas-os/internal/hybridpoolmgr":       "hybridpoolmgr should not remain at internal top level",
		"/home/mrafter/nas-os/internal/immusnap":            "immusnap should not remain at internal top level",
		"/home/mrafter/nas-os/internal/inlinededup":         "inlinededup should not remain at internal top level",
		"/home/mrafter/nas-os/internal/integrityverifier":   "integrityverifier should not remain at internal top level",
		"/home/mrafter/nas-os/internal/ipv6net":             "ipv6net should not remain at internal top level",
		"/home/mrafter/nas-os/internal/iscsiblockclone":     "iscsiblockclone should not remain at internal top level",
		"/home/mrafter/nas-os/internal/iscsifc":             "iscsifc should not remain at internal top level",
		"/home/mrafter/nas-os/internal/iscsitgtoffload":     "iscsitgtoffload should not remain at internal top level",
		"/home/mrafter/nas-os/internal/jsonrpc":             "jsonrpc should not remain at internal top level",
		"/home/mrafter/nas-os/internal/kanban":              "kanban should not remain at internal top level",
		"/home/mrafter/nas-os/internal/kanbanboard":         "kanbanboard should not remain at internal top level",
		"/home/mrafter/nas-os/internal/kerberos":            "kerberos should not remain at internal top level",
		"/home/mrafter/nas-os/internal/kmip":                "kmip should not remain at internal top level",
		"/home/mrafter/nas-os/internal/kmipclient":          "kmipclient should not remain at internal top level",
		"/home/mrafter/nas-os/internal/knowledgebase":       "knowledgebase should not remain at internal top level",
		"/home/mrafter/nas-os/internal/knowledgemap":        "knowledgemap should not remain at internal top level",
		"/home/mrafter/nas-os/internal/ldap":                "ldap should not remain at internal top level",
		"/home/mrafter/nas-os/internal/linkaggbondadvisor":  "linkaggbondadvisor should not remain at internal top level",
		"/home/mrafter/nas-os/internal/loadbalancer":        "loadbalancer should not remain at internal top level",
		"/home/mrafter/nas-os/internal/localchat":           "localchat should not remain at internal top level",
		"/home/mrafter/nas-os/internal/localknowledgebase":  "localknowledgebase should not remain at internal top level",
		"/home/mrafter/nas-os/internal/mailserver":          "mailserver should not remain at internal top level",
		"/home/mrafter/nas-os/internal/media":               "media should not remain at internal top level",
		"/home/mrafter/nas-os/internal/mpiofc":              "mpiofc should not remain at internal top level",
		"/home/mrafter/nas-os/internal/nasdiscovery":        "nasdiscovery should not remain at internal top level",
		"/home/mrafter/nas-os/internal/netdatawidget":       "netdatawidget should not remain at internal top level",
		"/home/mrafter/nas-os/internal/notes":               "notes should not remain at internal top level",
		"/home/mrafter/nas-os/internal/nvmeof":              "nvmeof should not remain at internal top level",
		"/home/mrafter/nas-os/internal/nvmeofenhanced":      "nvmeofenhanced should not remain at internal top level",
		"/home/mrafter/nas-os/internal/securep2pshare":      "securep2pshare should not remain at internal top level",
		"/home/mrafter/nas-os/internal/selfheal":            "selfheal should not remain at internal top level",
		"/home/mrafter/nas-os/internal/sharelinks":          "sharelinks should not remain at internal top level",
		"/home/mrafter/nas-os/internal/ssdcache":            "ssdcache should not remain at internal top level",
		"/home/mrafter/nas-os/internal/storagemigration":    "storagemigration should not remain at internal top level",
		"/home/mrafter/nas-os/internal/storagetiering":      "storagetiering should not remain at internal top level",
		"/home/mrafter/nas-os/internal/teamworkspace":       "teamworkspace should not remain at internal top level",
		"/home/mrafter/nas-os/internal/trafficclassifier":   "trafficclassifier should not remain at internal top level",
		"/home/mrafter/nas-os/internal/usbreset":            "usbreset should not remain at internal top level",
		"/home/mrafter/nas-os/internal/ztna":                "ztna should not remain at internal top level",
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

func TestModuleCatalogCoreOnlyFive(t *testing.T) {
	catalog := ModuleCatalogSnapshot()
	var cores []string
	for name, tier := range catalog {
		if tier == arch.ModuleTierCore {
			cores = append(cores, name)
		}
	}
	want := map[string]bool{
		moduleIdentity: true,
		moduleStorage:  true,
		moduleNetwork:  true,
		moduleSharing:  true,
		moduleSystem:   true,
	}
	if len(cores) != 5 {
		t.Fatalf("core modules = %v (len %d), want exactly 5", cores, len(cores))
	}
	for _, name := range cores {
		if !want[name] {
			t.Fatalf("unexpected core module %q", name)
		}
	}
}
