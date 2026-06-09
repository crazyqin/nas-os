package opsrunbook

import "time"

// LoadBuiltInTemplates 加载内置运维手册模板
func LoadBuiltInTemplates() []*Runbook {
	return []*Runbook{
		diskReplacementTemplate(),
		serviceRecoveryTemplate(),
		backupVerificationTemplate(),
		systemUpgradeTemplate(),
		networkTroubleshootTemplate(),
		emergencyShutdownTemplate(),
	}
}

// diskReplacementTemplate 磁盘更换手册
func diskReplacementTemplate() *Runbook {
	return &Runbook{
		ID:          "builtin_disk_replacement",
		Name:        "磁盘更换流程",
		Description: "标准化磁盘更换流程，包括故障确认、数据迁移、安全移除和新盘接入",
		Category:    "storage",
		Severity:    SevCritical,
		Tags:        []string{"storage", "disk", "hardware", "emergency"},
		Trigger:     TriggerAlert,
		Steps: []*Step{
			{
				ID:          "check_disk_status",
				Name:        "检查磁盘状态",
				Description: "确认磁盘故障状态，获取SMART信息",
				Type:        StepTypeCommand,
				Command:     "smartctl -a ${disk_device}",
				Timeout:     30 * time.Second,
				RetryCount:  2,
				RetryDelay:  5 * time.Second,
			},
			{
				ID:         "check_raid_status",
				Name:       "检查RAID阵列状态",
				Description: "确认RAID阵列降级状态",
				Type:       StepTypeCommand,
				Command:    "btrfs device stats ${mount_point}",
				Timeout:    15 * time.Second,
				DependsOn:  []string{"check_disk_status"},
			},
			{
				ID:          "notify_team",
				Name:        "通知运维团队",
				Description: "发送磁盘故障通知",
				Type:        StepTypeNotify,
				Command:     "磁盘故障告警: ${disk_device} 需要更换",
				DependsOn:   []string{"check_raid_status"},
			},
			{
				ID:          "approval_replace",
				Name:        "审批磁盘更换",
				Description: "确认可以执行磁盘更换操作",
				Type:        StepTypeApproval,
				DependsOn:   []string{"notify_team"},
			},
			{
				ID:          "start_replace",
				Name:        "启动磁盘替换",
				Description: "执行btrfs设备替换",
				Type:        StepTypeCommand,
				Command:     "btrfs device replace start ${disk_device} ${new_device} ${mount_point}",
				Timeout:     2 * time.Hour,
				DependsOn:   []string{"approval_replace"},
				Rollback: &Step{
					ID:      "cancel_replace",
					Name:    "取消磁盘替换",
					Type:    StepTypeCommand,
					Command: "btrfs device replace cancel ${mount_point}",
				},
			},
			{
				ID:          "verify_replace",
				Name:        "验证替换结果",
				Description: "确认新磁盘工作正常",
				Type:        StepTypeCheck,
				Command:     "btrfs device stats ${mount_point} | grep -c ' 0'",
				Timeout:     30 * time.Second,
				DependsOn:   []string{"start_replace"},
				RetryCount:  3,
				RetryDelay:  10 * time.Second,
			},
			{
				ID:          "update_status",
				Name:        "更新资产状态",
				Description: "记录磁盘更换完成",
				Type:        StepTypeNotify,
				Command:     "磁盘更换完成: ${disk_device} -> ${new_device}",
				DependsOn:   []string{"verify_replace"},
			},
		},
		Variables: []*Variable{
			{Name: "disk_device", Description: "故障磁盘设备路径", Type: "string", Required: true},
			{Name: "new_device", Description: "新磁盘设备路径", Type: "string", Required: true},
			{Name: "mount_point", Description: "挂载点", Type: "string", DefaultValue: "/mnt/storage", Required: true},
		},
		RollbackOn: "failure",
		Author:     "system",
	}
}

// serviceRecoveryTemplate 服务恢复手册
func serviceRecoveryTemplate() *Runbook {
	return &Runbook{
		ID:          "builtin_service_recovery",
		Name:        "服务故障恢复",
		Description: "标准服务故障恢复流程，包括状态检查、日志分析、自动修复和验证",
		Category:    "service",
		Severity:    SevError,
		Tags:        []string{"service", "recovery", "auto-heal"},
		Trigger:     TriggerAlert,
		Steps: []*Step{
			{
				ID:          "check_service",
				Name:        "检查服务状态",
				Description: "确认服务当前运行状态",
				Type:        StepTypeCommand,
				Command:     "systemctl status ${service_name} --no-pager",
				Timeout:     10 * time.Second,
			},
			{
				ID:          "collect_logs",
				Name:        "收集服务日志",
				Description: "获取最近的服务日志用于分析",
				Type:        StepTypeCommand,
				Command:     "journalctl -u ${service_name} -n 50 --no-pager",
				Timeout:     10 * time.Second,
				DependsOn:   []string{"check_service"},
			},
			{
				ID:          "check_resources",
				Name:        "检查系统资源",
				Description: "确认系统资源是否充足",
				Type:        StepTypeCheck,
				Command:     "test $(free -m | awk '/^Mem:/{print $7}') -gt 256",
				Timeout:     5 * time.Second,
				DependsOn:   []string{"collect_logs"},
			},
			{
				ID:          "restart_service",
				Name:        "重启服务",
				Description: "尝试重启故障服务",
				Type:        StepTypeCommand,
				Command:     "systemctl restart ${service_name}",
				Timeout:     60 * time.Second,
				DependsOn:   []string{"check_resources"},
				RetryCount:  2,
				RetryDelay:  5 * time.Second,
			},
			{
				ID:          "verify_service",
				Name:        "验证服务恢复",
				Description: "确认服务已恢复正常",
				Type:        StepTypeCheck,
				Command:     "systemctl is-active ${service_name}",
				Timeout:     30 * time.Second,
				DependsOn:   []string{"restart_service"},
				RetryCount:  3,
				RetryDelay:  10 * time.Second,
			},
			{
				ID:          "health_check",
				Name:        "服务健康检查",
				Description: "执行应用层健康检查",
				Type:        StepTypeCheck,
				Command:     "curl -sf http://localhost:${port}/health",
				Timeout:     15 * time.Second,
				DependsOn:   []string{"verify_service"},
				RetryCount:  3,
				RetryDelay:  5 * time.Second,
			},
		},
		Variables: []*Variable{
			{Name: "service_name", Description: "服务名称", Type: "string", Required: true},
			{Name: "port", Description: "服务端口", Type: "string", DefaultValue: "8080"},
		},
		RollbackOn: "never",
		Author:     "system",
	}
}

// backupVerificationTemplate 备份验证手册
func backupVerificationTemplate() *Runbook {
	return &Runbook{
		ID:          "builtin_backup_verify",
		Name:        "备份完整性验证",
		Description: "定期验证备份数据的完整性和可恢复性",
		Category:    "backup",
		Severity:    SevWarning,
		Tags:        []string{"backup", "verification", "scheduled"},
		Trigger:     TriggerSchedule,
		Steps: []*Step{
			{
				ID:          "list_backups",
				Name:        "列出可用备份",
				Description: "获取最新的备份列表",
				Type:        StepTypeCommand,
				Command:     "ls -lt ${backup_path} | head -10",
				Timeout:     15 * time.Second,
			},
			{
				ID:          "check_integrity",
				Name:        "检查备份完整性",
				Description: "验证备份文件校验和",
				Type:        StepTypeScript,
				Script:      "cd ${backup_path} && sha256sum -c ${backup_path}/checksums.sha256",
				Timeout:     5 * time.Minute,
				DependsOn:   []string{"list_backups"},
			},
			{
				ID:          "test_restore",
				Name:        "测试恢复",
				Description: "在临时目录测试恢复备份",
				Type:        StepTypeScript,
				Script:      "mkdir -p /tmp/backup_test && tar xzf ${backup_path}/latest.tar.gz -C /tmp/backup_test && ls /tmp/backup_test",
				Timeout:     10 * time.Minute,
				DependsOn:   []string{"check_integrity"},
			},
			{
				ID:          "cleanup_test",
				Name:        "清理测试数据",
				Description: "删除临时恢复测试数据",
				Type:        StepTypeCommand,
				Command:     "rm -rf /tmp/backup_test",
				DependsOn:   []string{"test_restore"},
				ContinueOn:  "always",
			},
			{
				ID:          "report",
				Name:        "生成验证报告",
				Description: "通知备份验证结果",
				Type:        StepTypeNotify,
				Command:     "备份验证完成: ${backup_path}",
				DependsOn:   []string{"cleanup_test"},
			},
		},
		Variables: []*Variable{
			{Name: "backup_path", Description: "备份存储路径", Type: "string", Required: true},
		},
		RollbackOn: "never",
		Author:     "system",
	}
}

// systemUpgradeTemplate 系统升级手册
func systemUpgradeTemplate() *Runbook {
	return &Runbook{
		ID:          "builtin_system_upgrade",
		Name:        "系统升级流程",
		Description: "标准系统升级流程，包括备份、升级、验证和回滚能力",
		Category:    "system",
		Severity:    SevCritical,
		Tags:        []string{"system", "upgrade", "maintenance"},
		Trigger:     TriggerManual,
		Steps: []*Step{
			{
				ID:          "pre_check",
				Name:        "升级前检查",
				Description: "检查系统状态和磁盘空间",
				Type:        StepTypeScript,
				Script:      "df -h / | tail -1 | awk '{print $5}' | sed 's/%//' | xargs test 80 -gt",
				Timeout:     10 * time.Second,
			},
			{
				ID:          "backup_config",
				Name:        "备份配置",
				Description: "备份当前系统配置",
				Type:        StepTypeScript,
				Script:      "tar czf /tmp/nas_config_backup_$(date +%Y%m%d%H%M%S).tar.gz /etc/nas-os/",
				Timeout:     2 * time.Minute,
				DependsOn:   []string{"pre_check"},
				Rollback: &Step{
					ID:      "restore_config",
					Name:    "恢复配置",
					Type:    StepTypeScript,
					Script:  "tar xzf /tmp/nas_config_backup_*.tar.gz -C /",
				},
			},
			{
				ID:          "approval_upgrade",
				Name:        "审批升级操作",
				Description: "确认执行系统升级",
				Type:        StepTypeApproval,
				DependsOn:   []string{"backup_config"},
			},
			{
				ID:          "stop_services",
				Name:        "停止服务",
				Description: "安全停止NAS服务",
				Type:        StepTypeCommand,
				Command:     "systemctl stop nasd",
				Timeout:     30 * time.Second,
				DependsOn:   []string{"approval_upgrade"},
				Rollback: &Step{
					ID:      "start_services",
					Name:    "恢复服务",
					Type:    StepTypeCommand,
					Command: "systemctl start nasd",
				},
			},
			{
				ID:          "apply_upgrade",
				Name:        "执行升级",
				Description: "应用系统更新",
				Type:        StepTypeCommand,
				Command:     "${upgrade_command}",
				Timeout:     10 * time.Minute,
				DependsOn:   []string{"stop_services"},
			},
			{
				ID:          "start_services",
				Name:        "启动服务",
				Description: "启动NAS服务",
				Type:        StepTypeCommand,
				Command:     "systemctl start nasd",
				Timeout:     60 * time.Second,
				DependsOn:   []string{"apply_upgrade"},
			},
			{
				ID:          "verify_upgrade",
				Name:        "验证升级结果",
				Description: "确认服务正常运行",
				Type:        StepTypeCheck,
				Command:     "systemctl is-active nasd",
				Timeout:     30 * time.Second,
				DependsOn:   []string{"start_services"},
				RetryCount:  3,
				RetryDelay:  10 * time.Second,
			},
		},
		Variables: []*Variable{
			{Name: "upgrade_command", Description: "升级命令", Type: "string", Required: true},
		},
		RollbackOn: "failure",
		Author:     "system",
	}
}

// networkTroubleshootTemplate 网络故障排查手册
func networkTroubleshootTemplate() *Runbook {
	return &Runbook{
		ID:          "builtin_network_troubleshoot",
		Name:        "网络故障排查",
		Description: "网络连接问题的标准排查流程",
		Category:    "network",
		Severity:    SevError,
		Tags:        []string{"network", "troubleshoot", "diagnostic"},
		Trigger:     TriggerAlert,
		Steps: []*Step{
			{
				ID:      "check_interfaces",
				Name:    "检查网络接口",
				Type:    StepTypeCommand,
				Command: "ip -br addr show",
				Timeout: 5 * time.Second,
			},
			{
				ID:        "check_dns",
				Name:      "检查DNS解析",
				Type:      StepTypeCheck,
				Command:   "nslookup ${target_host}",
				Timeout:   10 * time.Second,
				DependsOn: []string{"check_interfaces"},
			},
			{
				ID:        "ping_gateway",
				Name:      "Ping网关",
				Type:      StepTypeCheck,
				Command:   "ping -c 3 -W 2 ${gateway}",
				Timeout:   15 * time.Second,
				DependsOn: []string{"check_dns"},
			},
			{
				ID:        "ping_external",
				Name:      "Ping外部地址",
				Type:      StepTypeCheck,
				Command:   "ping -c 3 -W 2 ${target_host}",
				Timeout:   15 * time.Second,
				DependsOn: []string{"ping_gateway"},
			},
			{
				ID:        "check_firewall",
				Name:      "检查防火墙规则",
				Type:      StepTypeCommand,
				Command:   "iptables -L -n | head -30",
				Timeout:   5 * time.Second,
				DependsOn: []string{"ping_external"},
				ContinueOn: "always",
			},
			{
				ID:        "check_connections",
				Name:      "检查网络连接",
				Type:      StepTypeCommand,
				Command:   "ss -tuln | head -20",
				Timeout:   5 * time.Second,
				DependsOn: []string{"check_firewall"},
				ContinueOn: "always",
			},
			{
				ID:      "diagnose_report",
				Name:    "生成诊断报告",
				Type:    StepTypeNotify,
				Command: "网络诊断完成，请查看详细日志",
				DependsOn: []string{"check_connections"},
			},
		},
		Variables: []*Variable{
			{Name: "target_host", Description: "目标主机", Type: "string", DefaultValue: "8.8.8.8"},
			{Name: "gateway", Description: "网关地址", Type: "string", DefaultValue: "192.168.1.1"},
		},
		RollbackOn: "never",
		Author:     "system",
	}
}

// emergencyShutdownTemplate 紧急关机手册
func emergencyShutdownTemplate() *Runbook {
	return &Runbook{
		ID:          "builtin_emergency_shutdown",
		Name:        "紧急关机流程",
		Description: "紧急情况下的安全关机流程，确保数据完整性",
		Category:    "system",
		Severity:    SevCritical,
		Tags:        []string{"system", "emergency", "shutdown"},
		Trigger:     TriggerManual,
		Steps: []*Step{
			{
				ID:          "flush_data",
				Name:        "刷新数据缓存",
				Description: "确保所有缓存数据写入磁盘",
				Type:        StepTypeCommand,
				Command:     "sync",
				Timeout:     30 * time.Second,
			},
			{
				ID:          "stop_apps",
				Name:        "停止应用服务",
				Description: "安全停止所有应用服务",
				Type:        StepTypeScript,
				Script:      "systemctl stop nasd docker 2>/dev/null || true",
				Timeout:     60 * time.Second,
				DependsOn:   []string{"flush_data"},
			},
			{
				ID:          "unmount_storage",
				Name:        "卸载存储",
				Description: "安全卸载存储设备",
				Type:        StepTypeScript,
				Script:      "umount -a -t btrfs 2>/dev/null || true",
				Timeout:     30 * time.Second,
				DependsOn:   []string{"stop_apps"},
				ContinueOn:  "always",
			},
			{
				ID:          "notify_shutdown",
				Name:        "发送关机通知",
				Type:        StepTypeNotify,
				Command:     "系统正在执行紧急关机",
				DependsOn:   []string{"unmount_storage"},
			},
			{
				ID:          "execute_shutdown",
				Name:        "执行关机",
				Description: "执行系统关机",
				Type:        StepTypeCommand,
				Command:     "shutdown -h now",
				Timeout:     10 * time.Second,
				DependsOn:   []string{"notify_shutdown"},
			},
		},
		Variables:   []*Variable{},
		RollbackOn:  "never",
		Author:      "system",
	}
}
