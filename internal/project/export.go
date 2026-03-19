// Package project provides project export/import functionality
package project

import (
	"encoding/json"
	"errors"
	"time"
)

// ExportFormat 导出格式
type ExportFormat string

// 导出格式常量
const (
	ExportFormatJSON ExportFormat = "json"
)

// Export 项目导出数据
type Export struct {
	// 元数据
	ExportVersion string    `json:"export_version"`
	ExportedAt    time.Time `json:"exported_at"`
	ExportedBy    string    `json:"exported_by"`

	// 项目信息
	Project *Project `json:"project"`

	// 里程碑
	Milestones []*Milestone `json:"milestones,omitempty"`

	// 任务
	Tasks []*Task `json:"tasks,omitempty"`

	// 评论
	Comments map[string][]*TaskComment `json:"comments,omitempty"`

	// 历史记录
	History map[string][]*TaskHistory `json:"history,omitempty"`

	// 导出选项
	Options ExportOptions `json:"options"`
}

// 注意：已移除 ProjectExport 别名，请直接使用 Export 类型
// 原因：避免 stutter (project.ProjectExport -> project.Export)

// ExportOptions 导出选项
type ExportOptions struct {
	IncludeComments  bool `json:"include_comments"`
	IncludeHistory   bool `json:"include_history"`
	IncludeCompleted bool `json:"include_completed"`
	IncludeArchived  bool `json:"include_archived"`
	AnonymizeUsers   bool `json:"anonymize_users"`
	IncludeSensitive bool `json:"include_sensitive"`
}

// ImportOptions 导入选项
type ImportOptions struct {
	OverwriteExisting bool              `json:"overwrite_existing"`
	CreateNewProject  bool              `json:"create_new_project"`
	NewProjectName    string            `json:"new_project_name"`
	NewProjectKey     string            `json:"new_project_key"`
	PreserveIDs       bool              `json:"preserve_ids"`
	SkipExistingTasks bool              `json:"skip_existing_tasks"`
	UpdateExisting    bool              `json:"update_existing"`
	ImportComments    bool              `json:"import_comments"`
	ImportHistory     bool              `json:"import_history"`
	DefaultOwnerID    string            `json:"default_owner_id"`
	MapUsers          bool              `json:"map_users"`
	UserMapping       map[string]string `json:"user_mapping,omitempty"`
}

// ImportResult 导入结果
type ImportResult struct {
	ProjectID          string   `json:"project_id"`
	ImportedTasks      int      `json:"imported_tasks"`
	ImportedMilestones int      `json:"imported_milestones"`
	ImportedComments   int      `json:"imported_comments"`
	SkippedTasks       int      `json:"skipped_tasks"`
	Errors             []string `json:"errors,omitempty"`
	Warnings           []string `json:"warnings,omitempty"`
}

// ExportManager 导出管理器
type ExportManager struct {
	manager *Manager
}

// NewExportManager 创建导出管理器
func NewExportManager(mgr *Manager) *ExportManager {
	return &ExportManager{
		manager: mgr,
	}
}

// ExportProject 导出项目
func (em *ExportManager) ExportProject(projectID, exportedBy string, options ExportOptions) (*Export, error) {
	project, err := em.manager.GetProject(projectID)
	if err != nil {
		return nil, err
	}

	// 检查是否导出已完成/归档项目
	if !options.IncludeCompleted && project.Status == "completed" {
		return nil, errors.New("项目已完成，如需导出请设置 IncludeCompleted 选项")
	}
	if !options.IncludeArchived && project.Status == "archived" {
		return nil, errors.New("项目已归档，如需导出请设置 IncludeArchived 选项")
	}

	export := &Export{
		ExportVersion: "1.0",
		ExportedAt:    time.Now(),
		ExportedBy:    exportedBy,
		Project:       project,
		Options:       options,
	}

	// 导出里程碑
	export.Milestones = em.manager.ListMilestones(projectID)

	// 导出任务
	filter := TaskFilter{
		ProjectID: projectID,
		Limit:     10000,
	}
	export.Tasks = em.manager.ListTasks(filter)

	// 导出评论
	if options.IncludeComments {
		export.Comments = make(map[string][]*TaskComment)
		for _, task := range export.Tasks {
			export.Comments[task.ID] = em.manager.GetComments(task.ID)
		}
	}

	// 导出历史
	if options.IncludeHistory {
		export.History = make(map[string][]*TaskHistory)
		for _, task := range export.Tasks {
			export.History[task.ID] = em.manager.GetHistory(task.ID)
		}
	}

	// 匿名化用户
	if options.AnonymizeUsers {
		em.anonymizeExport(export)
	}

	return export, nil
}

// anonymizeExport 匿名化导出数据
func (em *ExportManager) anonymizeExport(export *Export) {
	// 匿名化项目信息
	export.Project.OwnerID = "user_1"
	export.Project.MemberIDs = []string{}
	export.Project.CreatedBy = "user_1"

	// 匿名化任务
	userMap := make(map[string]string)
	userCounter := 1

	for _, task := range export.Tasks {
		if task.AssigneeID != "" {
			if _, exists := userMap[task.AssigneeID]; !exists {
				userCounter++
				userMap[task.AssigneeID] = "user_" + string(rune('0'+userCounter))
			}
			task.AssigneeID = userMap[task.AssigneeID]
		}
		task.ReporterID = "user_1"
		task.CreatedBy = "user_1"
	}

	// 匿名化里程碑
	for _, ms := range export.Milestones {
		ms.CreatedBy = "user_1"
	}

	// 匿名化评论
	for taskID, comments := range export.Comments {
		for _, comment := range comments {
			comment.UserID = "user_1"
		}
		export.Comments[taskID] = comments
	}

	// 匿名化历史
	for taskID, history := range export.History {
		for _, h := range history {
			h.UserID = "user_1"
		}
		export.History[taskID] = history
	}

	// 移除敏感信息
	export.ExportedBy = "user_1"
}

// ExportToJSON 导出为JSON
func (em *ExportManager) ExportToJSON(projectID, exportedBy string, options ExportOptions) ([]byte, error) {
	export, err := em.ExportProject(projectID, exportedBy, options)
	if err != nil {
		return nil, err
	}

	return json.MarshalIndent(export, "", "  ")
}

// ImportProject 导入项目
func (em *ExportManager) ImportProject(data []byte, options ImportOptions) (*ImportResult, error) {
	var export Export
	if err := json.Unmarshal(data, &export); err != nil {
		return nil, err
	}

	result := &ImportResult{
		Errors:   make([]string, 0),
		Warnings: make([]string, 0),
	}

	// 创建或使用现有项目
	var project *Project
	var err error

	if options.CreateNewProject || options.NewProjectName != "" {
		name := options.NewProjectName
		if name == "" {
			name = export.Project.Name + " (导入)"
		}
		key := options.NewProjectKey
		if key == "" {
			key = export.Project.Key + "-imp"
		}
		ownerID := options.DefaultOwnerID
		if ownerID == "" {
			ownerID = export.Project.OwnerID
		}

		project, err = em.manager.CreateProject(name, key, export.Project.Description, ownerID, ownerID)
		if err != nil {
			return nil, err
		}
		result.ProjectID = project.ID
	} else {
		project, err = em.manager.GetProject(export.Project.ID)
		if err != nil {
			// 项目不存在，创建新项目
			ownerID := options.DefaultOwnerID
			if ownerID == "" {
				ownerID = export.Project.OwnerID
			}
			project, err = em.manager.CreateProject(
				export.Project.Name,
				export.Project.Key,
				export.Project.Description,
				ownerID,
				ownerID,
			)
			if err != nil {
				return nil, err
			}
		}
		result.ProjectID = project.ID
	}

	// 导入里程碑
	milestoneIDMap := make(map[string]string)
	for _, ms := range export.Milestones {
		newMs, err := em.manager.CreateMilestone(ms.Name, ms.Description, project.ID, ms.CreatedBy, ms.DueDate)
		if err != nil {
			result.Warnings = append(result.Warnings, "无法导入里程碑: "+ms.Name)
			continue
		}
		milestoneIDMap[ms.ID] = newMs.ID
		result.ImportedMilestones++
	}

	// 导入任务
	for _, task := range export.Tasks {
		// 用户映射
		assigneeID := task.AssigneeID
		if options.MapUsers && options.UserMapping != nil {
			if mapped, ok := options.UserMapping[task.AssigneeID]; ok {
				assigneeID = mapped
			}
		}

		reporterID := task.ReporterID
		if options.MapUsers && options.UserMapping != nil {
			if mapped, ok := options.UserMapping[task.ReporterID]; ok {
				reporterID = mapped
			}
		}

		newTask, err := em.manager.CreateTask(task.Title, task.Description, project.ID, reporterID, task.Priority)
		if err != nil {
			result.Warnings = append(result.Warnings, "无法导入任务: "+task.Title)
			result.SkippedTasks++
			continue
		}

		// 更新任务属性
		updates := map[string]interface{}{
			"tags":            task.Tags,
			"labels":          task.Labels,
			"estimated_hours": task.EstimatedHours,
			"actual_hours":    task.ActualHours,
			"progress":        task.Progress,
		}

		if assigneeID != "" {
			updates["assignee_id"] = assigneeID
		}

		// 映射里程碑
		if task.MilestoneID != "" {
			if newMsID, ok := milestoneIDMap[task.MilestoneID]; ok {
				updates["milestone_id"] = newMsID
			}
		}

		// 设置截止日期
		if task.DueDate != nil {
			updates["due_date"] = task.DueDate
		}

		// 设置状态
		if task.Status != TaskStatusTodo {
			updates["status"] = task.Status
		}

		_, _ = em.manager.UpdateTask(newTask.ID, reporterID, updates)

		// 导入评论
		if options.ImportComments && export.Comments != nil {
			comments := export.Comments[task.ID]
			for _, comment := range comments {
				userID := comment.UserID
				if options.MapUsers && options.UserMapping != nil {
					if mapped, ok := options.UserMapping[comment.UserID]; ok {
						userID = mapped
					}
				}
				_, _ = em.manager.AddComment(newTask.ID, userID, comment.Content)
				result.ImportedComments++
			}
		}

		result.ImportedTasks++
	}

	return result, nil
}

// ValidateImportData 验证导入数据
func (em *ExportManager) ValidateImportData(data []byte) (*Export, []string, error) {
	var export Export
	if err := json.Unmarshal(data, &export); err != nil {
		return nil, nil, err
	}

	warnings := make([]string, 0)

	// 检查导出版本
	if export.ExportVersion == "" {
		warnings = append(warnings, "缺少导出版本信息")
	}

	// 检查项目信息
	if export.Project == nil {
		return nil, nil, errors.New("缺少项目信息")
	}

	if export.Project.Name == "" {
		warnings = append(warnings, "项目名称为空")
	}

	// 检查任务
	if len(export.Tasks) == 0 {
		warnings = append(warnings, "没有任务数据")
	}

	// 检查里程碑引用
	milestoneIDs := make(map[string]bool)
	for _, ms := range export.Milestones {
		milestoneIDs[ms.ID] = true
	}
	for _, task := range export.Tasks {
		if task.MilestoneID != "" && !milestoneIDs[task.MilestoneID] {
			warnings = append(warnings, "任务 "+task.Title+" 引用了不存在的里程碑")
		}
	}

	return &export, warnings, nil
}

// GetExportSummary 获取导出摘要
func (em *ExportManager) GetExportSummary(projectID string) (*ExportSummary, error) {
	project, err := em.manager.GetProject(projectID)
	if err != nil {
		return nil, err
	}

	milestones := em.manager.ListMilestones(projectID)
	filter := TaskFilter{ProjectID: projectID, Limit: 10000}
	tasks := em.manager.ListTasks(filter)

	summary := &ExportSummary{
		ProjectID:         projectID,
		ProjectName:       project.Name,
		MilestoneCount:    len(milestones),
		TaskCount:         len(tasks),
		EstimatedDataSize: em.estimateSize(project, milestones, tasks),
	}

	// 统计任务状态
	summary.TasksByStatus = make(map[string]int)
	for _, task := range tasks {
		summary.TasksByStatus[string(task.Status)]++
	}

	// 统计评论
	totalComments := 0
	for _, task := range tasks {
		comments := em.manager.GetComments(task.ID)
		totalComments += len(comments)
	}
	summary.CommentCount = totalComments

	return summary, nil
}

// ExportSummary 导出摘要
type ExportSummary struct {
	ProjectID         string         `json:"project_id"`
	ProjectName       string         `json:"project_name"`
	MilestoneCount    int            `json:"milestone_count"`
	TaskCount         int            `json:"task_count"`
	CommentCount      int            `json:"comment_count"`
	TasksByStatus     map[string]int `json:"tasks_by_status"`
	EstimatedDataSize int64          `json:"estimated_data_size"` // 字节
}

// estimateSize 估算导出数据大小
func (em *ExportManager) estimateSize(project *Project, milestones []*Milestone, tasks []*Task) int64 {
	// 粗略估算
	baseSize := 500 // 基础JSON结构
	projectSize := 500
	milestoneSize := 300 * len(milestones)
	taskSize := 800 * len(tasks)

	return int64(baseSize + projectSize + milestoneSize + taskSize)
}

// BatchExport 批量导出
func (em *ExportManager) BatchExport(projectIDs []string, exportedBy string, options ExportOptions) (map[string]*Export, []error) {
	results := make(map[string]*Export)
	errors := make([]error, 0)

	for _, projectID := range projectIDs {
		export, err := em.ExportProject(projectID, exportedBy, options)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		results[projectID] = export
	}

	return results, errors
}

// MergeProjects 合并多个项目（导入到一个新项目）
func (em *ExportManager) MergeProjects(projectIDs []string, newName, newKey, ownerID string, options ExportOptions) (*Project, *ImportResult, error) {
	// 创建新项目
	project, err := em.manager.CreateProject(newName, newKey, "合并项目", ownerID, ownerID)
	if err != nil {
		return nil, nil, err
	}

	result := &ImportResult{
		ProjectID: project.ID,
		Errors:    make([]string, 0),
		Warnings:  make([]string, 0),
	}

	// 导入各项目数据
	for _, srcID := range projectIDs {
		export, err := em.ExportProject(srcID, ownerID, options)
		if err != nil {
			result.Errors = append(result.Errors, "无法导出项目 "+srcID)
			continue
		}

		// 导入里程碑
		for _, ms := range export.Milestones {
			_, err := em.manager.CreateMilestone(ms.Name+" (来自"+export.Project.Name+")", ms.Description, project.ID, ownerID, ms.DueDate)
			if err != nil {
				continue
			}
			result.ImportedMilestones++
		}

		// 导入任务
		for _, task := range export.Tasks {
			_, err := em.manager.CreateTask(task.Title, task.Description, project.ID, ownerID, task.Priority)
			if err != nil {
				result.SkippedTasks++
				continue
			}
			result.ImportedTasks++
		}
	}

	return project, result, nil
}
