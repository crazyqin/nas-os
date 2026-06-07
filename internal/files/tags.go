// 文件共享标签系统
//对标群晖DSM 7.3共享标签功能
// 兵部 Round 128

package files

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// SharedTag 共享标签
type SharedTag struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Color       string    `json:"color"` // 标签颜色 #RRGGBB
	Description string    `json:"description"`
	CreatedBy   string    `json:"created_by"` // 创建用户
	CreatedAt   time.Time `json:"created_at"`
	IsPublic    bool      `json:"is_public"`  // 是否公开标签
	FileCount   int       `json:"file_count"` // 关联文件数
}

// FileTagAssociation 文件-标签关联
type FileTagAssociation struct {
	FileID    string    `json:"file_id"`
	TagID     string    `json:"tag_id"`
	UserID    string    `json:"user_id"` // 标记用户
	CreatedAt time.Time `json:"created_at"`
	Note      string    `json:"note,omitempty"` // 备注
}

// SharedTagService 共享标签服务
type SharedTagService struct {
	tags         map[string]*SharedTag            // tagID -> tag
	associations map[string][]*FileTagAssociation // fileID -> associations
	userTags     map[string][]string              // userID -> tagIDs (私有标签)
	mu           sync.RWMutex
}

// NewSharedTagService 创建服务
func NewSharedTagService() *SharedTagService {
	return &SharedTagService{
		tags:         make(map[string]*SharedTag),
		associations: make(map[string][]*FileTagAssociation),
		userTags:     make(map[string][]string),
	}
}

// CreateTag 创建标签
func (s *SharedTagService) CreateTag(ctx context.Context, req *CreateTagRequest) (*SharedTag, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tagID := generateTagID()
	tag := &SharedTag{
		ID:          tagID,
		Name:        req.Name,
		Color:       req.Color,
		Description: req.Description,
		CreatedBy:   req.UserID,
		CreatedAt:   time.Now(),
		IsPublic:    req.IsPublic,
		FileCount:   0,
	}

	s.tags[tagID] = tag

	if !req.IsPublic {
		// 私有标签绑定用户
		s.userTags[req.UserID] = append(s.userTags[req.UserID], tagID)
	}

	return tag, nil
}

// GetTag 获取标签
func (s *SharedTagService) GetTag(tagID string) (*SharedTag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tag, exists := s.tags[tagID]
	if !exists {
		return nil, fmt.Errorf("tag not found: %s", tagID)
	}
	return tag, nil
}

// ListTags 列出标签
func (s *SharedTagService) ListTags(ctx context.Context, userID string, includePublic bool) ([]*SharedTag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := []*SharedTag{}

	// 用户私有标签
	if userTagIDs, exists := s.userTags[userID]; exists {
		for _, tagID := range userTagIDs {
			if tag, ok := s.tags[tagID]; ok {
				result = append(result, tag)
			}
		}
	}

	// 公开标签
	if includePublic {
		for _, tag := range s.tags {
			if tag.IsPublic {
				result = append(result, tag)
			}
		}
	}

	return result, nil
}

// TagFile 标记文件
func (s *SharedTagService) TagFile(ctx context.Context, req *TagFileRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 验证标签存在
	tag, exists := s.tags[req.TagID]
	if !exists {
		return fmt.Errorf("tag not found: %s", req.TagID)
	}

	// 验证权限
	if !tag.IsPublic && tag.CreatedBy != req.UserID {
		return fmt.Errorf("no permission to use private tag")
	}

	assoc := &FileTagAssociation{
		FileID:    req.FileID,
		TagID:     req.TagID,
		UserID:    req.UserID,
		CreatedAt: time.Now(),
		Note:      req.Note,
	}

	s.associations[req.FileID] = append(s.associations[req.FileID], assoc)
	tag.FileCount++

	return nil
}

// UntagFile 移除文件标签
func (s *SharedTagService) UntagFile(ctx context.Context, fileID, tagID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	assocs := s.associations[fileID]
	for i, assoc := range assocs {
		if assoc.TagID == tagID {
			// 移除关联
			s.associations[fileID] = append(assocs[:i], assocs[i+1:]...)

			// 更新计数
			if tag, ok := s.tags[tagID]; ok {
				tag.FileCount--
			}
			return nil
		}
	}

	return fmt.Errorf("tag association not found")
}

// GetFileTags 获取文件的所有标签
func (s *SharedTagService) GetFileTags(fileID string) ([]*SharedTag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	assocs := s.associations[fileID]
	tags := []*SharedTag{}

	for _, assoc := range assocs {
		if tag, ok := s.tags[assoc.TagID]; ok {
			tags = append(tags, tag)
		}
	}

	return tags, nil
}

// SearchByTag 按标签搜索文件
func (s *SharedTagService) SearchByTag(ctx context.Context, tagIDs []string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fileIDs := []string{}
	fileSet := make(map[string]bool)

	for _, tagID := range tagIDs {
		for fileID, assocs := range s.associations {
			for _, assoc := range assocs {
				if assoc.TagID == tagID {
					fileSet[fileID] = true
				}
			}
		}
	}

	for fileID := range fileSet {
		fileIDs = append(fileIDs, fileID)
	}

	return fileIDs, nil
}

// DeleteTag 删除标签
func (s *SharedTagService) DeleteTag(ctx context.Context, tagID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tag, exists := s.tags[tagID]
	if !exists {
		return fmt.Errorf("tag not found: %s", tagID)
	}

	// 权限检查:只有创建者可删除
	if tag.CreatedBy != userID {
		return fmt.Errorf("no permission to delete tag")
	}

	// 移除所有关联
	for fileID, assocs := range s.associations {
		newAssocs := []*FileTagAssociation{}
		for _, assoc := range assocs {
			if assoc.TagID != tagID {
				newAssocs = append(newAssocs, assoc)
			}
		}
		s.associations[fileID] = newAssocs
	}

	// 从用户列表移除
	if userTagIDs, exists := s.userTags[userID]; exists {
		newUserTags := []string{}
		for _, tid := range userTagIDs {
			if tid != tagID {
				newUserTags = append(newUserTags, tid)
			}
		}
		s.userTags[userID] = newUserTags
	}

	delete(s.tags, tagID)
	return nil
}

// ExportTags 导出标签数据
func (s *SharedTagService) ExportTags(ctx context.Context, userID string) ([]byte, error) {
	tags, err := s.ListTags(ctx, userID, true)
	if err != nil {
		return nil, err
	}

	return json.MarshalIndent(tags, "", "  ")
}

// 请求类型
type CreateTagRequest struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
	UserID      string `json:"user_id"`
	IsPublic    bool   `json:"is_public"`
}

type TagFileRequest struct {
	FileID string `json:"file_id"`
	TagID  string `json:"tag_id"`
	UserID string `json:"user_id"`
	Note   string `json:"note,omitempty"`
}

func generateTagID() string {
	return fmt.Sprintf("tag_%d", time.Now().UnixNano())
}
