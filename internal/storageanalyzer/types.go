package storageanalyzer

import (
	"sync"
	"time"
)

// StorageReport represents a comprehensive storage analysis report
type StorageReport struct {
	ID            string            `json:"id"`
	GeneratedAt   time.Time         `json:"generated_at"`
	TotalSpace    int64             `json:"total_space"`
	UsedSpace     int64             `json:"used_space"`
	FreeSpace     int64             `json:"free_space"`
	UsagePercent  float64           `json:"usage_percent"`
	ByDirectory   []DirUsage        `json:"by_directory"`
	ByFileType    []FileTypeUsage   `json:"by_file_type"`
	ByUser        []UserUsage       `json:"by_user"`
	ByTime        []TimeUsage       `json:"by_time"`
	Duplicates    []DuplicateGroup  `json:"duplicates"`
	BigFiles      []FileInfo        `json:"big_files"`
	Growth        GrowthTrend       `json:"growth"`
	Suggestions   []CleanSuggestion `json:"suggestions"`
	Heatmap       []HeatmapEntry    `json:"heatmap"`
	Snapshots     SnapshotUsage     `json:"snapshots"`
}

// DirUsage represents storage usage by directory
type DirUsage struct {
	Path       string  `json:"path"`
	Size       int64   `json:"size"`
	FileCount  int     `json:"file_count"`
	Percent    float64 `json:"percent"`
	SubDirs    int     `json:"sub_dirs"`
	LastAccess time.Time `json:"last_access"`
}

// FileTypeUsage represents storage usage by file type
type FileTypeUsage struct {
	Extension string  `json:"extension"`
	Size      int64   `json:"size"`
	Count     int     `json:"count"`
	Percent   float64 `json:"percent"`
	Category  string  `json:"category"`
}

// UserUsage represents storage usage by user
type UserUsage struct {
	UserID    int    `json:"user_id"`
	Username  string `json:"username"`
	Size      int64  `json:"size"`
	FileCount int    `json:"file_count"`
	Percent   float64 `json:"percent"`
}

// TimeUsage represents storage usage by time period
type TimeUsage struct {
	Period    string `json:"period"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Size      int64  `json:"size"`
	FileCount int    `json:"file_count"`
	Growth    int64  `json:"growth"`
}

// DuplicateGroup represents a group of duplicate files
type DuplicateGroup struct {
	Hash      string     `json:"hash"`
	Size      int64      `json:"size"`
	Count     int        `json:"count"`
	Wasted    int64      `json:"wasted"`
	Files     []FileInfo `json:"files"`
	Algorithm string     `json:"algorithm"`
}

// FileInfo represents information about a file
type FileInfo struct {
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	ModTime   time.Time `json:"mod_time"`
	AccessTime time.Time `json:"access_time"`
	Owner     string    `json:"owner"`
	Extension string    `json:"extension"`
	IsDir     bool      `json:"is_dir"`
	Mode      string    `json:"mode"`
}

// GrowthTrend represents storage growth trend analysis
type GrowthTrend struct {
	DailyAvg   int64     `json:"daily_avg"`
	WeeklyAvg  int64     `json:"weekly_avg"`
	MonthlyAvg int64     `json:"monthly_avg"`
	Predicted  Prediction `json:"predicted"`
	History    []Snapshot `json:"history"`
}

// Prediction represents storage usage prediction
type Prediction struct {
	FullDate      time.Time `json:"full_date"`
	DaysRemaining int       `json:"days_remaining"`
	Confidence    float64   `json:"confidence"`
	Assumptions   string    `json:"assumptions"`
}

// Snapshot represents a point-in-time storage measurement
type Snapshot struct {
	Timestamp  time.Time `json:"timestamp"`
	UsedSpace  int64     `json:"used_space"`
	FreeSpace  int64     `json:"free_space"`
	FileCount  int       `json:"file_count"`
}

// CleanSuggestion represents a suggestion for cleaning up storage
type CleanSuggestion struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	Reason      string    `json:"reason"`
	Priority    int       `json:"priority"`
	Safe        bool      `json:"safe"`
	LastAccess  time.Time `json:"last_access"`
	Description string    `json:"description"`
}

// HeatmapEntry represents an entry in the storage access heatmap
type HeatmapEntry struct {
	Path        string    `json:"path"`
	AccessCount int       `json:"access_count"`
	LastAccess  time.Time `json:"last_access"`
	Size        int64     `json:"size"`
	HotLevel    int       `json:"hot_level"`
}

// SnapshotUsage represents storage usage by snapshots
type SnapshotUsage struct {
	TotalSize    int64            `json:"total_size"`
	SnapshotCount int            `json:"snapshot_count"`
	ByDataset    []DatasetSnapshot `json:"by_dataset"`
	Oldest       time.Time       `json:"oldest"`
	Newest       time.Time       `json:"newest"`
}

// DatasetSnapshot represents snapshot usage for a dataset
type DatasetSnapshot struct {
	Dataset      string `json:"dataset"`
	Count        int    `json:"count"`
	Size         int64  `json:"size"`
	PercentUsed  float64 `json:"percent_used"`
}

// AnalysisConfig represents configuration for storage analysis
type AnalysisConfig struct {
	ScanPaths       []string      `json:"scan_paths"`
	ExcludePaths    []string      `json:"exclude_paths"`
	MaxFileSize     int64         `json:"max_file_size"`
	HashAlgorithm   string        `json:"hash_algorithm"`
	SnapshotPath    string        `json:"snapshot_path"`
	ReportRetention time.Duration `json:"report_retention"`
	ScheduleEnabled bool          `json:"schedule_enabled"`
	ScheduleCron    string        `json:"schedule_cron"`
}

// AnalysisJob represents a running or completed analysis job
type AnalysisJob struct {
	ID        string        `json:"id"`
	Status    string        `json:"status"`
	StartedAt time.Time     `json:"started_at"`
	EndedAt   *time.Time    `json:"ended_at,omitempty"`
	Progress  float64       `json:"progress"`
	Error     string        `json:"error,omitempty"`
	Report    *StorageReport `json:"report,omitempty"`
}

// Analyzer is the main storage analysis engine
type Analyzer struct {
	config     AnalysisConfig
	reports    map[string]*StorageReport
	jobs       map[string]*AnalysisJob
	history    []Snapshot
	mu         sync.RWMutex
	stopCh     chan struct{}
}

// FileTypeCategory defines file type categories
var FileTypeCategory = map[string]string{
	// Documents
	".pdf":  "documents", ".doc": "documents", ".docx": "documents",
	".xls":  "documents", ".xlsx": "documents", ".ppt": "documents",
	".pptx": "documents", ".txt": "documents", ".md": "documents",
	// Images
	".jpg": "images", ".jpeg": "images", ".png": "images",
	".gif": "images", ".bmp": "images", ".svg": "images",
	".webp": "images", ".tiff": "images", ".ico": "images",
	// Videos
	".mp4": "videos", ".avi": "videos", ".mkv": "videos",
	".mov": "videos", ".wmv": "videos", ".flv": "videos",
	".webm": "videos", ".m4v": "videos",
	// Audio
	".mp3": "audio", ".wav": "audio", ".flac": "audio",
	".aac": "audio", ".ogg": "audio", ".wma": "audio",
	".m4a": "audio",
	// Archives
	".zip": "archives", ".tar": "archives", ".gz": "archives",
	".rar": "archives", ".7z": "archives", ".bz2": "archives",
	".xz": "archives",
	// Code
	".go": "code", ".py": "code", ".js": "code", ".ts": "code",
	".java": "code", ".c": "code", ".cpp": "code", ".h": "code",
	".rs": "code", ".rb": "code", ".php": "code", ".swift": "code",
	// Databases
	".db": "databases", ".sqlite": "databases", ".sqlite3": "databases",
	".sql": "databases", ".mdb": "databases",
	// System
	".log": "system", ".tmp": "system", ".cache": "system",
	".bak": "system", ".swp": "system",
}

// DefaultConfig returns a default analysis configuration
func DefaultConfig() AnalysisConfig {
	return AnalysisConfig{
		ScanPaths:       []string{"/"},
		ExcludePaths:    []string{"/proc", "/sys", "/dev", "/run"},
		MaxFileSize:     10 * 1024 * 1024 * 1024, // 10GB
		HashAlgorithm:   "sha256",
		ReportRetention: 30 * 24 * time.Hour,
		ScheduleEnabled: false,
		ScheduleCron:    "0 2 * * *", // 2 AM daily
	}
}
