// Package sharedlabels 提供业务逻辑服务层
package sharedlabels

import (
	"context"
	"fmt"
	"time"
)

// ServiceOps 服务操作扩展（辅助方法）
// 提供标签生命周期管理和查询辅助方法

// DeleteLabelByID 按ID删除标签（带存在性检查）.
func (s *Service) DeleteLabelByID(ctx context.Context, labelID string) error {
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

	delete(s.labelFiles, labelID)
	delete(s.labels, labelID)
	return nil
}

// UpdateLabel 更新标签.
func (s *Service) UpdateLabel(ctx context.Context, labelID string, name, description string, color LabelColor) (*Label, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	label, ok := s.labels[labelID]
	if !ok {
		return nil, fmt.Errorf("标签不存在: %s", labelID)
	}

	if name != "" {
		label.Name = name
	}
	if description != "" {
		label.Description = description
	}
	if color != "" {
		label.Color = color
	}
	label.UpdatedAt = time.Now()

	return label, nil
}

// GetLabelFiles 获取标签下的所有文件.
func (s *Service) GetLabelFiles(ctx context.Context, labelID string) ([]*FileLabel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.labelFiles[labelID], nil
}
