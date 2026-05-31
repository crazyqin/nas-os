package applifecycle

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Types 类型定义
type AppStatus struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	State     AppState  `json:"state"`
	Version   string    `json:"version"`
	Uptime    string    `json:"uptime,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type LifecycleEvent struct {
	AppID     string    `json:"app_id"`
	EventType string    `json:"event_type"`
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
	Details   string    `json:"details"`
}

// RegisterAPI 注册API端点
func RegisterAPI(mux *http.ServeMux, m *Manager) {
	handler := NewHandler(m)
	handler.RegisterRoutes(mux)
}

// GetAppStatus 获取应用状态摘要
func GetAppStatus(app *App) AppStatus {
	status := AppStatus{
		ID:        app.ID,
		Name:      app.Name,
		State:     app.State,
		Version:   app.Version,
		UpdatedAt: app.UpdatedAt,
	}

	if app.StartedAt != nil {
		status.Uptime = time.Since(*app.StartedAt).Round(time.Second).String()
	}

	return status
}

// BatchOperation 批量操作
func (m *Manager) BatchOperation(appIDs []string, operation string) map[string]error {
	results := make(map[string]error)

	for _, id := range appIDs {
		switch operation {
		case "stop":
			results[id] = m.Stop(id)
		case "start":
			results[id] = m.Start(id)
		default:
			results[id] = fmt.Errorf("未知操作: %s", operation)
		}
	}

	return results
}

// MarshalJSON 自定义JSON序列化
func (a *App) MarshalJSON() ([]byte, error) {
	type Alias App
	return json.Marshal(&struct {
		*Alias
		State string `json:"state"`
	}{
		Alias: (*Alias)(a),
		State: string(a.State),
	})
}
