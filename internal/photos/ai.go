// Package photos AI 相册功能模块
// 提供人脸识别、场景分类、物体检测、智能相册等功能
package photos

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AIEngine AI 引擎接口
type AIEngine interface {
	DetectFaces(img image.Image) ([]FaceInfo, error)
	ClassifyScene(img image.Image) (string, float32, error)
	DetectObjects(img image.Image) ([]string, error)
	ExtractColors(img image.Image) ([]string, error)
	QualityScore(img image.Image) (*QualityMetrics, error)
	GenerateTags(result *AIClassification) []string
}

// LocalAIEngine 本地 AI 引擎（基于 CPU）
type LocalAIEngine struct {
	modelDir string
	enabled  bool
}

// CloudAIEngine 云端 AI 引擎（可选，更高精度）
type CloudAIEngine struct {
	apiKey   string
	endpoint string
	enabled  bool
}

// AITask AI 处理任务
type AITask struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"` // face_detect, scene_classify, object_detect, analyze_all
	PhotoID     string            `json:"photoId"`
	PhotoPath   string            `json:"photoPath"`
	Status      string            `json:"status"` // pending, running, completed, failed
	Progress    int               `json:"progress"`
	Result      *AIClassification `json:"result,omitempty"`
	Error       string            `json:"error,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	StartedAt   time.Time         `json:"startedAt,omitempty"`
	CompletedAt time.Time         `json:"completedAt,omitempty"`
}

// AIMemory AI 处理记录（用于持久化）
type AIMemory struct {
	PhotoID        string            `json:"photoId"`
	Classification *AIClassification `json:"classification"`
	ProcessedAt    time.Time         `json:"processedAt"`
	ModelVersion   string            `json:"modelVersion"`
}

// SmartAlbum 智能相册（自动生成）
type SmartAlbum struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Type        string                 `json:"type"`     // person, scene, object, location, time
	Criteria    map[string]interface{} `json:"criteria"` // 生成条件
	PhotoIDs    []string               `json:"photoIds"`
	AutoUpdate  bool                   `json:"autoUpdate"`
	CreatedAt   time.Time              `json:"createdAt"`
	UpdatedAt   time.Time              `json:"updatedAt"`
}

// MemoryAlbum 回忆相册（历史上的今天）
type MemoryAlbum struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Date         string    `json:"date"` // MM-DD
	Year         int       `json:"year"`
	PhotoIDs     []string  `json:"photoIds"`
	CoverPhotoID string    `json:"coverPhotoId"`
	CreatedAt    time.Time `json:"createdAt"`
}

// AIManager AI 相册管理器
type AIManager struct {
	photosManager *Manager
	localEngine   *LocalAIEngine
	cloudEngine   *CloudAIEngine
	useCloud      bool
	taskQueue     chan *AITask
	taskResults   map[string]*AITask
	taskMu        sync.RWMutex
	memoryCache   map[string]*AIMemory
	memoryMu      sync.RWMutex
	smartAlbums   map[string]*SmartAlbum
	albumMu       sync.RWMutex
	stopChan      chan struct{}
	wg            sync.WaitGroup
}

// NewAIManager 创建 AI 相册管理器
func NewAIManager(pm *Manager, modelDir string) (*AIManager, error) {
	aim := &AIManager{
		photosManager: pm,
		localEngine: &LocalAIEngine{
			modelDir: modelDir,
			enabled:  true,
		},
		cloudEngine: &CloudAIEngine{
			enabled: false, // 默认关闭云端
		},
		useCloud:    false,
		taskQueue:   make(chan *AITask, 100),
		taskResults: make(map[string]*AITask),
		memoryCache: make(map[string]*AIMemory),
		smartAlbums: make(map[string]*SmartAlbum),
		stopChan:    make(chan struct{}),
	}

	// 加载 AI 内存数据
	if err := aim.loadAIMemory(); err != nil {
		return nil, fmt.Errorf("加载 AI 内存失败：%w", err)
	}

	// 加载智能相册
	if err := aim.loadSmartAlbums(); err != nil {
		return nil, fmt.Errorf("加载智能相册失败：%w", err)
	}

	// 启动后台处理协程
	aim.wg.Add(3)
	go aim.processTaskQueue()
	go aim.autoGenerateMemories()
	go aim.updateSmartAlbums()

	return aim, nil
}

// processTaskQueue 处理任务队列
func (aim *AIManager) processTaskQueue() {
	defer aim.wg.Done()

	for {
		select {
		case task := <-aim.taskQueue:
			aim.executeTask(task)
		case <-aim.stopChan:
			return
		}
	}
}

// executeTask 执行单个 AI 任务
func (aim *AIManager) executeTask(task *AITask) {
	task.Status = "running"
	task.StartedAt = time.Now()
	aim.updateTask(task)

	defer func() {
		task.CompletedAt = time.Now()
		if task.Status != "failed" {
			task.Status = "completed"
			task.Progress = 100
		}
		aim.updateTask(task)
	}()

	// 打开图片
	img, err := aim.loadImage(task.PhotoPath)
	if err != nil {
		task.Error = fmt.Sprintf("加载图片失败：%v", err)
		task.Status = "failed"
		return
	}

	result := &AIClassification{
		PhotoID:  task.PhotoID,
		Metadata: make(map[string]interface{}),
	}

	switch task.Type {
	case "face_detect":
		task.Progress = 30
		faces, err := aim.detectFaces(img)
		if err != nil {
			task.Error = fmt.Sprintf("人脸检测失败：%v", err)
			task.Status = "failed"
			return
		}
		result.Faces = faces
		task.Progress = 100

	case "scene_classify":
		task.Progress = 30
		scene, confidence, err := aim.classifyScene(img)
		if err != nil {
			task.Error = fmt.Sprintf("场景分类失败：%v", err)
			task.Status = "failed"
			return
		}
		result.Scene = scene
		result.Confidence = confidence
		task.Progress = 100

	case "object_detect":
		task.Progress = 30
		objects, err := aim.detectObjects(img)
		if err != nil {
			task.Error = fmt.Sprintf("物体检测失败：%v", err)
			task.Status = "failed"
			return
		}
		result.Objects = objects
		task.Progress = 100

	case "analyze_all":
		// 完整分析
		// 1. 人脸检测
		task.Progress = 20
		faces, err := aim.detectFaces(img)
		if err != nil {
			// 人脸检测失败不影响其他分析
			result.Metadata["face_error"] = err.Error()
		} else {
			result.Faces = faces
		}

		// 2. 场景分类
		task.Progress = 35
		scene, confidence, err := aim.classifyScene(img)
		if err != nil {
			result.Metadata["scene_error"] = err.Error()
		} else {
			result.Scene = scene
			result.Confidence = confidence
		}

		// 3. 物体检测
		task.Progress = 50
		objects, err := aim.detectObjects(img)
		if err != nil {
			result.Metadata["object_error"] = err.Error()
		} else {
			result.Objects = objects
		}

		// 4. 颜色提取
		task.Progress = 65
		colors, err := aim.extractColors(img)
		if err != nil {
			result.Metadata["color_error"] = err.Error()
		} else {
			result.Colors = colors
		}

		// 5. 质量评分
		task.Progress = 80
		qualityMetrics, err := aim.qualityScore(img)
		if err != nil {
			result.Metadata["quality_error"] = err.Error()
		} else {
			result.QualityScore = qualityMetrics.OverallScore
			result.Metadata["qualityMetrics"] = qualityMetrics
		}

		// 6. 自动标签生成
		task.Progress = 90
		autoTags := aim.generateTags(result)
		result.AutoTags = autoTags

		task.Progress = 100

	default:
		task.Error = fmt.Sprintf("未知任务类型：%s", task.Type)
		task.Status = "failed"
		return
	}

	result.IsNSFW = aim.detectNSFW(result)

	// 保存结果
	aim.saveAIMemory(&AIMemory{
		PhotoID:        task.PhotoID,
		Classification: result,
		ProcessedAt:    time.Now(),
		ModelVersion:   "v1.0",
	})

	// 更新照片信息
	aim.updatePhotoWithAIResult(task.PhotoID, result)

	task.Result = result
}

// loadImage 加载图片
func (aim *AIManager) loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	img, _, err := image.Decode(f)
	if err != nil {
		// 尝试使用 ffmpeg 转换
		return aim.loadImageFFmpeg(path)
	}

	return img, nil
}

// loadImageFFmpeg 使用 ffmpeg 加载图片（支持 HEIC、RAW 等格式）
func (aim *AIManager) loadImageFFmpeg(path string) (image.Image, error) {
	// 创建临时文件
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("heic_%s.jpg", uuid.New().String()))
	defer func() { _ = os.Remove(tmpFile) }()

	// 使用 ffmpeg 转换 HEIC/RAW 为 JPEG
	cmd := exec.Command("ffmpeg",
		"-i", path,
		"-vf", "scale=2048:2048:force_original_aspect_ratio=decrease",
		"-q:v", "2",
		"-y", tmpFile,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ffmpeg 转换失败: %v, output: %s", err, string(output))
	}

	// 读取转换后的图片
	f, err := os.Open(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("打开转换后的文件失败: %w", err)
	}
	defer func() { _ = f.Close() }()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("解码转换后的图片失败: %w", err)
	}

	return img, nil
}

// detectFaces 人脸检测
func (aim *AIManager) detectFaces(img image.Image) ([]FaceInfo, error) {
	if aim.useCloud && aim.cloudEngine.enabled {
		return aim.cloudEngine.DetectFaces(img)
	}
	return aim.localEngine.DetectFaces(img)
}

// classifyScene 场景分类
func (aim *AIManager) classifyScene(img image.Image) (string, float32, error) {
	if aim.useCloud && aim.cloudEngine.enabled {
		return aim.cloudEngine.ClassifyScene(img)
	}
	return aim.localEngine.ClassifyScene(img)
}

// detectObjects 物体检测
func (aim *AIManager) detectObjects(img image.Image) ([]string, error) {
	if aim.useCloud && aim.cloudEngine.enabled {
		return aim.cloudEngine.DetectObjects(img)
	}
	return aim.localEngine.DetectObjects(img)
}

// extractColors 提取主色调
func (aim *AIManager) extractColors(img image.Image) ([]string, error) {
	if aim.useCloud && aim.cloudEngine.enabled {
		return aim.cloudEngine.ExtractColors(img)
	}
	return aim.localEngine.ExtractColors(img)
}

// qualityScore 计算照片质量评分
func (aim *AIManager) qualityScore(img image.Image) (*QualityMetrics, error) {
	if aim.useCloud && aim.cloudEngine.enabled {
		return aim.localEngine.QualityScore(img) // 云端暂不支持，使用本地
	}
	return aim.localEngine.QualityScore(img)
}

// generateTags 生成自动标签
func (aim *AIManager) generateTags(result *AIClassification) []string {
	return aim.localEngine.GenerateTags(result)
}

// detectNSFW 检测不当内容
func (aim *AIManager) detectNSFW(result *AIClassification) bool {
	// 简化实现：基于场景和物体判断
	nsfwScenes := []string{"nsfw", "explicit"}
	for _, scene := range nsfwScenes {
		if strings.Contains(strings.ToLower(result.Scene), scene) {
			return true
		}
	}
	return false
}

// updatePhotoWithAIResult 更新照片的 AI 信息
func (aim *AIManager) updatePhotoWithAIResult(photoID string, result *AIClassification) {
	aim.photosManager.mu.Lock()
	defer aim.photosManager.mu.Unlock()

	photo, exists := aim.photosManager.photos[photoID]
	if !exists {
		return
	}

	if len(result.Faces) > 0 {
		photo.Faces = result.Faces
	}
	if len(result.Objects) > 0 {
		photo.Objects = result.Objects
	}
	if result.Scene != "" {
		photo.Scene = result.Scene
	}
	if len(result.Colors) > 0 {
		photo.ColorPalette = result.Colors
	}

	// 更新人物统计
	for _, face := range result.Faces {
		if face.Name != "" {
			if person, exists := aim.photosManager.persons[face.Name]; exists {
				person.PhotoCount++
			}
		}
	}

	_ = aim.photosManager.savePersons()
}

// QueueTask 添加任务到队列
func (aim *AIManager) QueueTask(task *AITask) string {
	task.ID = uuid.New().String()
	task.Status = "pending"
	task.CreatedAt = time.Now()
	aim.taskQueue <- task
	aim.updateTask(task)
	return task.ID
}

// updateTask 更新任务状态
func (aim *AIManager) updateTask(task *AITask) {
	aim.taskMu.Lock()
	defer aim.taskMu.Unlock()
	aim.taskResults[task.ID] = task
}

// GetTaskStatus 获取任务状态
func (aim *AIManager) GetTaskStatus(taskID string) (*AITask, error) {
	aim.taskMu.RLock()
	defer aim.taskMu.RUnlock()

	task, exists := aim.taskResults[taskID]
	if !exists {
		return nil, fmt.Errorf("任务不存在")
	}

	return task, nil
}

// ListTasks 列出所有任务
func (aim *AIManager) ListTasks(status string) []*AITask {
	aim.taskMu.RLock()
	defer aim.taskMu.RUnlock()

	result := make([]*AITask, 0)
	for _, task := range aim.taskResults {
		if status == "" || task.Status == status {
			result = append(result, task)
		}
	}

	// 按创建时间排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}

// AnalyzePhoto 分析单张照片
func (aim *AIManager) AnalyzePhoto(photoID, photoPath string) string {
	task := &AITask{
		Type:      "analyze_all",
		PhotoID:   photoID,
		PhotoPath: photoPath,
	}
	return aim.QueueTask(task)
}

// BatchAnalyze 批量分析照片
func (aim *AIManager) BatchAnalyze(photos []*Photo) []string {
	taskIDs := make([]string, 0, len(photos))
	for _, photo := range photos {
		photoPath := filepath.Join(aim.photosManager.photosDir, photo.Path)
		taskID := aim.AnalyzePhoto(photo.ID, photoPath)
		taskIDs = append(taskIDs, taskID)
	}
	return taskIDs
}

// loadAIMemory 加载 AI 内存数据
func (aim *AIManager) loadAIMemory() error {
	path := filepath.Join(aim.photosManager.dataDir, "ai-memory.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var memories []AIMemory
	if err := json.Unmarshal(data, &memories); err != nil {
		return err
	}

	aim.memoryMu.Lock()
	defer aim.memoryMu.Unlock()

	for i := range memories {
		aim.memoryCache[memories[i].PhotoID] = &memories[i]
	}

	return nil
}

// saveAIMemory 保存 AI 内存数据
func (aim *AIManager) saveAIMemory(memory *AIMemory) {
	aim.memoryMu.Lock()
	defer aim.memoryMu.Unlock()

	aim.memoryCache[memory.PhotoID] = memory

	// 定期持久化到磁盘
	go func() {
		time.Sleep(5 * time.Second)
		_ = aim.persistAIMemory()
	}()
}

// persistAIMemory 持久化 AI 内存到磁盘
func (aim *AIManager) persistAIMemory() error {
	aim.memoryMu.RLock()
	defer aim.memoryMu.RUnlock()

	memories := make([]AIMemory, 0, len(aim.memoryCache))
	for _, m := range aim.memoryCache {
		memories = append(memories, *m)
	}

	data, err := json.MarshalIndent(memories, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(aim.photosManager.dataDir, "ai-memory.json")
	return os.WriteFile(path, data, 0644)
}

// GetPhotoAIResult 获取照片的 AI 分析结果
func (aim *AIManager) GetPhotoAIResult(photoID string) (*AIClassification, error) {
	aim.memoryMu.RLock()
	defer aim.memoryMu.RUnlock()

	memory, exists := aim.memoryCache[photoID]
	if !exists {
		return nil, fmt.Errorf("未找到 AI 分析结果")
	}

	return memory.Classification, nil
}

// loadSmartAlbums 加载智能相册
func (aim *AIManager) loadSmartAlbums() error {
	path := filepath.Join(aim.photosManager.dataDir, "smart-albums.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var albums []SmartAlbum
	if err := json.Unmarshal(data, &albums); err != nil {
		return err
	}

	aim.albumMu.Lock()
	defer aim.albumMu.Unlock()

	for i := range albums {
		aim.smartAlbums[albums[i].ID] = &albums[i]
	}

	return nil
}

// saveSmartAlbums 保存智能相册
func (aim *AIManager) saveSmartAlbums() error {
	aim.albumMu.RLock()
	defer aim.albumMu.RUnlock()

	albums := make([]SmartAlbum, 0, len(aim.smartAlbums))
	for _, album := range aim.smartAlbums {
		albums = append(albums, *album)
	}

	data, err := json.MarshalIndent(albums, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(aim.photosManager.dataDir, "smart-albums.json")
	return os.WriteFile(path, data, 0644)
}

// CreateSmartAlbum 创建智能相册
func (aim *AIManager) CreateSmartAlbum(name, albumType string, criteria map[string]interface{}) (*SmartAlbum, error) {
	album := &SmartAlbum{
		ID:         uuid.New().String(),
		Name:       name,
		Type:       albumType,
		Criteria:   criteria,
		PhotoIDs:   make([]string, 0),
		AutoUpdate: true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	switch albumType {
	case "person":
		album.Description = "包含特定人物的照片"
	case "scene":
		album.Description = "特定场景的照片"
	case "object":
		album.Description = "包含特定物体的照片"
	case "location":
		album.Description = "特定地点的照片"
	case "time":
		album.Description = "特定时间的照片"
	default:
		album.Description = "智能生成的相册"
	}

	aim.albumMu.Lock()
	aim.smartAlbums[album.ID] = album
	aim.albumMu.Unlock()

	// 初始填充照片
	aim.populateSmartAlbum(album)

	if err := aim.saveSmartAlbums(); err != nil {
		return nil, err
	}

	return album, nil
}

// populateSmartAlbum 填充智能相册
func (aim *AIManager) populateSmartAlbum(album *SmartAlbum) {
	aim.photosManager.mu.RLock()
	defer aim.photosManager.mu.RUnlock()

	photoIDs := make([]string, 0)

	for _, photo := range aim.photosManager.photos {
		if aim.photoMatchesCriteria(photo, album.Criteria) {
			photoIDs = append(photoIDs, photo.ID)
		}
	}

	album.PhotoIDs = photoIDs
	album.UpdatedAt = time.Now()
}

// photoMatchesCriteria 照片是否匹配条件
func (aim *AIManager) photoMatchesCriteria(photo *Photo, criteria map[string]interface{}) bool {
	for key, value := range criteria {
		switch key {
		case "person":
			if name, ok := value.(string); ok {
				found := false
				for _, face := range photo.Faces {
					if face.Name == name {
						found = true
						break
					}
				}
				if !found {
					return false
				}
			}

		case "scene":
			if scene, ok := value.(string); ok {
				if photo.Scene != scene {
					return false
				}
			}

		case "object":
			if obj, ok := value.(string); ok {
				found := false
				for _, o := range photo.Objects {
					if o == obj {
						found = true
						break
					}
				}
				if !found {
					return false
				}
			}

		case "location":
			if loc, ok := value.(string); ok {
				if photo.Location == nil || photo.Location.City != loc {
					return false
				}
			}

		case "date_range":
			// 支持多种日期范围格式
			if dateRange, ok := value.(map[string]interface{}); ok {
				// 开始日期
				if startStr, ok := dateRange["start"].(string); ok {
					if start, err := time.Parse("2006-01-02", startStr); err == nil {
						if photo.TakenAt.Before(start) {
							return false
						}
					}
				}
				// 结束日期
				if endStr, ok := dateRange["end"].(string); ok {
					if end, err := time.Parse("2006-01-02", endStr); err == nil {
						// 结束日期包含当天，所以加一天
						if photo.TakenAt.After(end.AddDate(0, 0, 1)) {
							return false
						}
					}
				}
				// 年份范围
				if yearStart, ok := dateRange["yearStart"].(float64); ok {
					if photo.TakenAt.Year() < int(yearStart) {
						return false
					}
				}
				if yearEnd, ok := dateRange["yearEnd"].(float64); ok {
					if photo.TakenAt.Year() > int(yearEnd) {
						return false
					}
				}
				// 月份范围
				if monthStart, ok := dateRange["monthStart"].(float64); ok {
					if photo.TakenAt.Month() < time.Month(monthStart) {
						return false
					}
				}
				if monthEnd, ok := dateRange["monthEnd"].(float64); ok {
					if photo.TakenAt.Month() > time.Month(monthEnd) {
						return false
					}
				}
			}
		}
	}

	return true
}

// ListSmartAlbums 列出智能相册
func (aim *AIManager) ListSmartAlbums() []*SmartAlbum {
	aim.albumMu.RLock()
	defer aim.albumMu.RUnlock()

	result := make([]*SmartAlbum, 0, len(aim.smartAlbums))
	for _, album := range aim.smartAlbums {
		result = append(result, album)
	}

	return result
}

// DeleteSmartAlbum 删除智能相册
func (aim *AIManager) DeleteSmartAlbum(albumID string) error {
	aim.albumMu.Lock()
	defer aim.albumMu.Unlock()

	if _, exists := aim.smartAlbums[albumID]; !exists {
		return fmt.Errorf("智能相册不存在")
	}

	delete(aim.smartAlbums, albumID)
	return aim.saveSmartAlbums()
}

// updateSmartAlbums 定期更新智能相册
func (aim *AIManager) updateSmartAlbums() {
	defer aim.wg.Done()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			aim.albumMu.RLock()
			albums := make([]*SmartAlbum, 0, len(aim.smartAlbums))
			for _, album := range aim.smartAlbums {
				if album.AutoUpdate {
					albums = append(albums, album)
				}
			}
			aim.albumMu.RUnlock()

			for _, album := range albums {
				aim.populateSmartAlbum(album)
			}
			_ = aim.saveSmartAlbums()

		case <-aim.stopChan:
			return
		}
	}
}

// autoGenerateMemories 自动生成回忆
func (aim *AIManager) autoGenerateMemories() {
	defer aim.wg.Done()

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// 启动时立即生成一次
	aim.generateMemories()

	for {
		select {
		case <-ticker.C:
			aim.generateMemories()
		case <-aim.stopChan:
			return
		}
	}
}

// generateMemories 生成回忆（历史上的今天）
func (aim *AIManager) generateMemories() {
	today := time.Now()
	monthDay := today.Format("01-02")

	aim.photosManager.mu.RLock()
	defer aim.photosManager.mu.RUnlock()

	// 查找往年今天的照片
	photosByYear := make(map[int][]*Photo)
	for _, photo := range aim.photosManager.photos {
		if photo.TakenAt.Format("01-02") == monthDay {
			year := photo.TakenAt.Year()
			photosByYear[year] = append(photosByYear[year], photo)
		}
	}

	// 为每个年份创建回忆
	for year, photos := range photosByYear {
		if len(photos) == 0 {
			continue
		}

		memory := &MemoryAlbum{
			ID:        uuid.New().String(),
			Title:     fmt.Sprintf("%d 年的今天", year),
			Date:      monthDay,
			Year:      year,
			PhotoIDs:  make([]string, 0, len(photos)),
			CreatedAt: time.Now(),
		}

		for _, photo := range photos {
			memory.PhotoIDs = append(memory.PhotoIDs, photo.ID)
		}

		// 设置封面
		if len(photos) > 0 {
			memory.CoverPhotoID = photos[0].ID
		}

		// 保存到存储
		aim.saveMemory(memory)
	}
}

// saveMemory 保存单个回忆到存储
func (aim *AIManager) saveMemory(memory *MemoryAlbum) {
	memoriesPath := filepath.Join(aim.photosManager.dataDir, "memories.json")

	// 确保目录存在
	if err := os.MkdirAll(aim.photosManager.dataDir, 0755); err != nil {
		return
	}

	// 读取已保存的回忆
	var memories []MemoryAlbum
	if data, err := os.ReadFile(memoriesPath); err == nil {
		_ = json.Unmarshal(data, &memories)
	}

	// 检查是否已存在相同年份和日期的回忆
	for i, m := range memories {
		if m.Date == memory.Date && m.Year == memory.Year {
			// 更新已有的回忆
			memories[i] = *memory
			data, err := json.MarshalIndent(memories, "", "  ")
			if err != nil {
				return
			}
			_ = os.WriteFile(memoriesPath, data, 0644)
			return
		}
	}

	// 添加新回忆
	memories = append(memories, *memory)

	// 保存到文件
	data, err := json.MarshalIndent(memories, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(memoriesPath, data, 0644)
}

// GetMemories 获取回忆列表
func (aim *AIManager) GetMemories(monthDay string) []*MemoryAlbum {
	aim.photosManager.mu.RLock()
	defer aim.photosManager.mu.RUnlock()

	// 如果没有指定日期，使用今天
	if monthDay == "" {
		monthDay = time.Now().Format("01-02")
	}

	// 加载已保存的回忆
	memories := make([]*MemoryAlbum, 0)
	memoriesPath := filepath.Join(aim.photosManager.dataDir, "memories.json")
	if data, err := os.ReadFile(memoriesPath); err == nil {
		var savedMemories []MemoryAlbum
		if json.Unmarshal(data, &savedMemories) == nil {
			for _, m := range savedMemories {
				if m.Date == monthDay {
					memories = append(memories, &m)
				}
			}
		}
	}

	// 实时查找历史上的今天
	photosByYear := make(map[int][]*Photo)
	currentYear := time.Now().Year()

	for _, photo := range aim.photosManager.photos {
		// 只查找非隐藏照片
		if photo.IsHidden {
			continue
		}
		if photo.TakenAt.Format("01-02") == monthDay {
			year := photo.TakenAt.Year()
			// 排除今年的照片
			if year < currentYear {
				photosByYear[year] = append(photosByYear[year], photo)
			}
		}
	}

	// 为每个年份创建回忆
	for year, photos := range photosByYear {
		if len(photos) == 0 {
			continue
		}

		// 检查是否已存在
		found := false
		for _, m := range memories {
			if m.Year == year && m.Date == monthDay {
				found = true
				break
			}
		}
		if found {
			continue
		}

		yearsAgo := currentYear - year
		var title string
		if yearsAgo == 1 {
			title = "去年的今天"
		} else {
			title = fmt.Sprintf("%d 年前的今天", yearsAgo)
		}

		memory := &MemoryAlbum{
			ID:        uuid.New().String(),
			Title:     title,
			Date:      monthDay,
			Year:      year,
			PhotoIDs:  make([]string, 0, len(photos)),
			CreatedAt: time.Now(),
		}

		for _, photo := range photos {
			memory.PhotoIDs = append(memory.PhotoIDs, photo.ID)
		}

		// 设置封面为第一张照片
		if len(photos) > 0 {
			memory.CoverPhotoID = photos[0].ID
		}

		memories = append(memories, memory)
	}

	// 按年份降序排序
	sort.Slice(memories, func(i, j int) bool {
		return memories[i].Year > memories[j].Year
	})

	return memories
}

// FindSimilarPhotos 查找相似照片
func (aim *AIManager) FindSimilarPhotos(photoID string, limit int) ([]*Photo, error) {
	aim.memoryMu.RLock()
	_, exists := aim.memoryCache[photoID]
	aim.memoryMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("源照片未分析")
	}

	aim.photosManager.mu.RLock()
	defer aim.photosManager.mu.RUnlock()

	sourcePhoto := aim.photosManager.photos[photoID]
	if sourcePhoto == nil {
		return nil, fmt.Errorf("源照片不存在")
	}

	similar := make([]*Photo, 0)
	scores := make(map[string]float64)

	for _, photo := range aim.photosManager.photos {
		if photo.ID == photoID {
			continue
		}

		score := aim.calculateSimilarity(sourcePhoto, photo)
		if score > 0.5 { // 相似度阈值
			scores[photo.ID] = score
			similar = append(similar, photo)
		}
	}

	// 按相似度排序
	sort.Slice(similar, func(i, j int) bool {
		return scores[similar[i].ID] > scores[similar[j].ID]
	})

	if limit > 0 && len(similar) > limit {
		similar = similar[:limit]
	}

	return similar, nil
}

// calculateSimilarity 计算照片相似度
func (aim *AIManager) calculateSimilarity(p1, p2 *Photo) float64 {
	score := 0.0
	factors := 0.0

	// 场景相似度
	if p1.Scene != "" && p1.Scene == p2.Scene {
		score += 0.3
	}
	factors += 0.3

	// 物体相似度
	commonObjects := 0
	for _, o1 := range p1.Objects {
		for _, o2 := range p2.Objects {
			if o1 == o2 {
				commonObjects++
				break
			}
		}
	}
	if len(p1.Objects) > 0 && len(p2.Objects) > 0 {
		objectScore := float64(commonObjects) / float64(max(len(p1.Objects), len(p2.Objects)))
		score += objectScore * 0.3
	}
	factors += 0.3

	// 颜色相似度
	commonColors := 0
	for _, c1 := range p1.ColorPalette {
		for _, c2 := range p2.ColorPalette {
			if c1 == c2 {
				commonColors++
				break
			}
		}
	}
	if len(p1.ColorPalette) > 0 && len(p2.ColorPalette) > 0 {
		colorScore := float64(commonColors) / float64(max(len(p1.ColorPalette), len(p2.ColorPalette)))
		score += colorScore * 0.2
	}
	factors += 0.2

	// 时间接近度
	if !p1.TakenAt.IsZero() && !p2.TakenAt.IsZero() {
		timeDiff := p1.TakenAt.Sub(p2.TakenAt).Abs()
		if timeDiff < 24*time.Hour {
			score += 0.2
		} else if timeDiff < 7*24*time.Hour {
			score += 0.1
		}
	}
	factors += 0.2

	if factors > 0 {
		return score / factors
	}
	return 0
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// EnableCloudAI 启用云端 AI
func (aim *AIManager) EnableCloudAI(apiKey, endpoint string) {
	aim.cloudEngine.apiKey = apiKey
	aim.cloudEngine.endpoint = endpoint
	aim.cloudEngine.enabled = true
	aim.useCloud = true
}

// DisableCloudAI 禁用云端 AI
func (aim *AIManager) DisableCloudAI() {
	aim.cloudEngine.enabled = false
	aim.useCloud = false
}

// GetAIStats 获取 AI 统计
func (aim *AIManager) GetAIStats() map[string]interface{} {
	aim.memoryMu.RLock()
	defer aim.memoryMu.RUnlock()

	stats := map[string]interface{}{
		"totalAnalyzed": len(aim.memoryCache),
		"totalFaces":    0,
		"totalScenes":   make(map[string]int),
		"totalObjects":  make(map[string]int),
	}

	faceCount := 0
	sceneCount := make(map[string]int)
	objectCount := make(map[string]int)

	for _, memory := range aim.memoryCache {
		faceCount += len(memory.Classification.Faces)
		sceneCount[memory.Classification.Scene]++
		for _, obj := range memory.Classification.Objects {
			objectCount[obj]++
		}
	}

	stats["totalFaces"] = faceCount
	stats["sceneDistribution"] = sceneCount
	stats["objectDistribution"] = objectCount

	return stats
}

// Close 关闭 AI 管理器
func (aim *AIManager) Close() {
	close(aim.stopChan)
	aim.wg.Wait()
	_ = aim.persistAIMemory()
	_ = aim.saveSmartAlbums()
}

// ClearAIData 清除所有 AI 内存数据
func (aim *AIManager) ClearAIData() error {
	aim.memoryMu.Lock()
	defer aim.memoryMu.Unlock()

	// 清空内存缓存
	aim.memoryCache = make(map[string]*AIMemory)

	// 清空照片的 AI 相关信息
	aim.photosManager.mu.Lock()
	for _, photo := range aim.photosManager.photos {
		photo.Faces = nil
		photo.Objects = nil
		photo.Scene = ""
		photo.ColorPalette = nil
	}
	aim.photosManager.mu.Unlock()

	// 删除存储文件
	path := filepath.Join(aim.photosManager.dataDir, "ai-memory.json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除 AI 内存文件失败: %w", err)
	}

	return nil
}

// ClearPhotoAIData 清除单张照片的 AI 数据
func (aim *AIManager) ClearPhotoAIData(photoID string) error {
	aim.memoryMu.Lock()
	defer aim.memoryMu.Unlock()

	// 从缓存中删除
	delete(aim.memoryCache, photoID)

	// 更新照片信息
	aim.photosManager.mu.Lock()
	if photo, exists := aim.photosManager.photos[photoID]; exists {
		photo.Faces = nil
		photo.Objects = nil
		photo.Scene = ""
		photo.ColorPalette = nil
	}
	aim.photosManager.mu.Unlock()

	return nil
}

// ReanalyzeAll 清除现有 AI 数据并重新分析所有照片
func (aim *AIManager) ReanalyzeAll() (int, error) {
	// 清除现有数据
	if err := aim.ClearAIData(); err != nil {
		return 0, err
	}

	// 清除任务结果
	aim.taskMu.Lock()
	aim.taskResults = make(map[string]*AITask)
	aim.taskMu.Unlock()

	// 获取所有照片并重新分析
	aim.photosManager.mu.RLock()
	photos := make([]*Photo, 0, len(aim.photosManager.photos))
	for _, photo := range aim.photosManager.photos {
		photos = append(photos, photo)
	}
	aim.photosManager.mu.RUnlock()

	// 批量分析
	taskIDs := aim.BatchAnalyze(photos)

	return len(taskIDs), nil
}

// SaveAIClassification 保存单张照片的 AI 分析结果
func (aim *AIManager) SaveAIClassification(photoID string, classification *AIClassification) error {
	memory := &AIMemory{
		PhotoID:        photoID,
		Classification: classification,
		ProcessedAt:    time.Now(),
		ModelVersion:   "v1.0",
	}

	aim.saveAIMemory(memory)
	return aim.persistAIMemory()
}

// SaveAllAIClassifications 保存所有 AI 分类结果到存储
func (aim *AIManager) SaveAllAIClassifications() error {
	return aim.persistAIMemory()
}

// ==================== 本地 AI 引擎实现（简化版） ====================

// DetectFaces 人脸检测
func (e *LocalAIEngine) DetectFaces(img image.Image) ([]FaceInfo, error) {
	// 基础实现：使用图像分析检测人脸区域
	// 在生产环境中，应集成 go-face 或 ONNX 模型
	// 这里提供一个简化的实现框架

	bounds := img.Bounds()

	faces := make([]FaceInfo, 0)

	// 简化的人脸检测：基于肤色检测
	// 这是一个基础实现，实际应使用专业的人脸检测库
	for y := bounds.Min.Y; y < bounds.Max.Y-50; y += 20 {
		for x := bounds.Min.X; x < bounds.Max.X-50; x += 20 {
			// 检测肤色区域
			if e.isSkinTone(img, x, y, 50, 50) {
				// 可能是人脸区域
				face := FaceInfo{
					ID:         uuid.New().String(),
					Bounds:     Rectangle{X: x, Y: y, Width: 50, Height: 50},
					Confidence: 0.6, // 基础置信度
				}
				faces = append(faces, face)
			}
		}
	}

	// 去重和合并重叠区域
	faces = e.mergeOverlappingFaces(faces)

	// 限制最大人脸数
	if len(faces) > 10 {
		faces = faces[:10]
	}

	return faces, nil
}

// isSkinTone 检测是否为肤色区域
func (e *LocalAIEngine) isSkinTone(img image.Image, x, y, w, h int) bool {
	skinPixels := 0
	totalPixels := 0

	bounds := img.Bounds()
	for dy := y; dy < y+h && dy < bounds.Max.Y; dy += 4 {
		for dx := x; dx < x+w && dx < bounds.Max.X; dx += 4 {
			r, g, b, _ := img.At(dx, dy).RGBA()
			r8, g8, b8 := r>>8, g>>8, b>>8

			// YCbCr 肤色检测
			yVal := 0.299*float64(r8) + 0.587*float64(g8) + 0.114*float64(b8)
			cb := 128 - 0.168736*float64(r8) - 0.331264*float64(g8) + 0.5*float64(b8)
			cr := 128 + 0.5*float64(r8) - 0.418688*float64(g8) - 0.081312*float64(b8)

			// 肤色范围 (YCbCr)
			if yVal > 80 && yVal < 230 &&
				cb > 77 && cb < 127 &&
				cr > 133 && cr < 173 {
				skinPixels++
			}
			totalPixels++
		}
	}

	// 如果超过40%的像素是肤色，则认为是肤色区域
	return totalPixels > 0 && float64(skinPixels)/float64(totalPixels) > 0.4
}

// mergeOverlappingFaces 合并重叠的人脸区域
func (e *LocalAIEngine) mergeOverlappingFaces(faces []FaceInfo) []FaceInfo {
	if len(faces) <= 1 {
		return faces
	}

	merged := make([]FaceInfo, 0)
	used := make(map[int]bool)

	for i, f1 := range faces {
		if used[i] {
			continue
		}
		for j, f2 := range faces {
			if i >= j || used[j] {
				continue
			}
			// 检查重叠
			if e.rectsOverlap(f1.Bounds, f2.Bounds) {
				// 合并
				conf := f1.Confidence
				if f2.Confidence > conf {
					conf = f2.Confidence
				}
				merged = append(merged, FaceInfo{
					ID:         f1.ID,
					Bounds:     e.mergeRects(f1.Bounds, f2.Bounds),
					Confidence: conf,
				})
				used[i] = true
				used[j] = true
				break
			}
		}
		if !used[i] {
			merged = append(merged, f1)
		}
	}

	return merged
}

// rectsOverlap 检查两个矩形是否重叠
func (e *LocalAIEngine) rectsOverlap(r1, r2 Rectangle) bool {
	return r1.X < r2.X+r2.Width &&
		r1.X+r1.Width > r2.X &&
		r1.Y < r2.Y+r2.Height &&
		r1.Y+r1.Height > r2.Y
}

// mergeRects 合并两个矩形
func (e *LocalAIEngine) mergeRects(r1, r2 Rectangle) Rectangle {
	x := min(r1.X, r2.X)
	y := min(r1.Y, r2.Y)
	w := max(r1.X+r1.Width, r2.X+r2.Width) - x
	h := max(r1.Y+r1.Height, r2.Y+r2.Height) - y
	return Rectangle{X: x, Y: y, Width: w, Height: h}
}

// ClassifyScene 场景分类
func (e *LocalAIEngine) ClassifyScene(img image.Image) (string, float32, error) {
	// 基础实现：基于图像特征分析场景
	// 在生产环境中，应集成 ResNet、MobileNet 等预训练模型

	bounds := img.Bounds()

	// 分析图像特征
	features := e.analyzeImageFeatures(img)

	// 基于特征推断场景
	scene := "unknown"
	confidence := float32(0.5)

	// 室内/室外判断
	if features.brightness > 150 && features.colorVariance < 2000 {
		scene = "indoor"
		confidence = 0.7
	} else if features.skyRatio > 0.3 {
		scene = "outdoor"
		confidence = 0.75
	}

	// 细分场景
	if features.greenRatio > 0.3 {
		scene = "nature"
		confidence = 0.8
	} else if features.blueRatio > 0.4 {
		scene = "sky"
		confidence = 0.75
	} else if features.brightness < 80 {
		scene = "night"
		confidence = 0.65
	}

	// 根据宽高比判断
	if float64(bounds.Dx())/float64(bounds.Dy()) > 1.8 {
		// 宽幅图像，可能是风景
		if scene == "outdoor" || scene == "nature" {
			scene = "landscape"
			confidence = 0.8
		}
	}

	return scene, confidence, nil
}

// imageFeatures 图像特征
type imageFeatures struct {
	brightness     float64
	colorVariance  float64
	greenRatio     float64
	blueRatio      float64
	skyRatio       float64
	warmColorRatio float64
}

// analyzeImageFeatures 分析图像特征
func (e *LocalAIEngine) analyzeImageFeatures(img image.Image) imageFeatures {
	bounds := img.Bounds()
	totalPixels := 0.0
	var rSum, gSum, bSum float64
	var rVar, gVar, bVar float64
	greenCount := 0.0
	blueCount := 0.0
	skyCount := 0.0
	warmCount := 0.0

	// 第一遍：计算均值
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 4 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 4 {
			r, g, b, _ := img.At(x, y).RGBA()
			r8, g8, b8 := float64(r>>8), float64(g>>8), float64(b>>8)

			rSum += r8
			gSum += g8
			bSum += b8
			totalPixels++

			// 统计颜色分布
			if g8 > r8 && g8 > b8 {
				greenCount++
			}
			if b8 > r8 && b8 > g8 {
				blueCount++
			}
			// 天空区域（上半部分，蓝色为主）
			if y < bounds.Max.Y/3 && b8 > r8 && b8 > g8 && b8 > 150 {
				skyCount++
			}
			// 暖色调
			if r8 > g8 && r8 > b8 {
				warmCount++
			}
		}
	}

	if totalPixels == 0 {
		return imageFeatures{}
	}

	// 计算平均亮度
	avgR := rSum / totalPixels
	avgG := gSum / totalPixels
	avgB := bSum / totalPixels
	brightness := (avgR + avgG + avgB) / 3

	// 第二遍：计算方差
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 4 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 4 {
			r, g, b, _ := img.At(x, y).RGBA()
			r8, g8, b8 := float64(r>>8), float64(g>>8), float64(b>>8)

			rVar += (r8 - avgR) * (r8 - avgR)
			gVar += (g8 - avgG) * (g8 - avgG)
			bVar += (b8 - avgB) * (b8 - avgB)
		}
	}

	colorVariance := (rVar + gVar + bVar) / totalPixels

	return imageFeatures{
		brightness:     brightness,
		colorVariance:  colorVariance,
		greenRatio:     greenCount / totalPixels,
		blueRatio:      blueCount / totalPixels,
		skyRatio:       skyCount / totalPixels,
		warmColorRatio: warmCount / totalPixels,
	}
}

// DetectObjects 物体检测
func (e *LocalAIEngine) DetectObjects(img image.Image) ([]string, error) {
	// 基础实现：基于颜色和纹理特征检测物体
	// 在生产环境中，应集成 YOLO、SSD 等目标检测模型

	objects := make([]string, 0)
	features := e.analyzeImageFeatures(img)

	// 基于特征推断物体
	// 植被
	if features.greenRatio > 0.3 {
		objects = append(objects, "vegetation", "plants")
	}

	// 天空
	if features.skyRatio > 0.2 {
		objects = append(objects, "sky")
	}

	// 水体（蓝色区域大，低亮度变化）
	if features.blueRatio > 0.4 && features.colorVariance < 3000 {
		objects = append(objects, "water")
	}

	// 日落/日出（暖色调比例高）
	if features.warmColorRatio > 0.4 && features.brightness > 100 {
		objects = append(objects, "sunset")
	}

	// 室内物品（低亮度变化）
	if features.colorVariance < 2000 && features.brightness > 80 {
		objects = append(objects, "furniture")
	}

	// 去重
	uniqueObjects := make(map[string]bool)
	result := make([]string, 0)
	for _, obj := range objects {
		if !uniqueObjects[obj] {
			uniqueObjects[obj] = true
			result = append(result, obj)
		}
	}

	return result, nil
}

// ExtractColors 提取主色调
func (e *LocalAIEngine) ExtractColors(img image.Image) ([]string, error) {
	// 简单的颜色量化
	bounds := img.Bounds()
	colorCount := make(map[string]int)
	totalPixels := 0

	for y := bounds.Min.Y; y < bounds.Max.Y; y += 4 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 4 {
			r, g, b, _ := img.At(x, y).RGBA()
			if r == 0 && g == 0 && b == 0 {
				continue
			}
			// 简化为 8 位颜色
			color := fmt.Sprintf("#%02X%02X%02X", r>>8, g>>8, b>>8)
			colorCount[color]++
			totalPixels++
		}
	}

	// 返回最多的 5 种颜色
	type colorFreq struct {
		color string
		count int
	}

	colors := make([]colorFreq, 0, len(colorCount))
	for c, n := range colorCount {
		colors = append(colors, colorFreq{c, n})
	}

	sort.Slice(colors, func(i, j int) bool {
		return colors[i].count > colors[j].count
	})

	result := make([]string, 0, 5)
	for i := 0; i < len(colors) && i < 5; i++ {
		result = append(result, colors[i].color)
	}

	return result, nil
}

// QualityScore 计算照片质量评分
// 基于亮度、对比度、清晰度、色彩丰富度、构图等指标综合评分
func (e *LocalAIEngine) QualityScore(img image.Image) (*QualityMetrics, error) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	metrics := &QualityMetrics{}

	// 1. 计算亮度
	var brightnessSum float64
	pixelCount := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 2 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 2 {
			r, g, b, _ := img.At(x, y).RGBA()
			// 转换为 0-255 范围
			r8, g8, b8 := float64(r>>8), float64(g>>8), float64(b>>8)
			// 使用加权平均计算亮度
			luminance := 0.299*r8 + 0.587*g8 + 0.114*b8
			brightnessSum += luminance
			pixelCount++
		}
	}
	metrics.Brightness = brightnessSum / float64(pixelCount)

	// 2. 计算对比度（亮度标准差）
	var contrastSum float64
	avgBrightness := metrics.Brightness
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 2 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 2 {
			r, g, b, _ := img.At(x, y).RGBA()
			r8, g8, b8 := float64(r>>8), float64(g>>8), float64(b>>8)
			luminance := 0.299*r8 + 0.587*g8 + 0.114*b8
			contrastSum += (luminance - avgBrightness) * (luminance - avgBrightness)
		}
	}
	metrics.Contrast = 0
	if pixelCount > 0 {
		metrics.Contrast = contrastSum / float64(pixelCount)
	}

	// 3. 计算清晰度（使用 Laplacian 算子估计）
	sharpnessSum := 0.0
	sharpnessCount := 0
	for y := bounds.Min.Y + 1; y < bounds.Max.Y-1; y += 2 {
		for x := bounds.Min.X + 1; x < bounds.Max.X-1; x += 2 {
			// Laplacian 算子
			r, g, b, _ := img.At(x, y).RGBA()
			rLeft, gLeft, bLeft, _ := img.At(x-1, y).RGBA()
			rRight, gRight, bRight, _ := img.At(x+1, y).RGBA()
			rTop, gTop, bTop, _ := img.At(x, y-1).RGBA()
			rBottom, gBottom, bBottom, _ := img.At(x, y+1).RGBA()

			// 转换为灰度
			gray := 0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(b>>8)
			grayLeft := 0.299*float64(rLeft>>8) + 0.587*float64(gLeft>>8) + 0.114*float64(bLeft>>8)
			grayRight := 0.299*float64(rRight>>8) + 0.587*float64(gRight>>8) + 0.114*float64(bRight>>8)
			grayTop := 0.299*float64(rTop>>8) + 0.587*float64(gTop>>8) + 0.114*float64(bTop>>8)
			grayBottom := 0.299*float64(rBottom>>8) + 0.587*float64(gBottom>>8) + 0.114*float64(bBottom>>8)

			// Laplacian 值
			laplacian := 4*gray - grayLeft - grayRight - grayTop - grayBottom
			sharpnessSum += laplacian * laplacian
			sharpnessCount++
		}
	}
	if sharpnessCount > 0 {
		metrics.Sharpness = sharpnessSum / float64(sharpnessCount)
	}

	// 4. 计算色彩丰富度
	var rgSum, ybSum float64
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 2 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 2 {
			r, g, b, _ := img.At(x, y).RGBA()
			r8, g8, b8 := float64(r>>8), float64(g>>8), float64(b>>8)

			// rg 和 yb 通道
			rg := r8 - g8
			yb := 0.5*(r8+g8) - b8

			rgSum += rg
			ybSum += yb
		}
	}
	pixelCountSample := float64((width/2 + 1) * (height/2 + 1))
	meanRg := rgSum / pixelCountSample
	meanYb := ybSum / pixelCountSample

	var rgVar, ybVar float64
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 2 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 2 {
			r, g, b, _ := img.At(x, y).RGBA()
			r8, g8, b8 := float64(r>>8), float64(g>>8), float64(b>>8)

			rg := r8 - g8
			yb := 0.5*(r8+g8) - b8

			rgVar += (rg - meanRg) * (rg - meanRg)
			ybVar += (yb - meanYb) * (yb - meanYb)
		}
	}

	rgVar /= pixelCountSample
	ybVar /= pixelCountSample

	metrics.Colorfulness = meanRg*meanRg + meanYb*meanYb + (rgVar+ybVar)*0.3

	// 5. 计算构图评分（基于三分法）
	metrics.Composition = e.evaluateComposition(img)

	// 6. 计算综合评分
	metrics.OverallScore = e.calculateOverallScore(metrics)

	return metrics, nil
}

// evaluateComposition 评估构图（基于三分法）
func (e *LocalAIEngine) evaluateComposition(img image.Image) float64 {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// 三分线位置
	thirdX1 := width / 3
	thirdX2 := width * 2 / 3
	thirdY1 := height / 3
	thirdY2 := height * 2 / 3

	// 分析三分线附近的边缘强度
	edgeStrength := 0.0
	sampleCount := 0

	// 垂直三分线
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 4 {
		for _, x := range []int{thirdX1, thirdX2} {
			if x > 0 && x < width-1 {
				r1, g1, b1, _ := img.At(x-1, y).RGBA()
				r2, g2, b2, _ := img.At(x+1, y).RGBA()

				gray1 := 0.299*float64(r1>>8) + 0.587*float64(g1>>8) + 0.114*float64(b1>>8)
				gray2 := 0.299*float64(r2>>8) + 0.587*float64(g2>>8) + 0.114*float64(b2>>8)

				edgeStrength += (gray2 - gray1) * (gray2 - gray1)
				sampleCount++
			}
		}
	}

	// 水平三分线
	for x := bounds.Min.X; x < bounds.Max.X; x += 4 {
		for _, y := range []int{thirdY1, thirdY2} {
			if y > 0 && y < height-1 {
				r1, g1, b1, _ := img.At(x, y-1).RGBA()
				r2, g2, b2, _ := img.At(x, y+1).RGBA()

				gray1 := 0.299*float64(r1>>8) + 0.587*float64(g1>>8) + 0.114*float64(b1>>8)
				gray2 := 0.299*float64(r2>>8) + 0.587*float64(g2>>8) + 0.114*float64(b2>>8)

				edgeStrength += (gray2 - gray1) * (gray2 - gray1)
				sampleCount++
			}
		}
	}

	if sampleCount > 0 {
		return edgeStrength / float64(sampleCount)
	}
	return 0
}

// calculateOverallScore 计算综合质量评分
func (e *LocalAIEngine) calculateOverallScore(m *QualityMetrics) float32 {
	score := 0.0

	// 亮度评分（适中亮度最佳，50-200 为理想范围）
	var brightnessScore float64
	if m.Brightness < 50 {
		brightnessScore = float64(m.Brightness) / 50.0 * 60 // 太暗
	} else if m.Brightness > 200 {
		brightnessScore = (255.0-float64(m.Brightness))/55.0*60 + 40 // 太亮
	} else {
		brightnessScore = 80 + (100-math.Abs(m.Brightness-125))/125.0*20
	}

	// 对比度评分（对比度适中为佳）
	contrastScore := math.Min(m.Contrast/100.0*50, 100)

	// 清晰度评分
	sharpnessScore := math.Min(m.Sharpness/1000.0*60, 100)

	// 色彩丰富度评分
	colorfulnessScore := math.Min(m.Colorfulness/50.0*70, 100)

	// 构图评分
	compositionScore := math.Min(m.Composition/100.0*60, 100)

	// 加权平均
	score = brightnessScore*0.2 + contrastScore*0.2 + sharpnessScore*0.25 + colorfulnessScore*0.2 + compositionScore*0.15

	return float32(math.Min(math.Max(score, 0), 100))
}

// GenerateTags 根据 AI 分析结果自动生成标签
func (e *LocalAIEngine) GenerateTags(result *AIClassification) []string {
	tags := make([]string, 0)
	tagSet := make(map[string]bool) // 用于去重

	// 辅助函数：添加标签
	addTag := func(tag string) {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag != "" && !tagSet[tag] && len(tags) < 20 {
			tagSet[tag] = true
			tags = append(tags, tag)
		}
	}

	// 1. 基于场景生成标签
	sceneTags := map[string][]string{
		"indoor":    {"室内", "indoor"},
		"outdoor":   {"室外", "户外", "outdoor"},
		"nature":    {"自然", "风景", "nature", "landscape"},
		"landscape": {"风景", "自然风光", "landscape", "scenery"},
		"sky":       {"天空", "sky"},
		"night":     {"夜景", "夜晚", "night", "night-scene"},
		"sunset":    {"日落", "黄昏", "sunset", "dusk"},
		"portrait":  {"人像", "portrait"},
		"food":      {"美食", "食物", "food"},
		"beach":     {"海滩", "海边", "beach"},
		"mountain":  {"山脉", "山", "mountain"},
		"city":      {"城市", "都市", "city", "urban"},
	}
	if result.Scene != "" {
		if mappedTags, ok := sceneTags[strings.ToLower(result.Scene)]; ok {
			for _, tag := range mappedTags {
				addTag(tag)
			}
		} else {
			addTag(result.Scene)
		}
	}

	// 2. 基于物体生成标签
	for _, obj := range result.Objects {
		// 物体标签映射
		objectTags := map[string][]string{
			"vegetation": {"植物", "绿色", "plants", "green"},
			"plants":     {"植物", "plants"},
			"sky":        {"天空", "sky"},
			"water":      {"水", "水域", "water"},
			"furniture":  {"家具", "室内", "furniture"},
		}
		if mappedTags, ok := objectTags[strings.ToLower(obj)]; ok {
			for _, tag := range mappedTags {
				addTag(tag)
			}
		} else {
			addTag(obj)
		}
	}

	// 3. 基于人脸生成标签
	if len(result.Faces) > 0 {
		addTag("人物")
		addTag("people")
		if len(result.Faces) > 1 {
			addTag("合影")
			addTag("group")
		}
		// 添加已识别的人物名称
		for _, face := range result.Faces {
			if face.Name != "" {
				addTag(face.Name)
			}
		}
	}

	// 4. 基于颜色生成标签
	colorTags := map[string]string{
		"#FF0000": "红色",
		"#00FF00": "绿色",
		"#0000FF": "蓝色",
		"#FFFF00": "黄色",
		"#FF00FF": "紫色",
		"#00FFFF": "青色",
		"#FFFFFF": "白色",
		"#000000": "黑色",
		"#FFA500": "橙色",
		"#FFC0CB": "粉色",
	}
	for _, color := range result.Colors {
		// 检查颜色是否在映射中
		if tagName, ok := colorTags[strings.ToUpper(color)]; ok {
			addTag(tagName)
		} else {
			// 尝试解析颜色并生成标签
			if len(color) >= 7 {
				r, _ := strconv.ParseInt(color[1:3], 16, 64)
				g, _ := strconv.ParseInt(color[3:5], 16, 64)
				b, _ := strconv.ParseInt(color[5:7], 16, 64)

				// 根据颜色值判断主要色调
				if r > 200 && g < 100 && b < 100 {
					addTag("暖色调")
				} else if r < 100 && g < 100 && b > 200 {
					addTag("冷色调")
				} else if r > 200 && g > 200 && b < 100 {
					addTag("明亮")
				}
			}
		}
	}

	// 5. 基于质量评分生成标签
	if result.QualityScore >= 80 {
		addTag("高质量")
		addTag("优质照片")
	} else if result.QualityScore >= 60 {
		addTag("良好")
	}

	// 6. 如果是 NSFW，添加标签（隐藏）
	if result.IsNSFW {
		addTag("敏感内容")
	}

	return tags
}

// ==================== 云端 AI 引擎实现 ====================

// DetectFaces 云端人脸检测
func (e *CloudAIEngine) DetectFaces(img image.Image) ([]FaceInfo, error) {
	if !e.enabled || e.apiKey == "" {
		return nil, fmt.Errorf("云端 AI 未配置")
	}

	// 优先尝试 Azure Face API
	if e.endpoint != "" && strings.Contains(e.endpoint, "azure") {
		return e.detectFacesAzure(img)
	}

	// 否则尝试 AWS Rekognition
	if strings.Contains(e.apiKey, "AKIA") || e.endpoint == "" {
		return e.detectFacesAWS(img)
	}

	return nil, fmt.Errorf("云端 AI 未配置")
}

// detectFacesAzure 使用 Azure Face API 检测人脸
func (e *CloudAIEngine) detectFacesAzure(img image.Image) ([]FaceInfo, error) {
	// 将图片编码为 JPEG
	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 90}); err != nil {
		return nil, fmt.Errorf("编码图片失败: %w", err)
	}

	// 构建请求 URL
	apiURL := e.endpoint
	if !strings.HasSuffix(apiURL, "/") {
		apiURL += "/"
	}
	apiURL += "face/v1.0/detect?returnFaceId=true&returnFaceAttributes=age,gender,emotion"

	// 创建 HTTP 请求
	req, err := http.NewRequest("POST", apiURL, buf)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Ocp-Apim-Subscription-Key", e.apiKey)

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API 返回错误: %s - %s", resp.Status, string(body))
	}

	// 解析响应
	var azureFaces []struct {
		FaceID        string `json:"faceId"`
		FaceRectangle struct {
			Top    int `json:"top"`
			Left   int `json:"left"`
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"faceRectangle"`
		FaceAttributes struct {
			Age     float64 `json:"age"`
			Gender  string  `json:"gender"`
			Emotion struct {
				Anger     float64 `json:"anger"`
				Contempt  float64 `json:"contempt"`
				Disgust   float64 `json:"disgust"`
				Fear      float64 `json:"fear"`
				Happiness float64 `json:"happiness"`
				Neutral   float64 `json:"neutral"`
				Sadness   float64 `json:"sadness"`
				Surprise  float64 `json:"surprise"`
			} `json:"emotion"`
		} `json:"faceAttributes"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&azureFaces); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 转换为 FaceInfo
	faces := make([]FaceInfo, 0, len(azureFaces))
	for _, af := range azureFaces {
		// 找到主要表情
		emotions := map[string]float64{
			"anger":     af.FaceAttributes.Emotion.Anger,
			"contempt":  af.FaceAttributes.Emotion.Contempt,
			"disgust":   af.FaceAttributes.Emotion.Disgust,
			"fear":      af.FaceAttributes.Emotion.Fear,
			"happiness": af.FaceAttributes.Emotion.Happiness,
			"neutral":   af.FaceAttributes.Emotion.Neutral,
			"sadness":   af.FaceAttributes.Emotion.Sadness,
			"surprise":  af.FaceAttributes.Emotion.Surprise,
		}
		var mainEmotion string
		var maxScore float64
		for emotion, score := range emotions {
			if score > maxScore {
				maxScore = score
				mainEmotion = emotion
			}
		}

		face := FaceInfo{
			ID:         af.FaceID,
			Bounds:     Rectangle{X: af.FaceRectangle.Left, Y: af.FaceRectangle.Top, Width: af.FaceRectangle.Width, Height: af.FaceRectangle.Height},
			Age:        int(af.FaceAttributes.Age),
			Gender:     af.FaceAttributes.Gender,
			Emotion:    mainEmotion,
			Confidence: 1.0,
		}
		faces = append(faces, face)
	}

	return faces, nil
}

// detectFacesAWS 使用 AWS Rekognition 检测人脸
func (e *CloudAIEngine) detectFacesAWS(img image.Image) ([]FaceInfo, error) {
	// 将图片编码为 JPEG
	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 90}); err != nil {
		return nil, fmt.Errorf("编码图片失败: %w", err)
	}

	// AWS Rekognition 需要 AWS SDK，这里提供基本的 HTTP 调用实现
	// 实际使用时建议使用 AWS SDK for Go
	// 这里返回一个简化的实现
	return nil, fmt.Errorf("AWS Rekognition 需要配置 AWS SDK，请设置 AWS_ACCESS_KEY_ID 和 AWS_SECRET_ACCESS_KEY 环境变量")
}

// ClassifyScene 云端场景分类
func (e *CloudAIEngine) ClassifyScene(img image.Image) (string, float32, error) {
	if !e.enabled || e.apiKey == "" {
		return "", 0, fmt.Errorf("云端 AI 未配置")
	}

	// 尝试 Azure Computer Vision
	if e.endpoint != "" && strings.Contains(e.endpoint, "azure") {
		return e.classifySceneAzure(img)
	}

	// 尝试 AWS Rekognition
	return e.classifySceneAWS(img)
}

// classifySceneAzure 使用 Azure Computer Vision 分类场景
func (e *CloudAIEngine) classifySceneAzure(img image.Image) (string, float32, error) {
	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 90}); err != nil {
		return "", 0, fmt.Errorf("编码图片失败: %w", err)
	}

	apiURL := e.endpoint
	if !strings.HasSuffix(apiURL, "/") {
		apiURL += "/"
	}
	apiURL += "vision/v3.2/analyze?visualFeatures=Categories,Tags"

	req, err := http.NewRequest("POST", apiURL, buf)
	if err != nil {
		return "", 0, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Ocp-Apim-Subscription-Key", e.apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("API 返回错误: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Categories []struct {
			Name  string  `json:"name"`
			Score float64 `json:"score"`
		} `json:"categories"`
		Tags []struct {
			Name       string  `json:"name"`
			Confidence float64 `json:"confidence"`
		} `json:"tags"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", 0, fmt.Errorf("解析响应失败: %w", err)
	}

	// 返回最高置信度的类别
	if len(result.Categories) > 0 {
		return result.Categories[0].Name, float32(result.Categories[0].Score), nil
	}

	// 如果没有类别，返回最高置信度的标签
	if len(result.Tags) > 0 {
		return result.Tags[0].Name, float32(result.Tags[0].Confidence), nil
	}

	return "unknown", 0, nil
}

// classifySceneAWS 使用 AWS Rekognition 分类场景
func (e *CloudAIEngine) classifySceneAWS(img image.Image) (string, float32, error) {
	return "", 0, fmt.Errorf("AWS Rekognition 需要配置 AWS SDK")
}

// DetectObjects 云端物体检测
func (e *CloudAIEngine) DetectObjects(img image.Image) ([]string, error) {
	if !e.enabled || e.apiKey == "" {
		return nil, fmt.Errorf("云端 AI 未配置")
	}

	// 尝试 Azure
	if e.endpoint != "" && strings.Contains(e.endpoint, "azure") {
		return e.detectObjectsAzure(img)
	}

	// 尝试 AWS
	return e.detectObjectsAWS(img)
}

// detectObjectsAzure 使用 Azure 检测物体
func (e *CloudAIEngine) detectObjectsAzure(img image.Image) ([]string, error) {
	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 90}); err != nil {
		return nil, fmt.Errorf("编码图片失败: %w", err)
	}

	apiURL := e.endpoint
	if !strings.HasSuffix(apiURL, "/") {
		apiURL += "/"
	}
	apiURL += "vision/v3.2/analyze?visualFeatures=Objects"

	req, err := http.NewRequest("POST", apiURL, buf)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Ocp-Apim-Subscription-Key", e.apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API 返回错误: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Objects []struct {
			Object string `json:"object"`
		} `json:"objects"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	objects := make([]string, 0, len(result.Objects))
	for _, obj := range result.Objects {
		objects = append(objects, obj.Object)
	}

	return objects, nil
}

// detectObjectsAWS 使用 AWS Rekognition 检测物体
func (e *CloudAIEngine) detectObjectsAWS(img image.Image) ([]string, error) {
	return nil, fmt.Errorf("AWS Rekognition 需要配置 AWS SDK")
}

// ExtractColors 云端颜色提取
func (e *CloudAIEngine) ExtractColors(img image.Image) ([]string, error) {
	if !e.enabled || e.apiKey == "" {
		return nil, fmt.Errorf("云端 AI 未配置")
	}

	// 尝试 Azure
	if e.endpoint != "" && strings.Contains(e.endpoint, "azure") {
		return e.extractColorsAzure(img)
	}

	// 默认使用本地提取
	return nil, fmt.Errorf("云端 AI 未配置颜色提取功能")
}

// extractColorsAzure 使用 Azure 提取颜色
func (e *CloudAIEngine) extractColorsAzure(img image.Image) ([]string, error) {
	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 90}); err != nil {
		return nil, fmt.Errorf("编码图片失败: %w", err)
	}

	apiURL := e.endpoint
	if !strings.HasSuffix(apiURL, "/") {
		apiURL += "/"
	}
	apiURL += "vision/v3.2/analyze?visualFeatures=Color"

	req, err := http.NewRequest("POST", apiURL, buf)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Ocp-Apim-Subscription-Key", e.apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API 返回错误: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Color struct {
			DominantColorsForeground string `json:"dominantColorForeground"`
			DominantColorsBackground string `json:"dominantColorBackground"`
			DominantColors           []struct {
				Color string `json:"color"`
			} `json:"dominantColors"`
		} `json:"color"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	colors := make([]string, 0)
	colors = append(colors, result.Color.DominantColorsForeground)
	colors = append(colors, result.Color.DominantColorsBackground)
	for _, c := range result.Color.DominantColors {
		colors = append(colors, c.Color)
	}

	// 去重
	uniqueColors := make(map[string]bool)
	result_colors := make([]string, 0)
	for _, c := range colors {
		if c != "" && !uniqueColors[c] {
			uniqueColors[c] = true
			result_colors = append(result_colors, c)
		}
	}

	return result_colors, nil
}

// QualityScore 云端暂不支持质量评分，返回未实现错误
func (e *CloudAIEngine) QualityScore(img image.Image) (*QualityMetrics, error) {
	return nil, fmt.Errorf("云端 AI 暂不支持质量评分功能")
}

// GenerateTags 云端暂不支持自动标签生成
func (e *CloudAIEngine) GenerateTags(result *AIClassification) []string {
	return nil
}
