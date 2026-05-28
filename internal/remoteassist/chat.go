// chat.go - 实时文字聊天
package remoteassist

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ChatService 聊天服务.
type ChatService struct {
	messages map[string][]*ChatMessage
	channels map[string]chan *ChatMessage
	mu       sync.RWMutex
}

// NewChatService 创建聊天服务.
func NewChatService() *ChatService {
	return &ChatService{
		messages: make(map[string][]*ChatMessage),
		channels: make(map[string]chan *ChatMessage),
	}
}

// SendMessage 发送消息.
func (s *ChatService) SendMessage(sessionID string, senderID, senderName, content, msgType string) (*ChatMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, exists := s.channels[sessionID]
	if !exists {
		return nil, fmt.Errorf("聊天频道不存在: %s", sessionID)
	}

	msg := &ChatMessage{
		ID:         uuid.New().String(),
		SessionID:  sessionID,
		SenderID:   senderID,
		SenderName: senderName,
		Type:       msgType,
		Content:    content,
		Timestamp:  time.Now(),
		Metadata:   make(map[string]string),
	}

	// 存储消息
	s.messages[sessionID] = append(s.messages[sessionID], msg)

	// 发送到频道
	select {
	case s.channels[sessionID] <- msg:
	default:
		log.Printf("⚠️ 聊天频道已满: %s", sessionID)
	}

	log.Printf("💬 发送消息: %s -> %s, 内容: %s", senderName, sessionID, truncate(content, 50))
	return msg, nil
}

// truncate 截断字符串.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// CreateChannel 创建聊天频道.
func (s *ChatService) CreateChannel(sessionID string, bufferSize int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if bufferSize <= 0 {
		bufferSize = 100
	}

	s.channels[sessionID] = make(chan *ChatMessage, bufferSize)
	s.messages[sessionID] = make([]*ChatMessage, 0)

	log.Printf("📢 创建聊天频道: %s, 缓冲区: %d", sessionID, bufferSize)
}

// CloseChannel 关闭聊天频道.
func (s *ChatService) CloseChannel(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ch, exists := s.channels[sessionID]; exists {
		close(ch)
		delete(s.channels, sessionID)
	}

	log.Printf("📢 关闭聊天频道: %s", sessionID)
}

// GetMessages 获取消息.
func (s *ChatService) GetMessages(sessionID string, limit int) ([]*ChatMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	messages, exists := s.messages[sessionID]
	if !exists {
		return nil, fmt.Errorf("聊天频道不存在: %s", sessionID)
	}

	if limit <= 0 || limit > len(messages) {
		limit = len(messages)
	}

	return messages[len(messages)-limit:], nil
}

// GetChannel 获取频道.
func (s *ChatService) GetChannel(sessionID string) (<-chan *ChatMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ch, exists := s.channels[sessionID]
	if !exists {
		return nil, fmt.Errorf("聊天频道不存在: %s", sessionID)
	}

	return ch, nil
}

// SendSystemMessage 发送系统消息.
func (s *ChatService) SendSystemMessage(sessionID string, content string) (*ChatMessage, error) {
	return s.SendMessage(sessionID, "system", "系统", content, "system")
}

// SendFileMessage 发送文件消息.
func (s *ChatService) SendFileMessage(sessionID string, senderID, senderName string, fileInfo *FileInfo) (*ChatMessage, error) {
	content := fmt.Sprintf("📎 %s (%s)", fileInfo.Name, formatFileSize(fileInfo.Size))
	return s.SendMessage(sessionID, senderID, senderName, content, "file")
}

// formatFileSize 格式化文件大小.
func formatFileSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.2f GB", float64(size)/float64(GB))
	case size >= MB:
		return fmt.Sprintf("%.2f MB", float64(size)/float64(MB))
	case size >= KB:
		return fmt.Sprintf("%.2f KB", float64(size)/float64(KB))
	default:
		return fmt.Sprintf("%d B", size)
	}
}

// SearchMessages 搜索消息.
func (s *ChatService) SearchMessages(sessionID string, keyword string) ([]*ChatMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	messages, exists := s.messages[sessionID]
	if !exists {
		return nil, fmt.Errorf("聊天频道不存在: %s", sessionID)
	}

	result := make([]*ChatMessage, 0)
	for _, msg := range messages {
		if contains(msg.Content, keyword) || contains(msg.SenderName, keyword) {
			result = append(result, msg)
		}
	}

	return result, nil
}

// contains 检查字符串包含.
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// GetMessageStats 获取消息统计.
func (s *ChatService) GetMessageStats(sessionID string) (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	messages, exists := s.messages[sessionID]
	if !exists {
		return nil, fmt.Errorf("聊天频道不存在: %s", sessionID)
	}

	stats := map[string]interface{}{
		"total_messages": len(messages),
		"by_type":        make(map[string]int),
		"by_sender":      make(map[string]int),
	}

	byType := stats["by_type"].(map[string]int)
	bySender := stats["by_sender"].(map[string]int)

	for _, msg := range messages {
		byType[msg.Type]++
		bySender[msg.SenderName]++
	}

	return stats, nil
}

// DeleteMessage 删除消息.
func (s *ChatService) DeleteMessage(sessionID string, messageID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	messages, exists := s.messages[sessionID]
	if !exists {
		return fmt.Errorf("聊天频道不存在: %s", sessionID)
	}

	for i, msg := range messages {
		if msg.ID == messageID {
			s.messages[sessionID] = append(messages[:i], messages[i+1:]...)
			log.Printf("🗑️ 删除消息: %s", messageID)
			return nil
		}
	}

	return fmt.Errorf("消息不存在: %s", messageID)
}

// ClearMessages 清空消息.
func (s *ChatService) ClearMessages(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.messages[sessionID]; !exists {
		return fmt.Errorf("聊天频道不存在: %s", sessionID)
	}

	s.messages[sessionID] = make([]*ChatMessage, 0)
	log.Printf("🧹 清空消息: %s", sessionID)

	return nil
}

// ListChannels 列出频道.
func (s *ChatService) ListChannels() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]string, 0, len(s.channels))
	for sessionID := range s.channels {
		result = append(result, sessionID)
	}
	return result
}

// GetChannelInfo 获取频道信息.
func (s *ChatService) GetChannelInfo(sessionID string) (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ch, exists := s.channels[sessionID]
	if !exists {
		return nil, fmt.Errorf("聊天频道不存在: %s", sessionID)
	}

	messages := s.messages[sessionID]

	info := map[string]interface{}{
		"session_id":     sessionID,
		"buffer_size":    cap(ch),
		"buffer_used":    len(ch),
		"total_messages": len(messages),
	}

	if len(messages) > 0 {
		info["last_message"] = messages[len(messages)-1]
	}

	return info, nil
}
