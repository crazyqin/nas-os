package surveillance

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTest(t *testing.T) (*Manager, *Handlers) {
	manager, err := NewManager("/tmp/surveillance-test")
	require.NoError(t, err)
	handler := NewHandlers(manager)
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

	err := manager.AddCamera(cam)
	require.NoError(t, err)
	assert.NotEmpty(t, cam.ID)
	assert.Equal(t, "offline", cam.Status)
}

func TestAddDuplicateCamera(t *testing.T) {
	manager, _ := setupTest(t)

	cam := &Camera{Name: "test", Protocol: "rtsp", URL: "rtsp://test"}
	err := manager.AddCamera(cam)
	require.NoError(t, err)

	// 同ID再添加应失败
	err = manager.AddCamera(cam)
	assert.Error(t, err)
}

func TestGetCamera(t *testing.T) {
	manager, _ := setupTest(t)

	cam := &Camera{Name: "test", Protocol: "rtsp", URL: "rtsp://test"}
	_ = manager.AddCamera(cam)

	got, err := manager.GetCamera(cam.ID)
	require.NoError(t, err)
	assert.Equal(t, cam.Name, got.Name)
}

func TestGetCameraNotFound(t *testing.T) {
	manager, _ := setupTest(t)

	_, err := manager.GetCamera("nonexistent")
	assert.Error(t, err)
}

func TestListCameras(t *testing.T) {
	manager, _ := setupTest(t)

	_ = manager.AddCamera(&Camera{Name: "cam1", Protocol: "rtsp", URL: "rtsp://1"})
	_ = manager.AddCamera(&Camera{Name: "cam2", Protocol: "rtsp", URL: "rtsp://2"})

	cameras := manager.ListCameras()
	assert.Len(t, cameras, 2)
}

func TestRemoveCamera(t *testing.T) {
	manager, _ := setupTest(t)

	cam := &Camera{Name: "test", Protocol: "rtsp", URL: "rtsp://test"}
	_ = manager.AddCamera(cam)

	err := manager.DeleteCamera(cam.ID)
	require.NoError(t, err)

	_, err = manager.GetCamera(cam.ID)
	assert.Error(t, err)
}

func TestStartStopRecording(t *testing.T) {
	manager, _ := setupTest(t)

	cam := &Camera{Name: "test", Protocol: "rtsp", URL: "rtsp://test"}
	_ = manager.AddCamera(cam)

	job, err := manager.StartRecording(cam.ID, RecordingModeContinuous)
	require.NoError(t, err)
	assert.NotEmpty(t, job.ID)

	err = manager.StopRecording(job.ID)
	require.NoError(t, err)
}

func TestReportMotion(t *testing.T) {
	manager, _ := setupTest(t)

	cam := &Camera{Name: "test", Protocol: "rtsp", URL: "rtsp://test"}
	_ = manager.AddCamera(cam)

	event := &MotionEvent{
		CameraID:   cam.ID,
		Confidence: 0.95,
		Region:     "center",
	}

	err := manager.ReportMotion(event)
	require.NoError(t, err)
	assert.NotEmpty(t, event.ID)
}

func TestGetStorageQuota(t *testing.T) {
	manager, _ := setupTest(t)

	cam := &Camera{Name: "test", Protocol: "rtsp", URL: "rtsp://test"}
	_ = manager.AddCamera(cam)

	quota, err := manager.GetStorageQuota(cam.ID)
	require.NoError(t, err)
	assert.NotNil(t, quota)
}

func TestGetTimeline(t *testing.T) {
	manager, _ := setupTest(t)

	cam := &Camera{Name: "test", Protocol: "rtsp", URL: "rtsp://test"}
	_ = manager.AddCamera(cam)

	timeline := manager.GetTimeline(cam.ID, time.Now())
	assert.NotNil(t, timeline)
	assert.Equal(t, cam.ID, timeline["camera_id"])
}
