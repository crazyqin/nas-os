package backup

import (
	"context"
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/studio-b12/gowebdav"
)

// SyncManager 双向同步管理器
type SyncManager struct {
	mu sync.RWMutex

	// 同步任务
	syncTasks map[string]*SyncTask

	// 版本管理
	versionManager *VersionManager
}

// SyncTask 同步任务
type SyncTask struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Source      string             `json:"source"`
	Destination string             `json:"destination"`
	Mode        SyncMode           `json:"mode"` // bidirectional, master-slave, one-way
	Status      TaskStatus         `json:"status"`
	LastSync    time.Time          `json:"lastSync"`
	NextSync    time.Time          `json:"nextSync"`
	Enabled     bool               `json:"enabled"`
	Schedule    string             `json:"schedule"` // cron 表达式
	Conflict    ConflictResolution `json:"conflict"` // 冲突解决策略

	// 远程目标配置
	RemoteType   RemoteType    `json:"remoteType"` // local, s3, webdav
	RemoteConfig *RemoteConfig `json:"remoteConfig,omitempty"`

	// 统计
	TotalFiles  int64 `json:"totalFiles"`
	SyncedFiles int64 `json:"syncedFiles"`
	FailedFiles int64 `json:"failedFiles"`
	TotalBytes  int64 `json:"totalBytes"`
	SyncedBytes int64 `json:"syncedBytes"`
}

// SyncMode 同步模式
type SyncMode string

const (
	SyncModeBidirectional SyncMode = "bidirectional" // 双向同步
	SyncModeMasterSlave   SyncMode = "master-slave"  // 主从同步
	SyncModeOneWay        SyncMode = "one-way"       // 单向同步
)

// ConflictResolution 冲突解决策略
type ConflictResolution string

const (
	ConflictLatest   ConflictResolution = "latest"    // 最新者胜
	ConflictSource   ConflictResolution = "source"    // 源优先
	ConflictDest     ConflictResolution = "dest"      // 目标优先
	ConflictKeepBoth ConflictResolution = "keep-both" // 保留两者
	ConflictManual   ConflictResolution = "manual"    // 手动解决
)

// RemoteType 远程类型
type RemoteType string

const (
	RemoteTypeLocal  RemoteType = "local"
	RemoteTypeS3     RemoteType = "s3"
	RemoteTypeWebDAV RemoteType = "webdav"
)

// RemoteConfig 远程配置
type RemoteConfig struct {
	// S3 配置
	Bucket    string `json:"bucket,omitempty"`
	Region    string `json:"region,omitempty"`
	Endpoint  string `json:"endpoint,omitempty"`
	AccessKey string `json:"accessKey,omitempty"`
	SecretKey string `json:"secretKey,omitempty"`

	// WebDAV 配置
	URL      string `json:"url,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`

	// 通用
	Insecure bool   `json:"insecure,omitempty"` // 跳过 TLS 验证
	Prefix   string `json:"prefix,omitempty"`   // 路径前缀
}

// VersionManager 版本管理器
type VersionManager struct {
	baseDir string
}

// VersionInfo 版本信息
type VersionInfo struct {
	VersionID string    `json:"versionId"`
	FilePath  string    `json:"filePath"`
	Size      int64     `json:"size"`
	Checksum  string    `json:"checksum"`
	CreatedAt time.Time `json:"createdAt"`
	ParentID  string    `json:"parentId,omitempty"`
}

// NewSyncManager 创建同步管理器
func NewSyncManager(baseDir string) *SyncManager {
	return &SyncManager{
		syncTasks:      make(map[string]*SyncTask),
		versionManager: NewVersionManager(filepath.Join(baseDir, "versions")),
	}
}

// NewVersionManager 创建版本管理器
func NewVersionManager(baseDir string) *VersionManager {
	os.MkdirAll(baseDir, 0755)
	return &VersionManager{baseDir: baseDir}
}

// ========== 同步任务管理 ==========

// CreateSyncTask 创建同步任务
func (sm *SyncManager) CreateSyncTask(task SyncTask) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if task.ID == "" {
		task.ID = generateID()
	}

	if task.Name == "" {
		return fmt.Errorf("同步任务名称不能为空")
	}

	if task.Source == "" {
		return fmt.Errorf("源路径不能为空")
	}

	if task.Destination == "" && task.RemoteConfig == nil {
		return fmt.Errorf("目标路径或远程配置不能为空")
	}

	if task.Mode == "" {
		task.Mode = SyncModeOneWay
	}

	if task.Conflict == "" {
		task.Conflict = ConflictLatest
	}

	sm.syncTasks[task.ID] = &task
	return nil
}

// ListSyncTasks 列出所有同步任务
func (sm *SyncManager) ListSyncTasks() []*SyncTask {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var tasks []*SyncTask
	for _, task := range sm.syncTasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// GetSyncTask 获取同步任务
func (sm *SyncManager) GetSyncTask(id string) (*SyncTask, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	task, ok := sm.syncTasks[id]
	if !ok {
		return nil, fmt.Errorf("同步任务不存在：%s", id)
	}
	return task, nil
}

// DeleteSyncTask 删除同步任务
func (sm *SyncManager) DeleteSyncTask(id string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, ok := sm.syncTasks[id]; !ok {
		return fmt.Errorf("同步任务不存在：%s", id)
	}

	delete(sm.syncTasks, id)
	return nil
}

// RunSync 执行同步任务
func (sm *SyncManager) RunSync(taskID string) error {
	sm.mu.Lock()
	task, ok := sm.syncTasks[taskID]
	if !ok {
		sm.mu.Unlock()
		return fmt.Errorf("同步任务不存在：%s", taskID)
	}

	task.Status = TaskStatusRunning
	task.LastSync = time.Now()
	sm.mu.Unlock()

	go sm.executeSync(task)
	return nil
}

// executeSync 执行同步逻辑
func (sm *SyncManager) executeSync(task *SyncTask) {
	defer func() {
		sm.mu.Lock()
		if task.Status == TaskStatusRunning {
			task.Status = TaskStatusCompleted
		}
		sm.mu.Unlock()
	}()

	var err error

	switch task.RemoteType {
	case RemoteTypeLocal:
		err = sm.syncLocal(task)
	case RemoteTypeS3:
		err = sm.syncToS3(task)
	case RemoteTypeWebDAV:
		err = sm.syncToWebDAV(task)
	default:
		err = fmt.Errorf("不支持的远程类型：%s", task.RemoteType)
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	if err != nil {
		task.Status = TaskStatusFailed
		fmt.Printf("同步任务失败 [%s]: %v\n", task.Name, err)
	} else {
		task.Status = TaskStatusCompleted
		task.NextSync = sm.calculateNextSync(task.Schedule)
	}
}

// syncLocal 本地同步（使用 rsync）
func (sm *SyncManager) syncLocal(task *SyncTask) error {
	args := []string{"-avz", "--progress", "--delete"}

	// 排除临时文件和系统文件
	excludes := []string{
		".DS_Store",
		"Thumbs.db",
		".tmp",
		".swp",
		"*~",
	}

	for _, exclude := range excludes {
		args = append(args, "--exclude", exclude)
	}

	// 根据同步模式调整参数
	switch task.Mode {
	case SyncModeBidirectional:
		// 双向同步需要特殊处理
		return sm.syncBidirectional(task)
	case SyncModeMasterSlave:
		// 主从同步，源为主
		args = append(args, "--delete")
	case SyncModeOneWay:
		// 单向同步，不删除目标
	}

	source := task.Source
	dest := task.Destination

	// 确保源路径以 / 结尾（rsync 语义）
	if !strings.HasSuffix(source, "/") {
		source = source + "/"
	}

	args = append(args, source, dest)

	cmd := exec.Command("rsync", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rsync 失败：%w, output: %s", err, string(output))
	}

	// 解析输出，更新统计
	sm.parseRsyncOutput(task, string(output))

	return nil
}

// syncBidirectional 双向同步
func (sm *SyncManager) syncBidirectional(task *SyncTask) error {
	// 1. 扫描两边文件列表
	sourceFiles, err := sm.scanDirectory(task.Source)
	if err != nil {
		return fmt.Errorf("扫描源目录失败：%w", err)
	}

	destFiles, err := sm.scanDirectory(task.Destination)
	if err != nil {
		return fmt.Errorf("扫描目标目录失败：%w", err)
	}

	// 2. 比较文件差异
	changes := sm.compareFiles(sourceFiles, destFiles, task.Conflict)

	// 3. 执行同步
	var syncedCount, failedCount int64
	var syncedBytes int64

	for _, change := range changes {
		if err := sm.applyChange(change, task); err != nil {
			failedCount++
			fmt.Printf("同步文件失败 [%s]: %v\n", change.Path, err)
		} else {
			syncedCount++
			syncedBytes += change.Size
		}
	}

	task.SyncedFiles = syncedCount
	task.FailedFiles = failedCount
	task.SyncedBytes = syncedBytes
	task.TotalFiles = int64(len(changes))

	return nil
}

// FileEntry 文件条目
type FileEntry struct {
	Path     string
	Size     int64
	ModTime  time.Time
	Checksum string
}

// scanDirectory 扫描目录
func (sm *SyncManager) scanDirectory(dir string) (map[string]FileEntry, error) {
	files := make(map[string]FileEntry)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		// 计算文件 checksum（简化：使用 md5）
		checksum, err := sm.calculateFileChecksum(path)
		if err != nil {
			checksum = ""
		}

		files[relPath] = FileEntry{
			Path:     relPath,
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			Checksum: checksum,
		}

		return nil
	})

	return files, err
}

// calculateFileChecksum 计算文件 checksum
func (sm *SyncManager) calculateFileChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// FileChange 文件变更
type FileChange struct {
	Path   string
	Type   string // new, modified, deleted, conflict
	Size   int64
	Source FileEntry
	Dest   FileEntry
	Action string // copy-src-to-dest, copy-dest-to-src, keep-both, skip
}

// compareFiles 比较文件差异
func (sm *SyncManager) compareFiles(sourceFiles, destFiles map[string]FileEntry, conflict ConflictResolution) []FileChange {
	var changes []FileChange

	// 检查源目录中的文件
	for path, srcEntry := range sourceFiles {
		destEntry, exists := destFiles[path]

		if !exists {
			// 源有，目标没有 -> 复制到目标
			changes = append(changes, FileChange{
				Path:   path,
				Type:   "new",
				Size:   srcEntry.Size,
				Source: srcEntry,
				Action: "copy-src-to-dest",
			})
		} else if srcEntry.Checksum != "" && destEntry.Checksum != "" && srcEntry.Checksum != destEntry.Checksum {
			// 文件不同 -> 冲突
			changes = append(changes, FileChange{
				Path:   path,
				Type:   "conflict",
				Size:   srcEntry.Size,
				Source: srcEntry,
				Dest:   destEntry,
				Action: sm.resolveConflict(srcEntry, destEntry, conflict),
			})
		}
	}

	// 检查目标目录中独有的文件（需要反向同步）
	for path, destEntry := range destFiles {
		if _, exists := sourceFiles[path]; !exists {
			changes = append(changes, FileChange{
				Path:   path,
				Type:   "new",
				Size:   destEntry.Size,
				Dest:   destEntry,
				Action: "copy-dest-to-src",
			})
		}
	}

	return changes
}

// resolveConflict 解决冲突
func (sm *SyncManager) resolveConflict(src, dest FileEntry, conflict ConflictResolution) string {
	switch conflict {
	case ConflictLatest:
		if src.ModTime.After(dest.ModTime) {
			return "copy-src-to-dest"
		}
		return "copy-dest-to-src"
	case ConflictSource:
		return "copy-src-to-dest"
	case ConflictDest:
		return "copy-dest-to-src"
	case ConflictKeepBoth:
		return "keep-both"
	default:
		return "skip"
	}
}

// applyChange 应用变更
func (sm *SyncManager) applyChange(change FileChange, task *SyncTask) error {
	switch change.Action {
	case "copy-src-to-dest":
		srcPath := filepath.Join(task.Source, change.Path)
		destPath := filepath.Join(task.Destination, change.Path)

		// 创建版本备份
		if _, err := os.Stat(destPath); err == nil {
			sm.versionManager.CreateVersion(destPath, change.Path)
		}

		return sm.copyFile(srcPath, destPath)

	case "copy-dest-to-src":
		srcPath := filepath.Join(task.Destination, change.Path)
		destPath := filepath.Join(task.Source, change.Path)

		// 创建版本备份
		if _, err := os.Stat(destPath); err == nil {
			sm.versionManager.CreateVersion(destPath, change.Path)
		}

		return sm.copyFile(srcPath, destPath)

	case "keep-both":
		// 保留两者，重命名冲突文件
		timestamp := time.Now().Format("20060102_150405")
		srcPath := filepath.Join(task.Source, change.Path)
		destPath := filepath.Join(task.Destination, change.Path)

		// 重命名目标文件
		dir := filepath.Dir(destPath)
		base := filepath.Base(destPath)
		ext := filepath.Ext(base)
		name := strings.TrimSuffix(base, ext)
		newName := fmt.Sprintf("%s_conflict_%s%s", name, timestamp, ext)
		newPath := filepath.Join(dir, newName)

		if err := os.Rename(destPath, newPath); err != nil {
			return err
		}

		return sm.copyFile(srcPath, destPath)

	default:
		return nil
	}
}

// copyFile 复制文件
func (sm *SyncManager) copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// syncToS3 同步到 S3
func (sm *SyncManager) syncToS3(task *SyncTask) error {
	cfg := task.RemoteConfig
	if cfg == nil {
		return fmt.Errorf("S3 配置为空")
	}

	// 创建 S3 客户端
	s3Client, err := sm.createS3Client(cfg)
	if err != nil {
		return err
	}

	// 扫描本地文件
	files, err := sm.scanDirectory(task.Source)
	if err != nil {
		return err
	}

	var syncedCount, failedCount int64
	var syncedBytes int64

	for path, entry := range files {
		localPath := filepath.Join(task.Source, path)
		s3Key := strings.TrimPrefix(path, "/")
		if cfg.Prefix != "" {
			s3Key = strings.TrimSuffix(cfg.Prefix, "/") + "/" + s3Key
		}

		if err := sm.uploadToS3(s3Client, cfg.Bucket, localPath, s3Key); err != nil {
			failedCount++
			fmt.Printf("上传到 S3 失败 [%s]: %v\n", path, err)
		} else {
			syncedCount++
			syncedBytes += entry.Size
		}
	}

	task.SyncedFiles = syncedCount
	task.FailedFiles = failedCount
	task.SyncedBytes = syncedBytes
	task.TotalFiles = int64(len(files))

	return nil
}

// createS3Client 创建 S3 客户端
func (sm *SyncManager) createS3Client(cfg *RemoteConfig) (*s3.Client, error) {
	// 使用静态凭证
	creds := aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
		return aws.Credentials{
			AccessKeyID:     cfg.AccessKey,
			SecretAccessKey: cfg.SecretKey,
		}, nil
	})

	s3Config := aws.Config{
		Credentials: creds,
		Region:      cfg.Region,
	}

	client := s3.NewFromConfig(s3Config, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		}
	})

	return client, nil
}

// uploadToS3 上传到 S3
func (sm *SyncManager) uploadToS3(client *s3.Client, bucket, localPath, key string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   file,
	})

	return err
}

// syncToWebDAV 同步到 WebDAV
func (sm *SyncManager) syncToWebDAV(task *SyncTask) error {
	cfg := task.RemoteConfig
	if cfg == nil {
		return fmt.Errorf("WebDAV 配置为空")
	}

	// 创建 WebDAV 客户端
	client := gowebdav.NewClient(cfg.URL, cfg.Username, cfg.Password)

	if cfg.Insecure {
		client.SetTransport(&http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		})
	}

	// 扫描本地文件
	files, err := sm.scanDirectory(task.Source)
	if err != nil {
		return err
	}

	var syncedCount, failedCount int64
	var syncedBytes int64

	for path, entry := range files {
		localPath := filepath.Join(task.Source, path)
		remotePath := path
		if cfg.Prefix != "" {
			remotePath = strings.TrimSuffix(cfg.Prefix, "/") + "/" + remotePath
		}

		if err := sm.uploadToWebDAV(client, localPath, remotePath); err != nil {
			failedCount++
			fmt.Printf("上传到 WebDAV 失败 [%s]: %v\n", path, err)
		} else {
			syncedCount++
			syncedBytes += entry.Size
		}
	}

	task.SyncedFiles = syncedCount
	task.FailedFiles = failedCount
	task.SyncedBytes = syncedBytes
	task.TotalFiles = int64(len(files))

	return nil
}

// uploadToWebDAV 上传到 WebDAV
func (sm *SyncManager) uploadToWebDAV(client *gowebdav.Client, localPath, remotePath string) error {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return err
	}

	remoteDir := filepath.Dir(remotePath)
	if err := client.MkdirAll(remoteDir, 0755); err != nil {
		return err
	}

	return client.Write(remotePath, data, 0644)
}

// parseRsyncOutput 解析 rsync 输出
func (sm *SyncManager) parseRsyncOutput(task *SyncTask, output string) {
	// 简化处理，实际应解析详细统计
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "sent") && strings.Contains(line, "received") {
			// 解析传输统计
			break
		}
	}
}

// calculateNextSync 计算下次同步时间
func (sm *SyncManager) calculateNextSync(schedule string) time.Time {
	if schedule == "" {
		return time.Time{}
	}

	// 简化：如果是 cron 表达式，解析并计算下次时间
	// 这里简化处理，返回 1 小时后
	return time.Now().Add(time.Hour)
}

// ========== 版本管理 ==========

// CreateVersion 创建文件版本
func (vm *VersionManager) CreateVersion(filePath, relativePath string) (*VersionInfo, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	// 生成版本 ID
	versionID := fmt.Sprintf("v%d_%s", time.Now().UnixNano(), filepath.Base(filePath))

	// 版本存储路径
	versionDir := filepath.Join(vm.baseDir, filepath.Dir(relativePath))
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		return nil, err
	}

	versionPath := filepath.Join(versionDir, versionID)

	// 复制文件到版本目录
	if err := copyFile(filePath, versionPath); err != nil {
		return nil, err
	}

	// 计算 checksum
	checksum, _ := vm.calculateChecksum(versionPath)

	versionInfo := &VersionInfo{
		VersionID: versionID,
		FilePath:  relativePath,
		Size:      info.Size(),
		Checksum:  checksum,
		CreatedAt: time.Now(),
	}

	// 保存元数据
	if err := vm.saveVersionMetadata(versionInfo); err != nil {
		return nil, err
	}

	return versionInfo, nil
}

// ListVersions 列出文件的所有版本
func (vm *VersionManager) ListVersions(relativePath string) ([]VersionInfo, error) {
	versionDir := filepath.Join(vm.baseDir, filepath.Dir(relativePath))

	entries, err := os.ReadDir(versionDir)
	if err != nil {
		return nil, err
	}

	var versions []VersionInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		metaPath := filepath.Join(versionDir, entry.Name()+".meta")
		if meta, err := vm.loadVersionMetadata(metaPath); err == nil {
			versions = append(versions, *meta)
		}
	}

	return versions, nil
}

// RestoreVersion 恢复指定版本
func (vm *VersionManager) RestoreVersion(versionID, targetPath string) error {
	versionPath := filepath.Join(vm.baseDir, versionID)

	if _, err := os.Stat(versionPath); os.IsNotExist(err) {
		return fmt.Errorf("版本不存在：%s", versionID)
	}

	return copyFile(versionPath, targetPath)
}

// DeleteOldVersions 删除旧版本（保留最近 N 个）
func (vm *VersionManager) DeleteOldVersions(relativePath string, keepCount int) error {
	versions, err := vm.ListVersions(relativePath)
	if err != nil {
		return err
	}

	if len(versions) <= keepCount {
		return nil
	}

	// 按时间排序，删除旧版本
	for i := 0; i < len(versions)-keepCount; i++ {
		versionPath := filepath.Join(vm.baseDir, versions[i].VersionID)
		os.Remove(versionPath)
		os.Remove(versionPath + ".meta")
	}

	return nil
}

// calculateChecksum 计算文件 checksum
func (vm *VersionManager) calculateChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// saveVersionMetadata 保存版本元数据
func (vm *VersionManager) saveVersionMetadata(version *VersionInfo) error {
	metaPath := filepath.Join(vm.baseDir, version.FilePath, version.VersionID+".meta")

	content := fmt.Sprintf(`{
  "versionId": "%s",
  "filePath": "%s",
  "size": %d,
  "checksum": "%s",
  "createdAt": "%s"
}`, version.VersionID, version.FilePath, version.Size, version.Checksum, version.CreatedAt.Format(time.RFC3339))

	return os.WriteFile(metaPath, []byte(content), 0644)
}

// loadVersionMetadata 加载版本元数据
func (vm *VersionManager) loadVersionMetadata(path string) (*VersionInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// 简化 JSON 解析
	version := &VersionInfo{}
	content := string(data)

	version.VersionID = extractJSONString(content, "versionId")
	version.FilePath = extractJSONString(content, "filePath")
	version.Checksum = extractJSONString(content, "checksum")
	version.Size = extractJSONInt(content, "size")

	if createdAt := extractJSONString(content, "createdAt"); createdAt != "" {
		version.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	}

	return version, nil
}

// copyFile 复制文件（包级辅助函数）
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// extractJSONString 提取 JSON 字符串字段（辅助函数）
func extractJSONString(json, key string) string {
	searchKey := fmt.Sprintf(`"%s":`, key)
	idx := strings.Index(json, searchKey)
	if idx == -1 {
		return ""
	}

	start := idx + len(searchKey)
	for start < len(json) && (json[start] == ' ' || json[start] == '"') {
		start++
	}

	end := start
	for end < len(json) && json[end] != '"' {
		if json[end] == '\\' && end+1 < len(json) {
			end += 2
		} else {
			end++
		}
	}

	return json[start:end]
}

// extractJSONInt 提取 JSON 整数字段（辅助函数）
func extractJSONInt(json, key string) int64 {
	searchKey := fmt.Sprintf(`"%s":`, key)
	idx := strings.Index(json, searchKey)
	if idx == -1 {
		return 0
	}

	start := idx + len(searchKey)
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
