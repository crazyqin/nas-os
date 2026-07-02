package bluetoothprovision

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler 蓝牙配网HTTP处理器.
type Handler struct {
	manager     *Manager
	scanner     *DefaultScanner
	provisioner *DefaultProvisioner
}

// NewHandler 创建HTTP处理器.
func NewHandler(manager *Manager, scanner *DefaultScanner, provisioner *DefaultProvisioner) *Handler {
	return &Handler{
		manager:     manager,
		scanner:     scanner,
		provisioner: provisioner,
	}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	bt := rg.Group("/bluetooth")
	{
		bt.POST("/scan", h.ScanDevices)
		bt.GET("/scan/status", h.ScanStatus)
		bt.POST("/scan/stop", h.StopScan)
		bt.POST("/provision", h.StartProvision)
		bt.GET("/provision/:sessionId", h.GetProvisionStatus)
		bt.POST("/provision/:sessionId/cancel", h.CancelProvision)
		bt.GET("/networks", h.GetNetworks)
		bt.POST("/networks", h.SaveNetwork)
		bt.DELETE("/networks/:id", h.DeleteNetwork)
		bt.GET("/history", h.GetHistory)
		bt.DELETE("/history", h.ClearHistory)
		bt.GET("/events", h.SubscribeEvents) // SSE事件流
	}
}

// ScanDevices 扫描BLE设备
// @Summary 扫描BLE设备
// @Description 启动BLE设备扫描，发现附近的配网设备
// @Tags 蓝牙配网
// @Accept json
// @Produce json
// @Param request body ScanRequest false "扫描配置"
// @Success 200 {object} Response{data=[]BLEDevice}
// @Router /api/v1/bluetooth/scan [post].
func (h *Handler) ScanDevices(c *gin.Context) {
	var req ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 使用默认参数
		req = ScanRequest{
			Duration:   10,
			MaxDevices: 20,
		}
	}

	// 参数默认值
	if req.Duration <= 0 {
		req.Duration = 10
	}
	if req.Duration > 60 {
		req.Duration = 60
	}
	if req.MaxDevices <= 0 {
		req.MaxDevices = 20
	}

	devices, err := h.scanner.Scan(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    http.StatusInternalServerError,
			Message: "扫描失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "扫描完成",
		Data:    devices,
	})
}

// ScanStatus 获取扫描状态.
func (h *Handler) ScanStatus(c *gin.Context) {
	devices := h.scanner.GetDevices()
	c.JSON(http.StatusOK, Response{
		Code: http.StatusOK,
		Data: gin.H{
			"scanning": h.scanner.IsScanning(),
			"devices":  devices,
			"count":    len(devices),
		},
	})
}

// StopScan 停止扫描.
func (h *Handler) StopScan(c *gin.Context) {
	if err := h.scanner.StopScan(); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "扫描已停止",
	})
}

// StartProvision 启动配网
// @Summary 启动配网
// @Description 向BLE设备发送WiFi配置进行配网
// @Tags 蓝牙配网
// @Accept json
// @Produce json
// @Param request body ProvisionRequest true "配网请求"
// @Success 200 {object} Response{data=ProvisionSession}
// @Router /api/v1/bluetooth/provision [post].
func (h *Handler) StartProvision(c *gin.Context) {
	var req ProvisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    http.StatusBadRequest,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	// 参数默认值
	if req.Timeout <= 0 {
		req.Timeout = 60
	}
	if req.RetryCount <= 0 {
		req.RetryCount = 3
	}

	session, err := h.provisioner.StartProvision(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    http.StatusInternalServerError,
			Message: "启动配网失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "配网已启动",
		Data:    session,
	})
}

// GetProvisionStatus 获取配网状态.
func (h *Handler) GetProvisionStatus(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, Response{
			Code:    http.StatusBadRequest,
			Message: "会话ID不能为空",
		})
		return
	}

	session, err := h.provisioner.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code: http.StatusOK,
		Data: session,
	})
}

// CancelProvision 取消配网.
func (h *Handler) CancelProvision(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, Response{
			Code:    http.StatusBadRequest,
			Message: "会话ID不能为空",
		})
		return
	}

	if err := h.provisioner.CancelProvision(sessionID); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "配网已取消",
	})
}

// GetNetworks 获取已保存的网络列表.
func (h *Handler) GetNetworks(c *gin.Context) {
	h.manager.mu.RLock()
	networks := make([]WiFiConfig, len(h.manager.networks))
	copy(networks, h.manager.networks)
	h.manager.mu.RUnlock()

	c.JSON(http.StatusOK, Response{
		Code: http.StatusOK,
		Data: networks,
	})
}

// SaveNetwork 保存网络配置.
func (h *Handler) SaveNetwork(c *gin.Context) {
	var config WiFiConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    http.StatusBadRequest,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	if config.SSID == "" {
		c.JSON(http.StatusBadRequest, Response{
			Code:    http.StatusBadRequest,
			Message: "WiFi SSID不能为空",
		})
		return
	}

	h.manager.mu.Lock()
	// 检查是否已存在
	for i, n := range h.manager.networks {
		if n.SSID == config.SSID {
			h.manager.networks[i] = config
			h.manager.mu.Unlock()
			c.JSON(http.StatusOK, Response{
				Code:    http.StatusOK,
				Message: "网络配置已更新",
			})
			return
		}
	}
	h.manager.networks = append(h.manager.networks, config)
	h.manager.mu.Unlock()

	c.JSON(http.StatusCreated, Response{
		Code:    http.StatusCreated,
		Message: "网络配置已保存",
	})
}

// DeleteNetwork 删除网络配置.
func (h *Handler) DeleteNetwork(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, Response{
			Code:    http.StatusBadRequest,
			Message: "网络ID不能为空",
		})
		return
	}

	h.manager.mu.Lock()
	defer h.manager.mu.Unlock()

	for i, n := range h.manager.networks {
		if n.SSID == id {
			h.manager.networks = append(h.manager.networks[:i], h.manager.networks[i+1:]...)
			c.JSON(http.StatusOK, Response{
				Code:    http.StatusOK,
				Message: "网络配置已删除",
			})
			return
		}
	}

	c.JSON(http.StatusNotFound, Response{
		Code:    http.StatusNotFound,
		Message: "网络配置不存在",
	})
}

// GetHistory 获取配网历史记录
// @Summary 获取配网历史
// @Description 获取配网历史记录列表
// @Tags 蓝牙配网
// @Produce json
// @Param limit query int false "返回数量限制" default(20)
// @Success 200 {object} Response{data=[]ProvisionHistory}
// @Router /api/v1/bluetooth/history [get].
func (h *Handler) GetHistory(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}

	history, err := h.provisioner.GetHistory(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    http.StatusInternalServerError,
			Message: "获取历史记录失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code: http.StatusOK,
		Data: history,
	})
}

// ClearHistory 清空配网历史.
func (h *Handler) ClearHistory(c *gin.Context) {
	if err := h.provisioner.ClearHistory(); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    http.StatusInternalServerError,
			Message: "清空历史记录失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "历史记录已清空",
	})
}

// SubscribeEvents SSE事件订阅.
func (h *Handler) SubscribeEvents(c *gin.Context) {
	subscriberID := uuid.New().String()
	events := h.manager.Subscribe(subscriberID)
	defer h.manager.Unsubscribe(subscriberID)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	c.Writer.Flush()

	for {
		select {
		case event, ok := <-events:
			if !ok {
				return
			}
			data, _ := json.Marshal(event)
			c.Writer.WriteString("data: " + string(data) + "\n\n")
			c.Writer.Flush()
		case <-c.Request.Context().Done():
			return
		}
	}
}

// Response 统一API响应结构.
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// SetDevice 设置扫描器中的设备（用于测试）.
func (h *Handler) SetDevice(device *BLEDevice) {
	h.scanner.mu.Lock()
	defer h.scanner.mu.Unlock()
	h.scanner.devices[device.ID] = device
}

// GetProvisionSession 获取配网会话（用于测试）.
func (h *Handler) GetProvisionSession(sessionID string) (*ProvisionSession, error) {
	return h.provisioner.GetSession(sessionID)
}

// SetProvisionSession 设置配网会话（用于测试）.
func (h *Handler) SetProvisionSession(session *ProvisionSession) {
	h.provisioner.mu.Lock()
	defer h.provisioner.mu.Unlock()
	h.provisioner.sessions[session.ID] = session
	log.Printf("[Handler] 设置测试会话: %s", session.ID)
}
