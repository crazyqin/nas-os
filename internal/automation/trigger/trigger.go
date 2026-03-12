package trigger

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

// TriggerType 触发器类型
type TriggerType string

const (
	TriggerTypeFile    TriggerType = "file"
	TriggerTypeTime    TriggerType = "time"
	TriggerTypeEvent   TriggerType = "event"
	TriggerTypeWebhook TriggerType = "webhook"
)

// Trigger 触发器接口
type Trigger interface {
	GetType() TriggerType
	Start(ctx context.Context, callback func(map[string]interface{})) error
	Stop() error
}

// FileTrigger 文件触发器 - 监控文件变化
type FileTrigger struct {
	Type      TriggerType `json:"type"`
	Path      string      `json:"path"`
	Pattern   string      `json:"pattern,omitempty"`
	Events    []string    `json:"events"` // created, modified, deleted
	Recursive bool        `json:"recursive"`
}

func (t *FileTrigger) GetType() TriggerType {
	return TriggerTypeFile
}

func (t *FileTrigger) Start(ctx context.Context, callback func(map[string]interface{})) error {
	// TODO: 实现文件监控
	fmt.Printf("Starting file trigger on path: %s\n", t.Path)
	return nil
}

func (t *FileTrigger) Stop() error {
	return nil
}

// TimeTrigger 时间触发器 - 定时执行
type TimeTrigger struct {
	Type     TriggerType `json:"type"`
	Schedule string      `json:"schedule"` // cron 表达式
	Timezone string      `json:"timezone,omitempty"`
	Once     bool        `json:"once,omitempty"`
}

func (t *TimeTrigger) GetType() TriggerType {
	return TriggerTypeTime
}

func (t *TimeTrigger) Start(ctx context.Context, callback func(map[string]interface{})) error {
	loc := time.Local
	if t.Timezone != "" {
		var err error
		loc, err = time.LoadLocation(t.Timezone)
		if err != nil {
			return fmt.Errorf("invalid timezone: %w", err)
		}
	}

	c := cron.New(cron.WithLocation(loc))
	_, err := c.AddFunc(t.Schedule, func() {
		callback(map[string]interface{}{
			"trigger_type": "time",
			"timestamp":    time.Now(),
		})
	})
	if err != nil {
		return fmt.Errorf("invalid cron schedule: %w", err)
	}

	c.Start()

	// 在 context 取消时停止
	go func() {
		<-ctx.Done()
		c.Stop()
	}()

	return nil
}

func (t *TimeTrigger) Stop() error {
	return nil
}

// EventTrigger 事件触发器 - 系统事件
type EventTrigger struct {
	Type      TriggerType            `json:"type"`
	EventType string                 `json:"event_type"` // system.startup, user.login, file.upload, etc.
	Filter    map[string]interface{} `json:"filter,omitempty"`
}

func (t *EventTrigger) GetType() TriggerType {
	return TriggerTypeEvent
}

func (t *EventTrigger) Start(ctx context.Context, callback func(map[string]interface{})) error {
	// TODO: 实现事件订阅
	fmt.Printf("Starting event trigger for: %s\n", t.EventType)
	return nil
}

func (t *EventTrigger) Stop() error {
	return nil
}

// WebhookTrigger Webhook 触发器 - HTTP 回调
type WebhookTrigger struct {
	Type    TriggerType       `json:"type"`
	Path    string            `json:"path"`
	Method  string            `json:"method,omitempty"`
	Secret  string            `json:"secret,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

func (t *WebhookTrigger) GetType() TriggerType {
	return TriggerTypeWebhook
}

func (t *WebhookTrigger) Start(ctx context.Context, callback func(map[string]interface{})) error {
	// TODO: 实现 webhook 端点
	fmt.Printf("Starting webhook trigger on path: %s\n", t.Path)
	return nil
}

func (t *WebhookTrigger) Stop() error {
	return nil
}

// TriggerConfig 触发器配置（用于 JSON 序列化）
type TriggerConfig struct {
	Type TriggerType `json:"type"`

	// File trigger fields
	Path      string   `json:"path,omitempty"`
	Pattern   string   `json:"pattern,omitempty"`
	Events    []string `json:"events,omitempty"`
	Recursive bool     `json:"recursive,omitempty"`

	// Time trigger fields
	Schedule string `json:"schedule,omitempty"`
	Timezone string `json:"timezone,omitempty"`
	Once     bool   `json:"once,omitempty"`

	// Event trigger fields
	EventType string                 `json:"event_type,omitempty"`
	Filter    map[string]interface{} `json:"filter,omitempty"`

	// Webhook trigger fields
	Method  string            `json:"method,omitempty"`
	Secret  string            `json:"secret,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// NewTriggerFromConfig 从配置创建触发器
func NewTriggerFromConfig(config TriggerConfig) (Trigger, error) {
	switch config.Type {
	case TriggerTypeFile:
		return &FileTrigger{
			Type:      config.Type,
			Path:      config.Path,
			Pattern:   config.Pattern,
			Events:    config.Events,
			Recursive: config.Recursive,
		}, nil
	case TriggerTypeTime:
		return &TimeTrigger{
			Type:     config.Type,
			Schedule: config.Schedule,
			Timezone: config.Timezone,
			Once:     config.Once,
		}, nil
	case TriggerTypeEvent:
		return &EventTrigger{
			Type:      config.Type,
			EventType: config.EventType,
			Filter:    config.Filter,
		}, nil
	case TriggerTypeWebhook:
		return &WebhookTrigger{
			Type:    config.Type,
			Path:    config.Path,
			Method:  config.Method,
			Secret:  config.Secret,
			Headers: config.Headers,
		}, nil
	default:
		return nil, fmt.Errorf("unknown trigger type: %s", config.Type)
	}
}
