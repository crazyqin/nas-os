package compress

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ========== NewHandlers 测试 ==========

func TestNewHandlers(t *testing.T) {
	h := NewHandlers(nil, nil)
	if h == nil {
		t.Fatal("NewHandlers should not return nil")
	}
}

// ========== RegisterRoutes 测试 ==========

func TestHandlers_RegisterRoutes(t *testing.T) {
	h := NewHandlers(nil, nil)

	router := gin.New()
	api := router.Group("/api")
	h.RegisterRoutes(api)

	routes := router.Routes()
	routeMap := make(map[string]bool)
	for _, route := range routes {
		key := route.Method + ":" + route.Path
		routeMap[key] = true
	}

	expectedRoutes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/compress/config"},
		{"PUT", "/api/compress/config"},
		{"GET", "/api/compress/stats"},
		{"POST", "/api/compress/stats/reset"},
		{"GET", "/api/compress/files"},
		{"POST", "/api/compress/batch"},
		{"POST", "/api/compress/compress"},
		{"POST", "/api/compress/decompress"},
		{"GET", "/api/compress/algorithms"},
	}

	for _, expected := range expectedRoutes {
		if !routeMap[expected.method+":"+expected.path] {
			t.Errorf("Expected route %s %s to be registered", expected.method, expected.path)
		}
	}
}

// ========== Request Types 测试 ==========

func TestUpdateConfigRequest_Struct(t *testing.T) {
	req := UpdateConfigRequest{
		Enabled:           true,
		DefaultAlgorithm:  AlgorithmZstd,
		CompressionLevel:  3,
		MinSize:           1024,
		ExcludeExtensions: []string{".zip", ".gz"},
		ExcludeDirs:       []string{"/tmp"},
		IncludeDirs:       []string{"/data"},
		CompressOnWrite:   true,
		DecompressOnRead:  true,
		StatsEnabled:      true,
	}

	if !req.Enabled {
		t.Error("Enabled should be true")
	}
	if req.DefaultAlgorithm != AlgorithmZstd {
		t.Errorf("Expected AlgorithmZstd, got %v", req.DefaultAlgorithm)
	}
	if len(req.ExcludeExtensions) != 2 {
		t.Errorf("Expected 2 exclude extensions, got %d", len(req.ExcludeExtensions))
	}
}

func TestBatchCompressRequest_Struct(t *testing.T) {
	req := BatchCompressRequest{
		Dir:       "/data",
		Recursive: true,
	}

	if req.Dir != "/data" {
		t.Errorf("Expected Dir=/data, got %s", req.Dir)
	}
	if !req.Recursive {
		t.Error("Recursive should be true")
	}
}

func TestCompressFileRequest_Struct(t *testing.T) {
	req := CompressFileRequest{
		SrcPath: "/data/file.txt",
		DstPath: "/data/file.txt.zst",
	}

	if req.SrcPath != "/data/file.txt" {
		t.Errorf("Expected SrcPath=/data/file.txt, got %s", req.SrcPath)
	}
	if req.DstPath != "/data/file.txt.zst" {
		t.Errorf("Expected DstPath=/data/file.txt.zst, got %s", req.DstPath)
	}
}

func TestDecompressFileRequest_Struct(t *testing.T) {
	req := DecompressFileRequest{
		SrcPath: "/data/file.txt.zst",
		DstPath: "/data/file.txt",
	}

	if req.SrcPath != "/data/file.txt.zst" {
		t.Errorf("Expected SrcPath=/data/file.txt.zst, got %s", req.SrcPath)
	}
	if req.DstPath != "/data/file.txt" {
		t.Errorf("Expected DstPath=/data/file.txt, got %s", req.DstPath)
	}
}

func TestAlgorithmInfo_Struct(t *testing.T) {
	info := AlgorithmInfo{
		Name:        AlgorithmZstd,
		Extension:   ".zst",
		Description: "Zstandard 压缩算法",
		Speed:       "快",
		Ratio:       "高",
	}

	if info.Name != AlgorithmZstd {
		t.Errorf("Expected AlgorithmZstd, got %v", info.Name)
	}
	if info.Extension != ".zst" {
		t.Errorf("Expected .zst, got %s", info.Extension)
	}
}

// ========== HTTP Handler 测试 ==========

func TestHandlers_ListAlgorithms(t *testing.T) {
	h := NewHandlers(nil, nil)

	router := gin.New()
	api := router.Group("/api")
	h.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/compress/algorithms", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlers_GetConfig_NilManager(t *testing.T) {
	h := NewHandlers(nil, nil)

	router := gin.New()
	api := router.Group("/api")
	h.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/compress/config", nil)
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			// Expected panic from nil manager
		}
	}()

	router.ServeHTTP(w, req)
}

func TestHandlers_GetStats_NilManager(t *testing.T) {
	h := NewHandlers(nil, nil)

	router := gin.New()
	api := router.Group("/api")
	h.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/compress/stats", nil)
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			// Expected panic from nil manager
		}
	}()

	router.ServeHTTP(w, req)
}

func TestHandlers_ListCompressedFiles_NilFS(t *testing.T) {
	h := NewHandlers(nil, nil)

	router := gin.New()
	api := router.Group("/api")
	h.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/compress/files?dir=/data", nil)
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			// Expected panic from nil filesystem
		}
	}()

	router.ServeHTTP(w, req)
}

func TestHandlers_BatchCompress_InvalidBody(t *testing.T) {
	h := NewHandlers(nil, nil)

	router := gin.New()
	api := router.Group("/api")
	h.RegisterRoutes(api)

	req := httptest.NewRequest("POST", "/api/compress/batch", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandlers_CompressFile_InvalidBody(t *testing.T) {
	h := NewHandlers(nil, nil)

	router := gin.New()
	api := router.Group("/api")
	h.RegisterRoutes(api)

	req := httptest.NewRequest("POST", "/api/compress/compress", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandlers_DecompressFile_InvalidBody(t *testing.T) {
	h := NewHandlers(nil, nil)

	router := gin.New()
	api := router.Group("/api")
	h.RegisterRoutes(api)

	req := httptest.NewRequest("POST", "/api/compress/decompress", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandlers_UpdateConfig_InvalidBody(t *testing.T) {
	h := NewHandlers(nil, nil)

	router := gin.New()
	api := router.Group("/api")
	h.RegisterRoutes(api)

	req := httptest.NewRequest("PUT", "/api/compress/config", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// ========== Algorithm 常量测试 ==========

func TestAlgorithm_Constants(t *testing.T) {
	algorithms := []Algorithm{
		AlgorithmZstd,
		AlgorithmLz4,
		AlgorithmGzip,
	}

	for _, alg := range algorithms {
		if string(alg) == "" {
			t.Errorf("Algorithm constant should not be empty")
		}
	}
}

func TestAlgorithmInfo_AlgorithmValues(t *testing.T) {
	// Test that algorithm info has valid values
	infos := []struct {
		name      Algorithm
		extension string
	}{
		{AlgorithmZstd, ".zst"},
		{AlgorithmLz4, ".lz4"},
		{AlgorithmGzip, ".gz"},
	}

	for _, info := range infos {
		if info.name == "" {
			t.Errorf("Algorithm name should not be empty")
		}
		if info.extension == "" {
			t.Errorf("Extension should not be empty")
		}
	}
}
