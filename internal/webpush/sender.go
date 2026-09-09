// sender.go - 通知中心渠道发送器适配（渠道类型 webpush）.
package webpush

import (
	"context"
	"fmt"
	"time"

	"nas-os/internal/notification"
)

// sendTimeout 单次渠道发送的整体超时（含全部订阅的推送请求）.
const sendTimeout = 30 * time.Second

// Sender 将 Web Push 适配为通知中心渠道发送器.
type Sender struct {
	mgr *Manager
}

// NewSender 创建发送器.
func NewSender(m *Manager) *Sender {
	return &Sender{mgr: m}
}

// Type 渠道类型.
func (s *Sender) Type() notification.ChannelType {
	return notification.ChannelWebPush
}

// Send 向全部订阅推送通知.
// 只要有一台送达即视为成功（部分失败不阻断渠道，失效订阅由 Manager 自动清除）.
func (s *Sender) Send(_ *notification.ChannelConfig, n *notification.Notification) error {
	if s.mgr.Count() == 0 {
		return fmt.Errorf("没有已订阅的浏览器")
	}
	url := "/"
	if n.Data != nil {
		if u, ok := n.Data["url"].(string); ok && u != "" {
			url = u
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()
	sent, err := s.mgr.Send(ctx, n.Title, n.Message, string(n.Level), url)
	if err != nil && sent == 0 {
		return err
	}
	return nil
}
