// Package aiphotomanager 提供AI驱动的智能相册管理系统
// 智能分类、人脸识别、场景识别、照片去重、备份同步
package aiphotomanager

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ========== 常量 ==========

const (
	// Version 模块版本
	Version = "1.0.0"

	// MaxConcurrentAnalysis 最大并发分析数
	MaxConcurrentAnalysis = 8

	// MaxFileSize 最大文件大小 (100MB)
	MaxFileSize = 100 * 1024 * 1024

	// SimilarityThreshold 相似度阈值 (0-100)
	SimilarityThreshold = 85

	// FaceDetectionConfidence 人脸检测置信度阈值
	FaceDetectionConfidence = 0.7

	// SceneRecognitionConfidence 场景识别置信度阈值
	SceneRecognitionConfidence = 0.6
)

// ========== 支持的图片格式 ==========

var SupportedFormats = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".bmp":  true,
	".tiff": true,
	".webp": true,
	".heic": true,
	".heif": true,
	".raw":  true,
	".cr2":  true,
	".nef":  true,
	".arw":  true,
	".dng":  true,
}

// ========== 分类类型 ==========

// CategoryType 分类类型
type CategoryType string

const (
	CategoryTime     CategoryType = "time"     // 时间分类
	CategoryLocation CategoryType = "location" // 地点分类
	CategoryPerson   CategoryType = "person"   // 人物分类
	CategoryScene    CategoryType = "scene"    // 场景分类
	CategoryTag      CategoryType = "tag"      // 标签分类
)

// ========== 场景类型 ==========

// SceneType 场景类型
type SceneType string

const (
	SceneLandscape  SceneType = "landscape"  // 风景
	ScenePortrait   SceneType = "portrait"   // 人像
	SceneFood       SceneType = "food"       // 美食
	SceneAnimal     SceneType = "animal"     // 动物
	SceneBuilding   SceneType = "building"   // 建筑
	SceneVehicle    SceneType = "vehicle"    // 交通工具
	SceneSports     SceneType = "sports"     // 运动
	SceneNature     SceneType = "nature"     // 自然
	SceneIndoor     SceneType = "indoor"     // 室内
	SceneDocument   SceneType = "document"   // 文档
	SceneScreenshot SceneType = "screenshot" // 截图
	SceneOther      SceneType = "other"      // 其他
)

// ========== 备份状态 ==========

// BackupStatus 备份状态
type BackupStatus string

const (
	BackupStatusPending   BackupStatus = "pending"
	BackupStatusRunning   BackupStatus = "running"
	BackupStatusCompleted BackupStatus = "completed"
	BackupStatusFailed    BackupStatus = "failed"
	BackupStatusCancelled BackupStatus = "cancelled"
)

// ========== 同步状态 ==========

// SyncStatus 同步状态
type SyncStatus string

const (
	SyncStatusIdle    SyncStatus = "idle"
	SyncStatusSyncing SyncStatus = "syncing"
	SyncStatusPaused  SyncStatus = "paused"
	SyncStatusError   SyncStatus = "error"
)

// ========== 数据结构 ==========

// Photo 照片信息
type Photo struct {
	ID             string    `json:"id"`
	Filename       string    `json:"filename"`
	Path           string    `json:"path"`
	Size           int64     `json:"size"`
	MimeType       string    `json:"mime_type"`
	Width          int       `json:"width"`
	Height         int       `json:"height"`
	Orientation    int       `json:"orientation"`
	TakenAt        time.Time `json:"taken_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Hash           string    `json:"hash"`
	PerceptualHash string    `json:"perceptual_hash"` // 感知哈希用于相似检测
	IsFavorite     bool      `json:"is_favorite"`
	IsHidden       bool      `json:"is_hidden"`
	Rating         int       `json:"rating"`
	Tags           []string  `json:"tags"`
}

// PhotoMetadata 照片元数据（EXIF + AI分析）
type PhotoMetadata struct {
	PhotoID         string            `json:"photo_id"`
	Camera          string            `json:"camera"`
	Lens            string            `json:"lens"`
	ISO             int               `json:"iso"`
	Aperture        float64           `json:"aperture"`
	ShutterSpeed    string            `json:"shutter_speed"`
	FocalLength     float64           `json:"focal_length"`
	Flash           bool              `json:"flash"`
	GPS             *GPSInfo          `json:"gps,omitempty"`
	Faces           []FaceInfo        `json:"faces"`
	Scene           SceneType         `json:"scene"`
	SceneConfidence float64           `json:"scene_confidence"`
	Objects         []ObjectInfo      `json:"objects"`
	Colors          []ColorInfo       `json:"colors"`
	Aesthetic       float64           `json:"aesthetic_score"`
	Tags            []string          `json:"tags"`
	AIAnalysis      map[string]string `json:"ai_analysis"`
}

// GPSInfo GPS坐标信息
type GPSInfo struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude"`
	Address   string  `json:"address"`
	City      string  `json:"city"`
	Country   string  `json:"country"`
}

// FaceInfo 人脸信息
type FaceInfo struct {
	ID         string    `json:"id"`
	PersonID   string    `json:"person_id,omitempty"`
	Name       string    `json:"name,omitempty"`
	X          int       `json:"x"`
	Y          int       `json:"y"`
	Width      int       `json:"width"`
	Height     int       `json:"height"`
	Confidence float64   `json:"confidence"`
	Age        int       `json:"age"`
	Gender     string    `json:"gender"`
	Emotion    string    `json:"emotion"`
	Features   []float64 `json:"features,omitempty"` // 人脸特征向量
}

// ObjectInfo 物体信息
type ObjectInfo struct {
	Name       string  `json:"name"`
	Confidence float64 `json:"confidence"`
	X          int     `json:"x"`
	Y          int     `json:"y"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
}

// ColorInfo 主要颜色
type ColorInfo struct {
	Hex        string  `json:"hex"`
	Percentage float64 `json:"percentage"`
	Name       string  `json:"name"`
}

// Person 识别的人物
type Person struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Avatar     string    `json:"avatar"`
	FaceCount  int       `json:"face_count"`
	PhotoCount int       `json:"photo_count"`
	FirstSeen  time.Time `json:"first_seen"`
	LastSeen   time.Time `json:"last_seen"`
	IsFavorite bool      `json:"is_favorite"`
	Tags       []string  `json:"tags"`
}

// Album 相册
type Album struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	CoverPhoto  string       `json:"cover_photo"`
	PhotoCount  int          `json:"photo_count"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	Type        CategoryType `json:"type"`
	IsSmart     bool         `json:"is_smart"`
	SmartRules  *SmartRules  `json:"smart_rules,omitempty"`
	Tags        []string     `json:"tags"`
}

// SmartRules 智能相册规则
type SmartRules struct {
	Filters   []Filter `json:"filters"`
	SortBy    string   `json:"sort_by"`
	SortOrder string   `json:"sort_order"`
	Limit     int      `json:"limit"`
	AutoAdd   bool     `json:"auto_add"`
}

// Filter 搜索过滤器
type Filter struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

// DuplicateGroup 重复照片组
type DuplicateGroup struct {
	ID         string   `json:"id"`
	Hash       string   `json:"hash"`
	Similarity float64  `json:"similarity"`
	Photos     []string `json:"photo_ids"`
	TotalSize  int64    `json:"total_size"`
	WastedSize int64    `json:"wasted_size"`
}

// BackupTask 备份任务
type BackupTask struct {
	ID          string       `json:"id"`
	Status      BackupStatus `json:"status"`
	SourcePath  string       `json:"source_path"`
	TargetPath  string       `json:"target_path"`
	TotalFiles  int          `json:"total_files"`
	DoneFiles   int          `json:"done_files"`
	FailedFiles int          `json:"failed_files"`
	TotalSize   int64        `json:"total_size"`
	DoneSize    int64        `json:"done_size"`
	StartedAt   time.Time    `json:"started_at"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
	Error       string       `json:"error,omitempty"`
}

// SyncConfig 同步配置
type SyncConfig struct {
	SourcePath      string   `json:"source_path"`
	TargetPaths     []string `json:"target_paths"`
	AutoSync        bool     `json:"auto_sync"`
	SyncInterval    int      `json:"sync_interval"` // 分钟
	ConflictRule    string   `json:"conflict_rule"` // "newer", "larger", "source", "target"
	ExcludePatterns []string `json:"exclude_patterns"`
}

// SyncState 同步状态
type SyncState struct {
	Status       SyncStatus `json:"status"`
	LastSyncAt   *time.Time `json:"last_sync_at,omitempty"`
	NextSyncAt   *time.Time `json:"next_sync_at,omitempty"`
	SyncedCount  int        `json:"synced_count"`
	FailedCount  int        `json:"failed_count"`
	ErrorMessage string     `json:"error_message,omitempty"`
}

// AlbumRecommendation 相册推荐
type AlbumRecommendation struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Type        CategoryType `json:"type"`
	PhotoIDs    []string     `json:"photo_ids"`
	Confidence  float64      `json:"confidence"`
	Reason      string       `json:"reason"`
}

// AIAnalysisResult AI分析结果
type AIAnalysisResult struct {
	Scene           SceneType    `json:"scene"`
	SceneConfidence float64      `json:"scene_confidence"`
	Faces           []FaceInfo   `json:"faces"`
	Objects         []ObjectInfo `json:"objects"`
	Colors          []ColorInfo  `json:"colors"`
	Aesthetic       float64      `json:"aesthetic_score"`
	Tags            []string     `json:"tags"`
	IsScreenshot    bool         `json:"is_screenshot"`
	IsDocument      bool         `json:"is_document"`
}

// ========== 管理器 ==========

// Manager AI相册管理器
type Manager struct {
	mu              sync.RWMutex
	photos          map[string]*Photo
	metadata        map[string]*PhotoMetadata
	persons         map[string]*Person
	albums          map[string]*Album
	duplicates      map[string]*DuplicateGroup
	backupTasks     map[string]*BackupTask
	syncConfig      *SyncConfig
	syncState       *SyncState
	storagePath     string
	indexPath       string
	aiEnabled       bool
	faceDB          map[string][]float64 // personID -> face features
	hashIndex       map[string][]string  // hash -> photo IDs
	perceptualIndex map[string][]string  // perceptual hash -> photo IDs
}

// NewManager 创建AI相册管理器
func NewManager(storagePath string, aiEnabled bool) *Manager {
	m := &Manager{
		photos:          make(map[string]*Photo),
		metadata:        make(map[string]*PhotoMetadata),
		persons:         make(map[string]*Person),
		albums:          make(map[string]*Album),
		duplicates:      make(map[string]*DuplicateGroup),
		backupTasks:     make(map[string]*BackupTask),
		storagePath:     storagePath,
		indexPath:       filepath.Join(storagePath, ".index"),
		aiEnabled:       aiEnabled,
		faceDB:          make(map[string][]float64),
		hashIndex:       make(map[string][]string),
		perceptualIndex: make(map[string][]string),
		syncState:       &SyncState{Status: SyncStatusIdle},
	}

	os.MkdirAll(filepath.Join(storagePath, "photos"), 0755)
	os.MkdirAll(filepath.Join(storagePath, "thumbnails"), 0755)
	os.MkdirAll(filepath.Join(storagePath, "backups"), 0755)
	os.MkdirAll(m.indexPath, 0755)

	return m
}

// ========== 照片导入 ==========

// ImportPhotos 导入照片
func (m *Manager) ImportPhotos(ctx context.Context, sourcePath string, recursive bool) ([]*Photo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var photos []*Photo

	err := filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if info.IsDir() {
			if !recursive && path != sourcePath {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !SupportedFormats[ext] {
			return nil
		}
		if info.Size() > MaxFileSize {
			return nil
		}
		photo, err := m.importSinglePhoto(path, info)
		if err != nil {
			return nil
		}
		m.photos[photo.ID] = photo
		m.hashIndex[photo.Hash] = append(m.hashIndex[photo.Hash], photo.ID)
		photos = append(photos, photo)
		return nil
	})

	if err != nil {
		return photos, fmt.Errorf("walk error: %w", err)
	}
	return photos, nil
}

func (m *Manager) importSinglePhoto(path string, info os.FileInfo) (*Photo, error) {
	hash, err := m.calculateFileHash(path)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	return &Photo{
		ID:        fmt.Sprintf("photo-%d-%s", now.UnixNano(), randomHex(8)),
		Filename:  filepath.Base(path),
		Path:      path,
		Size:      info.Size(),
		MimeType:  detectMimeType(path),
		CreatedAt: now,
		UpdatedAt: now,
		Hash:      hash,
		TakenAt:   info.ModTime(),
	}, nil
}

// ========== AI分析 ==========

// AnalyzePhoto AI分析照片
func (m *Manager) AnalyzePhoto(ctx context.Context, photoID string) (*AIAnalysisResult, error) {
	m.mu.RLock()
	photo, ok := m.photos[photoID]
	m.mu.RUnlock()

	if !ok {
		return nil, errors.New("photo not found")
	}
	if !m.aiEnabled {
		return &AIAnalysisResult{Scene: SceneOther}, nil
	}

	result := m.performAIAnalysis(ctx, photo)

	m.mu.Lock()
	metadata, exists := m.metadata[photoID]
	if !exists {
		metadata = &PhotoMetadata{PhotoID: photoID}
		m.metadata[photoID] = metadata
	}
	metadata.Scene = result.Scene
	metadata.SceneConfidence = result.SceneConfidence
	metadata.Faces = result.Faces
	metadata.Objects = result.Objects
	metadata.Colors = result.Colors
	metadata.Aesthetic = result.Aesthetic
	metadata.Tags = result.Tags
	m.mu.Unlock()

	if len(result.Faces) > 0 {
		m.processFaces(photoID, result.Faces)
	}
	m.autoClassifyPhoto(photoID, result)

	return result, nil
}

// BatchAnalyzePhotos 批量分析照片
func (m *Manager) BatchAnalyzePhotos(ctx context.Context, photoIDs []string) (map[string]*AIAnalysisResult, error) {
	results := make(map[string]*AIAnalysisResult)
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, MaxConcurrentAnalysis)

	for _, photoID := range photoIDs {
		wg.Add(1)
		go func(pid string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			result, err := m.AnalyzePhoto(ctx, pid)
			if err != nil {
				return
			}
			mu.Lock()
			results[pid] = result
			mu.Unlock()
		}(photoID)
	}

	wg.Wait()
	return results, nil
}

func (m *Manager) performAIAnalysis(_ context.Context, photo *Photo) *AIAnalysisResult {
	result := &AIAnalysisResult{Scene: SceneOther, SceneConfidence: 0.5}
	filename := strings.ToLower(photo.Filename)
	path := strings.ToLower(photo.Path)

	switch {
	case strings.Contains(filename, "screenshot") || strings.Contains(path, "screenshot"):
		result.Scene, result.SceneConfidence, result.IsScreenshot = SceneScreenshot, 0.9, true
	case strings.Contains(filename, "doc") || strings.Contains(filename, "scan"):
		result.Scene, result.SceneConfidence, result.IsDocument = SceneDocument, 0.8, true
	case strings.Contains(path, "food") || strings.Contains(path, "restaurant"):
		result.Scene, result.SceneConfidence = SceneFood, 0.7
	case strings.Contains(path, "landscape") || strings.Contains(path, "nature"):
		result.Scene, result.SceneConfidence = SceneLandscape, 0.7
	case strings.Contains(path, "portrait") || strings.Contains(path, "selfie"):
		result.Scene, result.SceneConfidence = ScenePortrait, 0.7
	case strings.Contains(path, "animal") || strings.Contains(path, "pet"):
		result.Scene, result.SceneConfidence = SceneAnimal, 0.7
	}

	result.Tags = generateTags(photo, result)
	return result
}

func generateTags(photo *Photo, result *AIAnalysisResult) []string {
	tags := []string{}
	sceneTags := map[SceneType][]string{
		SceneLandscape: {"风景", "自然"}, ScenePortrait: {"人像"},
		SceneFood: {"美食"}, SceneAnimal: {"动物"}, SceneBuilding: {"建筑"},
		SceneScreenshot: {"截图"}, SceneDocument: {"文档"},
	}
	if t, ok := sceneTags[result.Scene]; ok {
		tags = append(tags, t...)
	}

	hour := photo.TakenAt.Hour()
	timeTags := map[string]string{
		"dawn": "清晨", "morning": "上午", "noon": "中午",
		"afternoon": "下午", "evening": "傍晚", "night": "夜晚",
	}
	var period string
	switch {
	case hour >= 5 && hour < 8:
		period = "dawn"
	case hour >= 8 && hour < 12:
		period = "morning"
	case hour >= 12 && hour < 14:
		period = "noon"
	case hour >= 14 && hour < 18:
		period = "afternoon"
	case hour >= 18 && hour < 21:
		period = "evening"
	default:
		period = "night"
	}
	tags = append(tags, timeTags[period])
	return tags
}

// ========== 人脸识别 ==========

func (m *Manager) processFaces(_ string, faces []FaceInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range faces {
		face := &faces[i]
		personID := m.matchFace(face.Features)
		if personID != "" {
			face.PersonID = personID
			if person, ok := m.persons[personID]; ok {
				face.Name = person.Name
				person.FaceCount++
				person.PhotoCount++
				person.LastSeen = time.Now()
			}
		} else {
			person := &Person{
				ID:        fmt.Sprintf("person-%d", time.Now().UnixNano()),
				Name:      fmt.Sprintf("人物_%d", len(m.persons)+1),
				FaceCount: 1, PhotoCount: 1,
				FirstSeen: time.Now(), LastSeen: time.Now(),
			}
			m.persons[person.ID] = person
			m.faceDB[person.ID] = face.Features
			face.PersonID, face.Name = person.ID, person.Name
		}
	}
}

func (m *Manager) matchFace(features []float64) string {
	if len(features) == 0 {
		return ""
	}
	bestMatch, bestDist := "", 1.0
	for pid, stored := range m.faceDB {
		if len(stored) != len(features) {
			continue
		}
		d := euclideanDistance(features, stored)
		if d < bestDist && d < 0.6 {
			bestDist, bestMatch = d, pid
		}
	}
	return bestMatch
}

func euclideanDistance(a, b []float64) float64 {
	if len(a) != len(b) {
		return 1.0
	}
	sum := 0.0
	for i := range a {
		diff := a[i] - b[i]
		sum += diff * diff
	}
	return sum
}

// RenamePerson 重命名人物
func (m *Manager) RenamePerson(personID, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	person, ok := m.persons[personID]
	if !ok {
		return errors.New("person not found")
	}
	person.Name = name
	return nil
}

// MergePersons 合并人物
func (m *Manager) MergePersons(targetID, sourceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	target, ok := m.persons[targetID]
	if !ok {
		return errors.New("target person not found")
	}
	source, ok := m.persons[sourceID]
	if !ok {
		return errors.New("source person not found")
	}

	for _, meta := range m.metadata {
		for i := range meta.Faces {
			if meta.Faces[i].PersonID == sourceID {
				meta.Faces[i].PersonID = targetID
				meta.Faces[i].Name = target.Name
			}
		}
	}

	target.FaceCount += source.FaceCount
	target.PhotoCount += source.PhotoCount
	if source.FirstSeen.Before(target.FirstSeen) {
		target.FirstSeen = source.FirstSeen
	}
	if source.LastSeen.After(target.LastSeen) {
		target.LastSeen = source.LastSeen
	}
	if features, ok := m.faceDB[sourceID]; ok {
		m.faceDB[targetID] = features
		delete(m.faceDB, sourceID)
	}
	delete(m.persons, sourceID)
	return nil
}

// ========== 智能分类 ==========

func (m *Manager) autoClassifyPhoto(photoID string, result *AIAnalysisResult) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sceneAlbum := m.findOrCreateAlbum(string(result.Scene), CategoryScene, true)
	sceneAlbum.PhotoCount++

	if photo, ok := m.photos[photoID]; ok {
		timeAlbum := m.findOrCreateAlbum(photo.TakenAt.Format("2006年01月"), CategoryTime, true)
		timeAlbum.PhotoCount++
	}
	for _, face := range result.Faces {
		if face.PersonID != "" {
			album := m.findOrCreateAlbum(face.Name, CategoryPerson, true)
			album.PhotoCount++
		}
	}
}

func (m *Manager) findOrCreateAlbum(name string, albumType CategoryType, isSmart bool) *Album {
	for _, a := range m.albums {
		if a.Name == name && a.Type == albumType {
			return a
		}
	}
	now := time.Now()
	album := &Album{
		ID: fmt.Sprintf("album-%d", now.UnixNano()), Name: name,
		Type: albumType, IsSmart: isSmart, CreatedAt: now, UpdatedAt: now,
	}
	m.albums[album.ID] = album
	return album
}

// ClassifyByTime 按时间分类
func (m *Manager) ClassifyByTime() map[string][]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string][]string)
	for _, p := range m.photos {
		result[p.TakenAt.Format("2006-01")] = append(result[p.TakenAt.Format("2006-01")], p.ID)
	}
	return result
}

// ClassifyByLocation 按地点分类
func (m *Manager) ClassifyByLocation() map[string][]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string][]string)
	for pid, meta := range m.metadata {
		if meta.GPS != nil && meta.GPS.City != "" {
			result[meta.GPS.City] = append(result[meta.GPS.City], pid)
		}
	}
	return result
}

// ClassifyByScene 按场景分类
func (m *Manager) ClassifyByScene() map[SceneType][]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[SceneType][]string)
	for pid, meta := range m.metadata {
		if meta.Scene != "" {
			result[meta.Scene] = append(result[meta.Scene], pid)
		}
	}
	return result
}

// ClassifyByPerson 按人物分类
func (m *Manager) ClassifyByPerson() map[string][]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string][]string)
	for pid, meta := range m.metadata {
		for _, f := range meta.Faces {
			if f.PersonID != "" {
				result[f.PersonID] = append(result[f.PersonID], pid)
			}
		}
	}
	return result
}

// ========== 照片去重 ==========

// FindDuplicates 查找重复照片
func (m *Manager) FindDuplicates(_ context.Context) ([]*DuplicateGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var groups []*DuplicateGroup
	for hash, pids := range m.hashIndex {
		if len(pids) > 1 {
			var totalSize int64
			for _, id := range pids {
				if p, ok := m.photos[id]; ok {
					totalSize += p.Size
				}
			}
			h := hash
			if len(h) > 8 {
				h = h[:8]
			}
			groups = append(groups, &DuplicateGroup{
				ID: fmt.Sprintf("dup-%s", h), Hash: hash,
				Similarity: 100.0, Photos: pids,
				TotalSize: totalSize, WastedSize: totalSize - m.photos[pids[0]].Size,
			})
		}
	}

	groups = append(groups, m.findSimilarPhotos()...)
	sort.Slice(groups, func(i, j int) bool { return groups[i].WastedSize > groups[j].WastedSize })
	return groups, nil
}

func (m *Manager) findSimilarPhotos() []*DuplicateGroup {
	type ph struct {
		id, hash string
	}
	var hashes []ph
	for id, _ := range m.metadata {
		if p, ok := m.photos[id]; ok && p.PerceptualHash != "" {
			hashes = append(hashes, ph{id: id, hash: p.PerceptualHash})
		}
	}

	var groups []*DuplicateGroup
	visited := make(map[string]bool)
	for i := 0; i < len(hashes); i++ {
		if visited[hashes[i].id] {
			continue
		}
		similar := []string{hashes[i].id}
		for j := i + 1; j < len(hashes); j++ {
			if visited[hashes[j].id] {
				continue
			}
			if hammingSimilarity(hashes[i].hash, hashes[j].hash) >= SimilarityThreshold {
				similar = append(similar, hashes[j].id)
				visited[hashes[j].id] = true
			}
		}
		if len(similar) > 1 {
			visited[hashes[i].id] = true
			var totalSize int64
			for _, id := range similar {
				if p, ok := m.photos[id]; ok {
					totalSize += p.Size
				}
			}
			s := similar[0]
			if len(s) > 8 {
				s = s[:8]
			}
			groups = append(groups, &DuplicateGroup{
				ID: fmt.Sprintf("sim-%s", s), Similarity: float64(SimilarityThreshold),
				Photos: similar, TotalSize: totalSize, WastedSize: totalSize - m.photos[similar[0]].Size,
			})
		}
	}
	return groups
}

func hammingSimilarity(h1, h2 string) float64 {
	if len(h1) != len(h2) || len(h1) == 0 {
		return 0
	}
	same := 0
	for i := range h1 {
		if h1[i] == h2[i] {
			same++
		}
	}
	return float64(same) / float64(len(h1)) * 100
}

// RemoveDuplicates 删除重复照片（保留指定照片）
func (m *Manager) RemoveDuplicates(_ context.Context, groupID, keepPhotoID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, ok := m.duplicates[groupID]
	if !ok {
		return errors.New("duplicate group not found")
	}
	keepFound := false
	for _, id := range group.Photos {
		if id == keepPhotoID {
			keepFound = true
			break
		}
	}
	if !keepFound {
		return errors.New("keep photo not in group")
	}

	for _, id := range group.Photos {
		if id == keepPhotoID {
			continue
		}
		if photo, ok := m.photos[id]; ok {
			trashPath := filepath.Join(m.storagePath, "trash", photo.Filename)
			os.MkdirAll(filepath.Dir(trashPath), 0755)
			os.Rename(photo.Path, trashPath)
			delete(m.photos, id)
			delete(m.metadata, id)
		}
	}
	delete(m.duplicates, groupID)
	return nil
}

// ========== 相册管理 ==========

// CreateAlbum 创建相册
func (m *Manager) CreateAlbum(name, description string, albumType CategoryType) *Album {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	album := &Album{
		ID: fmt.Sprintf("album-%d", now.UnixNano()), Name: name,
		Description: description, Type: albumType, CreatedAt: now, UpdatedAt: now,
	}
	m.albums[album.ID] = album
	return album
}

// CreateSmartAlbum 创建智能相册
func (m *Manager) CreateSmartAlbum(name, description string, rules *SmartRules) *Album {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	album := &Album{
		ID: fmt.Sprintf("album-%d", now.UnixNano()), Name: name,
		Description: description, Type: CategoryTag, IsSmart: true,
		SmartRules: rules, CreatedAt: now, UpdatedAt: now,
	}
	m.albums[album.ID] = album
	return album
}

// GetAlbum 获取相册
func (m *Manager) GetAlbum(albumID string) (*Album, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.albums[albumID]
	return a, ok
}

// ListAlbums 列出所有相册
func (m *Manager) ListAlbums() []*Album {
	m.mu.RLock()
	defer m.mu.RUnlock()
	albums := make([]*Album, 0, len(m.albums))
	for _, a := range m.albums {
		albums = append(albums, a)
	}
	sort.Slice(albums, func(i, j int) bool { return albums[i].UpdatedAt.After(albums[j].UpdatedAt) })
	return albums
}

// AddPhotoToAlbum 添加照片到相册
func (m *Manager) AddPhotoToAlbum(albumID, photoID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	album, ok := m.albums[albumID]
	if !ok {
		return errors.New("album not found")
	}
	if _, ok := m.photos[photoID]; !ok {
		return errors.New("photo not found")
	}
	album.PhotoCount++
	album.UpdatedAt = time.Now()
	return nil
}

// RemovePhotoFromAlbum 从相册移除照片
func (m *Manager) RemovePhotoFromAlbum(albumID, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	album, ok := m.albums[albumID]
	if !ok {
		return errors.New("album not found")
	}
	if album.PhotoCount > 0 {
		album.PhotoCount--
	}
	album.UpdatedAt = time.Now()
	return nil
}

// DeleteAlbum 删除相册
func (m *Manager) DeleteAlbum(albumID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.albums[albumID]; !ok {
		return errors.New("album not found")
	}
	delete(m.albums, albumID)
	return nil
}

// ========== 智能推荐 ==========

// GetRecommendations 获取相册推荐
func (m *Manager) GetRecommendations(_ context.Context) ([]*AlbumRecommendation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var recs []*AlbumRecommendation

	recent := m.getRecentPhotos(7 * 24 * time.Hour)
	if len(recent) >= 3 {
		recs = append(recs, &AlbumRecommendation{
			ID:   fmt.Sprintf("rec-%d", time.Now().UnixNano()),
			Name: "本周精选", Description: "最近一周的精彩照片",
			Type: CategoryTime, PhotoIDs: recent, Confidence: 0.9,
			Reason: "基于拍摄时间的智能推荐",
		})
	}

	landscapes := m.getPhotosByScene(SceneLandscape)
	if len(landscapes) >= 3 {
		recs = append(recs, &AlbumRecommendation{
			ID:   fmt.Sprintf("rec-%d", time.Now().UnixNano()+1),
			Name: "风景集锦", Description: "AI识别的风景照片合集",
			Type: CategoryScene, PhotoIDs: landscapes, Confidence: 0.85,
			Reason: "基于场景识别的智能推荐",
		})
	}

	for pid, person := range m.persons {
		personPhotos := m.getPhotosByPerson(pid)
		if len(personPhotos) >= 5 {
			recs = append(recs, &AlbumRecommendation{
				ID:   fmt.Sprintf("rec-%d", time.Now().UnixNano()+2),
				Name: person.Name + "的照片", Description: fmt.Sprintf("包含%d张照片", len(personPhotos)),
				Type: CategoryPerson, PhotoIDs: personPhotos, Confidence: 0.8,
				Reason: "基于人脸识别的智能推荐",
			})
		}
	}

	return recs, nil
}

func (m *Manager) getRecentPhotos(d time.Duration) []string {
	var ids []string
	cutoff := time.Now().Add(-d)
	for _, p := range m.photos {
		if p.TakenAt.After(cutoff) {
			ids = append(ids, p.ID)
		}
	}
	return ids
}

func (m *Manager) getPhotosByScene(scene SceneType) []string {
	var ids []string
	for pid, meta := range m.metadata {
		if meta.Scene == scene {
			ids = append(ids, pid)
		}
	}
	return ids
}

func (m *Manager) getPhotosByPerson(personID string) []string {
	var ids []string
	for pid, meta := range m.metadata {
		for _, f := range meta.Faces {
			if f.PersonID == personID {
				ids = append(ids, pid)
			}
		}
	}
	return ids
}

// ListPersons 列出所有人物
func (m *Manager) ListPersons() []*Person {
	m.mu.RLock()
	defer m.mu.RUnlock()
	persons := make([]*Person, 0, len(m.persons))
	for _, p := range m.persons {
		persons = append(persons, p)
	}
	sort.Slice(persons, func(i, j int) bool { return persons[i].LastSeen.After(persons[j].LastSeen) })
	return persons
}

// GetPerson 获取人物
func (m *Manager) GetPerson(personID string) (*Person, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.persons[personID]
	return p, ok
}

// GetPhoto 获取照片
func (m *Manager) GetPhoto(photoID string) (*Photo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.photos[photoID]
	return p, ok
}

// GetPhotoMetadata 获取照片元数据
func (m *Manager) GetPhotoMetadata(photoID string) (*PhotoMetadata, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	meta, ok := m.metadata[photoID]
	return meta, ok
}

// ListPhotos 列出所有照片
func (m *Manager) ListPhotos() []*Photo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	photos := make([]*Photo, 0, len(m.photos))
	for _, p := range m.photos {
		photos = append(photos, p)
	}
	sort.Slice(photos, func(i, j int) bool { return photos[i].TakenAt.After(photos[j].TakenAt) })
	return photos
}

// UpdatePhoto 更新照片信息
func (m *Manager) UpdatePhoto(photoID string, updates map[string]interface{}) (*Photo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	photo, ok := m.photos[photoID]
	if !ok {
		return nil, errors.New("photo not found")
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
	if v, ok := updates["tags"].([]string); ok {
		photo.Tags = v
	}
	photo.UpdatedAt = time.Now()
	return photo, nil
}

// DeletePhoto 删除照片
func (m *Manager) DeletePhoto(photoID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	photo, ok := m.photos[photoID]
	if !ok {
		return errors.New("photo not found")
	}

	trashPath := filepath.Join(m.storagePath, "trash", photo.Filename)
	os.MkdirAll(filepath.Dir(trashPath), 0755)
	os.Rename(photo.Path, trashPath)

	delete(m.photos, photoID)
	delete(m.metadata, photoID)
	return nil
}

// ========== 照片备份 ==========

// StartBackup 启动备份任务
func (m *Manager) StartBackup(ctx context.Context, sourcePath, targetPath string) (*BackupTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task := &BackupTask{
		ID:         fmt.Sprintf("backup-%d", time.Now().UnixNano()),
		Status:     BackupStatusPending,
		SourcePath: sourcePath,
		TargetPath: targetPath,
		StartedAt:  time.Now(),
	}
	m.backupTasks[task.ID] = task

	go m.executeBackup(ctx, task)
	return task, nil
}

func (m *Manager) executeBackup(ctx context.Context, task *BackupTask) {
	m.mu.Lock()
	task.Status = BackupStatusRunning
	m.mu.Unlock()

	err := filepath.Walk(task.SourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !SupportedFormats[ext] {
			return nil
		}

		relPath, _ := filepath.Rel(task.SourcePath, path)
		dstPath := filepath.Join(task.TargetPath, relPath)
		os.MkdirAll(filepath.Dir(dstPath), 0755)

		if err := copyFile(path, dstPath); err != nil {
			m.mu.Lock()
			task.FailedFiles++
			m.mu.Unlock()
			return nil
		}

		m.mu.Lock()
		task.DoneFiles++
		task.DoneSize += info.Size()
		m.mu.Unlock()
		return nil
	})

	m.mu.Lock()
	now := time.Now()
	task.CompletedAt = &now
	if err != nil && err != context.Canceled {
		task.Status = BackupStatusFailed
		task.Error = err.Error()
	} else {
		task.Status = BackupStatusCompleted
	}
	m.mu.Unlock()
}

// GetBackupTask 获取备份任务状态
func (m *Manager) GetBackupTask(taskID string) (*BackupTask, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.backupTasks[taskID]
	return t, ok
}

// ListBackupTasks 列出所有备份任务
func (m *Manager) ListBackupTasks() []*BackupTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tasks := make([]*BackupTask, 0, len(m.backupTasks))
	for _, t := range m.backupTasks {
		tasks = append(tasks, t)
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].StartedAt.After(tasks[j].StartedAt) })
	return tasks
}

// ========== 照片同步 ==========

// ConfigureSync 配置同步
func (m *Manager) ConfigureSync(config *SyncConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.syncConfig = config
	m.syncState.Status = SyncStatusIdle
	return nil
}

// GetSyncState 获取同步状态
func (m *Manager) GetSyncState() *SyncState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state := *m.syncState
	return &state
}

// StartSync 启动同步
func (m *Manager) StartSync(ctx context.Context) error {
	m.mu.RLock()
	config := m.syncConfig
	m.mu.RUnlock()

	if config == nil {
		return errors.New("sync not configured")
	}

	m.mu.Lock()
	m.syncState.Status = SyncStatusSyncing
	m.mu.Unlock()

	go m.executeSync(ctx, config)
	return nil
}

func (m *Manager) executeSync(ctx context.Context, config *SyncConfig) {
	syncedCount, failedCount := 0, 0

	err := filepath.Walk(config.SourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !SupportedFormats[ext] {
			return nil
		}

		// 检查排除模式
		for _, pattern := range config.ExcludePatterns {
			if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
				return nil
			}
		}

		relPath, _ := filepath.Rel(config.SourcePath, path)
		for _, targetPath := range config.TargetPaths {
			dstPath := filepath.Join(targetPath, relPath)
			os.MkdirAll(filepath.Dir(dstPath), 0755)

			if err := copyFile(path, dstPath); err != nil {
				failedCount++
			} else {
				syncedCount++
			}
		}
		return nil
	})

	m.mu.Lock()
	now := time.Now()
	m.syncState.LastSyncAt = &now
	m.syncState.SyncedCount += syncedCount
	m.syncState.FailedCount += failedCount
	if err != nil && err != context.Canceled {
		m.syncState.Status = SyncStatusError
		m.syncState.ErrorMessage = err.Error()
	} else {
		m.syncState.Status = SyncStatusIdle
	}
	m.mu.Unlock()
}

// ========== 搜索 ==========

// SearchPhotos 搜索照片
func (m *Manager) SearchPhotos(query string, tags []string, scene SceneType, dateFrom, dateTo *time.Time) []*Photo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*Photo
	query = strings.ToLower(query)

	for _, photo := range m.photos {
		if query != "" {
			matched := strings.Contains(strings.ToLower(photo.Filename), query)
			if !matched {
				if meta, ok := m.metadata[photo.ID]; ok {
					for _, tag := range meta.Tags {
						if strings.Contains(strings.ToLower(tag), query) {
							matched = true
							break
						}
					}
				}
			}
			if !matched {
				continue
			}
		}

		if len(tags) > 0 {
			meta, hasMeta := m.metadata[photo.ID]
			if !hasMeta {
				continue
			}
			tagMatch := false
			for _, t := range tags {
				for _, mt := range meta.Tags {
					if strings.EqualFold(t, mt) {
						tagMatch = true
						break
					}
				}
				if tagMatch {
					break
				}
			}
			if !tagMatch {
				continue
			}
		}

		if scene != "" {
			meta, hasMeta := m.metadata[photo.ID]
			if !hasMeta || meta.Scene != scene {
				continue
			}
		}

		if dateFrom != nil && photo.TakenAt.Before(*dateFrom) {
			continue
		}
		if dateTo != nil && photo.TakenAt.After(*dateTo) {
			continue
		}

		results = append(results, photo)
	}

	sort.Slice(results, func(i, j int) bool { return results[i].TakenAt.After(results[j].TakenAt) })
	return results
}

// ========== 统计 ==========

// GetStats 获取统计信息
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var totalSize int64
	for _, p := range m.photos {
		totalSize += p.Size
	}

	return map[string]interface{}{
		"total_photos":  len(m.photos),
		"total_albums":  len(m.albums),
		"total_persons": len(m.persons),
		"total_size":    totalSize,
		"ai_enabled":    m.aiEnabled,
	}
}

// ========== 工具函数 ==========

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

func detectMimeType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	mimeTypes := map[string]string{
		".jpg": "image/jpeg", ".jpeg": "image/jpeg",
		".png": "image/png", ".gif": "image/gif",
		".bmp": "image/bmp", ".tiff": "image/tiff",
		".webp": "image/webp", ".heic": "image/heic",
		".heif": "image/heif",
	}
	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func randomHex(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = "0123456789abcdef"[time.Now().UnixNano()%16]
		time.Sleep(1)
	}
	return string(b)
}
