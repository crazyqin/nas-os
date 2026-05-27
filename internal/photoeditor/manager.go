// Package photoeditor 提供照片编辑功能
package photoeditor

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// Manager 照片编辑管理器.
type Manager struct {
	presets map[string]*Preset
}

// NewManager 创建管理器.
func NewManager() *Manager {
	mgr := &Manager{
		presets: make(map[string]*Preset),
	}
	mgr.initDefaultPresets()
	return mgr
}

// initDefaultPresets 初始化默认预设.
func (m *Manager) initDefaultPresets() {
	defaults := []Preset{
		{ID: "vintage", Name: "复古", Filter: FilterVintage},
		{ID: "bw", Name: "黑白", Filter: FilterBnW},
		{ID: "vivid", Name: "鲜艳", Filter: FilterVivid, Adjust: AdjustParams{Saturation: 30, Contrast: 15}},
		{ID: "soft", Name: "柔和", Filter: FilterSoft, Adjust: AdjustParams{Brightness: 10, Contrast: -10}},
		{ID: "warm", Name: "暖色", Filter: FilterWarm, Adjust: AdjustParams{Temperature: 25}},
		{ID: "cool", Name: "冷色", Filter: FilterCool, Adjust: AdjustParams{Temperature: -25}},
		{ID: "dramatic", Name: "戏剧", Filter: FilterDramatic, Adjust: AdjustParams{Contrast: 40, Saturation: -20}},
		{ID: "sepia", Name: "褐色", Filter: FilterSepia},
	}

	for i := range defaults {
		m.presets[defaults[i].ID] = &defaults[i]
	}
}

// ApplyEdit 应用编辑操作.
func (m *Manager) ApplyEdit(req EditRequest) (*EditResult, error) {
	if req.ImagePath == "" {
		return nil, fmt.Errorf("image path is required")
	}

	// 验证图片格式
	ext := strings.ToLower(filepath.Ext(req.ImagePath))
	supported := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true,
		".webp": true, ".bmp": true, ".tiff": true,
	}
	if !supported[ext] {
		return nil, fmt.Errorf("unsupported format: %s", ext)
	}

	// 设置默认输出路径
	outputPath := req.OutputPath
	if outputPath == "" {
		dir := filepath.Dir(req.ImagePath)
		name := strings.TrimSuffix(filepath.Base(req.ImagePath), ext)
		outputPath = filepath.Join(dir, name+"_edited"+ext)
	}

	// 设置默认质量
	quality := req.Quality
	if quality <= 0 {
		quality = 85
	}

	// 构建编辑历史
	var history []string

	if req.Adjust != nil {
		history = append(history, "adjust")
	}
	if req.Crop != nil {
		history = append(history, "crop")
	}
	if req.Rotate != nil {
		history = append(history, fmt.Sprintf("rotate_%d", *req.Rotate))
	}
	if req.Flip != nil {
		history = append(history, fmt.Sprintf("flip_%s", *req.Flip))
	}
	if req.Filter != nil {
		history = append(history, fmt.Sprintf("filter_%s", *req.Filter))
	}
	if req.Watermark != nil {
		history = append(history, "watermark")
	}

	result := &EditResult{
		OriginalPath: req.ImagePath,
		OutputPath:   outputPath,
		Width:        1920, // 模拟值
		Height:       1080,
		FileSize:     1024 * 500,
		Format:       ext[1:],
		EditHistory:  history,
		CreatedAt:    time.Now(),
	}

	return result, nil
}

// GetPresets 获取所有预设.
func (m *Manager) GetPresets() []Preset {
	var result []Preset
	for _, p := range m.presets {
		result = append(result, *p)
	}
	return result
}

// GetPreset 获取单个预设.
func (m *Manager) GetPreset(id string) (*Preset, error) {
	p, ok := m.presets[id]
	if !ok {
		return nil, fmt.Errorf("preset not found: %s", id)
	}
	return p, nil
}

// CreatePreset 创建自定义预设.
func (m *Manager) CreatePreset(preset Preset) (*Preset, error) {
	if preset.ID == "" {
		return nil, fmt.Errorf("preset ID is required")
	}
	if _, exists := m.presets[preset.ID]; exists {
		return nil, fmt.Errorf("preset already exists: %s", preset.ID)
	}

	preset.IsCustom = true
	m.presets[preset.ID] = &preset
	return &preset, nil
}

// DeletePreset 删除自定义预设.
func (m *Manager) DeletePreset(id string) error {
	p, ok := m.presets[id]
	if !ok {
		return fmt.Errorf("preset not found: %s", id)
	}
	if !p.IsCustom {
		return fmt.Errorf("cannot delete built-in preset: %s", id)
	}
	delete(m.presets, id)
	return nil
}
