// Package videostation 提供视频站 HTTP API 处理器
package videostation

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handlers 视频站 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册 HTTP 路由
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	// 视频
	mux.HandleFunc("/api/v1/videostation/videos", h.handleVideos)
	mux.HandleFunc("/api/v1/videostation/videos/", h.handleVideoByID)
	mux.HandleFunc("/api/v1/videostation/videos/play", h.handlePlayVideo)

	// 转码
	mux.HandleFunc("/api/v1/videostation/videos/transcode", h.handleTranscode)
	mux.HandleFunc("/api/v1/videostation/videos/transcode/", h.handleTranscodeByID)

	// 字幕
	mux.HandleFunc("/api/v1/videostation/videos/subtitles", h.handleSubtitles)
	mux.HandleFunc("/api/v1/videostation/videos/subtitles/", h.handleSubtitleByID)

	// 视频库
	mux.HandleFunc("/api/v1/videostation/libraries", h.handleLibraries)
	mux.HandleFunc("/api/v1/videostation/libraries/", h.handleLibraryByID)
	mux.HandleFunc("/api/v1/videostation/libraries/scan", h.handleScanLibrary)

	// 播放会话
	mux.HandleFunc("/api/v1/videostation/sessions", h.handleSessions)
	mux.HandleFunc("/api/v1/videostation/sessions/", h.handleSessionByID)

	// 最近播放
	mux.HandleFunc("/api/v1/videostation/recent", h.handleRecent)

	// 统计
	mux.HandleFunc("/api/v1/videostation/stats", h.handleStats)
}

// apiResponse 标准 API 响应
type apiResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// writeJSON 写入 JSON 响应
func writeJSON(w http.ResponseWriter, status int, resp apiResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

// writeError 写入错误响应
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiResponse{Code: 1, Message: msg})
}

// extractID 从路径中提取 ID
func extractID(path, prefix string) string {
	id := strings.TrimPrefix(path, prefix)
	id = strings.TrimSuffix(id, "/")
	if idx := strings.Index(id, "/"); idx != -1 {
		id = id[:idx]
	}
	return id
}

// handleVideos 处理视频列表
func (h *Handlers) handleVideos(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	libraryID := r.URL.Query().Get("library_id")
	category := r.URL.Query().Get("category")
	tag := r.URL.Query().Get("tag")

	videos := h.manager.ListVideos(libraryID, category, tag)
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    videos,
	})
}

// handleVideoByID 处理单个视频操作
func (h *Handlers) handleVideoByID(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path, "/api/v1/videostation/videos/")
	if id == "" || id == "play" || id == "transcode" || id == "subtitles" {
		return
	}

	switch r.Method {
	case http.MethodGet:
		video, err := h.manager.GetVideo(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "success",
			Data:    video,
		})

	case http.MethodPut:
		var req UpdateVideoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
			return
		}

		video, err := h.manager.UpdateVideo(id, &req)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "video updated",
			Data:    video,
		})

	case http.MethodDelete:
		if err := h.manager.DeleteVideo(id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "video deleted",
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handlePlayVideo 处理播放请求
func (h *Handlers) handlePlayVideo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		VideoID string      `json:"video_id"`
		UserID  string      `json:"user_id"`
		PlayReq PlayRequest `json:"play"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if req.VideoID == "" {
		writeError(w, http.StatusBadRequest, "video_id is required")
		return
	}

	if req.UserID == "" {
		req.UserID = "default"
	}

	resp, err := h.manager.PlayVideo(req.VideoID, req.UserID, &req.PlayReq)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "playback ready",
		Data:    resp,
	})
}

// handleTranscode 处理转码任务列表
func (h *Handlers) handleTranscode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	videoID := r.URL.Query().Get("video_id")
	jobs := h.manager.ListTranscodeJobs(videoID)
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    jobs,
	})
}

// handleTranscodeByID 处理转码任务操作
func (h *Handlers) handleTranscodeByID(w http.ResponseWriter, r *http.Request) {
	// 从路径中提取 video_id 和 job_id
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/videostation/videos/transcode/")
	parts := strings.SplitN(path, "/", 3)

	switch r.Method {
	case http.MethodPost:
		// 创建转码任务: /videos/transcode/{video_id}
		videoID := parts[0]
		if videoID == "" {
			writeError(w, http.StatusBadRequest, "video_id is required")
			return
		}

		var req TranscodeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
			return
		}

		job, err := h.manager.CreateTranscodeJob(videoID, &req)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, apiResponse{
			Code:    0,
			Message: "transcode job created",
			Data:    job,
		})

	case http.MethodGet:
		// 获取转码任务: /videos/transcode/job/{job_id}
		if len(parts) >= 2 && parts[0] == "job" {
			job, err := h.manager.GetTranscodeJob(parts[1])
			if err != nil {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, apiResponse{
				Code:    0,
				Message: "success",
				Data:    job,
			})
		} else {
			// 列出视频的转码任务
			videoID := parts[0]
			jobs := h.manager.ListTranscodeJobs(videoID)
			writeJSON(w, http.StatusOK, apiResponse{
				Code:    0,
				Message: "success",
				Data:    jobs,
			})
		}

	case http.MethodDelete:
		// 取消转码任务: /videos/transcode/{job_id}/cancel
		if len(parts) >= 2 && parts[1] == "cancel" {
			if err := h.manager.CancelTranscodeJob(parts[0]); err != nil {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, apiResponse{
				Code:    0,
				Message: "transcode job cancelled",
			})
		} else {
			writeError(w, http.StatusBadRequest, "invalid path")
		}

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleSubtitles 处理字幕列表
func (h *Handlers) handleSubtitles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	videoID := r.URL.Query().Get("video_id")
	if videoID == "" {
		writeError(w, http.StatusBadRequest, "video_id is required")
		return
	}

	subs := h.manager.ListSubtitles(videoID)
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    subs,
	})
}

// handleSubtitleByID 处理字幕操作
func (h *Handlers) handleSubtitleByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/videostation/videos/subtitles/")
	parts := strings.SplitN(path, "/", 2)

	switch r.Method {
	case http.MethodPost:
		// 添加字幕: /subtitles/{video_id}
		videoID := parts[0]
		if videoID == "" {
			writeError(w, http.StatusBadRequest, "video_id is required")
			return
		}

		var sub Subtitle
		if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
			return
		}

		created, err := h.manager.AddSubtitle(videoID, &sub)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, apiResponse{
			Code:    0,
			Message: "subtitle added",
			Data:    created,
		})

	case http.MethodDelete:
		// 删除字幕: /subtitles/delete/{subtitle_id}
		if len(parts) >= 2 && parts[0] == "delete" {
			if err := h.manager.DeleteSubtitle(parts[1]); err != nil {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, apiResponse{
				Code:    0,
				Message: "subtitle deleted",
			})
		} else {
			writeError(w, http.StatusBadRequest, "invalid path")
		}

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleLibraries 处理视频库列表
func (h *Handlers) handleLibraries(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		libs := h.manager.ListLibraries()
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "success",
			Data:    libs,
		})

	case http.MethodPost:
		var req CreateLibraryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
			return
		}

		if req.Name == "" || req.Path == "" {
			writeError(w, http.StatusBadRequest, "name and path are required")
			return
		}

		lib := h.manager.CreateLibrary(&req)
		writeJSON(w, http.StatusCreated, apiResponse{
			Code:    0,
			Message: "library created",
			Data:    lib,
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleLibraryByID 处理视频库操作
func (h *Handlers) handleLibraryByID(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path, "/api/v1/videostation/libraries/")
	if id == "" || id == "scan" {
		return
	}

	switch r.Method {
	case http.MethodGet:
		lib, err := h.manager.GetLibrary(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "success",
			Data:    lib,
		})

	case http.MethodDelete:
		if err := h.manager.DeleteLibrary(id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "library deleted",
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleScanLibrary 处理视频库扫描
func (h *Handlers) handleScanLibrary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		LibraryID string `json:"library_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if req.LibraryID == "" {
		writeError(w, http.StatusBadRequest, "library_id is required")
		return
	}

	result, err := h.manager.ScanLibrary(req.LibraryID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "scan completed",
		Data:    result,
	})
}

// handleSessions 处理播放会话
func (h *Handlers) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID := r.URL.Query().Get("user_id")
	sessions := h.manager.GetSessions(userID)
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    sessions,
	})
}

// handleSessionByID 处理会话更新
func (h *Handlers) handleSessionByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	id := extractID(r.URL.Path, "/api/v1/videostation/sessions/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "session_id is required")
		return
	}

	var req SessionUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	session, err := h.manager.UpdateSession(id, &req)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "session updated",
		Data:    session,
	})
}

// handleRecent 处理最近播放
func (h *Handlers) handleRecent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID := r.URL.Query().Get("user_id")
	limit := 10

	videos := h.manager.GetRecentlyPlayed(userID, limit)
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    videos,
	})
}

// handleStats 处理统计请求
func (h *Handlers) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	stats := h.manager.GetStats()
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}
