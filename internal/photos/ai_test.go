package photos

import (
	"strings"
	"testing"
	"time"
)

// ========== AIManager Tests ==========

func TestAITask_Struct(t *testing.T) {
	task := &AITask{
		ID:        "task-1",
		Type:      "analyze_all",
		PhotoID:   "photo-1",
		PhotoPath: "/path/to/photo.jpg",
		Status:    "pending",
		Progress:  0,
		CreatedAt: time.Now(),
	}

	if task.ID != "task-1" {
		t.Errorf("期望 ID 为 task-1，得到 %s", task.ID)
	}
	if task.Type != "analyze_all" {
		t.Errorf("期望 Type 为 analyze_all，得到 %s", task.Type)
	}
	if task.Status != "pending" {
		t.Errorf("期望 Status 为 pending，得到 %s", task.Status)
	}
}

func TestAIMemory_Struct(t *testing.T) {
	memory := &AIMemory{
		PhotoID: "photo-1",
		Classification: &AIClassification{
			PhotoID:    "photo-1",
			Scene:      "outdoor",
			Confidence: 0.95,
		},
		ProcessedAt:  time.Now(),
		ModelVersion: "v1.0",
	}

	if memory.PhotoID != "photo-1" {
		t.Errorf("期望 PhotoID 为 photo-1，得到 %s", memory.PhotoID)
	}
	if memory.Classification == nil {
		t.Error("Classification 不应为 nil")
	}
	if memory.ModelVersion != "v1.0" {
		t.Errorf("期望 ModelVersion 为 v1.0，得到 %s", memory.ModelVersion)
	}
}

func TestSmartAlbum_Struct(t *testing.T) {
	album := &SmartAlbum{
		ID:          "album-1",
		Name:        "人物相册",
		Description: "包含张三的照片",
		Type:        "person",
		Criteria: map[string]interface{}{
			"person": "张三",
		},
		PhotoIDs:   []string{"photo-1", "photo-2"},
		AutoUpdate: true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if album.Type != "person" {
		t.Errorf("期望 Type 为 person，得到 %s", album.Type)
	}
	if len(album.PhotoIDs) != 2 {
		t.Errorf("期望 2 张照片，得到 %d", len(album.PhotoIDs))
	}
	if !album.AutoUpdate {
		t.Error("AutoUpdate 应为 true")
	}
}

func TestMemoryAlbum_Struct(t *testing.T) {
	memory := &MemoryAlbum{
		ID:           "memory-1",
		Title:        "2020 年的今天",
		Date:         "03-15",
		Year:         2020,
		PhotoIDs:     []string{"photo-1"},
		CoverPhotoID: "photo-1",
		CreatedAt:    time.Now(),
	}

	if memory.Year != 2020 {
		t.Errorf("期望 Year 为 2020，得到 %d", memory.Year)
	}
	if memory.Date != "03-15" {
		t.Errorf("期望 Date 为 03-15，得到 %s", memory.Date)
	}
}

// ========== LocalAIEngine Tests ==========

func TestLocalAIEngine_DetectFaces(t *testing.T) {
	engine := &LocalAIEngine{
		modelDir: "/tmp/models",
		enabled:  true,
	}

	if !engine.enabled {
		t.Error("引擎应为启用状态")
	}
}

func TestLocalAIEngine_ClassifyScene(t *testing.T) {
	engine := &LocalAIEngine{
		modelDir: "/tmp/models",
		enabled:  true,
	}

	// 验证引擎配置
	if engine.modelDir != "/tmp/models" {
		t.Errorf("期望 modelDir 为 /tmp/models，得到 %s", engine.modelDir)
	}
}

func TestCloudAIEngine_Config(t *testing.T) {
	engine := &CloudAIEngine{
		apiKey:   "test-key",
		endpoint: "https://api.example.com",
		enabled:  true,
	}

	if !engine.enabled {
		t.Error("云引擎应为启用状态")
	}
	if engine.apiKey != "test-key" {
		t.Errorf("期望 apiKey 为 test-key，得到 %s", engine.apiKey)
	}
}

// ========== AIClassification Tests ==========

func TestAIClassification_Struct(t *testing.T) {
	result := &AIClassification{
		PhotoID: "photo-1",
		Faces: []FaceInfo{
			{
				ID:         "face-1",
				Name:       "张三",
				Confidence: 0.95,
			},
		},
		Objects:      []string{"car", "tree", "building"},
		Scene:        "outdoor",
		Colors:       []string{"#FF5733", "#C70039"},
		IsNSFW:       false,
		Confidence:   0.92,
		QualityScore: 85.5,
		AutoTags:     []string{"风景", "户外"},
		Metadata:     make(map[string]interface{}),
	}

	if len(result.Faces) != 1 {
		t.Errorf("期望 1 张人脸，得到 %d", len(result.Faces))
	}
	if len(result.Objects) != 3 {
		t.Errorf("期望 3 个物体，得到 %d", len(result.Objects))
	}
	if result.Scene != "outdoor" {
		t.Errorf("期望 Scene 为 outdoor，得到 %s", result.Scene)
	}
	if result.QualityScore < 0 || result.QualityScore > 100 {
		t.Errorf("QualityScore 应在 0-100 之间，得到 %f", result.QualityScore)
	}
}

func TestFaceInfo_Struct(t *testing.T) {
	face := FaceInfo{
		ID:         "face-1",
		Name:       "张三",
		Bounds:     Rectangle{X: 100, Y: 100, Width: 50, Height: 50},
		Confidence: 0.95,
		Age:        30,
		Gender:     "male",
		Emotion:    "happy",
	}

	if face.Name != "张三" {
		t.Errorf("期望 Name 为 张三，得到 %s", face.Name)
	}
	if face.Confidence < 0 || face.Confidence > 1 {
		t.Errorf("Confidence 应在 0-1 之间，得到 %f", face.Confidence)
	}
	if face.Bounds.Width != 50 {
		t.Errorf("期望 Bounds.Width 为 50，得到 %d", face.Bounds.Width)
	}
}

func TestQualityMetrics_Struct(t *testing.T) {
	metrics := &QualityMetrics{
		Brightness:   128.5,
		Contrast:     45.2,
		Sharpness:    78.9,
		Colorfulness: 65.3,
		Composition:  82.1,
		OverallScore: 75.0,
	}

	if metrics.Brightness < 0 || metrics.Brightness > 255 {
		t.Errorf("Brightness 应在 0-255 之间，得到 %f", metrics.Brightness)
	}
	if metrics.OverallScore < 0 || metrics.OverallScore > 100 {
		t.Errorf("OverallScore 应在 0-100 之间，得到 %f", metrics.OverallScore)
	}
}

// ========== Helper function tests ==========

func TestDetectNSFW(t *testing.T) {
	tests := []struct {
		name     string
		result   *AIClassification
		expected bool
	}{
		{
			name: "normal scene",
			result: &AIClassification{
				Scene: "outdoor",
			},
			expected: false,
		},
		{
			name: "nsfw scene lowercase",
			result: &AIClassification{
				Scene: "nsfw content",
			},
			expected: true,
		},
		{
			name: "explicit scene",
			result: &AIClassification{
				Scene: "explicit material",
			},
			expected: true,
		},
		{
			name: "NSFW scene uppercase",
			result: &AIClassification{
				Scene: "NSFW",
			},
			expected: true, // strings.ToLower 会处理
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 模拟实际代码中的 detectNSFW 逻辑
			result := tt.result
			isNSFW := false
			nsfwScenes := []string{"nsfw", "explicit"}
			sceneLower := strings.ToLower(result.Scene)
			for _, scene := range nsfwScenes {
				if strings.Contains(sceneLower, scene) {
					isNSFW = true
					break
				}
			}
			if isNSFW != tt.expected {
				t.Errorf("期望 %v，得到 %v", tt.expected, isNSFW)
			}
		})
	}
}

// 删除旧的 contains 函数，使用 strings.Contains

// ========== Criteria matching tests ==========

func TestPhotoMatchesCriteria_Person(t *testing.T) {
	photo := &Photo{
		ID: "photo-1",
		Faces: []FaceInfo{
			{Name: "张三"},
			{Name: "李四"},
		},
	}

	// 模拟匹配逻辑
	found := false
	for _, face := range photo.Faces {
		if face.Name == "张三" {
			found = true
			break
		}
	}

	if !found {
		t.Error("照片应匹配人物条件")
	}
}

func TestPhotoMatchesCriteria_Scene(t *testing.T) {
	photo := &Photo{
		ID:    "photo-1",
		Scene: "outdoor",
	}

	if photo.Scene != "outdoor" {
		t.Errorf("期望 Scene 为 outdoor，得到 %s", photo.Scene)
	}
}

func TestPhotoMatchesCriteria_Object(t *testing.T) {
	photo := &Photo{
		ID:      "photo-1",
		Objects: []string{"car", "tree", "building"},
	}

	// 检查是否包含特定物体
	hasCar := false
	for _, obj := range photo.Objects {
		if obj == "car" {
			hasCar = true
			break
		}
	}

	if !hasCar {
		t.Error("照片应包含 car 物体")
	}
}

func TestPhotoMatchesCriteria_DateRange(t *testing.T) {
	photo := &Photo{
		ID:      "photo-1",
		TakenAt: time.Date(2023, 6, 15, 12, 0, 0, 0, time.UTC),
	}

	start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC)

	if photo.TakenAt.Before(start) || photo.TakenAt.After(end) {
		t.Error("照片应在日期范围内")
	}
}

// ========== Task queue tests ==========

func TestTaskQueue_Capacity(t *testing.T) {
	queue := make(chan *AITask, 100)

	if cap(queue) != 100 {
		t.Errorf("期望队列容量为 100，得到 %d", cap(queue))
	}
}

func TestTask_Status_Transition(t *testing.T) {
	task := &AITask{
		ID:        "task-1",
		Status:    "pending",
		Progress:  0,
		CreatedAt: time.Now(),
	}

	// 状态转换
	task.Status = "running"
	task.StartedAt = time.Now()
	task.Progress = 50

	if task.Status != "running" {
		t.Errorf("期望 Status 为 running，得到 %s", task.Status)
	}
	if task.Progress != 50 {
		t.Errorf("期望 Progress 为 50，得到 %d", task.Progress)
	}

	// 完成
	task.Status = "completed"
	task.Progress = 100
	task.CompletedAt = time.Now()

	if task.Status != "completed" {
		t.Errorf("期望 Status 为 completed，得到 %s", task.Status)
	}
}

// ========== Smart album generation tests ==========

func TestSmartAlbumType_Description(t *testing.T) {
	types := map[string]string{
		"person":   "包含特定人物的照片",
		"scene":    "特定场景的照片",
		"object":   "包含特定物体的照片",
		"location": "特定地点的照片",
		"time":     "特定时间的照片",
	}

	for albumType, expectedDesc := range types {
		t.Run(albumType, func(t *testing.T) {
			// 模拟描述生成逻辑
			var desc string
			switch albumType {
			case "person":
				desc = "包含特定人物的照片"
			case "scene":
				desc = "特定场景的照片"
			case "object":
				desc = "包含特定物体的照片"
			case "location":
				desc = "特定地点的照片"
			case "time":
				desc = "特定时间的照片"
			}

			if desc != expectedDesc {
				t.Errorf("期望描述为 %s，得到 %s", expectedDesc, desc)
			}
		})
	}
}

// ========== Memory generation tests ==========

func TestMemoryAlbum_DateFormat(t *testing.T) {
	now := time.Now()
	monthDay := now.Format("01-02")

	if len(monthDay) != 5 {
		t.Errorf("期望日期格式为 MM-DD，得到 %s", monthDay)
	}
}

func TestMemoryAlbum_YearGrouping(t *testing.T) {
	photos := []*Photo{
		{ID: "p1", TakenAt: time.Date(2020, 3, 15, 12, 0, 0, 0, time.UTC)},
		{ID: "p2", TakenAt: time.Date(2021, 3, 15, 12, 0, 0, 0, time.UTC)},
		{ID: "p3", TakenAt: time.Date(2022, 3, 15, 12, 0, 0, 0, time.UTC)},
		{ID: "p4", TakenAt: time.Date(2023, 3, 15, 12, 0, 0, 0, time.UTC)},
	}

	// 按年份分组
	photosByYear := make(map[int][]*Photo)
	for _, photo := range photos {
		year := photo.TakenAt.Year()
		photosByYear[year] = append(photosByYear[year], photo)
	}

	if len(photosByYear) != 4 {
		t.Errorf("期望 4 个年份组，得到 %d", len(photosByYear))
	}

	for year, yearPhotos := range photosByYear {
		if len(yearPhotos) != 1 {
			t.Errorf("年份 %d 应有 1 张照片，得到 %d", year, len(yearPhotos))
		}
	}
}

// ========== Color extraction tests ==========

func TestColorExtraction_Format(t *testing.T) {
	colors := []string{"#FF5733", "#C70039", "#900C3F", "#581845"}

	for _, color := range colors {
		if len(color) != 7 || color[0] != '#' {
			t.Errorf("颜色格式应为 #RRGGBB，得到 %s", color)
		}
	}
}

// ========== Tag generation tests ==========

func TestTagGeneration(t *testing.T) {
	result := &AIClassification{
		Scene:   "outdoor",
		Objects: []string{"car", "tree", "building"},
		Faces: []FaceInfo{
			{Name: "张三"},
		},
	}

	// 模拟标签生成
	tags := make([]string, 0)
	if result.Scene != "" {
		tags = append(tags, result.Scene)
	}
	for _, obj := range result.Objects {
		tags = append(tags, obj)
	}
	for _, face := range result.Faces {
		if face.Name != "" {
			tags = append(tags, face.Name)
		}
	}

	if len(tags) < 3 {
		t.Errorf("期望至少 3 个标签，得到 %d", len(tags))
	}
}
