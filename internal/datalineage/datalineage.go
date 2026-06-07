// Package datalineage 数据血缘追踪核心实现
package datalineage

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// Manager 数据血缘管理器
type Manager struct {
	mu       sync.RWMutex
	nodes    map[string]*DataNode
	edges    map[string]*LineageEdge
	records  []*ProcessingRecord
	config   *Config
	dataFile string
}

// NewManager 创建管理器
func NewManager(dataFile string) *Manager {
	return &Manager{
		nodes:   make(map[string]*DataNode),
		edges:   make(map[string]*LineageEdge),
		records: make([]*ProcessingRecord, 0),
		config: &Config{
			MaxNodes:           10000,
			MaxDepth:           10,
			AutoClassify:       true,
			AuditRetentionDays: 365,
		},
		dataFile: dataFile,
	}
}

// Initialize 初始化
func (m *Manager) Initialize() error {
	return m.load()
}

// CreateNode 创建数据节点
func (m *Manager) CreateNode(node *DataNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.nodes[node.ID]; exists {
		return ErrNodeExists
	}
	if len(m.nodes) >= m.config.MaxNodes {
		return ErrMaxNodes
	}
	if !isValidSourceType(node.Type) {
		return ErrInvalidDataSource
	}

	node.CreatedAt = time.Now()
	node.UpdatedAt = time.Now()
	m.nodes[node.ID] = node
	return m.save()
}

// GetNode 获取数据节点
func (m *Manager) GetNode(id string) (*DataNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	node, ok := m.nodes[id]
	if !ok {
		return nil, ErrNodeNotFound
	}
	return node, nil
}

// ListNodes 列出数据节点
func (m *Manager) ListNodes(srcType DataSourceType, classification DataClassification) []*DataNode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*DataNode
	for _, node := range m.nodes {
		if srcType != "" && node.Type != srcType {
			continue
		}
		if classification != "" && node.Classification != classification {
			continue
		}
		result = append(result, node)
	}
	return result
}

// UpdateNode 更新数据节点
func (m *Manager) UpdateNode(id string, update *DataNode) (*DataNode, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, ok := m.nodes[id]
	if !ok {
		return nil, ErrNodeNotFound
	}

	if update.Name != "" {
		node.Name = update.Name
	}
	if update.Description != "" {
		node.Description = update.Description
	}
	if update.Classification != "" {
		node.Classification = update.Classification
	}
	if update.Owner != "" {
		node.Owner = update.Owner
	}
	if len(update.Tags) > 0 {
		node.Tags = update.Tags
	}
	node.UpdatedAt = time.Now()
	if err := m.save(); err != nil {
		return nil, err
	}
	return node, nil
}

// DeleteNode 删除数据节点
func (m *Manager) DeleteNode(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.nodes[id]; !ok {
		return ErrNodeNotFound
	}

	// 删除关联的边
	for edgeID, edge := range m.edges {
		if edge.SourceID == id || edge.TargetID == id {
			delete(m.edges, edgeID)
		}
	}
	delete(m.nodes, id)
	return m.save()
}

// CreateEdge 创建血缘关系
func (m *Manager) CreateEdge(edge *LineageEdge) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.edges[edge.ID]; exists {
		return ErrEdgeExists
	}
	if _, ok := m.nodes[edge.SourceID]; !ok {
		return fmt.Errorf("source node not found: %s", edge.SourceID)
	}
	if _, ok := m.nodes[edge.TargetID]; !ok {
		return fmt.Errorf("target node not found: %s", edge.TargetID)
	}

	// 检查循环
	if m.hasCycle(edge.SourceID, edge.TargetID) {
		return ErrCircularLineage
	}

	edge.CreatedAt = time.Now()
	m.edges[edge.ID] = edge
	return m.save()
}

// GetEdge 获取血缘关系
func (m *Manager) GetEdge(id string) (*LineageEdge, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	edge, ok := m.edges[id]
	if !ok {
		return nil, ErrEdgeNotFound
	}
	return edge, nil
}

// DeleteEdge 删除血缘关系
func (m *Manager) DeleteEdge(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.edges[id]; !ok {
		return ErrEdgeNotFound
	}
	delete(m.edges, id)
	return m.save()
}

// ListEdges 列出血缘关系
func (m *Manager) ListEdges(nodeID string) []*LineageEdge {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*LineageEdge
	for _, edge := range m.edges {
		if nodeID != "" && edge.SourceID != nodeID && edge.TargetID != nodeID {
			continue
		}
		result = append(result, edge)
	}
	return result
}

// GetLineageGraph 获取数据血缘图
func (m *Manager) GetLineageGraph(nodeID string, depth int) (*LineageGraph, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.nodes[nodeID]; !ok {
		return nil, ErrNodeNotFound
	}
	if depth <= 0 {
		depth = m.config.MaxDepth
	}

	visitedNodes := make(map[string]bool)
	visitedEdges := make(map[string]bool)
	var nodes []*DataNode
	var edges []*LineageEdge

	// 向前（上游）和向后（下游）遍历
	m.traverseUpstream(nodeID, depth, visitedNodes, visitedEdges, &nodes, &edges)
	m.traverseDownstream(nodeID, depth, visitedNodes, visitedEdges, &nodes, &edges)

	// 确保起始节点包含在内
	if !visitedNodes[nodeID] {
		nodes = append(nodes, m.nodes[nodeID])
	}

	return &LineageGraph{Nodes: nodes, Edges: edges}, nil
}

// ImpactAnalysis 影响分析：修改一个数据源会影响哪些下游
func (m *Manager) ImpactAnalysis(nodeID string, depth int) (*ImpactResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.nodes[nodeID]; !ok {
		return nil, ErrNodeNotFound
	}
	if depth <= 0 {
		depth = m.config.MaxDepth
	}

	visitedNodes := make(map[string]bool)
	visitedEdges := make(map[string]bool)
	var affected []*DataNode
	var edges []*LineageEdge

	m.traverseDownstream(nodeID, depth, visitedNodes, visitedEdges, &affected, &edges)

	return &ImpactResult{
		NodeID:     nodeID,
		Depth:      depth,
		Affected:   affected,
		Edges:      edges,
		TotalCount: len(affected),
	}, nil
}

// TraceSource 数据溯源：追踪数据质量问题的根源
func (m *Manager) TraceSource(nodeID string, depth int) (*TraceResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.nodes[nodeID]; !ok {
		return nil, ErrNodeNotFound
	}
	if depth <= 0 {
		depth = m.config.MaxDepth
	}

	visitedNodes := make(map[string]bool)
	visitedEdges := make(map[string]bool)
	var sources []*DataNode
	var edges []*LineageEdge

	m.traverseUpstream(nodeID, depth, visitedNodes, visitedEdges, &sources, &edges)

	return &TraceResult{
		NodeID:     nodeID,
		Depth:      depth,
		Sources:    sources,
		Edges:      edges,
		TotalCount: len(sources),
	}, nil
}

// AddProcessingRecord 添加处理记录（合规审计）
func (m *Manager) AddProcessingRecord(record *ProcessingRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.nodes[record.NodeID]; !ok {
		return ErrNodeNotFound
	}

	record.Timestamp = time.Now()
	m.records = append(m.records, record)
	return m.save()
}

// GetProcessingRecords 获取处理记录
func (m *Manager) GetProcessingRecords(nodeID string, regulation ComplianceRegulation) []*ProcessingRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*ProcessingRecord
	for _, rec := range m.records {
		if nodeID != "" && rec.NodeID != nodeID {
			continue
		}
		if regulation != "" && rec.Regulation != regulation {
			continue
		}
		result = append(result, rec)
	}
	return result
}

// GenerateComplianceReport 生成合规报告
func (m *Manager) GenerateComplianceReport(regulation ComplianceRegulation) map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalRecords := 0
	crossBorderCount := 0
	consentCount := 0
	nodesByClassification := make(map[string]int)

	for _, rec := range m.records {
		if regulation != "" && rec.Regulation != regulation {
			continue
		}
		totalRecords++
		if rec.CrossBorder {
			crossBorderCount++
		}
		if rec.ConsentObtained {
			consentCount++
		}
		if node, ok := m.nodes[rec.NodeID]; ok {
			nodesByClassification[string(node.Classification)]++
		}
	}

	return map[string]interface{}{
		"regulation":              regulation,
		"total_records":           totalRecords,
		"cross_border_count":      crossBorderCount,
		"consent_obtained_count":  consentCount,
		"nodes_by_classification": nodesByClassification,
		"generated_at":            time.Now(),
	}
}

// ManageClassification 管理数据分类标签
func (m *Manager) ManageClassification(nodeID string, classification DataClassification, tags []string) (*DataNode, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, ok := m.nodes[nodeID]
	if !ok {
		return nil, ErrNodeNotFound
	}

	node.Classification = classification
	if len(tags) > 0 {
		node.Tags = tags
	}
	node.UpdatedAt = time.Now()
	if err := m.save(); err != nil {
		return nil, err
	}
	return node, nil
}

// AutoCollectLineage 自动采集血缘（从SQL/配置解析）
func (m *Manager) AutoCollectLineage(records []AutoCollectRecord) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	collected := 0
	for _, rec := range records {
		// 确保节点存在
		if _, ok := m.nodes[rec.SourceID]; !ok {
			continue
		}
		if _, ok := m.nodes[rec.TargetID]; !ok {
			continue
		}

		edge := &LineageEdge{
			ID:          fmt.Sprintf("auto-%s-%s-%d", rec.SourceID, rec.TargetID, time.Now().UnixNano()),
			SourceID:    rec.SourceID,
			TargetID:    rec.TargetID,
			Type:        rec.EdgeType,
			Process:     rec.Process,
			SQL:         rec.SQL,
			Description: rec.Description,
			CreatedAt:   time.Now(),
		}
		m.edges[edge.ID] = edge
		collected++
	}

	if collected > 0 {
		if err := m.save(); err != nil {
			return collected, err
		}
	}
	return collected, nil
}

// GetStats 获取统计信息
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodesByType := make(map[string]int)
	nodesByClass := make(map[string]int)
	for _, node := range m.nodes {
		nodesByType[string(node.Type)]++
		nodesByClass[string(node.Classification)]++
	}

	edgesByType := make(map[string]int)
	for _, edge := range m.edges {
		edgesByType[string(edge.Type)]++
	}

	return map[string]interface{}{
		"total_nodes":    len(m.nodes),
		"total_edges":    len(m.edges),
		"total_records":  len(m.records),
		"nodes_by_type":  nodesByType,
		"nodes_by_class": nodesByClass,
		"edges_by_type":  edgesByType,
	}
}

// AutoCollectRecord 自动采集记录
type AutoCollectRecord struct {
	SourceID    string   `json:"source_id"`
	TargetID    string   `json:"target_id"`
	EdgeType    EdgeType `json:"edge_type"`
	Process     string   `json:"process"`
	SQL         string   `json:"sql,omitempty"`
	Description string   `json:"description,omitempty"`
}

// traverseDownstream 向下游遍历
func (m *Manager) traverseDownstream(nodeID string, depth int, visitedNodes map[string]bool, visitedEdges map[string]bool, nodes *[]*DataNode, edges *[]*LineageEdge) {
	if depth <= 0 || visitedNodes[nodeID] {
		return
	}
	visitedNodes[nodeID] = true

	for _, edge := range m.edges {
		if edge.SourceID == nodeID && !visitedEdges[edge.ID] {
			visitedEdges[edge.ID] = true
			*edges = append(*edges, edge)
			targetNode := m.nodes[edge.TargetID]
			if targetNode != nil && !visitedNodes[edge.TargetID] {
				*nodes = append(*nodes, targetNode)
			}
			m.traverseDownstream(edge.TargetID, depth-1, visitedNodes, visitedEdges, nodes, edges)
		}
	}
}

// traverseUpstream 向上游遍历
func (m *Manager) traverseUpstream(nodeID string, depth int, visitedNodes map[string]bool, visitedEdges map[string]bool, nodes *[]*DataNode, edges *[]*LineageEdge) {
	if depth <= 0 || visitedNodes[nodeID] {
		return
	}
	visitedNodes[nodeID] = true

	for _, edge := range m.edges {
		if edge.TargetID == nodeID && !visitedEdges[edge.ID] {
			visitedEdges[edge.ID] = true
			*edges = append(*edges, edge)
			sourceNode := m.nodes[edge.SourceID]
			if sourceNode != nil && !visitedNodes[edge.SourceID] {
				*nodes = append(*nodes, sourceNode)
			}
			m.traverseUpstream(edge.SourceID, depth-1, visitedNodes, visitedEdges, nodes, edges)
		}
	}
}

// hasCycle 检测是否会产生循环
func (m *Manager) hasCycle(from, to string) bool {
	if from == to {
		return true
	}
	visited := make(map[string]bool)
	return m.dfs(to, from, visited)
}

// dfs 深度优先搜索检测循环
func (m *Manager) dfs(current, target string, visited map[string]bool) bool {
	if current == target {
		return true
	}
	if visited[current] {
		return false
	}
	visited[current] = true

	for _, edge := range m.edges {
		if edge.SourceID == current {
			if m.dfs(edge.TargetID, target, visited) {
				return true
			}
		}
	}
	return false
}

func isValidSourceType(t DataSourceType) bool {
	switch t {
	case SourceAPI, SourceDatabase, SourceFile, SourceETL, SourceStream:
		return true
	}
	return false
}

// load 从文件加载
func (m *Manager) load() error {
	if m.dataFile == "" {
		return nil
	}
	data, err := os.ReadFile(m.dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var stored struct {
		Nodes   map[string]*DataNode    `json:"nodes"`
		Edges   map[string]*LineageEdge `json:"edges"`
		Records []*ProcessingRecord     `json:"records"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		return err
	}
	if stored.Nodes != nil {
		m.nodes = stored.Nodes
	}
	if stored.Edges != nil {
		m.edges = stored.Edges
	}
	if stored.Records != nil {
		m.records = stored.Records
	}
	return nil
}

// save 保存到文件
func (m *Manager) save() error {
	if m.dataFile == "" {
		return nil
	}
	data, err := json.MarshalIndent(struct {
		Nodes   map[string]*DataNode    `json:"nodes"`
		Edges   map[string]*LineageEdge `json:"edges"`
		Records []*ProcessingRecord     `json:"records"`
	}{m.nodes, m.edges, m.records}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.dataFile, data, 0644)
}
