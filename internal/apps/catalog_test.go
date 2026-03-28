// Package apps 应用目录测试
package apps

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"nas-os/pkg/app"
)

// TestNewCatalog 测试创建目录管理器
func TestNewCatalog(t *testing.T) {
	tmpDir := t.TempDir()

	catalog, err := NewCatalog(tmpDir)
	if err != nil {
		t.Fatalf("创建目录管理器失败: %v", err)
	}
	if catalog == nil {
		t.Fatal("目录管理器为空")
	}

	// 验证内置模板已加载
	if len(catalog.templates) == 0 {
		t.Error("内置模板未加载")
	}

	// 验证分类统计
	if len(catalog.categories) == 0 {
		t.Error("分类统计为空")
	}
}

// TestNewCatalogCreateDir 测试创建目录
func TestNewCatalogCreateDir(t *testing.T) {
	// 使用不存在的目录
	tmpDir := filepath.Join(t.TempDir(), "nested", "templates")

	catalog, err := NewCatalog(tmpDir)
	if err != nil {
		t.Fatalf("创建目录管理器失败: %v", err)
	}
	if catalog == nil {
		t.Fatal("目录管理器为空")
	}

	// 验证目录已创建
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		t.Error("模板目录未创建")
	}
}

// TestList 测试列出模板
func TestList(t *testing.T) {
	tmpDir := t.TempDir()

	catalog, err := NewCatalog(tmpDir)
	if err != nil {
		t.Fatalf("创建目录管理器失败: %v", err)
	}

	// 测试列出所有模板
	templates, err := catalog.List("")
	if err != nil {
		t.Errorf("列出模板失败: %v", err)
	}
	if len(templates) == 0 {
		t.Error("模板列表为空")
	}

	// 验证内置模板数量
	expectedBuiltin := 10 // 内置模板数量
	if len(templates) < expectedBuiltin {
		t.Errorf("内置模板数量不足: got %d, want at least %d", len(templates), expectedBuiltin)
	}

	// 测试按分类过滤
	mediaTemplates, err := catalog.List(app.CategoryMedia)
	if err != nil {
		t.Errorf("按分类列出模板失败: %v", err)
	}
	for _, tmpl := range mediaTemplates {
		if tmpl.Category != app.CategoryMedia {
			t.Errorf("模板分类不匹配: got %s, want %s", tmpl.Category, app.CategoryMedia)
		}
	}

	// 测试按不存在的分类过滤
	emptyTemplates, err := catalog.List("NonExistentCategory")
	if err != nil {
		t.Errorf("按不存在分类列出模板失败: %v", err)
	}
	if len(emptyTemplates) != 0 {
		t.Errorf("不存在分类应返回空列表: got %d", len(emptyTemplates))
	}
}

// TestGet 测试获取模板
func TestGet(t *testing.T) {
	tmpDir := t.TempDir()

	catalog, err := NewCatalog(tmpDir)
	if err != nil {
		t.Fatalf("创建目录管理器失败: %v", err)
	}

	// 测试获取存在的模板
	tests := []struct {
		id       string
		wantName string
	}{
		{id: "jellyfin", wantName: "jellyfin"},
		{id: "plex", wantName: "plex"},
		{id: "nextcloud", wantName: "nextcloud"},
		{id: "redis", wantName: "redis"},
		{id: "postgres", wantName: "postgres"},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			template, err := catalog.Get(tt.id)
			if err != nil {
				t.Errorf("获取模板失败: %v", err)
				return
			}
			if template.ID != tt.id {
				t.Errorf("模板ID不匹配: got %s, want %s", template.ID, tt.id)
			}
			if template.Name != tt.wantName {
				t.Errorf("模板名称不匹配: got %s, want %s", template.Name, tt.wantName)
			}
		})
	}

	// 测试获取不存在的模板
	_, err = catalog.Get("nonexistent")
	if err == nil {
		t.Error("期望获取不存在的模板失败，但返回nil")
	}
}

// TestSearch 测试搜索模板
func TestSearch(t *testing.T) {
	tmpDir := t.TempDir()

	catalog, err := NewCatalog(tmpDir)
	if err != nil {
		t.Fatalf("创建目录管理器失败: %v", err)
	}

	tests := []struct {
		query      string
		wantCount  int
		wantContain string
	}{
		{query: "media", wantContain: "jellyfin"},
		{query: "database", wantContain: "postgres"},
		{query: "redis", wantContain: "redis"},
		{query: "torrent", wantContain: "transmission"},
		{query: "home", wantContain: "homeassistant"},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			results, err := catalog.Search(tt.query)
			if err != nil {
				t.Errorf("搜索失败: %v", err)
				return
			}
			if len(results) == 0 {
				t.Error("搜索结果为空")
				return
			}

			// 检查是否包含期望的应用
			found := false
			for _, r := range results {
				if r.ID == tt.wantContain {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("搜索结果不包含 %s", tt.wantContain)
			}
		})
	}

	// 测试空搜索（应返回所有模板）
	allResults, err := catalog.Search("")
	if err != nil {
		t.Errorf("空搜索失败: %v", err)
	}
	if len(allResults) < 10 {
		t.Errorf("空搜索应返回所有模板: got %d, want at least 10", len(allResults))
	}
}

// TestCategories 测试获取分类
func TestCategories(t *testing.T) {
	tmpDir := t.TempDir()

	catalog, err := NewCatalog(tmpDir)
	if err != nil {
		t.Fatalf("创建目录管理器失败: %v", err)
	}

	categories := catalog.Categories()
	if len(categories) == 0 {
		t.Error("分类列表为空")
	}

	// 验证分类存在
	expectedCategories := []string{
		app.CategoryMedia,
		app.CategoryProductivity,
		app.CategoryDatabase,
		app.CategoryNetwork,
	}

	for _, expected := range expectedCategories {
		found := false
		for _, cat := range categories {
			if cat == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("缺少分类: %s", expected)
		}
	}
}

// TestAdd 测试添加模板
func TestAdd(t *testing.T) {
	tmpDir := t.TempDir()

	catalog, err := NewCatalog(tmpDir)
	if err != nil {
		t.Fatalf("创建目录管理器失败: %v", err)
	}

	// 创建新模板
	newTemplate := &app.Template{
		ID:          "test-app",
		Name:        "test-app",
		DisplayName: "测试应用",
		Description: "这是一个测试应用",
		Category:    app.CategoryOther,
		Icon:        "🧪",
		Version:     "1.0.0",
		Containers: []app.ContainerSpec{
			{
				Name:  "test-app",
				Image: "test:latest",
			},
		},
	}

	// 测试添加
	err = catalog.Add(newTemplate)
	if err != nil {
		t.Errorf("添加模板失败: %v", err)
	}

	// 验证添加成功
	template, err := catalog.Get("test-app")
	if err != nil {
		t.Errorf("获取添加的模板失败: %v", err)
	}
	if template.ID != "test-app" {
		t.Errorf("模板ID不匹配: got %s, want test-app", template.ID)
	}

	// 验证文件已创建
	filePath := filepath.Join(tmpDir, "test-app.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("模板文件未创建")
	}

	// 测试重复添加
	err = catalog.Add(newTemplate)
	if err == nil {
		t.Error("期望重复添加失败，但返回nil")
	}
}

// TestAddEmptyID 测试添加空ID模板
func TestAddEmptyID(t *testing.T) {
	tmpDir := t.TempDir()

	catalog, err := NewCatalog(tmpDir)
	if err != nil {
		t.Fatalf("创建目录管理器失败: %v", err)
	}

	// 创建空ID模板
	emptyIDTemplate := &app.Template{
		Name:        "empty-id",
		DisplayName: "空ID应用",
		Category:    app.CategoryOther,
		Containers: []app.ContainerSpec{
			{Name: "test", Image: "test:latest"},
		},
	}

	err = catalog.Add(emptyIDTemplate)
	if err == nil {
		t.Error("期望添加空ID模板失败，但返回nil")
	}
}

// TestUpdate 测试更新模板
func TestUpdate(t *testing.T) {
	tmpDir := t.TempDir()

	catalog, err := NewCatalog(tmpDir)
	if err != nil {
		t.Fatalf("创建目录管理器失败: %v", err)
	}

	// 先添加一个模板
	newTemplate := &app.Template{
		ID:          "update-test",
		Name:        "update-test",
		DisplayName: "更新测试",
		Description: "原始描述",
		Category:    app.CategoryOther,
		Containers: []app.ContainerSpec{
			{Name: "test", Image: "test:latest"},
		},
	}
	if err := catalog.Add(newTemplate); err != nil {
		t.Fatalf("添加模板失败: %v", err)
	}

	// 更新模板
	updatedTemplate := &app.Template{
		ID:          "update-test",
		Name:        "update-test",
		DisplayName: "更新后的名称",
		Description: "更新后的描述",
		Category:    app.CategoryOther,
		Version:     "2.0.0",
		Containers: []app.ContainerSpec{
			{Name: "test", Image: "test:v2"},
		},
	}

	err = catalog.Update(updatedTemplate)
	if err != nil {
		t.Errorf("更新模板失败: %v", err)
	}

	// 验证更新成功
	template, err := catalog.Get("update-test")
	if err != nil {
		t.Errorf("获取更新后的模板失败: %v", err)
	}
	if template.DisplayName != "更新后的名称" {
		t.Errorf("显示名称未更新: got %s, want 更新后的名称", template.DisplayName)
	}
	if template.Description != "更新后的描述" {
		t.Errorf("描述未更新: got %s, want 更新后的描述", template.Description)
	}

	// 测试更新不存在的模板
	nonexistentTemplate := &app.Template{
		ID:          "nonexistent",
		Name:        "nonexistent",
		DisplayName: "不存在",
		Category:    app.CategoryOther,
		Containers: []app.ContainerSpec{{Name: "test", Image: "test:latest"}},
	}
	err = catalog.Update(nonexistentTemplate)
	if err == nil {
		t.Error("期望更新不存在的模板失败，但返回nil")
	}
}

// TestRemove 测试删除模板
func TestRemove(t *testing.T) {
	tmpDir := t.TempDir()

	catalog, err := NewCatalog(tmpDir)
	if err != nil {
		t.Fatalf("创建目录管理器失败: %v", err)
	}

	// 先添加一个模板
	newTemplate := &app.Template{
		ID:          "remove-test",
		Name:        "remove-test",
		DisplayName: "删除测试",
		Category:    app.CategoryOther,
		Containers: []app.ContainerSpec{
			{Name: "test", Image: "test:latest"},
		},
	}
	if err := catalog.Add(newTemplate); err != nil {
		t.Fatalf("添加模板失败: %v", err)
	}

	// 测试删除
	err = catalog.Remove("remove-test")
	if err != nil {
		t.Errorf("删除模板失败: %v", err)
	}

	// 验证删除成功
	_, err = catalog.Get("remove-test")
	if err == nil {
		t.Error("期望获取已删除模板失败，但返回nil")
	}

	// 验证文件已删除
	filePath := filepath.Join(tmpDir, "remove-test.json")
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("模板文件未删除")
	}

	// 测试删除不存在的模板
	err = catalog.Remove("nonexistent")
	if err == nil {
		t.Error("期望删除不存在的模板失败，但返回nil")
	}
}

// TestLoadTemplateFile 测试加载模板文件
func TestLoadTemplateFile(t *testing.T) {
	tmpDir := t.TempDir()

	// 创建模板文件
	template := &app.Template{
		ID:          "file-test",
		Name:        "file-test",
		DisplayName: "文件加载测试",
		Description: "从文件加载的模板",
		Category:    app.CategoryOther,
		Version:     "1.0.0",
		Containers: []app.ContainerSpec{
			{Name: "test", Image: "test:latest"},
		},
	}

	data, err := json.MarshalIndent(template, "", "  ")
	if err != nil {
		t.Fatalf("序列化模板失败: %v", err)
	}

	filePath := filepath.Join(tmpDir, "file-test.json")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		t.Fatalf("写入模板文件失败: %v", err)
	}

	// 创建目录管理器（会加载文件）
	catalog, err := NewCatalog(tmpDir)
	if err != nil {
		t.Fatalf("创建目录管理器失败: %v", err)
	}

	// 验证模板已加载
	loaded, err := catalog.Get("file-test")
	if err != nil {
		t.Errorf("获取加载的模板失败: %v", err)
	}
	if loaded.DisplayName != "文件加载测试" {
		t.Errorf("模板显示名称不匹配: got %s, want 文件加载测试", loaded.DisplayName)
	}
}

// TestLoadInvalidTemplateFile 测试加载无效模板文件
func TestLoadInvalidTemplateFile(t *testing.T) {
	tmpDir := t.TempDir()

	// 创建无效的模板文件（空ID）
	invalidData := `{"name": "invalid", "displayName": "无效模板", "category": "Other"}`
	filePath := filepath.Join(tmpDir, "invalid.json")
	if err := os.WriteFile(filePath, []byte(invalidData), 0644); err != nil {
		t.Fatalf("写入无效模板文件失败: %v", err)
	}

	// 创建目录管理器（应忽略无效文件）
	catalog, err := NewCatalog(tmpDir)
	if err != nil {
		t.Fatalf("创建目录管理器失败: %v", err)
	}

	// 验证无效模板未被加载
	_, err = catalog.Get("invalid")
	if err == nil {
		t.Error("期望获取无效模板失败，但返回nil")
	}

	// 验证其他模板仍可正常工作
	_, err = catalog.Get("jellyfin")
	if err != nil {
		t.Errorf("内置模板应可正常获取: %v", err)
	}
}

// TestLoadMalformedJSONFile 测试加载格式错误的JSON文件
func TestLoadMalformedJSONFile(t *testing.T) {
	tmpDir := t.TempDir()

	// 创建格式错误的JSON文件
	malformedData := `{invalid json}`
	filePath := filepath.Join(tmpDir, "malformed.json")
	if err := os.WriteFile(filePath, []byte(malformedData), 0644); err != nil {
		t.Fatalf("写入格式错误文件失败: %v", err)
	}

	// 创建目录管理器（应忽略错误文件）
	catalog, err := NewCatalog(tmpDir)
	if err != nil {
		t.Fatalf("创建目录管理器失败: %v", err)
	}

	// 验证目录管理器正常工作
	templates, err := catalog.List("")
	if err != nil {
		t.Errorf("列出模板失败: %v", err)
	}
	if len(templates) == 0 {
		t.Error("内置模板应正常加载")
	}
}

// TestTemplateValidation 测试模板验证
func TestTemplateValidation(t *testing.T) {
	tests := []struct {
		name    string
		template *app.Template
		wantErr bool
	}{
		{
			name: "有效模板",
			template: &app.Template{
				ID:   "valid",
				Name: "valid",
				Containers: []app.ContainerSpec{{Name: "test", Image: "test:latest"}},
			},
			wantErr: false,
		},
		{
			name: "空ID",
			template: &app.Template{
				Name: "empty-id",
				Containers: []app.ContainerSpec{{Name: "test", Image: "test:latest"}},
			},
			wantErr: true,
		},
		{
			name: "空名称",
			template: &app.Template{
				ID: "empty-name",
				Containers: []app.ContainerSpec{{Name: "test", Image: "test:latest"}},
			},
			wantErr: true,
		},
		{
			name: "无容器",
			template: &app.Template{
				ID:   "no-container",
				Name: "no-container",
			},
			wantErr: true,
		},
		{
			name: "容器无镜像",
			template: &app.Template{
				ID:   "no-image",
				Name: "no-image",
				Containers: []app.ContainerSpec{{Name: "test"}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.template.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestSortTemplates 测试模板排序
func TestSortTemplates(t *testing.T) {
	tmpDir := t.TempDir()

	catalog, err := NewCatalog(tmpDir)
	if err != nil {
		t.Fatalf("创建目录管理器失败: %v", err)
	}

	// 获取模板列表（应已排序）
	templates, err := catalog.List("")
	if err != nil {
		t.Errorf("列出模板失败: %v", err)
	}

	// 验证排序（按显示名称）
	for i := 1; i < len(templates); i++ {
		if templates[i-1].DisplayName > templates[i].DisplayName {
			t.Errorf("模板未正确排序: %s 应在 %s 之后", templates[i-1].DisplayName, templates[i].DisplayName)
		}
	}
}

// TestConcurrentAccess 测试并发访问
func TestConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()

	catalog, err := NewCatalog(tmpDir)
	if err != nil {
		t.Fatalf("创建目录管理器失败: %v", err)
	}

	// 并发读取
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = catalog.List("")
			_, _ = catalog.Get("jellyfin")
			_ = catalog.Categories()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// 并发写入
	for i := 0; i < 5; i++ {
		go func(idx int) {
			template := &app.Template{
				ID:   "concurrent-" + string(rune('a'+idx)),
				Name: "concurrent-" + string(rune('a'+idx)),
				DisplayName: "并发测试" + string(rune('a'+idx)),
				Category: app.CategoryOther,
				Containers: []app.ContainerSpec{{Name: "test", Image: "test:latest"}},
			}
			_ = catalog.Add(template)
			done <- true
		}(i)
	}

	for i := 0; i < 5; i++ {
		<-done
	}
}

// TestBuiltinTemplates 测试内置模板完整性
func TestBuiltinTemplates(t *testing.T) {
	tmpDir := t.TempDir()

	catalog, err := NewCatalog(tmpDir)
	if err != nil {
		t.Fatalf("创建目录管理器失败: %v", err)
	}

	// 验证所有内置模板
	builtinIDs := []string{
		"jellyfin", "plex", "nextcloud", "syncthing",
		"homeassistant", "transmission", "pihole",
		"postgres", "redis", "qdrant", "nginx",
	}

	for _, id := range builtinIDs {
		t.Run(id, func(t *testing.T) {
			template, err := catalog.Get(id)
			if err != nil {
				t.Errorf("获取内置模板 %s 失败: %v", id, err)
				return
			}

			// 验证必要字段
			if template.ID == "" {
				t.Error("模板ID为空")
			}
			if template.Name == "" {
				t.Error("模板名称为空")
			}
			if template.DisplayName == "" {
				t.Error("显示名称为空")
			}
			if template.Description == "" {
				t.Error("描述为空")
			}
			if template.Category == "" {
				t.Error("分类为空")
			}
			if len(template.Containers) == 0 {
				t.Error("容器列表为空")
			}

			// 验证容器规格
			for _, container := range template.Containers {
				if container.Name == "" {
					t.Error("容器名称为空")
				}
				if container.Image == "" {
					t.Error("容器镜像为空")
				}
			}

			// 验证模板有效性
			if err := template.Validate(); err != nil {
				t.Errorf("模板验证失败: %v", err)
			}
		})
	}
}

// TestCategoryChange 测试分类变更
func TestCategoryChange(t *testing.T) {
	tmpDir := t.TempDir()

	catalog, err := NewCatalog(tmpDir)
	if err != nil {
		t.Fatalf("创建目录管理器失败: %v", err)
	}

	// 添加模板
	template := &app.Template{
		ID:          "category-change-test",
		Name:        "category-change-test",
		DisplayName: "分类变更测试",
		Category:    app.CategoryOther,
		Containers: []app.ContainerSpec{
			{Name: "test", Image: "test:latest"},
		},
	}
	if err := catalog.Add(template); err != nil {
		t.Fatalf("添加模板失败: %v", err)
	}

	// 获取初始分类统计
	initialCategories := catalog.Categories()
	initialOtherCount := 0
	for _, cat := range initialCategories {
		if cat == app.CategoryOther {
			initialOtherCount = catalog.categories[app.CategoryOther]
		}
	}

	// 更新分类
	updatedTemplate := &app.Template{
		ID:          "category-change-test",
		Name:        "category-change-test",
		DisplayName: "分类变更测试",
		Category:    app.CategoryMedia, // 更换分类
		Containers: []app.ContainerSpec{
			{Name: "test", Image: "test:latest"},
		},
	}
	if err := catalog.Update(updatedTemplate); err != nil {
		t.Errorf("更新模板失败: %v", err)
	}

	// 验证分类统计已更新
	otherCount := catalog.categories[app.CategoryOther]
	_ = otherCount // 用于验证

	if otherCount != initialOtherCount-1 {
		t.Errorf("Other分类计数未减少: got %d, want %d", otherCount, initialOtherCount-1)
	}

	// 验证Media分类统计增加
	mediaCount := catalog.categories[app.CategoryMedia]
	_ = mediaCount // 分类计数验证
	if mediaCount == 0 {
		t.Error("Media分类计数应增加")
	}

	// 验证模板分类已变更
	loaded, err := catalog.Get("category-change-test")
	if err != nil {
		t.Errorf("获取模板失败: %v", err)
	}
	if loaded.Category != app.CategoryMedia {
		t.Errorf("分类未变更: got %s, want %s", loaded.Category, app.CategoryMedia)
	}
}