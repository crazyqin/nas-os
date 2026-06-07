package zfsenhanced

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// IntegrityCheckResult 完整性检查结果
type IntegrityCheckResult struct {
	PoolName            string    `json:"pool_name"`
	CheckType           string    `json:"check_type"` // scrub, checksum, file_integrity
	StartTime           time.Time `json:"start_time"`
	EndTime             time.Time `json:"end_time"`
	Duration            string    `json:"duration"`
	Status              string    `json:"status"` // passed, failed, warning
	TotalChecked        int64     `json:"total_checked"`
	ErrorsFound         int64     `json:"errors_found"`
	RepairsMade         int64     `json:"repairs_made"`
	UncorrectableErrors int64     `json:"uncorrectable_errors"`
	Details             string    `json:"details,omitempty"`
}

// ScrubScheduleConfig Scrub调度配置
type ScrubScheduleConfig struct {
	Enabled         bool `json:"enabled"`
	IntervalDays    int  `json:"interval_days"`
	PreferredHour   int  `json:"preferred_hour"`
	IOPSThreshold   int  `json:"iops_threshold"`
	AutoPauseOnLoad bool `json:"auto_pause_on_load"`
	MaxErrorCount   int  `json:"max_error_count"`
	AutoRepair      bool `json:"auto_repair"`
}

// DefaultIntegrityConfig 默认完整性配置
func DefaultIntegrityConfig() ScrubScheduleConfig {
	return ScrubScheduleConfig{
		Enabled:         true,
		IntervalDays:    14,
		PreferredHour:   2,
		IOPSThreshold:   500,
		AutoPauseOnLoad: true,
		MaxErrorCount:   10,
		AutoRepair:      true,
	}
}

// IntegrityChecker 完整性检查器
type IntegrityChecker struct {
	poolManager  *PoolManager
	config       ScrubScheduleConfig
	lastCheck    time.Time
	checkHistory []IntegrityCheckResult
}

// NewIntegrityChecker 创建完整性检查器
func NewIntegrityChecker(pm *PoolManager, config ScrubScheduleConfig) *IntegrityChecker {
	return &IntegrityChecker{
		poolManager:  pm,
		config:       config,
		checkHistory: make([]IntegrityCheckResult, 0),
	}
}

// RunScrub 执行scrub检查
func (ic *IntegrityChecker) RunScrub(ctx context.Context, poolName string) (*IntegrityCheckResult, error) {
	startTime := time.Now()
	result := &IntegrityCheckResult{
		PoolName:  poolName,
		CheckType: "scrub",
		StartTime: startTime,
		Status:    "passed",
	}

	// 执行scrub
	cmd := exec.CommandContext(ctx, "zpool", "scrub", poolName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Status = "failed"
		result.EndTime = time.Now()
		result.Duration = time.Since(startTime).String()
		result.Details = fmt.Sprintf("failed to start scrub: %s, output: %s", err, string(output))
		ic.checkHistory = append(ic.checkHistory, *result)
		return result, fmt.Errorf("failed to start scrub: %w", err)
	}

	// 等待scrub完成（轮询状态）
	for {
		select {
		case <-ctx.Done():
			result.Status = "cancelled"
			result.EndTime = time.Now()
			result.Duration = time.Since(startTime).String()
			ic.checkHistory = append(ic.checkHistory, *result)
			return result, ctx.Err()
		case <-time.After(30 * time.Second):
			status := ic.getScrubStatus(ctx, poolName)
			if status == "completed" {
				result.Status = "passed"
				result.EndTime = time.Now()
				result.Duration = time.Since(startTime).String()
				ic.parseScrubResult(ctx, poolName, result)
				ic.checkHistory = append(ic.checkHistory, *result)
				ic.lastCheck = time.Now()
				return result, nil
			} else if status == "failed" {
				result.Status = "failed"
				result.EndTime = time.Now()
				result.Duration = time.Since(startTime).String()
				result.Details = "scrub failed"
				ic.checkHistory = append(ic.checkHistory, *result)
				return result, fmt.Errorf("scrub failed")
			}
			// 继续等待
		}
	}
}

// CheckChecksums 检查校验和
func (ic *IntegrityChecker) CheckChecksums(ctx context.Context, poolName string) (*IntegrityCheckResult, error) {
	startTime := time.Now()
	result := &IntegrityCheckResult{
		PoolName:  poolName,
		CheckType: "checksum",
		StartTime: startTime,
		Status:    "passed",
	}

	// 获取池状态中的校验和错误
	cmd := exec.CommandContext(ctx, "zpool", "status", poolName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Status = "failed"
		result.EndTime = time.Now()
		result.Duration = time.Since(startTime).String()
		result.Details = err.Error()
		return result, err
	}

	// 解析校验和错误数
	checksumErrors := ic.parseChecksumErrors(string(output))
	result.ErrorsFound = checksumErrors
	result.EndTime = time.Now()
	result.Duration = time.Since(startTime).String()

	if checksumErrors > 0 {
		result.Status = "warning"
		result.Details = fmt.Sprintf("found %d checksum errors", checksumErrors)
	}

	ic.checkHistory = append(ic.checkHistory, *result)
	return result, nil
}

// VerifyPoolIntegrity 验证池完整性
func (ic *IntegrityChecker) VerifyPoolIntegrity(ctx context.Context, poolName string) (*IntegrityCheckResult, error) {
	startTime := time.Now()
	result := &IntegrityCheckResult{
		PoolName:  poolName,
		CheckType: "pool_integrity",
		StartTime: startTime,
		Status:    "passed",
	}

	// 检查池状态
	pool, err := ic.poolManager.GetPoolStatus(ctx, poolName)
	if err != nil {
		result.Status = "failed"
		result.EndTime = time.Now()
		result.Duration = time.Since(startTime).String()
		result.Details = err.Error()
		return result, err
	}

	// 检查池健康状态
	if pool.Status != PoolStatusOnline {
		result.Status = "warning"
		result.Details = fmt.Sprintf("pool status: %s", pool.Status)
	}

	// 检查错误数
	totalErrors := pool.ReadErrors + pool.WriteErrors + pool.ChecksumErrors
	result.ErrorsFound = totalErrors
	result.TotalChecked = pool.SizeBytes
	result.EndTime = time.Now()
	result.Duration = time.Since(startTime).String()

	if totalErrors > 0 {
		result.Status = "warning"
		if result.Details != "" {
			result.Details += "; "
		}
		result.Details += fmt.Sprintf("found %d total errors (read: %d, write: %d, checksum: %d)",
			totalErrors, pool.ReadErrors, pool.WriteErrors, pool.ChecksumErrors)
	}

	ic.checkHistory = append(ic.checkHistory, *result)
	return result, nil
}

// ScheduleScrub 调度scrub
func (ic *IntegrityChecker) ScheduleScrub(ctx context.Context, poolName string) error {
	if !ic.config.Enabled {
		return fmt.Errorf("scrub scheduling is disabled")
	}

	// 检查是否需要执行scrub
	if !ic.lastCheck.IsZero() {
		nextCheck := ic.lastCheck.AddDate(0, 0, ic.config.IntervalDays)
		if time.Now().Before(nextCheck) {
			return fmt.Errorf("next scrub scheduled for %s", nextCheck.Format(time.RFC3339))
		}
	}

	// 检查当前时间是否在优选窗口
	now := time.Now()
	if now.Hour() != ic.config.PreferredHour {
		return fmt.Errorf("current hour %d is not preferred hour %d", now.Hour(), ic.config.PreferredHour)
	}

	// 执行scrub
	_, err := ic.RunScrub(ctx, poolName)
	return err
}

// GetCheckHistory 获取检查历史
func (ic *IntegrityChecker) GetCheckHistory() []IntegrityCheckResult {
	result := make([]IntegrityCheckResult, len(ic.checkHistory))
	copy(result, ic.checkHistory)
	return result
}

// GetLastCheckTime 获取最后检查时间
func (ic *IntegrityChecker) GetLastCheckTime() time.Time {
	return ic.lastCheck
}

// UpdateConfig 更新配置
func (ic *IntegrityChecker) UpdateConfig(config ScrubScheduleConfig) {
	ic.config = config
}

// GetConfig 获取配置
func (ic *IntegrityChecker) GetConfig() ScrubScheduleConfig {
	return ic.config
}

// GetPoolIntegritySummary 获取池完整性摘要
func (ic *IntegrityChecker) GetPoolIntegritySummary(ctx context.Context, poolName string) (map[string]interface{}, error) {
	pool, err := ic.poolManager.GetPoolStatus(ctx, poolName)
	if err != nil {
		return nil, err
	}

	summary := map[string]interface{}{
		"pool_name":       poolName,
		"status":          string(pool.Status),
		"health":          pool.Health,
		"read_errors":     pool.ReadErrors,
		"write_errors":    pool.WriteErrors,
		"checksum_errors": pool.ChecksumErrors,
		"scan_status":     pool.ScanStatus,
		"scan_progress":   pool.ScanProgress,
		"total_disks":     len(pool.Disks),
		"spare_disks":     len(pool.Spares),
		"last_check":      ic.lastCheck,
		"check_count":     len(ic.checkHistory),
	}

	// 计算错误率
	totalErrors := pool.ReadErrors + pool.WriteErrors + pool.ChecksumErrors
	if pool.SizeBytes > 0 {
		summary["error_rate"] = float64(totalErrors) / float64(pool.SizeBytes) * 1000000 // 每TB错误数
	}

	return summary, nil
}

// --- 内部方法 ---

func (ic *IntegrityChecker) getScrubStatus(ctx context.Context, poolName string) string {
	cmd := exec.CommandContext(ctx, "zpool", "status", poolName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "unknown"
	}

	outputStr := string(output)
	if strings.Contains(outputStr, "scrub repaired") {
		return "completed"
	}
	if strings.Contains(outputStr, "scrub in progress") {
		return "running"
	}
	if strings.Contains(outputStr, "scan: none requested") {
		return "idle"
	}

	return "unknown"
}

func (ic *IntegrityChecker) parseScrubResult(ctx context.Context, poolName string, result *IntegrityCheckResult) {
	cmd := exec.CommandContext(ctx, "zpool", "status", poolName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return
	}

	outputStr := string(output)
	scanner := bufio.NewScanner(strings.NewReader(outputStr))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// 解析扫描结果
		if strings.Contains(line, "repaired") {
			re := regexp.MustCompile(`repaired\s+(\S+)`)
			if matches := re.FindStringSubmatch(line); len(matches) > 1 {
				result.RepairsMade = parseSize(matches[1])
			}
		}

		// 解析错误数
		if strings.Contains(line, "errors:") {
			re := regexp.MustCompile(`(\d+)\s+data errors`)
			if matches := re.FindStringSubmatch(line); len(matches) > 1 {
				val, _ := strconv.ParseInt(matches[1], 10, 64)
				result.ErrorsFound = val
			}
		}
	}

	if result.ErrorsFound > int64(ic.config.MaxErrorCount) {
		result.Status = "warning"
	}
}

func (ic *IntegrityChecker) parseChecksumErrors(output string) int64 {
	var totalErrors int64

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		fields := strings.Fields(line)

		// 查找包含错误数的行
		if len(fields) >= 5 {
			// 格式: name state read write cksum
			for _, f := range fields[2:5] {
				if val, err := strconv.ParseInt(f, 10, 64); err == nil {
					totalErrors += val
				}
			}
		}
	}

	return totalErrors
}

// VerifyDatasetIntegrity 验证数据集完整性
func (ic *IntegrityChecker) VerifyDatasetIntegrity(ctx context.Context, dataset string) (*IntegrityCheckResult, error) {
	startTime := time.Now()
	result := &IntegrityCheckResult{
		CheckType: "dataset_integrity",
		StartTime: startTime,
		Status:    "passed",
	}

	// 获取数据集属性
	cmd := exec.CommandContext(ctx, "zfs", "get", "-H", "-p", "used,referenced,compressratio,dedup", dataset)
	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Status = "failed"
		result.EndTime = time.Now()
		result.Duration = time.Since(startTime).String()
		result.Details = err.Error()
		return result, err
	}

	// 解析输出
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 3 {
			property := fields[1]
			value := fields[2]

			switch property {
			case "used":
				if val, err := strconv.ParseInt(value, 10, 64); err == nil {
					result.TotalChecked = val
				}
			case "compressratio":
				// 记录压缩比
				result.Details = fmt.Sprintf("compressratio: %s", value)
			}
		}
	}

	result.EndTime = time.Now()
	result.Duration = time.Since(startTime).String()
	ic.checkHistory = append(ic.checkHistory, *result)

	return result, nil
}

// GetErrorDistribution 获取错误分布
func (ic *IntegrityChecker) GetErrorDistribution(ctx context.Context, poolName string) (map[string]int64, error) {
	cmd := exec.CommandContext(ctx, "zpool", "status", poolName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	distribution := make(map[string]int64)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		fields := strings.Fields(line)

		if len(fields) >= 5 {
			// 尝试解析磁盘错误行
			name := fields[0]
			if strings.HasPrefix(name, "sd") || strings.HasPrefix(name, "nvme") || strings.HasPrefix(name, "vd") {
				readErr, _ := strconv.ParseInt(fields[2], 10, 64)
				writeErr, _ := strconv.ParseInt(fields[3], 10, 64)
				cksumErr, _ := strconv.ParseInt(fields[4], 10, 64)

				if readErr > 0 || writeErr > 0 || cksumErr > 0 {
					distribution[name+"_read"] = readErr
					distribution[name+"_write"] = writeErr
					distribution[name+"_checksum"] = cksumErr
				}
			}
		}
	}

	return distribution, nil
}
