// Package api provides HTTP API handlers for the media center
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"nas-os/internal/media"
)

// MediaCenterAPI provides API handlers for the media center
type MediaCenterAPI struct {
	scraper    *media.UnifiedScraper
	transcoder *media.Transcoder
	streamer   *media.StreamServer
}

// NewMediaCenterAPI creates a new media center API
func NewMediaCenterAPI(
	scraper *media.UnifiedScraper,
	transcoder *media.Transcoder,
	streamer *media.StreamServer,
) *MediaCenterAPI {
	return &MediaCenterAPI{
		scraper:    scraper,
		transcoder: transcoder,
		streamer:   streamer,
	}
}

// RegisterRoutes registers API routes
func (api *MediaCenterAPI) RegisterRoutes(mux *http.ServeMux) {
	// Metadata scraping
	mux.HandleFunc("POST /api/media/scrape", api.ScrapeMedia)
	mux.HandleFunc("POST /api/media/batch-scrape", api.BatchScrapeMedia)
	mux.HandleFunc("GET /api/media/sources", api.GetSources)

	// Dolby/HDR configuration
	mux.HandleFunc("POST /api/media/analyze", api.AnalyzeMedia)
	mux.HandleFunc("GET /api/media/playback-config", api.GetPlaybackConfig)
	mux.HandleFunc("POST /api/media/dolby-config", api.SetDolbyConfig)

	// Transcoding
	mux.HandleFunc("POST /api/media/transcode", api.CreateTranscodeJob)
	mux.HandleFunc("GET /api/media/transcode/{id}", api.GetTranscodeJob)
	mux.HandleFunc("DELETE /api/media/transcode/{id}", api.CancelTranscodeJob)
	mux.HandleFunc("GET /api/media/transcode", api.ListTranscodeJobs)

	// Streaming
	mux.HandleFunc("POST /api/media/stream/hls", api.CreateHLSStream)
	mux.HandleFunc("POST /api/media/stream/dash", api.CreateDASHStream)
	mux.HandleFunc("GET /api/media/stream/{id}", api.GetStreamSession)
	mux.HandleFunc("DELETE /api/media/stream/{id}", api.StopStreamSession)
}

// ScrapeMedia handles media metadata scraping
func (api *MediaCenterAPI) ScrapeMedia(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title    string          `json:"title"`
		Year     int             `json:"year,omitempty"`
		Type     media.MediaType `json:"type,omitempty"`
		Filename string          `json:"filename,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var result interface{}
	var err error

	if req.Filename != "" {
		result, err = api.scraper.AutoScrape(r.Context(), req.Filename)
	} else if req.Type == media.MediaTypeTVShow {
		result, err = api.scraper.ScrapeTVShow(r.Context(), req.Title)
	} else {
		result, err = api.scraper.ScrapeMovie(r.Context(), req.Title, req.Year)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(result)
}

// BatchScrapeMedia handles batch scraping
func (api *MediaCenterAPI) BatchScrapeMedia(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Items  []media.ScrapeItem `json:"items"`
		Workers int               `json:"workers,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(req.Items) == 0 {
		http.Error(w, "no items provided", http.StatusBadRequest)
		return
	}

	result := api.scraper.BatchScrape(r.Context(), req.Items, req.Workers)
	_ = json.NewEncoder(w).Encode(result)
}

// GetSources returns available metadata sources
func (api *MediaCenterAPI) GetSources(w http.ResponseWriter, r *http.Request) {
	sources := api.scraper.GetAvailableSources()
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"sources": sources,
	})
}

// AnalyzeMedia analyzes a media file
func (api *MediaCenterAPI) AnalyzeMedia(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FilePath string `json:"filePath"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	analysis, err := media.AnalyzeMediaFile("", req.FilePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(analysis)
}

// GetPlaybackConfig returns optimal playback configuration
func (api *MediaCenterAPI) GetPlaybackConfig(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("file")
	if filePath == "" {
		http.Error(w, "file path required", http.StatusBadRequest)
		return
	}

	analysis, err := media.AnalyzeMediaFile("", filePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Default capabilities (could be loaded from device profile)
	caps := &media.PlaybackCapabilities{
		MaxResolution:       "3840x2160",
		SupportedHDR:        []media.HDRFormat{media.HDR10, media.DolbyVision},
		SupportedVideoCodecs: []media.VideoCodec{media.VideoHEVC, media.VideoH264, media.VideoAV1},
		BitDepthSupport:     10,
		DolbyVisionSupport:  true,
		SupportedAudioCodecs: []media.AudioCodec{media.AudioEAC3, media.AudioTrueHD, media.AudioAtmos},
		MaxAudioChannels:    8,
		AtmosSupport:        true,
		TrueHDSupport:       true,
		MaxBitrate:          50000000,
		HLSSupport:          true,
		DASHSupport:         true,
	}

	config := media.GetOptimalPlaybackConfig(analysis, caps)
	_ = json.NewEncoder(w).Encode(config)
}

// SetDolbyConfig sets Dolby configuration
func (api *MediaCenterAPI) SetDolbyConfig(w http.ResponseWriter, r *http.Request) {
	var config media.BluRayPlaybackConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Store configuration (in production, persist to database)
	// Return success
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"config":  config,
	})
}

// CreateTranscodeJob creates a new transcode job
func (api *MediaCenterAPI) CreateTranscodeJob(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InputPath  string                `json:"inputPath"`
		OutputPath string                `json:"outputPath"`
		Config     media.TranscodeConfig `json:"config"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	job, err := api.transcoder.CreateJob(req.InputPath, req.OutputPath, req.Config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Auto-start the job
	if err := api.transcoder.StartJob(job.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(job)
}

// GetTranscodeJob gets transcode job status
func (api *MediaCenterAPI) GetTranscodeJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	if jobID == "" {
		http.Error(w, "job id required", http.StatusBadRequest)
		return
	}

	job := api.transcoder.GetJob(jobID)
	if job == nil {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}

	_ = json.NewEncoder(w).Encode(job)
}

// CancelTranscodeJob cancels a transcode job
func (api *MediaCenterAPI) CancelTranscodeJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	if jobID == "" {
		http.Error(w, "job id required", http.StatusBadRequest)
		return
	}

	if err := api.transcoder.CancelJob(jobID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// ListTranscodeJobs lists all transcode jobs
func (api *MediaCenterAPI) ListTranscodeJobs(w http.ResponseWriter, r *http.Request) {
	jobs := api.transcoder.ListJobs()
	_ = json.NewEncoder(w).Encode(jobs)
}

// CreateHLSStream creates an HLS stream
func (api *MediaCenterAPI) CreateHLSStream(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SourcePath string `json:"sourcePath"`
		OutputDir  string `json:"outputDir"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	session, err := api.streamer.CreateHLSSession(req.SourcePath, req.OutputDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(session)
}

// CreateDASHStream creates a DASH stream
func (api *MediaCenterAPI) CreateDASHStream(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SourcePath string `json:"sourcePath"`
		OutputDir  string `json:"outputDir"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	session, err := api.streamer.CreateDASHSession(req.SourcePath, req.OutputDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(session)
}

// GetStreamSession gets stream session info
func (api *MediaCenterAPI) GetStreamSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	if sessionID == "" {
		http.Error(w, "session id required", http.StatusBadRequest)
		return
	}

	session := api.streamer.GetSession(sessionID)
	if session == nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(session)
}

// StopStreamSession stops a stream session
func (api *MediaCenterAPI) StopStreamSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	if sessionID == "" {
		http.Error(w, "session id required", http.StatusBadRequest)
		return
	}

	if err := api.streamer.StopSession(sessionID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// WriteError writes an error response
func WriteError(w http.ResponseWriter, code int, message string) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   http.StatusText(code),
		Code:    code,
		Message: message,
	})
}

// WriteJSON writes a JSON response
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// Timestamp is a helper for consistent timestamp formatting
func Timestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}