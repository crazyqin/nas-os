// Package webpush 提供浏览器 Web Push（VAPID）推送：密钥对生成与持久化、
// 订阅管理、加密推送发送。
//
// 设计要点：
//   - 状态目录为 DataDir/webpush（config 目录在容器部署常为只读挂载）。
//   - VAPID 密钥首次生成后持久化、不再更换——重新生成会使全部既有订阅失效。
//   - 订阅按 endpoint 去重；推送服务返回 404/410 视为订阅失效，自动清除。
//   - 发送走 RFC8291 报文加密 + VAPID 签名（webpush-go），离线消息 TTL 24 小时。
package webpush

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	webpushgo "github.com/SherClockHolmes/webpush-go"
)

const (
	// vapidFile / subsFile 状态目录下的持久化文件名.
	vapidFile = "vapid.json"
	subsFile  = "subscriptions.json"

	// defaultSubscriber VAPID JWT 的 Subscriber 联系方式（惯例填 mailto）.
	defaultSubscriber = "mailto:admin@nasos.local"

	// pushTTL 离线消息在推送服务侧的保留秒数（24 小时）.
	pushTTL = 24 * 60 * 60

	// titleMaxRunes / bodyMaxRunes 推送载荷截断上限（加密记录上限约 4KB）.
	titleMaxRunes = 120
	bodyMaxRunes  = 640

	// maxSubscriptions 单实例订阅数上限（防滥用）.
	maxSubscriptions = 100
)

// VAPIDKeys VAPID 密钥对（base64url）.
type VAPIDKeys struct {
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
}

// Subscription 浏览器推送订阅（endpoint 为去重主键）.
type Subscription struct {
	Endpoint  string    `json:"endpoint"`
	P256dh    string    `json:"p256dh"`
	Auth      string    `json:"auth"`
	UserAgent string    `json:"userAgent,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	LastSeen  time.Time `json:"lastSeen,omitempty"`
}

// Manager 管理 VAPID 密钥与订阅列表（并发安全）.
type Manager struct {
	dir  string
	mu   sync.Mutex
	keys VAPIDKeys
	subs []*Subscription
}

// NewManager 创建管理器，dir 为状态目录（DataDir/webpush）.
func NewManager(dir string) *Manager {
	return &Manager{dir: dir}
}

// Load 读取密钥与订阅；密钥缺失则生成并持久化（只生成一次）.
func (m *Manager) Load() error {
	if err := os.MkdirAll(m.dir, 0o700); err != nil {
		return fmt.Errorf("创建目录 %s 失败：%w", m.dir, err)
	}

	data, err := os.ReadFile(filepath.Join(m.dir, vapidFile))
	switch {
	case errors.Is(err, os.ErrNotExist):
		priv, pub, err := webpushgo.GenerateVAPIDKeys()
		if err != nil {
			return fmt.Errorf("生成 VAPID 密钥失败：%w", err)
		}
		m.keys = VAPIDKeys{PrivateKey: priv, PublicKey: pub}
		if err := m.persistVAPID(); err != nil {
			return err
		}
	case err != nil:
		return fmt.Errorf("读取 VAPID 密钥失败：%w", err)
	default:
		if err := json.Unmarshal(data, &m.keys); err != nil {
			return fmt.Errorf("解析 VAPID 密钥失败：%w", err)
		}
		if m.keys.PublicKey == "" || m.keys.PrivateKey == "" {
			return errors.New("VAPID 密钥文件不完整")
		}
	}

	subsData, err := os.ReadFile(filepath.Join(m.dir, subsFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取订阅列表失败：%w", err)
	}
	var list []*Subscription
	if err := json.Unmarshal(subsData, &list); err != nil {
		return fmt.Errorf("解析订阅列表失败：%w", err)
	}
	m.subs = list
	return nil
}

// PublicKey 返回 VAPID 公钥（base64url，前端订阅时使用）.
func (m *Manager) PublicKey() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.keys.PublicKey
}

// Count 返回当前订阅数.
func (m *Manager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.subs)
}

// Subscribe 新增或更新订阅（按 endpoint 去重）.
func (m *Manager) Subscribe(sub *Subscription) error {
	if sub == nil || sub.Endpoint == "" || sub.P256dh == "" || sub.Auth == "" {
		return errors.New("订阅信息不完整（endpoint/p256dh/auth 均不能为空）")
	}
	if !strings.HasPrefix(sub.Endpoint, "http://") && !strings.HasPrefix(sub.Endpoint, "https://") {
		return errors.New("无效的订阅 endpoint")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for _, s := range m.subs {
		if s.Endpoint == sub.Endpoint {
			s.P256dh, s.Auth, s.UserAgent, s.LastSeen = sub.P256dh, sub.Auth, sub.UserAgent, now
			return m.saveSubsLocked()
		}
	}
	if len(m.subs) >= maxSubscriptions {
		return fmt.Errorf("订阅数已达上限（%d）", maxSubscriptions)
	}
	sub.CreatedAt = now
	sub.LastSeen = now
	m.subs = append(m.subs, sub)
	return m.saveSubsLocked()
}

// Unsubscribe 按 endpoint 删除订阅（不存在不算错误）.
func (m *Manager) Unsubscribe(endpoint string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	kept := m.subs[:0]
	removed := false
	for _, s := range m.subs {
		if s.Endpoint != endpoint {
			kept = append(kept, s)
		} else {
			removed = true
		}
	}
	if !removed {
		return nil
	}
	m.subs = kept
	return m.saveSubsLocked()
}

// PushPayload 推送到 service worker 的载荷结构.
type PushPayload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Icon  string `json:"icon"`
	Badge string `json:"badge"`
	Tag   string `json:"tag"`
	URL   string `json:"url"`
	Level string `json:"level,omitempty"`
}

// Send 向全部订阅推送一条通知，返回成功数。
// 推送服务返回 404/410 的订阅视为失效并自动清除；部分失败不阻断其余订阅。
func (m *Manager) Send(ctx context.Context, title, body, level, url string) (int, error) {
	// 快照后锁外发送：慢推送服务不阻塞状态查询与订阅操作.
	m.mu.Lock()
	keys := m.keys
	subs := make([]*Subscription, len(m.subs))
	copy(subs, m.subs)
	m.mu.Unlock()
	if keys.PublicKey == "" || keys.PrivateKey == "" {
		return 0, errors.New("VAPID 密钥未初始化")
	}
	if len(subs) == 0 {
		return 0, errors.New("没有已订阅的浏览器")
	}
	if url == "" {
		url = "/"
	}
	payload, err := json.Marshal(PushPayload{
		Title: truncateRunes(title, titleMaxRunes),
		Body:  truncateRunes(body, bodyMaxRunes),
		Icon:  "/brand/logo/logo-192.png",
		Badge: "/brand/logo/logo-72.png",
		Tag:   "nas-os",
		URL:   url,
		Level: level,
	})
	if err != nil {
		return 0, err
	}

	urgency := webpushgo.UrgencyNormal
	if level == "critical" || level == "error" {
		urgency = webpushgo.UrgencyHigh
	}

	sent := 0
	var dead []string
	var errs []string
	for _, s := range subs {
		target := &webpushgo.Subscription{
			Endpoint: s.Endpoint,
			Keys:     webpushgo.Keys{Auth: s.Auth, P256dh: s.P256dh},
		}
		resp, err := webpushgo.SendNotificationWithContext(ctx, payload, target, &webpushgo.Options{
			Subscriber:      defaultSubscriber,
			TTL:             pushTTL,
			Urgency:         urgency,
			VAPIDPublicKey:  keys.PublicKey,
			VAPIDPrivateKey: keys.PrivateKey,
		})
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		status := resp.StatusCode
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		switch status {
		case http.StatusCreated, http.StatusOK:
			sent++
		case http.StatusNotFound, http.StatusGone:
			dead = append(dead, s.Endpoint)
		default:
			errs = append(errs, fmt.Sprintf("推送服务返回 %d", status))
		}
	}

	if len(dead) > 0 {
		m.mu.Lock()
		kept := m.subs[:0]
		for _, s := range m.subs {
			if !slices.Contains(dead, s.Endpoint) {
				kept = append(kept, s)
			}
		}
		m.subs = kept
		err := m.saveSubsLocked()
		m.mu.Unlock()
		if err != nil {
			errs = append(errs, "保存订阅列表失败: "+err.Error())
		}
	}

	if sent == 0 {
		if len(errs) > 0 {
			return 0, errors.New(strings.Join(errs, "；"))
		}
		return 0, errors.New("全部订阅已失效，已自动清除")
	}
	if len(errs) > 0 {
		return sent, fmt.Errorf("部分订阅发送失败：%s", strings.Join(errs, "；"))
	}
	return sent, nil
}

// ---- 内部实现 ----

// persistVAPID 落盘 VAPID 密钥（仅初始化路径调用，无需持锁）.
func (m *Manager) persistVAPID() error {
	data, err := json.MarshalIndent(m.keys, "", "  ") // #nosec G117 -- VAPID 私钥落盘为设计意图：仅写 DataDir/webpush/vapid.json（0600），与 TLS key.pem 同类，不出网不进日志
	if err != nil {
		return err
	}
	if err := writeAtomic(filepath.Join(m.dir, vapidFile), data, 0o600); err != nil {
		return fmt.Errorf("写入 VAPID 密钥失败：%w", err)
	}
	return nil
}

// saveSubsLocked 落盘订阅列表；调用方持有锁.
func (m *Manager) saveSubsLocked() error {
	data, err := json.MarshalIndent(m.subs, "", "  ")
	if err != nil {
		return err
	}
	if err := writeAtomic(filepath.Join(m.dir, subsFile), data, 0o600); err != nil {
		return fmt.Errorf("写入订阅列表失败：%w", err)
	}
	return nil
}

// writeAtomic 临时文件 + 原子改名（与 webtls/sysupdate 持久化策略一致）.
func writeAtomic(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// truncateRunes 按字符数截断，避免超长内容撑爆加密载荷.
func truncateRunes(s string, max int) string {
	if max <= 0 {
		return s
	}
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	return string(r[:max]) + "…"
}
