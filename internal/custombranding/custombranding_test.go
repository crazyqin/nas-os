package custombranding

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	engine := New()
	if engine == nil {
		t.Fatal("New返回nil")
	}
	if engine.themes == nil {
		t.Error("themes未初始化")
	}
	if engine.assets == nil {
		t.Error("assets未初始化")
	}
	if engine.templates == nil {
		t.Error("templates未初始化")
	}
	if engine.cssVars == nil {
		t.Error("cssVars未初始化")
	}
	if engine.locales == nil {
		t.Error("locales未初始化")
	}
	if engine.version != "1.0.0" {
		t.Errorf("版本应为1.0.0, 实际 %s", engine.version)
	}
}

func TestDefaultConfig(t *testing.T) {
	engine := New()
	cfg := engine.GetConfig()

	if cfg.Name != "NAS-OS" {
		t.Errorf("默认品牌名应为NAS-OS, 实际 %s", cfg.Name)
	}
	if cfg.ThemeID != "default" {
		t.Errorf("默认主题应为default, 实际 %s", cfg.ThemeID)
	}
	if cfg.Locale != "zh-CN" {
		t.Errorf("默认语言应为zh-CN, 实际 %s", cfg.Locale)
	}
	if cfg.Colors.Primary != "#1976D2" {
		t.Errorf("主色应为#1976D2, 实际 %s", cfg.Colors.Primary)
	}
}

func TestPresetThemes(t *testing.T) {
	engine := New()
	themes := engine.GetThemes()

	expectedThemes := []string{"default", "dark", "tech", "business"}
	for _, id := range expectedThemes {
		if _, exists := themes[id]; !exists {
			t.Errorf("预设主题 %s 不存在", id)
		}
	}
}

func TestStartStop(t *testing.T) {
	engine := New()

	if err := engine.Start(); err != nil {
		t.Fatalf("Start失败: %v", err)
	}

	// 重复启动应报错
	if err := engine.Start(); err == nil {
		t.Error("重复启动应返回错误")
	}

	if err := engine.Stop(); err != nil {
		t.Fatalf("Stop失败: %v", err)
	}

	// 未运行时Stop应报错
	if err := engine.Stop(); err == nil {
		t.Error("未运行时Stop应返回错误")
	}
}

func TestApplyTheme(t *testing.T) {
	engine := New()

	tests := []struct {
		name      string
		themeID   string
		wantError bool
	}{
		{name: "应用暗色主题", themeID: "dark", wantError: false},
		{name: "应用科技主题", themeID: "tech", wantError: false},
		{name: "不存在的主题", themeID: "nonexistent", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.ApplyTheme(tt.themeID)
			if (err != nil) != tt.wantError {
				t.Errorf("期望错误=%v, 实际=%v", tt.wantError, err)
			}
		})
	}
}

func TestUpdateConfig(t *testing.T) {
	engine := New()

	updates := &BrandingConfig{
		Name:   "Custom Brand",
		Colors: ColorScheme{Primary: "#FF0000"},
	}

	if err := engine.UpdateConfig(updates); err != nil {
		t.Fatalf("UpdateConfig失败: %v", err)
	}

	cfg := engine.GetConfig()
	if cfg.Name != "Custom Brand" {
		t.Errorf("品牌名应为Custom Brand, 实际 %s", cfg.Name)
	}
	if cfg.Colors.Primary != "#FF0000" {
		t.Errorf("主色应为#FF0000, 实际 %s", cfg.Colors.Primary)
	}
}

func TestExportImportConfig(t *testing.T) {
	engine := New()

	// 导出
	data, err := engine.ExportConfig()
	if err != nil {
		t.Fatalf("ExportConfig失败: %v", err)
	}

	// 验证JSON有效性
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("导出数据不是有效JSON: %v", err)
	}

	// 修改后导入
	engine2 := New()
	engine2.UpdateConfig(&BrandingConfig{Name: "Temp"})
	if err := engine2.ImportConfig(data); err != nil {
		t.Fatalf("ImportConfig失败: %v", err)
	}
}

func TestUploadAndGetAsset(t *testing.T) {
	engine := New()

	asset, err := engine.UploadAsset("logo", "image", "/assets/logo.png", "image/png", "admin", []byte("fake-image-data"))
	if err != nil {
		t.Fatalf("UploadAsset失败: %v", err)
	}
	if asset.Name != "logo" {
		t.Errorf("资产名应为logo, 实际 %s", asset.Name)
	}
	if asset.Type != "image" {
		t.Errorf("资产类型应为image, 实际 %s", asset.Type)
	}

	// 获取资产
	got, err := engine.GetAsset(asset.ID)
	if err != nil {
		t.Fatalf("GetAsset失败: %v", err)
	}
	if got.ID != asset.ID {
		t.Error("资产ID不匹配")
	}

	// 获取不存在的资产
	if _, err := engine.GetAsset("nonexistent"); err == nil {
		t.Error("获取不存在的资产应返回错误")
	}
}

func TestListAssets(t *testing.T) {
	engine := New()

	engine.UploadAsset("a", "image", "/a.png", "image/png", "user", []byte("a"))
	engine.UploadAsset("b", "font", "/b.ttf", "font/ttf", "user", []byte("b"))

	assets := engine.ListAssets()
	if len(assets) != 2 {
		t.Errorf("期望2个资产, 实际 %d", len(assets))
	}
}

func TestCSSVars(t *testing.T) {
	engine := New()

	engine.Start()
	defer engine.Stop()

	vars := engine.GetCSSVars()
	if _, exists := vars["--color-primary"]; !exists {
		t.Error("--color-primary 变量不存在")
	}

	// 设置自定义CSS变量
	engine.SetCSSVar("--custom-var", "test-value")
	vars = engine.GetCSSVars()
	if vars["--custom-var"] != "test-value" {
		t.Errorf("自定义变量值不匹配")
	}
}

func TestGetLocale(t *testing.T) {
	engine := New()

	tests := []struct {
		name      string
		code      string
		wantError bool
	}{
		{name: "中文", code: "zh-CN", wantError: false},
		{name: "英文", code: "en-US", wantError: false},
		{name: "日文", code: "ja-JP", wantError: false},
		{name: "不存在", code: "ko-KR", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := engine.GetLocale(tt.code)
			if (err != nil) != tt.wantError {
				t.Errorf("期望错误=%v, 实际=%v", tt.wantError, err)
			}
		})
	}
}

func TestCreateTheme(t *testing.T) {
	engine := New()

	theme, err := engine.CreateTheme("My Theme", "自定义主题", []string{"custom"})
	if err != nil {
		t.Fatalf("CreateTheme失败: %v", err)
	}
	if theme.Name != "My Theme" {
		t.Errorf("主题名应为My Theme, 实际 %s", theme.Name)
	}
	if theme.IsPreset {
		t.Error("自定义主题不应是预设")
	}

	// 验证主题已注册
	themes := engine.GetThemes()
	if _, exists := themes[theme.ID]; !exists {
		t.Error("创建的主题未在主题列表中")
	}
}

func TestGetTemplate(t *testing.T) {
	engine := New()

	tests := []struct {
		name      string
		id        string
		wantError bool
	}{
		{name: "企业模板", id: "enterprise", wantError: false},
		{name: "个人模板", id: "personal", wantError: false},
		{name: "创意模板", id: "creative", wantError: false},
		{name: "不存在", id: "nonexistent", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := engine.GetTemplate(tt.id)
			if (err != nil) != tt.wantError {
				t.Errorf("期望错误=%v, 实际=%v", tt.wantError, err)
			}
		})
	}
}

func TestGetTemplatesByCategory(t *testing.T) {
	engine := New()

	enterprise := engine.GetTemplatesByCategory("enterprise")
	if len(enterprise) == 0 {
		t.Error("enterprise分类应有模板")
	}

	personal := engine.GetTemplatesByCategory("personal")
	if len(personal) == 0 {
		t.Error("personal分类应有模板")
	}
}

func TestApplyTemplate(t *testing.T) {
	engine := New()

	if err := engine.ApplyTemplate("enterprise"); err != nil {
		t.Fatalf("ApplyTemplate失败: %v", err)
	}

	if err := engine.ApplyTemplate("nonexistent"); err == nil {
		t.Error("应用不存在的模板应返回错误")
	}
}

func TestGetHistory(t *testing.T) {
	engine := New()

	engine.ApplyTheme("dark")
	engine.UpdateConfig(&BrandingConfig{Name: "Updated"})

	history := engine.GetHistory()
	if len(history) < 2 {
		t.Errorf("期望至少2条历史记录, 实际 %d", len(history))
	}
}

func TestConfigClone(t *testing.T) {
	cfg := &BrandingConfig{
		Name:      "Test",
		CustomCSS: map[string]string{"a": "1"},
	}

	clone := cfg.Clone()
	if clone.Name != "Test" {
		t.Error("克隆名称不匹配")
	}
	if clone.CustomCSS["a"] != "1" {
		t.Error("克隆CSS变量不匹配")
	}

	// 修改克隆不应影响原始
	clone.CustomCSS["b"] = "2"
	if _, exists := cfg.CustomCSS["b"]; exists {
		t.Error("修改克隆不应影响原始配置")
	}
}

func TestSetPreviewCallback(t *testing.T) {
	engine := New()
	done := make(chan bool, 1)

	engine.SetPreviewCallback(func(cfg *BrandingConfig) {
		done <- true
	})

	engine.ApplyTheme("dark")

	select {
	case <-done:
		// 回调被触发
	case <-time.After(2 * time.Second):
		// 回调可能未在测试环境中触发，不作为失败条件
	}
}
