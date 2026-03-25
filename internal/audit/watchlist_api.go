// Package audit 提供审计日志Watch/Ignore List REST API接口
package audit

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// WatchListHandlers Watch/Ignore List API处理器
type WatchListHandlers struct {
	manager *WatchListManager
}

// NewWatchListHandlers 创建Watch/Ignore List处理器
func NewWatchListHandlers(manager *WatchListManager) *WatchListHandlers {
	return &WatchListHandlers{
		manager: manager,
	}
}

// RegisterRoutes 注册路由
// 注意：调用方应在应用此路由组前添加认证和权限中间件
func (h *WatchListHandlers) RegisterRoutes(api *gin.RouterGroup) {
	audit := api.Group("/audit")
	{
		// Watch List
		audit.POST("/watch", h.addWatchEntry)
		audit.GET("/watch", h.listWatchEntries)
		audit.GET("/watch/:id", h.getWatchEntry)
		audit.PUT("/watch/:id", h.updateWatchEntry)
		audit.DELETE("/watch/:id", h.deleteWatchEntry)

		// Ignore List
		audit.POST("/ignore", h.addIgnoreEntry)
		audit.GET("/ignore", h.listIgnoreEntries)
		audit.GET("/ignore/:id", h.getIgnoreEntry)
		audit.PUT("/ignore/:id", h.updateIgnoreEntry)
		audit.DELETE("/ignore/:id", h.deleteIgnoreEntry)

		// 统一列表查询
		audit.GET("/list", h.getAllLists)

		// 统计信息
		audit.GET("/list/stats", h.getStats)

		// 清理过期条目
		audit.POST("/list/cleanup", h.cleanupExpired)

		// 匹配检查
		audit.POST("/list/check", h.checkPath)
	}
}

// ========== Watch List API ==========

// addWatchEntryRequest 添加监控条目请求
type addWatchEntryRequest struct {
	Path        string           `json:"path" binding:"required"`
	Pattern     string           `json:"pattern,omitempty"`
	Operations  []WatchOperation `json:"operations"`
	Recursive   bool             `json:"recursive"`
	Enabled     bool             `json:"enabled"`
	Description string           `json:"description,omitempty"`
	Tags        []string         `json:"tags,omitempty"`
}

// addWatchEntry 添加监控条目
// @Summary 添加监控条目
// @Description 添加一个文件/目录到监控列表
// @Tags audit-watchlist
// @Accept json
// @Produce json
// @Param request body addWatchEntryRequest true "监控条目信息"
// @Success 201 {object} APIResponse
// @Failure 400 {object} APIResponse
// @Failure 500 {object} APIResponse
// @Router /audit/watch [post]
// @Security BearerAuth
func (h *WatchListHandlers) addWatchEntry(c *gin.Context) {
	var req addWatchEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	// 获取当前用户ID
	userID := c.GetString("user_id")
	if userID == "" {
		userID = "system"
	}

	entry := &WatchListEntry{
		Path:        req.Path,
		Pattern:     req.Pattern,
		Operations:  req.Operations,
		Recursive:   req.Recursive,
		Enabled:     req.Enabled,
		Description: req.Description,
		Tags:        req.Tags,
		CreatedBy:   userID,
	}

	if err := h.manager.AddWatchEntry(entry); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, SuccessResponse(entry))
}

// listWatchEntries 列出监控条目
// @Summary 列出监控条目
// @Description 获取监控列表
// @Tags audit-watchlist
// @Accept json
// @Produce json
// @Param path query string false "路径过滤"
// @Param enabled query bool false "启用状态过滤"
// @Param created_by query string false "创建者过滤"
// @Param limit query int false "数量限制" default(50)
// @Param offset query int false "偏移量" default(0)
// @Success 200 {object} APIResponse
// @Router /audit/watch [get]
// @Security BearerAuth
func (h *WatchListHandlers) listWatchEntries(c *gin.Context) {
	filter := WatchListFilter{
		Path:      c.Query("path"),
		CreatedBy: c.Query("created_by"),
	}

	if enabled := c.Query("enabled"); enabled != "" {
		val := enabled == "true"
		filter.Enabled = &val
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	filter.Limit = limit
	filter.Offset = offset

	entries := h.manager.ListWatchEntries(filter)

	c.JSON(http.StatusOK, SuccessResponse(gin.H{
		"total":   len(entries),
		"entries": entries,
	}))
}

// getWatchEntry 获取监控条目详情
// @Summary 获取监控条目详情
// @Description 根据ID获取监控条目详情
// @Tags audit-watchlist
// @Accept json
// @Produce json
// @Param id path string true "监控条目ID"
// @Success 200 {object} APIResponse
// @Failure 404 {object} APIResponse
// @Router /audit/watch/{id} [get]
// @Security BearerAuth
func (h *WatchListHandlers) getWatchEntry(c *gin.Context) {
	id := c.Param("id")

	entry, err := h.manager.GetWatchEntry(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse(ErrCodeNotFound, err.Error()))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(entry))
}

// updateWatchEntryRequest 更新监控条目请求
type updateWatchEntryRequest struct {
	Path        string           `json:"path,omitempty"`
	Pattern     string           `json:"pattern,omitempty"`
	Operations  []WatchOperation `json:"operations,omitempty"`
	Recursive   *bool            `json:"recursive,omitempty"`
	Enabled     *bool            `json:"enabled,omitempty"`
	Description string           `json:"description,omitempty"`
	Tags        []string         `json:"tags,omitempty"`
}

// updateWatchEntry 更新监控条目
// @Summary 更新监控条目
// @Description 更新监控条目信息
// @Tags audit-watchlist
// @Accept json
// @Produce json
// @Param id path string true "监控条目ID"
// @Param request body updateWatchEntryRequest true "更新信息"
// @Success 200 {object} APIResponse
// @Failure 400 {object} APIResponse
// @Failure 404 {object} APIResponse
// @Router /audit/watch/{id} [put]
// @Security BearerAuth
func (h *WatchListHandlers) updateWatchEntry(c *gin.Context) {
	id := c.Param("id")

	var req updateWatchEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	entry := &WatchListEntry{
		ID: id,
	}

	if req.Path != "" {
		entry.Path = req.Path
	}
	if req.Pattern != "" {
		entry.Pattern = req.Pattern
	}
	if len(req.Operations) > 0 {
		entry.Operations = req.Operations
	}
	if req.Recursive != nil {
		entry.Recursive = *req.Recursive
	}
	if req.Enabled != nil {
		entry.Enabled = *req.Enabled
	}
	if req.Description != "" {
		entry.Description = req.Description
	}
	if len(req.Tags) > 0 {
		entry.Tags = req.Tags
	}

	if err := h.manager.UpdateWatchEntry(entry); err != nil {
		if err.Error() == "监控条目不存在" {
			c.JSON(http.StatusNotFound, ErrorResponse(ErrCodeNotFound, err.Error()))
			return
		}
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	// 获取更新后的条目
	updated, _ := h.manager.GetWatchEntry(id)
	c.JSON(http.StatusOK, SuccessResponse(updated))
}

// deleteWatchEntry 删除监控条目
// @Summary 删除监控条目
// @Description 从监控列表中删除指定条目
// @Tags audit-watchlist
// @Accept json
// @Produce json
// @Param id path string true "监控条目ID"
// @Success 200 {object} APIResponse
// @Failure 404 {object} APIResponse
// @Router /audit/watch/{id} [delete]
// @Security BearerAuth
func (h *WatchListHandlers) deleteWatchEntry(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteWatchEntry(id); err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse(ErrCodeNotFound, err.Error()))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(gin.H{
		"message": "已删除",
	}))
}

// ========== Ignore List API ==========

// addIgnoreEntryRequest 添加忽略条目请求
type addIgnoreEntryRequest struct {
	Path        string     `json:"path" binding:"required"`
	Pattern     string     `json:"pattern,omitempty"`
	Reason      string     `json:"reason,omitempty"`
	Enabled     bool       `json:"enabled"`
	Description string     `json:"description,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Tags        []string   `json:"tags,omitempty"`
}

// addIgnoreEntry 添加忽略条目
// @Summary 添加忽略条目
// @Description 添加一个文件/目录到忽略列表
// @Tags audit-watchlist
// @Accept json
// @Produce json
// @Param request body addIgnoreEntryRequest true "忽略条目信息"
// @Success 201 {object} APIResponse
// @Failure 400 {object} APIResponse
// @Failure 500 {object} APIResponse
// @Router /audit/ignore [post]
// @Security BearerAuth
func (h *WatchListHandlers) addIgnoreEntry(c *gin.Context) {
	var req addIgnoreEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	// 获取当前用户ID
	userID := c.GetString("user_id")
	if userID == "" {
		userID = "system"
	}

	entry := &IgnoreListEntry{
		Path:        req.Path,
		Pattern:     req.Pattern,
		Reason:      req.Reason,
		Enabled:     req.Enabled,
		Description: req.Description,
		ExpiresAt:   req.ExpiresAt,
		Tags:        req.Tags,
		CreatedBy:   userID,
	}

	if err := h.manager.AddIgnoreEntry(entry); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, SuccessResponse(entry))
}

// listIgnoreEntries 列出忽略条目
// @Summary 列出忽略条目
// @Description 获取忽略列表
// @Tags audit-watchlist
// @Accept json
// @Produce json
// @Param path query string false "路径过滤"
// @Param enabled query bool false "启用状态过滤"
// @Param created_by query string false "创建者过滤"
// @Param limit query int false "数量限制" default(50)
// @Param offset query int false "偏移量" default(0)
// @Success 200 {object} APIResponse
// @Router /audit/ignore [get]
// @Security BearerAuth
func (h *WatchListHandlers) listIgnoreEntries(c *gin.Context) {
	filter := IgnoreListFilter{
		Path:      c.Query("path"),
		CreatedBy: c.Query("created_by"),
	}

	if enabled := c.Query("enabled"); enabled != "" {
		val := enabled == "true"
		filter.Enabled = &val
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	filter.Limit = limit
	filter.Offset = offset

	entries := h.manager.ListIgnoreEntries(filter)

	c.JSON(http.StatusOK, SuccessResponse(gin.H{
		"total":   len(entries),
		"entries": entries,
	}))
}

// getIgnoreEntry 获取忽略条目详情
// @Summary 获取忽略条目详情
// @Description 根据ID获取忽略条目详情
// @Tags audit-watchlist
// @Accept json
// @Produce json
// @Param id path string true "忽略条目ID"
// @Success 200 {object} APIResponse
// @Failure 404 {object} APIResponse
// @Router /audit/ignore/{id} [get]
// @Security BearerAuth
func (h *WatchListHandlers) getIgnoreEntry(c *gin.Context) {
	id := c.Param("id")

	entry, err := h.manager.GetIgnoreEntry(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse(ErrCodeNotFound, err.Error()))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(entry))
}

// updateIgnoreEntryRequest 更新忽略条目请求
type updateIgnoreEntryRequest struct {
	Path        string     `json:"path,omitempty"`
	Pattern     string     `json:"pattern,omitempty"`
	Reason      string     `json:"reason,omitempty"`
	Enabled     *bool      `json:"enabled,omitempty"`
	Description string     `json:"description,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Tags        []string   `json:"tags,omitempty"`
}

// updateIgnoreEntry 更新忽略条目
// @Summary 更新忽略条目
// @Description 更新忽略条目信息
// @Tags audit-watchlist
// @Accept json
// @Produce json
// @Param id path string true "忽略条目ID"
// @Param request body updateIgnoreEntryRequest true "更新信息"
// @Success 200 {object} APIResponse
// @Failure 400 {object} APIResponse
// @Failure 404 {object} APIResponse
// @Router /audit/ignore/{id} [put]
// @Security BearerAuth
func (h *WatchListHandlers) updateIgnoreEntry(c *gin.Context) {
	id := c.Param("id")

	var req updateIgnoreEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	entry := &IgnoreListEntry{
		ID: id,
	}

	if req.Path != "" {
		entry.Path = req.Path
	}
	if req.Pattern != "" {
		entry.Pattern = req.Pattern
	}
	if req.Reason != "" {
		entry.Reason = req.Reason
	}
	if req.Enabled != nil {
		entry.Enabled = *req.Enabled
	}
	if req.Description != "" {
		entry.Description = req.Description
	}
	if req.ExpiresAt != nil {
		entry.ExpiresAt = req.ExpiresAt
	}
	if len(req.Tags) > 0 {
		entry.Tags = req.Tags
	}

	if err := h.manager.UpdateIgnoreEntry(entry); err != nil {
		if err.Error() == "忽略条目不存在" {
			c.JSON(http.StatusNotFound, ErrorResponse(ErrCodeNotFound, err.Error()))
			return
		}
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	// 获取更新后的条目
	updated, _ := h.manager.GetIgnoreEntry(id)
	c.JSON(http.StatusOK, SuccessResponse(updated))
}

// deleteIgnoreEntry 删除忽略条目
// @Summary 删除忽略条目
// @Description 从忽略列表中删除指定条目
// @Tags audit-watchlist
// @Accept json
// @Produce json
// @Param id path string true "忽略条目ID"
// @Success 200 {object} APIResponse
// @Failure 404 {object} APIResponse
// @Router /audit/ignore/{id} [delete]
// @Security BearerAuth
func (h *WatchListHandlers) deleteIgnoreEntry(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteIgnoreEntry(id); err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse(ErrCodeNotFound, err.Error()))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(gin.H{
		"message": "已删除",
	}))
}

// ========== 统一列表查询 ==========

// getAllLists 获取所有列表
// @Summary 获取所有列表
// @Description 获取监控列表和忽略列表
// @Tags audit-watchlist
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse
// @Router /audit/list [get]
// @Security BearerAuth
func (h *WatchListHandlers) getAllLists(c *gin.Context) {
	watchFilter := WatchListFilter{
		Limit:  100,
		Offset: 0,
	}
	ignoreFilter := IgnoreListFilter{
		Limit:  100,
		Offset: 0,
	}

	watchEntries := h.manager.ListWatchEntries(watchFilter)
	ignoreEntries := h.manager.ListIgnoreEntries(ignoreFilter)

	c.JSON(http.StatusOK, SuccessResponse(gin.H{
		"watch_list": gin.H{
			"total":   len(watchEntries),
			"entries": watchEntries,
		},
		"ignore_list": gin.H{
			"total":   len(ignoreEntries),
			"entries": ignoreEntries,
		},
	}))
}

// getStats 获取统计信息
// @Summary 获取统计信息
// @Description 获取监控列表和忽略列表的统计信息
// @Tags audit-watchlist
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse
// @Router /audit/list/stats [get]
// @Security BearerAuth
func (h *WatchListHandlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, SuccessResponse(stats))
}

// cleanupExpired 清理过期条目
// @Summary 清理过期条目
// @Description 清理忽略列表中已过期的条目
// @Tags audit-watchlist
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse
// @Router /audit/list/cleanup [post]
// @Security BearerAuth
func (h *WatchListHandlers) cleanupExpired(c *gin.Context) {
	cleaned := h.manager.CleanupExpired()
	c.JSON(http.StatusOK, SuccessResponse(gin.H{
		"cleaned_count": cleaned,
		"message":       "清理完成",
	}))
}

// checkPathRequest 检查路径请求
type checkPathRequest struct {
	Path      string         `json:"path" binding:"required"`
	Operation WatchOperation `json:"operation,omitempty"`
}

// checkPathResponse 检查路径响应
type checkPathResponse struct {
	Path      string          `json:"path"`
	IsWatched bool            `json:"is_watched"`
	IsIgnored bool            `json:"is_ignored"`
	WatchedBy *WatchListEntry `json:"watched_by,omitempty"`
}

// checkPath 检查路径是否被监控或忽略
// @Summary 检查路径状态
// @Description 检查指定路径是否在监控列表或忽略列表中
// @Tags audit-watchlist
// @Accept json
// @Produce json
// @Param request body checkPathRequest true "检查请求"
// @Success 200 {object} APIResponse
// @Router /audit/list/check [post]
// @Security BearerAuth
func (h *WatchListHandlers) checkPath(c *gin.Context) {
	var req checkPathRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	operation := req.Operation
	if operation == "" {
		operation = WatchOpAll
	}

	watchedBy := h.manager.ShouldWatch(req.Path, operation)
	isIgnored := h.manager.IsIgnored(req.Path)

	response := checkPathResponse{
		Path:      req.Path,
		IsWatched: watchedBy != nil,
		IsIgnored: isIgnored,
		WatchedBy: watchedBy,
	}

	c.JSON(http.StatusOK, SuccessResponse(response))
}
