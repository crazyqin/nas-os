// Package workflow_automation 提供工作流自动化引擎
package workflow_automation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// TriggerManager 触发器管理器.
type TriggerManager struct {
	mu           sync.RWMutex
	triggers     map[string]*Trigger
	cronEntries  map[string]cron.EntryID
	engine       *Engine
	logger       *zap.Logger
	cron         *cron.Cron
	stopChan     chan struct{}
	running      bool
	eventCh      chan *TriggerEvent
	webhookCh    chan *WebhookRequest
	fileWatchers map[string]*FileWatcher
}

// WebhookRequest Webhook 请求.
type WebhookRequest struct {
	TriggerID string                 `json:"trigger_id"`
	Method    string                 `json:"method"`
	Path      string                 `json:"path"`
	Headers   map[string]string      `json:"headers,omitempty"`
	Body      map[string]interface{} `json:"body,omitempty"`
	Response  chan *WebhookResponse  `json:"-"`
}

// WebhookResponse Webhook 响应.
type WebhookResponse struct {
	StatusCode int         `json:"status_code"`
	Body       interface{} `json:"body,omitempty"`
}

// FileWatcher 文件监控器.
type FileWatcher struct {
	TriggerID string   `json:"trigger_id"`
	Paths     []string `json:"paths"`
	Patterns  []string `json:"patterns,omitempty"`
}

// NewTriggerManager 创建触发器管理器.
func NewTriggerManager(engine *Engine, logger *zap.Logger) *TriggerManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &TriggerManager{
		triggers:     make(map[string]*Trigger),
		cronEntries:  make(map[string]cron.EntryID),
		engine:       engine,
		logger:       logger,
		cron:         cron.New(cron.WithSeconds()),
		stopChan:     make(chan struct{}),
		eventCh:      make(chan *TriggerEvent, 100),
		webhookCh:    make(chan *WebhookRequest, 100),
		fileWatchers: make(map[string]*FileWatcher),
	}
}

// Start 启动触发器管理器.
func (tm *TriggerManager) Start() {
	tm.mu.Lock()
	if tm.running {
		tm.mu.Unlock()
		return
	}
	tm.running = true
	tm.stopChan = make(chan struct{})
	tm.mu.Unlock()

	tm.cron.Start()
	go tm.processEvents()
	go tm.processWebhooks()

	tm.logger.Info("trigger manager started")
}

// Stop 停止触发器管理器.
func (tm *TriggerManager) Stop() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if !tm.running {
		return
	}
	tm.running = false
	close(tm.stopChan)
	tm.cron.Stop()
	tm.logger.Info("trigger manager stopped")
}

// ========== 触发器 CRUD ==========

// CreateTrigger 创建触发器.
func (tm *TriggerManager) CreateTrigger(t *Trigger) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	t.CreatedAt = time.Now()

	if err := tm.validateTrigger(t); err != nil {
		return fmt.Errorf("invalid trigger: %w", err)
	}

	tm.triggers[t.ID] = t

	if t.Enabled {
		if err := tm.activateTrigger(t); err != nil {
			delete(tm.triggers, t.ID)
			return fmt.Errorf("activate trigger: %w", err)
		}
	}

	tm.logger.Info("trigger created",
		zap.String("id", t.ID),
		zap.String("type", string(t.Type)),
		zap.String("workflow_id", t.WorkflowID),
	)
	return nil
}

// GetTrigger 获取触发器.
func (tm *TriggerManager) GetTrigger(id string) (*Trigger, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	t, ok := tm.triggers[id]
	if !ok {
		return nil, ErrTriggerNotFound
	}
	return t, nil
}

// ListTriggers 列出触发器.
func (tm *TriggerManager) ListTriggers(workflowID string) []*Trigger {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	triggers := make([]*Trigger, 0)
	for _, t := range tm.triggers {
		if workflowID == "" || t.WorkflowID == workflowID {
			triggers = append(triggers, t)
		}
	}
	return triggers
}

// UpdateTrigger 更新触发器.
func (tm *TriggerManager) UpdateTrigger(t *Trigger) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	existing, ok := tm.triggers[t.ID]
	if !ok {
		return ErrTriggerNotFound
	}

	// 先停用旧触发器
	tm.deactivateTrigger(existing)

	if err := tm.validateTrigger(t); err != nil {
		// 重新激活旧的
		tm.activateTrigger(existing)
		return fmt.Errorf("invalid trigger: %w", err)
	}

	tm.triggers[t.ID] = t

	if t.Enabled {
		if err := tm.activateTrigger(t); err != nil {
			tm.logger.Error("failed to activate updated trigger", zap.Error(err))
		}
	}

	return nil
}

// DeleteTrigger 删除触发器.
func (tm *TriggerManager) DeleteTrigger(id string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	t, ok := tm.triggers[id]
	if !ok {
		return ErrTriggerNotFound
	}

	tm.deactivateTrigger(t)
	delete(tm.triggers, id)

	if tm.engine.store != nil {
		_ = tm.engine.store.DeleteTrigger(id)
	}

	tm.logger.Info("trigger deleted", zap.String("id", id))
	return nil
}

// EnableTrigger 启用触发器.
func (tm *TriggerManager) EnableTrigger(id string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	t, ok := tm.triggers[id]
	if !ok {
		return ErrTriggerNotFound
	}

	t.Enabled = true
	return tm.activateTrigger(t)
}

// DisableTrigger 停用触发器.
func (tm *TriggerManager) DisableTrigger(id string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	t, ok := tm.triggers[id]
	if !ok {
		return ErrTriggerNotFound
	}

	t.Enabled = false
	tm.deactivateTrigger(t)
	return nil
}

// DisableTriggersByWorkflow 停用指定工作流的所有触发器.
func (tm *TriggerManager) DisableTriggersByWorkflow(workflowID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for _, t := range tm.triggers {
		if t.WorkflowID == workflowID {
			t.Enabled = false
			tm.deactivateTrigger(t)
		}
	}
}

// ========== 触发器激活/停用 ==========

// activateTrigger 激活触发器.
func (tm *TriggerManager) activateTrigger(t *Trigger) error {
	switch t.Type {
	case TriggerCron:
		return tm.activateCronTrigger(t)
	case TriggerWebhook:
		// Webhook 由外部驱动，只需注册
		return nil
	case TriggerFile:
		return tm.activateFileTrigger(t)
	case TriggerOnEvent:
		// 事件触发器由外部事件驱动
		return nil
	case TriggerManual:
		// 手动触发器无需激活
		return nil
	default:
		return fmt.Errorf("unsupported trigger type: %s", t.Type)
	}
}

// activateCronTrigger 激活定时触发器.
func (tm *TriggerManager) activateCronTrigger(t *Trigger) error {
	schedule, ok := t.Config["schedule"]
	if !ok {
		return fmt.Errorf("cron trigger missing 'schedule' config")
	}

	entryID, err := tm.cron.AddFunc(schedule, func() {
		tm.fireCronTrigger(t)
	})
	if err != nil {
		return fmt.Errorf("invalid cron schedule %q: %w", schedule, err)
	}

	tm.cronEntries[t.ID] = entryID
	return nil
}

// activateFileTrigger 激活文件监控触发器.
func (tm *TriggerManager) activateFileTrigger(t *Trigger) error {
	paths, ok := t.Config["paths"]
	if !ok {
		return fmt.Errorf("file trigger missing 'paths' config")
	}

	patterns := t.Config["patterns"]

	watcher := &FileWatcher{
		TriggerID: t.ID,
		Paths:     splitPaths(paths),
		Patterns:  splitPaths(patterns),
	}
	tm.fileWatchers[t.ID] = watcher

	// 启动文件监控 goroutine
	go tm.watchFiles(t, watcher)

	return nil
}

// deactivateTrigger 停用触发器.
func (tm *TriggerManager) deactivateTrigger(t *Trigger) {
	switch t.Type {
	case TriggerCron:
		if entryID, ok := tm.cronEntries[t.ID]; ok {
			tm.cron.Remove(entryID)
			delete(tm.cronEntries, t.ID)
		}
	case TriggerFile:
		delete(tm.fileWatchers, t.ID)
	}
}

// ========== 触发事件处理 ==========

// fireCronTrigger 触发定时任务.
func (tm *TriggerManager) fireCronTrigger(t *Trigger) {
	now := time.Now()
	t.LastFiredAt = &now

	event := &TriggerEvent{
		TriggerID:  t.ID,
		WorkflowID: t.WorkflowID,
		Type:       TriggerCron,
		Payload:    make(map[string]interface{}),
		Timestamp:  now,
	}

	select {
	case tm.eventCh <- event:
	default:
		tm.logger.Warn("event channel full, dropping cron trigger event",
			zap.String("trigger_id", t.ID),
		)
	}
}

// FireEvent 触发事件（外部调用）.
func (tm *TriggerManager) FireEvent(event *TriggerEvent) {
	select {
	case tm.eventCh <- event:
	default:
		tm.logger.Warn("event channel full, dropping event",
			zap.String("trigger_id", event.TriggerID),
		)
	}
}

// FireWebhook 触发 Webhook.
func (tm *TriggerManager) FireWebhook(req *WebhookRequest) {
	select {
	case tm.webhookCh <- req:
	default:
		tm.logger.Warn("webhook channel full, dropping request",
			zap.String("trigger_id", req.TriggerID),
		)
	}
}

// processEvents 处理事件队列.
func (tm *TriggerManager) processEvents() {
	for {
		select {
		case <-tm.stopChan:
			return
		case event := <-tm.eventCh:
			tm.handleEvent(event)
		}
	}
}

// processWebhooks 处理 Webhook 队列.
func (tm *TriggerManager) processWebhooks() {
	for {
		select {
		case <-tm.stopChan:
			return
		case req := <-tm.webhookCh:
			tm.handleWebhook(req)
		}
	}
}

// handleEvent 处理触发事件.
func (tm *TriggerManager) handleEvent(event *TriggerEvent) {
	// 检查工作流是否活跃
	wf, err := tm.engine.GetWorkflow(event.WorkflowID)
	if err != nil {
		tm.logger.Error("workflow not found for trigger event",
			zap.String("workflow_id", event.WorkflowID),
			zap.Error(err),
		)
		return
	}

	if wf.Status != StatusActive {
		tm.logger.Debug("skipping event for inactive workflow",
			zap.String("workflow_id", event.WorkflowID),
			zap.String("status", string(wf.Status)),
		)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	_, err = tm.engine.Execute(ctx, event.WorkflowID, event)
	if err != nil {
		tm.logger.Error("failed to execute workflow from trigger",
			zap.String("workflow_id", event.WorkflowID),
			zap.String("trigger_id", event.TriggerID),
			zap.Error(err),
		)
	}
}

// handleWebhook 处理 Webhook 请求.
func (tm *TriggerManager) handleWebhook(req *WebhookRequest) {
	// 查找对应的触发器
	tm.mu.RLock()
	var trigger *Trigger
	for _, t := range tm.triggers {
		if t.ID == req.TriggerID && t.Type == TriggerWebhook && t.Enabled {
			trigger = t
			break
		}
	}
	tm.mu.RUnlock()

	if trigger == nil {
		if req.Response != nil {
			req.Response <- &WebhookResponse{
				StatusCode: 404,
				Body:       "trigger not found or disabled",
			}
		}
		return
	}

	now := time.Now()
	trigger.LastFiredAt = &now

	event := &TriggerEvent{
		TriggerID:  trigger.ID,
		WorkflowID: trigger.WorkflowID,
		Type:       TriggerWebhook,
		Payload:    req.Body,
		Timestamp:  now,
	}

	// 同步执行并返回结果
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	exec, err := tm.engine.Execute(ctx, trigger.WorkflowID, event)

	if req.Response != nil {
		if err != nil {
			req.Response <- &WebhookResponse{
				StatusCode: 500,
				Body:       err.Error(),
			}
		} else {
			req.Response <- &WebhookResponse{
				StatusCode: 200,
				Body:       map[string]string{"execution_id": exec.ID},
			}
		}
	}
}

// watchFiles 监控文件变化.
func (tm *TriggerManager) watchFiles(t *Trigger, watcher *FileWatcher) {
	// 简化实现：定期扫描文件变化
	// 生产环境应使用 fsnotify
	interval := 10 * time.Second
	if v, ok := t.Config["interval"]; ok {
		if d, err := time.ParseDuration(v); err == nil {
			interval = d
		}
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	lastModTimes := make(map[string]time.Time)

	for {
		select {
		case <-tm.stopChan:
			return
		case <-ticker.C:
			// 扫描文件变化（简化实现）
			for _, path := range watcher.Paths {
				tm.scanFileChanges(t, path, lastModTimes)
			}
		}
	}
}

// scanFileChanges 扫描文件变化.
func (tm *TriggerManager) scanFileChanges(t *Trigger, path string, lastModTimes map[string]time.Time) {
	// 实际实现需要读取目录内容并比较修改时间
	// 这里提供框架，具体文件系统操作依赖 os 包
}

// validateTrigger 验证触发器配置.
func (tm *TriggerManager) validateTrigger(t *Trigger) error {
	if t.WorkflowID == "" {
		return fmt.Errorf("workflow_id is required")
	}
	if t.Type == "" {
		return fmt.Errorf("trigger type is required")
	}
	if t.Config == nil {
		t.Config = make(map[string]string)
	}

	switch t.Type {
	case TriggerCron:
		if _, ok := t.Config["schedule"]; !ok {
			return fmt.Errorf("cron trigger requires 'schedule' config")
		}
	case TriggerWebhook:
		// Webhook 可选配置
	case TriggerFile:
		if _, ok := t.Config["paths"]; !ok {
			return fmt.Errorf("file trigger requires 'paths' config")
		}
	}
	return nil
}

// splitPaths 分割路径字符串.
func splitPaths(s string) []string {
	if s == "" {
		return nil
	}
	// 简单实现：按逗号分割
	result := make([]string, 0)
	current := ""
	for _, c := range s {
		if c == ',' || c == ';' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else if c != ' ' {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}
