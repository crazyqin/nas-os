// Package nvmetempool 提供 NVMe-oF 存储池 HTTP 处理器
package nvmetempool

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers NVMe-oF 存储池 HTTP 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	nvmeGroup := api.Group("/nvme-of")
	{
		// 目标端管理
		nvmeGroup.GET("/targets", h.listTargets)
		nvmeGroup.POST("/targets", h.addTarget)
		nvmeGroup.GET("/targets/:id", h.getTarget)
		nvmeGroup.DELETE("/targets/:id", h.removeTarget)

		// 设备管理
		nvmeGroup.GET("/devices", h.listDevices)
		nvmeGroup.POST("/devices", h.addDevice)
		nvmeGroup.GET("/devices/:id", h.getDevice)
		nvmeGroup.DELETE("/devices/:id", h.removeDevice)

		// 存储池管理
		nvmeGroup.GET("/pools", h.listPools)
		nvmeGroup.POST("/pools", h.createPool)
		nvmeGroup.GET("/pools/:id", h.getPool)
		nvmeGroup.DELETE("/pools/:id", h.deletePool)

		// 性能监控
		nvmeGroup.GET("/pools/:id/performance", h.getPoolPerformance)

		// 故障切换
		nvmeGroup.GET("/failovers", h.getFailoverEvents)

		// 设备发现
		nvmeGroup.POST("/discover", h.discoverTargets)
	}
}

// listTargets 列出所有目标端.
func (h *Handlers) listTargets(c *gin.Context) {
	targets := h.manager.ListTargets()
	c.JSON(http.StatusOK, gin.H{
		"targets": targets,
		"total":   len(targets),
	})
}

// addTargetRequest 添加目标端请求.
type addTargetRequest struct {
	ID        string        `json:"id" binding:"required"`
	Name      string        `json:"name" binding:"required"`
	Address   string        `json:"address" binding:"required"`
	Port      int           `json:"port" binding:"required"`
	Transport TransportType `json:"transport" binding:"required"`
	Subsystem string        `json:"subsystem"`
}

// addTarget 添加目标端.
func (h *Handlers) addTarget(c *gin.Context) {
	var req addTargetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	target := &NvmeTarget{
		ID:        req.ID,
		Name:      req.Name,
		Address:   req.Address,
		Port:      req.Port,
		Transport: req.Transport,
		Subsystem: req.Subsystem,
	}

	if err := h.manager.AddTarget(target); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "目标端添加成功",
		"target":  target,
	})
}

// getTarget 获取目标端.
func (h *Handlers) getTarget(c *gin.Context) {
	targetID := c.Param("id")
	target, err := h.manager.GetTarget(targetID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, target)
}

// removeTarget 移除目标端.
func (h *Handlers) removeTarget(c *gin.Context) {
	targetID := c.Param("id")
	if err := h.manager.RemoveTarget(targetID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":   "目标端已移除",
		"target_id": targetID,
	})
}

// listDevices 列出所有设备.
func (h *Handlers) listDevices(c *gin.Context) {
	devices := h.manager.ListDevices()
	c.JSON(http.StatusOK, gin.H{
		"devices": devices,
		"total":   len(devices),
	})
}

// addDeviceRequest 添加设备请求.
type addDeviceRequest struct {
	ID        string `json:"id" binding:"required"`
	Model     string `json:"model" binding:"required"`
	Serial    string `json:"serial"`
	Namespace string `json:"namespace"`
	Capacity  uint64 `json:"capacity" binding:"required"`
	TargetID  string `json:"targetId" binding:"required"`
}

// addDevice 添加设备.
func (h *Handlers) addDevice(c *gin.Context) {
	var req addDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	device := &NvmeDevice{
		ID:        req.ID,
		Model:     req.Model,
		Serial:    req.Serial,
		Namespace: req.Namespace,
		Capacity:  req.Capacity,
		TargetID:  req.TargetID,
	}

	if err := h.manager.AddDevice(device); err != nil {
		code := http.StatusConflict
		if err.Error() == "target not found: "+req.TargetID {
			code = http.StatusBadRequest
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "设备添加成功",
		"device":  device,
	})
}

// getDevice 获取设备.
func (h *Handlers) getDevice(c *gin.Context) {
	deviceID := c.Param("id")
	device, err := h.manager.GetDevice(deviceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, device)
}

// removeDevice 移除设备.
func (h *Handlers) removeDevice(c *gin.Context) {
	deviceID := c.Param("id")
	if err := h.manager.RemoveDevice(deviceID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":   "设备已移除",
		"device_id": deviceID,
	})
}

// listPools 列出所有存储池.
func (h *Handlers) listPools(c *gin.Context) {
	pools := h.manager.ListPools()
	c.JSON(http.StatusOK, gin.H{
		"pools": pools,
		"total": len(pools),
	})
}

// createPoolRequest 创建存储池请求.
type createPoolRequest struct {
	ID         string   `json:"id" binding:"required"`
	Name       string   `json:"name" binding:"required"`
	Devices    []string `json:"devices" binding:"required"`
	Redundancy string   `json:"redundancy"`
}

// createPool 创建存储池.
func (h *Handlers) createPool(c *gin.Context) {
	var req createPoolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	pool := &NvmePool{
		ID:         req.ID,
		Name:       req.Name,
		Devices:    req.Devices,
		Redundancy: req.Redundancy,
	}

	if err := h.manager.CreatePool(pool); err != nil {
		code := http.StatusConflict
		if err.Error() == "pool ID is required" || err.Error() == "pool already exists: "+req.ID {
			code = http.StatusBadRequest
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "存储池创建成功",
		"pool":    pool,
	})
}

// getPool 获取存储池.
func (h *Handlers) getPool(c *gin.Context) {
	poolID := c.Param("id")
	pool, err := h.manager.GetPool(poolID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, pool)
}

// deletePool 删除存储池.
func (h *Handlers) deletePool(c *gin.Context) {
	poolID := c.Param("id")
	if err := h.manager.DeletePool(poolID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "存储池已删除",
		"pool_id": poolID,
	})
}

// getPoolPerformance 获取存储池性能.
func (h *Handlers) getPoolPerformance(c *gin.Context) {
	poolID := c.Param("id")
	perf, err := h.manager.GetPoolPerformance(poolID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, perf)
}

// getFailoverEvents 获取故障切换事件.
func (h *Handlers) getFailoverEvents(c *gin.Context) {
	events := h.manager.GetFailoverEvents()
	c.JSON(http.StatusOK, gin.H{
		"events": events,
		"total":  len(events),
	})
}

// discoverTargetsRequest 发现目标请求.
type discoverTargetsRequest struct {
	Address   string        `json:"address" binding:"required"`
	Transport TransportType `json:"transport" binding:"required"`
}

// discoverTargets 发现目标.
func (h *Handlers) discoverTargets(c *gin.Context) {
	var req discoverTargetsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	targets, err := h.manager.DiscoverTargets(req.Address, req.Transport)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"targets": targets,
		"total":   len(targets),
	})
}

// UpdateDeviceStatus 更新设备状态 (供外部调用).
func (m *Manager) UpdateDeviceStatus(deviceID string, status DeviceStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.devices[deviceID]
	if !ok {
		return fmt.Errorf("device not found: %s", deviceID)
	}

	device.Status = status
	device.UpdatedAt = time.Now()

	// 更新存储池状态
	m.updatePoolStatus()

	return nil
}

// updatePoolStatus 更新存储池状态.
func (m *Manager) updatePoolStatus() {
	for _, pool := range m.pools {
		var onlineCount int
		for _, deviceID := range pool.Devices {
			if device, exists := m.devices[deviceID]; exists {
				if device.Status == DeviceStatusOnline {
					onlineCount++
				}
			}
		}

		if onlineCount == len(pool.Devices) {
			pool.Status = PoolStatusActive
		} else if onlineCount > 0 {
			pool.Status = PoolStatusDegraded
		} else {
			pool.Status = PoolStatusFault
		}
		pool.UpdatedAt = time.Now()
	}
}

// UpdatePoolUsage 更新存储池使用量 (供外部调用).
func (m *Manager) UpdatePoolUsage(poolID string, usedSpace uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, ok := m.pools[poolID]
	if !ok {
		return fmt.Errorf("pool not found: %s", poolID)
	}

	pool.UsedSpace = usedSpace
	pool.FreeSpace = pool.TotalSpace - usedSpace
	pool.UpdatedAt = time.Now()

	return nil
}

// GetPoolUsage 获取存储池使用率 (供外部调用).
func (m *Manager) GetPoolUsage(poolID string) (float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool, ok := m.pools[poolID]
	if !ok {
		return 0, fmt.Errorf("pool not found: %s", poolID)
	}

	if pool.TotalSpace == 0 {
		return 0, nil
	}

	return float64(pool.UsedSpace) / float64(pool.TotalSpace) * 100, nil
}
