// tls_routes_test.go - Web UI HTTPS 管理路由测试（不触网）.
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

// newTLSTestServer 构造仅挂载 TLS 路由的测试实例.
func newTLSTestServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := config.Default()
	cfg.Paths.DataDir = t.TempDir()
	s := &Server{cfg: cfg, engine: gin.New()}
	api := s.engine.Group("/api/v1")
	s.registerTLSRoutes(api)
	return s
}

func TestTLSRoutesStatusEmpty(t *testing.T) {
	s := newTLSTestServer(t)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/system/tls", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"hasCert":false`) {
		t.Fatalf("GET 初始状态: %d %s", w.Code, w.Body.String())
	}
}

func TestTLSRoutesGenerateAndToggle(t *testing.T) {
	s := newTLSTestServer(t)

	// 生成自签证书
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/system/tls/generate", strings.NewReader(`{"hosts":["nas-test.local"]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "self-signed") {
		t.Fatalf("POST generate: %d %s", w.Code, w.Body.String())
	}

	// 状态反映证书
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/system/tls", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"hasCert":true`) || !strings.Contains(w.Body.String(), "nas-test.local") {
		t.Fatalf("GET 生成后状态: %d %s", w.Code, w.Body.String())
	}

	// 开启跳转
	req = httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/system/tls/settings", strings.NewReader(`{"enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"enabled":true`) {
		t.Fatalf("PUT settings: %d %s", w.Code, w.Body.String())
	}

	// 删除证书 → 跳转联动关闭
	req = httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/system/tls/certificate", nil)
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"hasCert":false`) || !strings.Contains(w.Body.String(), `"enabled":false`) {
		t.Fatalf("DELETE certificate: %d %s", w.Code, w.Body.String())
	}
}

func TestTLSRoutesImportValidation(t *testing.T) {
	s := newTLSTestServer(t)

	// 缺字段
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/system/tls/import", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("空 body 应 400: %d", w.Code)
	}

	// 垃圾 PEM
	req = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/system/tls/import", strings.NewReader(`{"certPEM":"not a pem","keyPEM":"nope"}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("坏 PEM 应 400: %d %s", w.Code, w.Body.String())
	}
}

func TestTLSRoutesRedirectMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Default()
	cfg.Paths.DataDir = t.TempDir()
	s := &Server{cfg: cfg, engine: gin.New()}
	// 模拟构造期：中间件先挂，路由后注册
	s.engine.Use(s.httpsRedirectMiddleware())
	api := s.engine.Group("/api/v1")
	s.registerTLSRoutes(api)

	// 无证书：不跳转，正常响应
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/system/tls", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("无证书时不应跳转: %d", w.Code)
	}

	// 生成证书 + 开启跳转
	if st, err := s.tlsManager().Generate([]string{"redirect-test.local"}); err != nil {
		t.Fatalf("Generate: %v", err)
	} else if _, err := s.tlsManager().SetEnabled(true); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	} else if !st.HasCert {
		t.Fatal("生成失败")
	}

	// 非 loopback 明文请求 → 301
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/system/tls?x=1", nil)
	req.Host = "nas.example.com:8080"
	req.RemoteAddr = "192.168.1.50:55555"
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	if w.Code != http.StatusMovedPermanently || w.Header().Get("Location") != "https://nas.example.com:8080/api/v1/system/tls?x=1" {
		t.Fatalf("明文应 301: %d %s", w.Code, w.Header().Get("Location"))
	}

	// loopback 明文请求 → 豁免（探针/nasctl）
	req = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/system/tls", nil)
	req.Host = "nas.example.com:8080"
	req.RemoteAddr = "127.0.0.1:55555"
	w = httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("loopback 应豁免跳转: %d", w.Code)
	}
}
