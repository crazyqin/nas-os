// Package audit 提供 SMB/NFS 审计日志 REST API 接口
package audit

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// FileAuditHandlers 文件审计 API 处理器
type FileAuditHandlers struct {
	logger  *FileAuditLogger
	storage *FileAuditStorage
}

// NewFileAuditHandlers 创建文件审计处理器
func NewFileAuditHandlers(logger *FileAuditLogger) *FileAuditHandlers {
	return &FileAuditHandlers{
		logger:  logger,
		storage: logger.storage,
	}
}

// RegisterRoutes 注册路由
// 注意：调用方应在应用此路由组前添加认证和权限中间件
func (h *FileAuditHandlers) RegisterRoutes(api *gin.RouterGroup) {
	audit := api.Group("/file-audit")
	{
		// 日志查询
		audit.GET("/logs", h.getLogs)
		audit.GET("/logs/:id", h.getLogByID)
		audit.GET("/statistics", h.getStatistics)
		audit.GET("/timeline", h.getTimeline)
		audit.GET("/search", h.searchLogs)

		// 导出
		audit.GET("/export", h.exportLogs)

		// 存储管理
		audit.GET("/storage/info", h.getStorageInfo)
		audit.GET("/storage/dates", h.getAvailableDates)
		audit.POST("/storage/archive", h.archiveLogs)
		audit.POST("/storage/cleanup", h.cleanupLogs)

		// 配置管理
		audit.GET("/config", h.getConfig)
		audit.PUT("/config", h.updateConfig)
		audit.POST("/enable", h.enable)
		audit.POST("/disable", h.disable)

		// 操作记录接口（内部使用）
		audit.POST("/log/smb", h.logSMBOperation)
		audit.POST("/log/nfs", h.logNFSOperation)
		audit.POST("/log/file/create", h.logFileCreate)
		audit.POST("/log/file/delete", h.logFileDelete)
		audit.POST("/log/file/rename", h.logFileRename)
		audit.POST("/log/file/move", h.logFileMove)
		audit.POST("/log/file/write", h.logFileWrite)
	}
}

// ========== 日志查询 ==========

// getLogs 获取审计日志列表
// @Summary 获取文件审计日志列表
// @Description 查询 SMB/NFS 文件操作审计日志，支持多种过滤条件
// @Tags file-audit
// @Accept json
// @Produce json
// @Param limit query int false "每页数量" default(50)
// @Param offset query int false "偏移量" default(0)
// @Param start_time query string false "开始时间 (RFC3339)"
// @Param end_time query string false "结束时间 (RFC3339)"
// @Param protocol query string false "协议类型 (smb/nfs)"
// @Param user_id query string false "用户ID"
// @Param username query string false "用户名"
// @Param client_ip query string false "客户端IP"
// @Param operation query string false "操作类型"
// @Param status query string false "状态 (success/failure/denied)"
// @Param file_path query string false "文件路径"
// @Param share_name query string false "共享名称"
// @Param keyword query string false "关键词搜索"
// @Success 200 {object} FileAuditQueryResult
// @Failure 500 {object} APIResponse
// @Router /file-audit/logs [get]
// @Security BearerAuth
func (h *FileAuditHandlers) getLogs(c *gin.Context) {
	// 解析查询参数
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	opts := FileAuditQueryOptions{
		Limit:  limit,
		Offset: offset,
	}

	// 时间范围
	if startTime := c.Query("start_time"); startTime != "" {
		t, err := time.Parse(time.RFC3339, startTime)
		if err == nil {
			opts.StartTime = &t
		}
	}
	if endTime := c.Query("end_time"); endTime != "" {
		t, err := time.Parse(time.RFC3339, endTime)
		if err == nil {
			opts.EndTime = &t
		}
	}

	// 筛选条件
	if protocol := c.Query("protocol"); protocol != "" {
		opts.Protocol = Protocol(protocol)
	}
	if userID := c.Query("user_id"); userID != "" {
		opts.UserID = userID
	}
	if username := c.Query("username"); username != "" {
		opts.Username = username
	}
	if clientIP := c.Query("client_ip"); clientIP != "" {
		opts.ClientIP = clientIP
	}
	if operation := c.Query("operation"); operation != "" {
		opts.Operation = FileOperation(operation)
	}
	if status := c.Query("status"); status != "" {
		opts.Status = Status(status)
	}
	if filePath := c.Query("file_path"); filePath != "" {
		opts.FilePath = filePath
	}
	if shareName := c.Query("share_name"); shareName != "" {
		opts.ShareName = shareName
	}
	if keyword := c.Query("keyword"); keyword != "" {
		opts.Keyword = keyword
	}

	// 执行查询
	result, err := h.logger.Query(opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, result)
}

// getLogByID 根据ID获取日志
// @Summary 获取审计日志详情
// @Description 根据ID获取单条文件审计日志详情
// @Tags file-audit
// @Accept json
// @Produce json
// @Param id path string true "日志ID"
// @Success 200 {object} FileAuditEntry
// @Failure 404 {object} APIResponse
// @Router /file-audit/logs/{id} [get]
// @Security BearerAuth
func (h *FileAuditHandlers) getLogByID(c *gin.Context) {
	id := c.Param("id")

	entry, err := h.logger.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse(ErrCodeNotFound, err.Error()))
		return
	}

	c.JSON(http.StatusOK, entry)
}

// getStatistics 获取审计统计
// @Summary 获取文件审计统计信息
// @Description 获取 SMB/NFS 文件操作审计的统计汇总数据
// @Tags file-audit
// @Accept json
// @Produce json
// @Success 200 {object} FileAuditStatistics
// @Router /file-audit/statistics [get]
// @Security BearerAuth
func (h *FileAuditHandlers) getStatistics(c *gin.Context) {
	stats := h.logger.GetStatistics()
	c.JSON(http.StatusOK, stats)
}

// TimelineItem 时间线条目
type TimelineItem struct {
	Time       time.Time      `json:"time"`
	Count      int            `json:"count"`
	Operations map[string]int `json:"operations"`
	Users      map[string]int `json:"users"`
}

// getTimeline 获取事件时间线
// @Summary 获取事件时间线
// @Description 获取指定时间范围内的文件操作事件时间线
// @Tags file-audit
// @Accept json
// @Produce json
// @Param start_time query string false "开始时间 (RFC3339)"
// @Param end_time query string false "结束时间 (RFC3339)"
// @Param interval query string false "时间间隔 (hour/day)" default(hour)
// @Success 200 {object} []TimelineItem
// @Router /file-audit/timeline [get]
// @Security BearerAuth
func (h *FileAuditHandlers) getTimeline(c *gin.Context) {
	// 解析时间范围
	startTime := time.Now().Add(-24 * time.Hour)
	endTime := time.Now()

	if st := c.Query("start_time"); st != "" {
		t, err := time.Parse(time.RFC3339, st)
		if err == nil {
			startTime = t
		}
	}
	if et := c.Query("end_time"); et != "" {
		t, err := time.Parse(time.RFC3339, et)
		if err == nil {
			endTime = t
		}
	}

	interval := c.DefaultQuery("interval", "hour")

	// 查询日志
	opts := FileAuditQueryOptions{
		StartTime: &startTime,
		EndTime:   &endTime,
		Limit:     10000,
	}

	result, err := h.logger.Query(opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	// 生成时间线
	timeline := h.generateTimeline(result.Entries, interval)

	c.JSON(http.StatusOK, timeline)
}

// generateTimeline 生成时间线
func (h *FileAuditHandlers) generateTimeline(entries []*FileAuditEntry, interval string) []TimelineItem {
	timeline := make(map[string]*TimelineItem)

	for _, entry := range entries {
		var key string
		switch interval {
		case "day":
			key = entry.Timestamp.Format("2006-01-02")
		case "hour":
			key = entry.Timestamp.Format("2006-01-02T15")
		default:
			key = entry.Timestamp.Format("2006-01-02T15")
		}

		if item, exists := timeline[key]; exists {
			item.Count++
			item.Operations[string(entry.Operation)]++
			item.Users[entry.Username]++
		} else {
			t := entry.Timestamp
			timeline[key] = &TimelineItem{
				Time:       t,
				Count:      1,
				Operations: map[string]int{string(entry.Operation): 1},
				Users:      map[string]int{entry.Username: 1},
			}
		}
	}

	// 转换为切片并排序
	result := make([]TimelineItem, 0, len(timeline))
	for _, item := range timeline {
		result = append(result, *item)
	}

	return result
}

// searchLogs 搜索日志
// @Summary 搜索审计日志
// @Description 通过关键词搜索文件审计日志
// @Tags file-audit
// @Accept json
// @Produce json
// @Param q query string true "搜索关键词"
// @Param limit query int false "返回数量限制" default(50)
// @Success 200 {object} FileAuditQueryResult
// @Router /file-audit/search [get]
// @Security BearerAuth
func (h *FileAuditHandlers) searchLogs(c *gin.Context) {
	keyword := c.Query("q")
	if keyword == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, "缺少搜索关键词"))
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	opts := FileAuditQueryOptions{
		Keyword: keyword,
		Limit:   limit,
	}

	result, err := h.logger.Query(opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, result)
}

// ========== 导出 ==========

// exportLogs 导出日志
// @Summary 导出审计日志
// @Description 导出指定时间范围的文件审计日志
// @Tags file-audit
// @Accept json
// @Produce octet-stream
// @Param format query string false "导出格式 (json/csv)" default(json)
// @Param start_time query string false "开始时间 (RFC3339)"
// @Param end_time query string false "结束时间 (RFC3339)"
// @Param protocol query string false "协议类型 (smb/nfs)"
// @Param compress query bool false "是否压缩"
// @Success 200 {file} file
// @Failure 500 {object} APIResponse
// @Router /file-audit/export [get]
// @Security BearerAuth
func (h *FileAuditHandlers) exportLogs(c *gin.Context) {
	format := c.DefaultQuery("format", "json")

	// 时间范围
	startTime := time.Now().Add(-24 * time.Hour)
	endTime := time.Now()

	if st := c.Query("start_time"); st != "" {
		t, err := time.Parse(time.RFC3339, st)
		if err == nil {
			startTime = t
		}
	}
	if et := c.Query("end_time"); et != "" {
		t, err := time.Parse(time.RFC3339, et)
		if err == nil {
			endTime = t
		}
	}

	var protocol Protocol
	if p := c.Query("protocol"); p != "" {
		protocol = Protocol(p)
	}

	compress := c.Query("compress") == "true"

	opts := FileExportOptions{
		Format:    format,
		StartTime: startTime,
		EndTime:   endTime,
		Protocol:  protocol,
		Compress:  compress,
	}

	data, err := h.logger.Export(opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, "导出失败"))
		return
	}

	// 设置响应头
	contentType := "application/json"
	fileExt := "json"
	switch format {
	case "csv":
		contentType = "text/csv"
		fileExt = "csv"
	}

	filename := "file-audit-" + startTime.Format("20060102") + "-" + endTime.Format("20060102") + "." + fileExt

	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, contentType, data)
}

// ========== 存储管理 ==========

// getStorageInfo 获取存储信息
// @Summary 获取存储信息
// @Description 获取审计日志存储的使用情况
// @Tags file-audit
// @Accept json
// @Produce json
// @Success 200 {object} StorageInfo
// @Router /file-audit/storage/info [get]
// @Security BearerAuth
func (h *FileAuditHandlers) getStorageInfo(c *gin.Context) {
	info, err := h.storage.GetStorageInfo()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, info)
}

// getAvailableDates 获取可用日期
// @Summary 获取可用日期列表
// @Description 获取有审计日志记录的日期列表
// @Tags file-audit
// @Accept json
// @Produce json
// @Success 200 {object} []string
// @Router /file-audit/storage/dates [get]
// @Security BearerAuth
func (h *FileAuditHandlers) getAvailableDates(c *gin.Context) {
	dates, err := h.storage.ListAvailableDates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, dates)
}

// archiveRequest 归档请求
type archiveRequest struct {
	StartMonth string `json:"start_month" binding:"required"` // 格式: 2026-01
	EndMonth   string `json:"end_month" binding:"required"`   // 格式: 2026-03
}

// archiveLogs 归档日志
// @Summary 归档日志
// @Description 归档指定月份的审计日志
// @Tags file-audit
// @Accept json
// @Produce json
// @Param request body archiveRequest true "归档参数"
// @Success 200 {object} APIResponse
// @Failure 400 {object} APIResponse
// @Router /file-audit/storage/archive [post]
// @Security BearerAuth
func (h *FileAuditHandlers) archiveLogs(c *gin.Context) {
	var req archiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	if err := h.storage.Archive(req.StartMonth, req.EndMonth); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(nil))
}

// cleanupLogs 清理日志
// @Summary 清理过期日志
// @Description 手动触发清理过期的审计日志
// @Tags file-audit
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse
// @Router /file-audit/storage/cleanup [post]
// @Security BearerAuth
func (h *FileAuditHandlers) cleanupLogs(c *gin.Context) {
	if err := h.storage.Cleanup(); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(nil))
}

// ========== 配置管理 ==========

// getConfig 获取配置
// @Summary 获取审计配置
// @Description 获取文件审计日志的当前配置
// @Tags file-audit
// @Accept json
// @Produce json
// @Success 200 {object} FileAuditConfig
// @Router /file-audit/config [get]
// @Security BearerAuth
func (h *FileAuditHandlers) getConfig(c *gin.Context) {
	config := h.logger.GetConfig()
	c.JSON(http.StatusOK, config)
}

// updateConfig 更新配置
// @Summary 更新审计配置
// @Description 更新文件审计日志的配置
// @Tags file-audit
// @Accept json
// @Produce json
// @Param config body FileAuditConfig true "配置"
// @Success 200 {object} APIResponse
// @Router /file-audit/config [put]
// @Security BearerAuth
func (h *FileAuditHandlers) updateConfig(c *gin.Context) {
	var config FileAuditConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	h.logger.SetConfig(config)

	c.JSON(http.StatusOK, SuccessResponse(nil))
}

// enable 启用审计
// @Summary 启用审计
// @Description 启用文件审计日志记录
// @Tags file-audit
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse
// @Router /file-audit/enable [post]
// @Security BearerAuth
func (h *FileAuditHandlers) enable(c *gin.Context) {
	h.logger.Enable()
	c.JSON(http.StatusOK, SuccessResponse(nil))
}

// disable 禁用审计
// @Summary 禁用审计
// @Description 禁用文件审计日志记录
// @Tags file-audit
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse
// @Router /file-audit/disable [post]
// @Security BearerAuth
func (h *FileAuditHandlers) disable(c *gin.Context) {
	h.logger.Disable()
	c.JSON(http.StatusOK, SuccessResponse(nil))
}

// ========== 操作记录接口 ==========

// smbOperationRequest SMB操作请求
type smbOperationRequest struct {
	ShareName string                 `json:"share_name" binding:"required"`
	SharePath string                 `json:"share_path" binding:"required"`
	UserID    string                 `json:"user_id" binding:"required"`
	Username  string                 `json:"username" binding:"required"`
	ClientIP  string                 `json:"client_ip" binding:"required"`
	Operation FileOperation          `json:"operation" binding:"required"`
	FilePath  string                 `json:"file_path" binding:"required"`
	Status    Status                 `json:"status" binding:"required"`
	Details   map[string]interface{} `json:"details"`
}

// logSMBOperation 记录SMB操作
// @Summary 记录SMB操作
// @Description 记录SMB文件操作审计日志
// @Tags file-audit
// @Accept json
// @Produce json
// @Param request body smbOperationRequest true "操作信息"
// @Success 201 {object} APIResponse
// @Router /file-audit/log/smb [post]
// @Security BearerAuth
func (h *FileAuditHandlers) logSMBOperation(c *gin.Context) {
	var req smbOperationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	err := h.logger.LogSMBOperation(
		c.Request.Context(),
		req.ShareName,
		req.SharePath,
		req.UserID,
		req.Username,
		req.ClientIP,
		req.Operation,
		req.FilePath,
		req.Status,
		req.Details,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, SuccessResponse(nil))
}

// nfsOperationRequest NFS操作请求
type nfsOperationRequest struct {
	SharePath string                 `json:"share_path" binding:"required"`
	UserID    string                 `json:"user_id" binding:"required"`
	Username  string                 `json:"username" binding:"required"`
	ClientIP  string                 `json:"client_ip" binding:"required"`
	Operation FileOperation          `json:"operation" binding:"required"`
	FilePath  string                 `json:"file_path" binding:"required"`
	Status    Status                 `json:"status" binding:"required"`
	Details   map[string]interface{} `json:"details"`
}

// logNFSOperation 记录NFS操作
// @Summary 记录NFS操作
// @Description 记录NFS文件操作审计日志
// @Tags file-audit
// @Accept json
// @Produce json
// @Param request body nfsOperationRequest true "操作信息"
// @Success 201 {object} APIResponse
// @Router /file-audit/log/nfs [post]
// @Security BearerAuth
func (h *FileAuditHandlers) logNFSOperation(c *gin.Context) {
	var req nfsOperationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	err := h.logger.LogNFSOperation(
		c.Request.Context(),
		req.SharePath,
		req.UserID,
		req.Username,
		req.ClientIP,
		req.Operation,
		req.FilePath,
		req.Status,
		req.Details,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, SuccessResponse(nil))
}

// fileOperationRequest 文件操作请求
type fileOperationRequest struct {
	Protocol  Protocol `json:"protocol" binding:"required"`
	ShareName string   `json:"share_name"`
	SharePath string   `json:"share_path" binding:"required"`
	UserID    string   `json:"user_id" binding:"required"`
	Username  string   `json:"username" binding:"required"`
	ClientIP  string   `json:"client_ip" binding:"required"`
	FilePath  string   `json:"file_path" binding:"required"`
	IsDir     bool     `json:"is_directory"`
	Status    Status   `json:"status" binding:"required"`
}

// logFileCreate 记录文件创建
// @Summary 记录文件创建
// @Description 记录文件/目录创建操作
// @Tags file-audit
// @Accept json
// @Produce json
// @Param request body fileOperationRequest true "操作信息"
// @Success 201 {object} APIResponse
// @Router /file-audit/log/file/create [post]
// @Security BearerAuth
func (h *FileAuditHandlers) logFileCreate(c *gin.Context) {
	var req fileOperationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	err := h.logger.LogFileCreate(
		c.Request.Context(),
		req.Protocol,
		req.ShareName,
		req.SharePath,
		req.UserID,
		req.Username,
		req.ClientIP,
		req.FilePath,
		req.IsDir,
		req.Status,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, SuccessResponse(nil))
}

// logFileDelete 记录文件删除
// @Summary 记录文件删除
// @Description 记录文件/目录删除操作
// @Tags file-audit
// @Accept json
// @Produce json
// @Param request body fileOperationRequest true "操作信息"
// @Success 201 {object} APIResponse
// @Router /file-audit/log/file/delete [post]
// @Security BearerAuth
func (h *FileAuditHandlers) logFileDelete(c *gin.Context) {
	var req fileOperationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	err := h.logger.LogFileDelete(
		c.Request.Context(),
		req.Protocol,
		req.ShareName,
		req.SharePath,
		req.UserID,
		req.Username,
		req.ClientIP,
		req.FilePath,
		req.IsDir,
		req.Status,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, SuccessResponse(nil))
}

// fileRenameRequest 文件重命名请求
type fileRenameRequest struct {
	Protocol  Protocol `json:"protocol" binding:"required"`
	ShareName string   `json:"share_name"`
	SharePath string   `json:"share_path" binding:"required"`
	UserID    string   `json:"user_id" binding:"required"`
	Username  string   `json:"username" binding:"required"`
	ClientIP  string   `json:"client_ip" binding:"required"`
	OldPath   string   `json:"old_path" binding:"required"`
	NewPath   string   `json:"new_path" binding:"required"`
	Status    Status   `json:"status" binding:"required"`
}

// logFileRename 记录文件重命名
// @Summary 记录文件重命名
// @Description 记录文件重命名操作
// @Tags file-audit
// @Accept json
// @Produce json
// @Param request body fileRenameRequest true "操作信息"
// @Success 201 {object} APIResponse
// @Router /file-audit/log/file/rename [post]
// @Security BearerAuth
func (h *FileAuditHandlers) logFileRename(c *gin.Context) {
	var req fileRenameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	err := h.logger.LogFileRename(
		c.Request.Context(),
		req.Protocol,
		req.ShareName,
		req.SharePath,
		req.UserID,
		req.Username,
		req.ClientIP,
		req.OldPath,
		req.NewPath,
		req.Status,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, SuccessResponse(nil))
}

// fileMoveRequest 文件移动请求
type fileMoveRequest struct {
	Protocol  Protocol `json:"protocol" binding:"required"`
	ShareName string   `json:"share_name"`
	SharePath string   `json:"share_path" binding:"required"`
	UserID    string   `json:"user_id" binding:"required"`
	Username  string   `json:"username" binding:"required"`
	ClientIP  string   `json:"client_ip" binding:"required"`
	OldPath   string   `json:"old_path" binding:"required"`
	NewPath   string   `json:"new_path" binding:"required"`
	Status    Status   `json:"status" binding:"required"`
}

// logFileMove 记录文件移动
// @Summary 记录文件移动
// @Description 记录文件移动操作
// @Tags file-audit
// @Accept json
// @Produce json
// @Param request body fileMoveRequest true "操作信息"
// @Success 201 {object} APIResponse
// @Router /file-audit/log/file/move [post]
// @Security BearerAuth
func (h *FileAuditHandlers) logFileMove(c *gin.Context) {
	var req fileMoveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	err := h.logger.LogFileMove(
		c.Request.Context(),
		req.Protocol,
		req.ShareName,
		req.SharePath,
		req.UserID,
		req.Username,
		req.ClientIP,
		req.OldPath,
		req.NewPath,
		req.Status,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, SuccessResponse(nil))
}

// fileWriteRequest 文件写入请求
type fileWriteRequest struct {
	Protocol  Protocol `json:"protocol" binding:"required"`
	ShareName string   `json:"share_name"`
	SharePath string   `json:"share_path" binding:"required"`
	UserID    string   `json:"user_id" binding:"required"`
	Username  string   `json:"username" binding:"required"`
	ClientIP  string   `json:"client_ip" binding:"required"`
	FilePath  string   `json:"file_path" binding:"required"`
	FileSize  int64    `json:"file_size"`
	Status    Status   `json:"status" binding:"required"`
}

// logFileWrite 记录文件写入
// @Summary 记录文件写入
// @Description 记录文件写入操作
// @Tags file-audit
// @Accept json
// @Produce json
// @Param request body fileWriteRequest true "操作信息"
// @Success 201 {object} APIResponse
// @Router /file-audit/log/file/write [post]
// @Security BearerAuth
func (h *FileAuditHandlers) logFileWrite(c *gin.Context) {
	var req fileWriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(ErrCodeInvalidParam, err.Error()))
		return
	}

	err := h.logger.LogFileWrite(
		c.Request.Context(),
		req.Protocol,
		req.ShareName,
		req.SharePath,
		req.UserID,
		req.Username,
		req.ClientIP,
		req.FilePath,
		req.FileSize,
		req.Status,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse(ErrCodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, SuccessResponse(nil))
}
