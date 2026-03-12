package photos

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()

	m, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("创建管理器失败：%v", err)
	}

	if m == nil {
		t.Fatal("管理器不应为 nil")
	}

	// 验证目录创建
	if _, err := os.Stat(m.photosDir); os.IsNotExist(err) {
		t.Error("照片目录未创建")
	}
	if _, err := os.Stat(m.thumbsDir); os.IsNotExist(err) {
		t.Error("缩略图目录未创建")
	}
	if _, err := os.Stat(m.cacheDir); os.IsNotExist(err) {
		t.Error("缓存目录未创建")
	}
}

func TestCreateAlbum(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(tmpDir)

	album, err := m.CreateAlbum("测试相册", "测试描述", "user1")
	if err != nil {
		t.Fatalf("创建相册失败：%v", err)
	}

	if album.Name != "测试相册" {
		t.Errorf("相册名称错误：期望'测试相册'，得到'%s'", album.Name)
	}

	if album.Description != "测试描述" {
		t.Errorf("相册描述错误：期望'测试描述'，得到'%s'", album.Description)
	}

	if album.UserID != "user1" {
		t.Errorf("相册用户 ID 错误：期望'user1'，得到'%s'", album.UserID)
	}

	// 验证相册已保存
	albums := m.ListAlbums("user1")
	if len(albums) != 1 {
		t.Errorf("期望 1 个相册，得到 %d 个", len(albums))
	}
}

func TestUpdateAlbum(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(tmpDir)

	// 创建相册
	album, _ := m.CreateAlbum("原名", "原描述", "user1")

	// 更新相册
	updated, err := m.UpdateAlbum(album.ID, "新名", "新描述")
	if err != nil {
		t.Fatalf("更新相册失败：%v", err)
	}

	if updated.Name != "新名" {
		t.Errorf("相册名称更新失败：期望'新名'，得到'%s'", updated.Name)
	}

	if updated.Description != "新描述" {
		t.Errorf("相册描述更新失败：期望'新描述'，得到'%s'", updated.Description)
	}
}

func TestDeleteAlbum(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(tmpDir)

	// 创建相册
	album, _ := m.CreateAlbum("测试相册", "", "user1")

	// 删除相册
	err := m.DeleteAlbum(album.ID)
	if err != nil {
		t.Fatalf("删除相册失败：%v", err)
	}

	// 验证已删除
	albums := m.ListAlbums("user1")
	if len(albums) != 0 {
		t.Errorf("期望 0 个相册，得到 %d 个", len(albums))
	}
}

func TestAddPhotoToAlbum(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(tmpDir)

	// 创建相册
	album, _ := m.CreateAlbum("测试相册", "", "user1")

	// 创建测试照片
	photo := &Photo{
		ID:         "test-photo-1",
		Filename:   "test.jpg",
		Path:       "test.jpg",
		UserID:     "user1",
		UploadedAt: time.Now(),
		ModifiedAt: time.Now(),
	}
	m.photos[photo.ID] = photo

	// 添加照片到相册
	err := m.AddPhotoToAlbum(photo.ID, album.ID)
	if err != nil {
		t.Fatalf("添加照片失败：%v", err)
	}

	// 验证
	if photo.AlbumID != album.ID {
		t.Errorf("照片相册 ID 错误：期望'%s'，得到'%s'", album.ID, photo.AlbumID)
	}

	updatedAlbum, _ := m.GetAlbum(album.ID)
	if updatedAlbum.PhotoCount != 1 {
		t.Errorf("相册照片数错误：期望 1，得到 %d", updatedAlbum.PhotoCount)
	}
}

func TestRemovePhotoFromAlbum(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(tmpDir)

	// 创建相册
	album, _ := m.CreateAlbum("测试相册", "", "user1")

	// 创建并添加照片
	photo := &Photo{
		ID:         "test-photo-1",
		Filename:   "test.jpg",
		Path:       "test.jpg",
		UserID:     "user1",
		AlbumID:    album.ID,
		UploadedAt: time.Now(),
		ModifiedAt: time.Now(),
	}
	m.photos[photo.ID] = photo
	album.PhotoCount = 1

	// 移除照片
	err := m.RemovePhotoFromAlbum(photo.ID, album.ID)
	if err != nil {
		t.Fatalf("移除照片失败：%v", err)
	}

	// 验证
	if photo.AlbumID != "" {
		t.Errorf("照片相册 ID 应为空，得到'%s'", photo.AlbumID)
	}

	updatedAlbum, _ := m.GetAlbum(album.ID)
	if updatedAlbum.PhotoCount != 0 {
		t.Errorf("相册照片数错误：期望 0，得到 %d", updatedAlbum.PhotoCount)
	}
}

func TestToggleFavorite(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(tmpDir)

	// 创建照片
	photo := &Photo{
		ID:         "test-photo-1",
		Filename:   "test.jpg",
		Path:       "test.jpg",
		IsFavorite: false,
		UploadedAt: time.Now(),
		ModifiedAt: time.Now(),
	}
	m.photos[photo.ID] = photo

	// 切换收藏
	updated, err := m.ToggleFavorite(photo.ID)
	if err != nil {
		t.Fatalf("切换收藏失败：%v", err)
	}

	if !updated.IsFavorite {
		t.Error("期望收藏状态为 true")
	}

	// 再次切换
	updated2, _ := m.ToggleFavorite(photo.ID)
	if updated2.IsFavorite {
		t.Error("期望收藏状态为 false")
	}
}

func TestCreatePerson(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(tmpDir)

	person, err := m.CreatePerson("张三")
	if err != nil {
		t.Fatalf("创建人物失败：%v", err)
	}

	if person.Name != "张三" {
		t.Errorf("人物名称错误：期望'张三'，得到'%s'", person.Name)
	}

	// 验证人物已保存
	persons := m.ListPersons()
	if len(persons) != 1 {
		t.Errorf("期望 1 个人物，得到 %d 个", len(persons))
	}
}

func TestUpdatePerson(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(tmpDir)

	person, _ := m.CreatePerson("张三")

	updated, err := m.UpdatePerson(person.ID, "李四")
	if err != nil {
		t.Fatalf("更新人物失败：%v", err)
	}

	if updated.Name != "李四" {
		t.Errorf("人物名称更新失败：期望'李四'，得到'%s'", updated.Name)
	}
}

func TestDeletePerson(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(tmpDir)

	person, _ := m.CreatePerson("张三")

	err := m.DeletePerson(person.ID)
	if err != nil {
		t.Fatalf("删除人物失败：%v", err)
	}

	persons := m.ListPersons()
	if len(persons) != 0 {
		t.Errorf("期望 0 个人物，得到 %d 个", len(persons))
	}
}

func TestGetTimeline(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(tmpDir)

	// 创建测试照片
	now := time.Now()
	photos := []*Photo{
		{
			ID:         "photo1",
			Filename:   "photo1.jpg",
			Path:       "photo1.jpg",
			TakenAt:    now,
			UploadedAt: now,
			ModifiedAt: now,
		},
		{
			ID:         "photo2",
			Filename:   "photo2.jpg",
			Path:       "photo2.jpg",
			TakenAt:    now.AddDate(0, -1, 0), // 1 个月前
			UploadedAt: now,
			ModifiedAt: now,
		},
	}

	for _, photo := range photos {
		m.photos[photo.ID] = photo
	}

	// 获取时间线
	timeline, err := m.GetTimeline("", "month")
	if err != nil {
		t.Fatalf("获取时间线失败：%v", err)
	}

	if len(timeline) == 0 {
		t.Error("期望时间线不为空")
	}
}

func TestQueryPhotos(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(tmpDir)

	// 创建测试照片
	photos := []*Photo{
		{
			ID:         "photo1",
			Filename:   "photo1.jpg",
			Path:       "photo1.jpg",
			UserID:     "user1",
			IsFavorite: true,
			TakenAt:    time.Now(),
			UploadedAt: time.Now(),
			ModifiedAt: time.Now(),
		},
		{
			ID:         "photo2",
			Filename:   "photo2.jpg",
			Path:       "photo2.jpg",
			UserID:     "user1",
			IsFavorite: false,
			TakenAt:    time.Now(),
			UploadedAt: time.Now(),
			ModifiedAt: time.Now(),
		},
		{
			ID:         "photo3",
			Filename:   "photo3.jpg",
			Path:       "photo3.jpg",
			UserID:     "user2",
			IsFavorite: true,
			TakenAt:    time.Now(),
			UploadedAt: time.Now(),
			ModifiedAt: time.Now(),
		},
	}

	for _, photo := range photos {
		m.photos[photo.ID] = photo
	}

	// 按用户查询
	query := &PhotoQuery{UserID: "user1"}
	result, total, _ := m.QueryPhotos(query)
	if total != 2 {
		t.Errorf("期望 2 张照片，得到 %d", total)
	}

	// 按收藏查询
	query = &PhotoQuery{UserID: "user1", IsFavorite: boolPtr(true)}
	result, total, _ = m.QueryPhotos(query)
	if total != 1 {
		t.Errorf("期望 1 张收藏照片，得到 %d", total)
	}

	_ = result
}

func TestResizeDimensions(t *testing.T) {
	tests := []struct {
		width   int
		height  int
		maxSize int
		wantW   int
		wantH   int
	}{
		{1000, 800, 512, 512, 409},
		{800, 1000, 512, 409, 512},
		{500, 400, 512, 500, 400},   // 小于最大尺寸，不缩放
		{1000, 1000, 512, 512, 512}, // 正方形
	}

	for _, tt := range tests {
		gotW, gotH := resizeDimensions(tt.width, tt.height, tt.maxSize)
		if gotW != tt.wantW || gotH != tt.wantH {
			t.Errorf("resizeDimensions(%d, %d, %d) = (%d, %d), want (%d, %d)",
				tt.width, tt.height, tt.maxSize, gotW, gotH, tt.wantW, tt.wantH)
		}
	}
}

func TestSaveLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(tmpDir)

	// 修改配置
	m.config.EnableAI = false
	m.config.MaxUploadSize = 100 * 1024 * 1024

	// 保存配置
	if err := m.saveConfig(); err != nil {
		t.Fatalf("保存配置失败：%v", err)
	}

	// 创建新管理器并加载
	m2, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("创建管理器失败：%v", err)
	}

	if m2.config.EnableAI != false {
		t.Error("配置未正确加载")
	}
	if m2.config.MaxUploadSize != 100*1024*1024 {
		t.Error("配置未正确加载")
	}
}

func boolPtr(b bool) *bool {
	return &b
}

// 测试缩略图目录存在性
func TestThumbnailDirExists(t *testing.T) {
	tmpDir := t.TempDir()
	m, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("创建管理器失败：%v", err)
	}

	// 验证缩略图目录存在
	if _, err := os.Stat(m.thumbsDir); os.IsNotExist(err) {
		t.Error("缩略图目录应该存在")
	}
}

// 测试配置文件创建
func TestConfigFileCreated(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("创建管理器失败：%v", err)
	}

	configPath := filepath.Join(tmpDir, "photos-config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("配置文件应该被创建")
	}
}
