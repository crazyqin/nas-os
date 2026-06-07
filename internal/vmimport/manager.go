// Package vmimport 提供虚拟机镜像导入导出功能
package vmimport

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ========== 配置常量 ==========

const (
	// DefaultStoragePath 默认存储路径.
	DefaultStoragePath = "/var/lib/nas-os/vm/images"
	// DefaultMetadataPath 默认元数据路径.
	DefaultMetadataPath = "/var/lib/nas-os/vm/metadata"
	// ProgressUpdateInterval 进度更新间隔.
	ProgressUpdateInterval = 500 * time.Millisecond
	// MaxConcurrentImports 最大并发导入数.
	MaxConcurrentImports = 3
)

// Manager 虚拟机导入导出管理器.
type Manager struct {
	mu           sync.RWMutex
	imports      map[string]*ImportTask        // importID -> ImportTask
	exports      map[string]*ExportTask        // exportID -> ExportTask
	images       map[string]*VMImage           // imageID -> VMImage
	storagePath  string                        // 镜像存储路径
	metadataPath string                        // 元数据存储路径
	importSem    chan struct{}                 // 导入并发控制
	cancelFuncs  map[string]context.CancelFunc // importID -> cancel func
	exportCancel map[string]context.CancelFunc // exportID -> cancel func
}

// NewManager 创建虚拟机导入导出管理器.
func NewManager(storagePath, metadataPath string) (*Manager, error) {
	if storagePath == "" {
		storagePath = DefaultStoragePath
	}
	if metadataPath == "" {
		metadataPath = DefaultMetadataPath
	}

	// 确保目录存在.
	if err := os.MkdirAll(storagePath, 0o755); err != nil {
		return nil, fmt.Errorf("创建存储目录失败: %w", err)
	}
	if err := os.MkdirAll(metadataPath, 0o755); err != nil {
		return nil, fmt.Errorf("创建元数据目录失败: %w", err)
	}

	m := &Manager{
		imports:      make(map[string]*ImportTask),
		exports:      make(map[string]*ExportTask),
		images:       make(map[string]*VMImage),
		storagePath:  storagePath,
		metadataPath: metadataPath,
		importSem:    make(chan struct{}, MaxConcurrentImports),
		cancelFuncs:  make(map[string]context.CancelFunc),
		exportCancel: make(map[string]context.CancelFunc),
	}

	// 加载已有的镜像元数据.
	if err := m.loadImages(); err != nil {
		return nil, fmt.Errorf("加载镜像元数据失败: %w", err)
	}

	return m, nil
}

// ========== 生成ID ==========

// generateID 生成唯一ID.
func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// 回退到时间戳.
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// ========== 导入功能 ==========

// StartImport 启动导入任务.
func (m *Manager) StartImport(req ImportRequest) (*ImportTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查目标格式.
	if req.TargetFormat == "" {
		req.TargetFormat = FormatQCOW2
	}
	if !isSupportedFormat(req.TargetFormat) {
		return nil, ErrUnsupportedFormat
	}

	// 检查来源类型.
	if req.SourceType != "file" && req.SourceType != "url" {
		return nil, fmt.Errorf("不支持的来源类型: %s", req.SourceType)
	}

	// 检查文件来源.
	if req.SourceType == "file" {
		if _, err := os.Stat(req.Source); err != nil {
			return nil, fmt.Errorf("源文件不存在: %w", err)
		}
	}

	task := &ImportTask{
		ID:           generateID(),
		Source:       req.Source,
		SourceType:   req.SourceType,
		TargetName:   req.TargetName,
		TargetFormat: req.TargetFormat,
		Status:       StatusPending,
		Progress:     0,
		CreateVM:     req.CreateVM,
		VMName:       req.VMName,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		cancelCh:     make(chan struct{}),
	}

	m.imports[task.ID] = task

	// 异步执行导入.
	go m.executeImport(task)

	return task, nil
}

// executeImport 执行导入任务.
func (m *Manager) executeImport(task *ImportTask) {
	// 获取并发信号.
	m.importSem <- struct{}{}
	defer func() { <-m.importSem }()

	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.cancelFuncs[task.ID] = cancel
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.cancelFuncs, task.ID)
		m.mu.Unlock()
		cancel()
	}()

	// 更新状态为运行中.
	m.updateTaskStatus(task.ID, StatusRunning, 0, "")

	var (
		sourcePath string
		err        error
	)

	// 如果是URL，先下载.
	if task.SourceType == "url" {
		m.updateTaskStatus(task.ID, StatusRunning, 5, "")
		sourcePath, err = m.downloadFile(ctx, task)
		if err != nil {
			m.updateTaskStatus(task.ID, StatusFailed, 0, fmt.Sprintf("下载失败: %v", err))
			return
		}
		defer os.Remove(sourcePath) //nolint:errcheck
	} else {
		sourcePath = task.Source
	}

	// 检测源格式.
	m.updateTaskStatus(task.ID, StatusRunning, 10, "")
	sourceFormat, err := DetectFormat(sourcePath)
	if err != nil {
		m.updateTaskStatus(task.ID, StatusFailed, 0, fmt.Sprintf("格式检测失败: %v", err))
		return
	}

	m.mu.Lock()
	m.imports[task.ID].SourceFormat = sourceFormat
	m.mu.Unlock()

	// 获取文件大小.
	info, err := os.Stat(sourcePath)
	if err != nil {
		m.updateTaskStatus(task.ID, StatusFailed, 0, fmt.Sprintf("获取文件信息失败: %v", err))
		return
	}

	m.mu.Lock()
	m.imports[task.ID].TotalSize = info.Size()
	m.mu.Unlock()

	// 转换格式.
	targetPath := filepath.Join(m.storagePath, task.TargetName+"."+string(task.TargetFormat))
	m.updateTaskStatus(task.ID, StatusRunning, 20, "")

	if sourceFormat == task.TargetFormat {
		// 格式相同，直接复制.
		err = m.copyFile(ctx, task, sourcePath, targetPath)
	} else {
		// 格式不同，需要转换.
		err = ConvertImage(ctx, sourcePath, targetPath, task.TargetFormat, func(progress float64) {
			m.updateTaskStatus(task.ID, StatusRunning, 20+progress*0.6, "")
		})
	}

	if err != nil {
		m.updateTaskStatus(task.ID, StatusFailed, 0, fmt.Sprintf("转换失败: %v", err))
		return
	}

	// 检查取消.
	select {
	case <-task.cancelCh:
		os.Remove(targetPath) //nolint:errcheck
		m.updateTaskStatus(task.ID, StatusCancelled, m.getTaskProgress(task.ID), "")
		return
	default:
	}

	// 计算校验和.
	m.updateTaskStatus(task.ID, StatusRunning, 85, "")
	checksum, err := computeChecksum(targetPath)
	if err != nil {
		m.updateTaskStatus(task.ID, StatusFailed, 0, fmt.Sprintf("计算校验和失败: %v", err))
		return
	}

	// 获取目标文件大小.
	targetInfo, err := os.Stat(targetPath)
	if err != nil {
		m.updateTaskStatus(task.ID, StatusFailed, 0, fmt.Sprintf("获取目标文件信息失败: %v", err))
		return
	}

	// 获取虚拟磁盘大小.
	virtualSize, err := GetVirtualSize(targetPath)
	if err != nil {
		virtualSize = targetInfo.Size()
	}

	// 创建镜像记录.
	m.updateTaskStatus(task.ID, StatusRunning, 95, "")
	image := &VMImage{
		ID:             generateID(),
		Name:           task.TargetName,
		Format:         task.TargetFormat,
		FilePath:       targetPath,
		FileSize:       targetInfo.Size(),
		VirtualSize:    virtualSize,
		Checksum:       checksum,
		SourceImportID: task.ID,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	m.mu.Lock()
	m.images[image.ID] = image
	m.imports[task.ID].ImageID = image.ID
	m.mu.Unlock()

	// 保存元数据.
	if err := m.saveImageMetadata(image); err != nil {
		// 元数据保存失败不影响导入结果，仅记录日志.
		fmt.Printf("保存镜像元数据失败: %v\n", err)
	}

	// 完成.
	now := time.Now()
	m.mu.Lock()
	if t, ok := m.imports[task.ID]; ok {
		t.Status = StatusCompleted
		t.Progress = 100
		t.ProcessedSize = targetInfo.Size()
		t.UpdatedAt = now
		t.CompletedAt = &now
	}
	m.mu.Unlock()
}

// downloadFile 下载URL文件.
func (m *Manager) downloadFile(ctx context.Context, task *ImportTask) (string, error) {
	tmpPath := filepath.Join(m.storagePath, ".tmp", task.ID+".tmp")
	if err := os.MkdirAll(filepath.Dir(tmpPath), 0o755); err != nil {
		return "", fmt.Errorf("创建临时目录失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, task.Source, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载失败，状态码: %d", resp.StatusCode)
	}

	m.mu.Lock()
	m.imports[task.ID].TotalSize = resp.ContentLength
	m.mu.Unlock()

	f, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	defer writer.Flush()

	buf := make([]byte, 32*1024) // 32KB 缓冲区
	var downloaded int64

	for {
		select {
		case <-task.cancelCh:
			return "", fmt.Errorf("任务已取消")
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := writer.Write(buf[:n]); writeErr != nil {
				return "", fmt.Errorf("写入文件失败: %w", writeErr)
			}
			downloaded += int64(n)

			if resp.ContentLength > 0 {
				progress := float64(downloaded) / float64(resp.ContentLength) * 100
				m.updateTaskStatus(task.ID, StatusRunning, progress*0.15+5, "")
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return "", fmt.Errorf("读取响应失败: %w", readErr)
		}
	}

	return tmpPath, nil
}

// copyFile 复制文件.
func (m *Manager) copyFile(ctx context.Context, task *ImportTask, src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("打开源文件失败: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("创建目标文件失败: %w", err)
	}
	defer dstFile.Close()

	buf := make([]byte, 32*1024)
	var copied int64

	info, _ := os.Stat(src)
	totalSize := int64(0)
	if info != nil {
		totalSize = info.Size()
	}

	for {
		select {
		case <-task.cancelCh:
			os.Remove(dst) //nolint:errcheck
			return fmt.Errorf("任务已取消")
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, readErr := srcFile.Read(buf)
		if n > 0 {
			if _, writeErr := dstFile.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("写入文件失败: %w", writeErr)
			}
			copied += int64(n)

			if totalSize > 0 {
				progress := float64(copied) / float64(totalSize) * 60
				m.updateTaskStatus(task.ID, StatusRunning, 20+progress, "")
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return fmt.Errorf("读取文件失败: %w", readErr)
		}
	}

	return nil
}

// ========== 导出功能 ==========

// StartExport 启动导出任务.
func (m *Manager) StartExport(req ExportRequest) (*ExportTask, error) {
	m.mu.RLock()
	image, ok := m.images[req.ImageID]
	m.mu.RUnlock()

	if !ok {
		return nil, ErrImageNotFound
	}

	// 检查目标格式.
	if req.TargetFormat == "" {
		req.TargetFormat = image.Format
	}
	if !isSupportedFormat(req.TargetFormat) {
		return nil, ErrUnsupportedFormat
	}

	// 检查压缩格式.
	if req.Compress == "" {
		req.Compress = CompressNone
	}

	// 设置输出路径.
	outputPath := req.OutputPath
	if outputPath == "" {
		ext := string(req.TargetFormat)
		if req.Compress == CompressGzip {
			ext += ".gz"
		} else if req.Compress == CompressZstd {
			ext += ".zst"
		}
		outputPath = filepath.Join(m.storagePath, "exports", image.Name+"."+ext)
	}

	task := &ExportTask{
		ID:           generateID(),
		ImageID:      req.ImageID,
		ImageName:    image.Name,
		TargetFormat: req.TargetFormat,
		Compress:     req.Compress,
		OutputPath:   outputPath,
		Status:       StatusPending,
		Progress:     0,
		TotalSize:    image.FileSize,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		cancelCh:     make(chan struct{}),
	}

	m.mu.Lock()
	m.exports[task.ID] = task
	m.mu.Unlock()

	// 异步执行导出.
	go m.executeExport(task, image)

	return task, nil
}

// executeExport 执行导出任务.
func (m *Manager) executeExport(task *ExportTask, image *VMImage) {
	ctx, cancel := context.WithCancel(context.Background())

	m.mu.Lock()
	m.exportCancel[task.ID] = cancel
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.exportCancel, task.ID)
		m.mu.Unlock()
		cancel()
	}()

	// 更新状态为运行中.
	m.updateExportStatus(task.ID, StatusRunning, 0, "")

	// 确保输出目录存在.
	if err := os.MkdirAll(filepath.Dir(task.OutputPath), 0o755); err != nil {
		m.updateExportStatus(task.ID, StatusFailed, 0, fmt.Sprintf("创建输出目录失败: %v", err))
		return
	}

	// 如果格式不同，先转换.
	sourcePath := image.FilePath
	var err error

	if image.Format != task.TargetFormat {
		tmpPath := filepath.Join(m.storagePath, ".tmp", "export-"+task.ID+"."+string(task.TargetFormat))
		err = ConvertImage(ctx, sourcePath, tmpPath, task.TargetFormat, func(progress float64) {
			m.updateExportStatus(task.ID, StatusRunning, progress*0.7, "")
		})
		if err != nil {
			m.updateExportStatus(task.ID, StatusFailed, 0, fmt.Sprintf("格式转换失败: %v", err))
			return
		}
		sourcePath = tmpPath
		defer os.Remove(tmpPath) //nolint:errcheck
	}

	// 检查取消.
	select {
	case <-task.cancelCh:
		m.updateExportStatus(task.ID, StatusCancelled, m.getExportProgress(task.ID), "")
		return
	default:
	}

	// 复制或压缩到输出路径.
	m.updateExportStatus(task.ID, StatusRunning, 70, "")

	switch task.Compress {
	case CompressGzip:
		err = compressGzip(ctx, task, sourcePath, task.OutputPath)
	case CompressZstd:
		err = compressZstd(ctx, task, sourcePath, task.OutputPath)
	default:
		err = copyExportFile(ctx, task, sourcePath, task.OutputPath)
	}

	if err != nil {
		m.updateExportStatus(task.ID, StatusFailed, 0, fmt.Sprintf("导出失败: %v", err))
		return
	}

	// 完成.
	now := time.Now()
	m.mu.Lock()
	if t, ok := m.exports[task.ID]; ok {
		t.Status = StatusCompleted
		t.Progress = 100
		t.UpdatedAt = now
		t.CompletedAt = &now
	}
	m.mu.Unlock()
}

// compressGzip gzip压缩.
func compressGzip(ctx context.Context, task *ExportTask, src, dst string) error {
	// 使用 gzip 命令压缩.
	//nolint:gosec
	cmd := exec.CommandContext(ctx, "gzip", "-c", src)
	outFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("创建输出文件失败: %w", err)
	}
	defer outFile.Close()

	cmd.Stdout = outFile

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gzip压缩失败: %w", err)
	}

	return nil
}

// compressZstd zstd压缩.
func compressZstd(ctx context.Context, task *ExportTask, src, dst string) error {
	//nolint:gosec
	cmd := exec.CommandContext(ctx, "zstd", "-o", dst, src, "-f")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("zstd压缩失败: %w", err)
	}

	return nil
}

// copyExportFile 复制导出文件.
func copyExportFile(ctx context.Context, task *ExportTask, src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("打开源文件失败: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("创建目标文件失败: %w", err)
	}
	defer dstFile.Close()

	buf := make([]byte, 32*1024)
	var copied int64

	for {
		select {
		case <-task.cancelCh:
			return fmt.Errorf("任务已取消")
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, readErr := srcFile.Read(buf)
		if n > 0 {
			if _, writeErr := dstFile.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("写入文件失败: %w", writeErr)
			}
			copied += int64(n)

			if task.TotalSize > 0 {
				progress := float64(copied) / float64(task.TotalSize) * 25
				// 此处的进度从70%开始，总计100%.
				_ = progress
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return fmt.Errorf("读取文件失败: %w", readErr)
		}
	}

	return nil
}

// ========== 取消任务 ==========

// CancelImport 取消导入任务.
func (m *Manager) CancelImport(id string) error {
	m.mu.RLock()
	task, ok := m.imports[id]
	cancelFn := m.cancelFuncs[id]
	m.mu.RUnlock()

	if !ok {
		return ErrImportNotFound
	}

	if task.Status != StatusPending && task.Status != StatusRunning {
		return ErrTaskNotRunning
	}

	// 发送取消信号.
	close(task.cancelCh)

	// 调用context取消.
	if cancelFn != nil {
		cancelFn()
	}

	return nil
}

// CancelExport 取消导出任务.
func (m *Manager) CancelExport(id string) error {
	m.mu.RLock()
	task, ok := m.exports[id]
	m.mu.RUnlock()

	if !ok {
		return ErrExportNotFound
	}

	if task.Status != StatusPending && task.Status != StatusRunning {
		return ErrTaskNotRunning
	}

	close(task.cancelCh)

	m.mu.RLock()
	cancelFn := m.exportCancel[id]
	m.mu.RUnlock()

	if cancelFn != nil {
		cancelFn()
	}

	return nil
}

// ========== 查询功能 ==========

// GetImportStatus 获取导入状态.
func (m *Manager) GetImportStatus(id string) (*ImportTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.imports[id]
	if !ok {
		return nil, ErrImportNotFound
	}

	// 返回副本.
	copy := *task
	return &copy, nil
}

// GetExportStatus 获取导出状态.
func (m *Manager) GetExportStatus(id string) (*ExportTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.exports[id]
	if !ok {
		return nil, ErrExportNotFound
	}

	copy := *task
	return &copy, nil
}

// GetSupportedFormats 获取支持的格式列表.
func (m *Manager) GetSupportedFormats() []FormatInfo {
	return []FormatInfo{
		{Name: FormatQCOW2, Description: "QEMU Copy-On-Write v2", Extension: ".qcow2", CanImport: true, CanExport: true},
		{Name: FormatQED, Description: "QEMU Enhanced Disk", Extension: ".qed", CanImport: true, CanExport: true},
		{Name: FormatRAW, Description: "原始磁盘镜像", Extension: ".img", CanImport: true, CanExport: true},
		{Name: FormatVDI, Description: "VirtualBox 磁盘镜像", Extension: ".vdi", CanImport: true, CanExport: true},
		{Name: FormatVHDX, Description: "Hyper-V 虚拟硬盘", Extension: ".vhdx", CanImport: true, CanExport: true},
		{Name: FormatVMDK, Description: "VMware 虚拟磁盘", Extension: ".vmdk", CanImport: true, CanExport: true},
	}
}

// ListImages 列出所有镜像.
func (m *Manager) ListImages() []*VMImage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	images := make([]*VMImage, 0, len(m.images))
	for _, img := range m.images {
		copy := *img
		images = append(images, &copy)
	}

	return images
}

// GetImage 获取镜像详情.
func (m *Manager) GetImage(id string) (*VMImage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	img, ok := m.images[id]
	if !ok {
		return nil, ErrImageNotFound
	}

	copy := *img
	return &copy, nil
}

// DeleteImage 删除镜像.
func (m *Manager) DeleteImage(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	img, ok := m.images[id]
	if !ok {
		return ErrImageNotFound
	}

	// 删除文件.
	if err := os.Remove(img.FilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除镜像文件失败: %w", err)
	}

	// 删除元数据.
	metaPath := filepath.Join(m.metadataPath, id+".json")
	os.Remove(metaPath) //nolint:errcheck

	delete(m.images, id)

	return nil
}

// GetStorageUsage 获取存储空间使用情况.
func (m *Manager) GetStorageUsage() (*StorageUsage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var totalSize int64
	for _, img := range m.images {
		totalSize += img.FileSize
	}

	// 获取磁盘空间.
	var totalSpace, freeSpace int64

	// 使用 statfs 获取磁盘空间信息（简化实现）.
	// 在实际实现中应该调用系统API.
	totalSpace = 1024 * 1024 * 1024 * 100 // 默认100GB.
	freeSpace = totalSpace - totalSize

	return &StorageUsage{
		TotalSpace:      totalSpace,
		UsedSpace:       totalSize,
		FreeSpace:       freeSpace,
		ImageCount:      len(m.images),
		ImagesTotalSize: totalSize,
		StoragePath:     m.storagePath,
	}, nil
}

// ========== 内部辅助方法 ==========

// updateTaskStatus 更新导入任务状态.
func (m *Manager) updateTaskStatus(id string, status TaskStatus, progress float64, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.imports[id]
	if !ok {
		return
	}

	task.Status = status
	task.Progress = progress
	task.ErrorMessage = errMsg
	task.UpdatedAt = time.Now()
}

// updateExportStatus 更新导出任务状态.
func (m *Manager) updateExportStatus(id string, status TaskStatus, progress float64, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.exports[id]
	if !ok {
		return
	}

	task.Status = status
	task.Progress = progress
	task.ErrorMessage = errMsg
	task.UpdatedAt = time.Now()
}

// getTaskProgress 获取任务进度.
func (m *Manager) getTaskProgress(id string) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if task, ok := m.imports[id]; ok {
		return task.Progress
	}
	return 0
}

// getExportProgress 获取导出任务进度.
func (m *Manager) getExportProgress(id string) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if task, ok := m.exports[id]; ok {
		return task.Progress
	}
	return 0
}

// isSupportedFormat 检查是否支持的格式.
func isSupportedFormat(f DiskFormat) bool {
	for _, sf := range SupportedFormats {
		if sf == f {
			return true
		}
	}
	return false
}

// computeChecksum 计算文件SHA256校验和.
func computeChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// saveImageMetadata 保存镜像元数据.
func (m *Manager) saveImageMetadata(img *VMImage) error {
	metaPath := filepath.Join(m.metadataPath, img.ID+".json")
	data, err := json.MarshalIndent(img, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化元数据失败: %w", err)
	}

	return os.WriteFile(metaPath, data, 0o644)
}

// loadImages 加载镜像元数据.
func (m *Manager) loadImages() error {
	entries, err := os.ReadDir(m.metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		metaPath := filepath.Join(m.metadataPath, entry.Name())
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}

		var img VMImage
		if err := json.Unmarshal(data, &img); err != nil {
			continue
		}

		// 检查镜像文件是否还存在.
		if _, err := os.Stat(img.FilePath); err == nil {
			m.images[img.ID] = &img
		}
	}

	return nil
}
