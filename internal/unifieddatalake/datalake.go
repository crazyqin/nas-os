package unifieddatalake

import (
	"fmt"
	"time"
)

// ==================== 数据源管理 ====================

// AddSource 添加数据源
func (dl *DataLake) AddSource(source *DataSource) error {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	if source.ID == "" {
		source.ID = generateID()
	}
	source.CreatedAt = time.Now()
	source.UpdatedAt = time.Now()
	if source.Status == "" {
		source.Status = DSStatusOnline
	}
	dl.sources[source.ID] = source
	dl.updateStats()
	return nil
}

// RemoveSource 移除数据源
func (dl *DataLake) RemoveSource(id string) error {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	if _, ok := dl.sources[id]; !ok {
		return fmt.Errorf("source %s not found", id)
	}
	delete(dl.sources, id)
	dl.updateStats()
	return nil
}

// GetSource 获取数据源
func (dl *DataLake) GetSource(id string) (*DataSource, bool) {
	dl.mu.RLock()
	defer dl.mu.RUnlock()
	s, ok := dl.sources[id]
	return s, ok
}

// ListSources 列出所有数据源
func (dl *DataLake) ListSources() []*DataSource {
	dl.mu.RLock()
	defer dl.mu.RUnlock()
	result := make([]*DataSource, 0, len(dl.sources))
	for _, s := range dl.sources {
		result = append(result, s)
	}
	return result
}

// UpdateSource 更新数据源
func (dl *DataLake) UpdateSource(source *DataSource) error {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	if _, ok := dl.sources[source.ID]; !ok {
		return fmt.Errorf("source %s not found", source.ID)
	}
	source.UpdatedAt = time.Now()
	dl.sources[source.ID] = source
	dl.updateStats()
	return nil
}

// ==================== 数据对象管理 ====================

// RegisterObject 注册数据对象
func (dl *DataLake) RegisterObject(obj *DataObject) error {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	if _, ok := dl.sources[obj.SourceID]; !ok {
		return fmt.Errorf("source %s not found", obj.SourceID)
	}
	if obj.ID == "" {
		obj.ID = generateID()
	}
	now := time.Now()
	obj.CreatedAt = now
	obj.ModifiedAt = now
	obj.AccessedAt = now
	dl.objects[obj.ID] = obj
	dl.updateStats()
	return nil
}

// GetObject 获取数据对象
func (dl *DataLake) GetObject(id string) (*DataObject, bool) {
	dl.mu.RLock()
	defer dl.mu.RUnlock()
	obj, ok := dl.objects[id]
	return obj, ok
}

// ListObjects 列出数据对象（可按sourceID过滤）
func (dl *DataLake) ListObjects(sourceID string) []*DataObject {
	dl.mu.RLock()
	defer dl.mu.RUnlock()
	result := make([]*DataObject, 0)
	for _, obj := range dl.objects {
		if sourceID == "" || obj.SourceID == sourceID {
			result = append(result, obj)
		}
	}
	return result
}

// RemoveObject 移除数据对象
func (dl *DataLake) RemoveObject(id string) error {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	if _, ok := dl.objects[id]; !ok {
		return fmt.Errorf("object %s not found", id)
	}
	delete(dl.objects, id)
	// 同时移除对应的目录条目
	for cid, entry := range dl.catalogs {
		if entry.ObjectID == id {
			delete(dl.catalogs, cid)
		}
	}
	dl.updateStats()
	return nil
}

// ==================== 数据目录管理 ====================

// AddCatalogEntry 添加目录条目
func (dl *DataLake) AddCatalogEntry(entry *CatalogEntry) error {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	if _, ok := dl.objects[entry.ObjectID]; !ok {
		return fmt.Errorf("object %s not found", entry.ObjectID)
	}
	if entry.ID == "" {
		entry.ID = generateID()
	}
	entry.Version = 1
	entry.CreatedAt = time.Now()
	entry.UpdatedAt = time.Now()
	dl.catalogs[entry.ID] = entry
	dl.updateStats()
	return nil
}

// GetCatalogEntry 获取目录条目
func (dl *DataLake) GetCatalogEntry(id string) (*CatalogEntry, bool) {
	dl.mu.RLock()
	defer dl.mu.RUnlock()
	entry, ok := dl.catalogs[id]
	return entry, ok
}

// ListCatalogEntries 列出目录条目
func (dl *DataLake) ListCatalogEntries(category string) []*CatalogEntry {
	dl.mu.RLock()
	defer dl.mu.RUnlock()
	result := make([]*CatalogEntry, 0)
	for _, entry := range dl.catalogs {
		if category == "" || entry.Category == category {
			result = append(result, entry)
		}
	}
	return result
}

// UpdateCatalogEntry 更新目录条目
func (dl *DataLake) UpdateCatalogEntry(entry *CatalogEntry) error {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	existing, ok := dl.catalogs[entry.ID]
	if !ok {
		return fmt.Errorf("catalog entry %s not found", entry.ID)
	}
	entry.Version = existing.Version + 1
	entry.UpdatedAt = time.Now()
	dl.catalogs[entry.ID] = entry
	return nil
}

// SearchCatalog 搜索目录（按名称/标签）
func (dl *DataLake) SearchCatalog(query string) []*CatalogEntry {
	dl.mu.RLock()
	defer dl.mu.RUnlock()
	result := make([]*CatalogEntry, 0)
	for _, entry := range dl.catalogs {
		if matchSearch(entry, query) {
			result = append(result, entry)
		}
	}
	return result
}

func matchSearch(entry *CatalogEntry, query string) bool {
	if query == "" {
		return true
	}
	if containsIgnoreCase(entry.Name, query) || containsIgnoreCase(entry.Description, query) {
		return true
	}
	for _, v := range entry.Tags {
		if containsIgnoreCase(v, query) {
			return true
		}
	}
	return false
}

func containsIgnoreCase(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	s = toLower(s)
	sub = toLower(sub)
	return containsStr(s, sub)
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ==================== 数据血缘 ====================

// CreateLineage 创建血缘图
func (dl *DataLake) CreateLineage(graph *LineageGraph) error {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	if graph.ID == "" {
		graph.ID = generateID()
	}
	graph.CreatedAt = time.Now()
	graph.UpdatedAt = time.Now()
	dl.lineages[graph.ID] = graph
	dl.updateStats()
	return nil
}

// GetLineage 获取血缘图
func (dl *DataLake) GetLineage(id string) (*LineageGraph, bool) {
	dl.mu.RLock()
	defer dl.mu.RUnlock()
	graph, ok := dl.lineages[id]
	return graph, ok
}

// ListLineages 列出所有血缘图
func (dl *DataLake) ListLineages() []*LineageGraph {
	dl.mu.RLock()
	defer dl.mu.RUnlock()
	result := make([]*LineageGraph, 0, len(dl.lineages))
	for _, g := range dl.lineages {
		result = append(result, g)
	}
	return result
}

// AddLineageNode 添加血缘节点
func (dl *DataLake) AddLineageNode(lineageID string, node *LineageNode) error {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	graph, ok := dl.lineages[lineageID]
	if !ok {
		return fmt.Errorf("lineage %s not found", lineageID)
	}
	if node.ID == "" {
		node.ID = generateID()
	}
	node.CreatedAt = time.Now()
	graph.Nodes = append(graph.Nodes, node)
	graph.UpdatedAt = time.Now()
	return nil
}

// AddLineageEdge 添加血缘边
func (dl *DataLake) AddLineageEdge(lineageID string, edge *LineageEdge) error {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	graph, ok := dl.lineages[lineageID]
	if !ok {
		return fmt.Errorf("lineage %s not found", lineageID)
	}
	if edge.ID == "" {
		edge.ID = generateID()
	}
	edge.CreatedAt = time.Now()
	graph.Edges = append(graph.Edges, edge)
	graph.UpdatedAt = time.Now()
	return nil
}

// GetLineageByObject 根据对象ID获取血缘
func (dl *DataLake) GetLineageByObject(objectID string) []*LineageGraph {
	dl.mu.RLock()
	defer dl.mu.RUnlock()
	result := make([]*LineageGraph, 0)
	for _, graph := range dl.lineages {
		for _, node := range graph.Nodes {
			if node.ObjectID == objectID {
				result = append(result, graph)
				break
			}
		}
	}
	return result
}

// GetUpstream 获取上游节点
func (dl *DataLake) GetUpstream(lineageID, nodeID string) ([]*LineageNode, error) {
	dl.mu.RLock()
	defer dl.mu.RUnlock()

	graph, ok := dl.lineages[lineageID]
	if !ok {
		return nil, fmt.Errorf("lineage %s not found", lineageID)
	}

	upstreamIDs := make(map[string]bool)
	for _, edge := range graph.Edges {
		if edge.TargetNodeID == nodeID {
			upstreamIDs[edge.SourceNodeID] = true
		}
	}

	result := make([]*LineageNode, 0)
	for _, node := range graph.Nodes {
		if upstreamIDs[node.ID] {
			result = append(result, node)
		}
	}
	return result, nil
}

// GetDownstream 获取下游节点
func (dl *DataLake) GetDownstream(lineageID, nodeID string) ([]*LineageNode, error) {
	dl.mu.RLock()
	defer dl.mu.RUnlock()

	graph, ok := dl.lineages[lineageID]
	if !ok {
		return nil, fmt.Errorf("lineage %s not found", lineageID)
	}

	downstreamIDs := make(map[string]bool)
	for _, edge := range graph.Edges {
		if edge.SourceNodeID == nodeID {
			downstreamIDs[edge.TargetNodeID] = true
		}
	}

	result := make([]*LineageNode, 0)
	for _, node := range graph.Nodes {
		if downstreamIDs[node.ID] {
			result = append(result, node)
		}
	}
	return result, nil
}

// ==================== 数据质量管理 ====================

// AddQualityRule 添加质量规则
func (dl *DataLake) AddQualityRule(rule *QualityRule) error {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	if rule.ID == "" {
		rule.ID = generateID()
	}
	rule.CreatedAt = time.Now()
	dl.rules[rule.ID] = rule
	dl.updateStats()
	return nil
}

// GetQualityRule 获取质量规则
func (dl *DataLake) GetQualityRule(id string) (*QualityRule, bool) {
	dl.mu.RLock()
	defer dl.mu.RUnlock()
	rule, ok := dl.rules[id]
	return rule, ok
}

// ListQualityRules 列出质量规则
func (dl *DataLake) ListQualityRules() []*QualityRule {
	dl.mu.RLock()
	defer dl.mu.RUnlock()
	result := make([]*QualityRule, 0, len(dl.rules))
	for _, r := range dl.rules {
		result = append(result, r)
	}
	return result
}

// RemoveQualityRule 移除质量规则
func (dl *DataLake) RemoveQualityRule(id string) error {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	if _, ok := dl.rules[id]; !ok {
		return fmt.Errorf("rule %s not found", id)
	}
	delete(dl.rules, id)
	dl.updateStats()
	return nil
}

// RunQualityCheck 执行质量检查（模拟）
func (dl *DataLake) RunQualityCheck(objectID string, ruleIDs []string) (*QualityReport, error) {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	if _, ok := dl.objects[objectID]; !ok {
		return nil, fmt.Errorf("object %s not found", objectID)
	}

	results := make([]*QualityCheckResult, 0)
	for _, ruleID := range ruleIDs {
		rule, ok := dl.rules[ruleID]
		if !ok {
			continue
		}

		result := &QualityCheckResult{
			ID:        generateID(),
			RuleID:    ruleID,
			ObjectID:  objectID,
			TotalRows: 100, // 模拟值
			CheckedAt: time.Now(),
		}

		// 模拟质量检查
		switch rule.Type {
		case QualityNotNull:
			result.PassedRows = 95
			result.FailedRows = 5
			result.Passed = true
			result.Status = QStatusPass
			result.Score = 95.0
		case QualityUnique:
			result.PassedRows = 100
			result.FailedRows = 0
			result.Passed = true
			result.Status = QStatusPass
			result.Score = 100.0
		case QualityRange:
			result.PassedRows = 88
			result.FailedRows = 12
			result.Passed = true
			result.Status = QStatusWarn
			result.Score = 88.0
		default:
			result.PassedRows = 90
			result.FailedRows = 10
			result.Passed = true
			result.Status = QStatusPass
			result.Score = 90.0
		}

		results = append(results, result)
		dl.results[objectID] = append(dl.results[objectID], result)
	}

	// 计算总分
	totalScore := 0.0
	for _, r := range results {
		totalScore += r.Score
	}
	avgScore := 0.0
	if len(results) > 0 {
		avgScore = totalScore / float64(len(results))
	}

	report := &QualityReport{
		ObjectID:     objectID,
		OverallScore: avgScore,
		Results:      results,
		GeneratedAt:  time.Now(),
	}

	// 更新目录中的质量分数
	for _, entry := range dl.catalogs {
		if entry.ObjectID == objectID {
			entry.QualityScore = avgScore
			entry.UpdatedAt = time.Now()
		}
	}

	return report, nil
}

// GetQualityResults 获取质量检查结果
func (dl *DataLake) GetQualityResults(objectID string) []*QualityCheckResult {
	dl.mu.RLock()
	defer dl.mu.RUnlock()
	return dl.results[objectID]
}

// ==================== 统计 ====================

// GetStats 获取统计信息
func (dl *DataLake) GetStats() *DataLakeStats {
	dl.mu.RLock()
	defer dl.mu.RUnlock()
	return dl.stats
}

func (dl *DataLake) updateStats() {
	stats := &DataLakeStats{}
	stats.TotalSources = len(dl.sources)
	for _, s := range dl.sources {
		if s.Status == DSStatusOnline {
			stats.OnlineSources++
		}
		stats.TotalSize += s.Used
	}
	stats.TotalObjects = len(dl.objects)
	stats.TotalCatalogs = len(dl.catalogs)
	stats.TotalLineages = len(dl.lineages)
	stats.TotalRules = len(dl.rules)

	// 计算平均质量分数
	totalScore := 0.0
	count := 0
	for _, entry := range dl.catalogs {
		if entry.QualityScore > 0 {
			totalScore += entry.QualityScore
			count++
		}
	}
	if count > 0 {
		stats.AvgQualityScore = totalScore / float64(count)
	}

	dl.stats = stats
}

func generateID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}
