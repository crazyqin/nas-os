// Package diskhealthai 单元测试
package diskhealthai

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHealthStatus_Constants(t *testing.T) {
	assert.Equal(t, HealthStatus("excellent"), StatusExcellent)
	assert.Equal(t, HealthStatus("good"), StatusGood)
	assert.Equal(t, HealthStatus("fair"), StatusFair)
	assert.Equal(t, HealthStatus("poor"), StatusPoor)
	assert.Equal(t, HealthStatus("critical"), StatusCritical)
	assert.Equal(t, HealthStatus("failed"), StatusFailed)
}

func TestDiskInfo_Fields(t *testing.T) {
	now := time.Now()
	disk := DiskInfo{
		Device:        "/dev/sda",
		Model:         "WD Red Plus",
		Serial:        "WD-WCC1234567",
		CapacityBytes: 4 * 1024 * 1024 * 1024 * 1024,
		Firmware:      "01.01A01",
		Interface:     "SATA",
		Status:        StatusGood,
		SMARTEnabled:  true,
		RegisteredAt:  now,
		LastScanAt:    now,
	}

	assert.Equal(t, "/dev/sda", disk.Device)
	assert.Equal(t, "WD Red Plus", disk.Model)
	assert.Equal(t, uint64(4*1024*1024*1024*1024), disk.CapacityBytes)
	assert.Equal(t, StatusGood, disk.Status)
	assert.True(t, disk.SMARTEnabled)
}

func TestSMARTAttribute_Fields(t *testing.T) {
	attr := SMARTAttribute{
		ID:          5,
		Name:        "Reallocated_Sector_Ct",
		Value:       200,
		Worst:       200,
		Threshold:   140,
		RawValue:    0,
		IsCritical:  true,
		Failed:      false,
		Description: "重映射扇区计数",
	}

	assert.Equal(t, 5, attr.ID)
	assert.Equal(t, "Reallocated_Sector_Ct", attr.Name)
	assert.True(t, attr.IsCritical)
	assert.False(t, attr.Failed)
}

func TestSMARTSnapshot_Fields(t *testing.T) {
	snapshot := SMARTSnapshot{
		Device: "/dev/sda",
		Model:  "WD Red Plus",
	}

	assert.Equal(t, "/dev/sda", snapshot.Device)
	assert.Equal(t, "WD Red Plus", snapshot.Model)
}

func TestHealthReport_Fields(t *testing.T) {
	now := time.Now()
	report := HealthReport{
		Device:            "/dev/sda",
		Model:             "WD Red Plus",
		Serial:            "WD-WCC1234567",
		HealthScore:       85.5,
		Status:            StatusGood,
		Grade:             "A",
		EstimatedLifeDays: 365,
		RiskLevel:         "low",
		RiskFactors:       []RiskFactor{},
		Recommendations:   []string{"定期检查"},
		AnalyzedAt:        now,
	}

	assert.Equal(t, "/dev/sda", report.Device)
	assert.Equal(t, 85.5, report.HealthScore)
	assert.Equal(t, StatusGood, report.Status)
	assert.Equal(t, "A", report.Grade)
	assert.Equal(t, 365, report.EstimatedLifeDays)
}

func TestRiskFactor_Fields(t *testing.T) {
	rf := RiskFactor{
		ID:     "temp_high",
		Name:   "温度偏高",
		Level:  "medium",
		Weight: 0.3,
		Detail: "平均温度超过50°C",
	}

	assert.Equal(t, "temp_high", rf.ID)
	assert.Equal(t, "温度偏高", rf.Name)
	assert.Equal(t, "medium", rf.Level)
	assert.Equal(t, 0.3, rf.Weight)
}

func TestDimensionScores_Fields(t *testing.T) {
	scores := DimensionScores{
		SMARTScore: 90.0,
	}

	assert.Equal(t, 90.0, scores.SMARTScore)
}

func TestTrendAnalysis_Fields(t *testing.T) {
	trend := TrendAnalysis{
		HealthTrend:          "stable",
		TemperatureTrend:     "stable",
		WorkloadTrend:        "increasing",
		ProjectedScore90D:    82.0,
		ProjectionConfidence: 0.85,
	}

	assert.Equal(t, "stable", trend.HealthTrend)
	assert.Equal(t, 82.0, trend.ProjectedScore90D)
	assert.Equal(t, 0.85, trend.ProjectionConfidence)
}

func TestLifecycleStage_Constants(t *testing.T) {
	assert.NotEmpty(t, LifecycleStage("new"))
	assert.NotEmpty(t, LifecycleStage("aging"))
	assert.NotEmpty(t, LifecycleStage("end_of_life"))
}

func TestAlert_Fields(t *testing.T) {
	alert := Alert{
		ID:      "alert-1",
		Device:  "/dev/sda",
		Level:   "warning",
		Message: "温度偏高",
	}

	assert.Equal(t, "alert-1", alert.ID)
	assert.Equal(t, "warning", alert.Level)
}

func TestDiagnoseRequest_Fields(t *testing.T) {
	req := DiagnoseRequest{
		Device: "/dev/sda",
	}

	assert.Equal(t, "/dev/sda", req.Device)
}

func TestDiagnoseResponse_Fields(t *testing.T) {
	resp := DiagnoseResponse{
		Code:    200,
		Message: "success",
	}

	assert.Equal(t, 200, resp.Code)
	assert.Equal(t, "success", resp.Message)
}
