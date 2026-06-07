package homemedia

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Manager manages the home media system
type Manager struct {
	mu               sync.RWMutex
	media            map[string]*MediaFile
	metadata         map[string]*MediaMetadata
	collections      map[string]*Collection
	playlists        map[string]*Playlist
	sessions         map[string]*PlaybackSession
	scans            map[string]*ScanStatus
	transcodes       map[string]*TranscodeStatus
	storagePath      string
	supportedFormats map[string]bool
}

// NewManager creates a new media manager
func NewManager(storagePath string) *Manager {
	return &Manager{
		media:       make(map[string]*MediaFile),
		metadata:    make(map[string]*MediaMetadata),
		collections: make(map[string]*Collection),
		playlists:   make(map[string]*Playlist),
		sessions:    make(map[string]*PlaybackSession),
		scans:       make(map[string]*ScanStatus),
		transcodes:  make(map[string]*TranscodeStatus),
		storagePath: storagePath,
		supportedFormats: map[string]bool{
			".mp4": true, ".mkv": true, ".avi": true,
			".mov": true, ".wmv": true, ".flv": true,
			".webm": true, ".m4v": true, ".ts": true,
			".mp3": true, ".flac": true, ".wav": true,
			".aac": true, ".ogg": true, ".wma": true,
			".m4a": true,
		},
	}
}

// Scan scans a directory for media files
func (m *Manager) Scan(ctx context.Context, req *ScanRequest) (*ScanStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	status := &ScanStatus{
		ID:        fmt.Sprintf("scan-%d", time.Now().UnixNano()),
		Status:    "running",
		StartedAt: time.Now(),
	}
	m.scans[status.ID] = status

	go m.processScan(ctx, status, req)

	return status, nil
}

func (m *Manager) processScan(ctx context.Context, status *ScanStatus, req *ScanRequest) {
	defer func() {
		now := time.Now()
		status.CompletedAt = &now
		status.Status = "completed"
	}()

	err := filepath.Walk(req.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if info.IsDir() && !req.Recursive && path != req.Path {
			return filepath.SkipDir
		}

		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if !m.supportedFormats[ext] {
			return nil
		}

		status.Total++

		hash, err := m.calculateFileHash(path)
		if err != nil {
			status.Failed++
			status.Errors = append(status.Errors, fmt.Sprintf("hash error %s: %v", path, err))
			return nil
		}

		for _, existing := range m.media {
			if existing.Hash == hash {
				status.Processed++
				return nil
			}
		}

		media, err := m.importMediaFile(path, hash)
		if err != nil {
			status.Failed++
			status.Errors = append(status.Errors, fmt.Sprintf("import error %s: %v", path, err))
			return nil
		}

		m.media[media.ID] = media
		status.NewFiles++
		status.Processed++

		return nil
	})

	if err != nil {
		status.Errors = append(status.Errors, err.Error())
	}
}

func (m *Manager) importMediaFile(path, hash string) (*MediaFile, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	ext := strings.ToLower(filepath.Ext(path))
	mediaType := m.detectMediaType(ext)

	media := &MediaFile{
		ID:        fmt.Sprintf("media-%d", now.UnixNano()),
		Filename:  filepath.Base(path),
		Path:      path,
		Size:      info.Size(),
		MimeType:  m.detectMimeType(ext),
		CreatedAt: now,
		UpdatedAt: now,
		Hash:      hash,
		Quality:   m.detectQuality(filepath.Base(path)),
	}

	_ = mediaType

	return media, nil
}

func (m *Manager) calculateFileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (m *Manager) detectMediaType(ext string) string {
	videoExts := map[string]bool{
		".mp4": true, ".mkv": true, ".avi": true,
		".mov": true, ".wmv": true, ".flv": true,
		".webm": true, ".m4v": true, ".ts": true,
	}
	if videoExts[ext] {
		return "video"
	}
	return "audio"
}

func (m *Manager) detectMimeType(ext string) string {
	mimeTypes := map[string]string{
		".mp4":  "video/mp4",
		".mkv":  "video/x-matroska",
		".avi":  "video/x-msvideo",
		".mov":  "video/quicktime",
		".wmv":  "video/x-ms-wmv",
		".flv":  "video/x-flv",
		".webm": "video/webm",
		".m4v":  "video/mp4",
		".ts":   "video/mp2t",
		".mp3":  "audio/mpeg",
		".flac": "audio/flac",
		".wav":  "audio/wav",
		".aac":  "audio/aac",
		".ogg":  "audio/ogg",
		".wma":  "audio/x-ms-wma",
		".m4a":  "audio/mp4",
	}
	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}

func (m *Manager) detectQuality(filename string) string {
	lower := strings.ToLower(filename)
	qualities := []string{"2160p", "4k", "1080p", "720p", "480p", "360p"}
	for _, q := range qualities {
		if strings.Contains(lower, q) {
			return q
		}
	}
	return "unknown"
}

// Search searches media files
func (m *Manager) Search(req *MediaSearchRequest) *MediaSearchResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []MediaFile
	for _, media := range m.media {
		if m.matchesSearch(media, req) {
			results = append(results, *media)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		switch req.SortBy {
		case "title":
			if req.SortOrder == "asc" {
				return results[i].Filename < results[j].Filename
			}
			return results[i].Filename > results[j].Filename
		case "size":
			if req.SortOrder == "asc" {
				return results[i].Size < results[j].Size
			}
			return results[i].Size > results[j].Size
		case "rating":
			if req.SortOrder == "asc" {
				return results[i].Rating < results[j].Rating
			}
			return results[i].Rating > results[j].Rating
		default:
			return results[i].CreatedAt.After(results[j].CreatedAt)
		}
	})

	total := len(results)
	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	return &MediaSearchResult{
		Media:    results[start:end],
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		HasMore:  end < total,
	}
}

func (m *Manager) matchesSearch(media *MediaFile, req *MediaSearchRequest) bool {
	if req.Query != "" {
		query := strings.ToLower(req.Query)
		if !strings.Contains(strings.ToLower(media.Filename), query) {
			return false
		}
	}

	if req.Type != "" {
		ext := strings.ToLower(filepath.Ext(media.Filename))
		if req.Type == "video" && !m.isVideo(ext) {
			return false
		}
		if req.Type == "audio" && !m.isAudio(ext) {
			return false
		}
	}

	if req.Rating > 0 && float64(media.Rating) < req.Rating {
		return false
	}

	return true
}

func (m *Manager) isVideo(ext string) bool {
	videoExts := map[string]bool{
		".mp4": true, ".mkv": true, ".avi": true,
		".mov": true, ".wmv": true, ".flv": true,
		".webm": true, ".m4v": true, ".ts": true,
	}
	return videoExts[ext]
}

func (m *Manager) isAudio(ext string) bool {
	audioExts := map[string]bool{
		".mp3": true, ".flac": true, ".wav": true,
		".aac": true, ".ogg": true, ".wma": true,
		".m4a": true,
	}
	return audioExts[ext]
}

// GetMedia returns a media file by ID
func (m *Manager) GetMedia(id string) (*MediaFile, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	media, ok := m.media[id]
	return media, ok
}

// UpdateMedia updates a media file
func (m *Manager) UpdateMedia(id string, updates map[string]interface{}) (*MediaFile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	media, ok := m.media[id]
	if !ok {
		return nil, fmt.Errorf("media not found: %s", id)
	}

	if v, ok := updates["is_favorite"].(bool); ok {
		media.IsFavorite = v
	}
	if v, ok := updates["rating"].(int); ok {
		media.Rating = v
	}
	media.UpdatedAt = time.Now()

	return media, nil
}

// DeleteMedia deletes a media file
func (m *Manager) DeleteMedia(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	media, ok := m.media[id]
	if !ok {
		return fmt.Errorf("media not found: %s", id)
	}

	if err := os.Remove(media.Path); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: failed to delete file %s: %v", media.Path, err)
	}

	delete(m.media, id)
	delete(m.metadata, id)
	return nil
}

// CreateCollection creates a new collection
func (m *Manager) CreateCollection(name, description, collectionType string) *Collection {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	collection := &Collection{
		ID:          fmt.Sprintf("collection-%d", now.UnixNano()),
		Name:        name,
		Description: description,
		Type:        collectionType,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	m.collections[collection.ID] = collection
	return collection
}

// GetCollection returns a collection by ID
func (m *Manager) GetCollection(id string) (*Collection, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	collection, ok := m.collections[id]
	return collection, ok
}

// ListCollections lists all collections
func (m *Manager) ListCollections() []*Collection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	collections := make([]*Collection, 0, len(m.collections))
	for _, c := range m.collections {
		collections = append(collections, c)
	}
	return collections
}

// CreatePlaylist creates a new playlist
func (m *Manager) CreatePlaylist(name, description string) *Playlist {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	playlist := &Playlist{
		ID:          fmt.Sprintf("playlist-%d", now.UnixNano()),
		Name:        name,
		Description: description,
		RepeatMode:  "none",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	m.playlists[playlist.ID] = playlist
	return playlist
}

// GetPlaylist returns a playlist by ID
func (m *Manager) GetPlaylist(id string) (*Playlist, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	playlist, ok := m.playlists[id]
	return playlist, ok
}

// ListPlaylists lists all playlists
func (m *Manager) ListPlaylists() []*Playlist {
	m.mu.RLock()
	defer m.mu.RUnlock()

	playlists := make([]*Playlist, 0, len(m.playlists))
	for _, p := range m.playlists {
		playlists = append(playlists, p)
	}
	return playlists
}

// AddToPlaylist adds a media file to a playlist
func (m *Manager) AddToPlaylist(playlistID, mediaID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	playlist, ok := m.playlists[playlistID]
	if !ok {
		return fmt.Errorf("playlist not found: %s", playlistID)
	}

	if _, ok := m.media[mediaID]; !ok {
		return fmt.Errorf("media not found: %s", mediaID)
	}

	item := PlaylistItem{
		MediaID:  mediaID,
		Position: len(playlist.Items) + 1,
		AddedAt:  time.Now(),
	}
	playlist.Items = append(playlist.Items, item)
	playlist.UpdatedAt = time.Now()

	return nil
}

// StartPlayback starts a playback session
func (m *Manager) StartPlayback(mediaID, userID, deviceID, deviceName string) (*PlaybackSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	media, ok := m.media[mediaID]
	if !ok {
		return nil, fmt.Errorf("media not found: %s", mediaID)
	}

	now := time.Now()
	session := &PlaybackSession{
		ID:         fmt.Sprintf("session-%d", now.UnixNano()),
		MediaID:    mediaID,
		UserID:     userID,
		DeviceID:   deviceID,
		DeviceName: deviceName,
		StartTime:  now,
		Duration:   media.Duration,
		Status:     "playing",
	}
	m.sessions[session.ID] = session

	media.WatchCount++
	media.LastWatched = &now

	return session, nil
}

// UpdatePlaybackProgress updates playback progress
func (m *Manager) UpdatePlaybackProgress(sessionID string, currentTime int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.CurrentTime = currentTime
	if session.Duration > 0 {
		session.Progress = float64(currentTime) / float64(session.Duration) * 100
	}

	if media, ok := m.media[session.MediaID]; ok {
		media.Progress = session.Progress
	}

	return nil
}

// StopPlayback stops a playback session
func (m *Manager) StopPlayback(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.Status = "stopped"
	return nil
}

// GetStats returns media statistics
func (m *Manager) GetStats() *MediaStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &MediaStats{
		TotalMedia: len(m.media),
	}

	genreCounts := make(map[string]int)
	yearCounts := make(map[int]int)
	qualityCounts := make(map[string]int)
	monthCounts := make(map[string]*MonthStorage)

	var allMedia []MediaFile
	for _, media := range m.media {
		stats.TotalSize += media.Size
		stats.TotalDuration += media.Duration
		allMedia = append(allMedia, *media)

		ext := strings.ToLower(filepath.Ext(media.Filename))
		if m.isVideo(ext) {
			stats.TotalMovies++
		} else if m.isAudio(ext) {
			stats.TotalMusic++
		}

		qualityCounts[media.Quality]++
		monthKey := media.CreatedAt.Format("2006-01")
		if ms, ok := monthCounts[monthKey]; ok {
			ms.Count++
			ms.Size += media.Size
		} else {
			monthCounts[monthKey] = &MonthStorage{
				Month: monthKey,
				Count: 1,
				Size:  media.Size,
			}
		}
	}

	for _, meta := range m.metadata {
		if meta.Year > 0 {
			yearCounts[meta.Year]++
		}
		for _, genre := range meta.Genre {
			genreCounts[genre]++
		}
	}

	sort.Slice(allMedia, func(i, j int) bool {
		return allMedia[i].CreatedAt.After(allMedia[j].CreatedAt)
	})
	if len(allMedia) > 10 {
		allMedia = allMedia[:10]
	}
	stats.RecentlyAdded = allMedia

	for genre, count := range genreCounts {
		stats.GenreStats = append(stats.GenreStats, GenreCount{Genre: genre, Count: count})
	}

	for year, count := range yearCounts {
		stats.YearStats = append(stats.YearStats, YearCount{Year: year, Count: count})
	}

	for quality, count := range qualityCounts {
		stats.QualityStats = append(stats.QualityStats, QualityCount{Quality: quality, Count: count})
	}

	for _, ms := range monthCounts {
		stats.StorageByMonth = append(stats.StorageByMonth, *ms)
	}

	return stats
}

// GetScanStatus returns scan status
func (m *Manager) GetScanStatus(id string) (*ScanStatus, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	status, ok := m.scans[id]
	return status, ok
}

// GetTranscodeStatus returns transcode status
func (m *Manager) GetTranscodeStatus(id string) (*TranscodeStatus, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	status, ok := m.transcodes[id]
	return status, ok
}
