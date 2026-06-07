// Package raidplanner 提供RAID容量规划与计算器
// RAID级别对比、容量计算、性能预测、故障模拟、成本估算
// 对标群晖RAID计算器 + TrueNAS存储规划
package raidplanner

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// ========== 常量 ==========

const (
	Version   = "1.0.0"
	MaxDrives = 24
	MinDrives = 1
	MaxPlans  = 100
	MaxVDisks = 50
	TB        = 1024 * 1024 * 1024 * 1024
	GB        = 1024 * 1024 * 1024
	MB        = 1024 * 1024
)

// ========== 类型定义 ==========

// RAIDLevel RAID级别
type RAIDLevel string

const (
	RAID0  RAIDLevel = "raid0"
	RAID1  RAIDLevel = "raid1"
	RAID5  RAIDLevel = "raid5"
	RAID6  RAIDLevel = "raid6"
	RAID10 RAIDLevel = "raid10"
	RAID50 RAIDLevel = "raid50"
	RAID60 RAIDLevel = "raid60"
	RAIDZ1 RAIDLevel = "raidz1"
	RAIDZ2 RAIDLevel = "raidz2"
	RAIDZ3 RAIDLevel = "raidz3"
	JBOD   RAIDLevel = "jbod"
	Single RAIDLevel = "single"
)

// DiskType 磁盘类型
type DiskType string

const (
	DiskHDD  DiskType = "hdd"
	DiskSSD  DiskType = "ssd"
	DiskNVMe DiskType = "nvme"
	DiskSATA DiskType = "sata"
)

// Drive 磁盘信息
type Drive struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Capacity   int64    `json:"capacity"` // bytes
	Type       DiskType `json:"type"`
	RPM        int      `json:"rpm"`         // HDD only
	ReadSpeed  int64    `json:"read_speed"`  // MB/s
	WriteSpeed int64    `json:"write_speed"` // MB/s
	Price      float64  `json:"price"`       // 元
	MTBF       int      `json:"mtbf"`        // 小时
	Interface  string   `json:"interface"`   // SATA, SAS, NVMe
}

// RAIDConfig RAID配置
type RAIDConfig struct {
	Level      RAIDLevel `json:"level"`
	Drives     []*Drive  `json:"drives"`
	StripeSize int       `json:"stripe_size"` // KB
	Name       string    `json:"name"`
}

// RAIDPlan RAID方案
type RAIDPlan struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Config     RAIDConfig `json:"config"`
	TotalRaw   int64      `json:"total_raw"`   // 原始容量
	Usable     int64      `json:"usable"`      // 可用容量
	Redundancy int        `json:"redundancy"`  // 容错盘数
	ReadSpeed  int64      `json:"read_speed"`  // MB/s
	WriteSpeed int64      `json:"write_speed"` // MB/s
	IOPS       int        `json:"iops"`
	FaultRisk  float64    `json:"fault_risk"` // 年故障概率
	TotalPrice float64    `json:"total_price"`
	Efficiency float64    `json:"efficiency"` // 可用/原始
	Pros       []string   `json:"pros"`
	Cons       []string   `json:"cons"`
	CreatedAt  time.Time  `json:"created_at"`
}

// VDisk 虚拟磁盘
type VDisk struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Capacity  int64     `json:"capacity"`
	RAIDLevel RAIDLevel `json:"raid_level"`
	Drives    int       `json:"drives"`
	UsedSpace int64     `json:"used_space"`
	CreatedAt time.Time `json:"created_at"`
}

// FaultSimulation 故障模拟
type FaultSimulation struct {
	FailedDrives int     `json:"failed_drives"`
	DataLoss     bool    `json:"data_loss"`
	RebuildTime  string  `json:"rebuild_time"`
	DegradedPerf float64 `json:"degraded_perf"` // 性能百分比
	RiskLevel    string  `json:"risk_level"`
}

// CapacityReport 容量报告
type CapacityReport struct {
	TotalCapacity int64   `json:"total_capacity"`
	UsedCapacity  int64   `json:"used_capacity"`
	FreeCapacity  int64   `json:"free_capacity"`
	UsagePercent  float64 `json:"usage_percent"`
	DaysRemaining int     `json:"days_remaining"`
	GrowthRate    float64 `json:"growth_rate"` // GB/天
}

// PlannerStats 规划器统计
type PlannerStats struct {
	TotalPlans  int `json:"total_plans"`
	TotalVDisk  int `json:"total_vdisks"`
	TotalDrives int `json:"total_drives"`
}

// ========== 核心规划器 ==========

// Planner RAID规划器
type Planner struct {
	mu          sync.RWMutex
	plans       map[string]*RAIDPlan
	vdisks      map[string]*VDisk
	drives      []*Drive
	ctx         interface{}
	planCounter int64
	vdCounter   int64
}

// NewPlanner 创建规划器
func NewPlanner() *Planner {
	return &Planner{
		plans:  make(map[string]*RAIDPlan),
		vdisks: make(map[string]*VDisk),
		drives: make([]*Drive, 0),
	}
}

// ========== 磁盘管理 ==========

// AddDrive 添加磁盘
func (p *Planner) AddDrive(name string, capacity int64, dtype DiskType, price float64) *Drive {
	p.mu.Lock()
	defer p.mu.Unlock()

	drive := &Drive{
		ID:       fmt.Sprintf("drive_%d", len(p.drives)+1),
		Name:     name,
		Capacity: capacity,
		Type:     dtype,
		Price:    price,
		MTBF:     1000000,
	}

	switch dtype {
	case DiskNVMe:
		drive.ReadSpeed = 3500
		drive.WriteSpeed = 3000
		drive.Interface = "NVMe"
	case DiskSSD:
		drive.ReadSpeed = 550
		drive.WriteSpeed = 520
		drive.Interface = "SATA"
	case DiskHDD:
		drive.ReadSpeed = 200
		drive.WriteSpeed = 180
		drive.RPM = 7200
		drive.Interface = "SATA"
	}

	p.drives = append(p.drives, drive)
	return drive
}

// ListDrives 列出磁盘
func (p *Planner) ListDrives() []*Drive {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.drives
}

// ========== RAID计算 ==========

// CalculateRAID 计算RAID方案
func (p *Planner) CalculateRAID(config RAIDConfig) (*RAIDPlan, error) {
	if len(config.Drives) < 1 {
		return nil, errors.New("至少需要1块磁盘")
	}
	if len(config.Drives) > MaxDrives {
		return nil, fmt.Errorf("磁盘数量不能超过%d", MaxDrives)
	}

	plan := &RAIDPlan{
		Config:    config,
		CreatedAt: time.Now(),
	}

	// 找最小容量
	minCap := config.Drives[0].Capacity
	totalPrice := 0.0
	for _, d := range config.Drives {
		if d.Capacity < minCap {
			minCap = d.Capacity
		}
		totalPrice += d.Price
	}
	plan.TotalPrice = totalPrice

	n := len(config.Drives)
	plan.TotalRaw = int64(n) * minCap

	switch config.Level {
	case RAID0:
		plan.Usable = plan.TotalRaw
		plan.Redundancy = 0
		plan.ReadSpeed = int64(n) * config.Drives[0].ReadSpeed
		plan.WriteSpeed = int64(n) * config.Drives[0].WriteSpeed
		plan.IOPS = n * 1000
		plan.Pros = []string{"最高性能", "全部容量可用"}
		plan.Cons = []string{"无冗余", "单盘故障全丢"}
		plan.FaultRisk = 1 - math.Pow(0.99, float64(n))

	case RAID1:
		plan.Usable = minCap
		plan.Redundancy = n - 1
		plan.ReadSpeed = int64(n) * config.Drives[0].ReadSpeed
		plan.WriteSpeed = config.Drives[0].WriteSpeed
		plan.IOPS = 1000
		plan.Pros = []string{"最高安全性", "读取性能好"}
		plan.Cons = []string{"容量浪费大", "写入无提升"}
		plan.FaultRisk = math.Pow(1-0.99, float64(n))

	case RAID5:
		if n < 3 {
			return nil, errors.New("RAID5至少需要3块磁盘")
		}
		plan.Usable = int64(n-1) * minCap
		plan.Redundancy = 1
		plan.ReadSpeed = int64(n-1) * config.Drives[0].ReadSpeed
		plan.WriteSpeed = int64(n-1) * config.Drives[0].WriteSpeed / 2
		plan.IOPS = (n - 1) * 500
		plan.Pros = []string{"容量与安全平衡", "读取性能好"}
		plan.Cons = []string{"重建时间长", "写入有惩罚"}
		plan.FaultRisk = float64(n) * 0.01 * 0.99

	case RAID6:
		if n < 4 {
			return nil, errors.New("RAID6至少需要4块磁盘")
		}
		plan.Usable = int64(n-2) * minCap
		plan.Redundancy = 2
		plan.ReadSpeed = int64(n-2) * config.Drives[0].ReadSpeed
		plan.WriteSpeed = int64(n-2) * config.Drives[0].WriteSpeed / 3
		plan.IOPS = (n - 2) * 400
		plan.Pros = []string{"双盘容错", "大容量安全"}
		plan.Cons = []string{"写入惩罚更大", "容量损失更多"}
		plan.FaultRisk = float64(n*(n-1)) * 0.0001

	case RAID10:
		if n < 4 || n%2 != 0 {
			return nil, errors.New("RAID10需要偶数块磁盘，至少4块")
		}
		plan.Usable = int64(n/2) * minCap
		plan.Redundancy = n / 2
		plan.ReadSpeed = int64(n) * config.Drives[0].ReadSpeed
		plan.WriteSpeed = int64(n/2) * config.Drives[0].WriteSpeed
		plan.IOPS = n * 800
		plan.Pros = []string{"高性能+高安全", "重建快"}
		plan.Cons = []string{"容量损失50%", "成本高"}
		plan.FaultRisk = float64(n/2) * 0.0001

	case RAIDZ1:
		if n < 3 {
			return nil, errors.New("RAIDZ1至少需要3块磁盘")
		}
		plan.Usable = int64(n-1) * minCap
		plan.Redundancy = 1
		plan.ReadSpeed = int64(n-1) * config.Drives[0].ReadSpeed
		plan.WriteSpeed = int64(n-1) * config.Drives[0].WriteSpeed / 2
		plan.Pros = []string{"ZFS自愈", "快照支持"}
		plan.Cons = []string{"扩展不灵活", "重建慢"}
		plan.FaultRisk = float64(n) * 0.008

	case RAIDZ2:
		if n < 4 {
			return nil, errors.New("RAIDZ2至少需要4块磁盘")
		}
		plan.Usable = int64(n-2) * minCap
		plan.Redundancy = 2
		plan.ReadSpeed = int64(n-2) * config.Drives[0].ReadSpeed
		plan.WriteSpeed = int64(n-2) * config.Drives[0].WriteSpeed / 3
		plan.Pros = []string{"双盘容错+ZFS", "企业级安全"}
		plan.Cons = []string{"写入性能下降", "需要更多盘"}

	case JBOD:
		plan.Usable = plan.TotalRaw
		plan.Redundancy = 0
		plan.ReadSpeed = config.Drives[0].ReadSpeed
		plan.WriteSpeed = config.Drives[0].WriteSpeed
		plan.Pros = []string{"最大灵活性", "逐步扩展"}
		plan.Cons = []string{"无冗余", "单盘故障部分丢"}

	default:
		return nil, fmt.Errorf("不支持的RAID级别: %s", config.Level)
	}

	if plan.TotalRaw > 0 {
		plan.Efficiency = float64(plan.Usable) / float64(plan.TotalRaw)
	}

	p.mu.Lock()
	p.planCounter++
	plan.ID = fmt.Sprintf("plan_%d", p.planCounter)
	plan.Name = config.Name
	p.plans[plan.ID] = plan
	p.mu.Unlock()

	return plan, nil
}

// ========== 故障模拟 ==========

// SimulateFault 模拟磁盘故障
func (p *Planner) SimulateFault(level RAIDLevel, totalDrives, failedDrives int) *FaultSimulation {
	sim := &FaultSimulation{
		FailedDrives: failedDrives,
	}

	switch level {
	case RAID0:
		sim.DataLoss = failedDrives >= 1
		sim.RebuildTime = "N/A"
		sim.DegradedPerf = 0
		sim.RiskLevel = "极高"

	case RAID1:
		sim.DataLoss = failedDrives >= totalDrives
		sim.RebuildTime = fmt.Sprintf("%.0f小时", float64(totalDrives)*2)
		sim.DegradedPerf = 80
		sim.RiskLevel = "低"

	case RAID5, RAIDZ1:
		sim.DataLoss = failedDrives >= 2
		sim.RebuildTime = fmt.Sprintf("%.0f小时", float64(totalDrives)*4)
		sim.DegradedPerf = 40
		if failedDrives >= 1 {
			sim.RiskLevel = "高"
		} else {
			sim.RiskLevel = "低"
		}

	case RAID6, RAIDZ2:
		sim.DataLoss = failedDrives >= 3
		sim.RebuildTime = fmt.Sprintf("%.0f小时", float64(totalDrives)*6)
		sim.DegradedPerf = 50
		if failedDrives >= 2 {
			sim.RiskLevel = "高"
		} else {
			sim.RiskLevel = "低"
		}

	case RAID10:
		sim.DataLoss = failedDrives > totalDrives/2
		sim.RebuildTime = fmt.Sprintf("%.0f小时", float64(totalDrives)*1.5)
		sim.DegradedPerf = 70
		sim.RiskLevel = "低"
	}

	return sim
}

// ========== 虚拟磁盘 ==========

// CreateVDisk 创建虚拟磁盘
func (p *Planner) CreateVDisk(name string, capacity int64, level RAIDLevel, drives int) (*VDisk, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.vdisks) >= MaxVDisks {
		return nil, errors.New("虚拟磁盘数量已达上限")
	}

	p.vdCounter++
	vd := &VDisk{
		ID:        fmt.Sprintf("vd_%d", p.vdCounter),
		Name:      name,
		Capacity:  capacity,
		RAIDLevel: level,
		Drives:    drives,
		CreatedAt: time.Now(),
	}
	p.vdisks[vd.ID] = vd
	return vd, nil
}

// ListVDisk 列出虚拟磁盘
func (p *Planner) ListVDisk() []*VDisk {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]*VDisk, 0, len(p.vdisks))
	for _, vd := range p.vdisks {
		result = append(result, vd)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// ========== 推荐 ==========

// RecommendRAID 推荐RAID方案
func (p *Planner) RecommendRAID(drives int, priority string) []RAIDLevel {
	levels := make([]RAIDLevel, 0)

	switch priority {
	case "performance":
		if drives >= 4 {
			levels = append(levels, RAID10, RAID0)
		} else {
			levels = append(levels, RAID0)
		}
	case "safety":
		if drives >= 4 {
			levels = append(levels, RAID6, RAID10, RAID5)
		} else if drives >= 3 {
			levels = append(levels, RAID5, RAID1)
		} else {
			levels = append(levels, RAID1)
		}
	case "capacity":
		if drives >= 4 {
			levels = append(levels, RAID5, RAID6, RAIDZ1)
		} else {
			levels = append(levels, RAID5, JBOD)
		}
	case "balanced":
		if drives >= 6 {
			levels = append(levels, RAID6, RAID10, RAID50)
		} else if drives >= 4 {
			levels = append(levels, RAID5, RAID10)
		} else {
			levels = append(levels, RAID5, RAID1)
		}
	default:
		levels = append(levels, RAID5, RAID6, RAID10)
	}
	return levels
}

// ========== 统计 ==========

// GetStats 获取统计
func (p *Planner) GetStats() *PlannerStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return &PlannerStats{
		TotalPlans:  len(p.plans),
		TotalVDisk:  len(p.vdisks),
		TotalDrives: len(p.drives),
	}
}

// Close 关闭
func (p *Planner) Close() error {
	return nil
}
