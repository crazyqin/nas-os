package templates

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"nas-os/internal/automation/action"
	"nas-os/internal/automation/engine"
	"nas-os/internal/automation/trigger"
)

// Template 工作流模板.
type Template struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	Version     string          `json:"version"`
	Author      string          `json:"author,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
	Workflow    engine.Workflow `json:"workflow"`
}

// ValidationResult 验证结果.
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// TemplateManager 模板管理器.
type TemplateManager struct {
	customTemplates map[string]*Template
	storagePath     string
}

// NewTemplateManager 创建模板管理器.
func NewTemplateManager(storagePath string) (*TemplateManager, error) {
	tm := &TemplateManager{
		customTemplates: make(map[string]*Template),
		storagePath:     storagePath,
	}

	// 确保存储目录存在
	if storagePath != "" {
		if err := os.MkdirAll(storagePath, 0750); err != nil {
			return nil, fmt.Errorf("failed to create storage directory: %w", err)
		}
		// 加载自定义模板
		if err := tm.loadCustomTemplates(); err != nil {
			return nil, fmt.Errorf("failed to load custom templates: %w", err)
		}
	}

	return tm, nil
}

// loadCustomTemplates 从存储加载自定义模板.
func (tm *TemplateManager) loadCustomTemplates() error {
	if tm.storagePath == "" {
		return nil
	}

	entries, err := os.ReadDir(tm.storagePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(tm.storagePath, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var tpl Template
		if err := json.Unmarshal(data, &tpl); err != nil {
			continue
		}

		tm.customTemplates[tpl.ID] = &tpl
	}

	return nil
}

// GetTemplates 获取所有模板（预置 + 自定义）.
func GetTemplates() []Template {
	return GetBuiltInTemplates()
}

// GetBuiltInTemplates 获取预置模板.
func GetBuiltInTemplates() []Template {
	return []Template{
		// ==================== 备份任务模板 ====================
		{
			ID:          "tpl_backup_daily",
			Name:        "每日数据备份",
			Description: "每天定时备份指定文件夹，支持增量备份和完整备份",
			Category:    "备份任务",
			Version:     "1.0.0",
			Tags:        []string{"backup", "scheduled", "data-protection"},
			Workflow: engine.Workflow{
				Name:        "每日数据备份",
				Description: "每天凌晨 2 点备份重要文件到备份目录",
				Enabled:     true,
				Trigger: &trigger.TimeTrigger{
					Type:     trigger.TypeTime,
					Schedule: "0 2 * * *",
					Timezone: "Asia/Shanghai",
				},
				Actions: []action.Action{
					&action.CopyAction{
						Type:        action.TypeCopy,
						Source:      "{{source_path}}",
						Destination: "{{backup_path}}/{{date}}",
						Recursive:   true,
					},
					&action.NotifyAction{
						Type:    action.TypeNotify,
						Channel: "{{notify_channel}}",
						Title:   "备份完成",
						Message: "数据备份已完成，备份位置：{{backup_path}}/{{date}}",
					},
				},
			},
		},
		{
			ID:          "tpl_backup_incremental",
			Name:        "增量备份任务",
			Description: "监控文件夹变化，自动增量备份新增和修改的文件",
			Category:    "备份任务",
			Version:     "1.0.0",
			Tags:        []string{"backup", "incremental", "realtime"},
			Workflow: engine.Workflow{
				Name:        "增量备份",
				Description: "监控源文件夹，自动备份变化的文件",
				Enabled:     true,
				Trigger: &trigger.FileTrigger{
					Type:      trigger.TypeFile,
					Path:      "{{source_path}}",
					Events:    []string{"created", "modified"},
					Recursive: true,
				},
				Actions: []action.Action{
					&action.CopyAction{
						Type:        action.TypeCopy,
						Source:      "{{event.path}}",
						Destination: "{{backup_path}}/{{event.filename}}",
						Recursive:   false,
					},
				},
			},
		},
		{
			ID:          "tpl_backup_weekly_full",
			Name:        "每周完整备份",
			Description: "每周日凌晨执行完整备份并清理旧备份",
			Category:    "备份任务",
			Version:     "1.0.0",
			Tags:        []string{"backup", "weekly", "full"},
			Workflow: engine.Workflow{
				Name:        "每周完整备份",
				Description: "每周日凌晨 3 点执行完整备份",
				Enabled:     true,
				Trigger: &trigger.TimeTrigger{
					Type:     trigger.TypeTime,
					Schedule: "0 3 * * 0",
					Timezone: "Asia/Shanghai",
				},
				Actions: []action.Action{
					&action.CommandAction{
						Type:    action.TypeCommand,
						Command: "tar",
						Args:    []string{"-czf", "{{backup_path}}/full_backup_{{date}}.tar.gz", "{{source_path}}"},
					},
					&action.NotifyAction{
						Type:    action.TypeNotify,
						Channel: "{{notify_channel}}",
						Title:   "完整备份完成",
						Message: "每周完整备份已完成",
					},
				},
			},
		},

		// ==================== 定时清理模板 ====================
		{
			ID:          "tpl_cleanup_temp",
			Name:        "临时文件清理",
			Description: "定期清理临时文件夹中的过期文件，释放磁盘空间",
			Category:    "定时清理",
			Version:     "1.0.0",
			Tags:        []string{"cleanup", "temp", "scheduled"},
			Workflow: engine.Workflow{
				Name:        "临时文件清理",
				Description: "每天凌晨 4 点清理 7 天前的临时文件",
				Enabled:     true,
				Trigger: &trigger.TimeTrigger{
					Type:     trigger.TypeTime,
					Schedule: "0 4 * * *",
					Timezone: "Asia/Shanghai",
				},
				Actions: []action.Action{
					&action.CommandAction{
						Type:    action.TypeCommand,
						Command: "find",
						Args:    []string{"{{temp_path}}", "-type", "f", "-mtime", "+{{retention_days}}", "-delete"},
					},
					&action.NotifyAction{
						Type:    action.TypeNotify,
						Channel: "{{notify_channel}}",
						Title:   "清理完成",
						Message: "临时文件已清理",
					},
				},
			},
		},
		{
			ID:          "tpl_cleanup_logs",
			Name:        "日志文件清理",
			Description: "定期清理过期的日志文件，保留最近 N 天的日志",
			Category:    "定时清理",
			Version:     "1.0.0",
			Tags:        []string{"cleanup", "logs", "scheduled"},
			Workflow: engine.Workflow{
				Name:        "日志文件清理",
				Description: "每周清理 30 天前的日志文件",
				Enabled:     true,
				Trigger: &trigger.TimeTrigger{
					Type:     trigger.TypeTime,
					Schedule: "0 5 * * 1",
					Timezone: "Asia/Shanghai",
				},
				Actions: []action.Action{
					&action.CommandAction{
						Type:    action.TypeCommand,
						Command: "find",
						Args:    []string{"{{log_path}}", "-name", "*.log", "-mtime", "+{{retention_days}}", "-delete"},
					},
					&action.NotifyAction{
						Type:    action.TypeNotify,
						Channel: "{{notify_channel}}",
						Title:   "日志清理完成",
						Message: "过期日志文件已清理",
					},
				},
			},
		},
		{
			ID:          "tpl_cleanup_downloads",
			Name:        "下载文件夹清理",
			Description: "定期清理下载文件夹中的旧文件",
			Category:    "定时清理",
			Version:     "1.0.0",
			Tags:        []string{"cleanup", "downloads", "scheduled"},
			Workflow: engine.Workflow{
				Name:        "下载文件夹清理",
				Description: "每月 1 号清理 30 天前的下载文件",
				Enabled:     true,
				Trigger: &trigger.TimeTrigger{
					Type:     trigger.TypeTime,
					Schedule: "0 6 1 * *",
					Timezone: "Asia/Shanghai",
				},
				Actions: []action.Action{
					&action.CommandAction{
						Type:    action.TypeCommand,
						Command: "find",
						Args:    []string{"{{downloads_path}}", "-type", "f", "-mtime", "+{{retention_days}}", "-delete"},
					},
					&action.NotifyAction{
						Type:    action.TypeNotify,
						Channel: "{{notify_channel}}",
						Title:   "下载文件夹清理完成",
						Message: "过期下载文件已清理",
					},
				},
			},
		},

		// ==================== 存储告警模板 ====================
		{
			ID:          "tpl_alert_disk_space",
			Name:        "磁盘空间告警",
			Description: "监控磁盘使用率，超过阈值时发送告警通知",
			Category:    "存储告警",
			Version:     "1.0.0",
			Tags:        []string{"alert", "disk", "monitoring"},
			Workflow: engine.Workflow{
				Name:        "磁盘空间告警",
				Description: "每小时检查磁盘空间，超过 {{threshold}}% 时告警",
				Enabled:     true,
				Trigger: &trigger.TimeTrigger{
					Type:     trigger.TypeTime,
					Schedule: "0 * * * *",
					Timezone: "Asia/Shanghai",
				},
				Actions: []action.Action{
					&action.CommandAction{
						Type:    action.TypeCommand,
						Command: "df",
						Args:    []string{"-h", "{{disk_path}}"},
					},
					&action.NotifyAction{
						Type:    action.TypeNotify,
						Channel: "{{alert_channel}}",
						Title:   "磁盘空间告警",
						Message: "磁盘 {{disk_path}} 使用率已超过 {{threshold}}%",
					},
				},
			},
		},
		{
			ID:          "tpl_alert_disk_critical",
			Name:        "磁盘空间严重告警",
			Description: "磁盘使用率超过 90% 时发送紧急告警",
			Category:    "存储告警",
			Version:     "1.0.0",
			Tags:        []string{"alert", "disk", "critical"},
			Workflow: engine.Workflow{
				Name:        "磁盘空间严重告警",
				Description: "每 30 分钟检查，使用率超过 90% 时紧急告警",
				Enabled:     true,
				Trigger: &trigger.TimeTrigger{
					Type:     trigger.TypeTime,
					Schedule: "*/30 * * * *",
					Timezone: "Asia/Shanghai",
				},
				Actions: []action.Action{
					&action.CommandAction{
						Type:    action.TypeCommand,
						Command: "bash",
						Args:    []string{"-c", "df -h {{disk_path}} | tail -1 | awk '{print $5}' | tr -d '%'"},
					},
					&action.NotifyAction{
						Type:    action.TypeNotify,
						Channel: "{{alert_channel}}",
						Title:   "【紧急】磁盘空间严重不足",
						Message: "磁盘 {{disk_path}} 使用率已超过 90%，请立即处理！",
					},
				},
			},
		},
		{
			ID:          "tpl_alert_volume_health",
			Name:        "存储卷健康检查",
			Description: "定期检查存储卷状态，发现异常时告警",
			Category:    "存储告警",
			Version:     "1.0.0",
			Tags:        []string{"alert", "volume", "health"},
			Workflow: engine.Workflow{
				Name:        "存储卷健康检查",
				Description: "每 6 小时检查存储卷健康状态",
				Enabled:     true,
				Trigger: &trigger.TimeTrigger{
					Type:     trigger.TypeTime,
					Schedule: "0 */6 * * *",
					Timezone: "Asia/Shanghai",
				},
				Actions: []action.Action{
					&action.CommandAction{
						Type:    action.TypeCommand,
						Command: "smartctl",
						Args:    []string{"-H", "{{device_path}}"},
					},
					&action.NotifyAction{
						Type:    action.TypeNotify,
						Channel: "{{alert_channel}}",
						Title:   "存储卷健康状态",
						Message: "存储卷 {{device_path}} 健康检查完成",
					},
				},
			},
		},

		// ==================== 其他常用模板 ====================
		{
			ID:          "tpl_file_organize",
			Name:        "下载文件夹整理",
			Description: "监控下载文件夹，自动按类型分类整理文件",
			Category:    "文件管理",
			Version:     "1.0.0",
			Tags:        []string{"file", "organize", "automation"},
			Workflow: engine.Workflow{
				Name:        "下载文件夹整理",
				Description: "自动将下载文件按类型分类",
				Enabled:     true,
				Trigger: &trigger.FileTrigger{
					Type:      trigger.TypeFile,
					Path:      "{{downloads_path}}",
					Pattern:   "*",
					Events:    []string{"created"},
					Recursive: false,
				},
				Actions: []action.Action{
					&action.CommandAction{
						Type:    action.TypeCommand,
						Command: "bash",
						Args:    []string{"-c", "organize_downloads.sh"},
					},
				},
			},
		},
		{
			ID:          "tpl_media_convert",
			Name:        "视频格式转换",
			Description: "新视频文件自动转换为 MP4 格式",
			Category:    "媒体处理",
			Version:     "1.0.0",
			Tags:        []string{"media", "convert", "video"},
			Workflow: engine.Workflow{
				Name:        "视频格式转换",
				Description: "监控上传文件夹，自动转换视频格式",
				Enabled:     true,
				Trigger: &trigger.FileTrigger{
					Type:      trigger.TypeFile,
					Path:      "{{media_input}}",
					Pattern:   "*.avi,*.mkv,*.mov",
					Events:    []string{"created"},
					Recursive: true,
				},
				Actions: []action.Action{
					&action.ConvertAction{
						Type:        action.TypeConvert,
						Source:      "{{event.path}}",
						Destination: "{{media_output}}/{{event.filename}}.mp4",
						Format:      "mp4",
					},
					&action.NotifyAction{
						Type:    action.TypeNotify,
						Channel: "{{notify_channel}}",
						Title:   "转换完成",
						Message: "视频已转换：{{event.filename}}",
					},
				},
			},
		},
		{
			ID:          "tpl_system_health",
			Name:        "系统健康检查",
			Description: "定期检查系统状态并发送报告",
			Category:    "系统监控",
			Version:     "1.0.0",
			Tags:        []string{"system", "health", "monitoring"},
			Workflow: engine.Workflow{
				Name:        "系统健康检查",
				Description: "每天早上 8 点发送系统健康报告",
				Enabled:     true,
				Trigger: &trigger.TimeTrigger{
					Type:     trigger.TypeTime,
					Schedule: "0 8 * * *",
					Timezone: "Asia/Shanghai",
				},
				Actions: []action.Action{
					&action.CommandAction{
						Type:    action.TypeCommand,
						Command: "bash",
						Args:    []string{"-c", "system_health_check.sh"},
					},
					&action.NotifyAction{
						Type:    action.TypeNotify,
						Channel: "{{notify_channel}}",
						Title:   "系统健康报告",
						Message: "系统运行正常",
					},
				},
			},
		},
		{
			ID:          "tpl_sync_cloud",
			Name:        "云端同步",
			Description: "定时同步数据到云端存储",
			Category:    "数据同步",
			Version:     "1.0.0",
			Tags:        []string{"sync", "cloud", "backup"},
			Workflow: engine.Workflow{
				Name:        "云端同步",
				Description: "每天凌晨 3 点同步到云端",
				Enabled:     true,
				Trigger: &trigger.TimeTrigger{
					Type:     trigger.TypeTime,
					Schedule: "0 3 * * *",
					Timezone: "Asia/Shanghai",
				},
				Actions: []action.Action{
					&action.CommandAction{
						Type:    action.TypeCommand,
						Command: "rclone",
						Args:    []string{"sync", "{{local_path}}", "{{remote_path}}"},
					},
					&action.NotifyAction{
						Type:    action.TypeNotify,
						Channel: "{{notify_channel}}",
						Title:   "云端同步完成",
						Message: "数据已同步到云端",
					},
				},
			},
		},
	}
}

// GetTemplate 获取单个模板.
func GetTemplate(id string) (*Template, error) {
	templates := GetTemplates()
	for _, t := range templates {
		if t.ID == id {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("template not found: %s", id)
}

// GetTemplatesByCategory 按分类获取模板.
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

// GetCategories 获取所有分类.
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

// GetTemplatesByTag 按标签获取模板.
func GetTemplatesByTag(tag string) []Template {
	templates := GetTemplates()
	result := []Template{}
	for _, t := range templates {
		for _, ttag := range t.Tags {
			if ttag == tag {
				result = append(result, t)
				break
			}
		}
	}
	return result
}

// ==================== 模板验证功能 ====================

// ValidateTemplate 验证模板的完整性和有效性.
func ValidateTemplate(tpl *Template) *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []string{},
		Warnings: []string{},
	}

	// 验证基本信息
	if tpl.ID == "" {
		result.Errors = append(result.Errors, "模板 ID 不能为空")
		result.Valid = false
	}
	if tpl.Name == "" {
		result.Errors = append(result.Errors, "模板名称不能为空")
		result.Valid = false
	}
	if tpl.Category == "" {
		result.Warnings = append(result.Warnings, "建议设置模板分类")
	}

	// 验证版本号格式
	if tpl.Version != "" && !isValidVersion(tpl.Version) {
		result.Warnings = append(result.Warnings, "版本号格式建议使用语义化版本 (如 1.0.0)")
	}

	// 验证工作流
	validateWorkflow(&tpl.Workflow, result)

	return result
}

// validateWorkflow 验证工作流配置.
func validateWorkflow(wf *engine.Workflow, result *ValidationResult) {
	if wf.Name == "" {
		result.Errors = append(result.Errors, "工作流名称不能为空")
		result.Valid = false
	}

	// 验证触发器
	if wf.Trigger == nil {
		result.Errors = append(result.Errors, "工作流必须配置触发器")
		result.Valid = false
	} else {
		validateTrigger(wf.Trigger, result)
	}

	// 验证动作
	if len(wf.Actions) == 0 {
		result.Errors = append(result.Errors, "工作流必须至少配置一个动作")
		result.Valid = false
	} else {
		for i, act := range wf.Actions {
			if err := validateAction(act); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("动作 %d 无效: %v", i, err))
				result.Valid = false
			}
		}
	}
}

// validateTrigger 验证触发器配置.
func validateTrigger(trig trigger.Trigger, result *ValidationResult) {
	switch t := trig.(type) {
	case *trigger.TimeTrigger:
		if t.Schedule == "" {
			result.Errors = append(result.Errors, "时间触发器的 schedule 不能为空")
			result.Valid = false
		} else if !isValidCron(t.Schedule) {
			result.Warnings = append(result.Warnings, "时间触发器的 schedule 格式可能不正确")
		}
	case *trigger.FileTrigger:
		if t.Path == "" {
			result.Errors = append(result.Errors, "文件触发器的 path 不能为空")
			result.Valid = false
		}
		if len(t.Events) == 0 {
			result.Warnings = append(result.Warnings, "文件触发器建议配置 events")
		}
	case *trigger.EventTrigger:
		if t.EventType == "" {
			result.Errors = append(result.Errors, "事件触发器的 event_type 不能为空")
			result.Valid = false
		}
	case *trigger.WebhookTrigger:
		if t.Path == "" {
			result.Errors = append(result.Errors, "Webhook 触发器的 path 不能为空")
			result.Valid = false
		}
	}
}

// validateAction 验证动作配置.
func validateAction(act action.Action) error {
	switch a := act.(type) {
	case *action.CopyAction:
		if a.Source == "" {
			return fmt.Errorf("CopyAction 的 source 不能为空")
		}
		if a.Destination == "" {
			return fmt.Errorf("CopyAction 的 destination 不能为空")
		}
	case *action.MoveAction:
		if a.Source == "" {
			return fmt.Errorf("MoveAction 的 source 不能为空")
		}
		if a.Destination == "" {
			return fmt.Errorf("MoveAction 的 destination 不能为空")
		}
	case *action.DeleteAction:
		if a.Path == "" {
			return fmt.Errorf("DeleteAction 的 path 不能为空")
		}
	case *action.CommandAction:
		if a.Command == "" {
			return fmt.Errorf("CommandAction 的 command 不能为空")
		}
	case *action.NotifyAction:
		if a.Channel == "" {
			return fmt.Errorf("NotifyAction 的 channel 不能为空")
		}
	case *action.WebhookAction:
		if a.URL == "" {
			return fmt.Errorf("WebhookAction 的 url 不能为空")
		}
	case *action.EmailAction:
		if a.To == "" {
			return fmt.Errorf("EmailAction 的 to 不能为空")
		}
	case *action.ConvertAction:
		if a.Source == "" {
			return fmt.Errorf("ConvertAction 的 source 不能为空")
		}
		if a.Format == "" {
			return fmt.Errorf("ConvertAction 的 format 不能为空")
		}
	}
	return nil
}

// isValidVersion 检查版本号格式.
func isValidVersion(version string) bool {
	pattern := `^\d+\.\d+\.\d+(-[a-zA-Z0-9]+)?$`
	matched, err := regexp.MatchString(pattern, version)
	if err != nil {
		return false
	}
	return matched
}

// isValidCron 检查 cron 表达式格式.
func isValidCron(schedule string) bool {
	// 支持标准 5 字段和扩展 6 字段 cron
	parts := strings.Fields(schedule)
	if len(parts) < 5 || len(parts) > 6 {
		return false
	}

	// 简单验证每个字段的字符
	for _, part := range parts {
		// 允许的字符: 数字, *, /, -, ,
		validChars := regexp.MustCompile(`^[0-9*/\-,\?LW]+$`)
		if !validChars.MatchString(part) {
			return false
		}
	}

	return true
}

// ==================== 模板导入/导出功能 ====================

// ExportTemplate 导出模板为 JSON.
func ExportTemplate(tpl *Template) ([]byte, error) {
	// 验证模板
	result := ValidateTemplate(tpl)
	if !result.Valid {
		return nil, fmt.Errorf("模板验证失败: %s", strings.Join(result.Errors, "; "))
	}

	return json.MarshalIndent(tpl, "", "  ")
}

// ImportTemplate 从 JSON 导入模板.
func ImportTemplate(data []byte) (*Template, error) {
	var tpl Template
	if err := json.Unmarshal(data, &tpl); err != nil {
		return nil, fmt.Errorf("解析模板失败: %w", err)
	}

	// 验证模板
	result := ValidateTemplate(&tpl)
	if !result.Valid {
		return nil, fmt.Errorf("模板验证失败: %s", strings.Join(result.Errors, "; "))
	}

	return &tpl, nil
}

// ExportTemplateToFile 导出模板到文件.
func ExportTemplateToFile(tpl *Template, filePath string) error {
	data, err := ExportTemplate(tpl)
	if err != nil {
		return err
	}

	// 确保目录存在
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	return os.WriteFile(filePath, data, 0640)
}

// ImportTemplateFromFile 从文件导入模板.
func ImportTemplateFromFile(filePath string) (*Template, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	return ImportTemplate(data)
}

// ExportAllTemplates 导出所有模板到目录.
func ExportAllTemplates(outputDir string) error {
	templates := GetTemplates()

	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	for _, tpl := range templates {
		fileName := fmt.Sprintf("%s.json", tpl.ID)
		filePath := filepath.Join(outputDir, fileName)

		if err := ExportTemplateToFile(&tpl, filePath); err != nil {
			return fmt.Errorf("导出模板 %s 失败: %w", tpl.ID, err)
		}
	}

	return nil
}

// ==================== 模板管理器方法 ====================

// GetAllTemplates 获取所有模板（预置 + 自定义）.
func (tm *TemplateManager) GetAllTemplates() []Template {
	result := GetBuiltInTemplates()

	for _, tpl := range tm.customTemplates {
		result = append(result, *tpl)
	}

	return result
}

// GetTemplateByID 按 ID 获取模板.
func (tm *TemplateManager) GetTemplateByID(id string) (*Template, error) {
	// 先查找自定义模板
	if tpl, ok := tm.customTemplates[id]; ok {
		return tpl, nil
	}

	// 再查找预置模板
	return GetTemplate(id)
}

// AddCustomTemplate 添加自定义模板.
func (tm *TemplateManager) AddCustomTemplate(tpl *Template) error {
	// 验证模板
	result := ValidateTemplate(tpl)
	if !result.Valid {
		return fmt.Errorf("模板验证失败: %s", strings.Join(result.Errors, "; "))
	}

	// 检查 ID 是否已存在
	for _, builtIn := range GetBuiltInTemplates() {
		if builtIn.ID == tpl.ID {
			return fmt.Errorf("模板 ID %s 已被预置模板使用", tpl.ID)
		}
	}

	if _, exists := tm.customTemplates[tpl.ID]; exists {
		return fmt.Errorf("自定义模板 ID %s 已存在", tpl.ID)
	}

	// 保存到内存
	tm.customTemplates[tpl.ID] = tpl

	// 持久化到存储
	if tm.storagePath != "" {
		filePath := filepath.Join(tm.storagePath, tpl.ID+".json")
		if err := ExportTemplateToFile(tpl, filePath); err != nil {
			delete(tm.customTemplates, tpl.ID)
			return fmt.Errorf("保存模板失败: %w", err)
		}
	}

	return nil
}

// UpdateCustomTemplate 更新自定义模板.
func (tm *TemplateManager) UpdateCustomTemplate(tpl *Template) error {
	// 验证模板
	result := ValidateTemplate(tpl)
	if !result.Valid {
		return fmt.Errorf("模板验证失败: %s", strings.Join(result.Errors, "; "))
	}

	// 检查是否是预置模板
	for _, builtIn := range GetBuiltInTemplates() {
		if builtIn.ID == tpl.ID {
			return fmt.Errorf("不能修改预置模板 %s", tpl.ID)
		}
	}

	// 检查自定义模板是否存在
	if _, exists := tm.customTemplates[tpl.ID]; !exists {
		return fmt.Errorf("自定义模板 %s 不存在", tpl.ID)
	}

	// 更新内存
	tm.customTemplates[tpl.ID] = tpl

	// 更新存储
	if tm.storagePath != "" {
		filePath := filepath.Join(tm.storagePath, tpl.ID+".json")
		if err := ExportTemplateToFile(tpl, filePath); err != nil {
			return fmt.Errorf("更新模板失败: %w", err)
		}
	}

	return nil
}

// DeleteCustomTemplate 删除自定义模板.
func (tm *TemplateManager) DeleteCustomTemplate(id string) error {
	// 检查是否是预置模板
	for _, builtIn := range GetBuiltInTemplates() {
		if builtIn.ID == id {
			return fmt.Errorf("不能删除预置模板 %s", id)
		}
	}

	// 检查自定义模板是否存在
	if _, exists := tm.customTemplates[id]; !exists {
		return fmt.Errorf("自定义模板 %s 不存在", id)
	}

	// 从内存删除
	delete(tm.customTemplates, id)

	// 从存储删除
	if tm.storagePath != "" {
		filePath := filepath.Join(tm.storagePath, id+".json")
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("删除模板文件失败: %w", err)
		}
	}

	return nil
}

// CreateWorkflowFromTemplate 从模板创建工作流.
func CreateWorkflowFromTemplate(tpl *Template, params map[string]string) *engine.Workflow {
	wf := &engine.Workflow{
		Name:         tpl.Workflow.Name,
		Description:  tpl.Workflow.Description,
		Enabled:      tpl.Workflow.Enabled,
		Trigger:      tpl.Workflow.Trigger,
		Actions:      tpl.Workflow.Actions,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		RunCount:     0,
		SuccessCount: 0,
		FailCount:    0,
	}

	// 应用参数替换
	applyTemplateParams(wf, params)

	return wf
}

// applyTemplateParams 应用模板参数替换.
func applyTemplateParams(wf *engine.Workflow, params map[string]string) {
	// 替换触发器中的参数
	if trig, ok := wf.Trigger.(*trigger.TimeTrigger); ok {
		trig.Timezone = replaceParams(trig.Timezone, params)
	} else if trig, ok := wf.Trigger.(*trigger.FileTrigger); ok {
		trig.Path = replaceParams(trig.Path, params)
		trig.Pattern = replaceParams(trig.Pattern, params)
	} else if trig, ok := wf.Trigger.(*trigger.WebhookTrigger); ok {
		trig.Path = replaceParams(trig.Path, params)
	}

	// 替换动作中的参数
	for _, act := range wf.Actions {
		switch a := act.(type) {
		case *action.CopyAction:
			a.Source = replaceParams(a.Source, params)
			a.Destination = replaceParams(a.Destination, params)
		case *action.MoveAction:
			a.Source = replaceParams(a.Source, params)
			a.Destination = replaceParams(a.Destination, params)
		case *action.DeleteAction:
			a.Path = replaceParams(a.Path, params)
		case *action.CommandAction:
			a.Command = replaceParams(a.Command, params)
			for i, arg := range a.Args {
				a.Args[i] = replaceParams(arg, params)
			}
			a.WorkDir = replaceParams(a.WorkDir, params)
		case *action.NotifyAction:
			a.Message = replaceParams(a.Message, params)
			a.Title = replaceParams(a.Title, params)
			a.Channel = replaceParams(a.Channel, params)
		case *action.ConvertAction:
			a.Source = replaceParams(a.Source, params)
			a.Destination = replaceParams(a.Destination, params)
		case *action.WebhookAction:
			a.URL = replaceParams(a.URL, params)
			a.Body = replaceParams(a.Body, params)
		case *action.EmailAction:
			a.To = replaceParams(a.To, params)
			a.Subject = replaceParams(a.Subject, params)
			a.Body = replaceParams(a.Body, params)
		}
	}
}

// replaceParams 替换参数.
func replaceParams(s string, params map[string]string) string {
	if s == "" || params == nil {
		return s
	}

	result := s
	for key, value := range params {
		placeholder := "{{" + key + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result
}

// GetTemplateParams 获取模板中的参数占位符.
func GetTemplateParams(tpl *Template) []string {
	params := make(map[string]bool)

	// 从工作流中提取参数
	extractParamsFromWorkflow(&tpl.Workflow, params)

	// 转换为列表
	result := []string{}
	for param := range params {
		result = append(result, param)
	}

	return result
}

// extractParamsFromWorkflow 从工作流中提取参数.
func extractParamsFromWorkflow(wf *engine.Workflow, params map[string]bool) {
	// 提取参数的正则表达式
	re := regexp.MustCompile(`\{\{([a-zA-Z_][a-zA-Z0-9_]*)\}\}`)

	extract := func(s string) {
		matches := re.FindAllStringSubmatch(s, -1)
		for _, match := range matches {
			// 排除内置变量
			builtin := map[string]bool{
				"timestamp": true, "date": true, "time": true,
				"datetime": true, "unix": true,
				"event": true, // event.path, event.filename 等
			}
			if !builtin[match[1]] {
				params[match[1]] = true
			}
		}
	}

	// 从触发器提取
	if trig, ok := wf.Trigger.(*trigger.TimeTrigger); ok {
		extract(trig.Timezone)
	} else if trig, ok := wf.Trigger.(*trigger.FileTrigger); ok {
		extract(trig.Path)
		extract(trig.Pattern)
	} else if trig, ok := wf.Trigger.(*trigger.WebhookTrigger); ok {
		extract(trig.Path)
	}

	// 从动作提取
	for _, act := range wf.Actions {
		switch a := act.(type) {
		case *action.CopyAction:
			extract(a.Source)
			extract(a.Destination)
		case *action.MoveAction:
			extract(a.Source)
			extract(a.Destination)
		case *action.DeleteAction:
			extract(a.Path)
		case *action.CommandAction:
			extract(a.Command)
			for _, arg := range a.Args {
				extract(arg)
			}
			extract(a.WorkDir)
		case *action.NotifyAction:
			extract(a.Message)
			extract(a.Title)
			extract(a.Channel)
		case *action.ConvertAction:
			extract(a.Source)
			extract(a.Destination)
		case *action.WebhookAction:
			extract(a.URL)
			extract(a.Body)
		case *action.EmailAction:
			extract(a.To)
			extract(a.Subject)
			extract(a.Body)
		}
	}
}
