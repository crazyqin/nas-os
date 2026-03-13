package tiering

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMetrics_NewMetrics(t *testing.T) {
	metrics := NewMetrics()

	assert.NotNil(t, metrics)
	assert.NotNil(t, metrics.tierMetrics)
	assert.NotNil(t, metrics.migrationMetrics)
	assert.NotNil(t, metrics.policyMetrics)
	assert.NotNil(t, metrics.accessMetrics)
}

func TestMetrics_TierMetrics(t *testing.T) {
	metrics := NewMetrics()

	stats := &TierStats{
		Type:         TierTypeSSD,
		Name:         "SSD",
		Capacity:     1000000000,
		Used:         500000000,
		Available:    500000000,
		UsagePercent: 50,
		TotalFiles:   1000,
		HotFiles:     100,
		WarmFiles:    300,
		ColdFiles:    600,
	}

	metrics.UpdateTierMetrics(TierTypeSSD, stats)

	result := metrics.GetTierMetrics(TierTypeSSD)
	assert.Equal(t, int64(1000000000), result.TotalBytes)
	assert.Equal(t, int64(500000000), result.UsedBytes)
	assert.Equal(t, int64(1000), result.TotalFiles)
	assert.Equal(t, int64(100), result.HotFiles)
}

func TestMetrics_RecordTierIO(t *testing.T) {
	metrics := NewMetrics()

	metrics.RecordTierIO(TierTypeSSD, 1024, 512, 10, 5)

	result := metrics.GetTierMetrics(TierTypeSSD)
	assert.Equal(t, int64(1024), result.ReadBytes)
	assert.Equal(t, int64(512), result.WriteBytes)
	assert.Equal(t, int64(10), result.ReadOps)
	assert.Equal(t, int64(5), result.WriteOps)
}

func TestMetrics_RecordTierMigration(t *testing.T) {
	metrics := NewMetrics()

	metrics.RecordTierMigration(TierTypeSSD, 5, 10, 1024, 2048)

	result := metrics.GetTierMetrics(TierTypeSSD)
	assert.Equal(t, int64(5), result.FilesMigratedIn)
	assert.Equal(t, int64(10), result.FilesMigratedOut)
	assert.Equal(t, int64(1024), result.BytesMigratedIn)
	assert.Equal(t, int64(2048), result.BytesMigratedOut)
}

func TestMetrics_MigrationMetrics(t *testing.T) {
	metrics := NewMetrics()

	// 记录迁移开始
	metrics.RecordMigrationStart()
	assert.Equal(t, int64(1), metrics.GetMigrationMetrics().TotalTasks)
	assert.Equal(t, int64(1), metrics.GetMigrationMetrics().RunningTasks)

	// 记录迁移完成
	task := &MigrateTask{
		ProcessedBytes: 1024,
		ProcessedFiles: 10,
		FailedFiles:    1,
		TotalBytes:     2048,
		TotalFiles:     11,
	}
	metrics.RecordMigrationComplete(task, 5000)

	result := metrics.GetMigrationMetrics()
	assert.Equal(t, int64(0), result.RunningTasks)
	assert.Equal(t, int64(1), result.CompletedTasks)
	assert.Equal(t, int64(1024), result.TotalBytesMigrated)
	assert.Equal(t, int64(10), result.TotalFilesMigrated)
	assert.Equal(t, int64(5000), result.AverageMigrationTimeMs)
}

func TestMetrics_PolicyMetrics(t *testing.T) {
	metrics := NewMetrics()

	metrics.UpdatePolicyMetrics("policy-1", true, time.Now(), time.Now().Add(time.Hour))

	result := metrics.GetPolicyMetrics("policy-1")
	assert.True(t, result.Enabled)

	metrics.RecordPolicyExecution("policy-1", true)
	result = metrics.GetPolicyMetrics("policy-1")
	assert.Equal(t, int64(1), result.RunCount)
	assert.Equal(t, int64(1), result.SuccessCount)

	metrics.RecordPolicyExecution("policy-1", false)
	result = metrics.GetPolicyMetrics("policy-1")
	assert.Equal(t, int64(2), result.RunCount)
	assert.Equal(t, int64(1), result.FailureCount)
}

func TestMetrics_AccessMetrics(t *testing.T) {
	metrics := NewMetrics()

	stats := &AccessStats{
		TotalFiles:      1000,
		TotalAccesses:   5000,
		TotalReadBytes:  1000000,
		TotalWriteBytes: 500000,
		HotFiles:        100,
		WarmFiles:       300,
		ColdFiles:       600,
	}

	metrics.UpdateAccessMetrics(stats)

	result := metrics.GetAccessMetrics()
	assert.Equal(t, int64(1000), result.TotalFiles)
	assert.Equal(t, int64(5000), result.TotalAccesses)
	assert.Equal(t, int64(1000000), result.TotalReadBytes)
	assert.Equal(t, int64(500000), result.TotalWriteBytes)
}

func TestMetrics_GetSummary(t *testing.T) {
	metrics := NewMetrics()

	// 添加一些数据
	stats := &TierStats{
		Type:       TierTypeSSD,
		Name:       "SSD",
		Capacity:   1000000,
		Used:       500000,
	}
	metrics.UpdateTierMetrics(TierTypeSSD, stats)

	metrics.UpdatePolicyMetrics("policy-1", true, time.Now(), time.Now().Add(time.Hour))

	summary := metrics.GetSummary()

	assert.GreaterOrEqual(t, summary.TotalTiers, 1)
	assert.GreaterOrEqual(t, summary.TotalPolicies, 1)
	assert.GreaterOrEqual(t, summary.ActivePolicies, 1)
}

func TestMetrics_ExportPrometheus(t *testing.T) {
	metrics := NewMetrics()

	// 添加一些数据
	stats := &TierStats{
		Type:       TierTypeSSD,
		Name:       "SSD",
		Capacity:   1000000,
		Used:       500000,
	}
	metrics.UpdateTierMetrics(TierTypeSSD, stats)

	metrics.UpdateAccessMetrics(&AccessStats{
		TotalFiles: 1000,
	})

	output := metrics.ExportPrometheus()

	assert.Contains(t, output, "nas_tier_capacity_bytes")
	assert.Contains(t, output, "nas_tiering_total_tracked_files")
}

func TestAtomicCounter(t *testing.T) {
	counter := &AtomicCounter{}

	assert.Equal(t, int64(0), counter.Get())

	counter.Increment()
	assert.Equal(t, int64(1), counter.Get())

	counter.IncrementBy(5)
	assert.Equal(t, int64(6), counter.Get())

	counter.Reset()
	assert.Equal(t, int64(0), counter.Get())
}

func TestMetrics_AllTierMetrics(t *testing.T) {
	metrics := NewMetrics()

	metrics.UpdateTierMetrics(TierTypeSSD, &TierStats{Type: TierTypeSSD, Name: "SSD"})
	metrics.UpdateTierMetrics(TierTypeHDD, &TierStats{Type: TierTypeHDD, Name: "HDD"})

	allMetrics := metrics.GetAllTierMetrics()
	assert.Len(t, allMetrics, 2)
}

func TestMetrics_AllPolicyMetrics(t *testing.T) {
	metrics := NewMetrics()

	metrics.UpdatePolicyMetrics("policy-1", true, time.Now(), time.Now())
	metrics.UpdatePolicyMetrics("policy-2", false, time.Now(), time.Now())

	allMetrics := metrics.GetAllPolicyMetrics()
	assert.Len(t, allMetrics, 2)
}