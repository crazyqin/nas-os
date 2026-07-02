// Package sharedlabels 实现共享文件标签功能
// 对标群晖 DSM 7.3 Shared Labels 特性
// 支持为文件/文件夹打标签、按标签搜索、团队共享标签集管理
package sharedlabels

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 标签类型定义 ==========

// LabelType 标签类型.
type LabelType string

const (
	LabelTypeSystem LabelType = "system" // 系统标签
	LabelTypeUser   LabelType = "user"   // 用户标签
	LabelTypeTeam   LabelType = "team"   // 团队标签
)

// LabelColor 标签颜色.
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

// Label 共享标签定义.
type Label struct {
	ID          string     `json:"id"`          // 唯一标识
	Name        string     `json:"name"`        // 标签名称
	Description string     `json:"description"` // 标签描述
	Type        LabelType  `json:"type"`        // 标签类型
	Color       LabelColor `json:"color"`       // 标签颜色
	CreatedBy   string     `json:"created_by"`  // 创建者
	CreatedAt   time.Time  `json:"created_at"`  // 创建时间
	UpdatedAt   time.Time  `json:"updated_at"`  // 更新时间
	FileCount   int        `json:"file_count"`  // 使用此标签的文件数
	IsPublic    bool       `json:"is_public"`   // 是否公开可见
	TenantID    string     `json:"tenant_id"`   // 租户ID（多租户支持）
}

// FileLabel 文件标签关联.
type FileLabel struct {
	FileID    string    `json:"file_id"`    // 文件ID
	FilePath  string    `json:"file_path"`  // 文件路径
	LabelID   string    `json:"label_id"`   // 标签ID
	LabelName string    `json:"label_name"` // 标签名称
	AppliedBy string    `json:"applied_by"` // 操作人
	AppliedAt time.Time `json:"applied_at"` // 应用时间
}

// LabelStats 标签统计.
type LabelStats struct {
	TotalLabels  int            `json:"total_labels"`   // 标签总数
	TotalFiles   int            `json:"total_files"`    // 文件总数
	LabelsByType map[string]int `json:"labels_by_type"` // 按类型统计
	TopLabels    []LabelRank    `json:"top_labels"`     // 热门标签
}

// LabelRank 标签排名.
type LabelRank struct {
	LabelID   string `json:"label_id"`   // 标签ID
	LabelName string `json:"label_name"` // 标签名称
	FileCount int    `json:"file_count"` // 文件数
}

// ========== 请求/响应结构 ==========

// AssignLabelRequest 为文件分配标签请求.
type AssignLabelRequest struct {
	FileID    string   `json:"file_id" binding:"required"`   // 文件ID
	FilePath  string   `json:"file_path" binding:"required"` // 文件路径
	LabelIDs  []string `json:"label_ids" binding:"required"` // 标签ID列表
	AppliedBy string   `json:"applied_by"`                   // 操作人
}

// SearchByLabelRequest 按标签搜索文件请求.
type SearchByLabelRequest struct {
	LabelIDs []string `form:"label_ids" json:"label_ids"` // 标签ID列表
	TenantID string   `form:"tenant_id" json:"tenant_id"` // 租户ID
}

// CreateLabelRequest 创建标签请求.
type CreateLabelRequest struct {
	Name        string     `json:"name" binding:"required"`       // 标签名称
	Description string     `json:"description"`                   // 标签描述
	Type        LabelType  `json:"type" binding:"required"`       // 标签类型
	Color       LabelColor `json:"color"`                         // 标签颜色
	CreatedBy   string     `json:"created_by" binding:"required"` // 创建者
	IsPublic    bool       `json:"is_public"`                     // 是否公开
	TenantID    string     `json:"tenant_id"`                     // 租户ID
}

// RemoveLabelRequest 移除文件标签请求.
type RemoveLabelRequest struct {
	FileID   string   `json:"file_id" binding:"required"`   // 文件ID
	LabelIDs []string `json:"label_ids" binding:"required"` // 标签ID列表
}

// ========== 服务层 ==========

// Service 共享标签服务.
type Service struct {
	mu         sync.RWMutex
	labels     map[string]*Label       // 标签集合
	fileLabels map[string][]*FileLabel // fileID -> 标签列表
	labelFiles map[string][]*FileLabel // labelID -> 文件列表（索引）
}

// NewService 创建共享标签服务.
func NewService() *Service {
	return &Service{
		labels:     make(map[string]*Label),
		fileLabels: make(map[string][]*FileLabel),
		labelFiles: make(map[string][]*FileLabel),
	}
}

// CreateLabel 创建标签.
func (s *Service) CreateLabel(ctx context.Context, req CreateLabelRequest) (*Label, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查同名标签是否已存在
	for _, l := range s.labels {
		if l.Name == req.Name && l.TenantID == req.TenantID {
			return nil, fmt.Errorf("标签名称 %q 已存在", req.Name)
		}
	}

	now := time.Now()
	label := &Label{
		ID:          "label_" + uuid.New().String()[:8],
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		Color:       req.Color,
		CreatedBy:   req.CreatedBy,
		CreatedAt:   now,
		UpdatedAt:   now,
		IsPublic:    req.IsPublic,
		TenantID:    req.TenantID,
	}

	s.labels[label.ID] = label
	return label, nil
}

// ListLabels 列出标签.
func (s *Service) ListLabels(ctx context.Context, labelType LabelType, tenantID string) ([]*Label, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Label
	for _, label := range s.labels {
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

// GetLabel 获取标签详情.
func (s *Service) GetLabel(ctx context.Context, labelID string) (*Label, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	label, ok := s.labels[labelID]
	if !ok {
		return nil, fmt.Errorf("标签不存在: %s", labelID)
	}
	return label, nil
}

// AssignLabels 为文件分配标签（支持多标签）.
func (s *Service) AssignLabels(ctx context.Context, req AssignLabelRequest) ([]*FileLabel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	var result []*FileLabel

	for _, labelID := range req.LabelIDs {
		label, ok := s.labels[labelID]
		if !ok {
			return nil, fmt.Errorf("标签不存在: %s", labelID)
		}

		// 检查是否已分配
		alreadyAssigned := false
		for _, fl := range s.fileLabels[req.FileID] {
			if fl.LabelID == labelID {
				alreadyAssigned = true
				break
			}
		}
		if alreadyAssigned {
			continue // 已存在则跳过
		}

		fl := &FileLabel{
			FileID:    req.FileID,
			FilePath:  req.FilePath,
			LabelID:   labelID,
			LabelName: label.Name,
			AppliedBy: req.AppliedBy,
			AppliedAt: now,
		}

		s.fileLabels[req.FileID] = append(s.fileLabels[req.FileID], fl)
		s.labelFiles[labelID] = append(s.labelFiles[labelID], fl)
		label.FileCount++
		result = append(result, fl)
	}

	return result, nil
}

// RemoveLabels 移除文件标签.
func (s *Service) RemoveLabels(ctx context.Context, req RemoveLabelRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, labelID := range req.LabelIDs {
		// 从文件标签列表中移除
		fileLabels := s.fileLabels[req.FileID]
		found := false
		for i, fl := range fileLabels {
			if fl.LabelID == labelID {
				s.fileLabels[req.FileID] = append(fileLabels[:i], fileLabels[i+1:]...)
				found = true
				break
			}
		}

		// 从标签文件索引中移除
		labelFiles := s.labelFiles[labelID]
		for i, fl := range labelFiles {
			if fl.FileID == req.FileID {
				s.labelFiles[labelID] = append(labelFiles[:i], labelFiles[i+1:]...)
				break
			}
		}

		if found {
			if label, ok := s.labels[labelID]; ok {
				label.FileCount--
			}
		}
	}

	return nil
}

// SearchByLabels 按标签搜索文件.
func (s *Service) SearchByLabels(ctx context.Context, labelIDs []string) ([]*FileLabel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(labelIDs) == 0 {
		// 无标签过滤，返回所有文件（去重）
		seen := make(map[string]bool)
		var result []*FileLabel
		for _, fls := range s.fileLabels {
			for _, fl := range fls {
				if !seen[fl.FileID] {
					seen[fl.FileID] = true
					result = append(result, fl)
				}
			}
		}
		return result, nil
	}

	// 返回包含任意一个标签的文件（OR 语义）
	seen := make(map[string]bool)
	var result []*FileLabel
	for _, labelID := range labelIDs {
		for _, fl := range s.labelFiles[labelID] {
			if !seen[fl.FileID] {
				seen[fl.FileID] = true
				result = append(result, fl)
			}
		}
	}
	return result, nil
}

// GetFileLabels 获取文件的所有标签.
func (s *Service) GetFileLabels(ctx context.Context, fileID string) ([]*FileLabel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fileLabels[fileID], nil
}

// GetStats 获取标签统计.
func (s *Service) GetStats(ctx context.Context, tenantID string) (*LabelStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &LabelStats{
		LabelsByType: make(map[string]int),
	}

	uniqueFiles := make(map[string]bool)
	for _, label := range s.labels {
		if tenantID != "" && label.TenantID != tenantID {
			continue
		}
		stats.TotalLabels++
		stats.LabelsByType[string(label.Type)]++
		stats.TopLabels = append(stats.TopLabels, LabelRank{
			LabelID:   label.ID,
			LabelName: label.Name,
			FileCount: label.FileCount,
		})
	}

	for fileID := range s.fileLabels {
		uniqueFiles[fileID] = true
	}
	stats.TotalFiles = len(uniqueFiles)

	return stats, nil
}

// DeleteLabel 删除标签.
func (s *Service) DeleteLabel(ctx context.Context, labelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.labels[labelID]; !ok {
		return fmt.Errorf("标签不存在: %s", labelID)
	}

	// 清理文件关联
	for fileID, fls := range s.fileLabels {
		var newLabels []*FileLabel
		for _, fl := range fls {
			if fl.LabelID != labelID {
				newLabels = append(newLabels, fl)
			}
		}
		s.fileLabels[fileID] = newLabels
	}

	// 清理标签文件索引
	delete(s.labelFiles, labelID)
	delete(s.labels, labelID)
	return nil
}
