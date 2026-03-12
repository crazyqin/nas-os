package templates

import (
	"nas-os/internal/automation/action"
	"nas-os/internal/automation/engine"
	"nas-os/internal/automation/trigger"
)

// Template 工作流模板
type Template struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	Workflow    engine.Workflow `json:"workflow"`
}

// GetTemplates 获取所有预置模板
func GetTemplates() []Template {
	return []Template{
		// 文件管理模板
		{
			ID:          "tpl_file_backup",
			Name:        "文件自动备份",
			Description: "定时备份指定文件夹到备份目录",
			Category:    "文件管理",
			Workflow: engine.Workflow{
				Name:        "文件自动备份",
				Description: "每天凌晨 2 点备份重要文件",
				Enabled:     true,
				Trigger: &trigger.TimeTrigger{
					Type:     trigger.TriggerTypeTime,
					Schedule: "0 2 * * *",
					Timezone: "Asia/Shanghai",
				},
				Actions: []action.Action{
					&action.CopyAction{
						Type:        action.ActionTypeCopy,
						Source:      "/home/user/documents",
						Destination: "/backup/documents_{{timestamp}}",
						Recursive:   true,
					},
					&action.NotifyAction{
						Type:    action.ActionTypeNotify,
						Channel: "discord",
						Title:   "备份完成",
						Message: "文件备份已完成：{{timestamp}}",
					},
				},
			},
		},
		{
			ID:          "tpl_file_organize",
			Name:        "下载文件夹整理",
			Description: "监控下载文件夹，自动分类整理文件",
			Category:    "文件管理",
			Workflow: engine.Workflow{
				Name:        "下载文件夹整理",
				Description: "自动将下载文件按类型分类",
				Enabled:     true,
				Trigger: &trigger.FileTrigger{
					Type:      trigger.TriggerTypeFile,
					Path:      "/home/user/downloads",
					Pattern:   "*",
					Events:    []string{"created"},
					Recursive: false,
				},
				Actions: []action.Action{
					&action.CommandAction{
						Type:    action.ActionTypeCommand,
						Command: "bash",
						Args:    []string{"-c", "organize_downloads.sh"},
					},
				},
			},
		},
		{
			ID:          "tpl_file_cleanup",
			Name:        "临时文件清理",
			Description: "定期清理临时文件和缓存",
			Category:    "文件管理",
			Workflow: engine.Workflow{
				Name:        "临时文件清理",
				Description: "每周日清理 7 天前的临时文件",
				Enabled:     true,
				Trigger: &trigger.TimeTrigger{
					Type:     trigger.TriggerTypeTime,
					Schedule: "0 3 * * 0",
					Timezone: "Asia/Shanghai",
				},
				Actions: []action.Action{
					&action.DeleteAction{
						Type:      action.ActionTypeDelete,
						Path:      "/tmp/*",
						Recursive: true,
					},
					&action.NotifyAction{
						Type:    action.ActionTypeNotify,
						Channel: "discord",
						Title:   "清理完成",
						Message: "临时文件已清理",
					},
				},
			},
		},

		// 媒体处理模板
		{
			ID:          "tpl_media_convert",
			Name:        "视频格式转换",
			Description: "新视频文件自动转换为 MP4 格式",
			Category:    "媒体处理",
			Workflow: engine.Workflow{
				Name:        "视频格式转换",
				Description: "监控上传文件夹，自动转换视频格式",
				Enabled:     true,
				Trigger: &trigger.FileTrigger{
					Type:      trigger.TriggerTypeFile,
					Path:      "/media/uploads",
					Pattern:   "*.avi,*.mkv,*.mov",
					Events:    []string{"created"},
					Recursive: true,
				},
				Actions: []action.Action{
					&action.ConvertAction{
						Type:        action.ActionTypeConvert,
						Source:      "{{event.path}}",
						Destination: "/media/converted/{{event.filename}}.mp4",
						Format:      "mp4",
					},
					&action.NotifyAction{
						Type:    action.ActionTypeNotify,
						Channel: "discord",
						Title:   "转换完成",
						Message: "视频已转换：{{event.filename}}",
					},
				},
			},
		},
		{
			ID:          "tpl_media_thumbnail",
			Name:        "自动生成缩略图",
			Description: "为图片文件生成缩略图",
			Category:    "媒体处理",
			Workflow: engine.Workflow{
				Name:        "自动生成缩略图",
				Description: "为上传的图片生成缩略图",
				Enabled:     true,
				Trigger: &trigger.FileTrigger{
					Type:      trigger.TriggerTypeFile,
					Path:      "/media/photos",
					Pattern:   "*.jpg,*.png,*.webp",
					Events:    []string{"created"},
					Recursive: true,
				},
				Actions: []action.Action{
					&action.ConvertAction{
						Type:        action.ActionTypeConvert,
						Source:      "{{event.path}}",
						Destination: "/media/thumbnails/{{event.filename}}_thumb.jpg",
						Format:      "jpg",
					},
				},
			},
		},

		// 系统监控模板
		{
			ID:          "tpl_system_health",
			Name:        "系统健康检查",
			Description: "定期检查系统状态并发送报告",
			Category:    "系统监控",
			Workflow: engine.Workflow{
				Name:        "系统健康检查",
				Description: "每天早上 8 点发送系统健康报告",
				Enabled:     true,
				Trigger: &trigger.TimeTrigger{
					Type:     trigger.TriggerTypeTime,
					Schedule: "0 8 * * *",
					Timezone: "Asia/Shanghai",
				},
				Actions: []action.Action{
					&action.CommandAction{
						Type:    action.ActionTypeCommand,
						Command: "system_health_check.sh",
					},
					&action.NotifyAction{
						Type:    action.ActionTypeNotify,
						Channel: "discord",
						Title:   "系统健康报告",
						Message: "系统运行正常",
					},
				},
			},
		},
		{
			ID:          "tpl_disk_alert",
			Name:        "磁盘空间告警",
			Description: "磁盘空间不足时发送告警",
			Category:    "系统监控",
			Workflow: engine.Workflow{
				Name:        "磁盘空间告警",
				Description: "每小时检查磁盘空间",
				Enabled:     true,
				Trigger: &trigger.TimeTrigger{
					Type:     trigger.TriggerTypeTime,
					Schedule: "0 * * * *",
					Timezone: "Asia/Shanghai",
				},
				Actions: []action.Action{
					&action.CommandAction{
						Type:    action.ActionTypeCommand,
						Command: "check_disk_space.sh",
					},
				},
			},
		},

		// 通知模板
		{
			ID:          "tpl_notify_welcome",
			Name:        "欢迎消息",
			Description: "系统启动时发送欢迎消息",
			Category:    "通知",
			Workflow: engine.Workflow{
				Name:        "欢迎消息",
				Description: "系统启动时发送通知",
				Enabled:     true,
				Trigger: &trigger.EventTrigger{
					Type:      trigger.TriggerTypeEvent,
					EventType: "system.startup",
				},
				Actions: []action.Action{
					&action.NotifyAction{
						Type:    action.ActionTypeNotify,
						Channel: "discord",
						Title:   "系统已启动",
						Message: "NAS 系统已成功启动",
					},
				},
			},
		},
		{
			ID:          "tpl_notify_login",
			Name:        "登录通知",
			Description: "用户登录时发送通知",
			Category:    "通知",
			Workflow: engine.Workflow{
				Name:        "登录通知",
				Description: "监控用户登录事件",
				Enabled:     true,
				Trigger: &trigger.EventTrigger{
					Type:      trigger.TriggerTypeEvent,
					EventType: "user.login",
				},
				Actions: []action.Action{
					&action.NotifyAction{
						Type:    action.ActionTypeNotify,
						Channel: "discord",
						Title:   "用户登录",
						Message: "用户 {{event.username}} 已登录",
					},
				},
			},
		},

		// 数据同步模板
		{
			ID:          "tpl_sync_cloud",
			Name:        "云端同步",
			Description: "定时同步数据到云端存储",
			Category:    "数据同步",
			Workflow: engine.Workflow{
				Name:        "云端同步",
				Description: "每天凌晨 3 点同步到云端",
				Enabled:     true,
				Trigger: &trigger.TimeTrigger{
					Type:     trigger.TriggerTypeTime,
					Schedule: "0 3 * * *",
					Timezone: "Asia/Shanghai",
				},
				Actions: []action.Action{
					&action.CommandAction{
						Type:    action.ActionTypeCommand,
						Command: "rclone",
						Args:    []string{"sync", "/data", "remote:backup"},
					},
					&action.NotifyAction{
						Type:    action.ActionTypeNotify,
						Channel: "discord",
						Title:   "云端同步完成",
						Message: "数据已同步到云端",
					},
				},
			},
		},
	}
}

// GetTemplate 获取单个模板
func GetTemplate(id string) (*Template, error) {
	templates := GetTemplates()
	for _, t := range templates {
		if t.ID == id {
			return &t, nil
		}
	}
	return nil, nil
}

// GetTemplatesByCategory 按分类获取模板
func GetTemplatesByCategory(category string) []Template {
	templates := GetTemplates()
	result := []Template{}
	for _, t := range templates {
		if t.Category == category {
			result = append(result, t)
		}
	}
	return result
}

// GetCategories 获取所有分类
func GetCategories() []string {
	categories := make(map[string]bool)
	for _, t := range GetTemplates() {
		categories[t.Category] = true
	}

	result := []string{}
	for cat := range categories {
		result = append(result, cat)
	}
	return result
}
