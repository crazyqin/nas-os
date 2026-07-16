// Package selfheal 预置检查项：文件系统一致性
package selfheal

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// FilesystemChecker 文件系统一致性检查器.
type FilesystemChecker struct {
	mountPoints []string // 要检查的挂载点，为空时检查 /
}

// NewFilesystemChecker 创建文件系统检查器.
func NewFilesystemChecker(mountPoints ...string) *FilesystemChecker {
	if len(mountPoints) == 0 {
		mountPoints = []string{"/"}
	}
	return &FilesystemChecker{mountPoints: mountPoints}
}

// Name 返回检查器名称.
func (c *FilesystemChecker) Name() string { return "filesystem_consistency" }

// Category 返回检查类别.
func (c *FilesystemChecker) Category() CheckCategory { return CategoryFilesystem }

// Description 返回描述.
func (c *FilesystemChecker) Description() string {
	return "检查文件系统一致性，检测磁盘空间和只读挂载"
}

// HealAction 返回默认自愈策略.
func (c *FilesystemChecker) HealAction() HealAction { return HealActionNone }

// Check 执行文件系统检查.
func (c *FilesystemChecker) Check(ctx *CheckContext) *CheckResult {
	result := &CheckResult{
		Name:     c.Name(),
		Category: c.Category(),
		Details:  make(map[string]interface{}),
	}

	checkCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var issues []string
	mountResults := make(map[string]interface{})

	for _, mp := range c.mountPoints {
		mpResult := c.checkMountPoint(checkCtx, mp)
		mountResults[mp] = mpResult

		if status, ok := mpResult["status"].(string); ok && status != string(StatusHealthy) {
			if msg, ok := mpResult["message"].(string); ok {
				issues = append(issues, fmt.Sprintf("%s: %s", mp, msg))
			}
		}
	}

	result.Details["mount_points"] = mountResults

	if len(issues) > 0 {
		result.Status = StatusDegraded
		result.Message = fmt.Sprintf("文件系统问题: %s", strings.Join(issues, "; "))
	} else {
		result.Status = StatusHealthy
		result.Message = fmt.Sprintf("全部 %d 个挂载点正常", len(c.mountPoints))
	}

	result.Timestamp = time.Now()
	return result
}

// Heal 修复（文件系统问题需人工处理）.
func (c *FilesystemChecker) Heal(ctx *CheckContext, result *CheckResult) *HealResult {
	return &HealResult{
		Success:       false,
		Action:        "manual_repair",
		Message:       "文件系统一致性问题需要人工检查，请运行 fsck 修复",
		NeedsApproval: true,
	}
}

// checkMountPoint 检查单个挂载点.
func (c *FilesystemChecker) checkMountPoint(ctx context.Context, mountPoint string) map[string]interface{} {
	info := map[string]interface{}{
		"mount_point": mountPoint,
		"status":      string(StatusHealthy),
	}

	// 使用 df 检查磁盘使用率
	cmd := exec.CommandContext(ctx, "df", "-h", mountPoint)
	output, err := cmd.Output()
	if err != nil {
		info["status"] = string(StatusUnhealthy)
		info["message"] = fmt.Sprintf("无法获取挂载点信息: %v", err)
		return info
	}

	// 解析 df 输出
	lines := strings.Split(string(output), "\n")
	if len(lines) >= 2 {
		fields := strings.Fields(lines[1])
		if len(fields) >= 5 {
			info["filesystem"] = fields[0]
			info["size"] = fields[1]
			info["used"] = fields[2]
			info["available"] = fields[3]
			info["use_percent"] = fields[4]

			// 解析使用率
			var usedPercent int
			_, _ = fmt.Sscanf(fields[4], "%d%%", &usedPercent)

			if usedPercent >= 95 {
				info["status"] = string(StatusUnhealthy)
				info["message"] = fmt.Sprintf("磁盘使用率 %d%% 已达危险水平", usedPercent)
			} else if usedPercent >= 85 {
				info["status"] = string(StatusDegraded)
				info["message"] = fmt.Sprintf("磁盘使用率 %d%% 偏高", usedPercent)
			} else {
				info["message"] = fmt.Sprintf("磁盘使用率 %d%%，正常", usedPercent)
			}
		}
	}

	// 检查是否为只读挂载
	cmdRO := exec.CommandContext(ctx, "grep", mountPoint, "/proc/mounts")
	outputRO, err := cmdRO.Output()
	if err == nil && strings.Contains(string(outputRO), "ro,") {
		info["status"] = string(StatusDegraded)
		info["read_only"] = true
		info["message"] = "挂载点为只读模式"
	}

	return info
}
