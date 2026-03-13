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
		dedup.POST("/deduplicate", h.deduplicate)

		// 报告和统计
		dedup.GET("/report", h.getReport)
		dedup.GET("/stats", h.getStats)

		// 配置管理
		dedup.GET("/config", h.getConfig)
		dedup.PUT("/config", h.updateConfig)
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

	result, err := h.manager.Scan(req.Paths)
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
// @Success 200 {object} GenericResponse "成功"
// @Router /api/v1/dedup/duplicates [get]
func (h *Handlers) getDuplicates(c *gin.Context) {
	duplicates, err := h.manager.GetDuplicates()
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
	policy := DedupPolicy{
		Mode:          req.Mode,
		Action:        req.Action,
		MinMatchCount: 1,
		PreserveAttrs: true,
	}

	if policy.Mode == "" {
		policy.Mode = "file"
	}
	if policy.Action == "" {
		policy.Action = "softlink"
	}

	if err := h.manager.Deduplicate(req.Checksum, req.KeepPath, policy); err != nil {
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

// getReport 获取去重报告
// @Summary 获取去重报告
// @Description 获取详细的去重分析报告
// @Tags dedup
// @Accept json
// @Produce json
// @Success 200 {object} GenericResponse "成功"
// @Router /api/v1/dedup/report [get]
func (h *Handlers) getReport(c *gin.Context) {
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
		"data":    h.manager.config,
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

// ========== 请求/响应类型 ==========

// ScanRequest 扫描请求
type ScanRequest struct {
	Paths []string `json:"paths"` // 要扫描的路径，为空则使用配置中的路径
}

// DeduplicateRequest 去重请求
type DeduplicateRequest struct {
	Checksum string `json:"checksum" binding:"required"` // 重复文件组的校验和
	KeepPath string `json:"keepPath" binding:"required"` // 要保留的文件路径
	Mode     string `json:"mode"`                        // file, chunk, hybrid
	Action   string `json:"action"`                      // report, softlink, hardlink
}

// GenericResponse 通用响应
type GenericResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
