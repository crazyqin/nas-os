package office

import (
	"io"
	"testing"
	"time"
)

// MockFileAccessor 模拟文件访问器
type MockFileAccessor struct {
	files map[string]*FileInfo
}

func NewMockFileAccessor() *MockFileAccessor {
	return &MockFileAccessor{
		files: make(map[string]*FileInfo),
	}
}

func (m *MockFileAccessor) GetFileInfo(fileID string) (*FileInfo, error) {
	if f, ok := m.files[fileID]; ok {
		return f, nil
	}
	return nil, nil
}

func (m *MockFileAccessor) GetFileURL(fileID string) (string, error) {
	return "http://localhost:8080/files/" + fileID, nil
}

func (m *MockFileAccessor) SaveFile(fileID string, reader io.Reader) error {
	return nil
}

func (m *MockFileAccessor) GetFilePath(fileID string) (string, error) {
	return "/mnt/data/" + fileID, nil
}

func (m *MockFileAccessor) AddFile(fileID, name string, size int64) {
	m.files[fileID] = &FileInfo{
		ID:   fileID,
		Name: name,
		Path: "/mnt/data/" + fileID,
		Size: size,
	}
}

// ========== 测试用例 ==========

func TestNewManager(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, err := NewManager("", accessor)
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	if mgr == nil {
		t.Fatal("管理器为 nil")
	}

	// 检查默认配置
	cfg := mgr.GetConfig()
	if cfg == nil {
		t.Fatal("配置为 nil")
	}

	if cfg.SessionTimeout != 3600 {
		t.Errorf("默认会话超时时间应为 3600，实际: %d", cfg.SessionTimeout)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Enabled != false {
		t.Error("默认应禁用")
	}

	if len(cfg.EnabledTypes) == 0 {
		t.Error("应有默认支持的文件类型")
	}

	// 检查常用格式
	found := false
	for _, ext := range cfg.EnabledTypes {
		if ext == "docx" {
			found = true
			break
		}
	}
	if !found {
		t.Error("默认支持的文件类型应包含 docx")
	}
}

func TestGetDocumentType(t *testing.T) {
	mgr, _ := NewManager("", NewMockFileAccessor())

	tests := []struct {
		ext      string
		expected string
	}{
		{"docx", "word"},
		{"xlsx", "cell"},
		{"pptx", "slide"},
		{"pdf", "word"},
		{"odt", "word"},
		{"ods", "cell"},
		{"odp", "slide"},
		{"unknown", "word"}, // 未知类型默认为 word
	}

	for _, tt := range tests {
		result := mgr.getDocumentType(tt.ext)
		if result != tt.expected {
			t.Errorf("getExtensionType(%s) = %s, want %s", tt.ext, result, tt.expected)
		}
	}
}

func TestIsSupported(t *testing.T) {
	accessor := NewMockFileAccessor()
	mgr, _ := NewManager("", accessor)

	// 默认支持的格式
	if !mgr.isSupported("docx") {
		t.Error("docx 应被支持")
	}
	if !mgr.isSupported("xlsx") {
		t.Error("xlsx 应被支持")
	}
	if !mgr.isSupported("pptx") {
		t.Error("pptx 应被支持")
	}

	// 不支持的格式
	if mgr.isSupported("exe") {
		t.Error("exe 不应被支持")
	}
}

func TestCreateSession(t *testing.T) {
	accessor := NewMockFileAccessor()
	accessor.AddFile("file123", "test.docx", 1024)

	mgr, err := NewManager("", accessor)
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	// 启用服务
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	session, config, err := mgr.CreateSession("file123", "user1", "Test User", "edit")
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}

	if session == nil {
		t.Fatal("会话为 nil")
	}

	if session.FileID != "file123" {
		t.Errorf("FileID = %s, want file123", session.FileID)
	}

	if session.UserID != "user1" {
		t.Errorf("UserID = %s, want user1", session.UserID)
	}

	if session.Status != SessionStatusActive {
		t.Errorf("Status = %s, want active", session.Status)
	}

	if config == nil {
		t.Fatal("编辑器配置为 nil")
	}

	if config.DocumentType != "word" {
		t.Errorf("DocumentType = %s, want word", config.DocumentType)
	}
}

func TestGetSession(t *testing.T) {
	accessor := NewMockFileAccessor()
	accessor.AddFile("file123", "test.docx", 1024)

	mgr, _ := NewManager("", accessor)
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	// 创建会话
	session, _, _ := mgr.CreateSession("file123", "user1", "Test User", "edit")

	// 获取会话
	retrieved, err := mgr.GetSession(session.ID)
	if err != nil {
		t.Fatalf("获取会话失败: %v", err)
	}

	if retrieved.ID != session.ID {
		t.Errorf("ID 不匹配")
	}
}

func TestListSessions(t *testing.T) {
	accessor := NewMockFileAccessor()
	accessor.AddFile("file1", "test1.docx", 1024)
	accessor.AddFile("file2", "test2.xlsx", 2048)

	mgr, _ := NewManager("", accessor)
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	// 创建多个会话
	mgr.CreateSession("file1", "user1", "User 1", "edit")
	mgr.CreateSession("file2", "user2", "User 2", "edit")

	// 列出所有会话
	sessions, total := mgr.ListSessions("", 10, 0)
	if total != 2 {
		t.Errorf("总数 = %d, want 2", total)
	}

	if len(sessions) != 2 {
		t.Errorf("返回会话数 = %d, want 2", len(sessions))
	}

	// 分页测试
	sessions, _ = mgr.ListSessions("", 1, 0)
	if len(sessions) != 1 {
		t.Errorf("分页返回会话数 = %d, want 1", len(sessions))
	}
}

func TestCloseSession(t *testing.T) {
	accessor := NewMockFileAccessor()
	accessor.AddFile("file123", "test.docx", 1024)

	mgr, _ := NewManager("", accessor)
	mgr.UpdateConfig(UpdateConfigRequest{Enabled: boolPtr(true)})

	session, _, _ := mgr.CreateSession("file123", "user1", "Test User", "edit")

	// 关闭会话
	err := mgr.CloseSession(session.ID)
	if err != nil {
		t.Fatalf("关闭会话失败: %v", err)
	}

	// 验证会话已关闭
	_, err = mgr.GetSession(session.ID)
	if err == nil {
		t.Error("会话应该已关闭")
	}
}

func TestSessionExpiry(t *testing.T) {
	session := &EditingSession{
		ID:        "test",
		StartedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}

	if !session.IsExpired() {
		t.Error("会话应该已过期")
	}

	// 未过期会话
	session.ExpiresAt = time.Now().Add(1 * time.Hour)
	if session.IsExpired() {
		t.Error("会话不应过期")
	}
}

func TestFileAssociations(t *testing.T) {
	associations := DefaultFileAssociations()

	// 检查常用格式
	formats := []string{"docx", "xlsx", "pptx", "pdf", "odt", "ods", "odp"}
	for _, ext := range formats {
		if _, ok := associations[ext]; !ok {
			t.Errorf("缺少文件关联: %s", ext)
		}
	}

	// 检查 PDF 是只读的
	if pdf, ok := associations["pdf"]; ok {
		if pdf.CanEdit {
			t.Error("PDF 不应支持编辑")
		}
		if !pdf.CanView {
			t.Error("PDF 应支持查看")
		}
	}
}

func TestGetFileAssociation(t *testing.T) {
	mgr, _ := NewManager("", NewMockFileAccessor())

	assoc, ok := mgr.GetFileAssociation("docx")
	if !ok {
		t.Fatal("应找到 docx 关联")
	}

	if assoc.MimeType == "" {
		t.Error("MIME 类型不应为空")
	}

	if !assoc.CanEdit {
		t.Error("docx 应支持编辑")
	}
}

func TestUpdateConfig(t *testing.T) {
	mgr, _ := NewManager("", NewMockFileAccessor())

	// 更新配置
	err := mgr.UpdateConfig(UpdateConfigRequest{
		ServerURL:      strPtr("http://onlyoffice:80"),
		SecretKey:      strPtr("test-secret"),
		Enabled:        boolPtr(true),
		SessionTimeout: intPtr(7200),
	})

	if err != nil {
		t.Fatalf("更新配置失败: %v", err)
	}

	cfg := mgr.GetConfig()
	if cfg.ServerURL != "http://onlyoffice:80" {
		t.Errorf("ServerURL = %s", cfg.ServerURL)
	}

	if cfg.SecretKey != "test-secret" {
		t.Errorf("SecretKey = %s", cfg.SecretKey)
	}

	if cfg.SessionTimeout != 7200 {
		t.Errorf("SessionTimeout = %d", cfg.SessionTimeout)
	}
}

func TestBuildEditorConfig(t *testing.T) {
	accessor := NewMockFileAccessor()
	accessor.AddFile("file123", "test.xlsx", 1024)

	mgr, _ := NewManager("", accessor)
	mgr.UpdateConfig(UpdateConfigRequest{
		Enabled:   boolPtr(true),
		SecretKey: strPtr("test-secret"),
	})

	session := &EditingSession{
		ID:        "session123",
		FileID:    "file123",
		FileName:  "test.xlsx",
		FileKey:   "key123",
		UserID:    "user1",
		UserName:  "Test User",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	fileInfo := &FileInfo{
		ID:   "file123",
		Name: "test.xlsx",
	}

	config := mgr.buildEditorConfig(session, fileInfo, "http://localhost/file", "edit", "cell")

	if config.DocumentType != "cell" {
		t.Errorf("DocumentType = %s, want cell", config.DocumentType)
	}

	if config.Document.Key != "key123" {
		t.Errorf("Document.Key = %s, want key123", config.Document.Key)
	}

	if config.Editor.User.ID != "user1" {
		t.Errorf("User.ID = %s, want user1", config.Editor.User.ID)
	}

	if config.Token == "" {
		t.Error("应生成 JWT Token")
	}
}

func TestCallbackStatus(t *testing.T) {
	tests := []struct {
		status   int
		expected string
	}{
		{CallbackStatusEditing, "editing"},
		{CallbackStatusSaved, "saved"},
		{CallbackStatusSaving, "saving"},
		{CallbackStatusClosed, "closed"},
		{CallbackStatusForceSave, "forcesave"},
	}

	for _, tt := range tests {
		// 验证常量定义正确
		if tt.status <= 0 {
			t.Errorf("无效的状态码: %d", tt.status)
		}
	}
}

// ========== 辅助函数 ==========

func boolPtr(b bool) *bool       { return &b }
func strPtr(s string) *string    { return &s }
func intPtr(i int) *int          { return &i }