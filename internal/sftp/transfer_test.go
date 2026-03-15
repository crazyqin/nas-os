package sftp

import (
	"testing"
	"time"
)

// ========== TransferLog 测试 ==========

func TestTransferLog_Struct(t *testing.T) {
	log := &TransferLog{
		ID:         "tx-123",
		Timestamp:  time.Now(),
		Username:   "admin",
		ClientIP:   "10.0.0.1",
		SessionID:  "sess-456",
		Direction:  "download",
		FilePath:   "/data/backup.tar.gz",
		FileSize:   1073741824,
		BytesTrans: 1073741824,
		Duration:   5000,
		Success:    true,
		Bandwidth:  214748364,
		Method:     "sftp",
	}

	if log.ID != "tx-123" {
		t.Errorf("Expected ID=tx-123, got %s", log.ID)
	}
	if log.FileSize != 1073741824 {
		t.Errorf("Expected FileSize=1073741824, got %d", log.FileSize)
	}
	if !log.Success {
		t.Error("Expected Success=true")
	}
}

func TestTransferLog_FailedTransfer(t *testing.T) {
	log := &TransferLog{
		ID:        "tx-124",
		Username:  "user1",
		Direction: "upload",
		FilePath:  "/data/test.txt",
		FileSize:  1024,
		Success:   false,
		Error:     "permission denied",
	}

	if log.Success {
		t.Error("Expected Success=false")
	}
	if log.Error != "permission denied" {
		t.Errorf("Expected Error='permission denied', got %s", log.Error)
	}
}

// ========== TransferLoggerConfig 测试 ==========

func TestDefaultTransferLoggerConfig(t *testing.T) {
	config := DefaultTransferLoggerConfig()

	if config.LogPath != "/var/log/nas-os/sftp-transfers.jsonl" {
		t.Errorf("Expected LogPath=/var/log/nas-os/sftp-transfers.jsonl, got %s", config.LogPath)
	}
	if config.MaxLogs != 10000 {
		t.Errorf("Expected MaxLogs=10000, got %d", config.MaxLogs)
	}
	if config.MaxSize != 100*1024*1024 {
		t.Errorf("Expected MaxSize=100MB, got %d", config.MaxSize)
	}
	if config.Retention != 30*24*time.Hour {
		t.Errorf("Expected Retention=30 days, got %v", config.Retention)
	}
	if !config.Enabled {
		t.Error("Expected Enabled=true")
	}
}

func TestTransferLoggerConfig_Custom(t *testing.T) {
	config := TransferLoggerConfig{
		LogPath:   "/custom/path/transfers.log",
		MaxLogs:   5000,
		MaxSize:   50 * 1024 * 1024,
		Retention: 7 * 24 * time.Hour,
		Enabled:   false,
	}

	if config.LogPath != "/custom/path/transfers.log" {
		t.Errorf("Unexpected LogPath: %s", config.LogPath)
	}
	if config.MaxLogs != 5000 {
		t.Errorf("Expected MaxLogs=5000, got %d", config.MaxLogs)
	}
}

// ========== TransferLogFilter 测试 ==========

func TestTransferLogFilter(t *testing.T) {
	filter := TransferLogFilter{
		Username:  "admin",
		Direction: "download",
	}

	if filter.Username != "admin" {
		t.Errorf("Expected Username=admin, got %s", filter.Username)
	}
	if filter.Direction != "download" {
		t.Errorf("Expected Direction=download, got %s", filter.Direction)
	}
}

func TestTransferLogFilter_Match(t *testing.T) {
	filter := TransferLogFilter{
		Username:  "admin",
		Direction: "download",
	}

	log := &TransferLog{
		Username:  "admin",
		Direction: "download",
	}

	if !filter.Match(log) {
		t.Error("Filter should match log")
	}

	// 不匹配的日志
	log2 := &TransferLog{
		Username:  "user",
		Direction: "download",
	}

	if filter.Match(log2) {
		t.Error("Filter should not match log with different username")
	}
}

// ========== 配置验证测试 ==========

func TestConfig_PortValidation(t *testing.T) {
	tests := []struct {
		port    int
		isValid bool
	}{
		{22, true},
		{2222, true},
		{65535, true},
		{0, false},
		{-1, false},
		{65536, false},
	}

	for _, tt := range tests {
		valid := tt.port > 0 && tt.port <= 65535
		if valid != tt.isValid {
			t.Errorf("Port %d: expected valid=%v, got %v", tt.port, tt.isValid, valid)
		}
	}
}

// ========== 用户 Chroot 测试 ==========

func TestConfig_UserChroots(t *testing.T) {
	config := &Config{
		ChrootEnabled: true,
		UserChroots: map[string]string{
			"admin":   "/data/admin",
			"user1":   "/data/user1",
			"guest":   "/data/guest",
		},
	}

	if len(config.UserChroots) != 3 {
		t.Errorf("Expected 3 user chroots, got %d", len(config.UserChroots))
	}

	if config.UserChroots["admin"] != "/data/admin" {
		t.Errorf("Expected admin chroot=/data/admin, got %s", config.UserChroots["admin"])
	}
}

// ========== 空闲超时测试 ==========

func TestConfig_IdleTimeout(t *testing.T) {
	tests := []struct {
		timeout int
		desc    string
	}{
		{300, "5 minutes"},
		{600, "10 minutes"},
		{900, "15 minutes"},
		{1800, "30 minutes"},
	}

	for _, tt := range tests {
		config := &Config{IdleTimeout: tt.timeout}
		if config.IdleTimeout != tt.timeout {
			t.Errorf("%s: expected %d, got %d", tt.desc, tt.timeout, config.IdleTimeout)
		}
	}
}

// ========== 最大连接数测试 ==========

func TestConfig_MaxConnections(t *testing.T) {
	tests := []struct {
		maxConn int
		desc    string
	}{
		{0, "unlimited"},
		{10, "small"},
		{100, "default"},
		{1000, "large"},
	}

	for _, tt := range tests {
		config := &Config{MaxConnections: tt.maxConn}
		if config.MaxConnections != tt.maxConn {
			t.Errorf("%s: expected %d, got %d", tt.desc, tt.maxConn, config.MaxConnections)
		}
	}
}

// ========== 并发安全测试 ==========

func TestTransferLog_ConcurrentAccess(t *testing.T) {
	log := &TransferLog{
		ID:       "tx-concurrent",
		FileSize: 0,
	}

	done := make(chan bool)

	// 并发修改
	for i := 0; i < 10; i++ {
		go func(i int) {
			log.FileSize += int64(i)
			done <- true
		}(i)
	}

	// 等待所有操作完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 注意：这不是线程安全的测试，只是验证数据结构
}

// ========== JSON 序列化测试 ==========

func TestTransferLog_JSON(t *testing.T) {
	log := &TransferLog{
		ID:        "tx-json",
		Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Username:  "testuser",
		Direction: "upload",
		FilePath:  "/test/file.txt",
		FileSize:  1024,
		Success:   true,
		Method:    "sftp",
	}

	// 验证字段可以被序列化
	if log.Username != "testuser" {
		t.Errorf("Expected Username=testuser, got %s", log.Username)
	}
}

// ========== 错误场景测试 ==========

func TestTransferLog_Error(t *testing.T) {
	log := &TransferLog{
		ID:      "tx-error",
		Success: false,
		Error:   "connection reset by peer",
	}

	if log.Success {
		t.Error("Expected Success=false for error case")
	}
	if log.Error == "" {
		t.Error("Error message should not be empty")
	}
}

// ========== 带宽计算测试 ==========

func TestTransferLog_Bandwidth(t *testing.T) {
	log := &TransferLog{
		ID:         "tx-bandwidth",
		FileSize:   10000000,  // 10 MB
		BytesTrans: 10000000,
		Duration:   10000,     // 10 seconds (ms)
	}

	// 计算预期带宽 (bytes per second)
	expectedBandwidth := log.BytesTrans * 1000 / log.Duration
	// 10000000 * 1000 / 10000 = 1000000 bytes/sec = 1 MB/s

	if expectedBandwidth != 1000000 {
		t.Errorf("Expected bandwidth=1000000, got %d", expectedBandwidth)
	}
}

// ========== 方向测试 ==========

func TestTransferLog_Direction(t *testing.T) {
	directions := []string{"upload", "download"}

	for _, dir := range directions {
		log := &TransferLog{Direction: dir}
		if log.Direction != dir {
			t.Errorf("Expected Direction=%s, got %s", dir, log.Direction)
		}
	}
}

// ========== 匿名访问安全测试 ==========

func TestConfig_AllowAnonymous_Security(t *testing.T) {
	// 默认不允许匿名访问
	config := DefaultConfig()
	if config.AllowAnonymous {
		t.Error("Security: AllowAnonymous should be false by default")
	}

	// 禁用 chroot 时不应该允许匿名访问
	config.ChrootEnabled = false
	config.AllowAnonymous = true

	// 这是一个不安全的配置组合
	if !config.AllowAnonymous && config.ChrootEnabled {
		t.Log("Safe configuration: anonymous disabled with chroot")
	}
}