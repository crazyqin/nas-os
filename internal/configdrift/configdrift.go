// Package configdrift 提供配置漂移检测与管理功能
package configdrift

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
)

// ChangeType 变更类型
type ChangeType string

const (
	ChangeAdd    ChangeType = "Add"
	ChangeModify ChangeType = "Modify"
	ChangeDelete ChangeType = "Delete"
)

// DriftSeverity 漂移严重程度
type DriftSeverity int

const (
	SeverityLow      DriftSeverity = iota // 低
	SeverityMedium                        // 中
	SeverityHigh                          // 高
	SeverityCritical                      // 严重
)

// String 返回严重程度的字符串表示
func (s DriftSeverity) String() string {
	switch s {
	case SeverityLow:
		return "Low"
	case SeverityMedium:
		return "Medium"
	case SeverityHigh:
		return "High"
	case SeverityCritical:
		return "Critical"
	default:
		return "Unknown"
	}
}

// ConfigSnapshot 配置快照
type ConfigSnapshot struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Hash      string                 `json:"hash"`
	Config    map[string]interface{} `json:"config"`
	Label     string                 `json:"label,omitempty"`
}

// ConfigChange 配置变更
type ConfigChange struct {
	Path      string      `json:"path"`
	OldValue  interface{} `json:"old_value,omitempty"`
	NewValue  interface{} `json:"new_value,omitempty"`
	Type      ChangeType  `json:"type"`
}

// DriftReport 漂移报告
type DriftReport struct {
	BaselineID  string         `json:"baseline_id"`
	CurrentID   string         `json:"current_id"`
	Changes     []ConfigChange `json:"changes"`
	DriftScore  float64        `json:"drift_score"`
	Severity    DriftSeverity  `json:"severity"`
	GeneratedAt time.Time      `json:"generated_at"`
}

// Manager 配置漂移管理器
type Manager struct {
	mu          sync.RWMutex
	storageDir  string
	snapshots   map[string]*ConfigSnapshot
	baselineID  string
	driftHistory []DriftReport
}

// NewManager 创建配置漂移管理器
func NewManager(storageDir string) *Manager {
	if storageDir == "" {
		storageDir = "/tmp/configdrift"
	}

	m := &Manager{
		storageDir:   storageDir,
		snapshots:    make(map[string]*ConfigSnapshot),
		driftHistory: make([]DriftReport, 0),
	}

	// 确保存储目录存在
	os.MkdirAll(storageDir, 0755)

	// 加载已有快照
	m.loadSnapshots()

	return m
}

// generateID 生成唯一 ID
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// hashConfig 计算配置内容的哈希值
func hashConfig(config map[string]interface{}) string {
	data, _ := json.Marshal(config)
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash[:8])
}

// TakeSnapshot 拍摄配置快照
func (m *Manager) TakeSnapshot(label string) (*ConfigSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 读取系统配置（这里模拟读取，实际应用中应该读取真实配置）
	config := m.readSystemConfig()

	snapshot := &ConfigSnapshot{
		ID:        generateID(),
		Timestamp: time.Now(),
		Hash:      hashConfig(config),
		Config:    config,
		Label:     label,
	}

	m.snapshots[snapshot.ID] = snapshot

	// 持久化存储
	if err := m.saveSnapshot(snapshot); err != nil {
		return nil, fmt.Errorf("failed to save snapshot: %w", err)
	}

	return snapshot, nil
}

// readSystemConfig 读取系统配置（模拟实现）
func (m *Manager) readSystemConfig() map[string]interface{} {
	// 模拟读取系统配置，实际应用中应该读取真实的配置文件
	return map[string]interface{}{
		"hostname": "nas-server",
		"network": map[string]interface{}{
			"interface": "eth0",
			"ip":        "192.168.1.100",
			"gateway":   "192.168.1.1",
			"dns":       []string{"8.8.8.8", "8.8.4.4"},
		},
		"storage": map[string]interface{}{
			"mount_point": "/data",
			"filesystem":  "ext4",
			"options":     []string{"defaults", "noatime"},
		},
		"services": map[string]interface{}{
			"smb": map[string]interface{}{
				"enabled": true,
				"port":    445,
			},
			"nfs": map[string]interface{}{
				"enabled": false,
				"port":    2049,
			},
		},
	}
}

// GetSnapshot 获取指定快照
func (m *Manager) GetSnapshot(id string) (*ConfigSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot, ok := m.snapshots[id]
	if !ok {
		return nil, fmt.Errorf("snapshot not found: %s", id)
	}

	return snapshot, nil
}

// ListSnapshots 列出所有快照
func (m *Manager) ListSnapshots() ([]ConfigSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshots := make([]ConfigSnapshot, 0, len(m.snapshots))
	for _, s := range m.snapshots {
		snapshots = append(snapshots, *s)
	}

	// 按时间排序
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Timestamp.After(snapshots[j].Timestamp)
	})

	return snapshots, nil
}

// SetBaseline 设置基线快照
func (m *Manager) SetBaseline(snapshotID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.snapshots[snapshotID]; !ok {
		return fmt.Errorf("snapshot not found: %s", snapshotID)
	}

	m.baselineID = snapshotID
	return nil
}

// GetBaseline 获取基线快照
func (m *Manager) GetBaseline() (*ConfigSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.baselineID == "" {
		return nil, fmt.Errorf("no baseline set")
	}

	snapshot, ok := m.snapshots[m.baselineID]
	if !ok {
		return nil, fmt.Errorf("baseline snapshot not found: %s", m.baselineID)
	}

	return snapshot, nil
}

// CompareSnapshots 比较两个快照的差异
func (m *Manager) CompareSnapshots(baselineID, currentID string) (*DriftReport, error) {
	m.mu.RLock()
	baseline, ok := m.snapshots[baselineID]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("baseline snapshot not found: %s", baselineID)
	}

	current, ok := m.snapshots[currentID]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("current snapshot not found: %s", currentID)
	}
	m.mu.RUnlock()

	// 深度比较配置差异
	changes := m.deepCompare(baseline.Config, current.Config, "")

	// 计算漂移评分
	driftScore := m.calculateDriftScore(changes, baseline.Config)

	// 确定严重程度
	severity := m.determineSeverity(driftScore, changes)

	report := DriftReport{
		BaselineID:  baselineID,
		CurrentID:   currentID,
		Changes:     changes,
		DriftScore:  driftScore,
		Severity:    severity,
		GeneratedAt: time.Now(),
	}

	// 保存到历史记录
	m.mu.Lock()
	m.driftHistory = append(m.driftHistory, report)
	m.mu.Unlock()

	return &report, nil
}

// DetectDrift 检测配置漂移（与最新基线对比）
func (m *Manager) DetectDrift() (*DriftReport, error) {
	m.mu.RLock()
	baselineID := m.baselineID
	m.mu.RUnlock()

	if baselineID == "" {
		return nil, fmt.Errorf("no baseline set")
	}

	// 创建当前配置快照
	currentSnapshot, err := m.TakeSnapshot("auto-detect")
	if err != nil {
		return nil, fmt.Errorf("failed to take current snapshot: %w", err)
	}

	return m.CompareSnapshots(baselineID, currentSnapshot.ID)
}

// deepCompare 深度比较两个配置
func (m *Manager) deepCompare(baseline, current interface{}, path string) []ConfigChange {
	var changes []ConfigChange

	// 如果类型不同，直接标记为修改
	if reflect.TypeOf(baseline) != reflect.TypeOf(current) {
		changes = append(changes, ConfigChange{
			Path:     path,
			OldValue: baseline,
			NewValue: current,
			Type:     ChangeModify,
		})
		return changes
	}

	switch baseVal := baseline.(type) {
	case map[string]interface{}:
		currentMap, ok := current.(map[string]interface{})
		if !ok {
			return changes
		}

		// 检查删除的键
		for key := range baseVal {
			currentPath := path
			if currentPath == "" {
				currentPath = key
			} else {
				currentPath = path + "." + key
			}

			if _, exists := currentMap[key]; !exists {
				changes = append(changes, ConfigChange{
					Path:     currentPath,
					OldValue: baseVal[key],
					Type:     ChangeDelete,
				})
			} else {
				// 递归比较
				subChanges := m.deepCompare(baseVal[key], currentMap[key], currentPath)
				changes = append(changes, subChanges...)
			}
		}

		// 检查新增的键
		for key := range currentMap {
			currentPath := path
			if currentPath == "" {
				currentPath = key
			} else {
				currentPath = path + "." + key
			}

			if _, exists := baseVal[key]; !exists {
				changes = append(changes, ConfigChange{
					Path:     currentPath,
					NewValue: currentMap[key],
					Type:     ChangeAdd,
				})
			}
		}

	case []interface{}:
		currentSlice, ok := current.([]interface{})
		if !ok {
			return changes
		}

		// 比较数组
		if !reflect.DeepEqual(baseVal, currentSlice) {
			changes = append(changes, ConfigChange{
				Path:     path,
				OldValue: baseVal,
				NewValue: currentSlice,
				Type:     ChangeModify,
			})
		}

	default:
		// 基础类型直接比较
		if !reflect.DeepEqual(baseline, current) {
			changes = append(changes, ConfigChange{
				Path:     path,
				OldValue: baseline,
				NewValue: current,
				Type:     ChangeModify,
			})
		}
	}

	return changes
}

// calculateDriftScore 计算漂移评分 (0-100)
func (m *Manager) calculateDriftScore(changes []ConfigChange, baseline map[string]interface{}) float64 {
	if len(changes) == 0 {
		return 0
	}

	// 统计配置项总数
	totalKeys := m.countConfigKeys(baseline)
	if totalKeys == 0 {
		return 100
	}

	// 根据变更类型加权
	weightedChanges := 0.0
	for _, change := range changes {
		switch change.Type {
		case ChangeDelete:
			weightedChanges += 1.5 // 删除权重更高
		case ChangeAdd:
			weightedChanges += 1.0
		case ChangeModify:
			weightedChanges += 1.2
		}
	}

	score := (weightedChanges / float64(totalKeys)) * 100
	if score > 100 {
		score = 100
	}

	return score
}

// countConfigKeys 统计配置项总数
func (m *Manager) countConfigKeys(config map[string]interface{}) int {
	count := 0
	for _, v := range config {
		count++
		if subMap, ok := v.(map[string]interface{}); ok {
			count += m.countConfigKeys(subMap)
		}
	}
	return count
}

// determineSeverity 确定漂移严重程度
func (m *Manager) determineSeverity(score float64, changes []ConfigChange) DriftSeverity {
	// 检查是否有关键配置变更
	hasCriticalChange := false
	criticalPaths := []string{"hostname", "network.ip", "network.gateway", "storage.mount_point"}

	for _, change := range changes {
		for _, critical := range criticalPaths {
			if strings.HasPrefix(change.Path, critical) {
				hasCriticalChange = true
				break
			}
		}
		if hasCriticalChange {
			break
		}
	}

	if hasCriticalChange || score >= 50 {
		return SeverityCritical
	} else if score >= 30 {
		return SeverityHigh
	} else if score >= 10 {
		return SeverityMedium
	}
	return SeverityLow
}

// AutoRollback 自动回滚到指定快照
func (m *Manager) AutoRollback(snapshotID string) error {
	m.mu.RLock()
	snapshot, ok := m.snapshots[snapshotID]
	if !ok {
		m.mu.RUnlock()
		return fmt.Errorf("snapshot not found: %s", snapshotID)
	}
	m.mu.RUnlock()

	// 在实际应用中，这里应该将配置写回系统
	// 这里模拟回滚操作
	_ = snapshot

	return nil
}

// GetDriftHistory 获取漂移历史
func (m *Manager) GetDriftHistory() ([]DriftReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history := make([]DriftReport, len(m.driftHistory))
	copy(history, m.driftHistory)

	return history, nil
}

// ExportReport 导出漂移报告为 JSON
func (m *Manager) ExportReport(report DriftReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

// saveSnapshot 保存快照到文件
func (m *Manager) saveSnapshot(snapshot *ConfigSnapshot) error {
	filename := filepath.Join(m.storageDir, snapshot.ID+".json")
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

// loadSnapshots 从文件加载快照
func (m *Manager) loadSnapshots() {
	files, err := os.ReadDir(m.storageDir)
	if err != nil {
		return
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) != ".json" {
			continue
		}

		filename := filepath.Join(m.storageDir, file.Name())
		data, err := os.ReadFile(filename)
		if err != nil {
			continue
		}

		var snapshot ConfigSnapshot
		if err := json.Unmarshal(data, &snapshot); err != nil {
			continue
		}

		m.snapshots[snapshot.ID] = &snapshot
	}
}
