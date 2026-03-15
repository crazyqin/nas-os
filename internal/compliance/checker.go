// Package compliance 提供合规检查功能
package compliance

import (
	"context"
	"fmt"
	"time"
)

// ComplianceLevel 合规级别
type ComplianceLevel string

const (
	LevelA ComplianceLevel = "A" // 完全合规
	LevelB ComplianceLevel = "B" // 基本合规
	LevelC ComplianceLevel = "C" // 部分合规
	LevelD ComplianceLevel = "D" // 不合规
)

// CheckType 检查类型
type CheckType string

const (
	CheckSecurity CheckType = "security"
	CheckAccess   CheckType = "access"
	CheckData     CheckType = "data"
	CheckAudit    CheckType = "audit"
	CheckPrivacy  CheckType = "privacy"
)

// CheckResult 检查结果
type CheckResult struct {
	ID          string                 `json:"id"`
	Type        CheckType              `json:"type"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Level       ComplianceLevel        `json:"level"`
	Passed      bool                   `json:"passed"`
	Message     string                 `json:"message"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
}

// ComplianceChecker 合规检查器
type ComplianceChecker struct {
	checks []ComplianceCheck
}

// ComplianceCheck 合规检查接口
type ComplianceCheck interface {
	ID() string
	Type() CheckType
	Name() string
	Description() string
	Execute(ctx context.Context) (CheckResult, error)
}

// NewComplianceChecker 创建合规检查器
func NewComplianceChecker() *ComplianceChecker {
	return &ComplianceChecker{
		checks: make([]ComplianceCheck, 0),
	}
}

// RegisterCheck 注册检查项
func (c *ComplianceChecker) RegisterCheck(check ComplianceCheck) {
	c.checks = append(c.checks, check)
}

// RunChecks 运行所有检查
func (c *ComplianceChecker) RunChecks(ctx context.Context) (*ComplianceReport, error) {
	report := &ComplianceReport{
		ID:        generateReportID(),
		Timestamp: time.Now(),
		Results:   make([]CheckResult, 0),
	}

	for _, check := range c.checks {
		result, err := check.Execute(ctx)
		if err != nil {
			result = CheckResult{
				ID:          check.ID(),
				Type:        check.Type(),
				Name:        check.Name(),
				Description: check.Description(),
				Level:       LevelD,
				Passed:      false,
				Message:     fmt.Sprintf("检查执行失败: %v", err),
				Timestamp:   time.Now(),
			}
		}
		report.Results = append(report.Results, result)
	}

	// 计算总体合规级别
	report.OverallLevel = calculateOverallLevel(report.Results)
	report.PassedCount = countPassed(report.Results)
	report.FailedCount = len(report.Results) - report.PassedCount

	return report, nil
}

// RunChecksByType 按类型运行检查
func (c *ComplianceChecker) RunChecksByType(ctx context.Context, checkType CheckType) (*ComplianceReport, error) {
	report := &ComplianceReport{
		ID:        generateReportID(),
		Timestamp: time.Now(),
		Results:   make([]CheckResult, 0),
	}

	for _, check := range c.checks {
		if check.Type() == checkType {
			result, err := check.Execute(ctx)
			if err != nil {
				result = CheckResult{
					ID:          check.ID(),
					Type:        check.Type(),
					Name:        check.Name(),
					Description: check.Description(),
					Level:       LevelD,
					Passed:      false,
					Message:     fmt.Sprintf("检查执行失败: %v", err),
					Timestamp:   time.Now(),
				}
			}
			report.Results = append(report.Results, result)
		}
	}

	report.OverallLevel = calculateOverallLevel(report.Results)
	report.PassedCount = countPassed(report.Results)
	report.FailedCount = len(report.Results) - report.PassedCount

	return report, nil
}

// GetRegisteredChecks 获取已注册的检查项
func (c *ComplianceChecker) GetRegisteredChecks() []ComplianceCheck {
	return c.checks
}

func calculateOverallLevel(results []CheckResult) ComplianceLevel {
	if len(results) == 0 {
		return LevelD
	}

	passRate := float64(countPassed(results)) / float64(len(results))

	switch {
	case passRate >= 0.9:
		return LevelA
	case passRate >= 0.7:
		return LevelB
	case passRate >= 0.5:
		return LevelC
	default:
		return LevelD
	}
}

func countPassed(results []CheckResult) int {
	count := 0
	for _, r := range results {
		if r.Passed {
			count++
		}
	}
	return count
}

func generateReportID() string {
	return fmt.Sprintf("compliance_%d", time.Now().UnixNano())
}
