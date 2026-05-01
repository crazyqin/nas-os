// Package drivesync 提供协作编辑服务
package drivesync

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// CollaborationService 协作编辑服务.
type CollaborationService struct {
	mu        sync.RWMutex
	manager   *Manager
	listeners map[string][]chan WebSocketMessage // filePath -> []chan
}

// NewCollaborationService 创建协作编辑服务.
func NewCollaborationService(manager *Manager) *CollaborationService {
	return &CollaborationService{
		manager:   manager,
		listeners: make(map[string][]chan WebSocketMessage),
	}
}

// Subscribe 订阅文件变更通知.
func (cs *CollaborationService) Subscribe(filePath string, clientID string) chan WebSocketMessage {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	ch := make(chan WebSocketMessage, 50)
	cs.listeners[filePath] = append(cs.listeners[filePath], ch)

	// 记录活动
	cs.manager.mu.Lock()
	cs.manager.addActivity(ActivitySyncStarted, filePath, clientID, "", "用户订阅了文件变更通知")
	cs.manager.mu.Unlock()

	return ch
}

// Unsubscribe 取消订阅文件变更通知.
func (cs *CollaborationService) Unsubscribe(filePath string, ch chan WebSocketMessage) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	listeners := cs.listeners[filePath]
	for i, listener := range listeners {
		if listener == ch {
			cs.listeners[filePath] = append(listeners[:i], listeners[i+1:]...)
			close(ch)
			break
		}
	}
}

// NotifyFileChange 通知文件变更.
func (cs *CollaborationService) NotifyFileChange(filePath string, changeType ActivityType, userID string) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	msg := WebSocketMessage{
		Type: "file_change",
		Payload: map[string]interface{}{
			"file_path":    filePath,
			"change_type":  string(changeType),
			"user_id":      userID,
		},
		Time: time.Now(),
	}

	// 广播给所有监听者
	if listeners, exists := cs.listeners[filePath]; exists {
		for _, ch := range listeners {
			select {
			case ch <- msg:
			default:
				// 缓冲区满，跳过
			}
		}
	}

	// 同时通过 Manager 的 WebSocket 广播
	cs.manager.broadcastWS(msg)
}

// NotifyLockChange 通知锁变更.
func (cs *CollaborationService) NotifyLockChange(filePath string, lock *FileLock, action string) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	msg := WebSocketMessage{
		Type: "lock_change",
		Payload: map[string]interface{}{
			"file_path": filePath,
			"action":    action,
			"lock":      lock,
		},
		Time: time.Now(),
	}

	if listeners, exists := cs.listeners[filePath]; exists {
		for _, ch := range listeners {
			select {
			case ch <- msg:
			default:
			}
		}
	}
}

// AddCommentWithMention 添加评论并通知被@提及的用户.
func (cs *CollaborationService) AddCommentWithMention(filePath string, input CommentInput) *Comment {
	comment := cs.manager.AddComment(filePath, input)

	// 通知被@提及的用户
	if len(input.Mentions) > 0 {
		msg := WebSocketMessage{
			Type: "mention",
			Payload: map[string]interface{}{
				"file_path":  filePath,
				"comment_id": comment.ID,
				"from_user":  input.UserName,
				"mentions":   input.Mentions,
				"content":    input.Content,
			},
			Time: time.Now(),
		}

		cs.manager.broadcastWS(msg)
	}

	return comment
}

// GetActivityStream 获取活动流.
func (cs *CollaborationService) GetActivityStream(filePath string, limit int) []*Activity {
	activities := cs.manager.GetActivities(limit)

	if filePath == "" {
		return activities
	}

	// 过滤特定文件的活动
	var filtered []*Activity
	for _, a := range activities {
		if a.FilePath == filePath {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

// ExtractMentions 从评论内容中提取@提及.
// 格式: @username 或 @user_id.
func ExtractMentions(content string) []string {
	var mentions []string
	// 先按空白分词，再按逗号分词
	fields := strings.Fields(content)
	var tokens []string
	for _, f := range fields {
		for _, part := range strings.Split(f, ",") {
			if part != "" {
				tokens = append(tokens, part)
			}
		}
	}
	for _, word := range tokens {
		if strings.HasPrefix(word, "@") && len(word) > 1 {
			mention := strings.TrimPrefix(word, "@")
			// 移除可能的标点符号
			mention = strings.TrimRight(mention, ".,;:!?")
			if mention != "" {
				mentions = append(mentions, mention)
			}
		}
	}
	return mentions
}

// GetOnlineUsers 获取当前在线用户（基于最近活动）.
func (cs *CollaborationService) GetOnlineUsers(timeout time.Duration) []string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	// 通过活动记录推断在线用户
	activities := cs.manager.GetActivities(100)
	seen := make(map[string]time.Time)

	now := time.Now()
	for _, a := range activities {
		if a.UserID != "" && now.Sub(a.CreatedAt) < timeout {
			if _, exists := seen[a.UserID]; !exists {
				seen[a.UserID] = a.CreatedAt
			}
		}
	}

	users := make([]string, 0, len(seen))
	for userID := range seen {
		users = append(users, userID)
	}
	return users
}

// GetCollaborators 获取文件的当前协作者.
func (cs *CollaborationService) GetCollaborators(filePath string) []string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	// 从活动记录中提取最近操作过该文件的用户
	activities := cs.manager.GetActivities(200)
	seen := make(map[string]bool)
	var collaborators []string

	for _, a := range activities {
		if a.FilePath == filePath && a.UserID != "" && !seen[a.UserID] {
			seen[a.UserID] = true
			collaborators = append(collaborators, a.UserID)
		}
	}

	return collaborators
}

// CreateFileLockWithNotification 创建文件锁并通知协作者.
func (cs *CollaborationService) CreateFileLockWithNotification(filePath string, input FileLockInput) (*FileLock, error) {
	lock, err := cs.manager.LockFile(filePath, input)
	if err != nil {
		return nil, fmt.Errorf("锁定文件失败: %w", err)
	}

	// 通知协作者
	cs.NotifyLockChange(filePath, lock, "locked")

	return lock, nil
}

// ReleaseFileLockWithNotification 释放文件锁并通知协作者.
func (cs *CollaborationService) ReleaseFileLockWithNotification(filePath string, userID string) error {
	if err := cs.manager.UnlockFile(filePath, userID); err != nil {
		return fmt.Errorf("解锁文件失败: %w", err)
	}

	// 通知协作者
	cs.NotifyLockChange(filePath, nil, "unlocked")

	return nil
}
