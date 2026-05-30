package arvrmedia

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func newTestManager(t *testing.T) (*Manager, string) {
	t.Helper()
	dir := t.TempDir()
	return NewManager(dir), dir
}

func createTestFile(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// ==================== 全景媒体测试 ====================

func TestCreatePanorama(t *testing.T) {
	m, _ := newTestManager(t)

	req := &PanoramaMedia{
		Name:       "Test Panorama",
		Path:       "/test/panorama.jpg",
		Projection: ProjectionEquirectangular,
		Width:      8192,
		Height:     4096,
	}

	media, err := m.CreatePanorama(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if media.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if media.Name != "Test Panorama" {
		t.Errorf("expected name 'Test Panorama', got %q", media.Name)
	}
	if media.Projection != ProjectionEquirectangular {
		t.Errorf("expected projection equirectangular, got %q", media.Projection)
	}
}

func TestCreatePanoramaEmptyName(t *testing.T) {
	m, _ := newTestManager(t)

	_, err := m.CreatePanorama(&PanoramaMedia{})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestGetPanorama(t *testing.T) {
	m, _ := newTestManager(t)

	media, _ := m.CreatePanorama(&PanoramaMedia{Name: "Test"})

	got, ok := m.GetPanorama(media.ID)
	if !ok {
		t.Fatal("expected to find panorama")
	}
	if got.Name != "Test" {
		t.Errorf("expected name 'Test', got %q", got.Name)
	}

	_, ok = m.GetPanorama("nonexistent")
	if ok {
		t.Fatal("expected not to find nonexistent panorama")
	}
}

func TestUpdatePanorama(t *testing.T) {
	m, _ := newTestManager(t)

	media, _ := m.CreatePanorama(&PanoramaMedia{Name: "Old Name"})

	updated, err := m.UpdatePanorama(media.ID, map[string]interface{}{
		"name":        "New Name",
		"description": "Updated description",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Name != "New Name" {
		t.Errorf("expected name 'New Name', got %q", updated.Name)
	}

	_, err = m.UpdatePanorama("nonexistent", map[string]interface{}{"name": "x"})
	if err == nil {
		t.Fatal("expected error for nonexistent panorama")
	}
}

func TestDeletePanorama(t *testing.T) {
	m, dir := newTestManager(t)

	path := createTestFile(t, dir, "pano.jpg")
	media, _ := m.CreatePanorama(&PanoramaMedia{Name: "Delete Me", Path: path})

	if err := m.DeletePanorama(media.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, ok := m.GetPanorama(media.ID)
	if ok {
		t.Fatal("expected panorama to be deleted")
	}

	if err := m.DeletePanorama("nonexistent"); err == nil {
		t.Fatal("expected error for nonexistent panorama")
	}
}

func TestListPanoramas(t *testing.T) {
	m, _ := newTestManager(t)

	m.CreatePanorama(&PanoramaMedia{Name: "Photo 1", IsVideo: false})
	m.CreatePanorama(&PanoramaMedia{Name: "Photo 2", IsVideo: false})
	m.CreatePanorama(&PanoramaMedia{Name: "Video 1", IsVideo: true})

	// 列出所有
	all, total := m.ListPanoramas("", 1, 10)
	if total != 3 {
		t.Errorf("expected 3 total, got %d", total)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 items, got %d", len(all))
	}

	// 只列出照片
	photos, total := m.ListPanoramas("photo", 1, 10)
	if total != 2 {
		t.Errorf("expected 2 photos, got %d", total)
	}
	if len(photos) != 2 {
		t.Errorf("expected 2 items, got %d", len(photos))
	}

	// 只列出视频
	videos, total := m.ListPanoramas("video", 1, 10)
	if total != 1 {
		t.Errorf("expected 1 video, got %d", total)
	}
	if len(videos) != 1 {
		t.Errorf("expected 1 item, got %d", len(videos))
	}

	// 分页
	page1, total := m.ListPanoramas("", 1, 2)
	if total != 3 {
		t.Errorf("expected 3 total, got %d", total)
	}
	if len(page1) != 2 {
		t.Errorf("expected 2 items on page 1, got %d", len(page1))
	}

	page2, _ := m.ListPanoramas("", 2, 2)
	if len(page2) != 1 {
		t.Errorf("expected 1 item on page 2, got %d", len(page2))
	}
}

// ==================== 3D模型测试 ====================

func TestCreateModel(t *testing.T) {
	m, _ := newTestManager(t)

	req := &Model3D{
		Name:   "Test Model",
		Path:   "/test/model.gltf",
		Format: ModelFormatGLTF,
		Size:   1024,
	}

	model, err := m.CreateModel(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if model.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if model.Format != ModelFormatGLTF {
		t.Errorf("expected format gltf, got %q", model.Format)
	}
}

func TestCreateModelValidation(t *testing.T) {
	m, _ := newTestManager(t)

	_, err := m.CreateModel(&Model3D{})
	if err == nil {
		t.Fatal("expected error for empty name")
	}

	_, err = m.CreateModel(&Model3D{Name: "Test"})
	if err == nil {
		t.Fatal("expected error for empty format")
	}
}

func TestListModelFormats(t *testing.T) {
	m, _ := newTestManager(t)

	m.CreateModel(&Model3D{Name: "Model 1", Format: ModelFormatGLTF})
	m.CreateModel(&Model3D{Name: "Model 2", Format: ModelFormatOBJ})
	m.CreateModel(&Model3D{Name: "Model 3", Format: ModelFormatSTL})

	all, total := m.ListModels("", 1, 10)
	if total != 3 {
		t.Errorf("expected 3 total, got %d", total)
	}
	_ = all

	gltfModels, total := m.ListModels("gltf", 1, 10)
	if total != 1 {
		t.Errorf("expected 1 gltf model, got %d", total)
	}
	if len(gltfModels) != 1 {
		t.Errorf("expected 1 item, got %d", len(gltfModels))
	}
}

func TestDeleteModel(t *testing.T) {
	m, dir := newTestManager(t)

	path := createTestFile(t, dir, "model.gltf")
	model, _ := m.CreateModel(&Model3D{Name: "Delete Me", Path: path, Format: ModelFormatGLTF})

	if err := m.DeleteModel(model.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, ok := m.GetModel(model.ID)
	if ok {
		t.Fatal("expected model to be deleted")
	}
}

// ==================== VR画廊测试 ====================

func TestCreateGallery(t *testing.T) {
	m, _ := newTestManager(t)

	req := &VREntry{
		Name:       "My Gallery",
		Background: "museum",
		Layout:     "wall",
		MediaIDs:   []string{"pano-1", "pano-2"},
	}

	gallery, err := m.CreateGallery(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gallery.Layout != "wall" {
		t.Errorf("expected layout 'wall', got %q", gallery.Layout)
	}
	if gallery.Background != "museum" {
		t.Errorf("expected background 'museum', got %q", gallery.Background)
	}
}

func TestCreateGalleryDefaults(t *testing.T) {
	m, _ := newTestManager(t)

	gallery, _ := m.CreateGallery(&VREntry{Name: "Default Gallery"})
	if gallery.Layout != "wall" {
		t.Errorf("expected default layout 'wall', got %q", gallery.Layout)
	}
	if gallery.Background != "museum" {
		t.Errorf("expected default background 'museum', got %q", gallery.Background)
	}
}

func TestUpdateGallery(t *testing.T) {
	m, _ := newTestManager(t)

	gallery, _ := m.CreateGallery(&VREntry{Name: "Old Name"})

	updated, err := m.UpdateGallery(gallery.ID, map[string]interface{}{
		"name":        "New Name",
		"background":  "space",
		"media_ids":   []string{"pano-1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Name != "New Name" {
		t.Errorf("expected name 'New Name', got %q", updated.Name)
	}
	if updated.Background != "space" {
		t.Errorf("expected background 'space', got %q", updated.Background)
	}
}

func TestListGalleries(t *testing.T) {
	m, _ := newTestManager(t)

	m.CreateGallery(&VREntry{Name: "Gallery 1"})
	m.CreateGallery(&VREntry{Name: "Gallery 2"})

	galleries := m.ListGalleries()
	if len(galleries) != 2 {
		t.Errorf("expected 2 galleries, got %d", len(galleries))
	}
}

// ==================== 空间音频测试 ====================

func TestCreateAudioConfig(t *testing.T) {
	m, _ := newTestManager(t)

	req := &SpatialAudioConfig{
		Name:  "Test Audio",
		Mode:  AudioModeSpatialized,
		Gain:  0.8,
		Enabled: true,
	}

	config, err := m.CreateAudioConfig(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Mode != AudioModeSpatialized {
		t.Errorf("expected mode 'spatialized', got %q", config.Mode)
	}
}

func TestCreateAudioConfigDefaults(t *testing.T) {
	m, _ := newTestManager(t)

	config, _ := m.CreateAudioConfig(&SpatialAudioConfig{Name: "Default"})
	if config.Gain != 1.0 {
		t.Errorf("expected default gain 1.0, got %f", config.Gain)
	}
	if config.RoomSize != "medium" {
		t.Errorf("expected default room_size 'medium', got %q", config.RoomSize)
	}
}

func TestUpdateAudioConfig(t *testing.T) {
	m, _ := newTestManager(t)

	config, _ := m.CreateAudioConfig(&SpatialAudioConfig{Name: "Test", Gain: 1.0})

	updated, err := m.UpdateAudioConfig(config.ID, map[string]interface{}{
		"gain":        0.5,
		"room_size":   "large",
		"reverb_level": 0.7,
		"enabled":     false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Gain != 0.5 {
		t.Errorf("expected gain 0.5, got %f", updated.Gain)
	}
	if updated.RoomSize != "large" {
		t.Errorf("expected room_size 'large', got %q", updated.RoomSize)
	}
}

// ==================== 沉浸式影院测试 ====================

func TestCreateTheater(t *testing.T) {
	m, _ := newTestManager(t)

	req := &ImmersiveTheater{
		Name:        "Home Cinema",
		ScreenType:  "curved",
		Environment: "cinema",
		MaxViewers:  5,
	}

	theater, err := m.CreateTheater(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if theater.MaxViewers != 5 {
		t.Errorf("expected max_viewers 5, got %d", theater.MaxViewers)
	}
}

func TestCreateTheaterDefaults(t *testing.T) {
	m, _ := newTestManager(t)

	theater, _ := m.CreateTheater(&ImmersiveTheater{Name: "Default Theater"})
	if theater.ScreenType != "curved" {
		t.Errorf("expected default screen_type 'curved', got %q", theater.ScreenType)
	}
	if theater.Environment != "cinema" {
		t.Errorf("expected default environment 'cinema', got %q", theater.Environment)
	}
	if theater.MaxViewers != 1 {
		t.Errorf("expected default max_viewers 1, got %d", theater.MaxViewers)
	}
}

func TestListTheaters(t *testing.T) {
	m, _ := newTestManager(t)

	m.CreateTheater(&ImmersiveTheater{Name: "Theater 1"})
	m.CreateTheater(&ImmersiveTheater{Name: "Theater 2"})

	theaters := m.ListTheaters()
	if len(theaters) != 2 {
		t.Errorf("expected 2 theaters, got %d", len(theaters))
	}
}

// ==================== WebXR会话测试 ====================

func TestCreateSession(t *testing.T) {
	m, _ := newTestManager(t)

	session, err := m.CreateSession(XRModeVR, "device-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.Mode != XRModeVR {
		t.Errorf("expected mode 'vr', got %q", session.Mode)
	}
	if session.Status != "connecting" {
		t.Errorf("expected status 'connecting', got %q", session.Status)
	}
	if session.FrameRate != 60 {
		t.Errorf("expected frame_rate 60, got %d", session.FrameRate)
	}
}

func TestUpdateSessionStatus(t *testing.T) {
	m, _ := newTestManager(t)

	session, _ := m.CreateSession(XRModeAR, "device-456")

	updated, err := m.UpdateSessionStatus(session.ID, "active")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Status != "active" {
		t.Errorf("expected status 'active', got %q", updated.Status)
	}
	if updated.EndTime != nil {
		t.Error("expected no end time for active session")
	}

	ended, _ := m.UpdateSessionStatus(session.ID, "ended")
	if ended.EndTime == nil {
		t.Error("expected end time for ended session")
	}
}

func TestListActiveSessions(t *testing.T) {
	m, _ := newTestManager(t)

	s1, _ := m.CreateSession(XRModeVR, "d1")
	m.CreateSession(XRModeAR, "d2")
	m.UpdateSessionStatus(s1.ID, "ended")

	active := m.ListActiveSessions()
	if len(active) != 1 {
		t.Errorf("expected 1 active session, got %d", len(active))
	}
}

// ==================== 导入测试 ====================

func TestImportMedia(t *testing.T) {
	m, dir := newTestManager(t)

	// 创建测试文件
	mediaDir := filepath.Join(dir, "media")
	os.MkdirAll(mediaDir, 0755)
	os.WriteFile(filepath.Join(mediaDir, "pano1.jpg"), []byte("pano1"), 0644)
	os.WriteFile(filepath.Join(mediaDir, "pano2.jpg"), []byte("pano2"), 0644)
	os.WriteFile(filepath.Join(mediaDir, "video.mp4"), []byte("video"), 0644)

	task, err := m.ImportMedia(context.Background(), mediaDir, MediaTypePanorama)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.ID == "" {
		t.Fatal("expected non-empty task ID")
	}

	// 等待导入完成
	// 使用轮询等待（简化测试）
	for i := 0; i < 100; i++ {
		tk, ok := m.GetImportTask(task.ID)
		if ok && tk.Status != TaskStatusPending && tk.Status != TaskStatusProcessing {
			break
		}
	}

	imported, ok := m.GetImportTask(task.ID)
	if !ok {
		t.Fatal("expected to find import task")
	}
	if imported.TotalFiles != 3 {
		t.Errorf("expected 3 total files, got %d", imported.TotalFiles)
	}
}

func TestImportMediaNotFound(t *testing.T) {
	m, _ := newTestManager(t)

	_, err := m.ImportMedia(context.Background(), "/nonexistent/path", MediaTypePanorama)
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}

// ==================== 统计测试 ====================

func TestGetStats(t *testing.T) {
	m, _ := newTestManager(t)

	m.CreatePanorama(&PanoramaMedia{Name: "Pano 1", Size: 1000})
	m.CreatePanorama(&PanoramaMedia{Name: "Video 1", IsVideo: true, Size: 2000})
	m.CreateModel(&Model3D{Name: "Model 1", Format: ModelFormatGLTF, Size: 3000})
	m.CreateGallery(&VREntry{Name: "Gallery 1"})
	m.CreateTheater(&ImmersiveTheater{Name: "Theater 1"})

	stats := m.GetStats()
	if stats.TotalPanoramas != 2 {
		t.Errorf("expected 2 panoramas, got %d", stats.TotalPanoramas)
	}
	if stats.TotalVideos360 != 1 {
		t.Errorf("expected 1 video 360, got %d", stats.TotalVideos360)
	}
	if stats.TotalModels3D != 1 {
		t.Errorf("expected 1 model 3d, got %d", stats.TotalModels3D)
	}
	if stats.TotalGalleries != 1 {
		t.Errorf("expected 1 gallery, got %d", stats.TotalGalleries)
	}
	if stats.TotalTheaters != 1 {
		t.Errorf("expected 1 theater, got %d", stats.TotalTheaters)
	}
	if stats.TotalSize != 6000 {
		t.Errorf("expected total size 6000, got %d", stats.TotalSize)
	}
}

// ==================== HTTP Handler 测试 ====================

func TestHandlePanoramasList(t *testing.T) {
	m, _ := newTestManager(t)
	m.CreatePanorama(&PanoramaMedia{Name: "Test Panorama"})

	handler := HandlePanoramas(m)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/arvr/panoramas?page=1&pageSize=10", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var result map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result)
	if result["total"].(float64) != 1 {
		t.Errorf("expected total 1, got %v", result["total"])
	}
}

func TestHandlePanoramasCreate(t *testing.T) {
	m, _ := newTestManager(t)

	body := bytes.NewBufferString(`{"name":"New Panorama","path":"/test/pano.jpg","width":4096,"height":2048}`)
	handler := HandlePanoramas(m)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/arvr/panoramas", body)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var result PanoramaMedia
	json.NewDecoder(w.Body).Decode(&result)
	if result.Name != "New Panorama" {
		t.Errorf("expected name 'New Panorama', got %q", result.Name)
	}
}

func TestHandlePanoramasMethodNotAllowed(t *testing.T) {
	m, _ := newTestManager(t)

	handler := HandlePanoramas(m)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/arvr/panoramas", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestHandlePanoramaByIDGet(t *testing.T) {
	m, _ := newTestManager(t)
	media, _ := m.CreatePanorama(&PanoramaMedia{Name: "Test"})

	handler := HandlePanoramaByID(m)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/arvr/panoramas/"+media.ID, nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlePanoramaByIDNotFound(t *testing.T) {
	m, _ := newTestManager(t)

	handler := HandlePanoramaByID(m)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/arvr/panoramas/nonexistent", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandleModelsCreate(t *testing.T) {
	m, _ := newTestManager(t)

	body := bytes.NewBufferString(`{"name":"Test Model","format":"gltf","path":"/test/model.gltf"}`)
	handler := HandleModels(m)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/arvr/models", body)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
}

func TestHandleGalleriesCreate(t *testing.T) {
	m, _ := newTestManager(t)

	body := bytes.NewBufferString(`{"name":"VR Gallery","background":"museum","layout":"wall"}`)
	handler := HandleGalleries(m)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/arvr/galleries", body)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
}

func TestHandleTheatersCreate(t *testing.T) {
	m, _ := newTestManager(t)

	body := bytes.NewBufferString(`{"name":"Home Cinema","screen_type":"dome"}`)
	handler := HandleTheaters(m)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/arvr/theaters", body)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
}

func TestHandleSessionsCreate(t *testing.T) {
	m, _ := newTestManager(t)

	body := bytes.NewBufferString(`{"mode":"vr","device_id":"oculus-quest-3"}`)
	handler := HandleSessions(m)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/arvr/sessions", body)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
}

func TestHandleStats(t *testing.T) {
	m, _ := newTestManager(t)
	m.CreatePanorama(&PanoramaMedia{Name: "Pano", Size: 100})

	handler := HandleStats(m)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/arvr/stats", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var stats ARVRStats
	json.NewDecoder(w.Body).Decode(&stats)
	if stats.TotalPanoramas != 1 {
		t.Errorf("expected 1 panorama, got %d", stats.TotalPanoramas)
	}
}

func TestHandleWebXRManifest(t *testing.T) {
	handler := HandleWebXRManifest()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/arvr/webxr/manifest", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var manifest map[string]interface{}
	json.NewDecoder(w.Body).Decode(&manifest)
	if manifest["name"] != "NAS-OS AR/VR Media" {
		t.Errorf("expected name 'NAS-OS AR/VR Media', got %q", manifest["name"])
	}
}
