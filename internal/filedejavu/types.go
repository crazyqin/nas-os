// Package filedejavu 提供重复文件智能检测功能
// 三阶段扫描：快速预筛（大小）→ 哈希分组 → 感知哈希相似度
package filedejavu

import (
	"sync"
	"time"
)

// DuplicateType 重复类型
type DuplicateType string

const (
	DupExact   DuplicateType = "exact"   // 完全重复（SHA-256 相同）
	DupSimilar DuplicateType = "similar" // 相似图片（pHash 相似度 > 阈值）
)

// KeepStrategy 保留策略
type KeepStrategy string

const (
	KeepNewest  KeepStrategy = "newest"  // 保留最新修改时间
	KeepOldest  KeepStrategy = "oldest"  // 保留最旧修改时间
	KeepLargest KeepStrategy = "largest" // 保留最大文件
	KeepFirst   KeepStrategy = "first"   // 保留路径字典序第一个
)

// DedupAction 去重动作类型
type DedupAction string

const (
	ActionDelete   DedupAction = "delete"   // 直接删除
	ActionRecycle  DedupAction = "recycle"  // 移到回收站
	ActionSymlink  DedupAction = "symlink"  // 替换为符号链接
	ActionHardlink DedupAction = "hardlink" // 替换为硬链接
	ActionReport   DedupAction = "report"   // 仅报告，不操作
)

// FileFingerprint 文件指纹
type FileFingerprint struct {
	Path        string    `json:"path"`        // 文件路径
	Size        int64     `json:"size"`        // 文件大小（字节）
	ModTime     time.Time `json:"modTime"`     // 修改时间
	SHA256      string    `json:"sha256"`      // SHA-256 哈希
	PerceptHash string    `json:"perceptHash"` // 感知哈希（pHash）
	IsImage     bool      `json:"isImage"`     // 是否为图片文件
}

// DuplicateGroup 重复文件组
type DuplicateGroup struct {
	ID        string             `json:"id"`        // 组 ID
	Type      DuplicateType      `json:"type"`      // 重复类型
	Hash      string             `json:"hash"`      // 内容哈希（exact 模式）
	SimScore  float64            `json:"simScore"`  // 相似度分数（similar 模式，0-1）
	Files     []*FileFingerprint `json:"files"`     // 重复文件列表
	Savings   int64              `json:"savings"`   // 可节省空间（字节）
	Recommend *FileFingerprint   `json:"recommend"` // 推荐保留的文件
}

// ScanConfig 扫描配置
type ScanConfig struct {
	Paths           []string     `json:"paths"`           // 扫描路径
	ExcludePatterns []string     `json:"excludePatterns"` // 排除模式（glob）
	MinFileSize     int64        `json:"minFileSize"`     // 最小文件大小
	MaxFileSize     int64        `json:"maxFileSize"`     // 最大文件大小（0=无限制）
	Threshold       float64      `json:"threshold"`       // 相似度阈值（0-1，默认 0.85）
	ScanImages      bool         `json:"scanImages"`      // 是否扫描图片相似度
	KeepStrategy    KeepStrategy `json:"keepStrategy"`    // 保留策略
	Action          DedupAction  `json:"action"`          // 去重动作
	DryRun          bool         `json:"dryRun"`          // 试运行模式
	MaxWorkers      int          `json:"maxWorkers"`      // 最大并行数
}

// DefaultScanConfig 返回默认扫描配置
func DefaultScanConfig() *ScanConfig {
	return &ScanConfig{
		MinFileSize:  1024, // 1KB
		MaxFileSize:  0,
		Threshold:    0.85,
		ScanImages:   true,
		KeepStrategy: KeepNewest,
		Action:       ActionReport,
		DryRun:       true,
		MaxWorkers:   4,
	}
}

// ScanResult 扫描结果
type ScanResult struct {
	mu sync.RWMutex

	TotalFiles     int64             `json:"totalFiles"`      // 扫描文件总数
	TotalSize      int64             `json:"totalSize"`       // 扫描文件总大小
	Groups         []*DuplicateGroup `json:"groups"`          // 重复组列表
	DuplicateCount int64             `json:"duplicateCount"`  // 重复文件数
	SavingsTotal   int64             `json:"savingsTotal"`    // 总可节省空间
	StartTime      time.Time         `json:"startTime"`       // 扫描开始时间
	EndTime        time.Time         `json:"endTime"`         // 扫描结束时间
	Duration       time.Duration     `json:"duration"`        // 扫描耗时
	Status         string            `json:"status"`          // 状态: running/completed/cancelled/error
	Error          string            `json:"error,omitempty"` // 错误信息
}

// AddGroup 添加重复组（线程安全）
func (r *ScanResult) AddGroup(g *DuplicateGroup) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Groups = append(r.Groups, g)
	r.DuplicateCount += int64(len(g.Files) - 1)
	r.SavingsTotal += g.Savings
}

// GetGroups 获取所有重复组（线程安全）
func (r *ScanResult) GetGroups() []*DuplicateGroup {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*DuplicateGroup, len(r.Groups))
	copy(result, r.Groups)
	return result
}

// BatchDedupRequest 批量去重请求
type BatchDedupRequest struct {
	GroupIDs []string     `json:"groupIds"` // 要处理的组 ID 列表
	Action   DedupAction  `json:"action"`   // 去重动作
	Strategy KeepStrategy `json:"strategy"` // 保留策略
	DryRun   bool         `json:"dryRun"`   // 试运行
}

// BatchDedupResult 批量去重结果
type BatchDedupResult struct {
	ProcessedGroups int      `json:"processedGroups"`  // 处理的组数
	DeletedFiles    int      `json:"deletedFiles"`     // 删除的文件数
	SymlinkFiles    int      `json:"symlinkFiles"`     // 替换为符号链接的文件数
	HardlinkFiles   int      `json:"hardlinkFiles"`    // 替换为硬链接的文件数
	SavedBytes      int64    `json:"savedBytes"`       // 节省的空间
	Errors          []string `json:"errors,omitempty"` // 错误列表
}

// APIResponse 通用 API 响应
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// 图片文件扩展名
var imageExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
	".bmp": true, ".tiff": true, ".tif": true, ".webp": true,
	".svg": true, ".ico": true, ".heic": true, ".heif": true,
	".raw": true, ".cr2": true, ".nef": true, ".arw": true,
}

// IsImageFile 判断是否为图片文件
func IsImageFile(path string) bool {
	for ext := range imageExtensions {
		if len(path) >= len(ext) {
			if path[len(path)-len(ext):] == ext {
				return true
			}
		}
	}
	return false
}
