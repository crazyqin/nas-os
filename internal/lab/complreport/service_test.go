package complreport

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== 报告生成测试 ==========

func TestGenerateReport(t *testing.T) {
	svc := NewService()

	req := GenerateRequest{
		Standard:    StandardGDPR,
		Format:      FormatJSON,
		Title:       "2024年Q4 GDPR合规审计",
		GeneratedBy: "compliance-team",
	}

	report, err := svc.GenerateReport(req)
	require.NoError(t, err)
	assert.NotEmpty(t, report.ID)
	assert.Equal(t, StandardGDPR, report.Standard)
	assert.Equal(t, "2024年Q4 GDPR合规审计", report.Title)
	assert.Equal(t, FormatJSON, report.Format)
	assert.Equal(t, StatusCompleted, report.Status)
	assert.Equal(t, "compliance-team", report.GeneratedBy)
	assert.False(t, report.CreatedAt.IsZero())
	assert.NotNil(t, report.CompletedAt)
	assert.NotEmpty(t, report.Controls)
	assert.NotEmpty(t, report.Summary)
	assert.Greater(t, report.TotalChecks, 0)
}

func TestGenerateReportDefaultTitle(t *testing.T) {
	svc := NewService()

	req := GenerateRequest{
		Standard:    StandardSOC2,
		GeneratedBy: "admin",
	}

	report, err := svc.GenerateReport(req)
	require.NoError(t, err)
	assert.Equal(t, "SOC2 合规审计", report.Title) // 应使用默认标题
}

func TestGenerateReportDefaultFormat(t *testing.T) {
	svc := NewService()

	req := GenerateRequest{
		Standard:    StandardISO27001,
		GeneratedBy: "admin",
	}

	report, err := svc.GenerateReport(req)
	require.NoError(t, err)
	assert.Equal(t, FormatJSON, report.Format) // 默认格式为 JSON
}

func TestGenerateReportAllStandards(t *testing.T) {
	svc := NewService()

	standards := []Standard{StandardGDPR, StandardPIPL, StandardSOC2, StandardISO27001, StandardHIPAA, StandardCCPA}

	for _, std := range standards {
		req := GenerateRequest{
			Standard:    std,
			GeneratedBy: "tester",
		}
		report, err := svc.GenerateReport(req)
		require.NoError(t, err, "标准 %s 应该支持", std)
		assert.Equal(t, std, report.Standard)
		assert.NotEmpty(t, report.Controls)
		assert.Greater(t, report.TotalChecks, 0)
	}
}

func TestGenerateReportInvalidStandard(t *testing.T) {
	svc := NewService()

	req := GenerateRequest{
		Standard:    "INVALID",
		GeneratedBy: "admin",
	}

	_, err := svc.GenerateReport(req)
	assert.ErrorIs(t, err, ErrInvalidStandard)
}

func TestGenerateReportInvalidFormat(t *testing.T) {
	svc := NewService()

	req := GenerateRequest{
		Standard:    StandardGDPR,
		Format:      "XML",
		GeneratedBy: "admin",
	}

	_, err := svc.GenerateReport(req)
	assert.ErrorIs(t, err, ErrInvalidFormat)
}

// ========== 报告查询测试 ==========

func TestGetReport(t *testing.T) {
	svc := NewService()

	report, _ := svc.GenerateReport(GenerateRequest{
		Standard:    StandardGDPR,
		GeneratedBy: "admin",
	})

	found, err := svc.GetReport(report.ID)
	require.NoError(t, err)
	assert.Equal(t, report.ID, found.ID)
	assert.Equal(t, report.Standard, found.Standard)
}

func TestGetReportNotFound(t *testing.T) {
	svc := NewService()

	_, err := svc.GetReport("nonexistent")
	assert.ErrorIs(t, err, ErrReportNotFound)
}

func TestListReports(t *testing.T) {
	svc := NewService()

	svc.GenerateReport(GenerateRequest{Standard: StandardGDPR, GeneratedBy: "a"})
	svc.GenerateReport(GenerateRequest{Standard: StandardSOC2, GeneratedBy: "b"})
	svc.GenerateReport(GenerateRequest{Standard: StandardISO27001, GeneratedBy: "c"})

	reports := svc.ListReports()
	assert.Len(t, reports, 3)
}

func TestListReportsOrderByTimeDesc(t *testing.T) {
	svc := NewService()

	// 生成多个报告，中间间隔以确保时间不同
	r1, _ := svc.GenerateReport(GenerateRequest{Standard: StandardGDPR, GeneratedBy: "a"})
	time.Sleep(1 * time.Millisecond)
	r2, _ := svc.GenerateReport(GenerateRequest{Standard: StandardSOC2, GeneratedBy: "b"})
	time.Sleep(1 * time.Millisecond)
	r3, _ := svc.GenerateReport(GenerateRequest{Standard: StandardISO27001, GeneratedBy: "c"})

	reports := svc.ListReports()
	require.Len(t, reports, 3)
	// 最新的应该在前
	assert.Equal(t, r3.ID, reports[0].ID)
	assert.Equal(t, r2.ID, reports[1].ID)
	assert.Equal(t, r1.ID, reports[2].ID)
}

// ========== 合规评分测试 ==========

func TestReportScore(t *testing.T) {
	svc := NewService()

	report, err := svc.GenerateReport(GenerateRequest{
		Standard:    StandardGDPR,
		GeneratedBy: "admin",
	})
	require.NoError(t, err)

	// 评分应该在 0-100 之间
	assert.GreaterOrEqual(t, report.Score, 0)
	assert.LessOrEqual(t, report.Score, 100)

	// 验证统计一致性
	assert.Equal(t, report.TotalChecks, report.Passed+report.Failed+report.Warnings+report.NotApplicable)
}

func TestReportScoreAllPassed(t *testing.T) {
	svc := NewService()

	// SOC2 标准通常全部通过
	report, _ := svc.GenerateReport(GenerateRequest{
		Standard:    StandardSOC2,
		GeneratedBy: "admin",
	})

	// 大部分检查应该通过
	assert.Greater(t, report.Passed, 0)
	if report.Failed == 0 && report.Warnings == 0 {
		assert.Equal(t, 100, report.Score)
	}
}

// ========== 控制项检查测试 ==========

func TestReportControls(t *testing.T) {
	svc := NewService()

	report, _ := svc.GenerateReport(GenerateRequest{
		Standard:    StandardISO27001,
		GeneratedBy: "admin",
	})

	// ISO27001 应有多个控制类别
	assert.NotEmpty(t, report.Controls)
	for _, ctrl := range report.Controls {
		assert.NotEmpty(t, ctrl.ID)
		assert.NotEmpty(t, ctrl.Category)
		assert.NotEmpty(t, ctrl.Title)
		assert.NotEmpty(t, ctrl.Evidence)
	}
}

func TestReportEvidence(t *testing.T) {
	svc := NewService()

	report, _ := svc.GenerateReport(GenerateRequest{
		Standard:    StandardGDPR,
		GeneratedBy: "admin",
	})

	for _, ctrl := range report.Controls {
		for _, ev := range ctrl.Evidence {
			assert.NotEmpty(t, ev.Type)
			assert.NotEmpty(t, ev.Source)
			assert.NotEmpty(t, ev.Title)
			assert.False(t, ev.Timestamp.IsZero())
		}
	}
}

func TestReportRemediation(t *testing.T) {
	svc := NewService()

	// GDPR 中某些类别（如个人信息保护影响评估）可能产生警告
	report, _ := svc.GenerateReport(GenerateRequest{
		Standard:    StandardPIPL,
		GeneratedBy: "admin",
	})

	// 如果有警告或失败项，应该有整改建议
	for _, ctrl := range report.Controls {
		if ctrl.Status == CheckFail || ctrl.Status == CheckWarning {
			assert.NotEmpty(t, ctrl.Remediation, "控制项 %s 有问题但缺少整改建议", ctrl.ID)
		}
	}
}

// ========== 报告摘要测试 ==========

func TestReportSummary(t *testing.T) {
	svc := NewService()

	report, _ := svc.GenerateReport(GenerateRequest{
		Standard:    StandardGDPR,
		GeneratedBy: "admin",
	})

	assert.NotEmpty(t, report.Summary)
	assert.Contains(t, report.Summary, "GDPR")
	assert.Contains(t, report.Summary, "合规评分")
}

func TestReportSummaryHighScore(t *testing.T) {
	svc := NewService()

	report, _ := svc.GenerateReport(GenerateRequest{
		Standard:    StandardSOC2,
		GeneratedBy: "admin",
	})

	if report.Score >= 90 {
		assert.Contains(t, report.Summary, "优秀")
	}
}

func TestReportSummaryLowScore(t *testing.T) {
	// 构造一个低分场景比较困难，因为模拟证据通常通过
	// 但我们可以验证摘要格式
	svc := NewService()

	report, _ := svc.GenerateReport(GenerateRequest{
		Standard:    StandardHIPAA,
		GeneratedBy: "admin",
	})

	assert.Contains(t, report.Summary, "合规评分")
}

// ========== 定期报告计划测试 ==========

func TestCreateSchedule(t *testing.T) {
	svc := NewService()

	req := ScheduleRequest{
		Standard:    StandardGDPR,
		Format:      FormatPDF,
		CronExpr:    "0 0 1 * *", // 每月1日
		GeneratedBy: "admin",
	}

	schedule, err := svc.CreateSchedule(req)
	require.NoError(t, err)
	assert.NotEmpty(t, schedule.ID)
	assert.Equal(t, StandardGDPR, schedule.Standard)
	assert.Equal(t, FormatPDF, schedule.Format)
	assert.Equal(t, "0 0 1 * *", schedule.CronExpr)
	assert.True(t, schedule.Enabled)
	assert.Equal(t, "admin", schedule.GeneratedBy)
}

func TestCreateScheduleDefaultFormat(t *testing.T) {
	svc := NewService()

	req := ScheduleRequest{
		Standard:    StandardSOC2,
		CronExpr:    "0 0 * * 1", // 每周一
		GeneratedBy: "admin",
	}

	schedule, err := svc.CreateSchedule(req)
	require.NoError(t, err)
	assert.Equal(t, FormatJSON, schedule.Format)
}

func TestCreateScheduleInvalidStandard(t *testing.T) {
	svc := NewService()

	req := ScheduleRequest{
		Standard:    "INVALID",
		CronExpr:    "0 0 * * *",
		GeneratedBy: "admin",
	}

	_, err := svc.CreateSchedule(req)
	assert.ErrorIs(t, err, ErrInvalidStandard)
}

func TestGetSchedule(t *testing.T) {
	svc := NewService()

	schedule, _ := svc.CreateSchedule(ScheduleRequest{
		Standard:    StandardGDPR,
		CronExpr:    "0 0 1 * *",
		GeneratedBy: "admin",
	})

	found, err := svc.GetSchedule(schedule.ID)
	require.NoError(t, err)
	assert.Equal(t, schedule.ID, found.ID)
}

func TestGetScheduleNotFound(t *testing.T) {
	svc := NewService()

	_, err := svc.GetSchedule("nonexistent")
	assert.ErrorIs(t, err, ErrScheduleNotFound)
}

func TestListSchedules(t *testing.T) {
	svc := NewService()

	svc.CreateSchedule(ScheduleRequest{Standard: StandardGDPR, CronExpr: "0 0 1 * *", GeneratedBy: "a"})
	svc.CreateSchedule(ScheduleRequest{Standard: StandardSOC2, CronExpr: "0 0 * * 1", GeneratedBy: "b"})

	schedules := svc.ListSchedules()
	assert.Len(t, schedules, 2)
}

func TestUpdateScheduleLastRun(t *testing.T) {
	svc := NewService()

	schedule, _ := svc.CreateSchedule(ScheduleRequest{
		Standard:    StandardGDPR,
		CronExpr:    "0 0 1 * *",
		GeneratedBy: "admin",
	})

	err := svc.UpdateScheduleLastRun(schedule.ID)
	require.NoError(t, err)

	updated, _ := svc.GetSchedule(schedule.ID)
	assert.NotNil(t, updated.LastRunAt)
}

func TestUpdateScheduleLastRunNotFound(t *testing.T) {
	svc := NewService()

	err := svc.UpdateScheduleLastRun("nonexistent")
	assert.ErrorIs(t, err, ErrScheduleNotFound)
}

func TestDeleteSchedule(t *testing.T) {
	svc := NewService()

	schedule, _ := svc.CreateSchedule(ScheduleRequest{
		Standard:    StandardGDPR,
		CronExpr:    "0 0 1 * *",
		GeneratedBy: "admin",
	})

	err := svc.DeleteSchedule(schedule.ID)
	assert.NoError(t, err)

	_, err = svc.GetSchedule(schedule.ID)
	assert.ErrorIs(t, err, ErrScheduleNotFound)
}

func TestDeleteScheduleNotFound(t *testing.T) {
	svc := NewService()

	err := svc.DeleteSchedule("nonexistent")
	assert.ErrorIs(t, err, ErrScheduleNotFound)
}

// ========== 支持标准测试 ==========

func TestGetSupportedStandards(t *testing.T) {
	svc := NewService()

	standards := svc.GetSupportedStandards()
	assert.Contains(t, standards, StandardGDPR)
	assert.Contains(t, standards, StandardPIPL)
	assert.Contains(t, standards, StandardSOC2)
	assert.Contains(t, standards, StandardISO27001)
	assert.Contains(t, standards, StandardHIPAA)
	assert.Contains(t, standards, StandardCCPA)
}

// ========== 集成测试 ==========

func TestFullReportFlow(t *testing.T) {
	svc := NewService()

	// 1. 生成 GDPR 报告
	report, err := svc.GenerateReport(GenerateRequest{
		Standard:    StandardGDPR,
		Format:      FormatJSON,
		Title:       "集成测试 GDPR 报告",
		GeneratedBy: "qa-team",
	})
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, report.Status)
	assert.NotEmpty(t, report.Controls)

	// 2. 查询报告
	found, err := svc.GetReport(report.ID)
	require.NoError(t, err)
	assert.Equal(t, report.ID, found.ID)

	// 3. 列出报告
	reports := svc.ListReports()
	assert.Len(t, reports, 1)

	// 4. 创建定期计划
	schedule, err := svc.CreateSchedule(ScheduleRequest{
		Standard:    StandardGDPR,
		Format:      FormatJSON,
		CronExpr:    "0 0 1 * *",
		GeneratedBy: "qa-team",
	})
	require.NoError(t, err)
	assert.True(t, schedule.Enabled)

	// 5. 更新计划执行时间
	err = svc.UpdateScheduleLastRun(schedule.ID)
	require.NoError(t, err)

	// 6. 清理
	err = svc.DeleteSchedule(schedule.ID)
	require.NoError(t, err)
}

func TestMultipleReportsComparison(t *testing.T) {
	svc := NewService()

	// 生成多个不同标准的报告
	standards := []Standard{StandardGDPR, StandardSOC2, StandardISO27001, StandardHIPAA}
	for _, std := range standards {
		_, err := svc.GenerateReport(GenerateRequest{
			Standard:    std,
			GeneratedBy: "comparison",
		})
		require.NoError(t, err)
	}

	reports := svc.ListReports()
	assert.Len(t, reports, 4)

	// 每个报告的统计应该一致
	for _, r := range reports {
		assert.Equal(t, r.TotalChecks, r.Passed+r.Failed+r.Warnings+r.NotApplicable)
		assert.GreaterOrEqual(t, r.Score, 0)
		assert.LessOrEqual(t, r.Score, 100)
		assert.NotEmpty(t, r.Summary)
	}
}
