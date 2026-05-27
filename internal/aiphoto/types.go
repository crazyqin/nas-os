// Package aiphoto 提供 AI 照片增强功能，包括去噪、超分辨率、修复、智能裁剪
package aiphoto

import (
	"fmt"
	"time"
)

// TaskType 任务类型
type TaskType string

const (
	TaskTypeDenoise  TaskType = "denoise"  // 去噪
	TaskTypeUpscale  TaskType = "upscale"  // 超分辨率
	TaskTypeRestore  TaskType = "restore"  // 修复
	TaskTypeSmartCrop TaskType = "smartcrop" // 智能裁剪
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"    // 等待中
	TaskStatusProcessing TaskStatus = "processing" // 处理中
	TaskStatusCompleted  TaskStatus = "completed"  // 已完成
	TaskStatusFailed     TaskStatus = "failed"     // 失败
	TaskStatusCancelled  TaskStatus = "cancelled"  // 已取消
)

// DenoiseOptions 去噪选项
type DenoiseOptions struct {
	Strength   float64 `json:"strength"`   // 去噪强度 0.0-1.0，默认 0.5
	PreserveDetail bool `json:"preserveDetail"` // 是否保留细节，默认 true
	Algorithm  string  `json:"algorithm"`  // 算法：nlm (Non-Local Means) / bilateral / bm3d，默认 nlm
}

// UpscaleOptions 超分辨率选项
type UpscaleOptions struct {
	ScaleFactor int    `json:"scaleFactor"` // 放大倍数：2/4，默认 2
	Model       string `json:"model"`       // 模型：realesrgan / esrgan / lanczos，默认 realesrgan
	Denoise     bool   `json:"denoise"`     // 放大时是否去噪，默认 false
	Format      string `json:"format"`      // 输出格式：jpg / png / webp，默认同原图
}

// RestoreOptions 修复选项
type RestoreOptions struct {
	RepairScratches  bool    `json:"repairScratches"`  // 修复划痕，默认 true
	RepairStains     bool    `json:"repairStains"`     // 修复污渍，默认 true
	Colorize         bool    `json:"colorize"`         // 黑白照片上色，默认 false
	EnhanceFace      bool    `json:"enhanceFace"`      // 人脸增强，默认 true
	Strength         float64 `json:"strength"`         // 修复强度 0.0-1.0，默认 0.7
	RemoveWatermark  bool    `json:"removeWatermark"`  // 去水印，默认 false
}

// SmartCropOptions 智能裁剪选项
type SmartCropOptions struct {
	TargetWidth  int    `json:"targetWidth"`  // 目标宽度
	TargetHeight int    `json:"targetHeight"` // 目标高度
	Strategy     string `json:"strategy"`     // 策略：entropy / face / center / attention，默认 entropy
	PaddingRatio float64 `json:"paddingRatio"` // 边距比例 0.0-0.3，默认 0.05
	MinFaceSize  int    `json:"minFaceSize"`  // 最小人脸尺寸（像素），默认 50
}

// PhotoTask 照片处理任务
type PhotoTask struct {
	ID          string           `json:"id"`
	Type        TaskType         `json:"type"`
	Status      TaskStatus       `json:"status"`
	InputPath   string           `json:"inputPath"`
	OutputPath  string           `json:"outputPath"`
	Options     interface{}      `json:"options"`
	Progress    float64          `json:"progress"` // 0-100
	Error       string           `json:"error,omitempty"`
	CreatedAt   time.Time        `json:"createdAt"`
	StartedAt   *time.Time       `json:"startedAt,omitempty"`
	CompletedAt *time.Time       `json:"completedAt,omitempty"`
	Duration    time.Duration    `json:"duration,omitempty"`
	Metadata    *PhotoMetadata   `json:"metadata,omitempty"`
}

// PhotoMetadata 照片元数据
type PhotoMetadata struct {
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Format     string `json:"format"`
	Size       int64  `json:"size"`
	HasAlpha   bool   `json:"hasAlpha"`
	ColorSpace string `json:"colorSpace"`
}

// ProcessResult 处理结果
type ProcessResult struct {
	TaskID       string         `json:"taskId"`
	Success      bool           `json:"success"`
	OutputPath   string         `json:"outputPath"`
	InputMeta    *PhotoMetadata `json:"inputMeta"`
	OutputMeta   *PhotoMetadata `json:"outputMeta"`
	Duration     time.Duration  `json:"duration"`
	Improvements map[string]float64 `json:"improvements,omitempty"` // 指标改善
}

// ValidationError 参数校验错误
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("参数校验失败 [%s]: %s", e.Field, e.Message)
}

// Validate 校验 DenoiseOptions
func (o *DenoiseOptions) Validate() error {
	if o.Strength < 0 || o.Strength > 1 {
		return &ValidationError{Field: "strength", Message: "必须在 0.0-1.0 之间"}
	}
	if o.Algorithm != "" && o.Algorithm != "nlm" && o.Algorithm != "bilateral" && o.Algorithm != "bm3d" {
		return &ValidationError{Field: "algorithm", Message: "必须是 nlm / bilateral / bm3d"}
	}
	return nil
}

// Validate 校验 UpscaleOptions
func (o *UpscaleOptions) Validate() error {
	if o.ScaleFactor != 0 && o.ScaleFactor != 2 && o.ScaleFactor != 4 {
		return &ValidationError{Field: "scaleFactor", Message: "必须是 2 或 4"}
	}
	if o.Model != "" && o.Model != "realesrgan" && o.Model != "esrgan" && o.Model != "lanczos" {
		return &ValidationError{Field: "model", Message: "必须是 realesrgan / esrgan / lanczos"}
	}
	if o.Format != "" && o.Format != "jpg" && o.Format != "png" && o.Format != "webp" {
		return &ValidationError{Field: "format", Message: "必须是 jpg / png / webp"}
	}
	return nil
}

// Validate 校验 RestoreOptions
func (o *RestoreOptions) Validate() error {
	if o.Strength < 0 || o.Strength > 1 {
		return &ValidationError{Field: "strength", Message: "必须在 0.0-1.0 之间"}
	}
	return nil
}

// Validate 校验 SmartCropOptions
func (o *SmartCropOptions) Validate() error {
	if o.TargetWidth <= 0 {
		return &ValidationError{Field: "targetWidth", Message: "必须大于 0"}
	}
	if o.TargetHeight <= 0 {
		return &ValidationError{Field: "targetHeight", Message: "必须大于 0"}
	}
	if o.Strategy != "" && o.Strategy != "entropy" && o.Strategy != "face" && o.Strategy != "center" && o.Strategy != "attention" {
		return &ValidationError{Field: "strategy", Message: "必须是 entropy / face / center / attention"}
	}
	if o.PaddingRatio < 0 || o.PaddingRatio > 0.3 {
		return &ValidationError{Field: "paddingRatio", Message: "必须在 0.0-0.3 之间"}
	}
	return nil
}

// DefaultDenoiseOptions 默认去噪选项
func DefaultDenoiseOptions() *DenoiseOptions {
	return &DenoiseOptions{
		Strength:       0.5,
		PreserveDetail: true,
		Algorithm:      "nlm",
	}
}

// DefaultUpscaleOptions 默认超分辨率选项
func DefaultUpscaleOptions() *UpscaleOptions {
	return &UpscaleOptions{
		ScaleFactor: 2,
		Model:       "realesrgan",
		Denoise:     false,
	}
}

// DefaultRestoreOptions 默认修复选项
func DefaultRestoreOptions() *RestoreOptions {
	return &RestoreOptions{
		RepairScratches: true,
		RepairStains:    true,
		Colorize:        false,
		EnhanceFace:     true,
		Strength:        0.7,
		RemoveWatermark: false,
	}
}

// DefaultSmartCropOptions 默认智能裁剪选项
func DefaultSmartCropOptions() *SmartCropOptions {
	return &SmartCropOptions{
		TargetWidth:  1024,
		TargetHeight: 768,
		Strategy:     "entropy",
		PaddingRatio: 0.05,
		MinFaceSize:  50,
	}
}
