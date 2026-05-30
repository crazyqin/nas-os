// Package sharedlabels 实现共享标签功能
// 对标群晖 DSM 7.3 Shared Labels 特性
// 支持团队协作文件组织和共享
package sharedlabels

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// LabelType 标签类型
type LabelType string

const (
	LabelTypeSystem LabelType = "system" // 系统标签
	LabelTypeUser   LabelType = "user"   // 用户标签
	LabelTypeTeam   LabelType = "team"   // 团队标签
)

// LabelColor 标签颜色
type LabelColor string

const (
	ColorRed    LabelColor = "red"
	ColorOrange LabelColor = "orange"
	ColorYellow LabelColor = "yellow"
	ColorGreen  LabelColor = "green"
	ColorBlue   LabelColor = "blue"
	ColorPurple LabelColor = "purple"
	ColorPink   LabelColor = "pink"
	ColorGray   LabelColor = "gray"
)

// Label 共享标签定义
type Label struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Type        LabelType  `json:"type"`
	Color       LabelColor `json:"color"`
	CreatedBy   string     `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	FileCount   int        `json:"file_count"` // 使用此标签的文件数
	IsPublic    bool       `json:"is_public"`  // 是否公开可见
	TenantID    string     `json:"tenant_id"`  // 租户ID（多租户支持）
}

// FileLabel 文件标签关联
type FileLabel struct {
	FileID    string    `json:"file_id"`
	FilePath  string    `json:"file_path"`
	LabelID   string    `json:"label_id"`
	LabelName string    `json:"label_name"`
	AppliedBy string    `json:"applied_by"`
	AppliedAt time.Time `json:"applied_at"`
}

// LabelStats 标签统计
type LabelStats struct {
	TotalLabels  int            `json:"total_labels"`
	TotalFiles   int            `json:"total_files"`
	LabelsByType map[string]int `json:"labels_by_type"`
	TopLabels    []LabelRank    `json:"top_labels"`
}

// LabelRank 标签排名
type LabelRank struct {
	LabelID   string `json:"label_id"`
	LabelName string `json:"label_name"`
	FileCount int    `json:"file_count"`
}

// Manager 共享标签管理器
type Manager struct {
	mu          sync.RWMutex
	labels      map[string]*Label
	fileLabels  map[string][]*FileLabel // fileID -> labels
	storagePath string
}

// NewManager 创建共享标签管理器
func NewManager(storagePath string) *Manager {
	return &Manager{
		labels:      make(map[string]*Label),
		fileLabels:  make(map[string][]*FileLabel),
		storagePath: storagePath,
	}
}

// CreateLabel 创建标签
func (m *Manager) CreateLabel(ctx context.Context, label Label) (*Label, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if label.ID == "" {
		label.ID = fmt.Sprintf("label_%d", time.Now().UnixNano())
	}
	label.CreatedAt = time.Now()
	label.UpdatedAt = time.Now()

	m.labels[label.ID] = &label
	return &label, nil
}

// GetLabel 获取标签
func (m *Manager) GetLabel(ctx context.Context, labelID string) (*Label, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	label, exists := m.labels[labelID]
	if !exists {
		return nil, fmt.Errorf("标签不存在: %s", labelID)
	}
	return label, nil
}

// ListLabels 列出标签
func (m *Manager) ListLabels(ctx context.Context, labelType LabelType, tenantID string) ([]*Label, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Label
	for _, label := range m.labels {
		if labelType != "" && label.Type != labelType {
			continue
		}
		if tenantID != "" && label.TenantID != tenantID {
			continue
		}
		result = append(result, label)
	}
	return result, nil
}

// ApplyLabel 应用标签到文件
func (m *Manager) ApplyLabel(ctx context.Context, fileID, filePath, labelID, appliedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	label, exists := m.labels[labelID]
	if !exists {
		return fmt.Errorf("标签不存在: %s", labelID)
	}

	fl := &FileLabel{
		FileID:    fileID,
		FilePath:  filePath,
		LabelID:   labelID,
		LabelName: label.Name,
		AppliedBy: appliedBy,
		AppliedAt: time.Now(),
	}

	m.fileLabels[fileID] = append(m.fileLabels[fileID], fl)
	label.FileCount++
	return nil
}

// RemoveLabel 移除文件标签
func (m *Manager) RemoveLabel(ctx context.Context, fileID, labelID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	labels := m.fileLabels[fileID]
	for i, fl := range labels {
		if fl.LabelID == labelID {
			m.fileLabels[fileID] = append(labels[:i], labels[i+1:]...)
			if label, exists := m.labels[labelID]; exists {
				label.FileCount--
			}
			return nil
		}
	}
	return fmt.Errorf("文件 %s 未应用标签 %s", fileID, labelID)
}

// GetFileLabels 获取文件的所有标签
func (m *Manager) GetFileLabels(ctx context.Context, fileID string) ([]*FileLabel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.fileLabels[fileID], nil
}

// GetLabelFiles 获取标签下的所有文件
func (m *Manager) GetLabelFiles(ctx context.Context, labelID string) ([]*FileLabel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*FileLabel
	for _, labels := range m.fileLabels {
		for _, fl := range labels {
			if fl.LabelID == labelID {
				result = append(result, fl)
			}
		}
	}
	return result, nil
}

// GetStats 获取标签统计
func (m *Manager) GetStats(ctx context.Context, tenantID string) (*LabelStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &LabelStats{
		LabelsByType: make(map[string]int),
	}

	for _, label := range m.labels {
		if tenantID != "" && label.TenantID != tenantID {
			continue
		}
		stats.TotalLabels++
		stats.LabelsByType[string(label.Type)]++
		stats.TotalFiles += label.FileCount
		stats.TopLabels = append(stats.TopLabels, LabelRank{
			LabelID:   label.ID,
			LabelName: label.Name,
			FileCount: label.FileCount,
		})
	}

	return stats, nil
}

// DeleteLabel 删除标签
func (m *Manager) DeleteLabel(ctx context.Context, labelID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.labels[labelID]; !exists {
		return fmt.Errorf("标签不存在: %s", labelID)
	}

	// 移除所有文件关联
	for fileID, labels := range m.fileLabels {
		var newLabels []*FileLabel
		for _, fl := range labels {
			if fl.LabelID != labelID {
				newLabels = append(newLabels, fl)
			}
		}
		m.fileLabels[fileID] = newLabels
	}

	delete(m.labels, labelID)
	return nil
}
