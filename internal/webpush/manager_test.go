package webpush

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// genTestKeys 生成真实可用的订阅密钥（webpush-go 加密需要合法的 P-256 公钥点与 16 字节 auth）.
func genTestKeys(t *testing.T) (p256dh, auth string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("生成测试密钥: %v", err)
	}
	pk, err := key.PublicKey.ECDH()
	if err != nil {
		t.Fatalf("生成 ECDH 公钥: %v", err)
	}
	pt := pk.Bytes()
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("生成 auth: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(pt), base64.RawURLEncoding.EncodeToString(raw)
}

func TestVAPIDKeysPersistAcrossReload(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	if err := m.Load(); err != nil {
		t.Fatalf("首次 Load: %v", err)
	}
	if m.PublicKey() == "" {
		t.Fatal("公钥为空")
	}
	// 重新打开同一目录：密钥不应重新生成（否则既有订阅全部失效）.
	m2 := NewManager(dir)
	if err := m2.Load(); err != nil {
		t.Fatalf("二次 Load: %v", err)
	}
	if m2.PublicKey() != m.PublicKey() {
		t.Fatal("重开后 VAPID 公钥变化，会使既有订阅失效")
	}
}

func TestSubscribeValidation(t *testing.T) {
	m := NewManager(t.TempDir())
	if err := m.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := m.Subscribe(&Subscription{Endpoint: "", P256dh: "p", Auth: "a"}); err == nil {
		t.Fatal("空 endpoint 应报错")
	}
	if err := m.Subscribe(&Subscription{Endpoint: "ftp://push.example.com", P256dh: "p", Auth: "a"}); err == nil {
		t.Fatal("非 http(s) endpoint 应报错")
	}
	if err := m.Subscribe(&Subscription{Endpoint: "https://push.example.com/s", P256dh: "", Auth: "a"}); err == nil {
		t.Fatal("空 p256dh 应报错")
	}
}

func TestSubscribePersistenceAndDedupe(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	if err := m.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	endpoint := "https://push.example.com/sub/1"
	if err := m.Subscribe(&Subscription{Endpoint: endpoint, P256dh: "p1", Auth: "a1"}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	// 同 endpoint 重复订阅 → 更新而非追加.
	if err := m.Subscribe(&Subscription{Endpoint: endpoint, P256dh: "p2", Auth: "a2"}); err != nil {
		t.Fatalf("重复 Subscribe: %v", err)
	}
	if m.Count() != 1 {
		t.Fatalf("同 endpoint 应去重: %d", m.Count())
	}
	// 落盘后重开仍在.
	m2 := NewManager(dir)
	if err := m2.Load(); err != nil {
		t.Fatalf("重开 Load: %v", err)
	}
	if m2.Count() != 1 {
		t.Fatalf("订阅应持久化: %d", m2.Count())
	}
	// 退订后重开为空.
	if err := m2.Unsubscribe(endpoint); err != nil {
		t.Fatalf("Unsubscribe: %v", err)
	}
	m3 := NewManager(dir)
	if err := m3.Load(); err != nil {
		t.Fatalf("退订后重开 Load: %v", err)
	}
	if m3.Count() != 0 {
		t.Fatalf("退订应持久化: %d", m3.Count())
	}
}

func TestSendNoSubscribers(t *testing.T) {
	m := NewManager(t.TempDir())
	if err := m.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	sent, err := m.Send(context.Background(), "标题", "内容", "info", "")
	if sent != 0 || err == nil || !strings.Contains(err.Error(), "没有已订阅") {
		t.Fatalf("无订阅时应报错: sent=%d err=%v", sent, err)
	}
}

func TestSendRemovesDeadSubscriptions(t *testing.T) {
	// 假推送服务：先正常受理（201），再切换为 410 Gone 模拟订阅失效.
	status := http.StatusCreated
	svc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(status)
	}))
	defer svc.Close()

	m := NewManager(t.TempDir())
	if err := m.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	p256dh, auth := genTestKeys(t)
	if err := m.Subscribe(&Subscription{Endpoint: svc.URL, P256dh: p256dh, Auth: auth}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	sent, err := m.Send(context.Background(), "标题", "内容", "info", "")
	if err != nil || sent != 1 {
		t.Fatalf("首次推送应成功: sent=%d err=%v", sent, err)
	}

	status = http.StatusGone
	sent, err = m.Send(context.Background(), "标题", "内容", "info", "")
	if sent != 0 || err == nil {
		t.Fatalf("订阅失效后应推送失败: sent=%d err=%v", sent, err)
	}
	if m.Count() != 0 {
		t.Fatalf("失效订阅应被自动清除: %d", m.Count())
	}
}
