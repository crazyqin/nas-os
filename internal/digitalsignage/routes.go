package digitalsignage

import (
	"encoding/json"
	"net/http"
	"time"
)

// RegisterRoutes 注册HTTP路由
func (m *Manager) RegisterRoutes(mux *http.ServeMux) {
	// 内容管理
	mux.HandleFunc("/api/signage/contents", m.handleContents)
	mux.HandleFunc("/api/signage/contents/", m.handleContentByID)

	// 播放列表管理
	mux.HandleFunc("/api/signage/playlists", m.handlePlaylists)
	mux.HandleFunc("/api/signage/playlists/", m.handlePlaylistByID)

	// 排程管理
	mux.HandleFunc("/api/signage/schedules", m.handleSchedules)
	mux.HandleFunc("/api/signage/schedules/", m.handleScheduleByID)
	mux.HandleFunc("/api/signage/schedules/urgent", m.handleUrgentInsert)

	// 设备管理
	mux.HandleFunc("/api/signage/devices", m.handleDevices)
	mux.HandleFunc("/api/signage/devices/", m.handleDeviceByID)
	mux.HandleFunc("/api/signage/devices/heartbeat", m.handleDeviceHeartbeat)
	mux.HandleFunc("/api/signage/devices/volume", m.handleDeviceVolume)
	mux.HandleFunc("/api/signage/devices/brightness", m.handleDeviceBrightness)

	// 设备组
	mux.HandleFunc("/api/signage/groups", m.handleDeviceGroups)

	// 模板
	mux.HandleFunc("/api/signage/templates", m.handleTemplates)
	mux.HandleFunc("/api/signage/templates/", m.handleTemplateByID)

	// 播放控制
	mux.HandleFunc("/api/signage/push", m.handlePush)
	mux.HandleFunc("/api/signage/stop", m.handleStop)
	mux.HandleFunc("/api/signage/status", m.handlePlaybackStatus)
}

func (m *Manager) handleContents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		var contentType *ContentType
		if ct := r.URL.Query().Get("type"); ct != "" {
			c := ContentType(ct)
			contentType = &c
		}
		contents := m.ListContents(contentType)
		writeJSON(w, contents)
	case http.MethodPost:
		var content Content
		if err := json.NewDecoder(r.Body).Decode(&content); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.CreateContent(&content); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, content)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleContentByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/signage/contents/"):]
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		content, err := m.GetContent(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, content)
	case http.MethodPut:
		var content Content
		if err := json.NewDecoder(r.Body).Decode(&content); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		content.ID = id
		if err := m.UpdateContent(&content); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, content)
	case http.MethodDelete:
		if err := m.DeleteContent(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handlePlaylists(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		playlists := m.ListPlaylists()
		writeJSON(w, playlists)
	case http.MethodPost:
		var playlist Playlist
		if err := json.NewDecoder(r.Body).Decode(&playlist); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.CreatePlaylist(&playlist); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, playlist)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handlePlaylistByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/signage/playlists/"):]
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		playlist, err := m.GetPlaylist(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, playlist)
	case http.MethodPut:
		var playlist Playlist
		if err := json.NewDecoder(r.Body).Decode(&playlist); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		playlist.ID = id
		if err := m.UpdatePlaylist(&playlist); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, playlist)
	case http.MethodDelete:
		if err := m.DeletePlaylist(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleSchedules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		schedules := m.ListSchedules()
		writeJSON(w, schedules)
	case http.MethodPost:
		var schedule Schedule
		if err := json.NewDecoder(r.Body).Decode(&schedule); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.CreateSchedule(&schedule); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, schedule)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleScheduleByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/signage/schedules/"):]
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		schedule, err := m.GetSchedule(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, schedule)
	case http.MethodPut:
		var schedule Schedule
		if err := json.NewDecoder(r.Body).Decode(&schedule); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		schedule.ID = id
		if err := m.UpdateSchedule(&schedule); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, schedule)
	case http.MethodDelete:
		if err := m.DeleteSchedule(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleUrgentInsert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		PlaylistID string   `json:"playlist_id"`
		DeviceIDs  []string `json:"device_ids"`
		Duration   int      `json:"duration_seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Duration <= 0 {
		req.Duration = 300 // 默认5分钟
	}

	if err := m.UrgentInsert(req.PlaylistID, req.DeviceIDs, time.Duration(req.Duration)*time.Second); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "ok", "message": "紧急插播已创建"})
}

func (m *Manager) handleDevices(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		var group *string
		if g := r.URL.Query().Get("group"); g != "" {
			group = &g
		}
		devices := m.ListDevices(group)
		writeJSON(w, devices)
	case http.MethodPost:
		var device Device
		if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.RegisterDevice(&device); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, device)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleDeviceByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/signage/devices/"):]
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		device, err := m.GetDevice(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, device)
	case http.MethodPut:
		var device Device
		if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		device.ID = id
		if err := m.UpdateDevice(&device); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, device)
	case http.MethodDelete:
		if err := m.DeleteDevice(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleDeviceHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DeviceID string `json:"device_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := m.DeviceHeartbeat(req.DeviceID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "ok"})
}

func (m *Manager) handleDeviceVolume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DeviceID string `json:"device_id"`
		Volume   int    `json:"volume"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := m.SetDeviceVolume(req.DeviceID, req.Volume); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "ok"})
}

func (m *Manager) handleDeviceBrightness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DeviceID   string `json:"device_id"`
		Brightness int    `json:"brightness"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := m.SetDeviceBrightness(req.DeviceID, req.Brightness); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "ok"})
}

func (m *Manager) handleDeviceGroups(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		groups := m.ListDeviceGroups()
		writeJSON(w, groups)
	case http.MethodPost:
		var group DeviceGroup
		if err := json.NewDecoder(r.Body).Decode(&group); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.CreateDeviceGroup(&group); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, group)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	templates := m.ListTemplates()
	writeJSON(w, templates)
}

func (m *Manager) handleTemplateByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/signage/templates/"):]
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tpl, err := m.GetTemplate(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, tpl)
}

func (m *Manager) handlePush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DeviceID   string `json:"device_id"`
		PlaylistID string `json:"playlist_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := m.PushToDevice(req.DeviceID, req.PlaylistID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "ok", "message": "推送成功"})
}

func (m *Manager) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DeviceID string `json:"device_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := m.StopDevice(req.DeviceID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "ok", "message": "停止播放"})
}

func (m *Manager) handlePlaybackStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	deviceID := r.URL.Query().Get("device_id")
	if deviceID == "" {
		http.Error(w, "device_id required", http.StatusBadRequest)
		return
	}

	status, err := m.GetPlaybackStatus(deviceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, status)
}
