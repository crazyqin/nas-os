// Package sharesearchadvisor turns NAS usage signals into practical WebShare and search recommendations.
package sharesearchadvisor

import (
	"sort"
	"strings"
)

// Signal describes current file sharing and search usage signals.
type Signal struct {
	TotalFiles          int
	IndexedFiles        int
	SharedLinks         int
	ExternalShares      int
	PhotoFiles          int
	VideoFiles          int
	OfficeFiles         int
	EncryptedDatasets   int
	RemoteUsers         int
	FailedSearches      int
	AverageSearchMillis int
	SnapshotCount       int
	SMBEnabled          bool
	NFSEnabled          bool
	WebShareEnabled     bool
	SearchEnabled       bool
	MobileAccessEnabled bool
	PasskeyEnabled      bool
}

// Recommendation is an actionable product suggestion for UI, notifications, or onboarding.
type Recommendation struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Priority string   `json:"priority"`
	Reason   string   `json:"reason"`
	Actions  []string `json:"actions"`
}

// Report is the generated advisor output.
type Report struct {
	ReadinessScore  int              `json:"readinessScore"`
	CoveragePercent int              `json:"coveragePercent"`
	Recommendations []Recommendation `json:"recommendations"`
}

// Advisor evaluates WebShare, TrueSearch-like indexing, and secure sharing readiness.
type Advisor struct{}

// New creates a share/search advisor.
func New() *Advisor { return &Advisor{} }

// Generate builds a deterministic report from current signals.
func (a *Advisor) Generate(s Signal) Report {
	coverage := coveragePercent(s.IndexedFiles, s.TotalFiles)
	recs := make([]Recommendation, 0, 6)

	if !s.WebShareEnabled && s.TotalFiles > 0 {
		recs = append(recs, Recommendation{
			ID:       "enable-webshare",
			Title:    "开启浏览器文件分享",
			Priority: "high",
			Reason:   "已有文件库但未启用 WebShare，移动端和外部协作仍依赖 SMB/NFS 客户端。",
			Actions: []string{
				"为常用数据集开启 WebShare 入口",
				"提供上传、下载、筛选和隐藏文件开关",
				"为外部访问生成可撤销分享链接",
			},
		})
	}

	if s.SearchEnabled && s.TotalFiles >= 1000 && coverage < 80 {
		recs = append(recs, Recommendation{
			ID:       "expand-search-index",
			Title:    "补齐文件搜索索引",
			Priority: priorityByCoverage(coverage),
			Reason:   "文件量已进入需要秒级搜索的规模，但索引覆盖率不足。",
			Actions: []string{
				"优先索引文件名、类型、mtime 和常用文档内容",
				"把 SSD 或快速卷作为索引缓存位置",
				"对加密数据集显示不可索引原因",
			},
		})
	}

	if !s.SearchEnabled && s.TotalFiles >= 500 {
		recs = append(recs, Recommendation{
			ID:       "enable-search",
			Title:    "启用本地统一搜索",
			Priority: "high",
			Reason:   "文件规模增长后，目录遍历会明显降低查找效率。",
			Actions: []string{
				"启用本地文件名与类型索引",
				"为照片、视频、Office 文件建立分类过滤器",
				"保留本地处理，避免隐私外传",
			},
		})
	}

	if s.ExternalShares > 0 && !s.PasskeyEnabled {
		recs = append(recs, Recommendation{
			ID:       "secure-external-share",
			Title:    "加固外部分享认证",
			Priority: "high",
			Reason:   "存在外部分享链接，建议使用更强认证和过期策略降低泄露风险。",
			Actions: []string{
				"为外链启用 passkey 或一次性访问码",
				"设置到期时间、下载次数和审计记录",
				"对高风险链接提示撤销或转为内网访问",
			},
		})
	}

	if s.PhotoFiles+s.VideoFiles >= 300 && s.MobileAccessEnabled {
		recs = append(recs, Recommendation{
			ID:       "mobile-media-experience",
			Title:    "优化移动端相册与影音体验",
			Priority: "medium",
			Reason:   "照片和视频库已具备家庭媒体中心特征，移动端体验会直接影响日常使用。",
			Actions: []string{
				"按时间线、人物/地点和媒体类型生成快捷筛选",
				"为视频准备海报墙、字幕匹配和续播状态",
				"为大文件外链提供预览而非直接下载",
			},
		})
	}

	if s.SnapshotCount == 0 && s.SharedLinks+s.ExternalShares > 0 {
		recs = append(recs, Recommendation{
			ID:       "snapshot-before-share",
			Title:    "分享前建立快照保护",
			Priority: "medium",
			Reason:   "共享协作会增加误删和覆盖风险，快照可提供低成本回滚。",
			Actions: []string{
				"为共享目录创建只读快照策略",
				"在 WebShare 中展示快照时间线入口",
				"将删除/覆盖事件写入审计日志",
			},
		})
	}

	if s.FailedSearches >= 10 || s.AverageSearchMillis > 1500 {
		recs = append(recs, Recommendation{
			ID:       "search-quality-tuning",
			Title:    "调优搜索质量与延迟",
			Priority: "medium",
			Reason:   "搜索失败或延迟偏高会削弱桌面级文件查找体验。",
			Actions: []string{
				"记录无结果关键词并补充同义词/扩展名映射",
				"将热门目录加入增量索引队列",
				"对慢查询输出索引命中率和扫描路径",
			},
		})
	}

	sort.SliceStable(recs, func(i, j int) bool {
		return priorityRank(recs[i].Priority) < priorityRank(recs[j].Priority)
	})

	return Report{
		ReadinessScore:  readinessScore(s, coverage, recs),
		CoveragePercent: coverage,
		Recommendations: recs,
	}
}

// SummarizeActions returns a compact semicolon-separated action list for notifications.
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

func coveragePercent(indexed, total int) int {
	if total <= 0 || indexed <= 0 {
		return 0
	}
	if indexed >= total {
		return 100
	}
	return indexed * 100 / total
}

func priorityByCoverage(coverage int) string {
	if coverage < 40 {
		return "high"
	}
	return "medium"
}

func priorityRank(priority string) int {
	switch priority {
	case "high":
		return 0
	case "medium":
		return 1
	default:
		return 2
	}
}

func readinessScore(s Signal, coverage int, recs []Recommendation) int {
	score := 45
	if s.WebShareEnabled {
		score += 15
	}
	if s.SearchEnabled {
		score += 10
	}
	if s.MobileAccessEnabled {
		score += 5
	}
	if s.PasskeyEnabled {
		score += 10
	}
	if s.SnapshotCount > 0 {
		score += 10
	}
	score += coverage / 5
	for _, rec := range recs {
		if rec.Priority == "high" {
			score -= 10
		}
	}
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}
