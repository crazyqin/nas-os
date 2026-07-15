package application

import "nas-os/internal/arch"

// ModuleCatalogSnapshot 返回当前架构收敛视角下的模块清单。
// 仅 Core 允许进入进程生命周期主图；其余模块默认归为 Extension 或 Lab。
func ModuleCatalogSnapshot() map[string]arch.ModuleTier {
	return map[string]arch.ModuleTier{
		moduleIdentity: arch.ModuleTierCore,
		moduleStorage:  arch.ModuleTierCore,
		moduleNetwork:  arch.ModuleTierCore,
		moduleSharing:  arch.ModuleTierCore,
		moduleSystem:   arch.ModuleTierCore,

		"acl":           arch.ModuleTierExtension,
		"acme":          arch.ModuleTierExtension,
		"activebackup":  arch.ModuleTierExtension,
		"activeprotect": arch.ModuleTierExtension,
		"agentworkflow": arch.ModuleTierExtension,
		"ai":            arch.ModuleTierExtension,
		"aiguardrails":  arch.ModuleTierExtension,
		"alerting":      arch.ModuleTierExtension,
		"container":     arch.ModuleTierExtension,
		"download":      arch.ModuleTierExtension,
		"reports":       arch.ModuleTierExtension,
		"selfheal":      arch.ModuleTierExtension,
		"smartpricing":  arch.ModuleTierExtension,
		"voicehub":      arch.ModuleTierExtension,
		"ztna":          arch.ModuleTierExtension,

		"brandinsight":       arch.ModuleTierLab,
		"costbenchmark":      arch.ModuleTierLab,
		"datasovereignty2":   arch.ModuleTierLab,
		"draid2":             arch.ModuleTierLab,
		"featurematrix":      arch.ModuleTierLab,
		"forensics2":         arch.ModuleTierLab,
		"netshield":          arch.ModuleTierLab,
		"posterwallpro":      arch.ModuleTierLab,
		"releasemanager":     arch.ModuleTierLab,
		"safeaccess":         arch.ModuleTierLab,
		"storagecostpredict": arch.ModuleTierLab,
		"truecloudbk":        arch.ModuleTierLab,
		"updatedirector":     arch.ModuleTierLab,
	}
}
