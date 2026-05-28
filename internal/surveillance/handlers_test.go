package surveillance

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupTestHandlers(t *testing.T) (*Handlers, *Manager) {
	gin.SetMode(gin.TestMode)
	
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	assert.NoError(t, err)

	handlers := NewHandlers(manager)
	return handlers, manager
}

func setupTestRouter(handlers *Handlers) *gin.Engine {
	r := gin.New()
	api := r.Group("/api/v1")
	handlers.RegisterRoutes(api)
	return r
}

// ==================== 摄像头管理测试 ====================

func TestAddCamera(t *testing.T) {
	handlers, _ := setupTestHandlers(t)
	r := setupTestRouter(handlers)

	cam := Camera{
		Name:     "Test Camera",
		URI:      "rtsp://192.168.1.100/stream1",
		Resolution: "1920x1080",
		FPS:      30,
		Codec:    "h264",
		Enabled:  true,
	}

	body, _ := json.Marshal(cam)
	req := httptest.NewRequest("POST", "/api/v1/surveillance/cameras", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])
}

func TestListCameras(t *testing.T) {
	handlers, manager := setupTestHandlers(t)
	r := setupTestRouter(handlers)

	// 添加一个摄像头
	cam := &Camera{
		ID:       "cam-1",
		Name:     "Test Camera",
		URI:      "rtsp://192.168.1.100/stream1",
		Enabled:  true,
	}
	err := manager.AddCamera(cam)
	assert.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/surveillance/cameras", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])
}

func TestGetCamera(t *testing.T) {
	handlers, manager := setupTestHandlers(t)
	r := setupTestRouter(handlers)

	cam := &Camera{
		ID:       "cam-1",
		Name:     "Test Camera",
		URI:      "rtsp://192.168.1.100/stream1",
		Enabled:  true,
	}
	err := manager.AddCamera(cam)
	assert.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/surveillance/cameras/cam-1", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetCameraNotFound(t *testing.T) {
	handlers, _ := setupTestHandlers(t)
	r := setupTestRouter(handlers)

	req := httptest.NewRequest("GET", "/api/v1/surveillance/cameras/nonexistent", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateCamera(t *testing.T) {
	handlers, manager := setupTestHandlers(t)
	r := setupTestRouter(handlers)

	cam := &Camera{
		ID:       "cam-1",
		Name:     "Test Camera",
		URI:      "rtsp://192.168.1.100/stream1",
		Enabled:  true,
	}
	err := manager.AddCamera(cam)
	assert.NoError(t, err)

	updatedCam := Camera{
		Name:     "Updated Camera",
		URI:      "rtsp://192.168.1.100/stream2",
		Enabled:  false,
	}

	body, _ := json.Marshal(updatedCam)
	req := httptest.NewRequest("PUT", "/api/v1/surveillance/cameras/cam-1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDeleteCamera(t *testing.T) {
	handlers, manager := setupTestHandlers(t)
	r := setupTestRouter(handlers)

	cam := &Camera{
		ID:       "cam-1",
		Name:     "Test Camera",
		URI:      "rtsp://192.168.1.100/stream1",
		Enabled:  true,
	}
	err := manager.AddCamera(cam)
	assert.NoError(t, err)

	req := httptest.NewRequest("DELETE", "/api/v1/surveillance/cameras/cam-1", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDiscoverCameras(t *testing.T) {
	handlers, _ := setupTestHandlers(t)
	r := setupTestRouter(handlers)

	req := httptest.NewRequest("GET", "/api/v1/surveillance/cameras/discover", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ==================== 录制管理测试 ====================

func TestStartRecording(t *testing.T) {
	handlers, manager := setupTestHandlers(t)
	r := setupTestRouter(handlers)

	// 添加一个在线摄像头
	cam := &Camera{
		ID:       "cam-1",
		Name:     "Test Camera",
		URI:      "rtsp://192.168.1.100/stream1",
		Status:   CameraStatusOnline,
		Enabled:  true,
	}
	err := manager.AddCamera(cam)
	assert.NoError(t, err)

	reqBody := map[string]interface{}{
		"cameraId": "cam-1",
		"mode":     "continuous",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/surveillance/recordings/start", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestStartRecordingCameraOffline(t *testing.T) {
	handlers, manager := setupTestHandlers(t)
	r := setupTestRouter(handlers)

	// 添加一个离线摄像头
	cam := &Camera{
		ID:       "cam-1",
		Name:     "Test Camera",
		URI:      "rtsp://192.168.1.100/stream1",
		Status:   CameraStatusOffline,
		Enabled:  true,
	}
	err := manager.AddCamera(cam)
	assert.NoError(t, err)

	reqBody := map[string]interface{}{
		"cameraId": "cam-1",
		"mode":     "continuous",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/surveillance/recordings/start", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestStopRecording(t *testing.T) {
	handlers, manager := setupTestHandlers(t)
	r := setupTestRouter(handlers)

	// 添加一个在线摄像头并开始录制
	cam := &Camera{
		ID:       "cam-1",
		Name:     "Test Camera",
		URI:      "rtsp://192.168.1.100/stream1",
		Status:   CameraStatusOnline,
		Enabled:  true,
	}
	err := manager.AddCamera(cam)
	assert.NoError(t, err)

	job, err := manager.StartRecording("cam-1", RecordingModeContinuous)
	assert.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/v1/surveillance/recordings/"+job.ID+"/stop", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetRecordings(t *testing.T) {
	handlers, manager := setupTestHandlers(t)
	r := setupTestRouter(handlers)

	// 添加摄像头并录制
	cam := &Camera{
		ID:       "cam-1",
		Name:     "Test Camera",
		URI:      "rtsp://192.168.1.100/stream1",
		Status:   CameraStatusOnline,
		Enabled:  true,
	}
	err := manager.AddCamera(cam)
	assert.NoError(t, err)

	_, err = manager.StartRecording("cam-1", RecordingModeContinuous)
	assert.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/surveillance/recordings?cameraId=cam-1", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ==================== 事件管理测试 ====================

func TestGetEvents(t *testing.T) {
	handlers, manager := setupTestHandlers(t)
	r := setupTestRouter(handlers)

	// 添加一些事件
	manager.AddEvent("cam-1", "Motion detected")

	req := httptest.NewRequest("GET", "/api/v1/surveillance/events?cameraId=cam-1", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAddEvent(t *testing.T) {
	handlers, _ := setupTestHandlers(t)
	r := setupTestRouter(handlers)

	reqBody := map[string]interface{}{
		"cameraId": "cam-1",
		"message":  "Test event",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/surveillance/events", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ==================== 流媒体测试 ====================

func TestStartStream(t *testing.T) {
	handlers, manager := setupTestHandlers(t)
	r := setupTestRouter(handlers)

	cam := &Camera{
		ID:       "cam-1",
		Name:     "Test Camera",
		URI:      "rtsp://192.168.1.100/stream1",
		Status:   CameraStatusOnline,
		Enabled:  true,
	}
	err := manager.AddCamera(cam)
	assert.NoError(t, err)

	reqBody := map[string]interface{}{
		"cameraId": "cam-1",
		"protocol": "hls",
		"clientId": "client-1",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/surveillance/streams/start", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestStopStream(t *testing.T) {
	handlers, manager := setupTestHandlers(t)
	r := setupTestRouter(handlers)

	cam := &Camera{
		ID:       "cam-1",
		Name:     "Test Camera",
		URI:      "rtsp://192.168.1.100/stream1",
		Status:   CameraStatusOnline,
		Enabled:  true,
	}
	err := manager.AddCamera(cam)
	assert.NoError(t, err)

	session, err := manager.StartStream("cam-1", "hls", "client-1")
	assert.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/v1/surveillance/streams/"+session.ID+"/stop", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetActiveStreams(t *testing.T) {
	handlers, _ := setupTestHandlers(t)
	r := setupTestRouter(handlers)

	req := httptest.NewRequest("GET", "/api/v1/surveillance/streams/active", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ==================== 移动侦测测试 ====================

func TestSetMotionDetection(t *testing.T) {
	handlers, manager := setupTestHandlers(t)
	r := setupTestRouter(handlers)

	cam := &Camera{
		ID:       "cam-1",
		Name:     "Test Camera",
		URI:      "rtsp://192.168.1.100/stream1",
		Enabled:  true,
	}
	err := manager.AddCamera(cam)
	assert.NoError(t, err)

	cfg := MotionDetectionConfig{
		CameraID:    "cam-1",
		Enabled:     true,
		Sensitivity: 75,
		Regions: []MotionRegion{
			{
				ID:      "region-1",
				Name:    "Main Area",
				X:       0,
				Y:       0,
				Width:   1920,
				Height:  1080,
				Enabled: true,
			},
		},
		Cooldown: 30,
	}

	body, _ := json.Marshal(cfg)
	req := httptest.NewRequest("PUT", "/api/v1/surveillance/motion/cam-1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetMotionDetection(t *testing.T) {
	handlers, manager := setupTestHandlers(t)
	r := setupTestRouter(handlers)

	cam := &Camera{
		ID:       "cam-1",
		Name:     "Test Camera",
		URI:      "rtsp://192.168.1.100/stream1",
		Enabled:  true,
	}
	err := manager.AddCamera(cam)
	assert.NoError(t, err)

	cfg := &MotionDetectionConfig{
		CameraID:    "cam-1",
		Enabled:     true,
		Sensitivity: 75,
	}
	err = manager.SetMotionDetection(cfg)
	assert.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/surveillance/motion/cam-1", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ==================== 回放测试 ====================

func TestQueryPlayback(t *testing.T) {
	handlers, _ := setupTestHandlers(t)
	r := setupTestRouter(handlers)

	query := PlaybackQuery{
		CameraID:  "cam-1",
		StartTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now(),
	}

	body, _ := json.Marshal(query)
	req := httptest.NewRequest("POST", "/api/v1/surveillance/playback/query", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ==================== 导出测试 ====================

func TestCreateExport(t *testing.T) {
	handlers, manager := setupTestHandlers(t)
	r := setupTestRouter(handlers)

	cam := &Camera{
		ID:       "cam-1",
		Name:     "Test Camera",
		URI:      "rtsp://192.168.1.100/stream1",
		Enabled:  true,
	}
	err := manager.AddCamera(cam)
	assert.NoError(t, err)

	reqBody := ExportRequest{
		CameraID:  "cam-1",
		StartTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now(),
		Format:    "mp4",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/surveillance/exports", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetExportJob(t *testing.T) {
	handlers, manager := setupTestHandlers(t)
	r := setupTestRouter(handlers)

	cam := &Camera{
		ID:       "cam-1",
		Name:     "Test Camera",
		URI:      "rtsp://192.168.1.100/stream1",
		Enabled:  true,
	}
	err := manager.AddCamera(cam)
	assert.NoError(t, err)

	exportReq := ExportRequest{
		CameraID:  "cam-1",
		StartTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now(),
		Format:    "mp4",
	}

	job, err := manager.CreateExport(exportReq)
	assert.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/surveillance/exports/"+job.ID, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ==================== 存储配额测试 ====================

func TestSetStorageQuota(t *testing.T) {
	handlers, manager := setupTestHandlers(t)
	r := setupTestRouter(handlers)

	cam := &Camera{
		ID:       "cam-1",
		Name:     "Test Camera",
		URI:      "rtsp://192.168.1.100/stream1",
		Enabled:  true,
	}
	err := manager.AddCamera(cam)
	assert.NoError(t, err)

	quota := StorageQuota{
		CameraID:      "cam-1",
		MaxSizeGB:     100,
		RetentionDays: 30,
		AutoDelete:    true,
	}

	body, _ := json.Marshal(quota)
	req := httptest.NewRequest("PUT", "/api/v1/surveillance/storage/cam-1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetStorageQuota(t *testing.T) {
	handlers, _ := setupTestHandlers(t)
	r := setupTestRouter(handlers)

	req := httptest.NewRequest("GET", "/api/v1/surveillance/storage/cam-1", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ==================== PTZ 控制测试 ====================

func TestSendPTZCommand(t *testing.T) {
	handlers, manager := setupTestHandlers(t)
	r := setupTestRouter(handlers)

	cam := &Camera{
		ID:       "cam-1",
		Name:     "Test Camera",
		URI:      "rtsp://192.168.1.100/stream1",
		Status:   CameraStatusOnline,
		Enabled:  true,
	}
	err := manager.AddCamera(cam)
	assert.NoError(t, err)

	cmd := PTZCommand{
		CameraID: "cam-1",
		Action:   "pan",
		Speed:    50,
	}

	body, _ := json.Marshal(cmd)
	req := httptest.NewRequest("POST", "/api/v1/surveillance/ptz", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ==================== 录制计划测试 ====================

func TestAddSchedule(t *testing.T) {
	handlers, manager := setupTestHandlers(t)
	r := setupTestRouter(handlers)

	cam := &Camera{
		ID:       "cam-1",
		Name:     "Test Camera",
		URI:      "rtsp://192.168.1.100/stream1",
		Enabled:  true,
	}
	err := manager.AddCamera(cam)
	assert.NoError(t, err)

	schedule := RecordingSchedule{
		CameraID:  "cam-1",
		Name:      "Work Hours",
		Mode:      RecordingModeSchedule,
		DaysOfWeek: []int{1, 2, 3, 4, 5},
		StartTime: "09:00",
		EndTime:   "18:00",
		Enabled:   true,
	}

	body, _ := json.Marshal(schedule)
	req := httptest.NewRequest("POST", "/api/v1/surveillance/schedules", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestListSchedules(t *testing.T) {
	handlers, manager := setupTestHandlers(t)
	r := setupTestRouter(handlers)

	cam := &Camera{
		ID:       "cam-1",
		Name:     "Test Camera",
		URI:      "rtsp://192.168.1.100/stream1",
		Enabled:  true,
	}
	err := manager.AddCamera(cam)
	assert.NoError(t, err)

	schedule := &RecordingSchedule{
		ID:        "sched-1",
		CameraID:  "cam-1",
		Name:      "Work Hours",
		Mode:      RecordingModeSchedule,
		DaysOfWeek: []int{1, 2, 3, 4, 5},
		StartTime: "09:00",
		EndTime:   "18:00",
		Enabled:   true,
	}
	err = manager.AddSchedule(schedule)
	assert.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/surveillance/schedules?cameraId=cam-1", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDeleteSchedule(t *testing.T) {
	handlers, manager := setupTestHandlers(t)
	r := setupTestRouter(handlers)

	cam := &Camera{
		ID:       "cam-1",
		Name:     "Test Camera",
		URI:      "rtsp://192.168.1.100/stream1",
		Enabled:  true,
	}
	err := manager.AddCamera(cam)
	assert.NoError(t, err)

	schedule := &RecordingSchedule{
		ID:        "sched-1",
		CameraID:  "cam-1",
		Name:      "Work Hours",
		Mode:      RecordingModeSchedule,
		DaysOfWeek: []int{1, 2, 3, 4, 5},
		StartTime: "09:00",
		EndTime:   "18:00",
		Enabled:   true,
	}
	err = manager.AddSchedule(schedule)
	assert.NoError(t, err)

	req := httptest.NewRequest("DELETE", "/api/v1/surveillance/schedules/sched-1", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ==================== 统计测试 ====================

func TestGetStats(t *testing.T) {
	handlers, _ := setupTestHandlers(t)
	r := setupTestRouter(handlers)

	req := httptest.NewRequest("GET", "/api/v1/surveillance/stats", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(0), resp["code"])
}

// ==================== Manager 测试 ====================

func TestNewManager(t *testing.T) {
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	assert.NoError(t, err)
	assert.NotNil(t, manager)

	manager.Close()
}

func TestAddDuplicateCamera(t *testing.T) {
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	assert.NoError(t, err)
	defer manager.Close()

	cam := &Camera{
		ID:       "cam-1",
		Name:     "Test Camera",
		URI:      "rtsp://192.168.1.100/stream1",
		Enabled:  true,
	}

	err = manager.AddCamera(cam)
	assert.NoError(t, err)

	// 尝试添加重复的摄像头
	err = manager.AddCamera(cam)
	assert.Error(t, err)
}

func TestGetStatsWithData(t *testing.T) {
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	assert.NoError(t, err)
	defer manager.Close()

	// 添加摄像头
	cam1 := &Camera{
		ID:       "cam-1",
		Name:     "Camera 1",
		URI:      "rtsp://192.168.1.100/stream1",
		Status:   CameraStatusOnline,
		Enabled:  true,
	}
	cam2 := &Camera{
		ID:       "cam-2",
		Name:     "Camera 2",
		URI:      "rtsp://192.168.1.101/stream1",
		Status:   CameraStatusOffline,
		Enabled:  true,
	}

	err = manager.AddCamera(cam1)
	assert.NoError(t, err)
	err = manager.AddCamera(cam2)
	assert.NoError(t, err)

	stats := manager.GetStats()
	assert.Equal(t, 2, stats.TotalCameras)
	assert.Equal(t, 1, stats.OnlineCameras)
	assert.Equal(t, 1, stats.OfflineCameras)
}

func TestMotionDetectionWorkflow(t *testing.T) {
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	assert.NoError(t, err)
	defer manager.Close()

	cam := &Camera{
		ID:       "cam-1",
		Name:     "Test Camera",
		URI:      "rtsp://192.168.1.100/stream1",
		Enabled:  true,
	}
	err = manager.AddCamera(cam)
	assert.NoError(t, err)

	cfg := &MotionDetectionConfig{
		CameraID:    "cam-1",
		Enabled:     true,
		Sensitivity: 75,
		Regions: []MotionRegion{
			{
				ID:      "region-1",
				Name:    "Main Area",
				X:       0,
				Y:       0,
				Width:   1920,
				Height:  1080,
				Enabled: true,
			},
		},
		Cooldown: 30,
	}

	err = manager.SetMotionDetection(cfg)
	assert.NoError(t, err)

	savedCfg, err := manager.GetMotionDetection("cam-1")
	assert.NoError(t, err)
	assert.Equal(t, 75, savedCfg.Sensitivity)
	assert.Len(t, savedCfg.Regions, 1)
}

func TestRecordingWorkflow(t *testing.T) {
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	assert.NoError(t, err)
	defer manager.Close()

	cam := &Camera{
		ID:       "cam-1",
		Name:     "Test Camera",
		URI:      "rtsp://192.168.1.100/stream1",
		Status:   CameraStatusOnline,
		Enabled:  true,
	}
	err = manager.AddCamera(cam)
	assert.NoError(t, err)

	// 开始录制
	job, err := manager.StartRecording("cam-1", RecordingModeContinuous)
	assert.NoError(t, err)
	assert.Equal(t, "recording", job.Status)

	// 停止录制
	err = manager.StopRecording(job.ID)
	assert.NoError(t, err)

	// 获取录制列表
	recordings := manager.GetRecordings("cam-1")
	assert.Len(t, recordings, 1)
	assert.Equal(t, "completed", recordings[0].Status)
}

func TestStreamWorkflow(t *testing.T) {
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	assert.NoError(t, err)
	defer manager.Close()

	cam := &Camera{
		ID:       "cam-1",
		Name:     "Test Camera",
		URI:      "rtsp://192.168.1.100/stream1",
		Status:   CameraStatusOnline,
		Enabled:  true,
	}
	err = manager.AddCamera(cam)
	assert.NoError(t, err)

	// 开始流
	session, err := manager.StartStream("cam-1", "hls", "client-1")
	assert.NoError(t, err)
	assert.True(t, session.Active)

	// 获取活跃流
	streams := manager.GetActiveStreams()
	assert.Len(t, streams, 1)

	// 停止流
	err = manager.StopStream(session.ID)
	assert.NoError(t, err)

	streams = manager.GetActiveStreams()
	assert.Len(t, streams, 0)
}
