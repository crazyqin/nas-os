// Package hafailover 高可用故障转移模块
package hafailover

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// APIResponse 统一API响应格式.
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// RegisterRoutes 注册HTTP路由.
func RegisterRoutes(r *gin.RouterGroup, mgr *Manager) {
	h := &handler{mgr: mgr}

	// 配置管理
	r.GET("/config", h.GetConfig)
	r.PUT("/config", h.UpdateConfig)

	// 节点管理
	r.GET("/nodes", h.ListNodes)
	r.GET("/nodes/:id", h.GetNode)
	r.POST("/nodes", h.RegisterNode)

	// HA状态
	r.GET("/status", h.GetHAStatus)

	// 心跳管理
	r.POST("/heartbeat/:level/start", h.StartHeartbeat)
	r.POST("/heartbeat/:level/stop", h.StopHeartbeat)
	r.GET("/heartbeat/status", h.GetHeartbeatStatus)

	// 故障切换
	r.POST("/failover", h.ManualFailover)
	r.GET("/failover/history", h.GetFailoverHistory)

	// 数据同步
	r.POST("/sync", h.TriggerSync)
	r.GET("/sync/status", h.GetSyncStatus)
}

type handler struct {
	mgr *Manager
}

// GetConfig 获取HA配置.
func (h *handler) GetConfig(c *gin.Context) {
	config := h.mgr.GetConfig()
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    config,
	})
}

// UpdateConfig 更新HA配置.
func (h *handler) UpdateConfig(c *gin.Context) {
	var req HAConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	config, err := h.mgr.UpdateConfig(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    config,
	})
}

// ListNodes 列出所有节点.
func (h *handler) ListNodes(c *gin.Context) {
	nodes := h.mgr.ListNodes()
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    nodes,
	})
}

// GetNode 获取节点信息.
func (h *handler) GetNode(c *gin.Context) {
	id := c.Param("id")

	node, err := h.mgr.GetNode(id)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    -1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    node,
	})
}

// RegisterNode 注册节点.
func (h *handler) RegisterNode(c *gin.Context) {
	var req NodeInfo
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	node, err := h.mgr.RegisterNode(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    node,
	})
}

// GetHAStatus 获取HA集群状态.
func (h *handler) GetHAStatus(c *gin.Context) {
	status := h.mgr.GetHAStatus()
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    status,
	})
}

// StartHeartbeat 启动心跳检测.
func (h *handler) StartHeartbeat(c *gin.Context) {
	level := HeartbeatLevel(c.Param("level"))

	if err := h.mgr.StartHeartbeat(level); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "心跳已启动",
	})
}

// StopHeartbeat 停止心跳检测.
func (h *handler) StopHeartbeat(c *gin.Context) {
	level := HeartbeatLevel(c.Param("level"))

	if err := h.mgr.StopHeartbeat(level); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "心跳已停止",
	})
}

// GetHeartbeatStatus 获取心跳状态.
func (h *handler) GetHeartbeatStatus(c *gin.Context) {
	status := h.mgr.GetHeartbeatStatus()
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    status,
	})
}

// ManualFailover 手动故障切换.
func (h *handler) ManualFailover(c *gin.Context) {
	var req FailoverRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	event, err := h.mgr.ManualFailover(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "故障切换完成",
		Data:    event,
	})
}

// GetFailoverHistory 获取切换历史.
func (h *handler) GetFailoverHistory(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 50
	}

	events := h.mgr.GetFailoverHistory(limit)
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    events,
	})
}

// TriggerSync 触发同步.
func (h *handler) TriggerSync(c *gin.Context) {
	status, err := h.mgr.TriggerSync()
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    -1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "同步已触发",
		Data:    status,
	})
}

// GetSyncStatus 获取同步状态.
func (h *handler) GetSyncStatus(c *gin.Context) {
	status := h.mgr.GetSyncStatus()
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "ok",
		Data:    status,
	})
}
