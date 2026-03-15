package audit

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	config := DefaultConfig()
	m := NewManager(config)

	if m == nil {
		t.Fatal("manager should not be nil")
	}

	if !m.config.Enabled {
		t.Error("audit should be enabled by default")
	}

	m.Stop()
}

func TestLogEntry(t *testing.T) {
	config := DefaultConfig()
	m := NewManager(config)
	defer m.Stop()

	entry := &Entry{
		Level:    LevelInfo,
		Category: CategoryAuth,
		Event:    "login",
		UserID:   "user-001",
		Username: "testuser",
		IP:       "192.168.1.1",
		Status:   StatusSuccess,
		Message:  "User logged in successfully",
	}

	err := m.Log(entry)
	if err != nil {
		t.Fatalf("failed to log entry: %v", err)
	}

	// 验证ID和时间戳已设置
	if entry.ID == "" {
		t.Error("entry ID should be set")
	}
	if entry.Timestamp.IsZero() {
		t.Error("entry timestamp should be set")
	}

	// 验证签名（如果启用）
	if config.EnableSignatures && entry.Signature == "" {
		t.Error("entry signature should be set when signatures are enabled")
	}
}

func TestQueryLogs(t *testing.T) {
	config := DefaultConfig()
	m := NewManager(config)
	defer m.Stop()

	// 添加测试日志
	for i := 0; i < 10; i++ {
		entry := &Entry{
			Level:    LevelInfo,
			Category: CategoryAuth,
			Event:    "login",
			UserID:   "user-001",
			Username: "testuser",
			IP:       "192.168.1.1",
			Status:   StatusSuccess,
			Message:  "User logged in",
		}
		m.Log(entry)
	}

	// 查询所有日志
	opts := QueryOptions{
		Limit:  100,
		Offset: 0,
	}

	result, err := m.Query(opts)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if result.Total != 10 {
		t.Errorf("expected 10 entries, got %d", result.Total)
	}

	// 测试分页
	opts.Limit = 5
	opts.Offset = 0
	result, err = m.Query(opts)
	if err != nil {
		t.Fatalf("query with pagination failed: %v", err)
	}

	if len(result.Entries) != 5 {
		t.Errorf("expected 5 entries, got %d", len(result.Entries))
	}

	// 测试筛选
	opts = QueryOptions{
		Limit:    100,
		Category: CategoryAuth,
	}
	result, err = m.Query(opts)
	if err != nil {
		t.Fatalf("query with filter failed: %v", err)
	}

	if result.Total != 10 {
		t.Errorf("expected 10 auth entries, got %d", result.Total)
	}
}

func TestLogAuth(t *testing.T) {
	config := DefaultConfig()
	m := NewManager(config)
	defer m.Stop()

	err := m.LogAuth("login", "user-001", "testuser", "192.168.1.1", "Mozilla/5.0", StatusSuccess, "Login successful", nil)
	if err != nil {
		t.Fatalf("failed to log auth event: %v", err)
	}

	// 验证日志已记录
	opts := QueryOptions{
		Category: CategoryAuth,
		Limit:    10,
	}
	result, _ := m.Query(opts)

	if result.Total != 1 {
		t.Errorf("expected 1 auth entry, got %d", result.Total)
	}

	if result.Entries[0].Event != "login" {
		t.Errorf("expected event 'login', got '%s'", result.Entries[0].Event)
	}
}

func TestLogAccess(t *testing.T) {
	config := DefaultConfig()
	m := NewManager(config)
	defer m.Stop()

	err := m.LogAccess("user-001", "testuser", "192.168.1.1", "/api/users", "read", StatusSuccess, nil)
	if err != nil {
		t.Fatalf("failed to log access event: %v", err)
	}

	opts := QueryOptions{
		Category: CategoryAccess,
		Limit:    10,
	}
	result, _ := m.Query(opts)

	if result.Total != 1 {
		t.Errorf("expected 1 access entry, got %d", result.Total)
	}
}

func TestLogSecurity(t *testing.T) {
	config := DefaultConfig()
	m := NewManager(config)
	defer m.Stop()

	err := m.LogSecurity("intrusion_attempt", "user-001", "testuser", "10.0.0.1", LevelCritical, "Potential intrusion detected", nil)
	if err != nil {
		t.Fatalf("failed to log security event: %v", err)
	}

	opts := QueryOptions{
		Category: CategorySecurity,
		Level:    LevelCritical,
		Limit:    10,
	}
	result, _ := m.Query(opts)

	if result.Total != 1 {
		t.Errorf("expected 1 critical security entry, got %d", result.Total)
	}
}

func TestLogDataOperation(t *testing.T) {
	config := DefaultConfig()
	m := NewManager(config)
	defer m.Stop()

	err := m.LogDataOperation("user-001", "testuser", "192.168.1.1", "/data/sensitive", "delete", StatusSuccess, nil)
	if err != nil {
		t.Fatalf("failed to log data operation: %v", err)
	}

	opts := QueryOptions{
		Category: CategoryData,
		Limit:    10,
	}
	result, _ := m.Query(opts)

	if result.Total != 1 {
		t.Errorf("expected 1 data entry, got %d", result.Total)
	}
}

func TestLogConfigChange(t *testing.T) {
	config := DefaultConfig()
	m := NewManager(config)
	defer m.Stop()

	err := m.LogConfigChange("user-001", "admin", "192.168.1.1", "security.policy", "update", "old_policy", "new_policy")
	if err != nil {
		t.Fatalf("failed to log config change: %v", err)
	}

	opts := QueryOptions{
		Category: CategorySystem,
		Event:    "config_change",
		Limit:    10,
	}
	result, _ := m.Query(opts)

	if result.Total != 1 {
		t.Errorf("expected 1 config change entry, got %d", result.Total)
	}
}

func TestGetStatistics(t *testing.T) {
	config := DefaultConfig()
	m := NewManager(config)
	defer m.Stop()

	// 添加多种类型的日志
	m.LogAuth("login", "user-001", "user1", "192.168.1.1", "", StatusSuccess, "", nil)
	m.LogAuth("login", "user-002", "user2", "192.168.1.2", "", StatusFailure, "", nil)
	m.LogAccess("user-001", "user1", "192.168.1.1", "/resource", "read", StatusSuccess, nil)
	m.LogSecurity("alert", "user-001", "user1", "192.168.1.1", LevelWarning, "test", nil)

	stats := m.GetStatistics()

	if stats.TotalEntries != 4 {
		t.Errorf("expected 4 total entries, got %d", stats.TotalEntries)
	}

	if stats.EventsByCategory[string(CategoryAuth)] != 2 {
		t.Errorf("expected 2 auth events, got %d", stats.EventsByCategory[string(CategoryAuth)])
	}

	if len(stats.TopUsers) == 0 {
		t.Error("expected top users to be populated")
	}

	if len(stats.TopIPs) == 0 {
		t.Error("expected top IPs to be populated")
	}
}

func TestIntegrity(t *testing.T) {
	config := DefaultConfig()
	config.EnableSignatures = true
	m := NewManager(config)
	defer m.Stop()

	// 记录日志
	entry := &Entry{
		Level:    LevelInfo,
		Category: CategoryAuth,
		Event:    "login",
		UserID:   "user-001",
		Username: "testuser",
		IP:       "192.168.1.1",
		Status:   StatusSuccess,
	}
	m.Log(entry)

	// 验证完整性
	report := m.VerifyIntegrity()

	if !report.Valid {
		t.Error("integrity should be valid")
	}

	if report.Tampered > 0 {
		t.Errorf("expected 0 tampered entries, got %d", report.Tampered)
	}
}

func TestExport(t *testing.T) {
	config := DefaultConfig()
	m := NewManager(config)
	defer m.Stop()

	// 添加日志
	for i := 0; i < 5; i++ {
		m.Log(&Entry{
			Level:    LevelInfo,
			Category: CategoryAuth,
			Event:    "login",
			UserID:   "user-001",
			Username: "testuser",
			IP:       "192.168.1.1",
			Status:   StatusSuccess,
			Message:  "Test log",
		})
	}

	opts := ExportOptions{
		Format:    ExportJSON,
		StartTime: time.Now().Add(-time.Hour),
		EndTime:   time.Now().Add(time.Hour),
	}

	data, err := m.Export(opts)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("export data should not be empty")
	}

	// 测试CSV导出
	opts.Format = ExportCSV
	data, err = m.Export(opts)
	if err != nil {
		t.Fatalf("CSV export failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("CSV export data should not be empty")
	}

	// 测试XML导出
	opts.Format = ExportXML
	data, err = m.Export(opts)
	if err != nil {
		t.Fatalf("XML export failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("XML export data should not be empty")
	}
}

func TestCleanup(t *testing.T) {
	config := DefaultConfig()
	config.MaxAgeDays = 1
	m := NewManager(config)
	defer m.Stop()

	// 添加日志
	m.Log(&Entry{
		Level:    LevelInfo,
		Category: CategoryAuth,
		Event:    "login",
		UserID:   "user-001",
		Status:   StatusSuccess,
	})

	// 清理（当前日志不应该被清理）
	cleaned := m.Cleanup()
	// 由于时间设置，不应该清理任何日志
	if cleaned < 0 {
		t.Error("cleanup should not return negative")
	}
}

// ========== 合规报告测试 ==========

func TestComplianceReport(t *testing.T) {
	config := DefaultConfig()
	m := NewManager(config)
	defer m.Stop()

	// 添加测试日志
	m.LogAuth("login", "user-001", "user1", "192.168.1.1", "", StatusSuccess, "", nil)
	m.LogAuth("login", "user-002", "user2", "192.168.1.2", "", StatusFailure, "", nil)
	m.LogAccess("user-001", "user1", "192.168.1.1", "/resource", "read", StatusFailure, nil)

	reporter := NewComplianceReporter(m)

	start := time.Now().Add(-time.Hour)
	end := time.Now()

	report, err := reporter.GenerateReport(ComplianceGDPR, start, end)
	if err != nil {
		t.Fatalf("failed to generate report: %v", err)
	}

	if report.Standard != ComplianceGDPR {
		t.Errorf("expected GDPR standard, got %s", report.Standard)
	}

	if report.Summary.TotalEvents != 3 {
		t.Errorf("expected 3 events, got %d", report.Summary.TotalEvents)
	}

	// 测试等级保护报告
	report, err = reporter.GenerateReport(ComplianceMLPS, start, end)
	if err != nil {
		t.Fatalf("failed to generate MLPS report: %v", err)
	}

	if report.Standard != ComplianceMLPS {
		t.Errorf("expected MLPS standard, got %s", report.Standard)
	}
}

func TestGenerateDashboardData(t *testing.T) {
	config := DefaultConfig()
	m := NewManager(config)
	defer m.Stop()

	m.LogAuth("login", "user-001", "user1", "192.168.1.1", "", StatusSuccess, "", nil)
	m.LogAuth("login", "user-002", "user2", "192.168.1.2", "", StatusFailure, "", nil)

	reporter := NewComplianceReporter(m)
	data := reporter.GenerateDashboardData()

	if data == nil {
		t.Fatal("dashboard data should not be nil")
	}

	totalEvents, ok := data["total_events"].(int)
	if !ok {
		t.Fatal("total_events should be an int")
	}

	if totalEvents != 2 {
		t.Errorf("expected 2 total events, got %d", totalEvents)
	}
}

// ========== 完整性管理器测试 ==========

func TestIntegrityManager(t *testing.T) {
	im := NewIntegrityManager()

	entry := &Entry{
		ID:        "test-001",
		Timestamp: time.Now(),
		Level:     LevelInfo,
		Category:  CategoryAuth,
		Event:     "login",
		UserID:    "user-001",
		Resource:  "/auth",
		Status:    StatusSuccess,
	}

	// 签名
	sig := im.SignEntry(entry, nil)
	if sig == "" {
		t.Error("signature should not be empty")
	}

	// 验证
	entry.Signature = sig
	if !im.VerifyEntry(entry, nil) {
		t.Error("signature verification should succeed")
	}

	// 篡改后验证
	entry.Message = "tampered"
	if im.VerifyEntry(entry, nil) {
		t.Error("tampered entry should fail verification")
	}
}

func TestMerkleRoot(t *testing.T) {
	im := NewIntegrityManager()

	entries := make([]*Entry, 4)
	for i := 0; i < 4; i++ {
		entries[i] = &Entry{
			ID:        string(rune('A' + i)),
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			Level:     LevelInfo,
			Category:  CategoryAuth,
			Event:     "test",
			Status:    StatusSuccess,
		}
	}

	root := im.GenerateMerkleRoot(entries)
	if root == "" {
		t.Error("merkle root should not be empty")
	}

	// 修改条目后根哈希应该不同
	entries[0].Message = "modified"
	newRoot := im.GenerateMerkleRoot(entries)
	if root == newRoot {
		t.Error("merkle root should change when entries are modified")
	}
}

func TestAuditProof(t *testing.T) {
	im := NewIntegrityManager()

	entries := make([]*Entry, 4)
	for i := 0; i < 4; i++ {
		entries[i] = &Entry{
			ID:        string(rune('A' + i)),
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			Level:     LevelInfo,
			Category:  CategoryAuth,
			Event:     "test",
			Status:    StatusSuccess,
		}
	}

	// 生成证明
	proof, err := im.GenerateAuditProof(entries, "A")
	if err != nil {
		t.Fatalf("failed to generate audit proof: %v", err)
	}

	if proof.RootHash == "" {
		t.Error("root hash should not be empty")
	}

	if len(proof.ProofPath) == 0 {
		t.Error("proof path should not be empty")
	}

	// 验证证明
	if !im.VerifyAuditProof(proof, entries[0]) {
		t.Error("audit proof verification should succeed")
	}
}

// ========== 基准测试 ==========

func BenchmarkLogEntry(b *testing.B) {
	config := DefaultConfig()
	m := NewManager(config)
	defer m.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Log(&Entry{
			Level:    LevelInfo,
			Category: CategoryAuth,
			Event:    "benchmark",
			UserID:   "user-001",
			Status:   StatusSuccess,
		})
	}
}

func BenchmarkQuery(b *testing.B) {
	config := DefaultConfig()
	m := NewManager(config)
	defer m.Stop()

	// 预填充日志
	for i := 0; i < 1000; i++ {
		m.Log(&Entry{
			Level:    LevelInfo,
			Category: CategoryAuth,
			Event:    "benchmark",
			UserID:   "user-001",
			Status:   StatusSuccess,
		})
	}

	opts := QueryOptions{
		Limit:  100,
		Offset: 0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Query(opts)
	}
}