// Package mobileapi 提供移动端远程管理API服务
package mobileapi

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// PushConfig 推送服务配置.
type PushConfig struct {
	FCM  *FCMConfig  `json:"fcm,omitempty"`  // FCM配置
	APNs *APNsConfig `json:"apns,omitempty"` // APNs配置
}

// FCMConfig Firebase Cloud Messaging 配置.
type FCMConfig struct {
	ProjectID      string `json:"projectId"`      // Firebase项目ID
	ServiceAccount string `json:"-"`              // 服务账号JSON（敏感信息）
	Enabled        bool   `json:"enabled"`        // 启用FCM
}

// APNsConfig Apple Push Notification Service 配置.
type APNsConfig struct {
	TeamID    string `json:"teamId"`    // Apple Team ID
	KeyID     string `json:"keyId"`     // 密钥ID
	BucketID  string `json:"bucketId"`  // 环境 (production/development)
	CertPath  string `json:"certPath"`  // 证书路径
	KeyPath   string `json:"keyPath"`   // 密钥路径
	Enabled   bool   `json:"enabled"`   // 启用APNs
	UseToken  bool   `json:"useToken"`  // 使用Token认证
}

// DefaultPushConfig 返回默认推送配置.
func DefaultPushConfig() *PushConfig {
	return &PushConfig{
		FCM: &FCMConfig{
			Enabled: true,
		},
		APNs: &APNsConfig{
			Enabled:  true,
			BucketID: "production",
			UseToken: true,
		},
	}
}

// PushService 推送通知服务.
type PushService struct {
	mu      sync.RWMutex
	config  *PushConfig
	history []*PushNotification
	workers int
	queue   chan *PushNotification
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewPushService 创建推送服务.
func NewPushService(config *PushConfig, workers int) *PushService {
	if config == nil {
		config = DefaultPushConfig()
	}
	if workers <= 0 {
		workers = 5
	}

	ctx, cancel := context.WithCancel(context.Background())

	svc := &PushService{
		config:  config,
		workers: workers,
		queue:   make(chan *PushNotification, 1000),
		ctx:     ctx,
		cancel:  cancel,
	}

	// 启动工作协程
	for i := 0; i < workers; i++ {
		go svc.worker(i)
	}

	return svc
}

// Send 发送推送通知.
func (s *PushService) Send(notification *PushNotification) error {
	if notification.ID == "" {
		notification.ID = generateID()
	}

	// 获取设备信息确定推送提供商
	if notification.Provider == "" {
		// TODO: 从设备信息获取Provider
		notification.Provider = ProviderFCM
	}

	// 加入队列
	select {
	case s.queue <- notification:
		return nil
	default:
		return fmt.Errorf("push queue is full")
	}
}

// worker 工作协程.
func (s *PushService) worker(id int) {
	for {
		select {
		case <-s.ctx.Done():
			return
		case notification := <-s.queue:
			s.processNotification(notification)
		}
	}
}

// processNotification 处理推送通知.
func (s *PushService) processNotification(notification *PushNotification) {
	var err error

	switch notification.Provider {
	case ProviderFCM:
		err = s.sendFCM(notification)
	case ProviderAPNs:
		err = s.sendAPNs(notification)
	default:
		err = fmt.Errorf("unsupported push provider: %s", notification.Provider)
	}

	now := time.Now()
	if err != nil {
		notification.Error = err.Error()
		notification.Sent = false
	} else {
		notification.SentAt = &now
		notification.Sent = true
	}

	// 保存到历史记录
	s.mu.Lock()
	s.history = append(s.history, notification)
	// 限制历史记录数量
	if len(s.history) > 1000 {
		s.history = s.history[len(s.history)-1000:]
	}
	s.mu.Unlock()
}

// sendFCM 发送FCM推送.
func (s *PushService) sendFCM(notification *PushNotification) error {
	if !s.config.FCM.Enabled {
		return fmt.Errorf("FCM is disabled")
	}

	// TODO: 实现FCM推送
	// 使用 firebase.google.com/go/messaging 包
	// 示例代码：
	// client, err := firebase.NewApp(ctx, nil, option.WithCredentialsJSON([]byte(s.config.FCM.ServiceAccount)))
	// messagingClient, err := client.Messaging(ctx)
	// message := &messaging.Message{
	//     Token: notification.DeviceToken,
	//     Notification: &messaging.Notification{
	//         Title: notification.Title,
	//         Body:  notification.Body,
	//         Image: notification.Image,
	//     },
	//     Data: notification.Data,
	// }
	// _, err = messagingClient.Send(ctx, message)

	return nil
}

// sendAPNs 发送APNs推送.
func (s *PushService) sendAPNs(notification *PushNotification) error {
	if !s.config.APNs.Enabled {
		return fmt.Errorf("APNs is disabled")
	}

	// TODO: 实现APNs推送
	// 使用 github.com/sideshow/apns2 包
	// 示例代码：
	// client := apns2.NewTokenClient(token)
	// notification := &apns2.Notification{
	//     DeviceToken: notification.DeviceToken,
	//     Topic:       "com.nasos.mobile",
	//     Payload: map[string]interface{}{
	//         "aps": map[string]interface{}{
	//             "alert": map[string]interface{}{
	//                 "title": notification.Title,
	//                 "body":  notification.Body,
	//             },
	//             "badge": notification.Badge,
	//             "sound": notification.Sound,
	//         },
	//         "custom": notification.Data,
	//     },
	// }
	// _, err := client.Push(notification)

	return nil
}

// GetHistory 获取推送历史.
func (s *PushService) GetHistory() []*PushNotification {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 返回副本
	history := make([]*PushNotification, len(s.history))
	copy(history, s.history)
	return history
}

// ClearHistory 清空推送历史.
func (s *PushService) ClearHistory() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = nil
}

// Stop 停止推送服务.
func (s *PushService) Stop() {
	s.cancel()
}
