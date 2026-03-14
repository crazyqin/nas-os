package trigger

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
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

// EventManager 全局事件管理器
type EventManager struct {
	mu          sync.RWMutex
	subscribers map[string][]chan map[string]interface{}
	logger      *zap.Logger
}

var (
	globalEventManager *EventManager
	eventManagerOnce   sync.Once
)

// GetEventManager 获取全局事件管理器实例
func GetEventManager(logger *zap.Logger) *EventManager {
	eventManagerOnce.Do(func() {
		globalEventManager = &EventManager{
			subscribers: make(map[string][]chan map[string]interface{}),
			logger:      logger,
		}
	})
	return globalEventManager
}

// Subscribe 订阅事件
func (em *EventManager) Subscribe(eventType string) <-chan map[string]interface{} {
	em.mu.Lock()
	defer em.mu.Unlock()

	ch := make(chan map[string]interface{}, 100)
	em.subscribers[eventType] = append(em.subscribers[eventType], ch)
	if em.logger != nil {
		em.logger.Debug("事件订阅已创建", zap.String("event_type", eventType))
	}
	return ch
}

// Unsubscribe 取消订阅
func (em *EventManager) Unsubscribe(eventType string, ch <-chan map[string]interface{}) {
	em.mu.Lock()
	defer em.mu.Unlock()

	subscribers := em.subscribers[eventType]
	for i, sub := range subscribers {
		// 比较通道
		if sub == ch {
			em.subscribers[eventType] = append(subscribers[:i], subscribers[i+1:]...)
			close(sub)
			if em.logger != nil {
				em.logger.Debug("事件订阅已取消", zap.String("event_type", eventType))
			}
			break
		}
	}
}

// Publish 发布事件
func (em *EventManager) Publish(eventType string, data map[string]interface{}) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	if em.logger != nil {
		em.logger.Debug("发布事件", zap.String("event_type", eventType), zap.Any("data", data))
	}

	for _, ch := range em.subscribers[eventType] {
		select {
		case ch <- data:
		default:
			// 通道已满，跳过
			if em.logger != nil {
				em.logger.Warn("事件通道已满，跳过消息", zap.String("event_type", eventType))
			}
		}
	}
}

// FileTrigger 文件触发器 - 监控文件变化
type FileTrigger struct {
	Type      TriggerType `json:"type"`
	Path      string      `json:"path"`
	Pattern   string      `json:"pattern,omitempty"`
	Events    []string    `json:"events"` // created, modified, deleted
	Recursive bool        `json:"recursive"`

	watcher *fsnotify.Watcher
	logger  *zap.Logger
	cancel  context.CancelFunc
}

// SetLogger 设置日志器
func (t *FileTrigger) SetLogger(logger *zap.Logger) {
	t.logger = logger
}

func (t *FileTrigger) GetType() TriggerType {
	return TriggerTypeFile
}

func (t *FileTrigger) Start(ctx context.Context, callback func(map[string]interface{})) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	t.watcher = watcher

	// 添加监控路径
	if t.Recursive {
		err = filepath.Walk(t.Path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				if err := watcher.Add(path); err != nil {
					if t.logger != nil {
						t.logger.Warn("添加目录监控失败", zap.String("path", path), zap.Error(err))
					}
				}
			}
			return nil
		})
	} else {
		err = watcher.Add(t.Path)
	}
	if err != nil {
		watcher.Close()
		return fmt.Errorf("failed to watch path %s: %w", t.Path, err)
	}

	ctx, cancel := context.WithCancel(ctx)
	t.cancel = cancel

	if t.logger != nil {
		t.logger.Info("文件监控已启动", zap.String("path", t.Path), zap.Bool("recursive", t.Recursive))
	}

	// 构建事件过滤器
	eventFilter := make(map[fsnotify.Op]bool)
	for _, e := range t.Events {
		switch strings.ToLower(e) {
		case "created":
			eventFilter[fsnotify.Create] = true
		case "modified":
			eventFilter[fsnotify.Write] = true
		case "deleted":
			eventFilter[fsnotify.Remove] = true
		case "renamed":
			eventFilter[fsnotify.Rename] = true
		}
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				// 检查模式匹配
				if t.Pattern != "" {
					matched, err := filepath.Match(t.Pattern, filepath.Base(event.Name))
					if err != nil || !matched {
						continue
					}
				}

				// 检查事件类型
				if len(eventFilter) > 0 {
					matched := false
					for op := range eventFilter {
						if event.Op&op != 0 {
							matched = true
							break
						}
					}
					if !matched {
						continue
					}
				}

				// 如果是目录创建，且开启了递归，则添加监控
				if event.Op&fsnotify.Create != 0 && t.Recursive {
					if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
						if err := watcher.Add(event.Name); err != nil {
							if t.logger != nil {
								t.logger.Warn("添加新目录监控失败", zap.String("path", event.Name), zap.Error(err))
							}
						}
					}
				}

				eventType := "unknown"
				switch {
				case event.Op&fsnotify.Create != 0:
					eventType = "created"
				case event.Op&fsnotify.Write != 0:
					eventType = "modified"
				case event.Op&fsnotify.Remove != 0:
					eventType = "deleted"
				case event.Op&fsnotify.Rename != 0:
					eventType = "renamed"
				}

				if t.logger != nil {
					t.logger.Debug("文件事件触发", zap.String("path", event.Name), zap.String("event", eventType))
				}

				callback(map[string]interface{}{
					"trigger_type": "file",
					"event_type":   eventType,
					"path":         event.Name,
					"operation":    event.Op.String(),
					"timestamp":    time.Now(),
				})

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				if t.logger != nil {
					t.logger.Error("文件监控错误", zap.Error(err))
				}
			}
		}
	}()

	return nil
}

func (t *FileTrigger) Stop() error {
	if t.cancel != nil {
		t.cancel()
	}
	if t.watcher != nil {
		return t.watcher.Close()
	}
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

	eventCh <-chan map[string]interface{}
	logger  *zap.Logger
	cancel  context.CancelFunc
}

// SetLogger 设置日志器
func (t *EventTrigger) SetLogger(logger *zap.Logger) {
	t.logger = logger
}

func (t *EventTrigger) GetType() TriggerType {
	return TriggerTypeEvent
}

func (t *EventTrigger) Start(ctx context.Context, callback func(map[string]interface{})) error {
	em := GetEventManager(t.logger)
	if em == nil {
		return fmt.Errorf("event manager not initialized")
	}

	// 订阅事件
	t.eventCh = em.Subscribe(t.EventType)

	ctx, cancel := context.WithCancel(ctx)
	t.cancel = cancel

	if t.logger != nil {
		t.logger.Info("事件订阅已启动", zap.String("event_type", t.EventType))
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				em.Unsubscribe(t.EventType, t.eventCh)
				return
			case data, ok := <-t.eventCh:
				if !ok {
					return
				}

				// 检查过滤条件
				if len(t.Filter) > 0 && !matchFilter(data, t.Filter) {
					continue
				}

				if t.logger != nil {
					t.logger.Debug("事件触发", zap.String("event_type", t.EventType), zap.Any("data", data))
				}

				callback(map[string]interface{}{
					"trigger_type": "event",
					"event_type":   t.EventType,
					"data":         data,
					"timestamp":    time.Now(),
				})
			}
		}
	}()

	return nil
}

func (t *EventTrigger) Stop() error {
	if t.cancel != nil {
		t.cancel()
	}
	return nil
}

// matchFilter 检查数据是否匹配过滤条件
func matchFilter(data map[string]interface{}, filter map[string]interface{}) bool {
	for key, expected := range filter {
		actual, exists := data[key]
		if !exists {
			return false
		}
		if actual != expected {
			return false
		}
	}
	return true
}

// WebhookTrigger Webhook 触发器 - HTTP 回调
type WebhookTrigger struct {
	Type    TriggerType       `json:"type"`
	Path    string            `json:"path"`
	Method  string            `json:"method,omitempty"`
	Secret  string            `json:"secret,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`

	server   *http.Server
	logger   *zap.Logger
	cancel   context.CancelFunc
	callback func(map[string]interface{})
}

// SetLogger 设置日志器
func (t *WebhookTrigger) SetLogger(logger *zap.Logger) {
	t.logger = logger
}

func (t *WebhookTrigger) GetType() TriggerType {
	return TriggerTypeWebhook
}

func (t *WebhookTrigger) Start(ctx context.Context, callback func(map[string]interface{})) error {
	t.callback = callback

	// 默认方法为 POST
	method := t.Method
	if method == "" {
		method = "POST"
	}

	mux := http.NewServeMux()
	mux.HandleFunc(t.Path, t.handleWebhook)

	// 使用随机可用端口
	t.server = &http.Server{
		Addr:    ":0", // 随机端口
		Handler: mux,
	}

	ctx, cancel := context.WithCancel(ctx)
	t.cancel = cancel

	// 启动服务器
	go func() {
		if t.logger != nil {
			t.logger.Info("Webhook 服务器启动", zap.String("path", t.Path), zap.String("method", method))
		}

		if err := t.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			if t.logger != nil {
				t.logger.Error("Webhook 服务器错误", zap.Error(err))
			}
		}
	}()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 获取实际监听地址
	if t.logger != nil {
		t.logger.Info("Webhook 端点已就绪", zap.String("addr", t.server.Addr), zap.String("path", t.Path))
	}

	return nil
}

func (t *WebhookTrigger) handleWebhook(w http.ResponseWriter, r *http.Request) {
	// 检查方法
	if t.Method != "" && r.Method != t.Method {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 检查必要的请求头
	for key, expected := range t.Headers {
		actual := r.Header.Get(key)
		if actual != expected {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	}

	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// 验证签名（如果配置了 secret）
	if t.Secret != "" {
		signature := r.Header.Get("X-Hub-Signature-256")
		if signature == "" {
			signature = r.Header.Get("X-Signature")
		}
		if signature != "" {
			if !t.verifySignature(body, signature) {
				http.Error(w, "Invalid signature", http.StatusUnauthorized)
				return
			}
		}
	}

	// 解析请求体
	var payload map[string]interface{}
	contentType := r.Header.Get("Content-Type")

	if strings.Contains(contentType, "application/json") {
		if err := json.Unmarshal(body, &payload); err != nil {
			// 如果不是有效 JSON，将原始内容作为字符串
			payload = map[string]interface{}{
				"raw_body": string(body),
			}
		}
	} else if strings.Contains(contentType, "application/x-www-form-urlencoded") {
		payload = make(map[string]interface{})
		if err := r.ParseForm(); err == nil {
			for key, values := range r.Form {
				if len(values) == 1 {
					payload[key] = values[0]
				} else {
					payload[key] = values
				}
			}
		}
	} else {
		// 尝试解析为 JSON，否则存储原始内容
		if err := json.Unmarshal(body, &payload); err != nil {
			payload = map[string]interface{}{
				"raw_body": string(body),
			}
		}
	}

	// 收集请求头信息
	headers := make(map[string]string)
	for key, values := range r.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	if t.logger != nil {
		t.logger.Info("Webhook 收到请求", zap.String("path", t.Path), zap.Any("payload", payload))
	}

	// 触发回调
	go t.callback(map[string]interface{}{
		"trigger_type": "webhook",
		"path":         t.Path,
		"method":       r.Method,
		"headers":      headers,
		"query":        r.URL.Query(),
		"payload":      payload,
		"remote_addr":  r.RemoteAddr,
		"timestamp":    time.Now(),
	})

	// 返回成功响应
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// verifySignature 验证 webhook 签名
func (t *WebhookTrigger) verifySignature(body []byte, signature string) bool {
	// 移除 "sha256=" 前缀
	sig := strings.TrimPrefix(signature, "sha256=")

	mac := hmac.New(sha256.New, []byte(t.Secret))
	mac.Write(body)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(sig), []byte(expectedMAC))
}

func (t *WebhookTrigger) Stop() error {
	if t.cancel != nil {
		t.cancel()
	}
	if t.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return t.server.Shutdown(ctx)
	}
	return nil
}

// WebhookRegistry Webhook 注册表（用于集中管理 webhook 端点）
type WebhookRegistry struct {
	mu       sync.RWMutex
	handlers map[string]func(map[string]interface{})
	logger   *zap.Logger
}

var (
	globalWebhookRegistry *WebhookRegistry
	registryOnce          sync.Once
)

// GetWebhookRegistry 获取全局 webhook 注册表
func GetWebhookRegistry(logger *zap.Logger) *WebhookRegistry {
	registryOnce.Do(func() {
		globalWebhookRegistry = &WebhookRegistry{
			handlers: make(map[string]func(map[string]interface{})),
			logger:   logger,
		}
	})
	return globalWebhookRegistry
}

// Register 注册 webhook 路径
func (r *WebhookRegistry) Register(path string, handler func(map[string]interface{})) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[path] = handler
	if r.logger != nil {
		r.logger.Info("Webhook 路径已注册", zap.String("path", path))
	}
}

// Unregister 注销 webhook 路径
func (r *WebhookRegistry) Unregister(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.handlers, path)
	if r.logger != nil {
		r.logger.Info("Webhook 路径已注销", zap.String("path", path))
	}
}

// Handle 处理 webhook 请求
func (r *WebhookRegistry) Handle(path string, data map[string]interface{}) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handler, exists := r.handlers[path]
	if !exists {
		return false
	}

	go handler(data)
	return true
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
