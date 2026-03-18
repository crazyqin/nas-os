package dedup

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 去重 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{
		manager: manager,
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	dedup := r.Group("/dedup")
	{
		// 扫描操作
		dedup.POST("/scan", h.scan)
		dedup.POST("/scan/cancel", h.cancelScan)

		// 重复文件
		dedup.GET("/duplicates", h.getDuplicates)
		dedup.GET("/duplicates/cross-user", h.getCrossUserDuplicates)
		dedup.POST("/deduplicate", h.deduplicate)
		dedup.POST("/deduplicate/all", h.deduplicateAll)

		// 报告和统计
		dedup.GET("/report", h.getReport)
		dedup.GET("/stats", h.getStats)
		dedup.GET("/stats/users", h.getUserStats)

		// 配置管理
		dedup.GET("/config", h.getConfig)
		dedup.PUT("/config", h.updateConfig)

		// 自动去重
		dedup.GET("/auto", h.getAutoTask)
		dedup.POST("/auto/enable", h.enableAutoDedup)
		dedup.POST("/auto/run", h.runAutoDedup)

		// 块管理
		dedup.GET("/chunks", h.getChunks)
		dedup.GET("/chunks/shared", h.getSharedChunks)
		dedup.POST("/chunks/file", h.chunkFile)
	}
}

// ========== API 处理函数 ==========

// scan 扫描重复文件
// @Summary 扫描重复文件
// @Description 扫描指定路径查找重复文件
// @Tags dedup
// @Accept json
// @Produce json
// @Param request body ScanRequest true "扫描参数"
// @Success 200 {object} GenericResponse "扫描完成"
// @Failure 400 {object} GenericResponse "请求参数错误"
// @Failure 500 {object} GenericResponse "服务器内部错误"
// @Router /api/v1/dedup/scan [post]
func (h *Handlers) scan(c *gin.Context) {
	var req ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 使用默认路径
		req.Paths = []string{}
	}

	result, err := h.manager.ScanForUser(req.Paths, req.User)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "扫描完成",
		"data":    result,
	})
}

// cancelScan 取消扫描
// @Summary 取消扫描
// @Description 取消正在进行的扫描任务
// @Tags dedup
// @Accept json
// @Produce json
// @Success 200 {object} GenericResponse "已取消"
// @Router /api/v1/dedup/scan/cancel [post]
func (h *Handlers) cancelScan(c *gin.Context) {
	h.manager.CancelScan()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "扫描已取消",
	})
}

// getDuplicates 获取重复文件列表
// @Summary 获取重复文件列表
// @Description 获取扫描发现的重复文件分组列表
// @Tags dedup
// @Accept json
// @Produce json
// @Param user query string false "用户名（可选，筛选特定用户的重复文件）"
// @Success 200 {object} GenericResponse "成功"
// @Router /api/v1/dedup/duplicates [get]
func (h *Handlers) getDuplicates(c *gin.Context) {
	user := c.Query("user")

	duplicates, err := h.manager.GetDuplicatesForUser(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    duplicates,
	})
}

// getCrossUserDuplicates 获取跨用户重复文件
// @Summary 获取跨用户重复文件
// @Description 获取跨用户共享的重复文件列表
// @Tags dedup
// @Accept json
// @Produce json
// @Success 200 {object} GenericResponse "成功"
// @Router /api/v1/dedup/duplicates/cross-user [get]
func (h *Handlers) getCrossUserDuplicates(c *gin.Context) {
	duplicates, err := h.manager.GetCrossUserDuplicates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    duplicates,
	})
}

// deduplicate 执行去重
// @Summary 执行去重操作
// @Description 对指定的重复文件组执行去重操作
// @Tags dedup
// @Accept json
// @Produce json
// @Param request body DeduplicateRequest true "去重参数"
// @Success 200 {object} GenericResponse "去重完成"
// @Failure 400 {object} GenericResponse "请求参数错误"
// @Failure 500 {object} GenericResponse "服务器内部错误"
// @Router /api/v1/dedup/deduplicate [post]
func (h *Handlers) deduplicate(c *gin.Context) {
	var req DeduplicateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if req.Checksum == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "checksum 不能为空",
		})
		return
	}

	if req.KeepPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "keepPath 不能为空",
		})
		return
	}

	// 设置默认策略
	policy := Policy{
		Mode:          Mode(req.Mode),
		Action:        Action(req.Action),
		MinMatchCount: 1,
		PreserveAttrs: true,
		CrossUser:     req.CrossUser,
	}

	if policy.Mode == "" {
		policy.Mode = ModeFile
	}
	if policy.Action == "" {
		policy.Action = ActionSoftlink
	}

	if err := h.manager.DeduplicateForUser(req.Checksum, req.KeepPath, policy, req.User); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "去重操作完成",
	})
}

// deduplicateAll 批量去重
// @Summary 批量去重
// @Description 对所有重复文件执行批量去重操作
// @Tags dedup
// @Accept json
// @Produce json
// @Param request body DeduplicateAllRequest true "批量去重参数"
// @Success 200 {object} GenericResponse "去重完成"
// @Failure 400 {object} GenericResponse "请求参数错误"
// @Failure 500 {object} GenericResponse "服务器内部错误"
// @Router /api/v1/dedup/deduplicate/all [post]
func (h *Handlers) deduplicateAll(c *gin.Context) {
	var req DeduplicateAllRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 使用默认值
		req = DeduplicateAllRequest{}
	}

	policy := Policy{
		Mode:          Mode(req.Mode),
		Action:        Action(req.Action),
		MinMatchCount: 2,
		PreserveAttrs: true,
		CrossUser:     req.CrossUser,
	}

	if policy.Mode == "" {
		policy.Mode = ModeFile
	}
	if policy.Action == "" {
		policy.Action = ActionSoftlink
	}

	result, err := h.manager.DeduplicateAll(policy, req.DryRun)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "批量去重完成",
		"data":    result,
	})
}

// getReport 获取去重报告
// @Summary 获取去重报告
// @Description 获取详细的去重分析报告
// @Tags dedup
// @Accept json
// @Produce json
// @Param user query string false "用户名（可选，获取特定用户的报告）"
// @Success 200 {object} GenericResponse "成功"
// @Router /api/v1/dedup/report [get]
func (h *Handlers) getReport(c *gin.Context) {
	user := c.Query("user")

	report, err := h.manager.GetReportForUser(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    report,
	})
}

// getStats 获取统计信息
// @Summary 获取去重统计信息
// @Description 获取去重模块的统计信息
// @Tags dedup
// @Accept json
// @Produce json
// @Success 200 {object} GenericResponse "成功"
// @Router /api/v1/dedup/stats [get]
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// getUserStats 获取用户统计信息
// @Summary 获取用户统计信息
// @Description 获取各用户的去重统计信息
// @Tags dedup
// @Accept json
// @Produce json
// @Param user query string false "用户名（可选，获取特定用户的统计）"
// @Success 200 {object} GenericResponse "成功"
// @Router /api/v1/dedup/stats/users [get]
func (h *Handlers) getUserStats(c *gin.Context) {
	user := c.Query("user")

	if user != "" {
		stats, err := h.manager.GetUserStats(user)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data":    stats,
		})
		return
	}

	// 返回所有用户统计
	report, err := h.manager.GetReport()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    report.UserReports,
	})
}

// getConfig 获取配置
// @Summary 获取去重配置
// @Description 获取去重模块的配置信息
// @Tags dedup
// @Accept json
// @Produce json
// @Success 200 {object} GenericResponse "成功"
// @Router /api/v1/dedup/config [get]
func (h *Handlers) getConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    h.manager.GetConfig(),
	})
}

// updateConfig 更新配置
// @Summary 更新去重配置
// @Description 更新去重模块的配置
// @Tags dedup
// @Accept json
// @Produce json
// @Param request body Config true "配置参数"
// @Success 200 {object} GenericResponse "更新成功"
// @Failure 400 {object} GenericResponse "请求参数错误"
// @Failure 500 {object} GenericResponse "服务器内部错误"
// @Router /api/v1/dedup/config [put]
func (h *Handlers) updateConfig(c *gin.Context) {
	var config Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.manager.UpdateConfig(&config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "配置更新成功",
	})
}

// ========== 自动去重 API ==========

// getAutoTask 获取自动去重任务
// @Summary 获取自动去重任务状态
// @Description 获取自动去重任务的配置和状态
// @Tags dedup
// @Accept json
// @Produce json
// @Success 200 {object} GenericResponse "成功"
// @Router /api/v1/dedup/auto [get]
func (h *Handlers) getAutoTask(c *gin.Context) {
	task := h.manager.GetAutoTask()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    task,
	})
}

// enableAutoDedup 启用/禁用自动去重
// @Summary 启用或禁用自动去重
// @Description 配置自动去重任务的启用状态和调度
// @Tags dedup
// @Accept json
// @Produce json
// @Param request body EnableAutoRequest true "自动去重配置"
// @Success 200 {object} GenericResponse "配置成功"
// @Failure 400 {object} GenericResponse "请求参数错误"
// @Failure 500 {object} GenericResponse "服务器内部错误"
// @Router /api/v1/dedup/auto/enable [post]
func (h *Handlers) enableAutoDedup(c *gin.Context) {
	var req EnableAutoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.manager.EnableAutoDedup(req.Enabled, req.Schedule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "自动去重配置已更新",
	})
}

// runAutoDedup 执行自动去重
// @Summary 手动执行自动去重
// @Description 手动触发一次自动去重任务
// @Tags dedup
// @Accept json
// @Produce json
// @Success 200 {object} GenericResponse "执行成功"
// @Failure 500 {object} GenericResponse "服务器内部错误"
// @Router /api/v1/dedup/auto/run [post]
func (h *Handlers) runAutoDedup(c *gin.Context) {
	result, err := h.manager.RunAutoDedup()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "自动去重执行完成",
		"data":    result,
	})
}

// ========== 块管理 API ==========

// getChunks 获取块列表
// @Summary 获取数据块列表
// @Description 获取所有存储的数据块信息
// @Tags dedup
// @Accept json
// @Produce json
// @Success 200 {object} GenericResponse "成功"
// @Router /api/v1/dedup/chunks [get]
func (h *Handlers) getChunks(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"chunksStored":   stats.ChunksStored,
			"chunkDataSize":  stats.ChunkDataSize,
			"sharedChunks":   stats.SharedChunks,
			"sharedDataSize": stats.SharedDataSize,
		},
	})
}

// getSharedChunks 获取共享块
// @Summary 获取共享数据块
// @Description 获取被多个用户共享的数据块列表
// @Tags dedup
// @Accept json
// @Produce json
// @Success 200 {object} GenericResponse "成功"
// @Router /api/v1/dedup/chunks/shared [get]
func (h *Handlers) getSharedChunks(c *gin.Context) {
	chunks, err := h.manager.GetSharedChunks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    chunks,
	})
}

// chunkFile 文件分块
// @Summary 文件分块
// @Description 将文件分割成数据块
// @Tags dedup
// @Accept json
// @Produce json
// @Param request body ChunkFileRequest true "分块参数"
// @Success 200 {object} GenericResponse "分块成功"
// @Failure 400 {object} GenericResponse "请求参数错误"
// @Failure 500 {object} GenericResponse "服务器内部错误"
// @Router /api/v1/dedup/chunks/file [post]
func (h *Handlers) chunkFile(c *gin.Context) {
	var req ChunkFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if req.FilePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "filePath 不能为空",
		})
		return
	}

	chunks, err := h.manager.ChunkFile(req.FilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "文件分块完成",
		"data": gin.H{
			"file":   req.FilePath,
			"chunks": chunks,
			"count":  len(chunks),
		},
	})
}

// ========== 请求/响应类型 ==========

// ScanRequest 扫描请求
type ScanRequest struct {
	Paths []string `json:"paths"` // 要扫描的路径，为空则使用配置中的路径
	User  string   `json:"user"`  // 用户名（可选）
}

// DeduplicateRequest 去重请求
type DeduplicateRequest struct {
	Checksum  string `json:"checksum" binding:"required"` // 重复文件组的校验和
	KeepPath  string `json:"keepPath" binding:"required"` // 要保留的文件路径
	Mode      string `json:"mode"`                        // file, chunk, hybrid
	Action    string `json:"action"`                      // report, softlink, hardlink
	User      string `json:"user"`                        // 用户名（可选，用于权限检查）
	CrossUser bool   `json:"crossUser"`                   // 是否允许跨用户去重
}

// DeduplicateAllRequest 批量去重请求
type DeduplicateAllRequest struct {
	Mode      string `json:"mode"`      // file, chunk, hybrid
	Action    string `json:"action"`    // softlink, hardlink
	DryRun    bool   `json:"dryRun"`    // 是否只预览不执行
	CrossUser bool   `json:"crossUser"` // 是否允许跨用户去重
}

// EnableAutoRequest 启用自动去重请求
type EnableAutoRequest struct {
	Enabled  bool   `json:"enabled"`
	Schedule string `json:"schedule"` // cron 表达式
}

// ChunkFileRequest 文件分块请求
type ChunkFileRequest struct {
	FilePath string `json:"filePath" binding:"required"`
}

// GenericResponse 通用响应
type GenericResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
