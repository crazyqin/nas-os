package airecommend

import (
	"sync"
	"testing"
	"time"
)

func TestNewEngine(t *testing.T) {
	config := DefaultConfig()
	engine := NewEngine(config)

	if engine == nil {
		t.Fatal("expected engine to be created")
	}

	if engine.config == nil {
		t.Fatal("expected config to be set")
	}

	if engine.users == nil {
		t.Fatal("expected users map to be initialized")
	}

	if engine.files == nil {
		t.Fatal("expected files map to be initialized")
	}

	if engine.cache == nil {
		t.Fatal("expected cache map to be initialized")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.CacheTTL != 30*time.Minute {
		t.Errorf("expected CacheTTL 30m, got %v", config.CacheTTL)
	}

	if config.MaxResults != 20 {
		t.Errorf("expected MaxResults 20, got %d", config.MaxResults)
	}

	if config.Weights.TimeDecay != 0.3 {
		t.Errorf("expected TimeDecay weight 0.3, got %f", config.Weights.TimeDecay)
	}

	if config.Weights.Frequency != 0.3 {
		t.Errorf("expected Frequency weight 0.3, got %f", config.Weights.Frequency)
	}

	if config.Weights.Collaborative != 0.2 {
		t.Errorf("expected Collaborative weight 0.2, got %f", config.Weights.Collaborative)
	}

	if config.Weights.Content != 0.2 {
		t.Errorf("expected Content weight 0.2, got %f", config.Weights.Content)
	}
}

func TestAddUser(t *testing.T) {
	engine := NewEngine(nil)
	engine.AddUser("user1")

	profile := engine.GetUserProfile("user1")
	if profile == nil {
		t.Fatal("expected user to be created")
	}

	if profile.UserID != "user1" {
		t.Errorf("expected UserID user1, got %s", profile.UserID)
	}

	if profile.Preferences == nil {
		t.Fatal("expected Preferences to be initialized")
	}
}

func TestAddFile(t *testing.T) {
	engine := NewEngine(nil)
	engine.AddFile(&FileItem{
		FileID: "file1",
		Name:   "test.txt",
		Path:   "/path/to/test.txt",
		Type:   "text",
		Size:   1024,
	})

	file := engine.GetFile("file1")
	if file == nil {
		t.Fatal("expected file to be created")
	}

	if file.Name != "test.txt" {
		t.Errorf("expected Name test.txt, got %s", file.Name)
	}
}

func TestAddAccessRecord(t *testing.T) {
	engine := NewEngine(nil)
	engine.AddUser("user1")
	engine.AddFile(&FileItem{
		FileID: "file1",
		Name:   "test.txt",
		Type:   "text",
	})

	engine.AddAccessRecord(&AccessRecord{
		UserID: "user1",
		FileID: "file1",
		Action: "view",
	})

	profile := engine.GetUserProfile("user1")
	if len(profile.AccessHistory) != 1 {
		t.Fatalf("expected 1 access record, got %d", len(profile.AccessHistory))
	}

	if profile.AccessHistory[0].FileID != "file1" {
		t.Errorf("expected FileID file1, got %s", profile.AccessHistory[0].FileID)
	}
}

func TestGetRecommendations(t *testing.T) {
	engine := NewEngine(nil)
	engine.AddUser("user1")

	// 添加一些文件
	engine.AddFile(&FileItem{FileID: "file1", Name: "doc1.txt", Type: "text"})
	engine.AddFile(&FileItem{FileID: "file2", Name: "doc2.txt", Type: "text"})
	engine.AddFile(&FileItem{FileID: "file3", Name: "image.jpg", Type: "image"})

	// 添加访问记录
	engine.AddAccessRecord(&AccessRecord{UserID: "user1", FileID: "file1", Action: "view"})
	engine.AddAccessRecord(&AccessRecord{UserID: "user1", FileID: "file1", Action: "view"})
	engine.AddAccessRecord(&AccessRecord{UserID: "user1", FileID: "file2", Action: "view"})

	recommendations := engine.GetRecommendations("user1", 10)
	if recommendations == nil {
		t.Fatal("expected recommendations to be returned")
	}

	// 应该至少有一些推荐
	t.Logf("Got %d recommendations", len(recommendations))
}

func TestCacheHit(t *testing.T) {
	engine := NewEngine(&Config{
		CacheTTL:   5 * time.Minute,
		MaxResults: 10,
		Weights: Weights{
			TimeDecay:     0.3,
			Frequency:     0.3,
			Collaborative: 0.2,
			Content:       0.2,
		},
	})

	engine.AddUser("user1")
	engine.AddFile(&FileItem{FileID: "file1", Name: "test.txt", Type: "text"})
	engine.AddAccessRecord(&AccessRecord{UserID: "user1", FileID: "file1", Action: "view"})

	// 第一次调用
	rec1 := engine.GetRecommendations("user1", 5)

	// 第二次调用应该命中缓存
	rec2 := engine.GetRecommendations("user1", 5)

	if len(rec1) != len(rec2) {
		t.Errorf("expected same results from cache, got %d and %d", len(rec1), len(rec2))
	}
}

func TestInvalidateCache(t *testing.T) {
	engine := NewEngine(nil)
	engine.AddUser("user1")
	engine.AddFile(&FileItem{FileID: "file1", Name: "test.txt", Type: "text"})
	engine.AddAccessRecord(&AccessRecord{UserID: "user1", FileID: "file1", Action: "view"})

	// 获取推荐
	engine.GetRecommendations("user1", 5)

	// 使缓存失效
	engine.InvalidateCache("user1")

	// 再次获取推荐
	rec := engine.GetRecommendations("user1", 5)
	if rec == nil {
		t.Fatal("expected recommendations after cache invalidation")
	}
}

func TestConcurrentAccess(t *testing.T) {
	engine := NewEngine(nil)
	engine.AddUser("user1")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			engine.AddAccessRecord(&AccessRecord{
				UserID: "user1",
				FileID: "file1",
				Action: "view",
			})
		}()
	}

	wg.Wait()

	profile := engine.GetUserProfile("user1")
	if len(profile.AccessHistory) != 100 {
		t.Errorf("expected 100 records, got %d", len(profile.AccessHistory))
	}
}

func TestTimeScoreCalculation(t *testing.T) {
	engine := NewEngine(nil)
	engine.AddUser("user1")
	engine.AddFile(&FileItem{FileID: "file1", Name: "test.txt", Type: "text"})

	// 添加访问记录
	engine.AddAccessRecord(&AccessRecord{UserID: "user1", FileID: "file1", Action: "view"})

	profile := engine.GetUserProfile("user1")
	score := engine.calculateTimeScore(profile, "file1")

	// 刚访问的文件应该有较高的时间分数
	if score < 0.9 {
		t.Errorf("expected time score > 0.9 for recent access, got %f", score)
	}
}

func TestFrequencyScoreCalculation(t *testing.T) {
	engine := NewEngine(nil)
	engine.AddUser("user1")
	engine.AddFile(&FileItem{FileID: "file1", Name: "test.txt", Type: "text"})

	// 添加多次访问
	for i := 0; i < 5; i++ {
		engine.AddAccessRecord(&AccessRecord{UserID: "user1", FileID: "file1", Action: "view"})
	}

	profile := engine.GetUserProfile("user1")
	score := engine.calculateFrequencyScore(profile, "file1")

	// 5次访问应该有中等频率分数
	if score < 0.3 || score > 0.8 {
		t.Errorf("expected frequency score between 0.3 and 0.8, got %f", score)
	}
}

func TestCollaborativeScoreCalculation(t *testing.T) {
	engine := NewEngine(nil)
	engine.AddUser("user1")
	engine.AddUser("user2")
	engine.AddFile(&FileItem{FileID: "file1", Name: "test.txt", Type: "text"})

	// 两个用户都访问了同一个文件
	engine.AddAccessRecord(&AccessRecord{UserID: "user1", FileID: "file1", Action: "view"})
	engine.AddAccessRecord(&AccessRecord{UserID: "user2", FileID: "file1", Action: "view"})

	score := engine.calculateCollaborativeScore("user1", "file1")

	// 有相似用户访问应该有协同分数
	if score < 0.1 {
		t.Errorf("expected collaborative score > 0.1, got %f", score)
	}
}

func TestContentScoreCalculation(t *testing.T) {
	engine := NewEngine(nil)
	engine.AddUser("user1")

	// 设置用户偏好
	profile := engine.GetUserProfile("user1")
	profile.Preferences["text"] = 0.8

	file := &FileItem{FileID: "file1", Name: "test.txt", Type: "text"}
	score := engine.calculateContentScore(profile, file)

	// 符合偏好的文件应该有高内容分数
	if score < 0.7 {
		t.Errorf("expected content score > 0.7 for preferred type, got %f", score)
	}
}

func TestGenerateReason(t *testing.T) {
	reason := generateReason(0.8, 0.6, 0.4, 0.3)
	if reason != "最近访问过、经常访问" {
		t.Errorf("expected '最近访问过、经常访问', got '%s'", reason)
	}

	reason = generateReason(0.5, 0.3, 0.6, 0.8)
	if reason != "相似用户也喜欢、符合你的偏好" {
		t.Errorf("expected '相似用户也喜欢、符合你的偏好', got '%s'", reason)
	}

	reason = generateReason(0.1, 0.1, 0.1, 0.1)
	if reason != "可能感兴趣" {
		t.Errorf("expected '可能感兴趣', got '%s'", reason)
	}
}

func TestGetAccessLog(t *testing.T) {
	engine := NewEngine(nil)
	engine.AddUser("user1")
	engine.AddFile(&FileItem{FileID: "file1", Name: "test.txt", Type: "text"})

	for i := 0; i < 10; i++ {
		engine.AddAccessRecord(&AccessRecord{UserID: "user1", FileID: "file1", Action: "view"})
	}

	log := engine.GetAccessLog("user1", 5)
	if len(log) != 5 {
		t.Errorf("expected 5 records, got %d", len(log))
	}
}

func TestGetUserProfileNotFound(t *testing.T) {
	engine := NewEngine(nil)
	profile := engine.GetUserProfile("nonexistent")
	if profile != nil {
		t.Error("expected nil for nonexistent user")
	}
}

func TestGetFileNotFound(t *testing.T) {
	engine := NewEngine(nil)
	file := engine.GetFile("nonexistent")
	if file != nil {
		t.Error("expected nil for nonexistent file")
	}
}
