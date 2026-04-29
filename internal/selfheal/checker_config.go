// Package selfheal 预置检查项：配置完整性
package selfheal

import (
	"fmt"
	"os"
	"time"
)

// ConfigChecker 配置完整性检查器.
type ConfigChecker struct {
	configPaths []string // 配置文件路径列表
}

// NewConfigChecker 创建配置完整性检查器.
func NewConfigChecker(configPaths ...string) *ConfigChecker {
	if len(configPaths) == 0 {
		configPaths = []string{
			"/etc/nas-os/config.yaml",
			"/etc/nas-os/config.json",
		}
	}
	return &ConfigChecker{configPaths: configPaths}
}

// Name 返回检查器名称.
func (c *ConfigChecker) Name() string { return "config_integrity" }

// Category 返回检查类别.
func (c *ConfigChecker) Category() CheckCategory { return CategoryConfig }

// Description 返回描述.
func (c *ConfigChecker) Description() string {
	return "检查关键配置文件完整性和可访问性"
}

// HealAction 返回默认自愈策略.
func (c *ConfigChecker) HealAction() HealAction { return HealActionNone }

// Check 执行配置完整性检查.
func (c *ConfigChecker) Check(ctx *CheckContext) *CheckResult {
	result := &CheckResult{
		Name:     c.Name(),
		Category: c.Category(),
		Details:  make(map[string]interface{}),
	}

	var issues []string
	fileResults := make(map[string]interface{})

	for _, path := range c.configPaths {
		fileResult := c.checkFile(path)
		fileResults[path] = fileResult

		if status, ok := fileResult["status"].(string); ok && status != string(StatusHealthy) {
			issues = append(issues, fmt.Sprintf("%s: %s", path, fileResult["message"]))
		}
	}

	result.Details["files"] = fileResults

	if len(issues) > 0 {
		result.Status = StatusDegraded
		result.Message = fmt.Sprintf("配置文件问题: %s", fmt.Sprint(issues))
	} else {
		result.Status = StatusHealthy
		result.Message = fmt.Sprintf("全部 %d 个配置文件正常", len(c.configPaths))
	}

	result.Timestamp = time.Now()
	return result
}

// Heal 修复（配置问题需人工处理）.
func (c *ConfigChecker) Heal(ctx *CheckContext, result *CheckResult) *HealResult {
	return &HealResult{
		Success:       false,
		Action:        "manual_fix",
		Message:       "配置文件问题需要人工检查和修复",
		NeedsApproval: true,
	}
}

// checkFile 检查单个配置文件.
func (c *ConfigChecker) checkFile(path string) map[string]interface{} {
	info := map[string]interface{}{
		"path":   path,
		"status": string(StatusHealthy),
	}

	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			info["status"] = string(StatusDegraded)
			info["message"] = "配置文件不存在"
			info["exists"] = false
		} else {
			info["status"] = string(StatusUnhealthy)
			info["message"] = fmt.Sprintf("无法访问配置文件: %v", err)
		}
		return info
	}

	info["exists"] = true
	info["size"] = stat.Size()
	info["mode"] = stat.Mode().String()
	info["modified"] = stat.ModTime().Format(time.RFC3339)

	// 检查文件是否为空
	if stat.Size() == 0 {
		info["status"] = string(StatusUnhealthy)
		info["message"] = "配置文件为空"
		return info
	}

	// 检查权限（不应过于宽松）
	if stat.Mode().Perm()&0002 != 0 {
		info["status"] = string(StatusDegraded)
		info["message"] = "配置文件权限过于宽松（other 可写）"
		info["warning"] = "world_writable"
		return info
	}

	info["message"] = "配置文件正常"
	return info
}
