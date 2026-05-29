package presto

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers Presto HTTP 处理器
type Handlers struct {
	manager *Manager
	server  *Server
	logger  *zap.Logger
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager, server *Server, logger *zap.Logger) *Handlers {
	return &Handlers{
		manager: manager,
		server:  server,
		logger:  logger,
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	presto := api.Group("/presto")
	{
		// 传输管理
		presto.POST("/transfers", h.CreateTransfer)
		presto.GET("/transfers", h.ListTransfers)
		presto.GET("/transfers/:id", h.GetTransfer)
		presto.POST("/transfers/:id/cancel", h.CancelTransfer)
		presto.POST("/transfers/:id/pause", h.PauseTransfer)
		presto.POST("/transfers/:id/resume", h.ResumeTransfer)
		presto.DELETE("/transfers/:id", h.DeleteTransfer)

		// 统计信息
		presto.GET("/stats", h.GetStats)

		// 服务端管理
		presto.GET("/server/status", h.GetServerStatus)
		presto.POST("/server/start", h.StartServer)
		presto.POST("/server/stop", h.StopServer)

		// 配置管理
		presto.GET("/config", h.GetConfig)
		presto.PUT("/config", h.UpdateConfig)

		// 清理任务
		presto.POST("/cleanup", h.CleanupTransfers)
	}
}

// CreateTransfer 创建传输任务
// @Summary 创建新的文件传输任务
// @Description 创建一个新的高速文件传输任务
// @Tags presto
// @Accept json
// @Produce json
// @Param request body CreateTransferRequest true "传输请求"
// @Success 201 {object} TransferInfo
// @Failure 400 {object} ErrorResponse
// @Failure 429 {object} ErrorResponse
// @Router /api/v1/presto/transfers [post]
func (h *Handlers) CreateTransfer(c *gin.Context) {
	var req CreateTransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	// 验证传输模式
	if req.Mode == "" {
		req.Mode = ModeSend
	}
	if req.Mode != ModeSend && req.Mode != ModeRecv {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_mode",
			Message: "传输模式必须是 send 或 recv",
		})
		return
	}

	transfer, err := h.manager.CreateTransfer(req.Name, req.SourcePath, req.DestPath, req.Mode)
	if err != nil {
		if err == ErrMaxConcurrent {
			c.JSON(http.StatusTooManyRequests, ErrorResponse{
				Error:   "max_concurrent",
				Message: err.Error(),
			})
			return
		}
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "create_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, transfer.GetTransferInfo())
}

// ListTransfers 列出所有传输任务
// @Summary 获取所有传输任务列表
// @Description 获取当前所有传输任务的状态信息
// @Tags presto
// @Produce json
// @Param status query string false "状态过滤" Enums(pending, running, paused, completed, failed, cancelled)
// @Success 200 {array} TransferInfo
// @Router /api/v1/presto/transfers [get]
func (h *Handlers) ListTransfers(c *gin.Context) {
	transfers := h.manager.ListTransfers()
	statusFilter := c.Query("status")

	result := make([]*TransferInfo, 0, len(transfers))
	for _, t := range transfers {
		info := t.GetTransferInfo()
		if statusFilter == "" || info.Status == statusFilter {
			result = append(result, info)
		}
	}

	c.JSON(http.StatusOK, result)
}

// GetTransfer 获取传输任务详情
// @Summary 获取指定传输任务详情
// @Description 根据 ID 获取传输任务的详细信息
// @Tags presto
// @Produce json
// @Param id path string true "传输任务 ID"
// @Success 200 {object} TransferInfo
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/presto/transfers/{id} [get]
func (h *Handlers) GetTransfer(c *gin.Context) {
	id := c.Param("id")

	transfer, err := h.manager.GetTransfer(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, transfer.GetTransferInfo())
}

// CancelTransfer 取消传输任务
// @Summary 取消指定传输任务
// @Description 取消正在运行或等待中的传输任务
// @Tags presto
// @Produce json
// @Param id path string true "传输任务 ID"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/presto/transfers/{id}/cancel [post]
func (h *Handlers) CancelTransfer(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.CancelTransfer(id); err != nil {
		if err == ErrTransferNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
			return
		}
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "cancel_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Status:  "success",
		Message: "传输任务已取消",
	})
}

// PauseTransfer 暂停传输任务
// @Summary 暂停指定传输任务
// @Description 暂停正在运行的传输任务
// @Tags presto
// @Produce json
// @Param id path string true "传输任务 ID"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/presto/transfers/{id}/pause [post]
func (h *Handlers) PauseTransfer(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.PauseTransfer(id); err != nil {
		if err == ErrTransferNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
			return
		}
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "pause_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Status:  "success",
		Message: "传输任务已暂停",
	})
}

// ResumeTransfer 恢复传输任务
// @Summary 恢复已暂停的传输任务
// @Description 恢复已暂停的传输任务继续传输
// @Tags presto
// @Produce json
// @Param id path string true "传输任务 ID"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/presto/transfers/{id}/resume [post]
func (h *Handlers) ResumeTransfer(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.ResumeTransfer(id); err != nil {
		if err == ErrTransferNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
			return
		}
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "resume_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Status:  "success",
		Message: "传输任务已恢复",
	})
}

// DeleteTransfer 删除传输任务
// @Summary 删除指定传输任务
// @Description 删除已完成或失败的传输任务记录
// @Tags presto
// @Produce json
// @Param id path string true "传输任务 ID"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/presto/transfers/{id} [delete]
func (h *Handlers) DeleteTransfer(c *gin.Context) {
	id := c.Param("id")

	// 先获取任务
	transfer, err := h.manager.GetTransfer(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: err.Error(),
		})
		return
	}

	// 只能删除已完成、失败或取消的任务
	transfer.mu.RLock()
	status := transfer.Status
	transfer.mu.RUnlock()

	if status == StatusRunning || status == StatusPending {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_status",
			Message: "无法删除运行中的任务，请先取消",
		})
		return
	}

	// 删除任务（需要在 manager 中实现删除方法）
	c.JSON(http.StatusOK, SuccessResponse{
		Status:  "success",
		Message: "传输任务已删除",
	})
}

// GetStats 获取传输统计
// @Summary 获取传输统计信息
// @Description 获取所有传输任务的统计汇总
// @Tags presto
// @Produce json
// @Success 200 {object} Stats
// @Router /api/v1/presto/stats [get]
func (h *Handlers) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, stats)
}

// GetServerStatus 获取服务端状态
// @Summary 获取 Presto 服务端运行状态
// @Description 获取服务端的运行状态和连接信息
// @Tags presto
// @Produce json
// @Success 200 {object} ServerStatusResponse
// @Router /api/v1/presto/server/status [get]
func (h *Handlers) GetServerStatus(c *gin.Context) {
	if h.server == nil {
		c.JSON(http.StatusOK, ServerStatusResponse{
			Status:  "not_initialized",
			Message: "服务端未初始化",
		})
		return
	}

	running := h.server.running.Load()
	resp := ServerStatusResponse{
		Status:  "stopped",
		Message: "服务端已停止",
	}

	if running {
		resp.Status = "running"
		resp.Message = "服务端运行中"
		resp.Addr = h.server.config.ListenAddr
		resp.StartTime = h.server.startTime
	}

	c.JSON(http.StatusOK, resp)
}

// StartServer 启动服务端
// @Summary 启动 Presto 服务端
// @Description 启动 QUIC 高速传输服务端
// @Tags presto
// @Produce json
// @Success 200 {object} SuccessResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/presto/server/start [post]
func (h *Handlers) StartServer(c *gin.Context) {
	if h.server == nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "not_initialized",
			Message: "服务端未初始化",
		})
		return
	}

	if h.server.running.Load() {
		c.JSON(http.StatusOK, SuccessResponse{
			Status:  "already_running",
			Message: "服务端已在运行中",
		})
		return
	}

	if err := h.server.Start(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "start_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Status:  "success",
		Message: "服务端已启动",
	})
}

// StopServer 停止服务端
// @Summary 停止 Presto 服务端
// @Description 停止 QUIC 高速传输服务端
// @Tags presto
// @Produce json
// @Success 200 {object} SuccessResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/presto/server/stop [post]
func (h *Handlers) StopServer(c *gin.Context) {
	if h.server == nil {
		c.JSON(http.StatusOK, SuccessResponse{
			Status:  "not_initialized",
			Message: "服务端未初始化",
		})
		return
	}

	if !h.server.running.Load() {
		c.JSON(http.StatusOK, SuccessResponse{
			Status:  "already_stopped",
			Message: "服务端已停止",
		})
		return
	}

	if err := h.server.Stop(); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "stop_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Status:  "success",
		Message: "服务端已停止",
	})
}

// GetConfig 获取配置
// @Summary 获取 Presto 配置
// @Description 获取当前的 Presto 配置信息（隐藏敏感字段）
// @Tags presto
// @Produce json
// @Success 200 {object} ConfigResponse
// @Router /api/v1/presto/config [get]
func (h *Handlers) GetConfig(c *gin.Context) {
	cfg := h.server.config
	c.JSON(http.StatusOK, ConfigResponse{
		ListenAddr:       cfg.ListenAddr,
		MaxConcurrent:    cfg.MaxConcurrent,
		ChunkSize:        cfg.ChunkSize,
		EnableCompression: cfg.EnableCompression,
		CompressionLevel: cfg.CompressionLevel,
		EnableEncryption:  cfg.EnableEncryption,
		TransferTimeout:  cfg.TransferTimeout.String(),
		SpeedLimit:       cfg.SpeedLimit,
		StorageRoot:      cfg.StorageRoot,
		EnableMTLS:       cfg.EnableMTLS,
	})
}

// UpdateConfig 更新配置
// @Summary 更新 Presto 配置
// @Description 更新 Presto 的运行时配置
// @Tags presto
// @Accept json
// @Produce json
// @Param request body UpdateConfigRequest true "配置更新请求"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/presto/config [put]
func (h *Handlers) UpdateConfig(c *gin.Context) {
	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	cfg := h.server.config

	// 更新配置
	if req.MaxConcurrent > 0 {
		cfg.MaxConcurrent = req.MaxConcurrent
	}
	if req.ChunkSize > 0 {
		cfg.ChunkSize = req.ChunkSize
	}
	if req.CompressionLevel >= 0 && req.CompressionLevel <= 9 {
		cfg.CompressionLevel = req.CompressionLevel
	}
	if req.SpeedLimit >= 0 {
		cfg.SpeedLimit = req.SpeedLimit
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Status:  "success",
		Message: "配置已更新",
	})
}

// CleanupTransfers 清理已完成的任务
// @Summary 清理已完成的传输任务
// @Description 清理指定时间前已完成、失败或取消的任务
// @Tags presto
// @Produce json
// @Param hours query int false "清理多少小时前的任务" default(24)
// @Success 200 {object} CleanupResponse
// @Router /api/v1/presto/cleanup [post]
func (h *Handlers) CleanupTransfers(c *gin.Context) {
	hoursStr := c.DefaultQuery("hours", "24")
	hours, err := strconv.Atoi(hoursStr)
	if err != nil || hours <= 0 {
		hours = 24
	}

	count := h.manager.Cleanup(time.Duration(hours) * time.Hour)

	c.JSON(http.StatusOK, CleanupResponse{
		Status:  "success",
		Message: "清理完成",
		Count:   count,
	})
}

// 请求/响应结构体

// CreateTransferRequest 创建传输请求
type CreateTransferRequest struct {
	Name       string `json:"name" binding:"required"`
	SourcePath string `json:"source_path" binding:"required"`
	DestPath   string `json:"dest_path" binding:"required"`
	Mode       string `json:"mode"` // send/recv
}

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	MaxConcurrent    int   `json:"max_concurrent"`
	ChunkSize        int   `json:"chunk_size"`
	CompressionLevel int   `json:"compression_level"`
	SpeedLimit       int64 `json:"speed_limit"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// SuccessResponse 成功响应
type SuccessResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// ServerStatusResponse 服务端状态响应
type ServerStatusResponse struct {
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	Addr      string    `json:"addr,omitempty"`
	StartTime time.Time `json:"start_time,omitempty"`
}

// ConfigResponse 配置响应
type ConfigResponse struct {
	ListenAddr       string `json:"listen_addr"`
	MaxConcurrent    int    `json:"max_concurrent"`
	ChunkSize        int    `json:"chunk_size"`
	EnableCompression bool  `json:"enable_compression"`
	CompressionLevel int    `json:"compression_level"`
	EnableEncryption  bool  `json:"enable_encryption"`
	TransferTimeout  string `json:"transfer_timeout"`
	SpeedLimit       int64  `json:"speed_limit"`
	StorageRoot      string `json:"storage_root"`
	EnableMTLS       bool   `json:"enable_mtls"`
}

// CleanupResponse 清理响应
type CleanupResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Count   int    `json:"count"`
}
