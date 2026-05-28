// Package smartsurveillance 提供智能监控中心功能
// smartsurveillance_test.go - 完整测试
package smartsurveillance

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupTestEnvironment(t *testing.T) (*SurveillanceEngine, *DetectionEngine, *ZoneManager, *RecordingManager, *AlertManager) {
	logger, _ := zap.NewDevelopment()
	engine := NewSurveillanceEngine(logger, "/tmp/surveillance-test")
	detection := NewDetectionEngine(logger, engine)
	zoneMgr := NewZoneManager(logger, engine)
	recordingMgr := NewRecordingManager(logger, engine, "/tmp/surveillance-test")
	alertMgr := NewAlertManager(logger, engine)

	return engine, detection, zoneMgr, recordingMgr, alertMgr
}

func addTestCamera(t *testing.T, engine *SurveillanceEngine, id, name string) *Camera {
	camera := &Camera{
		ID:             id,
		Name:           name,
		Protocol:       "rtsp",
		URL:            "rtsp://192.168.1.100:554/stream1",
		Location:       "前门",
		Resolution:     "1080p",
		FPS:            30,
		AIEnabled:      true,
		DetectionTypes: []DetectionType{DetectionTypeFace, DetectionTypeObject, DetectionTypePlate},
	}
	err := engine.AddCamera(camera)
	require.NoError(t, err)
	return camera
}

// ========== 摄像头管理测试 ==========

func TestAddCamera(t *testing.T) {
	t.Run("成功添加摄像头", func(t *testing.T) {
		engine, _, _, _, _ := setupTestEnvironment(t)

		camera := &Camera{
			Name:     "前门摄像头",
			Protocol: "rtsp",
			URL:      "rtsp://192.168.1.100:554/stream1",
			Location: "前门",
		}

		err := engine.AddCamera(camera)
		require.NoError(t, err)
		assert.NotEmpty(t, camera.ID)
		assert.Equal(t, CameraStatusOffline, camera.Status)
	})

	t.Run("重复添加失败", func(t *testing.T) {
		engine, _, _, _, _ := setupTestEnvironment(t)

		camera := &Camera{
			ID:   "cam-1",
			Name: "摄像头1",
		}
		_ = engine.AddCamera(camera)

		err := engine.AddCamera(camera)
		assert.ErrorIs(t, err, ErrCameraExists)
	})
}

func TestGetCamera(t *testing.T) {
	t.Run("获取存在的摄像头", func(t *testing.T) {
		engine, _, _, _, _ := setupTestEnvironment(t)
		addTestCamera(t, engine, "cam-1", "前门")

		camera, err := engine.GetCamera("cam-1")
		require.NoError(t, err)
		assert.Equal(t, "前门", camera.Name)
	})

	t.Run("获取不存在的摄像头", func(t *testing.T) {
		engine, _, _, _, _ := setupTestEnvironment(t)

		_, err := engine.GetCamera("nonexistent")
		assert.ErrorIs(t, err, ErrCameraNotFound)
	})
}

func TestRemoveCamera(t *testing.T) {
	engine, _, _, _, _ := setupTestEnvironment(t)
	addTestCamera(t, engine, "cam-1", "前门")

	err := engine.RemoveCamera("cam-1")
	require.NoError(t, err)

	_, err = engine.GetCamera("cam-1")
	assert.ErrorIs(t, err, ErrCameraNotFound)
}

func TestListCameras(t *testing.T) {
	engine, _, _, _, _ := setupTestEnvironment(t)

	addTestCamera(t, engine, "cam-1", "前门")
	addTestCamera(t, engine, "cam-2", "后门")

	cameras := engine.ListCameras()
	assert.Len(t, cameras, 2)
}

func TestUpdateCameraStatus(t *testing.T) {
	engine, _, _, _, _ := setupTestEnvironment(t)
	addTestCamera(t, engine, "cam-1", "前门")

	err := engine.UpdateCameraStatus("cam-1", CameraStatusOnline)
	require.NoError(t, err)

	camera, _ := engine.GetCamera("cam-1")
	assert.Equal(t, CameraStatusOnline, camera.Status)
}

// ========== 录像管理测试 ==========

func TestStartStopRecording(t *testing.T) {
	engine, _, _, _, _ := setupTestEnvironment(t)
	addTestCamera(t, engine, "cam-1", "前门")

	t.Run("开始录像", func(t *testing.T) {
		err := engine.StartRecording("cam-1")
		require.NoError(t, err)

		camera, _ := engine.GetCamera("cam-1")
		assert.Equal(t, CameraStatusRecording, camera.Status)
	})

	t.Run("停止录像", func(t *testing.T) {
		err := engine.StopRecording("cam-1")
		require.NoError(t, err)

		camera, _ := engine.GetCamera("cam-1")
		assert.Equal(t, CameraStatusOnline, camera.Status)
	})
}

func TestGetRecordings(t *testing.T) {
	engine, _, _, recordingMgr, _ := setupTestEnvironment(t)
	addTestCamera(t, engine, "cam-1", "前门")

	// 开始并停止录像
	_ = engine.StartRecording("cam-1")
	time.Sleep(100 * time.Millisecond)
	_ = engine.StopRecording("cam-1")

	query := RecordingQuery{
		CameraID: "cam-1",
		Page:     1,
		PageSize: 10,
	}
	recordings := recordingMgr.SearchRecordings(query)
	assert.True(t, len(recordings) >= 1)
}

// ========== 事件管理测试 ==========

func TestReportEvent(t *testing.T) {
	engine, _, _, _, _ := setupTestEnvironment(t)
	addTestCamera(t, engine, "cam-1", "前门")

	event := &Event{
		CameraID:    "cam-1",
		Type:        DetectionTypeFace,
		Confidence:  0.95,
		Description: "检测到人脸",
	}

	err := engine.ReportEvent(event)
	require.NoError(t, err)
	assert.NotEmpty(t, event.ID)
}

func TestGetEvents(t *testing.T) {
	engine, _, _, _, _ := setupTestEnvironment(t)
	addTestCamera(t, engine, "cam-1", "前门")

	_ = engine.ReportEvent(&Event{
		CameraID:   "cam-1",
		Type:       DetectionTypeFace,
		Confidence: 0.9,
	})
	_ = engine.ReportEvent(&Event{
		CameraID:   "cam-1",
		Type:       DetectionTypeObject,
		Confidence: 0.85,
	})

	query := EventQuery{
		CameraID: "cam-1",
		Page:     1,
		PageSize: 10,
	}
	events := engine.GetEvents(query)
	assert.Len(t, events, 2)
}

func TestGetEventsByType(t *testing.T) {
	engine, _, _, _, _ := setupTestEnvironment(t)
	addTestCamera(t, engine, "cam-1", "前门")

	_ = engine.ReportEvent(&Event{
		CameraID:   "cam-1",
		Type:       DetectionTypeFace,
		Confidence: 0.9,
	})
	_ = engine.ReportEvent(&Event{
		CameraID:   "cam-1",
		Type:       DetectionTypeObject,
		Confidence: 0.85,
	})

	query := EventQuery{
		CameraID: "cam-1",
		Types:    []DetectionType{DetectionTypeFace},
		Page:     1,
		PageSize: 10,
	}
	events := engine.GetEvents(query)
	assert.Len(t, events, 1)
}

// ========== AI检测测试 ==========

func TestGetModels(t *testing.T) {
	_, detection, _, _, _ := setupTestEnvironment(t)

	models := detection.GetModels()
	assert.True(t, len(models) >= 4) // 默认4个模型
}

func TestGetModel(t *testing.T) {
	_, detection, _, _, _ := setupTestEnvironment(t)

	model, err := detection.GetModel("model-face-recognition")
	require.NoError(t, err)
	assert.Equal(t, "人脸识别模型", model.Name)
}

func TestProcessFrame(t *testing.T) {
	engine, detection, _, _, _ := setupTestEnvironment(t)
	camera := addTestCamera(t, engine, "cam-1", "前门")
	camera.AIEnabled = true
	_ = engine.UpdateCamera(camera)

	frameData := []byte("fake-frame-data")
	result, err := detection.ProcessFrame("cam-1", frameData)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.ProcessMs >= 0)
}

// ========== 区域管理测试 ==========

func TestCreateZone(t *testing.T) {
	engine, _, zoneMgr, _, _ := setupTestEnvironment(t)
	addTestCamera(t, engine, "cam-1", "前门")

	zone := &Zone{
		CameraID: "cam-1",
		Name:     "禁区",
		Type:     ZoneTypeRectangle,
		Points: []Point{
			{X: 0, Y: 0},
			{X: 100, Y: 100},
		},
		DetectionTypes: []DetectionType{DetectionTypeIntrusion},
		AlertLevel:     AlertLevelCritical,
	}

	err := zoneMgr.CreateZone(zone)
	require.NoError(t, err)
	assert.NotEmpty(t, zone.ID)
}

func TestGetCameraZones(t *testing.T) {
	engine, _, zoneMgr, _, _ := setupTestEnvironment(t)
	addTestCamera(t, engine, "cam-1", "前门")

	_ = zoneMgr.CreateZone(&Zone{
		CameraID: "cam-1",
		Name:     "区域1",
		Type:     ZoneTypeRectangle,
		Points:   []Point{{X: 0, Y: 0}, {X: 100, Y: 100}},
	})
	_ = zoneMgr.CreateZone(&Zone{
		CameraID: "cam-1",
		Name:     "区域2",
		Type:     ZoneTypePolygon,
		Points:   []Point{{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 50, Y: 100}},
	})

	zones := zoneMgr.GetCameraZones("cam-1")
	assert.Len(t, zones, 2)
}

func TestCheckIntrusion(t *testing.T) {
	engine, _, zoneMgr, _, _ := setupTestEnvironment(t)
	addTestCamera(t, engine, "cam-1", "前门")

	_ = zoneMgr.CreateZone(&Zone{
		CameraID:       "cam-1",
		Name:           "禁区",
		Type:           ZoneTypeRectangle,
		Points:         []Point{{X: 0, Y: 0}, {X: 100, Y: 100}},
		DetectionTypes: []DetectionType{DetectionTypeIntrusion},
	})

	// 测试区域内点
	triggered := zoneMgr.CheckIntrusion("cam-1", Position{X: 50, Y: 50}, DetectionTypeIntrusion)
	assert.Len(t, triggered, 1)

	// 测试区域外点
	triggered = zoneMgr.CheckIntrusion("cam-1", Position{X: 200, Y: 200}, DetectionTypeIntrusion)
	assert.Len(t, triggered, 0)
}

// ========== 告警管理测试 ==========

func TestCreateAlert(t *testing.T) {
	_, _, _, _, alertMgr := setupTestEnvironment(t)

	alert := &Alert{
		CameraID:    "cam-1",
		Level:       AlertLevelWarning,
		Title:       "测试告警",
		Description: "测试描述",
	}

	err := alertMgr.CreateAlert(alert)
	require.NoError(t, err)
	assert.NotEmpty(t, alert.ID)
	assert.Equal(t, AlertStatusPending, alert.Status)
}

func TestAckAlert(t *testing.T) {
	_, _, _, _, alertMgr := setupTestEnvironment(t)

	alert := &Alert{
		ID:    "alert-1",
		Level: AlertLevelWarning,
		Title: "测试告警",
	}
	_ = alertMgr.CreateAlert(alert)

	err := alertMgr.AckAlert("alert-1", "admin")
	require.NoError(t, err)

	got, _ := alertMgr.GetAlert("alert-1")
	assert.Equal(t, AlertStatusAcked, got.Status)
	assert.Equal(t, "admin", got.AckedBy)
}

func TestResolveAlert(t *testing.T) {
	_, _, _, _, alertMgr := setupTestEnvironment(t)

	alert := &Alert{
		ID:    "alert-1",
		Level: AlertLevelWarning,
		Title: "测试告警",
	}
	_ = alertMgr.CreateAlert(alert)

	err := alertMgr.ResolveAlert("alert-1")
	require.NoError(t, err)

	got, _ := alertMgr.GetAlert("alert-1")
	assert.Equal(t, AlertStatusResolved, got.Status)
}

func TestGetActiveAlerts(t *testing.T) {
	_, _, _, _, alertMgr := setupTestEnvironment(t)

	_ = alertMgr.CreateAlert(&Alert{ID: "a1", Level: AlertLevelWarning, Title: "告警1"})
	_ = alertMgr.CreateAlert(&Alert{ID: "a2", Level: AlertLevelCritical, Title: "告警2"})
	_ = alertMgr.CreateAlert(&Alert{ID: "a3", Level: AlertLevelInfo, Title: "告警3"})

	_ = alertMgr.ResolveAlert("a3")

	active := alertMgr.GetActiveAlerts()
	assert.Len(t, active, 2)
}

func TestQueryAlerts(t *testing.T) {
	_, _, _, _, alertMgr := setupTestEnvironment(t)

	_ = alertMgr.CreateAlert(&Alert{Level: AlertLevelWarning, Title: "警告"})
	_ = alertMgr.CreateAlert(&Alert{Level: AlertLevelCritical, Title: "严重"})
	_ = alertMgr.CreateAlert(&Alert{Level: AlertLevelInfo, Title: "信息"})

	query := AlertQuery{
		Levels:   []AlertLevel{AlertLevelWarning, AlertLevelCritical},
		Page:     1,
		PageSize: 10,
	}
	alerts := alertMgr.QueryAlerts(query)
	assert.Len(t, alerts, 2)
}

func TestGetAlertStats(t *testing.T) {
	_, _, _, _, alertMgr := setupTestEnvironment(t)

	_ = alertMgr.CreateAlert(&Alert{Level: AlertLevelWarning, Title: "告警1"})
	_ = alertMgr.CreateAlert(&Alert{Level: AlertLevelCritical, Title: "告警2"})

	stats := alertMgr.GetAlertStats()
	assert.Equal(t, 2, stats["total"])
}

// ========== 时间线测试 ==========

func TestGetTimeline(t *testing.T) {
	engine, _, _, _, _ := setupTestEnvironment(t)
	addTestCamera(t, engine, "cam-1", "前门")

	// 添加录像
	_ = engine.StartRecording("cam-1")
	time.Sleep(100 * time.Millisecond)
	_ = engine.StopRecording("cam-1")

	// 添加事件
	_ = engine.ReportEvent(&Event{
		CameraID:   "cam-1",
		Type:       DetectionTypeMotion,
		Confidence: 0.9,
	})

	timeline, err := engine.GetTimeline("cam-1", time.Now())
	require.NoError(t, err)
	assert.NotNil(t, timeline)
	assert.Equal(t, "cam-1", timeline.CameraID)
}

// ========== 系统状态测试 ==========

func TestGetSystemStatus(t *testing.T) {
	engine, _, _, _, _ := setupTestEnvironment(t)

	addTestCamera(t, engine, "cam-1", "前门")
	addTestCamera(t, engine, "cam-2", "后门")

	_ = engine.UpdateCameraStatus("cam-1", CameraStatusOnline)
	_ = engine.UpdateCameraStatus("cam-2", CameraStatusOnline)

	status := engine.GetSystemStatus()
	assert.Equal(t, 2, status.TotalCameras)
	assert.Equal(t, 2, status.OnlineCameras)
}
