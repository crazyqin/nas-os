package filemanager

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Operations 文件操作管理器
type Operations struct {
	mu         sync.RWMutex
	rootPath   string
	tempDir    string
	operations map[string]*FileOperation // id -> operation
	logger     *zap.Logger
}

// NewOperations 创建文件操作管理器
func NewOperations(rootPath, tempDir string, logger *zap.Logger) *Operations {
	if logger == nil {
		logger = zap.NewNop()
	}

	// 确保临时目录存在
	os.MkdirAll(tempDir, 0755)

	return &Operations{
		rootPath:   rootPath,
		tempDir:    tempDir,
		operations: make(map[string]*FileOperation),
		logger:     logger,
	}
}

// Copy 复制文件或目录
func (ops *Operations) Copy(sources []string, destination string, overwrite bool, userID string) (*FileOperation, error) {
	// 验证路径
	if err := ops.validatePaths(sources, destination); err != nil {
		return nil, err
	}

	// 创建操作记录
	op := ops.createOperation(OpCopy, sources, destination, userID)

	// 异步执行复制
	go ops.executeCopy(op, overwrite)

	return op, nil
}

// Move 移动文件或目录
func (ops *Operations) Move(sources []string, destination string, overwrite bool, userID string) (*FileOperation, error) {
	if err := ops.validatePaths(sources, destination); err != nil {
		return nil, err
	}

	op := ops.createOperation(OpMove, sources, destination, userID)
	go ops.executeMove(op, overwrite)

	return op, nil
}

// Delete 删除文件或目录
func (ops *Operations) Delete(sources []string, userID string) (*FileOperation, error) {
	for _, src := range sources {
		cleanPath, err := ops.validatePath(src)
		if err != nil {
			return nil, err
		}
		if cleanPath == ops.rootPath {
			return nil, fmt.Errorf("不能删除根目录")
		}
	}

	op := ops.createOperation(OpDelete, sources, "", userID)
	go ops.executeDelete(op)

	return op, nil
}

// Rename 重命名文件或目录
func (ops *Operations) Rename(oldPath, newName string, userID string) (*FileOperation, error) {
	cleanPath, err := ops.validatePath(oldPath)
	if err != nil {
		return nil, err
	}

	// 验证新名称
	if newName == "" {
		return nil, fmt.Errorf("新名称不能为空")
	}
	if strings.ContainsAny(newName, "/\\:*?\"<>|") {
		return nil, fmt.Errorf("名称包含非法字符")
	}

	parentDir := filepath.Dir(cleanPath)
	newPath := filepath.Join(parentDir, newName)

	// 检查新路径是否已存在
	if _, err := os.Stat(newPath); err == nil {
		return nil, fmt.Errorf("目标已存在: %s", newName)
	}

	op := ops.createOperation(OpRename, []string{cleanPath}, newPath, userID)
	go ops.executeRename(op, newName)

	return op, nil
}

// Compress 压缩文件
func (ops *Operations) Compress(opts CompressOptions, userID string) (*FileOperation, error) {
	// 验证源路径
	for _, src := range opts.Sources {
		if _, err := ops.validatePath(src); err != nil {
			return nil, err
		}
	}

	// 验证目标路径
	targetPath, err := ops.validatePath(opts.Target)
	if err != nil {
		// 如果目标路径不在根目录下，使用临时目录
		targetPath = filepath.Join(ops.tempDir, opts.Target)
	}

	op := ops.createOperation(OpCompress, opts.Sources, targetPath, userID)
	go ops.executeCompress(op, opts)

	return op, nil
}

// Extract 解压文件
func (ops *Operations) Extract(opts ExtractOptions, userID string) (*FileOperation, error) {
	sourcePath, err := ops.validatePath(opts.Source)
	if err != nil {
		return nil, err
	}

	destPath, err := ops.validatePath(opts.Destination)
	if err != nil {
		return nil, err
	}

	op := ops.createOperation(OpExtract, []string{sourcePath}, destPath, userID)
	go ops.executeExtract(op, opts)

	return op, nil
}

// GetOperation 获取操作状态
func (ops *Operations) GetOperation(id string) (*FileOperation, error) {
	ops.mu.RLock()
	defer ops.mu.RUnlock()

	op, ok := ops.operations[id]
	if !ok {
		return nil, fmt.Errorf("操作不存在: %s", id)
	}
	return op, nil
}

// ListOperations 列出所有操作
func (ops *Operations) ListOperations() []*FileOperation {
	ops.mu.RLock()
	defer ops.mu.RUnlock()

	result := make([]*FileOperation, 0, len(ops.operations))
	for _, op := range ops.operations {
		result = append(result, op)
	}
	return result
}

// CancelOperation 取消操作
func (ops *Operations) CancelOperation(id string) error {
	ops.mu.Lock()
	defer ops.mu.Unlock()

	op, ok := ops.operations[id]
	if !ok {
		return fmt.Errorf("操作不存在: %s", id)
	}

	if op.Status == StatusCompleted || op.Status == StatusFailed {
		return fmt.Errorf("操作已完成，无法取消")
	}

	op.Status = StatusCancelled
	now := time.Now()
	op.CompletedAt = &now

	return nil
}

// BatchOperation 批量操作
func (ops *Operations) BatchOperation(req BatchOperation, userID string) (*FileOperation, error) {
	switch req.Operation {
	case OpCopy:
		return ops.Copy(req.Sources, req.Destination, req.Overwrite, userID)
	case OpMove:
		return ops.Move(req.Sources, req.Destination, req.Overwrite, userID)
	case OpDelete:
		return ops.Delete(req.Sources, userID)
	default:
		return nil, fmt.Errorf("不支持的批量操作: %s", req.Operation)
	}
}

// DragDrop 拖拽操作
func (ops *Operations) DragDrop(req DragDropRequest, userID string) (*FileOperation, error) {
	if req.Action == "copy" {
		return ops.Copy(req.Sources, req.Destination, false, userID)
	}
	return ops.Move(req.Sources, req.Destination, false, userID)
}

// ============================================================
// 内部实现
// ============================================================

// createOperation 创建操作记录
func (ops *Operations) createOperation(opType OperationType, sources []string, dest, userID string) *FileOperation {
	op := &FileOperation{
		ID:          uuid.New().String(),
		Type:        opType,
		Status:      StatusPending,
		Source:      sources,
		Destination: dest,
		StartedAt:   time.Now(),
		CreatedBy:   userID,
	}

	ops.mu.Lock()
	ops.operations[op.ID] = op
	ops.mu.Unlock()

	return op
}

// executeCopy 执行复制操作
func (ops *Operations) executeCopy(op *FileOperation, overwrite bool) {
	ops.updateStatus(op, StatusRunning)

	var totalFiles int
	var totalSize int64

	for _, src := range op.Source {
		// 检查是否取消
		if ops.isCancelled(op) {
			return
		}

		srcInfo, err := os.Stat(src)
		if err != nil {
			ops.setError(op, fmt.Sprintf("源路径不存在: %s", src))
			return
		}

		destPath := filepath.Join(op.Destination, srcInfo.Name())

		if srcInfo.IsDir() {
			files, size, err := ops.copyDirectory(src, destPath, overwrite, op)
			if err != nil {
				ops.setError(op, err.Error())
				return
			}
			totalFiles += files
			totalSize += size
		} else {
			if err := ops.copyFile(src, destPath, overwrite); err != nil {
				ops.setError(op, err.Error())
				return
			}
			totalFiles++
			totalSize += srcInfo.Size()
		}

		op.Processed = totalFiles
		op.TotalFiles = totalFiles
		op.TotalSize = totalSize
	}

	ops.completeOperation(op)
}

// executeMove 执行移动操作
func (ops *Operations) executeMove(op *FileOperation, overwrite bool) {
	ops.updateStatus(op, StatusRunning)

	for _, src := range op.Source {
		if ops.isCancelled(op) {
			return
		}

		srcInfo, err := os.Stat(src)
		if err != nil {
			ops.setError(op, fmt.Sprintf("源路径不存在: %s", src))
			return
		}

		destPath := filepath.Join(op.Destination, srcInfo.Name())

		// 检查是否同文件系统
		if err := os.Rename(src, destPath); err != nil {
			// 跨文件系统，需要复制后删除
			if srcInfo.IsDir() {
				files, size, err := ops.copyDirectory(src, destPath, overwrite, op)
				if err != nil {
					ops.setError(op, err.Error())
					return
				}
				op.TotalFiles = files
				op.TotalSize = size
			} else {
				if err := ops.copyFile(src, destPath, overwrite); err != nil {
					ops.setError(op, err.Error())
					return
				}
				op.TotalFiles = 1
				op.TotalSize = srcInfo.Size()
			}

			// 删除源文件
			if err := os.RemoveAll(src); err != nil {
				ops.logger.Warn("删除源文件失败", zap.String("path", src), zap.Error(err))
			}
		}

		op.Processed = op.TotalFiles
	}

	ops.completeOperation(op)
}

// executeDelete 执行删除操作
func (ops *Operations) executeDelete(op *FileOperation) {
	ops.updateStatus(op, StatusRunning)

	var totalFiles int

	for _, src := range op.Source {
		if ops.isCancelled(op) {
			return
		}

		// 统计文件数
		count := ops.countFiles(src)
		totalFiles += count
		op.TotalFiles = totalFiles

		if err := os.RemoveAll(src); err != nil {
			ops.setError(op, fmt.Sprintf("删除失败: %s: %v", src, err))
			return
		}

		op.Processed += count
	}

	ops.completeOperation(op)
}

// executeRename 执行重命名操作
func (ops *Operations) executeRename(op *FileOperation, newName string) {
	ops.updateStatus(op, StatusRunning)

	oldPath := op.Source[0]
	newPath := op.Destination

	if err := os.Rename(oldPath, newPath); err != nil {
		ops.setError(op, fmt.Sprintf("重命名失败: %v", err))
		return
	}

	op.TotalFiles = 1
	op.Processed = 1
	ops.completeOperation(op)
}

// executeCompress 执行压缩操作
func (ops *Operations) executeCompress(op *FileOperation, opts CompressOptions) {
	ops.updateStatus(op, StatusRunning)

	// 确保目标目录存在
	destDir := filepath.Dir(opts.Target)
	os.MkdirAll(destDir, 0755)

	// 根据格式选择压缩方式
	switch strings.ToLower(opts.Format) {
	case "zip":
		if err := ops.compressZip(op, opts); err != nil {
			ops.setError(op, err.Error())
			return
		}
	case "tar.gz", "tgz":
		if err := ops.compressTarGz(op, opts); err != nil {
			ops.setError(op, err.Error())
			return
		}
	default:
		ops.setError(op, fmt.Sprintf("不支持的压缩格式: %s", opts.Format))
		return
	}

	ops.completeOperation(op)
}

// executeExtract 执行解压操作
func (ops *Operations) executeExtract(op *FileOperation, opts ExtractOptions) {
	ops.updateStatus(op, StatusRunning)

	// 检测压缩格式
	ext := strings.ToLower(filepath.Ext(opts.Source))

	switch {
	case ext == ".zip":
		if err := ops.extractZip(op, opts); err != nil {
			ops.setError(op, err.Error())
			return
		}
	case ext == ".gz" || ext == ".tgz":
		if err := ops.extractTarGz(op, opts); err != nil {
			ops.setError(op, err.Error())
			return
		}
	default:
		ops.setError(op, fmt.Sprintf("不支持的压缩格式: %s", ext))
		return
	}

	ops.completeOperation(op)
}

// copyFile 复制单个文件
func (ops *Operations) copyFile(src, dest string, overwrite bool) error {
	if !overwrite {
		if _, err := os.Stat(dest); err == nil {
			return fmt.Errorf("目标文件已存在: %s", dest)
		}
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, srcFile); err != nil {
		return err
	}

	// 保留权限
	return os.Chmod(dest, srcInfo.Mode())
}

// copyDirectory 复制目录
func (ops *Operations) copyDirectory(src, dest string, overwrite bool, op *FileOperation) (int, int64, error) {
	var fileCount int
	var totalSize int64

	// 创建目标目录
	if err := os.MkdirAll(dest, 0755); err != nil {
		return 0, 0, err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return 0, 0, err
	}

	for _, entry := range entries {
		if ops.isCancelled(op) {
			return fileCount, totalSize, fmt.Errorf("操作已取消")
		}

		srcPath := filepath.Join(src, entry.Name())
		destPath := filepath.Join(dest, entry.Name())

		if entry.IsDir() {
			files, size, err := ops.copyDirectory(srcPath, destPath, overwrite, op)
			if err != nil {
				return fileCount, totalSize, err
			}
			fileCount += files
			totalSize += size
		} else {
			info, err := entry.Info()
			if err != nil {
				continue
			}

			if err := ops.copyFile(srcPath, destPath, overwrite); err != nil {
				return fileCount, totalSize, err
			}

			fileCount++
			totalSize += info.Size()
			op.Processed = fileCount
		}
	}

	return fileCount, totalSize, nil
}

// compressZip ZIP压缩
func (ops *Operations) compressZip(op *FileOperation, opts CompressOptions) error {
	zipFile, err := os.Create(opts.Target)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	writer := zip.NewWriter(zipFile)
	defer writer.Close()

	for _, src := range opts.Sources {
		if err := ops.addToZip(writer, src, ""); err != nil {
			return err
		}
	}

	return nil
}

// addToZip 添加文件到ZIP
func (ops *Operations) addToZip(writer *zip.Writer, path, baseInZip string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			newBase := filepath.Join(baseInZip, info.Name())
			if err := ops.addToZip(writer, filepath.Join(path, entry.Name()), newBase); err != nil {
				return err
			}
		}
		return nil
	}

	// 添加文件
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// 设置ZIP内的路径
	zipPath := filepath.Join(baseInZip, info.Name())
	f, err := writer.Create(zipPath)
	if err != nil {
		return err
	}

	_, err = io.Copy(f, file)
	return err
}

// compressTarGz TAR.GZ压缩
func (ops *Operations) compressTarGz(op *FileOperation, opts CompressOptions) error {
	file, err := os.Create(opts.Target)
	if err != nil {
		return err
	}
	defer file.Close()

	gw := gzip.NewWriter(file)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	for _, src := range opts.Sources {
		if err := ops.addToTar(tw, src, ""); err != nil {
			return err
		}
	}

	return nil
}

// addToTar 添加文件到TAR
func (ops *Operations) addToTar(tw *tar.Writer, path, baseInTar string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	// 创建tar头
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = filepath.Join(baseInTar, info.Name())

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			newBase := filepath.Join(baseInTar, info.Name())
			if err := ops.addToTar(tw, filepath.Join(path, entry.Name()), newBase); err != nil {
				return err
			}
		}
		return nil
	}

	// 写入文件内容
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(tw, file)
	return err
}

// extractZip 解压ZIP
func (ops *Operations) extractZip(op *FileOperation, opts ExtractOptions) error {
	r, err := zip.OpenReader(opts.Source)
	if err != nil {
		return err
	}
	defer r.Close()

	op.TotalFiles = len(r.File)

	for _, f := range r.File {
		if ops.isCancelled(op) {
			return fmt.Errorf("操作已取消")
		}

		path := filepath.Join(opts.Destination, f.Name)

		// 安全检查：防止路径遍历
		if !strings.HasPrefix(path, filepath.Clean(opts.Destination)+string(os.PathSeparator)) {
			return fmt.Errorf("非法路径: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(path, 0755)
			continue
		}

		// 创建目录
		os.MkdirAll(filepath.Dir(path), 0755)

		// 解压文件
		outFile, err := os.Create(path)
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()

		if err != nil {
			return err
		}

		op.Processed++
	}

	return nil
}

// extractTarGz 解压TAR.GZ
func (ops *Operations) extractTarGz(op *FileOperation, opts ExtractOptions) error {
	file, err := os.Open(opts.Source)
	if err != nil {
		return err
	}
	defer file.Close()

	gr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		path := filepath.Join(opts.Destination, header.Name)

		// 安全检查
		if !strings.HasPrefix(path, filepath.Clean(opts.Destination)+string(os.PathSeparator)) {
			return fmt.Errorf("非法路径: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(path, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(path), 0755)

			outFile, err := os.Create(path)
			if err != nil {
				return err
			}

			_, err = io.Copy(outFile, tr)
			outFile.Close()

			if err != nil {
				return err
			}

			op.Processed++
		}
	}

	return nil
}

// countFiles 统计文件数
func (ops *Operations) countFiles(path string) int {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}

	if !info.IsDir() {
		return 1
	}

	count := 0
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0
	}

	for _, entry := range entries {
		if entry.IsDir() {
			count += ops.countFiles(filepath.Join(path, entry.Name()))
		} else {
			count++
		}
	}

	return count
}

// validatePath 验证路径
func (ops *Operations) validatePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("路径不能为空")
	}

	// 转换为绝对路径
	if !filepath.IsAbs(path) {
		path = filepath.Join(ops.rootPath, path)
	}

	cleanPath := filepath.Clean(path)

	// 安全检查
	if !strings.HasPrefix(cleanPath, ops.rootPath) {
		return "", fmt.Errorf("路径超出根目录范围: %s", path)
	}

	return cleanPath, nil
}

// validatePaths 验证多个路径
func (ops *Operations) validatePaths(sources []string, destination string) error {
	if len(sources) == 0 {
		return fmt.Errorf("源路径列表不能为空")
	}

	for _, src := range sources {
		if _, err := ops.validatePath(src); err != nil {
			return err
		}
	}

	if destination != "" {
		if _, err := ops.validatePath(destination); err != nil {
			return err
		}
	}

	return nil
}

// updateStatus 更新操作状态
func (ops *Operations) updateStatus(op *FileOperation, status OperationStatus) {
	ops.mu.Lock()
	defer ops.mu.Unlock()

	op.Status = status
}

// setError 设置操作错误
func (ops *Operations) setError(op *FileOperation, errMsg string) {
	ops.mu.Lock()
	defer ops.mu.Unlock()

	op.Status = StatusFailed
	op.Error = errMsg
	now := time.Now()
	op.CompletedAt = &now

	ops.logger.Error("文件操作失败",
		zap.String("id", op.ID),
		zap.String("type", string(op.Type)),
		zap.String("error", errMsg))
}

// completeOperation 完成操作
func (ops *Operations) completeOperation(op *FileOperation) {
	ops.mu.Lock()
	defer ops.mu.Unlock()

	op.Status = StatusCompleted
	op.Progress = 100
	now := time.Now()
	op.CompletedAt = &now

	ops.logger.Info("文件操作完成",
		zap.String("id", op.ID),
		zap.String("type", string(op.Type)),
		zap.Int("files", op.TotalFiles))
}

// isCancelled 检查操作是否已取消
func (ops *Operations) isCancelled(op *FileOperation) bool {
	ops.mu.RLock()
	defer ops.mu.RUnlock()

	return op.Status == StatusCancelled
}
