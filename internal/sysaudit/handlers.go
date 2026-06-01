// Package sysaudit HTTP API 处理器
package sysaudit

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Store 审计日志存储接口
type Store interface {
	// Query 查询审计日志
	Query(opts QueryOptions) (*QueryResult, error)
	// GetByID 根据ID获取日志
	GetByID(id string) (*Entry, error)
	// Create 创建审计日志
	Create(entry *Entry) error
	// Stats 获取统计信息
	Stats() (*Statistics, error)
	// Categories 获取所有事件分类
	Categories() []Category
	// Export 导出审计日志
	Export(opts ExportOptions) ([]byte, error)
	// Cleanup 清理过期日志
	Cleanup(maxAgeDays int) (int, error)
	// ListEvents 列出事件（带分页）
	ListEvents(limit, offset int) ([]*Entry, int, error)
	// SearchEvents 搜索事件
	SearchEvents(opts QueryOptions) (*QueryResult, error)
	// ListReports 列出合规报告
	ListReports() ([]*ComplianceReport, error)
	// GenerateReport 生成合规报告
	GenerateReport(standard ComplianceStandard, start, end time.Time) (*ComplianceReport, error)
}

// Handler HTTP 处理器
type Handler struct {
	store Store
}

// NewHandler 创建处理器
func NewHandler(store Store) *Handler {
	return &Handler{store: store}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	group := router.Group("/sys-audit")
	{
		// 审计日志
		group.GET("/logs", h.GetLogs)
		group.POST("/logs", h.CreateLog)
		group.GET("/logs/:id", h.GetLogByID)

		// 事件
		group.GET("/events", h.ListEvents)
		group.POST("/events/search", h.SearchEvents)

		// 统计
		group.GET("/stats", h.GetStats)

		// 报告
		group.GET("/reports", h.ListReports)
		group.POST("/reports/generate", h.GenerateReport)

		// 其他
		group.GET("/categories", h.GetCategories)
		group.GET("/export", h.ExportLogs)
		group.POST("/cleanup", h.Cleanup)
	}
}

// GetLogs 获取审计日志列表
func (h *Handler) GetLogs(c *gin.Context) {
	opts := QueryOptions{
		Limit:  50,
		Offset: 0,
	}

	// 分页参数
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		opts.Limit = l
	}
	if o, err := strconv.Atoi(c.Query("offset")); err == nil && o >= 0 {
		opts.Offset = o
	}

	// 过滤参数
	opts.Level = Level(c.Query("level"))
	opts.Category = Category(c.Query("category"))
	opts.UserID = c.Query("user_id")
	opts.Username = c.Query("username")
	opts.IP = c.Query("ip")
	opts.Status = Status(c.Query("status"))
	opts.Event = c.Query("event")
	opts.Resource = c.Query("resource")
	opts.Keyword = c.Query("keyword")
	opts.SessionID = c.Query("session_id")
	opts.Hostname = c.Query("hostname")

	// 时间范围
	if startStr := c.Query("start_time"); startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			opts.StartTime = &t
		}
	}
	if endStr := c.Query("end_time"); endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			opts.EndTime = &t
		}
	}

	result, err := h.store.Query(opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(result))
}

// CreateLog 创建审计日志
func (h *Handler) CreateLog(c *gin.Context) {
	var entry Entry
	if err := c.ShouldBindJSON(&entry); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, "无效的请求参数: "+err.Error()))
		return
	}

	// 自动生成ID和时间戳
	if entry.ID == "" {
		entry.ID = "audit_" + time.Now().Format("20060102150405") + "_" + strconv.FormatInt(time.Now().UnixNano()%100000, 10)
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	if err := h.store.Create(&entry); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, SuccessResponse(entry))
}

// GetLogByID 获取单条日志详情
func (h *Handler) GetLogByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, "缺少日志ID"))
		return
	}

	entry, err := h.store.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse(ErrCodeNotFound, ErrEntryNotFound))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(entry))
}

// GetStats 获取审计统计
func (h *Handler) GetStats(c *gin.Context) {
	stats, err := h.store.Stats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(stats))
}

// GetCategories 获取事件分类列表
func (h *Handler) GetCategories(c *gin.Context) {
	categories := h.store.Categories()
	c.JSON(http.StatusOK, SuccessResponse(categories))
}

// ExportLogs 导出审计日志
func (h *Handler) ExportLogs(c *gin.Context) {
	opts := ExportOptions{
		Format: ExportJSON,
	}

	// 解析时间范围
	startStr := c.Query("start_time")
	endStr := c.Query("end_time")

	if startStr == "" || endStr == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, "导出需要指定 start_time 和 end_time"))
		return
	}

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, "无效的 start_time 格式，需 RFC3339"))
		return
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, "无效的 end_time 格式，需 RFC3339"))
		return
	}

	opts.StartTime = start
	opts.EndTime = end

	// 分类过滤
	if cats := c.QueryArray("category"); len(cats) > 0 {
		for _, cat := range cats {
			opts.Categories = append(opts.Categories, Category(cat))
		}
	}

	// 其他选项
	if c.Query("include_signatures") == "true" {
		opts.IncludeSignatures = true
	}
	if c.Query("compress") == "true" {
		opts.Compress = true
	}

	data, err := h.store.Export(opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, ErrExportFailed))
		return
	}

	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", "attachment; filename=audit_export.json")
	c.Data(http.StatusOK, "application/json", data)
}

// Cleanup 清理过期日志
func (h *Handler) Cleanup(c *gin.Context) {
	var req struct {
		MaxAgeDays int `json:"max_age_days"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, "无效的请求参数: "+err.Error()))
		return
	}

	if req.MaxAgeDays <= 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, "max_age_days 必须大于 0"))
		return
	}

	count, err := h.store.Cleanup(req.MaxAgeDays)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(gin.H{
		"deleted_count": count,
		"max_age_days":  req.MaxAgeDays,
	}))
}

// ListEvents 获取事件列表
func (h *Handler) ListEvents(c *gin.Context) {
	limit := 50
	offset := 0

	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}
	if o, err := strconv.Atoi(c.Query("offset")); err == nil && o >= 0 {
		offset = o
	}

	events, total, err := h.store.ListEvents(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(gin.H{
		"total":  total,
		"events": events,
	}))
}

// SearchEventsRequest 事件搜索请求
type SearchEventsRequest struct {
	Keyword  string   `json:"keyword"`
	Level    Level    `json:"level"`
	Category Category `json:"category"`
	Status   Status   `json:"status"`
	UserID   string   `json:"user_id"`
	IP       string   `json:"ip"`
	Start    string   `json:"start_time"`
	End      string   `json:"end_time"`
	Limit    int      `json:"limit"`
	Offset   int      `json:"offset"`
}

// SearchEvents 搜索事件
func (h *Handler) SearchEvents(c *gin.Context) {
	var req SearchEventsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, "无效的请求参数: "+err.Error()))
		return
	}

	opts := QueryOptions{
		Keyword:  req.Keyword,
		Level:    req.Level,
		Category: req.Category,
		Status:   req.Status,
		UserID:   req.UserID,
		IP:       req.IP,
		Limit:    50,
		Offset:   0,
	}

	if req.Limit > 0 {
		opts.Limit = req.Limit
	}
	if req.Offset >= 0 {
		opts.Offset = req.Offset
	}

	if req.Start != "" {
		if t, err := time.Parse(time.RFC3339, req.Start); err == nil {
			opts.StartTime = &t
		}
	}
	if req.End != "" {
		if t, err := time.Parse(time.RFC3339, req.End); err == nil {
			opts.EndTime = &t
		}
	}

	result, err := h.store.SearchEvents(opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(result))
}

// ListReports 获取报告列表
func (h *Handler) ListReports(c *gin.Context) {
	reports, err := h.store.ListReports()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(gin.H{
		"total":   len(reports),
		"reports": reports,
	}))
}

// GenerateReportRequest 生成报告请求
type GenerateReportRequest struct {
	Standard ComplianceStandard `json:"standard" binding:"required"`
	Start    string             `json:"start_time" binding:"required"`
	End      string             `json:"end_time" binding:"required"`
}

// GenerateReport 生成合规报告
func (h *Handler) GenerateReport(c *gin.Context) {
	var req GenerateReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, "无效的请求参数: "+err.Error()))
		return
	}

	start, err := time.Parse(time.RFC3339, req.Start)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, "无效的 start_time 格式，需 RFC3339"))
		return
	}
	end, err := time.Parse(time.RFC3339, req.End)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, "无效的 end_time 格式，需 RFC3339"))
		return
	}

	report, err := h.store.GenerateReport(req.Standard, start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(report))
}
