// Package networktopology HTTP API 处理器
package networktopology

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	service *TopologyService
}

// NewHandler 创建处理器
func NewHandler(service *TopologyService) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	group := router.Group("/network-topology")
	{
		// 设备管理
		group.GET("/devices", h.ListDevices)
		group.POST("/devices", h.AddDevice)
		group.PUT("/devices/:id", h.UpdateDevice)
		group.DELETE("/devices/:id", h.DeleteDevice)

		// 连接关系
		group.GET("/links", h.GetLinks)

		// 网络扫描
		group.POST("/scan", h.StartScan)

		// 拓扑数据
		group.GET("/topology", h.GetTopology)
		group.GET("/stats", h.GetStats)
	}
}

// ListDevices 获取设备列表
func (h *Handler) ListDevices(c *gin.Context) {
	h.service.mu.RLock()
	defer h.service.mu.RUnlock()

	devices := make([]*TopologyDevice, 0, len(h.service.devices))
	for _, d := range h.service.devices {
		devices = append(devices, d)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    devices,
		"total":   len(devices),
	})
}

// AddDevice 添加设备
func (h *Handler) AddDevice(c *gin.Context) {
	var device TopologyDevice
	if err := c.ShouldBindJSON(&device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	// 验证必填字段
	if device.IP == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "IP 地址不能为空"})
		return
	}

	// 设置默认值
	if device.ID == "" {
		if device.MAC != "" {
			device.ID = device.MAC
		} else {
			device.ID = device.IP
		}
	}
	if device.DeviceType == "" {
		device.DeviceType = DeviceTypeUnknown
	}
	if device.State == "" {
		device.State = DeviceStateUnknown
	}
	device.FirstSeen = time.Now()
	device.LastSeen = time.Now()

	h.service.mu.Lock()
	h.service.devices[device.ID] = &device
	h.service.mu.Unlock()

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": device})
}

// UpdateDevice 更新设备
func (h *Handler) UpdateDevice(c *gin.Context) {
	id := c.Param("id")

	var update TopologyDevice
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	h.service.mu.Lock()
	existing, exists := h.service.devices[id]
	if !exists {
		h.service.mu.Unlock()
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "设备不存在"})
		return
	}

	// 更新字段
	if update.IP != "" {
		existing.IP = update.IP
	}
	if update.MAC != "" {
		existing.MAC = update.MAC
	}
	if update.Hostname != "" {
		existing.Hostname = update.Hostname
	}
	if update.Vendor != "" {
		existing.Vendor = update.Vendor
	}
	if update.DeviceType != "" {
		existing.DeviceType = update.DeviceType
	}
	if update.State != "" {
		existing.State = update.State
	}
	if update.OS != "" {
		existing.OS = update.OS
	}
	if update.VLAN != "" {
		existing.VLAN = update.VLAN
	}
	if update.Subnet != "" {
		existing.Subnet = update.Subnet
	}
	if update.Tags != nil {
		existing.Tags = update.Tags
	}
	if update.Properties != nil {
		if existing.Properties == nil {
			existing.Properties = make(map[string]string)
		}
		for k, v := range update.Properties {
			existing.Properties[k] = v
		}
	}
	existing.LastSeen = time.Now()

	h.service.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{"success": true, "data": existing})
}

// DeleteDevice 删除设备
func (h *Handler) DeleteDevice(c *gin.Context) {
	id := c.Param("id")

	h.service.mu.Lock()
	_, exists := h.service.devices[id]
	if !exists {
		h.service.mu.Unlock()
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "设备不存在"})
		return
	}
	delete(h.service.devices, id)
	h.service.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// GetLinks 获取连接关系
func (h *Handler) GetLinks(c *gin.Context) {
	h.service.mu.RLock()
	defer h.service.mu.RUnlock()

	edges := make([]TopologyEdge, 0)
	if h.service.topology != nil {
		edges = h.service.topology.Edges
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    edges,
		"total":   len(edges),
	})
}

// StartScan 触发网络扫描
func (h *Handler) StartScan(c *gin.Context) {
	var req ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	if req.Network == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "网络 CIDR 不能为空"})
		return
	}

	// 创建扫描任务
	scanID := "scan_" + time.Now().Format("20060102150405")
	methods := make([]ScanMethod, 0, len(req.Methods))
	for _, m := range req.Methods {
		methods = append(methods, ScanMethod(m))
	}
	if len(methods) == 0 {
		methods = []ScanMethod{ScanMethodAll}
	}

	task := &ScanTask{
		ID:     scanID,
		Type:   "scan",
		Status: "pending",
		Network: req.Network,
		Config: ScanConfig{
			Network:    req.Network,
			Methods:    methods,
			Timeout:    30 * time.Second,
			Concurrent: 10,
			PortsTop:   req.PortsTop,
			DeepScan:   req.DeepScan,
		},
		StartTime: time.Now(),
	}

	h.service.mu.Lock()
	h.service.tasks[scanID] = task
	h.service.mu.Unlock()

	// 异步执行扫描（实际实现需要调用扫描逻辑）
	go h.executeScan(task)

	c.JSON(http.StatusAccepted, gin.H{
		"success": true,
		"data": gin.H{
			"scanId":  scanID,
			"status":  "pending",
			"network": req.Network,
		},
	})
}

// executeScan 执行扫描任务（占位实现）
func (h *Handler) executeScan(task *ScanTask) {
	h.service.mu.Lock()
	task.Status = "running"
	h.service.mu.Unlock()

	// TODO: 实现实际的网络扫描逻辑
	// 这里只是模拟扫描完成
	time.Sleep(2 * time.Second)

	h.service.mu.Lock()
	task.Status = "completed"
	task.Progress = 100
	task.EndTime = time.Now()
	task.Duration = task.EndTime.Sub(task.StartTime)
	h.service.mu.Unlock()
}

// GetTopology 获取完整拓扑数据
func (h *Handler) GetTopology(c *gin.Context) {
	h.service.mu.RLock()
	defer h.service.mu.RUnlock()

	if h.service.topology == nil {
		// 构建拓扑数据
		topology := &NetworkTopology{
			Nodes:     make([]TopologyNode, 0),
			Edges:     make([]TopologyEdge, 0),
			Subnets:   make([]SubnetInfo, 0),
			VLANs:     make([]VLANInfo, 0),
			UpdatedAt: time.Now(),
		}

		// 从设备列表构建节点
		for _, d := range h.service.devices {
			node := TopologyNode{
				ID:         d.ID,
				IP:         d.IP,
				MAC:        d.MAC,
				Hostname:   d.Hostname,
				DeviceType: d.DeviceType,
				State:      d.State,
				Vendor:     d.Vendor,
				Subnet:     d.Subnet,
				VLAN:       d.VLAN,
			}
			// 提取服务名列表
			for _, svc := range d.Services {
				node.Services = append(node.Services, svc.Name)
			}
			topology.Nodes = append(topology.Nodes, node)
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "data": topology})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": h.service.topology})
}

// GetStats 获取网络统计
func (h *Handler) GetStats(c *gin.Context) {
	h.service.mu.RLock()
	defer h.service.mu.RUnlock()

	// 统计设备类型分布
	deviceTypes := make(map[DeviceType]int)
	onlineCount := 0
	offlineCount := 0

	for _, d := range h.service.devices {
		deviceTypes[d.DeviceType]++
		if d.State == DeviceStateOnline {
			onlineCount++
		} else if d.State == DeviceStateOffline {
			offlineCount++
		}
	}

	// 统计风险数量
	riskCount := len(h.service.risks)

	// 计算安全评分
	securityScore := 100
	if riskCount > 0 {
		criticalCount := 0
		highCount := 0
		mediumCount := 0
		for _, r := range h.service.risks {
			switch r.Level {
			case RiskLevelCritical:
				criticalCount++
			case RiskLevelHigh:
				highCount++
			case RiskLevelMedium:
				mediumCount++
			}
		}
		securityScore = 100 - (criticalCount * 20) - (highCount * 10) - (mediumCount * 5)
		if securityScore < 0 {
			securityScore = 0
		}
	}

	// 获取子网和 VLAN 数量
	subnetCount := 0
	vlanCount := 0
	if h.service.topology != nil {
		subnetCount = len(h.service.topology.Subnets)
		vlanCount = len(h.service.topology.VLANs)
	}

	// 获取最后扫描时间
	var lastScanTime time.Time
	for _, task := range h.service.tasks {
		if task.Type == "scan" && task.Status == "completed" && task.EndTime.After(lastScanTime) {
			lastScanTime = task.EndTime
		}
	}

	stats := TopologyOverview{
		TotalDevices:   len(h.service.devices),
		OnlineDevices:  onlineCount,
		OfflineDevices: offlineCount,
		SubnetCount:    subnetCount,
		VLANCount:      vlanCount,
		RiskCount:      riskCount,
		SecurityScore:  securityScore,
		DeviceTypes:    deviceTypes,
		LastScanTime:   lastScanTime,
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": stats})
}

// GetEvents 获取设备事件（辅助接口）
func (h *Handler) GetEvents(c *gin.Context) {
	eventType := c.Query("type")
	limit := 50
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}

	h.service.mu.RLock()
	defer h.service.mu.RUnlock()

	events := make([]DeviceEvent, 0)
	for i := len(h.service.events) - 1; i >= 0 && len(events) < limit; i-- {
		if eventType == "" || h.service.events[i].EventType == eventType {
			events = append(events, h.service.events[i])
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    events,
		"total":   len(events),
	})
}

// GetRiskReport 获取风险报告（辅助接口）
func (h *Handler) GetRiskReport(c *gin.Context) {
	h.service.mu.RLock()
	defer h.service.mu.RUnlock()

	summary := RiskSummary{}
	for _, r := range h.service.risks {
		if r.Resolved {
			continue
		}
		summary.Total++
		switch r.Level {
		case RiskLevelCritical:
			summary.Critical++
		case RiskLevelHigh:
			summary.High++
		case RiskLevelMedium:
			summary.Medium++
		case RiskLevelLow:
			summary.Low++
		}
	}

	// 计算安全评分
	summary.Score = 100 - (summary.Critical * 20) - (summary.High * 10) - (summary.Medium * 5)
	if summary.Score < 0 {
		summary.Score = 0
	}

	report := RiskReport{
		Summary:     summary,
		Risks:       h.service.risks,
		DeviceCount: len(h.service.devices),
		ScannedAt:   time.Now(),
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": report})
}

// GetPerformance 获取性能数据（辅助接口）
func (h *Handler) GetPerformance(c *gin.Context) {
	deviceID := c.Query("deviceId")
	ip := c.Query("ip")
	minutes := 60
	if m, err := strconv.Atoi(c.Query("minutes")); err == nil && m > 0 {
		minutes = m
	}

	h.service.mu.RLock()
	defer h.service.mu.RUnlock()

	// 查找性能历史
	var history *PerformanceHistory
	if deviceID != "" {
		history = h.service.perfHistory[deviceID]
	} else if ip != "" {
		for _, h := range h.service.perfHistory {
			if h.IP == ip {
				history = h
				break
			}
		}
	}

	if history == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    nil,
			"message": "未找到性能数据",
		})
		return
	}

	// 过滤时间范围
	cutoff := time.Now().Add(-time.Duration(minutes) * time.Minute)
	filtered := make([]PerformanceMetrics, 0)
	for _, m := range history.Metrics {
		if m.Timestamp.After(cutoff) {
			filtered = append(filtered, m)
		}
	}

	result := PerformanceHistory{
		DeviceID: history.DeviceID,
		IP:       history.IP,
		Metrics:  filtered,
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}
