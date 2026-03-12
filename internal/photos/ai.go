// Package photos AI 相册功能模块
// 提供人脸识别、场景分类、物体检测、智能相册等功能
package photos

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sort"
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
	ID         string    `json:"id"`
	Type       string    `json:"type"` // face_detect, scene_classify, object_detect, analyze_all
	PhotoID    string    `json:"photoId"`
	PhotoPath  string    `json:"photoPath"`
	Status     string    `json:"status"` // pending, running, completed, failed
	Progress   int       `json:"progress"`
	Result     *AIClassification `json:"result,omitempty"`
	Error      string    `json:"error,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	StartedAt  time.Time `json:"startedAt,omitempty"`
	CompletedAt time.Time `json:"completedAt,omitempty"`
}

// AIMemory AI 处理记录（用于持久化）
type AIMemory struct {
	PhotoID       string    `json:"photoId"`
	Classification *AIClassification `json:"classification"`
	ProcessedAt   time.Time `json:"processedAt"`
	ModelVersion  string    `json:"modelVersion"`
}

// SmartAlbum 智能相册（自动生成）
type SmartAlbum struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Type        string    `json:"type"` // person, scene, object, location, time
	Criteria    map[string]interface{} `json:"criteria"` // 生成条件
	PhotoIDs    []string  `json:"photoIds"`
	AutoUpdate  bool      `json:"autoUpdate"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// MemoryAlbum 回忆相册（历史上的今天）
type MemoryAlbum struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Date        string    `json:"date"` // MM-DD
	Year        int       `json:"year"`
	PhotoIDs    []string  `json:"photoIds"`
	CoverPhotoID string `json:"coverPhotoId"`
	CreatedAt   time.Time `json:"createdAt"`
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
		task.Progress = 25
		faces, err := aim.detectFaces(img)
		if err != nil {
			// 人脸检测失败不影响其他分析
			result.Metadata["face_error"] = err.Error()
		} else {
			result.Faces = faces
		}

		// 2. 场景分类
		task.Progress = 50
		scene, confidence, err := aim.classifyScene(img)
		if err != nil {
			result.Metadata["scene_error"] = err.Error()
		} else {
			result.Scene = scene
			result.Confidence = confidence
		}

		// 3. 物体检测
		task.Progress = 75
		objects, err := aim.detectObjects(img)
		if err != nil {
			result.Metadata["object_error"] = err.Error()
		} else {
			result.Objects = objects
		}

		// 4. 颜色提取
		task.Progress = 90
		colors, err := aim.extractColors(img)
		if err != nil {
			result.Metadata["color_error"] = err.Error()
		} else {
			result.Colors = colors
		}

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
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		// 尝试使用 ffmpeg 转换
		return aim.loadImageFFmpeg(path)
	}

	return img, nil
}

// loadImageFFmpeg 使用 ffmpeg 加载图片
func (aim *AIManager) loadImageFFmpeg(path string) (image.Image, error) {
	// TODO: 使用 ffmpeg 转换为 JPEG 后加载
	return nil, fmt.Errorf("暂不支持该格式")
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
		ID:          uuid.New().String(),
		Name:        name,
		Type:        albumType,
		Criteria:    criteria,
		PhotoIDs:    make([]string, 0),
		AutoUpdate:  true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
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
			// TODO: 实现日期范围匹配
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

		// TODO: 保存到存储
		_ = memory
	}
}

// GetMemories 获取回忆列表
func (aim *AIManager) GetMemories(monthDay string) []*MemoryAlbum {
	// TODO: 实现回忆查询
	return make([]*MemoryAlbum, 0)
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
	aim.persistAIMemory()
	aim.saveSmartAlbums()
}

// ==================== 本地 AI 引擎实现（简化版） ====================

// DetectFaces 人脸检测（简化实现）
func (e *LocalAIEngine) DetectFaces(img image.Image) ([]FaceInfo, error) {
	// TODO: 集成 go-face 或 ONNX 模型
	// 这里返回空结果，实际使用需要集成真实模型
	return []FaceInfo{}, nil
}

// ClassifyScene 场景分类（简化实现）
func (e *LocalAIEngine) ClassifyScene(img image.Image) (string, float32, error) {
	// TODO: 集成预训练模型（如 ResNet、MobileNet）
	// 这里返回默认结果
	return "unknown", 0.5, nil
}

// DetectObjects 物体检测（简化实现）
func (e *LocalAIEngine) DetectObjects(img image.Image) ([]string, error) {
	// TODO: 集成 YOLO 或 SSD 模型
	return []string{}, nil
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

// ==================== 云端 AI 引擎实现（占位） ====================

// DetectFaces 云端人脸检测
func (e *CloudAIEngine) DetectFaces(img image.Image) ([]FaceInfo, error) {
	// TODO: 调用云端 API（如 Azure Face API、AWS Rekognition）
	return []FaceInfo{}, fmt.Errorf("云端 AI 未配置")
}

// ClassifyScene 云端场景分类
func (e *CloudAIEngine) ClassifyScene(img image.Image) (string, float32, error) {
	// TODO: 调用云端 API
	return "", 0, fmt.Errorf("云端 AI 未配置")
}

// DetectObjects 云端物体检测
func (e *CloudAIEngine) DetectObjects(img image.Image) ([]string, error) {
	// TODO: 调用云端 API
	return []string{}, fmt.Errorf("云端 AI 未配置")
}

// ExtractColors 云端颜色提取
func (e *CloudAIEngine) ExtractColors(img image.Image) ([]string, error) {
	// TODO: 调用云端 API
	return []string{}, fmt.Errorf("云端 AI 未配置")
}
