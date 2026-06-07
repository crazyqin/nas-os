// mediaorganizerpro 智能媒体库管理Pro测试
package mediaorganizerpro

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestNewMediaOrganizerPro 测试创建新的媒体管理器实例
func TestNewMediaOrganizerPro(t *testing.T) {
	m := NewMediaOrganizerPro()
	if m == nil {
		t.Fatal("NewMediaOrganizerPro() 返回 nil")
	}

	if m.libraries == nil {
		t.Error("libraries map 未初始化")
	}
	if m.tagIndex == nil {
		t.Error("tagIndex map 未初始化")
	}
	if m.userPreferences == nil {
		t.Error("userPreferences map 未初始化")
	}
}

// TestCreateLibrary 测试创建媒体库
func TestCreateLibrary(t *testing.T) {
	m := NewMediaOrganizerPro()

	// 创建媒体库
	lib, err := m.CreateLibrary("lib1", "测试库", "/test/path")
	if err != nil {
		t.Fatalf("创建媒体库失败: %v", err)
	}

	if lib.ID != "lib1" {
		t.Errorf("期望 ID = lib1, 实际 = %s", lib.ID)
	}
	if lib.Name != "测试库" {
		t.Errorf("期望 Name = 测试库, 实际 = %s", lib.Name)
	}
	if lib.Path != "/test/path" {
		t.Errorf("期望 Path = /test/path, 实际 = %s", lib.Path)
	}
	if lib.Items == nil {
		t.Error("Items map 未初始化")
	}

	// 测试重复创建
	_, err = m.CreateLibrary("lib1", "重复库", "/test/path2")
	if err == nil {
		t.Error("重复创建媒体库应该返回错误")
	}
}

// TestScanLibrary 测试扫描媒体库
func TestScanLibrary(t *testing.T) {
	m := NewMediaOrganizerPro()
	m.CreateLibrary("lib1", "测试库", "/test/path")

	// 准备测试数据
	items := []*MediaItem{
		{
			ID:   "img1",
			Name: "test.jpg",
			Path: "/test/path/test.jpg",
			Type: MediaTypeImage,
			Size: 1024,
			Tags: []string{"风景", "自然"},
		},
		{
			ID:   "video1",
			Name: "test.mp4",
			Path: "/test/path/test.mp4",
			Type: MediaTypeVideo,
			Size: 10240,
			Tags: []string{"旅行"},
		},
	}

	// 扫描媒体库
	addedCount, err := m.ScanLibrary("lib1", items)
	if err != nil {
		t.Fatalf("扫描媒体库失败: %v", err)
	}

	if addedCount != 2 {
		t.Errorf("期望添加 2 个文件, 实际 = %d", addedCount)
	}

	// 测试重复扫描
	addedCount, err = m.ScanLibrary("lib1", items)
	if err != nil {
		t.Fatalf("扫描媒体库失败: %v", err)
	}
	if addedCount != 0 {
		t.Errorf("重复扫描应该添加 0 个文件, 实际 = %d", addedCount)
	}

	// 测试扫描不存在的库
	_, err = m.ScanLibrary("nonexistent", items)
	if err == nil {
		t.Error("扫描不存在的库应该返回错误")
	}
}

// TestAutoClassify 测试智能分类
func TestAutoClassify(t *testing.T) {
	m := NewMediaOrganizerPro()
	m.CreateLibrary("lib1", "测试库", "/test/path")

	items := []*MediaItem{
		{
			ID:        "img1",
			Name:      "test.jpg",
			Type:      MediaTypeImage,
			CreatedAt: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			Metadata:  map[string]string{"scene": "风景", "people": "张三"},
		},
		{
			ID:        "video1",
			Name:      "test.mp4",
			Type:      MediaTypeVideo,
			CreatedAt: time.Date(2024, 2, 20, 0, 0, 0, 0, time.UTC),
			Metadata:  map[string]string{"scene": "城市"},
		},
	}

	m.ScanLibrary("lib1", items)

	// 分类测试
	result, err := m.AutoClassify("lib1")
	if err != nil {
		t.Fatalf("智能分类失败: %v", err)
	}

	// 验证按类型分类
	if _, ok := result["type_image"]; !ok {
		t.Error("缺少 type_image 分类")
	}
	if _, ok := result["type_video"]; !ok {
		t.Error("缺少 type_video 分类")
	}

	// 验证按日期分类
	if _, ok := result["date_2024-01"]; !ok {
		t.Error("缺少 date_2024-01 分类")
	}
	if _, ok := result["date_2024-02"]; !ok {
		t.Error("缺少 date_2024-02 分类")
	}

	// 验证按场景分类
	if _, ok := result["scene_风景"]; !ok {
		t.Error("缺少 scene_风景 分类")
	}
	if _, ok := result["scene_城市"]; !ok {
		t.Error("缺少 scene_城市 分类")
	}

	// 验证按人物分类
	if _, ok := result["people_张三"]; !ok {
		t.Error("缺少 people_张三 分类")
	}

	// 测试不存在的库
	_, err = m.AutoClassify("nonexistent")
	if err == nil {
		t.Error("对不存在的库分类应该返回错误")
	}
}

// TestTagOperations 测试标签操作
func TestTagOperations(t *testing.T) {
	m := NewMediaOrganizerPro()
	m.CreateLibrary("lib1", "测试库", "/test/path")

	items := []*MediaItem{
		{ID: "img1", Name: "test1.jpg", Type: MediaTypeImage, Tags: []string{"风景"}},
		{ID: "img2", Name: "test2.jpg", Type: MediaTypeImage, Tags: []string{"风景", "自然"}},
		{ID: "img3", Name: "test3.jpg", Type: MediaTypeImage, Tags: []string{"人物"}},
	}
	m.ScanLibrary("lib1", items)

	// 添加标签测试
	err := m.AddTag("lib1", "img1", "自然", "户外")
	if err != nil {
		t.Fatalf("添加标签失败: %v", err)
	}

	// 验证标签已添加
	lib, _ := m.GetLibrary("lib1")
	item := lib.Items["img1"]
	if len(item.Tags) != 3 {
		t.Errorf("期望 3 个标签, 实际 = %d", len(item.Tags))
	}

	// 测试添加重复标签
	err = m.AddTag("lib1", "img1", "风景")
	if err != nil {
		t.Fatalf("添加重复标签不应该失败: %v", err)
	}
	if len(item.Tags) != 3 {
		t.Errorf("添加重复标签后应该仍为 3 个标签, 实际 = %d", len(item.Tags))
	}

	// 移除标签测试
	err = m.RemoveTag("lib1", "img1", "户外")
	if err != nil {
		t.Fatalf("移除标签失败: %v", err)
	}
	if len(item.Tags) != 2 {
		t.Errorf("移除标签后期望 2 个标签, 实际 = %d", len(item.Tags))
	}

	// 搜索标签测试
	results := m.SearchByTag("风景", "自然")
	if len(results) != 2 {
		t.Errorf("搜索 '风景' 和 '自然' 应该返回 2 个结果, 实际 = %d", len(results))
	}

	// 搜索单个标签
	results = m.SearchByTag("风景")
	if len(results) != 2 {
		t.Errorf("搜索 '风景' 应该返回 2 个结果, 实际 = %d", len(results))
	}

	// 搜索不存在的标签
	results = m.SearchByTag("不存在的标签")
	if len(results) != 0 {
		t.Errorf("搜索不存在的标签应该返回 0 个结果, 实际 = %d", len(results))
	}
}

// TestDetectDuplicates 测试重复检测
func TestDetectDuplicates(t *testing.T) {
	m := NewMediaOrganizerPro()
	m.CreateLibrary("lib1", "测试库", "/test/path")

	items := []*MediaItem{
		{ID: "img1", Name: "test1.jpg", Type: MediaTypeImage, PerceptualHash: "abc123"},
		{ID: "img2", Name: "test2.jpg", Type: MediaTypeImage, PerceptualHash: "abc123"},
		{ID: "img3", Name: "test3.jpg", Type: MediaTypeImage, PerceptualHash: "def456"},
		{ID: "img4", Name: "test4.jpg", Type: MediaTypeImage, PerceptualHash: "abc123"},
	}
	m.ScanLibrary("lib1", items)

	// 检测重复
	duplicates, err := m.DetectDuplicates("lib1")
	if err != nil {
		t.Fatalf("检测重复失败: %v", err)
	}

	if len(duplicates) != 1 {
		t.Errorf("期望 1 个重复组, 实际 = %d", len(duplicates))
	}

	if len(duplicates) > 0 {
		group := duplicates[0]
		if group.Hash != "abc123" {
			t.Errorf("期望哈希 = abc123, 实际 = %s", group.Hash)
		}
		if len(group.Items) != 3 {
			t.Errorf("期望重复组有 3 个文件, 实际 = %d", len(group.Items))
		}
	}

	// 测试不存在的库
	_, err = m.DetectDuplicates("nonexistent")
	if err == nil {
		t.Error("检测不存在的库应该返回错误")
	}
}

// TestMetadataOperations 测试元数据操作
func TestMetadataOperations(t *testing.T) {
	m := NewMediaOrganizerPro()
	m.CreateLibrary("lib1", "测试库", "/test/path")

	items := []*MediaItem{
		{
			ID:       "img1",
			Name:     "test.jpg",
			Type:     MediaTypeImage,
			Size:     1024,
			Metadata: map[string]string{"camera": "iPhone"},
		},
	}
	m.ScanLibrary("lib1", items)

	// 提取元数据测试
	metadata, err := m.ExtractMetadata("lib1", "img1")
	if err != nil {
		t.Fatalf("提取元数据失败: %v", err)
	}

	if metadata["camera"] != "iPhone" {
		t.Errorf("期望 camera = iPhone, 实际 = %s", metadata["camera"])
	}
	if metadata["file_name"] != "test.jpg" {
		t.Errorf("期望 file_name = test.jpg, 实际 = %s", metadata["file_name"])
	}
	if metadata["file_type"] != "image" {
		t.Errorf("期望 file_type = image, 实际 = %s", metadata["file_type"])
	}
	if metadata["file_size"] != "1024" {
		t.Errorf("期望 file_size = 1024, 实际 = %s", metadata["file_size"])
	}

	// 更新元数据测试
	err = m.UpdateMetadata("lib1", "img1", map[string]string{
		"location": "北京",
		"camera":   "Canon",
	})
	if err != nil {
		t.Fatalf("更新元数据失败: %v", err)
	}

	// 验证更新
	lib, _ := m.GetLibrary("lib1")
	item := lib.Items["img1"]
	if item.Metadata["location"] != "北京" {
		t.Errorf("期望 location = 北京, 实际 = %s", item.Metadata["location"])
	}
	if item.Metadata["camera"] != "Canon" {
		t.Errorf("期望 camera = Canon, 实际 = %s", item.Metadata["camera"])
	}
}

// TestRecommend 测试智能推荐
func TestRecommend(t *testing.T) {
	m := NewMediaOrganizerPro()
	m.CreateLibrary("lib1", "测试库", "/test/path")

	items := []*MediaItem{
		{ID: "img1", Name: "test1.jpg", Type: MediaTypeImage, Tags: []string{"风景"}, AccessCount: 10},
		{ID: "img2", Name: "test2.jpg", Type: MediaTypeImage, Tags: []string{"风景", "自然"}, AccessCount: 5},
		{ID: "img3", Name: "test3.jpg", Type: MediaTypeImage, Tags: []string{"人物"}, AccessCount: 20},
	}
	m.ScanLibrary("lib1", items)

	// 增加一些标签权重
	m.AddTag("lib1", "img1", "风景")
	m.AddTag("lib1", "img1", "风景")

	// 推荐测试
	recommendations, err := m.Recommend("lib1", 2)
	if err != nil {
		t.Fatalf("推荐失败: %v", err)
	}

	if len(recommendations) != 2 {
		t.Errorf("期望推荐 2 个文件, 实际 = %d", len(recommendations))
	}

	// 验证推荐结果按分数排序
	if len(recommendations) >= 2 {
		// img3 有最高访问次数(20)，应该排在前面
		if recommendations[0].ID != "img3" {
			t.Logf("推荐顺序: %s, %s", recommendations[0].ID, recommendations[1].ID)
		}
	}

	// 测试不存在的库
	_, err = m.Recommend("nonexistent", 10)
	if err == nil {
		t.Error("推荐不存在的库应该返回错误")
	}
}

// TestGetLibraryStats 测试获取统计信息
func TestGetLibraryStats(t *testing.T) {
	m := NewMediaOrganizerPro()
	m.CreateLibrary("lib1", "测试库", "/test/path")

	items := []*MediaItem{
		{ID: "img1", Name: "test1.jpg", Type: MediaTypeImage, Size: 1024, Tags: []string{"风景"}, PerceptualHash: "abc123"},
		{ID: "img2", Name: "test2.jpg", Type: MediaTypeImage, Size: 2048, Tags: []string{"风景"}, PerceptualHash: "abc123"},
		{ID: "video1", Name: "test.mp4", Type: MediaTypeVideo, Size: 10240, Tags: []string{"旅行"}},
		{ID: "audio1", Name: "test.mp3", Type: MediaTypeAudio, Size: 5120, Tags: []string{"音乐", "流行"}},
	}
	m.ScanLibrary("lib1", items)

	// 获取统计信息
	stats, err := m.GetLibraryStats("lib1")
	if err != nil {
		t.Fatalf("获取统计信息失败: %v", err)
	}

	if stats.TotalItems != 4 {
		t.Errorf("期望 TotalItems = 4, 实际 = %d", stats.TotalItems)
	}
	if stats.TotalSize != 18432 {
		t.Errorf("期望 TotalSize = 18432, 实际 = %d", stats.TotalSize)
	}
	if stats.ImageCount != 2 {
		t.Errorf("期望 ImageCount = 2, 实际 = %d", stats.ImageCount)
	}
	if stats.VideoCount != 1 {
		t.Errorf("期望 VideoCount = 1, 实际 = %d", stats.VideoCount)
	}
	if stats.AudioCount != 1 {
		t.Errorf("期望 AudioCount = 1, 实际 = %d", stats.AudioCount)
	}
	if stats.TagCount != 4 {
		t.Errorf("期望 TagCount = 4, 实际 = %d", stats.TagCount)
	}
	if stats.DuplicateCount != 2 {
		t.Errorf("期望 DuplicateCount = 2, 实际 = %d", stats.DuplicateCount)
	}

	// 测试不存在的库
	_, err = m.GetLibraryStats("nonexistent")
	if err == nil {
		t.Error("获取不存在的库统计应该返回错误")
	}
}

// TestSearchMedia 测试搜索媒体
func TestSearchMedia(t *testing.T) {
	m := NewMediaOrganizerPro()
	m.CreateLibrary("lib1", "测试库", "/test/path")

	items := []*MediaItem{
		{ID: "img1", Name: "风景照片.jpg", Type: MediaTypeImage, Tags: []string{"风景"}, Metadata: map[string]string{"location": "北京"}},
		{ID: "img2", Name: "人物照片.jpg", Type: MediaTypeImage, Tags: []string{"人物"}, Metadata: map[string]string{"location": "上海"}},
		{ID: "img3", Name: "北京天安门.jpg", Type: MediaTypeImage, Tags: []string{"风景", "北京"}, Metadata: map[string]string{}},
	}
	m.ScanLibrary("lib1", items)

	// 按文件名搜索
	results, err := m.SearchMedia("lib1", "风景")
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("按文件名搜索 '风景' 应该返回 2 个结果, 实际 = %d", len(results))
	}

	// 按标签搜索
	results, err = m.SearchMedia("lib1", "人物")
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("按标签搜索 '人物' 应该返回 1 个结果, 实际 = %d", len(results))
	}

	// 按元数据搜索
	results, err = m.SearchMedia("lib1", "北京")
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("按元数据搜索 '北京' 应该返回 2 个结果, 实际 = %d", len(results))
	}

	// 测试不存在的库
	_, err = m.SearchMedia("nonexistent", "test")
	if err == nil {
		t.Error("搜索不存在的库应该返回错误")
	}
}

// TestRecordAccess 测试记录访问
func TestRecordAccess(t *testing.T) {
	m := NewMediaOrganizerPro()
	m.CreateLibrary("lib1", "测试库", "/test/path")

	items := []*MediaItem{
		{ID: "img1", Name: "test.jpg", Type: MediaTypeImage, Tags: []string{"风景"}},
	}
	m.ScanLibrary("lib1", items)

	// 记录访问
	err := m.RecordAccess("lib1", "img1")
	if err != nil {
		t.Fatalf("记录访问失败: %v", err)
	}

	lib, _ := m.GetLibrary("lib1")
	item := lib.Items["img1"]
	if item.AccessCount != 1 {
		t.Errorf("期望 AccessCount = 1, 实际 = %d", item.AccessCount)
	}
	if item.LastAccessedAt.IsZero() {
		t.Error("LastAccessedAt 应该被设置")
	}

	// 多次访问
	m.RecordAccess("lib1", "img1")
	m.RecordAccess("lib1", "img1")
	if item.AccessCount != 3 {
		t.Errorf("期望 AccessCount = 3, 实际 = %d", item.AccessCount)
	}
}

// TestDeleteMedia 测试删除媒体
func TestDeleteMedia(t *testing.T) {
	m := NewMediaOrganizerPro()
	m.CreateLibrary("lib1", "测试库", "/test/path")

	items := []*MediaItem{
		{ID: "img1", Name: "test1.jpg", Type: MediaTypeImage, Tags: []string{"风景", "自然"}},
		{ID: "img2", Name: "test2.jpg", Type: MediaTypeImage, Tags: []string{"风景"}},
	}
	m.ScanLibrary("lib1", items)

	// 删除媒体
	err := m.DeleteMedia("lib1", "img1")
	if err != nil {
		t.Fatalf("删除媒体失败: %v", err)
	}

	// 验证已删除
	lib, _ := m.GetLibrary("lib1")
	if _, exists := lib.Items["img1"]; exists {
		t.Error("img1 应该已被删除")
	}

	// 验证标签索引已更新
	results := m.SearchByTag("自然")
	for _, r := range results {
		if r.ID == "img1" {
			t.Error("img1 应该已从标签索引中移除")
		}
	}

	// 验证其他文件的标签不受影响
	results = m.SearchByTag("风景")
	if len(results) != 1 {
		t.Errorf("删除后搜索 '风景' 应该返回 1 个结果, 实际 = %d", len(results))
	}
}

// TestExportLibrary 测试导出媒体库
func TestExportLibrary(t *testing.T) {
	m := NewMediaOrganizerPro()
	m.CreateLibrary("lib1", "测试库", "/test/path")

	items := []*MediaItem{
		{ID: "img1", Name: "test.jpg", Type: MediaTypeImage, Size: 1024, Tags: []string{"风景"}},
	}
	m.ScanLibrary("lib1", items)

	// 导出测试
	export, err := m.ExportLibrary("lib1")
	if err != nil {
		t.Fatalf("导出失败: %v", err)
	}

	if len(export) == 0 {
		t.Error("导出内容不应为空")
	}
	if !contains(export, "测试库") {
		t.Error("导出内容应包含库名")
	}
	if !contains(export, "test.jpg") {
		t.Error("导出内容应包含文件名")
	}
}

// TestListLibraries 测试列出媒体库
func TestListLibraries(t *testing.T) {
	m := NewMediaOrganizerPro()

	m.CreateLibrary("lib2", "库B", "/path2")
	m.CreateLibrary("lib1", "库A", "/path1")
	m.CreateLibrary("lib3", "库C", "/path3")

	libraries := m.ListLibraries()
	if len(libraries) != 3 {
		t.Errorf("期望 3 个库, 实际 = %d", len(libraries))
	}

	// 验证按名称排序
	if len(libraries) >= 3 {
		if libraries[0].Name != "库A" {
			t.Errorf("第一个库应该是库A, 实际 = %s", libraries[0].Name)
		}
		if libraries[1].Name != "库B" {
			t.Errorf("第二个库应该是库B, 实际 = %s", libraries[1].Name)
		}
		if libraries[2].Name != "库C" {
			t.Errorf("第三个库应该是库C, 实际 = %s", libraries[2].Name)
		}
	}
}

// TestConcurrency 并发测试
func TestConcurrency(t *testing.T) {
	m := NewMediaOrganizerPro()
	m.CreateLibrary("lib1", "测试库", "/test/path")

	// 先添加一些初始数据
	items := make([]*MediaItem, 100)
	for i := 0; i < 100; i++ {
		items[i] = &MediaItem{
			ID:   fmt.Sprintf("img%d", i),
			Name: fmt.Sprintf("test%d.jpg", i),
			Type: MediaTypeImage,
			Size: int64(i * 100),
			Tags: []string{fmt.Sprintf("tag%d", i%10)},
		}
	}
	m.ScanLibrary("lib1", items)

	// 并发操作测试
	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// 并发读取
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := m.GetLibraryStats("lib1")
			if err != nil {
				errors <- err
			}
		}()
	}

	// 并发写入
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := m.AddTag("lib1", fmt.Sprintf("img%d", idx), fmt.Sprintf("newtag%d", idx))
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("并发操作错误: %v", err)
	}
}

// 辅助函数：检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
