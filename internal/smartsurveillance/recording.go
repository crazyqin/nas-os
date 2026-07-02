// Package smartsurveillance 提供智能监控中心功能
// recording.go - 录像管理，支持连续录像、事件录像、时间线回放
package smartsurveillance

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// RecordingManager 录像管理器.
type RecordingManager struct {
	mu            sync.RWMutex
	logger        *zap.Logger
	engine        *SurveillanceEngine
	storagePath   string
	retentionDays int
}

// NewRecordingManager 创建录像管理器.
func NewRecordingManager(logger *zap.Logger, engine *SurveillanceEngine, storagePath string) *RecordingManager {
	return &RecordingManager{
		logger:        logger,
		engine:        engine,
		storagePath:   storagePath,
		retentionDays: 30, // 默认保留30天
	}
}

// SetRetention 设置录像保留天数.
func (rm *RecordingManager) SetRetention(days int) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.retentionDays = days
}

// GetRetention 获取录像保留天数.
func (rm *RecordingManager) GetRetention() int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.retentionDays
}

// StartContinuousRecording 开始连续录像.
func (rm *RecordingManager) StartContinuousRecording(cameraID string) error {
	return rm.engine.StartRecording(cameraID)
}

// StopContinuousRecording 停止连续录像.
func (rm *RecordingManager) StopContinuousRecording(cameraID string) error {
	return rm.engine.StopRecording(cameraID)
}

// GetRecordingTimeline 获取录像时间线.
func (rm *RecordingManager) GetRecordingTimeline(cameraID string, date time.Time) (*TimelineData, error) {
	return rm.engine.GetTimeline(cameraID, date)
}

// SearchRecordings 搜索录像.
func (rm *RecordingManager) SearchRecordings(query RecordingQuery) []*Recording {
	return rm.engine.GetRecordings(query)
}

// GetRecordingsByTimeRange 按时间范围获取录像.
func (rm *RecordingManager) GetRecordingsByTimeRange(cameraID string, start, end time.Time) []*Recording {
	query := RecordingQuery{
		CameraID:  cameraID,
		StartTime: &start,
		EndTime:   &end,
		Page:      1,
		PageSize:  1000,
	}
	return rm.engine.GetRecordings(query)
}

// GetRecordingStats 获取录像统计.
func (rm *RecordingManager) GetRecordingStats(cameraID string) map[string]interface{} {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	query := RecordingQuery{
		CameraID: cameraID,
		Page:     1,
		PageSize: 10000,
	}
	recordings := rm.engine.GetRecordings(query)

	totalDuration := 0
	totalSize := int64(0)
	eventCount := 0

	for _, rec := range recordings {
		totalDuration += rec.Duration
		totalSize += rec.FileSize
		if rec.HasEvents {
			eventCount++
		}
	}

	return map[string]interface{}{
		"total_recordings": len(recordings),
		"total_duration":   totalDuration,
		"total_size_bytes": totalSize,
		"event_recordings": eventCount,
	}
}

// CleanupExpiredRecordings 清理过期录像.
func (rm *RecordingManager) CleanupExpiredRecordings() int {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -rm.retentionDays)
	cleaned := 0

	// 遍历所有摄像头的录像
	for cameraID, recordings := range rm.engine.recordings {
		var valid []*Recording
		for _, rec := range recordings {
			if rec.StartTime.Before(cutoff) {
				cleaned++
				rm.logger.Info("清理过期录像",
					zap.String("camera", cameraID),
					zap.String("recording", rec.ID))
			} else {
				valid = append(valid, rec)
			}
		}
		rm.engine.recordings[cameraID] = valid
	}

	return cleaned
}

// ExportRecording 导出录像.
func (rm *RecordingManager) ExportRecording(recordingID string, format string) (string, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	// 查找录像
	for _, recordings := range rm.engine.recordings {
		for _, rec := range recordings {
			if rec.ID == recordingID {
				exportPath := fmt.Sprintf("%s/export/%s.%s",
					rm.storagePath, recordingID, format)
				rm.logger.Info("录像已导出",
					zap.String("recording", recordingID),
					zap.String("path", exportPath))
				return exportPath, nil
			}
		}
	}

	return "", ErrRecordingNotFound
}

// GetEventRecordings 获取包含事件的录像.
func (rm *RecordingManager) GetEventRecordings(cameraID string, start, end time.Time) []*Recording {
	query := RecordingQuery{
		CameraID:  cameraID,
		StartTime: &start,
		EndTime:   &end,
		HasEvents: boolPtr(true),
		Page:      1,
		PageSize:  1000,
	}
	return rm.engine.GetRecordings(query)
}

// GetRecordingPlaybackURL 获取回放URL.
func (rm *RecordingManager) GetRecordingPlaybackURL(recordingID string, timestamp time.Time) (string, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	for _, recordings := range rm.engine.recordings {
		for _, rec := range recordings {
			if rec.ID == recordingID {
				url := fmt.Sprintf("%s/playback/%s?time=%s",
					rm.storagePath, recordingID, timestamp.Format(time.RFC3339))
				return url, nil
			}
		}
	}

	return "", ErrRecordingNotFound
}

// GetTimeRangePlayback 获取时间段回放.
func (rm *RecordingManager) GetTimeRangePlayback(cameraID string, start, end time.Time) ([]PlaybackSegment, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if _, err := rm.engine.GetCamera(cameraID); err != nil {
		return nil, err
	}

	recordings := rm.engine.GetRecordings(RecordingQuery{
		CameraID:  cameraID,
		StartTime: &start,
		EndTime:   &end,
		Page:      1,
		PageSize:  1000,
	})

	var segments []PlaybackSegment
	for _, rec := range recordings {
		segments = append(segments, PlaybackSegment{
			RecordingID: rec.ID,
			StartTime:   rec.StartTime,
			EndTime:     rec.EndTime,
			Duration:    rec.Duration,
			HasEvents:   rec.HasEvents,
			FilePath:    rec.FilePath,
		})
	}

	sort.Slice(segments, func(i, j int) bool {
		return segments[i].StartTime.Before(segments[j].StartTime)
	})

	return segments, nil
}

// PlaybackSegment 回放片段.
type PlaybackSegment struct {
	RecordingID string    `json:"recording_id"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	Duration    int       `json:"duration_sec"`
	HasEvents   bool      `json:"has_events"`
	FilePath    string    `json:"file_path"`
}

// boolPtr 返回bool指针.
func boolPtr(b bool) *bool {
	return &b
}
