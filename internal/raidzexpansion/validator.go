// Package raidzexpansion 磁盘验证器
package raidzexpansion

import (
	"fmt"
	"strings"
)

// Validator 磁盘验证器
type Validator struct {
	// knownDevicePrefixes 已知设备前缀
	knownDevicePrefixes []string
}

// NewValidator 创建验证器
func NewValidator() *Validator {
	return &Validator{
		knownDevicePrefixes: []string{"/dev/sd", "/dev/nvme", "/dev/hd", "/dev/vd"},
	}
}

// ValidateExpansionRequest 验证扩展请求
func (v *Validator) ValidateExpansionRequest(req *ExpansionRequest, pool *PoolInfo) *ValidationResult {
	result := &ValidationResult{
		Valid:             true,
		Errors:            []ValidationError{},
		Warnings:          []ValidationWarning{},
		DiskCompatibility: []*DiskCompatibilityResult{},
	}

	// 验证池名称
	if req.PoolName == "" {
		result.addError("EMPTY_POOL_NAME", "存储池名称不能为空", "")
	}

	// 验证磁盘列表
	if len(req.NewDisks) == 0 {
		result.addError("NO_DISKS", "至少需要一块新磁盘", "")
	}

	// 验证设备路径格式
	for _, disk := range req.NewDisks {
		if !v.isValidDevicePath(disk) {
			result.addError("INVALID_DEVICE_PATH",
				fmt.Sprintf("无效的设备路径: %s", disk), disk)
		}
	}

	// 验证无重复磁盘
	if err := v.validateNoDuplicates(req.NewDisks); err != "" {
		result.addError("DUPLICATE_DISKS", err, "")
	}

	// 如果池存在，验证兼容性
	if pool != nil {
		v.validateDiskCompatibility(req.NewDisks, pool, result)
		v.validateExpansionRules(pool, result)
	}

	// 设置总体验证结果
	result.Valid = len(result.Errors) == 0
	return result
}

// ValidateDiskHealth 验证磁盘健康状态
func (v *Validator) ValidateDiskHealth(disk *DiskInfo) *ValidationResult {
	result := &ValidationResult{
		Valid:  true,
		Errors: []ValidationError{},
	}

	if !disk.Healthy {
		result.addError("DISK_UNHEALTHY",
			fmt.Sprintf("磁盘 %s 不健康", disk.Device), disk.Device)
	}

	if disk.SizeBytes == 0 {
		result.addError("INVALID_DISK_SIZE",
			fmt.Sprintf("磁盘 %s 容量无效", disk.Device), disk.Device)
	}

	if disk.Temperature > 60 {
		result.addWarning("HIGH_TEMPERATURE",
			fmt.Sprintf("磁盘 %s 温度过高: %d°C", disk.Device, disk.Temperature))
	}

	if disk.Temperature > 70 {
		result.addError("CRITICAL_TEMPERATURE",
			fmt.Sprintf("磁盘 %s 温度危险: %d°C，禁止操作", disk.Device, disk.Temperature),
			disk.Device)
	}

	result.Valid = len(result.Errors) == 0
	return result
}

// GetMinDiskSize 获取最小磁盘容量要求
func (v *Validator) GetMinDiskSize(pool *PoolInfo) uint64 {
	if pool == nil || len(pool.Disks) == 0 {
		return 0
	}

	// 新磁盘不能小于池中最小磁盘
	minSize := pool.Disks[0].SizeBytes
	for _, d := range pool.Disks {
		if d.SizeBytes < minSize {
			minSize = d.SizeBytes
		}
	}
	return minSize
}

// validateDevicePath 验证设备路径格式
func (v *Validator) isValidDevicePath(path string) bool {
	if path == "" {
		return false
	}
	for _, prefix := range v.knownDevicePrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// validateNoDuplicates 验证无重复磁盘
func (v *Validator) validateNoDuplicates(disks []string) string {
	seen := make(map[string]bool)
	for _, d := range disks {
		if seen[d] {
			return fmt.Sprintf("存在重复磁盘: %s", d)
		}
		seen[d] = true
	}
	return ""
}

// validateDiskCompatibility 验证磁盘兼容性
func (v *Validator) validateDiskCompatibility(newDisks []string, pool *PoolInfo, result *ValidationResult) {
	minSize := v.GetMinDiskSize(pool)

	for _, device := range newDisks {
		compat := &DiskCompatibilityResult{
			Device:          device,
			Compatible:      true,
			MinRequiredSize: minSize,
		}

		// 检查磁盘是否已在池中
		for _, existing := range pool.Disks {
			if existing.Device == device {
				compat.Compatible = false
				compat.Reason = "磁盘已在存储池中"
				result.addError("DISK_ALREADY_IN_POOL",
					fmt.Sprintf("磁盘 %s 已在存储池中", device), device)
				break
			}
		}

		result.DiskCompatibility = append(result.DiskCompatibility, compat)
	}
}

// validateExpansionRules 验证扩展规则
func (v *Validator) validateExpansionRules(pool *PoolInfo, result *ValidationResult) {
	// 检查池是否正在扩展
	if pool.IsExpanding {
		result.addError("POOL_EXPANDING",
			"存储池正在扩展中，请等待完成后再试", "")
	}

	// 检查池健康状态
	if pool.Health != "ONLINE" {
		result.addWarning("POOL_NOT_HEALTHY",
			fmt.Sprintf("存储池状态为 %s，建议先修复再扩展", pool.Health))
	}

	// 检查 RAID-Z 等级限制
	if pool.RaidzLevel < Raidz1 || pool.RaidzLevel > Raidz3 {
		result.addError("INVALID_RAIDZ_LEVEL",
			fmt.Sprintf("不支持的 RAID-Z 等级: %d", pool.RaidzLevel), "")
	}

	// RAID-Z1 最多支持 255 块数据盘
	maxDisks := 255 - int(pool.RaidzLevel)
	if pool.DiskCount >= maxDisks {
		result.addError("MAX_DISKS_REACHED",
			fmt.Sprintf("RAID-Z%d 最多支持 %d 块磁盘", pool.RaidzLevel, maxDisks), "")
	}
}

// addError 添加错误
func (r *ValidationResult) addError(code, message, disk string) {
	r.Errors = append(r.Errors, ValidationError{
		Code:    code,
		Message: message,
		Disk:    disk,
	})
	r.Valid = false
}

// addWarning 添加警告
func (r *ValidationResult) addWarning(code, message string) {
	r.Warnings = append(r.Warnings, ValidationWarning{
		Code:    code,
		Message: message,
	})
}
