package containerhealthmon

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// HealthStatus 健康状态
type HealthStatus string

const (
	HealthOK        HealthStatus = "healthy"
	HealthDegraded  HealthStatus = "degraded"
	HealthCritical  HealthStatus = "critical"
	HealthUnknown   HealthStatus = "unknown"
)

// ContainerInfo 容器信息
type ContainerInfo struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Image        string       `json:"image"`
	Status       string       `json:"status"`
	Health       HealthStatus `json:"health"`
	CPUPercent   float64      `json:"cpu_percent"`
	MemMB        int64        `json:"mem_mb"`
	RestartCount int          `json:"restart_count"`
	LastCheck    time.Time    `json:"last_check"`
	Uptime       string       `json:"uptime"`
}

// HealthRule 健康规则
type HealthRule struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Enabled        bool   `json:"enabled"`
	MaxCPU         float64 `json:"max_cpu"`
	MaxMemMB       int64  `json:"max_mem_mb"`
	MaxRestarts    int    `json:"max_restarts"`
	AutoRestart    bool   `json:"auto_restart"`
	AlertOnFailure bool   `json:"alert_on_failure"`
}

// MonitorEvent 监控事件
type MonitorEvent struct {
	ID        string       `json:"id"`
	Timestamp time.Time    `json:"timestamp"`
	Container string       `json:"container"`
	EventType string       `json:"event_type"`
	Status    HealthStatus `json:"status"`
	Message   string       `json:"message"`
}

// Manager 容器健康监控管理器
type Manager struct {
	mu         sync.RWMutex
	logger     *zap.Logger
	containers map[string]*ContainerInfo
	rules      map[string]*HealthRule
	events     []*MonitorEvent
	dataPath   string
}

// NewManager 创建管理器
func NewManager(logger *zap.Logger, dataPath string) *Manager {
	m := &Manager{
		logger:     logger,
		containers: make(map[string]*ContainerInfo),
		rules:      make(map[string]*HealthRule),
		dataPath:   dataPath,
	}
	_ = m.loadData()
	return m
}

// RegisterContainer 注册容器
func (m *Manager) RegisterContainer(name, image string) *ContainerInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := &ContainerInfo{
		ID:        genID(),
		Name:      name,
		Image:     image,
		Status:    "registered",
		Health:    HealthUnknown,
		LastCheck: time.Now(),
	}
	m.containers[c.ID] = c
	_ = m.saveData()
	return c
}

// UpdateHealth 更新健康状态
func (m *Manager) UpdateHealth(id string, status HealthStatus, cpu float64, memMB int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.containers[id]
	if !ok {
		return fmt.Errorf("container not found")
	}
	prev := c.Health
	c.Health = status
	c.CPUPercent = cpu
	c.MemMB = memMB
	c.LastCheck = time.Now()
	if status != prev {
		event := &MonitorEvent{
			ID:        genID(),
			Timestamp: time.Now(),
			Container: c.Name,
			EventType: "health_change",
			Status:    status,
			Message:   fmt.Sprintf("Health changed from %s to %s", prev, status),
		}
		m.events = append(m.events, event)
	}
	_ = m.saveData()
	return nil
}

// IncrementRestart 增加重启计数
func (m *Manager) IncrementRestart(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.containers[id]
	if !ok {
		return fmt.Errorf("container not found")
	}
	c.RestartCount++
	c.Status = "restarted"
	m.events = append(m.events, &MonitorEvent{
		ID:        genID(),
		Timestamp: time.Now(),
		Container: c.Name,
		EventType: "restart",
		Status:    c.Health,
		Message:   fmt.Sprintf("Restart #%d", c.RestartCount),
	})
	_ = m.saveData()
	return nil
}

// ListContainers 列出容器
func (m *Manager) ListContainers(health HealthStatus) []*ContainerInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*ContainerInfo
	for _, c := range m.containers {
		if health != "" && c.Health != health {
			continue
		}
		result = append(result, c)
	}
	return result
}

// CreateRule 创建规则
func (m *Manager) CreateRule(name string, maxCPU float64, maxMemMB int64, maxRestarts int) *HealthRule {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := &HealthRule{
		ID:             genID(),
		Name:           name,
		Enabled:        true,
		MaxCPU:         maxCPU,
		MaxMemMB:       maxMemMB,
		MaxRestarts:    maxRestarts,
		AutoRestart:    true,
		AlertOnFailure: true,
	}
	m.rules[r.ID] = r
	_ = m.saveData()
	return r
}

// GetEvents 获取事件
func (m *Manager) GetEvents(limit int) []*MonitorEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit <= 0 || limit > len(m.events) {
		limit = len(m.events)
	}
	start := len(m.events) - limit
	result := make([]*MonitorEvent, limit)
	copy(result, m.events[start:])
	return result
}

// GetStats 获取统计
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	stats := map[string]interface{}{
		"total":       len(m.containers),
		"healthy":     0,
		"degraded":    0,
		"critical":    0,
		"total_events": len(m.events),
	}
	for _, c := range m.containers {
		switch c.Health {
		case HealthOK:
			stats["healthy"] = stats["healthy"].(int) + 1
		case HealthDegraded:
			stats["degraded"] = stats["degraded"].(int) + 1
		case HealthCritical:
			stats["critical"] = stats["critical"].(int) + 1
		}
	}
	return stats
}

func (m *Manager) loadData() error {
	if m.dataPath == "" {
		return nil
	}
	dataFile := filepath.Join(m.dataPath, "container_healthmon.json")
	data, err := os.ReadFile(dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var stored struct {
		Containers map[string]*ContainerInfo `json:"containers"`
		Rules      map[string]*HealthRule    `json:"rules"`
		Events     []*MonitorEvent           `json:"events"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		return err
	}
	if stored.Containers != nil {
		m.containers = stored.Containers
	}
	if stored.Rules != nil {
		m.rules = stored.Rules
	}
	m.events = stored.Events
	return nil
}

func (m *Manager) saveData() error {
	if m.dataPath == "" {
		return nil
	}
	_ = os.MkdirAll(m.dataPath, 0o755)
	stored := struct {
		Containers map[string]*ContainerInfo `json:"containers"`
		Rules      map[string]*HealthRule    `json:"rules"`
		Events     []*MonitorEvent           `json:"events"`
	}{Containers: m.containers, Rules: m.rules, Events: m.events}
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(m.dataPath, "container_healthmon.json"), data, 0o644)
}

func genID() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

// Handlers API
type Handlers struct{ mgr *Manager }

func NewHandlers(mgr *Manager) *Handlers { return &Handlers{mgr: mgr} }

func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/container-health")
	{
		g.GET("", h.list)
		g.POST("", h.register)
		g.PUT("/:id/health", h.updateHealth)
		g.PUT("/:id/restart", h.restart)
		g.GET("/events", h.events)
		g.GET("/stats", h.stats)
	}
	r := rg.Group("/health-rules")
	{
		r.POST("", h.createRule)
	}
}

func (h *Handlers) list(c *gin.Context) {
	health := HealthStatus(c.Query("health"))
	c.JSON(http.StatusOK, gin.H{"containers": h.mgr.ListContainers(health)})
}

func (h *Handlers) register(c *gin.Context) {
	var req struct {
		Name  string `json:"name" binding:"required"`
		Image string `json:"image" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, h.mgr.RegisterContainer(req.Name, req.Image))
}

func (h *Handlers) updateHealth(c *gin.Context) {
	var req struct {
		Status HealthStatus `json:"status" binding:"required"`
		CPU    float64      `json:"cpu"`
		MemMB  int64        `json:"mem_mb"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.mgr.UpdateHealth(c.Param("id"), req.Status, req.CPU, req.MemMB); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handlers) restart(c *gin.Context) {
	if err := h.mgr.IncrementRestart(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handlers) events(c *gin.Context) {
	limit := 50
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	c.JSON(http.StatusOK, gin.H{"events": h.mgr.GetEvents(limit)})
}

func (h *Handlers) stats(c *gin.Context) {
	c.JSON(http.StatusOK, h.mgr.GetStats())
}

func (h *Handlers) createRule(c *gin.Context) {
	var req struct {
		Name        string  `json:"name" binding:"required"`
		MaxCPU      float64 `json:"max_cpu"`
		MaxMemMB    int64   `json:"max_mem_mb"`
		MaxRestarts int     `json:"max_restarts"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, h.mgr.CreateRule(req.Name, req.MaxCPU, req.MaxMemMB, req.MaxRestarts))
}
