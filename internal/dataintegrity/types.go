// Package dataintegrity 提供数据完整性校验功能
// 参考 TrueNAS Self-Healing Checksums 设计，提供文件校验和计算、损坏检测、
// 自动修复建议、完整性报告等功能。
package dataintegrity

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrFileNotFound 文件不存在.
	ErrFileNotFound = errors.New("文件不存在")
	// ErrChecksumNotFound 未找到存储的校验和记录.
	ErrChecksumNotFound = errors.New("未找到校验和记录")
	// ErrIntegrityJobNotFound 完整性检查任务不存在.
	ErrIntegrityJobNotFound = errors.New("完整性检查任务不存在")
	// ErrJobAlreadyRunning 已有任务正在运行.
	ErrJobAlreadyRunning = errors.New("已有任务正在运行")
	// ErrJobNotRunning 当前没有运行中的任务.
	ErrJobNotRunning = errors.New("当前没有运行中的任务")
	// ErrInvalidAlgorithm 不支持的校验算法.
	ErrInvalidAlgorithm = errors.New("不支持的校验算法")
	// ErrCorruptionDetected 检测到数据损坏.
	ErrCorruptionDetected = errors.New("检测到数据损坏")
	// ErrRepairFailed 修复失败.
	ErrRepairFailed = errors.New("修复失败")
	// ErrPoolNotFound 存储池不存在.
	ErrPoolNotFound = errors.New("存储池不存在")
	// ErrPathRequired 必须指定路径.
	ErrPathRequired = errors.New("必须指定路径")
)

// ========== 校验算法类型 ==========

// Algorithm 校验算法.
type Algorithm string

const (
	// AlgorithmSHA256 SHA-256 算法（默认）.
	AlgorithmSHA256 Algorithm = "sha256"
	// AlgorithmSHA512 SHA-512 算法.
	AlgorithmSHA512 Algorithm = "sha512"
	// AlgorithmBLAKE2b BLAKE2b-256 算法.
	AlgorithmBLAKE2b Algorithm = "blake2b"
	// AlgorithmCRC32 CRC-32 算法（快速但不防篡改）.
	AlgorithmCRC32 Algorithm = "crc32"
)

// supportedAlgorithms 支持的算法集合.
var supportedAlgorithms = map[Algorithm]bool{
	AlgorithmSHA256:  true,
	AlgorithmSHA512:  true,
	AlgorithmBLAKE2b: true,
	AlgorithmCRC32:   true,
}

// IsSupportedAlgorithm 检查算法是否受支持.
func IsSupportedAlgorithm(algo Algorithm) bool {
	return supportedAlgorithms[algo]
}

// ========== 完整性状态类型 ==========

// IntegrityStatus 完整性状态.
type IntegrityStatus string

const (
	// StatusIntact 完整.
	StatusIntact IntegrityStatus = "intact"
	// StatusCorrupted 损坏.
	StatusCorrupted IntegrityStatus = "corrupted"
	// StatusUnknown 未知（未校验过）.
	StatusUnknown IntegrityStatus = "unknown"
	// StatusRepaired 已修复.
	StatusRepaired IntegrityStatus = "repaired"
	// StatusRepairFailed 修复失败.
	StatusRepairFailed IntegrityStatus = "repair_failed"
)

// ========== 任务状态类型 ==========

// JobState 完整性检查任务状态.
type JobState string

const (
	// JobStatePending 等待执行.
	JobStatePending JobState = "pending"
	// JobStateRunning 运行中.
	JobStateRunning JobState = "running"
	// JobStateCompleted 已完成.
	JobStateCompleted JobState = "completed"
	// JobStateFailed 失败.
	JobStateFailed JobState = "failed"
	// JobStateCancelled 已取消.
	JobStateCancelled JobState = "cancelled"
)

// ========== 核心数据结构 ==========

// FileChecksum 文件校验和记录.
type FileChecksum struct {
	ID        int64     `json:"id"`
	FilePath  string    `json:"file_path"`  // 文件路径
	Algorithm Algorithm `json:"algorithm"`   // 校验算法
	Checksum  string    `json:"checksum"`    // 校验和值（十六进制）
	FileSize  int64     `json:"file_size"`   // 文件大小（字节）
	ModTime   time.Time `json:"mod_time"`    // 文件修改时间
	CreatedAt time.Time `json:"created_at"`  // 记录创建时间
	UpdatedAt time.Time `json:"updated_at"`  // 记录更新时间
}

// IntegrityCheck 完整性检查记录.
type IntegrityCheck struct {
	ID           int64           `json:"id"`
	FilePath     string          `json:"file_path"`
	Algorithm    Algorithm       `json:"algorithm"`
	ExpectedHash string          `json:"expected_hash"` // 预期校验和
	ActualHash   string          `json:"actual_hash"`   // 实际校验和
	Status       IntegrityStatus `json:"status"`
	Message      string          `json:"message"`
	CheckedAt    time.Time       `json:"checked_at"`
}

// RepairSuggestion 修复建议.
type RepairSuggestion struct {
	FilePath    string          `json:"file_path"`
	Status      IntegrityStatus `json:"status"`
	Strategy    RepairStrategy  `json:"strategy"`
	Description string          `json:"description"`
	Sources     []RepairSource  `json:"sources,omitempty"` // 可用的修复来源
}

// RepairStrategy 修复策略.
type RepairStrategy string

const (
	// StrategySnapshotRestore 从快照恢复.
	StrategySnapshotRestore RepairStrategy = "snapshot_restore"
	// StrategyReplicaRestore 从副本恢复.
	StrategyReplicaRestore RepairStrategy = "replica_restore"
	// StrategyBackupRestore 从备份恢复.
	StrategyBackupRestore RepairStrategy = "backup_restore"
	// StrategyScrubRepair 通过ZFS Scrub修复.
	StrategyScrubRepair RepairStrategy = "scrub_repair"
	// StrategyManual 人工处理.
	StrategyManual RepairStrategy = "manual"
)

// RepairSource 修复来源.
type RepairSource struct {
	Type    string `json:"type"`    // 来源类型: snapshot, replica, backup
	Source  string `json:"source"`  // 来源标识
	ModTime string `json:"mod_time"` // 来源文件修改时间
	Size    int64  `json:"size"`    // 来源文件大小
}

// IntegrityJob 完整性检查任务.
type IntegrityJob struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Path       string    `json:"path"`        // 扫描路径（文件或目录）
	Algorithm  Algorithm `json:"algorithm"`
	Recursive  bool      `json:"recursive"`   // 是否递归扫描子目录
	State      JobState  `json:"state"`
	Progress   float64   `json:"progress"`    // 0.0 ~ 1.0
	TotalFiles int       `json:"total_files"` // 总文件数
	Scanned    int       `json:"scanned"`     // 已扫描文件数
	Intact     int       `json:"intact"`      // 完整文件数
	Corrupted  int       `json:"corrupted"`   // 损坏文件数
	Unknown    int       `json:"unknown"`     // 未校验过
	ErrorMsg   string    `json:"error_msg,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	EndedAt    time.Time `json:"ended_at,omitempty"`
}

// IntegrityReport 完整性报告.
type IntegrityReport struct {
	GeneratedAt  time.Time              `json:"generated_at"`
	Summary      ReportSummary          `json:"summary"`
	Files        []FileIntegrityEntry   `json:"files"`
	RepairNeeded []RepairSuggestion     `json:"repair_needed,omitempty"`
}

// ReportSummary 报告摘要.
type ReportSummary struct {
	TotalFiles   int       `json:"total_files"`
	Intact       int       `json:"intact"`
	Corrupted    int       `json:"corrupted"`
	Unknown      int       `json:"unknown"`
	Repaired     int       `json:"repaired"`
	TotalSize    int64     `json:"total_size"`
	LastScanTime time.Time `json:"last_scan_time"`
}

// FileIntegrityEntry 文件完整性条目.
type FileIntegrityEntry struct {
	FilePath  string          `json:"file_path"`
	FileSize  int64           `json:"file_size"`
	Algorithm Algorithm       `json:"algorithm"`
	Checksum  string          `json:"checksum"`
	Status    IntegrityStatus `json:"status"`
	LastCheck time.Time       `json:"last_check"`
}

// ========== 请求/响应结构 ==========

// CalculateChecksumRequest 计算校验和请求.
type CalculateChecksumRequest struct {
	FilePath  string    `json:"file_path" binding:"required"`
	Algorithm Algorithm `json:"algorithm"`
}

// VerifyFileRequest 校验文件请求.
type VerifyFileRequest struct {
	FilePath string `json:"file_path" binding:"required"`
}

// CreateJobRequest 创建完整性检查任务请求.
type CreateJobRequest struct {
	Name      string    `json:"name" binding:"required"`
	Path      string    `json:"path" binding:"required"`
	Algorithm Algorithm `json:"algorithm"`
	Recursive bool      `json:"recursive"`
}

// RepairRequest 修复请求.
type RepairRequest struct {
	FilePath string         `json:"file_path" binding:"required"`
	Strategy RepairStrategy `json:"strategy" binding:"required"`
	Source   string         `json:"source"` // 修复来源标识
}

// ReportRequest 报告请求.
type ReportRequest struct {
	Path      string    `json:"path"`
	Algorithm Algorithm `json:"algorithm"`
}

// ========== 接口定义 ==========

// ChecksumStore 校验和存储接口.
type ChecksumStore interface {
	// SaveChecksum 保存校验和记录.
	SaveChecksum(cs *FileChecksum) error
	// GetChecksum 获取指定文件的校验和.
	GetChecksum(filePath string, algo Algorithm) (*FileChecksum, error)
	// ListChecksums 列出所有校验和记录.
	ListChecksums(pathPrefix string, algo Algorithm) ([]*FileChecksum, error)
	// DeleteChecksum 删除校验和记录.
	DeleteChecksum(filePath string, algo Algorithm) error
	// SaveIntegrityCheck 保存完整性检查记录.
	SaveIntegrityCheck(check *IntegrityCheck) error
	// GetIntegrityHistory 获取文件的完整性检查历史.
	GetIntegrityHistory(filePath string, limit int) ([]*IntegrityCheck, error)
	// SaveJob 保存任务.
	SaveJob(job *IntegrityJob) error
	// GetJob 获取任务.
	GetJob(jobID int64) (*IntegrityJob, error)
	// ListJobs 列出任务.
	ListJobs(limit int) ([]*IntegrityJob, error)
}

// FileSystemProvider 文件系统接口.
type FileSystemProvider interface {
	// Stat 获取文件信息.
	Stat(path string) (*FileInfo, error)
	// ListDir 列出目录内容.
	ListDir(path string) ([]*FileInfo, error)
	// ReadFile 读取文件内容（用于计算校验和）.
	ReadFile(path string, offset int64, size int) ([]byte, error)
	// GetFileSize 获取文件大小.
	GetFileSize(path string) (int64, error)
}

// SnapshotProvider 快照提供者接口.
type SnapshotProvider interface {
	// ListSnapshots 列出路径相关的快照.
	ListSnapshots(path string) ([]*SnapshotInfo, error)
	// RestoreFromSnapshot 从快照恢复文件.
	RestoreFromSnapshot(snapshotID string, filePath string) error
}

// SnapshotInfo 快照信息.
type SnapshotInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"created_at"`
	Size      int64     `json:"size"`
}

// ReplicationProvider 副本提供者接口.
type ReplicationProvider interface {
	// ListReplicas 列出文件副本.
	ListReplicas(filePath string) ([]*ReplicaInfo, error)
	// RestoreFromReplica 从副本恢复.
	RestoreFromReplica(replicaID string, targetPath string) error
}

// ReplicaInfo 副本信息.
type ReplicaInfo struct {
	ID        string    `json:"id"`
	Source    string    `json:"source"`
	Target    string    `json:"target"`
	Status    string    `json:"status"`
	ModTime   time.Time `json:"mod_time"`
	Size      int64     `json:"size"`
}

// FileInfo 文件信息.
type FileInfo struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
	IsDir   bool      `json:"is_dir"`
}
