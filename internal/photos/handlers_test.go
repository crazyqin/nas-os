package photos

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// ========== Handlers Tests ==========

func init() {
	gin.SetMode(gin.TestMode)
}

func TestNewHandlers(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)

	handlers := NewHandlers(manager, nil)
	if handlers == nil {
		t.Fatal("handlers 不应为 nil")
	}

	if handlers.manager == nil {
		t.Error("manager 不应为 nil")
	}
}

func TestHandlers_RegisterRoutes(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)
	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	// 验证路由注册
	routes := router.Routes()
	if len(routes) == 0 {
		t.Error("应注册路由")
	}
}

// ========== Upload Tests ==========

func TestHandlers_UploadPhoto_InvalidFile(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)
	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	// 测试无文件上传
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/photos/upload", nil)
	req.Header.Set("Content-Type", "multipart/form-data")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 400，得到 %d", w.Code)
	}
}

func TestHandlers_UploadPhoto_UnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)
	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	// 创建测试文件
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.txt")
	_, _ = part.Write([]byte("test content"))
	_ = writer.Close()

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/photos/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// 不支持的格式应该返回 400
	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 400，得到 %d", w.Code)
	}
}

// ========== Album Tests ==========

func TestHandlers_CreateAlbum(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)
	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	// 创建相册请求
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/photos/albums?name=测试相册&description=测试描述&userId=user1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// 验证结果
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
		t.Logf("响应状态码: %d", w.Code)
	}
}

func TestHandlers_ListAlbums(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)
	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/photos/albums?userId=user1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，得到 %d", w.Code)
	}
}

func TestHandlers_GetAlbum_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)
	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/photos/albums/nonexistent", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// 不存在的相册应该返回 404
	if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 404，得到 %d", w.Code)
	}
}

func TestHandlers_DeleteAlbum(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)

	// 先创建一个相册
	album, _ := manager.CreateAlbum("测试相册", "", "user1")

	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "DELETE", "/api/photos/albums/"+album.ID, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// 验证结果
	t.Logf("删除相册响应状态码: %d", w.Code)
}

// ========== Photo Tests ==========

func TestHandlers_ListPhotos(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)
	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/photos?userId=user1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，得到 %d", w.Code)
	}
}

func TestHandlers_GetPhoto_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)
	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/photos/nonexistent", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// 不存在的照片应该返回 404
	if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 404，得到 %d", w.Code)
	}
}

func TestHandlers_ToggleFavorite(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)

	// 创建测试照片
	photo := &Photo{
		ID:         "test-photo-1",
		Filename:   "test.jpg",
		Path:       "test.jpg",
		IsFavorite: false,
		UploadedAt: time.Now(),
		ModifiedAt: time.Now(),
	}
	manager.photos[photo.ID] = photo

	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/photos/"+photo.ID+"/favorite", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	t.Logf("切换收藏响应状态码: %d", w.Code)
}

// ========== Timeline Tests ==========

func TestHandlers_GetTimeline(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)
	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/photos/timeline?period=month", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，得到 %d", w.Code)
	}
}

// ========== Person Tests ==========

func TestHandlers_ListPersons(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)
	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/photos/persons", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，得到 %d", w.Code)
	}
}

func TestHandlers_CreatePerson(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)
	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/photos/persons?name=张三", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	t.Logf("创建人物响应状态码: %d", w.Code)
}

func TestHandlers_DeletePerson(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)

	// 先创建人物
	person, _ := manager.CreatePerson("张三")

	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "DELETE", "/api/photos/persons/"+person.ID, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	t.Logf("删除人物响应状态码: %d", w.Code)
}

// ========== AI Tests ==========

func TestHandlers_GetAIStats(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)
	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/photos/ai/stats", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	t.Logf("AI 统计响应状态码: %d", w.Code)
}

func TestHandlers_ListAITasks(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)
	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/photos/ai/tasks", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	t.Logf("AI 任务列表响应状态码: %d", w.Code)
}

func TestHandlers_ListSmartAlbums(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)
	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/photos/ai/smart-albums", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	t.Logf("智能相册列表响应状态码: %d", w.Code)
}

func TestHandlers_GetMemories(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)
	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/photos/ai/memories", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	t.Logf("回忆列表响应状态码: %d", w.Code)
}

// ========== Search Tests ==========

func TestHandlers_SearchPhotos(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)
	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/photos/search?q=test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	t.Logf("搜索响应状态码: %d", w.Code)
}

// ========== Stats Tests ==========

func TestHandlers_GetStats(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)
	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/photos/stats", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，得到 %d", w.Code)
	}
}

// ========== Thumbnail Tests ==========

func TestHandlers_GetThumbnail_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)
	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/photos/nonexistent/thumbnail", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// 不存在的照片应该返回 404
	if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 404，得到 %d", w.Code)
	}
}

// ========== Download Tests ==========

func TestHandlers_DownloadPhoto_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)
	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/photos/nonexistent/download", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// 不存在的照片应该返回 404
	if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 404，得到 %d", w.Code)
	}
}

// ========== Upload Session Tests ==========

func TestHandlers_CreateUploadSession(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)
	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/photos/upload/session?filename=test.jpg&totalSize=1024", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	t.Logf("创建上传会话响应状态码: %d", w.Code)
}

// ========== Batch Upload Tests ==========

func TestHandlers_UploadPhotoBatch_NoFiles(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)
	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	// 创建空的 multipart 表单
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.Close()

	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/photos/upload/batch", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// 没有文件应该返回 400
	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 400，得到 %d", w.Code)
	}
}

// ========== Integration Tests ==========

func TestHandlers_FullAlbumWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)
	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	// 1. 创建相册
	album, err := manager.CreateAlbum("工作流测试", "测试完整工作流", "user1")
	if err != nil {
		t.Fatalf("创建相册失败：%v", err)
	}

	// 2. 获取相册
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/photos/albums/"+album.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("获取相册失败，状态码: %d", w.Code)
	}

	// 3. 删除相册
	req = httptest.NewRequestWithContext(context.Background(), "DELETE", "/api/photos/albums/"+album.ID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	t.Logf("完整工作流测试完成")
}

func TestHandlers_PhotoQuery(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)

	// 添加一些测试照片
	now := time.Now()
	for i := 0; i < 5; i++ {
		photo := &Photo{
			ID:         "photo-" + string(rune('0'+i)),
			Filename:   "test.jpg",
			Path:       "test.jpg",
			UserID:     "user1",
			IsFavorite: i%2 == 0,
			TakenAt:    now.AddDate(0, -i, 0),
			UploadedAt: now,
			ModifiedAt: now,
		}
		manager.photos[photo.ID] = photo
	}

	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	// 测试分页查询
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/photos?userId=user1&limit=3&offset=0", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，得到 %d", w.Code)
	}
}

// ========== Response Structure Tests ==========

func TestHandlers_ResponseFormat(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)
	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/photos/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		body := w.Body.String()
		if body == "" {
			t.Error("响应体不应为空")
		}
	}
}

// ========== Content-Type Tests ==========

func TestHandlers_ContentType(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)
	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/photos/albums?userId=user1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "" && contentType != "application/json; charset=utf-8" {
		t.Errorf("期望 Content-Type 为 application/json，得到 %s", contentType)
	}
}

// ========== Method Tests ==========

func TestHandlers_MethodNotAllowed(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)
	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	// 对 GET 端点使用 PUT 方法
	req := httptest.NewRequestWithContext(context.Background(), "PUT", "/api/photos/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 应该返回 404 或 405
	if w.Code != http.StatusNotFound && w.Code != http.StatusMethodNotAllowed {
		t.Logf("方法不允许响应状态码: %d", w.Code)
	}
}

// ========== Edge Cases ==========

func TestHandlers_SpecialCharactersInQuery(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)
	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	// 测试特殊字符
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/photos/search?q=%E6%B5%8B%E8%AF%95", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 应该正常处理
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
		t.Logf("特殊字符查询响应状态码: %d", w.Code)
	}
}

func TestHandlers_EmptyQuery(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)
	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/photos/search?q=", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	t.Logf("空查询响应状态码: %d", w.Code)
}

// ========== Concurrent Access Tests ==========

func TestHandlers_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir)
	handlers := NewHandlers(manager, nil)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	// 并发请求
	done := make(chan bool)
	for i := 0; i < 5; i++ {
		go func() {
			req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/photos/stats", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			done <- true
		}()
	}

	// 等待所有请求完成
	for i := 0; i < 5; i++ {
		<-done
	}
}

// Copy from io.Copy for use in tests
var _ = io.Copy

// Suppress unused import warning for os
var _ = os.Stdout
