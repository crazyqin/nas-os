// Package team 评论与@提及功能
package team

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// CommentManager 评论管理器.
type CommentManager struct {
	mu            sync.RWMutex
	comments      map[string]*Comment        // commentID -> Comment
	resourceIndex map[string]map[string]bool // resourceID -> commentID set
	userComments  map[string]map[string]bool // userID -> commentID set
	configPath    string
	manager       *Manager
	notifier      *Notifier
}

// NewCommentManager 创建评论管理器.
func NewCommentManager(configPath string, manager *Manager) *CommentManager {
	cm := &CommentManager{
		comments:      make(map[string]*Comment),
		resourceIndex: make(map[string]map[string]bool),
		userComments:  make(map[string]map[string]bool),
		configPath:    configPath,
		manager:       manager,
		notifier:      NewNotifier(),
	}

	// 加载配置
	if configPath != "" {
		cm.loadConfig()
	}

	return cm
}

// loadConfig 加载配置.
func (cm *CommentManager) loadConfig() error {
	if _, err := os.Stat(cm.configPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return err
	}

	var config struct {
		Comments map[string]*Comment `json:"comments"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	cm.comments = config.Comments

	// 重建索引
	for id, comment := range cm.comments {
		if cm.resourceIndex[comment.ResourceID] == nil {
			cm.resourceIndex[comment.ResourceID] = make(map[string]bool)
		}
		cm.resourceIndex[comment.ResourceID][id] = true

		if cm.userComments[comment.UserID] == nil {
			cm.userComments[comment.UserID] = make(map[string]bool)
		}
		cm.userComments[comment.UserID][id] = true
	}

	return nil
}

// saveConfig 保存配置.
func (cm *CommentManager) saveConfig() error {
	if cm.configPath == "" {
		return nil
	}

	cm.mu.RLock()
	config := struct {
		Comments map[string]*Comment `json:"comments"`
	}{
		Comments: cm.comments,
	}
	cm.mu.RUnlock()

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(cm.configPath), 0750); err != nil {
		return err
	}

	return os.WriteFile(cm.configPath, data, 0600)
}

// mentionRegex @提及正则表达式.
var mentionRegex = regexp.MustCompile(`@(\w+)`)

// parseMentions 解析评论中的@提及.
func parseMentions(content string) []Mention {
	mentions := make([]Mention, 0)
	matches := mentionRegex.FindAllStringSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) >= 4 {
			username := content[match[2]:match[3]]
			mentions = append(mentions, Mention{
				Username: username,
				Position: match[0],
			})
		}
	}

	return mentions
}

// CreateComment 创建评论.
func (cm *CommentManager) CreateComment(input CommentInput, userID, username string) (*Comment, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 解析@提及
	mentions := parseMentions(input.Content)

	// 解析提及的用户ID
	// 实际使用时需要从用户系统查询
	for i := range mentions {
		mentions[i].UserID = "" // 需要根据username查询
	}

	comment := &Comment{
		ID:           generateID(),
		ResourceType: input.ResourceType,
		ResourceID:   input.ResourceID,
		ResourcePath: input.ResourcePath,
		ParentID:     input.ParentID,
		UserID:       userID,
		Username:     username,
		Content:      input.Content,
		Mentions:     mentions,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Reactions:    make(map[string]int),
	}

	cm.comments[comment.ID] = comment

	// 建立资源索引
	if cm.resourceIndex[input.ResourceID] == nil {
		cm.resourceIndex[input.ResourceID] = make(map[string]bool)
	}
	cm.resourceIndex[input.ResourceID][comment.ID] = true

	// 建立用户索引
	if cm.userComments[userID] == nil {
		cm.userComments[userID] = make(map[string]bool)
	}
	cm.userComments[userID][comment.ID] = true

	// 发送通知给被@的用户
	for _, mention := range mentions {
		if mention.UserID != "" && mention.UserID != userID {
			cm.notifier.Notify(&Notification{
				Type:     NotifyMention,
				UserID:   mention.UserID,
				FromUser: username,
				Title:    "您被@提及",
				Content:  fmt.Sprintf("%s 在评论中提到了您: %s", username, truncateContent(input.Content, 50)),
				Data: map[string]interface{}{
					"comment_id":    comment.ID,
					"resource_id":   input.ResourceID,
					"resource_type": input.ResourceType,
				},
			})
		}
	}

	// 如果是回复，通知被回复者
	if input.ParentID != "" {
		if parent, ok := cm.comments[input.ParentID]; ok && parent.UserID != userID {
			cm.notifier.Notify(&Notification{
				Type:     NotifyCommentAdded,
				UserID:   parent.UserID,
				FromUser: username,
				Title:    "评论回复",
				Content:  fmt.Sprintf("%s 回复了您的评论: %s", username, truncateContent(input.Content, 50)),
				Data: map[string]interface{}{
					"comment_id":    comment.ID,
					"parent_id":     input.ParentID,
					"resource_id":   input.ResourceID,
					"resource_type": input.ResourceType,
				},
			})
		}
	}

	// 记录审计日志
	if cm.manager != nil && cm.manager.audit != nil {
		cm.manager.audit.Log(&TeamAuditLog{
			UserID:       userID,
			Username:     username,
			Action:       AuditCommentCreate,
			ResourceType: string(input.ResourceType),
			ResourceID:   input.ResourceID,
			ResourcePath: input.ResourcePath,
			Details: map[string]interface{}{
				"comment_id":   comment.ID,
				"has_mentions": len(mentions) > 0,
				"is_reply":     input.ParentID != "",
			},
		})
	}

	cm.saveConfig()
	return comment, nil
}

// GetComment 获取评论.
func (cm *CommentManager) GetComment(commentID string) (*Comment, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	comment, ok := cm.comments[commentID]
	if !ok {
		return nil, ErrCommentNotFound
	}
	return comment, nil
}

// UpdateComment 更新评论.
func (cm *CommentManager) UpdateComment(commentID, content, userID, username string) (*Comment, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	comment, ok := cm.comments[commentID]
	if !ok {
		return nil, ErrCommentNotFound
	}

	// 只有评论者可以编辑
	if comment.UserID != userID {
		return nil, ErrNoPermission
	}

	// 解析新的@提及
	mentions := parseMentions(content)

	comment.Content = content
	comment.Mentions = mentions
	comment.UpdatedAt = time.Now()
	comment.IsEdited = true

	// 发送通知给新提及的用户
	for _, mention := range mentions {
		if mention.UserID != "" && mention.UserID != userID {
			// 检查是否是新增的提及
			cm.notifier.Notify(&Notification{
				Type:     NotifyMention,
				UserID:   mention.UserID,
				FromUser: username,
				Title:    "您被@提及",
				Content:  fmt.Sprintf("%s 在评论中提到了您", username),
				Data: map[string]interface{}{
					"comment_id":    comment.ID,
					"resource_id":   comment.ResourceID,
					"resource_type": comment.ResourceType,
				},
			})
		}
	}

	// 记录审计日志
	if cm.manager != nil && cm.manager.audit != nil {
		cm.manager.audit.Log(&TeamAuditLog{
			UserID:       userID,
			Username:     username,
			Action:       AuditCommentUpdate,
			ResourceType: string(comment.ResourceType),
			ResourceID:   comment.ResourceID,
			Details: map[string]interface{}{
				"comment_id": commentID,
			},
		})
	}

	cm.saveConfig()
	return comment, nil
}

// DeleteComment 删除评论.
func (cm *CommentManager) DeleteComment(commentID, userID, username string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	comment, ok := cm.comments[commentID]
	if !ok {
		return ErrCommentNotFound
	}

	// 只有评论者或管理员可以删除
	if comment.UserID != userID {
		if cm.manager == nil || !cm.manager.hasPermissionForUser(userID, RoleAdmin) {
			return ErrNoPermission
		}
	}

	// 软删除
	comment.IsDeleted = true
	comment.UpdatedAt = time.Now()

	// 记录审计日志
	if cm.manager != nil && cm.manager.audit != nil {
		cm.manager.audit.Log(&TeamAuditLog{
			UserID:       userID,
			Username:     username,
			Action:       AuditCommentDelete,
			ResourceType: string(comment.ResourceType),
			ResourceID:   comment.ResourceID,
			Details: map[string]interface{}{
				"comment_id": commentID,
			},
		})
	}

	cm.saveConfig()
	return nil
}

// ListResourceComments 列出资源的评论.
func (cm *CommentManager) ListResourceComments(resourceID string, includeDeleted bool) []*Comment {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	commentIDs := cm.resourceIndex[resourceID]
	if commentIDs == nil {
		return []*Comment{}
	}

	comments := make([]*Comment, 0)
	for id := range commentIDs {
		if comment, ok := cm.comments[id]; ok {
			if includeDeleted || !comment.IsDeleted {
				comments = append(comments, comment)
			}
		}
	}

	// 按时间排序
	sortCommentsByTime(comments)
	return comments
}

// ListUserComments 列出用户的评论.
func (cm *CommentManager) ListUserComments(userID string, limit int) []*Comment {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	commentIDs := cm.userComments[userID]
	if commentIDs == nil {
		return []*Comment{}
	}

	comments := make([]*Comment, 0)
	for id := range commentIDs {
		if comment, ok := cm.comments[id]; ok && !comment.IsDeleted {
			comments = append(comments, comment)
			if limit > 0 && len(comments) >= limit {
				break
			}
		}
	}

	sortCommentsByTime(comments)
	return comments
}

// GetCommentReplies 获取评论的回复.
func (cm *CommentManager) GetCommentReplies(parentID string) []*Comment {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	replies := make([]*Comment, 0)
	for _, comment := range cm.comments {
		if comment.ParentID == parentID && !comment.IsDeleted {
			replies = append(replies, comment)
		}
	}

	sortCommentsByTime(replies)
	return replies
}

// AddReaction 添加表情反应.
func (cm *CommentManager) AddReaction(commentID, emoji, userID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	comment, ok := cm.comments[commentID]
	if !ok {
		return ErrCommentNotFound
	}

	if comment.Reactions == nil {
		comment.Reactions = make(map[string]int)
	}

	comment.Reactions[emoji]++
	cm.saveConfig()
	return nil
}

// RemoveReaction 移除表情反应.
func (cm *CommentManager) RemoveReaction(commentID, emoji, userID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	comment, ok := cm.comments[commentID]
	if !ok {
		return ErrCommentNotFound
	}

	if comment.Reactions != nil && comment.Reactions[emoji] > 0 {
		comment.Reactions[emoji]--
		if comment.Reactions[emoji] == 0 {
			delete(comment.Reactions, emoji)
		}
	}

	cm.saveConfig()
	return nil
}

// GetMentions 获取用户被提及的列表.
func (cm *CommentManager) GetMentions(userID string) []*Comment {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	mentions := make([]*Comment, 0)

	for _, comment := range cm.comments {
		if comment.IsDeleted {
			continue
		}
		for _, m := range comment.Mentions {
			if m.UserID == userID {
				mentions = append(mentions, comment)
				break
			}
		}
	}

	sortCommentsByTime(mentions)
	return mentions
}

// GetCommentThread 获取评论线程（包括所有回复）.
func (cm *CommentManager) GetCommentThread(resourceID string) []*Comment {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	commentIDs := cm.resourceIndex[resourceID]
	if commentIDs == nil {
		return []*Comment{}
	}

	// 构建树形结构
	threads := make([]*Comment, 0)
	repliesMap := make(map[string][]*Comment)

	for id := range commentIDs {
		if comment, ok := cm.comments[id]; ok && !comment.IsDeleted {
			if comment.ParentID == "" {
				threads = append(threads, comment)
			} else {
				repliesMap[comment.ParentID] = append(repliesMap[comment.ParentID], comment)
			}
		}
	}

	// 按时间排序主评论
	sortCommentsByTime(threads)

	// 构建结果（扁平化，但保持顺序）
	result := make([]*Comment, 0)
	for _, comment := range threads {
		result = append(result, comment)
		if replies, ok := repliesMap[comment.ID]; ok {
			sortCommentsByTime(replies)
			result = append(result, replies...)
		}
	}

	return result
}

// GetStats 获取评论统计.
func (cm *CommentManager) GetStats() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	totalComments := 0
	activeComments := 0
	deletedComments := 0
	totalReplies := 0
	totalMentions := 0

	for _, comment := range cm.comments {
		totalComments++
		if comment.IsDeleted {
			deletedComments++
		} else {
			activeComments++
		}
		if comment.ParentID != "" {
			totalReplies++
		}
		totalMentions += len(comment.Mentions)
	}

	return map[string]interface{}{
		"total_comments":   totalComments,
		"active_comments":  activeComments,
		"deleted_comments": deletedComments,
		"total_replies":    totalReplies,
		"total_mentions":   totalMentions,
	}
}

// SearchComments 搜索评论.
func (cm *CommentManager) SearchComments(query string, limit int) []*Comment {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	results := make([]*Comment, 0)
	query = strings.ToLower(query)

	for _, comment := range cm.comments {
		if comment.IsDeleted {
			continue
		}
		if strings.Contains(strings.ToLower(comment.Content), query) {
			results = append(results, comment)
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}

	sortCommentsByTime(results)
	return results
}

// truncateContent 截断内容.
func truncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "..."
}

// sortCommentsByTime 按时间排序评论.
func sortCommentsByTime(comments []*Comment) {
	// 简单冒泡排序，对于小规模数据足够
	for i := 0; i < len(comments)-1; i++ {
		for j := i + 1; j < len(comments); j++ {
			if comments[i].CreatedAt.Before(comments[j].CreatedAt) {
				comments[i], comments[j] = comments[j], comments[i]
			}
		}
	}
}
