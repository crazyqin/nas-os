package ftp

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTransferLogger(t *testing.T) {
	// 创建临时日志文件
	tmpDir := t.TempDir()
	logPath := tmpDir + "/transfers.jsonl"

	config := TransferLoggerConfig{
		LogPath:   logPath,
		MaxLogs:   100,
		MaxSize:   1024 * 1024,
		Retention: 24 * time.Hour,
		Enabled:   true,
	}

	logger, err := NewTransferLogger(config)
	assert.NoError(t, err)
	assert.NotNil(t, logger)
	defer logger.Close()

	// 测试记录日志
	log := &TransferLog{
		ID:        "test-001",
		Timestamp: time.Now(),
		Username:  "testuser",
		ClientIP:  "192.168.1.100",
		Direction: "upload",
		FilePath:  "/test/file.txt",
		FileSize:  1024,
		BytesTrans: 1024,
		Duration:  1000,
		Success:   true,
		Bandwidth: 8192,
	}

	logger.Log(log)

	// 获取日志
	logs := logger.GetLogs(10, 0, nil)
	assert.Len(t, logs, 1)
	assert.Equal(t, "test-001", logs[0].ID)
	assert.Equal(t, "testuser", logs[0].Username)
	assert.Equal(t, "upload", logs[0].Direction)
}

func TestTransferLoggerFilter(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := tmpDir + "/transfers.jsonl"

	config := TransferLoggerConfig{
		LogPath: logPath,
		MaxLogs: 100,
		Enabled: true,
	}

	logger, err := NewTransferLogger(config)
	assert.NoError(t, err)
	defer logger.Close()

	// 记录多条日志
	now := time.Now()
	logs := []*TransferLog{
		{
			ID:        "1",
			Timestamp: now.Add(-2 * time.Hour),
			Username:  "user1",
			Direction: "upload",
			BytesTrans: 1000,
			Success:   true,
		},
		{
			ID:        "2",
			Timestamp: now.Add(-1 * time.Hour),
			Username:  "user1",
			Direction: "download",
			BytesTrans: 2000,
			Success:   true,
		},
		{
			ID:        "3",
			Timestamp: now,
			Username:  "user2",
			Direction: "upload",
			BytesTrans: 3000,
			Success:   false,
		},
	}

	for _, log := range logs {
		logger.Log(log)
	}

	// 按用户名过滤
	filter := &TransferLogFilter{Username: "user1"}
	result := logger.GetLogs(10, 0, filter)
	assert.Len(t, result, 2)

	// 按方向过滤
	filter = &TransferLogFilter{Direction: "upload"}
	result = logger.GetLogs(10, 0, filter)
	assert.Len(t, result, 2)

	// 按成功状态过滤
	success := false
	filter = &TransferLogFilter{Success: &success}
	result = logger.GetLogs(10, 0, filter)
	assert.Len(t, result, 1)
	assert.Equal(t, "3", result[0].ID)
}

func TestTransferLoggerStats(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := tmpDir + "/transfers.jsonl"

	config := TransferLoggerConfig{
		LogPath: logPath,
		MaxLogs: 100,
		Enabled: true,
	}

	logger, err := NewTransferLogger(config)
	assert.NoError(t, err)
	defer logger.Close()

	// 记录多条日志
	now := time.Now()
	for i := 0; i < 5; i++ {
		direction := "upload"
		if i%2 == 0 {
			direction = "download"
		}
		logger.Log(&TransferLog{
			ID:          string(rune('A' + i)),
			Timestamp:   now.Add(-time.Duration(i) * time.Minute),
			Username:    "user1",
			Direction:   direction,
			BytesTrans:  int64((i + 1) * 1000),
			Success:     i != 4,
			Bandwidth:   8000,
		})
	}

	stats := logger.GetStats(time.Hour)
	assert.Equal(t, 5, stats.TotalTransfers)
	assert.Equal(t, 3, stats.Downloads)  // i=0,2,4 是 download
	assert.Equal(t, 2, stats.Uploads)    // i=1,3 是 upload
	assert.Equal(t, 4, stats.SuccessfulTransfers)
	assert.Equal(t, 1, stats.FailedTransfers)
}

func TestTransferLoggerClear(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := tmpDir + "/transfers.jsonl"

	config := TransferLoggerConfig{
		LogPath: logPath,
		MaxLogs: 100,
		Enabled: true,
	}

	logger, err := NewTransferLogger(config)
	assert.NoError(t, err)
	defer logger.Close()

	// 记录日志
	logger.Log(&TransferLog{
		ID:        "1",
		Timestamp: time.Now(),
		Username:  "user1",
		Direction: "upload",
		Success:   true,
	})

	// 清除日志
	err = logger.Clear()
	assert.NoError(t, err)

	// 验证日志已清除
	logs := logger.GetLogs(10, 0, nil)
	assert.Len(t, logs, 0)
}

func TestTransferLoggerEnableDisable(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := tmpDir + "/transfers.jsonl"

	config := TransferLoggerConfig{
		LogPath: logPath,
		MaxLogs: 100,
		Enabled: true,
	}

	logger, err := NewTransferLogger(config)
	assert.NoError(t, err)
	defer logger.Close()

	assert.True(t, logger.IsEnabled())

	// 禁用
	logger.SetEnabled(false)
	assert.False(t, logger.IsEnabled())

	// 禁用时不应该记录日志
	logger.Log(&TransferLog{
		ID:        "1",
		Timestamp: time.Now(),
		Username:  "user1",
		Success:   true,
	})

	logs := logger.GetLogs(10, 0, nil)
	assert.Len(t, logs, 0)

	// 重新启用
	logger.SetEnabled(true)
	assert.True(t, logger.IsEnabled())
}

func TestTransferLogFilterMatch(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name   string
		filter TransferLogFilter
		log    TransferLog
		want   bool
	}{
		{
			name:   "match all empty filter",
			filter: TransferLogFilter{},
			log:    TransferLog{Username: "user1"},
			want:   true,
		},
		{
			name:   "match username",
			filter: TransferLogFilter{Username: "user1"},
			log:    TransferLog{Username: "user1"},
			want:   true,
		},
		{
			name:   "no match username",
			filter: TransferLogFilter{Username: "user1"},
			log:    TransferLog{Username: "user2"},
			want:   false,
		},
		{
			name:   "match direction",
			filter: TransferLogFilter{Direction: "upload"},
			log:    TransferLog{Direction: "upload"},
			want:   true,
		},
		{
			name:   "match time range",
			filter: TransferLogFilter{
				StartTime: now.Add(-2 * time.Hour),
				EndTime:   now,
			},
			log:  TransferLog{Timestamp: now.Add(-1 * time.Hour)},
			want: true,
		},
		{
			name: "match size range",
			filter: TransferLogFilter{
				MinSize: 100,
				MaxSize: 1000,
			},
			log:  TransferLog{BytesTrans: 500},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.filter.Match(&tt.log)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStartAndCompleteTransfer(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := tmpDir + "/transfers.jsonl"

	config := TransferLoggerConfig{
		LogPath: logPath,
		Enabled: true,
	}

	logger, err := NewTransferLogger(config)
	assert.NoError(t, err)
	defer logger.Close()

	// 开始传输
	log := logger.StartTransfer("user1", "192.168.1.1", "upload", "/test/file.txt", 10000, 0)
	assert.NotNil(t, log)
	assert.NotEmpty(t, log.ID)
	assert.Equal(t, "user1", log.Username)
	assert.Equal(t, "upload", log.Direction)

	// 完成传输
	logger.CompleteTransfer(log, 10000, 5*time.Second, true, "")

	// 验证日志
	logs := logger.GetLogs(10, 0, nil)
	assert.Len(t, logs, 1)
	assert.Equal(t, int64(10000), logs[0].BytesTrans)
	assert.True(t, logs[0].Success)
	assert.Greater(t, logs[0].Bandwidth, int64(0))
}

func TestLogFileCreation(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := tmpDir + "/subdir/transfers.jsonl"

	config := TransferLoggerConfig{
		LogPath: logPath,
		Enabled: true,
	}

	logger, err := NewTransferLogger(config)
	assert.NoError(t, err)
	defer logger.Close()

	// 验证文件被创建
	_, err = os.Stat(logPath)
	assert.NoError(t, err)
}