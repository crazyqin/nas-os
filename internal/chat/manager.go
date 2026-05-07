// Package chat 提供即时通讯核心业务逻辑
package chat

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 即时通讯管理器.
type Manager struct {
	channels map[string]*Channel
	messages map[string]*Message      // messageID -> Message
	members  map[string][]*ChannelMember // channelID -> members
	mu       sync.RWMutex
}

// NewManager 创建即时通讯管理器.
func NewManager() *Manager {
	return &Manager{
		channels: make(map[string]*Channel),
		messages: make(map[string]*Message),
		members:  make(map[string][]*ChannelMember),
	}
}

// ========== Channel CRUD ==========

// CreateChannel 创建频道.
func (m *Manager) CreateChannel(req CreateChannelRequest) *Channel {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	ch := &Channel{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		CreatorID:   req.CreatorID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	m.channels[ch.ID] = ch

	// 创建者自动加入频道，角色为 owner
	m.members[ch.ID] = []*ChannelMember{
		{
			ChannelID: ch.ID,
			UserID:    req.CreatorID,
			Role:      MemberRoleOwner,
			JoinedAt:  now,
			LastRead:  now,
		},
	}

	return ch
}

// GetChannel 获取频道详情.
func (m *Manager) GetChannel(id string) (*Channel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ch, ok := m.channels[id]
	if !ok {
		return nil, fmt.Errorf("channel %q not found", id)
	}
	return ch, nil
}

// ListChannels 列出所有频道.
func (m *Manager) ListChannels() []*Channel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	chs := make([]*Channel, 0, len(m.channels))
	for _, ch := range m.channels {
		chs = append(chs, ch)
	}

	sort.Slice(chs, func(i, j int) bool {
		return chs[i].UpdatedAt.After(chs[j].UpdatedAt)
	})

	return chs
}

// ListChannelsByUser 列出用户所在的频道.
func (m *Manager) ListChannelsByUser(userID string) []*Channel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	chSet := make(map[string]struct{})
	for _, members := range m.members {
		for _, mem := range members {
			if mem.UserID == userID {
				chSet[mem.ChannelID] = struct{}{}
			}
		}
	}

	chs := make([]*Channel, 0, len(chSet))
	for chID := range chSet {
		if ch, ok := m.channels[chID]; ok {
			chs = append(chs, ch)
		}
	}

	sort.Slice(chs, func(i, j int) bool {
		return chs[i].UpdatedAt.After(chs[j].UpdatedAt)
	})

	return chs
}

// UpdateChannel 更新频道.
func (m *Manager) UpdateChannel(id string, req UpdateChannelRequest) (*Channel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch, ok := m.channels[id]
	if !ok {
		return nil, fmt.Errorf("channel %q not found", id)
	}

	if req.Name != nil {
		ch.Name = *req.Name
	}
	if req.Description != nil {
		ch.Description = *req.Description
	}

	ch.UpdatedAt = time.Now()
	return ch, nil
}

// DeleteChannel 删除频道及其消息和成员.
func (m *Manager) DeleteChannel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.channels[id]; !ok {
		return fmt.Errorf("channel %q not found", id)
	}

	// 删除该频道所有消息
	for mid, msg := range m.messages {
		if msg.ChannelID == id {
			delete(m.messages, mid)
		}
	}

	// 删除成员列表
	delete(m.members, id)
	delete(m.channels, id)
	return nil
}

// ========== Message CRUD ==========

// SendMessage 发送消息.
func (m *Manager) SendMessage(channelID string, req SendMessageRequest) (*Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.channels[channelID]; !ok {
		return nil, fmt.Errorf("channel %q not found", channelID)
	}

	// 验证 ReplyTo 存在性
	if req.ReplyTo != "" {
		replyMsg, ok := m.messages[req.ReplyTo]
		if !ok || replyMsg.ChannelID != channelID {
			return nil, fmt.Errorf("reply target %q not found in channel", req.ReplyTo)
		}
	}

	msgType := req.Type
	if msgType == "" {
		msgType = MessageTypeText
	}

	now := time.Now()
	msg := &Message{
		ID:        uuid.New().String(),
		ChannelID: channelID,
		SenderID:  req.SenderID,
		Content:   req.Content,
		Type:      msgType,
		ReplyTo:   req.ReplyTo,
		Reactions: []Reaction{},
		CreatedAt: now,
	}

	m.messages[msg.ID] = msg

	// 更新频道 UpdatedAt
	m.channels[channelID].UpdatedAt = now

	return msg, nil
}

// GetMessages 获取频道消息列表（分页，按时间倒序）.
func (m *Manager) GetMessages(channelID string, limit, offset int) ([]*Message, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.channels[channelID]; !ok {
		return nil, 0, fmt.Errorf("channel %q not found", channelID)
	}

	var msgs []*Message
	for _, msg := range m.messages {
		if msg.ChannelID == channelID && msg.DeletedAt == nil {
			msgs = append(msgs, msg)
		}
	}

	sort.Slice(msgs, func(i, j int) bool {
		return msgs[i].CreatedAt.After(msgs[j].CreatedAt)
	})

	total := len(msgs)

	// 应用 offset
	if offset > 0 && offset < len(msgs) {
		msgs = msgs[offset:]
	} else if offset >= len(msgs) {
		return []*Message{}, total, nil
	}

	// 应用 limit
	if limit > 0 && limit < len(msgs) {
		msgs = msgs[:limit]
	}

	return msgs, total, nil
}

// EditMessage 编辑消息.
func (m *Manager) EditMessage(msgID string, content string) (*Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	msg, ok := m.messages[msgID]
	if !ok {
		return nil, fmt.Errorf("message %q not found", msgID)
	}

	if msg.DeletedAt != nil {
		return nil, fmt.Errorf("message %q has been deleted", msgID)
	}

	msg.Content = content
	now := time.Now()
	msg.EditedAt = &now

	return msg, nil
}

// DeleteMessage 删除消息（软删除）.
func (m *Manager) DeleteMessage(msgID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	msg, ok := m.messages[msgID]
	if !ok {
		return fmt.Errorf("message %q not found", msgID)
	}

	now := time.Now()
	msg.DeletedAt = &now
	return nil
}

// ========== Member Management ==========

// AddMember 添加成员到频道.
func (m *Manager) AddMember(channelID string, req AddMemberRequest) (*ChannelMember, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.channels[channelID]; !ok {
		return nil, fmt.Errorf("channel %q not found", channelID)
	}

	// 检查是否已是成员
	for _, mem := range m.members[channelID] {
		if mem.UserID == req.UserID {
			return nil, fmt.Errorf("user %q is already a member of channel %q", req.UserID, channelID)
		}
	}

	role := req.Role
	if role == "" {
		role = MemberRoleMember
	}

	member := &ChannelMember{
		ChannelID: channelID,
		UserID:    req.UserID,
		Role:      role,
		JoinedAt:  time.Now(),
		LastRead:  time.Time{},
	}

	m.members[channelID] = append(m.members[channelID], member)
	return member, nil
}

// RemoveMember 从频道移除成员.
func (m *Manager) RemoveMember(channelID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	members, ok := m.members[channelID]
	if !ok {
		return fmt.Errorf("channel %q not found", channelID)
	}

	for i, mem := range members {
		if mem.UserID == userID {
			m.members[channelID] = append(members[:i], members[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("user %q is not a member of channel %q", userID, channelID)
}

// UpdateMemberRole 更新成员角色.
func (m *Manager) UpdateMemberRole(channelID, userID string, role MemberRole) (*ChannelMember, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	members, ok := m.members[channelID]
	if !ok {
		return nil, fmt.Errorf("channel %q not found", channelID)
	}

	for _, mem := range members {
		if mem.UserID == userID {
			mem.Role = role
			return mem, nil
		}
	}

	return nil, fmt.Errorf("user %q is not a member of channel %q", userID, channelID)
}

// ListMembers 列出频道成员.
func (m *Manager) ListMembers(channelID string) ([]*ChannelMember, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	members, ok := m.members[channelID]
	if !ok {
		return nil, fmt.Errorf("channel %q not found", channelID)
	}

	result := make([]*ChannelMember, len(members))
	copy(result, members)
	return result, nil
}

// ========== Reactions ==========

// AddReaction 给消息添加反应.
func (m *Manager) AddReaction(msgID string, req AddReactionRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	msg, ok := m.messages[msgID]
	if !ok {
		return fmt.Errorf("message %q not found", msgID)
	}

	if msg.DeletedAt != nil {
		return fmt.Errorf("message %q has been deleted", msgID)
	}

	// 检查是否已添加相同 reaction
	for _, r := range msg.Reactions {
		if r.Emoji == req.Emoji && r.UserID == req.UserID {
			return fmt.Errorf("user %q already reacted with %q on message %q", req.UserID, req.Emoji, msgID)
		}
	}

	msg.Reactions = append(msg.Reactions, Reaction{
		Emoji:     req.Emoji,
		UserID:    req.UserID,
		CreatedAt: time.Now(),
	})

	return nil
}

// RemoveReaction 移除消息反应.
func (m *Manager) RemoveReaction(msgID string, req RemoveReactionRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	msg, ok := m.messages[msgID]
	if !ok {
		return fmt.Errorf("message %q not found", msgID)
	}

	for i, r := range msg.Reactions {
		if r.Emoji == req.Emoji && r.UserID == req.UserID {
			msg.Reactions = append(msg.Reactions[:i], msg.Reactions[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("reaction %q from user %q not found on message %q", req.Emoji, req.UserID, msgID)
}

// ========== Read Tracking ==========

// MarkAsRead 标记频道消息为已读.
func (m *Manager) MarkAsRead(channelID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	members, ok := m.members[channelID]
	if !ok {
		return fmt.Errorf("channel %q not found", channelID)
	}

	for _, mem := range members {
		if mem.UserID == userID {
			mem.LastRead = time.Now()
			return nil
		}
	}

	return fmt.Errorf("user %q is not a member of channel %q", userID, channelID)
}

// GetUnreadCount 获取用户在各频道的未读消息数.
func (m *Manager) GetUnreadCount(userID string) []UnreadCount {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []UnreadCount

	// 找出用户所在的频道
	for chID, members := range m.members {
		var lastRead time.Time
		found := false
		for _, mem := range members {
			if mem.UserID == userID {
				lastRead = mem.LastRead
				found = true
				break
			}
		}

		if !found {
			continue
		}

		// 统计该频道中 lastRead 之后的消息数
		count := 0
		for _, msg := range m.messages {
			if msg.ChannelID == chID && msg.DeletedAt == nil && msg.CreatedAt.After(lastRead) {
				count++
			}
		}

		if count > 0 {
			result = append(result, UnreadCount{
				ChannelID: chID,
				Count:     count,
			})
		}
	}

	return result
}

// ========== Search ==========

// SearchMessages 搜索消息.
func (m *Manager) SearchMessages(query string, channelID string) []*Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	q := strings.ToLower(query)
	var result []*Message

	for _, msg := range m.messages {
		if msg.DeletedAt != nil {
			continue
		}
		if channelID != "" && msg.ChannelID != channelID {
			continue
		}
		if strings.Contains(strings.ToLower(msg.Content), q) {
			result = append(result, msg)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}
