// Package selfheal 预置检查项：服务存活检查
package selfheal

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ServiceChecker 服务存活检查器.
type ServiceChecker struct {
	services []string // 要检查的服务名称列表
}

// NewServiceChecker 创建服务检查器.
// services 为空时检查关键 NAS 服务.
func NewServiceChecker(services ...string) *ServiceChecker {
	if len(services) == 0 {
		services = []string{"smbd", "nfs-server", "sshd", "nginx", "cron"}
	}
	return &ServiceChecker{services: services}
}

// Name 返回检查器名称.
func (c *ServiceChecker) Name() string { return "service_liveness" }

// Category 返回检查类别.
func (c *ServiceChecker) Category() CheckCategory { return CategoryService }

// Description 返回描述.
func (c *ServiceChecker) Description() string {
	return "检查关键服务是否存活（systemd）"
}

// HealAction 返回默认自愈策略.
func (c *ServiceChecker) HealAction() HealAction { return HealActionAuto }

// Check 执行服务存活检查.
func (c *ServiceChecker) Check(ctx *CheckContext) *CheckResult {
	result := &CheckResult{
		Name:     c.Name(),
		Category: c.Category(),
		Details:  make(map[string]interface{}),
	}

	checkCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var failed []string
	var notFound []string
	svcResults := make(map[string]interface{})

	for _, svc := range c.services {
		svcResult := c.checkService(checkCtx, svc)
		svcResults[svc] = svcResult

		if status, ok := svcResult["status"].(string); ok {
			switch status {
			case "not_found":
				notFound = append(notFound, svc)
			case "inactive":
				failed = append(failed, svc)
			}
		}
	}

	result.Details["services"] = svcResults

	if len(failed) > 0 {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("以下服务未运行: %s", strings.Join(failed, ", "))
	} else if len(notFound) > 0 {
		result.Status = StatusDegraded
		result.Message = fmt.Sprintf("以下服务未安装: %s", strings.Join(notFound, ", "))
	} else {
		result.Status = StatusHealthy
		result.Message = fmt.Sprintf("全部 %d 个服务运行正常", len(c.services))
	}

	result.Timestamp = time.Now()
	return result
}

// Heal 尝试重启失败的服务.
func (c *ServiceChecker) Heal(ctx *CheckContext, result *CheckResult) *HealResult {
	// 从 result.Details 中获取失败的服务
	services, ok := result.Details["services"].(map[string]interface{})
	if !ok {
		return &HealResult{
			Success: false,
			Action:  "restart_failed",
			Message: "无法获取服务状态详情",
		}
	}

	var restarted []string
	var failed []string

	for svc, svcInfo := range services {
		info, ok := svcInfo.(map[string]interface{})
		if !ok {
			continue
		}
		status, _ := info["status"].(string)
		if status != "inactive" {
			continue
		}

		// 尝试重启
		healCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		cmd := exec.CommandContext(healCtx, "systemctl", "restart", svc)
		err := cmd.Run()
		cancel()

		if err != nil {
			failed = append(failed, svc)
		} else {
			restarted = append(restarted, svc)
		}
	}

	msg := ""
	if len(restarted) > 0 {
		msg += fmt.Sprintf("已重启: %s. ", strings.Join(restarted, ", "))
	}
	if len(failed) > 0 {
		msg += fmt.Sprintf("重启失败: %s", strings.Join(failed, ", "))
	}

	return &HealResult{
		Success: len(failed) == 0,
		Action:  "restart_service",
		Message: msg,
	}
}

// checkService 检查单个服务.
func (c *ServiceChecker) checkService(ctx context.Context, service string) map[string]interface{} {
	info := map[string]interface{}{
		"service": service,
		"status":  "active",
	}

	cmd := exec.CommandContext(ctx, "systemctl", "is-active", service)
	output, err := cmd.Output()
	status := strings.TrimSpace(string(output))

	switch status {
	case "active":
		info["status"] = "active"
	case "inactive", "failed":
		info["status"] = "inactive"
		info["message"] = fmt.Sprintf("服务 %s 状态: %s", service, status)
	default:
		if err != nil {
			info["status"] = "not_found"
			info["message"] = fmt.Sprintf("服务 %s 未安装或无法访问", service)
		} else {
			info["status"] = "inactive"
			info["message"] = fmt.Sprintf("服务 %s 状态: %s", service, status)
		}
	}

	return info
}
