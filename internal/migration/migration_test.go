package migration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== 任务创建测试 ==========

func TestCreateTask_Success(t *testing.T) {
	m := NewManager()

	task, err := m.CreateTask(&CreateRequest{
		Name:         "测试迁移",
		SourceDevice: "nas-001",
		TargetDevice: "nas-002",
		SourcePath:   "/data/shared",
		TargetPath:   "/data/shared",
		Mode:         ModeFull,
	})

	require.NoError(t, err)
	assert.NotEmpty(t, task.ID)
	assert.Equal(t, "测试迁移", task.Name)
	assert.Equal(t, "nas-001", task.SourceDevice)
	assert.Equal(t, "nas-002", task.TargetDevice)
	assert.Equal(t, ModeFull, task.Mode)
	assert.Equal(t, StatusPending, task.Status)
	assert.Equal(t, 0, task.Progress)
}

func TestCreateTask_MissingFields(t *testing.T) {
	m := NewManager()

	tests := []struct {
		name string
		req  *CreateRequest
	}{
		{
			name: "缺少源设备",
			req: &CreateRequest{
				Name:         "test",
				TargetDevice: "nas-002",
				SourcePath:   "/data",
				TargetPath:   "/data",
			},
		},
		{
			name: "缺少目标设备",
			req: &CreateRequest{
				Name:         "test",
				SourceDevice: "nas-001",
				SourcePath:   "/data",
				TargetPath:   "/data",
			},
		},
		{
			name: "缺少源路径",
			req: &CreateRequest{
				Name:         "test",
				SourceDevice: "nas-001",
				TargetDevice: "nas-002",
				TargetPath:   "/data",
			},
		},
		{
			name: "缺少目标路径",
			req: &CreateRequest{
				Name:         "test",
				SourceDevice: "nas-001",
				TargetDevice: "nas-002",
				SourcePath:   "/data",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := m.CreateTask(tt.req)
			assert.Error(t, err)
		})
	}
}

func TestCreateTask_DefaultMode(t *testing.T) {
	m := NewManager()

	task, err := m.CreateTask(&CreateRequest{
		Name:         "default mode",
		SourceDevice: "nas-001",
		TargetDevice: "nas-002",
		SourcePath:   "/data",
		TargetPath:   "/data",
	})
	require.NoError(t, err)
	assert.Equal(t, ModeFull, task.Mode)
}

// ========== 预扫描测试 ==========

func TestScan_Success(t *testing.T) {
	m := NewManager()
	task, _ := m.CreateTask(&CreateRequest{
		Name:         "scan test",
		SourceDevice: "nas-001",
		TargetDevice: "nas-002",
		SourcePath:   "/data",
		TargetPath:   "/data",
	})

	m.SetScanFunc(func(_ context.Context, _ string) (*ScanResult, error) {
		return &ScanResult{
			TotalSize:    5 * 1024 * 1024 * 1024,
			TotalFiles:   5000,
			EstimatedSec: 300,
		}, nil
	})

	result, err := m.Scan(context.Background(), task.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(5*1024*1024*1024), result.TotalSize)
	assert.Equal(t, int64(5000), result.TotalFiles)

	updated, _ := m.GetTask(task.ID)
	assert.Equal(t, StatusPending, updated.Status)
	assert.Equal(t, result.TotalSize, updated.TotalSize)
}

func TestScan_NotFound(t *testing.T) {
	m := NewManager()
	_, err := m.Scan(context.Background(), "non-existent")
	assert.ErrorIs(t, err, ErrTaskNotFound)
}

func TestScan_Error(t *testing.T) {
	m := NewManager()
	task, _ := m.CreateTask(&CreateRequest{
		Name:         "scan error",
		SourceDevice: "nas-001",
		TargetDevice: "nas-002",
		SourcePath:   "/data",
		TargetPath:   "/data",
	})

	scanErr := errors.New("设备离线")
	m.SetScanFunc(func(_ context.Context, _ string) (*ScanResult, error) {
		return nil, scanErr
	})

	_, err := m.Scan(context.Background(), task.ID)
	assert.Error(t, err)
	assert.Equal(t, scanErr, err)

	updated, _ := m.GetTask(task.ID)
	assert.Equal(t, StatusFailed, updated.Status)
}

func TestScan_IncrementalResult(t *testing.T) {
	m := NewManager()
	task, _ := m.CreateTask(&CreateRequest{
		Name:         "incremental scan",
		SourceDevice: "nas-001",
		TargetDevice: "nas-002",
		SourcePath:   "/data",
		TargetPath:   "/data",
		Mode:         ModeIncremental,
	})

	m.SetScanFunc(func(_ context.Context, _ string) (*ScanResult, error) {
		return &ScanResult{
			TotalSize:    10 * 1024 * 1024 * 1024,
			TotalFiles:   10000,
			Incremental:  true,
			ChangedFiles: 200,
			ChangedSize:  500 * 1024 * 1024,
			EstimatedSec: 30,
		}, nil
	})

	result, err := m.Scan(context.Background(), task.ID)
	require.NoError(t, err)
	assert.True(t, result.Incremental)
	assert.Equal(t, int64(200), result.ChangedFiles)
	assert.Equal(t, int64(500*1024*1024), result.ChangedSize)
}

// ========== 迁移执行测试 ==========

func TestStart_Success(t *testing.T) {
	m := NewManager()
	task, _ := m.CreateTask(&CreateRequest{
		Name:         "start test",
		SourceDevice: "nas-001",
		TargetDevice: "nas-002",
		SourcePath:   "/data",
		TargetPath:   "/data",
	})

	transferDone := make(chan struct{})
	m.SetTransferFunc(func(ctx context.Context, src, dst string, progress func(int64)) error {
		defer close(transferDone)
		progress(5 * 1024 * 1024)
		progress(5 * 1024 * 1024)
		return nil
	})

	err := m.Start(task.ID)
	require.NoError(t, err)

	select {
	case <-transferDone:
		time.Sleep(50 * time.Millisecond)
	case <-time.After(5 * time.Second):
		t.Fatal("迁移超时")
	}

	updated, _ := m.GetTask(task.ID)
	assert.Equal(t, StatusCompleted, updated.Status)
	assert.Equal(t, 100, updated.Progress)
	assert.NotEmpty(t, updated.SnapshotID)
}

func TestStart_AlreadyRunning(t *testing.T) {
	m := NewManager()
	task, _ := m.CreateTask(&CreateRequest{
		Name:         "duplicate test",
		SourceDevice: "nas-001",
		TargetDevice: "nas-002",
		SourcePath:   "/data",
		TargetPath:   "/data",
	})

	started := make(chan struct{})
	m.SetTransferFunc(func(ctx context.Context, _, _ string, _ func(int64)) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})

	go m.Start(task.ID)
	<-started
	time.Sleep(20 * time.Millisecond)

	err := m.Start(task.ID)
	assert.ErrorIs(t, err, ErrAlreadyRunning)

	m.Cancel(task.ID)
}

func TestStart_TransferError(t *testing.T) {
	m := NewManager()
	task, _ := m.CreateTask(&CreateRequest{
		Name:         "transfer error",
		SourceDevice: "nas-001",
		TargetDevice: "nas-002",
		SourcePath:   "/data",
		TargetPath:   "/data",
	})

	transferErr := errors.New("网络中断")
	m.SetTransferFunc(func(ctx context.Context, _, _ string, _ func(int64)) error {
		return transferErr
	})

	done := make(chan struct{})
	go func() {
		m.Start(task.ID)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("超时")
	}

	time.Sleep(50 * time.Millisecond)
	updated, _ := m.GetTask(task.ID)
	assert.Equal(t, StatusFailed, updated.Status)
	assert.Equal(t, "网络中断", updated.Error)
}

// ========== 取消测试 ==========

func TestCancel_Success(t *testing.T) {
	m := NewManager()
	task, _ := m.CreateTask(&CreateRequest{
		Name:         "cancel test",
		SourceDevice: "nas-001",
		TargetDevice: "nas-002",
		SourcePath:   "/data",
		TargetPath:   "/data",
	})

	started := make(chan struct{})
	m.SetTransferFunc(func(ctx context.Context, _, _ string, _ func(int64)) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})

	go m.Start(task.ID)
	<-started
	time.Sleep(20 * time.Millisecond)

	err := m.Cancel(task.ID)
	assert.NoError(t, err)

	time.Sleep(50 * time.Millisecond)
	updated, _ := m.GetTask(task.ID)
	assert.Equal(t, StatusCancelled, updated.Status)
}

func TestCancel_NotRunning(t *testing.T) {
	m := NewManager()
	task, _ := m.CreateTask(&CreateRequest{
		Name:         "not running",
		SourceDevice: "nas-001",
		TargetDevice: "nas-002",
		SourcePath:   "/data",
		TargetPath:   "/data",
	})

	err := m.Cancel(task.ID)
	assert.ErrorIs(t, err, ErrTaskNotRunning)
}

// ========== 回滚测试 ==========

func TestRollback_Success(t *testing.T) {
	m := NewManager()
	task, _ := m.CreateTask(&CreateRequest{
		Name:         "rollback test",
		SourceDevice: "nas-001",
		TargetDevice: "nas-002",
		SourcePath:   "/data",
		TargetPath:   "/data",
	})

	// 先完成迁移
	m.SetTransferFunc(func(ctx context.Context, _, _ string, progress func(int64)) error {
		progress(10 * 1024 * 1024)
		return nil
	})

	done := make(chan struct{})
	go func() { m.Start(task.ID); close(done) }()
	<-done

	time.Sleep(50 * time.Millisecond)
	updated, _ := m.GetTask(task.ID)
	assert.NotEmpty(t, updated.SnapshotID)

	err := m.Rollback(task.ID)
	assert.NoError(t, err)

	rolled, _ := m.GetTask(task.ID)
	assert.Equal(t, StatusRolledBack, rolled.Status)
	assert.Equal(t, 0, rolled.Progress)
	assert.Empty(t, rolled.Error)
}

func TestRollback_NoSnapshot(t *testing.T) {
	m := NewManager()
	task, _ := m.CreateTask(&CreateRequest{
		Name:         "no snapshot",
		SourceDevice: "nas-001",
		TargetDevice: "nas-002",
		SourcePath:   "/data",
		TargetPath:   "/data",
	})

	err := m.Rollback(task.ID)
	assert.ErrorIs(t, err, ErrNoSnapshot)
}

func TestRollback_NotFound(t *testing.T) {
	m := NewManager()
	err := m.Rollback("non-existent")
	assert.ErrorIs(t, err, ErrTaskNotFound)
}

// ========== 验证测试 ==========

func TestVerify_Success(t *testing.T) {
	m := NewManager()
	task, _ := m.CreateTask(&CreateRequest{
		Name:         "verify test",
		SourceDevice: "nas-001",
		TargetDevice: "nas-002",
		SourcePath:   "/data",
		TargetPath:   "/data",
	})

	m.SetVerifyFunc(func(_ context.Context, _, _ string) (*VerifyResult, error) {
		return &VerifyResult{
			Valid:        true,
			CheckedFiles: 100,
			Mismatches:   0,
			Duration:     1500,
		}, nil
	})

	result, err := m.Verify(context.Background(), task.ID)
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Equal(t, int64(100), result.CheckedFiles)
	assert.Equal(t, int64(0), result.Mismatches)

	updated, _ := m.GetTask(task.ID)
	assert.Equal(t, StatusCompleted, updated.Status)
}

func TestVerify_Mismatches(t *testing.T) {
	m := NewManager()
	task, _ := m.CreateTask(&CreateRequest{
		Name:         "verify mismatch",
		SourceDevice: "nas-001",
		TargetDevice: "nas-002",
		SourcePath:   "/data",
		TargetPath:   "/data",
	})

	m.SetVerifyFunc(func(_ context.Context, _, _ string) (*VerifyResult, error) {
		return &VerifyResult{
			Valid:        false,
			CheckedFiles: 100,
			Mismatches:   3,
			Duration:     2000,
		}, nil
	})

	result, err := m.Verify(context.Background(), task.ID)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Equal(t, int64(3), result.Mismatches)

	updated, _ := m.GetTask(task.ID)
	assert.Equal(t, StatusFailed, updated.Status)
	assert.Contains(t, updated.Error, "3")
}

func TestVerify_Error(t *testing.T) {
	m := NewManager()
	task, _ := m.CreateTask(&CreateRequest{
		Name:         "verify error",
		SourceDevice: "nas-001",
		TargetDevice: "nas-002",
		SourcePath:   "/data",
		TargetPath:   "/data",
	})

	verifyErr := errors.New("连接超时")
	m.SetVerifyFunc(func(_ context.Context, _, _ string) (*VerifyResult, error) {
		return nil, verifyErr
	})

	_, err := m.Verify(context.Background(), task.ID)
	assert.Error(t, err)

	updated, _ := m.GetTask(task.ID)
	assert.Equal(t, StatusFailed, updated.Status)
}

// ========== 查询测试 ==========

func TestGetTask_Success(t *testing.T) {
	m := NewManager()
	task, _ := m.CreateTask(&CreateRequest{
		Name:         "get test",
		SourceDevice: "nas-001",
		TargetDevice: "nas-002",
		SourcePath:   "/data",
		TargetPath:   "/data",
	})

	got, err := m.GetTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, task.ID, got.ID)
	assert.Equal(t, task.Name, got.Name)
}

func TestGetTask_NotFound(t *testing.T) {
	m := NewManager()
	_, err := m.GetTask("non-existent")
	assert.ErrorIs(t, err, ErrTaskNotFound)
}

func TestListTasks(t *testing.T) {
	m := NewManager()

	m.CreateTask(&CreateRequest{
		Name: "task-1", SourceDevice: "s1", TargetDevice: "t1",
		SourcePath: "/a", TargetPath: "/a",
	})
	m.CreateTask(&CreateRequest{
		Name: "task-2", SourceDevice: "s2", TargetDevice: "t2",
		SourcePath: "/b", TargetPath: "/b",
	})

	tasks := m.ListTasks()
	assert.Len(t, tasks, 2)
}

// ========== 删除测试 ==========

func TestDeleteTask_Success(t *testing.T) {
	m := NewManager()
	task, _ := m.CreateTask(&CreateRequest{
		Name: "delete test", SourceDevice: "s1", TargetDevice: "t1",
		SourcePath: "/a", TargetPath: "/a",
	})

	err := m.DeleteTask(task.ID)
	assert.NoError(t, err)

	_, err = m.GetTask(task.ID)
	assert.ErrorIs(t, err, ErrTaskNotFound)
}

func TestDeleteTask_NotFound(t *testing.T) {
	m := NewManager()
	err := m.DeleteTask("non-existent")
	assert.ErrorIs(t, err, ErrTaskNotFound)
}

// ========== 进度跟踪测试 ==========

func TestProgressTracking(t *testing.T) {
	m := NewManager()
	task, _ := m.CreateTask(&CreateRequest{
		Name: "progress test", SourceDevice: "s1", TargetDevice: "t1",
		SourcePath: "/a", TargetPath: "/a",
	})

	// 先设置扫描
	m.SetScanFunc(func(_ context.Context, _ string) (*ScanResult, error) {
		return &ScanResult{TotalSize: 4 * 1024 * 1024, TotalFiles: 4}, nil
	})
	m.Scan(context.Background(), task.ID)

	// 然后执行迁移，每次传输 1MB
	m.SetTransferFunc(func(ctx context.Context, _, _ string, progress func(int64)) error {
		progress(1024 * 1024)
		progress(1024 * 1024)
		progress(1024 * 1024)
		progress(1024 * 1024)
		return nil
	})

	done := make(chan struct{})
	go func() { m.Start(task.ID); close(done) }()
	<-done

	time.Sleep(50 * time.Millisecond)
	updated, _ := m.GetTask(task.ID)
	assert.Equal(t, 100, updated.Progress)
	assert.Equal(t, int64(4*1024*1024), updated.Transferred)
	assert.True(t, updated.Speed > 0)
	assert.NotZero(t, updated.StartedAt)
	assert.NotZero(t, updated.FinishedAt)
}

// ========== 错误类型测试 ==========

func TestMigrationError_Interfaces(t *testing.T) {
	err := &MigrationError{Code: 404, Message: "not found"}
	assert.True(t, err.NotFound())
	assert.False(t, err.BadRequest())
	assert.False(t, err.Conflict())

	err2 := &MigrationError{Code: 400, Message: "bad"}
	assert.True(t, err2.BadRequest())

	err3 := &MigrationError{Code: 409, Message: "conflict"}
	assert.True(t, err3.Conflict())
}

func TestMigrationError_ErrorString(t *testing.T) {
	baseErr := errors.New("底层错误")
	err := &MigrationError{Code: 500, Message: "内部错误", Err: baseErr}
	assert.Contains(t, err.Error(), "内部错误")
	assert.Contains(t, err.Error(), "底层错误")
	assert.Equal(t, baseErr, err.Unwrap())
}

func TestMigrationError_ErrorNoWrap(t *testing.T) {
	err := &MigrationError{Code: 500, Message: "简单错误"}
	assert.Equal(t, "简单错误", err.Error())
	assert.Nil(t, err.Unwrap())
}

// ========== 校验和测试 ==========

func TestComputeChecksum(t *testing.T) {
	r := strings.NewReader("hello world")
	hash, err := ComputeChecksum(r)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.Len(t, hash, 64) // SHA-256 hex = 64 chars
}

func TestComputeChecksum_SameInput(t *testing.T) {
	r1 := strings.NewReader("test data")
	r2 := strings.NewReader("test data")
	h1, _ := ComputeChecksum(r1)
	h2, _ := ComputeChecksum(r2)
	assert.Equal(t, h1, h2)
}

func TestComputeChecksum_DifferentInput(t *testing.T) {
	r1 := strings.NewReader("data A")
	r2 := strings.NewReader("data B")
	h1, _ := ComputeChecksum(r1)
	h2, _ := ComputeChecksum(r2)
	assert.NotEqual(t, h1, h2)
}
