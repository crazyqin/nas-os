// Package nvrmgr 单元测试
package nvrmgr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ========== Manager 测试 ==========

func TestNewManager(t *testing.T) {
	manager := NewManager()
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.cameras)
	assert.NotNil(t, manager.recordings)
	assert.NotNil(t, manager.motionRules)
	assert.NotNil(t, manager.motionEvents)
	assert.NotNil(t, manager.alerts)
	assert.NotNil(t, manager.storagePlans)
}

func TestDefaultStoragePlans(t *testing.T) {
	manager := NewManager()

	// 验证 3 个默认存储计划
	assert.Equal(t, 3, len(manager.storagePlans))

	plan7d, exists := manager.storagePlans["plan-7d"]
	assert.True(t, exists)
	assert.Equal(t, "7天存储", plan7d.Name)
	assert.Equal(t, 7, plan7d.RetentionDays)

	plan30d, exists := manager.storagePlans["plan-30d"]
	assert.True(t, exists)
	assert.Equal(t, "30天存储", plan30d.Name)
	assert.Equal(t, 30, plan30d.RetentionDays)

	plan90d, exists := manager.storagePlans["plan-90d"]
	assert.True(t, exists)
	assert.Equal(t, "90天存储", plan90d.Name)
	assert.Equal(t, 90, plan90d.RetentionDays)
}

func TestAddCamera(t *testing.T) {
	manager := NewManager()

	cam := &Camera{
		ID:         "cam-001",
		Name:       "大门摄像头",
		URL:        "rtsp://192.168.1.100:554/stream1",
		Protocol:   ProtocolRTSP,
		Resolution: "1920x1080",
		FPS:        30,
		Location:   "大门",
	}

	result, err := manager.AddCamera(cam)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "cam-001", result.ID)
	assert.Equal(t, CameraStatusOffline, result.Status)
	assert.True(t, result.Enabled)
}

func TestAddCameraDuplicate(t *testing.T) {
	manager := NewManager()

	cam := &Camera{
		ID:   "cam-001",
		Name: "摄像头1",
		URL:  "rtsp://192.168.1.100:554/stream1",
	}

	_, err := manager.AddCamera(cam)
	assert.NoError(t, err)

	// 重复添加应该失败
	_, err = manager.AddCamera(cam)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已存在")
}

func TestAddCameraEmptyID(t *testing.T) {
	manager := NewManager()

	cam := &Camera{
		Name: "摄像头",
		URL:  "rtsp://192.168.1.100:554/stream1",
	}

	_, err := manager.AddCamera(cam)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ID 不能为空")
}

func TestUpdateCamera(t *testing.T) {
	manager := NewManager()

	cam := &Camera{
		ID:   "cam-001",
		Name: "原始名称",
		URL:  "rtsp://192.168.1.100:554/stream1",
	}
	manager.AddCamera(cam)

	// 更新摄像头
	updated := &Camera{
		Name:     "更新后的名称",
		URL:      "rtsp://192.168.1.100:554/stream2",
		Location: "后门",
	}

	result, err := manager.UpdateCamera("cam-001", updated)
	assert.NoError(t, err)
	assert.Equal(t, "更新后的名称", result.Name)
	assert.Equal(t, "rtsp://192.168.1.100:554/stream2", result.URL)
	assert.Equal(t, "cam-001", result.ID)
}

func TestUpdateCameraNotFound(t *testing.T) {
	manager := NewManager()

	cam := &Camera{Name: "test"}
	_, err := manager.UpdateCamera("nonexistent", cam)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestDeleteCamera(t *testing.T) {
	manager := NewManager()

	cam := &Camera{
		ID:   "cam-001",
		Name: "摄像头",
		URL:  "rtsp://192.168.1.100:554/stream1",
	}
	manager.AddCamera(cam)

	err := manager.DeleteCamera("cam-001")
	assert.NoError(t, err)

	_, err = manager.GetCamera("cam-001")
	assert.Error(t, err)
}

func TestDeleteCameraNotFound(t *testing.T) {
	manager := NewManager()

	err := manager.DeleteCamera("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestGetCamera(t *testing.T) {
	manager := NewManager()

	cam := &Camera{
		ID:   "cam-001",
		Name: "摄像头",
		URL:  "rtsp://192.168.1.100:554/stream1",
	}
	manager.AddCamera(cam)

	result, err := manager.GetCamera("cam-001")
	assert.NoError(t, err)
	assert.Equal(t, "摄像头", result.Name)
}

func TestGetCameraNotFound(t *testing.T) {
	manager := NewManager()

	_, err := manager.GetCamera("nonexistent")
	assert.Error(t, err)
}

func TestListCameras(t *testing.T) {
	manager := NewManager()

	for i := 0; i < 3; i++ {
		cam := &Camera{
			ID:   fmt.Sprintf("cam-%03d", i),
			Name: fmt.Sprintf("摄像头%d", i),
			URL:  fmt.Sprintf("rtsp://192.168.1.%d:554/stream1", i),
		}
		manager.AddCamera(cam)
	}

	cameras, err := manager.ListCameras()
	assert.NoError(t, err)
	assert.Equal(t, 3, len(cameras))
}

func TestStartRecording(t *testing.T) {
	manager := NewManager()

	cam := &Camera{
		ID:   "cam-001",
		Name: "摄像头",
		URL:  "rtsp://192.168.1.100:554/stream1",
	}
	manager.AddCamera(cam)

	err := manager.StartRecording("cam-001")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(manager.recordings))
	assert.Equal(t, "cam-001", manager.recordings[0].CameraID)
}

func TestStartRecordingCameraNotFound(t *testing.T) {
	manager := NewManager()

	err := manager.StartRecording("nonexistent")
	assert.Error(t, err)
}

func TestStopRecording(t *testing.T) {
	manager := NewManager()

	cam := &Camera{
		ID:   "cam-001",
		Name: "摄像头",
		URL:  "rtsp://192.168.1.100:554/stream1",
	}
	manager.AddCamera(cam)

	manager.StartRecording("cam-001")
	time.Sleep(100 * time.Millisecond)
	err := manager.StopRecording("cam-001")
	assert.NoError(t, err)

	// 验证录像已停止
	assert.False(t, manager.recordings[0].EndTime.IsZero())
	assert.GreaterOrEqual(t, manager.recordings[0].Duration, int64(0))
}

func TestStopRecordingNotRecording(t *testing.T) {
	manager := NewManager()

	cam := &Camera{
		ID:   "cam-001",
		Name: "摄像头",
		URL:  "rtsp://192.168.1.100:554/stream1",
	}
	manager.AddCamera(cam)

	err := manager.StopRecording("cam-001")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "没有正在录制")
}

func TestGetRecordings(t *testing.T) {
	manager := NewManager()

	cam := &Camera{
		ID:   "cam-001",
		Name: "摄像头",
		URL:  "rtsp://192.168.1.100:554/stream1",
	}
	manager.AddCamera(cam)

	// 创建多个录像
	for i := 0; i < 5; i++ {
		manager.StartRecording("cam-001")
		time.Sleep(10 * time.Millisecond)
		manager.StopRecording("cam-001")
	}

	recordings, total, err := manager.GetRecordings("cam-001", time.Time{}, time.Time{}, 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Equal(t, 5, len(recordings))
}

func TestGetRecordingsPagination(t *testing.T) {
	manager := NewManager()

	cam := &Camera{
		ID:   "cam-001",
		Name: "摄像头",
		URL:  "rtsp://192.168.1.100:554/stream1",
	}
	manager.AddCamera(cam)

	// 创建 10 个录像
	for i := 0; i < 10; i++ {
		manager.StartRecording("cam-001")
		time.Sleep(10 * time.Millisecond)
		manager.StopRecording("cam-001")
	}

	// 第一页
	recordings, total, err := manager.GetRecordings("cam-001", time.Time{}, time.Time{}, 1, 3)
	assert.NoError(t, err)
	assert.Equal(t, 10, total)
	assert.Equal(t, 3, len(recordings))

	// 第二页
	recordings, total, err = manager.GetRecordings("cam-001", time.Time{}, time.Time{}, 2, 3)
	assert.NoError(t, err)
	assert.Equal(t, 10, total)
	assert.Equal(t, 3, len(recordings))
}

func TestDeleteRecording(t *testing.T) {
	manager := NewManager()

	cam := &Camera{
		ID:   "cam-001",
		Name: "摄像头",
		URL:  "rtsp://192.168.1.100:554/stream1",
	}
	manager.AddCamera(cam)

	manager.StartRecording("cam-001")
	recID := manager.recordings[0].ID

	err := manager.DeleteRecording(recID)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(manager.recordings))
}

func TestDeleteRecordingNotFound(t *testing.T) {
	manager := NewManager()

	err := manager.DeleteRecording("nonexistent")
	assert.Error(t, err)
}

func TestAddMotionRule(t *testing.T) {
	manager := NewManager()

	cam := &Camera{
		ID:   "cam-001",
		Name: "摄像头",
		URL:  "rtsp://192.168.1.100:554/stream1",
	}
	manager.AddCamera(cam)

	err := manager.AddMotionRule("cam-001", "zone-1", 0.8)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(manager.motionRules["cam-001"]))
}

func TestAddMotionRuleInvalidSensitivity(t *testing.T) {
	manager := NewManager()

	cam := &Camera{
		ID:   "cam-001",
		Name: "摄像头",
		URL:  "rtsp://192.168.1.100:554/stream1",
	}
	manager.AddCamera(cam)

	err := manager.AddMotionRule("cam-001", "zone-1", 1.5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "灵敏度")
}

func TestGetMotionEvents(t *testing.T) {
	manager := NewManager()

	cam := &Camera{
		ID:   "cam-001",
		Name: "摄像头",
		URL:  "rtsp://192.168.1.100:554/stream1",
	}
	manager.AddCamera(cam)

	// 手动添加移动侦测事件
	evt := &MotionEvent{
		ID:         "evt-001",
		CameraID:   "cam-001",
		Timestamp:  time.Now(),
		Duration:   5,
		Confidence: 0.95,
		Zone:       "zone-1",
	}
	manager.motionEvents = append(manager.motionEvents, evt)

	events, err := manager.GetMotionEvents("cam-001", time.Time{}, time.Time{})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(events))
}

func TestCreateAlert(t *testing.T) {
	manager := NewManager()

	cam := &Camera{
		ID:   "cam-001",
		Name: "摄像头",
		URL:  "rtsp://192.168.1.100:554/stream1",
	}
	manager.AddCamera(cam)

	alert := &Alert{
		CameraID: "cam-001",
		Type:     AlertMotion,
		Message:  "检测到移动",
	}

	result, err := manager.CreateAlert(alert)
	assert.NoError(t, err)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, "cam-001", result.CameraID)
	assert.False(t, result.Acknowledged)
}

func TestListAlerts(t *testing.T) {
	manager := NewManager()

	cam := &Camera{
		ID:   "cam-001",
		Name: "摄像头",
		URL:  "rtsp://192.168.1.100:554/stream1",
	}
	manager.AddCamera(cam)

	// 创建多个告警
	for i := 0; i < 3; i++ {
		alert := &Alert{
			CameraID: "cam-001",
			Type:     AlertMotion,
			Message:  fmt.Sprintf("告警 %d", i),
		}
		manager.CreateAlert(alert)
	}

	alerts, err := manager.ListAlerts(false)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(alerts))
}

func TestListAlertsUnread(t *testing.T) {
	manager := NewManager()

	cam := &Camera{
		ID:   "cam-001",
		Name: "摄像头",
		URL:  "rtsp://192.168.1.100:554/stream1",
	}
	manager.AddCamera(cam)

	// 创建 3 个告警
	for i := 0; i < 3; i++ {
		alert := &Alert{
			CameraID: "cam-001",
			Type:     AlertMotion,
			Message:  fmt.Sprintf("告警 %d", i),
		}
		manager.CreateAlert(alert)
	}

	// 确认第一个告警
	manager.AcknowledgeAlert(manager.alerts[0].ID)

	unread, err := manager.ListAlerts(true)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(unread))
}

func TestAcknowledgeAlert(t *testing.T) {
	manager := NewManager()

	cam := &Camera{
		ID:   "cam-001",
		Name: "摄像头",
		URL:  "rtsp://192.168.1.100:554/stream1",
	}
	manager.AddCamera(cam)

	alert := &Alert{
		CameraID: "cam-001",
		Type:     AlertMotion,
		Message:  "检测到移动",
	}
	manager.CreateAlert(alert)

	err := manager.AcknowledgeAlert(manager.alerts[0].ID)
	assert.NoError(t, err)
	assert.True(t, manager.alerts[0].Acknowledged)
}

func TestAcknowledgeAlertNotFound(t *testing.T) {
	manager := NewManager()

	err := manager.AcknowledgeAlert("nonexistent")
	assert.Error(t, err)
}

func TestCreateStoragePlan(t *testing.T) {
	manager := NewManager()

	plan := &StoragePlan{
		ID:            "plan-custom",
		Name:          "自定义计划",
		RetentionDays: 14,
		MaxSize:       200 * 1024 * 1024 * 1024,
		Quality:       "high",
		Schedule:      "weekday",
	}

	result, err := manager.CreateStoragePlan(plan)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "plan-custom", result.ID)
}

func TestCreateStoragePlanDuplicate(t *testing.T) {
	manager := NewManager()

	plan := &StoragePlan{
		ID:   "plan-7d",
		Name: "重复计划",
	}

	_, err := manager.CreateStoragePlan(plan)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已存在")
}

func TestCreateStoragePlanEmptyID(t *testing.T) {
	manager := NewManager()

	plan := &StoragePlan{
		Name: "无ID计划",
	}

	_, err := manager.CreateStoragePlan(plan)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不能为空")
}

func TestApplyStoragePlan(t *testing.T) {
	manager := NewManager()

	cam := &Camera{
		ID:   "cam-001",
		Name: "摄像头",
		URL:  "rtsp://192.168.1.100:554/stream1",
	}
	manager.AddCamera(cam)

	err := manager.ApplyStoragePlan("plan-7d", []string{"cam-001"})
	assert.NoError(t, err)

	plan := manager.storagePlans["plan-7d"]
	assert.Contains(t, plan.Cameras, "cam-001")
}

func TestApplyStoragePlanNotFound(t *testing.T) {
	manager := NewManager()

	err := manager.ApplyStoragePlan("nonexistent", []string{"cam-001"})
	assert.Error(t, err)
}

func TestCleanupOldRecordings(t *testing.T) {
	manager := NewManager()

	cam := &Camera{
		ID:   "cam-001",
		Name: "摄像头",
		URL:  "rtsp://192.168.1.100:554/stream1",
	}
	manager.AddCamera(cam)

	// 添加一个过期录像（手动设置时间）
	recording := &Recording{
		ID:        "rec-old",
		CameraID:  "cam-001",
		StartTime: time.Now().AddDate(0, 0, -60), // 60天前
		EndTime:   time.Now().AddDate(0, 0, -59),
		FilePath:  "/recordings/cam-001/rec-old.mp4",
		CreatedAt: time.Now().AddDate(0, 0, -60),
	}
	manager.recordings = append(manager.recordings, recording)

	// 应用存储计划
	manager.ApplyStoragePlan("plan-7d", []string{"cam-001"})

	cleaned, err := manager.CleanupOldRecordings()
	assert.NoError(t, err)
	assert.Equal(t, 1, cleaned)
	assert.Equal(t, 0, len(manager.recordings))
}

func TestGetStorageUsage(t *testing.T) {
	manager := NewManager()

	cam := &Camera{
		ID:   "cam-001",
		Name: "摄像头",
		URL:  "rtsp://192.168.1.100:554/stream1",
	}
	manager.AddCamera(cam)

	// 添加录像
	recording := &Recording{
		ID:       "rec-001",
		CameraID: "cam-001",
		Size:     1024 * 1024 * 100, // 100MB
	}
	manager.recordings = append(manager.recordings, recording)

	usage, err := manager.GetStorageUsage()
	assert.NoError(t, err)
	assert.Equal(t, int64(1024*1024*100), usage["cam-001"])
	assert.Equal(t, int64(1024*1024*100), usage["total"])
}

func TestGetCameraStatus(t *testing.T) {
	manager := NewManager()

	// 添加不同状态的摄像头
	cam1 := &Camera{ID: "cam-001", Name: "在线", URL: "rtsp://192.168.1.1", Status: CameraStatusOnline}
	cam2 := &Camera{ID: "cam-002", Name: "离线", URL: "rtsp://192.168.1.2", Status: CameraStatusOffline}
	cam3 := &Camera{ID: "cam-003", Name: "错误", URL: "rtsp://192.168.1.3", Status: CameraStatusError}

	manager.cameras["cam-001"] = cam1
	manager.cameras["cam-002"] = cam2
	manager.cameras["cam-003"] = cam3

	status, err := manager.GetCameraStatus()
	assert.NoError(t, err)
	assert.Equal(t, 3, status["total"])
	assert.Equal(t, 1, status["online"])
	assert.Equal(t, 1, status["offline"])
	assert.Equal(t, 1, status["error"])
}

// ========== Handler 测试 ==========

func TestHandlerRegisterRoutes(t *testing.T) {
	manager := NewManager()
	handler := NewHandler(manager)
	assert.NotNil(t, handler)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// 验证路由注册成功（通过发起请求来验证）
	req := httptest.NewRequest(http.MethodGet, "/api/nvrmgr/cameras", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleCamerasGet(t *testing.T) {
	manager := NewManager()
	handler := NewHandler(manager)

	// 添加测试数据
	cam := &Camera{ID: "cam-001", Name: "测试摄像头", URL: "rtsp://192.168.1.100:554/stream1"}
	manager.AddCamera(cam)

	req := httptest.NewRequest(http.MethodGet, "/api/nvrmgr/cameras", nil)
	w := httptest.NewRecorder()
	handler.handleCameras(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp APIResponse
	json.NewDecoder(w.Body).Decode(&resp)
	assert.True(t, resp.Success)
}

func TestHandleCamerasPost(t *testing.T) {
	manager := NewManager()
	handler := NewHandler(manager)

	cam := Camera{
		ID:   "cam-001",
		Name: "新摄像头",
		URL:  "rtsp://192.168.1.100:554/stream1",
	}

	body, _ := json.Marshal(cam)
	req := httptest.NewRequest(http.MethodPost, "/api/nvrmgr/cameras", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.handleCameras(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp APIResponse
	json.NewDecoder(w.Body).Decode(&resp)
	assert.True(t, resp.Success)
}

func TestHandleCameraByIDGet(t *testing.T) {
	manager := NewManager()
	handler := NewHandler(manager)

	cam := &Camera{ID: "cam-001", Name: "测试摄像头", URL: "rtsp://192.168.1.100:554/stream1"}
	manager.AddCamera(cam)

	req := httptest.NewRequest(http.MethodGet, "/api/nvrmgr/cameras/cam-001", nil)
	w := httptest.NewRecorder()
	handler.handleCameraByID(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleCameraByIDPut(t *testing.T) {
	manager := NewManager()
	handler := NewHandler(manager)

	cam := &Camera{ID: "cam-001", Name: "原始名称", URL: "rtsp://192.168.1.100:554/stream1"}
	manager.AddCamera(cam)

	updated := Camera{Name: "更新名称"}
	body, _ := json.Marshal(updated)
	req := httptest.NewRequest(http.MethodPut, "/api/nvrmgr/cameras/cam-001", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.handleCameraByID(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleCameraByIDDelete(t *testing.T) {
	manager := NewManager()
	handler := NewHandler(manager)

	cam := &Camera{ID: "cam-001", Name: "测试摄像头", URL: "rtsp://192.168.1.100:554/stream1"}
	manager.AddCamera(cam)

	req := httptest.NewRequest(http.MethodDelete, "/api/nvrmgr/cameras/cam-001", nil)
	w := httptest.NewRecorder()
	handler.handleCameraByID(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleRecordingsGet(t *testing.T) {
	manager := NewManager()
	handler := NewHandler(manager)

	cam := &Camera{ID: "cam-001", Name: "摄像头", URL: "rtsp://192.168.1.100:554/stream1"}
	manager.AddCamera(cam)
	manager.StartRecording("cam-001")

	req := httptest.NewRequest(http.MethodGet, "/api/nvrmgr/recordings?cameraId=cam-001&page=1&pageSize=10", nil)
	w := httptest.NewRecorder()
	handler.handleRecordings(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleRecordingsPost(t *testing.T) {
	manager := NewManager()
	handler := NewHandler(manager)

	cam := &Camera{ID: "cam-001", Name: "摄像头", URL: "rtsp://192.168.1.100:554/stream1"}
	manager.AddCamera(cam)

	reqBody := map[string]string{"cameraId": "cam-001"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/nvrmgr/recordings", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.handleRecordings(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestHandleTimelineGet(t *testing.T) {
	manager := NewManager()
	handler := NewHandler(manager)

	cam := &Camera{ID: "cam-001", Name: "摄像头", URL: "rtsp://192.168.1.100:554/stream1"}
	manager.AddCamera(cam)

	req := httptest.NewRequest(http.MethodGet, "/api/nvrmgr/timeline?cameraId=cam-001&date=2024-01-01", nil)
	w := httptest.NewRecorder()
	handler.handleTimeline(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleMotionGet(t *testing.T) {
	manager := NewManager()
	handler := NewHandler(manager)

	cam := &Camera{ID: "cam-001", Name: "摄像头", URL: "rtsp://192.168.1.100:554/stream1"}
	manager.AddCamera(cam)

	req := httptest.NewRequest(http.MethodGet, "/api/nvrmgr/motion?cameraId=cam-001", nil)
	w := httptest.NewRecorder()
	handler.handleMotion(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleMotionPost(t *testing.T) {
	manager := NewManager()
	handler := NewHandler(manager)

	cam := &Camera{ID: "cam-001", Name: "摄像头", URL: "rtsp://192.168.1.100:554/stream1"}
	manager.AddCamera(cam)

	reqBody := map[string]interface{}{
		"cameraId":    "cam-001",
		"zone":        "zone-1",
		"sensitivity": 0.8,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/nvrmgr/motion", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.handleMotion(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestHandleAlertsGet(t *testing.T) {
	manager := NewManager()
	handler := NewHandler(manager)

	cam := &Camera{ID: "cam-001", Name: "摄像头", URL: "rtsp://192.168.1.100:554/stream1"}
	manager.AddCamera(cam)

	alert := &Alert{CameraID: "cam-001", Type: AlertMotion, Message: "测试告警"}
	manager.CreateAlert(alert)

	req := httptest.NewRequest(http.MethodGet, "/api/nvrmgr/alerts", nil)
	w := httptest.NewRecorder()
	handler.handleAlerts(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleAlertsPost(t *testing.T) {
	manager := NewManager()
	handler := NewHandler(manager)

	cam := &Camera{ID: "cam-001", Name: "摄像头", URL: "rtsp://192.168.1.100:554/stream1"}
	manager.AddCamera(cam)

	alert := Alert{
		CameraID: "cam-001",
		Type:     AlertMotion,
		Message:  "新告警",
	}
	body, _ := json.Marshal(alert)
	req := httptest.NewRequest(http.MethodPost, "/api/nvrmgr/alerts", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.handleAlerts(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestHandleAlertByIDPut(t *testing.T) {
	manager := NewManager()
	handler := NewHandler(manager)

	cam := &Camera{ID: "cam-001", Name: "摄像头", URL: "rtsp://192.168.1.100:554/stream1"}
	manager.AddCamera(cam)

	alert := &Alert{CameraID: "cam-001", Type: AlertMotion, Message: "测试告警"}
	manager.CreateAlert(alert)

	req := httptest.NewRequest(http.MethodPut, "/api/nvrmgr/alerts/"+manager.alerts[0].ID, nil)
	w := httptest.NewRecorder()
	handler.handleAlertByID(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleStoragePlansGet(t *testing.T) {
	manager := NewManager()
	handler := NewHandler(manager)

	req := httptest.NewRequest(http.MethodGet, "/api/nvrmgr/storage-plans", nil)
	w := httptest.NewRecorder()
	handler.handleStoragePlans(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp APIResponse
	json.NewDecoder(w.Body).Decode(&resp)
	assert.True(t, resp.Success)
}

func TestHandleStatsGet(t *testing.T) {
	manager := NewManager()
	handler := NewHandler(manager)

	req := httptest.NewRequest(http.MethodGet, "/api/nvrmgr/stats", nil)
	w := httptest.NewRecorder()
	handler.handleStats(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp APIResponse
	json.NewDecoder(w.Body).Decode(&resp)
	assert.True(t, resp.Success)
}

func TestHandleMethodNotAllowed(t *testing.T) {
	manager := NewManager()
	handler := NewHandler(manager)

	req := httptest.NewRequest(http.MethodPatch, "/api/nvrmgr/cameras", nil)
	w := httptest.NewRecorder()
	handler.handleCameras(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}
