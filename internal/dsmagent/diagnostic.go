package dsmagent

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// DiagnosticAgent 智能诊断代理
// 提供系统状态智能诊断、异常检测、根因分析和预测性告警
type DiagnosticAgent struct {
	mu              sync.RWMutex
	health          *SystemHealth
	diagnoses       []*DiagnosisResult     // 诊断结果历史
	predictions     []*Prediction          // 预测性告警
	correlations    map[string]*Correlation // 指标关联分析
	alertRules      []*AlertRule           // 告警规则
	anomalyBaseline *AnomalyBaseline       // 异常检测基线
}

// DiagnosisResult 诊断结果
type DiagnosisResult struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	Severity    Severity               `json:"severity"`
	Category    string                 `json:"category"` // cpu, memory, disk, network, service
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	RootCause   string                 `json:"root_cause,omitempty"`  // 根因分析
	Evidence    []Evidence             `json:"evidence"`              // 证据
	Suggestions []Suggestion           `json:"suggestions"`           // 修复建议
	AutoFixed   bool                   `json:"auto_fixed"`            // 是否已自动修复
	Details     map[string]interface{} `json:"details,omitempty"`
}

// Severity 严重程度
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

// Evidence 诊断证据
type Evidence struct {
	Type     string      `json:"type"`     // metric, log, event
	Source   string      `json:"source"`   // 来源组件
	Value    interface{} `json:"value"`    // 观测值
	Expected interface{} `json:"expected"` // 期望值
	Message  string      `json:"message"`
}

// Suggestion 修复建议
type Suggestion struct {
	Action      string `json:"action"`      // 建议动作
	Description string `json:"description"` // 详细说明
	Priority    int    `json:"priority"`    // 优先级 1-5
	Automatic   bool   `json:"automatic"`   // 是否可自动执行
	Risk        string `json:"risk"`        // 风险等级: low, medium, high
}

// Prediction 预测性告警
type Prediction struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	Category    string    `json:"category"`
	Prediction  string    `json:"prediction"`  // 预测内容
	Confidence  float64   `json:"confidence"`  // 置信度 0-1
	ETA         string    `json:"eta"`         // 预计发生时间
	Prevention  []string  `json:"prevention"`  // 预防措施
}

// Correlation 指标关联分析
type Correlation struct {
	MetricA     string    `json:"metric_a"`
	MetricB     string    `json:"metric_b"`
	Coefficient float64   `json:"coefficient"` // 相关系数 -1 到 1
	Insight     string    `json:"insight"`     // 关联洞察
	UpdatedAt   time.Time `json:"updated_at"`
}

// AlertRule 告警规则
type AlertRule struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Enabled     bool                   `json:"enabled"`
	Metric      string                 `json:"metric"`       // 监控指标
	Condition   string                 `json:"condition"`    // 条件: gt, lt, eq, between
	Threshold   float64                `json:"threshold"`    // 阈值
	Duration    time.Duration          `json:"duration"`     // 持续时间
	Severity    Severity               `json:"severity"`
	Message     string                 `json:"message"`
	Actions     []string               `json:"actions"`      // 触发的动作
}

// AnomalyBaseline 异常检测基线
type AnomalyBaseline struct {
	CPUMean     float64   `json:"cpu_mean"`
	CPUStdDev   float64   `json:"cpu_stddev"`
	MemMean     float64   `json:"mem_mean"`
	MemStdDev   float64   `json:"mem_stddev"`
	DiskMean    float64   `json:"disk_mean"`
	DiskStdDev  float64   `json:"disk_stddev"`
	TempMean    float64   `json:"temp_mean"`
	TempStdDev  float64   `json:"temp_stddev"`
	SampleCount int       `json:"sample_count"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// DiagnosticSummary 诊断摘要
type DiagnosticSummary struct {
	OverallHealth   string            `json:"overall_health"`   // good, warning, critical
	Score           int               `json:"score"`            // 健康评分 0-100
	Issues          int               `json:"issues"`           // 发现的问题数
	Warnings        int               `json:"warnings"`         // 警告数
	Predictions     int               `json:"predictions"`      // 预测告警数
	Uptime          string            `json:"uptime"`
	LastDiagnosis   time.Time         `json:"last_diagnosis"`
	TopIssues       []string          `json:"top_issues"`
	Recommendations []string          `json:"recommendations"`
}

// NewDiagnosticAgent 创建智能诊断代理实例
func NewDiagnosticAgent() *DiagnosticAgent {
	agent := &DiagnosticAgent{
		diagnoses:    make([]*DiagnosisResult, 0),
		predictions:  make([]*Prediction, 0),
		correlations: make(map[string]*Correlation),
		alertRules:   make([]*AlertRule, 0),
		anomalyBaseline: &AnomalyBaseline{},
	}

	// 初始化默认告警规则
	agent.initDefaultAlertRules()

	log.Printf("[DiagnosticAgent] 智能诊断代理已初始化")
	return agent
}

// initDefaultAlertRules 初始化默认告警规则
func (d *DiagnosticAgent) initDefaultAlertRules() {
	d.alertRules = []*AlertRule{
		{
			ID: "rule_cpu_high", Name: "CPU使用率过高", Enabled: true,
			Metric: "cpu_usage", Condition: "gt", Threshold: 90.0,
			Duration: 5 * time.Minute, Severity: SeverityWarning,
			Message: "CPU使用率持续超过90%",
		},
		{
			ID: "rule_cpu_critical", Name: "CPU使用率严重过高", Enabled: true,
			Metric: "cpu_usage", Condition: "gt", Threshold: 98.0,
			Duration: 2 * time.Minute, Severity: SeverityCritical,
			Message: "CPU使用率持续超过98%，系统可能无响应",
		},
		{
			ID: "rule_memory_high", Name: "内存使用率过高", Enabled: true,
			Metric: "memory_usage", Condition: "gt", Threshold: 90.0,
			Duration: 5 * time.Minute, Severity: SeverityWarning,
			Message: "内存使用率持续超过90%",
		},
		{
			ID: "rule_disk_high", Name: "磁盘空间不足", Enabled: true,
			Metric: "disk_usage", Condition: "gt", Threshold: 90.0,
			Duration: 0, Severity: SeverityWarning,
			Message: "磁盘使用率超过90%，建议清理空间",
		},
		{
			ID: "rule_disk_critical", Name: "磁盘空间严重不足", Enabled: true,
			Metric: "disk_usage", Condition: "gt", Threshold: 95.0,
			Duration: 0, Severity: SeverityCritical,
			Message: "磁盘使用率超过95%，系统可能不稳定",
		},
		{
			ID: "rule_temp_high", Name: "温度过高", Enabled: true,
			Metric: "temperature", Condition: "gt", Threshold: 80.0,
			Duration: 3 * time.Minute, Severity: SeverityWarning,
			Message: "系统温度持续过高，检查散热",
		},
		{
			ID: "rule_temp_critical", Name: "温度严重过高", Enabled: true,
			Metric: "temperature", Condition: "gt", Threshold: 95.0,
			Duration: 1 * time.Minute, Severity: SeverityCritical,
			Message: "系统温度严重过高，可能触发自动关机保护",
		},
	}

	log.Printf("[DiagnosticAgent] 初始化了 %d 条默认告警规则", len(d.alertRules))
}

// RunDiagnosis 执行完整系统诊断
func (d *DiagnosticAgent) RunDiagnosis(health *SystemHealth) *DiagnosticSummary {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.health = health
	diagnosisTime := time.Now()

	// 执行各项检查
	issues := 0
	warnings := 0
	var topIssues []string
	var recommendations []string

	// CPU诊断
	if result := d.diagnoseCPU(health); result != nil {
		d.diagnoses = append(d.diagnoses, result)
		if result.Severity == SeverityCritical || result.Severity == SeverityError {
			issues++
			topIssues = append(topIssues, result.Title)
		} else if result.Severity == SeverityWarning {
			warnings++
		}
		recommendations = append(recommendations, d.getRecommendations(result)...)
	}

	// 内存诊断
	if result := d.diagnoseMemory(health); result != nil {
		d.diagnoses = append(d.diagnoses, result)
		if result.Severity == SeverityCritical || result.Severity == SeverityError {
			issues++
			topIssues = append(topIssues, result.Title)
		} else if result.Severity == SeverityWarning {
			warnings++
		}
		recommendations = append(recommendations, d.getRecommendations(result)...)
	}

	// 磁盘诊断
	if result := d.diagnoseDisk(health); result != nil {
		d.diagnoses = append(d.diagnoses, result)
		if result.Severity == SeverityCritical || result.Severity == SeverityError {
			issues++
			topIssues = append(topIssues, result.Title)
		} else if result.Severity == SeverityWarning {
			warnings++
		}
		recommendations = append(recommendations, d.getRecommendations(result)...)
	}

	// 温度诊断
	if result := d.diagnoseTemperature(health); result != nil {
		d.diagnoses = append(d.diagnoses, result)
		if result.Severity == SeverityCritical || result.Severity == SeverityError {
			issues++
			topIssues = append(topIssues, result.Title)
		} else if result.Severity == SeverityWarning {
			warnings++
		}
		recommendations = append(recommendations, d.getRecommendations(result)...)
	}

	// 异常检测
	d.detectAnomalies(health)

	// 预测分析
	d.runPredictions(health)

	// 计算健康评分
	score := d.calculateHealthScore(health, issues, warnings)
	overallHealth := "good"
	if score < 60 {
		overallHealth = "critical"
	} else if score < 80 {
		overallHealth = "warning"
	}

	// 限制topIssues数量
	if len(topIssues) > 5 {
		topIssues = topIssues[:5]
	}

	summary := &DiagnosticSummary{
		OverallHealth:   overallHealth,
		Score:           score,
		Issues:          issues,
		Warnings:        warnings,
		Predictions:     len(d.predictions),
		LastDiagnosis:   diagnosisTime,
		TopIssues:       topIssues,
		Recommendations: recommendations,
	}

	log.Printf("[DiagnosticAgent] 诊断完成: 评分=%d, 问题=%d, 警告=%d", score, issues, warnings)
	return summary
}

// diagnoseCPU 诊断CPU状态
func (d *DiagnosticAgent) diagnoseCPU(health *SystemHealth) *DiagnosisResult {
	if health.CPUUsage < 80 {
		return nil
	}

	severity := SeverityWarning
	if health.CPUUsage >= 95 {
		severity = SeverityCritical
	} else if health.CPUUsage >= 90 {
		severity = SeverityError
	}

	return &DiagnosisResult{
		ID:          fmt.Sprintf("diag_cpu_%d", time.Now().UnixNano()),
		Timestamp:   time.Now(),
		Severity:    severity,
		Category:    "cpu",
		Title:       "CPU使用率过高",
		Description: fmt.Sprintf("当前CPU使用率为 %.1f%%", health.CPUUsage),
		RootCause:   "可能存在计算密集型进程或资源竞争",
		Evidence: []Evidence{
			{Type: "metric", Source: "system", Value: health.CPUUsage, Expected: "< 80%", Message: "CPU使用率超出正常范围"},
		},
		Suggestions: []Suggestion{
			{Action: "检查高CPU进程", Description: "使用top或htop查看占用CPU最高的进程", Priority: 1, Risk: "low"},
			{Action: "考虑负载均衡", Description: "如果有多个服务，考虑分散负载", Priority: 2, Risk: "low"},
		},
	}
}

// diagnoseMemory 诊断内存状态
func (d *DiagnosticAgent) diagnoseMemory(health *SystemHealth) *DiagnosisResult {
	if health.MemoryUsage < 80 {
		return nil
	}

	severity := SeverityWarning
	if health.MemoryUsage >= 95 {
		severity = SeverityCritical
	} else if health.MemoryUsage >= 90 {
		severity = SeverityError
	}

	return &DiagnosisResult{
		ID:          fmt.Sprintf("diag_mem_%d", time.Now().UnixNano()),
		Timestamp:   time.Now(),
		Severity:    severity,
		Category:    "memory",
		Title:       "内存使用率过高",
		Description: fmt.Sprintf("当前内存使用率为 %.1f%%", health.MemoryUsage),
		RootCause:   "可能存在内存泄漏或应用内存占用过大",
		Evidence: []Evidence{
			{Type: "metric", Source: "system", Value: health.MemoryUsage, Expected: "< 80%", Message: "内存使用率超出正常范围"},
		},
		Suggestions: []Suggestion{
			{Action: "检查内存占用进程", Description: "找出占用内存最多的进程", Priority: 1, Risk: "low"},
			{Action: "重启问题服务", Description: "如果某个服务内存持续增长，考虑重启", Priority: 2, Risk: "medium"},
		},
	}
}

// diagnoseDisk 诊断磁盘状态
func (d *DiagnosticAgent) diagnoseDisk(health *SystemHealth) *DiagnosisResult {
	if health.DiskUsage < 85 {
		return nil
	}

	severity := SeverityWarning
	if health.DiskUsage >= 95 {
		severity = SeverityCritical
	} else if health.DiskUsage >= 90 {
		severity = SeverityError
	}

	return &DiagnosisResult{
		ID:          fmt.Sprintf("diag_disk_%d", time.Now().UnixNano()),
		Timestamp:   time.Now(),
		Severity:    severity,
		Category:    "disk",
		Title:       "磁盘空间不足",
		Description: fmt.Sprintf("当前磁盘使用率为 %.1f%%", health.DiskUsage),
		RootCause:   "日志文件、临时文件或快照占用过多空间",
		Evidence: []Evidence{
			{Type: "metric", Source: "storage", Value: health.DiskUsage, Expected: "< 85%", Message: "磁盘使用率超出安全范围"},
		},
		Suggestions: []Suggestion{
			{Action: "清理日志文件", Description: "删除过期的系统和应用日志", Priority: 1, Automatic: true, Risk: "low"},
			{Action: "清理临时文件", Description: "删除/tmp等临时目录中的过期文件", Priority: 2, Automatic: true, Risk: "low"},
			{Action: "清理旧快照", Description: "删除不需要的存储快照", Priority: 3, Risk: "medium"},
			{Action: "扩展存储", Description: "考虑添加新磁盘或扩展现有存储池", Priority: 4, Risk: "low"},
		},
	}
}

// diagnoseTemperature 诊断温度状态
func (d *DiagnosticAgent) diagnoseTemperature(health *SystemHealth) *DiagnosisResult {
	if health.Temperature < 70 {
		return nil
	}

	severity := SeverityWarning
	if health.Temperature >= 90 {
		severity = SeverityCritical
	} else if health.Temperature >= 80 {
		severity = SeverityError
	}

	return &DiagnosisResult{
		ID:          fmt.Sprintf("diag_temp_%d", time.Now().UnixNano()),
		Timestamp:   time.Now(),
		Severity:    severity,
		Category:    "temperature",
		Title:       "系统温度过高",
		Description: fmt.Sprintf("当前系统温度为 %.1f°C", health.Temperature),
		RootCause:   "散热不良、环境温度过高或负载过重",
		Evidence: []Evidence{
			{Type: "metric", Source: "sensor", Value: health.Temperature, Expected: "< 70°C", Message: "温度超出正常范围"},
		},
		Suggestions: []Suggestion{
			{Action: "检查散热系统", Description: "确认风扇正常运转，散热器无灰尘", Priority: 1, Risk: "low"},
			{Action: "降低系统负载", Description: "减少运行的服务或任务", Priority: 2, Risk: "low"},
			{Action: "改善通风", Description: "确保设备周围有足够的通风空间", Priority: 3, Risk: "low"},
		},
	}
}

// detectAnomalies 异常检测
func (d *DiagnosticAgent) detectAnomalies(health *SystemHealth) {
	baseline := d.anomalyBaseline
	if baseline.SampleCount < 10 {
		// 样本不足，更新基线
		d.updateBaseline(health)
		return
	}

	// 使用3-sigma规则检测异常
	cpuZScore := (health.CPUUsage - baseline.CPUMean) / baseline.CPUStdDev
	memZScore := (health.MemoryUsage - baseline.MemMean) / baseline.MemStdDev
	tempZScore := (health.Temperature - baseline.TempMean) / baseline.TempStdDev

	if cpuZScore > 3 || memZScore > 3 || tempZScore > 3 {
		diagnosis := &DiagnosisResult{
			ID:        fmt.Sprintf("diag_anomaly_%d", time.Now().UnixNano()),
			Timestamp: time.Now(),
			Severity:  SeverityWarning,
			Category:  "anomaly",
			Title:     "检测到系统异常",
			Description: "系统指标偏离正常基线，可能存在异常情况",
			RootCause: "指标偏离历史基线超过3个标准差",
		}

		if cpuZScore > 3 {
			diagnosis.Evidence = append(diagnosis.Evidence, Evidence{
				Type: "anomaly", Source: "cpu",
				Value: health.CPUUsage, Expected: baseline.CPUMean,
				Message: fmt.Sprintf("CPU使用率异常偏离 (Z-score: %.2f)", cpuZScore),
			})
		}

		d.diagnoses = append(d.diagnoses, diagnosis)
	}

	// 更新基线
	d.updateBaseline(health)
}

// updateBaseline 更新异常检测基线（增量平均）
func (d *DiagnosticAgent) updateBaseline(health *SystemHealth) {
	b := d.anomalyBaseline
	n := float64(b.SampleCount)
	b.SampleCount++

	// 增量计算均值
	b.CPUMean = (b.CPUMean*n + health.CPUUsage) / float64(b.SampleCount)
	b.MemMean = (b.MemMean*n + health.MemoryUsage) / float64(b.SampleCount)
	b.TempMean = (b.TempMean*n + health.Temperature) / float64(b.SampleCount)

	// 简化的标准差估算
	b.CPUStdDev = max(b.CPUStdDev, 5.0)
	b.MemStdDev = max(b.MemStdDev, 5.0)
	b.TempStdDev = max(b.TempStdDev, 3.0)
	b.UpdatedAt = time.Now()
}

// runPredictions 运行预测分析
func (d *DiagnosticAgent) runPredictions(health *SystemHealth) {
	// 磁盘空间预测
	if health.DiskUsage > 70 {
		// 简化预测：假设每天增长0.5%
		daysToFull := (95.0 - health.DiskUsage) / 0.5
		if daysToFull < 30 {
			d.predictions = append(d.predictions, &Prediction{
				ID:         fmt.Sprintf("pred_disk_%d", time.Now().UnixNano()),
				Timestamp:  time.Now(),
				Category:   "disk",
				Prediction: fmt.Sprintf("磁盘空间预计在 %.0f 天后耗尽", daysToFull),
				Confidence: 0.75,
				ETA:        fmt.Sprintf("%.0f天", daysToFull),
				Prevention: []string{"清理日志文件", "删除不需要的快照", "扩展存储容量"},
			})
		}
	}
}

// calculateHealthScore 计算健康评分
func (d *DiagnosticAgent) calculateHealthScore(health *SystemHealth, issues, warnings int) int {
	score := 100

	// 根据资源使用情况扣分
	if health.CPUUsage > 90 {
		score -= 15
	} else if health.CPUUsage > 80 {
		score -= 10
	}

	if health.MemoryUsage > 90 {
		score -= 15
	} else if health.MemoryUsage > 80 {
		score -= 10
	}

	if health.DiskUsage > 90 {
		score -= 20
	} else if health.DiskUsage > 80 {
		score -= 10
	}

	if health.Temperature > 80 {
		score -= 15
	} else if health.Temperature > 70 {
		score -= 5
	}

	// 根据问题和警告扣分
	score -= issues * 10
	score -= warnings * 5

	if score < 0 {
		score = 0
	}
	return score
}

// getRecommendations 从诊断结果中提取建议
func (d *DiagnosticAgent) getRecommendations(result *DiagnosisResult) []string {
	var recs []string
	for _, s := range result.Suggestions {
		if s.Priority <= 2 {
			recs = append(recs, s.Description)
		}
	}
	return recs
}

// GetDiagnoses 获取诊断历史
func (d *DiagnosticAgent) GetDiagnoses(limit int) []*DiagnosisResult {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if limit <= 0 || limit > len(d.diagnoses) {
		limit = len(d.diagnoses)
	}
	start := len(d.diagnoses) - limit
	if start < 0 {
		start = 0
	}
	return d.diagnoses[start:]
}

// GetPredictions 获取预测告警
func (d *DiagnosticAgent) GetPredictions() []*Prediction {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.predictions
}

// AddAlertRule 添加自定义告警规则
func (d *DiagnosticAgent) AddAlertRule(rule *AlertRule) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.alertRules = append(d.alertRules, rule)
	log.Printf("[DiagnosticAgent] 添加告警规则: %s", rule.Name)
}

// ListAlertRules 列出所有告警规则
func (d *DiagnosticAgent) ListAlertRules() []*AlertRule {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.alertRules
}

// max 返回两个float64中较大的一个
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
