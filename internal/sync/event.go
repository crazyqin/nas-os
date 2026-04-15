package sync

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// EventType 事件类型.
type EventType string

const (
	EventMigrationStart    EventType = "migration_start"
	EventMigrationComplete EventType = "migration_complete"
	EventMigrationFailed   EventType = "migration_failed"
	EventProgress          EventType = "progress"
	EventFileSynced        EventType = "file_synced"
	EventConflictDetected  EventType = "conflict_detected"
	EventConflictResolved  EventType = "conflict_resolved"
	EventScanStart         EventType = "scan_start"
	EventScanComplete      EventType = "scan_complete"
	EventWatchStarted      EventType = "watch_started"
	EventWatchStopped      EventType = "watch_stopped"
	EventError             EventType = "error"
)

// Event 同步事件.
type Event struct {
	Type      EventType   `json:"type"`
	TaskID    string      `json:"taskID"`
	Timestamp time.Time   `json:"timestamp"`
	Progress  *Progress   `json:"progress,omitempty"`
	Conflict  *Conflict   `json:"conflict,omitempty"`
	Error     string      `json:"error,omitempty"`
	Message   string      `json:"message,omitempty"`
	Extra     interface{} `json:"extra,omitempty"`
}

// EventHandler 事件处理器接口.
type EventHandler interface {
	HandleEvent(ctx context.Context, event *Event) error
}

// EventHandlerFunc 函数式事件处理器.
type EventHandlerFunc func(ctx context.Context, event *Event) error

func (f EventHandlerFunc) HandleEvent(ctx context.Context, event *Event) error {
	return f(ctx, event)
}

// Notifier 事件通知器.
type Notifier struct {
	mu       sync.RWMutex
	handlers []EventHandler
	queue    chan *Event
	stop     chan struct{}
	wg       sync.WaitGroup
}

// NewNotifier 创建通知器.
func NewNotifier() *Notifier {
	return &Notifier{
		handlers: make([]EventHandler, 0),
		queue:    make(chan *Event, 500),
		stop:     make(chan struct{}),
	}
}

// RegisterHandler 注册事件处理器.
func (n *Notifier) RegisterHandler(handler EventHandler) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.handlers = append(n.handlers, handler)
}

// RegisterFunc 注册函数式处理器.
func (n *Notifier) RegisterFunc(fn func(ctx context.Context, event *Event) error) {
	n.RegisterHandler(EventHandlerFunc(fn))
}

// Start 启动通知器.
func (n *Notifier) Start(ctx context.Context) {
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case <-n.stop:
				return
			case event, ok := <-n.queue:
				if !ok {
					return
				}
				n.dispatch(ctx, event)
			}
		}
	}()
}

// Stop 停止通知器.
func (n *Notifier) Stop() {
	close(n.stop)
	n.wg.Wait()
}

// Emit 发送事件.
func (n *Notifier) Emit(event *Event) {
	// 非阻塞发送
	select {
	case n.queue <- event:
	default:
		// 队列满，跳过（防止阻塞主同步流程）
	}
}

// EmitMigrationStart 发送迁移开始事件.
func (n *Notifier) EmitMigrationStart(taskID string, direction SyncDirection) {
	n.Emit(&Event{
		Type:      EventMigrationStart,
		TaskID:    taskID,
		Timestamp: time.Now(),
		Message:   fmt.Sprintf("开始 %s 同步", direction),
	})
}

// EmitMigrationComplete 发送迁移完成事件.
func (n *Notifier) EmitMigrationComplete(taskID string, p *Progress) {
	n.Emit(&Event{
		Type:      EventMigrationComplete,
		TaskID:    taskID,
		Timestamp: time.Now(),
		Progress:  p,
		Message:   "同步完成",
	})
}

// EmitMigrationFailed 发送迁移失败事件.
func (n *Notifier) EmitMigrationFailed(taskID string, err error, p *Progress) {
	e := &Event{
		Type:      EventMigrationFailed,
		TaskID:    taskID,
		Timestamp: time.Now(),
		Progress:  p,
		Message:   "同步失败",
	}
	if err != nil {
		e.Error = err.Error()
	}
	n.Emit(e)
}

// EmitProgress 发送进度事件.
func (n *Notifier) EmitProgress(taskID string, p *Progress) {
	n.Emit(&Event{
		Type:      EventProgress,
		TaskID:    taskID,
		Timestamp: time.Now(),
		Progress:  p,
	})
}

// EmitConflict 发送冲突检测事件.
func (n *Notifier) EmitConflict(conflict *Conflict) {
	n.Emit(&Event{
		Type:     EventConflictDetected,
		TaskID:   conflict.TaskID,
		Timestamp: time.Now(),
		Conflict: conflict,
		Message:  fmt.Sprintf("检测到冲突: %s", conflict.RelPath),
	})
}

// EmitConflictResolved 发送冲突解决事件.
func (n *Notifier) EmitConflictResolved(conflict *Conflict, action SyncOpType) {
	n.Emit(&Event{
		Type:      EventConflictResolved,
		TaskID:    conflict.TaskID,
		Timestamp: time.Now(),
		Conflict:  conflict,
		Message:   fmt.Sprintf("冲突已解决 [%s]: %s", action, conflict.RelPath),
	})
}

// EmitError 发送错误事件.
func (n *Notifier) EmitError(taskID string, err error, msg string) {
	e := &Event{
		Type:      EventError,
		TaskID:    taskID,
		Timestamp: time.Now(),
		Message:   msg,
	}
	if err != nil {
		e.Error = err.Error()
	}
	n.Emit(e)
}

func (n *Notifier) dispatch(ctx context.Context, event *Event) {
	n.mu.RLock()
	handlers := make([]EventHandler, len(n.handlers))
	copy(handlers, n.handlers)
	n.mu.RUnlock()

	for _, h := range handlers {
		if err := h.HandleEvent(ctx, event); err != nil {
			// 单个 handler 出错不影响其他 handler
			_ = err
		}
	}
}

// BufferedEventHandler 缓冲事件处理器（批量发送）.
type BufferedEventHandler struct {
	inner      EventHandler
	buffer     []*Event
	bufferSize int
	flushInt   time.Duration
	stop       chan struct{}
}

// NewBufferedEventHandler 创建缓冲处理器.
func NewBufferedEventHandler(inner EventHandler, bufferSize int, flushInt time.Duration) *BufferedEventHandler {
	return &BufferedEventHandler{
		inner:      inner,
		buffer:     make([]*Event, 0, bufferSize),
		bufferSize: bufferSize,
		flushInt:  flushInt,
		stop:      make(chan struct{}),
	}
}

func (b *BufferedEventHandler) HandleEvent(ctx context.Context, event *Event) error {
	b.buffer = append(b.buffer, event)
	if len(b.buffer) >= b.bufferSize {
		return b.flush(ctx)
	}
	return nil
}

func (b *BufferedEventHandler) flush(ctx context.Context) error {
	for _, e := range b.buffer {
		if err := b.inner.HandleEvent(ctx, e); err != nil {
			return err
		}
	}
	b.buffer = b.buffer[:0]
	return nil
}

func (b *BufferedEventHandler) Stop() {
	close(b.stop)
	_ = b.flush(context.Background())
}
