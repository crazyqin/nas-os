package experienceadvisor

import (
	"sort"
	"time"
)

const bytesPerGB = int64(1024 * 1024 * 1024)

// Advisor generates recommendations from local, aggregate-only signals.
type Advisor struct {
	config AdvisorConfig
	now    func() time.Time
}

// New creates an advisor with default thresholds unless cfg is provided.
func New(cfg *AdvisorConfig) *Advisor {
	config := DefaultConfig()
	if cfg != nil {
		config = *cfg
	}
	return &Advisor{config: config, now: time.Now}
}

// WithNow overrides the clock for deterministic tests.
func (a *Advisor) WithNow(now func() time.Time) *Advisor {
	if now != nil {
		a.now = now
	}
	return a
}

// Recommend converts signals into prioritized next-best-action cards.
func (a *Advisor) Recommend(signals []Signal) []Recommendation {
	var out []Recommendation
	for _, signal := range signals {
		if !signal.Enabled {
			continue
		}
		out = append(out, a.recommendForSignal(signal)...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority == out[j].Priority {
			return out[i].ID < out[j].ID
		}
		return out[i].Priority > out[j].Priority
	})
	return out
}

func (a *Advisor) recommendForSignal(signal Signal) []Recommendation {
	var out []Recommendation
	sizeGB := signal.SizeBytes / bytesPerGB

	switch signal.Workload {
	case WorkloadPhotos:
		if signal.ItemCount >= a.config.LargePhotoLibraryCount {
			out = append(out, Recommendation{
				ID:       "photos-ai-curation",
				Workload: WorkloadPhotos,
				Title:    "启用本地 AI 相册整理",
				Description: "检测到大型照片库，建议开启人物聚类、场景标签、动态照片解析和重复照片清理，" +
					"提升移动端备份后的检索体验。",
				Priority:   92,
				Actions:    []string{"启用 photoai 索引", "开启 motionphoto 解析", "安排重复照片扫描"},
				Benchmarks: []string{"飞牛 fnOS AI 相册", "Synology Photos"},
			})
		}
	case WorkloadMedia:
		if sizeGB >= a.config.LargeMediaLibraryGB {
			out = append(out, Recommendation{
				ID:          "media-posterwall-transcode",
				Workload:    WorkloadMedia,
				Title:       "构建智能海报墙与转码策略",
				Description: "媒体库规模较大，建议执行刮削、字幕匹配、播放进度同步和按设备带宽生成转码配置。",
				Priority:    88,
				Actions:     []string{"运行媒体刮削", "生成转码预设", "开启跨端播放进度同步"},
				Benchmarks:  []string{"飞牛影视", "Plex", "Emby"},
			})
		}
	case WorkloadBackup:
		if sizeGB >= a.config.BackupSizeGB || signal.ActiveDevices >= a.config.MinActiveDevices {
			out = append(out, Recommendation{
				ID:          "backup-lifecycle-tiering",
				Workload:    WorkloadBackup,
				Title:       "配置备份生命周期与分层保留",
				Description: "备份数据增长明显，建议采用每日/每周/月度保留、冷数据归档、加密与恢复演练。",
				Priority:    86,
				Actions:     []string{"生成 smartlifebackup 策略", "启用恢复校验", "归档长期备份"},
				Benchmarks:  []string{"Synology Hyper Backup", "TrueNAS Replication"},
			})
		}
	case WorkloadRemote:
		if signal.ActiveDevices >= a.config.MinActiveDevices || signal.ErrorCount >= a.config.HighErrorCount {
			priority := 80
			if signal.ErrorCount >= a.config.HighErrorCount {
				priority = 90
			}
			out = append(out, Recommendation{
				ID:          "remote-access-health",
				Workload:    WorkloadRemote,
				Title:       "优化远程访问健康度",
				Description: "多设备或异常连接较多，建议检查 NAT 穿透、DDNS、证书续期和 WebDAV 暴露面。",
				Priority:    priority,
				Actions:     []string{"运行 nasconnect 健康检查", "验证证书续期", "收敛公网入口"},
				Benchmarks:  []string{"飞牛 FN Connect", "Synology QuickConnect"},
			})
		}
	case WorkloadStorage:
		if signal.ErrorCount >= a.config.HighErrorCount {
			out = append(out, Recommendation{
				ID:          "storage-snapshot-scrub",
				Workload:    WorkloadStorage,
				Title:       "加强快照与数据完整性巡检",
				Description: "存储异常次数偏高，建议增加不可变快照、scrub、SMART 检查和故障预警。",
				Priority:    95,
				Actions:     []string{"创建 smartsnapshot 策略", "安排 scrub", "开启磁盘健康 AI 预警"},
				Benchmarks:  []string{"TrueNAS ZFS", "Synology Snapshot Replication"},
			})
		}
	case WorkloadApps:
		if a.isStale(signal.LastActivity) {
			out = append(out, Recommendation{
				ID:          "apps-curation-cleanup",
				Workload:    WorkloadApps,
				Title:       "清理低活跃应用并推荐替代模板",
				Description: "应用中心近期活跃度低，建议归档未使用应用、刷新推荐模板并优先展示备份/安全/媒体刚需应用。",
				Priority:    72,
				Actions:     []string{"标记低活跃应用", "刷新 smartappcurator 推荐", "检查 compose 模板版本"},
				Benchmarks:  []string{"DSM Package Center", "TrueNAS Apps"},
			})
		}
	}
	return out
}

func (a *Advisor) isStale(last time.Time) bool {
	if last.IsZero() {
		return true
	}
	return a.now().Sub(last) >= time.Duration(a.config.StaleDays)*24*time.Hour
}
