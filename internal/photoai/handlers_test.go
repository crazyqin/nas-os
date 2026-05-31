package photoai

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	cfg := DefaultPhotoAIConfig()
	return NewManager(cfg)
}

func setupTestHandlers(t *testing.T) (*Handler, *Manager) {
	t.Helper()
	manager := setupTestManager(t)
	handler := NewHandler(manager)
	return handler, manager
}

func setupTestMux(handler *Handler) *http.ServeMux {
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func addTestPhoto(t *testing.T, m *Manager, id string) *Photo {
	t.Helper()
	photo := &Photo{
		ID:         id,
		Filename:   "test_" + id + ".jpg",
		FilePath:   "/data/photos/test_" + id + ".jpg",
		FileSize:   1024000,
		MimeType:   "image/jpeg",
		Width:      1920,
		Height:     1080,
		Status:     StatusPending,
		Score:      75.0,
		Tags:       []string{"test", "photo"},
		Categories: []PhotoCategory{CategoryLandscape},
		CreatedAt:  time.Now(),
	}
	if err := m.AddPhoto(photo); err != nil {
		t.Fatalf("add photo %s: %v", id, err)
	}
	return photo
}

// ==================== 照片管理测试 ====================

func TestAddPhoto(t *testing.T) {
	handler, _ := setupTestHandlers(t)
	mux := setupTestMux(handler)

	photo := Photo{
		Filename: "test.jpg",
		FilePath: "/data/photos/test.jpg",
		FileSize: 1024000,
		MimeType: "image/jpeg",
		Width:    1920,
		Height:   1080,
	}

	body, _ := json.Marshal(photo)
	req := httptest.NewRequest("POST", "/api/v1/photo/photos", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var resp apiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}
}

func TestListPhotos(t *testing.T) {
	handler, manager := setupTestHandlers(t)
	mux := setupTestMux(handler)

	addTestPhoto(t, manager, "photo-1")
	addTestPhoto(t, manager, "photo-2")

	req := httptest.NewRequest("GET", "/api/v1/photo/photos?page=1&page_size=10", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp apiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}
}

func TestGetPhoto(t *testing.T) {
	handler, manager := setupTestHandlers(t)
	mux := setupTestMux(handler)

	addTestPhoto(t, manager, "photo-1")

	req := httptest.NewRequest("GET", "/api/v1/photo/photos/photo-1", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestGetPhotoNotFound(t *testing.T) {
	handler, _ := setupTestHandlers(t)
	mux := setupTestMux(handler)

	req := httptest.NewRequest("GET", "/api/v1/photo/photos/nonexistent", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestUpdatePhoto(t *testing.T) {
	handler, manager := setupTestHandlers(t)
	mux := setupTestMux(handler)

	addTestPhoto(t, manager, "photo-1")

	updated := Photo{
		Filename: "updated.jpg",
		FilePath: "/data/photos/updated.jpg",
	}

	body, _ := json.Marshal(updated)
	req := httptest.NewRequest("PUT", "/api/v1/photo/photos/photo-1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestDeletePhoto(t *testing.T) {
	handler, manager := setupTestHandlers(t)
	mux := setupTestMux(handler)

	addTestPhoto(t, manager, "photo-1")

	req := httptest.NewRequest("DELETE", "/api/v1/photo/photos/photo-1", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestSetFavorite(t *testing.T) {
	handler, manager := setupTestHandlers(t)
	mux := setupTestMux(handler)

	addTestPhoto(t, manager, "photo-1")

	reqBody := FavoriteRequest{IsFavorite: true}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", "/api/v1/photo/photos/photo-1/favorite", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// 验证收藏状态
	photo, err := manager.GetPhoto("photo-1")
	if err != nil {
		t.Fatalf("get photo: %v", err)
	}
	if !photo.IsFavorite {
		t.Error("expected photo to be favorite")
	}
}

// ==================== 搜索测试 ====================

func TestSearchPhotos(t *testing.T) {
	handler, manager := setupTestHandlers(t)
	mux := setupTestMux(handler)

	p := addTestPhoto(t, manager, "photo-1")
	p.Tags = []string{"sunset", "beach"}
	p.Score = 85.0
	manager.UpdatePhoto(p)

	query := SearchQuery{
		Keywords:  "sunset",
		MinScore:  float64Ptr(80.0),
		Page:      1,
		PageSize:  10,
	}

	body, _ := json.Marshal(query)
	req := httptest.NewRequest("POST", "/api/v1/photo/search", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp apiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}
}

func TestSearchPhotosByCategory(t *testing.T) {
	handler, manager := setupTestHandlers(t)
	mux := setupTestMux(handler)

	p := addTestPhoto(t, manager, "photo-1")
	p.Categories = []PhotoCategory{CategoryPortrait}
	manager.UpdatePhoto(p)

	query := SearchQuery{
		Categories: []PhotoCategory{CategoryPortrait},
		Page:       1,
		PageSize:   10,
	}

	body, _ := json.Marshal(query)
	req := httptest.NewRequest("POST", "/api/v1/photo/search", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

// ==================== AI 分析测试 ====================

func TestAnalyzePhoto(t *testing.T) {
	handler, manager := setupTestHandlers(t)
	mux := setupTestMux(handler)

	addTestPhoto(t, manager, "photo-1")

	req := httptest.NewRequest("POST", "/api/v1/photo/analyze/photo-1", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// 验证照片状态已更新
	photo, err := manager.GetPhoto("photo-1")
	if err != nil {
		t.Fatalf("get photo: %v", err)
	}
	if photo.Status != StatusReady {
		t.Errorf("expected status %s, got %s", StatusReady, photo.Status)
	}
}

func TestAnalyzePhotoNotFound(t *testing.T) {
	handler, _ := setupTestHandlers(t)
	mux := setupTestMux(handler)

	req := httptest.NewRequest("POST", "/api/v1/photo/analyze/nonexistent", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// ==================== 智能相册测试 ====================

func TestCreateAlbum(t *testing.T) {
	handler, _ := setupTestHandlers(t)
	mux := setupTestMux(handler)

	reqBody := AlbumRequest{
		Name:        "Best Landscapes",
		Description: "High-scoring landscape photos",
		Type:        AlbumTypeScore,
		Rules: []AlbumRule{
			{Field: "score", Operator: "gt", Value: 80},
			{Field: "category", Operator: "eq", Value: "landscape"},
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/photo/albums", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}
}

func TestListAlbums(t *testing.T) {
	handler, manager := setupTestHandlers(t)
	mux := setupTestMux(handler)

	// 创建相册
	_, err := manager.CreateAlbum(&AlbumRequest{
		Name: "Test Album",
		Type: AlbumTypeCustom,
		Rules: []AlbumRule{
			{Field: "tag", Operator: "eq", Value: "test"},
		},
	})
	if err != nil {
		t.Fatalf("create album: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/v1/photo/albums", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestGetAlbum(t *testing.T) {
	handler, manager := setupTestHandlers(t)
	mux := setupTestMux(handler)

	album, err := manager.CreateAlbum(&AlbumRequest{
		Name: "Test Album",
		Type: AlbumTypeCustom,
		Rules: []AlbumRule{
			{Field: "tag", Operator: "eq", Value: "test"},
		},
	})
	if err != nil {
		t.Fatalf("create album: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/v1/photo/albums/"+album.ID, nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestDeleteAlbum(t *testing.T) {
	handler, manager := setupTestHandlers(t)
	mux := setupTestMux(handler)

	album, err := manager.CreateAlbum(&AlbumRequest{
		Name: "Test Album",
		Type: AlbumTypeCustom,
		Rules: []AlbumRule{
			{Field: "tag", Operator: "eq", Value: "test"},
		},
	})
	if err != nil {
		t.Fatalf("create album: %v", err)
	}

	req := httptest.NewRequest("DELETE", "/api/v1/photo/albums/"+album.ID, nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestRefreshAlbums(t *testing.T) {
	handler, manager := setupTestHandlers(t)
	mux := setupTestMux(handler)

	// 创建相册
	_, err := manager.CreateAlbum(&AlbumRequest{
		Name: "Landscape",
		Type: AlbumTypeTag,
		Rules: []AlbumRule{
			{Field: "category", Operator: "eq", Value: "landscape"},
		},
	})
	if err != nil {
		t.Fatalf("create album: %v", err)
	}

	// 添加照片
	p := addTestPhoto(t, manager, "photo-1")
	p.Categories = []PhotoCategory{CategoryLandscape}
	manager.UpdatePhoto(p)

	req := httptest.NewRequest("POST", "/api/v1/photo/albums-refresh", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

// ==================== 扫描 & 导入测试 ====================

func TestScan(t *testing.T) {
	handler, _ := setupTestHandlers(t)
	mux := setupTestMux(handler)

	reqBody := ScanRequest{
		Directory: "/data/photos",
		Recursive: true,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/photo/scan", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestImportPhotos(t *testing.T) {
	handler, _ := setupTestHandlers(t)
	mux := setupTestMux(handler)

	reqBody := ImportRequest{
		Paths: []string{"/data/photos/test1.jpg", "/data/photos/test2.jpg"},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/photo/import", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp apiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}
}

// ==================== 分享测试 ====================

func TestCreateShareLink(t *testing.T) {
	handler, manager := setupTestHandlers(t)
	mux := setupTestMux(handler)

	addTestPhoto(t, manager, "photo-1")

	reqBody := ShareRequest{
		PhotoIDs:  []string{"photo-1"},
		ExpiresIn: 24,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/photo/share", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}
}

func TestGetShareLink(t *testing.T) {
	handler, manager := setupTestHandlers(t)
	mux := setupTestMux(handler)

	addTestPhoto(t, manager, "photo-1")

	// 创建分享
	link, err := manager.CreateShareLink(&ShareRequest{
		PhotoIDs: []string{"photo-1"},
	})
	if err != nil {
		t.Fatalf("create share: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/v1/photo/share/"+link.Token, nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

// ==================== 去重测试 ====================

func TestDetectDuplicates(t *testing.T) {
	handler, manager := setupTestHandlers(t)
	mux := setupTestMux(handler)

	// 添加两张相同尺寸的照片
	p1 := addTestPhoto(t, manager, "photo-1")
	p1.Width = 1920
	p1.Height = 1080
	manager.UpdatePhoto(p1)

	p2 := addTestPhoto(t, manager, "photo-2")
	p2.Filename = p1.Filename
	p2.Width = 1920
	p2.Height = 1080
	manager.UpdatePhoto(p2)

	req := httptest.NewRequest("GET", "/api/v1/photo/duplicates", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

// ==================== 统计测试 ====================

func TestGetStats(t *testing.T) {
	handler, manager := setupTestHandlers(t)
	mux := setupTestMux(handler)

	addTestPhoto(t, manager, "photo-1")
	addTestPhoto(t, manager, "photo-2")

	req := httptest.NewRequest("GET", "/api/v1/photo/stats", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp apiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}
}

func TestGetCategories(t *testing.T) {
	handler, manager := setupTestHandlers(t)
	mux := setupTestMux(handler)

	p := addTestPhoto(t, manager, "photo-1")
	p.Categories = []PhotoCategory{CategoryLandscape}
	manager.UpdatePhoto(p)

	req := httptest.NewRequest("GET", "/api/v1/photo/categories", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestGetTimeline(t *testing.T) {
	handler, manager := setupTestHandlers(t)
	mux := setupTestMux(handler)

	addTestPhoto(t, manager, "photo-1")

	req := httptest.NewRequest("GET", "/api/v1/photo/timeline", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

// ==================== Manager 方法测试 ====================

func TestManagerAddPhoto(t *testing.T) {
	m := setupTestManager(t)

	photo := &Photo{
		ID:       "test-1",
		Filename: "test.jpg",
		FilePath: "/data/photos/test.jpg",
	}

	if err := m.AddPhoto(photo); err != nil {
		t.Fatalf("add photo: %v", err)
	}

	got, err := m.GetPhoto("test-1")
	if err != nil {
		t.Fatalf("get photo: %v", err)
	}
	if got.Filename != "test.jpg" {
		t.Errorf("expected filename test.jpg, got %s", got.Filename)
	}
}

func TestManagerAddDuplicatePhoto(t *testing.T) {
	m := setupTestManager(t)

	photo := &Photo{ID: "test-1", Filename: "test.jpg"}
	if err := m.AddPhoto(photo); err != nil {
		t.Fatalf("first add: %v", err)
	}

	if err := m.AddPhoto(photo); err == nil {
		t.Error("expected error for duplicate photo")
	}
}

func TestManagerDeletePhoto(t *testing.T) {
	m := setupTestManager(t)

	photo := &Photo{ID: "test-1", Filename: "test.jpg"}
	m.AddPhoto(photo)

	if err := m.DeletePhoto("test-1"); err != nil {
		t.Fatalf("delete photo: %v", err)
	}

	if _, err := m.GetPhoto("test-1"); err == nil {
		t.Error("expected error for deleted photo")
	}
}

func TestManagerSearchPhotos(t *testing.T) {
	m := setupTestManager(t)

	p := &Photo{
		ID:         "test-1",
		Filename:   "sunset.jpg",
		Tags:       []string{"sunset", "beach"},
		Score:      90.0,
		Categories: []PhotoCategory{CategoryLandscape},
		Status:     StatusReady,
		CreatedAt:  time.Now(),
	}
	m.AddPhoto(p)

	// 搜索关键词
	result := m.SearchPhotos(&SearchQuery{Keywords: "sunset", Page: 1, PageSize: 10})
	if result.Total != 1 {
		t.Errorf("expected 1 result, got %d", result.Total)
	}

	// 搜索评分
	minScore := 85.0
	result = m.SearchPhotos(&SearchQuery{MinScore: &minScore, Page: 1, PageSize: 10})
	if result.Total != 1 {
		t.Errorf("expected 1 result, got %d", result.Total)
	}
}

func TestManagerCreateAlbum(t *testing.T) {
	m := setupTestManager(t)

	album, err := m.CreateAlbum(&AlbumRequest{
		Name: "Test Album",
		Type: AlbumTypeCustom,
		Rules: []AlbumRule{
			{Field: "tag", Operator: "eq", Value: "test"},
		},
	})
	if err != nil {
		t.Fatalf("create album: %v", err)
	}

	if album.Name != "Test Album" {
		t.Errorf("expected name Test Album, got %s", album.Name)
	}
}

func TestManagerGetStats(t *testing.T) {
	m := setupTestManager(t)

	p := &Photo{
		ID:        "test-1",
		Filename:  "test.jpg",
		Status:    StatusReady,
		Score:     80.0,
		CreatedAt: time.Now(),
	}
	m.AddPhoto(p)

	stats := m.GetStats()
	total, ok := stats["total_photos"].(int)
	if !ok || total != 1 {
		t.Errorf("expected total_photos=1, got %v", stats["total_photos"])
	}
}

func TestManagerBatchTag(t *testing.T) {
	m := setupTestManager(t)

	p := &Photo{
		ID:       "test-1",
		Filename: "test.jpg",
		Tags:     []string{"old"},
		CreatedAt: time.Now(),
	}
	m.AddPhoto(p)

	count, err := m.BatchTag(&BatchTagRequest{
		PhotoIDs: []string{"test-1"},
		Tags:     []string{"new", "added"},
		Action:   "add",
	})
	if err != nil {
		t.Fatalf("batch tag: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 updated, got %d", count)
	}

	photo, _ := m.GetPhoto("test-1")
	if len(photo.Tags) != 3 {
		t.Errorf("expected 3 tags, got %d: %v", len(photo.Tags), photo.Tags)
	}
}

func TestManagerDetectDuplicates(t *testing.T) {
	m := setupTestManager(t)

	// 添加两张相同特征的照片
	p1 := &Photo{
		ID:        "dup-1",
		Filename:  "same.jpg",
		Width:     1920,
		Height:    1080,
		CreatedAt: time.Now(),
	}
	p2 := &Photo{
		ID:        "dup-2",
		Filename:  "same.jpg",
		Width:     1920,
		Height:    1080,
		CreatedAt: time.Now(),
	}
	m.AddPhoto(p1)
	m.AddPhoto(p2)

	groups := m.DetectDuplicates()
	if len(groups) == 0 {
		t.Error("expected at least 1 duplicate group")
	}
}

func TestManagerRefreshAlbums(t *testing.T) {
	m := setupTestManager(t)

	// 创建相册
	m.CreateAlbum(&AlbumRequest{
		Name:  "Landscapes",
		Type:  AlbumTypeTag,
		Rules: []AlbumRule{{Field: "category", Operator: "eq", Value: "landscape"}},
	})

	// 添加匹配的照片
	p := &Photo{
		ID:         "test-1",
		Filename:   "test.jpg",
		Categories: []PhotoCategory{CategoryLandscape},
		Status:     StatusReady,
		CreatedAt:  time.Now(),
	}
	m.AddPhoto(p)

	refreshed := m.RefreshAlbums()
	if refreshed != 1 {
		t.Errorf("expected 1 refreshed, got %d", refreshed)
	}

	albums := m.ListAlbums()
	if len(albums) == 0 {
		t.Fatal("expected albums")
	}
	if albums[0].PhotoCount != 1 {
		t.Errorf("expected album photo_count=1, got %d", albums[0].PhotoCount)
	}
}

func TestManagerCreateShareLink(t *testing.T) {
	m := setupTestManager(t)

	p := &Photo{
		ID:        "test-1",
		Filename:  "test.jpg",
		CreatedAt: time.Now(),
	}
	m.AddPhoto(p)

	link, err := m.CreateShareLink(&ShareRequest{
		PhotoIDs:  []string{"test-1"},
		ExpiresIn: 24,
		MaxViews:  10,
	})
	if err != nil {
		t.Fatalf("create share: %v", err)
	}

	if link.Token == "" {
		t.Error("expected non-empty token")
	}
	if link.ExpiresAt == nil {
		t.Error("expected non-nil expires_at")
	}
}

func TestManagerMergePersons(t *testing.T) {
	m := setupTestManager(t)

	// 手动添加两个人物
	person1 := &Person{
		ID:         "person-1",
		Name:       "Alice",
		PhotoIDs:   []string{"p1"},
		FaceIDs:    []string{"f1"},
		PhotoCount: 1,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	person2 := &Person{
		ID:         "person-2",
		Name:       "A. Smith",
		PhotoIDs:   []string{"p2"},
		FaceIDs:    []string{"f2"},
		PhotoCount: 1,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	m.mu.Lock()
	m.persons["person-1"] = person1
	m.persons["person-2"] = person2
	m.mu.Unlock()

	if err := m.MergePersons("person-1", "person-2"); err != nil {
		t.Fatalf("merge persons: %v", err)
	}

	merged, err := m.GetPerson("person-1")
	if err != nil {
		t.Fatalf("get merged person: %v", err)
	}
	if merged.PhotoCount != 2 {
		t.Errorf("expected photo_count=2, got %d", merged.PhotoCount)
	}

	// source 应该被删除
	if _, err := m.GetPerson("person-2"); err == nil {
		t.Error("expected source person to be deleted")
	}
}

// ==================== 辅助函数 ====================

func float64Ptr(v float64) *float64 {
	return &v
}
