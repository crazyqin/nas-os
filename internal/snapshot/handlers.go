package snapshot

import (
	"time"

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handlers 快照策略 API 处理器
type Handlers struct {
	policyManager *PolicyManager
}

// NewHandlers 创建处理器
func NewHandlers(pm *PolicyManager) *Handlers {
	return &Handlers{
		policyManager: pm,
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	snapshots := r.Group("/snapshots")
	{
		// 策略管理
		snapshots.GET("/policies", h.listPolicies)
		snapshots.POST("/policies", h.createPolicy)
		snapshots.GET("/policies/:id", h.getPolicy)
		snapshots.PUT("/policies/:id", h.updatePolicy)
		snapshots.DELETE("/policies/:id", h.deletePolicy)
		snapshots.POST("/policies/:id/enable", h.enablePolicy)
		snapshots.POST("/policies/:id/disable", h.disablePolicy)
		snapshots.POST("/policies/:id/execute", h.executePolicy)

		// 调度信息
		snapshots.GET("/policies/:id/schedule", h.getScheduleInfo)
		snapshots.GET("/schedules", h.listSchedules)

		// 清理预览
		snapshots.POST("/policies/:id/cleanup-preview", h.cleanupPreview)
	}
}

// ========== 策略管理 ==========

// listPolicies 列出所有策略
// @Summary 列出快照策略
// @Description 获取所有快照策略列表，支持按卷名、类型、启用状态过滤
// @Tags snapshot
// @Accept json
// @Produce json
// @Param volume query string false "卷名"
// @Param type query string false "策略类型"
// @Param enabled query string false "是否启用 (true/false)"
// @Success 200 {object} api.Response{data=[]Policy}
// @Router /snapshots/policies [get]
// @Security BearerAuth
func (h *Handlers) listPolicies(c *gin.Context) {
	volumeName := c.Query("volume")
	policyType := c.Query("type")
	enabled := c.Query("enabled")

	policies := h.policyManager.ListPolicies()

	// 过滤
	var filtered []*Policy
	for _, p := range policies {
		if volumeName != "" && p.VolumeName != volumeName {
			continue
		}
		if policyType != "" && string(p.Type) != policyType {
			continue
		}
		if enabled != "" {
			isEnabled := enabled == "true"
			if p.Enabled != isEnabled {
				continue
			}
		}
		filtered = append(filtered, p)
	}

	api.OK(c, filtered)
}

// getPolicy 获取单个策略
// @Summary 获取快照策略详情
// @Description 获取指定快照策略的详细信息
// @Tags snapshot
// @Accept json
// @Produce json
// @Param id path string true "策略 ID"
// @Success 200 {object} api.Response{data=Policy}
// @Failure 404 {object} api.Response
// @Router /snapshots/policies/{id} [get]
// @Security BearerAuth
func (h *Handlers) getPolicy(c *gin.Context) {
	id := c.Param("id")

	policy, err := h.policyManager.GetPolicy(id)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, policy)
}

// createPolicy 创建策略
// @Summary 创建快照策略
// @Description 创建新的快照策略
// @Tags snapshot
// @Accept json
// @Produce json
// @Param policy body Policy true "快照策略配置"
// @Success 200 {object} api.Response{data=Policy}
// @Failure 400 {object} api.Response
// @Router /snapshots/policies [post]
// @Security BearerAuth
func (h *Handlers) createPolicy(c *gin.Context) {
	var policy Policy
	if err := c.ShouldBindJSON(&policy); err != nil {
		api.BadRequest(c, "无效的请求数据: "+err.Error())
		return
	}

	if err := h.policyManager.CreatePolicy(&policy); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "策略创建成功", policy)
}

// updatePolicy 更新策略
// @Summary 更新快照策略
// @Description 更新指定的快照策略配置
// @Tags snapshot
// @Accept json
// @Produce json
// @Param id path string true "策略 ID"
// @Param policy body Policy true "快照策略配置"
// @Success 200 {object} api.Response{data=Policy}
// @Failure 400 {object} api.Response
// @Router /snapshots/policies/{id} [put]
// @Security BearerAuth
func (h *Handlers) updatePolicy(c *gin.Context) {
	id := c.Param("id")

	var policy Policy
	if err := c.ShouldBindJSON(&policy); err != nil {
		api.BadRequest(c, "无效的请求数据: "+err.Error())
		return
	}

	if err := h.policyManager.UpdatePolicy(id, &policy); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, policy)
}

// deletePolicy 删除策略
// @Summary 删除快照策略
// @Description 删除指定的快照策略
// @Tags snapshot
// @Accept json
// @Produce json
// @Param id path string true "策略 ID"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Router /snapshots/policies/{id} [delete]
// @Security BearerAuth
func (h *Handlers) deletePolicy(c *gin.Context) {
	id := c.Param("id")

	if err := h.policyManager.DeletePolicy(id); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "策略已删除", nil)
}

// enablePolicy 启用策略
// @Summary 启用快照策略
// @Description 启用指定的快照策略
// @Tags snapshot
// @Accept json
// @Produce json
// @Param id path string true "策略 ID"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Router /snapshots/policies/{id}/enable [post]
// @Security BearerAuth
func (h *Handlers) enablePolicy(c *gin.Context) {
	id := c.Param("id")

	if err := h.policyManager.EnablePolicy(id, true); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "策略已启用", nil)
}

// disablePolicy 禁用策略
// @Summary 禁用快照策略
// @Description 禁用指定的快照策略
// @Tags snapshot
// @Accept json
// @Produce json
// @Param id path string true "策略 ID"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Router /snapshots/policies/{id}/disable [post]
// @Security BearerAuth
func (h *Handlers) disablePolicy(c *gin.Context) {
	id := c.Param("id")

	if err := h.policyManager.EnablePolicy(id, false); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "策略已禁用", nil)
}

// executePolicy 手动执行策略
// @Summary 手动执行快照策略
// @Description 立即执行指定的快照策略，创建快照
// @Tags snapshot
// @Accept json
// @Produce json
// @Param id path string true "策略 ID"
// @Success 200 {object} api.Response{data=map[string]interface{}}
// @Failure 500 {object} api.Response
// @Router /snapshots/policies/{id}/execute [post]
// @Security BearerAuth
func (h *Handlers) executePolicy(c *gin.Context) {
	id := c.Param("id")

	snapshotName, err := h.policyManager.ExecutePolicy(id)
	if err != nil {
		api.InternalError(c, "执行失败: "+err.Error())
		return
	}

	api.OK(c, gin.H{
		"snapshotName": snapshotName,
		"executedAt":   time.Now(),
	})
}

// ========== 调度信息 ==========

// getScheduleInfo 获取策略调度信息
func (h *Handlers) getScheduleInfo(c *gin.Context) {
	id := c.Param("id")

	jobInfo, err := h.policyManager.GetScheduler().GetJobStatus(id)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, jobInfo)
}

// listSchedules 列出所有调度任务
func (h *Handlers) listSchedules(c *gin.Context) {
	jobs := h.policyManager.GetScheduler().ListJobs()
	api.OK(c, jobs)
}

// ========== 清理预览 ==========

// cleanupPreview 清理预览
func (h *Handlers) cleanupPreview(c *gin.Context) {
	id := c.Param("id")

	policy, err := h.policyManager.GetPolicy(id)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	preview, err := h.policyManager.GetCleaner().PreviewDryRun(policy)
	if err != nil {
		api.InternalError(c, "预览失败: "+err.Error())
		return
	}

	api.OK(c, preview)
}

// ========== 辅助端点 ==========

// GetPolicyStats 获取策略统计
func (h *Handlers) GetPolicyStats(c *gin.Context) {
	id := c.Param("id")

	policy, err := h.policyManager.GetPolicy(id)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	stats := gin.H{
		"policyId":              policy.ID,
		"policyName":            policy.Name,
		"type":                  policy.Type,
		"enabled":               policy.Enabled,
		"totalRuns":             policy.Stats.TotalRuns,
		"successfulRuns":        policy.Stats.SuccessfulRuns,
		"failedRuns":            policy.Stats.FailedRuns,
		"totalSnapshotsCreated": policy.Stats.TotalSnapshotsCreated,
		"totalSnapshotsDeleted": policy.Stats.TotalSnapshotsDeleted,
		"totalBytesSaved":       policy.Stats.TotalBytesSaved,
	}

	if policy.Stats.LastSuccessAt != nil {
		stats["lastSuccessAt"] = policy.Stats.LastSuccessAt
	}
	if policy.Stats.LastFailureAt != nil {
		stats["lastFailureAt"] = policy.Stats.LastFailureAt
	}
	if policy.LastError != "" {
		stats["lastError"] = policy.LastError
	}

	// 计算成功率
	if policy.Stats.TotalRuns > 0 {
		stats["successRate"] = float64(policy.Stats.SuccessfulRuns) / float64(policy.Stats.TotalRuns) * 100
	}

	api.OK(c, stats)
}

// ValidateCron 验证 cron 表达式
func (h *Handlers) ValidateCron(c *gin.Context) {
	var req struct {
		Expression string `json:"expression" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "缺少 cron 表达式")
		return
	}

	valid := h.policyManager.GetScheduler().ValidateCron(req.Expression)

	api.OK(c, gin.H{
		"expression": req.Expression,
		"valid":      valid,
	})
}

// GetPresetPolicies 获取预设策略模板
func (h *Handlers) GetPresetPolicies(c *gin.Context) {
	presets := []PolicyTemplate{
		{
			Name:        "每小时快照",
			Description: "每小时自动创建快照，保留最近 24 个",
			Type:        PolicyTypeScheduled,
			Schedule:    &ScheduleConfig{Type: ScheduleTypeHourly, Minute: 0},
			Retention:   &RetentionPolicy{Type: RetentionByCount, MaxCount: 24},
			ReadOnly:    true,
		},
		{
			Name:        "每日快照",
			Description: "每天凌晨 2 点创建快照，保留最近 7 天",
			Type:        PolicyTypeScheduled,
			Schedule:    &ScheduleConfig{Type: ScheduleTypeDaily, Hour: 2, Minute: 0},
			Retention:   &RetentionPolicy{Type: RetentionByAge, MaxAgeDays: 7},
			ReadOnly:    true,
		},
		{
			Name:        "每周快照",
			Description: "每周日凌晨 3 点创建快照，保留最近 4 周",
			Type:        PolicyTypeScheduled,
			Schedule:    &ScheduleConfig{Type: ScheduleTypeWeekly, DayOfWeek: 0, Hour: 3, Minute: 0},
			Retention:   &RetentionPolicy{Type: RetentionByCount, MaxCount: 4},
			ReadOnly:    true,
		},
		{
			Name:        "每月快照",
			Description: "每月 1 号凌晨 4 点创建快照，保留最近 12 个月",
			Type:        PolicyTypeScheduled,
			Schedule:    &ScheduleConfig{Type: ScheduleTypeMonthly, DayOfMonth: 1, Hour: 4, Minute: 0},
			Retention:   &RetentionPolicy{Type: RetentionByCount, MaxCount: 12},
			ReadOnly:    true,
		},
		{
			Name:        "数据库一致性快照",
			Description: "应用一致性快照，执行数据库刷盘脚本前后创建",
			Type:        PolicyTypeApplicationConsistent,
			Schedule:    &ScheduleConfig{Type: ScheduleTypeDaily, Hour: 1, Minute: 30},
			Retention:   &RetentionPolicy{Type: RetentionByAge, MaxAgeDays: 14},
			ReadOnly:    true,
			Scripts: &ScriptConfig{
				PreSnapshotScript:  "mysql -e 'FLUSH TABLES WITH READ LOCK; SYSTEM sync'",
				PostSnapshotScript: "mysql -e 'UNLOCK TABLES'",
				TimeoutSeconds:     60,
			},
		},
	}

	api.OK(c, presets)
}

// PolicyTemplate 策略模板
type PolicyTemplate struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Type        PolicyType       `json:"type"`
	Schedule    *ScheduleConfig  `json:"schedule,omitempty"`
	Retention   *RetentionPolicy `json:"retention"`
	ReadOnly    bool             `json:"readOnly"`
	Scripts     *ScriptConfig    `json:"scripts,omitempty"`
}
