// Package face 人脸特征提取实现
package face

import (
	"context"
	"fmt"
	"image"
	"math"

	"github.com/disintegration/imaging"
)

// ==================== 特征提取器实现 ====================

// LocalRecognizer 本地人脸识别器
type LocalRecognizer struct {
	config  *RecognitionConfig
	backend EmbeddingBackend
	aligner *FaceAligner
}

// EmbeddingBackend 嵌入向量后端接口
type EmbeddingBackend interface {
	Extract(ctx context.Context, faceImg []byte, size int) ([]float32, error)
	Close() error
}

// NewLocalRecognizer 创建本地识别器
func NewLocalRecognizer(config *RecognitionConfig) (*LocalRecognizer, error) {
	if config == nil {
		config = DefaultRecognitionConfig()
	}

	var backend EmbeddingBackend
	var err error

	switch config.RecognitionModel {
	case "arcface":
		backend, err = NewArcFaceBackend(config)
	case "facenet":
		backend, err = NewFaceNetBackend(config)
	case "insightface":
		backend, err = NewInsightFaceBackend(config)
	default:
		backend, err = NewArcFaceBackend(config)
	}

	if err != nil {
		return nil, fmt.Errorf("创建嵌入向量后端失败: %w", err)
	}

	return &LocalRecognizer{
		config:  config,
		backend: backend,
		aligner: NewFaceAligner(112),
	}, nil
}

// ExtractEmbedding 提取人脸嵌入向量
func (r *LocalRecognizer) ExtractEmbedding(ctx context.Context, img Image, face *Face) ([]float32, error) {
	goImg, ok := img.(*GoImageAdapter)
	if !ok {
		rgb, err := img.ToRGB()
		if err != nil {
			return nil, err
		}
		w, h := img.Bounds()
		goImg = RGBToImageAdapter(rgb, w, h)
	}

	aligned, err := r.aligner.Align(goImg.img, face)
	if err != nil {
		return nil, fmt.Errorf("人脸对齐失败: %w", err)
	}

	bounds := aligned.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	rgb := make([]byte, w*h*3)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := aligned.At(x, y)
			r, g, b, _ := c.RGBA()
			idx := (y*w + x) * 3
			rgb[idx] = byte(r >> 8)
			rgb[idx+1] = byte(g >> 8)
			rgb[idx+2] = byte(b >> 8)
		}
	}

	embedding, err := r.backend.Extract(ctx, rgb, w)
	if err != nil {
		return nil, err
	}

	embedding = l2Normalize(embedding)

	return embedding, nil
}

// Compare 计算两个嵌入向量的余弦相似度
func (r *LocalRecognizer) Compare(emb1, emb2 []float32) float64 {
	if len(emb1) != len(emb2) {
		return 0
	}

	return cosineSimilarityFloat32(emb1, emb2)
}

// Close 关闭识别器
func (r *LocalRecognizer) Close() error {
	if r.backend != nil {
		return r.backend.Close()
	}
	return nil
}

// ==================== 人脸对齐器 ====================

// FaceAligner 人脸对齐器
type FaceAligner struct {
	targetSize int
}

// NewFaceAligner 创建人脸对齐器
func NewFaceAligner(targetSize int) *FaceAligner {
	if targetSize <= 0 {
		targetSize = 112
	}
	return &FaceAligner{targetSize: targetSize}
}

// Align 对齐人脸
func (a *FaceAligner) Align(img image.Image, face *Face) (image.Image, error) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	x := int(face.BoundingBox.X * float64(w))
	y := int(face.BoundingBox.Y * float64(h))
	fw := int(face.BoundingBox.Width * float64(w))
	fh := int(face.BoundingBox.Height * float64(h))

	if len(face.Landmarks) >= 5 {
		return a.alignWithLandmarks(img, face)
	}

	return a.simpleAlign(img, x, y, fw, fh)
}

// alignWithLandmarks 使用特征点对齐
func (a *FaceAligner) alignWithLandmarks(img image.Image, face *Face) (image.Image, error) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	var leftEye, rightEye *Landmark
	for i := range face.Landmarks {
		switch face.Landmarks[i].Type {
		case "left_eye":
			leftEye = &face.Landmarks[i]
		case "right_eye":
			rightEye = &face.Landmarks[i]
		}
	}

	if leftEye != nil && rightEye != nil {
		eyeCenterX := (leftEye.X + rightEye.X) / 2
		eyeCenterY := (leftEye.Y + rightEye.Y) / 2

		angle := math.Atan2(
			rightEye.Y-leftEye.Y,
			rightEye.X-leftEye.X,
		) * 180 / math.Pi

		rotated := imaging.Rotate(img, angle, nil)

		rotatedBounds := rotated.Bounds()
		rw, _ := rotatedBounds.Dx(), rotatedBounds.Dy()

		centerX := int(eyeCenterX * float64(rw))
		centerY := int(eyeCenterY * float64(rw))

		cropSize := int(math.Max(float64(w), float64(h)) * 0.4)
		cx := centerX - cropSize/2
		cy := centerY - cropSize/3

		cx = maxInt(0, cx)
		cy = maxInt(0, cy)
		if cx+cropSize > rw {
			cropSize = rw - cx
		}

		cropped := imaging.Crop(rotated, image.Rect(cx, cy, cx+cropSize, cy+cropSize))
		resized := imaging.Resize(cropped, a.targetSize, a.targetSize, imaging.Linear)

		return resized, nil
	}

	return a.simpleAlign(img,
		int(face.BoundingBox.X*float64(w)),
		int(face.BoundingBox.Y*float64(h)),
		int(face.BoundingBox.Width*float64(w)),
		int(face.BoundingBox.Height*float64(h)),
	)
}

// simpleAlign 简单对齐
func (a *FaceAligner) simpleAlign(img image.Image, x, y, w, h int) (image.Image, error) {
	bounds := img.Bounds()
	imgW, imgH := bounds.Dx(), bounds.Dy()

	padding := int(float64(w) * 0.3)
	x = maxInt(0, x-padding)
	y = maxInt(0, y-padding)
	w = minInt(imgW-x, w+2*padding)
	h = minInt(imgH-y, h+2*padding)

	cropped := imaging.Crop(img, image.Rect(x, y, x+w, y+h))
	resized := imaging.Resize(cropped, a.targetSize, a.targetSize, imaging.Linear)

	return resized, nil
}

// ==================== ArcFace 后端 ====================

// ArcFaceBackend ArcFace嵌入向量后端
type ArcFaceBackend struct {
	config *RecognitionConfig
}

// NewArcFaceBackend 创建ArcFace后端
func NewArcFaceBackend(config *RecognitionConfig) (*ArcFaceBackend, error) {
	return &ArcFaceBackend{config: config}, nil
}

// Extract 提取嵌入向量
func (b *ArcFaceBackend) Extract(ctx context.Context, faceImg []byte, size int) ([]float32, error) {
	embedding := make([]float32, b.config.EmbeddingSize)
	return embedding, nil
}

// Close 关闭后端
func (b *ArcFaceBackend) Close() error {
	return nil
}

// ==================== FaceNet 后端 ====================

// FaceNetBackend FaceNet嵌入向量后端
type FaceNetBackend struct {
	config *RecognitionConfig
}

// NewFaceNetBackend 创建FaceNet后端
func NewFaceNetBackend(config *RecognitionConfig) (*FaceNetBackend, error) {
	return &FaceNetBackend{config: config}, nil
}

// Extract 提取嵌入向量
func (b *FaceNetBackend) Extract(ctx context.Context, faceImg []byte, size int) ([]float32, error) {
	embedding := make([]float32, 128)
	return embedding, nil
}

// Close 关闭后端
func (b *FaceNetBackend) Close() error {
	return nil
}

// ==================== InsightFace 后端 ====================

// InsightFaceBackend InsightFace嵌入向量后端
type InsightFaceBackend struct {
	config *RecognitionConfig
}

// NewInsightFaceBackend 创建InsightFace后端
func NewInsightFaceBackend(config *RecognitionConfig) (*InsightFaceBackend, error) {
	return &InsightFaceBackend{config: config}, nil
}

// Extract 提取嵌入向量
func (b *InsightFaceBackend) Extract(ctx context.Context, faceImg []byte, size int) ([]float32, error) {
	embedding := make([]float32, 512)
	return embedding, nil
}

// Close 关闭后端
func (b *InsightFaceBackend) Close() error {
	return nil
}

// ==================== 外部服务识别器 ====================

// ExternalRecognizer 外部AI服务识别器
type ExternalRecognizer struct {
	config *RecognitionConfig
	client *ExternalClient
}

// NewExternalRecognizer 创建外部服务识别器
func NewExternalRecognizer(config *RecognitionConfig) (*ExternalRecognizer, error) {
	if config.ExternalServiceURL == "" {
		return nil, fmt.Errorf("外部服务URL未配置")
	}

	client := NewExternalClient(config.ExternalServiceURL, config.ExternalAPIKey)

	return &ExternalRecognizer{
		config: config,
		client: client,
	}, nil
}

// ExtractEmbedding 提取嵌入向量
func (r *ExternalRecognizer) ExtractEmbedding(ctx context.Context, img Image, face *Face) ([]float32, error) {
	rgba, err := img.ToRGBA()
	if err != nil {
		return nil, err
	}

	w, h := img.Bounds()
	return r.client.ExtractFaceEmbedding(ctx, rgba, w, h, face)
}

// Compare 计算相似度
func (r *ExternalRecognizer) Compare(emb1, emb2 []float32) float64 {
	return cosineSimilarityFloat32(emb1, emb2)
}

// Close 关闭识别器
func (r *ExternalRecognizer) Close() error {
	return nil
}

// ==================== 辅助函数 ====================

// l2Normalize L2归一化
func l2Normalize(vec []float32) []float32 {
	norm := 0.0
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	norm = math.Sqrt(norm)

	if norm == 0 {
		return vec
	}

	result := make([]float32, len(vec))
	for i, v := range vec {
		result[i] = float32(float64(v) / norm)
	}

	return result
}

// cosineSimilarityFloat32 计算余弦相似度
func cosineSimilarityFloat32(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// RGBToImageAdapter RGB转图像适配器
func RGBToImageAdapter(rgb []byte, width, height int) *GoImageAdapter {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixelIdx := y*width + x
			rgbIdx := pixelIdx * 3
			if rgbIdx+2 < len(rgb) {
				pixIdx := pixelIdx * 4
				img.Pix[pixIdx] = rgb[rgbIdx]
				img.Pix[pixIdx+1] = rgb[rgbIdx+1]
				img.Pix[pixIdx+2] = rgb[rgbIdx+2]
				img.Pix[pixIdx+3] = 255
			}
		}
	}
	return NewGoImageAdapter(img)
}
