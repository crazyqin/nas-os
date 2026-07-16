// Package vmimport 提供虚拟机镜像导入导出功能
package vmimport

import (
	"os"
	"path/filepath"
	"testing"
)

// ========== Manager 测试 ==========

// TestNewManager 测试创建管理器.
func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "storage")
	metaPath := filepath.Join(tmpDir, "metadata")

	m, err := NewManager(storagePath, metaPath)
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	if m == nil {
		t.Fatal("管理器不应为nil")
	}

	// 检查目录是否创建.
	if _, err := os.Stat(storagePath); os.IsNotExist(err) {
		t.Error("存储目录应已创建")
	}
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Error("元数据目录应已创建")
	}
}

// TestNewManagerDefaultPaths 测试默认路径.
func TestNewManagerDefaultPaths(t *testing.T) {
	// 使用临时目录模拟默认路径测试
	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "storage")
	metaPath := filepath.Join(tmpDir, "meta")
	m, err := NewManager(storagePath, metaPath)
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}
	if m == nil {
		t.Fatal("管理器不应为nil")
	}
}

// TestGetSupportedFormats 测试获取支持格式列表.
func TestGetSupportedFormats(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(filepath.Join(tmpDir, "s"), filepath.Join(tmpDir, "m"))

	formats := m.GetSupportedFormats()
	if len(formats) != 6 {
		t.Errorf("期望6种格式, 实际 %d", len(formats))
	}

	// 检查每种格式.
	formatMap := make(map[DiskFormat]bool)
	for _, f := range formats {
		formatMap[f.Name] = true
		if !f.CanImport {
			t.Errorf("格式 %s 应支持导入", f.Name)
		}
		if !f.CanExport {
			t.Errorf("格式 %s 应支持导出", f.Name)
		}
	}

	expectedFormats := []DiskFormat{FormatQCOW2, FormatQED, FormatRAW, FormatVDI, FormatVHDX, FormatVMDK}
	for _, ef := range expectedFormats {
		if !formatMap[ef] {
			t.Errorf("缺少格式: %s", ef)
		}
	}
}

// TestImportInvalidFormat 测试导入无效格式.
func TestImportInvalidFormat(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(filepath.Join(tmpDir, "s"), filepath.Join(tmpDir, "m"))

	req := ImportRequest{
		Source:       "/nonexistent/file.img",
		SourceType:   "file",
		TargetName:   "test",
		TargetFormat: "invalid_format",
	}

	_, err := m.StartImport(req)
	if err == nil {
		t.Error("期望返回不支持的格式错误")
	}
}

// TestImportFileNotFound 测试导入不存在的文件.
func TestImportFileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(filepath.Join(tmpDir, "s"), filepath.Join(tmpDir, "m"))

	req := ImportRequest{
		Source:       "/nonexistent/file.img",
		SourceType:   "file",
		TargetName:   "test",
		TargetFormat: FormatQCOW2,
	}

	_, err := m.StartImport(req)
	if err == nil {
		t.Error("期望返回文件不存在错误")
	}
}

// TestImportInvalidSourceType 测试导入无效来源类型.
func TestImportInvalidSourceType(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(filepath.Join(tmpDir, "s"), filepath.Join(tmpDir, "m"))

	// 创建一个临时文件.
	tmpFile := filepath.Join(tmpDir, "test.img")
	os.WriteFile(tmpFile, make([]byte, 1024), 0o644)

	req := ImportRequest{
		Source:       tmpFile,
		SourceType:   "ftp", // 无效的来源类型.
		TargetName:   "test",
		TargetFormat: FormatQCOW2,
	}

	_, err := m.StartImport(req)
	if err == nil {
		t.Error("期望返回不支持的来源类型错误")
	}
}

// TestListImagesEmpty 测试空镜像列表.
func TestListImagesEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(filepath.Join(tmpDir, "s"), filepath.Join(tmpDir, "m"))

	images := m.ListImages()
	if len(images) != 0 {
		t.Errorf("期望空列表, 实际 %d 个镜像", len(images))
	}
}

// TestGetImageNotFound 测试获取不存在的镜像.
func TestGetImageNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(filepath.Join(tmpDir, "s"), filepath.Join(tmpDir, "m"))

	_, err := m.GetImage("nonexistent-id")
	if err != ErrImageNotFound {
		t.Errorf("期望 ErrImageNotFound, 实际 %v", err)
	}
}

// TestDeleteImageNotFound 测试删除不存在的镜像.
func TestDeleteImageNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(filepath.Join(tmpDir, "s"), filepath.Join(tmpDir, "m"))

	err := m.DeleteImage("nonexistent-id")
	if err != ErrImageNotFound {
		t.Errorf("期望 ErrImageNotFound, 实际 %v", err)
	}
}

// TestImportStatusNotFound 测试获取不存在的导入状态.
func TestImportStatusNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(filepath.Join(tmpDir, "s"), filepath.Join(tmpDir, "m"))

	_, err := m.GetImportStatus("nonexistent-id")
	if err != ErrImportNotFound {
		t.Errorf("期望 ErrImportNotFound, 实际 %v", err)
	}
}

// TestExportStatusNotFound 测试获取不存在的导出状态.
func TestExportStatusNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(filepath.Join(tmpDir, "s"), filepath.Join(tmpDir, "m"))

	_, err := m.GetExportStatus("nonexistent-id")
	if err != ErrExportNotFound {
		t.Errorf("期望 ErrExportNotFound, 实际 %v", err)
	}
}

// TestCancelImportNotFound 测试取消不存在的导入.
func TestCancelImportNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(filepath.Join(tmpDir, "s"), filepath.Join(tmpDir, "m"))

	err := m.CancelImport("nonexistent-id")
	if err != ErrImportNotFound {
		t.Errorf("期望 ErrImportNotFound, 实际 %v", err)
	}
}

// TestCancelExportNotFound 测试取消不存在的导出.
func TestCancelExportNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(filepath.Join(tmpDir, "s"), filepath.Join(tmpDir, "m"))

	err := m.CancelExport("nonexistent-id")
	if err != ErrExportNotFound {
		t.Errorf("期望 ErrExportNotFound, 实际 %v", err)
	}
}

// TestExportImageNotFound 测试导出不存在的镜像.
func TestExportImageNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(filepath.Join(tmpDir, "s"), filepath.Join(tmpDir, "m"))

	req := ExportRequest{
		ImageID:      "nonexistent-id",
		TargetFormat: FormatQCOW2,
	}

	_, err := m.StartExport(req)
	if err != ErrImageNotFound {
		t.Errorf("期望 ErrImageNotFound, 实际 %v", err)
	}
}

// TestExportInvalidFormat 测试导出无效格式.
func TestExportInvalidFormat(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(filepath.Join(tmpDir, "s"), filepath.Join(tmpDir, "m"))

	// 创建一个镜像.
	imgFile := filepath.Join(tmpDir, "test.qcow2")
	os.WriteFile(imgFile, make([]byte, 1024), 0o644)
	m.images["test-img"] = &VMImage{
		ID:       "test-img",
		Name:     "test",
		Format:   FormatQCOW2,
		FilePath: imgFile,
		FileSize: 1024,
	}

	req := ExportRequest{
		ImageID:      "test-img",
		TargetFormat: "invalid_format",
	}

	_, err := m.StartExport(req)
	if err == nil {
		t.Error("期望返回不支持的格式错误")
	}
}

// TestGetStorageUsage 测试获取存储空间使用情况.
func TestGetStorageUsage(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(filepath.Join(tmpDir, "s"), filepath.Join(tmpDir, "m"))

	// 添加一个镜像.
	imgFile := filepath.Join(tmpDir, "test.img")
	os.WriteFile(imgFile, make([]byte, 1024), 0o644)
	m.images["test-img"] = &VMImage{
		ID:       "test-img",
		Name:     "test",
		Format:   FormatRAW,
		FilePath: imgFile,
		FileSize: 1024,
	}

	usage, err := m.GetStorageUsage()
	if err != nil {
		t.Fatalf("获取存储使用情况失败: %v", err)
	}

	if usage.ImageCount != 1 {
		t.Errorf("期望1个镜像, 实际 %d", usage.ImageCount)
	}
	if usage.ImagesTotalSize != 1024 {
		t.Errorf("期望镜像总大小1024, 实际 %d", usage.ImagesTotalSize)
	}
}

// TestDetectFormatRAW 测试检测RAW格式.
func TestDetectFormatRAW(t *testing.T) {
	tmpDir := t.TempDir()
	rawFile := filepath.Join(tmpDir, "test.img")

	// 创建一个随机数据的RAW文件.
	data := make([]byte, 4096)
	for i := range data {
		data[i] = byte(i % 256)
	}
	os.WriteFile(rawFile, data, 0o644)

	format, err := DetectFormat(rawFile)
	if err != nil {
		t.Fatalf("检测格式失败: %v", err)
	}

	if format != FormatRAW {
		t.Errorf("期望 RAW, 实际 %s", format)
	}
}

// TestValidateImageNotFound 测试验证不存在的文件.
func TestValidateImageNotFound(t *testing.T) {
	result, err := ValidateImage("/nonexistent/file.img")
	if err != nil {
		t.Fatalf("不应返回错误, 实际 %v", err)
	}

	if result.Valid {
		t.Error("不存在的文件不应有效")
	}
}

// TestValidateImageEmpty 测试验证空文件.
func TestValidateImageEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	emptyFile := filepath.Join(tmpDir, "empty.img")
	os.WriteFile(emptyFile, []byte{}, 0o644)

	result, err := ValidateImage(emptyFile)
	if err != nil {
		t.Fatalf("不应返回错误, 实际 %v", err)
	}

	if result.Valid {
		t.Error("空文件不应有效")
	}
}

// TestValidateImageValid 测试验证有效的RAW文件.
func TestValidateImageValid(t *testing.T) {
	tmpDir := t.TempDir()
	rawFile := filepath.Join(tmpDir, "valid.img")
	os.WriteFile(rawFile, make([]byte, 4096), 0o644)

	result, err := ValidateImage(rawFile)
	if err != nil {
		t.Fatalf("验证失败: %v", err)
	}

	if !result.Valid {
		t.Error("有效的RAW文件应通过验证")
	}
	if result.Format != FormatRAW {
		t.Errorf("期望格式 RAW, 实际 %s", result.Format)
	}
	if result.FileSize != 4096 {
		t.Errorf("期望文件大小 4096, 实际 %d", result.FileSize)
	}
}

// TestIsSupportedFormat 测试格式支持检查.
func TestIsSupportedFormat(t *testing.T) {
	tests := []struct {
		format   DiskFormat
		expected bool
	}{
		{FormatQCOW2, true},
		{FormatQED, true},
		{FormatRAW, true},
		{FormatVDI, true},
		{FormatVHDX, true},
		{FormatVMDK, true},
		{"vhd", false},
		{"iso", false},
		{"", false},
	}

	for _, tt := range tests {
		result := isSupportedFormat(tt.format)
		if result != tt.expected {
			t.Errorf("isSupportedFormat(%s) = %v, 期望 %v", tt.format, result, tt.expected)
		}
	}
}

// TestGenerateID 测试生成ID.
func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()

	if id1 == "" {
		t.Error("生成的ID不应为空")
	}
	if id1 == id2 {
		t.Error("两次生成的ID不应相同")
	}
}

// TestMatchBytes 测试字节匹配.
func TestMatchBytes(t *testing.T) {
	tests := []struct {
		a, b     []byte
		expected bool
	}{
		{[]byte{1, 2, 3}, []byte{1, 2, 3}, true},
		{[]byte{1, 2, 3}, []byte{1, 2, 4}, false},
		{[]byte{1, 2}, []byte{1, 2, 3}, false},
		{[]byte{}, []byte{}, true},
	}

	for _, tt := range tests {
		result := matchBytes(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("matchBytes(%v, %v) = %v, 期望 %v", tt.a, tt.b, result, tt.expected)
		}
	}
}

// TestImageLifecycle 测试镜像完整生命周期.
func TestImageLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	m, _ := NewManager(filepath.Join(tmpDir, "s"), filepath.Join(tmpDir, "m"))

	// 手动添加镜像.
	imgFile := filepath.Join(tmpDir, "test.qcow2")
	os.WriteFile(imgFile, make([]byte, 2048), 0o644)

	img := &VMImage{
		ID:          "test-image-001",
		Name:        "test-vm",
		Format:      FormatQCOW2,
		FilePath:    imgFile,
		FileSize:    2048,
		VirtualSize: 10240,
	}
	m.images[img.ID] = img

	// 列出镜像.
	images := m.ListImages()
	if len(images) != 1 {
		t.Fatalf("期望1个镜像, 实际 %d", len(images))
	}

	// 获取镜像.
	got, err := m.GetImage("test-image-001")
	if err != nil {
		t.Fatalf("获取镜像失败: %v", err)
	}
	if got.Name != "test-vm" {
		t.Errorf("期望名称 test-vm, 实际 %s", got.Name)
	}

	// 删除镜像.
	if err := m.DeleteImage("test-image-001"); err != nil {
		t.Fatalf("删除镜像失败: %v", err)
	}

	// 确认已删除.
	images = m.ListImages()
	if len(images) != 0 {
		t.Errorf("期望0个镜像, 实际 %d", len(images))
	}

	_, err = m.GetImage("test-image-001")
	if err != ErrImageNotFound {
		t.Errorf("期望 ErrImageNotFound, 实际 %v", err)
	}
}
