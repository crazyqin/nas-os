// Package phototimeline provides photo timeline management for NAS-OS.
package phototimeline

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func setupTestHandler() (*Handler, *TimelineManager, *AlbumManager, *DedupManager) {
	config := DefaultConfig()
	photos := make(map[string]*Photo)
	tm := NewTimelineManager(config)
	am := NewAlbumManager(config, photos)
	dm := NewDedupManager(config, photos)
	logger := zap.NewNop()
	handler := NewHandler(tm, am, dm, config, logger)
	return handler, tm, am, dm
}

func setupTestRouter(handler *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/api/v1")
	handler.RegisterRoutes(rg)
	return r
}

func TestNewHandler(t *testing.T) {
	handler, _, _, _ := setupTestHandler()
	if handler == nil {
		t.Error("Expected handler to be created")
	}
}

func TestRegisterRoutes(t *testing.T) {
	handler, _, _, _ := setupTestHandler()
	r := setupTestRouter(handler)

	// 检查路由是否注册
	routes := r.Routes()
	if len(routes) == 0 {
		t.Error("Expected routes to be registered")
	}
}

func TestUploadPhoto(t *testing.T) {
	handler, _, _, _ := setupTestHandler()
	r := setupTestRouter(handler)

	// 创建测试文件
	body := &bytes.Buffer{}
	body.WriteString("--boundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"file\"; filename=\"test.jpg\"\r\n")
	body.WriteString("Content-Type: image/jpeg\r\n\r\n")
	body.WriteString("fake image data")
	body.WriteString("\r\n--boundary--\r\n")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/photos", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}
}

func TestListPhotos(t *testing.T) {
	handler, tm, _, _ := setupTestHandler()
	r := setupTestRouter(handler)

	// 添加测试照片
	tm.AddPhoto(&Photo{ID: "1", Filename: "test1.jpg", TakenAt: time.Now()})
	tm.AddPhoto(&Photo{ID: "2", Filename: "test2.jpg", TakenAt: time.Now()})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/photos?page=1&page_size=10", nil)

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetPhoto(t *testing.T) {
	handler, tm, _, _ := setupTestHandler()
	r := setupTestRouter(handler)

	tm.AddPhoto(&Photo{ID: "test1", Filename: "test.jpg", TakenAt: time.Now()})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/photos/test1", nil)

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var photo Photo
	json.Unmarshal(w.Body.Bytes(), &photo)
	if photo.ID != "test1" {
		t.Errorf("Expected photo ID 'test1', got '%s'", photo.ID)
	}
}

func TestGetPhoto_NotFound(t *testing.T) {
	handler, _, _, _ := setupTestHandler()
	r := setupTestRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/photos/nonexistent", nil)

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestUpdatePhoto(t *testing.T) {
	handler, tm, _, _ := setupTestHandler()
	r := setupTestRouter(handler)

	tm.AddPhoto(&Photo{ID: "test1", Filename: "test.jpg", TakenAt: time.Now()})

	update := map[string]interface{}{
		"favorite": true,
		"rating":   5,
	}
	body, _ := json.Marshal(update)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/photos/test1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestDeletePhoto(t *testing.T) {
	handler, tm, _, _ := setupTestHandler()
	r := setupTestRouter(handler)

	tm.AddPhoto(&Photo{ID: "test1", Filename: "test.jpg", TakenAt: time.Now()})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/photos/test1", nil)

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetTimeline(t *testing.T) {
	handler, tm, _, _ := setupTestHandler()
	r := setupTestRouter(handler)

	tm.AddPhoto(&Photo{ID: "1", Filename: "test.jpg", TakenAt: time.Now()})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/photos/timeline?view=month&page=1&page_size=10", nil)

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetTimelineStats(t *testing.T) {
	handler, tm, _, _ := setupTestHandler()
	r := setupTestRouter(handler)

	tm.AddPhoto(&Photo{ID: "1", Filename: "test.jpg", TakenAt: time.Now(), Size: 1024})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/photos/timeline/stats", nil)

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestSearchPhotos(t *testing.T) {
	handler, tm, _, _ := setupTestHandler()
	r := setupTestRouter(handler)

	tm.AddPhoto(&Photo{ID: "1", Filename: "test.jpg", TakenAt: time.Now()})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/photos/search?keyword=test&page=1", nil)

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestBatchOperation_Delete(t *testing.T) {
	handler, tm, _, _ := setupTestHandler()
	r := setupTestRouter(handler)

	tm.AddPhoto(&Photo{ID: "1", Filename: "test1.jpg", TakenAt: time.Now()})
	tm.AddPhoto(&Photo{ID: "2", Filename: "test2.jpg", TakenAt: time.Now()})

	batch := BatchRequest{
		Operation: BatchOpDelete,
		PhotoIDs:  []string{"1", "2"},
	}
	body, _ := json.Marshal(batch)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/photos/batch", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result BatchResult
	json.Unmarshal(w.Body.Bytes(), &result)
	if result.Success != 2 {
		t.Errorf("Expected 2 successes, got %d", result.Success)
	}
}

func TestBatchOperation_Favorite(t *testing.T) {
	handler, tm, _, _ := setupTestHandler()
	r := setupTestRouter(handler)

	tm.AddPhoto(&Photo{ID: "1", Filename: "test1.jpg", TakenAt: time.Now()})

	batch := BatchRequest{
		Operation: BatchOpFav,
		PhotoIDs:  []string{"1"},
	}
	body, _ := json.Marshal(batch)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/photos/batch", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetEXIF(t *testing.T) {
	handler, tm, _, _ := setupTestHandler()
	r := setupTestRouter(handler)

	tm.AddPhoto(&Photo{
		ID:       "test1",
		Filename: "test.jpg",
		TakenAt:  time.Now(),
		EXIF:     EXIFData{CameraModel: "iPhone 15"},
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/photos/test1/exif", nil)

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestUpdateEXIF(t *testing.T) {
	handler, tm, _, _ := setupTestHandler()
	r := setupTestRouter(handler)

	tm.AddPhoto(&Photo{ID: "test1", Filename: "test.jpg", TakenAt: time.Now()})

	exif := EXIFData{CameraModel: "Canon EOS R5"}
	body, _ := json.Marshal(exif)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/photos/test1/exif", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAddTags(t *testing.T) {
	handler, tm, _, _ := setupTestHandler()
	r := setupTestRouter(handler)

	tm.AddPhoto(&Photo{ID: "test1", Filename: "test.jpg", TakenAt: time.Now()})

	reqBody := map[string][]string{"tags": {"vacation", "beach"}}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/photos/test1/tags", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestRemoveTags(t *testing.T) {
	handler, tm, _, _ := setupTestHandler()
	r := setupTestRouter(handler)

	tm.AddPhoto(&Photo{ID: "test1", Filename: "test.jpg", TakenAt: time.Now(), Tags: []string{"vacation", "beach"}})

	reqBody := map[string][]string{"tags": {"beach"}}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/photos/test1/tags", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAddPeople(t *testing.T) {
	handler, tm, _, _ := setupTestHandler()
	r := setupTestRouter(handler)

	tm.AddPhoto(&Photo{ID: "test1", Filename: "test.jpg", TakenAt: time.Now()})

	reqBody := map[string][]string{"people": {"Alice", "Bob"}}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/photos/test1/people", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestRemovePeople(t *testing.T) {
	handler, tm, _, _ := setupTestHandler()
	r := setupTestRouter(handler)

	tm.AddPhoto(&Photo{ID: "test1", Filename: "test.jpg", TakenAt: time.Now(), People: []string{"Alice", "Bob"}})

	reqBody := map[string][]string{"people": {"Bob"}}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/photos/test1/people", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestCreateAlbumAPI(t *testing.T) {
	handler, _, _, _ := setupTestHandler()
	r := setupTestRouter(handler)

	album := Album{
		ID:   "album1",
		Name: "Test Album",
		Type: AlbumTypeManual,
	}
	body, _ := json.Marshal(album)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/albums", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}
}

func TestListAlbumsAPI(t *testing.T) {
	handler, _, am, _ := setupTestHandler()
	r := setupTestRouter(handler)

	am.CreateAlbum(&Album{ID: "1", Name: "Album 1", Type: AlbumTypeManual})
	am.CreateAlbum(&Album{ID: "2", Name: "Album 2", Type: AlbumTypePerson})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/albums", nil)

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetAlbumAPI(t *testing.T) {
	handler, _, am, _ := setupTestHandler()
	r := setupTestRouter(handler)

	am.CreateAlbum(&Album{ID: "album1", Name: "Test Album", Type: AlbumTypeManual})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/albums/album1", nil)

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestDeleteAlbumAPI(t *testing.T) {
	handler, _, am, _ := setupTestHandler()
	r := setupTestRouter(handler)

	am.CreateAlbum(&Album{ID: "album1", Name: "Test Album", Type: AlbumTypeManual})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/albums/album1", nil)

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetAlbumPhotosAPI(t *testing.T) {
	handler, _, am, _ := setupTestHandler()
	r := setupTestRouter(handler)

	am.CreateAlbum(&Album{ID: "album1", Name: "Test Album", Type: AlbumTypeManual, PhotoCount: 0})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/albums/album1/photos?page=1&page_size=10", nil)

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetDedupStatsAPI(t *testing.T) {
	handler, _, _, _ := setupTestHandler()
	r := setupTestRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/dedup/stats", nil)

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetDuplicateGroupsAPI(t *testing.T) {
	handler, _, _, _ := setupTestHandler()
	r := setupTestRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/dedup/groups", nil)

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestCreateShareLinkAPI(t *testing.T) {
	handler, _, _, _ := setupTestHandler()
	r := setupTestRouter(handler)

	link := ShareLink{
		AlbumID:       "album1",
		AllowDownload: true,
	}
	body, _ := json.Marshal(link)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/shares", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}
}

func TestGetShareLinkAPI(t *testing.T) {
	handler, _, _, _ := setupTestHandler()
	r := setupTestRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/shares/testtoken", nil)

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestGetMapDataAPI(t *testing.T) {
	handler, _, _, _ := setupTestHandler()
	r := setupTestRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/photos/map", nil)

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}
