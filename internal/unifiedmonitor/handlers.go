// Package unifiedmonitor HTTP API 处理器
package unifiedmonitor

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	group := router.Group("/unified-monitor")
	{
		// 集群概览
		group.GET("/dashboard", h.GetDashboard)
		group.GET("/health", h.GetClusterHealth)

		// 节点管理
		group.GET("/nodes", h.ListNodes)
		group.POST("/nodes", h.RegisterNode)
		group.GET("/nodes/:id", h.GetNode)
		group.DELETE("/nodes/:id", h.RemoveNode)
		group.PUT("/nodes/:id/metrics", h.UpdateNodeMetrics)

		// 指标查询
		group.GET("/metrics", h.QueryMetrics)
		group.GET("/metrics/aggregate", h.AggregateMetrics)

		// 告警管理
		group.GET("/alerts", h.ListAlerts)
		group.POST("/alerts/:id/acknowledge", h.AcknowledgeAlert)
		group.POST("/alerts/:id/resolve", h.ResolveAlert)
		group.GET("/alerts/correlated", h.GetCorrelatedAlerts)

		// 告警规则
		group.GET("/rules", h.ListRules)
		group.POST("/rules", h.CreateRule)
		group.DELETE("/rules/:id", h.DeleteRule)

		// 延迟矩阵
		group.GET("/latency", h.GetLatencyMatrix)
		group.POST("/latency", h.RecordLatency)
	}
}

// ========== 集群概览 ==========

// GetDashboard 获取仪表板数据
func (h *Handler) GetDashboard(c *gin.Context) {
	data := h.manager.GetDashboard()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": data})
}

// GetClusterHealth 获取集群健康评分
func (h *Handler) GetClusterHealth(c *gin.Context) {
	health := h.manager.GetClusterHealth()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": health})
}

// ========== 节点管理 ==========

// ListNodes 列出所有节点
func (h *Handler) ListNodes(c *gin.Context) {
	nodes := h.manager.ListNodes()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": nodes})
}

// RegisterNodeRequest 注册节点请求
type RegisterNodeRequest struct {
	ID        string            `json:"id" binding:"required"`
	Name      string            `json:"name" binding:"required"`
	Hostname  string            `json:"hostname"`
	IPAddress string            `json:"ip_address"`
	Role      string            `json:"role"`
	Tags      map[string]string `json:"tags"`
}

// RegisterNode 注册节点
func (h *Handler) RegisterNode(c *gin.Context) {
	var req RegisterNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": fmt.Sprintf("参数错误: %v", err)})
		return
	}

	node := &ClusterNode{
		ID:           req.ID,
		Name:         req.Name,
		Hostname:     req.Hostname,
		IPAddress:    req.IPAddress,
		Role:         NodeRole(req.Role),
		Status:       NodeStatusOnline,
		Tags:         req.Tags,
		RegisteredAt: time.Now(),
		LastSeen:     time.Now(),
	}

	if node.Role == "" {
		node.Role = RoleWorker
	}

	if err := h.manager.RegisterNode(node); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": fmt.Sprintf("注册失败: %v", err)})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "节点已注册", "data": node})
}

// GetNode 获取节点详情
func (h *Handler) GetNode(c *gin.Context) {
	nodeID := c.Param("id")
	node, exists := h.manager.GetNode(nodeID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "节点不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": node})
}

// RemoveNode 移除节点
func (h *Handler) RemoveNode(c *gin.Context) {
	nodeID := c.Param("id")
	if !h.manager.RemoveNode(nodeID) {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "节点不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "节点已移除"})
}

// UpdateNodeMetricsRequest 更新节点指标请求
type UpdateNodeMetricsRequest struct {
	Metrics NodeMetrics `json:"metrics" binding:"required"`
}

// UpdateNodeMetrics 更新节点指标
func (h *Handler) UpdateNodeMetrics(c *gin.Context) {
	nodeID := c.Param("id")
	var req UpdateNodeMetricsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": fmt.Sprintf("参数错误: %v", err)})
		return
	}

	if err := h.manager.UpdateNodeMetrics(nodeID, req.Metrics); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": fmt.Sprintf("更新失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "指标已更新"})
}

// ========== 指标查询 ==========

// QueryMetrics 查询指标
func (h *Handler) QueryMetrics(c *gin.Context) {
	name := c.Query("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "name 参数必填"})
		return
	}

	nodeID := c.Query("node_id")
	startStr := c.DefaultQuery("start", time.Now().Add(-1*time.Hour).Format(time.RFC3339))
	endStr := c.DefaultQuery("end", time.Now().Format(time.RFC3339))

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": fmt.Sprintf("start 时间格式错误: %v", err)})
		return
	}

	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": fmt.Sprintf("end 时间格式错误: %v", err)})
		return
	}

	points, err := h.manager.QueryMetrics(context.Background(), name, nodeID, start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": fmt.Sprintf("查询失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": points})
}

// AggregateMetrics 聚合指标
func (h *Handler) AggregateMetrics(c *gin.Context) {
	name := c.Query("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "name 参数必填"})
		return
	}

	startStr := c.DefaultQuery("start", time.Now().Add(-1*time.Hour).Format(time.RFC3339))
	endStr := c.DefaultQuery("end", time.Now().Format(time.RFC3339))

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": fmt.Sprintf("start 时间格式错误: %v", err)})
		return
	}

	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": fmt.Sprintf("end 时间格式错误: %v", err)})
		return
	}

	agg, err := h.manager.AggregateMetrics(context.Background(), name, start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": fmt.Sprintf("聚合失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": agg})
}

// ========== 告警管理 ==========

// ListAlerts 列出告警
func (h *Handler) ListAlerts(c *gin.Context) {
	statusStr := c.DefaultQuery("status", "firing")
	limitStr := c.DefaultQuery("limit", "100")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	var status AlertStatus
	switch statusStr {
	case "firing":
		status = AlertStatusFiring
	case "resolved":
		status = AlertStatusResolved
	case "silenced":
		status = AlertStatusSilenced
	default:
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": fmt.Sprintf("无效状态: %s", statusStr)})
		return
	}

	alerts, err := h.manager.ListAlerts(context.Background(), status, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": fmt.Sprintf("查询失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": alerts})
}

// AcknowledgeAlert 确认告警
func (h *Handler) AcknowledgeAlert(c *gin.Context) {
	alertID := c.Param("id")
	if err := h.manager.AcknowledgeAlert(context.Background(), alertID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "告警不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "告警已确认"})
}

// ResolveAlert 解决告警
func (h *Handler) ResolveAlert(c *gin.Context) {
	alertID := c.Param("id")
	if err := h.manager.ResolveAlert(context.Background(), alertID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "告警不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "告警已解决"})
}

// GetCorrelatedAlerts 获取关联告警
func (h *Handler) GetCorrelatedAlerts(c *gin.Context) {
	correlated := h.manager.GetCorrelatedAlerts()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": correlated})
}

// ========== 告警规则 ==========

// ListRules 列出规则
func (h *Handler) ListRules(c *gin.Context) {
	rules := h.manager.ListRules()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": rules})
}

// CreateRule 创建规则
func (h *Handler) CreateRule(c *gin.Context) {
	var rule AlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": fmt.Sprintf("参数错误: %v", err)})
		return
	}

	if err := h.manager.AddRule(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": fmt.Sprintf("创建失败: %v", err)})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "规则已创建", "data": rule})
}

// DeleteRule 删除规则
func (h *Handler) DeleteRule(c *gin.Context) {
	ruleID := c.Param("id")
	h.manager.RemoveRule(ruleID)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "规则已删除"})
}

// ========== 延迟矩阵 ==========

// GetLatencyMatrix 获取延迟矩阵
func (h *Handler) GetLatencyMatrix(c *gin.Context) {
	matrix := h.manager.GetLatencyMatrix()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": matrix})
}

// RecordLatencyRequest 记录延迟请求
type RecordLatencyRequest struct {
	SourceNodeID string  `json:"source_node_id" binding:"required"`
	TargetNodeID string  `json:"target_node_id" binding:"required"`
	LatencyMs    float64 `json:"latency_ms" binding:"required"`
	JitterMs     float64 `json:"jitter_ms"`
	PacketLoss   float64 `json:"packet_loss"`
}

// RecordLatency 记录延迟
func (h *Handler) RecordLatency(c *gin.Context) {
	var req RecordLatencyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": fmt.Sprintf("参数错误: %v", err)})
		return
	}

	latency := &NodeLatency{
		SourceNodeID: req.SourceNodeID,
		TargetNodeID: req.TargetNodeID,
		Latency:      time.Duration(req.LatencyMs * float64(time.Millisecond)),
		Jitter:       time.Duration(req.JitterMs * float64(time.Millisecond)),
		PacketLoss:   req.PacketLoss,
		MeasuredAt:   time.Now(),
	}

	h.manager.RecordLatency(latency)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "延迟已记录"})
}

// ========== Manager 方法 ==========

// RegisterNode 注册节点
func (m *Manager) RegisterNode(node *ClusterNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.nodes[node.ID]; exists {
		return fmt.Errorf("节点 %s 已存在", node.ID)
	}

	m.nodes[node.ID] = node
	return nil
}

// GetNode 获取节点
func (m *Manager) GetNode(nodeID string) (*ClusterNode, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	node, exists := m.nodes[nodeID]
	return node, exists
}

// RemoveNode 移除节点
func (m *Manager) RemoveNode(nodeID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.nodes[nodeID]; !exists {
		return false
	}

	delete(m.nodes, nodeID)
	return true
}

// ListNodes 列出节点
func (m *Manager) ListNodes() []ClusterNode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]ClusterNode, 0, len(m.nodes))
	for _, node := range m.nodes {
		nodes = append(nodes, *node)
	}
	return nodes
}

// UpdateNodeMetrics 更新节点指标
func (m *Manager) UpdateNodeMetrics(nodeID string, metrics NodeMetrics) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, exists := m.nodes[nodeID]
	if !exists {
		return fmt.Errorf("节点 %s 不存在", nodeID)
	}

	node.Metrics = metrics
	node.LastSeen = time.Now()
	node.Status = NodeStatusOnline

	return nil
}

// RecordMetric 记录指标
func (m *Manager) RecordMetric(ctx context.Context, point MetricPoint) error {
	return m.metricStore.Store(point)
}

// QueryMetrics 查询指标
func (m *Manager) QueryMetrics(ctx context.Context, name, nodeID string, start, end time.Time) ([]MetricPoint, error) {
	return m.metricStore.Query(name, nodeID, start, end)
}

// AggregateMetrics 聚合指标
func (m *Manager) AggregateMetrics(ctx context.Context, name string, start, end time.Time) (*AggregatedMetrics, error) {
	return m.metricStore.Aggregate(name, start, end)
}

// AddRule 添加规则
func (m *Manager) AddRule(rule *AlertRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		return fmt.Errorf("规则 ID 不能为空")
	}
	if rule.Name == "" {
		return fmt.Errorf("规则名称不能为空")
	}
	if rule.Metric == "" {
		return fmt.Errorf("指标名称不能为空")
	}

	// 设置默认值
	if rule.Type == "" {
		rule.Type = RuleTypeThreshold
	}
	if rule.Condition == "" {
		rule.Condition = ConditionAbove
	}
	if rule.Severity == "" {
		rule.Severity = SeverityWarning
	}
	if rule.Duration == 0 {
		rule.Duration = 5 * time.Minute
	}
	rule.Enabled = true
	rule.CreatedAt = time.Now()

	m.rules[rule.ID] = rule
	return nil
}

// RemoveRule 删除规则
func (m *Manager) RemoveRule(ruleID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rules, ruleID)
}

// ListRules 列出规则
func (m *Manager) ListRules() []AlertRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]AlertRule, 0, len(m.rules))
	for _, rule := range m.rules {
		rules = append(rules, *rule)
	}
	return rules
}

// ListAlerts 列出告警
func (m *Manager) ListAlerts(ctx context.Context, status AlertStatus, limit int) ([]Alert, error) {
	return m.alertStore.Query(status, limit)
}

// AcknowledgeAlert 确认告警
func (m *Manager) AcknowledgeAlert(ctx context.Context, alertID string) error {
	return m.alertStore.UpdateStatus(alertID, AlertStatusSilenced)
}

// ResolveAlert 解决告警
func (m *Manager) ResolveAlert(ctx context.Context, alertID string) error {
	return m.alertStore.UpdateStatus(alertID, AlertStatusResolved)
}

// GetCorrelatedAlerts 获取关联告警
func (m *Manager) GetCorrelatedAlerts() []CorrelatedAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.correlated
}

// RecordLatency 记录延迟
func (m *Manager) RecordLatency(latency *NodeLatency) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.latency[latency.SourceNodeID]; !exists {
		m.latency[latency.SourceNodeID] = make(map[string]*NodeLatency)
	}
	m.latency[latency.SourceNodeID][latency.TargetNodeID] = latency
}

// GetLatencyMatrix 获取延迟矩阵
func (m *Manager) GetLatencyMatrix() LatencyMatrix {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]string, 0, len(m.nodes))
	for id := range m.nodes {
		nodes = append(nodes, id)
	}
	sort.Strings(nodes)

	matrix := make(map[string]map[string]time.Duration)
	var totalMs float64
	var maxMs float64
	minMs := math.MaxFloat64
	count := 0

	for _, srcID := range nodes {
		matrix[srcID] = make(map[string]time.Duration)
		for _, dstID := range nodes {
			if srcID == dstID {
				matrix[srcID][dstID] = 0
				continue
			}

			if lat, ok := m.latency[srcID][dstID]; ok {
				matrix[srcID][dstID] = lat.Latency
				ms := float64(lat.Latency.Milliseconds())
				totalMs += ms
				count++
				if ms > maxMs {
					maxMs = ms
				}
				if ms < minMs {
					minMs = ms
				}
			}
		}
	}

	if count == 0 {
		minMs = 0
	}

	avgMs := float64(0)
	if count > 0 {
		avgMs = totalMs / float64(count)
	}

	return LatencyMatrix{
		Nodes:   nodes,
		Matrix:  matrix,
		AvgMs:   avgMs,
		MaxMs:   maxMs,
		MinMs:   minMs,
		Updated: time.Now(),
	}
}

// GetClusterHealth 获取集群健康评分
func (m *Manager) GetClusterHealth() ClusterHealthScore {
	m.mu.RLock()
	defer m.mu.RUnlock()

	score := ClusterHealthScore{
		PerNode:  make(map[string]int),
		Details:  make(map[string]int),
		LastEval: time.Now(),
	}

	// CPU 评分 (权重 30%)
	cpuScore := m.evaluateCPUHealth()
	score.Details["cpu"] = cpuScore

	// 内存评分 (权重 25%)
	memScore := m.evaluateMemoryHealth()
	score.Details["memory"] = memScore

	// 磁盘评分 (权重 25%)
	diskScore := m.evaluateDiskHealth()
	score.Details["disk"] = diskScore

	// 网络评分 (权重 20%)
	netScore := m.evaluateNetworkHealth()
	score.Details["network"] = netScore

	// 计算总分
	total := float64(cpuScore)*0.3 + float64(memScore)*0.25 +
		float64(diskScore)*0.25 + float64(netScore)*0.2
	score.Score = int(math.Round(total))

	// 确定等级
	switch {
	case score.Score >= 80:
		score.Level = "good"
	case score.Score >= 60:
		score.Level = "warning"
	default:
		score.Level = "critical"
	}

	// 各节点得分
	for id, node := range m.nodes {
		nodeScore := m.evaluateNodeHealth(node)
		score.PerNode[id] = nodeScore
	}

	return score
}

// evaluateCPUHealth 评估 CPU 健康度
func (m *Manager) evaluateCPUHealth() int {
	totalScore := 0
	count := 0

	for _, node := range m.nodes {
		if node.Status == NodeStatusOffline {
			continue
		}
		count++
		totalScore += scoreByThreshold(node.Metrics.CPUPercent, 70, 85, 95)
	}

	if count == 0 {
		return 100
	}
	return totalScore / count
}

// evaluateMemoryHealth 评估内存健康度
func (m *Manager) evaluateMemoryHealth() int {
	totalScore := 0
	count := 0

	for _, node := range m.nodes {
		if node.Status == NodeStatusOffline {
			continue
		}
		count++
		totalScore += scoreByThreshold(node.Metrics.MemPercent, 75, 85, 95)
	}

	if count == 0 {
		return 100
	}
	return totalScore / count
}

// evaluateDiskHealth 评估磁盘健康度
func (m *Manager) evaluateDiskHealth() int {
	totalScore := 0
	count := 0

	for _, node := range m.nodes {
		if node.Status == NodeStatusOffline {
			continue
		}
		count++
		totalScore += scoreByThreshold(node.Metrics.DiskPercent, 80, 90, 95)
	}

	if count == 0 {
		return 100
	}
	return totalScore / count
}

// evaluateNetworkHealth 评估网络健康度
func (m *Manager) evaluateNetworkHealth() int {
	onlineCount := 0
	totalCount := 0

	for _, node := range m.nodes {
		totalCount++
		if node.Status == NodeStatusOnline {
			onlineCount++
		}
	}

	if totalCount == 0 {
		return 100
	}

	return (onlineCount * 100) / totalCount
}

// evaluateNodeHealth 评估单节点健康度
func (m *Manager) evaluateNodeHealth(node *ClusterNode) int {
	if node.Status == NodeStatusOffline {
		return 0
	}

	cpuScore := scoreByThreshold(node.Metrics.CPUPercent, 70, 85, 95)
	memScore := scoreByThreshold(node.Metrics.MemPercent, 75, 85, 95)
	diskScore := scoreByThreshold(node.Metrics.DiskPercent, 80, 90, 95)

	return int(math.Round(float64(cpuScore)*0.4 + float64(memScore)*0.3 + float64(diskScore)*0.3))
}

// scoreByThreshold 按阈值评分
func scoreByThreshold(value, warn, high, crit float64) int {
	switch {
	case value < warn:
		return 100
	case value < high:
		return 70
	case value < crit:
		return 40
	default:
		return 10
	}
}

// GetDashboard 获取仪表板数据
func (m *Manager) GetDashboard() DashboardData {
	health := m.GetClusterHealth()
	nodes := m.ListNodes()

	// 获取活跃告警
	alerts, _ := m.alertStore.Query(AlertStatusFiring, 20)

	// 获取延迟矩阵
	latency := m.GetLatencyMatrix()

	// 识别主要问题
	issues := m.identifyTopIssues()

	return DashboardData{
		ClusterHealth: health,
		Nodes:         nodes,
		ActiveAlerts:  alerts,
		Correlated:    m.correlated,
		Latency:       latency,
		Aggregated:    make(map[string]AggregatedMetrics),
		TopIssues:     issues,
		Timestamp:     time.Now(),
	}
}

// identifyTopIssues 识别主要问题
func (m *Manager) identifyTopIssues() []string {
	issues := make([]string, 0)

	for _, node := range m.nodes {
		if node.Status == NodeStatusOffline {
			issues = append(issues, fmt.Sprintf("节点 %s 离线", node.Name))
			continue
		}
		if node.Metrics.CPUPercent > 90 {
			issues = append(issues, fmt.Sprintf("节点 %s CPU 使用率过高 (%.1f%%)", node.Name, node.Metrics.CPUPercent))
		}
		if node.Metrics.MemPercent > 90 {
			issues = append(issues, fmt.Sprintf("节点 %s 内存使用率过高 (%.1f%%)", node.Name, node.Metrics.MemPercent))
		}
		if node.Metrics.DiskPercent > 90 {
			issues = append(issues, fmt.Sprintf("节点 %s 磁盘使用率过高 (%.1f%%)", node.Name, node.Metrics.DiskPercent))
		}
	}

	sort.Slice(issues, func(i, j int) bool {
		return len(issues[i]) > len(issues[j])
	})

	if len(issues) > 5 {
		return issues[:5]
	}
	return issues
}
