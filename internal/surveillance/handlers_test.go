package surveillance

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewManager(t *testing.T) {
	manager := NewManager()
	assert.NotNil(t, manager)
}

func TestAddCamera(t *testing.T) {
	manager := NewManager()
	
	cam := &Camera{
		ID:           "cam-001",
		Name:         "Test Camera",
		Protocol:     ProtocolRTSP,
		Host:         "192.168.1.100",
		Port:         554,
		StreamPath:   "/stream1",
		Resolution:   "1920x1080",
		FPS:          30,
		Enabled:      true,
	}
	
	err := manager.AddCamera(cam)
	assert.NoError(t, err)
	
	// 验证可以获取
	got, err := manager.GetCamera(cam.ID)
	assert.NoError(t, err)
	assert.Equal(t, cam.Name, got.Name)
}

func TestRemoveCamera(t *testing.T) {
	manager := NewManager()
	
	cam := &Camera{
		ID:         "cam-002",
		Name:       "Test Camera",
		Protocol:   ProtocolRTSP,
		Host:       "192.168.1.100",
		Port:       554,
		StreamPath: "/stream1",
		Enabled:    true,
	}
	
	manager.AddCamera(cam)
	
	err := manager.RemoveCamera(cam.ID)
	assert.NoError(t, err)
	
	_, err = manager.GetCamera(cam.ID)
	assert.Error(t, err)
}

func TestListCameras(t *testing.T) {
	manager := NewManager()
	
	// 添加多个摄像头
	for i := 0; i < 3; i++ {
		cam := &Camera{
			ID:         fmt.Sprintf("cam-%d", i),
			Name:       fmt.Sprintf("Camera %d", i),
			Protocol:   ProtocolRTSP,
			Host:       "192.168.1.100",
			Port:       554,
			StreamPath: "/stream",
			Enabled:    true,
		}
		manager.AddCamera(cam)
	}
	
	cameras := manager.ListCameras()
	assert.Equal(t, 3, len(cameras))
}

func TestStartRecording(t *testing.T) {
	manager := NewManager()
	
	cam := &Camera{
		ID:         "cam-003",
		Name:       "Test Camera",
		Protocol:   ProtocolRTSP,
		Host:       "192.168.1.100",
		Port:       554,
		StreamPath: "/stream1",
		Enabled:    true,
	}
	manager.AddCamera(cam)
	
	recording, err := manager.StartRecording(cam.ID, RecordingModeContinuous)
	assert.NoError(t, err)
	assert.NotNil(t, recording)
	assert.Equal(t, cam.ID, recording.CameraID)
}

func TestAddSchedule(t *testing.T) {
	manager := NewManager()
	
	cam := &Camera{
		ID:         "cam-004",
		Name:       "Test Camera",
		Protocol:   ProtocolRTSP,
		Host:       "192.168.1.100",
		Port:       554,
		StreamPath: "/stream1",
		Enabled:    true,
	}
	manager.AddCamera(cam)
	
	schedule := &RecordingSchedule{
		ID:       "sched-001",
		CameraID: cam.ID,
		Mode:     RecordingModeSchedule,
		Days:     []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday}, // 周一到周五
		Start:    "09:00",
		End:      "18:00",
		Enabled:  true,
	}
	
	err := manager.AddSchedule(schedule)
	assert.NoError(t, err)
	assert.NotEmpty(t, schedule.ID)
}

func TestSetMotionDetection(t *testing.T) {
	manager := NewManager()
	
	cam := &Camera{
		ID:         "cam-005",
		Name:       "Test Camera",
		Protocol:   ProtocolRTSP,
		Host:       "192.168.1.100",
		Port:       554,
		StreamPath: "/stream1",
		Enabled:    true,
	}
	manager.AddCamera(cam)
	
	config := &MotionDetection{
		CameraID:    cam.ID,
		Enabled:     true,
		Sensitivity: SensitivityHigh,
		Regions: []MotionRegion{
			{ID: "r1", Name: "Region 1", X: 0, Y: 0, Width: 100, Height: 100},
		},
	}
	
	err := manager.SetMotionDetection(config)
	assert.NoError(t, err)
	
	got, err := manager.GetMotionDetection(cam.ID)
	assert.NoError(t, err)
	assert.Equal(t, config.Enabled, got.Enabled)
}

func TestGetEvents(t *testing.T) {
	manager := NewManager()
	
	cam := &Camera{
		ID:         "cam-006",
		Name:       "Test Camera",
		Protocol:   ProtocolRTSP,
		Host:       "192.168.1.100",
		Port:       554,
		StreamPath: "/stream1",
		Enabled:    true,
	}
	manager.AddCamera(cam)
	
	// 获取事件列表（应该为空或nil）
	events := manager.GetEvents(cam.ID, 10)
	assert.Equal(t, 0, len(events))
}

func TestAddActionRule(t *testing.T) {
	manager := NewManager()
	
	cam := &Camera{
		ID:         "cam-007",
		Name:       "Test Camera",
		Protocol:   ProtocolRTSP,
		Host:       "192.168.1.100",
		Port:       554,
		StreamPath: "/stream1",
		Enabled:    true,
	}
	manager.AddCamera(cam)
	
	rule := &ActionRule{
		ID:        "rule-001",
		CameraID:  cam.ID,
		EventType: EventMotionDetection,
		Actions:   []ActionTrigger{ActionRecord, ActionNotify},
		Enabled:   true,
	}
	
	err := manager.AddActionRule(rule)
	assert.NoError(t, err)
	assert.NotEmpty(t, rule.ID)
}

func TestCreateGroup(t *testing.T) {
	manager := NewManager()
	
	group := &CameraGroup{
		ID:          "group-001",
		Name:        "Office Cameras",
		Description: "Office area cameras",
		CameraIDs:   []string{},
		Layout: Layout{
			Rows:    2,
			Columns: 2,
		},
	}
	
	err := manager.CreateGroup(group)
	assert.NoError(t, err)
	assert.NotEmpty(t, group.ID)
}

func TestHandlerRegisterRoutes(t *testing.T) {
	manager := NewManager()
	handler := NewHandler(manager)
	assert.NotNil(t, handler)
}

func TestAddDuplicateCamera(t *testing.T) {
	manager := NewManager()
	
	cam1 := &Camera{
		ID:         "cam-008",
		Name:       "Camera 1",
		Protocol:   ProtocolRTSP,
		Host:       "192.168.1.100",
		Port:       554,
		StreamPath: "/stream1",
		Enabled:    true,
	}
	
	cam2 := &Camera{
		ID:         "cam-009",
		Name:       "Camera 2",
		Protocol:   ProtocolRTSP,
		Host:       "192.168.1.100",
		Port:       554,
		StreamPath: "/stream1",
		Enabled:    true,
	}
	
	err := manager.AddCamera(cam1)
	assert.NoError(t, err)
	
	// 不同 ID 可以添加
	err = manager.AddCamera(cam2)
	assert.NoError(t, err)
}

func TestUpdateCamera(t *testing.T) {
	manager := NewManager()
	
	cam := &Camera{
		ID:         "cam-010",
		Name:       "Original Name",
		Protocol:   ProtocolRTSP,
		Host:       "192.168.1.100",
		Port:       554,
		StreamPath: "/stream1",
		Enabled:    true,
	}
	manager.AddCamera(cam)
	
	// 更新名称
	cam.Name = "Updated Name"
	err := manager.UpdateCamera(cam)
	assert.NoError(t, err)
	
	got, _ := manager.GetCamera(cam.ID)
	assert.Equal(t, "Updated Name", got.Name)
}
