package dlnamedia

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRouter() (*gin.Engine, *Manager, *Handler) {
	gin.SetMode(gin.TestMode)

	manager := NewManager("/tmp/test-media", false)
	handler := NewHandler(manager)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	return router, manager, handler
}

func TestListDevices(t *testing.T) {
	router, manager, _ := setupTestRouter()

	// 添加测试设备
	manager.mu.Lock()
	manager.devices["device-1"] = &DLNADevice{
		ID:           "device-1",
		FriendlyName: "Test TV",
		DeviceType:   DeviceTypeRenderer,
		IPAddress:    "192.168.1.100",
		IsOnline:     true,
	}
	manager.mu.Unlock()

	req := httptest.NewRequest("GET", "/api/v1/dlna/devices", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, float64(1), resp["total"])
}

func TestGetDevice(t *testing.T) {
	router, manager, _ := setupTestRouter()

	// 添加测试设备
	manager.mu.Lock()
	manager.devices["device-1"] = &DLNADevice{
		ID:           "device-1",
		FriendlyName: "Test TV",
		DeviceType:   DeviceTypeRenderer,
		IPAddress:    "192.168.1.100",
		IsOnline:     true,
	}
	manager.mu.Unlock()

	// 测试获取存在的设备
	req := httptest.NewRequest("GET", "/api/v1/dlna/devices/device-1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var device DLNADevice
	err := json.Unmarshal(w.Body.Bytes(), &device)
	require.NoError(t, err)
	assert.Equal(t, "Test TV", device.FriendlyName)
}

func TestGetDeviceNotFound(t *testing.T) {
	router, _, _ := setupTestRouter()

	req := httptest.NewRequest("GET", "/api/v1/dlna/devices/nonexistent", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCreateLibrary(t *testing.T) {
	router, _, _ := setupTestRouter()

	reqBody := CreateLibraryRequest{
		Name:         "Test Library",
		Path:         "/tmp",
		MediaType:    MediaTypeVideo,
		Recursive:    true,
		AutoScan:     true,
		ScanInterval: 60,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/dlna/libraries", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var lib MediaLibrary
	err := json.Unmarshal(w.Body.Bytes(), &lib)
	require.NoError(t, err)
	assert.Equal(t, "Test Library", lib.Name)
	assert.Equal(t, "/tmp", lib.Path)
}

func TestListLibraries(t *testing.T) {
	router, manager, _ := setupTestRouter()

	// 添加测试库
	manager.mu.Lock()
	manager.libraries["lib-1"] = &MediaLibrary{
		ID:        "lib-1",
		Name:      "Movies",
		Path:      "/movies",
		MediaType: MediaTypeVideo,
	}
	manager.mu.Unlock()

	req := httptest.NewRequest("GET", "/api/v1/dlna/libraries", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(1), resp["total"])
}

func TestGetLibrary(t *testing.T) {
	router, manager, _ := setupTestRouter()

	// 添加测试库
	manager.mu.Lock()
	manager.libraries["lib-1"] = &MediaLibrary{
		ID:        "lib-1",
		Name:      "Movies",
		Path:      "/movies",
		MediaType: MediaTypeVideo,
	}
	manager.mu.Unlock()

	req := httptest.NewRequest("GET", "/api/v1/dlna/libraries/lib-1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var lib MediaLibrary
	err := json.Unmarshal(w.Body.Bytes(), &lib)
	require.NoError(t, err)
	assert.Equal(t, "Movies", lib.Name)
}

func TestUpdateLibrary(t *testing.T) {
	router, manager, _ := setupTestRouter()

	// 添加测试库
	manager.mu.Lock()
	manager.libraries["lib-1"] = &MediaLibrary{
		ID:        "lib-1",
		Name:      "Movies",
		Path:      "/movies",
		MediaType: MediaTypeVideo,
	}
	manager.mu.Unlock()

	newName := "Updated Movies"
	reqBody := UpdateLibraryRequest{
		Name: &newName,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("PUT", "/api/v1/dlna/libraries/lib-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var lib MediaLibrary
	err := json.Unmarshal(w.Body.Bytes(), &lib)
	require.NoError(t, err)
	assert.Equal(t, "Updated Movies", lib.Name)
}

func TestDeleteLibrary(t *testing.T) {
	router, manager, _ := setupTestRouter()

	// 添加测试库
	manager.mu.Lock()
	manager.libraries["lib-1"] = &MediaLibrary{
		ID:        "lib-1",
		Name:      "Movies",
		Path:      "/movies",
		MediaType: MediaTypeVideo,
	}
	manager.mu.Unlock()

	req := httptest.NewRequest("DELETE", "/api/v1/dlna/libraries/lib-1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证已删除
	req = httptest.NewRequest("GET", "/api/v1/dlna/libraries/lib-1", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSearchMedia(t *testing.T) {
	router, manager, _ := setupTestRouter()

	// 添加测试媒体
	manager.mu.Lock()
	manager.mediaItems["media-1"] = &MediaItem{
		ID:        "media-1",
		Title:     "Test Video.mp4",
		FilePath:  "/media/test.mp4",
		MediaType: MediaTypeVideo,
		LibraryID: "lib-1",
	}
	manager.mediaItems["media-2"] = &MediaItem{
		ID:        "media-2",
		Title:     "Test Audio.mp3",
		FilePath:  "/media/test.mp3",
		MediaType: MediaTypeAudio,
		LibraryID: "lib-1",
	}
	manager.mu.Unlock()

	// 测试搜索
	req := httptest.NewRequest("GET", "/api/v1/dlna/media/search?q=Test&type=video", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(1), resp["total"])
}

func TestGetMediaItem(t *testing.T) {
	router, manager, _ := setupTestRouter()

	// 添加测试媒体
	manager.mu.Lock()
	manager.mediaItems["media-1"] = &MediaItem{
		ID:        "media-1",
		Title:     "Test Video.mp4",
		FilePath:  "/media/test.mp4",
		MediaType: MediaTypeVideo,
	}
	manager.mu.Unlock()

	req := httptest.NewRequest("GET", "/api/v1/dlna/media/media-1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var item MediaItem
	err := json.Unmarshal(w.Body.Bytes(), &item)
	require.NoError(t, err)
	assert.Equal(t, "Test Video.mp4", item.Title)
}

func TestPushMedia(t *testing.T) {
	router, manager, _ := setupTestRouter()

	// 添加测试设备和媒体
	manager.mu.Lock()
	manager.devices["device-1"] = &DLNADevice{
		ID:           "device-1",
		FriendlyName: "Test TV",
		IsOnline:     true,
	}
	manager.mediaItems["media-1"] = &MediaItem{
		ID:       "media-1",
		Title:    "Test Video.mp4",
		Duration: 3600,
	}
	manager.mu.Unlock()

	reqBody := PushMediaRequest{
		DeviceID: "device-1",
		MediaID:  "media-1",
		Position: 0,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/dlna/playback/push", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var session PlaybackSession
	err := json.Unmarshal(w.Body.Bytes(), &session)
	require.NoError(t, err)
	assert.Equal(t, PlayStatePlaying, session.State)
	assert.Equal(t, "device-1", session.DeviceID)
}

func TestPushMediaDeviceNotFound(t *testing.T) {
	router, manager, _ := setupTestRouter()

	// 添加测试媒体
	manager.mu.Lock()
	manager.mediaItems["media-1"] = &MediaItem{
		ID:       "media-1",
		Title:    "Test Video.mp4",
		Duration: 3600,
	}
	manager.mu.Unlock()

	reqBody := PushMediaRequest{
		DeviceID: "nonexistent",
		MediaID:  "media-1",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/dlna/playback/push", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPushMediaOfflineDevice(t *testing.T) {
	router, manager, _ := setupTestRouter()

	// 添加离线设备
	manager.mu.Lock()
	manager.devices["device-1"] = &DLNADevice{
		ID:           "device-1",
		FriendlyName: "Test TV",
		IsOnline:     false,
	}
	manager.mediaItems["media-1"] = &MediaItem{
		ID:       "media-1",
		Title:    "Test Video.mp4",
		Duration: 3600,
	}
	manager.mu.Unlock()

	reqBody := PushMediaRequest{
		DeviceID: "device-1",
		MediaID:  "media-1",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/dlna/playback/push", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestControlPlayback(t *testing.T) {
	router, manager, _ := setupTestRouter()

	// 添加测试会话
	manager.mu.Lock()
	manager.sessions["session-1"] = &PlaybackSession{
		ID:       "session-1",
		DeviceID: "device-1",
		State:    PlayStatePlaying,
		Position: 100,
		Duration: 3600,
		Volume:   50,
	}
	manager.mu.Unlock()

	// 测试暂停
	reqBody := ControlPlaybackRequest{
		Action: "pause",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/dlna/playback/session-1/control", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var session PlaybackSession
	err := json.Unmarshal(w.Body.Bytes(), &session)
	require.NoError(t, err)
	assert.Equal(t, PlayStatePaused, session.State)
}

func TestControlPlaybackSeek(t *testing.T) {
	router, manager, _ := setupTestRouter()

	// 添加测试会话
	manager.mu.Lock()
	manager.sessions["session-1"] = &PlaybackSession{
		ID:       "session-1",
		DeviceID: "device-1",
		State:    PlayStatePlaying,
		Position: 100,
		Duration: 3600,
		Volume:   50,
	}
	manager.mu.Unlock()

	// 测试 seek
	reqBody := ControlPlaybackRequest{
		Action:   "seek",
		Position: 500,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/dlna/playback/session-1/control", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var session PlaybackSession
	err := json.Unmarshal(w.Body.Bytes(), &session)
	require.NoError(t, err)
	assert.Equal(t, int64(500), session.Position)
}

func TestSetVolume(t *testing.T) {
	router, manager, _ := setupTestRouter()

	// 添加测试会话
	manager.mu.Lock()
	manager.sessions["session-1"] = &PlaybackSession{
		ID:       "session-1",
		DeviceID: "device-1",
		State:    PlayStatePlaying,
		Volume:   50,
	}
	manager.mu.Unlock()

	reqBody := SetVolumeRequest{
		Level: 75,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/dlna/playback/session-1/volume", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSetVolumeInvalid(t *testing.T) {
	router, manager, _ := setupTestRouter()

	// 添加测试会话
	manager.mu.Lock()
	manager.sessions["session-1"] = &PlaybackSession{
		ID:       "session-1",
		DeviceID: "device-1",
		State:    PlayStatePlaying,
		Volume:   50,
	}
	manager.mu.Unlock()

	reqBody := SetVolumeRequest{
		Level: 150, // 超出范围
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/dlna/playback/session-1/volume", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestStopSession(t *testing.T) {
	router, manager, _ := setupTestRouter()

	// 添加测试会话
	manager.mu.Lock()
	manager.sessions["session-1"] = &PlaybackSession{
		ID:       "session-1",
		DeviceID: "device-1",
		State:    PlayStatePlaying,
	}
	manager.mu.Unlock()

	req := httptest.NewRequest("DELETE", "/api/v1/dlna/playback/session-1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证会话已删除
	req = httptest.NewRequest("GET", "/api/v1/dlna/playback/session-1", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListSessions(t *testing.T) {
	router, manager, _ := setupTestRouter()

	// 添加测试会话
	manager.mu.Lock()
	manager.sessions["session-1"] = &PlaybackSession{
		ID:       "session-1",
		DeviceID: "device-1",
		State:    PlayStatePlaying,
	}
	manager.sessions["session-2"] = &PlaybackSession{
		ID:       "session-2",
		DeviceID: "device-2",
		State:    PlayStatePaused,
	}
	manager.mu.Unlock()

	req := httptest.NewRequest("GET", "/api/v1/dlna/playback/sessions", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(2), resp["total"])
}

func TestCreateGroup(t *testing.T) {
	router, manager, _ := setupTestRouter()

	// 添加测试设备
	manager.mu.Lock()
	manager.devices["device-1"] = &DLNADevice{
		ID:           "device-1",
		FriendlyName: "Living Room",
		IsOnline:     true,
	}
	manager.devices["device-2"] = &DLNADevice{
		ID:           "device-2",
		FriendlyName: "Bedroom",
		IsOnline:     true,
	}
	manager.mu.Unlock()

	reqBody := CreateGroupRequest{
		Name:      "Multi-Room Audio",
		DeviceIDs: []string{"device-1", "device-2"},
		IsSync:    true,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/dlna/groups", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var group DeviceGroup
	err := json.Unmarshal(w.Body.Bytes(), &group)
	require.NoError(t, err)
	assert.Equal(t, "Multi-Room Audio", group.Name)
	assert.Equal(t, 2, len(group.DeviceIDs))
	assert.True(t, group.IsSync)
}

func TestCreateGroupDeviceNotFound(t *testing.T) {
	router, _, _ := setupTestRouter()

	reqBody := CreateGroupRequest{
		Name:      "Test Group",
		DeviceIDs: []string{"nonexistent"},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/dlna/groups", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListGroups(t *testing.T) {
	router, manager, _ := setupTestRouter()

	// 添加测试分组
	manager.mu.Lock()
	manager.deviceGroup["group-1"] = &DeviceGroup{
		ID:        "group-1",
		Name:      "Living Room",
		DeviceIDs: []string{"device-1"},
		IsSync:    true,
		Volume:    50,
	}
	manager.mu.Unlock()

	req := httptest.NewRequest("GET", "/api/v1/dlna/groups", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(1), resp["total"])
}

func TestGetGroup(t *testing.T) {
	router, manager, _ := setupTestRouter()

	// 添加测试分组
	manager.mu.Lock()
	manager.deviceGroup["group-1"] = &DeviceGroup{
		ID:        "group-1",
		Name:      "Living Room",
		DeviceIDs: []string{"device-1"},
		IsSync:    true,
		Volume:    50,
	}
	manager.mu.Unlock()

	req := httptest.NewRequest("GET", "/api/v1/dlna/groups/group-1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var group DeviceGroup
	err := json.Unmarshal(w.Body.Bytes(), &group)
	require.NoError(t, err)
	assert.Equal(t, "Living Room", group.Name)
}

func TestUpdateGroup(t *testing.T) {
	router, manager, _ := setupTestRouter()

	// 添加测试分组
	manager.mu.Lock()
	manager.deviceGroup["group-1"] = &DeviceGroup{
		ID:        "group-1",
		Name:      "Living Room",
		DeviceIDs: []string{"device-1"},
		IsSync:    true,
		Volume:    50,
	}
	manager.mu.Unlock()

	newName := "Updated Group"
	newVolume := 75
	reqBody := UpdateGroupRequest{
		Name:   &newName,
		Volume: &newVolume,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("PUT", "/api/v1/dlna/groups/group-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var group DeviceGroup
	err := json.Unmarshal(w.Body.Bytes(), &group)
	require.NoError(t, err)
	assert.Equal(t, "Updated Group", group.Name)
	assert.Equal(t, 75, group.Volume)
}

func TestDeleteGroup(t *testing.T) {
	router, manager, _ := setupTestRouter()

	// 添加测试分组
	manager.mu.Lock()
	manager.deviceGroup["group-1"] = &DeviceGroup{
		ID:        "group-1",
		Name:      "Living Room",
		DeviceIDs: []string{"device-1"},
	}
	manager.mu.Unlock()

	req := httptest.NewRequest("DELETE", "/api/v1/dlna/groups/group-1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证已删除
	req = httptest.NewRequest("GET", "/api/v1/dlna/groups/group-1", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestManageQueue(t *testing.T) {
	router, manager, _ := setupTestRouter()

	// 添加测试媒体
	manager.mu.Lock()
	manager.mediaItems["media-1"] = &MediaItem{
		ID:    "media-1",
		Title: "Test Video 1.mp4",
	}
	manager.mediaItems["media-2"] = &MediaItem{
		ID:    "media-2",
		Title: "Test Video 2.mp4",
	}
	manager.mu.Unlock()

	// 添加到队列
	reqBody := ManageQueueRequest{
		Action:   "add",
		MediaIDs: []string{"media-1", "media-2"},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/dlna/queues/device-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var queue PlayQueue
	err := json.Unmarshal(w.Body.Bytes(), &queue)
	require.NoError(t, err)
	assert.Equal(t, 2, len(queue.Items))
}

func TestGetQueue(t *testing.T) {
	router, manager, _ := setupTestRouter()

	// 添加测试队列
	manager.mu.Lock()
	manager.queues["device-1"] = &PlayQueue{
		ID:       "queue-1",
		DeviceID: "device-1",
		Items: []QueueItem{
			{Index: 0, MediaID: "media-1"},
			{Index: 1, MediaID: "media-2"},
		},
		CurrentIndex: 0,
	}
	manager.mu.Unlock()

	req := httptest.NewRequest("GET", "/api/v1/dlna/queues/device-1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var queue PlayQueue
	err := json.Unmarshal(w.Body.Bytes(), &queue)
	require.NoError(t, err)
	assert.Equal(t, 2, len(queue.Items))
}

func TestGetContentDirectory(t *testing.T) {
	router, manager, _ := setupTestRouter()

	// 添加测试库和媒体
	manager.mu.Lock()
	manager.libraries["lib-1"] = &MediaLibrary{
		ID:        "lib-1",
		Name:      "Movies",
		Path:      "/movies",
		MediaType: MediaTypeVideo,
		ItemCount: 1,
	}
	manager.mediaItems["media-1"] = &MediaItem{
		ID:        "media-1",
		Title:     "Test Video.mp4",
		LibraryID: "lib-1",
	}
	manager.mu.Unlock()

	// 获取根目录
	req := httptest.NewRequest("GET", "/api/v1/dlna/content-directory", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(1), resp["total"])

	// 获取库目录
	req = httptest.NewRequest("GET", "/api/v1/dlna/content-directory?parent_id=lib-1", nil)
	w = httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(1), resp["total"])
}
