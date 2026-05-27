package surveillance

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupTest(t *testing.T) (*SurveillanceManager, *Handler) {
	logger, _ zap.NewDevelopment()
	manager := NewSurveillanceManager(logger, "/tmp/surveillance-test")
	handler := NewHandler(manager, logger)
	return manager, handler
}

func TestAddCamera(t *testing.T) {
	manager, _ := setupTest(t)

	cam := &Camera{
		Name:     "前门摄像头",
		Protocol: "rtsp",
		URL:      "rtsp://192.168.1.100:554/stream1",
		Location: "前门",
	}

	err := manager.AddCamera(nil, cam)
	require.NoError(t, err)
	assert.NotEmpty(t, cam.ID)
	assert.Equal(t, "offline", cam.Status)
}

func TestAddDuplicateCamera(t *testing.T) {
	manager, _ := setupTest(t)

	cam := &Camera{Name: "test", Protocol: "rtsp", URL: "rtsp://test"}
	err := manager.AddCamera(nil, cam)
	require.NoError(t, err)

	// 同ID再添加应失败
	err = manager.AddCamera(nil, cam)
	assert.Error(t, err)
}

func TestGetCamera(t *testing.T) {
	manager, _ := setupTest(t)

	cam := &Camera{Name: "test", Protocol: "rtsp", URL: "rtsp://test"}
	_ = manager.AddCamera(nil, cam)

	got, err := manager.GetCamera(nil, cam.ID)
	require.NoError(t, err)
	assert.Equal(t, cam.Name, got.Name)
}

func TestGetCameraNotFound(t *testing.T) {
	manager, _ := setupTest(t)

	_, err := manager.GetCamera(nil, "nonexistent")
	assert.Error(t, err)
}

func TestListCameras(t *testing.T) {
	manager, _ := setupTest(t)

	_ = manager.AddCamera(nil, &Camera{Name: "cam1", Protocol: "rtsp", URL: "rtsp://1"})
	_ = manager.AddCamera(nil, &Camera{Name: "cam2", Protocol: "rtsp", URL: "rtsp://2"})

	cameras := manager.ListCameras(nil)
	assert.Len(t, cameras, 2)
}

func TestRemoveCamera(t *testing.T) {
	manager, _ := setupTest(t)

	cam := &Camera{Name: "test", Protocol: "rtsp", URL: "rtsp://test"}
	_ = manager.AddCamera(nil, cam)

	err := manager.RemoveCamera(nil, cam.ID)
	require.NoError(t, err)

	_, err = manager.GetCamera(nil, cam.ID)
	assert.Error(t, err)
}

func TestStartStopRecording(t *testing.T) {
	manager, _ := setupTest(t)

	cam := &Camera{Name: "test", Protocol: "rtsp", URL: "rtsp://test"}
	_ = manager.AddCamera(nil, cam)

	err := manager.StartRecording(nil, cam.ID)
	require.NoError(t, err)

	updated, _ := manager.GetCamera(nil, cam.ID)
	assert.Equal(t, "recording", updated.Status)

	err = manager.StopRecording(nil, cam.ID)
	require.NoError(t, err)

	updated, _ = manager.GetCamera(nil, cam.ID)
	assert.Equal(t, "online", updated.Status)
}

func TestReportMotion(t *testing.T) {
	manager, _ := setupTest(t)

	cam := &Camera{Name: "test", Protocol: "rtsp", URL: "rtsp://test"}
	_ = manager.AddCamera(nil, cam)

	event := &MotionEvent{
		CameraID:   cam.ID,
		Confidence: 0.95,
		Region:     "center",
	}

	err := manager.ReportMotion(nil, event)
	require.NoError(t, err)
	assert.NotEmpty(t, event.ID)
}

func TestGetStorageQuota(t *testing.T) {
	manager, _ := setupTest(t)

	cam := &Camera{Name: "test", Protocol: "rtsp", URL: "rtsp://test"}
	_ = manager.AddCamera(nil, cam)

	quota, err := manager.GetStorageQuota(nil, cam.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(100*1024*1024*1024), quota.TotalBytes)
}

func TestGetTimeline(t *testing.T) {
	manager, _ := setupTest(t)

	cam := &Camera{Name: "test", Protocol: "rtsp", URL: "rtsp://test"}
	_ = manager.AddCamera(nil, cam)

	timeline := manager.GetTimeline(nil, cam.ID, time.Now())
	assert.NotNil(t, timeline)
	assert.Equal(t, cam.ID, timeline["camera_id"])
}
