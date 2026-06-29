package mediascraper

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SubtitleManager 字幕管理器
// 负责搜索、下载和管理字幕文件，支持多语言
type SubtitleManager struct {
	scraper *Scraper // 引用刮削器，复用其字幕数据库
	saveDir string   // 字幕保存目录
}

// NewSubtitleManager 创建字幕管理器
// saveDir 为字幕保存根目录，为空则默认使用 /tmp/subtitles
func NewSubtitleManager(scraper *Scraper, saveDir string) *SubtitleManager {
	if saveDir == "" {
		saveDir = "/tmp/subtitles"
	}
	return &SubtitleManager{
		scraper: scraper,
		saveDir: saveDir,
	}
}

// Search 搜索字幕，返回指定媒体和语言的可用字幕列表
// mediaKey 为媒体的查找键（如 "inception_2010"）
// lang 为语言代码（如 "zh-CN", "en-US"），为空则返回所有语言
func (m *SubtitleManager) Search(mediaKey string, lang string) ([]subtitleRecord, error) {
	records, ok := m.scraper.subtitleDB[mediaKey]
	if !ok {
		return nil, fmt.Errorf("未找到媒体 %s 的字幕资源", mediaKey)
	}

	if lang == "" {
		// 返回所有可用字幕
		result := make([]subtitleRecord, len(records))
		copy(result, records)
		return result, nil
	}

	// 按语言过滤
	filtered := make([]subtitleRecord, 0)
	for _, r := range records {
		if r.Language == lang {
			filtered = append(filtered, r)
		}
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("未找到媒体 %s 的 %s 字幕", mediaKey, lang)
	}
	return filtered, nil
}

// SearchByItem 通过 MediaItem 搜索字幕
func (m *SubtitleManager) SearchByItem(item *MediaItem, lang string) ([]subtitleRecord, error) {
	if item == nil {
		return nil, errors.New("媒体项不能为空")
	}
	return m.Search(item.ID, lang)
}

// Download 下载字幕并保存到文件
// mediaKey 为媒体查找键，lang 为目标语言
// 返回 SubtitleResult 包含保存路径等信息
func (m *SubtitleManager) Download(mediaKey string, lang string) *SubtitleResult {
	records, err := m.Search(mediaKey, lang)
	if err != nil {
		return &SubtitleResult{
			Error: err,
		}
	}
	if len(records) == 0 {
		return &SubtitleResult{
			Error: fmt.Errorf("没有可用的 %s 字幕", lang),
		}
	}

	// 选取第一个匹配的字幕
	sub := records[0]

	// 确保保存目录存在
	if err := os.MkdirAll(m.saveDir, 0755); err != nil {
		return &SubtitleResult{
			Error: fmt.Errorf("创建字幕目录失败: %w", err),
		}
	}

	// 生成字幕文件名：媒体键_语言.srt
	// 将语言代码中的特殊字符替换为下划线
	safeLang := strings.ReplaceAll(lang, "-", "_")
	filename := fmt.Sprintf("%s_%s.srt", mediaKey, safeLang)
	savePath := filepath.Join(m.saveDir, filename)

	// 写入字幕内容
	if err := os.WriteFile(savePath, []byte(sub.Content), 0644); err != nil {
		return &SubtitleResult{
			Error: fmt.Errorf("写入字幕文件失败: %w", err),
		}
	}

	return &SubtitleResult{
		FilePath:     savePath,
		Language:     sub.Language,
		Source:       sub.Source,
		DownloadedAt: time.Now(),
		Error:        nil,
	}
}

// DownloadByItem 通过 MediaItem 下载字幕
func (m *SubtitleManager) DownloadByItem(item *MediaItem, lang string) *SubtitleResult {
	if item == nil {
		return &SubtitleResult{
			Error: errors.New("媒体项不能为空"),
		}
	}
	return m.Download(item.ID, lang)
}

// DownloadMulti 下载多语言字幕
// langs 为需要下载的语言列表，返回每种语言的结果
func (m *SubtitleManager) DownloadMulti(mediaKey string, langs []string) []*SubtitleResult {
	results := make([]*SubtitleResult, 0, len(langs))
	for _, lang := range langs {
		result := m.Download(mediaKey, lang)
		results = append(results, result)
	}
	return results
}

// ListAvailableLanguages 列出指定媒体可用的字幕语言
func (m *SubtitleManager) ListAvailableLanguages(mediaKey string) ([]string, error) {
	records, ok := m.scraper.subtitleDB[mediaKey]
	if !ok {
		return nil, fmt.Errorf("未找到媒体 %s 的字幕资源", mediaKey)
	}

	langs := make([]string, 0, len(records))
	seen := make(map[string]bool)
	for _, r := range records {
		if !seen[r.Language] {
			langs = append(langs, r.Language)
			seen[r.Language] = true
		}
	}
	return langs, nil
}

// GetSaveDir 获取字幕保存目录
func (m *SubtitleManager) GetSaveDir() string {
	return m.saveDir
}

// SetSaveDir 设置字幕保存目录
func (m *SubtitleManager) SetSaveDir(dir string) {
	m.saveDir = dir
}
