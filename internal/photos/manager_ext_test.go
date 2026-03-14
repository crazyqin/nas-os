package photos

import (
	"testing"
	"time"
)

// ========== Manager Extended Tests ==========

func TestManager_GetConfig(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(tmpDir)

	config := m.GetConfig()
	if config == nil {
		t.Fatal("config 不应为 nil")
	}

	// 验证默认配置值
	if config.MaxUploadSize == 0 {
		t.Error("MaxUploadSize 应有默认值")
	}
}

func TestManager_UpdateConfig(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(tmpDir)

	newConfig := &Config{
		MaxUploadSize:    100 * 1024 * 1024,
		EnableAI:         true,
		SupportedFormats: []string{".jpg", ".png"},
	}

	err := m.UpdateConfig(newConfig)
	if err != nil {
		t.Fatalf("更新配置失败：%v", err)
	}

	config := m.GetConfig()
	if config.MaxUploadSize != 100*1024*1024 {
		t.Error("配置未正确更新")
	}
}

func TestManager_GetAlbum(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(tmpDir)

	// 创建相册
	album, _ := m.CreateAlbum("测试相册", "描述", "user1")

	// 获取相册
	result, err := m.GetAlbum(album.ID)
	if err != nil {
		t.Fatalf("获取相册失败：%v", err)
	}

	if result.Name != "测试相册" {
		t.Errorf("相册名称错误")
	}
}

func TestManager_GetAlbum_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(tmpDir)

	_, err := m.GetAlbum("nonexistent")
	if err == nil {
		t.Error("期望返回错误")
	}
}

func TestManager_GetPhoto(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(tmpDir)

	// 创建照片
	photo := &Photo{
		ID:         "photo-1",
		Filename:   "test.jpg",
		Path:       "test.jpg",
		UploadedAt: time.Now(),
		ModifiedAt: time.Now(),
	}
	m.photos[photo.ID] = photo

	// 获取照片
	result, err := m.GetPhoto(photo.ID)
	if err != nil {
		t.Fatalf("获取照片失败：%v", err)
	}

	if result.ID != "photo-1" {
		t.Error("照片 ID 错误")
	}
}

func TestManager_GetPhoto_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(tmpDir)

	_, err := m.GetPhoto("nonexistent")
	if err == nil {
		t.Error("期望返回错误")
	}
}

func TestManager_DeletePhoto(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(tmpDir)

	// 创建照片
	photo := &Photo{
		ID:         "photo-1",
		Filename:   "test.jpg",
		Path:       "test.jpg",
		UploadedAt: time.Now(),
		ModifiedAt: time.Now(),
	}
	m.photos[photo.ID] = photo

	// 删除照片
	err := m.DeletePhoto(photo.ID)
	if err != nil {
		t.Fatalf("删除照片失败：%v", err)
	}

	// 验证已删除
	if _, exists := m.photos[photo.ID]; exists {
		t.Error("照片应该已被删除")
	}
}

func TestManager_DeletePhoto_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(tmpDir)

	err := m.DeletePhoto("nonexistent")
	if err == nil {
		t.Error("期望返回错误")
	}
}

func TestManager_ListPersons(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(tmpDir)

	// 创建人物
	m.CreatePerson("张三")
	m.CreatePerson("李四")

	persons := m.ListPersons()
	if len(persons) != 2 {
		t.Errorf("期望 2 个人物，得到 %d", len(persons))
	}
}

func TestManager_UpdatePerson(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(tmpDir)

	// 创建人物
	person, _ := m.CreatePerson("张三")

	// 更新人物
	updated, err := m.UpdatePerson(person.ID, "李四")
	if err != nil {
		t.Fatalf("更新人物失败：%v", err)
	}

	if updated.Name != "李四" {
		t.Errorf("人物名称应为李四，得到 %s", updated.Name)
	}
}

func TestManager_DeletePerson(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(tmpDir)

	// 创建人物
	person, _ := m.CreatePerson("张三")

	// 删除人物
	err := m.DeletePerson(person.ID)
	if err != nil {
		t.Fatalf("删除人物失败：%v", err)
	}

	persons := m.ListPersons()
	if len(persons) != 0 {
		t.Error("人物应该已被删除")
	}
}

func TestManager_QueryPhotos_Filters(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(tmpDir)

	// 创建测试照片
	now := time.Now()
	photos := []*Photo{
		{
			ID:         "photo-1",
			Filename:   "photo1.jpg",
			Path:       "photo1.jpg",
			UserID:     "user1",
			IsFavorite: true,
			TakenAt:    now,
			UploadedAt: now,
			ModifiedAt: now,
		},
		{
			ID:         "photo-2",
			Filename:   "photo2.jpg",
			Path:       "photo2.jpg",
			UserID:     "user1",
			IsFavorite: false,
			TakenAt:    now,
			UploadedAt: now,
			ModifiedAt: now,
		},
		{
			ID:         "photo-3",
			Filename:   "photo3.jpg",
			Path:       "photo3.jpg",
			UserID:     "user2",
			IsFavorite: true,
			TakenAt:    now,
			UploadedAt: now,
			ModifiedAt: now,
		},
	}

	for _, photo := range photos {
		m.photos[photo.ID] = photo
	}

	// 按用户查询
	query := &PhotoQuery{UserID: "user1"}
	results, total, err := m.QueryPhotos(query)
	if err != nil {
		t.Fatalf("查询失败：%v", err)
	}
	if total != 2 {
		t.Errorf("期望 2 张照片，得到 %d", total)
	}

	// 按收藏查询
	query = &PhotoQuery{UserID: "user1", IsFavorite: boolPtr(true)}
	results, total, _ = m.QueryPhotos(query)
	if total != 1 {
		t.Errorf("期望 1 张收藏照片，得到 %d", total)
	}

	// 验证结果
	if len(results) == 0 {
		t.Error("结果不应为空")
	}
}

func TestManager_QueryPhotos_Sort(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(tmpDir)

	// 创建测试照片
	now := time.Now()
	for i := 0; i < 5; i++ {
		photo := &Photo{
			ID:         "photo-" + string(rune('0'+i)),
			Filename:   "test.jpg",
			Path:       "test.jpg",
			UserID:     "user1",
			TakenAt:    now.Add(time.Duration(i) * time.Hour),
			UploadedAt: now,
			ModifiedAt: now,
		}
		m.photos[photo.ID] = photo
	}

	// 按时间降序
	query := &PhotoQuery{
		UserID:    "user1",
		SortBy:    "takenAt",
		SortOrder: "desc",
	}
	results, _, _ := m.QueryPhotos(query)

	if len(results) != 5 {
		t.Errorf("期望 5 张照片，得到 %d", len(results))
	}
}

func TestManager_QueryPhotos_Pagination(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(tmpDir)

	// 创建测试照片
	now := time.Now()
	for i := 0; i < 10; i++ {
		photo := &Photo{
			ID:         "photo-" + string(rune('0'+i)),
			Filename:   "test.jpg",
			Path:       "test.jpg",
			UserID:     "user1",
			TakenAt:    now,
			UploadedAt: now,
			ModifiedAt: now,
		}
		m.photos[photo.ID] = photo
	}

	// 分页查询
	query := &PhotoQuery{
		UserID: "user1",
		Limit:  3,
		Offset: 0,
	}
	results, total, _ := m.QueryPhotos(query)

	if total != 10 {
		t.Errorf("期望总共 10 张照片，得到 %d", total)
	}

	if len(results) != 3 {
		t.Errorf("期望返回 3 张照片，得到 %d", len(results))
	}
}

func TestManager_ListAlbums_UserFilter(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(tmpDir)

	// 创建相册
	m.CreateAlbum("相册1", "", "user1")
	m.CreateAlbum("相册2", "", "user1")
	m.CreateAlbum("相册3", "", "user2")

	albums := m.ListAlbums("user1")
	if len(albums) != 2 {
		t.Errorf("期望 user1 有 2 个相册，得到 %d", len(albums))
	}
}

// ========== EXIF Tests ==========

func TestPhoto_EXIF(t *testing.T) {
	photo := &Photo{
		ID:       "photo-1",
		Filename: "test.jpg",
		EXIF: &EXIFData{
			Make:         "Canon",
			Model:        "EOS R5",
			ExposureTime: "1/1000",
			FNumber:      2.8,
			ISO:          400,
			FocalLength:  85.0,
		},
		UploadedAt: time.Now(),
		ModifiedAt: time.Now(),
	}

	if photo.EXIF.Make != "Canon" {
		t.Errorf("期望 Make 为 Canon，得到 %s", photo.EXIF.Make)
	}

	if photo.EXIF.ISO != 400 {
		t.Errorf("期望 ISO 为 400，得到 %d", photo.EXIF.ISO)
	}
}

// ========== Location Tests ==========

func TestPhoto_Location(t *testing.T) {
	photo := &Photo{
		ID:       "photo-1",
		Filename: "test.jpg",
		Location: &LocationInfo{
			Latitude:  31.2304,
			Longitude: 121.4737,
			City:      "上海",
			Country:   "中国",
		},
		UploadedAt: time.Now(),
		ModifiedAt: time.Now(),
	}

	if photo.Location.City != "上海" {
		t.Errorf("期望城市为上海，得到 %s", photo.Location.City)
	}
}

// ========== Device Tests ==========

func TestPhoto_Device(t *testing.T) {
	photo := &Photo{
		ID:       "photo-1",
		Filename: "test.jpg",
		Device: &DeviceInfo{
			Brand: "Apple",
			Model: "iPhone 15 Pro",
			OS:    "iOS 17",
			App:   "Camera",
		},
		UploadedAt: time.Now(),
		ModifiedAt: time.Now(),
	}

	if photo.Device.Brand != "Apple" {
		t.Errorf("期望品牌为 Apple，得到 %s", photo.Device.Brand)
	}
}

// ========== ShareInfo Tests ==========

func TestPhoto_ShareInfo(t *testing.T) {
	photo := &Photo{
		ID:       "photo-1",
		Filename: "test.jpg",
		ShareInfo: &ShareInfo{
			ShareID:       "share-123",
			ShareURL:      "https://example.com/share/123",
			Password:      "abc123",
			AllowDownload: true,
		},
		UploadedAt: time.Now(),
		ModifiedAt: time.Now(),
	}

	if photo.ShareInfo.ShareID != "share-123" {
		t.Errorf("期望 ShareID 为 share-123，得到 %s", photo.ShareInfo.ShareID)
	}
}

// ========== EditHistory Tests ==========

func TestPhoto_EditHistory(t *testing.T) {
	photo := &Photo{
		ID:       "photo-1",
		Filename: "test.jpg",
		EditHistory: []EditOperation{
			{
				Type:      "crop",
				Params:    map[string]int{"x": 0, "y": 0, "w": 100, "h": 100},
				Timestamp: time.Now(),
			},
			{
				Type:      "filter",
				Params:    "vintage",
				Timestamp: time.Now(),
			},
		},
		UploadedAt: time.Now(),
		ModifiedAt: time.Now(),
	}

	if len(photo.EditHistory) != 2 {
		t.Errorf("期望 2 个编辑操作，得到 %d", len(photo.EditHistory))
	}
}

// ========== PhotoQuery Tests ==========

func TestPhotoQuery_Filters(t *testing.T) {
	query := &PhotoQuery{
		UserID:     "user1",
		AlbumID:    "album1",
		StartDate:  time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:    time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
		Tags:       []string{"风景", "旅游"},
		IsFavorite: boolPtr(true),
		MimeType:   "image/jpeg",
		SortBy:     "takenAt",
		SortOrder:  "desc",
		Limit:      10,
		Offset:     0,
	}

	if query.UserID != "user1" {
		t.Error("UserID 错误")
	}
	if len(query.Tags) != 2 {
		t.Error("Tags 数量错误")
	}
	if query.Limit != 10 {
		t.Error("Limit 错误")
	}
}

// ========== ThumbnailConfig Tests ==========

func TestThumbnailConfig_Default(t *testing.T) {
	config := DefaultThumbnailConfig

	if config.SmallSize != 128 {
		t.Errorf("期望 SmallSize 为 128，得到 %d", config.SmallSize)
	}

	if config.Quality != 85 {
		t.Errorf("期望 Quality 为 85，得到 %d", config.Quality)
	}
}

// ========== UploadSession Tests ==========

func TestUploadSession_Struct(t *testing.T) {
	session := &UploadSession{
		SessionID:      "session-1",
		UserID:         "user1",
		Filename:       "video.mp4",
		TotalSize:      1024 * 1024 * 100, // 100MB
		UploadedSize:   1024 * 1024 * 50,  // 50MB
		ChunkSize:      1024 * 1024,       // 1MB
		TotalChunks:    100,
		UploadedChunks: []int{0, 1, 2, 3, 4},
		CreatedAt:      time.Now(),
		ExpiresAt:      time.Now().Add(24 * time.Hour),
	}

	if session.TotalChunks != 100 {
		t.Errorf("期望 TotalChunks 为 100，得到 %d", session.TotalChunks)
	}

	progress := float64(len(session.UploadedChunks)) / float64(session.TotalChunks) * 100
	if progress != 5.0 {
		t.Errorf("期望进度 5%%，得到 %.1f%%", progress)
	}
}

// ========== Album Tests ==========

func TestAlbum_Struct(t *testing.T) {
	album := &Album{
		ID:           "album-1",
		Name:         "测试相册",
		Description:  "测试描述",
		UserID:       "user1",
		CoverPhotoID: "photo-1",
		PhotoCount:   10,
		IsShared:     true,
		IsFavorite:   false,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Tags:         []string{"旅游", "风景"},
		Location:     "上海",
	}

	if album.PhotoCount != 10 {
		t.Errorf("期望 PhotoCount 为 10，得到 %d", album.PhotoCount)
	}

	if !album.IsShared {
		t.Error("IsShared 应为 true")
	}
}

// ========== Person Tests ==========

func TestPerson_Struct(t *testing.T) {
	person := &Person{
		ID:           "person-1",
		Name:         "张三",
		PhotoCount:   50,
		CoverPhotoID: "photo-1",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if person.PhotoCount != 50 {
		t.Errorf("期望 PhotoCount 为 50，得到 %d", person.PhotoCount)
	}
}

// ========== TimelineGroup Tests ==========

func TestTimelineGroup_Struct(t *testing.T) {
	group := &TimelineGroup{
		Period:   "2023-03",
		Photos:   []*Photo{{ID: "photo-1"}},
		Count:    1,
		Location: "上海",
	}

	if group.Period != "2023-03" {
		t.Errorf("期望 Period 为 2023-03，得到 %s", group.Period)
	}

	if group.Count != 1 {
		t.Errorf("期望 Count 为 1，得到 %d", group.Count)
	}
}

// ========== Config Tests ==========

func TestConfig_Default(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(tmpDir)

	config := m.GetConfig()

	if config.SupportedFormats == nil || len(config.SupportedFormats) == 0 {
		t.Error("SupportedFormats 应有默认值")
	}

	if config.ThumbnailConfig.Quality <= 0 {
		t.Error("ThumbnailConfig.Quality 应大于 0")
	}
}
