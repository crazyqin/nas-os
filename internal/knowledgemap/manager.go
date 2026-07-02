// Package knowledgemap 业务逻辑实现
package knowledgemap

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ==================== 节点管理 ====================

// ListNodes 列出节点.
func (m *Manager) ListNodes(nodeType, tag string) []*KnowledgeNode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]*KnowledgeNode, 0)
	for _, node := range m.nodes {
		if nodeType != "" && string(node.Type) != nodeType {
			continue
		}
		if tag != "" && !containsTag(node.Tags, tag) {
			continue
		}
		nodes = append(nodes, node)
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].UpdatedAt.After(nodes[j].UpdatedAt)
	})
	return nodes
}

// CreateNode 创建节点.
func (m *Manager) CreateNode(req *NodeCreateRequest) (*KnowledgeNode, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	node := &KnowledgeNode{
		ID:         generateID("node"),
		Title:      req.Title,
		Content:    req.Content,
		Type:       req.Type,
		Tags:       req.Tags,
		Source:     req.Source,
		SourceURL:  req.SourceURL,
		Importance: req.Importance,
		Mastery:    0,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if node.Importance < 1 || node.Importance > 5 {
		node.Importance = 3
	}
	if node.Tags == nil {
		node.Tags = make([]string, 0)
	}

	m.nodes[node.ID] = node
	return node, nil
}

// GetNode 获取节点.
func (m *Manager) GetNode(id string) (*KnowledgeNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	node, ok := m.nodes[id]
	if !ok {
		return nil, fmt.Errorf("node not found: %s", id)
	}
	return node, nil
}

// UpdateNode 更新节点.
func (m *Manager) UpdateNode(id string, req *NodeUpdateRequest) (*KnowledgeNode, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, ok := m.nodes[id]
	if !ok {
		return nil, fmt.Errorf("node not found: %s", id)
	}

	if req.Title != "" {
		node.Title = req.Title
	}
	if req.Content != "" {
		node.Content = req.Content
	}
	if req.Type != "" && IsValidNodeType(req.Type) {
		node.Type = req.Type
	}
	if req.Tags != nil {
		node.Tags = req.Tags
	}
	if req.Source != "" {
		node.Source = req.Source
	}
	if req.SourceURL != "" {
		node.SourceURL = req.SourceURL
	}
	if req.Importance >= 1 && req.Importance <= 5 {
		node.Importance = req.Importance
	}
	if req.Mastery >= 0 && req.Mastery <= 100 {
		node.Mastery = req.Mastery
	}
	node.UpdatedAt = time.Now()
	return node, nil
}

// DeleteNode 删除节点.
func (m *Manager) DeleteNode(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.nodes[id]; !ok {
		return fmt.Errorf("node not found: %s", id)
	}

	// 删除关联的关系
	for relID, rel := range m.relations {
		if rel.SourceID == id || rel.TargetID == id {
			delete(m.relations, relID)
		}
	}

	// 从分类中移除
	for _, class := range m.classifications {
		class.NodeIDs = removeString(class.NodeIDs, id)
	}

	// 从复习队列中移除
	m.reviewQueue = removeString(m.reviewQueue, id)

	delete(m.nodes, id)
	return nil
}

// ReviewNode 记录复习.
func (m *Manager) ReviewNode(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, ok := m.nodes[id]
	if !ok {
		return fmt.Errorf("node not found: %s", id)
	}

	now := time.Now()
	node.ReviewCount++
	node.LastReview = &now
	node.UpdatedAt = now

	if node.Mastery < 100 {
		node.Mastery += 10
		if node.Mastery > 100 {
			node.Mastery = 100
		}
	}

	m.reviewQueue = removeString(m.reviewQueue, id)
	return nil
}

// ==================== 关联关系 ====================

// ListRelations 列出关系.
func (m *Manager) ListRelations(relType string) []*NodeRelation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	relations := make([]*NodeRelation, 0)
	for _, rel := range m.relations {
		if relType != "" && string(rel.Type) != relType {
			continue
		}
		relations = append(relations, rel)
	}
	return relations
}

// CreateRelation 创建关系.
func (m *Manager) CreateRelation(req *RelationCreateRequest) (*NodeRelation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.nodes[req.SourceID]; !ok {
		return nil, fmt.Errorf("source node not found: %s", req.SourceID)
	}
	if _, ok := m.nodes[req.TargetID]; !ok {
		return nil, fmt.Errorf("target node not found: %s", req.TargetID)
	}

	// 检查是否已存在相同关系
	for _, rel := range m.relations {
		if rel.SourceID == req.SourceID && rel.TargetID == req.TargetID && rel.Type == req.Type {
			return nil, fmt.Errorf("relation already exists")
		}
	}

	weight := req.Weight
	if weight <= 0 || weight > 1 {
		weight = 0.5
	}

	rel := &NodeRelation{
		ID:          generateID("rel"),
		SourceID:    req.SourceID,
		TargetID:    req.TargetID,
		Type:        req.Type,
		Weight:      weight,
		Description: req.Description,
		CreatedAt:   time.Now(),
	}

	m.relations[rel.ID] = rel
	return rel, nil
}

// DeleteRelation 删除关系.
func (m *Manager) DeleteRelation(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.relations[id]; !ok {
		return fmt.Errorf("relation not found: %s", id)
	}
	delete(m.relations, id)
	return nil
}

// GetNodeRelated 获取节点关联.
func (m *Manager) GetNodeRelated(nodeID, relType string) []*KnowledgeNode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.nodes[nodeID]; !ok {
		return nil
	}

	relatedIDs := make(map[string]bool)
	for _, rel := range m.relations {
		if relType != "" && string(rel.Type) != relType {
			continue
		}
		if rel.SourceID == nodeID {
			relatedIDs[rel.TargetID] = true
		}
		if rel.TargetID == nodeID {
			relatedIDs[rel.SourceID] = true
		}
	}

	nodes := make([]*KnowledgeNode, 0)
	for id := range relatedIDs {
		if node, ok := m.nodes[id]; ok {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// ==================== 智能检索 ====================

// SearchNodes 搜索节点.
func (m *Manager) SearchNodes(query *SearchQuery) *SearchResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([]*KnowledgeNode, 0)
	relevances := make([]float64, 0)
	keyword := strings.ToLower(query.Keyword)

	for _, node := range m.nodes {
		if query.Type != "" && node.Type != query.Type {
			continue
		}
		if query.MinMastery > 0 && node.Mastery < query.MinMastery {
			continue
		}
		if query.MaxMastery > 0 && node.Mastery > query.MaxMastery {
			continue
		}

		relevance := 0.0
		if keyword != "" {
			titleMatch := strings.Contains(strings.ToLower(node.Title), keyword)
			contentMatch := strings.Contains(strings.ToLower(node.Content), keyword)
			if titleMatch {
				relevance += 0.6
			}
			if contentMatch {
				relevance += 0.4
			}
			if relevance == 0 {
				continue
			}
		} else {
			relevance = 1.0
		}

		if len(query.Tags) > 0 {
			tagMatch := 0
			for _, tag := range query.Tags {
				if containsTag(node.Tags, tag) {
					tagMatch++
				}
			}
			if tagMatch == 0 {
				continue
			}
			relevance += float64(tagMatch) / float64(len(query.Tags)) * 0.3
		}

		results = append(results, node)
		relevances = append(relevances, relevance)
	}

	sort.Slice(results, func(i, j int) bool {
		return relevances[i] > relevances[j]
	})

	total := len(results)
	offset := query.Offset
	if offset > total {
		offset = total
	}
	end := offset + query.Limit
	if end > total {
		end = total
	}

	return &SearchResult{
		Nodes:     results[offset:end],
		Total:     total,
		Relevance: relevances[offset:end],
	}
}

// SearchByTags 按标签搜索.
func (m *Manager) SearchByTags(tags []string) []*KnowledgeNode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]*KnowledgeNode, 0)
	for _, node := range m.nodes {
		for _, tag := range tags {
			if containsTag(node.Tags, strings.TrimSpace(tag)) {
				nodes = append(nodes, node)
				break
			}
		}
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].UpdatedAt.After(nodes[j].UpdatedAt)
	})
	return nodes
}

// ==================== 分类管理 ====================

// ListClassifications 列出分类.
func (m *Manager) ListClassifications(dimension string) []*Classification {
	m.mu.RLock()
	defer m.mu.RUnlock()

	classifications := make([]*Classification, 0)
	for _, class := range m.classifications {
		if dimension != "" && string(class.Dimension) != dimension {
			continue
		}
		classifications = append(classifications, class)
	}
	return classifications
}

// CreateClassification 创建分类.
func (m *Manager) CreateClassification(req *ClassificationCreateRequest) (*Classification, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.ParentID != "" {
		if _, ok := m.classifications[req.ParentID]; !ok {
			return nil, fmt.Errorf("parent classification not found: %s", req.ParentID)
		}
	}

	class := &Classification{
		ID:        generateID("cls"),
		Name:      req.Name,
		Dimension: req.Dimension,
		ParentID:  req.ParentID,
		NodeIDs:   make([]string, 0),
		CreatedAt: time.Now(),
	}

	m.classifications[class.ID] = class
	return class, nil
}

// GetClassification 获取分类.
func (m *Manager) GetClassification(id string) (*Classification, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	class, ok := m.classifications[id]
	if !ok {
		return nil, fmt.Errorf("classification not found: %s", id)
	}
	return class, nil
}

// DeleteClassification 删除分类.
func (m *Manager) DeleteClassification(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.classifications[id]; !ok {
		return fmt.Errorf("classification not found: %s", id)
	}
	delete(m.classifications, id)
	return nil
}

// AddNodeToClassification 添加节点到分类.
func (m *Manager) AddNodeToClassification(classID, nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	class, ok := m.classifications[classID]
	if !ok {
		return fmt.Errorf("classification not found: %s", classID)
	}
	if _, ok := m.nodes[nodeID]; !ok {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	for _, id := range class.NodeIDs {
		if id == nodeID {
			return nil
		}
	}

	class.NodeIDs = append(class.NodeIDs, nodeID)
	return nil
}

// RemoveNodeFromClassification 从分类移除节点.
func (m *Manager) RemoveNodeFromClassification(classID, nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	class, ok := m.classifications[classID]
	if !ok {
		return fmt.Errorf("classification not found: %s", classID)
	}
	class.NodeIDs = removeString(class.NodeIDs, nodeID)
	return nil
}

// ==================== 图谱可视化 ====================

// GetGraph 获取图谱数据.
func (m *Manager) GetGraph(maxNodes int) *GraphData {
	m.mu.RLock()
	defer m.mu.RUnlock()

	graphNodes := make([]GraphNode, 0)
	nodeIDSet := make(map[string]bool)

	count := 0
	for _, node := range m.nodes {
		if count >= maxNodes {
			break
		}

		size := 1
		for _, rel := range m.relations {
			if rel.SourceID == node.ID || rel.TargetID == node.ID {
				size++
			}
		}

		graphNodes = append(graphNodes, GraphNode{
			ID:    node.ID,
			Label: node.Title,
			Type:  node.Type,
			Size:  size,
			Color: nodeTypeColor(node.Type),
			Tags:  node.Tags,
		})
		nodeIDSet[node.ID] = true
		count++
	}

	graphEdges := make([]GraphEdge, 0)
	for _, rel := range m.relations {
		if nodeIDSet[rel.SourceID] && nodeIDSet[rel.TargetID] {
			graphEdges = append(graphEdges, GraphEdge{
				Source: rel.SourceID,
				Target: rel.TargetID,
				Type:   rel.Type,
				Weight: rel.Weight,
				Label:  string(rel.Type),
			})
		}
	}

	return &GraphData{Nodes: graphNodes, Edges: graphEdges}
}

// GetSubgraph 获取节点子图.
func (m *Manager) GetSubgraph(nodeID string, depth int) (*GraphData, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.nodes[nodeID]; !ok {
		return nil, fmt.Errorf("node not found: %s", nodeID)
	}

	visited := make(map[string]bool)
	graphNodes := make([]GraphNode, 0)
	graphEdges := make([]GraphEdge, 0)

	var traverse func(id string, d int)
	traverse = func(id string, d int) {
		if d < 0 || visited[id] {
			return
		}
		visited[id] = true

		if node, ok := m.nodes[id]; ok {
			size := 1
			for _, rel := range m.relations {
				if rel.SourceID == id || rel.TargetID == id {
					size++
				}
			}
			graphNodes = append(graphNodes, GraphNode{
				ID:    node.ID,
				Label: node.Title,
				Type:  node.Type,
				Size:  size,
				Color: nodeTypeColor(node.Type),
				Tags:  node.Tags,
			})
		}

		for _, rel := range m.relations {
			neighborID := ""
			if rel.SourceID == id {
				neighborID = rel.TargetID
			} else if rel.TargetID == id {
				neighborID = rel.SourceID
			}
			if neighborID != "" && !visited[neighborID] {
				graphEdges = append(graphEdges, GraphEdge{
					Source: rel.SourceID,
					Target: rel.TargetID,
					Type:   rel.Type,
					Weight: rel.Weight,
					Label:  string(rel.Type),
				})
				traverse(neighborID, d-1)
			}
		}
	}

	traverse(nodeID, depth)
	return &GraphData{Nodes: graphNodes, Edges: graphEdges}, nil
}

// ==================== 导入导出 ====================

// ImportData 导入数据.
func (m *Manager) ImportData(req *ImportData) (int, error) {
	switch req.Format {
	case "json":
		return m.importJSON(req.Content, req.Overwrite)
	case "markdown":
		return m.importMarkdown(req.Content, req.Overwrite)
	default:
		return 0, fmt.Errorf("unsupported format: %s", req.Format)
	}
}

func (m *Manager) importJSON(content string, overwrite bool) (int, error) {
	var nodes []KnowledgeNode
	if err := json.Unmarshal([]byte(content), &nodes); err != nil {
		return 0, fmt.Errorf("invalid JSON: %v", err)
	}

	count := 0
	for _, node := range nodes {
		m.mu.Lock()
		if node.ID == "" {
			node.ID = generateID("node")
		}
		if _, ok := m.nodes[node.ID]; ok && !overwrite {
			m.mu.Unlock()
			continue
		}
		node.CreatedAt = time.Now()
		node.UpdatedAt = time.Now()
		if node.Tags == nil {
			node.Tags = make([]string, 0)
		}
		m.nodes[node.ID] = &node
		m.mu.Unlock()
		count++
	}

	return count, nil
}

func (m *Manager) importMarkdown(content string, overwrite bool) (int, error) {
	lines := strings.Split(content, "\n")
	count := 0

	var currentNode *KnowledgeNode
	flushNode := func() {
		if currentNode != nil {
			m.mu.Lock()
			if currentNode.ID == "" {
				currentNode.ID = generateID("node")
			}
			currentNode.CreatedAt = time.Now()
			currentNode.UpdatedAt = time.Now()
			m.nodes[currentNode.ID] = currentNode
			m.mu.Unlock()
			count++
			currentNode = nil
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "# ") {
			flushNode()
			currentNode = &KnowledgeNode{
				Title: strings.TrimPrefix(trimmed, "# "),
				Type:  NodeTypeNote,
				Tags:  make([]string, 0),
			}
		} else if currentNode != nil {
			if strings.HasPrefix(trimmed, "tags:") {
				tagStr := strings.TrimPrefix(trimmed, "tags:")
				tags := strings.Split(tagStr, ",")
				for _, t := range tags {
					t = strings.TrimSpace(t)
					if t != "" {
						currentNode.Tags = append(currentNode.Tags, t)
					}
				}
			} else if strings.HasPrefix(trimmed, "source:") {
				currentNode.Source = strings.TrimSpace(strings.TrimPrefix(trimmed, "source:"))
			} else if strings.HasPrefix(trimmed, "type:") {
				typeStr := strings.TrimSpace(strings.TrimPrefix(trimmed, "type:"))
				if IsValidNodeType(NodeType(typeStr)) {
					currentNode.Type = NodeType(typeStr)
				}
			} else {
				if currentNode.Content != "" {
					currentNode.Content += "\n"
				}
				currentNode.Content += line
			}
		}
	}

	flushNode()
	return count, nil
}

// ExportData 导出数据.
func (m *Manager) ExportData(req *ExportData) (interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]*KnowledgeNode, 0)
	if len(req.NodeIDs) > 0 {
		for _, id := range req.NodeIDs {
			if node, ok := m.nodes[id]; ok {
				nodes = append(nodes, node)
			}
		}
	} else {
		for _, node := range m.nodes {
			nodes = append(nodes, node)
		}
	}

	switch req.Format {
	case "json":
		return nodes, nil
	case "markdown":
		return exportToMarkdown(nodes), nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", req.Format)
	}
}

func exportToMarkdown(nodes []*KnowledgeNode) *MarkdownExport {
	var sb strings.Builder
	sb.WriteString("# Knowledge Map Export\n\n")

	for _, node := range nodes {
		fmt.Fprintf(&sb, "## %s\n\n", node.Title)
		fmt.Fprintf(&sb, "type: %s\n", node.Type)
		if len(node.Tags) > 0 {
			fmt.Fprintf(&sb, "tags: %s\n", strings.Join(node.Tags, ", "))
		}
		if node.Source != "" {
			fmt.Fprintf(&sb, "source: %s\n", node.Source)
		}
		sb.WriteString("\n")
		if node.Content != "" {
			sb.WriteString(node.Content + "\n")
		}
		sb.WriteString("\n---\n\n")
	}

	return &MarkdownExport{
		Title:   "Knowledge Map Export",
		Content: sb.String(),
	}
}

// ==================== 学习追踪 ====================

// GetStats 获取学习统计.
func (m *Manager) GetStats() *LearningStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &LearningStats{
		TotalNodes:     len(m.nodes),
		TotalRelations: len(m.relations),
		NodesByType:    make(map[NodeType]int),
		NodesByTag:     make(map[string]int),
	}

	totalMastery := 0
	for _, node := range m.nodes {
		stats.NodesByType[node.Type]++
		for _, tag := range node.Tags {
			stats.NodesByTag[tag]++
		}
		totalMastery += node.Mastery
		if node.Mastery < 50 {
			stats.ReviewPending++
		}
	}

	if stats.TotalNodes > 0 {
		stats.AvgMastery = float64(totalMastery) / float64(stats.TotalNodes)
	}

	// 计算活跃天数
	activeDays := make(map[string]bool)
	for _, node := range m.nodes {
		activeDays[node.CreatedAt.Format("2006-01-02")] = true
		if node.LastReview != nil {
			activeDays[node.LastReview.Format("2006-01-02")] = true
		}
	}
	stats.ActiveDays = len(activeDays)

	return stats
}

// GetGrowthTrend 获取增长趋势.
func (m *Manager) GetGrowthTrend(days int) []DailyGrowth {
	m.mu.RLock()
	defer m.mu.RUnlock()

	trend := make([]DailyGrowth, 0)
	now := time.Now()

	for i := days - 1; i >= 0; i-- {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		growth := DailyGrowth{Date: date}

		for _, node := range m.nodes {
			if node.CreatedAt.Format("2006-01-02") == date {
				growth.NewNodes++
			}
			if node.LastReview != nil && node.LastReview.Format("2006-01-02") == date {
				growth.Reviews++
			}
		}

		trend = append(trend, growth)
	}

	return trend
}

// GetPendingReviews 获取待复习节点.
func (m *Manager) GetPendingReviews() []*KnowledgeNode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]*KnowledgeNode, 0)
	for _, node := range m.nodes {
		if node.Mastery < 50 {
			nodes = append(nodes, node)
		}
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Mastery < nodes[j].Mastery
	})

	return nodes
}

// ==================== 辅助函数 ====================

func generateID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) []string {
	result := make([]string, 0)
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}

func nodeTypeColor(t NodeType) string {
	colors := map[NodeType]string{
		NodeTypeConcept: "#4A90D9",
		NodeTypeArticle: "#50C878",
		NodeTypeNote:    "#FFD700",
		NodeTypeBook:    "#FF6B6B",
		NodeTypeCourse:  "#9B59B6",
		NodeTypeProject: "#E67E22",
		NodeTypeTool:    "#1ABC9C",
		NodeTypePerson:  "#E91E63",
		NodeTypeCustom:  "#95A5A6",
	}
	if color, ok := colors[t]; ok {
		return color
	}
	return "#95A5A6"
}
