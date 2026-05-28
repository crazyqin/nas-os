package wiki

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestHandler(t *testing.T) (*Handler, *Manager, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "wiki.json")

	mgr := NewManager(configPath)
	handler := NewHandler(mgr)

	router := gin.New()
	api := router.Group("/api")
	handler.RegisterRoutes(api)

	return handler, mgr, router
}

func TestCreateSpace(t *testing.T) {
	_, _, router := setupTestHandler(t)

	body := `{"name":"技术文档","description":"项目技术文档空间","is_public":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/wiki/spaces", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var space Space
	if err := json.Unmarshal(w.Body.Bytes(), &space); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if space.Name != "技术文档" {
		t.Errorf("expected name '技术文档', got '%s'", space.Name)
	}

	if !space.IsPublic {
		t.Errorf("expected is_public true")
	}

	if len(space.Members) != 1 {
		t.Errorf("expected 1 member, got %d", len(space.Members))
	}
}

func TestListSpaces(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	// Create two spaces
	mgr.CreateSpace(CreateSpaceRequest{Name: "空间1"}, "user1")
	mgr.CreateSpace(CreateSpaceRequest{Name: "空间2"}, "user2")

	req := httptest.NewRequest(http.MethodGet, "/api/wiki/spaces", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Spaces []*Space `json:"spaces"`
		Total  int      `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Total != 2 {
		t.Errorf("expected 2 spaces, got %d", resp.Total)
	}
}

func TestGetSpace(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	space, _ := mgr.CreateSpace(CreateSpaceRequest{Name: "测试空间"}, "user1")

	req := httptest.NewRequest(http.MethodGet, "/api/wiki/spaces/"+space.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp Space
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.ID != space.ID {
		t.Errorf("expected space ID %s, got %s", space.ID, resp.ID)
	}
}

func TestGetSpaceNotFound(t *testing.T) {
	_, _, router := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/wiki/spaces/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestCreatePage(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	space, _ := mgr.CreateSpace(CreateSpaceRequest{Name: "测试空间"}, "user1")

	body := `{"space_id":"` + space.ID + `","title":"入门指南","content":"# 入门指南\n\n这是入门指南内容","status":"published"}`
	req := httptest.NewRequest(http.MethodPost, "/api/wiki/pages", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var page Page
	json.Unmarshal(w.Body.Bytes(), &page)

	if page.Title != "入门指南" {
		t.Errorf("expected title '入门指南', got '%s'", page.Title)
	}

	if page.Status != "published" {
		t.Errorf("expected status 'published', got '%s'", page.Status)
	}

	if page.Version != 1 {
		t.Errorf("expected version 1, got %d", page.Version)
	}
}

func TestGetPage(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	space, _ := mgr.CreateSpace(CreateSpaceRequest{Name: "测试空间"}, "user1")
	page, _ := mgr.CreatePage(CreatePageRequest{
		SpaceID: space.ID,
		Title:   "测试页面",
		Content: "测试内容",
	}, "user1", "User 1")

	req := httptest.NewRequest(http.MethodGet, "/api/wiki/pages/"+page.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp Page
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.ID != page.ID {
		t.Errorf("expected page ID %s, got %s", page.ID, resp.ID)
	}
}

func TestListPages(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	space, _ := mgr.CreateSpace(CreateSpaceRequest{Name: "测试空间"}, "user1")
	mgr.CreatePage(CreatePageRequest{SpaceID: space.ID, Title: "页面1"}, "user1", "User 1")
	mgr.CreatePage(CreatePageRequest{SpaceID: space.ID, Title: "页面2"}, "user1", "User 1")

	req := httptest.NewRequest(http.MethodGet, "/api/wiki/pages?space_id="+space.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Pages []*Page `json:"pages"`
		Total int     `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Total != 2 {
		t.Errorf("expected 2 pages, got %d", resp.Total)
	}
}

func TestUpdatePage(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	space, _ := mgr.CreateSpace(CreateSpaceRequest{Name: "测试空间"}, "user1")
	page, _ := mgr.CreatePage(CreatePageRequest{
		SpaceID: space.ID,
		Title:   "原标题",
		Content: "原内容",
	}, "user1", "User 1")

	body := `{"title":"新标题","content":"新内容","comment":"更新标题和内容"}`
	req := httptest.NewRequest(http.MethodPut, "/api/wiki/pages/"+page.ID, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp Page
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Title != "新标题" {
		t.Errorf("expected title '新标题', got '%s'", resp.Title)
	}

	if resp.Version != 2 {
		t.Errorf("expected version 2, got %d", resp.Version)
	}
}

func TestDeletePage(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	space, _ := mgr.CreateSpace(CreateSpaceRequest{Name: "测试空间"}, "user1")
	page, _ := mgr.CreatePage(CreatePageRequest{
		SpaceID: space.ID,
		Title:   "待删除",
	}, "user1", "User 1")

	req := httptest.NewRequest(http.MethodDelete, "/api/wiki/pages/"+page.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Verify deleted
	req2 := httptest.NewRequest(http.MethodGet, "/api/wiki/pages/"+page.ID, nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", w2.Code)
	}
}

func TestSearchPages(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	space, _ := mgr.CreateSpace(CreateSpaceRequest{Name: "测试空间"}, "user1")
	mgr.CreatePage(CreatePageRequest{
		SpaceID: space.ID,
		Title:   "Go语言入门",
		Content: "Go是Google开发的编程语言",
		Status:  "published",
	}, "user1", "User 1")

	mgr.CreatePage(CreatePageRequest{
		SpaceID: space.ID,
		Title:   "Python教程",
		Content: "Python是一种解释型语言",
		Status:  "published",
	}, "user1", "User 1")

	req := httptest.NewRequest(http.MethodGet, "/api/wiki/search?q=Go", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Results []*SearchResult `json:"results"`
		Total   int             `json:"total"`
		Query   string          `json:"query"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Total < 1 {
		t.Errorf("expected at least 1 result, got %d", resp.Total)
	}

	if resp.Query != "Go" {
		t.Errorf("expected query 'Go', got '%s'", resp.Query)
	}
}

func TestGetPageVersions(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	space, _ := mgr.CreateSpace(CreateSpaceRequest{Name: "测试空间"}, "user1")
	page, _ := mgr.CreatePage(CreatePageRequest{
		SpaceID: space.ID,
		Title:   "测试页面",
		Content: "初始内容",
	}, "user1", "User 1")

	// Update page to create version history
	mgr.UpdatePage(page.ID, UpdatePageRequest{
		Content: strPtr("更新内容"),
		Comment: "第一次更新",
	}, "user1", "User 1")

	req := httptest.NewRequest(http.MethodGet, "/api/wiki/pages/"+page.ID+"/versions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Versions []*PageVersion `json:"versions"`
		Total    int            `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Total != 2 {
		t.Errorf("expected 2 versions, got %d", resp.Total)
	}
}

func TestAddComment(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	space, _ := mgr.CreateSpace(CreateSpaceRequest{Name: "测试空间"}, "user1")
	page, _ := mgr.CreatePage(CreatePageRequest{
		SpaceID: space.ID,
		Title:   "测试页面",
	}, "user1", "User 1")

	body := `{"content":"这是一条评论"}`
	req := httptest.NewRequest(http.MethodPost, "/api/wiki/pages/"+page.ID+"/comments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var comment Comment
	json.Unmarshal(w.Body.Bytes(), &comment)

	if comment.Content != "这是一条评论" {
		t.Errorf("expected content '这是一条评论', got '%s'", comment.Content)
	}
}

func strPtr(s string) *string {
	return &s
}
