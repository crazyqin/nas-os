package smartalbum

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestRouter() (*gin.Engine, *Handlers) {
	gin.SetMode(gin.TestMode)
	mgr := NewManager()
	handlers := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	return router, handlers
}

func TestAddPhotoAPI(t *testing.T) {
	router, _ := setupTestRouter()

	photo := Photo{
		Filename: "test.jpg",
		Path:     "/photos/test.jpg",
		Size:     1024,
		MimeType: "image/jpeg",
	}

	body, _ := json.Marshal(photo)
	req := httptest.NewRequest("POST", "/api/smart-album/photos", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	if response["code"] != float64(0) {
		t.Errorf("expected code 0, got %v", response["code"])
	}
}

func TestGetPhotoAPI(t *testing.T) {
	router, handlers := setupTestRouter()

	// 先添加照片
	photo := Photo{Filename: "test.jpg", Path: "/photos/test.jpg"}
	added, _ := handlers.mgr.AddPhoto(photo)

	req := httptest.NewRequest("GET", "/api/smart-album/photos/"+added.ID, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestListPhotosAPI(t *testing.T) {
	router, handlers := setupTestRouter()

	// 添加几张照片
	for i := 0; i < 3; i++ {
		handlers.mgr.AddPhoto(Photo{Filename: "photo.jpg", Path: "/photos/photo.jpg"})
	}

	req := httptest.NewRequest("GET", "/api/smart-album/photos?limit=2&offset=0", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestSemanticSearchAPI(t *testing.T) {
	router, handlers := setupTestRouter()

	// 添加带嵌入向量的照片
	handlers.mgr.AddPhoto(Photo{
		Filename:  "sunset.jpg",
		Embedding: []float64{0.9, 0.1, 0.0},
	})

	searchReq := SemanticSearchRequest{
		Embedding: []float64{0.85, 0.15, 0.05},
		TopK:      5,
		MinScore:  0.5,
	}

	body, _ := json.Marshal(searchReq)
	req := httptest.NewRequest("POST", "/api/smart-album/search/semantic", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestMapClustersAPI(t *testing.T) {
	router, handlers := setupTestRouter()

	// 添加带 GPS 的照片
	handlers.mgr.AddPhoto(Photo{
		Filename: "location.jpg",
		GPS: &GPSInfo{
			Latitude:  39.9042,
			Longitude: 116.4074,
			City:      "北京",
		},
	})

	req := httptest.NewRequest("GET", "/api/smart-album/map/clusters?zoom=10", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestGetTimelineAPI(t *testing.T) {
	router, _ := setupTestRouter()

	req := httptest.NewRequest("GET", "/api/smart-album/timeline", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestGetStatsAPI(t *testing.T) {
	router, _ := setupTestRouter()

	req := httptest.NewRequest("GET", "/api/smart-album/stats", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}
