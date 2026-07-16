// archive.go - 识别结果归档和索引
package aiocr

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Archiver 归档管理器.
type Archiver struct {
	config    *Config
	index     map[string]*ArchiveEntry
	documents map[string]*OCRResult
}

// NewArchiver 创建归档管理器.
func NewArchiver(cfg *Config) *Archiver {
	a := &Archiver{
		config:    cfg,
		index:     make(map[string]*ArchiveEntry),
		documents: make(map[string]*OCRResult),
	}

	// 创建归档目录
	if err := os.MkdirAll(cfg.ArchivePath, 0755); err != nil {
		log.Printf("⚠️ 创建归档目录失败: %v", err)
	}

	return a
}

// Archive 归档识别结果.
func (a *Archiver) Archive(result *OCRResult) error {
	log.Printf("📁 归档识别结果: %s", result.ID)

	// 生成归档路径
	archivePath := a.generatePath(result)

	// 创建归档目录
	if err := os.MkdirAll(filepath.Dir(archivePath), 0755); err != nil {
		return fmt.Errorf("创建归档目录失败: %w", err)
	}

	// 保存识别结果
	if err := a.saveResult(result, archivePath); err != nil {
		return fmt.Errorf("保存识别结果失败: %w", err)
	}

	// 计算校验和
	checksum, err := a.calculateChecksum(archivePath)
	if err != nil {
		log.Printf("⚠️ 计算校验和失败: %v", err)
	}

	// 获取文件信息
	info, err := os.Stat(archivePath)
	if err != nil {
		log.Printf("⚠️ 获取文件信息失败: %v", err)
	}

	// 创建索引条目
	entry := &ArchiveEntry{
		ID:        result.ID,
		DocID:     result.ID,
		FilePath:  archivePath,
		Category:  result.Template,
		Tags:      a.generateTags(result),
		Summary:   a.generateSummary(result),
		IndexedAt: time.Now(),
		Size:      info.Size(),
		Checksum:  checksum,
	}

	// 保存索引
	a.index[entry.ID] = entry
	a.documents[result.ID] = result

	// 持久化索引
	if err := a.saveIndex(); err != nil {
		log.Printf("⚠️ 保存索引失败: %v", err)
	}

	log.Printf("✅ 归档完成: %s -> %s", result.ID, archivePath)
	return nil
}

// generatePath 生成归档路径.
func (a *Archiver) generatePath(result *OCRResult) string {
	// 按日期和分类组织目录结构
	now := time.Now()
	datePath := now.Format("2006/01/02")
	category := "other"
	if result.Template != "" {
		category = result.Template
	}

	// 文件名：ID + 扩展名
	filename := result.ID + ".json"

	return filepath.Join(a.config.ArchivePath, datePath, category, filename)
}

// saveResult 保存识别结果.
func (a *Archiver) saveResult(result *OCRResult, path string) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化结果失败: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// calculateChecksum 计算校验和.
func (a *Archiver) calculateChecksum(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash), nil
}

// generateTags 生成标签.
func (a *Archiver) generateTags(result *OCRResult) []string {
	tags := make([]string, 0)

	// 添加分类标签
	if result.Template != "" {
		tags = append(tags, result.Template)
	}

	// 添加语言标签
	if result.Language != "" {
		tags = append(tags, "lang:"+result.Language)
	}

	// 从结构化数据提取标签
	if result.Structured != nil {
		for key := range result.Structured.Fields {
			tags = append(tags, "field:"+key)
		}
	}

	// 添加置信度标签
	if result.Confidence > 0.9 {
		tags = append(tags, "high-confidence")
	} else if result.Confidence > 0.7 {
		tags = append(tags, "medium-confidence")
	} else {
		tags = append(tags, "low-confidence")
	}

	return tags
}

// generateSummary 生成摘要.
func (a *Archiver) generateSummary(result *OCRResult) string {
	// 提取前 200 个字符作为摘要
	text := result.FullText
	if len(text) > 200 {
		text = text[:200] + "..."
	}

	// 清理换行符
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.TrimSpace(text)

	return text
}

// saveIndex 保存索引.
func (a *Archiver) saveIndex() error {
	indexPath := filepath.Join(a.config.ArchivePath, "index.json")

	data, err := json.MarshalIndent(a.index, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化索引失败: %w", err)
	}

	return os.WriteFile(indexPath, data, 0644)
}

// loadIndex 加载索引.
func (a *Archiver) loadIndex() error {
	indexPath := filepath.Join(a.config.ArchivePath, "index.json")

	data, err := os.ReadFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取索引失败: %w", err)
	}

	return json.Unmarshal(data, &a.index)
}

// Search 搜索归档文档.
func (a *Archiver) Search(query *SearchQuery) ([]*ArchiveEntry, error) {
	log.Printf("🔍 搜索归档文档: %v", query)

	results := make([]*ArchiveEntry, 0)

	for _, entry := range a.index {
		if a.matchEntry(entry, query) {
			results = append(results, entry)
		}
	}

	// 应用分页
	if query.Offset > 0 && query.Offset < len(results) {
		results = results[query.Offset:]
	}
	if query.Limit > 0 && query.Limit < len(results) {
		results = results[:query.Limit]
	}

	log.Printf("✅ 搜索完成，结果数: %d", len(results))
	return results, nil
}

// matchEntry 匹配索引条目.
func (a *Archiver) matchEntry(entry *ArchiveEntry, query *SearchQuery) bool {
	// 关键词匹配
	if query.Keyword != "" {
		keyword := strings.ToLower(query.Keyword)
		if !strings.Contains(strings.ToLower(entry.Summary), keyword) &&
			!strings.Contains(strings.ToLower(entry.Category), keyword) {
			// 检查标签
			found := false
			for _, tag := range entry.Tags {
				if strings.Contains(strings.ToLower(tag), keyword) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	// 分类匹配
	if query.Category != "" && entry.Category != query.Category {
		return false
	}

	// 标签匹配
	if len(query.Tags) > 0 {
		for _, queryTag := range query.Tags {
			found := false
			for _, entryTag := range entry.Tags {
				if entryTag == queryTag {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	// 时间范围匹配
	if query.StartTime != nil && entry.IndexedAt.Before(*query.StartTime) {
		return false
	}
	if query.EndTime != nil && entry.IndexedAt.After(*query.EndTime) {
		return false
	}

	return true
}

// GetDocument 获取归档文档.
func (a *Archiver) GetDocument(id string) (*OCRResult, error) {
	result, exists := a.documents[id]
	if !exists {
		// 尝试从文件加载
		entry, exists := a.index[id]
		if !exists {
			return nil, fmt.Errorf("文档不存在: %s", id)
		}

		result, err := a.loadDocument(entry.FilePath)
		if err != nil {
			return nil, fmt.Errorf("加载文档失败: %w", err)
		}

		a.documents[id] = result
		return result, nil
	}

	return result, nil
}

// loadDocument 从文件加载文档.
func (a *Archiver) loadDocument(path string) (*OCRResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	result := &OCRResult{}
	if err := json.Unmarshal(data, result); err != nil {
		return nil, err
	}

	return result, nil
}

// DeleteDocument 删除归档文档.
func (a *Archiver) DeleteDocument(id string) error {
	entry, exists := a.index[id]
	if !exists {
		return fmt.Errorf("文档不存在: %s", id)
	}

	// 删除文件
	if err := os.Remove(entry.FilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除文件失败: %w", err)
	}

	// 删除索引
	delete(a.index, id)
	delete(a.documents, id)

	// 保存索引
	if err := a.saveIndex(); err != nil {
		log.Printf("⚠️ 保存索引失败: %v", err)
	}

	log.Printf("✅ 删除文档: %s", id)
	return nil
}

// Cleanup 清理过期文档.
func (a *Archiver) Cleanup() (int, error) {
	log.Printf("🧹 开始清理过期文档，保留天数: %d", a.config.RetentionDays)

	cutoff := time.Now().AddDate(0, 0, -a.config.RetentionDays)
	deleted := 0

	for id, entry := range a.index {
		if entry.IndexedAt.Before(cutoff) {
			if err := a.DeleteDocument(id); err != nil {
				log.Printf("⚠️ 删除过期文档失败: %s, %v", id, err)
			} else {
				deleted++
			}
		}
	}

	log.Printf("✅ 清理完成，删除文档: %d", deleted)
	return deleted, nil
}

// GetStats 获取归档统计.
func (a *Archiver) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"total_documents": len(a.index),
		"categories":      make(map[string]int),
		"total_size":      int64(0),
	}

	categories := stats["categories"].(map[string]int)
	var totalSize int64

	for _, entry := range a.index {
		categories[entry.Category]++
		totalSize += entry.Size
	}

	stats["total_size"] = totalSize

	return stats
}

// GetEntries 获取所有索引条目.
func (a *Archiver) GetEntries() []*ArchiveEntry {
	entries := make([]*ArchiveEntry, 0, len(a.index))
	for _, entry := range a.index {
		entries = append(entries, entry)
	}
	return entries
}

// GetEntry 获取索引条目.
func (a *Archiver) GetEntry(id string) (*ArchiveEntry, error) {
	entry, exists := a.index[id]
	if !exists {
		return nil, fmt.Errorf("条目不存在: %s", id)
	}
	return entry, nil
}
