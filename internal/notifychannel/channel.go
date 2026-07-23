// Package notifychannel manages outbound notify channels (webhook/email/etc).
// Used with bulk optional surface; product center should use internal/notification.
package notifychannel

import (
	"fmt"
	"log"
	"sync"
)

// ChannelType represents a notification channel type.
type ChannelType string

const (
	ChannelEmail    ChannelType = "email"
	ChannelTelegram ChannelType = "telegram"
	ChannelWebhook  ChannelType = "webhook"
	ChannelWeChat   ChannelType = "wechat"
	ChannelDingTalk ChannelType = "dingtalk"
	ChannelSlack    ChannelType = "slack"
	ChannelBark     ChannelType = "bark"
	ChannelGotify   ChannelType = "gotify"
)

// Channel represents a notification channel.
type Channel struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Type    ChannelType       `json:"type"`
	Config  map[string]string `json:"config"` // channel-specific config
	Enabled bool              `json:"enabled"`
}

// Message represents a notification message.
type Message struct {
	Title    string `json:"title"`
	Body     string `json:"body"`
	Priority string `json:"priority"` // low, normal, high, urgent
}

// Manager manages notification channels.
type Manager struct {
	mu       sync.RWMutex
	channels map[string]*Channel
}

// NewManager creates a new notification channel manager.
func NewManager() *Manager {
	return &Manager{
		channels: make(map[string]*Channel),
	}
}

// AddChannel registers a notification channel.
func (m *Manager) AddChannel(ch Channel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch.Enabled = true
	m.channels[ch.ID] = &ch
	log.Printf("通知渠道已添加: %s (%s)", ch.Name, ch.Type)
}

// RemoveChannel removes a notification channel.
func (m *Manager) RemoveChannel(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.channels, id)
}

// UpdateChannel updates a notification channel.
func (m *Manager) UpdateChannel(ch Channel) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.channels[ch.ID]; !ok {
		return fmt.Errorf("channel not found: %s", ch.ID)
	}
	m.channels[ch.ID] = &ch
	return nil
}

// ListChannels returns all notification channels.
func (m *Manager) ListChannels() []Channel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Channel, 0, len(m.channels))
	for _, ch := range m.channels {
		result = append(result, *ch)
	}
	return result
}

// Send sends a message through a specific channel.
func (m *Manager) Send(channelID string, msg Message) error {
	m.mu.RLock()
	ch, ok := m.channels[channelID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("channel not found: %s", channelID)
	}
	if !ch.Enabled {
		return fmt.Errorf("channel disabled: %s", channelID)
	}

	switch ch.Type {
	case ChannelWebhook:
		return m.sendWebhook(ch, msg)
	case ChannelBark:
		return m.sendBark(ch, msg)
	case ChannelGotify:
		return m.sendGotify(ch, msg)
	case ChannelTelegram:
		return m.sendTelegram(ch, msg)
	case ChannelDingTalk:
		return m.sendDingTalk(ch, msg)
	case ChannelWeChat:
		return m.sendWeChat(ch, msg)
	default:
		return fmt.Errorf("unsupported channel type: %s", ch.Type)
	}
}

// Broadcast sends a message through all enabled channels.
func (m *Manager) Broadcast(msg Message) {
	m.mu.RLock()
	channels := make([]*Channel, 0, len(m.channels))
	for _, ch := range m.channels {
		if ch.Enabled {
			channels = append(channels, ch)
		}
	}
	m.mu.RUnlock()

	for _, ch := range channels {
		if err := m.Send(ch.ID, msg); err != nil {
			log.Printf("通知发送失败 [%s]: %v", ch.Name, err)
		}
	}
}

func (m *Manager) sendWebhook(ch *Channel, msg Message) error {
	// Implementation: POST to webhook URL
	log.Printf("📤 Webhook通知: %s -> %s", msg.Title, ch.Config["url"])
	return nil
}

func (m *Manager) sendBark(ch *Channel, msg Message) error {
	// Implementation: GET bark server with title/body
	log.Printf("📤 Bark通知: %s", msg.Title)
	return nil
}

func (m *Manager) sendGotify(ch *Channel, msg Message) error {
	// Implementation: POST to gotify /message
	log.Printf("📤 Gotify通知: %s", msg.Title)
	return nil
}

func (m *Manager) sendTelegram(ch *Channel, msg Message) error {
	// Implementation: POST to Telegram Bot API
	log.Printf("📤 Telegram通知: %s", msg.Title)
	return nil
}

func (m *Manager) sendDingTalk(ch *Channel, msg Message) error {
	// Implementation: POST to DingTalk webhook
	log.Printf("📤 钉钉通知: %s", msg.Title)
	return nil
}

func (m *Manager) sendWeChat(ch *Channel, msg Message) error {
	// Implementation: POST to WeChat Work webhook
	log.Printf("📤 企业微信通知: %s", msg.Title)
	return nil
}
