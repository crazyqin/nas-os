// Package face 提供人脸识别功能
// 支持人脸检测、特征提取、聚类和标签管理
package face

import (
	"context"
	"time"
)

// ==================== 核心类型定义 ====================

// Face 人脸检测结果
type Face struct {
	ID          string       `json:"id"`
	PhotoID     string       `json:"photoId"`
	BoundingBox BoundingBox  `json:"boundingBox"`
	Landmarks   []Landmark   `json:"landmarks,omitempty"`
	Embedding   []float32    `json:"embedding,omitempty"`
	Confidence  float64      `json:"confidence"` // 检测置信度 0-1
	Quality     float64      `json:"quality"`    // 人脸质量评分 0-1
	BlurScore   float64      `json:"blurScore,omitempty"`
	PersonID    string       `json:"personId,omitempty"`
	PersonName  string       `json:"personName,omitempty"`
	ClusterID   int          `json:"clusterId,omitempty"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

// BoundingBox 人脸边界框 (归一化坐标 0-1)
type BoundingBox struct {
	X      float64 `json:"x"`      // 左上角 X
	Y      float64 `json:"y"`      // 左上角 Y
	Width  float64 `json:"width"`  // 宽度
	Height float64 `json:"height"` // 高度
}

// Landmark 面部特征点
type Landmark struct {
	Type string  `json:"type"` // left_eye, right_eye, nose, left_mouth, right_mouth, chin
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
}

// Person 人物实体
type Person struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	FaceCount          int       `json:"faceCount"`
	CoverFaceID        string    `json:"coverFaceId,omitempty"`
	CoverPhotoID       string    `json:"coverPhotoId,omitempty"`
	RepresentativeFace string    `json:"representativeFace,omitempty"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

// ClusterResult 聚类结果
type ClusterResult struct {
	Persons      []Person `json:"persons"`
	Unassigned   []Face   `json:"unassigned"`
	ClusterCount int      `json:"clusterCount"`
}

// DetectionResult 检测结果
type DetectionResult struct {
	Faces     []Face `json:"faces"`
	PhotoID   string `json:"photoId"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	ProcessMs int64  `json:"processMs"`
}

// RecognitionConfig 人脸识别配置
type RecognitionConfig struct {
	// 检测配置
	MinFaceSize      int     `json:"minFaceSize"`      // 最小人脸尺寸(像素)
	MaxFacesPerPhoto int     `json:"maxFacesPerPhoto"` // 每张照片最大人脸数
	DetectionModel   string  `json:"detectionModel"`   // retinaface, mtcnn, hog

	// 识别配置
	RecognitionModel   string  `json:"recognitionModel"`   // arcface, facenet, insightface
	EmbeddingSize      int     `json:"embeddingSize"`      // 嵌入向量维度
	ConfidenceThresh   float64 `json:"confidenceThresh"`   // 检测置信度阈值
	ClusterThresh      float64 `json:"clusterThresh"`      // 聚类相似度阈值
	MinClusterSize     int     `json:"minClusterSize"`     // 最小聚类大小

	// 外部服务配置
	ExternalServiceURL string `json:"externalServiceUrl,omitempty"`
	ExternalAPIKey     string `json:"externalApiKey,omitempty"`
	UseExternalService bool   `json:"useExternalService"`

	// 性能配置
	UseGPU     bool `json:"useGPU"`
	BatchSize  int  `json:"batchSize"`
	NumWorkers int  `json:"numWorkers"`
}

// DefaultRecognitionConfig 默认配置
func DefaultRecognitionConfig() *RecognitionConfig {
	return &RecognitionConfig{
		MinFaceSize:        30,
		MaxFacesPerPhoto:   50,
		DetectionModel:     "retinaface",
		RecognitionModel:   "arcface",
		EmbeddingSize:      512,
		ConfidenceThresh:   0.8,
		ClusterThresh:      0.6,
		MinClusterSize:     2,
		UseExternalService: false,
		UseGPU:             false,
		BatchSize:          32,
		NumWorkers:         4,
	}
}

// ==================== 接口定义 ====================

// Detector 人脸检测器接口
type Detector interface {
	// Detect 检测图像中的人脸
	Detect(ctx context.Context, img Image) (*DetectionResult, error)

	// Close 关闭检测器
	Close() error
}

// Recognizer 人脸识别器接口
type Recognizer interface {
	// ExtractEmbedding 提取人脸嵌入向量
	ExtractEmbedding(ctx context.Context, img Image, face *Face) ([]float32, error)

	// Compare 计算两个嵌入向量的相似度
	Compare(emb1, emb2 []float32) float64

	// Close 关闭识别器
	Close() error
}

// Clusterer 人脸聚类器接口
type Clusterer interface {
	// Cluster 对人脸进行聚类
	Cluster(ctx context.Context, faces []Face) (*ClusterResult, error)

	// Assign 将新人脸分配到现有聚类
	Assign(ctx context.Context, face *Face, persons []Person, embeddings map[string][]float32) (string, error)
}

// LabelManager 人脸标签管理器接口
type LabelManager interface {
	// CreatePerson 创建人物
	CreatePerson(ctx context.Context, name string) (*Person, error)

	// GetPerson 获取人物
	GetPerson(ctx context.Context, personID string) (*Person, error)

	// GetPersonByName 根据名称获取人物
	GetPersonByName(ctx context.Context, name string) (*Person, error)

	// ListPersons 列出所有人物
	ListPersons(ctx context.Context, limit, offset int) ([]Person, int, error)

	// UpdatePerson 更新人物
	UpdatePerson(ctx context.Context, personID string, updates map[string]interface{}) error

	// DeletePerson 删除人物
	DeletePerson(ctx context.Context, personID string) error

	// AssignFaceToPerson 将人脸分配给人物
	AssignFaceToPerson(ctx context.Context, faceID, personID string) error

	// UnassignFace 取消人脸分配
	UnassignFace(ctx context.Context, faceID string) error

	// GetFacesByPerson 获取人物的所有人脸
	GetFacesByPerson(ctx context.Context, personID string) ([]Face, error)

	// GetFacesByPhoto 获取照片中的所有人脸
	GetFacesByPhoto(ctx context.Context, photoID string) ([]Face, error)
}

// Image 图像接口
type Image interface {
	// Bounds 返回图像边界
	Bounds() (width, height int)

	// ToRGB 转换为RGB字节
	ToRGB() ([]byte, error)

	// ToRGBA 转换为RGBA字节
	ToRGBA() ([]byte, error)

	// GetPixel 获取指定位置的像素
	GetPixel(x, y int) (r, g, b, a uint8)
}

// Service 人脸识别服务接口
type Service interface {
	// DetectFaces 检测人脸
	DetectFaces(ctx context.Context, img Image, photoID string) (*DetectionResult, error)

	// ExtractEmbeddings 提取人脸嵌入向量
	ExtractEmbeddings(ctx context.Context, img Image, faces []Face) ([]Face, error)

	// RecognizeFaces 检测并识别人脸
	RecognizeFaces(ctx context.Context, img Image, photoID string) ([]Face, error)

	// ClusterFaces 人脸聚类
	ClusterFaces(ctx context.Context, photoIDs []string) (*ClusterResult, error)

	// AutoLabel 自动标记人脸
	AutoLabel(ctx context.Context, personID string, photoIDs []string) error

	// GetStats 获取统计信息
	GetStats(ctx context.Context) (*Stats, error)
}

// Stats 人脸识别统计
type Stats struct {
	TotalFaces    int            `json:"totalFaces"`
	TotalPersons  int            `json:"totalPersons"`
	Unassigned    int            `json:"unassigned"`
	PhotosWith    int            `json:"photosWithFaces"`
	TopPersons    []PersonStats  `json:"topPersons"`
	ModelInfo     ModelInfo      `json:"modelInfo"`
	LastProcessed time.Time      `json:"lastProcessed"`
}

// PersonStats 人物统计
type PersonStats struct {
	PersonID   string `json:"personId"`
	PersonName string `json:"personName"`
	FaceCount  int    `json:"faceCount"`
}

// ModelInfo 模型信息
type ModelInfo struct {
	DetectionModel   string `json:"detectionModel"`
	RecognitionModel string `json:"recognitionModel"`
	UseGPU           bool   `json:"useGPU"`
	ExternalService  bool   `json:"externalService"`
}

// FaceSearchResult 人脸搜索结果
type FaceSearchResult struct {
	FaceID   string  `json:"faceId"`
	PersonID string  `json:"personId"`
	Score    float64 `json:"score"`
}

// ==================== 错误定义 ====================

// Error 人脸识别错误
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

func (e *Error) Error() string {
	return e.Message
}

// 错误码定义
const (
	ErrCodeNoFace         = "NO_FACE_DETECTED"
	ErrCodeLowQuality     = "LOW_QUALITY_FACE"
	ErrCodeModelNotLoaded = "MODEL_NOT_LOADED"
	ErrCodeServiceError   = "EXTERNAL_SERVICE_ERROR"
	ErrCodeInvalidImage   = "INVALID_IMAGE"
	ErrCodePersonNotFound = "PERSON_NOT_FOUND"
	ErrCodeFaceNotFound   = "FACE_NOT_FOUND"
)

// NewError 创建错误
func NewError(code, message, detail string) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Detail:  detail,
	}
}