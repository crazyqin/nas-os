// Package main 文件管理器增强插件
//
// 提供文件批量操作、快捷键支持、文件预览等功能
//
// 注意：此文件为示例代码，实际构建插件时需要：
// 1. 创建独立的 go module
// 2. 导入 nas-os/internal/plugin 包
// 3. 使用 go build -buildmode=plugin 构建
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// 插件信息（导出变量供加载器读取）
var PluginInfo = PluginInfoStruct{
	ID:          "com.nas-os.filemanager-enhance",
	Name:        "文件管理器增强",
	Version:     "1.0.0",
	Author:      "NAS-OS Team",
	Description: "增强文件管理器功能，支持批量操作、快捷键、文件预览等",
	Category:    CategoryFileManager,
	Tags:        []string{"文件管理", "批量操作", "预览", "快捷键"},
	Entrypoint:  "New",
	MainFile:    "filemanager-enhance.so",
	Icon:        "folder-open",
	License:     "MIT",
	Price:       "free",
}

// Category 插件分类
type Category string

const (
	CategoryFileManager Category = "file-manager"
	CategoryTheme       Category = "theme"
	CategoryMedia       Category = "media"
	CategoryBackup      Category = "backup"
)

// PluginInfoStruct 插件元信息
type PluginInfoStruct struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Author      string   `json:"author"`
	Description string   `json:"description"`
	Category    Category `json:"category"`
	Tags        []string `json:"tags"`
	Entrypoint  string   `json:"entrypoint"`
	MainFile    string   `json:"mainFile"`
	Icon        string   `json:"icon"`
	License     string   `json:"license"`
	Price       string   `json:"price"`
}

// Plugin 插件接口
type Plugin interface {
	Info() PluginInfoStruct
	Init(config map[string]interface{}) error
	Start() error
	Stop() error
	Destroy() error
}
type FileManagerEnhance struct {
	config   map[string]interface{}
	handlers map[string]interface{}
	mu       sync.RWMutex
	running  bool
	rootPath string // 根目录，用于路径验证
}

// New 创建插件实例（入口函数）
func New() Plugin {
	return &FileManagerEnhance{
		config:   make(map[string]interface{}),
		handlers: make(map[string]interface{}),
	}
}

// Info 返回插件信息
func (p *FileManagerEnhance) Info() PluginInfoStruct {
	return PluginInfo
}

// Init 初始化插件
func (p *FileManagerEnhance) Init(config map[string]interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 合并配置
	for k, v := range config {
		p.config[k] = v
	}

	// 设置默认值
	if _, ok := p.config["enableBatch"]; !ok {
		p.config["enableBatch"] = true
	}
	if _, ok := p.config["enablePreview"]; !ok {
		p.config["enablePreview"] = true
	}
	if _, ok := p.config["previewSize"]; !ok {
		p.config["previewSize"] = 300
	}

	// 设置根目录（用于路径安全验证）
	if rootPath, ok := p.config["rootPath"].(string); ok && rootPath != "" {
		p.rootPath = filepath.Clean(rootPath)
	} else {
		p.rootPath = "/data" // 默认根目录
	}

	// 初始化处理器
	p.handlers["batchCopy"] = p.batchCopy
	p.handlers["batchMove"] = p.batchMove
	p.handlers["batchDelete"] = p.batchDelete
	p.handlers["batchRename"] = p.batchRename
	p.handlers["preview"] = p.preview
	p.handlers["search"] = p.advancedSearch

	return nil
}

// Start 启动插件
func (p *FileManagerEnhance) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return nil
	}

	// 注册扩展点处理器
	p.registerExtensions()

	p.running = true
	return nil
}

// Stop 停止插件
func (p *FileManagerEnhance) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.running = false
	return nil
}

// Destroy 销毁插件
func (p *FileManagerEnhance) Destroy() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config = make(map[string]interface{})
	p.handlers = make(map[string]interface{})
	p.running = false

	return nil
}

// registerExtensions 注册扩展
func (p *FileManagerEnhance) registerExtensions() {
	// 这里可以注册扩展点
	// 实际实现需要与主程序通信
}

// ========== 功能实现 ==========

// BatchOperationRequest 批量操作请求
type BatchOperationRequest struct {
	Files   []string `json:"files"`
	Target  string   `json:"target,omitempty"`
	Pattern string   `json:"pattern,omitempty"` // 重命名模式
	DryRun  bool     `json:"dryRun,omitempty"`  // 仅预览
}

// BatchOperationResult 批量操作结果
type BatchOperationResult struct {
	Success []string         `json:"success"`
	Failed  []FileError      `json:"failed"`
	Summary OperationSummary `json:"summary"`
}

// FileError 文件错误
type FileError struct {
	File  string `json:"file"`
	Error string `json:"error"`
}

// OperationSummary 操作统计
type OperationSummary struct {
	Total     int `json:"total"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
}

// batchCopy 批量复制
func (p *FileManagerEnhance) batchCopy(req BatchOperationRequest) (*BatchOperationResult, error) {
	result := &BatchOperationResult{
		Success: []string{},
		Failed:  []FileError{},
	}

	if req.Target == "" {
		return nil, fmt.Errorf("目标目录不能为空")
	}

	// 验证目标路径是否在允许范围内
	if err := p.validatePaths(req.Target); err != nil {
		return nil, fmt.Errorf("目标路径验证失败: %w", err)
	}

	// 验证所有源文件路径
	for _, src := range req.Files {
		if err := p.validatePaths(src); err != nil {
			return nil, fmt.Errorf("源文件路径验证失败: %w", err)
		}
	}

	// 确保目标目录存在
	if err := os.MkdirAll(req.Target, 0755); err != nil {
		return nil, fmt.Errorf("创建目标目录失败: %w", err)
	}

	for _, src := range req.Files {
		filename := filepath.Base(src)
		dst := filepath.Join(req.Target, filename)

		if req.DryRun {
			result.Success = append(result.Success, dst)
			continue
		}

		if err := copyFile(src, dst); err != nil {
			result.Failed = append(result.Failed, FileError{
				File:  src,
				Error: err.Error(),
			})
		} else {
			result.Success = append(result.Success, dst)
		}
	}

	result.Summary = OperationSummary{
		Total:     len(req.Files),
		Succeeded: len(result.Success),
		Failed:    len(result.Failed),
	}

	return result, nil
}

// batchMove 批量移动
func (p *FileManagerEnhance) batchMove(req BatchOperationRequest) (*BatchOperationResult, error) {
	result := &BatchOperationResult{
		Success: []string{},
		Failed:  []FileError{},
	}

	if req.Target == "" {
		return nil, fmt.Errorf("目标目录不能为空")
	}

	// 验证目标路径是否在允许范围内
	if err := p.validatePaths(req.Target); err != nil {
		return nil, fmt.Errorf("目标路径验证失败: %w", err)
	}

	// 验证所有源文件路径
	for _, src := range req.Files {
		if err := p.validatePaths(src); err != nil {
			return nil, fmt.Errorf("源文件路径验证失败: %w", err)
		}
	}

	if err := os.MkdirAll(req.Target, 0755); err != nil {
		return nil, fmt.Errorf("创建目标目录失败: %w", err)
	}

	for _, src := range req.Files {
		filename := filepath.Base(src)
		dst := filepath.Join(req.Target, filename)

		if req.DryRun {
			result.Success = append(result.Success, dst)
			continue
		}

		if err := os.Rename(src, dst); err != nil {
			result.Failed = append(result.Failed, FileError{
				File:  src,
				Error: err.Error(),
			})
		} else {
			result.Success = append(result.Success, dst)
		}
	}

	result.Summary = OperationSummary{
		Total:     len(req.Files),
		Succeeded: len(result.Success),
		Failed:    len(result.Failed),
	}

	return result, nil
}

// batchDelete 批量删除
func (p *FileManagerEnhance) batchDelete(req BatchOperationRequest) (*BatchOperationResult, error) {
	result := &BatchOperationResult{
		Success: []string{},
		Failed:  []FileError{},
	}

	// 验证所有文件路径
	for _, file := range req.Files {
		if err := p.validatePaths(file); err != nil {
			return nil, fmt.Errorf("文件路径验证失败: %w", err)
		}
	}

	for _, file := range req.Files {
		if req.DryRun {
			result.Success = append(result.Success, file)
			continue
		}

		if err := os.RemoveAll(file); err != nil {
			result.Failed = append(result.Failed, FileError{
				File:  file,
				Error: err.Error(),
			})
		} else {
			result.Success = append(result.Success, file)
		}
	}

	result.Summary = OperationSummary{
		Total:     len(req.Files),
		Succeeded: len(result.Success),
		Failed:    len(result.Failed),
	}

	return result, nil
}

// batchRename 批量重命名
func (p *FileManagerEnhance) batchRename(req BatchOperationRequest) (*BatchOperationResult, error) {
	result := &BatchOperationResult{
		Success: []string{},
		Failed:  []FileError{},
	}

	if req.Pattern == "" {
		return nil, fmt.Errorf("重命名模式不能为空")
	}

	// 验证所有文件路径
	for _, src := range req.Files {
		if err := p.validatePaths(src); err != nil {
			return nil, fmt.Errorf("文件路径验证失败: %w", err)
		}
	}

	for i, src := range req.Files {
		dir := filepath.Dir(src)
		ext := filepath.Ext(src)
		name := strings.TrimSuffix(filepath.Base(src), ext)

		// 解析模式
		// {n} - 序号
		// {name} - 原文件名
		// {ext} - 扩展名
		newName := strings.ReplaceAll(req.Pattern, "{n}", fmt.Sprintf("%03d", i+1))
		newName = strings.ReplaceAll(newName, "{name}", name)
		newName = strings.ReplaceAll(newName, "{ext}", ext)

		dst := filepath.Join(dir, newName)

		if req.DryRun {
			result.Success = append(result.Success, dst)
			continue
		}

		if err := os.Rename(src, dst); err != nil {
			result.Failed = append(result.Failed, FileError{
				File:  src,
				Error: err.Error(),
			})
		} else {
			result.Success = append(result.Success, dst)
		}
	}

	result.Summary = OperationSummary{
		Total:     len(req.Files),
		Succeeded: len(result.Success),
		Failed:    len(result.Failed),
	}

	return result, nil
}

// preview 文件预览
func (p *FileManagerEnhance) preview(path string) (map[string]interface{}, error) {
	// 验证路径
	if err := p.validatePaths(path); err != nil {
		return nil, fmt.Errorf("路径验证失败: %w", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"path":  path,
		"name":  filepath.Base(path),
		"size":  info.Size(),
		"mode":  info.Mode().String(),
		"mtime": info.ModTime().Format(time.RFC3339),
	}

	// 根据文件类型提供不同的预览
	ext := strings.ToLower(filepath.Ext(path))

	if info.IsDir() {
		// 目录预览：列出文件
		entries, err := os.ReadDir(path)
		if err != nil {
			result["error"] = fmt.Sprintf("读取目录失败: %v", err)
			return result, nil
		}
		files := []map[string]string{}
		for _, e := range entries {
			files = append(files, map[string]string{
				"name": e.Name(),
				"type": map[bool]string{true: "dir", false: "file"}[e.IsDir()],
			})
		}
		result["type"] = "directory"
		result["files"] = files
	} else if isImage(ext) {
		result["type"] = "image"
		result["previewable"] = true
	} else if isText(ext) {
		// 文本文件预览前几行
		data, err := os.ReadFile(path)
		if err == nil {
			lines := strings.Split(string(data), "\n")
			if len(lines) > 100 {
				lines = lines[:100]
			}
			result["type"] = "text"
			result["content"] = strings.Join(lines, "\n")
			result["previewable"] = true
		}
	} else if isVideo(ext) {
		result["type"] = "video"
		result["previewable"] = true
	} else if isAudio(ext) {
		result["type"] = "audio"
		result["previewable"] = true
	} else {
		result["type"] = "binary"
		result["previewable"] = false
	}

	return result, nil
}

// advancedSearch 高级搜索
func (p *FileManagerEnhance) advancedSearch(root, query string, options map[string]interface{}) ([]string, error) {
	// 验证搜索根目录
	if err := p.validatePaths(root); err != nil {
		return nil, fmt.Errorf("搜索路径验证失败: %w", err)
	}

	results := []string{}
	query = strings.ToLower(query)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		name := strings.ToLower(info.Name())

		// 简单的名称匹配
		if strings.Contains(name, query) {
			results = append(results, path)
		}

		return nil
	})

	return results, err
}

// ========== 辅助函数 ==========

// isPathAllowed 检查路径是否在允许的根目录内（防止路径遍历攻击）
// 安全做法：先将用户路径与根目录连接，然后清理，最后验证是否仍在根目录内
func (p *FileManagerEnhance) isPathAllowed(path string) bool {
	if p.rootPath == "" {
		return false
	}

	// 清理根目录
	cleanRoot := filepath.Clean(p.rootPath)

	// 将用户路径与根目录连接后清理，防止路径遍历
	cleanPath := filepath.Clean(filepath.Join(cleanRoot, path))

	// 确保清理后的路径以根目录开头
	// 需要确保 rootPath 以路径分隔符结尾进行比较
	if !strings.HasPrefix(cleanPath, cleanRoot+string(filepath.Separator)) && cleanPath != cleanRoot {
		return false
	}

	return true
}

// validatePaths 验证多个路径是否都在允许范围内
func (p *FileManagerEnhance) validatePaths(paths ...string) error {
	for _, path := range paths {
		if !p.isPathAllowed(path) {
			return fmt.Errorf("路径不在允许范围内: %s", path)
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

func isImage(ext string) bool {
	images := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".svg"}
	for _, i := range images {
		if ext == i {
			return true
		}
	}
	return false
}

func isText(ext string) bool {
	texts := []string{".txt", ".md", ".json", ".yaml", ".yml", ".xml", ".html", ".css", ".js", ".go", ".py", ".sh"}
	for _, t := range texts {
		if ext == t {
			return true
		}
	}
	return false
}

func isVideo(ext string) bool {
	videos := []string{".mp4", ".mkv", ".avi", ".mov", ".webm"}
	for _, v := range videos {
		if ext == v {
			return true
		}
	}
	return false
}

func isAudio(ext string) bool {
	audios := []string{".mp3", ".wav", ".flac", ".aac", ".ogg"}
	for _, a := range audios {
		if ext == a {
			return true
		}
	}
	return false
}

// 插件导入（实际使用时取消注释）
// import "nas-os/internal/plugin"

func main() {} // 插件模式需要 main 函数
