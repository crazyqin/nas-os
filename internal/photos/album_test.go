package photos

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ==================== AutoClassifier 测试 ====================

func TestNewAutoClassifier(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	classifier := NewAutoClassifier(m)
	if classifier == nil {
		t.Fatal("分类器不应为 nil")
	}

	if classifier.manager != m {
		t.Error("分类器应该引用正确的管理器")
	}
}

func TestClassifyByPerson(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	classifier := NewAutoClassifier(m)

	// 创建测试照片
	now := time.Now()
	photos := []*Photo{
		{
			ID:         "photo1",
			Filename:   "photo1.jpg",
			Path:       "photo1.jpg",
			TakenAt:    now,
			Faces:      []FaceInfo{{ID: "f1", Name: "张三"}},
			UploadedAt: now,
		},
		{
			ID:         "photo2",
			Filename:   "photo2.jpg",
			Path:       "photo2.jpg",
			TakenAt:    now,
			Faces:      []FaceInfo{{ID: "f2", Name: "张三"}, {ID: "f3", Name: "李四"}},
			UploadedAt: now,
		},
		{
			ID:         "photo3",
			Filename:   "photo3.jpg",
			Path:       "photo3.jpg",
			TakenAt:    now,
			Faces:      []FaceInfo{{ID: "f4", Name: "李四"}},
			UploadedAt: now,
		},
		{
			ID:         "photo4",
			Filename:   "photo4.jpg",
			Path:       "photo4.jpg",
			TakenAt:    now,
			Faces:      []FaceInfo{{ID: "f5", Name: ""}}, // 未命名人脸
			UploadedAt: now,
		},
		{
			ID:       "photo5",
			Filename: "photo5.jpg",
			Path:     "photo5.jpg",
			TakenAt:  now,
			// 无人脸
			UploadedAt: now,
		},
	}

	for _, p := range photos {
		m.photos[p.ID] = p
	}

	categories := classifier.ClassifyByPerson(photos)

	// 应该有 3 个分类：张三、李四、未命名人物
	if len(categories) < 3 {
		t.Errorf("期望至少 3 个人物分类，得到 %d 个", len(categories))
	}

	// 验证张三分类
	var zhangSanCat *PhotoCategory
	for _, cat := range categories {
		if cat.Name == "张三" {
			zhangSanCat = cat
			break
		}
	}

	if zhangSanCat == nil {
		t.Fatal("应该有张三的分类")
	}

	if len(zhangSanCat.PhotoIDs) != 2 {
		t.Errorf("张三分类应该有 2 张照片，得到 %d 张", len(zhangSanCat.PhotoIDs))
	}

	// 验证分类类型
	for _, cat := range categories {
		if cat.Type != "person" {
			t.Errorf("分类类型应该是 'person'，得到 '%s'", cat.Type)
		}
	}
}

func TestClassifyByLocation(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	classifier := NewAutoClassifier(m)

	now := time.Now()
	photos := []*Photo{
		{
			ID:         "photo1",
			Filename:   "photo1.jpg",
			Path:       "photo1.jpg",
			TakenAt:    now,
			Location:   &LocationInfo{City: "北京", Country: "中国", Latitude: 39.9, Longitude: 116.4},
			UploadedAt: now,
		},
		{
			ID:         "photo2",
			Filename:   "photo2.jpg",
			Path:       "photo2.jpg",
			TakenAt:    now,
			Location:   &LocationInfo{City: "北京", Country: "中国", Latitude: 39.9, Longitude: 116.4},
			UploadedAt: now,
		},
		{
			ID:         "photo3",
			Filename:   "photo3.jpg",
			Path:       "photo3.jpg",
			TakenAt:    now,
			Location:   &LocationInfo{City: "上海", Country: "中国", Latitude: 31.2, Longitude: 121.5},
			UploadedAt: now,
		},
		{
			ID:       "photo4",
			Filename: "photo4.jpg",
			Path:     "photo4.jpg",
			TakenAt:  now,
			// 无地点
			UploadedAt: now,
		},
	}

	for _, p := range photos {
		m.photos[p.ID] = p
	}

	categories := classifier.ClassifyByLocation(photos)

	// 应该有 3 个分类：北京、上海、无地点信息
	if len(categories) < 3 {
		t.Errorf("期望至少 3 个地点分类，得到 %d 个", len(categories))
	}

	// 验证北京分类
	var beijingCat *PhotoCategory
	for _, cat := range categories {
		if cat.Name == "北京" {
			beijingCat = cat
			break
		}
	}

	if beijingCat == nil {
		t.Fatal("应该有北京的分类")
	}

	if len(beijingCat.PhotoIDs) != 2 {
		t.Errorf("北京分类应该有 2 张照片，得到 %d 张", len(beijingCat.PhotoIDs))
	}

	// 验证元数据
	if beijingCat.Metadata["city"] != "北京" {
		t.Error("地点分类应该包含城市元数据")
	}
}

func TestClassifyByTime(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	classifier := NewAutoClassifier(m)

	now := time.Now()
	lastMonth := now.AddDate(0, -1, 0)
	lastYear := now.AddDate(-1, 0, 0)

	photos := []*Photo{
		{
			ID:         "photo1",
			Filename:   "photo1.jpg",
			Path:       "photo1.jpg",
			TakenAt:    now,
			UploadedAt: now,
		},
		{
			ID:         "photo2",
			Filename:   "photo2.jpg",
			Path:       "photo2.jpg",
			TakenAt:    now.Add(-24 * time.Hour), // 昨天
			UploadedAt: now,
		},
		{
			ID:         "photo3",
			Filename:   "photo3.jpg",
			Path:       "photo3.jpg",
			TakenAt:    lastMonth,
			UploadedAt: now,
		},
		{
			ID:         "photo4",
			Filename:   "photo4.jpg",
			Path:       "photo4.jpg",
			TakenAt:    lastYear,
			UploadedAt: now,
		},
	}

	for _, p := range photos {
		m.photos[p.ID] = p
	}

	categories := classifier.ClassifyByTime(photos)

	// 应该有年份分类
	if len(categories) == 0 {
		t.Fatal("应该有时间分类")
	}

	// 验证年份分类
	var thisYearCat *PhotoCategory
	for _, cat := range categories {
		if year, ok := cat.Metadata["year"].(int); ok && year == now.Year() {
			thisYearCat = cat
			break
		}
	}

	if thisYearCat == nil {
		t.Fatal("应该有今年的分类")
	}

	// 验证"最近30天"分类
	var recentCat *PhotoCategory
	for _, cat := range categories {
		if cat.Name == "最近30天" {
			recentCat = cat
			break
		}
	}

	if recentCat != nil {
		// photo1 和 photo2 应该在最近30天分类中
		if len(recentCat.PhotoIDs) < 2 {
			t.Errorf("最近30天分类应该有至少 2 张照片，得到 %d 张", len(recentCat.PhotoIDs))
		}
	}
}

func TestClassifyByScene(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	classifier := NewAutoClassifier(m)

	now := time.Now()
	photos := []*Photo{
		{
			ID:         "photo1",
			Filename:   "photo1.jpg",
			Path:       "photo1.jpg",
			TakenAt:    now,
			Scene:      "nature",
			UploadedAt: now,
		},
		{
			ID:         "photo2",
			Filename:   "photo2.jpg",
			Path:       "photo2.jpg",
			TakenAt:    now,
			Scene:      "nature",
			UploadedAt: now,
		},
		{
			ID:         "photo3",
			Filename:   "photo3.jpg",
			Path:       "photo3.jpg",
			TakenAt:    now,
			Scene:      "night",
			UploadedAt: now,
		},
		{
			ID:       "photo4",
			Filename: "photo4.jpg",
			Path:     "photo4.jpg",
			TakenAt:  now,
			// 无场景
			UploadedAt: now,
		},
	}

	for _, p := range photos {
		m.photos[p.ID] = p
	}

	categories := classifier.ClassifyByScene(photos)

	// 应该有场景分类
	if len(categories) == 0 {
		t.Fatal("应该有场景分类")
	}

	// 验证自然风光分类
	var natureCat *PhotoCategory
	for _, cat := range categories {
		if cat.Name == "自然风光" {
			natureCat = cat
			break
		}
	}

	if natureCat == nil {
		t.Fatal("应该有自然风光的分类")
	}

	if len(natureCat.PhotoIDs) != 2 {
		t.Errorf("自然风光分类应该有 2 张照片，得到 %d 张", len(natureCat.PhotoIDs))
	}
}

func TestClassifyAll(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	classifier := NewAutoClassifier(m)

	now := time.Now()
	photo := &Photo{
		ID:         "photo1",
		Filename:   "photo1.jpg",
		Path:       "photo1.jpg",
		TakenAt:    now,
		Scene:      "nature",
		Location:   &LocationInfo{City: "北京"},
		Faces:      []FaceInfo{{Name: "张三"}},
		UploadedAt: now,
	}
	m.photos[photo.ID] = photo

	result, err := classifier.ClassifyAll()
	if err != nil {
		t.Fatalf("分类失败: %v", err)
	}

	if result == nil {
		t.Fatal("分类结果不应为 nil")
	}

	if len(result.PersonCategories) == 0 {
		t.Error("应该有人物分类")
	}

	if len(result.LocationCategories) == 0 {
		t.Error("应该有地点分类")
	}

	if len(result.TimeCategories) == 0 {
		t.Error("应该有时间分类")
	}

	if len(result.SceneCategories) == 0 {
		t.Error("应该有场景分类")
	}
}

func TestGetSceneDisplayName(t *testing.T) {
	classifier := &AutoClassifier{}

	tests := []struct {
		scene    string
		expected string
	}{
		{"indoor", "室内"},
		{"outdoor", "户外"},
		{"nature", "自然风光"},
		{"night", "夜景"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		result := classifier.getSceneDisplayName(tt.scene)
		if result != tt.expected {
			t.Errorf("getSceneDisplayName(%s) = %s, want %s", tt.scene, result, tt.expected)
		}
	}
}

// ==================== SmartAlbumManager 测试 ====================

func TestNewSmartAlbumManager(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	classifier := NewAutoClassifier(m)

	sam := NewSmartAlbumManager(m, classifier)
	if sam == nil {
		t.Fatal("智能相册管理器不应为 nil")
	}

	templates := sam.GetBuiltinTemplates()
	if len(templates) == 0 {
		t.Error("应该有内置模板")
	}
}

func TestCreateSmartAlbum(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	classifier := NewAutoClassifier(m)
	sam := NewSmartAlbumManager(m, classifier)

	now := time.Now()
	photos := []*Photo{
		{
			ID:         "photo1",
			Filename:   "photo1.jpg",
			Path:       "photo1.jpg",
			TakenAt:    now,
			IsFavorite: true,
			UploadedAt: now,
		},
		{
			ID:         "photo2",
			Filename:   "photo2.jpg",
			Path:       "photo2.jpg",
			TakenAt:    now,
			IsFavorite: false,
			UploadedAt: now,
		},
	}

	for _, p := range photos {
		m.photos[p.ID] = p
	}

	rules := []SmartAlbumRule{
		{ID: "r1", Type: "rating", Operator: "equals", Value: "favorite"},
	}

	album, err := sam.CreateSmartAlbum("收藏照片", "我的收藏", rules, "all")
	if err != nil {
		t.Fatalf("创建智能相册失败: %v", err)
	}

	if album.Name != "收藏照片" {
		t.Errorf("相册名称错误: %s", album.Name)
	}

	if len(album.PhotoIDs) != 1 {
		t.Errorf("应该有 1 张匹配的照片，得到 %d 张", len(album.PhotoIDs))
	}
}

func TestCreateSmartAlbumWithEmptyName(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	classifier := NewAutoClassifier(m)
	sam := NewSmartAlbumManager(m, classifier)

	rules := []SmartAlbumRule{{ID: "r1", Type: "rating", Operator: "equals", Value: "favorite"}}

	_, err := sam.CreateSmartAlbum("", "描述", rules, "all")
	if err == nil {
		t.Error("空名称应该返回错误")
	}
}

func TestCreateSmartAlbumWithEmptyRules(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	classifier := NewAutoClassifier(m)
	sam := NewSmartAlbumManager(m, classifier)

	_, err := sam.CreateSmartAlbum("测试", "描述", []SmartAlbumRule{}, "all")
	if err == nil {
		t.Error("空规则应该返回错误")
	}
}

func TestCreateFromTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	classifier := NewAutoClassifier(m)
	sam := NewSmartAlbumManager(m, classifier)

	album, err := sam.CreateFromTemplate("template-favorites", "我的收藏")
	if err != nil {
		t.Fatalf("从模板创建失败: %v", err)
	}

	if album.Name != "我的收藏" {
		t.Errorf("相册名称错误: %s", album.Name)
	}
}

func TestCreateFromNonExistentTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	classifier := NewAutoClassifier(m)
	sam := NewSmartAlbumManager(m, classifier)

	_, err := sam.CreateFromTemplate("non-existent", "")
	if err == nil {
		t.Error("不存在的模板应该返回错误")
	}
}

func TestMatchPersonRule(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	classifier := NewAutoClassifier(m)
	sam := NewSmartAlbumManager(m, classifier)

	now := time.Now()
	photos := []*Photo{
		{
			ID:         "photo1",
			Filename:   "photo1.jpg",
			Path:       "photo1.jpg",
			TakenAt:    now,
			Faces:      []FaceInfo{{Name: "张三"}},
			UploadedAt: now,
		},
		{
			ID:         "photo2",
			Filename:   "photo2.jpg",
			Path:       "photo2.jpg",
			TakenAt:    now,
			Faces:      []FaceInfo{{Name: "李四"}},
			UploadedAt: now,
		},
	}

	for _, p := range photos {
		m.photos[p.ID] = p
	}

	rules := []SmartAlbumRule{
		{ID: "r1", Type: "person", Operator: "equals", Value: "张三"},
	}

	album, _ := sam.CreateSmartAlbum("张三的照片", "", rules, "all")

	if len(album.PhotoIDs) != 1 {
		t.Errorf("应该匹配 1 张照片，得到 %d 张", len(album.PhotoIDs))
	}
}

func TestMatchLocationRule(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	classifier := NewAutoClassifier(m)
	sam := NewSmartAlbumManager(m, classifier)

	now := time.Now()
	photos := []*Photo{
		{
			ID:         "photo1",
			Filename:   "photo1.jpg",
			Path:       "photo1.jpg",
			TakenAt:    now,
			Location:   &LocationInfo{City: "北京", Country: "中国"},
			UploadedAt: now,
		},
		{
			ID:         "photo2",
			Filename:   "photo2.jpg",
			Path:       "photo2.jpg",
			TakenAt:    now,
			Location:   &LocationInfo{City: "上海", Country: "中国"},
			UploadedAt: now,
		},
	}

	for _, p := range photos {
		m.photos[p.ID] = p
	}

	rules := []SmartAlbumRule{
		{ID: "r1", Type: "location", Operator: "equals", Value: "北京"},
	}

	album, _ := sam.CreateSmartAlbum("北京照片", "", rules, "all")

	if len(album.PhotoIDs) != 1 {
		t.Errorf("应该匹配 1 张照片，得到 %d 张", len(album.PhotoIDs))
	}
}

func TestMatchSceneRule(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	classifier := NewAutoClassifier(m)
	sam := NewSmartAlbumManager(m, classifier)

	now := time.Now()
	photos := []*Photo{
		{
			ID:         "photo1",
			Filename:   "photo1.jpg",
			Path:       "photo1.jpg",
			TakenAt:    now,
			Scene:      "night",
			UploadedAt: now,
		},
		{
			ID:         "photo2",
			Filename:   "photo2.jpg",
			Path:       "photo2.jpg",
			TakenAt:    now,
			Scene:      "nature",
			UploadedAt: now,
		},
	}

	for _, p := range photos {
		m.photos[p.ID] = p
	}

	rules := []SmartAlbumRule{
		{ID: "r1", Type: "scene", Operator: "equals", Value: "night"},
	}

	album, _ := sam.CreateSmartAlbum("夜景照片", "", rules, "all")

	if len(album.PhotoIDs) != 1 {
		t.Errorf("应该匹配 1 张照片，得到 %d 张", len(album.PhotoIDs))
	}
}

func TestMatchObjectRule(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	classifier := NewAutoClassifier(m)
	sam := NewSmartAlbumManager(m, classifier)

	now := time.Now()
	photos := []*Photo{
		{
			ID:         "photo1",
			Filename:   "photo1.jpg",
			Path:       "photo1.jpg",
			TakenAt:    now,
			Objects:    []string{"vegetation", "sky"},
			UploadedAt: now,
		},
		{
			ID:         "photo2",
			Filename:   "photo2.jpg",
			Path:       "photo2.jpg",
			TakenAt:    now,
			Objects:    []string{"water"},
			UploadedAt: now,
		},
	}

	for _, p := range photos {
		m.photos[p.ID] = p
	}

	rules := []SmartAlbumRule{
		{ID: "r1", Type: "object", Operator: "equals", Value: "vegetation"},
	}

	album, _ := sam.CreateSmartAlbum("植物照片", "", rules, "all")

	if len(album.PhotoIDs) != 1 {
		t.Errorf("应该匹配 1 张照片，得到 %d 张", len(album.PhotoIDs))
	}
}

func TestMatchAnyMode(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	classifier := NewAutoClassifier(m)
	sam := NewSmartAlbumManager(m, classifier)

	now := time.Now()
	photos := []*Photo{
		{
			ID:         "photo1",
			Filename:   "photo1.jpg",
			Path:       "photo1.jpg",
			TakenAt:    now,
			Scene:      "night",
			UploadedAt: now,
		},
		{
			ID:         "photo2",
			Filename:   "photo2.jpg",
			Path:       "photo2.jpg",
			TakenAt:    now,
			Scene:      "nature",
			UploadedAt: now,
		},
		{
			ID:         "photo3",
			Filename:   "photo3.jpg",
			Path:       "photo3.jpg",
			TakenAt:    now,
			Scene:      "city",
			UploadedAt: now,
		},
	}

	for _, p := range photos {
		m.photos[p.ID] = p
	}

	rules := []SmartAlbumRule{
		{ID: "r1", Type: "scene", Operator: "equals", Value: "night"},
		{ID: "r2", Type: "scene", Operator: "equals", Value: "nature"},
	}

	// ANY 模式：匹配任意一条规则
	album, _ := sam.CreateSmartAlbum("夜拍和自然", "", rules, "any")

	if len(album.PhotoIDs) != 2 {
		t.Errorf("ANY 模式应该匹配 2 张照片，得到 %d 张", len(album.PhotoIDs))
	}
}

func TestUpdateSmartAlbum(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	classifier := NewAutoClassifier(m)
	sam := NewSmartAlbumManager(m, classifier)

	// 创建初始相册
	rules := []SmartAlbumRule{
		{ID: "r1", Type: "scene", Operator: "equals", Value: "night"},
	}
	album, _ := sam.CreateSmartAlbum("测试", "", rules, "all")

	// 更新规则
	newRules := []SmartAlbumRule{
		{ID: "r1", Type: "scene", Operator: "equals", Value: "nature"},
	}
	updated, err := sam.UpdateSmartAlbum(album.ID, newRules, "all")
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}

	if len(updated.Rules) != 1 {
		t.Error("规则应该被更新")
	}
}

func TestDeleteSmartAlbum(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	classifier := NewAutoClassifier(m)
	sam := NewSmartAlbumManager(m, classifier)

	rules := []SmartAlbumRule{
		{ID: "r1", Type: "rating", Operator: "equals", Value: "favorite"},
	}
	album, _ := sam.CreateSmartAlbum("测试", "", rules, "all")

	err := sam.DeleteSmartAlbum(album.ID)
	if err != nil {
		t.Fatalf("删除失败: %v", err)
	}

	_, err = sam.GetSmartAlbum(album.ID)
	if err == nil {
		t.Error("删除后应该无法获取")
	}
}

func TestListSmartAlbums(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	classifier := NewAutoClassifier(m)
	sam := NewSmartAlbumManager(m, classifier)

	rules := []SmartAlbumRule{
		{ID: "r1", Type: "rating", Operator: "equals", Value: "favorite"},
	}

	sam.CreateSmartAlbum("相册1", "", rules, "all")
	sam.CreateSmartAlbum("相册2", "", rules, "all")

	list := sam.ListSmartAlbums()
	if len(list) != 2 {
		t.Errorf("应该有 2 个智能相册，得到 %d 个", len(list))
	}
}

func TestRefreshSmartAlbum(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	classifier := NewAutoClassifier(m)
	sam := NewSmartAlbumManager(m, classifier)

	rules := []SmartAlbumRule{
		{ID: "r1", Type: "rating", Operator: "equals", Value: "favorite"},
	}
	album, _ := sam.CreateSmartAlbum("收藏", "", rules, "all")

	// 添加新照片
	now := time.Now()
	newPhoto := &Photo{
		ID:         "newPhoto",
		Filename:   "new.jpg",
		Path:       "new.jpg",
		TakenAt:    now,
		IsFavorite: true,
		UploadedAt: now,
	}
	m.photos[newPhoto.ID] = newPhoto

	// 刷新
	err := sam.RefreshSmartAlbum(album.ID)
	if err != nil {
		t.Fatalf("刷新失败: %v", err)
	}

	updated, _ := sam.GetSmartAlbum(album.ID)
	if len(updated.PhotoIDs) != 1 {
		t.Errorf("刷新后应该有 1 张照片，得到 %d 张", len(updated.PhotoIDs))
	}
}

func TestGetBuiltinTemplates(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	classifier := NewAutoClassifier(m)
	sam := NewSmartAlbumManager(m, classifier)

	templates := sam.GetBuiltinTemplates()

	if len(templates) == 0 {
		t.Fatal("应该有内置模板")
	}

	// 验证模板结构
	for _, tmpl := range templates {
		if tmpl.ID == "" || tmpl.Name == "" {
			t.Error("模板应该有 ID 和 Name")
		}
		if len(tmpl.Rules) == 0 {
			t.Error("模板应该有规则")
		}
	}
}

// ==================== DuplicateDetector 测试 ====================

func TestNewDuplicateDetector(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	detector := NewDuplicateDetector(m)
	if detector == nil {
		t.Fatal("重复检测器不应为 nil")
	}
}

func TestDetectDuplicatesEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	detector := NewDuplicateDetector(m)

	report, err := detector.DetectDuplicates()
	if err != nil {
		t.Fatalf("检测失败: %v", err)
	}

	if report == nil {
		t.Fatal("报告不应为 nil")
	}

	if report.TotalPhotos != 0 {
		t.Errorf("空库应该有 0 张照片")
	}
}

func TestDetectExactDuplicates(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	detector := NewDuplicateDetector(m)

	// 创建测试照片文件
	testData := []byte("test image data")
	photo1Path := filepath.Join(tmpDir, "photos", "photo1.jpg")
	photo2Path := filepath.Join(tmpDir, "photos", "photo2.jpg")

	os.MkdirAll(filepath.Dir(photo1Path), 0750)
	os.WriteFile(photo1Path, testData, 0640)
	os.WriteFile(photo2Path, testData, 0640) // 相同内容

	now := time.Now()
	photo1 := &Photo{
		ID:         "photo1",
		Filename:   "photo1.jpg",
		Path:       "photo1.jpg",
		Size:       uint64(len(testData)),
		Width:      1920,
		Height:     1080,
		TakenAt:    now,
		UploadedAt: now,
	}
	photo2 := &Photo{
		ID:         "photo2",
		Filename:   "photo2.jpg",
		Path:       "photo2.jpg",
		Size:       uint64(len(testData)),
		Width:      1280,
		Height:     720,
		TakenAt:    now,
		UploadedAt: now,
	}

	m.photos[photo1.ID] = photo1
	m.photos[photo2.ID] = photo2

	report, err := detector.DetectDuplicates()
	if err != nil {
		t.Fatalf("检测失败: %v", err)
	}

	// 应该检测到完全重复
	if len(report.ExactDuplicates) == 0 {
		t.Error("应该检测到完全重复的照片")
	}
}

func TestDetectBurstPhotos(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	detector := NewDuplicateDetector(m)

	now := time.Now()
	// 创建连拍照片（间隔小于 3 秒）
	photos := []*Photo{
		{
			ID:         "burst1",
			Filename:   "burst1.jpg",
			Path:       "burst1.jpg",
			TakenAt:    now,
			Size:       1000,
			UploadedAt: now,
		},
		{
			ID:         "burst2",
			Filename:   "burst2.jpg",
			Path:       "burst2.jpg",
			TakenAt:    now.Add(1 * time.Second),
			Size:       1000,
			UploadedAt: now,
		},
		{
			ID:         "burst3",
			Filename:   "burst3.jpg",
			Path:       "burst3.jpg",
			TakenAt:    now.Add(2 * time.Second),
			Size:       1000,
			UploadedAt: now,
		},
		{
			ID:         "normal1",
			Filename:   "normal1.jpg",
			Path:       "normal1.jpg",
			TakenAt:    now.Add(10 * time.Second), // 间隔较大
			Size:       1000,
			UploadedAt: now,
		},
	}

	for _, p := range photos {
		m.photos[p.ID] = p
	}

	burstGroups := detector.detectBurstPhotos(photos)

	if len(burstGroups) == 0 {
		t.Error("应该检测到连拍照片组")
	}

	// 连拍组应该有 3 张照片
	if len(burstGroups) > 0 && len(burstGroups[0].Photos) != 3 {
		t.Errorf("连拍组应该有 3 张照片，得到 %d 张", len(burstGroups[0].Photos))
	}
}

func TestSelectBestPhoto(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	detector := NewDuplicateDetector(m)

	photos := []*Photo{
		{
			ID:         "photo1",
			Filename:   "photo1.jpg",
			Path:       "photo1.jpg",
			Width:      1920,
			Height:     1080,
			Size:       2000000,
			IsFavorite: false,
		},
		{
			ID:         "photo2",
			Filename:   "photo2.jpg",
			Path:       "photo2.jpg",
			Width:      1280,
			Height:     720,
			Size:       1000000,
			IsFavorite: true, // 收藏的照片
		},
		{
			ID:         "photo3",
			Filename:   "photo3.jpg",
			Path:       "photo3.jpg",
			Width:      3840,
			Height:     2160,
			Size:       5000000,
			IsFavorite: false,
		},
	}

	best := detector.selectBestPhoto(photos)

	// photo3 分辨率最高，文件最大，应该是最佳选择
	if best.ID != "photo3" {
		t.Errorf("最佳照片应该是 photo3，得到 %s", best.ID)
	}
}

func TestScorePhoto(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	detector := NewDuplicateDetector(m)

	photo1 := &Photo{
		ID:         "photo1",
		Width:      1920,
		Height:     1080,
		Size:       2000000,
		IsFavorite: false,
	}

	photo2 := &Photo{
		ID:         "photo2",
		Width:      1280,
		Height:     720,
		Size:       1000000,
		IsFavorite: true,
	}

	score1 := detector.scorePhoto(photo1)
	score2 := detector.scorePhoto(photo2)

	// photo1 分辨率更高，应该得分更高
	if score1 <= 0 {
		t.Error("照片得分应该大于 0")
	}

	// 但 photo2 收藏了，有加分
	t.Logf("Score1: %f, Score2: %f", score1, score2)
}

func TestCalculateSpaceSavings(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	detector := NewDuplicateDetector(m)

	now := time.Now()
	photos := []*Photo{
		{ID: "p1", Filename: "p1.jpg", Path: "p1.jpg", Size: 1000, TakenAt: now, UploadedAt: now},
		{ID: "p2", Filename: "p2.jpg", Path: "p2.jpg", Size: 1000, TakenAt: now, UploadedAt: now},
	}

	for _, p := range photos {
		m.photos[p.ID] = p
	}

	exactDups := []*DuplicateGroup{
		{
			ID:          "g1",
			Type:        "exact",
			Photos:      photos,
			KeepPhotoID: "p1",
		},
	}

	savings := detector.calculateSpaceSavings(exactDups, nil, nil)

	// p2 会被删除，节省 1000 字节
	if savings != 1000 {
		t.Errorf("可节省空间应该是 1000，得到 %d", savings)
	}
}

func TestGetDuplicatePhotos(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	detector := NewDuplicateDetector(m)

	// 创建测试文件
	testData := []byte("duplicate test data")
	photo1Path := filepath.Join(tmpDir, "photos", "p1.jpg")
	photo2Path := filepath.Join(tmpDir, "photos", "p2.jpg")

	os.MkdirAll(filepath.Dir(photo1Path), 0750)
	os.WriteFile(photo1Path, testData, 0640)
	os.WriteFile(photo2Path, testData, 0640)

	now := time.Now()
	photo1 := &Photo{
		ID:         "p1",
		Filename:   "p1.jpg",
		Path:       "p1.jpg",
		Size:       uint64(len(testData)),
		TakenAt:    now,
		UploadedAt: now,
	}
	photo2 := &Photo{
		ID:         "p2",
		Filename:   "p2.jpg",
		Path:       "p2.jpg",
		Size:       uint64(len(testData)),
		TakenAt:    now,
		UploadedAt: now,
	}

	m.photos[photo1.ID] = photo1
	m.photos[photo2.ID] = photo2

	dups, similarity, err := detector.GetDuplicatePhotos("p1")
	if err != nil {
		t.Fatalf("获取重复照片失败: %v", err)
	}

	if len(dups) == 0 {
		t.Error("应该找到重复照片")
	}

	if similarity != 1.0 {
		t.Errorf("完全重复的相似度应该是 1.0，得到 %f", similarity)
	}
}

func TestGetDuplicatePhotosNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	detector := NewDuplicateDetector(m)

	_, _, err := detector.GetDuplicatePhotos("non-existent")
	if err == nil {
		t.Error("不存在的照片应该返回错误")
	}
}

// ==================== 辅助函数测试 ====================

func TestRoundTo(t *testing.T) {
	tests := []struct {
		val       float64
		precision int
		expected  float64
	}{
		{39.9042, 2, 39.9},
		{116.4074, 1, 116.4},
		{3.14159, 3, 3.142},
	}

	for _, tt := range tests {
		result := roundTo(tt.val, tt.precision)
		if result != tt.expected {
			t.Errorf("roundTo(%f, %d) = %f, want %f", tt.val, tt.precision, result, tt.expected)
		}
	}
}

// ==================== 集成测试 ====================

func TestFullWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	// 创建测试照片
	now := time.Now()
	testPhotos := []*Photo{
		{
			ID:         "p1",
			Filename:   "p1.jpg",
			Path:       "p1.jpg",
			TakenAt:    now,
			Scene:      "nature",
			Location:   &LocationInfo{City: "北京"},
			Faces:      []FaceInfo{{Name: "张三"}},
			IsFavorite: true,
			Size:       2000000,
			Width:      1920,
			Height:     1080,
			UploadedAt: now,
		},
		{
			ID:         "p2",
			Filename:   "p2.jpg",
			Path:       "p2.jpg",
			TakenAt:    now.Add(-24 * time.Hour),
			Scene:      "night",
			Location:   &LocationInfo{City: "北京"},
			Faces:      []FaceInfo{{Name: "张三"}},
			IsFavorite: false,
			Size:       1500000,
			Width:      1280,
			Height:     720,
			UploadedAt: now,
		},
		{
			ID:         "p3",
			Filename:   "p3.jpg",
			Path:       "p3.jpg",
			TakenAt:    now.Add(-48 * time.Hour),
			Scene:      "nature",
			Location:   &LocationInfo{City: "上海"},
			Faces:      []FaceInfo{{Name: "李四"}},
			IsFavorite: true,
			Size:       3000000,
			Width:      3840,
			Height:     2160,
			UploadedAt: now,
		},
	}

	for _, p := range testPhotos {
		m.photos[p.ID] = p
	}

	// 1. 自动分类
	classifier := NewAutoClassifier(m)
	result, err := classifier.ClassifyAll()
	if err != nil {
		t.Fatalf("分类失败: %v", err)
	}

	if len(result.PersonCategories) == 0 {
		t.Error("应该有人物分类")
	}

	// 2. 创建智能相册
	sam := NewSmartAlbumManager(m, classifier)
	favoriteRules := []SmartAlbumRule{
		{ID: "r1", Type: "rating", Operator: "equals", Value: "favorite"},
	}
	favoriteAlbum, err := sam.CreateSmartAlbum("我的收藏", "", favoriteRules, "all")
	if err != nil {
		t.Fatalf("创建智能相册失败: %v", err)
	}

	if len(favoriteAlbum.PhotoIDs) != 2 {
		t.Errorf("收藏相册应该有 2 张照片，得到 %d 张", len(favoriteAlbum.PhotoIDs))
	}

	// 3. 检测重复
	detector := NewDuplicateDetector(m)
	report, err := detector.DetectDuplicates()
	if err != nil {
		t.Fatalf("重复检测失败: %v", err)
	}

	if report.TotalPhotos != 3 {
		t.Errorf("应该有 3 张照片，得到 %d 张", report.TotalPhotos)
	}

	t.Logf("分类完成: %d 人物分类, %d 地点分类, %d 时间分类",
		len(result.PersonCategories),
		len(result.LocationCategories),
		len(result.TimeCategories))
	t.Logf("智能相册: %s (%d 张照片)", favoriteAlbum.Name, len(favoriteAlbum.PhotoIDs))
	t.Logf("重复检测: %d 完全重复组, %d 相似组, %d 连拍组",
		len(report.ExactDuplicates),
		len(report.SimilarPhotos),
		len(report.BurstGroups))
}

// ==================== 保存/加载测试 ====================

func TestSaveLoadCategories(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	classifier := NewAutoClassifier(m)

	// 创建分类
	now := time.Now()
	photos := []*Photo{
		{
			ID:         "p1",
			Filename:   "p1.jpg",
			Path:       "p1.jpg",
			TakenAt:    now,
			Scene:      "nature",
			UploadedAt: now,
		},
	}
	m.photos["p1"] = photos[0]

	classifier.ClassifyAll()

	// 保存
	if err := classifier.saveCategories(); err != nil {
		t.Fatalf("保存失败: %v", err)
	}

	// 验证文件存在
	categoriesPath := filepath.Join(tmpDir, "data", "photo-categories.json")
	if _, err := os.Stat(categoriesPath); os.IsNotExist(err) {
		t.Error("分类文件应该存在")
	}
}

func TestSaveLoadSmartAlbums(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	classifier := NewAutoClassifier(m)
	sam := NewSmartAlbumManager(m, classifier)

	rules := []SmartAlbumRule{
		{ID: "r1", Type: "rating", Operator: "equals", Value: "favorite"},
	}
	_, _ = sam.CreateSmartAlbum("测试相册", "", rules, "all")

	// 验证文件存在
	albumsPath := filepath.Join(tmpDir, "data", "smart-albums-config.json")
	if _, err := os.Stat(albumsPath); os.IsNotExist(err) {
		t.Error("智能相册文件应该存在")
	}

	// 重新加载
	sam2 := NewSmartAlbumManager(m, classifier)
	albums := sam2.ListSmartAlbums()

	if len(albums) == 0 {
		t.Error("加载后应该有智能相册")
	}
}
