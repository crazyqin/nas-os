// Package sysupdate 提供系统更新检查：对比 GitHub Releases 最新版本与当前运行版本.
package sysupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	appversion "nas-os/internal/version"

	"golang.org/x/mod/semver"
)

const (
	// defaultAPIURL 公开仓库的 latest Release 接口，无需认证.
	defaultAPIURL = "https://api.github.com/repos/crazyqin/nas-os/releases/latest"
	// requestTimeout 网络超时，避免弱网环境拖住页面.
	requestTimeout = 10 * time.Second
	// notesMaxRunes 返回给前端的更新说明最大字符数.
	notesMaxRunes = 4000
)

// UpdateStatus 更新检查结果.
type UpdateStatus struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion,omitempty"`
	UpdateAvailable bool   `json:"updateAvailable"`
	ReleaseURL      string `json:"releaseUrl,omitempty"`
	ReleaseNotes    string `json:"releaseNotes,omitempty"`
	PublishedAt     string `json:"publishedAt,omitempty"`
	CheckedAt       string `json:"checkedAt"`
	Error           string `json:"error,omitempty"`
}

// Checker 更新检查器.
type Checker struct {
	apiURL  string
	current string
	client  *http.Client
}

// NewChecker 创建更新检查器.
func NewChecker() *Checker {
	return &Checker{
		apiURL:  defaultAPIURL,
		current: appversion.GetVersion(),
		client:  &http.Client{Timeout: requestTimeout},
	}
}

// githubRelease 仅解析需要的字段.
type githubRelease struct {
	TagName     string `json:"tag_name"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
	Body        string `json:"body"`
}

// Check 查询 GitHub 最新 Release 并与当前版本比较.
// 网络不可达或应答异常时不返回 error，而是带 Error 说明的降级结果（UpdateAvailable=false）.
func (c *Checker) Check(ctx context.Context) UpdateStatus {
	status := UpdateStatus{
		CurrentVersion: c.current,
		CheckedAt:      time.Now().Format(time.RFC3339),
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL, nil)
	if err != nil {
		status.Error = fmt.Sprintf("构建请求失败: %v", err)
		return status
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.client.Do(req)
	if err != nil {
		status.Error = "无法连接 GitHub（设备可能离线或网络受限）"
		return status
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		status.Error = "上游尚无任何发布版本"
		return status
	}
	if resp.StatusCode != http.StatusOK {
		status.Error = fmt.Sprintf("GitHub 返回异常状态: %d", resp.StatusCode)
		return status
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		status.Error = fmt.Sprintf("解析发布信息失败: %v", err)
		return status
	}
	if strings.TrimSpace(release.TagName) == "" {
		status.Error = "发布信息缺少版本标签"
		return status
	}

	status.LatestVersion = release.TagName
	status.ReleaseURL = release.HTMLURL
	status.PublishedAt = release.PublishedAt
	status.ReleaseNotes = truncateRunes(release.Body, notesMaxRunes)
	status.UpdateAvailable = compareVersions(c.current, release.TagName) < 0
	return status
}

// compareVersions 比较当前版本与 Release 标签（v 前缀可有可无）.
// 返回 -1 表示有新版本，0 相同，1 当前版本更新.
// 标签不符合 semver 时退化为字符串比较：不同即视为有更新.
func compareVersions(current, tag string) int {
	cur := semver.Canonical("v" + strings.TrimPrefix(current, "v"))
	lat := semver.Canonical("v" + strings.TrimPrefix(tag, "v"))
	if semver.IsValid(cur) && semver.IsValid(lat) {
		return semver.Compare(cur, lat)
	}
	if current == tag {
		return 0
	}
	return -1
}

// truncateRunes 按字符数截断，避免超长 changelog 撑爆响应.
func truncateRunes(s string, max int) string {
	if max <= 0 {
		return s
	}
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	return string(r[:max]) + "\n…"
}
