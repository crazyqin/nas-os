// Package photodedup 提供照片重复检测与清理功能
package photodedup

import (
	"errors"
	"time"
)

// 预定义错误.
var (
	// ErrTaskNotFound 任务不存在.
	ErrTaskNotFound = errors.New("scan task not found")
	// ErrGroupNotFound 重复组不存在.
	ErrGroupNotFound = errors.New("duplicate group not found")
	// ErrTaskRunning 任务正在运行，无法重复启动.
	ErrTaskRunning = errors.New("task is already running")
	// ErrTaskNotRunning 任务未运行，无法暂停/取消.
	ErrTaskNotRunning = errors.New("task is not running")
	// ErrInvalidThreshold 阈值无效（须在 0-100 之间）.
	ErrInvalidThreshold = errors.New("threshold must be between 0 and 100")
	// ErrInvalidHashAlgorithm 哈希算法无效.
	ErrInvalidHashAlgorithm = errors.New("invalid hash algorithm, must be phash, dhash, or ahash")
	// ErrInvalidRetainPolicy 保留策略无效.
	ErrInvalidRetainPolicy = errors.New("invalid retain policy")
	// ErrBatchNotConfirmed 批量操作未经确认.
	ErrBatchNotConfirmed = errors.New("batch operation not confirmed")
	// ErrNoDuplicates 没有发现重复照片.
	ErrNoDuplicates = errors.New("no duplicate photos found")
)

// HashAlgorithm 感知哈希算法类型.
type HashAlgorithm string

const (
	// HashPHash 感知哈希（pHash）—— 基于 DCT 变换，抗缩放旋转能力最强.
	HashPHash HashAlgorithm = "phash"
	// HashDHash 差值哈希（dHash）—— 基于相邻像素差，速度快.
	HashDHash HashAlgorithm = "dhash"
	// HashAHash 均值哈希（aHash）—— 基于均值比较，最简单.
	HashAHash HashAlgorithm = "ahash"
)

// TaskStatus 扫描任务状态.
type TaskStatus string

const (
	// StatusPending 等待开始.
	StatusPending TaskStatus = "pending"
	// StatusRunning 扫描中.
	StatusRunning TaskStatus = "running"
	// StatusPaused 已暂停.
	StatusPaused TaskStatus = "paused"
	// StatusCompleted 已完成.
	StatusCompleted TaskStatus = "completed"
	// StatusCancelled 已取消.
	StatusCancelled TaskStatus = "cancelled"
	// StatusFailed 失败.
	StatusFailed TaskStatus = "failed"
)

// RetainPolicy 保留策略.
type RetainPolicy string

const (
	// RetainLargest 保留最大文件.
	RetainLargest RetainPolicy = "largest"
	// RetainSmallest 保留最小文件.
	RetainSmallest RetainPolicy = "smallest"
	// RetainNewest 保留最新文件.
	RetainNewest RetainPolicy = "newest"
	// RetainOldest 保留最早文件.
	RetainOldest RetainPolicy = "oldest"
	// RetainSharpest 保留最清晰文件（基于拉普拉斯方差）.
	RetainSharpest RetainPolicy = "sharpest"
	// RetainManual 手动选择保留.
	RetainManual RetainPolicy = "manual"
)

// CleanupAction 清理动作.
type CleanupAction string

const (
	// ActionDelete 永久删除.
	ActionDelete CleanupAction = "delete"
	// ActionTrash 移动到回收站.
	ActionTrash CleanupAction = "trash"
)

// PhotoInfo 照片信息.
type PhotoInfo struct {
	ID           string    `json:"id"`
	FilePath     string    `json:"file_path"`
	FileName     string    `json:"file_name"`
	FileSize     int64     `json:"file_size"`     // 字节
	Width        int       `json:"width"`         // 像素
	Height       int       `json:"height"`        // 像素
	ModTime      time.Time `json:"mod_time"`      // 修改时间
	HashValue    uint64    `json:"hash_value"`    // 感知哈希值
	BlurScore    float64   `json:"blur_score"`    // 清晰度评分（拉普拉斯方差，越高越清晰）
	ThumbnailURL string    `json:"thumbnail_url"` // 缩略图访问 URL
}

// DuplicateGroup 重复照片组.
type DuplicateGroup struct {
	ID         string       `json:"id"`
	Similarity float64      `json:"similarity"`  // 组内最低相似度 (%)
	Photos     []*PhotoInfo `json:"photos"`      // 组内照片列表
	RetainID   string       `json:"retain_id"`   // 建议保留的照片 ID
	TotalSize  int64        `json:"total_size"`  // 组内总大小（字节）
	WastedSize int64        `json:"wasted_size"` // 可节省空间（字节）
}

// ScanTask 扫描任务.
type ScanTask struct {
	ID          string        `json:"id"`
	Status      TaskStatus    `json:"status"`
	ScanDirs    []string      `json:"scan_dirs"`    // 扫描目录
	ExcludeDirs []string      `json:"exclude_dirs"` // 排除目录
	Threshold   int           `json:"threshold"`    // 相似度阈值 (0-100)
	Algorithm   HashAlgorithm `json:"algorithm"`    // 哈希算法
	CreatedAt   time.Time     `json:"created_at"`
	StartedAt   *time.Time    `json:"started_at,omitempty"`
	FinishedAt  *time.Time    `json:"finished_at,omitempty"`
	Progress    float64       `json:"progress"`     // 0-100
	TotalFiles  int           `json:"total_files"`  // 已扫描文件数
	TotalGroups int           `json:"total_groups"` // 重复组数
	TotalWasted int64         `json:"total_wasted"` // 可节省空间（字节）
	Error       string        `json:"error,omitempty"`
}

// ScanStats 扫描结果统计.
type ScanStats struct {
	TotalScanned    int   `json:"total_scanned"`    // 总扫描文件数
	TotalGroups     int   `json:"total_groups"`     // 重复组数
	TotalDuplicates int   `json:"total_duplicates"` // 重复照片数
	TotalWasted     int64 `json:"total_wasted"`     // 可节省空间（字节）
}

// ========== 请求/响应结构 ==========

// StartScanRequest 启动扫描请求.
type StartScanRequest struct {
	ScanDirs    []string      `json:"scan_dirs" binding:"required,min=1"`
	ExcludeDirs []string      `json:"exclude_dirs,omitempty"`
	Threshold   int           `json:"threshold"` // 默认 90
	Algorithm   HashAlgorithm `json:"algorithm"` // 默认 phash
}

// UpdateConfigRequest 更新配置请求.
type UpdateConfigRequest struct {
	ExcludeDirs []string       `json:"exclude_dirs,omitempty"`
	Threshold   *int           `json:"threshold,omitempty"`
	Algorithm   *HashAlgorithm `json:"algorithm,omitempty"`
}

// SetRetainRequest 设置保留照片请求.
type SetRetainRequest struct {
	PhotoID string `json:"photo_id" binding:"required"`
}

// BatchCleanupRequest 批量清理请求.
type BatchCleanupRequest struct {
	GroupIDs     []string      `json:"group_ids" binding:"required,min=1"`
	RetainPolicy RetainPolicy  `json:"retain_policy" binding:"required"`
	Action       CleanupAction `json:"action"`    // 默认 trash
	Confirmed    bool          `json:"confirmed"` // 二次确认标记
}

// CleanupPreview 清理预览.
type CleanupPreview struct {
	GroupCount    int   `json:"group_count"`    // 涉及组数
	DeleteCount   int   `json:"delete_count"`   // 将删除照片数
	ReclaimedSize int64 `json:"reclaimed_size"` // 将释放空间（字节）
}

// CleanupResult 清理结果.
type CleanupResult struct {
	DeletedCount  int   `json:"deleted_count"`  // 已删除照片数
	ReclaimedSize int64 `json:"reclaimed_size"` // 已释放空间（字节）
}

// ScheduleConfig 定时扫描配置.
type ScheduleConfig struct {
	Enabled     bool          `json:"enabled"`
	Cron        string        `json:"cron"` // cron 表达式
	ScanDirs    []string      `json:"scan_dirs"`
	ExcludeDirs []string      `json:"exclude_dirs,omitempty"`
	Threshold   int           `json:"threshold"`
	Algorithm   HashAlgorithm `json:"algorithm"`
}
