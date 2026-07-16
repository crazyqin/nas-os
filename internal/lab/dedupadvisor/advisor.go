// Package dedupadvisor converts存储池去重信号为可操作的去重优化建议。
package dedupadvisor

import (
	"sort"
	"strings"
	"time"
)

// Signal 描述 NAS 存储池去重相关的信号，参考 TrueNAS ZFS Fast Dedup
// 和 Synology 存储效率管理。
type Signal struct {
	PoolName             string // 存储池名称
	TotalSizeGB          int    // 池总容量（GB）
	UsedSizeGB           int    // 已用容量（GB）
	DedupRatio           float64 // 当前去重率（1.0 = 无去重收益）
	DedupEnabled         bool    // 是否已启用去重
	FileCount            int     // 池中文件总数
	DuplicateFileEstimate int    // 估算重复文件数量
	AvgFileSizeMB        int     // 平均文件大小（MB）
	PoolType             string  // 池类型：zfs / btrfs / xfs
	HasSSDTier           bool    // 是否有 SSD 缓存层
	FreePercent          int     // 可用空间百分比
	CompressEnabled      bool    // 是否已启用压缩
	WorkloadType         string  // 工作负载类型：archive / media / vm / photos / documents
}

// Recommendation 是一条可操作的去重优化建议。
type Recommendation struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Priority string   `json:"priority"`
	Reason   string   `json:"reason"`
	Actions  []string `json:"actions"`
}

// Report 汇总当前去重状态和优化建议。
type Report struct {
	GeneratedAt     time.Time        `json:"generated_at"`
	PoolName        string           `json:"pool_name"`
	DedupScore      int              `json:"dedup_score"`
	DedupPotential  string           `json:"dedup_potential"`
	Recommendations []Recommendation `json:"recommendations"`
}

// Advisor 评估存储池去重姿态，供仪表盘和通知使用。
type Advisor struct{ now func() time.Time }

// New 创建一个去重顾问。
func New() *Advisor { return &Advisor{now: time.Now} }

// WithNow 返回使用确定性时钟的副本，用于测试。
func (a Advisor) WithNow(now func() time.Time) Advisor {
	if now != nil {
		a.now = now
	}
	return a
}

// Generate 根据当前存储池信号生成确定性报告。
func (a Advisor) Generate(s Signal) Report {
	recs := make([]Recommendation, 0, 6)
	potential := dedupPotential(s.DuplicateFileEstimate, s.FileCount)
	dupPercent := dupPercent(s.DuplicateFileEstimate, s.FileCount)

	// 未启用去重且重复文件估算 > 10% 时，建议启用去重
	if !s.DedupEnabled && dupPercent > 10 {
		priority := "medium"
		if dupPercent > 30 {
			priority = "high"
		}
		recs = append(recs, Recommendation{
			ID:       "enable-block-dedup",
			Title:    "启用块级去重",
			Priority: priority,
			Reason:   "存储池存在大量重复文件，启用块级去重可显著节省存储空间。",
			Actions: []string{
				"在低峰时段启用 ZFS Fast Dedup 或 Btrfs 去重",
				"启用前完成全量备份和一致性检查",
				"去重完成后监控内存和碎片指标",
			},
		})
	}

	// 已启用去重但去重率 < 1.1 且文件数少时，建议评估去重收益
	if s.DedupEnabled && s.DedupRatio < 1.1 && s.FileCount < 10000 {
		recs = append(recs, Recommendation{
			ID:       "evaluate-dedup-benefit",
			Title:    "评估去重收益",
			Priority: "medium",
			Reason:   "已启用去重但去重率极低，去重开销可能超过空间节省收益。",
			Actions: []string{
				"对比去重前后的实际空间节省与性能开销",
				"若收益不足，考虑关闭去重以降低元数据负担",
				"将评估结果写入存储优化报告",
			},
		})
	}

	// 无 SSD 缓存层且池类型为 zfs 时，建议添加 SSD 缓存提升去重性能
	if s.PoolType == "zfs" && !s.HasSSDTier && s.DedupEnabled {
		recs = append(recs, Recommendation{
			ID:       "add-ssd-tier-for-dedup",
			Title:    "添加 SSD 缓存层",
			Priority: "medium",
			Reason:   "ZFS 去重依赖大量元数据随机读写，缺少 SSD 缓存时性能下降明显。",
			Actions: []string{
				"添加 SSD L2ARC 或 Special vdev 存放去重元数据",
				"选择写入耐久度高的企业级 SSD",
				"部署后监控 ARC 命中率和去重查表延迟",
			},
		})
	}

	// 压缩未启用时，建议配合压缩
	if !s.CompressEnabled {
		priority := "low"
		if s.DedupEnabled {
			priority = "medium"
		}
		recs = append(recs, Recommendation{
			ID:       "enable-compression",
			Title:    "配合启用压缩",
			Priority: priority,
			Reason:   "压缩与去重配合使用可叠加空间节省效果，单独使用去重不够经济。",
			Actions: []string{
				"启用 ZFS lz4 或 gzip 压缩，或 Btrfs zlib 压缩",
				"压缩对新写入数据即时生效，旧数据可异步重写",
				"监控 CPU 开销确保不影响在线服务",
			},
		})
	}

	// 可用空间 < 20% 时，建议先清理再开去重
	if s.FreePercent > 0 && s.FreePercent < 20 && !s.DedupEnabled && dupPercent > 5 {
		recs = append(recs, Recommendation{
			ID:       "free-space-before-dedup",
			Title:    "清理空间后再开启去重",
			Priority: "high",
			Reason:   "去重过程需要额外空间存放元数据和临时数据，可用空间不足会增加风险。",
			Actions: []string{
				"清理回收站、过期备份和重复下载文件",
				"将冷数据迁移到外置盘或对象存储",
				"可用空间恢复到 20% 以上后再启用去重",
			},
		})
	}

	// 工作负载为 vm/photos 时不建议块级去重（收益低），建议文件级或跳过
	if (s.WorkloadType == "vm" || s.WorkloadType == "photos") && !s.DedupEnabled && dupPercent > 10 {
		var rec Recommendation
		if s.WorkloadType == "vm" {
			rec = Recommendation{
				ID:       "skip-block-dedup-vm",
				Title:    "VM 工作负载不建议块级去重",
				Priority: "medium",
				Reason:   "虚拟机磁盘镜像以大块随机写入为主，块级去重收益低且性能开销大。",
				Actions: []string{
					"改用精简配置（thin provisioning）减少空间占用",
					"对虚拟机模板和克隆使用文件级去重",
					"避免对活跃 VM 镜像启用块级去重",
				},
			}
		} else {
			rec = Recommendation{
				ID:       "skip-block-dedup-photos",
				Title:    "照片工作负载不建议块级去重",
				Priority: "medium",
				Reason:   "照片文件已压缩且唯一性高，块级去重收益极低。",
				Actions: []string{
					"使用文件级去重或哈希比对清理完全相同的照片",
					"对照片库启用压缩而非去重",
					"定期导出和归档旧照片释放空间",
				},
			}
		}
		recs = append(recs, rec)
	}

	sort.SliceStable(recs, func(i, j int) bool {
		left, right := priorityRank(recs[i].Priority), priorityRank(recs[j].Priority)
		if left == right {
			return recs[i].ID < recs[j].ID
		}
		return left < right
	})

	return Report{
		GeneratedAt:     a.now(),
		PoolName:        s.PoolName,
		DedupScore:      dedupScore(s, recs),
		DedupPotential:  potential,
		Recommendations: recs,
	}
}

// SummarizeActions 返回紧凑的下一步操作摘要，供通知使用。
func SummarizeActions(recs []Recommendation) string {
	parts := make([]string, 0, len(recs))
	for _, rec := range recs {
		if len(rec.Actions) == 0 {
			continue
		}
		parts = append(parts, rec.Title+": "+rec.Actions[0])
	}
	return strings.Join(parts, "; ")
}

// dedupPotential 根据重复文件比例评定去重潜力等级。
func dedupPotential(dupEstimate, fileCount int) string {
	percent := dupPercent(dupEstimate, fileCount)
	switch {
	case percent > 30:
		return "high"
	case percent > 10:
		return "medium"
	case percent > 5:
		return "low"
	default:
		return "none"
	}
}

// dupPercent 计算重复文件占比百分比。
func dupPercent(dupEstimate, fileCount int) int {
	if fileCount <= 0 {
		return 0
	}
	return dupEstimate * 100 / fileCount
}

func priorityRank(priority string) int {
	switch priority {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	default:
		return 3
	}
}

// dedupScore 从 100 开始，有建议则扣分；去重已启用且效果好则加分。
func dedupScore(s Signal, recs []Recommendation) int {
	score := 100
	for _, rec := range recs {
		switch rec.Priority {
		case "critical":
			score -= 30
		case "high":
			score -= 18
		case "medium":
			score -= 9
		default:
			score -= 4
		}
	}
	if s.DedupEnabled && s.DedupRatio >= 1.5 {
		score += 10
	}
	if s.CompressEnabled {
		score += 5
	}
	if s.HasSSDTier {
		score += 5
	}
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}
