// screen.go - 屏幕共享引擎
package remoteassist

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ScreenEngine 屏幕共享引擎.
type ScreenEngine struct {
	sessions map[string]*ScreenShare
	frames   map[string][]*ScreenFrame
	mu       sync.RWMutex
}

// NewScreenEngine 创建屏幕共享引擎.
func NewScreenEngine() *ScreenEngine {
	return &ScreenEngine{
		sessions: make(map[string]*ScreenShare),
		frames:   make(map[string][]*ScreenFrame),
	}
}

// StartSharing 开始屏幕共享.
func (e *ScreenEngine) StartSharing(sessionID string, options *ScreenShareOptions) (*ScreenShare, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 检查是否已有共享
	if _, exists := e.sessions[sessionID]; exists {
		return nil, fmt.Errorf("会话已有屏幕共享: %s", sessionID)
	}

	share := &ScreenShare{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Width:     options.Width,
		Height:    options.Height,
		FPS:       options.FPS,
		Quality:   options.Quality,
		Bitrate:   options.Bitrate,
		Codec:     options.Codec,
		Status:    "active",
		StartedAt: time.Now(),
		Cursor: &CursorPosition{
			X: 0,
			Y: 0,
		},
	}

	e.sessions[sessionID] = share
	e.frames[sessionID] = make([]*ScreenFrame, 0)

	log.Printf("🖥️ 开始屏幕共享: %s, 分辨率: %dx%d, 帧率: %d",
		share.ID, share.Width, share.Height, share.FPS)

	return share, nil
}

// ScreenShareOptions 屏幕共享选项.
type ScreenShareOptions struct {
	Width   int    `json:"width"`   // 宽度
	Height  int    `json:"height"`  // 高度
	FPS     int    `json:"fps"`     // 帧率
	Quality int    `json:"quality"` // 质量
	Bitrate int    `json:"bitrate"` // 码率
	Codec   string `json:"codec"`   // 编码格式
}

// StopSharing 停止屏幕共享.
func (e *ScreenEngine) StopSharing(sessionID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	share, exists := e.sessions[sessionID]
	if !exists {
		return fmt.Errorf("屏幕共享不存在: %s", sessionID)
	}

	share.Status = "stopped"
	delete(e.sessions, sessionID)
	delete(e.frames, sessionID)

	log.Printf("🖥️ 停止屏幕共享: %s", share.ID)
	return nil
}

// GetSharing 获取屏幕共享.
func (e *ScreenEngine) GetSharing(sessionID string) (*ScreenShare, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	share, exists := e.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("屏幕共享不存在: %s", sessionID)
	}
	return share, nil
}

// SendFrame 发送屏幕帧.
func (e *ScreenEngine) SendFrame(sessionID string, frame *ScreenFrame) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	share, exists := e.sessions[sessionID]
	if !exists {
		return fmt.Errorf("屏幕共享不存在: %s", sessionID)
	}

	if share.Status != "active" {
		return fmt.Errorf("屏幕共享未激活: %s", share.Status)
	}

	// 存储帧（保留最近100帧）
	frames := e.frames[sessionID]
	if len(frames) >= 100 {
		frames = frames[1:]
	}
	frames = append(frames, frame)
	e.frames[sessionID] = frames

	return nil
}

// GetFrame 获取最新帧.
func (e *ScreenEngine) GetFrame(sessionID string) (*ScreenFrame, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	frames, exists := e.frames[sessionID]
	if !exists || len(frames) == 0 {
		return nil, fmt.Errorf("无可用帧: %s", sessionID)
	}

	return frames[len(frames)-1], nil
}

// GetFrames 获取多帧.
func (e *ScreenEngine) GetFrames(sessionID string, count int) ([]*ScreenFrame, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	frames, exists := e.frames[sessionID]
	if !exists {
		return nil, fmt.Errorf("无可用帧: %s", sessionID)
	}

	if count <= 0 || count > len(frames) {
		count = len(frames)
	}

	return frames[len(frames)-count:], nil
}

// UpdateCursor 更新光标位置.
func (e *ScreenEngine) UpdateCursor(sessionID string, pos *CursorPosition) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	share, exists := e.sessions[sessionID]
	if !exists {
		return fmt.Errorf("屏幕共享不存在: %s", sessionID)
	}

	share.Cursor = pos
	return nil
}

// UpdateQuality 更新共享质量.
func (e *ScreenEngine) UpdateQuality(sessionID string, quality int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	share, exists := e.sessions[sessionID]
	if !exists {
		return fmt.Errorf("屏幕共享不存在: %s", sessionID)
	}

	if quality < 1 || quality > 100 {
		return fmt.Errorf("质量参数无效: %d", quality)
	}

	share.Quality = quality
	log.Printf("🖥️ 更新共享质量: %s -> %d", sessionID, quality)
	return nil
}

// UpdateResolution 更新分辨率.
func (e *ScreenEngine) UpdateResolution(sessionID string, width, height int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	share, exists := e.sessions[sessionID]
	if !exists {
		return fmt.Errorf("屏幕共享不存在: %s", sessionID)
	}

	share.Width = width
	share.Height = height
	log.Printf("🖥️ 更新分辨率: %s -> %dx%d", sessionID, width, height)
	return nil
}

// GetStats 获取共享统计.
func (e *ScreenEngine) GetStats(sessionID string) (map[string]interface{}, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	share, exists := e.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("屏幕共享不存在: %s", sessionID)
	}

	frames := e.frames[sessionID]
	totalSize := 0
	for _, frame := range frames {
		totalSize += frame.Size
	}

	stats := map[string]interface{}{
		"session_id":  sessionID,
		"share_id":    share.ID,
		"status":      share.Status,
		"resolution":  fmt.Sprintf("%dx%d", share.Width, share.Height),
		"fps":         share.FPS,
		"quality":     share.Quality,
		"frame_count": len(frames),
		"total_size":  totalSize,
		"duration":    time.Since(share.StartedAt).Seconds(),
	}

	return stats, nil
}

// ListSharings 列出所有共享.
func (e *ScreenEngine) ListSharings() []*ScreenShare {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*ScreenShare, 0, len(e.sessions))
	for _, share := range e.sessions {
		result = append(result, share)
	}
	return result
}

// HandleMouseEvent 处理鼠标事件.
func (e *ScreenEngine) HandleMouseEvent(sessionID string, event *MouseEvent) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	share, exists := e.sessions[sessionID]
	if !exists {
		return fmt.Errorf("屏幕共享不存在: %s", sessionID)
	}

	// 更新光标位置
	share.Cursor = &CursorPosition{
		X:     event.X,
		Y:     event.Y,
		Type:  event.CursorType,
		Click: event.Button != "",
	}

	return nil
}

// MouseEvent 鼠标事件.
type MouseEvent struct {
	X          int    `json:"x"`           // X坐标
	Y          int    `json:"y"`           // Y坐标
	Button     string `json:"button"`      // 按钮(left/right/middle)
	Action     string `json:"action"`      // 动作(click/dblclick/move)
	CursorType string `json:"cursor_type"` // 光标类型
	WheelDelta int    `json:"wheel_delta"` // 滚轮增量
}

// HandleKeyboardEvent 处理键盘事件.
func (e *ScreenEngine) HandleKeyboardEvent(sessionID string, event *KeyboardEvent) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	_, exists := e.sessions[sessionID]
	if !exists {
		return fmt.Errorf("屏幕共享不存在: %s", sessionID)
	}

	// 键盘事件处理
	log.Printf("⌨️ 键盘事件: %s, 按键: %s, 动作: %s",
		sessionID, event.Key, event.Action)

	return nil
}

// KeyboardEvent 键盘事件.
type KeyboardEvent struct {
	Key      string `json:"key"`      // 按键
	Code     string `json:"code"`     // 按键代码
	Action   string `json:"action"`   // 动作(up/down/press)
	Shift    bool   `json:"shift"`    // Shift键
	Control  bool   `json:"control"`  // Ctrl键
	Alt      bool   `json:"alt"`      // Alt键
	Meta     bool   `json:"meta"`     // Meta键
}
