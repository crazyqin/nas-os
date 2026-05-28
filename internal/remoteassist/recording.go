// recording.go - 会话录制和回放
package remoteassist

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Recorder 录制器.
type Recorder struct {
	config     *RecordingConfig
	recordings map[string]*Recording
	events     map[string][]*RecordingEvent
	mu         sync.RWMutex
}

// NewRecorder 创建录制器.
func NewRecorder(cfg *RecordingConfig) *Recorder {
	if cfg == nil {
		cfg = &RecordingConfig{
			Enabled:       true,
			AutoRecord:    false,
			Format:        "webm",
			Resolution:    "1080p",
			MaxSize:       1024 * 1024 * 1024,
			RetentionDays: 30,
			StoragePath:   "/var/nas-os/remoteassist/recordings",
		}
	}

	r := &Recorder{
		config:     cfg,
		recordings: make(map[string]*Recording),
		events:     make(map[string][]*RecordingEvent),
	}

	// 创建存储目录
	if err := os.MkdirAll(cfg.StoragePath, 0755); err != nil {
		log.Printf("⚠️ 创建录制目录失败: %v", err)
	}

	return r
}

// StartRecording 开始录制.
func (r *Recorder) StartRecording(sessionID string, userID string) (*Recording, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.config.Enabled {
		return nil, fmt.Errorf("录制功能未启用")
	}

	// 检查是否已在录制
	if _, exists := r.recordings[sessionID]; exists {
		return nil, fmt.Errorf("会话已在录制: %s", sessionID)
	}

	recording := &Recording{
		ID:         uuid.New().String(),
		SessionID:  sessionID,
		FileName:   fmt.Sprintf("%s_%s.%s", sessionID, time.Now().Format("20060102_150405"), r.config.Format),
		Format:     r.config.Format,
		Resolution: r.config.Resolution,
		Status:     "recording",
		StartedAt:  time.Now(),
		CreatedBy:  userID,
		Tags:       make([]string, 0),
	}

	recording.FilePath = filepath.Join(r.config.StoragePath, recording.FileName)

	r.recordings[sessionID] = recording
	r.events[recording.ID] = make([]*RecordingEvent, 0)

	log.Printf("🔴 开始录制: %s, 会话: %s", recording.ID, sessionID)
	return recording, nil
}

// StopRecording 停止录制.
func (r *Recorder) StopRecording(sessionID string) (*Recording, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	recording, exists := r.recordings[sessionID]
	if !exists {
		return nil, fmt.Errorf("录制不存在: %s", sessionID)
	}

	now := time.Now()
	recording.Status = "completed"
	recording.EndedAt = &now
	recording.Duration = int64(now.Sub(recording.StartedAt).Seconds())

	// 保存录制文件
	if err := r.saveRecording(recording); err != nil {
		log.Printf("⚠️ 保存录制失败: %v", err)
	}

	delete(r.recordings, sessionID)

	log.Printf("⏹️ 停止录制: %s, 时长: %d秒", recording.ID, recording.Duration)
	return recording, nil
}

// saveRecording 保存录制.
func (r *Recorder) saveRecording(recording *Recording) error {
	// 创建录制文件（简化实现）
	file, err := os.Create(recording.FilePath)
	if err != nil {
		return fmt.Errorf("创建录制文件失败: %w", err)
	}
	defer file.Close()

	// 写入录制数据
	data := map[string]interface{}{
		"id":         recording.ID,
		"session_id": recording.SessionID,
		"started_at": recording.StartedAt,
		"ended_at":   recording.EndedAt,
		"duration":   recording.Duration,
		"events":     r.events[recording.ID],
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("写入录制数据失败: %w", err)
	}

	// 获取文件大小
	info, err := file.Stat()
	if err == nil {
		recording.FileSize = info.Size()
	}

	return nil
}

// RecordEvent 录制事件.
func (r *Recorder) RecordEvent(sessionID string, eventType string, data []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	recording, exists := r.recordings[sessionID]
	if !exists {
		return fmt.Errorf("录制不存在: %s", sessionID)
	}

	event := &RecordingEvent{
		ID:          uuid.New().String(),
		RecordingID: recording.ID,
		Type:        eventType,
		Data:        data,
		Timestamp:   time.Now().UnixMilli(),
		Sequence:    int64(len(r.events[recording.ID]) + 1),
	}

	r.events[recording.ID] = append(r.events[recording.ID], event)

	return nil
}

// RecordScreenFrame 录制屏幕帧.
func (r *Recorder) RecordScreenFrame(sessionID string, frame *ScreenFrame) error {
	data, err := json.Marshal(frame)
	if err != nil {
		return fmt.Errorf("序列化帧数据失败: %w", err)
	}

	return r.RecordEvent(sessionID, "screen_frame", data)
}

// RecordMouseEvent 录制鼠标事件.
func (r *Recorder) RecordMouseEvent(sessionID string, event *MouseEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("序列化鼠标事件失败: %w", err)
	}

	return r.RecordEvent(sessionID, "mouse_event", data)
}

// RecordKeyboardEvent 录制键盘事件.
func (r *Recorder) RecordKeyboardEvent(sessionID string, event *KeyboardEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("序列化键盘事件失败: %w", err)
	}

	return r.RecordEvent(sessionID, "keyboard_event", data)
}

// RecordChatMessage 录制聊天消息.
func (r *Recorder) RecordChatMessage(sessionID string, msg *ChatMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("序列化聊天消息失败: %w", err)
	}

	return r.RecordEvent(sessionID, "chat_message", data)
}

// RecordFileTransfer 录制文件传输.
func (r *Recorder) RecordFileTransfer(sessionID string, transfer *FileTransfer) error {
	data, err := json.Marshal(transfer)
	if err != nil {
		return fmt.Errorf("序列化文件传输失败: %w", err)
	}

	return r.RecordEvent(sessionID, "file_transfer", data)
}

// GetRecording 获取录制.
func (r *Recorder) GetRecording(recordingID string) (*Recording, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 查找已完成的录制文件
	files, err := os.ReadDir(r.config.StoragePath)
	if err != nil {
		return nil, fmt.Errorf("读取录制目录失败: %w", err)
	}

	for _, file := range files {
		if file.Name() == recordingID+".json" {
			return r.loadRecording(filepath.Join(r.config.StoragePath, file.Name()))
		}
	}

	// 检查进行中的录制
	for _, recording := range r.recordings {
		if recording.ID == recordingID {
			return recording, nil
		}
	}

	return nil, fmt.Errorf("录制不存在: %s", recordingID)
}

// loadRecording 加载录制.
func (r *Recorder) loadRecording(filePath string) (*Recording, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var data map[string]interface{}
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return nil, err
	}

	recording := &Recording{}
	if id, ok := data["id"].(string); ok {
		recording.ID = id
	}
	if sessionID, ok := data["session_id"].(string); ok {
		recording.SessionID = sessionID
	}
	// 解析其他字段...

	return recording, nil
}

// ListRecordings 列出录制.
func (r *Recorder) ListRecordings(sessionID string) ([]*Recording, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Recording, 0)

	// 列出录制文件
	files, err := os.ReadDir(r.config.StoragePath)
	if err != nil {
		return nil, fmt.Errorf("读取录制目录失败: %w", err)
	}

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}

		recording, err := r.loadRecording(filepath.Join(r.config.StoragePath, file.Name()))
		if err != nil {
			continue
		}

		if sessionID != "" && recording.SessionID != sessionID {
			continue
		}

		result = append(result, recording)
	}

	// 添加进行中的录制
	for _, recording := range r.recordings {
		if sessionID != "" && recording.SessionID != sessionID {
			continue
		}
		result = append(result, recording)
	}

	return result, nil
}

// DeleteRecording 删除录制.
func (r *Recorder) DeleteRecording(recordingID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 删除录制文件
	filePath := filepath.Join(r.config.StoragePath, recordingID+".json")
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除录制文件失败: %w", err)
	}

	// 删除事件数据
	delete(r.events, recordingID)

	log.Printf("🗑️ 删除录制: %s", recordingID)
	return nil
}

// GetRecordingEvents 获取录制事件.
func (r *Recorder) GetRecordingEvents(recordingID string, startTime, endTime int64) ([]*RecordingEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	events, exists := r.events[recordingID]
	if !exists {
		return nil, fmt.Errorf("录制事件不存在: %s", recordingID)
	}

	result := make([]*RecordingEvent, 0)
	for _, event := range events {
		if startTime > 0 && event.Timestamp < startTime {
			continue
		}
		if endTime > 0 && event.Timestamp > endTime {
			continue
		}
		result = append(result, event)
	}

	return result, nil
}

// CleanupRecordings 清理过期录制.
func (r *Recorder) CleanupRecordings() (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -r.config.RetentionDays)
	cleaned := 0

	files, err := os.ReadDir(r.config.StoragePath)
	if err != nil {
		return 0, fmt.Errorf("读取录制目录失败: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			filePath := filepath.Join(r.config.StoragePath, file.Name())
			if err := os.Remove(filePath); err == nil {
				cleaned++
			}
		}
	}

	log.Printf("🧹 清理过期录制: %d 个", cleaned)
	return cleaned, nil
}

// GetRecordingStats 获取录制统计.
func (r *Recorder) GetRecordingStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := map[string]interface{}{
		"active_recordings": len(r.recordings),
		"total_events":      0,
	}

	totalEvents := 0
	for _, events := range r.events {
		totalEvents += len(events)
	}
	stats["total_events"] = totalEvents

	return stats
}
