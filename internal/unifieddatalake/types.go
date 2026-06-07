package unifieddatalake

import (
	"sync"
	"time"
)

// StorageType 存储类型
type StorageType string

const (
	StorageS3    StorageType = "s3"
	StorageNFS   StorageType = "nfs"
	StorageLocal StorageType = "local"
)

// DataSourceStatus 数据源状态
type DataSourceStatus string

const (
	DSStatusOnline  DataSourceStatus = "online"
	DSStatusOffline DataSourceStatus = "offline"
	DSStatusSyncing DataSourceStatus = "syncing"
	DSStatusError   DataSourceStatus = "error"
)

// DataSource 数据源
type DataSource struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        StorageType       `json:"type"`
	Endpoint    string            `json:"endpoint"`
	Bucket      string            `json:"bucket,omitempty"`
	MountPath   string            `json:"mount_path,omitempty"`
	Credentials map[string]string `json:"credentials,omitempty"`
	Status      DataSourceStatus  `json:"status"`
	Capacity    int64             `json:"capacity"`
	Used        int64             `json:"used"`
	Tags        map[string]string `json:"tags,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// DataObject 数据对象（文件/目录）
type DataObject struct {
	ID          string            `json:"id"`
	SourceID    string            `json:"source_id"`
	Path        string            `json:"path"`
	Name        string            `json:"name"`
	IsDir       bool              `json:"is_dir"`
	Size        int64             `json:"size"`
	ContentType string            `json:"content_type"`
	Checksum    string            `json:"checksum,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
	CustomMeta  map[string]string `json:"custom_meta,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	ModifiedAt  time.Time         `json:"modified_at"`
	AccessedAt  time.Time         `json:"accessed_at"`
}

// CatalogEntry 数据目录条目
type CatalogEntry struct {
	ID           string            `json:"id"`
	ObjectID     string            `json:"object_id"`
	Name         string            `json:"name"`
	Description  string            `json:"description,omitempty"`
	Category     string            `json:"category,omitempty"`
	Owner        string            `json:"owner,omitempty"`
	Tags         map[string]string `json:"tags,omitempty"`
	Schema       *DataSchema       `json:"schema,omitempty"`
	RowCount     int64             `json:"row_count,omitempty"`
	ColCount     int               `json:"col_count,omitempty"`
	Stats        *DatasetStats     `json:"stats,omitempty"`
	QualityScore float64           `json:"quality_score"`
	LineageID    string            `json:"lineage_id,omitempty"`
	Version      int               `json:"version"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// DataSchema 数据Schema
type DataSchema struct {
	Fields []SchemaField `json:"fields"`
}

// SchemaField Schema字段
type SchemaField struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Comment  string `json:"comment,omitempty"`
}

// DatasetStats 数据集统计
type DatasetStats struct {
	TotalRows   int64             `json:"total_rows"`
	TotalCols   int               `json:"total_cols"`
	TotalSize   int64             `json:"total_size"`
	NullCount   map[string]int64  `json:"null_count,omitempty"`
	UniqueCount map[string]int64  `json:"unique_count,omitempty"`
	MinValues   map[string]string `json:"min_values,omitempty"`
	MaxValues   map[string]string `json:"max_values,omitempty"`
}

// LineageNode 血缘节点
type LineageNode struct {
	ID        string    `json:"id"`
	ObjectID  string    `json:"object_id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"` // source, transform, destination
	CreatedAt time.Time `json:"created_at"`
}

// LineageEdge 血缘边
type LineageEdge struct {
	ID           string    `json:"id"`
	SourceNodeID string    `json:"source_node_id"`
	TargetNodeID string    `json:"target_node_id"`
	Transform    string    `json:"transform,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// LineageGraph 血缘图
type LineageGraph struct {
	ID        string         `json:"id"`
	Nodes     []*LineageNode `json:"nodes"`
	Edges     []*LineageEdge `json:"edges"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// QualityRule 质量规则
type QualityRule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Type        QualityRuleType   `json:"type"`
	Field       string            `json:"field,omitempty"`
	Params      map[string]string `json:"params,omitempty"`
	Severity    QualitySeverity   `json:"severity"`
	Enabled     bool              `json:"enabled"`
	CreatedAt   time.Time         `json:"created_at"`
}

// QualityRuleType 质量规则类型
type QualityRuleType string

const (
	QualityNotNull     QualityRuleType = "not_null"
	QualityUnique      QualityRuleType = "unique"
	QualityRange       QualityRuleType = "range"
	QualityRegex       QualityRuleType = "regex"
	QualityCustom      QualityRuleType = "custom"
	QualityReferential QualityRuleType = "referential"
)

// QualitySeverity 质量严重程度
type QualitySeverity string

const (
	SeverityCritical QualitySeverity = "critical"
	SeverityHigh     QualitySeverity = "high"
	SeverityMedium   QualitySeverity = "medium"
	SeverityLow      QualitySeverity = "low"
)

// QualityCheckResult 质量检查结果
type QualityCheckResult struct {
	ID         string        `json:"id"`
	RuleID     string        `json:"rule_id"`
	ObjectID   string        `json:"object_id"`
	Status     QualityStatus `json:"status"`
	Passed     bool          `json:"passed"`
	TotalRows  int64         `json:"total_rows"`
	PassedRows int64         `json:"passed_rows"`
	FailedRows int64         `json:"failed_rows"`
	Score      float64       `json:"score"`
	Details    string        `json:"details,omitempty"`
	CheckedAt  time.Time     `json:"checked_at"`
}

// QualityStatus 质量状态
type QualityStatus string

const (
	QStatusPass QualityStatus = "pass"
	QStatusFail QualityStatus = "fail"
	QStatusWarn QualityStatus = "warn"
	QStatusSkip QualityStatus = "skip"
)

// QualityReport 质量报告
type QualityReport struct {
	ObjectID     string                `json:"object_id"`
	OverallScore float64               `json:"overall_score"`
	Results      []*QualityCheckResult `json:"results"`
	GeneratedAt  time.Time             `json:"generated_at"`
}

// DataLakeStats 数据湖统计
type DataLakeStats struct {
	TotalSources    int     `json:"total_sources"`
	OnlineSources   int     `json:"online_sources"`
	TotalObjects    int     `json:"total_objects"`
	TotalCatalogs   int     `json:"total_catalogs"`
	TotalSize       int64   `json:"total_size"`
	TotalLineages   int     `json:"total_lineages"`
	TotalRules      int     `json:"total_rules"`
	AvgQualityScore float64 `json:"avg_quality_score"`
}

// DataLake 统一数据湖
type DataLake struct {
	mu       sync.RWMutex
	sources  map[string]*DataSource
	objects  map[string]*DataObject
	catalogs map[string]*CatalogEntry
	lineages map[string]*LineageGraph
	rules    map[string]*QualityRule
	results  map[string][]*QualityCheckResult // key: objectID
	stats    *DataLakeStats
}

// NewDataLake 创建数据湖
func NewDataLake() *DataLake {
	return &DataLake{
		sources:  make(map[string]*DataSource),
		objects:  make(map[string]*DataObject),
		catalogs: make(map[string]*CatalogEntry),
		lineages: make(map[string]*LineageGraph),
		rules:    make(map[string]*QualityRule),
		results:  make(map[string][]*QualityCheckResult),
		stats:    &DataLakeStats{},
	}
}
