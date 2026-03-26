// Package face 人脸检测实现
package face

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"sort"
	"sync"
	"time"

	"github.com/disintegration/imaging"
)

// ==================== 检测器实现 ====================

// LocalDetector 本地人脸检测器
type LocalDetector struct {
	config  *RecognitionConfig
	backend DetectionBackend
	mu      sync.RWMutex
}

// DetectionBackend 检测后端接口
type DetectionBackend interface {
	Detect(ctx context.Context, rgb []byte, width, height int) ([]Face, error)
	Close() error
}

// NewLocalDetector 创建本地检测器
func NewLocalDetector(config *RecognitionConfig) (*LocalDetector, error) {
	if config == nil {
		config = DefaultRecognitionConfig()
	}

	var backend DetectionBackend
	var err error

	switch config.DetectionModel {
	case "retinaface":
		backend, err = NewRetinaFaceBackend(config)
	case "mtcnn":
		backend, err = NewMTCNNBackend(config)
	case "hog":
		backend, err = NewHOGBackend(config)
	default:
		backend, err = NewHOGBackend(config)
	}

	if err != nil {
		return nil, fmt.Errorf("创建检测后端失败: %w", err)
	}

	return &LocalDetector{
		config:  config,
		backend: backend,
	}, nil
}

// Detect 检测人脸
func (d *LocalDetector) Detect(ctx context.Context, img Image) (*DetectionResult, error) {
	start := time.Now()

	width, height := img.Bounds()
	rgb, err := img.ToRGB()
	if err != nil {
		return nil, NewError(ErrCodeInvalidImage, "图像转换失败", err.Error())
	}

	faces, err := d.backend.Detect(ctx, rgb, width, height)
	if err != nil {
		return nil, err
	}

	filtered := d.filterFaces(faces, width, height)

	if len(filtered) > d.config.MaxFacesPerPhoto {
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].Quality > filtered[j].Quality
		})
		filtered = filtered[:d.config.MaxFacesPerPhoto]
	}

	for i := range filtered {
		if filtered[i].ID == "" {
			filtered[i].ID = generateID("face")
		}
		filtered[i].CreatedAt = time.Now()
	}

	return &DetectionResult{
		Faces:     filtered,
		Width:     width,
		Height:    height,
		ProcessMs: time.Since(start).Milliseconds(),
	}, nil
}

// filterFaces 过滤低质量人脸
func (d *LocalDetector) filterFaces(faces []Face, imgWidth, imgHeight int) []Face {
	filtered := make([]Face, 0, len(faces))

	for _, face := range faces {
		if face.Confidence < d.config.ConfidenceThresh {
			continue
		}

		faceWidth := int(face.BoundingBox.Width * float64(imgWidth))
		faceHeight := int(face.BoundingBox.Height * float64(imgHeight))

		if faceWidth < d.config.MinFaceSize || faceHeight < d.config.MinFaceSize {
			continue
		}

		if face.BoundingBox.X < 0 || face.BoundingBox.Y < 0 {
			continue
		}
		if face.BoundingBox.X+face.BoundingBox.Width > 1.0 {
			continue
		}
		if face.BoundingBox.Y+face.BoundingBox.Height > 1.0 {
			continue
		}

		aspect := face.BoundingBox.Width / face.BoundingBox.Height
		if aspect < 0.5 || aspect > 2.0 {
			continue
		}

		filtered = append(filtered, face)
	}

	return filtered
}

// Close 关闭检测器
func (d *LocalDetector) Close() error {
	if d.backend != nil {
		return d.backend.Close()
	}
	return nil
}

// ==================== HOG 后端实现 ====================

// HOGBackend HOG特征人脸检测后端
type HOGBackend struct {
	config *RecognitionConfig
}

// NewHOGBackend 创建HOG后端
func NewHOGBackend(config *RecognitionConfig) (*HOGBackend, error) {
	return &HOGBackend{config: config}, nil
}

// Detect 使用HOG特征检测人脸
func (b *HOGBackend) Detect(ctx context.Context, rgb []byte, width, height int) ([]Face, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	gray := rgbToGray(rgb, width, height)
	faces := b.detectMultiScale(gray, width, height)

	return faces, nil
}

// detectMultiScale 多尺度检测
func (b *HOGBackend) detectMultiScale(gray []uint8, width, height int) []Face {
	faces := make([]Face, 0)

	scales := []float64{1.0, 0.75, 0.5, 0.25}
	for _, scale := range scales {
		scaledWidth := int(float64(width) * scale)
		scaledHeight := int(float64(height) * scale)

		if scaledWidth < 64 || scaledHeight < 64 {
			continue
		}

		windowSize := int(float64(b.config.MinFaceSize) / scale)
		if windowSize < 24 {
			windowSize = 24
		}

		step := windowSize / 4
		for y := 0; y < scaledHeight-windowSize; y += step {
			for x := 0; x < scaledWidth-windowSize; x += step {
				window := extractWindow(gray, width, height, x, y, windowSize, windowSize)
				if window == nil {
					continue
				}

				confidence := b.classifyWindow(window, windowSize)

				if confidence >= b.config.ConfidenceThresh {
					face := Face{
						BoundingBox: BoundingBox{
							X:      float64(x) / float64(scaledWidth),
							Y:      float64(y) / float64(scaledHeight),
							Width:  float64(windowSize) / float64(scaledWidth),
							Height: float64(windowSize) / float64(scaledHeight),
						},
						Confidence: confidence,
						Quality:    confidence,
					}
					faces = append(faces, face)
				}
			}
		}
	}

	return nms(faces, 0.3)
}

// classifyWindow 分类窗口
func (b *HOGBackend) classifyWindow(window []uint8, size int) float64 {
	variance := computeVariance(window)
	if variance < 500 {
		return 0
	}

	eyeRegionY := size * 2 / 7
	eyeRegionH := size / 5
	eyeScore := b.detectEyePattern(window, size, eyeRegionY, eyeRegionH)

	noseScore := b.detectNosePattern(window, size)

	score := (eyeScore*0.6 + noseScore*0.4)
	return score
}

// detectEyePattern 检测眼睛模式
func (b *HOGBackend) detectEyePattern(window []uint8, size, regionY, regionH int) float64 {
	sum := 0.0
	for y := regionY; y < regionY+regionH && y < size; y++ {
		rowSum := 0
		for x := size / 4; x < size*3/4 && x < size; x++ {
			rowSum += int(window[y*size+x])
		}
		avg := float64(rowSum) / float64(size/2)
		if avg < 128 {
			sum += 0.5
		}
	}
	if regionH == 0 {
		return 0
	}
	return sum / float64(regionH)
}

// detectNosePattern 检测鼻子模式
func (b *HOGBackend) detectNosePattern(window []uint8, size int) float64 {
	centerX := size / 2
	centerY := size / 2
	regionSize := size / 6

	sum := 0
	count := 0
	for y := centerY - regionSize; y < centerY+regionSize && y < size; y++ {
		for x := centerX - regionSize/2; x < centerX+regionSize/2 && x < size; x++ {
			if y >= 0 && x >= 0 {
				sum += int(window[y*size+x])
				count++
			}
		}
	}

	if count == 0 {
		return 0
	}

	avg := float64(sum) / float64(count)
	if avg > 100 {
		return 0.8
	}
	return 0.4
}

// Close 关闭后端
func (b *HOGBackend) Close() error {
	return nil
}

// ==================== RetinaFace 后端 ====================

// RetinaFaceBackend RetinaFace检测后端
type RetinaFaceBackend struct {
	config *RecognitionConfig
}

// NewRetinaFaceBackend 创建RetinaFace后端
func NewRetinaFaceBackend(config *RecognitionConfig) (*RetinaFaceBackend, error) {
	return &RetinaFaceBackend{config: config}, nil
}

// Detect 检测人脸
func (b *RetinaFaceBackend) Detect(ctx context.Context, rgb []byte, width, height int) ([]Face, error) {
	return []Face{}, nil
}

// Close 关闭后端
func (b *RetinaFaceBackend) Close() error {
	return nil
}

// ==================== MTCNN 后端 ====================

// MTCNNBackend MTCNN检测后端
type MTCNNBackend struct {
	config *RecognitionConfig
}

// NewMTCNNBackend 创建MTCNN后端
func NewMTCNNBackend(config *RecognitionConfig) (*MTCNNBackend, error) {
	return &MTCNNBackend{config: config}, nil
}

// Detect 检测人脸
func (b *MTCNNBackend) Detect(ctx context.Context, rgb []byte, width, height int) ([]Face, error) {
	return []Face{}, nil
}

// Close 关闭后端
func (b *MTCNNBackend) Close() error {
	return nil
}

// ==================== 外部服务检测器 ====================

// ExternalDetector 外部AI服务检测器
type ExternalDetector struct {
	config *RecognitionConfig
	client *ExternalClient
}

// NewExternalDetector 创建外部服务检测器
func NewExternalDetector(config *RecognitionConfig) (*ExternalDetector, error) {
	if config.ExternalServiceURL == "" {
		return nil, fmt.Errorf("外部服务URL未配置")
	}

	client := NewExternalClient(config.ExternalServiceURL, config.ExternalAPIKey)

	return &ExternalDetector{
		config: config,
		client: client,
	}, nil
}

// Detect 调用外部服务检测人脸
func (d *ExternalDetector) Detect(ctx context.Context, img Image) (*DetectionResult, error) {
	start := time.Now()

	rgba, err := img.ToRGBA()
	if err != nil {
		return nil, err
	}

	width, height := img.Bounds()

	faces, err := d.client.DetectFaces(ctx, rgba, width, height)
	if err != nil {
		return nil, err
	}

	return &DetectionResult{
		Faces:     faces,
		Width:     width,
		Height:    height,
		ProcessMs: time.Since(start).Milliseconds(),
	}, nil
}

// Close 关闭检测器
func (d *ExternalDetector) Close() error {
	return nil
}

// ==================== 图像适配器 ====================

// GoImageAdapter Go标准图像适配器
type GoImageAdapter struct {
	img image.Image
}

// NewGoImageAdapter 创建Go图像适配器
func NewGoImageAdapter(img image.Image) *GoImageAdapter {
	return &GoImageAdapter{img: img}
}

// Bounds 返回边界
func (a *GoImageAdapter) Bounds() (int, int) {
	b := a.img.Bounds()
	return b.Dx(), b.Dy()
}

// ToRGB 转换为RGB
func (a *GoImageAdapter) ToRGB() ([]byte, error) {
	b := a.img.Bounds()
	w, h := b.Dx(), b.Dy()
	rgb := make([]byte, w*h*3)

	idx := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c, ok := color.NRGBAModel.Convert(a.img.At(x, y)).(color.NRGBA)
			if !ok {
				continue
			}
			rgb[idx] = c.R
			rgb[idx+1] = c.G
			rgb[idx+2] = c.B
			idx += 3
		}
	}

	return rgb, nil
}

// ToRGBA 转换为RGBA
func (a *GoImageAdapter) ToRGBA() ([]byte, error) {
	b := a.img.Bounds()
	w, h := b.Dx(), b.Dy()
	rgba := make([]byte, w*h*4)

	idx := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c, ok := color.NRGBAModel.Convert(a.img.At(x, y)).(color.NRGBA)
			if !ok {
				continue
			}
			rgba[idx] = c.R
			rgba[idx+1] = c.G
			rgba[idx+2] = c.B
			rgba[idx+3] = c.A
			idx += 4
		}
	}

	return rgba, nil
}

// GetPixel 获取像素
func (a *GoImageAdapter) GetPixel(x, y int) (r, g, b, alpha uint8) {
	c, ok := color.NRGBAModel.Convert(a.img.At(x, y)).(color.NRGBA)
	if !ok {
		return 0, 0, 0, 0
	}
	return c.R, c.G, c.B, c.A
}

// ==================== 辅助函数 ====================

// rgbToGray RGB转灰度
func rgbToGray(rgb []byte, width, height int) []uint8 {
	gray := make([]uint8, width*height)
	for i := 0; i < width*height; i++ {
		r := float64(rgb[i*3])
		g := float64(rgb[i*3+1])
		b := float64(rgb[i*3+2])
		gray[i] = uint8(0.299*r + 0.587*g + 0.114*b)
	}
	return gray
}

// extractWindow 提取窗口
func extractWindow(gray []uint8, width, height, x, y, w, h int) []uint8 {
	if x+w > width || y+h > height {
		return nil
	}

	window := make([]uint8, w*h)
	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			window[dy*w+dx] = gray[(y+dy)*width+(x+dx)]
		}
	}
	return window
}

// computeVariance 计算方差
func computeVariance(data []uint8) float64 {
	n := len(data)
	if n == 0 {
		return 0
	}

	sum := 0.0
	for _, v := range data {
		sum += float64(v)
	}
	mean := sum / float64(n)

	variance := 0.0
	for _, v := range data {
		diff := float64(v) - mean
		variance += diff * diff
	}

	return variance / float64(n)
}

// nms 非极大值抑制
func nms(faces []Face, thresh float64) []Face {
	if len(faces) == 0 {
		return faces
	}

	sort.Slice(faces, func(i, j int) bool {
		return faces[i].Confidence > faces[j].Confidence
	})

	keep := make([]Face, 0, len(faces))
	used := make([]bool, len(faces))

	for i := 0; i < len(faces); i++ {
		if used[i] {
			continue
		}

		keep = append(keep, faces[i])

		for j := i + 1; j < len(faces); j++ {
			if used[j] {
				continue
			}

			iou := computeIOU(faces[i].BoundingBox, faces[j].BoundingBox)
			if iou > thresh {
				used[j] = true
			}
		}
	}

	return keep
}

// computeIOU 计算IoU
func computeIOU(a, b BoundingBox) float64 {
	ax1, ay1 := a.X, a.Y
	ax2, ay2 := a.X+a.Width, a.Y+a.Height
	bx1, by1 := b.X, b.Y
	bx2, by2 := b.X+b.Width, b.Y+b.Height

	ix1 := max(ax1, bx1)
	iy1 := max(ay1, by1)
	ix2 := min(ax2, bx2)
	iy2 := min(ay2, by2)

	if ix2 <= ix1 || iy2 <= iy1 {
		return 0
	}

	inter := (ix2 - ix1) * (iy2 - iy1)

	areaA := a.Width * a.Height
	areaB := b.Width * b.Height
	union := areaA + areaB - inter

	return inter / union
}

// cropFace 裁剪人脸
func cropFace(img image.Image, face *Face, padding float64) image.Image {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	x := int(face.BoundingBox.X * float64(w))
	y := int(face.BoundingBox.Y * float64(h))
	fw := int(face.BoundingBox.Width * float64(w))
	fh := int(face.BoundingBox.Height * float64(h))

	p := int(float64(fw) * padding)
	x = maxInt(0, x-p)
	y = maxInt(0, y-p)
	fw = minInt(w-x, fw+2*p)
	fh = minInt(h-y, fh+2*p)

	cropped := imaging.Crop(img, image.Rect(x, y, x+fw, y+fh))
	return cropped
}

func generateID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
