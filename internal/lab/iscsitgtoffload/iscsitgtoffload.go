// Package iscsitgtoffload 提供 iSCSI Target 硬件卸载管理功能，
// 支持将 iSCSI Target 处理卸载到智能网卡/专用 HBA 卡，
// 减少 CPU 开销，提升 IOPS 和吞吐量。
// 兵部开发。
package iscsitgtoffload

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// OffloadStatus 卸载状态.
type OffloadStatus string

const (
	OffloadStatusDisabled  OffloadStatus = "disabled"
	OffloadStatusEnabling  OffloadStatus = "enabling"
	OffloadStatusEnabled   OffloadStatus = "enabled"
	OffloadStatusFailed    OffloadStatus = "failed"
	OffloadStatusDegraded  OffloadStatus = "degraded"
)

// OffloadType 卸载硬件类型.
type OffloadType string

const (
	OffloadTypeNIC     OffloadType = "smartnic"    // 智能网卡
	OffloadTypeHBAL    OffloadType = "hba"          // 专用 HBA
	OffloadTypeDPU     OffloadType = "dpu"           // DPU 数据处理单元
	OffloadTypeTCPOff  OffloadType = "tcp_offload"   // TCP 卸载引擎
)

// OffloadEngine 卸载引擎.
type OffloadEngine struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Type         OffloadType   `json:"type"`
	Device       string        `json:"device"`
	PCISlot      string        `json:"pci_slot"`
	Status       OffloadStatus  `json:"status"`
	Firmware     string        `json:"firmware"`
	MaxTargets   int           `json:"max_targets"`
	MaxSessions  int           `json:"max_sessions"`
	MaxLunMBps   int           `json:"max_lun_mbps"`
	ActiveTargets int          `json:"active_targets"`
	ActiveSessions int         `json:"active_sessions"`
	CPUUsage     float64       `json:"cpu_usage"` // 卸载引擎自身 CPU 占用
	Temperature  float64       `json:"temperature_c"`
}

// OffloadTarget 卸载 iSCSI Target.
type OffloadTarget struct {
	ID          string   `json:"id"`
	IQN         string   `json:"iqn"`
	LunCount    int      `json:"lun_count"`
	EngineID    string   `json:"engine_id"`
	ReadMBps    float64  `json:"read_mbps"`
	WriteMBps   float64  `json:"write_mbps"`
	ReadIOPS    int64    `json:"read_iops"`
	WriteIOPS   int64    `json:"write_iops"`
	CPUOffload  float64  `json:"cpu_offload_percent"` // CPU 占用减少百分比
}

// OffloadStats 卸载性能统计.
type OffloadStats struct {
	EngineID        string  `json:"engine_id"`
	TotalReadMBps   float64 `json:"total_read_mbps"`
	TotalWriteMBps  float64 `json:"total_write_mbps"`
	TotalReadIOPS   int64   `json:"total_read_iops"`
	TotalWriteIOPS  int64   `json:"total_write_iops"`
	CpuSavedPercent float64 `json:"cpu_saved_percent"`
	SessionCount    int     `json:"session_count"`
	LatencyAvgUs    float64 `json:"latency_avg_us"`
	ErrorsLast1h    int      `json:"errors_last_1h"`
}

// Manager 卸载管理器.
type Manager struct {
	mu       sync.RWMutex
	engines  map[string]*OffloadEngine
	targets  map[string]*OffloadTarget
	stats    map[string]*OffloadStats
}

var idCounter uint64

// NewManager 创建管理器.
func NewManager() *Manager {
	return &Manager{
		engines: make(map[string]*OffloadEngine),
		targets: make(map[string]*OffloadTarget),
		stats:   make(map[string]*OffloadStats),
	}
}

// RegisterEngine 注册卸载引擎.
func (m *Manager) RegisterEngine(e *OffloadEngine) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e.Name == "" {
		return fmt.Errorf("engine name required")
	}
	if e.ID == "" {
		n := atomic.AddUint64(&idCounter, 1)
		e.ID = fmt.Sprintf("offload-%d-%d", time.Now().UnixMilli(), n)
	}
	if e.Status == "" {
		e.Status = OffloadStatusDisabled
	}
	m.engines[e.ID] = e
	return nil
}

// EnableOffload 启用卸载.
func (m *Manager) EnableOffload(engineID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.engines[engineID]
	if !ok {
		return fmt.Errorf("engine %s not found", engineID)
	}
	if e.Status == OffloadStatusEnabled {
		return fmt.Errorf("already enabled")
	}
	e.Status = OffloadStatusEnabling
	// 实际场景需要检查固件、驱动
	e.Status = OffloadStatusEnabled
	return nil
}

// DisableOffload 禁用卸载.
func (m *Manager) DisableOffload(engineID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.engines[engineID]
	if !ok {
		return fmt.Errorf("engine %s not found", engineID)
	}
	if e.Status != OffloadStatusEnabled {
		return fmt.Errorf("not enabled")
	}
	e.Status = OffloadStatusDisabled
	e.ActiveTargets = 0
	e.ActiveSessions = 0
	return nil
}

// AssignTarget 将 Target 分配到卸载引擎.
func (m *Manager) AssignTarget(t *OffloadTarget) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.engines[t.EngineID]
	if !ok {
		return fmt.Errorf("engine %s not found", t.EngineID)
	}
	if e.Status != OffloadStatusEnabled {
		return fmt.Errorf("engine not enabled")
	}
	if e.ActiveTargets >= e.MaxTargets {
		return fmt.Errorf("max targets reached")
	}
	if t.ID == "" {
		t.ID = fmt.Sprintf("tgt-%d", time.Now().UnixMilli())
	}
	m.targets[t.ID] = t
	e.ActiveTargets++
	return nil
}

// RemoveTarget 移除 Target.
func (m *Manager) RemoveTarget(targetID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.targets[targetID]
	if !ok {
		return fmt.Errorf("target %s not found", targetID)
	}
	e := m.engines[t.EngineID]
	if e != nil && e.ActiveTargets > 0 {
		e.ActiveTargets--
	}
	delete(m.targets, targetID)
	return nil
}

// ListEngines 列出引擎.
func (m *Manager) ListEngines() []*OffloadEngine {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*OffloadEngine, 0, len(m.engines))
	for _, e := range m.engines {
		result = append(result, e)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// ListTargets 列出 Target.
func (m *Manager) ListTargets() []*OffloadTarget {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*OffloadTarget, 0, len(m.targets))
	for _, t := range m.targets {
		result = append(result, t)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].IQN < result[j].IQN })
	return result
}

// GetStats 获取引擎统计.
func (m *Manager) GetStats(engineID string) (*OffloadStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	stats, ok := m.stats[engineID]
	if !ok {
		return &OffloadStats{EngineID: engineID}, nil
	}
	return stats, nil
}

// UpdateStats 更新统计.
func (m *Manager) UpdateStats(engineID string, stats *OffloadStats) {
	m.mu.Lock()
	defer m.mu.Unlock()
	stats.EngineID = engineID
	m.stats[engineID] = stats
}

// HealthCheck 健康检查.
func (m *Manager) HealthCheck() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]string)
	for id, e := range m.engines {
		switch {
		case e.Status == OffloadStatusFailed:
			result[id] = "failed"
		case e.Temperature > 80:
			result[id] = "overheating"
		case e.ActiveSessions >= e.MaxSessions:
			result[id] = "max_sessions"
		default:
			result[id] = string(e.Status)
		}
	}
	return result
}

// RecommendEngine 推荐引擎.
func (m *Manager) RecommendEngine(requiredMBps int) *OffloadEngine {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, e := range m.engines {
		if e.Status == OffloadStatusEnabled && e.MaxLunMBps >= requiredMBps {
			if e.ActiveTargets < e.MaxTargets {
				return e
			}
		}
	}
	return nil
}