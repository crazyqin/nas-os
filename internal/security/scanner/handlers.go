package scanner

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 安全扫描器 HTTP 处理器
type Handlers struct {
	filesystemScanner  *FilesystemScanner
	permissionChecker  *PermissionChecker
	vulnScanner        *VulnerabilityScanner
	scoreEngine        *ScoreEngine
}

// NewHandlers 创建安全扫描器处理器
func NewHandlers(
	fsScanner *FilesystemScanner,
	permChecker *PermissionChecker,
	vulnScanner *VulnerabilityScanner,
	scoreEngine *ScoreEngine,
) *Handlers {
	return &Handlers{
		filesystemScanner: fsScanner,
		permissionChecker: permChecker,
		vulnScanner:       vulnScanner,
		scoreEngine:       scoreEngine,
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	security := api.Group("/security/scanner")
	{
		// 文件系统扫描
		fs := security.Group("/filesystem")
		{
			fs.POST("/scan", h.createScanTask)
			fs.GET("/tasks", h.listScanTasks)
			fs.GET("/tasks/:task_id", h.getScanTask)
			fs.POST("/tasks/:task_id/start", h.startScan)
			fs.POST("/tasks/:task_id/cancel", h.cancelScan)
			fs.GET("/tasks/:task_id/findings", h.getScanFindings)
			fs.GET("/tasks/:task_id/report", h.getScanReport)
		}

		// 权限检查
		perm := security.Group("/permissions")
		{
			perm.POST("/check", h.checkPermissions)
			perm.POST("/check/paths", h.checkPathsPermissions)
			perm.POST("/check/sensitive", h.checkSensitivePaths)
			perm.POST("/check/ssh", h.checkSSHSecurity)
			perm.POST("/check/home", h.checkUserHomeDirs)
			perm.POST("/check/system", h.checkSystemConfig)
			perm.GET("/rules", h.listPermissionRules)
			perm.POST("/rules", h.addPermissionRule)
			perm.DELETE("/rules/:rule_id", h.removePermissionRule)
			perm.POST("/fix", h.fixPermissions)
		}

		// 漏洞扫描
		vuln := security.Group("/vulnerabilities")
		{
			vuln.POST("/scan", h.scanVulnerability)
			vuln.POST("/scan/batch", h.scanVulnerabilitiesBatch)
			vuln.GET("/cve/:cve_id", h.getVulnerability)
			vuln.GET("/databases", h.listVulnDatabases)
			vuln.POST("/databases/sync", h.syncVulnDatabase)
		}

		// 安全评分
		score := security.Group("/score")
		{
			score.GET("/calculate", h.calculateScore)
			score.GET("/current", h.getCurrentScore)
			score.GET("/history", h.getScoreHistory)
			score.GET("/trend", h.getTrendAnalysis)
			score.GET("/report", h.getScoreReport)
			score.GET("/categories", h.getScoreCategories)
			score.PUT("/categories", h.updateScoreCategory)
		}

		// 敏感数据检测规则
		sensitive := security.Group("/sensitive-rules")
		{
			sensitive.GET("", h.listSensitiveRules)
			sensitive.POST("", h.addSensitiveRule)
			sensitive.PUT("/:rule_id", h.updateSensitiveRule)
			sensitive.DELETE("/:rule_id", h.deleteSensitiveRule)
		}

		// 仪表板
		security.GET("/dashboard", h.getDashboardData)
		security.GET("/statistics", h.getStatistics)
	}
}

// ========== 文件系统扫描处理器 ==========

// createScanTask 创建扫描任务
func (h *Handlers) createScanTask(c *gin.Context) {
	if h.filesystemScanner == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "文件系统扫描器未启用"))
		return
	}

	var req struct {
		Name        string      `json:"name" binding:"required"`
		Type        ScanType    `json:"type"`
		TargetPaths []string    `json:"target_paths" binding:"required"`
		ExcludePaths []string   `json:"exclude_paths"`
		Options     *ScanOptions `json:"options"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(400, err.Error()))
		return
	}

	if req.Type == "" {
		req.Type = ScanTypeQuick
	}

	task := h.filesystemScanner.CreateScanTask(req.Name, req.Type, req.TargetPaths, req.ExcludePaths, req.Options)
	c.JSON(http.StatusCreated, SuccessResponse(task))
}

// listScanTasks 列出扫描任务
func (h *Handlers) listScanTasks(c *gin.Context) {
	if h.filesystemScanner == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "文件系统扫描器未启用"))
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	tasks := h.filesystemScanner.ListScanTasks(limit)

	c.JSON(http.StatusOK, SuccessResponse(tasks))
}

// getScanTask 获取扫描任务详情
func (h *Handlers) getScanTask(c *gin.Context) {
	if h.filesystemScanner == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "文件系统扫描器未启用"))
		return
	}

	taskID := c.Param("task_id")
	task := h.filesystemScanner.GetScanTask(taskID)

	if task == nil {
		c.JSON(http.StatusNotFound, ErrorResponse(404, "扫描任务不存在"))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(task))
}

// startScan 启动扫描
func (h *Handlers) startScan(c *gin.Context) {
	if h.filesystemScanner == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "文件系统扫描器未启用"))
		return
	}

	taskID := c.Param("task_id")
	if err := h.filesystemScanner.StartScan(taskID); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(gin.H{"message": "扫描已启动"}))
}

// cancelScan 取消扫描
func (h *Handlers) cancelScan(c *gin.Context) {
	if h.filesystemScanner == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "文件系统扫描器未启用"))
		return
	}

	taskID := c.Param("task_id")
	if err := h.filesystemScanner.CancelScan(taskID); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(gin.H{"message": "扫描已取消"}))
}

// getScanFindings 获取扫描发现
func (h *Handlers) getScanFindings(c *gin.Context) {
	if h.filesystemScanner == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "文件系统扫描器未启用"))
		return
	}

	taskID := c.Param("task_id")
	findings := h.filesystemScanner.GetFindings(taskID)

	// 筛选
	severity := c.Query("severity")
	findingType := c.Query("type")

	filtered := make([]*FileFinding, 0)
	for _, f := range findings {
		if severity != "" && string(f.Severity) != severity {
			continue
		}
		if findingType != "" && string(f.Type) != findingType {
			continue
		}
		filtered = append(filtered, f)
	}

	c.JSON(http.StatusOK, SuccessResponse(filtered))
}

// getScanReport 获取扫描报告
func (h *Handlers) getScanReport(c *gin.Context) {
	if h.filesystemScanner == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "文件系统扫描器未启用"))
		return
	}

	taskID := c.Param("task_id")
	report := h.filesystemScanner.GetReport(taskID)

	if report == nil {
		c.JSON(http.StatusNotFound, ErrorResponse(404, "报告不存在"))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(report))
}

// ========== 权限检查处理器 ==========

// checkPermissions 检查权限
func (h *Handlers) checkPermissions(c *gin.Context) {
	if h.permissionChecker == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "权限检查器未启用"))
		return
	}

	var req struct {
		Path string `json:"path" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(400, err.Error()))
		return
	}

	result := h.permissionChecker.CheckPath(req.Path)
	c.JSON(http.StatusOK, SuccessResponse(result))
}

// checkPathsPermissions 批量检查路径权限
func (h *Handlers) checkPathsPermissions(c *gin.Context) {
	if h.permissionChecker == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "权限检查器未启用"))
		return
	}

	var req struct {
		Paths []string `json:"paths" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(400, err.Error()))
		return
	}

	result := h.permissionChecker.CheckPaths(req.Paths)
	c.JSON(http.StatusOK, SuccessResponse(result))
}

// checkSensitivePaths 检查敏感路径
func (h *Handlers) checkSensitivePaths(c *gin.Context) {
	if h.permissionChecker == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "权限检查器未启用"))
		return
	}

	result := h.permissionChecker.CheckSensitivePaths()
	c.JSON(http.StatusOK, SuccessResponse(result))
}

// checkSSHSecurity 检查SSH安全
func (h *Handlers) checkSSHSecurity(c *gin.Context) {
	if h.permissionChecker == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "权限检查器未启用"))
		return
	}

	result := h.permissionChecker.CheckSSHSecurity()
	c.JSON(http.StatusOK, SuccessResponse(result))
}

// checkUserHomeDirs 检查用户主目录
func (h *Handlers) checkUserHomeDirs(c *gin.Context) {
	if h.permissionChecker == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "权限检查器未启用"))
		return
	}

	result := h.permissionChecker.CheckUserHomeDirs()
	c.JSON(http.StatusOK, SuccessResponse(result))
}

// checkSystemConfig 检查系统配置
func (h *Handlers) checkSystemConfig(c *gin.Context) {
	if h.permissionChecker == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "权限检查器未启用"))
		return
	}

	result := h.permissionChecker.CheckSystemConfig()
	c.JSON(http.StatusOK, SuccessResponse(result))
}

// listPermissionRules 列出权限规则
func (h *Handlers) listPermissionRules(c *gin.Context) {
	if h.permissionChecker == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "权限检查器未启用"))
		return
	}

	rules := h.permissionChecker.ListRules()
	c.JSON(http.StatusOK, SuccessResponse(rules))
}

// addPermissionRule 添加权限规则
func (h *Handlers) addPermissionRule(c *gin.Context) {
	if h.permissionChecker == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "权限检查器未启用"))
		return
	}

	var rule PermissionRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(400, err.Error()))
		return
	}

	h.permissionChecker.AddRule(rule)
	c.JSON(http.StatusCreated, SuccessResponse(rule))
}

// removePermissionRule 移除权限规则
func (h *Handlers) removePermissionRule(c *gin.Context) {
	if h.permissionChecker == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "权限检查器未启用"))
		return
	}

	ruleID := c.Param("rule_id")
	h.permissionChecker.RemoveRule(ruleID)
	c.JSON(http.StatusOK, SuccessResponse(gin.H{"message": "已删除"}))
}

// fixPermissions 修复权限
func (h *Handlers) fixPermissions(c *gin.Context) {
	if h.permissionChecker == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "权限检查器未启用"))
		return
	}

	var req struct {
		Issues []*PermissionIssue `json:"issues" binding:"required"`
		DryRun bool               `json:"dry_run"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(400, err.Error()))
		return
	}

	results, err := h.permissionChecker.FixPermissions(req.Issues, req.DryRun)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(results))
}

// ========== 漏洞扫描处理器 ==========

// scanVulnerability 扫描组件漏洞
func (h *Handlers) scanVulnerability(c *gin.Context) {
	if h.vulnScanner == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "漏洞扫描器未启用"))
		return
	}

	var req struct {
		Component string `json:"component" binding:"required"`
		Version   string `json:"version" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(400, err.Error()))
		return
	}

	result, err := h.vulnScanner.ScanComponent(req.Component, req.Version)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(result))
}

// scanVulnerabilitiesBatch 批量扫描漏洞
func (h *Handlers) scanVulnerabilitiesBatch(c *gin.Context) {
	if h.vulnScanner == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "漏洞扫描器未启用"))
		return
	}

	var req struct {
		Components []ComponentInfo `json:"components" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(400, err.Error()))
		return
	}

	results, err := h.vulnScanner.ScanMultipleComponents(req.Components)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(results))
}

// getVulnerability 获取漏洞详情
func (h *Handlers) getVulnerability(c *gin.Context) {
	if h.vulnScanner == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "漏洞扫描器未启用"))
		return
	}

	cveID := c.Param("cve_id")
	vuln, err := h.vulnScanner.GetVulnerability(cveID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse(404, "漏洞不存在"))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(vuln))
}

// listVulnDatabases 列出漏洞数据库
func (h *Handlers) listVulnDatabases(c *gin.Context) {
	if h.vulnScanner == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "漏洞扫描器未启用"))
		return
	}

	dbs := h.vulnScanner.ListDatabases()
	c.JSON(http.StatusOK, SuccessResponse(dbs))
}

// syncVulnDatabase 同步漏洞数据库
func (h *Handlers) syncVulnDatabase(c *gin.Context) {
	if h.vulnScanner == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "漏洞扫描器未启用"))
		return
	}

	var req struct {
		Name string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(400, err.Error()))
		return
	}

	if err := h.vulnScanner.SyncDatabase(req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(gin.H{"message": "同步已启动"}))
}

// ========== 安全评分处理器 ==========

// calculateScore 计算安全评分
func (h *Handlers) calculateScore(c *gin.Context) {
	if h.scoreEngine == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "安全评分引擎未启用"))
		return
	}

	// 获取各项检查结果
	var permResult *PermissionCheckResult
	var scanReport *FileScanReport
	var vulnResult *VulnerabilityScanResult

	// 如果有扫描器，执行检查
	if h.permissionChecker != nil {
		permResult = h.permissionChecker.CheckSensitivePaths()
	}

	// 计算评分
	score := h.scoreEngine.CalculateScore(permResult, scanReport, vulnResult)

	c.JSON(http.StatusOK, SuccessResponse(score))
}

// getCurrentScore 获取当前评分
func (h *Handlers) getCurrentScore(c *gin.Context) {
	if h.scoreEngine == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "安全评分引擎未启用"))
		return
	}

	// 获取最近的评分历史
	history := h.scoreEngine.GetHistory(1)
	if len(history) == 0 {
		c.JSON(http.StatusOK, SuccessResponse(gin.H{"message": "暂无评分记录"}))
		return
	}

	latest := history[0]
	c.JSON(http.StatusOK, SuccessResponse(latest))
}

// getScoreHistory 获取评分历史
func (h *Handlers) getScoreHistory(c *gin.Context) {
	if h.scoreEngine == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "安全评分引擎未启用"))
		return
	}

	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	history := h.scoreEngine.GetHistory(days)

	c.JSON(http.StatusOK, SuccessResponse(history))
}

// getTrendAnalysis 获取趋势分析
func (h *Handlers) getTrendAnalysis(c *gin.Context) {
	if h.scoreEngine == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "安全评分引擎未启用"))
		return
	}

	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	analysis := h.scoreEngine.GetTrendAnalysis(days)

	c.JSON(http.StatusOK, SuccessResponse(analysis))
}

// getScoreReport 获取评分报告
func (h *Handlers) getScoreReport(c *gin.Context) {
	if h.scoreEngine == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "安全评分引擎未启用"))
		return
	}

	// 计算最新评分
	var permResult *PermissionCheckResult
	if h.permissionChecker != nil {
		permResult = h.permissionChecker.CheckSensitivePaths()
	}

	score := h.scoreEngine.CalculateScore(permResult, nil, nil)
	report := h.scoreEngine.GenerateScoreReport(score)

	c.JSON(http.StatusOK, SuccessResponse(report))
}

// getScoreCategories 获取评分类别
func (h *Handlers) getScoreCategories(c *gin.Context) {
	if h.scoreEngine == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "安全评分引擎未启用"))
		return
	}

	categories := h.scoreEngine.GetCategories()
	c.JSON(http.StatusOK, SuccessResponse(categories))
}

// updateScoreCategory 更新评分类别
func (h *Handlers) updateScoreCategory(c *gin.Context) {
	if h.scoreEngine == nil {
		c.JSON(http.StatusServiceUnavailable, ErrorResponse(503, "安全评分引擎未启用"))
		return
	}

	var category ScoreCategory
	if err := c.ShouldBindJSON(&category); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(400, err.Error()))
		return
	}

	h.scoreEngine.UpdateCategory(category)
	c.JSON(http.StatusOK, SuccessResponse(category))
}

// ========== 敏感数据规则处理器 ==========

// listSensitiveRules 列出敏感数据规则
func (h *Handlers) listSensitiveRules(c *gin.Context) {
	rules := DefaultSensitiveDataRules()
	c.JSON(http.StatusOK, SuccessResponse(rules))
}

// addSensitiveRule 添加敏感数据规则
func (h *Handlers) addSensitiveRule(c *gin.Context) {
	var rule SensitiveDataRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(400, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, SuccessResponse(rule))
}

// updateSensitiveRule 更新敏感数据规则
func (h *Handlers) updateSensitiveRule(c *gin.Context) {
	ruleID := c.Param("rule_id")

	var rule SensitiveDataRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(400, err.Error()))
		return
	}

	rule.ID = ruleID
	c.JSON(http.StatusOK, SuccessResponse(rule))
}

// deleteSensitiveRule 删除敏感数据规则
func (h *Handlers) deleteSensitiveRule(c *gin.Context) {
	c.JSON(http.StatusOK, SuccessResponse(gin.H{"message": "已删除"}))
}

// ========== 仪表板处理器 ==========

// getDashboardData 获取仪表板数据
func (h *Handlers) getDashboardData(c *gin.Context) {
	dashboard := gin.H{
		"timestamp": time.Now(),
	}

	// 权限检查摘要
	if h.permissionChecker != nil {
		permResult := h.permissionChecker.CheckSensitivePaths()
		dashboard["permission_check"] = gin.H{
			"total_checked":    permResult.TotalChecked,
			"issues_found":     permResult.IssuesFound,
			"critical_issues":  permResult.CriticalIssues,
			"warning_issues":   permResult.WarningIssues,
		}
	}

	// 评分摘要
	if h.scoreEngine != nil {
		history := h.scoreEngine.GetHistory(1)
		if len(history) > 0 {
			dashboard["security_score"] = gin.H{
				"score":  history[0].Score,
				"grade":  history[0].Grade,
				"trend":  h.scoreEngine.GetTrendAnalysis(7)["trend"],
			}
		}
	}

	// 扫描任务摘要
	if h.filesystemScanner != nil {
		tasks := h.filesystemScanner.ListScanTasks(5)
		dashboard["recent_scans"] = tasks
	}

	c.JSON(http.StatusOK, SuccessResponse(dashboard))
}

// getStatistics 获取统计数据
func (h *Handlers) getStatistics(c *gin.Context) {
	stats := gin.H{
		"timestamp": time.Now(),
	}

	// 权限检查统计
	if h.permissionChecker != nil {
		permResult := h.permissionChecker.CheckSensitivePaths()
		stats["permissions"] = h.permissionChecker.GetStatistics(permResult)
	}

	// 评分趋势
	if h.scoreEngine != nil {
		stats["score_trend"] = h.scoreEngine.GetTrendAnalysis(30)
	}

	// 扫描任务统计
	if h.filesystemScanner != nil {
		tasks := h.filesystemScanner.ListScanTasks(0)
		statusCount := make(map[string]int)
		for _, task := range tasks {
			statusCount[string(task.Status)]++
		}
		stats["scan_tasks"] = gin.H{
			"total":      len(tasks),
			"by_status":  statusCount,
		}
	}

	c.JSON(http.StatusOK, SuccessResponse(stats))
}