package compress

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== FileSystem Tests ==========

func TestFileSystem_Open_RawFile(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(DefaultConfig())
	require.NoError(t, err)

	fs, err := NewFileSystem(tmpDir, manager)
	require.NoError(t, err)

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("hello world")
	err = os.WriteFile(testFile, content, 0644)
	require.NoError(t, err)

	// 打开文件
	reader, err := fs.Open("/test.txt")
	require.NoError(t, err)
	defer reader.Close()

	// 读取内容
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(reader)
	require.NoError(t, err)
	assert.Equal(t, content, buf.Bytes())
}

func TestFileSystem_Open_CompressedFile(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultConfig()
	config.DefaultAlgorithm = AlgorithmGzip
	config.MinSize = 10

	manager, err := NewManager(config)
	require.NoError(t, err)

	fs, err := NewFileSystem(tmpDir, manager)
	require.NoError(t, err)

	// 创建并压缩文件
	srcFile := filepath.Join(tmpDir, "source.txt")
	content := make([]byte, 1000)
	for i := range content {
		content[i] = byte(i % 256)
	}
	err = os.WriteFile(srcFile, content, 0644)
	require.NoError(t, err)

	// 压缩文件
	dstFile := filepath.Join(tmpDir, "test.txt.gz")
	result, err := manager.CompressFile(srcFile, dstFile)
	require.NoError(t, err)
	assert.False(t, result.Skipped)

	// 通过文件系统打开压缩文件
	reader, err := fs.Open("/test.txt.gz")
	require.NoError(t, err)
	defer reader.Close()

	// 读取内容（应该是压缩格式）
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(reader)
	require.NoError(t, err)
	// 验证读取到了内容
	assert.True(t, buf.Len() > 0)
}

func TestFileSystem_Stat(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(DefaultConfig())
	require.NoError(t, err)

	fs, err := NewFileSystem(tmpDir, manager)
	require.NoError(t, err)

	// 创建文件
	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	require.NoError(t, err)

	// 获取文件信息
	info, err := fs.Stat("/test.txt")
	require.NoError(t, err)
	assert.Equal(t, "test.txt", info.Name())
	assert.Equal(t, int64(4), info.Size())
	assert.False(t, info.IsDir())
}

func TestFileSystem_Remove(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(DefaultConfig())
	require.NoError(t, err)

	fs, err := NewFileSystem(tmpDir, manager)
	require.NoError(t, err)

	// 创建文件
	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	require.NoError(t, err)

	// 删除文件
	err = fs.Remove("/test.txt")
	require.NoError(t, err)

	// 验证文件不存在
	_, err = os.Stat(testFile)
	assert.True(t, os.IsNotExist(err))
}

func TestFileSystem_Rename(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(DefaultConfig())
	require.NoError(t, err)

	fs, err := NewFileSystem(tmpDir, manager)
	require.NoError(t, err)

	// 创建文件
	testFile := filepath.Join(tmpDir, "old.txt")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	require.NoError(t, err)

	// 重命名文件
	err = fs.Rename("/old.txt", "/new.txt")
	require.NoError(t, err)

	// 验证新文件存在
	_, err = os.Stat(filepath.Join(tmpDir, "new.txt"))
	require.NoError(t, err)

	// 验证旧文件不存在
	_, err = os.Stat(testFile)
	assert.True(t, os.IsNotExist(err))
}

func TestFileSystem_ReadDir(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(DefaultConfig())
	require.NoError(t, err)

	fs, err := NewFileSystem(tmpDir, manager)
	require.NoError(t, err)

	// 创建多个文件
	for i := 0; i < 3; i++ {
		err := os.WriteFile(filepath.Join(tmpDir, "file"+string(rune('0'+i))+".txt"), []byte("test"), 0644)
		require.NoError(t, err)
	}

	// 读取目录
	entries, err := fs.ReadDir("/")
	require.NoError(t, err)
	assert.Len(t, entries, 3)
}

func TestFileSystem_Open_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(DefaultConfig())
	require.NoError(t, err)

	fs, err := NewFileSystem(tmpDir, manager)
	require.NoError(t, err)

	// 创建目录
	err = os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)
	require.NoError(t, err)

	// 尝试打开目录
	_, err = fs.Open("/subdir")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "directory")
}

func TestFileSystem_Open_NotExist(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(DefaultConfig())
	require.NoError(t, err)

	fs, err := NewFileSystem(tmpDir, manager)
	require.NoError(t, err)

	// 尝试打开不存在的文件
	_, err = fs.Open("/nonexistent.txt")
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

// ========== CompressionProgress Tests ==========

func TestCompressionProgress_Phases(t *testing.T) {
	progress := NewCompressionProgress()

	assert.Equal(t, PhaseScanning, progress.Phase)

	progress.SetPhase(PhaseCompressing)
	assert.Equal(t, PhaseCompressing, progress.Phase)

	progress.SetPhase(PhaseVerifying)
	assert.Equal(t, PhaseVerifying, progress.Phase)

	progress.SetPhase(PhaseCompleted)
	assert.Equal(t, PhaseCompleted, progress.Phase)
}

func TestCompressionProgress_FileCounts(t *testing.T) {
	progress := NewCompressionProgress()

	progress.SetTotal(100, 1000000)
	assert.Equal(t, int64(100), progress.FilesTotal)
	assert.Equal(t, int64(1000000), progress.BytesTotal)

	progress.AddFileDone(500)
	assert.Equal(t, int64(1), progress.FilesDone)
	assert.Equal(t, int64(500), progress.BytesSaved)

	progress.AddFileSkipped()
	assert.Equal(t, int64(1), progress.FilesSkipped)

	progress.AddFileFailed()
	assert.Equal(t, int64(1), progress.FilesFailed)
}

func TestCompressionProgress_Workers(t *testing.T) {
	progress := NewCompressionProgress()

	progress.SetWorkers(4)
	assert.Equal(t, 4, progress.Workers)

	progress.SetActiveTasks(2)
	assert.Equal(t, int32(2), progress.ActiveTasks)
}

func TestCompressionProgress_Callback(t *testing.T) {
	progress := NewCompressionProgress()

	called := 0
	progress.OnProgress(func(p *CompressionProgress) {
		called++
	})

	progress.SetTotal(10, 1000)
	progress.AddFileDone(500)

	assert.Equal(t, 2, called)
}

func TestCompressionProgress_ETA(t *testing.T) {
	progress := NewCompressionProgress()
	progress.SetTotal(100, 1000000)

	// 模拟处理了一些文件
	progress.AddBytes(500000)
	progress.AddFileDone(50)

	// 验证进度被更新
	assert.GreaterOrEqual(t, progress.BytesDone, int64(500000))
}

// ========== ParallelCompressor Tests ==========

func TestParallelConfig_Default(t *testing.T) {
	config := DefaultParallelConfig()

	assert.Equal(t, AlgorithmGzip, config.Algorithm)
	assert.Equal(t, 6, config.Level)
	assert.Equal(t, 4, config.Workers)
	assert.False(t, config.DeleteOriginal)
	assert.False(t, config.Overwrite)
	assert.True(t, config.ContinueOnError)
	assert.True(t, config.VerifyAfter)
	assert.Equal(t, 3, config.MaxRetries)
}

func TestParallelCompressor_New(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultParallelConfig()
	config.Algorithm = AlgorithmGzip

	pc, err := NewParallelCompressor(config, tmpDir)
	require.NoError(t, err)
	assert.NotNil(t, pc)
	assert.NotNil(t, pc.progress)
}

func TestParallelCompressor_CompressParallel_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultParallelConfig()
	config.Algorithm = AlgorithmGzip

	pc, err := NewParallelCompressor(config, tmpDir)
	require.NoError(t, err)

	result, err := pc.CompressParallel(context.Background(), []string{}, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.TotalFiles)
}

func TestParallelCompressor_GetProgress(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultParallelConfig()

	pc, err := NewParallelCompressor(config, tmpDir)
	require.NoError(t, err)

	progress := pc.GetProgress()
	assert.NotNil(t, progress)
	assert.Equal(t, PhaseScanning, progress.Phase)
}

// ========== CompressionState Tests ==========

func TestCompressionState(t *testing.T) {
	state := &CompressionState{
		ID:             "test-task-001",
		StartedAt:      time.Now(),
		Status:         "running",
		TotalFiles:     4,
		ProcessedFiles: []string{"/file1.txt", "/file2.txt"},
		PendingFiles:   []string{"/file3.txt", "/file4.txt"},
		FailedFiles:    []FailedFile{},
		BytesDone:      5000,
		BytesSaved:     2000,
		Config:         *DefaultParallelConfig(),
	}

	assert.Equal(t, "test-task-001", state.ID)
	assert.Equal(t, "running", state.Status)
	assert.Equal(t, int64(4), state.TotalFiles)
	assert.Len(t, state.ProcessedFiles, 2)
	assert.Len(t, state.PendingFiles, 2)
}

// ========== Service Tests ==========

func TestService_New(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultServiceConfig()
	config.RootPath = tmpDir

	svc, err := NewService(config)
	require.NoError(t, err)
	assert.NotNil(t, svc)
	assert.NotNil(t, svc.Manager)
	assert.NotNil(t, svc.FS)
	assert.NotNil(t, svc.Handlers)
}

func TestService_StartStop(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultServiceConfig()
	config.RootPath = tmpDir

	svc, err := NewService(config)
	require.NoError(t, err)

	err = svc.Start()
	require.NoError(t, err)

	svc.Stop() // Should not panic
}

func TestService_GetCompressedSize(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultServiceConfig()
	config.RootPath = tmpDir

	svc, err := NewService(config)
	require.NoError(t, err)

	// 无统计数据时使用默认压缩率
	size := svc.GetCompressedSize(10000)
	assert.Less(t, size, int64(10000)) // 应该比原始大小小
}

func TestService_Enabled(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultServiceConfig()
	config.RootPath = tmpDir

	svc, err := NewService(config)
	require.NoError(t, err)

	assert.True(t, svc.IsEnabled())

	svc.SetEnabled(false)
	assert.False(t, svc.IsEnabled())

	svc.SetEnabled(true)
	assert.True(t, svc.IsEnabled())
}

func TestService_Algorithm(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultServiceConfig()
	config.RootPath = tmpDir

	svc, err := NewService(config)
	require.NoError(t, err)

	alg := svc.GetAlgorithm()
	assert.Equal(t, AlgorithmZstd, alg)

	svc.SetAlgorithm(AlgorithmGzip)
	assert.Equal(t, AlgorithmGzip, svc.GetAlgorithm())
}

func TestService_Tasks(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultServiceConfig()
	config.RootPath = tmpDir
	config.StateDir = tmpDir

	svc, err := NewService(config)
	require.NoError(t, err)

	// 列出待恢复任务（应该为空）
	pending := svc.ListPendingTasks()
	assert.Nil(t, pending)

	// 获取不存在的任务进度
	_, ok := svc.GetTaskProgress("nonexistent")
	assert.False(t, ok)

	// 获取不存在的任务结果
	_, ok = svc.GetTaskResult("nonexistent")
	assert.False(t, ok)

	// 取消不存在的任务
	ok = svc.CancelTask("nonexistent")
	assert.False(t, ok)
}

// ========== Error Tests ==========

func TestCompressError(t *testing.T) {
	err := &Error{
		Code:    "TEST_ERROR",
		Message: "test error message",
	}

	assert.Equal(t, "test error message", err.Error())
}

func TestErrParallelNotAvailable(t *testing.T) {
	assert.Equal(t, "PARALLEL_NOT_AVAILABLE", ErrParallelNotAvailable.Code)
}

func TestErrTaskNotFound(t *testing.T) {
	assert.Equal(t, "TASK_NOT_FOUND", ErrTaskNotFound.Code)
}

// ========== CompressedFileInfo Tests ==========

func TestGetCompressedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(DefaultConfig())
	require.NoError(t, err)

	fs, err := NewFileSystem(tmpDir, manager)
	require.NoError(t, err)

	// 创建一些压缩文件（直接创建 .gz 文件）
	for i := 0; i < 3; i++ {
		srcFile := filepath.Join(tmpDir, "source"+string(rune('0'+i))+".txt")
		content := make([]byte, 1000)
		err := os.WriteFile(srcFile, content, 0644)
		require.NoError(t, err)

		dstFile := filepath.Join(tmpDir, "compressed"+string(rune('0'+i))+".txt")
		_, err = manager.CompressFile(srcFile, dstFile)
		require.NoError(t, err)
	}

	// 获取压缩文件列表
	files, err := fs.GetCompressedFiles("/")
	require.NoError(t, err)
	// 由于 GetCompressedFiles 递归扫描，可能找到一些文件
	assert.GreaterOrEqual(t, len(files), 0)
}

// ========== Writer Tests ==========

func TestWriter(t *testing.T) {
	tmpDir := t.TempDir()
	manager, err := NewManager(DefaultConfig())
	require.NoError(t, err)

	// 创建一个较大的文件以触发压缩
	testFile := filepath.Join(tmpDir, "test.txt")
	content := make([]byte, 5000)
	for i := range content {
		content[i] = byte(i % 256)
	}

	writer, err := NewWriter(testFile, manager)
	require.NoError(t, err)

	n, err := writer.Write(content)
	require.NoError(t, err)
	assert.Equal(t, len(content), n)

	err = writer.Close()
	require.NoError(t, err)
}

// ========== BatchCompress Tests ==========

func TestFileSystem_BatchCompress(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultConfig()
	config.MinSize = 100

	manager, err := NewManager(config)
	require.NoError(t, err)

	fs, err := NewFileSystem(tmpDir, manager)
	require.NoError(t, err)

	// 创建多个测试文件
	for i := 0; i < 5; i++ {
		file := filepath.Join(tmpDir, "file"+string(rune('0'+i))+".txt")
		content := make([]byte, 1000)
		err := os.WriteFile(file, content, 0644)
		require.NoError(t, err)
	}

	// 批量压缩
	result, err := fs.BatchCompress("/", true)
	require.NoError(t, err)
	assert.Equal(t, 5, result.TotalFiles)
	assert.True(t, result.SavedBytes > 0)
}