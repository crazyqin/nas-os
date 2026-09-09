// update_routes_test.go - 更新检查路由测试（不触网：仅设置读写端点）.
package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nas-os/internal/config"

	"github.com/gin-gonic/gin"
)

func TestUpdateSettingsRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Default()
	cfg.Paths.DataDir = t.TempDir()
	s := &Server{cfg: cfg, engine: gin.New()}
	api := s.engine.Group("/api/v1")
	s.registerUpdateRoutes(api)

	// 默认设置
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/system/update-settings", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"autoCheck":false`) {
		t.Fatalf("GET 默认设置: %d %s", w.Code, w.Body.String())
	}

	// 保存
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/system/update-settings", strings.NewReader(`{"autoCheck":true}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT 设置: %d %s", w.Code, w.Body.String())
	}

	// 读回
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/system/update-settings", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"autoCheck":true`) {
		t.Fatalf("GET 回读: %d %s", w.Code, w.Body.String())
	}
}
