package smartphoto

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

// Manager manages the smart photo system
type Manager struct {
	mu           sync.RWMutex
	photos       map[string]*Photo
	metadata     map[string]*PhotoMetadata
	persons      map[string]*Person
	albums       map[string]*Album
	shares       map[string]*ShareLink
	imports      map[string]*ImportStatus
	storagePath  string
	indexPath    string
	aiEnabled    bool
	maxFileSize  int64
	supportedFormats map[string]bool
}

// NewManager creates a new photo manager
func NewManager(storagePath string, aiEnabled bool) *Manager {
	return &Manager{
		photos:     make(map[string]*Photo),
		metadata:   make(map[string]*PhotoMetadata),
		persons:    make(map[string]*Person),
		albums:     make(map[string]*Album),
		shares:     make(map[string]*ShareLink),
		imports:    make(map[string]*ImportStatus),
		storagePath: storagePath,
		indexPath:  filepath.Join(storagePath, ".index"),
		aiEnabled:  aiEnabled,
		maxFileSize: 100 * 1024 * 1024, // 100MB
		supportedFormats: map[string]bool{
			".jpg": true, ".jpeg": true, ".png": true,
			".gif": true, ".bmp": true, ".tiff": true,
			".webp": true, ".heic": true, ".heif": true,
			".raw": true, ".cr2": true, ".nef": true,
			".arw": true, ".dng": true,
		},
	}
}

// Import imports photos from a directory
func (m *Manager) Import(ctx context.Context, req *ImportRequest) (*ImportStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	status := &ImportStatus{
		ID:        fmt.Sprintf("import-%d", time.Now().UnixNano()),
		Status:    "running",
		StartedAt: time.Now(),
	}
	m.imports[status.ID] = status

	go m.processImport(ctx, status, req)

	return status, nil
}

func (m *Manager) processImport(ctx context.Context, status *ImportStatus, req *ImportRequest) {
	defer func() {
		now := time.Now()
		status.CompletedAt = &now
		status.Status = "completed"
	}()

	err := filepath.Walk(req.SourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if info.IsDir() && !req.Recursive && path != req.SourcePath {
			return filepath.SkipDir
		}

		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if !m.supportedFormats[ext] {
			return nil
		}

		if info.Size() > m.maxFileSize {
			status.Failed++
			status.Errors = append(status.Errors, fmt.Sprintf("file too large: %s", path))
			return nil
		}

		status.Total++

		hash, err := m.calculateFileHash(path)
		if err != nil {
			status.Failed++
			status.Errors = append(status.Errors, fmt.Sprintf("hash error %s: %v", path, err))
			return nil
		}

		if req.DuplicateCheck {
			for _, p := range m.photos {
				if p.Hash == hash {
					status.Skipped++
					return nil
				}
			}
		}

		photo, err := m.importSinglePhoto(path, hash, req.Tags)
		if err != nil {
			status.Failed++
			status.Errors = append(status.Errors, fmt.Sprintf("import error %s: %v", path, err))
			return nil
		}

		m.photos[photo.ID] = photo

		if req.AlbumID != "" {
			if album, ok := m.albums[req.AlbumID]; ok {
				album.PhotoCount++
			}
		}

		status.Processed++
		return nil
	})

	if err != nil {
		status.Errors = append(status.Errors, err.Error())
	}
}

func (m *Manager) importSinglePhoto(path, hash string, tags []string) (*Photo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	photo := &Photo{
		ID:        fmt.Sprintf("photo-%d", now.UnixNano()),
		Filename:  filepath.Base(path),
		Path:      path,
		Size:      info.Size(),
		MimeType:  m.detectMimeType(path),
		CreatedAt: now,
		UpdatedAt: now,
		Hash:      hash,
	}

	return photo, nil
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

func (m *Manager) detectMimeType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	mimeTypes := map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".bmp":  "image/bmp",
		".tiff": "image/tiff",
		".webp": "image/webp",
		".heic": "image/heic",
		".heif": "image/heif",
	}
	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}

// Search searches photos based on criteria
func (m *Manager) Search(req *SearchRequest) *SearchResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []Photo
	for _, photo := range m.photos {
		if m.matchesSearch(photo, req) {
			results = append(results, *photo)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		switch req.SortBy {
		case "taken_at":
			if req.SortOrder == "asc" {
				return results[i].TakenAt.Before(results[j].TakenAt)
			}
			return results[i].TakenAt.After(results[j].TakenAt)
		case "size":
			if req.SortOrder == "asc" {
				return results[i].Size < results[j].Size
			}
			return results[i].Size > results[j].Size
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

	return &SearchResult{
		Photos:   results[start:end],
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		HasMore:  end < total,
	}
}

func (m *Manager) matchesSearch(photo *Photo, req *SearchRequest) bool {
	if req.Query != "" {
		query := strings.ToLower(req.Query)
		if !strings.Contains(strings.ToLower(photo.Filename), query) &&
			!strings.Contains(strings.ToLower(photo.Comments), query) {
			return false
		}
	}

	if req.Rating > 0 && photo.Rating < req.Rating {
		return false
	}

	if req.IsFavorite != nil && photo.IsFavorite != *req.IsFavorite {
		return false
	}

	if req.DateFrom != nil && photo.TakenAt.Before(*req.DateFrom) {
		return false
	}

	if req.DateTo != nil && photo.TakenAt.After(*req.DateTo) {
		return false
	}

	return true
}

// GetPhoto returns a photo by ID
func (m *Manager) GetPhoto(id string) (*Photo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	photo, ok := m.photos[id]
	return photo, ok
}

// UpdatePhoto updates a photo
func (m *Manager) UpdatePhoto(id string, updates map[string]interface{}) (*Photo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	photo, ok := m.photos[id]
	if !ok {
		return nil, fmt.Errorf("photo not found: %s", id)
	}

	if v, ok := updates["is_favorite"].(bool); ok {
		photo.IsFavorite = v
	}
	if v, ok := updates["is_hidden"].(bool); ok {
		photo.IsHidden = v
	}
	if v, ok := updates["rating"].(int); ok {
		photo.Rating = v
	}
	if v, ok := updates["comments"].(string); ok {
		photo.Comments = v
	}
	photo.UpdatedAt = time.Now()

	return photo, nil
}

// DeletePhoto deletes a photo
func (m *Manager) DeletePhoto(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	photo, ok := m.photos[id]
	if !ok {
		return fmt.Errorf("photo not found: %s", id)
	}

	if err := os.Remove(photo.Path); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: failed to delete file %s: %v", photo.Path, err)
	}

	delete(m.photos, id)
	delete(m.metadata, id)
	return nil
}

// CreateAlbum creates a new album
func (m *Manager) CreateAlbum(name, description string, isSmart bool, rules *SmartRules) *Album {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	album := &Album{
		ID:          fmt.Sprintf("album-%d", now.UnixNano()),
		Name:        name,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
		IsSmart:     isSmart,
		SmartRules:  rules,
	}
	m.albums[album.ID] = album
	return album
}

// GetAlbum returns an album by ID
func (m *Manager) GetAlbum(id string) (*Album, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	album, ok := m.albums[id]
	return album, ok
}

// ListAlbums lists all albums
func (m *Manager) ListAlbums() []*Album {
	m.mu.RLock()
	defer m.mu.RUnlock()

	albums := make([]*Album, 0, len(m.albums))
	for _, album := range m.albums {
		albums = append(albums, album)
	}
	return albums
}

// CreatePerson creates a new person
func (m *Manager) CreatePerson(name string) *Person {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	person := &Person{
		ID:        fmt.Sprintf("person-%d", now.UnixNano()),
		Name:      name,
		FirstSeen: now,
		LastSeen:  now,
	}
	m.persons[person.ID] = person
	return person
}

// GetPerson returns a person by ID
func (m *Manager) GetPerson(id string) (*Person, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	person, ok := m.persons[id]
	return person, ok
}

// ListPersons lists all persons
func (m *Manager) ListPersons() []*Person {
	m.mu.RLock()
	defer m.mu.RUnlock()

	persons := make([]*Person, 0, len(m.persons))
	for _, person := range m.persons {
		persons = append(persons, person)
	}
	return persons
}

// CreateShare creates a share link
func (m *Manager) CreateShare(req *ShareRequest) *ShareLink {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	share := &ShareLink{
		ID:            fmt.Sprintf("share-%d", now.UnixNano()),
		URL:           fmt.Sprintf("/share/%s", fmt.Sprintf("%x", now.UnixNano())),
		PhotoIDs:      req.PhotoIDs,
		AlbumID:       req.AlbumID,
		CreatedAt:     now,
		Password:      req.Password,
		MaxViews:      req.MaxViews,
		AllowDownload: req.AllowDownload,
	}

	if req.ExpiresIn > 0 {
		expires := now.Add(time.Duration(req.ExpiresIn) * time.Hour)
		share.ExpiresAt = &expires
	}

	m.shares[share.ID] = share
	return share
}

// GetShare returns a share by ID
func (m *Manager) GetShare(id string) (*ShareLink, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	share, ok := m.shares[id]
	return share, ok
}

// GetStats returns photo statistics
func (m *Manager) GetStats() *PhotoStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &PhotoStats{
		TotalPhotos:  len(m.photos),
		TotalAlbums:  len(m.albums),
		TotalPersons: len(m.persons),
	}

	tagCounts := make(map[string]int)
	locationCounts := make(map[string]int)
	cameraCounts := make(map[string]int)
	monthCounts := make(map[string]*MonthStorage)

	now := time.Now()
	today := now.Truncate(24 * time.Hour)
	weekAgo := today.AddDate(0, 0, -7)
	monthAgo := today.AddDate(0, -1, 0)

	for _, photo := range m.photos {
		stats.TotalSize += photo.Size

		if photo.CreatedAt.After(today) {
			stats.TodayCount++
		}
		if photo.CreatedAt.After(weekAgo) {
			stats.WeekCount++
		}
		if photo.CreatedAt.After(monthAgo) {
			stats.MonthCount++
		}

		monthKey := photo.CreatedAt.Format("2006-01")
		if ms, ok := monthCounts[monthKey]; ok {
			ms.Count++
			ms.Size += photo.Size
		} else {
			monthCounts[monthKey] = &MonthStorage{
				Month: monthKey,
				Count: 1,
				Size:  photo.Size,
			}
		}
	}

	for _, meta := range m.metadata {
		if meta.Camera != "" {
			cameraCounts[meta.Camera]++
		}
		if meta.GPS != nil && meta.GPS.City != "" {
			locationCounts[meta.GPS.City]++
		}
		for _, tag := range meta.Tags {
			tagCounts[tag]++
		}
	}

	for tag, count := range tagCounts {
		stats.TopTags = append(stats.TopTags, TagCount{Tag: tag, Count: count})
	}
	sort.Slice(stats.TopTags, func(i, j int) bool {
		return stats.TopTags[i].Count > stats.TopTags[j].Count
	})
	if len(stats.TopTags) > 10 {
		stats.TopTags = stats.TopTags[:10]
	}

	for loc, count := range locationCounts {
		stats.TopLocations = append(stats.TopLocations, LocationCount{Location: loc, Count: count})
	}
	sort.Slice(stats.TopLocations, func(i, j int) bool {
		return stats.TopLocations[i].Count > stats.TopLocations[j].Count
	})
	if len(stats.TopLocations) > 10 {
		stats.TopLocations = stats.TopLocations[:10]
	}

	for cam, count := range cameraCounts {
		stats.CameraStats = append(stats.CameraStats, CameraCount{Camera: cam, Count: count})
	}

	for _, ms := range monthCounts {
		stats.StorageByMonth = append(stats.StorageByMonth, *ms)
	}
	sort.Slice(stats.StorageByMonth, func(i, j int) bool {
		return stats.StorageByMonth[i].Month < stats.StorageByMonth[j].Month
	})

	return stats
}

// FindDuplicates finds duplicate photos
func (m *Manager) FindDuplicates() []*DuplicateGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hashGroups := make(map[string][]*Photo)
	for _, photo := range m.photos {
		hashGroups[photo.Hash] = append(hashGroups[photo.Hash], photo)
	}

	var groups []*DuplicateGroup
	for hash, photos := range hashGroups {
		if len(photos) > 1 {
			var totalSize int64
			for _, p := range photos {
				totalSize += p.Size
			}
			photoList := make([]Photo, len(photos))
			for i, p := range photos {
				photoList[i] = *p
			}
			idLen := 8
			if len(hash) < idLen {
				idLen = len(hash)
			}
			groups = append(groups, &DuplicateGroup{
				ID:        fmt.Sprintf("dup-%s", hash[:idLen]),
				Hash:      hash,
				Photos:    photoList,
				TotalSize: totalSize,
			})
		}
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].TotalSize > groups[j].TotalSize
	})

	return groups
}

// Cleanup performs photo cleanup
func (m *Manager) Cleanup(req *CleanupRequest) *CleanupResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := &CleanupResult{}

	if req.Duplicates {
		hashGroups := make(map[string][]string)
		for id, photo := range m.photos {
			hashGroups[photo.Hash] = append(hashGroups[photo.Hash], id)
		}

		for _, ids := range hashGroups {
			if len(ids) > 1 {
				result.Duplicates += len(ids) - 1
				if !req.DryRun {
					for i := 1; i < len(ids); i++ {
						if photo, ok := m.photos[ids[i]]; ok {
							result.SpaceFreed += photo.Size
							os.Remove(photo.Path)
							delete(m.photos, ids[i])
							delete(m.metadata, ids[i])
						}
					}
				}
			}
		}
	}

	result.TotalFound = result.Duplicates + result.Screenshots + result.Blurry + result.LowQuality
	result.TotalRemoved = result.TotalFound

	return result
}

// GetImportStatus returns the status of an import operation
func (m *Manager) GetImportStatus(id string) (*ImportStatus, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	status, ok := m.imports[id]
	return status, ok
}
