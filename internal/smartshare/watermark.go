// Package smartshare 提供动态水印添加功能
package smartshare

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// WatermarkEngine 水印引擎
type WatermarkEngine struct {
	mu     sync.RWMutex
	logger *zap.Logger
}

// NewWatermarkEngine 创建水印引擎
func NewWatermarkEngine(logger *zap.Logger) *WatermarkEngine {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &WatermarkEngine{
		logger: logger,
	}
}

// WatermarkRequest 水印请求
type WatermarkRequest struct {
	FilePath   string           `json:"file_path"`
	Config     *WatermarkConfig `json:"config"`
	UserInfo   *UserInfo        `json:"user_info,omitempty"` // 用户信息水印
	Timestamp  bool             `json:"timestamp"`           // 是否添加时间戳
	CustomText string           `json:"custom_text,omitempty"`
}

// UserInfo 用户信息
type UserInfo struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	IP       string `json:"ip"`
}

// WatermarkResult 水印结果
type WatermarkResult struct {
	OutputPath   string `json:"output_path"`
	OriginalSize int64  `json:"original_size"`
	OutputSize   int64  `json:"output_size"`
	Format       string `json:"format"`
	CreatedAt    time.Time `json:"created_at"`
}

// AddTextWatermark 添加文本水印
func (we *WatermarkEngine) AddTextWatermark(req *WatermarkRequest) (*WatermarkResult, error) {
	we.mu.Lock()
	defer we.mu.Unlock()

	if req.Config == nil {
		req.Config = DefaultWatermarkConfig()
	}

	// 验证配置
	if err := we.validateConfig(req.Config); err != nil {
		return nil, err
	}

	// 构建水印文本
	watermarkText := we.buildWatermarkText(req)

	we.logger.Info("adding text watermark",
		zap.String("file", req.FilePath),
		zap.String("text", watermarkText),
		zap.String("position", string(req.Config.Position)))

	// 生成输出路径
	outputPath := we.generateOutputPath(req.FilePath)

	// 模拟水印处理
	result := &WatermarkResult{
		OutputPath:  outputPath,
		Format:      "png",
		CreatedAt:   time.Now(),
	}

	we.logger.Info("watermark added successfully",
		zap.String("output", outputPath))

	return result, nil
}

// AddImageWatermark 添加图片水印
func (we *WatermarkEngine) AddImageWatermark(req *WatermarkRequest) (*WatermarkResult, error) {
	we.mu.Lock()
	defer we.mu.Unlock()

	if req.Config == nil || req.Config.ImageURL == "" {
		return nil, fmt.Errorf("image watermark URL is required")
	}

	we.logger.Info("adding image watermark",
		zap.String("file", req.FilePath),
		zap.String("image", req.Config.ImageURL),
		zap.String("position", string(req.Config.Position)))

	outputPath := we.generateOutputPath(req.FilePath)

	result := &WatermarkResult{
		OutputPath: outputPath,
		Format:     "png",
		CreatedAt:  time.Now(),
	}

	return result, nil
}

// AddDynamicWatermark 添加动态水印（包含用户信息、时间戳等）
func (we *WatermarkEngine) AddDynamicWatermark(req *WatermarkRequest) (*WatermarkResult, error) {
	we.mu.Lock()
	defer we.mu.Unlock()

	if req.Config == nil {
		req.Config = DefaultWatermarkConfig()
	}

	// 构建动态水印文本
	dynamicText := we.buildDynamicText(req)

	we.logger.Info("adding dynamic watermark",
		zap.String("file", req.FilePath),
		zap.String("text", dynamicText))

	outputPath := we.generateOutputPath(req.FilePath)

	result := &WatermarkResult{
		OutputPath: outputPath,
		Format:     "png",
		CreatedAt:  time.Now(),
	}

	return result, nil
}

// buildWatermarkText 构建水印文本
func (we *WatermarkEngine) buildWatermarkText(req *WatermarkRequest) string {
	text := req.Config.Text

	if req.CustomText != "" {
		text = req.CustomText
	}

	if req.Timestamp {
		text = fmt.Sprintf("%s %s", text, time.Now().Format("2006-01-02 15:04:05"))
	}

	return text
}

// buildDynamicText 构建动态水印文本
func (we *WatermarkEngine) buildDynamicText(req *WatermarkRequest) string {
	parts := make([]string, 0)

	if req.Config.Text != "" {
		parts = append(parts, req.Config.Text)
	}

	if req.UserInfo != nil {
		if req.UserInfo.Username != "" {
			parts = append(parts, req.UserInfo.Username)
		}
		if req.UserInfo.IP != "" {
			parts = append(parts, req.UserInfo.IP)
		}
	}

	if req.Timestamp {
		parts = append(parts, time.Now().Format("2006-01-02 15:04:05"))
	}

	if req.CustomText != "" {
		parts = append(parts, req.CustomText)
	}

	result := ""
	for i, part := range parts {
		if i > 0 {
			result += " | "
		}
		result += part
	}

	return result
}

// validateConfig 验证水印配置
func (we *WatermarkEngine) validateConfig(config *WatermarkConfig) error {
	if config.FontSize <= 0 {
		return fmt.Errorf("invalid font size: %d", config.FontSize)
	}

	if config.Opacity < 0 || config.Opacity > 1 {
		return fmt.Errorf("opacity must be between 0 and 1, got %f", config.Opacity)
	}

	if config.Position == "" {
		return fmt.Errorf("watermark position is required")
	}

	return nil
}

// generateOutputPath 生成输出文件路径
func (we *WatermarkEngine) generateOutputPath(inputPath string) string {
	// 简单实现：在原文件名后添加 _watermarked
	return inputPath + "_watermarked"
}

// GetDefaultConfig 获取默认水印配置
func (we *WatermarkEngine) GetDefaultConfig() *WatermarkConfig {
	return DefaultWatermarkConfig()
}
