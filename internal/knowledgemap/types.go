// Package knowledgemap 个人知识图谱管理
// 提供知识节点管理、关联关系、图谱可视化、智能检索、学习追踪等功能
package knowledgemap

import "time"

// NodeType 知识节点类型
type NodeType string

const (
	NodeTypeConcept NodeType = "concept" // 概念
	NodeTypeArticle NodeType = "article" // 文章
	NodeTypeNote    NodeType = "note"    // 笔记
	NodeTypeBook    NodeType = "book"    // 书籍
	NodeTypeCourse  NodeType = "course"  // 课程
	NodeTypeProject NodeType = "project" // 项目
	NodeTypeTool    NodeType = "tool"    // 工具
	NodeTypePerson  NodeType = "person"  // 人物
	NodeTypeCustom  NodeType = "custom"  // 自定义
)

// RelationType 关联关系类型
type RelationType string

const (
	RelationReference   RelationType = "reference"   // 引用
	RelationDependency  RelationType = "dependency"  // 依赖
	RelationSimilar     RelationType = "similar"     // 相似
	RelationBelongsTo   RelationType = "belongs_to"  // 属于
	RelationPartOf      RelationType = "part_of"     // 部分
	RelationRelated     RelationType = "related"     // 相关
	RelationContradicts RelationType = "contradicts" // 矛盾
	RelationSupports    RelationType = "supports"    // 支持
	RelationCustom      RelationType = "custom"      // 自定义
)

// ClassificationDimension 分类维度
type ClassificationDimension string

const (
	DimensionTopic   ClassificationDimension = "topic"   // 主题
	DimensionDomain  ClassificationDimension = "domain"  // 领域
	DimensionProject ClassificationDimension = "project" // 项目
)

// KnowledgeNode 知识节点
type KnowledgeNode struct {
	ID          string     `json:"id"`
	Title       string     `json:"title" binding:"required"`
	Content     string     `json:"content,omitempty"`
	Type        NodeType   `json:"type" binding:"required"`
	Tags        []string   `json:"tags,omitempty"`
	Source      string     `json:"source,omitempty"`
	SourceURL   string     `json:"source_url,omitempty"`
	Importance  int        `json:"importance"` // 1-5 重要度
	Mastery     int        `json:"mastery"`    // 0-100 掌握度
	ReviewCount int        `json:"review_count"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	LastReview  *time.Time `json:"last_review,omitempty"`
}

// NodeRelation 节点关联关系
type NodeRelation struct {
	ID          string       `json:"id"`
	SourceID    string       `json:"source_id" binding:"required"`
	TargetID    string       `json:"target_id" binding:"required"`
	Type        RelationType `json:"type" binding:"required"`
	Weight      float64      `json:"weight"` // 关联强度 0-1
	Description string       `json:"description,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
}

// Classification 知识分类
type Classification struct {
	ID        string                  `json:"id"`
	Name      string                  `json:"name" binding:"required"`
	Dimension ClassificationDimension `json:"dimension" binding:"required"`
	ParentID  string                  `json:"parent_id,omitempty"`
	NodeIDs   []string                `json:"node_ids,omitempty"`
	CreatedAt time.Time               `json:"created_at"`
}

// GraphData 图谱可视化数据
type GraphData struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// GraphNode 图谱节点
type GraphNode struct {
	ID    string   `json:"id"`
	Label string   `json:"label"`
	Type  NodeType `json:"type"`
	Size  int      `json:"size"`  // 节点大小(基于关联数)
	Color string   `json:"color"` // 节点颜色
	Tags  []string `json:"tags,omitempty"`
}

// GraphEdge 图谱边
type GraphEdge struct {
	Source string       `json:"source"`
	Target string       `json:"target"`
	Type   RelationType `json:"type"`
	Weight float64      `json:"weight"`
	Label  string       `json:"label,omitempty"`
}

// SearchQuery 搜索查询
type SearchQuery struct {
	Keyword    string   `json:"keyword,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Type       NodeType `json:"type,omitempty"`
	MinMastery int      `json:"min_mastery,omitempty"`
	MaxMastery int      `json:"max_mastery,omitempty"`
	Limit      int      `json:"limit,omitempty"`
	Offset     int      `json:"offset,omitempty"`
}

// SearchResult 搜索结果
type SearchResult struct {
	Nodes     []*KnowledgeNode `json:"nodes"`
	Total     int              `json:"total"`
	Relevance []float64        `json:"relevance,omitempty"` // 相关度得分
}

// LearningStats 学习统计
type LearningStats struct {
	TotalNodes     int              `json:"total_nodes"`
	TotalRelations int              `json:"total_relations"`
	NodesByType    map[NodeType]int `json:"nodes_by_type"`
	NodesByTag     map[string]int   `json:"nodes_by_tag"`
	GrowthTrend    []DailyGrowth    `json:"growth_trend"`
	ActiveDays     int              `json:"active_days"`
	AvgMastery     float64          `json:"avg_mastery"`
	ReviewPending  int              `json:"review_pending"`
}

// DailyGrowth 每日增长
type DailyGrowth struct {
	Date     string `json:"date"`
	NewNodes int    `json:"new_nodes"`
	Reviews  int    `json:"reviews"`
}

// ImportData 导入数据
type ImportData struct {
	Format    string `json:"format" binding:"required"` // markdown, json
	Content   string `json:"content" binding:"required"`
	Overwrite bool   `json:"overwrite,omitempty"`
}

// ExportData 导出数据
type ExportData struct {
	Format  string   `json:"format" binding:"required"` // markdown, json
	NodeIDs []string `json:"node_ids,omitempty"`        // 为空则导出全部
}

// MarkdownExport Markdown导出格式
type MarkdownExport struct {
	Title   string `json:"title"`
	Content string `json:"content"` // Markdown内容
}

// NodeCreateRequest 创建节点请求
type NodeCreateRequest struct {
	Title      string   `json:"title" binding:"required"`
	Content    string   `json:"content,omitempty"`
	Type       NodeType `json:"type" binding:"required"`
	Tags       []string `json:"tags,omitempty"`
	Source     string   `json:"source,omitempty"`
	SourceURL  string   `json:"source_url,omitempty"`
	Importance int      `json:"importance,omitempty"`
}

// NodeUpdateRequest 更新节点请求
type NodeUpdateRequest struct {
	Title      string   `json:"title,omitempty"`
	Content    string   `json:"content,omitempty"`
	Type       NodeType `json:"type,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Source     string   `json:"source,omitempty"`
	SourceURL  string   `json:"source_url,omitempty"`
	Importance int      `json:"importance,omitempty"`
	Mastery    int      `json:"mastery,omitempty"`
}

// RelationCreateRequest 创建关联请求
type RelationCreateRequest struct {
	SourceID    string       `json:"source_id" binding:"required"`
	TargetID    string       `json:"target_id" binding:"required"`
	Type        RelationType `json:"type" binding:"required"`
	Weight      float64      `json:"weight,omitempty"`
	Description string       `json:"description,omitempty"`
}

// ClassificationCreateRequest 创建分类请求
type ClassificationCreateRequest struct {
	Name      string                  `json:"name" binding:"required"`
	Dimension ClassificationDimension `json:"dimension" binding:"required"`
	ParentID  string                  `json:"parent_id,omitempty"`
}

// NodeAddToClassification 添加节点到分类请求
type NodeAddToClassification struct {
	NodeID string `json:"node_id" binding:"required"`
}

// ApiResponse 标准API响应
type ApiResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// DefaultNodeTypes 默认节点类型列表
func DefaultNodeTypes() []NodeType {
	return []NodeType{
		NodeTypeConcept, NodeTypeArticle, NodeTypeNote,
		NodeTypeBook, NodeTypeCourse, NodeTypeProject,
		NodeTypeTool, NodeTypePerson, NodeTypeCustom,
	}
}

// DefaultRelationTypes 默认关联类型列表
func DefaultRelationTypes() []RelationType {
	return []RelationType{
		RelationReference, RelationDependency, RelationSimilar,
		RelationBelongsTo, RelationPartOf, RelationRelated,
		RelationContradicts, RelationSupports, RelationCustom,
	}
}

// IsValidNodeType 检查节点类型是否有效
func IsValidNodeType(t NodeType) bool {
	for _, nt := range DefaultNodeTypes() {
		if nt == t {
			return true
		}
	}
	return false
}

// IsValidRelationType 检查关联类型是否有效
func IsValidRelationType(t RelationType) bool {
	for _, rt := range DefaultRelationTypes() {
		if rt == t {
			return true
		}
	}
	return false
}
