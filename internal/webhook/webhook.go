package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// Webhook represents a registered webhook endpoint.
type Webhook struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	URL         string            `json:"url"`
	Events      []string          `json:"events"`    // e.g., "backup.complete", "alert.critical"
	Headers     map[string]string `json:"headers"`
	Enabled     bool              `json:"enabled"`
	Secret      string            `json:"secret,omitempty"`
	LastTrigger time.Time         `json:"last_trigger"`
	FailCount   int               `json:"fail_count"`
}

// Event represents a webhook event payload.
type Event struct {
	Type      string      `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// Manager manages webhooks.
type Manager struct {
	mu        sync.RWMutex
	webhooks  map[string]*Webhook
	client    *http.Client
	queue     chan eventJob
	stopCh    chan struct{}
}

type eventJob struct {
	webhook *Webhook
	event   Event
}

// NewManager creates a new webhook manager.
func NewManager() *Manager {
	m := &Manager{
		webhooks: make(map[string]*Webhook),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		queue:  make(chan eventJob, 1000),
		stopCh: make(chan struct{}),
	}
	go m.worker()
	return m
}

// Stop stops the webhook manager.
func (m *Manager) Stop() {
	close(m.stopCh)
}

// AddWebhook registers a new webhook.
func (m *Manager) AddWebhook(wh Webhook) {
	m.mu.Lock()
	defer m.mu.Unlock()
	wh.Enabled = true
	m.webhooks[wh.ID] = &wh
	log.Printf("Webhook已注册: %s -> %s", wh.Name, wh.URL)
}

// RemoveWebhook removes a webhook.
func (m *Manager) RemoveWebhook(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.webhooks, id)
}

// UpdateWebhook updates a webhook.
func (m *Manager) UpdateWebhook(wh Webhook) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.webhooks[wh.ID]; !ok {
		return fmt.Errorf("webhook not found: %s", wh.ID)
	}
	m.webhooks[wh.ID] = &wh
	return nil
}

// ListWebhooks returns all webhooks.
func (m *Manager) ListWebhooks() []Webhook {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Webhook, 0, len(m.webhooks))
	for _, w := range m.webhooks {
		result = append(result, *w)
	}
	return result
}

// FireEvent sends an event to all matching webhooks.
func (m *Manager) FireEvent(event Event) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, wh := range m.webhooks {
		if !wh.Enabled {
			continue
		}
		if !m.matchesEvent(wh, event.Type) {
			continue
		}
		select {
		case m.queue <- eventJob{webhook: wh, event: event}:
		default:
			log.Printf("Webhook队列已满，丢弃事件: %s -> %s", event.Type, wh.Name)
		}
	}
}

func (m *Manager) matchesEvent(wh *Webhook, eventType string) bool {
	if len(wh.Events) == 0 {
		return true // no filter = all events
	}
	for _, e := range wh.Events {
		if e == eventType || e == "*" {
			return true
		}
	}
	return false
}

func (m *Manager) worker() {
	for {
		select {
		case <-m.stopCh:
			return
		case job := <-m.queue:
			m.sendWebhook(job)
		}
	}
}

func (m *Manager) sendWebhook(job eventJob) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	payload, err := json.Marshal(job.event)
	if err != nil {
		log.Printf("Webhook序列化失败: %v", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, "POST", job.webhook.URL, bytes.NewReader(payload))
	if err != nil {
		log.Printf("Webhook请求创建失败: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "NAS-OS-Webhook/1.0")
	for k, v := range job.webhook.Headers {
		req.Header.Set(k, v)
	}

	// Retry up to 3 times
	for attempt := 0; attempt < 3; attempt++ {
		resp, err := m.client.Do(req)
		if err != nil {
			log.Printf("Webhook发送失败 (attempt %d/3): %s -> %v", attempt+1, job.webhook.Name, err)
			time.Sleep(time.Duration(attempt+1) * 5 * time.Second)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			m.mu.Lock()
			job.webhook.LastTrigger = time.Now()
			job.webhook.FailCount = 0
			m.mu.Unlock()
			return
		}

		log.Printf("Webhook返回非2xx (attempt %d/3): %s -> %d", attempt+1, job.webhook.Name, resp.StatusCode)
		time.Sleep(time.Duration(attempt+1) * 5 * time.Second)
	}

	m.mu.Lock()
	job.webhook.FailCount++
	if job.webhook.FailCount >= 10 {
		job.webhook.Enabled = false
		log.Printf("⚠️ Webhook连续失败10次，已禁用: %s", job.webhook.Name)
	}
	m.mu.Unlock()
}
