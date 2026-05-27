package diskspace

import (
	"time"
)

// DiskUsage represents disk space usage statistics
type DiskUsage struct {
	Total         int64   `json:"total"`
	Used          int64   `json:"used"`
	Free          int64   `json:"free"`
	UsagePercent  float64 `json:"usage_percent"`
	InodeTotal    int64   `json:"inode_total"`
	InodeUsed     int64   `json:"inode_used"`
	InodeFree     int64   `json:"inode_free"`
	InodePercent  float64 `json:"inode_percent"`
}

// DirectoryNode represents a directory in the file tree
type DirectoryNode struct {
	Path       string           `json:"path"`
	Name       string           `json:"name"`
	Size       int64            `json:"size"`
	FileCount  int              `json:"file_count"`
	DirCount   int              `json:"dir_count"`
	Children   []DirectoryNode  `json:"children,omitempty"`
	Depth      int              `json:"depth"`
	ParentPath string           `json:"parent_path,omitempty"`
}

// FileTypeStats represents statistics for a file type
type FileTypeStats struct {
	Extension  string  `json:"extension"`
	Count      int     `json:"count"`
	TotalSize  int64   `json:"total_size"`
	Percentage float64 `json:"percentage"`
}

// LargeFileInfo represents information about a large file
type LargeFileInfo struct {
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modified_at"`
	Owner      string    `json:"owner"`
	Extension  string    `json:"extension"`
}

// FileInfo represents basic file information for duplicate detection
type FileInfo struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// DuplicateFile represents a group of duplicate files
type DuplicateFile struct {
	Hash             string     `json:"hash"`
	Files            []FileInfo `json:"files"`
	TotalWastedSpace int64      `json:"total_wasted_space"`
}

// TreemapData represents data for treemap visualization
type TreemapData struct {
	Name     string        `json:"name"`
	Size     int64         `json:"size"`
	Children []TreemapData `json:"children,omitempty"`
	Color    string        `json:"color,omitempty"`
	Path     string        `json:"path"`
}

// ScanProgress represents the progress of a disk scan
type ScanProgress struct {
	ScannedDirs  int     `json:"scanned_dirs"`
	ScannedFiles int     `json:"scanned_files"`
	CurrentPath  string  `json:"current_path"`
	Percent      float64 `json:"percent"`
	Elapsed      float64 `json:"elapsed"`
	Status       string  `json:"status"`
}

// ScanConfig represents configuration for a disk scan
type ScanConfig struct {
	MaxDepth      int      `json:"max_depth"`
	IncludeHidden bool     `json:"include_hidden"`
	ExcludePaths  []string `json:"exclude_paths,omitempty"`
	MinFileSize   int64    `json:"min_file_size,omitempty"`
}

// GrowthTrend represents disk usage trend over time
type GrowthTrend struct {
	Date       time.Time `json:"date"`
	UsedSpace  int64     `json:"used_space"`
	FileCount  int       `json:"file_count"`
}

// DiskStats represents complete disk statistics
type DiskStats struct {
	DiskUsage            DiskUsage        `json:"disk_usage"`
	TopDirectories       []DirectoryNode  `json:"top_directories"`
	FileTypeDistribution []FileTypeStats  `json:"file_type_distribution"`
	LargestFiles         []LargeFileInfo  `json:"largest_files"`
	Duplicates           []DuplicateFile  `json:"duplicates,omitempty"`
	GrowthTrend          []GrowthTrend    `json:"growth_trend"`
}

// ScanRequest represents a request to start a scan
type ScanRequest struct {
	Path   string     `json:"path" validate:"required"`
	Config ScanConfig `json:"config"`
}

// ExportRequest represents a request to export a report
type ExportRequest struct {
	Format string `json:"format" validate:"required"`
	Path   string `json:"path,omitempty"`
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// SuccessResponse represents a generic API success response
type SuccessResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
}
