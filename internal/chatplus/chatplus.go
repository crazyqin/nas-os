package chatplus

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// NewChatPlusManager 创建 ChatPlus 管理器.
func NewChatPlusManager(config *ChatPlusConfig) *ChatPlusManager {
	if config == nil {
		config = DefaultChatPlusConfig()
	}

	return &ChatPlusManager{
		config:      config,
		users:       make(map[string]*ChatUser),
		channels:    make(map[string]*ChatChannel),
		messages:    make(map[string][]*ChatMessage),
		onlineUsers: make(map[string]bool),
		stopCh:      make(chan struct{}),
	}
}

// Start 启动 ChatPlus 管理器.
func (m *ChatPlusManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("ChatPlus 管理器已在运行")
	}

	m.running = true
	log.Println("[ChatPlus] 企业即时通讯管理器启动")

	// 启动消息清理器
	go m.messageCleaner()

	// 启动在线状态监控
	go m.onlineStatusMonitor()

	return nil
}

// Stop 停止 ChatPlus 管理器.
func (m *ChatPlusManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	close(m.stopCh)
	m.running = false
	log.Println("[ChatPlus] 企业即时通讯管理器停止")
}

// IsRunning 检查是否运行中.
func (m *ChatPlusManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// CreateUser 创建用户.
func (m *ChatPlusManager) CreateUser(username, fullName, email string) (*ChatUser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查用户名是否已存在
	for _, user := range m.users {
		if user.Username == username {
			return nil, fmt.Errorf("用户名已存在: %s", username)
		}
	}

	user := &ChatUser{
		ID:        uuid.New().String(),
		Username:  username,
		FullName:  fullName,
		Email:     email,
		Status:    UserStatusOffline,
		CreatedAt: time.Now(),
	}

	m.users[user.ID] = user
	log.Printf("[ChatPlus] 创建用户: %s (%s)", username, fullName)

	return user, nil
}

// GetUser 获取用户.
func (m *ChatPlusManager) GetUser(userID string) (*ChatUser, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	user, exists := m.users[userID]
	if !exists {
		return nil, fmt.Errorf("用户不存在: %s", userID)
	}

	return user, nil
}

// UpdateUserStatus 更新用户状态.
func (m *ChatPlusManager) UpdateUserStatus(userID string, status UserStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, exists := m.users[userID]
	if !exists {
		return fmt.Errorf("用户不存在: %s", userID)
	}

	user.Status = status
	user.LastSeen = time.Now()

	if status == UserStatusOnline {
		m.onlineUsers[userID] = true
	} else {
		delete(m.onlineUsers, userID)
	}

	return nil
}

// ListUsers 列出所有用户.
func (m *ChatPlusManager) ListUsers() []*ChatUser {
	m.mu.RLock()
	defer m.mu.RUnlock()

	users := make([]*ChatUser, 0, len(m.users))
	for _, user := range m.users {
		users = append(users, user)
	}
	return users
}

// CreateChannel 创建频道.
func (m *ChatPlusManager) CreateChannel(name, description string, channelType ChannelType, creatorID string, isPrivate bool) (*ChatChannel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.channels) >= m.config.MaxChannels {
		return nil, fmt.Errorf("已达到最大频道数: %d", m.config.MaxChannels)
	}

	channel := &ChatChannel{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		Type:        channelType,
		IsPrivate:   isPrivate,
		Members:     []string{creatorID},
		Admins:      []string{creatorID},
		CreatedBy:   creatorID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.channels[channel.ID] = channel
	m.messages[channel.ID] = make([]*ChatMessage, 0)

	log.Printf("[ChatPlus] 创建频道: %s (%s) - %s", name, channelType, creatorID)

	return channel, nil
}

// GetChannel 获取频道.
func (m *ChatPlusManager) GetChannel(channelID string) (*ChatChannel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	channel, exists := m.channels[channelID]
	if !exists {
		return nil, fmt.Errorf("频道不存在: %s", channelID)
	}

	return channel, nil
}

// ListChannels 列出用户可见的频道.
func (m *ChatPlusManager) ListChannels(userID string) []*ChatChannel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	channels := make([]*ChatChannel, 0)
	for _, channel := range m.channels {
		// 检查用户是否是成员
		for _, memberID := range channel.Members {
			if memberID == userID {
				channels = append(channels, channel)
				break
			}
		}
	}

	return channels
}

// JoinChannel 加入频道.
func (m *ChatPlusManager) JoinChannel(channelID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	channel, exists := m.channels[channelID]
	if !exists {
		return fmt.Errorf("频道不存在: %s", channelID)
	}

	if len(channel.Members) >= m.config.MaxMembersPerChannel {
		return fmt.Errorf("频道已达到最大成员数: %d", m.config.MaxMembersPerChannel)
	}

	// 检查是否已经是成员
	for _, memberID := range channel.Members {
		if memberID == userID {
			return nil // 已经是成员
		}
	}

	channel.Members = append(channel.Members, userID)
	channel.UpdatedAt = time.Now()

	log.Printf("[ChatPlus] 用户 %s 加入频道 %s", userID, channelID)

	return nil
}

// LeaveChannel 离开频道.
func (m *ChatPlusManager) LeaveChannel(channelID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	channel, exists := m.channels[channelID]
	if !exists {
		return fmt.Errorf("频道不存在: %s", channelID)
	}

	// 移除成员
	for i, memberID := range channel.Members {
		if memberID == userID {
			channel.Members = append(channel.Members[:i], channel.Members[i+1:]...)
			break
		}
	}

	// 移除管理员
	for i, adminID := range channel.Admins {
		if adminID == userID {
			channel.Admins = append(channel.Admins[:i], channel.Admins[i+1:]...)
			break
		}
	}

	channel.UpdatedAt = time.Now()

	log.Printf("[ChatPlus] 用户 %s 离开频道 %s", userID, channelID)

	return nil
}

// SendMessage 发送消息.
func (m *ChatPlusManager) SendMessage(channelID, senderID, content string, msgType MessageType) (*ChatMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil, fmt.Errorf("ChatPlus 管理器未运行")
	}

	// 检查频道是否存在
	channel, exists := m.channels[channelID]
	if !exists {
		return nil, fmt.Errorf("频道不存在: %s", channelID)
	}

	// 检查用户是否是频道成员
	isMember := false
	for _, memberID := range channel.Members {
		if memberID == senderID {
			isMember = true
			break
		}
	}
	if !isMember {
		return nil, fmt.Errorf("用户不是频道成员")
	}

	// 检查消息长度
	if len(content) > m.config.MaxMessageLength {
		return nil, fmt.Errorf("消息超过最大长度: %d", m.config.MaxMessageLength)
	}

	// 获取发送者信息
	sender, exists := m.users[senderID]
	if !exists {
		return nil, fmt.Errorf("发送者不存在: %s", senderID)
	}

	message := &ChatMessage{
		ID:         uuid.New().String(),
		ChannelID:  channelID,
		SenderID:   senderID,
		SenderName: sender.FullName,
		Type:       msgType,
		Content:    content,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	m.messages[channelID] = append(m.messages[channelID], message)

	// 更新频道最后消息
	channel.LastMessage = message
	channel.UpdatedAt = time.Now()

	// 触发回调
	if m.onMessage != nil {
		go m.onMessage(message)
	}

	log.Printf("[ChatPlus] 消息发送: %s -> %s (%s)", senderID, channelID, msgType)

	return message, nil
}

// GetMessages 获取频道消息.
func (m *ChatPlusManager) GetMessages(channelID string, limit int, before *time.Time) ([]*ChatMessage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	messages, exists := m.messages[channelID]
	if !exists {
		return nil, fmt.Errorf("频道不存在: %s", channelID)
	}

	// 过滤和排序
	filtered := make([]*ChatMessage, 0)
	for _, msg := range messages {
		if before != nil && msg.CreatedAt.After(*before) {
			continue
		}
		filtered = append(filtered, msg)
	}

	// 按时间降序排序
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	// 限制数量
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}

	return filtered, nil
}

// SearchMessages 搜索消息.
func (m *ChatPlusManager) SearchMessages(query string, channelID string, userID string) *SearchResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*ChatMessage
	query = strings.ToLower(query)

	for chID, messages := range m.messages {
		// 如果指定了频道，只搜索该频道
		if channelID != "" && chID != channelID {
			continue
		}

		// 检查用户是否有权限访问该频道
		channel, exists := m.channels[chID]
		if !exists {
			continue
		}

		hasAccess := false
		for _, memberID := range channel.Members {
			if memberID == userID {
				hasAccess = true
				break
			}
		}
		if !hasAccess {
			continue
		}

		// 搜索消息内容
		for _, msg := range messages {
			if strings.Contains(strings.ToLower(msg.Content), query) {
				results = append(results, msg)
			}
		}
	}

	// 按时间降序排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	return &SearchResult{
		Messages: results,
		Total:    len(results),
		Page:     1,
		PageSize: len(results),
		Query:    query,
	}
}

// EditMessage 编辑消息.
func (m *ChatPlusManager) EditMessage(messageID, channelID, userID, newContent string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	messages, exists := m.messages[channelID]
	if !exists {
		return fmt.Errorf("频道不存在: %s", channelID)
	}

	for _, msg := range messages {
		if msg.ID == messageID {
			// 检查是否是消息发送者
			if msg.SenderID != userID {
				return fmt.Errorf("只能编辑自己发送的消息")
			}

			msg.Content = newContent
			msg.Edited = true
			now := time.Now()
			msg.EditedAt = &now
			msg.UpdatedAt = now

			log.Printf("[ChatPlus] 消息已编辑: %s", messageID)
			return nil
		}
	}

	return fmt.Errorf("消息不存在: %s", messageID)
}

// DeleteMessage 删除消息.
func (m *ChatPlusManager) DeleteMessage(messageID, channelID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	messages, exists := m.messages[channelID]
	if !exists {
		return fmt.Errorf("频道不存在: %s", channelID)
	}

	for i, msg := range messages {
		if msg.ID == messageID {
			// 检查是否是消息发送者或管理员
			if msg.SenderID != userID {
				channel := m.channels[channelID]
				isAdmin := false
				for _, adminID := range channel.Admins {
					if adminID == userID {
						isAdmin = true
						break
					}
				}
				if !isAdmin {
					return fmt.Errorf("无权删除此消息")
				}
			}

			m.messages[channelID] = append(messages[:i], messages[i+1:]...)
			log.Printf("[ChatPlus] 消息已删除: %s", messageID)
			return nil
		}
	}

	return fmt.Errorf("消息不存在: %s", messageID)
}

// SetOnMessageCallback 设置消息回调.
func (m *ChatPlusManager) SetOnMessageCallback(callback func(msg *ChatMessage)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.onMessage = callback
}

// SetOnTypingCallback 设置输入回调.
func (m *ChatPlusManager) SetOnTypingCallback(callback func(channelID, userID string)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.onTyping = callback
}

// GetStats 获取统计信息.
func (m *ChatPlusManager) GetStats() *ChatStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalMessages := 0
	for _, messages := range m.messages {
		totalMessages += len(messages)
	}

	return &ChatStats{
		TotalUsers:    len(m.users),
		OnlineUsers:   len(m.onlineUsers),
		TotalChannels: len(m.channels),
		TotalMessages: totalMessages,
		LastUpdated:   time.Now(),
	}
}

// GetConfig 获取配置.
func (m *ChatPlusManager) GetConfig() *ChatPlusConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.config
}

// UpdateConfig 更新配置.
func (m *ChatPlusManager) UpdateConfig(config *ChatPlusConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = config
	log.Printf("[ChatPlus] 配置已更新")
}

// messageCleaner 消息清理器.
func (m *ChatPlusManager) messageCleaner() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.cleanOldMessages()
		}
	}
}

// cleanOldMessages 清理旧消息.
func (m *ChatPlusManager) cleanOldMessages() {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -m.config.RetentionDays)

	for channelID, messages := range m.messages {
		filtered := make([]*ChatMessage, 0)
		for _, msg := range messages {
			if msg.CreatedAt.After(cutoff) {
				filtered = append(filtered, msg)
			}
		}
		m.messages[channelID] = filtered
	}
}

// onlineStatusMonitor 在线状态监控.
func (m *ChatPlusManager) onlineStatusMonitor() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkOnlineStatus()
		}
	}
}

// checkOnlineStatus 检查在线状态.
func (m *ChatPlusManager) checkOnlineStatus() {
	m.mu.Lock()
	defer m.mu.Unlock()

	timeout := 10 * time.Minute
	now := time.Now()

	for userID, user := range m.users {
		if user.Status == UserStatusOnline && now.Sub(user.LastSeen) > timeout {
			user.Status = UserStatusAway
			delete(m.onlineUsers, userID)
			log.Printf("[ChatPlus] 用户 %s 状态变为离开", userID)
		}
	}
}

// GetOnlineUsers 获取在线用户列表.
func (m *ChatPlusManager) GetOnlineUsers() []*ChatUser {
	m.mu.RLock()
	defer m.mu.RUnlock()

	users := make([]*ChatUser, 0)
	for userID := range m.onlineUsers {
		if user, exists := m.users[userID]; exists {
			users = append(users, user)
		}
	}

	return users
}

// AddChannelAdmin 添加频道管理员.
func (m *ChatPlusManager) AddChannelAdmin(channelID, userID, adminID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	channel, exists := m.channels[channelID]
	if !exists {
		return fmt.Errorf("频道不存在: %s", channelID)
	}

	// 检查操作者是否是管理员
	isAdmin := false
	for _, id := range channel.Admins {
		if id == adminID {
			isAdmin = true
			break
		}
	}
	if !isAdmin {
		return fmt.Errorf("只有管理员可以添加管理员")
	}

	// 检查用户是否是成员
	isMember := false
	for _, id := range channel.Members {
		if id == userID {
			isMember = true
			break
		}
	}
	if !isMember {
		return fmt.Errorf("用户不是频道成员")
	}

	// 检查是否已经是管理员
	for _, id := range channel.Admins {
		if id == userID {
			return nil
		}
	}

	channel.Admins = append(channel.Admins, userID)
	channel.UpdatedAt = time.Now()

	log.Printf("[ChatPlus] 用户 %s 成为频道 %s 的管理员", userID, channelID)

	return nil
}

// RemoveChannelAdmin 移除频道管理员.
func (m *ChatPlusManager) RemoveChannelAdmin(channelID, userID, adminID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	channel, exists := m.channels[channelID]
	if !exists {
		return fmt.Errorf("频道不存在: %s", channelID)
	}

	// 检查操作者是否是管理员
	isAdmin := false
	for _, id := range channel.Admins {
		if id == adminID {
			isAdmin = true
			break
		}
	}
	if !isAdmin {
		return fmt.Errorf("只有管理员可以移除管理员")
	}

	// 移除管理员
	for i, id := range channel.Admins {
		if id == userID {
			channel.Admins = append(channel.Admins[:i], channel.Admins[i+1:]...)
			channel.UpdatedAt = time.Now()
			log.Printf("[ChatPlus] 用户 %s 被移除频道 %s 的管理员权限", userID, channelID)
			return nil
		}
	}

	return fmt.Errorf("用户不是管理员")
}
