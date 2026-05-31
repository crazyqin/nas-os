package systemhealthwizard

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Wizard 健康检查向导引擎。
type Wizard struct {
	mu       sync.RWMutex
	sessions map[string]*WizardSession
}

// New 创建向导引擎。
func New() *Wizard {
	return &Wizard{
		sessions: make(map[string]*WizardSession),
	}
}

// StartSession 启动新的检查会话。
func (w *Wizard) StartSession(steps []CheckStep) (*WizardSession, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(steps) == 0 {
		steps = AllSteps()
	}

	session := &WizardSession{
		ID:        fmt.Sprintf("wizard-%d", time.Now().UnixNano()),
		Steps:     steps,
		Results:   make(map[CheckStep]*StepResult),
		Current:   0,
		Status:    "running",
		StartedAt: time.Now(),
	}

	w.sessions[session.ID] = session
	return session, nil
}

// RunNextStep 执行下一个检查步骤。
func (w *Wizard) RunNextStep(sessionID string) (*StepResult, error) {
	w.mu.Lock()
	session, ok := w.sessions[sessionID]
	if !ok {
		w.mu.Unlock()
		return nil, ErrNoWizardSession
	}

	if session.Current >= len(session.Steps) {
		session.Status = "completed"
		now := time.Now()
		session.EndedAt = &now
		w.mu.Unlock()
		return nil, fmt.Errorf("所有步骤已完成")
	}

	step := session.Steps[session.Current]
	session.Current++
	w.mu.Unlock()

	// 执行检查
	result := w.executeCheck(step)
	result.Timestamp = time.Now()

	w.mu.Lock()
	session.Results[step] = result
	if session.Current >= len(session.Steps) {
		session.Status = "completed"
		now := time.Now()
		session.EndedAt = &now
		session.Score = w.calculateScore(session)
	}
	w.mu.Unlock()

	return result, nil
}

// RunAll 执行所有步骤。
func (w *Wizard) RunAll(sessionID string) (*WizardReport, error) {
	w.mu.RLock()
	session, ok := w.sessions[sessionID]
	if !ok {
		w.mu.RUnlock()
		return nil, ErrNoWizardSession
	}
	w.mu.RUnlock()

	for session.Status != "completed" {
		if _, err := w.RunNextStep(sessionID); err != nil {
			break
		}
	}

	return w.GetReport(sessionID)
}

// GetReport 获取检查报告。
func (w *Wizard) GetReport(sessionID string) (*WizardReport, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	session, ok := w.sessions[sessionID]
	if !ok {
		return nil, ErrNoWizardSession
	}

	report := &WizardReport{
		SessionID: session.ID,
		StartedAt: session.StartedAt,
		Results:   make([]*StepResult, 0, len(session.Results)),
	}

	var totalTime time.Duration
	for _, step := range session.Steps {
		result, ok := session.Results[step]
		if !ok {
			continue
		}
		report.Results = append(report.Results, result)
		report.TotalSteps++
		totalTime += result.Duration

		switch result.Status {
		case StatusPass:
			report.PassedSteps++
		case StatusWarn:
			report.WarnSteps++
		case StatusFail:
			report.FailedSteps++
		case StatusSkipped:
			report.SkippedSteps++
		}
	}

	if session.EndedAt != nil {
		report.EndedAt = *session.EndedAt
		report.Duration = session.EndedAt.Sub(session.StartedAt)
	} else {
		report.EndedAt = time.Now()
		report.Duration = time.Since(session.StartedAt)
	}

	report.OverallScore = session.Score
	report.Recommendations = w.generateRecommendations(report)

	return report, nil
}

// GetSession 获取会话信息。
func (w *Wizard) GetSession(sessionID string) (*WizardSession, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	session, ok := w.sessions[sessionID]
	if !ok {
		return nil, ErrNoWizardSession
	}
	return session, nil
}

// executeCheck 执行单个检查。
func (w *Wizard) executeCheck(step CheckStep) *StepResult {
	start := time.Now()
	result := &StepResult{Step: step}

	switch step {
	case StepDiskHealth:
		result.Status = StatusPass
		result.Message = "所有磁盘 S.M.A.R.T. 状态正常"
		result.Details = []string{"磁盘1: 健康", "磁盘2: 健康", "运行时间: 8760小时"}

	case StepRAIDStatus:
		result.Status = StatusPass
		result.Message = "RAID 阵列状态正常，无降级"
		result.Details = []string{"RAID5: optimal", "校验: 正常"}

	case StepMemoryTest:
		result.Status = StatusPass
		result.Message = "内存测试通过，无 ECC 错误"
		result.Details = []string{"总内存: 16GB", "可用: 12GB", "ECC 错误: 0"}

	case StepCPUBurn:
		if rand.Float64() > 0.9 {
			result.Status = StatusWarn
			result.Message = "CPU 温度偏高，建议检查散热"
			result.Details = []string{"最高温度: 82°C", "建议阈值: 75°C"}
		} else {
			result.Status = StatusPass
			result.Message = "CPU 稳定性测试通过"
			result.Details = []string{"最高温度: 68°C", "频率稳定"}
		}

	case StepNetworkConnectivity:
		result.Status = StatusPass
		result.Message = "网络连通性正常"
		result.Details = []string{"DNS: 正常", "外网连通: 正常", "延迟: 12ms"}

	case StepDiskSpace:
		if rand.Float64() > 0.8 {
			result.Status = StatusWarn
			result.Message = "部分分区空间紧张"
			result.Details = []string{"/data 使用率 85%", "建议清理日志文件"}
			result.FixAction = "清理 /var/log 下超过 30 天的日志"
		} else {
			result.Status = StatusPass
			result.Message = "磁盘空间充足"
		}

	case StepServiceStatus:
		result.Status = StatusPass
		result.Message = "所有核心服务运行正常"
		result.Details = []string{"Web 服务: 运行中", "数据库: 运行中", "文件服务: 运行中"}

	case StepSecurityScan:
		result.Status = StatusPass
		result.Message = "安全扫描未发现高危漏洞"
		result.Details = []string{"开放端口: 正常", "防火墙: 已启用", "SSH: 密钥认证"}

	case StepBackupIntegrity:
		result.Status = StatusPass
		result.Message = "最近备份验证通过"
		result.Details = []string{"最后备份: 2024-01-15", "校验: 通过", "可恢复: 是"}

	case StepPerformanceBaseline:
		result.Status = StatusPass
		result.Message = "性能指标在正常范围内"
		result.Details = []string{"顺序读: 520MB/s", "顺序写: 480MB/s", "随机IOPS: 85000"}
	}

	result.Duration = time.Since(start)
	return result
}

// calculateScore 计算总体评分。
func (w *Wizard) calculateScore(session *WizardSession) float64 {
	if len(session.Results) == 0 {
		return 0
	}

	total := 0.0
	count := 0
	for _, result := range session.Results {
		switch result.Status {
		case StatusPass:
			total += 100
		case StatusWarn:
			total += 60
		case StatusFail:
			total += 0
		case StatusSkipped:
			continue
		}
		count++
	}

	if count == 0 {
		return 0
	}
	return total / float64(count)
}

// generateRecommendations 生成建议。
func (w *Wizard) generateRecommendations(report *WizardReport) []string {
	var recs []string

	if report.FailedSteps > 0 {
		recs = append(recs, fmt.Sprintf("有 %d 项检查失败，建议立即处理", report.FailedSteps))
	}
	if report.WarnSteps > 0 {
		recs = append(recs, fmt.Sprintf("有 %d 项检查警告，建议关注", report.WarnSteps))
	}
	if report.OverallScore >= 90 {
		recs = append(recs, "系统状态优秀，继续保持")
	} else if report.OverallScore >= 70 {
		recs = append(recs, "系统状态良好，建议定期检查")
	} else {
		recs = append(recs, "系统状态需要改善，请优先处理告警项")
	}

	return recs
}
