// Package selfheal 预置检查项：磁盘SMART状态
package selfheal

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// SMARTChecker 磁盘 SMART 状态检查器.
type SMARTChecker struct {
	devices []string // 要检查的设备列表，为空时自动扫描
}

// NewSMARTChecker 创建 SMART 检查器.
// devices 为空时自动扫描所有磁盘.
func NewSMARTChecker(devices ...string) *SMARTChecker {
	return &SMARTChecker{devices: devices}
}

// Name 返回检查器名称.
func (c *SMARTChecker) Name() string { return "disk_smart" }

// Category 返回检查类别.
func (c *SMARTChecker) Category() CheckCategory { return CategoryDisk }

// Description 返回描述.
func (c *SMARTChecker) Description() string {
	return "检查磁盘SMART状态，预警硬盘故障"
}

// HealAction 返回默认自愈策略.
func (c *SMARTChecker) HealAction() HealAction { return HealActionNone }

// Check 执行 SMART 检查.
func (c *SMARTChecker) Check(ctx *CheckContext) *CheckResult {
	result := &CheckResult{
		Name:     c.Name(),
		Category: c.Category(),
		Details:  make(map[string]interface{}),
	}

	// 确定设备列表
	devices := c.devices
	if len(devices) == 0 {
		var err error
		devices, err = scanBlockDevices()
		if err != nil {
			result.Status = StatusUnhealthy
			result.Message = fmt.Sprintf("扫描块设备失败: %v", err)
			result.Timestamp = time.Now()
			return result
		}
	}

	if len(devices) == 0 {
		result.Status = StatusHealthy
		result.Message = "未发现磁盘设备"
		result.Timestamp = time.Now()
		return result
	}

	checkCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var unhealthy []string
	var degraded []string
	allDevices := make(map[string]interface{})

	for _, dev := range devices {
		devResult := c.checkDevice(checkCtx, dev)
		allDevices[dev] = devResult

		if status, ok := devResult["status"].(string); ok {
			switch Status(status) {
			case StatusUnhealthy:
				unhealthy = append(unhealthy, dev)
			case StatusDegraded:
				degraded = append(degraded, dev)
			}
		}
	}

	result.Details["devices"] = allDevices
	result.Details["total"] = len(devices)

	if len(unhealthy) > 0 {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("以下磁盘SMART异常: %s", strings.Join(unhealthy, ", "))
	} else if len(degraded) > 0 {
		result.Status = StatusDegraded
		result.Message = fmt.Sprintf("以下磁盘存在警告: %s", strings.Join(degraded, ", "))
	} else {
		result.Status = StatusHealthy
		result.Message = fmt.Sprintf("全部 %d 个磁盘 SMART 状态正常", len(devices))
	}

	result.Timestamp = time.Now()
	return result
}

// Heal 修复（SMART 问题无法自动修复，仅告警）.
func (c *SMARTChecker) Heal(ctx *CheckContext, result *CheckResult) *HealResult {
	return &HealResult{
		Success: false,
		Action:  "alert_only",
		Message: "SMART 异常无法自动修复，请检查磁盘并做好数据备份",
	}
}

// checkDevice 检查单个设备.
func (c *SMARTChecker) checkDevice(ctx context.Context, device string) map[string]interface{} {
	info := map[string]interface{}{
		"device": device,
		"status": string(StatusHealthy),
	}

	cmd := exec.CommandContext(ctx, "smartctl", "-H", device)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// smartctl 返回非零不一定是错误，检查输出
		outputStr := string(output)
		if strings.Contains(outputStr, "PASSED") || strings.Contains(outputStr, "OK") {
			info["health"] = "PASSED"
		} else if strings.Contains(outputStr, "FAILED") {
			info["health"] = "FAILED"
			info["status"] = string(StatusUnhealthy)
			info["message"] = "SMART 健康检查未通过"
		} else {
			info["health"] = "unknown"
			info["status"] = string(StatusDegraded)
			info["message"] = "无法获取 SMART 信息"
		}
		return info
	}

	outputStr := string(output)
	if strings.Contains(outputStr, "PASSED") || strings.Contains(outputStr, "OK") {
		info["health"] = "PASSED"
	} else {
		info["health"] = "UNKNOWN"
		info["status"] = string(StatusDegraded)
	}

	return info
}

// scanBlockDevices 扫描块设备.
func scanBlockDevices() ([]string, error) {
	cmd := exec.Command("lsblk", "-d", "-n", "-o", "NAME,TYPE")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var devices []string
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 2 && fields[1] == "disk" {
			devices = append(devices, "/dev/"+fields[0])
		}
	}
	return devices, nil
}
