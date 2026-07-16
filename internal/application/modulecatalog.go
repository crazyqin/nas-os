package application

import "nas-os/internal/arch"

var moduleCatalog = map[string]arch.ModuleTier{
	moduleIdentity: arch.ModuleTierCore,
	moduleStorage:  arch.ModuleTierCore,
	moduleNetwork:  arch.ModuleTierCore,
	moduleSharing:  arch.ModuleTierCore,
	moduleSystem:   arch.ModuleTierCore,

	"acl":            arch.ModuleTierExtension,
	"acme":           arch.ModuleTierExtension,
	"activebackup":   arch.ModuleTierExtension,
	"activeprotect":  arch.ModuleTierExtension,
	"agentworkflow":  arch.ModuleTierExtension,
	"ai":             arch.ModuleTierExtension,
	"aiguardrails":   arch.ModuleTierExtension,
	"alerting":       arch.ModuleTierExtension,
	"compliancescan": arch.ModuleTierExtension,
	"container":      arch.ModuleTierExtension,
	"deployorch":     arch.ModuleTierExtension,
	"download":       arch.ModuleTierExtension,
	"netdiag":        arch.ModuleTierExtension,
	"reports":        arch.ModuleTierExtension,
	"selfheal":       arch.ModuleTierExtension,
	"smartpricing":   arch.ModuleTierExtension,
	"voicehub":       arch.ModuleTierExtension,
	"ztna":           arch.ModuleTierExtension,

	"benchmarkpro":       arch.ModuleTierLab,
	"blockdedup2":        arch.ModuleTierLab,
	"brandinsight":       arch.ModuleTierLab,
	"cinemarec":          arch.ModuleTierLab,
	"cloudsync2":         arch.ModuleTierLab,
	"containerpro":       arch.ModuleTierLab,
	"costbenchmark":      arch.ModuleTierLab,
	"datasovereignty2":   arch.ModuleTierLab,
	"digitalassetvault":  arch.ModuleTierLab,
	"draid2":             arch.ModuleTierLab,
	"familyactivityhub":  arch.ModuleTierLab,
	"featurematrix":      arch.ModuleTierLab,
	"filetimemachine2":   arch.ModuleTierLab,
	"forensics2":         arch.ModuleTierLab,
	"guidedalert":        arch.ModuleTierLab,
	"guidedalerts":       arch.ModuleTierLab,
	"iotedgegateway":     arch.ModuleTierLab,
	"netshield":          arch.ModuleTierLab,
	"posterwallpro":      arch.ModuleTierLab,
	"releasemanager":     arch.ModuleTierLab,
	"resmonpro":          arch.ModuleTierLab,
	"safeaccess":         arch.ModuleTierLab,
	"smartalert":         arch.ModuleTierLab,
	"snapviz":            arch.ModuleTierLab,
	"smarthomehubpro":   arch.ModuleTierLab,
	"smartthermal2":      arch.ModuleTierLab,
	"storagecostpredict": arch.ModuleTierLab,
	"tcodash":            arch.ModuleTierLab,
	"themepro":           arch.ModuleTierLab,
	"truecloudbk":        arch.ModuleTierLab,
	"updatedirector":     arch.ModuleTierLab,

	// demoted: smart* pseudo-core → lab
	"smartrecipe":            arch.ModuleTierLab,
	"smartcam":            arch.ModuleTierLab,
	"smartpowerschedule":            arch.ModuleTierLab,
	"smartwearleveling":            arch.ModuleTierLab,
	"smartsleep":            arch.ModuleTierLab,
	"smartredundancy":            arch.ModuleTierLab,
	"smartlink":            arch.ModuleTierLab,
	"smartlifebackup":            arch.ModuleTierLab,
	"smartappcurator":            arch.ModuleTierLab,
	"smartresource":            arch.ModuleTierLab,
	"smartrecycle":            arch.ModuleTierLab,
	"smartrebuild":            arch.ModuleTierLab,
	"smartinsight":            arch.ModuleTierLab,
	"smartlifecycle":            arch.ModuleTierLab,

	// demoted: carbon/energy duplicates → lab
	"carbonaware":            arch.ModuleTierLab,
	"carbonfootprint":            arch.ModuleTierLab,
	"carbontracker":            arch.ModuleTierLab,
	"smartcarbon":            arch.ModuleTierLab,
	"energydashboard":            arch.ModuleTierLab,
	"energycost":            arch.ModuleTierLab,
	"energymanager":            arch.ModuleTierLab,

	// demoted: budget/finance duplicates → lab
	"budgetalert":            arch.ModuleTierLab,
	"budgetforecast":            arch.ModuleTierLab,
	"budgetmgr":            arch.ModuleTierLab,
	"budgetplan":            arch.ModuleTierLab,
	"smartbudget":            arch.ModuleTierLab,
	"familyfinance":            arch.ModuleTierLab,

	// demoted: quantum pseudo-core → lab
	"quantumcrypto":       arch.ModuleTierLab,
	"quantumsafe":         arch.ModuleTierLab,
	"quantumsafevault":   arch.ModuleTierLab,
	"quantumsecurecomm":  arch.ModuleTierLab,

	// demoted: duplicate AI modules → lab
	"aiagentorch":       arch.ModuleTierLab,
	"aianalyzer":       arch.ModuleTierLab,
	"aiassistant":      arch.ModuleTierLab,
	"aichatbot":        arch.ModuleTierLab,
	"aicodeassist":     arch.ModuleTierLab,
	"aiconsole":         arch.ModuleTierLab,
	"aiconttag":         arch.ModuleTierLab,
	"aidatadedup":       arch.ModuleTierLab,
	"aidatamasking":     arch.ModuleTierLab,
	"aidefrag":          arch.ModuleTierLab,
	"aideidentification": arch.ModuleTierLab,
	"aifilearchive":     arch.ModuleTierLab,
	"ailoganalyzer":     arch.ModuleTierLab,
	"aiops":             arch.ModuleTierLab,
	"aiplatform":        arch.ModuleTierLab,
	"aisysadmin":        arch.ModuleTierLab,
	"aitaskagent":       arch.ModuleTierLab,
	"aitokenmeter":      arch.ModuleTierLab,
	"aitraffic":         arch.ModuleTierLab,
	"aitranscription":   arch.ModuleTierLab,
	"aivideounderstand": arch.ModuleTierLab,
	"aiworkflow":        arch.ModuleTierLab,
	"aiwriter":          arch.ModuleTierLab,
	"airplay":           arch.ModuleTierLab,

	// demoted: cost/billing/budget/tco/cloud duplicates → lab
	"cloudbillfc":       arch.ModuleTierLab,
	"cloudbilling":      arch.ModuleTierLab,
	"cloudfederation":   arch.ModuleTierLab,
	"cloudfuse":         arch.ModuleTierLab,
	"cloudportal":       arch.ModuleTierLab,
	"cloudsyncmgr":      arch.ModuleTierLab,
	"cloudui":           arch.ModuleTierLab,
	"cloudbackupsync":   arch.ModuleTierLab,
	"cloudmount":        arch.ModuleTierLab,
	"cloudconnect":      arch.ModuleTierLab,
	"cloudbackup":       arch.ModuleTierLab,
	"costdashboard":     arch.ModuleTierLab,
	"costforecast":      arch.ModuleTierLab,
	"costgovernance":    arch.ModuleTierLab,
	"costpredict":       arch.ModuleTierLab,
	"costanalyzer":      arch.ModuleTierLab,
	"tcocalc":           arch.ModuleTierLab,
	"tcodashboard":      arch.ModuleTierLab,
	"tcoshield":         arch.ModuleTierLab,
	"billing":           arch.ModuleTierLab,
	"budget":            arch.ModuleTierLab,
	"cost":              arch.ModuleTierLab,
}

// ModuleTierFor 返回模块在当前收敛目录中的层级；未知模块默认按 Extension 处理，避免伪装成 Core。
func ModuleTierFor(name string) arch.ModuleTier {
	if tier, ok := moduleCatalog[name]; ok {
		return tier
	}
	return arch.ModuleTierExtension
}

// ModuleCatalogSnapshot 返回当前架构收敛视角下的模块清单。
// 仅 Core 允许进入进程生命周期主图；其余模块默认归为 Extension 或 Lab。
func ModuleCatalogSnapshot() map[string]arch.ModuleTier {
	catalog := make(map[string]arch.ModuleTier, len(moduleCatalog))
	for name, tier := range moduleCatalog {
		catalog[name] = tier
	}
	return catalog
}
