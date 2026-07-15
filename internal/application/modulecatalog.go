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

	"brandinsight":       arch.ModuleTierLab,
	"cinemarec":          arch.ModuleTierLab,
	"costbenchmark":      arch.ModuleTierLab,
	"datasovereignty2":   arch.ModuleTierLab,
	"draid2":             arch.ModuleTierLab,
	"featurematrix":      arch.ModuleTierLab,
	"forensics2":         arch.ModuleTierLab,
	"netshield":          arch.ModuleTierLab,
	"posterwallpro":      arch.ModuleTierLab,
	"releasemanager":     arch.ModuleTierLab,
	"safeaccess":         arch.ModuleTierLab,
	"snapviz":            arch.ModuleTierLab,
	"storagecostpredict": arch.ModuleTierLab,
	"tcodash":            arch.ModuleTierLab,
	"truecloudbk":        arch.ModuleTierLab,
	"updatedirector":     arch.ModuleTierLab,
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
