// Package remotebackup 远程备份引擎模块测试
package remotebackup

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "config.json")
	return NewManager(configPath)
}

func setupTestRouter(mgr *Manager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/remotebackup")
	RegisterRoutes(api, mgr)
	return r
}

// ========== 类型测试 ==========

func TestBackupTargetType_Constants(t *testing.T) {
	assert.Equal(t, BackupTargetType("s3"), TargetTypeS3)
	assert.Equal(t, BackupTargetType("ftp"), TargetTypeFTP)
	assert.Equal(t, BackupTargetType("sftp"), TargetTypeSFTP)
	assert.Equal(t, BackupTargetType("webdav"), TargetTypeWebDAV)
	assert.Equal(t, BackupTargetType("rsync"), TargetTypeRsync)
}

func TestBackupStatus_Constants(t *testing.T) {
	assert.Equal(t, BackupStatus("pending"), StatusPending)
	assert.Equal(t, BackupStatus("running"), StatusRunning)
	assert.Equal(t, BackupStatus("completed"), StatusCompleted)
	assert.Equal(t, BackupStatus("failed"), StatusFailed)
	assert.Equal(t, BackupStatus("cancelled"), StatusCancelled)
	assert.Equal(t, BackupStatus("paused"), StatusPaused)
}

func TestBackupStrategy_Constants(t *testing.T) {
	assert.Equal(t, BackupStrategy("full"), StrategyFull)
	assert.Equal(t, BackupStrategy("incremental"), StrategyIncremental)
	assert.Equal(t, BackupStrategy("differential"), StrategyDifferential)
}

func TestRetentionUnit_Constants(t *testing.T) {
	assert.Equal(t, RetentionUnit("days"), RetentionDays)
	assert.Equal(t, RetentionUnit("versions"), RetentionVersions)
	assert.Equal(t, RetentionUnit("forever"), RetentionForever)
}

// ========== 目标管理测试 ==========

func TestManager_CreateTarget(t *testing.T) {
	mgr := setupTestManager(t)

	t.Run("创建S3目标", func(t *testing.T) {
		target, err := mgr.CreateTarget(&BackupTarget{
			Name:      "test-s3",
			Type:      TargetTypeS3,
			Endpoint:  "https://s3.amazonaws.com",
			Bucket:    "my-bucket",
			AccessKey: "AKID",
			SecretKey: "SECRET",
			Region:    "us-east-1",
		})
		require.NoError(t, err)
		assert.NotEmpty(t, target.ID)
		assert.Equal(t, "test-s3", target.Name)
		assert.Equal(t, TargetTypeS3, target.Type)
	})

	t.Run("创建FTP目标", func(t *testing.T) {
		target, err := mgr.CreateTarget(&BackupTarget{
			Name:     "test-ftp",
			Type:     TargetTypeFTP,
			Endpoint: "ftp.example.com",
			Port:     21,
			Username: "user",
			Password: "pass",
		})
		require.NoError(t, err)
		assert.Equal(t, TargetTypeFTP, target.Type)
	})

	t.Run("名称为空", func(t *testing.T) {
		_, err := mgr.CreateTarget(&BackupTarget{
			Type: TargetTypeS3,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "目标名称不能为空")
	})

	t.Run("类型为空", func(t *testing.T) {
		_, err := mgr.CreateTarget(&BackupTarget{
			Name: "test",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "目标类型不能为空")
	})
}

func TestManager_ListTargets(t *testing.T) {
	mgr := setupTestManager(t)

	// 初始为空
	targets := mgr.ListTargets()
	assert.Empty(t, targets)

	// 创建目标
	_, err := mgr.CreateTarget(&BackupTarget{
		Name:      "target1",
		Type:      TargetTypeS3,
		Endpoint:  "https://s3.amazonaws.com",
		SecretKey: "mysupersecretkey123",
	})
	require.NoError(t, err)

	_, err = mgr.CreateTarget(&BackupTarget{
		Name:     "target2",
		Type:     TargetTypeSFTP,
		Endpoint: "sftp.example.com",
	})
	require.NoError(t, err)

	// 列出
	targets = mgr.ListTargets()
	assert.Len(t, targets, 2)

	// 验证脱敏
	for _, t := range targets {
		if t.SecretKey != "" {
			assert.NotContains(t, t.SecretKey, "supersecret")
		}
	}
}

func TestManager_GetTarget(t *testing.T) {
	mgr := setupTestManager(t)

	created, err := mgr.CreateTarget(&BackupTarget{
		Name:      "target1",
		Type:      TargetTypeS3,
		Endpoint:  "https://s3.amazonaws.com",
		SecretKey: "secret123",
	})
	require.NoError(t, err)

	t.Run("获取存在目标", func(t *testing.T) {
		target, err := mgr.GetTarget(created.ID)
		require.NoError(t, err)
		assert.Equal(t, "target1", target.Name)
		// 验证脱敏
		assert.NotContains(t, target.SecretKey, "secret123")
	})

	t.Run("获取不存在目标", func(t *testing.T) {
		_, err := mgr.GetTarget("nonexistent")
		assert.Error(t)
		assert.Contains(t, err.Error(), "不存在")
	})
}

func TestManager_UpdateTarget(t *testing.T) {
	mgr := setupTestManager(t)

	created, err := mgr.CreateTarget(&BackupTarget{
		Name:     "target1",
		Type:     TargetTypeS3,
		Endpoint: "https://s3.amazonaws.com",
	})
	require.NoError(t, err)

	t.Run("更新成功", func(t *testing.T) {
		updated, err := mgr.UpdateTarget(created.ID, &BackupTarget{
			Name:     "updated-name",
			Endpoint: "https://new-endpoint.com",
		})
		require.NoError(t, err)
		assert.Equal(t, "updated-name", updated.Name)
		assert.Equal(t, "https://new-endpoint.com", updated.Endpoint)
	})

	t.Run("更新不存在目标", func(t *testing.T) {
		_, err := mgr.UpdateTarget("nonexistent", &BackupTarget{
			Name: "test",
		})
		assert.Error(t)
	})
}

func TestManager_DeleteTarget(t *testing.T) {
	mgr := setupTestManager(t)

	created, err := mgr.CreateTarget(&BackupTarget{
		Name:     "target1",
		Type:     TargetTypeS3,
		Endpoint: "https://s3.amazonaws.com",
		Bucket:   "bucket",
	})
	require.NoError(t, err)

	t.Run("删除成功", func(t *testing.T) {
		err := mgr.DeleteTarget(created.ID)
		assert.NoError(t, err)

		// 确认已删除
		targets := mgr.ListTargets()
		assert.Empty(t, targets)
	})

	t.Run("删除不存在", func(t *testing.T) {
		err := mgr.DeleteTarget("nonexistent")
		assert.Error(t)
	})

	t.Run("删除被任务引用的目标", func(t *testing.T) {
		target, _ := mgr.CreateTarget(&BackupTarget{
			Name:     "target2",
			Type:     TargetTypeS3,
			Endpoint: "https://s3.amazonaws.com",
			Bucket:   "bucket",
		})
		_, _ = mgr.CreateJob(&BackupJob{
			Name:        "job1",
			SourcePaths: []string{"/tmp"},
			TargetID:    target.ID,
		})

		err := mgr.DeleteTarget(target.ID)
		assert.Error(t)
		assert.Contains(t, err.Error(), "仍被任务")
	})
}

func TestManager_TestConnection(t *testing.T) {
	mgr := setupTestManager(t)

	target, _ := mgr.CreateTarget(&BackupTarget{
		Name:     "s3-target",
		Type:     TargetTypeS3,
		Endpoint: "https://s3.amazonaws.com",
		Bucket:   "bucket",
	})

	t.Run("测试S3连接", func(t *testing.T) {
		err := mgr.TestConnection(target.ID)
		assert.NoError(t, err)
	})

	t.Run("测试不存在目标", func(t *testing.T) {
		err := mgr.TestConnection("nonexistent")
		assert.Error(t)
	})

	t.Run("测试不完整S3配置", func(t *testing.T) {
		target2, _ := mgr.CreateTarget(&BackupTarget{
			Name: "bad-s3",
			Type: TargetTypeS3,
		})
		err := mgr.TestConnection(target2.ID)
		assert.Error(t)
	})

	t.Run("测试FTP连接", func(t *testing.T) {
		ftpTarget, _ := mgr.CreateTarget(&BackupTarget{
			Name:     "ftp-target",
			Type:     TargetTypeFTP,
			Endpoint: "ftp.example.com",
		})
		err := mgr.TestConnection(ftpTarget.ID)
		assert.NoError(t, err)
	})

	t.Run("测试SFTP连接", func(t *testing.T) {
		sftpTarget, _ := mgr.CreateTarget(&BackupTarget{
			Name:     "sftp-target",
			Type:     TargetTypeSFTP,
			Endpoint: "sftp.example.com",
		})
		err := mgr.TestConnection(sftpTarget.ID)
		assert.NoError(t, err)
	})

	t.Run("测试WebDAV连接", func(t *testing.T) {
		webdavTarget, _ := mgr.CreateTarget(&BackupTarget{
			Name:     "webdav-target",
			Type:     TargetTypeWebDAV,
			Endpoint: "https://webdav.example.com",
		})
		err := mgr.TestConnection(webdavTarget.ID)
		assert.NoError(t, err)
	})

	t.Run("测试Rsync连接", func(t *testing.T) {
		rsyncTarget, _ := mgr.CreateTarget(&BackupTarget{
			Name:     "rsync-target",
			Type:     TargetTypeRsync,
			Endpoint: "rsync.example.com",
		})
		err := mgr.TestConnection(rsyncTarget.ID)
		assert.NoError(t, err)
	})
}

// ========== 任务管理测试 ==========

func TestManager_CreateJob(t *testing.T) {
	mgr := setupTestManager(t)

	target, _ := mgr.CreateTarget(&BackupTarget{
		Name:     "target1",
		Type:     TargetTypeS3,
		Endpoint: "https://s3.amazonaws.com",
		Bucket:   "bucket",
	})

	t.Run("创建成功", func(t *testing.T) {
		job, err := mgr.CreateJob(&BackupJob{
			Name:        "test-job",
			SourcePaths: []string{"/tmp/test"},
			TargetID:    target.ID,
			Strategy:    StrategyFull,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, job.ID)
		assert.Equal(t, StatusPending, job.Status)
	})

	t.Run("名称为空", func(t *testing.T) {
		_, err := mgr.CreateJob(&BackupJob{
			SourcePaths: []string{"/tmp"},
			TargetID:    target.ID,
		})
		assert.Error(t)
		assert.Contains(t, err.Error(), "任务名称不能为空")
	})

	t.Run("源路径为空", func(t *testing.T) {
		_, err := mgr.CreateJob(&BackupJob{
			Name:     "test",
			TargetID: target.ID,
		})
		assert.Error(t)
		assert.Contains(t, err.Error(), "源路径不能为空")
	})

	t.Run("目标ID为空", func(t *testing.T) {
		_, err := mgr.CreateJob(&BackupJob{
			Name:        "test",
			SourcePaths: []string{"/tmp"},
		})
		assert.Error(t)
		assert.Contains(t, err.Error(), "目标ID不能为空")
	})

	t.Run("目标不存在", func(t *testing.T) {
		_, err := mgr.CreateJob(&BackupJob{
			Name:        "test",
			SourcePaths: []string{"/tmp"},
			TargetID:    "nonexistent",
		})
		assert.Error(t)
		assert.Contains(t, err.Error(), "不存在")
	})
}

func TestManager_ListJobs(t *testing.T) {
	mgr := setupTestManager(t)

	target, _ := mgr.CreateTarget(&BackupTarget{
		Name:     "target1",
		Type:     TargetTypeS3,
		Endpoint: "https://s3.amazonaws.com",
		Bucket:   "bucket",
	})

	// 初始为空
	jobs := mgr.ListJobs()
	assert.Empty(t, jobs)

	// 创建任务
	_, _ = mgr.CreateJob(&BackupJob{
		Name:        "job1",
		SourcePaths: []string{"/tmp"},
		TargetID:    target.ID,
	})
	_, _ = mgr.CreateJob(&BackupJob{
		Name:        "job2",
		SourcePaths: []string{"/tmp"},
		TargetID:    target.ID,
	})

	jobs = mgr.ListJobs()
	assert.Len(t, jobs, 2)
}

func TestManager_RunJob(t *testing.T) {
	mgr := setupTestManager(t)

	// 创建临时目录和文件
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("hello world"), 0644)

	target, _ := mgr.CreateTarget(&BackupTarget{
		Name:     "target1",
		Type:     TargetTypeS3,
		Endpoint: "https://s3.amazonaws.com",
		Bucket:   "bucket",
	})

	job, _ := mgr.CreateJob(&BackupJob{
		Name:        "test-job",
		SourcePaths: []string{tmpDir},
		TargetID:    target.ID,
		Strategy:    StrategyFull,
	})

	t.Run("执行成功", func(t *testing.T) {
		version, err := mgr.RunJob(context.Background(), job.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, version.ID)
		assert.Equal(t, StatusCompleted, version.Status)
		assert.Greater(t, version.TotalSize, int64(0))
		assert.NotEmpty(t, version.Checksum)
	})

	t.Run("重复运行失败（已取消/已完成可重新运行）", func(t *testing.T) {
		// 任务已完成，可以再次运行
		_, err := mgr.RunJob(context.Background(), job.ID)
		assert.NoError(t, err)
	})

	t.Run("不存在任务", func(t *testing.T) {
		_, err := mgr.RunJob(context.Background(), "nonexistent")
		assert.Error(t)
	})
}

func TestManager_CancelJob(t *testing.T) {
	mgr := setupTestManager(t)

	target, _ := mgr.CreateTarget(&BackupTarget{
		Name:     "target1",
		Type:     TargetTypeS3,
		Endpoint: "https://s3.amazonaws.com",
		Bucket:   "bucket",
	})

	job, _ := mgr.CreateJob(&BackupJob{
		Name:        "test-job",
		SourcePaths: []string{"/tmp"},
		TargetID:    target.ID,
	})

	t.Run("取消未运行任务", func(t *testing.T) {
		err := mgr.CancelJob(job.ID)
		assert.Error(t)
		assert.Contains(t, err.Error(), "未在运行中")
	})

	t.Run("取消不存在任务", func(t *testing.T) {
		err := mgr.CancelJob("nonexistent")
		assert.Error(t)
	})
}

func TestManager_ListVersions(t *testing.T) {
	mgr := setupTestManager(t)

	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("data"), 0644)

	target, _ := mgr.CreateTarget(&BackupTarget{
		Name:     "target1",
		Type:     TargetTypeS3,
		Endpoint: "https://s3.amazonaws.com",
		Bucket:   "bucket",
	})

	job, _ := mgr.CreateJob(&BackupJob{
		Name:        "test-job",
		SourcePaths: []string{tmpDir},
		TargetID:    target.ID,
	})

	// 初始无版本
	versions, err := mgr.ListVersions(job.ID)
	require.NoError(t, err)
	assert.Empty(t, versions)

	// 执行备份后有版本
	_, _ = mgr.RunJob(context.Background(), job.ID)
	versions, _ = mgr.ListVersions(job.ID)
	assert.Len(t, versions, 1)

	// 不存在任务
	_, err = mgr.ListVersions("nonexistent")
	assert.Error(t)
}

func TestManager_Restore(t *testing.T) {
	mgr := setupTestManager(t)

	tmpDir := t.TempDir()
	restoreDir := filepath.Join(t.TempDir(), "restore")
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("data"), 0644)

	target, _ := mgr.CreateTarget(&BackupTarget{
		Name:     "target1",
		Type:     TargetTypeS3,
		Endpoint: "https://s3.amazonaws.com",
		Bucket:   "bucket",
	})

	job, _ := mgr.CreateJob(&BackupJob{
		Name:        "test-job",
		SourcePaths: []string{tmpDir},
		TargetID:    target.ID,
	})

	// 先执行备份
	_, _ = mgr.RunJob(context.Background(), job.ID)

	t.Run("恢复成功", func(t *testing.T) {
		result, err := mgr.Restore(&RestoreRequest{
			JobID:       job.ID,
			RestorePath: restoreDir,
		})
		require.NoError(t, err)
		assert.Equal(t, job.ID, result.JobID)
	})

	t.Run("指定版本恢复", func(t *testing.T) {
		versions, _ := mgr.ListVersions(job.ID)
		result, err := mgr.Restore(&RestoreRequest{
			JobID:       job.ID,
			VersionID:   versions[0].ID,
			RestorePath: restoreDir,
		})
		require.NoError(t, err)
		assert.Equal(t, versions[0].ID, result.VersionID)
	})

	t.Run("恢复不存在任务", func(t *testing.T) {
		_, err := mgr.Restore(&RestoreRequest{
			JobID:       "nonexistent",
			RestorePath: restoreDir,
		})
		assert.Error(t)
	})

	t.Run("无版本恢复", func(t *testing.T) {
		job2, _ := mgr.CreateJob(&BackupJob{
			Name:        "empty-job",
			SourcePaths: []string{"/tmp"},
			TargetID:    target.ID,
		})
		_, err := mgr.Restore(&RestoreRequest{
			JobID:       job2.ID,
			RestorePath: restoreDir,
		})
		assert.Error(t)
		assert.Contains(t, err.Error(), "没有可用的备份版本")
	})
}

func TestManager_GetRestorePoints(t *testing.T) {
	mgr := setupTestManager(t)

	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("data"), 0644)

	target, _ := mgr.CreateTarget(&BackupTarget{
		Name:     "target1",
		Type:     TargetTypeS3,
		Endpoint: "https://s3.amazonaws.com",
		Bucket:   "bucket",
	})

	job, _ := mgr.CreateJob(&BackupJob{
		Name:        "test-job",
		SourcePaths: []string{tmpDir},
		TargetID:    target.ID,
	})

	// 执行备份
	_, _ = mgr.RunJob(context.Background(), job.ID)

	t.Run("获取恢复点", func(t *testing.T) {
		points, err := mgr.GetRestorePoints(job.ID)
		require.NoError(t, err)
		assert.Len(t, points, 1)
	})

	t.Run("不存在任务", func(t *testing.T) {
		_, err := mgr.GetRestorePoints("nonexistent")
		assert.Error(t)
	})
}

// ========== 加密测试 ==========

func TestManager_EncryptDecrypt(t *testing.T) {
	mgr := setupTestManager(t)

	original := []byte("这是一段测试数据，用于验证加密解密功能")
	passphrase := "my-secret-passphrase"

	// 加密
	encrypted, err := mgr.EncryptData(original, passphrase)
	require.NoError(t, err)
	assert.NotEqual(t, original, encrypted)

	// 解密
	decrypted, err := mgr.DecryptData(encrypted, passphrase)
	require.NoError(t, err)
	assert.Equal(t, original, decrypted)

	// 错误密码
	_, err = mgr.DecryptData(encrypted, "wrong-password")
	assert.Error(t)
}

func TestManager_ComputeSHA256(t *testing.T) {
	mgr := setupTestManager(t)

	hash := mgr.ComputeSHA256([]byte("test data"))
	assert.Len(t, hash, 64) // SHA-256 hex string

	// 相同输入相同输出
	hash2 := mgr.ComputeSHA256([]byte("test data"))
	assert.Equal(t, hash, hash2)

	// 不同输入不同输出
	hash3 := mgr.ComputeSHA256([]byte("different data"))
	assert.NotEqual(t, hash, hash3)
}

// ========== 配置持久化测试 ==========

func TestManager_ConfigPersistence(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")

	// 创建管理器并添加数据
	mgr1 := NewManager(configPath)
	target, _ := mgr1.CreateTarget(&BackupTarget{
		Name:     "persist-target",
		Type:     TargetTypeS3,
		Endpoint: "https://s3.amazonaws.com",
		Bucket:   "bucket",
	})
	_, _ = mgr1.CreateJob(&BackupJob{
		Name:        "persist-job",
		SourcePaths: []string{"/tmp"},
		TargetID:    target.ID,
	})

	// 创建新管理器加载配置
	mgr2 := NewManager(configPath)

	targets := mgr2.ListTargets()
	assert.Len(t, targets, 1)
	assert.Equal(t, "persist-target", targets[0].Name)

	jobs := mgr2.ListJobs()
	assert.Len(t, jobs, 1)
	assert.Equal(t, "persist-job", jobs[0].Name)
}

// ========== HTTP API 测试 ==========

func TestAPI_ListTargets(t *testing.T) {
	mgr := setupTestManager(t)
	r := setupTestRouter(mgr)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/remotebackup/targets", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 0, resp.Code)
}

func TestAPI_CreateTarget(t *testing.T) {
	mgr := setupTestManager(t)
	r := setupTestRouter(mgr)

	body := `{"name":"api-target","type":"s3","endpoint":"https://s3.amazonaws.com","bucket":"bucket"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/remotebackup/targets", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 0, resp.Code)
}

func TestAPI_CreateTarget_BadRequest(t *testing.T) {
	mgr := setupTestManager(t)
	r := setupTestRouter(mgr)

	t.Run("无效JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/remotebackup/targets", bytes.NewBufferString("invalid"))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("缺少必填字段", func(t *testing.T) {
		body := `{"type":"s3"}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/remotebackup/targets", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestAPI_UpdateTarget(t *testing.T) {
	mgr := setupTestManager(t)
	r := setupTestRouter(mgr)

	target, _ := mgr.CreateTarget(&BackupTarget{
		Name:     "target1",
		Type:     TargetTypeS3,
		Endpoint: "https://s3.amazonaws.com",
		Bucket:   "bucket",
	})

	body := `{"name":"updated-name"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/remotebackup/targets/"+target.ID, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPI_DeleteTarget(t *testing.T) {
	mgr := setupTestManager(t)
	r := setupTestRouter(mgr)

	target, _ := mgr.CreateTarget(&BackupTarget{
		Name:     "target1",
		Type:     TargetTypeS3,
		Endpoint: "https://s3.amazonaws.com",
		Bucket:   "bucket",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/remotebackup/targets/"+target.ID, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPI_TestConnection(t *testing.T) {
	mgr := setupTestManager(t)
	r := setupTestRouter(mgr)

	target, _ := mgr.CreateTarget(&BackupTarget{
		Name:     "target1",
		Type:     TargetTypeS3,
		Endpoint: "https://s3.amazonaws.com",
		Bucket:   "bucket",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/remotebackup/targets/"+target.ID+"/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPI_ListJobs(t *testing.T) {
	mgr := setupTestManager(t)
	r := setupTestRouter(mgr)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/remotebackup/jobs", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPI_CreateJob(t *testing.T) {
	mgr := setupTestManager(t)
	r := setupTestRouter(mgr)

	target, _ := mgr.CreateTarget(&BackupTarget{
		Name:     "target1",
		Type:     TargetTypeS3,
		Endpoint: "https://s3.amazonaws.com",
		Bucket:   "bucket",
	})

	body := fmt.Sprintf(`{"name":"api-job","source_paths":["/tmp"],"target_id":"%s"}`, target.ID)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/remotebackup/jobs", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPI_RunJob(t *testing.T) {
	mgr := setupTestManager(t)
	r := setupTestRouter(mgr)

	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("data"), 0644)

	target, _ := mgr.CreateTarget(&BackupTarget{
		Name:     "target1",
		Type:     TargetTypeS3,
		Endpoint: "https://s3.amazonaws.com",
		Bucket:   "bucket",
	})

	job, _ := mgr.CreateJob(&BackupJob{
		Name:        "test-job",
		SourcePaths: []string{tmpDir},
		TargetID:    target.ID,
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/remotebackup/jobs/"+job.ID+"/run", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPI_CancelJob(t *testing.T) {
	mgr := setupTestManager(t)
	r := setupTestRouter(mgr)

	target, _ := mgr.CreateTarget(&BackupTarget{
		Name:     "target1",
		Type:     TargetTypeS3,
		Endpoint: "https://s3.amazonaws.com",
		Bucket:   "bucket",
	})

	job, _ := mgr.CreateJob(&BackupJob{
		Name:        "test-job",
		SourcePaths: []string{"/tmp"},
		TargetID:    target.ID,
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/remotebackup/jobs/"+job.ID+"/cancel", nil)
	r.ServeHTTP(w, req)

	// 任务未运行，应返回错误
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAPI_ListVersions(t *testing.T) {
	mgr := setupTestManager(t)
	r := setupTestRouter(mgr)

	target, _ := mgr.CreateTarget(&BackupTarget{
		Name:     "target1",
		Type:     TargetTypeS3,
		Endpoint: "https://s3.amazonaws.com",
		Bucket:   "bucket",
	})

	job, _ := mgr.CreateJob(&BackupJob{
		Name:        "test-job",
		SourcePaths: []string{"/tmp"},
		TargetID:    target.ID,
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/remotebackup/jobs/"+job.ID+"/versions", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPI_Restore(t *testing.T) {
	mgr := setupTestManager(t)
	r := setupTestRouter(mgr)

	tmpDir := t.TempDir()
	restoreDir := filepath.Join(t.TempDir(), "restore")
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("data"), 0644)

	target, _ := mgr.CreateTarget(&BackupTarget{
		Name:     "target1",
		Type:     TargetTypeS3,
		Endpoint: "https://s3.amazonaws.com",
		Bucket:   "bucket",
	})

	job, _ := mgr.CreateJob(&BackupJob{
		Name:        "test-job",
		SourcePaths: []string{tmpDir},
		TargetID:    target.ID,
	})

	// 先执行备份
	_, _ = mgr.RunJob(context.Background(), job.ID)

	body := fmt.Sprintf(`{"job_id":"%s","restore_path":"%s"}`, job.ID, restoreDir)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/remotebackup/restore", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPI_Restore_BadRequest(t *testing.T) {
	mgr := setupTestManager(t)
	r := setupTestRouter(mgr)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/remotebackup/restore", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ========== 辅助函数测试 ==========

func TestMaskString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"abc", "***"},
		{"abcd", "****"},
		{"abcde", "ab*de"},
		{"abcdef", "ab**ef"},
		{"abcdefgh", "ab****gh"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, maskString(tt.input))
		})
	}
}

func TestTransferStats(t *testing.T) {
	stats := TransferStats{
		SpeedBytesPerSec: 1024 * 1024,
		TransferredBytes: 512 * 1024,
		TotalBytes:       1024 * 1024,
		RemainingTimeSec: 1,
		StartTime:        time.Now(),
	}

	assert.Equal(t, int64(1024*1024), stats.SpeedBytesPerSec)
	assert.Equal(t, int64(512*1024), stats.TransferredBytes)
	assert.Equal(t, int64(1024*1024), stats.TotalBytes)
}

func TestEncryptionConfig(t *testing.T) {
	config := EncryptionConfig{
		Enabled:    true,
		Algorithm:  "aes-256-gcm",
		KeyID:      "key-1",
		Passphrase: "secret",
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, "aes-256-gcm", config.Algorithm)
}

func TestRetentionPolicy(t *testing.T) {
	policy := RetentionPolicy{
		Unit:  RetentionVersions,
		Value: 7,
	}

	assert.Equal(t, RetentionVersions, policy.Unit)
	assert.Equal(t, 7, policy.Value)
}

func TestBackupSchedule(t *testing.T) {
	nextRun := time.Now().Add(1 * time.Hour)
	schedule := BackupSchedule{
		Enabled:  true,
		Cron:     "0 2 * * *",
		Interval: 86400,
		NextRun:  &nextRun,
	}

	assert.True(t, schedule.Enabled)
	assert.Equal(t, "0 2 * * *", schedule.Cron)
	assert.NotNil(t, schedule.NextRun)
}

func TestRestoreRequest(t *testing.T) {
	req := RestoreRequest{
		JobID:       "job-1",
		VersionID:   "ver-1",
		RestorePath: "/tmp/restore",
		Files:       []string{"file1.txt", "file2.txt"},
		Overwrite:   true,
	}

	assert.Equal(t, "job-1", req.JobID)
	assert.Equal(t, "ver-1", req.VersionID)
	assert.True(t, req.Overwrite)
	assert.Len(t, req.Files, 2)
}
