package aiphoto

import (
	"context"
	"fmt"
	"image"
	"math"
)

// SmartCropper 智能裁剪器
type SmartCropper struct {
	opts *SmartCropOptions
}

// NewSmartCropper 创建智能裁剪器
func NewSmartCropper(opts *SmartCropOptions) *SmartCropper {
	if opts == nil {
		opts = DefaultSmartCropOptions()
	}
	if opts.Strategy == "" {
		opts.Strategy = "entropy"
	}
	if opts.PaddingRatio == 0 {
		opts.PaddingRatio = 0.05
	}
	if opts.MinFaceSize == 0 {
		opts.MinFaceSize = 50
	}
	return &SmartCropper{opts: opts}
}

// CropResult 裁剪结果
type CropResult struct {
	Rect    image.Rectangle `json:"rect"`    // 裁剪区域
	Score   float64         `json:"score"`   // 裁剪质量评分
	Image   image.Image     `json:"-"`       // 裁剪后的图像
}

// SmartCrop 对图像执行智能裁剪
func (sc *SmartCropper) SmartCrop(ctx context.Context, src image.Image) (*CropResult, error) {
	if src == nil {
		return nil, fmt.Errorf("源图像不能为 nil")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	bounds := src.Bounds()
	srcWidth := bounds.Dx()
	srcHeight := bounds.Dy()
	targetW := sc.opts.TargetWidth
	targetH := sc.opts.TargetHeight

	// 如果目标尺寸大于源图，返回错误
	if targetW > srcWidth || targetH > srcHeight {
		return nil, fmt.Errorf("目标尺寸 (%dx%d) 不能大于源图尺寸 (%dx%d)", targetW, targetH, srcWidth, srcHeight)
	}

	var bestRect image.Rectangle
	var bestScore float64

	switch sc.opts.Strategy {
	case "face":
		bestRect, bestScore = sc.faceBasedCrop(src, bounds, targetW, targetH)
	case "center":
		bestRect, bestScore = sc.centerCrop(bounds, targetW, targetH)
	case "attention":
		bestRect, bestScore = sc.attentionCrop(src, bounds, targetW, targetH)
	default: // entropy
		bestRect, bestScore = sc.entropyCrop(src, bounds, targetW, targetH)
	}

	// 应用边距
	if sc.opts.PaddingRatio > 0 {
		bestRect = sc.applyPadding(bestRect, bounds)
	}

	// 执行裁剪
	cropped := src.(interface {
		SubImage(r image.Rectangle) image.Image
	}).SubImage(bestRect)

	return &CropResult{
		Rect:  bestRect,
		Score: bestScore,
		Image: cropped,
	}, nil
}

// entropyCrop 基于信息熵的裁剪
func (sc *SmartCropper) entropyCrop(src image.Image, bounds image.Rectangle, targetW, targetH int) (image.Rectangle, float64) {
	bestRect := image.Rectangle{}
	bestScore := -1.0

	step := maxInt(1, minInt(bounds.Dx(), bounds.Dy())/20)

	for y := bounds.Min.Y; y <= bounds.Max.Y-targetH; y += step {
		for x := bounds.Min.X; x <= bounds.Max.X-targetW; x += step {
			rect := image.Rect(x, y, x+targetW, y+targetH)
			score := calculateEntropy(src, rect)
			if score > bestScore {
				bestScore = score
				bestRect = rect
			}
		}
	}

	// 如果没有找到合适的区域，使用中心裁剪
	if bestScore < 0 {
		bestRect, bestScore = sc.centerCrop(bounds, targetW, targetH)
	}

	return bestRect, bestScore
}

// faceBasedCrop 基于人脸检测的裁剪
func (sc *SmartCropper) faceBasedCrop(src image.Image, bounds image.Rectangle, targetW, targetH int) (image.Rectangle, float64) {
	// 简化实现：检测肤色区域作为人脸近似
	faceRegions := sc.detectSkinRegions(src, bounds)

	if len(faceRegions) > 0 {
		// 找到包含所有人脸的最小区域
		minX, minY := bounds.Max.X, bounds.Max.Y
		maxX, maxY := bounds.Min.X, bounds.Min.Y
		for _, region := range faceRegions {
			if region.Min.X < minX {
				minX = region.Min.X
			}
			if region.Min.Y < minY {
				minY = region.Min.Y
			}
			if region.Max.X > maxX {
				maxX = region.Max.X
			}
			if region.Max.Y > maxY {
				maxY = region.Max.Y
			}
		}

		// 计算裁剪中心点
		centerX := (minX + maxX) / 2
		centerY := (minY + maxY) / 2

		// 调整裁剪区域确保在图像范围内
		cropX := maxInt(bounds.Min.X, centerX-targetW/2)
		cropY := maxInt(bounds.Min.Y, centerY-targetH/2)
		cropX = minInt(cropX, bounds.Max.X-targetW)
		cropY = minInt(cropY, bounds.Max.Y-targetH)

		rect := image.Rect(cropX, cropY, cropX+targetW, cropY+targetH)
		return rect, 0.8 // 人脸检测基础得分
	}

	// 没有检测到人脸，回退到熵裁剪
	return sc.entropyCrop(src, bounds, targetW, targetH)
}

// centerCrop 中心裁剪
func (sc *SmartCropper) centerCrop(bounds image.Rectangle, targetW, targetH int) (image.Rectangle, float64) {
	centerX := bounds.Min.X + bounds.Dx()/2
	centerY := bounds.Min.Y + bounds.Dy()/2

	cropX := centerX - targetW/2
	cropY := centerY - targetH/2

	// 确保在图像范围内
	cropX = maxInt(bounds.Min.X, minInt(cropX, bounds.Max.X-targetW))
	cropY = maxInt(bounds.Min.Y, minInt(cropY, bounds.Max.Y-targetH))

	rect := image.Rect(cropX, cropY, cropX+targetW, cropY+targetH)
	return rect, 0.5 // 中心裁剪基础得分
}

// attentionCrop 基于注意力机制的裁剪
func (sc *SmartCropper) attentionCrop(src image.Image, bounds image.Rectangle, targetW, targetH int) (image.Rectangle, float64) {
	// 计算注意力图：边缘密度 + 色彩饱和度 + 对比度
	attentionMap := sc.calculateAttentionMap(src, bounds)

	// 在注意力图上找到最佳裁剪窗口
	bestRect := image.Rectangle{}
	bestScore := -1.0

	step := maxInt(1, minInt(bounds.Dx(), bounds.Dy())/20)

	for y := bounds.Min.Y; y <= bounds.Max.Y-targetH; y += step {
		for x := bounds.Min.X; x <= bounds.Max.X-targetW; x += step {
			rect := image.Rect(x, y, x+targetW, y+targetH)
			score := sc.regionAttentionScore(attentionMap, rect, bounds)
			if score > bestScore {
				bestScore = score
				bestRect = rect
			}
		}
	}

	if bestScore < 0 {
		bestRect, bestScore = sc.centerCrop(bounds, targetW, targetH)
	}

	return bestRect, bestScore
}

// calculateAttentionMap 计算注意力图
func (sc *SmartCropper) calculateAttentionMap(src image.Image, bounds image.Rectangle) [][]float64 {
	w, h := bounds.Dx(), bounds.Dy()
	attentionMap := make([][]float64, h)
	for i := range attentionMap {
		attentionMap[i] = make([]float64, w)
	}

	for y := bounds.Min.Y + 1; y < bounds.Max.Y-1; y++ {
		for x := bounds.Min.X + 1; x < bounds.Max.X-1; x++ {
			r, g, b, _ := src.At(x, y).RGBA()

			// 计算边缘强度（Sobel 简化）
			rx, gx, bx, _ := src.At(x+1, y).RGBA()
			ry, gy, by, _ := src.At(x, y+1).RGBA()

			edgeR := math.Abs(float64(rx)-float64(r)) + math.Abs(float64(ry)-float64(r))
			edgeG := math.Abs(float64(gx)-float64(g)) + math.Abs(float64(gy)-float64(g))
			edgeB := math.Abs(float64(bx)-float64(b)) + math.Abs(float64(by)-float64(b))

			edgeStrength := (edgeR + edgeG + edgeB) / 3.0

			// 饱和度
			maxC := maxUint32(r, maxUint32(g, b))
			minC := minUint32(r, minUint32(g, b))
			saturation := 0.0
			if maxC > 0 {
				saturation = float64(maxC-minC) / float64(maxC)
			}

			// 组合注意力分数
			yIdx := y - bounds.Min.Y
			xIdx := x - bounds.Min.X
			attentionMap[yIdx][xIdx] = edgeStrength/65535.0*0.5 + saturation*0.5
		}
	}

	return attentionMap
}

// regionAttentionScore 计算区域注意力分数
func (sc *SmartCropper) regionAttentionScore(attentionMap [][]float64, rect image.Rectangle, bounds image.Rectangle) float64 {
	sum := 0.0
	count := 0

	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			yIdx := y - bounds.Min.Y
			xIdx := x - bounds.Min.X
			if yIdx >= 0 && yIdx < len(attentionMap) && xIdx >= 0 && xIdx < len(attentionMap[0]) {
				sum += attentionMap[yIdx][xIdx]
				count++
			}
		}
	}

	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

// detectSkinRegions 检测肤色区域
func (sc *SmartCropper) detectSkinRegions(src image.Image, bounds image.Rectangle) []image.Rectangle {
	regions := []image.Rectangle{}

	for y := bounds.Min.Y; y < bounds.Max.Y; y += 10 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 10 {
			r, g, b, _ := src.At(x, y).RGBA()
			r8 := float64(r >> 8)
			g8 := float64(g >> 8)
			b8 := float64(b >> 8)

			// 简单肤色检测（YCbCr 空间近似）
			if isSkinColor(r8, g8, b8) {
				regions = append(regions, image.Rect(x, y, x+sc.opts.MinFaceSize, y+sc.opts.MinFaceSize))
			}
		}
	}

	return regions
}

// isSkinColor 判断是否为肤色
func isSkinColor(r, g, b float64) bool {
	// RGB 到 YCbCr 近似转换
	y := 0.299*r + 0.587*g + 0.114*b
	cb := 128 - 0.169*r - 0.331*g + 0.500*b
	cr := 128 + 0.500*r - 0.419*g - 0.081*b

	// 肤色范围阈值
	return y > 80 && cb > 85 && cb < 135 && cr > 135 && cr < 180
}

// applyPadding 应用边距
func (sc *SmartCropper) applyPadding(rect image.Rectangle, bounds image.Rectangle) image.Rectangle {
	paddingX := int(float64(rect.Dx()) * sc.opts.PaddingRatio)
	paddingY := int(float64(rect.Dy()) * sc.opts.PaddingRatio)

	newRect := image.Rect(
		maxInt(bounds.Min.X, rect.Min.X-paddingX),
		maxInt(bounds.Min.Y, rect.Min.Y-paddingY),
		minInt(bounds.Max.X, rect.Max.X+paddingX),
		minInt(bounds.Max.Y, rect.Max.Y+paddingY),
	)

	return newRect
}

// calculateEntropy 计算区域信息熵
func calculateEntropy(src image.Image, rect image.Rectangle) float64 {
	// 统计灰度直方图
	var hist [256]int
	totalPixels := 0

	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			r, g, b, _ := src.At(x, y).RGBA()
			gray := (r*299 + g*587 + b*114) / 1000
			hist[gray>>8]++
			totalPixels++
		}
	}

	if totalPixels == 0 {
		return 0
	}

	// 计算熵
	entropy := 0.0
	n := float64(totalPixels)
	for _, count := range hist {
		if count > 0 {
			p := float64(count) / n
			entropy -= p * math.Log2(p)
		}
	}

	return entropy
}

func maxUint32(a, b uint32) uint32 {
	if a > b {
		return a
	}
	return b
}

func minUint32(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}
