package backup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// IncrementalBackup 增量备份管理器
type IncrementalBackup struct {
	baseDir string
}

// NewIncrementalBackup 创建增量备份管理器
func NewIncrementalBackup(baseDir string) *IncrementalBackup {
	return &IncrementalBackup{
		baseDir: baseDir,
	}
}

// BackupResult 备份结果
type BackupResult struct {
	BackupPath    string
	IsIncremental bool
	TotalFiles    int64
	ChangedFiles  int64
	Size          int64
	Duration      time.Duration
	PreviousLink  string // 指向的上一个备份
}

// CreateBackup 创建增量备份
// 使用 rsync --link-dest 实现硬链接增量备份
func (ib *IncrementalBackup) CreateBackup(sourceDir, backupName string) (*BackupResult, error) {
	startTime := time.Now()

	// 创建备份根目录
	backupRoot := filepath.Join(ib.baseDir, backupName)
	if err := os.MkdirAll(backupRoot, 0755); err != nil {
		return nil, fmt.Errorf("创建备份目录失败：%w", err)
	}

	// 生成时间戳目录名
	timestamp := time.Now().Format("20060102_150405")
	currentBackup := filepath.Join(backupRoot, timestamp)

	// 查找上一个备份（用于硬链接）
	previousBackup, err := ib.findPreviousBackup(backupRoot)
	if err != nil {
		previousBackup = "" // 没有上一个备份，执行完整备份
	}

	// 构建 rsync 命令
	rsyncArgs := []string{
		"-av",
		"--delete",
		"--hard-links",
		"--numeric-ids",
		"--stats",
	}

	// 如果有上一个备份，使用 --link-dest 实现增量
	if previousBackup != "" {
		rsyncArgs = append(rsyncArgs, "--link-dest="+previousBackup)
	}

	rsyncArgs = append(rsyncArgs, sourceDir+"/", currentBackup)

	// 执行 rsync
	cmd := exec.Command("rsync", rsyncArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("rsync 失败：%w, output: %s", err, string(output))
	}

	// 解析 rsync 输出统计
	stats := ib.parseRsyncStats(string(output))

	// 创建备份元数据
	if err := ib.writeBackupMetadata(currentBackup, &BackupMetadata{
		Timestamp:     timestamp,
		SourceDir:     sourceDir,
		IsIncremental: previousBackup != "",
		PreviousLink:  previousBackup,
		TotalFiles:    stats.totalFiles,
		ChangedFiles:  stats.changedFiles,
		Size:          stats.totalSize,
	}); err != nil {
		return nil, fmt.Errorf("写入元数据失败：%w", err)
	}

	// 创建 latest 符号链接
	latestLink := filepath.Join(backupRoot, "latest")
	os.Remove(latestLink) // 删除旧链接
	if err := os.Symlink(currentBackup, latestLink); err != nil {
		return nil, fmt.Errorf("创建 latest 链接失败：%w", err)
	}

	duration := time.Since(startTime)

	return &BackupResult{
		BackupPath:    currentBackup,
		IsIncremental: previousBackup != "",
		TotalFiles:    stats.totalFiles,
		ChangedFiles:  stats.changedFiles,
		Size:          stats.totalSize,
		Duration:      duration,
		PreviousLink:  previousBackup,
	}, nil
}

// BackupMetadata 备份元数据
type BackupMetadata struct {
	Timestamp     string `json:"timestamp"`
	SourceDir     string `json:"sourceDir"`
	IsIncremental bool   `json:"isIncremental"`
	PreviousLink  string `json:"previousLink,omitempty"`
	TotalFiles    int64  `json:"totalFiles"`
	ChangedFiles  int64  `json:"changedFiles"`
	Size          int64  `json:"size"`
}

// writeBackupMetadata 写入备份元数据
func (ib *IncrementalBackup) writeBackupMetadata(backupPath string, meta *BackupMetadata) error {
	metaPath := filepath.Join(backupPath, ".backup-meta.json")

	// 简单的 JSON 手动序列化（避免导入 encoding/json）
	content := fmt.Sprintf(`{
  "timestamp": "%s",
  "sourceDir": "%s",
  "isIncremental": %v,
  "previousLink": "%s",
  "totalFiles": %d,
  "changedFiles": %d,
  "size": %d
}`, meta.Timestamp, meta.SourceDir, meta.IsIncremental, meta.PreviousLink, meta.TotalFiles, meta.ChangedFiles, meta.Size)

	return os.WriteFile(metaPath, []byte(content), 0644)
}

// rsyncStats rsync 统计信息
type rsyncStats struct {
	totalFiles   int64
	changedFiles int64
	totalSize    int64
}

// parseRsyncStats 解析 rsync 统计输出
func (ib *IncrementalBackup) parseRsyncStats(output string) rsyncStats {
	stats := rsyncStats{}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 解析总文件数
		if strings.Contains(line, "Number of regular files transferred:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				if val, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64); err == nil {
					stats.totalFiles = val
				}
			}
		}

		// 解析总大小
		if strings.Contains(line, "Total file size:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				sizeStr := strings.TrimSpace(parts[1])
				if val, err := ib.parseSize(sizeStr); err == nil {
					stats.totalSize = val
				}
			}
		}
	}

	// 估算变化文件数（通过检查新创建的文件）
	stats.changedFiles = stats.totalFiles // 默认假设全部变化

	return stats
}

// parseSize 解析带单位的大小字符串 (如 "1.23G", "456.78M")
func (ib *IncrementalBackup) parseSize(sizeStr string) (int64, error) {
	sizeStr = strings.ToUpper(strings.TrimSpace(sizeStr))

	var multiplier int64 = 1
	if strings.HasSuffix(sizeStr, "G") {
		multiplier = 1024 * 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "G")
	} else if strings.HasSuffix(sizeStr, "M") {
		multiplier = 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "M")
	} else if strings.HasSuffix(sizeStr, "K") {
		multiplier = 1024
		sizeStr = strings.TrimSuffix(sizeStr, "K")
	}

	var val float64
	_, err := fmt.Sscanf(sizeStr, "%f", &val)
	if err != nil {
		return 0, err
	}

	return int64(val * float64(multiplier)), nil
}

// findPreviousBackup 查找上一个备份
func (ib *IncrementalBackup) findPreviousBackup(backupRoot string) (string, error) {
	// 读取 latest 符号链接
	latestLink := filepath.Join(backupRoot, "latest")

	prevPath, err := os.Readlink(latestLink)
	if err != nil {
		return "", err // 没有上一个备份
	}

	// 如果是绝对路径，直接使用；否则拼接
	if !filepath.IsAbs(prevPath) {
		prevPath = filepath.Join(backupRoot, prevPath)
	}

	return prevPath, nil
}

// ListBackups 列出所有备份
func (ib *IncrementalBackup) ListBackups(backupName string) ([]BackupInfo, error) {
	backupRoot := filepath.Join(ib.baseDir, backupName)

	entries, err := os.ReadDir(backupRoot)
	if err != nil {
		return nil, err
	}

	var backups []BackupInfo
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "latest" {
			continue
		}

		backupPath := filepath.Join(backupRoot, entry.Name())
		meta, err := ib.readMetadata(backupPath)
		if err != nil {
			continue // 跳过没有元数据的目录
		}

		backups = append(backups, BackupInfo{
			Name:     entry.Name(),
			Path:     backupPath,
			Metadata: meta,
		})
	}

	return backups, nil
}

// BackupInfo 备份信息
type BackupInfo struct {
	Name     string
	Path     string
	Metadata *BackupMetadata
}

// readMetadata 读取备份元数据
func (ib *IncrementalBackup) readMetadata(backupPath string) (*BackupMetadata, error) {
	metaPath := filepath.Join(backupPath, ".backup-meta.json")

	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}

	// 简单的 JSON 手动解析
	meta := &BackupMetadata{}
	content := string(data)

	// 提取字段（简化解析）
	meta.Timestamp = ib.extractJSONString(content, "timestamp")
	meta.SourceDir = ib.extractJSONString(content, "sourceDir")
	meta.PreviousLink = ib.extractJSONString(content, "previousLink")
	meta.IsIncremental = strings.Contains(content, `"isIncremental": true`)
	meta.TotalFiles = ib.extractJSONInt(content, "totalFiles")
	meta.ChangedFiles = ib.extractJSONInt(content, "changedFiles")
	meta.Size = ib.extractJSONInt(content, "size")

	return meta, nil
}

// extractJSONString 提取 JSON 字符串字段
func (ib *IncrementalBackup) extractJSONString(json, key string) string {
	searchKey := fmt.Sprintf(`"%s":`, key)
	idx := strings.Index(json, searchKey)
	if idx == -1 {
		return ""
	}

	start := idx + len(searchKey)
	// 跳过空白和引号
	for start < len(json) && (json[start] == ' ' || json[start] == '"') {
		start++
	}

	end := start
	for end < len(json) && json[end] != '"' {
		if json[end] == '\\' && end+1 < len(json) {
			end += 2 // 跳过转义字符
		} else {
			end++
		}
	}

	return json[start:end]
}

// extractJSONInt 提取 JSON 整数字段
func (ib *IncrementalBackup) extractJSONInt(json, key string) int64 {
	searchKey := fmt.Sprintf(`"%s":`, key)
	idx := strings.Index(json, searchKey)
	if idx == -1 {
		return 0
	}

	start := idx + len(searchKey)
	// 跳过空白
	for start < len(json) && json[start] == ' ' {
		start++
	}

	end := start
	for end < len(json) && json[end] >= '0' && json[end] <= '9' {
		end++
	}

	var val int64
	_, _ = fmt.Sscanf(json[start:end], "%d", &val)
	return val
}

// DeleteBackup 删除备份
func (ib *IncrementalBackup) DeleteBackup(backupName, timestamp string) error {
	backupPath := filepath.Join(ib.baseDir, backupName, timestamp)

	// 检查是否有后续备份引用此备份的硬链接
	// 如果有，需要先复制实际文件

	return os.RemoveAll(backupPath)
}

// GetSpaceUsage 获取备份空间使用情况
func (ib *IncrementalBackup) GetSpaceUsage(backupName string) (int64, error) {
	backupRoot := filepath.Join(ib.baseDir, backupName)

	// 使用 du 命令获取目录大小
	cmd := exec.Command("du", "-sb", backupRoot)
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	parts := strings.Fields(string(output))
	if len(parts) < 1 {
		return 0, fmt.Errorf("du 输出格式错误")
	}

	var size int64
	_, _ = fmt.Sscanf(parts[0], "%d", &size)
	return size, nil
}
