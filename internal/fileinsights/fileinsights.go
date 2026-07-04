// Package fileinsights turns file listings into actionable NAS housekeeping hints.
package fileinsights

import (
	"fmt"
	"sort"
	"time"

	"nas-os/internal/smartfolders"
)

// Action is a user-facing recommendation inspired by polished NAS assistants.
type Action struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Severity string `json:"severity"`
	Reason   string `json:"reason"`
	NextStep string `json:"next_step"`
	Matched  int    `json:"matched"`
	Bytes    int64  `json:"bytes"`
}

// Profile summarizes a file corpus for dashboard/advisor use.
type Profile struct {
	GeneratedAt  time.Time                        `json:"generated_at"`
	Scanned      int                              `json:"scanned"`
	ByClass      map[smartfolders.FileClass]int   `json:"by_class"`
	BytesByClass map[smartfolders.FileClass]int64 `json:"bytes_by_class"`
	Actions      []Action                         `json:"actions"`
}

// Advisor generates deterministic file-governance recommendations.
type Advisor struct {
	largeFileThreshold int64
	archiveThreshold   int64
	now                func() time.Time
}

// NewAdvisor creates an advisor with conservative home-NAS thresholds.
func NewAdvisor() Advisor {
	return Advisor{
		largeFileThreshold: 1 << 30,
		archiveThreshold:   10 << 30,
		now:                time.Now,
	}
}

// WithThresholds returns a copy using custom thresholds. Non-positive values keep defaults.
func (a Advisor) WithThresholds(largeFileThreshold, archiveThreshold int64) Advisor {
	if largeFileThreshold > 0 {
		a.largeFileThreshold = largeFileThreshold
	}
	if archiveThreshold > 0 {
		a.archiveThreshold = archiveThreshold
	}
	return a
}

// BuildProfile analyzes a smartfolders result and emits Synology/fnOS/TrueNAS-style next steps.
func (a Advisor) BuildProfile(res *smartfolders.Result) Profile {
	profile := Profile{
		GeneratedAt:  a.now(),
		ByClass:      make(map[smartfolders.FileClass]int),
		BytesByClass: make(map[smartfolders.FileClass]int64),
	}
	if res == nil {
		return profile
	}
	profile.Scanned = res.Scanned
	for class, count := range res.Summary.ByClass {
		profile.ByClass[class] = count
	}
	for class, bytes := range res.Summary.SizeByClass {
		profile.BytesByClass[class] = bytes
	}

	var largeCount int
	var largeBytes int64
	for _, item := range res.Items {
		if item.Size >= a.largeFileThreshold {
			largeCount++
			largeBytes += item.Size
		}
	}
	if largeCount > 0 {
		profile.Actions = append(profile.Actions, Action{
			ID:       "review-large-files",
			Title:    "复核大文件占用",
			Severity: severityByBytes(largeBytes, a.archiveThreshold),
			Reason:   fmt.Sprintf("发现 %d 个大文件，占用 %s", largeCount, humanBytes(largeBytes)),
			NextStep: "打开智能文件夹 large-files，删除重复下载、转移冷数据或加入归档策略。",
			Matched:  largeCount,
			Bytes:    largeBytes,
		})
	}
	if photoCount := profile.ByClass[smartfolders.ClassPhoto]; photoCount >= 3 {
		profile.Actions = append(profile.Actions, Action{
			ID:       "enable-ai-photo-index",
			Title:    "开启照片语义整理",
			Severity: "info",
			Reason:   fmt.Sprintf("照片库已有 %d 个文件，适合建立时间线、人脸和以文搜图索引", photoCount),
			NextStep: "为 photos 智能文件夹启用缩略图、EXIF 时间线和本地语义索引。",
			Matched:  photoCount,
			Bytes:    profile.BytesByClass[smartfolders.ClassPhoto],
		})
	}
	if videoCount := profile.ByClass[smartfolders.ClassVideo]; videoCount >= 2 {
		profile.Actions = append(profile.Actions, Action{
			ID:       "prepare-media-library",
			Title:    "整理影视媒体库",
			Severity: "info",
			Reason:   fmt.Sprintf("视频库已有 %d 个文件，可提升海报墙、字幕和跨端续播体验", videoCount),
			NextStep: "为 videos 智能文件夹启用刮削、字幕匹配和转码能力检测。",
			Matched:  videoCount,
			Bytes:    profile.BytesByClass[smartfolders.ClassVideo],
		})
	}
	sort.SliceStable(profile.Actions, func(i, j int) bool {
		return severityRank(profile.Actions[i].Severity) > severityRank(profile.Actions[j].Severity)
	})
	return profile
}

func severityByBytes(bytes, high int64) string {
	if bytes >= high {
		return "warning"
	}
	return "info"
}

func severityRank(sev string) int {
	switch sev {
	case "critical":
		return 4
	case "warning":
		return 3
	case "info":
		return 2
	default:
		return 1
	}
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
