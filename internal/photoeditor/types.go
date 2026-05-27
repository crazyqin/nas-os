// Package photoeditor 提供照片编辑功能，对标群晖Synology Photos编辑器
package photoeditor

import (
	"time"
)

// FilterType 滤镜类型.
type FilterType string

const (
	FilterVintage  FilterType = "vintage"   // 复古
	FilterBnW      FilterType = "bw"        // 黑白
	FilterVivid    FilterType = "vivid"     // 鲜艳
	FilterSoft     FilterType = "soft"      // 柔和
	FilterWarm     FilterType = "warm"      // 暖色
	FilterCool     FilterType = "cool"      // 冷色
	FilterDramatic FilterType = "dramatic"  // 戏剧
	FilterSepia    FilterType = "sepia"     // 褐色
)

// RotateType 旋转类型.
type RotateType int

const (
	Rotate90  RotateType = 90
	Rotate180 RotateType = 180
	Rotate270 RotateType = 270
)

// FlipType 翻转类型.
type FlipType string

const (
	FlipHorizontal FlipType = "horizontal"
	FlipVertical   FlipType = "vertical"
)

// AdjustParams 调整参数.
type AdjustParams struct {
	Brightness  float64 `json:"brightness"`  // -100 ~ 100
	Contrast    float64 `json:"contrast"`    // -100 ~ 100
	Saturation  float64 `json:"saturation"`  // -100 ~ 100
	Sharpness   float64 `json:"sharpness"`   // 0 ~ 100
	Exposure    float64 `json:"exposure"`    // -100 ~ 100
	Temperature float64 `json:"temperature"` // -100 ~ 100
	Tint        float64 `json:"tint"`        // -100 ~ 100
	Highlights  float64 `json:"highlights"`  // -100 ~ 100
	Shadows     float64 `json:"shadows"`     // -100 ~ 100
}

// CropParams 裁剪参数.
type CropParams struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// WatermarkParams 水印参数.
type WatermarkParams struct {
	Text     string `json:"text"`
	Position string `json:"position"` // top-left, top-right, bottom-left, bottom-right, center
	Opacity  int    `json:"opacity"`  // 0-100
	FontSize int    `json:"font_size"`
	Color    string `json:"color"`
}

// EditRequest 编辑请求.
type EditRequest struct {
	ImagePath  string           `json:"image_path" binding:"required"`
	Adjust     *AdjustParams    `json:"adjust,omitempty"`
	Crop       *CropParams      `json:"crop,omitempty"`
	Rotate     *RotateType      `json:"rotate,omitempty"`
	Flip       *FlipType        `json:"flip,omitempty"`
	Filter     *FilterType      `json:"filter,omitempty"`
	Watermark  *WatermarkParams `json:"watermark,omitempty"`
	OutputPath string           `json:"output_path"`
	Quality    int              `json:"quality"` // 1-100
}

// EditResult 编辑结果.
type EditResult struct {
	OriginalPath string `json:"original_path"`
	OutputPath   string `json:"output_path"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	FileSize     int64  `json:"file_size"`
	Format       string `json:"format"`
	EditHistory  []string `json:"edit_history"`
	CreatedAt    time.Time `json:"created_at"`
}

// Preset 预设.
type Preset struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Filter   FilterType    `json:"filter,omitempty"`
	Adjust   AdjustParams  `json:"adjust"`
	IsCustom bool          `json:"is_custom"`
}
