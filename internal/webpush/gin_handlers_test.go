// gin_handlers_test.go - Web Push 管理端点测试（不触网，推送走 httptest 本地服务）.
package webpush

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newHandlerRouter(t *testing.T) (*gin.Engine, *Manager) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	m := NewManager(t.TempDir())
	if err := m.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	r := gin.New()
	NewGinHandler(m).RegisterRoutes(r.Group("/api/v1"))
	return r, m
}

func doJSON(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequestWithContext(context.Background(), method, path, nil)
	} else {
		req = httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
	}
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestWebPushRoutesStatusEmpty(t *testing.T) {
	r, m := newHandlerRouter(t)
	w := doJSON(r, http.MethodGet, "/api/v1/webpush/status", "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET status: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"publicKey":"`) || !strings.Contains(w.Body.String(), `"subscriptions":0`) {
		t.Fatalf("初始状态应含非空公钥与 0 订阅：%s", w.Body.String())
	}
	if m.Count() != 0 {
		t.Fatalf("订阅数应为 0")
	}
}

func TestWebPushRoutesSubscribeFlow(t *testing.T) {
	r, _ := newHandlerRouter(t)
	p256dh, auth := genTestKeys(t)

	// 非法 body → 400
	if w := doJSON(r, http.MethodPost, "/api/v1/webpush/subscribe", `{}`); w.Code != http.StatusBadRequest {
		t.Fatalf("空 body 应 400：%d", w.Code)
	}
	if w := doJSON(r, http.MethodPost, "/api/v1/webpush/subscribe",
		`{"endpoint":"https://push.example.com/s","keys":{"p256dh":"","auth":"a"}}`); w.Code != http.StatusBadRequest {
		t.Fatalf("缺密钥应 400：%d", w.Code)
	}

	// 正常订阅（PushSubscription.toJSON 形态）
	body := `{"endpoint":"https://push.example.com/s/1","keys":{"p256dh":"` + p256dh + `","auth":"` + auth + `"}}`
	if w := doJSON(r, http.MethodPost, "/api/v1/webpush/subscribe", body); w.Code != http.StatusOK {
		t.Fatalf("订阅应 200：%d %s", w.Code, w.Body.String())
	}
	if w := doJSON(r, http.MethodGet, "/api/v1/webpush/status", ""); !strings.Contains(w.Body.String(), `"subscriptions":1`) {
		t.Fatalf("状态应含 1 订阅：%s", w.Body.String())
	}

	// 退订
	if w := doJSON(r, http.MethodPost, "/api/v1/webpush/unsubscribe", `{"endpoint":"https://push.example.com/s/1"}`); w.Code != http.StatusOK {
		t.Fatalf("退订应 200：%d %s", w.Code, w.Body.String())
	}
	if w := doJSON(r, http.MethodGet, "/api/v1/webpush/status", ""); !strings.Contains(w.Body.String(), `"subscriptions":0`) {
		t.Fatalf("退订后应为 0：%s", w.Body.String())
	}
	// 退订不存在的 endpoint 不算错误
	if w := doJSON(r, http.MethodPost, "/api/v1/webpush/unsubscribe", `{"endpoint":"https://push.example.com/nope"}`); w.Code != http.StatusOK {
		t.Fatalf("退订不存在端点应 200：%d", w.Code)
	}
}

func TestWebPushRoutesTestPush(t *testing.T) {
	r, _ := newHandlerRouter(t)

	// 无订阅 → 400 且不触网
	if w := doJSON(r, http.MethodPost, "/api/v1/webpush/test", `{}`); w.Code != http.StatusBadRequest {
		t.Fatalf("无订阅测试推送应 400：%d %s", w.Code, w.Body.String())
	}

	// 本地假推送服务 → 201 → 测试推送成功
	svc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer svc.Close()

	p256dh, auth := genTestKeys(t)
	body := `{"endpoint":"` + svc.URL + `","keys":{"p256dh":"` + p256dh + `","auth":"` + auth + `"}}`
	if w := doJSON(r, http.MethodPost, "/api/v1/webpush/subscribe", body); w.Code != http.StatusOK {
		t.Fatalf("订阅假推送服务应 200：%d %s", w.Code, w.Body.String())
	}
	if w := doJSON(r, http.MethodPost, "/api/v1/webpush/test", `{}`); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"sent":1`) {
		t.Fatalf("测试推送应送达 1：%d %s", w.Code, w.Body.String())
	}
}
