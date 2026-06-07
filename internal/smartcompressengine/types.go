// Package smartcompressengine 提供AI智能压缩引擎功能
// v2.561.0 - AI驱动的智能压缩引擎
package smartcompressengine

import (
	"sync"
	"time"
)

// CompressionAlgorithm 压缩算法类型.
type CompressionAlgorithm string

const (
	AlgorithmGzip    CompressionAlgorithm = "gzip"
	AlgorithmZstd    CompressionAlgorithm = "zstd"
	AlgorithmLZ4     CompressionAlgorithm = "lz4"
	AlgorithmBrotli  CompressionAlgorithm = "brotli"
	AlgorithmSnappy  CompressionAlgorithm = "snappy"
	AlgorithmXZ      CompressionAlgorithm = "xz"
	AlgorithmZlib    CompressionAlgorithm = "zlib"
)

// FileType 文件类型.
type FileType string

const (
	FileTypeText     FileType = "text"
	FileTypeBinary   FileType = "binary"
	FileTypeMedia    FileType = "media"
	FileTypeArchive  FileType = "archive"
	FileTypeDatabase FileType = "database"
	FileTypeLog      FileType = "log"
	FileTypeCode     FileType = "code"
	FileTypeDoc      FileType = "document"
)

// CompressionLevel 压缩级别.
type CompressionLevel int

const (
	LevelFast    CompressionLevel = 1
	LevelBalanced CompressionLevel = 5
	LevelMax     CompressionLevel = 9
)

// CompressTask 压缩任务.
type CompressTask struct {
	ID            string             `json:"id"`
	SourcePath    string             `json:"sourcePath"`
	DestPath      string             `json:"destPath"`
	Algorithm     CompressionAlgorithm `json:"algorithm"`
	Level         CompressionLevel   `json:"level"`
	Status        TaskStatus         `json:"status"`
	OriginalSize  int64              `json:"originalSize"`
	CompressedSize int64             `json:"compressedSize"`
	Ratio         float64            `json:"ratio"`
	StartTime     time.Time          `json:"startTime"`
	EndTime       time.Time          `json:"endTime"`
	Error         string             `json:"error,omitempty"`
}

// TaskStatus 任务状态.
type TaskStatus string

const (
	StatusPending   TaskStatus = "pending"
	StatusRunning   TaskStatus = "running"
	StatusCompleted TaskStatus = "completed"
	StatusFailed    TaskStatus = "failed"
)

// FileAnalysis 文件分析结果.
type FileAnalysis struct {
	FilePath     string             `json:"filePath"`
	FileType     FileType           `json:"fileType"`
	FileSize     int64              `json:"fileSize"`
	Entropy      float64            `json:"entropy"`
	Compressible bool               `json:"compressible"`
	Recommended  CompressionAlgorithm `json:"recommended"`
	EstRatio     float64            `json:"estRatio"`
}

// CompressionStats 压缩统计.
type CompressionStats struct {
	TotalTasks      int64   `json:"totalTasks"`
	CompletedTasks  int64   `json:"completedTasks"`
	FailedTasks     int64   `json:"failedTasks"`
	TotalOriginal   int64   `json:"totalOriginal"`
	TotalCompressed int64   `json:"totalCompressed"`
	AvgRatio        float64 `json:"avgRatio"`
	TimeSaved       float64 `json:"timeSaved"`
}

// EngineConfig 引擎配置.
type EngineConfig struct {
	DefaultAlgorithm CompressionAlgorithm `json:"defaultAlgorithm"`
	DefaultLevel     CompressionLevel     `json:"defaultLevel"`
	MaxConcurrency   int                  `json:"maxConcurrency"`
	EnableAI         bool                 `json:"enableAI"`
	DataDir          string               `json:"dataDir"`
	MinFileSize      int64                `json:"minFileSize"`
	SkipEncrypted    bool                 `json:"skipEncrypted"`
}

// Manager 智能压缩引擎管理器.
type Manager struct {
	mu          sync.RWMutex
	config      EngineConfig
	tasks       map[string]*CompressTask
	stats       CompressionStats
	running     bool
	stopChan    chan struct{}
	workerPool  chan struct{}
	analyzeFunc func(string) *FileAnalysis
}
