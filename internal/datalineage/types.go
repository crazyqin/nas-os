// Package datalineage 数据血缘追踪 - 数据全生命周期可追溯
package datalineage

import (
	"errors"
	"time"
)

// DataSourceType 数据源类型
type DataSourceType string

const (
	SourceAPI      DataSourceType = "api"
	SourceDatabase DataSourceType = "database"
	SourceFile     DataSourceType = "file"
	SourceETL      DataSourceType = "etl"
	SourceStream   DataSourceType = "stream"
)

// DataClassification 数据分类
type DataClassification string

const (
	ClassPublic       DataClassification = "public"
	ClassInternal     DataClassification = "internal"
	ClassConfidential DataClassification = "confidential"
	ClassRestricted   DataClassification = "restricted"
	ClassPII          DataClassification = "pii"
)

// ComplianceRegulation 合规法规
type ComplianceRegulation string

const (
	RegGDPR  ComplianceRegulation = "gdpr"
	RegCCPA  ComplianceRegulation = "ccpa"
	RegHIPAA ComplianceRegulation = "hipaa"
	RegSOX   ComplianceRegulation = "sox"
)

// ProcessingPurpose 数据处理目的
type ProcessingPurpose string

const (
	PurposeAnalytics    ProcessingPurpose = "analytics"
	PurposeStorage      ProcessingPurpose = "storage"
	PurposeTransfer     ProcessingPurpose = "transfer"
	PurposeDeletion     ProcessingPurpose = "deletion"
	PurposeTransformation ProcessingPurpose = "transformation"
	PurposeAggregation  ProcessingPurpose = "aggregation"
)

// DataNode 数据节点（数据资产）
type DataNode struct {
	ID               string             `json:"id"`
	Name             string             `json:"name"`
	Description      string             `json:"description,omitempty"`
	Type             DataSourceType     `json:"type"`
	Location         string             `json:"location,omitempty"`
	Database         string             `json:"database,omitempty"`
	Schema           string             `json:"schema,omitempty"`
	Table            string             `json:"table,omitempty"`
	Columns          []string           `json:"columns,omitempty"`
	Classification   DataClassification `json:"classification"`
	Tags             []string           `json:"tags,omitempty"`
	Owner            string             `json:"owner,omitempty"`
	CreatedAt        time.Time          `json:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at"`
}

// EdgeType 边类型
type EdgeType string

const (
	EdgeDirect        EdgeType = "direct"
	EdgeTransform     EdgeType = "transform"
	EdgeAggregate     EdgeType = "aggregate"
	EdgeFilter        EdgeType = "filter"
	EdgeJoin          EdgeType = "join"
	EdgeDerived       EdgeType = "derived"
)

// LineageEdge 血缘边（数据流转关系）
type LineageEdge struct {
	ID          string    `json:"id"`
	SourceID    string    `json:"source_id"`
	TargetID    string    `json:"target_id"`
	Type        EdgeType  `json:"type"`
	Process     string    `json:"process,omitempty"`
	SQL         string    `json:"sql,omitempty"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// ProcessingRecord 处理记录（合规审计用）
type ProcessingRecord struct {
	ID             string               `json:"id"`
	NodeID         string               `json:"node_id"`
	Operation      ProcessingPurpose    `json:"operation"`
	Regulation     ComplianceRegulation `json:"regulation"`
	Purpose        string               `json:"purpose"`
	Processor      string               `json:"processor"`
	LegalBasis     string               `json:"legal_basis,omitempty"`
	Retention      string               `json:"retention,omitempty"`
	CrossBorder    bool                 `json:"cross_border"`
	DestCountry    string               `json:"dest_country,omitempty"`
	ConsentObtained bool               `json:"consent_obtained"`
	Timestamp      time.Time            `json:"timestamp"`
}

// ImpactResult 影响分析结果
type ImpactResult struct {
	NodeID     string      `json:"node_id"`
	Depth      int         `json:"depth"`
	Affected   []*DataNode `json:"affected"`
	Edges      []*LineageEdge `json:"edges"`
	TotalCount int         `json:"total_count"`
}

// TraceResult 溯源结果
type TraceResult struct {
	NodeID     string      `json:"node_id"`
	Depth      int         `json:"depth"`
	Sources    []*DataNode `json:"sources"`
	Edges      []*LineageEdge `json:"edges"`
	TotalCount int         `json:"total_count"`
}

// LineageGraph 血缘图
type LineageGraph struct {
	Nodes []*DataNode   `json:"nodes"`
	Edges []*LineageEdge `json:"edges"`
}

// Config 配置
type Config struct {
	MaxNodes           int  `json:"max_nodes"`
	MaxDepth           int  `json:"_max_depth"`
	AutoClassify       bool `json:"auto_classify"`
	AuditRetentionDays int  `json:"audit_retention_days"`
}

var (
	ErrNodeNotFound      = errors.New("data node not found")
	ErrNodeExists        = errors.New("data node already exists")
	ErrEdgeNotFound      = errors.New("lineage edge not found")
	ErrEdgeExists        = errors.New("lineage edge already exists")
	ErrMaxNodes          = errors.New("max nodes reached")
	ErrCircularLineage   = errors.New("circular lineage detected")
	ErrInvalidDataSource = errors.New("invalid data source type")
)
