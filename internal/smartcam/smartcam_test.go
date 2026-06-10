// Package smartcam 测试
package smartcam

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	return NewManager(logger)
}

func setupTestRouter(t *testing.T, manager *Manager) *gin.Engine {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	handler := NewHandler(manager, logger)
	r := gin.New()
	api := r.Group("/api/v1")
	handler.RegisterRoutes(api)
	return r
}

// ========== Manager 测试 ==========

func TestNewManager(t *testing.T) {
	m := setupTestManager(t)
	if m == nil {
		t.Fatal("管理器不应为 nil")
	}
	if m.cameras == nil {
		t.Error("cameras map 应已初始化")
	}
	if m.config.MaxCameras != 64 {
		t.Errorf("默认最大摄像头数应为 64，得到 %d", m.config.MaxCameras)
	}
}

func TestNewManagerNilLogger(t *testing.T) {
	m := NewManager(nil)
	if m == nil {
		t.Fatal("管理器不应为 nil")
	}
}

func TestAddCamera(t *testing.T) {
	m := setupTestManager(t)

	cam, err := m.AddCamera(AddCameraRequest{
		Name:      "前门摄像头",
		Location:  "大门入口",
		IPAddress: "192.168.1.100",
		Port:      554,
		Protocol:  ProtocolRTSP,
		Username:  "admin",
		Password:  "password123",
	})
	if err != nil {
		t.Fatalf("添加摄像头失败: %v", err)
	}

	if cam.ID == "" {
		t.Error("摄像头 ID 不应为空")
	}
	if cam.Name != "前门摄像头" {
		t.Errorf("名称不匹配: got %s, want 前门摄像头", cam.Name)
	}
	if cam.IPAddress != "192.168.1.100" {
		t.Errorf("IP 不匹配: got %s", cam.IPAddress)
	}
	if cam.Status != CameraStatusOnline {
		t.Errorf("状态应为 online，得到 %s", cam.Status)
	}
	if cam.Password != "" {
		t.Error("返回的摄像头不应包含密码")
	}
	if cam.Recording.Mode != RecordingModeMotion {
		t.Errorf("默认录像模式应为 motion，得到 %s", cam.Recording.Mode)
	}
	if !cam.Motion.Enabled {
		t.Error("默认移动侦测应启用")
	}
}

func TestAddCameraDefaults(t *testing.T) {
	m := setupTestManager(t)

	cam, err := m.AddCamera(AddCameraRequest{
		Name:      "测试摄像头",
		IPAddress: "10.0.0.1",
	})
	if err != nil {
		t.Fatalf("添加摄像头失败: %v", err)
	}

	if cam.Port != 554 {
		t.Errorf("默认端口应为 554，得到 %d", cam.Port)
	}
	if cam.Protocol != ProtocolRTSP {
		t.Errorf("默认协议应为 rtsp，得到 %s", cam.Protocol)
	}
	if cam.FrameRate != 25 {
		t.Errorf("默认帧率应为 25，得到 %d", cam.FrameRate)
	}
	if cam.Resolution != "1080p" {
		t.Errorf("默认分辨率应为 1080p，得到 %s", cam.Resolution)
	}
}

func TestAddCameraDuplicateIP(t *testing.T) {
	m := setupTestManager(t)

	_, err := m.AddCamera(AddCameraRequest{
		Name:      "摄像头1",
		IPAddress: "192.168.1.100",
		Port:      554,
	})
	if err != nil {
		t.Fatalf("添加摄像头失败: %v", err)
	}

	_, err = m.AddCamera(AddCameraRequest{
		Name:      "摄像头2",
		IPAddress: "192.168.1.100",
		Port:      554,
	})
	if err == nil {
		t.Error("重复 IP 应返回错误")
	}
}

func TestAddCameraMaxLimit(t *testing.T) {
	m := setupTestManager(t)
	m.config.MaxCameras = 2

	m.AddCamera(AddCameraRequest{Name: "c1", IPAddress: "10.0.0.1"})
	m.AddCamera(AddCameraRequest{Name: "c2", IPAddress: "10.0.0.2"})

	_, err := m.AddCamera(AddCameraRequest{Name: "c3", IPAddress: "10.0.0.3"})
	if err == nil {
		t.Error("超过上限应返回错误")
	}
}

func TestGetCamera(t *testing.T) {
	m := setupTestManager(t)

	added, _ := m.AddCamera(AddCameraRequest{
		Name:      "测试摄像头",
		IPAddress: "192.168.1.100",
	})

	found, err := m.GetCamera(added.ID)
	if err != nil {
		t.Fatalf("获取摄像头失败: %v", err)
	}
	if found.Name != "测试摄像头" {
		t.Errorf("名称不匹配: got %s", found.Name)
	}
}

func TestGetCameraNotFound(t *testing.T) {
	m := setupTestManager(t)

	_, err := m.GetCamera("nonexistent")
	if err == nil {
		t.Error("不存在的摄像头应返回错误")
	}
}

func TestUpdateCamera(t *testing.T) {
	m := setupTestManager(t)

	added, _ := m.AddCamera(AddCameraRequest{
		Name:      "原始名称",
		IPAddress: "192.168.1.100",
	})

	newName := "新名称"
	newLocation := "新位置"
	updated, err := m.UpdateCamera(added.ID, UpdateCameraRequest{
		Name:     &newName,
		Location: &newLocation,
	})
	if err != nil {
		t.Fatalf("更新摄像头失败: %v", err)
	}
	if updated.Name != "新名称" {
		t.Errorf("名称未更新: got %s", updated.Name)
	}
	if updated.Location != "新位置" {
		t.Errorf("位置未更新: got %s", updated.Location)
	}
}

func TestUpdateCameraDisable(t *testing.T) {
	m := setupTestManager(t)

	added, _ := m.AddCamera(AddCameraRequest{
		Name:      "测试摄像头",
		IPAddress: "192.168.1.100",
	})

	enabled := false
	updated, _ := m.UpdateCamera(added.ID, UpdateCameraRequest{Enabled: &enabled})
	if updated.Status != CameraStatusDisabled {
		t.Errorf("禁用后状态应为 disabled，得到 %s", updated.Status)
	}

	enabled = true
	updated, _ = m.UpdateCamera(added.ID, UpdateCameraRequest{Enabled: &enabled})
	if updated.Status != CameraStatusOnline {
		t.Errorf("启用后状态应为 online，得到 %s", updated.Status)
	}
}

func TestRemoveCamera(t *testing.T) {
	m := setupTestManager(t)

	added, _ := m.AddCamera(AddCameraRequest{
		Name:      "要删除的摄像头",
		IPAddress: "192.168.1.100",
	})

	if err := m.RemoveCamera(added.ID); err != nil {
		t.Fatalf("删除摄像头失败: %v", err)
	}

	_, err := m.GetCamera(added.ID)
	if err == nil {
		t.Error("删除后不应能获取摄像头")
	}
}

func TestRemoveCameraNotFound(t *testing.T) {
	m := setupTestManager(t)

	err := m.RemoveCamera("nonexistent")
	if err == nil {
		t.Error("不存在的摄像头应返回错误")
	}
}

func TestListCameras(t *testing.T) {
	m := setupTestManager(t)

	if len(m.ListCameras()) != 0 {
		t.Error("初始摄像头列表应为空")
	}

	m.AddCamera(AddCameraRequest{Name: "c1", IPAddress: "10.0.0.1"})
	m.AddCamera(AddCameraRequest{Name: "c2", IPAddress: "10.0.0.2"})
	m.AddCamera(AddCameraRequest{Name: "c3", IPAddress: "10.0.0.3"})

	cameras := m.ListCameras()
	if len(cameras) != 3 {
		t.Errorf("应有 3 个摄像头，得到 %d", len(cameras))
	}
}

// ========== 录像测试 ==========

func TestStartRecording(t *testing.T) {
	m := setupTestManager(t)

	cam, _ := m.AddCamera(AddCameraRequest{
		Name:      "录像测试",
		IPAddress: "192.168.1.100",
	})

	rec, err := m.StartRecording(cam.ID, "manual")
	if err != nil {
		t.Fatalf("开始录像失败: %v", err)
	}
	if rec.CameraID != cam.ID {
		t.Errorf("摄像头ID不匹配: got %s", rec.CameraID)
	}
	if rec.Trigger != "manual" {
		t.Errorf("触发方式不匹配: got %s", rec.Trigger)
	}
	if rec.EndTime.IsZero() != true {
		t.Error("结束时间应为零值（录像中）")
	}
}

func TestStartRecordingCameraNotFound(t *testing.T) {
	m := setupTestManager(t)

	_, err := m.StartRecording("nonexistent", "manual")
	if err == nil {
		t.Error("不存在的摄像头应返回错误")
	}
}

func TestStartRecordingCameraOffline(t *testing.T) {
	m := setupTestManager(t)

	cam, _ := m.AddCamera(AddCameraRequest{
		Name:      "离线摄像头",
		IPAddress: "192.168.1.100",
	})

	// 禁用摄像头
	enabled := false
	m.UpdateCamera(cam.ID, UpdateCameraRequest{Enabled: &enabled})

	_, err := m.StartRecording(cam.ID, "manual")
	if err == nil {
		t.Error("离线摄像头不应能开始录像")
	}
}

func TestStartRecordingDuplicate(t *testing.T) {
	m := setupTestManager(t)

	cam, _ := m.AddCamera(AddCameraRequest{
		Name:      "重复录像测试",
		IPAddress: "192.168.1.100",
	})

	_, err := m.StartRecording(cam.ID, "manual")
	if err != nil {
		t.Fatalf("第一次录像失败: %v", err)
	}

	_, err = m.StartRecording(cam.ID, "manual")
	if err == nil {
		t.Error("重复录像应返回错误")
	}
}

func TestStopRecording(t *testing.T) {
	m := setupTestManager(t)

	cam, _ := m.AddCamera(AddCameraRequest{
		Name:      "停止录像测试",
		IPAddress: "192.168.1.100",
	})

	m.StartRecording(cam.ID, "manual")

	rec, err := m.StopRecording(cam.ID)
	if err != nil {
		t.Fatalf("停止录像失败: %v", err)
	}
	if rec.EndTime.IsZero() {
		t.Error("结束时间不应为零值")
	}
	if rec.Duration <= 0 {
		t.Error("录像时长应大于 0")
	}
}

func TestStopRecordingNotRecording(t *testing.T) {
	m := setupTestManager(t)

	cam, _ := m.AddCamera(AddCameraRequest{
		Name:      "未录像摄像头",
		IPAddress: "192.168.1.100",
	})

	_, err := m.StopRecording(cam.ID)
	if err == nil {
		t.Error("未录像的摄像头应返回错误")
	}
}

func TestGetRecordings(t *testing.T) {
	m := setupTestManager(t)

	cam, _ := m.AddCamera(AddCameraRequest{
		Name:      "录像列表测试",
		IPAddress: "192.168.1.100",
	})

	// 创建几段录像
	for i := 0; i < 3; i++ {
		m.StartRecording(cam.ID, "manual")
		m.StopRecording(cam.ID)
	}

	recordings := m.GetRecordings("", 0)
	if len(recordings) != 3 {
		t.Errorf("应有 3 段录像，得到 %d", len(recordings))
	}

	// 按摄像头过滤
	recordings = m.GetRecordings(cam.ID, 2)
	if len(recordings) != 2 {
		t.Errorf("限制 2 条，得到 %d", len(recordings))
	}
}

func TestDeleteRecording(t *testing.T) {
	m := setupTestManager(t)

	cam, _ := m.AddCamera(AddCameraRequest{
		Name:      "删除录像测试",
		IPAddress: "192.168.1.100",
	})

	m.StartRecording(cam.ID, "manual")
	rec, _ := m.StopRecording(cam.ID)

	err := m.DeleteRecording(rec.ID)
	if err != nil {
		t.Fatalf("删除录像失败: %v", err)
	}

	recordings := m.GetRecordings("", 0)
	if len(recordings) != 0 {
		t.Error("删除后应无录像")
	}
}

func TestDeleteRecordingNotFound(t *testing.T) {
	m := setupTestManager(t)

	err := m.DeleteRecording("nonexistent")
	if err == nil {
		t.Error("不存在的录像应返回错误")
	}
}

// ========== 移动侦测测试 ==========

func TestTriggerMotionEvent(t *testing.T) {
	m := setupTestManager(t)

	cam, _ := m.AddCamera(AddCameraRequest{
		Name:      "移动侦测测试",
		IPAddress: "192.168.1.100",
	})

	event, err := m.TriggerMotionEvent(cam.ID, "zone1", 0.85, &BoundingBox{
		X: 100, Y: 100, Width: 200, Height: 150,
	})
	if err != nil {
		t.Fatalf("触发移动侦测失败: %v", err)
	}

	if event.CameraID != cam.ID {
		t.Errorf("摄像头ID不匹配: got %s", event.CameraID)
	}
	if event.Confidence != 0.85 {
		t.Errorf("置信度不匹配: got %f", event.Confidence)
	}
}

func TestTriggerMotionEventCameraNotFound(t *testing.T) {
	m := setupTestManager(t)

	_, err := m.TriggerMotionEvent("nonexistent", "", 0.8, nil)
	if err == nil {
		t.Error("不存在的摄像头应返回错误")
	}
}

func TestTriggerMotionEventDisabled(t *testing.T) {
	m := setupTestManager(t)

	cam, _ := m.AddCamera(AddCameraRequest{
		Name:      "禁用侦测测试",
		IPAddress: "192.168.1.100",
	})

	// 禁用移动侦测
	cfg := cam.Motion
	cfg.Enabled = false
	m.UpdateCamera(cam.ID, UpdateCameraRequest{})

	// 重新读取配置来禁用
	cam2, _ := m.GetCamera(cam.ID)
	_ = cam2

	// 直接修改管理器中的配置
	m.mu.Lock()
	if c, ok := m.cameras[cam.ID]; ok {
		c.Motion.Enabled = false
	}
	m.mu.Unlock()

	_, err := m.TriggerMotionEvent(cam.ID, "", 0.8, nil)
	if err == nil {
		t.Error("禁用侦测的摄像头应返回错误")
	}
}

func TestTriggerMotionEventCooldown(t *testing.T) {
	m := setupTestManager(t)

	cam, _ := m.AddCamera(AddCameraRequest{
		Name:      "冷却测试",
		IPAddress: "192.168.1.100",
	})

	// 设置冷却时间为 60 秒
	m.mu.Lock()
	m.cameras[cam.ID].Motion.CooldownSec = 60
	m.mu.Unlock()

	// 第一次触发应成功
	_, err := m.TriggerMotionEvent(cam.ID, "", 0.8, nil)
	if err != nil {
		t.Fatalf("第一次触发失败: %v", err)
	}

	// 立即再次触发应在冷却期内
	_, err = m.TriggerMotionEvent(cam.ID, "", 0.8, nil)
	if err == nil {
		t.Error("冷却期内应返回错误")
	}
}

func TestGetMotionEvents(t *testing.T) {
	m := setupTestManager(t)

	cam, _ := m.AddCamera(AddCameraRequest{
		Name:      "事件列表测试",
		IPAddress: "192.168.1.100",
	})

	// 触发一些事件（使用不同摄像头或不同冷却）
	for i := 0; i < 3; i++ {
		m.motionEvents = append(m.motionEvents, &MotionEvent{
			ID:       "event-" + string(rune('a'+i)),
			CameraID: cam.ID,
		})
	}

	events := m.GetMotionEvents(MotionEventQuery{CameraID: cam.ID, Limit: 10})
	if len(events) != 3 {
		t.Errorf("应有 3 个事件，得到 %d", len(events))
	}
}

// ========== 系统状态测试 ==========

func TestGetStatus(t *testing.T) {
	m := setupTestManager(t)

	m.AddCamera(AddCameraRequest{Name: "c1", IPAddress: "10.0.0.1"})
	m.AddCamera(AddCameraRequest{Name: "c2", IPAddress: "10.0.0.2"})

	status := m.GetStatus()
	if status.TotalCameras != 2 {
		t.Errorf("总摄像头应为 2，得到 %d", status.TotalCameras)
	}
	if status.OnlineCameras != 2 {
		t.Errorf("在线摄像头应为 2，得到 %d", status.OnlineCameras)
	}
	if status.Uptime == "" {
		t.Error("运行时间不应为空")
	}
}

func TestGetStorageStats(t *testing.T) {
	m := setupTestManager(t)

	cam, _ := m.AddCamera(AddCameraRequest{
		Name:      "存储统计测试",
		IPAddress: "192.168.1.100",
	})

	m.StartRecording(cam.ID, "manual")
	m.StopRecording(cam.ID)

	stats := m.GetStorageStats()
	if stats.TotalRecordings != 1 {
		t.Errorf("总录像数应为 1，得到 %d", stats.TotalRecordings)
	}
}

func TestGetConfig(t *testing.T) {
	m := setupTestManager(t)

	cfg := m.GetConfig()
	if cfg.MaxCameras != 64 {
		t.Errorf("默认最大摄像头数应为 64，得到 %d", cfg.MaxCameras)
	}
	if cfg.RetentionDays != 30 {
		t.Errorf("默认保留天数应为 30，得到 %d", cfg.RetentionDays)
	}
}

func TestUpdateConfig(t *testing.T) {
	m := setupTestManager(t)

	newCfg := SystemConfig{
		StoragePath:   "/custom/path",
		MaxStorageGB:  1000,
		MaxCameras:    128,
		RetentionDays: 60,
	}
	m.UpdateConfig(newCfg)

	cfg := m.GetConfig()
	if cfg.MaxCameras != 128 {
		t.Errorf("最大摄像头数应为 128，得到 %d", cfg.MaxCameras)
	}
	if cfg.StoragePath != "/custom/path" {
		t.Errorf("存储路径不匹配: got %s", cfg.StoragePath)
	}
}

// ========== Handler/Router 测试 ==========

func TestHandlerListCameras(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	m.AddCamera(AddCameraRequest{Name: "test", IPAddress: "10.0.0.1"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/surveillance/cameras", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，得到 %d", w.Code)
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 {
		t.Errorf("响应码应为 0，得到 %d", resp.Code)
	}
}

func TestHandlerAddCamera(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := AddCameraRequest{
		Name:      "API 添加",
		IPAddress: "192.168.1.200",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/surveillance/cameras", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("状态码应为 201，得到 %d", w.Code)
	}
}

func TestHandlerAddCameraInvalidBody(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/surveillance/cameras", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("无效请求应返回 400，得到 %d", w.Code)
	}
}

func TestHandlerGetCamera(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	cam, _ := m.AddCamera(AddCameraRequest{Name: "test", IPAddress: "10.0.0.1"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/surveillance/cameras/"+cam.ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，得到 %d", w.Code)
	}
}

func TestHandlerGetCameraNotFound(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/surveillance/cameras/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("不存在的摄像头应返回 404，得到 %d", w.Code)
	}
}

func TestHandlerUpdateCamera(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	cam, _ := m.AddCamera(AddCameraRequest{Name: "test", IPAddress: "10.0.0.1"})

	newName := "更新后的名称"
	body := UpdateCameraRequest{Name: &newName}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/surveillance/cameras/"+cam.ID, bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，得到 %d", w.Code)
	}
}

func TestHandlerDeleteCamera(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	cam, _ := m.AddCamera(AddCameraRequest{Name: "test", IPAddress: "10.0.0.1"})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/surveillance/cameras/"+cam.ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，得到 %d", w.Code)
	}
}

func TestHandlerStartRecording(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	cam, _ := m.AddCamera(AddCameraRequest{Name: "test", IPAddress: "10.0.0.1"})

	body := StartRecordingRequest{CameraID: cam.ID, Mode: "manual"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/surveillance/recordings/start", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("状态码应为 201，得到 %d", w.Code)
	}
}

func TestHandlerStopRecording(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	cam, _ := m.AddCamera(AddCameraRequest{Name: "test", IPAddress: "10.0.0.1"})
	m.StartRecording(cam.ID, "manual")

	body := map[string]string{"cameraId": cam.ID}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/surveillance/recordings/stop", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，得到 %d", w.Code)
	}
}

func TestHandlerListRecordings(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/surveillance/recordings?limit=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，得到 %d", w.Code)
	}
}

func TestHandlerGetMotionEvents(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/surveillance/motion/events?limit=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，得到 %d", w.Code)
	}
}

func TestHandlerTriggerMotion(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	cam, _ := m.AddCamera(AddCameraRequest{Name: "test", IPAddress: "10.0.0.1"})

	body := map[string]interface{}{
		"cameraId":   cam.ID,
		"confidence": 0.9,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/surveillance/motion/trigger", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，得到 %d", w.Code)
	}
}

func TestHandlerGetStatus(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/surveillance/system/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，得到 %d", w.Code)
	}
}

func TestHandlerGetStorageStats(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/surveillance/system/storage", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，得到 %d", w.Code)
	}
}

func TestHandlerGetConfig(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/surveillance/system/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，得到 %d", w.Code)
	}
}

func TestHandlerUpdateConfig(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	cfg := SystemConfig{
		MaxCameras:   128,
		StoragePath:  "/custom",
		MaxStorageGB: 1000,
	}
	jsonBody, _ := json.Marshal(cfg)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/surveillance/system/config", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，得到 %d", w.Code)
	}
}

func TestHandlerEnableDisableCamera(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	cam, _ := m.AddCamera(AddCameraRequest{Name: "test", IPAddress: "10.0.0.1"})

	// 禁用
	req := httptest.NewRequest(http.MethodPost, "/api/v1/surveillance/cameras/"+cam.ID+"/disable", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("禁用状态码应为 200，得到 %d", w.Code)
	}

	// 启用
	req = httptest.NewRequest(http.MethodPost, "/api/v1/surveillance/cameras/"+cam.ID+"/enable", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("启用状态码应为 200，得到 %d", w.Code)
	}
}
