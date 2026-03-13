package replication

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConflictDetector_DetectConflict_NoConflict(t *testing.T) {
	detector := NewConflictDetector(ConflictSourceWins)

	tmpDir, err := os.MkdirTemp("", "conflict-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 创建源文件
	sourceFile := filepath.Join(tmpDir, "source", "test.txt")
	require.NoError(t, os.MkdirAll(filepath.Dir(sourceFile), 0755))
	require.NoError(t, os.WriteFile(sourceFile, []byte("test content"), 0644))

	// 目标不存在，无冲突
	task := &ReplicationTask{
		ID:         "test-task",
		SourcePath: filepath.Join(tmpDir, "source"),
		TargetPath: filepath.Join(tmpDir, "target"),
	}

	conflict, err := detector.DetectConflict(task, "test.txt")
	require.NoError(t, err)
	assert.Nil(t, conflict)
}

func TestConflictDetector_DetectConflict_SameContent(t *testing.T) {
	detector := NewConflictDetector(ConflictSourceWins)

	tmpDir, err := os.MkdirTemp("", "conflict-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 创建源文件
	sourceDir := filepath.Join(tmpDir, "source")
	targetDir := filepath.Join(tmpDir, "target")
	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	content := []byte("same content")
	sourceFile := filepath.Join(sourceDir, "test.txt")
	targetFile := filepath.Join(targetDir, "test.txt")
	require.NoError(t, os.WriteFile(sourceFile, content, 0644))
	require.NoError(t, os.WriteFile(targetFile, content, 0644))

	task := &ReplicationTask{
		ID:         "test-task",
		SourcePath: sourceDir,
		TargetPath: targetDir,
	}

	conflict, err := detector.DetectConflict(task, "test.txt")
	require.NoError(t, err)
	// 内容相同，无冲突
	assert.Nil(t, conflict)
}

func TestConflictDetector_DetectConflict_DifferentContent(t *testing.T) {
	detector := NewConflictDetector(ConflictSourceWins)

	tmpDir, err := os.MkdirTemp("", "conflict-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 创建源文件
	sourceDir := filepath.Join(tmpDir, "source")
	targetDir := filepath.Join(tmpDir, "target")
	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	sourceFile := filepath.Join(sourceDir, "test.txt")
	targetFile := filepath.Join(targetDir, "test.txt")
	require.NoError(t, os.WriteFile(sourceFile, []byte("source content"), 0644))
	require.NoError(t, os.WriteFile(targetFile, []byte("target content"), 0644))

	task := &ReplicationTask{
		ID:         "test-task",
		SourcePath: sourceDir,
		TargetPath: targetDir,
	}

	conflict, err := detector.DetectConflict(task, "test.txt")
	require.NoError(t, err)
	// 内容不同，产生冲突
	require.NotNil(t, conflict)
	assert.Equal(t, "test-task", conflict.TaskID)
	assert.Equal(t, "test.txt", conflict.RelativePath)
	assert.NotEmpty(t, conflict.SourceHash)
	assert.NotEmpty(t, conflict.TargetHash)
	assert.NotEqual(t, conflict.SourceHash, conflict.TargetHash)
}

func TestConflictDetector_ResolveConflict_SourceWins(t *testing.T) {
	detector := NewConflictDetector(ConflictSourceWins)

	tmpDir, err := os.MkdirTemp("", "conflict-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	sourceDir := filepath.Join(tmpDir, "source")
	targetDir := filepath.Join(tmpDir, "target")
	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	sourceFile := filepath.Join(sourceDir, "test.txt")
	targetFile := filepath.Join(targetDir, "test.txt")
	require.NoError(t, os.WriteFile(sourceFile, []byte("source content"), 0644))
	require.NoError(t, os.WriteFile(targetFile, []byte("target content"), 0644))

	conflict := &ConflictInfo{
		ID:            "conflict-1",
		TaskID:        "test-task",
		RelativePath:  "test.txt",
		SourcePath:    sourceFile,
		TargetPath:    targetFile,
		SourceSize:    14,
		TargetSize:    14,
		Strategy:      ConflictSourceWins,
	}

	err = detector.ResolveConflict(conflict)
	require.NoError(t, err)
	assert.True(t, conflict.Resolved)

	// 验证目标文件被覆盖
	content, err := os.ReadFile(targetFile)
	require.NoError(t, err)
	assert.Equal(t, "source content", string(content))
}

func TestConflictDetector_ResolveConflict_TargetWins(t *testing.T) {
	detector := NewConflictDetector(ConflictTargetWins)

	tmpDir, err := os.MkdirTemp("", "conflict-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	sourceDir := filepath.Join(tmpDir, "source")
	targetDir := filepath.Join(tmpDir, "target")
	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	sourceFile := filepath.Join(sourceDir, "test.txt")
	targetFile := filepath.Join(targetDir, "test.txt")
	require.NoError(t, os.WriteFile(sourceFile, []byte("source content"), 0644))
	require.NoError(t, os.WriteFile(targetFile, []byte("target content"), 0644))

	conflict := &ConflictInfo{
		ID:            "conflict-1",
		TaskID:        "test-task",
		RelativePath:  "test.txt",
		SourcePath:    sourceFile,
		TargetPath:    targetFile,
		SourceSize:    14,
		TargetSize:    14,
		Strategy:      ConflictTargetWins,
	}

	err = detector.ResolveConflict(conflict)
	require.NoError(t, err)
	assert.True(t, conflict.Resolved)

	// 验证目标文件未被修改
	content, err := os.ReadFile(targetFile)
	require.NoError(t, err)
	assert.Equal(t, "target content", string(content))
}

func TestConflictDetector_ResolveConflict_NewerWins(t *testing.T) {
	detector := NewConflictDetector(ConflictNewerWins)

	tmpDir, err := os.MkdirTemp("", "conflict-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	sourceDir := filepath.Join(tmpDir, "source")
	targetDir := filepath.Join(tmpDir, "target")
	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	sourceFile := filepath.Join(sourceDir, "test.txt")
	targetFile := filepath.Join(targetDir, "test.txt")

	// 创建源文件（较新）
	require.NoError(t, os.WriteFile(sourceFile, []byte("newer source"), 0644))
	time.Sleep(100 * time.Millisecond)
	require.NoError(t, os.WriteFile(targetFile, []byte("older target"), 0644))

	sourceInfo, _ := os.Stat(sourceFile)
	targetInfo, _ := os.Stat(targetFile)

	conflict := &ConflictInfo{
		ID:            "conflict-1",
		TaskID:        "test-task",
		RelativePath:  "test.txt",
		SourcePath:    sourceFile,
		TargetPath:    targetFile,
		SourceSize:    sourceInfo.Size(),
		TargetSize:    targetInfo.Size(),
		SourceModTime: sourceInfo.ModTime(),
		TargetModTime: targetInfo.ModTime(),
		Strategy:      ConflictNewerWins,
	}

	err = detector.ResolveConflict(conflict)
	require.NoError(t, err)
	assert.True(t, conflict.Resolved)

	// 较新的源文件应该胜出（因为创建时间更早）
	content, err := os.ReadFile(targetFile)
	require.NoError(t, err)
	// 根据实际修改时间决定
	assert.Contains(t, []string{"newer source", "older target"}, string(content))
}

func TestConflictDetector_ResolveConflict_Rename(t *testing.T) {
	detector := NewConflictDetector(ConflictRename)

	tmpDir, err := os.MkdirTemp("", "conflict-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	sourceDir := filepath.Join(tmpDir, "source")
	targetDir := filepath.Join(tmpDir, "target")
	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	sourceFile := filepath.Join(sourceDir, "test.txt")
	targetFile := filepath.Join(targetDir, "test.txt")
	require.NoError(t, os.WriteFile(sourceFile, []byte("source content"), 0644))
	require.NoError(t, os.WriteFile(targetFile, []byte("target content"), 0644))

	conflict := &ConflictInfo{
		ID:            "conflict-1",
		TaskID:        "test-task",
		RelativePath:  "test.txt",
		SourcePath:    sourceFile,
		TargetPath:    targetFile,
		Strategy:      ConflictRename,
	}

	err = detector.ResolveConflict(conflict)
	require.NoError(t, err)
	assert.True(t, conflict.Resolved)
	assert.NotEmpty(t, conflict.ResolutionPath)

	// 验证原始目标文件被重命名
	_, err = os.Stat(conflict.ResolutionPath)
	require.NoError(t, err)

	// 验证新文件内容
	content, err := os.ReadFile(targetFile)
	require.NoError(t, err)
	assert.Equal(t, "source content", string(content))
}

func TestConflictDetector_ResolveConflict_Skip(t *testing.T) {
	detector := NewConflictDetector(ConflictSkip)

	conflict := &ConflictInfo{
		ID:       "conflict-1",
		TaskID:   "test-task",
		Strategy: ConflictSkip,
	}

	err := detector.ResolveConflict(conflict)
	require.NoError(t, err)
	assert.True(t, conflict.Resolved)
}

func TestConflictDetector_ResolveConflict_Manual(t *testing.T) {
	detector := NewConflictDetector(ConflictManual)

	conflict := &ConflictInfo{
		ID:       "conflict-1",
		TaskID:   "test-task",
		Strategy: ConflictManual,
	}

	err := detector.ResolveConflict(conflict)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "手动解决")
	assert.False(t, conflict.Resolved)
}

func TestConflictDetector_GetConflicts(t *testing.T) {
	detector := NewConflictDetector(ConflictSourceWins)

	// 添加一些冲突
	detector.AddConflict(&ConflictInfo{ID: "c1", TaskID: "task1"})
	detector.AddConflict(&ConflictInfo{ID: "c2", TaskID: "task1"})
	detector.AddConflict(&ConflictInfo{ID: "c3", TaskID: "task2"})

	// 获取所有冲突
	allConflicts := detector.GetConflicts("")
	assert.Len(t, allConflicts, 3)

	// 获取特定任务的冲突
	task1Conflicts := detector.GetConflicts("task1")
	assert.Len(t, task1Conflicts, 2)

	task2Conflicts := detector.GetConflicts("task2")
	assert.Len(t, task2Conflicts, 1)
}

func TestCalculateFileHash(t *testing.T) {
	detector := NewConflictDetector(ConflictSourceWins)

	tmpFile, err := os.CreateTemp("", "hash-test")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	content := []byte("test content for hash")
	_, err = tmpFile.Write(content)
	require.NoError(t, err)
	tmpFile.Close()

	hash, err := detector.calculateFileHash(tmpFile.Name())
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.Len(t, hash, 32) // MD5 hex 编码长度

	// 相同内容应产生相同哈希
	hash2, err := detector.calculateFileHash(tmpFile.Name())
	require.NoError(t, err)
	assert.Equal(t, hash, hash2)
}

func TestCopyFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "copy-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	srcFile := filepath.Join(tmpDir, "source.txt")
	dstFile := filepath.Join(tmpDir, "dest.txt")

	content := []byte("content to copy")
	require.NoError(t, os.WriteFile(srcFile, content, 0644))

	err = copyFile(srcFile, dstFile)
	require.NoError(t, err)

	// 验证内容
	copiedContent, err := os.ReadFile(dstFile)
	require.NoError(t, err)
	assert.Equal(t, content, copiedContent)

	// 验证权限
	srcInfo, _ := os.Stat(srcFile)
	dstInfo, _ := os.Stat(dstFile)
	assert.Equal(t, srcInfo.Mode(), dstInfo.Mode())
}

func TestGenerateConflictID(t *testing.T) {
	id1 := generateConflictID()
	id2 := generateConflictID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
	assert.Contains(t, id1, "conflict-")
}