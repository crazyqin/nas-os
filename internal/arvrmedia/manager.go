package arvrmedia

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Manager 管理 AR/VR 媒体体验模块
type Manager struct {
	mu               sync.RWMutex
	panoramas        map[string]*PanoramaMedia
	models           map[string]*Model3D
	galleries        map[string]*VREntry
	audioConfigs     map[string]*SpatialAudioConfig
	theaters         map[string]*ImmersiveTheater
	sessions         map[string]*WebXRSession
	imports          map[string]*ImportTask
	storagePath      string
	maxFileSize      int64
	supportedFormats map[string]bool
	modelFormats     map[string]bool
}

// NewManager 创建新的 AR/VR 媒体管理器
func NewManager(storagePath string) *Manager {
	return &Manager{
		panoramas:    make(map[string]*PanoramaMedia),
		models:       make(map[string]*Model3D),
		galleries:    make(map[string]*VREntry),
		audioConfigs: make(map[string]*SpatialAudioConfig),
		theaters:     make(map[string]*ImmersiveTheater),
		sessions:     make(map[string]*WebXRSession),
		imports:      make(map[string]*ImportTask),
		storagePath:  storagePath,
		maxFileSize:  500 * 1024 * 1024, // 500MB
		supportedFormats: map[string]bool{
			".jpg": true, ".jpeg": true, ".png": true,
			".webp": true, ".mp4": true, ".webm": true,
		},
		modelFormats: map[string]bool{
			".gltf": true, ".glb": true, ".obj": true, ".stl": true,
		},
	}
}

// ==================== 全景媒体管理 ====================

// CreatePanorama 创建全景媒体记录
func (m *Manager) CreatePanorama(req *PanoramaMedia) (*PanoramaMedia, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	now := time.Now()
	media := &PanoramaMedia{
		ID:          fmt.Sprintf("pano-%d", now.UnixNano()),
		Name:        req.Name,
		Description: req.Description,
		Path:        req.Path,
		MimeType:    req.MimeType,
		Size:        req.Size,
		Width:       req.Width,
		Height:      req.Height,
		Projection:  req.Projection,
		IsVideo:     req.IsVideo,
		Duration:    req.Duration,
		Tags:        req.Tags,
		Metadata:    req.Metadata,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if media.Projection == "" {
		media.Projection = ProjectionEquirectangular
	}

	m.panoramas[media.ID] = media
	return media, nil
}

// GetPanorama 获取全景媒体
func (m *Manager) GetPanorama(id string) (*PanoramaMedia, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	media, ok := m.panoramas[id]
	return media, ok
}

// UpdatePanorama 更新全景媒体
func (m *Manager) UpdatePanorama(id string, updates map[string]interface{}) (*PanoramaMedia, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	media, ok := m.panoramas[id]
	if !ok {
		return nil, fmt.Errorf("panorama not found: %s", id)
	}

	if v, ok := updates["name"].(string); ok && v != "" {
		media.Name = v
	}
	if v, ok := updates["description"].(string); ok {
		media.Description = v
	}
	if v, ok := updates["tags"].([]string); ok {
		media.Tags = v
	}
	media.UpdatedAt = time.Now()

	return media, nil
}

// DeletePanorama 删除全景媒体
func (m *Manager) DeletePanorama(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	media, ok := m.panoramas[id]
	if !ok {
		return fmt.Errorf("panorama not found: %s", id)
	}

	if media.Path != "" {
		if err := os.Remove(media.Path); err != nil && !os.IsNotExist(err) {
			log.Printf("Warning: failed to delete file %s: %v", media.Path, err)
		}
	}

	delete(m.panoramas, id)
	return nil
}

// ListPanoramas 列出全景媒体
func (m *Manager) ListPanoramas(mediaType string, page, pageSize int) ([]PanoramaMedia, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []PanoramaMedia
	for _, p := range m.panoramas {
		if mediaType == "photo" && p.IsVideo {
			continue
		}
		if mediaType == "video" && !p.IsVideo {
			continue
		}
		list = append(list, *p)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})

	total := len(list)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	return list[start:end], total
}

// ==================== 3D模型管理 ====================

// CreateModel 创建3D模型记录
func (m *Manager) CreateModel(req *Model3D) (*Model3D, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if req.Format == "" {
		return nil, fmt.Errorf("format is required")
	}

	now := time.Now()
	model := &Model3D{
		ID:           fmt.Sprintf("model-%d", now.UnixNano()),
		Name:         req.Name,
		Description:  req.Description,
		Path:         req.Path,
		Format:       req.Format,
		Size:         req.Size,
		VertexCount:  req.VertexCount,
		FaceCount:    req.FaceCount,
		HasTextures:  req.HasTextures,
		HasAnimation: req.HasAnimation,
		TexturePaths: req.TexturePaths,
		BoundingBox:  req.BoundingBox,
		Tags:         req.Tags,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	m.models[model.ID] = model
	return model, nil
}

// GetModel 获取3D模型
func (m *Manager) GetModel(id string) (*Model3D, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	model, ok := m.models[id]
	return model, ok
}

// DeleteModel 删除3D模型
func (m *Manager) DeleteModel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	model, ok := m.models[id]
	if !ok {
		return fmt.Errorf("model not found: %s", id)
	}

	if model.Path != "" {
		if err := os.Remove(model.Path); err != nil && !os.IsNotExist(err) {
			log.Printf("Warning: failed to delete file %s: %v", model.Path, err)
		}
	}

	delete(m.models, id)
	return nil
}

// ListModels 列出3D模型
func (m *Manager) ListModels(format string, page, pageSize int) ([]Model3D, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []Model3D
	for _, md := range m.models {
		if format != "" && string(md.Format) != format {
			continue
		}
		list = append(list, *md)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})

	total := len(list)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	return list[start:end], total
}

// ==================== VR画廊管理 ====================

// CreateGallery 创建VR画廊
func (m *Manager) CreateGallery(req *VREntry) (*VREntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	now := time.Now()
	gallery := &VREntry{
		ID:          fmt.Sprintf("gallery-%d", now.UnixNano()),
		Name:        req.Name,
		Description: req.Description,
		Layout:      req.Layout,
		MediaIDs:    req.MediaIDs,
		Background:  req.Background,
		Lighting:    req.Lighting,
		SkyboxPath:  req.SkyboxPath,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if gallery.Layout == "" {
		gallery.Layout = "wall"
	}
	if gallery.Background == "" {
		gallery.Background = "museum"
	}

	m.galleries[gallery.ID] = gallery
	return gallery, nil
}

// GetGallery 获取VR画廊
func (m *Manager) GetGallery(id string) (*VREntry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	gallery, ok := m.galleries[id]
	return gallery, ok
}

// UpdateGallery 更新VR画廊
func (m *Manager) UpdateGallery(id string, updates map[string]interface{}) (*VREntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	gallery, ok := m.galleries[id]
	if !ok {
		return nil, fmt.Errorf("gallery not found: %s", id)
	}

	if v, ok := updates["name"].(string); ok && v != "" {
		gallery.Name = v
	}
	if v, ok := updates["description"].(string); ok {
		gallery.Description = v
	}
	if v, ok := updates["media_ids"].([]string); ok {
		gallery.MediaIDs = v
	}
	if v, ok := updates["background"].(string); ok {
		gallery.Background = v
	}
	gallery.UpdatedAt = time.Now()

	return gallery, nil
}

// DeleteGallery 删除VR画廊
func (m *Manager) DeleteGallery(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.galleries[id]; !ok {
		return fmt.Errorf("gallery not found: %s", id)
	}
	delete(m.galleries, id)
	return nil
}

// ListGalleries 列出VR画廊
func (m *Manager) ListGalleries() []*VREntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*VREntry, 0, len(m.galleries))
	for _, g := range m.galleries {
		list = append(list, g)
	}
	return list
}

// ==================== 空间音频配置 ====================

// CreateAudioConfig 创建空间音频配置
func (m *Manager) CreateAudioConfig(req *SpatialAudioConfig) (*SpatialAudioConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	now := time.Now()
	config := &SpatialAudioConfig{
		ID:               fmt.Sprintf("audio-%d", now.UnixNano()),
		Name:             req.Name,
		Mode:             req.Mode,
		SourcePosition:   req.SourcePosition,
		ListenerPosition: req.ListenerPosition,
		Gain:             req.Gain,
		DopplerFactor:    req.DopplerFactor,
		RolloffFactor:    req.RolloffFactor,
		RefDistance:      req.RefDistance,
		MaxDistance:      req.MaxDistance,
		RoomSize:         req.RoomSize,
		ReverbLevel:      req.ReverbLevel,
		Enabled:          req.Enabled,
		CreatedAt:        now,
	}

	if config.Mode == "" {
		config.Mode = AudioModeSpatialized
	}
	if config.Gain == 0 {
		config.Gain = 1.0
	}
	if config.RoomSize == "" {
		config.RoomSize = "medium"
	}

	m.audioConfigs[config.ID] = config
	return config, nil
}

// GetAudioConfig 获取空间音频配置
func (m *Manager) GetAudioConfig(id string) (*SpatialAudioConfig, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	config, ok := m.audioConfigs[id]
	return config, ok
}

// UpdateAudioConfig 更新空间音频配置
func (m *Manager) UpdateAudioConfig(id string, updates map[string]interface{}) (*SpatialAudioConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	config, ok := m.audioConfigs[id]
	if !ok {
		return nil, fmt.Errorf("audio config not found: %s", id)
	}

	if v, ok := updates["gain"].(float64); ok {
		config.Gain = v
	}
	if v, ok := updates["room_size"].(string); ok {
		config.RoomSize = v
	}
	if v, ok := updates["reverb_level"].(float64); ok {
		config.ReverbLevel = v
	}
	if v, ok := updates["enabled"].(bool); ok {
		config.Enabled = v
	}

	return config, nil
}

// DeleteAudioConfig 删除空间音频配置
func (m *Manager) DeleteAudioConfig(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.audioConfigs[id]; !ok {
		return fmt.Errorf("audio config not found: %s", id)
	}
	delete(m.audioConfigs, id)
	return nil
}

// ListAudioConfigs 列出空间音频配置
func (m *Manager) ListAudioConfigs() []*SpatialAudioConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*SpatialAudioConfig, 0, len(m.audioConfigs))
	for _, c := range m.audioConfigs {
		list = append(list, c)
	}
	return list
}

// ==================== 沉浸式影院管理 ====================

// CreateTheater 创建沉浸式影院
func (m *Manager) CreateTheater(req *ImmersiveTheater) (*ImmersiveTheater, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	now := time.Now()
	theater := &ImmersiveTheater{
		ID:           fmt.Sprintf("theater-%d", now.UnixNano()),
		Name:         req.Name,
		Description:  req.Description,
		ScreenType:   req.ScreenType,
		ScreenWidth:  req.ScreenWidth,
		ScreenHeight: req.ScreenHeight,
		Distance:     req.Distance,
		Environment:  req.Environment,
		SeatPosition: req.SeatPosition,
		AudioConfig:  req.AudioConfig,
		MaxViewers:   req.MaxViewers,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if theater.ScreenType == "" {
		theater.ScreenType = "curved"
	}
	if theater.Environment == "" {
		theater.Environment = "cinema"
	}
	if theater.MaxViewers == 0 {
		theater.MaxViewers = 1
	}

	m.theaters[theater.ID] = theater
	return theater, nil
}

// GetTheater 获取沉浸式影院
func (m *Manager) GetTheater(id string) (*ImmersiveTheater, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	theater, ok := m.theaters[id]
	return theater, ok
}

// DeleteTheater 删除沉浸式影院
func (m *Manager) DeleteTheater(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.theaters[id]; !ok {
		return fmt.Errorf("theater not found: %s", id)
	}
	delete(m.theaters, id)
	return nil
}

// ListTheaters 列出沉浸式影院
func (m *Manager) ListTheaters() []*ImmersiveTheater {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*ImmersiveTheater, 0, len(m.theaters))
	for _, t := range m.theaters {
		list = append(list, t)
	}
	return list
}

// ==================== WebXR会话管理 ====================

// CreateSession 创建WebXR会话
func (m *Manager) CreateSession(mode XRMode, deviceID string) (*WebXRSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	session := &WebXRSession{
		ID:        fmt.Sprintf("xr-%d", now.UnixNano()),
		Mode:      mode,
		DeviceID:  deviceID,
		Status:    "connecting",
		StartTime: now,
		FrameRate: 60,
		Resolution: &Resolution{
			Width:  2880,
			Height: 1600,
		},
		Capabilities: []string{"local-floor", "bounded-floor", "hand-tracking"},
	}

	m.sessions[session.ID] = session
	return session, nil
}

// GetSession 获取WebXR会话
func (m *Manager) GetSession(id string) (*WebXRSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, ok := m.sessions[id]
	return session, ok
}

// UpdateSessionStatus 更新WebXR会话状态
func (m *Manager) UpdateSessionStatus(id, status string) (*WebXRSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", id)
	}

	session.Status = status
	if status == "ended" {
		now := time.Now()
		session.EndTime = &now
	}

	return session, nil
}

// ListActiveSessions 列出活跃会话
func (m *Manager) ListActiveSessions() []*WebXRSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []*WebXRSession
	for _, s := range m.sessions {
		if s.Status != "ended" {
			list = append(list, s)
		}
	}
	return list
}

// ==================== 媒体导入 ====================

// ImportMedia 导入媒体文件
func (m *Manager) ImportMedia(ctx context.Context, sourcePath string, mediaType MediaType) (*ImportTask, error) {
	m.mu.Lock()

	if _, err := os.Stat(sourcePath); err != nil {
		m.mu.Unlock()
		return nil, fmt.Errorf("source path not found: %s", sourcePath)
	}

	now := time.Now()
	task := &ImportTask{
		ID:         fmt.Sprintf("import-%d", now.UnixNano()),
		Status:     TaskStatusPending,
		SourcePath: sourcePath,
		MediaType:  mediaType,
		StartedAt:  now,
	}
	m.imports[task.ID] = task
	m.mu.Unlock()

	go m.processImport(ctx, task)

	return task, nil
}

func (m *Manager) processImport(ctx context.Context, task *ImportTask) {
	m.mu.Lock()
	task.Status = TaskStatusProcessing
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		now := time.Now()
		task.CompletedAt = &now
		if task.Failed == 0 {
			task.Status = TaskStatusCompleted
		} else if task.Processed == 0 {
			task.Status = TaskStatusFailed
		} else {
			task.Status = TaskStatusCompleted
		}
	}()

	_ = filepath.Walk(task.SourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if info.IsDir() || info.Size() > m.maxFileSize {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))

		switch task.MediaType {
		case MediaTypePanorama, MediaType360Video:
			if !m.supportedFormats[ext] {
				return nil
			}
			task.TotalFiles++
			_, err := m.CreatePanorama(&PanoramaMedia{
				Name:       info.Name(),
				Path:       path,
				Size:       info.Size(),
				IsVideo:    ext == ".mp4" || ext == ".webm",
				Projection: ProjectionEquirectangular,
			})
			if err != nil {
				task.Failed++
				task.Errors = append(task.Errors, fmt.Sprintf("%s: %v", path, err))
			} else {
				task.Processed++
			}

		case MediaType3DModel:
			if !m.modelFormats[ext] {
				return nil
			}
			task.TotalFiles++
			format := ModelFormat(strings.TrimPrefix(ext, "."))
			_, err := m.CreateModel(&Model3D{
				Name:   info.Name(),
				Path:   path,
				Size:   info.Size(),
				Format: format,
			})
			if err != nil {
				task.Failed++
				task.Errors = append(task.Errors, fmt.Sprintf("%s: %v", path, err))
			} else {
				task.Processed++
			}
		}

		return nil
	})
}

// GetImportTask 获取导入任务状态
func (m *Manager) GetImportTask(id string) (*ImportTask, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	task, ok := m.imports[id]
	return task, ok
}

// ==================== 统计 ====================

// GetStats 获取AR/VR媒体统计
func (m *Manager) GetStats() *ARVRStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &ARVRStats{
		TotalPanoramas: len(m.panoramas),
		TotalModels3D:  len(m.models),
		TotalGalleries: len(m.galleries),
		TotalTheaters:  len(m.theaters),
	}

	for _, p := range m.panoramas {
		stats.TotalSize += p.Size
		if p.IsVideo {
			stats.TotalVideos360++
		}
	}
	for _, md := range m.models {
		stats.TotalSize += md.Size
	}
	for _, s := range m.sessions {
		if s.Status != "ended" {
			stats.ActiveSessions++
		}
	}

	return stats
}
