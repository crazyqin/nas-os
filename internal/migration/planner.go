package migration

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// Planner 迁移规划器.
// 负责源系统检测、数据映射生成、迁移计划创建.
type Planner struct {
	// detectorFn 源系统探测函数（可替换为 mock）
	detectorFn SourceDetector
	// mapperFn 数据映射生成函数
	mapperFn MapperFunc
}

// SourceDetector 源系统探测器接口.
type SourceDetector interface {
	Detect(ctx context.Context, host string, port int, user, pass string) (*SourceSystemInfo, error)
}

// MapperFunc 数据映射函数签名.
type MapperFunc func(ctx context.Context, info *SourceSystemInfo) ([]DataMapping, error)

// defaultDetector 默认源系统探测器.
type defaultDetector struct{}

func (d *defaultDetector) Detect(ctx context.Context, host string, port int, user, pass string) (*SourceSystemInfo, error) {
	// 默认实现：模拟探测
	return &SourceSystemInfo{
		Type:         SourceGenericNAS,
		Version:      "1.0",
		Hostname:     host,
		TotalStorage: 1024 * 1024 * 1024 * 100, // 100GB
		UsedStorage:  1024 * 1024 * 1024 * 50,   // 50GB
		TotalUsers:   5,
		TotalShares:  10,
		TotalApps:    3,
		IPAddresses:  []string{host},
	}, nil
}

// NewPlanner 创建迁移规划器.
func NewPlanner() *Planner {
	p := &Planner{
		detectorFn: &defaultDetector{},
	}
	p.mapperFn = p.defaultMapping
	return p
}

// SetDetector 设置源系统探测器.
func (p *Planner) SetDetector(d SourceDetector) {
	p.detectorFn = d
}

// SetMapper 设置数据映射函数.
func (p *Planner) SetMapper(fn MapperFunc) {
	p.mapperFn = fn
}

// DetectSource 探测源系统信息.
func (p *Planner) DetectSource(ctx context.Context, req *CreateMigrationRequest) (*SourceSystemInfo, error) {
	if req.SourceHost == "" {
		return nil, fmt.Errorf("源主机地址不能为空")
	}

	port := req.SourcePort
	if port == 0 {
		port = 22 // 默认 SSH 端口
	}

	slog.Info("开始探测源系统",
		"host", req.SourceHost,
		"type", req.SourceType,
	)

	info, err := p.detectorFn.Detect(ctx, req.SourceHost, port, req.SourceUser, req.SourcePass)
	if err != nil {
		return nil, fmt.Errorf("源系统探测失败: %w", err)
	}

	// 如果用户未指定源类型，使用探测结果
	if req.SourceType == "" {
		req.SourceType = info.Type
	}

	slog.Info("源系统探测完成",
		"type", info.Type,
		"version", info.Version,
		"hostname", info.Hostname,
		"users", info.TotalUsers,
		"shares", info.TotalShares,
	)

	return info, nil
}

// GeneratePlan 生成迁移计划.
func (p *Planner) GeneratePlan(ctx context.Context, task *MigrationTask, sourceInfo *SourceSystemInfo) (*MigrationPlan, error) {
	slog.Info("开始生成迁移计划", "taskId", task.ID)

	// 生成数据映射
	mappings, err := p.mapperFn(ctx, sourceInfo)
	if err != nil {
		return nil, fmt.Errorf("生成数据映射失败: %w", err)
	}

	// 评估兼容性和警告
	warnings, compatible, notes := p.evaluateCompatibility(sourceInfo, mappings)

	// 计算总量
	var totalSize int64
	var totalItems int
	for _, m := range mappings {
		if m.Selected {
			totalSize += m.TotalSize
			totalItems += m.ItemCount
		}
	}

	// 估算迁移时间（假设 100MB/s 的传输速度）
	estimatedTime := time.Duration(totalSize/(100*1024*1024)) * time.Second
	if estimatedTime < time.Minute {
		estimatedTime = time.Minute
	}

	plan := &MigrationPlan{
		ID:                 uuid.New().String(),
		TaskID:             task.ID,
		SourceType:         sourceInfo.Type,
		SourceVersion:      sourceInfo.Version,
		SourceHost:         sourceInfo.Hostname,
		TotalDataSize:      totalSize,
		TotalItems:         totalItems,
		Mappings:           mappings,
		Warnings:           warnings,
		EstimatedTime:      estimatedTime,
		Compatible:         compatible,
		CompatibilityNotes: notes,
		CreatedAt:          time.Now(),
	}

	slog.Info("迁移计划生成完成",
		"planId", plan.ID,
		"mappings", len(mappings),
		"totalSize", totalSize,
		"compatible", compatible,
	)

	return plan, nil
}

// defaultMapping 默认数据映射生成.
func (p *Planner) defaultMapping(ctx context.Context, info *SourceSystemInfo) ([]DataMapping, error) {
	mappings := make([]DataMapping, 0)

	// 系统配置
	mappings = append(mappings, DataMapping{
		ID:         uuid.New().String(),
		Category:   CategorySystem,
		SourcePath: "/etc",
		TargetPath: "/etc/nas-os",
		ItemCount:  50,
		TotalSize:  1024 * 1024, // 1MB
		Selected:   true,
		Order:      1,
	})

	// 用户数据
	if info.TotalUsers > 0 {
		mappings = append(mappings, DataMapping{
			ID:         uuid.New().String(),
			Category:   CategoryUsers,
			SourcePath: "/var/users",
			TargetPath: "/home",
			ItemCount:  info.TotalUsers,
			TotalSize:  int64(info.TotalUsers) * 1024 * 1024 * 100, // 100MB/用户
			Selected:   true,
			Order:      2,
		})
	}

	// 共享文件夹
	if info.TotalShares > 0 {
		mappings = append(mappings, DataMapping{
			ID:         uuid.New().String(),
			Category:   CategoryShares,
			SourcePath: "/volume1",
			TargetPath: "/data/shares",
			ItemCount:  info.TotalShares,
			TotalSize:  info.UsedStorage / 2, // 假设一半是共享数据
			Selected:   true,
			Order:      3,
		})
	}

	// 应用数据
	if info.TotalApps > 0 {
		mappings = append(mappings, DataMapping{
			ID:          uuid.New().String(),
			Category:    CategoryApps,
			SourcePath:  "/var/packages",
			TargetPath:  "/opt/apps",
			ItemCount:   info.TotalApps,
			TotalSize:   int64(info.TotalApps) * 1024 * 1024 * 50, // 50MB/应用
			Selected:    true,
			Convertible: true,
			ConvertNote: "部分应用配置可能需要手动调整",
			Order:       4,
		})
	}

	// Docker 容器
	mappings = append(mappings, DataMapping{
		ID:          uuid.New().String(),
		Category:    CategoryDocker,
		SourcePath:  "/var/docker",
		TargetPath:  "/var/lib/docker",
		ItemCount:   5,
		TotalSize:   1024 * 1024 * 1024, // 1GB
		Selected:    true,
		Convertible: true,
		ConvertNote: "Docker Compose 文件将自动转换",
		Order:       5,
	})

	// 证书
	mappings = append(mappings, DataMapping{
		ID:         uuid.New().String(),
		Category:   CategoryCerts,
		SourcePath: "/etc/ssl",
		TargetPath: "/etc/ssl/certs",
		ItemCount:  3,
		TotalSize:  1024 * 50, // 50KB
		Selected:   true,
		Order:      6,
	})

	// 定时任务
	mappings = append(mappings, DataMapping{
		ID:         uuid.New().String(),
		Category:   CategoryScheduled,
		SourcePath: "/etc/cron.d",
		TargetPath: "/etc/cron.d",
		ItemCount:  10,
		TotalSize:  1024 * 10, // 10KB
		Selected:   true,
		Order:      7,
	})

	return mappings, nil
}

// evaluateCompatibility 评估兼容性.
func (p *Planner) evaluateCompatibility(info *SourceSystemInfo, mappings []DataMapping) (warnings []PlanWarning, compatible bool, notes []string) {
	compatible = true
	warnings = make([]PlanWarning, 0)
	notes = make([]string, 0)

	// 检查源系统类型
	switch info.Type {
	case SourceSynology:
		notes = append(notes, "群晖 DSM 系统数据将被转换为 NAS-OS 格式")
	case SourceQNAP:
		notes = append(notes, "QNAP QTS 应用可能需要手动重新安装")
	case SourceTrueNAS:
		notes = append(notes, "ZFS 池配置将保留")
	case SourceWindows:
		warnings = append(warnings, PlanWarning{
			Level:    "warning",
			Category: "system",
			Message:  "Windows 权限模型与 Linux 不同，可能需要手动调整 ACL",
		})
	}

	// 检查数据映射中的可转换项
	for _, m := range mappings {
		if m.Convertible && m.Selected {
			warnings = append(warnings, PlanWarning{
				Level:    "info",
				Category: string(m.Category),
				Message:  m.ConvertNote,
			})
		}
	}

	// 检查存储空间
	var totalSelected int64
	for _, m := range mappings {
		if m.Selected {
			totalSelected += m.TotalSize
		}
	}

	if totalSelected > info.TotalStorage-info.UsedStorage {
		warnings = append(warnings, PlanWarning{
			Level:    "error",
			Category: "storage",
			Message:  "目标存储空间可能不足",
		})
		compatible = false
	}

	return warnings, compatible, notes
}

// UpdateMappingSelection 更新映射选择状态.
func (p *Planner) UpdateMappingSelection(plan *MigrationPlan, category DataCategory, selected bool) bool {
	for i := range plan.Mappings {
		if plan.Mappings[i].Category == category {
			plan.Mappings[i].Selected = selected
			return true
		}
	}
	return false
}

// RecalculatePlan 重新计算迁移计划（映射变更后）.
func (p *Planner) RecalculatePlan(plan *MigrationPlan) {
	var totalSize int64
	var totalItems int
	for _, m := range plan.Mappings {
		if m.Selected {
			totalSize += m.TotalSize
			totalItems += m.ItemCount
		}
	}

	plan.TotalDataSize = totalSize
	plan.TotalItems = totalItems
	plan.EstimatedTime = time.Duration(totalSize/(100*1024*1024)) * time.Second
	if plan.EstimatedTime < time.Minute {
		plan.EstimatedTime = time.Minute
	}

	// 重新评估兼容性
	warnings, compatible, notes := p.evaluateCompatibility(&SourceSystemInfo{
		Type: plan.SourceType,
	}, plan.Mappings)
	plan.Warnings = warnings
	plan.Compatible = compatible
	plan.CompatibilityNotes = notes
}
