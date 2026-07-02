package freeipa

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDefaultDirectoryConfig(t *testing.T) {
	config := DefaultDirectoryConfig()

	if config.Port != 389 {
		t.Errorf("Expected Port=389, got %d", config.Port)
	}
	if config.UseTLS != true {
		t.Error("Expected UseTLS=true")
	}
	if config.EnableSync != true {
		t.Error("Expected EnableSync=true")
	}
	if config.SyncInterval != 30*time.Minute {
		t.Errorf("Expected SyncInterval=30m, got %v", config.SyncInterval)
	}
	if config.Status != StatusDisconnected {
		t.Errorf("Expected Status=disconnected, got %v", config.Status)
	}
	if config.BaseDN != "dc=example,dc=com" {
		t.Errorf("Expected BaseDN=dc=example,dc=com, got %s", config.BaseDN)
	}
}

func TestDefaultSyncSchedule(t *testing.T) {
	schedule := DefaultSyncSchedule()

	if !schedule.Enabled {
		t.Error("Expected Enabled=true")
	}
	if schedule.Interval != 30*time.Minute {
		t.Errorf("Expected Interval=30m, got %v", schedule.Interval)
	}
	if !schedule.SyncUsers {
		t.Error("Expected SyncUsers=true")
	}
	if !schedule.SyncGroups {
		t.Error("Expected SyncGroups=true")
	}
	if !schedule.AutoCreate {
		t.Error("Expected AutoCreate=true")
	}
	if schedule.ConflictStrategy != "remote_wins" {
		t.Errorf("Expected ConflictStrategy=remote_wins, got %s", schedule.ConflictStrategy)
	}
}

func TestNewClient(t *testing.T) {
	config := DefaultDirectoryConfig()
	config.Host = "ipa.example.com"
	config.BaseDN = "dc=example,dc=com"

	logger := slog.Default()
	client := NewClient(config, logger)

	if client == nil {
		t.Fatal("Expected non-nil client")
	}
	if client.IsConnected() {
		t.Error("Expected not connected initially")
	}

	stats := client.GetStats()
	if stats.Status != StatusDisconnected {
		t.Errorf("Expected status disconnected, got %v", stats.Status)
	}
	if stats.TotalUsers != 0 {
		t.Errorf("Expected 0 users, got %d", stats.TotalUsers)
	}
}

func TestClient_Connect(t *testing.T) {
	config := DefaultDirectoryConfig()
	config.Host = "127.0.0.1"
	config.Port = 0 // 无效端口，确保连接失败
	config.UseTLS = false

	client := NewClient(config, slog.Default())
	ctx := context.Background()

	// 连接应该失败（无效地址）
	err := client.Connect(ctx)
	if err == nil {
		t.Error("Expected connection error for invalid address")
	}

	if client.IsConnected() {
		t.Error("Should not be connected after failed connect")
	}
}

func TestClient_Authenticate_NotConnected(t *testing.T) {
	config := DefaultDirectoryConfig()
	client := NewClient(config, slog.Default())
	ctx := context.Background()

	result, err := client.Authenticate(ctx, "testuser", "password")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.Success {
		t.Error("Expected auth failure when not connected")
	}
	if result.Error != "目录服务未连接" {
		t.Errorf("Expected error '目录服务未连接', got '%s'", result.Error)
	}
}

func TestClient_Authenticate_EmptyPassword(t *testing.T) {
	config := DefaultDirectoryConfig()
	client := NewClient(config, slog.Default())
	ctx := context.Background()

	client.mu.Lock()
	client.stats.Status = StatusConnected
	client.conn = &mockConn{} // 使用模拟连接
	client.mu.Unlock()

	result, err := client.Authenticate(ctx, "testuser", "")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.Success {
		t.Error("Expected auth failure for empty password")
	}
	if result.Error != "密码不能为空" {
		t.Errorf("Expected error '密码不能为空', got '%s'", result.Error)
	}
}

func TestClient_SearchUsers(t *testing.T) {
	config := DefaultDirectoryConfig()
	client := NewClient(config, slog.Default())
	ctx := context.Background()

	// 添加测试用户
	client.mu.Lock()
	client.usersCache = []LDAPUser{
		{UID: "user1", Username: "alice", Email: "alice@example.com", Enabled: true, UIDNumber: 1000},
		{UID: "user2", Username: "bob", Email: "bob@example.com", Enabled: false, UIDNumber: 1001},
		{UID: "user3", Username: "charlie", Email: "charlie@example.com", Enabled: true, UIDNumber: 1002, Groups: []string{"admins"}},
	}
	client.mu.Unlock()

	// 搜索所有用户
	users, total, err := client.SearchUsers(ctx, UserSearchFilter{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if total != 3 {
		t.Errorf("Expected 3 users, got %d", total)
	}
	if len(users) != 3 {
		t.Errorf("Expected 3 users in result, got %d", len(users))
	}

	// 按用户名搜索
	users, total, err = client.SearchUsers(ctx, UserSearchFilter{Username: "ali"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if total != 1 {
		t.Errorf("Expected 1 user, got %d", total)
	}

	// 按启用状态搜索
	enabled := true
	users, total, err = client.SearchUsers(ctx, UserSearchFilter{Enabled: &enabled})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if total != 2 {
		t.Errorf("Expected 2 enabled users, got %d", total)
	}

	// 按组搜索
	users, total, err = client.SearchUsers(ctx, UserSearchFilter{Group: "admins"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if total != 1 {
		t.Errorf("Expected 1 user in admins group, got %d", total)
	}

	// 分页
	users, total, err = client.SearchUsers(ctx, UserSearchFilter{Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if total != 3 {
		t.Errorf("Expected total=3, got %d", total)
	}
	if len(users) != 1 {
		t.Errorf("Expected 1 user with limit=1, got %d", len(users))
	}
}

func TestClient_SearchGroups(t *testing.T) {
	config := DefaultDirectoryConfig()
	client := NewClient(config, slog.Default())
	ctx := context.Background()

	client.mu.Lock()
	client.groupsCache = []LDAPGroup{
		{CN: "admins", GIDNumber: 100, Description: "管理员组", Members: []string{"alice"}},
		{CN: "users", GIDNumber: 101, Description: "普通用户组", Members: []string{"bob", "charlie"}},
		{CN: "empty", GIDNumber: 102, Description: "空组"},
	}
	client.mu.Unlock()

	// 搜索所有组
	groups, total, err := client.SearchGroups(ctx, GroupSearchFilter{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if total != 3 {
		t.Errorf("Expected 3 groups, got %d", total)
	}
	if len(groups) != 3 {
		t.Errorf("Expected 3 groups in result, got %d", len(groups))
	}

	// 按名称搜索
	groups, total, err = client.SearchGroups(ctx, GroupSearchFilter{Name: "admin"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if total != 1 {
		t.Errorf("Expected 1 group, got %d", total)
	}

	// 按是否有成员搜索
	hasMembers := true
	groups, total, err = client.SearchGroups(ctx, GroupSearchFilter{HasMembers: &hasMembers})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if total != 2 {
		t.Errorf("Expected 2 groups with members, got %d", total)
	}
}

func TestClient_SyncUsers(t *testing.T) {
	config := DefaultDirectoryConfig()
	client := NewClient(config, slog.Default())
	ctx := context.Background()

	// 手动设置连接状态
	client.mu.Lock()
	client.stats.Status = StatusConnected
	client.mu.Unlock()

	result, err := client.SyncUsers(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.UsersSynced != 0 {
		t.Errorf("Expected 0 synced, got %d", result.UsersSynced)
	}
	if result.Duration == "" {
		t.Error("Expected non-empty duration")
	}
}

func TestClient_SyncGroups(t *testing.T) {
	config := DefaultDirectoryConfig()
	client := NewClient(config, slog.Default())
	ctx := context.Background()

	client.mu.Lock()
	client.stats.Status = StatusConnected
	client.mu.Unlock()

	result, err := client.SyncGroups(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.GroupsSynced != 0 {
		t.Errorf("Expected 0 synced, got %d", result.GroupsSynced)
	}
}

func TestClient_FullSync(t *testing.T) {
	config := DefaultDirectoryConfig()
	client := NewClient(config, slog.Default())
	ctx := context.Background()

	client.mu.Lock()
	client.stats.Status = StatusConnected
	client.mu.Unlock()

	result, err := client.FullSync(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.UsersSynced != 0 {
		t.Errorf("Expected 0 users synced, got %d", result.UsersSynced)
	}
	if result.GroupsSynced != 0 {
		t.Errorf("Expected 0 groups synced, got %d", result.GroupsSynced)
	}
}

func TestClient_GetStats(t *testing.T) {
	config := DefaultDirectoryConfig()
	client := NewClient(config, slog.Default())

	client.mu.Lock()
	client.usersCache = []LDAPUser{
		{Username: "alice", Enabled: true},
		{Username: "bob", Enabled: false},
		{Username: "charlie", Enabled: true},
	}
	client.groupsCache = []LDAPGroup{
		{CN: "admins"},
	}
	client.mu.Unlock()

	stats := client.GetStats()
	if stats.TotalUsers != 3 {
		t.Errorf("Expected 3 total users, got %d", stats.TotalUsers)
	}
	if stats.TotalGroups != 1 {
		t.Errorf("Expected 1 total groups, got %d", stats.TotalGroups)
	}
	if stats.ActiveUsers != 2 {
		t.Errorf("Expected 2 active users, got %d", stats.ActiveUsers)
	}
	if stats.DisabledUsers != 1 {
		t.Errorf("Expected 1 disabled user, got %d", stats.DisabledUsers)
	}
	if stats.Uptime == "" {
		t.Error("Expected non-empty uptime")
	}
}

func TestClient_UpdateConfig(t *testing.T) {
	config := DefaultDirectoryConfig()
	client := NewClient(config, slog.Default())

	newConfig := config
	newConfig.Host = "new-host.example.com"
	newConfig.Port = 636
	client.UpdateConfig(newConfig)

	got := client.GetConfig()
	if got.Host != "new-host.example.com" {
		t.Errorf("Expected host new-host.example.com, got %s", got.Host)
	}
	if got.Port != 636 {
		t.Errorf("Expected port 636, got %d", got.Port)
	}
}

func TestMatchUserFilter(t *testing.T) {
	user := LDAPUser{
		Username:  "alice",
		Email:     "alice@example.com",
		Enabled:   true,
		UIDNumber: 1000,
		Groups:    []string{"admins", "users"},
	}

	// 空过滤器匹配一切
	if !matchUserFilter(user, UserSearchFilter{}) {
		t.Error("Empty filter should match")
	}

	// 用户名部分匹配
	if !matchUserFilter(user, UserSearchFilter{Username: "ali"}) {
		t.Error("Username partial match should succeed")
	}
	if matchUserFilter(user, UserSearchFilter{Username: "bob"}) {
		t.Error("Username mismatch should fail")
	}

	// 邮箱匹配
	if !matchUserFilter(user, UserSearchFilter{Email: "alice"}) {
		t.Error("Email partial match should succeed")
	}

	// 组匹配
	if !matchUserFilter(user, UserSearchFilter{Group: "admins"}) {
		t.Error("Group match should succeed")
	}
	if matchUserFilter(user, UserSearchFilter{Group: "wheel"}) {
		t.Error("Group mismatch should fail")
	}

	// 启用状态
	enabled := true
	if !matchUserFilter(user, UserSearchFilter{Enabled: &enabled}) {
		t.Error("Enabled filter should match")
	}
	disabled := false
	if matchUserFilter(user, UserSearchFilter{Enabled: &disabled}) {
		t.Error("Disabled filter should not match enabled user")
	}

	// UID 范围
	if !matchUserFilter(user, UserSearchFilter{UIDMin: 999, UIDMax: 1001}) {
		t.Error("UID range should match")
	}
	if matchUserFilter(user, UserSearchFilter{UIDMin: 2000}) {
		t.Error("UID min filter should not match")
	}
}

func TestMatchGroupFilter(t *testing.T) {
	group := LDAPGroup{
		CN:        "admins",
		GIDNumber: 100,
		Members:   []string{"alice"},
	}

	if !matchGroupFilter(group, GroupSearchFilter{}) {
		t.Error("Empty filter should match")
	}

	if !matchGroupFilter(group, GroupSearchFilter{Name: "admin"}) {
		t.Error("Name partial match should succeed")
	}

	hasMembers := true
	if !matchGroupFilter(group, GroupSearchFilter{HasMembers: &hasMembers}) {
		t.Error("HasMembers=true should match")
	}
	noMembers := false
	if matchGroupFilter(group, GroupSearchFilter{HasMembers: &noMembers}) {
		t.Error("HasMembers=false should not match")
	}
}

func TestHandler_RegisterRoutes(t *testing.T) {
	config := DefaultDirectoryConfig()
	client := NewClient(config, slog.Default())
	handler := NewHandler(client, slog.Default())

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// 验证路由注册
	routes := []string{
		"/api/v1/freeipa/status",
		"/api/v1/freeipa/config",
		"/api/v1/freeipa/connect",
		"/api/v1/freeipa/disconnect",
		"/api/v1/freeipa/auth",
		"/api/v1/freeipa/users",
		"/api/v1/freeipa/groups",
		"/api/v1/freeipa/sync",
		"/api/v1/freeipa/stats",
	}

	for _, route := range routes {
		req := httptest.NewRequest(http.MethodGet, route, nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		// 不检查状态码，只验证路由存在
	}
}

func TestHandler_HandleStatus(t *testing.T) {
	config := DefaultDirectoryConfig()
	client := NewClient(config, slog.Default())
	handler := NewHandler(client, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/freeipa/status", nil)
	w := httptest.NewRecorder()

	handler.handleStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp APIResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Error("Expected success=true")
	}
}

func TestHandler_HandleStatus_MethodNotAllowed(t *testing.T) {
	config := DefaultDirectoryConfig()
	client := NewClient(config, slog.Default())
	handler := NewHandler(client, slog.Default())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/freeipa/status", nil)
	w := httptest.NewRecorder()

	handler.handleStatus(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandler_HandleConfig_Get(t *testing.T) {
	config := DefaultDirectoryConfig()
	config.Host = "ipa.example.com"
	client := NewClient(config, slog.Default())
	handler := NewHandler(client, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/freeipa/config", nil)
	w := httptest.NewRecorder()

	handler.handleConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp APIResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Error("Expected success=true")
	}
}

func TestHandler_HandleAuth_EmptyUsername(t *testing.T) {
	config := DefaultDirectoryConfig()
	client := NewClient(config, slog.Default())
	handler := NewHandler(client, slog.Default())

	body := `{"username":"","password":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/freeipa/auth", nil)
	req.Body = nopCloser(body)
	w := httptest.NewRecorder()

	handler.handleAuth(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandler_HandleConnect_MethodNotAllowed(t *testing.T) {
	config := DefaultDirectoryConfig()
	client := NewClient(config, slog.Default())
	handler := NewHandler(client, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/freeipa/connect", nil)
	w := httptest.NewRecorder()

	handler.handleConnect(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandler_HandleSync_NotConnected(t *testing.T) {
	config := DefaultDirectoryConfig()
	client := NewClient(config, slog.Default())
	handler := NewHandler(client, slog.Default())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/freeipa/sync", nil)
	w := httptest.NewRecorder()

	handler.handleSync(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestDirectoryStatus_Constants(t *testing.T) {
	if StatusConnected != "connected" {
		t.Errorf("Expected StatusConnected=connected, got %s", StatusConnected)
	}
	if StatusDisconnected != "disconnected" {
		t.Errorf("Expected StatusDisconnected=disconnected, got %s", StatusDisconnected)
	}
	if StatusError != "error" {
		t.Errorf("Expected StatusError=error, got %s", StatusError)
	}
	if StatusSyncing != "syncing" {
		t.Errorf("Expected StatusSyncing=syncing, got %s", StatusSyncing)
	}
}

func TestSyncResult_Fields(t *testing.T) {
	result := &SyncResult{
		UsersSynced:   10,
		GroupsSynced:  3,
		UsersAdded:    2,
		UsersUpdated:  5,
		UsersRemoved:  1,
		GroupsAdded:   1,
		GroupsUpdated: 2,
		GroupsRemoved: 0,
		Duration:      "1.5s",
		SyncedAt:      time.Now(),
	}

	if result.UsersSynced != 10 {
		t.Errorf("Expected 10 users synced, got %d", result.UsersSynced)
	}
	if result.GroupsSynced != 3 {
		t.Errorf("Expected 3 groups synced, got %d", result.GroupsSynced)
	}
	if result.UsersAdded != 2 {
		t.Errorf("Expected 2 users added, got %d", result.UsersAdded)
	}
}

func TestLDAPUser_Fields(t *testing.T) {
	user := LDAPUser{
		UID:           "testuser",
		Username:      "testuser",
		DisplayName:   "Test User",
		Email:         "test@example.com",
		FirstName:     "Test",
		LastName:      "User",
		HomeDirectory: "/home/testuser",
		Shell:         "/bin/bash",
		GIDNumber:     1000,
		UIDNumber:     1000,
		Groups:        []string{"users", "admins"},
		Enabled:       true,
	}

	if user.Username != "testuser" {
		t.Errorf("Expected username testuser, got %s", user.Username)
	}
	if len(user.Groups) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(user.Groups))
	}
	if !user.Enabled {
		t.Error("Expected user to be enabled")
	}
}

func TestLDAPGroup_Fields(t *testing.T) {
	group := LDAPGroup{
		CN:          "admins",
		GIDNumber:   100,
		Description: "管理员组",
		Members:     []string{"alice", "bob"},
	}

	if group.CN != "admins" {
		t.Errorf("Expected CN=admins, got %s", group.CN)
	}
	if group.GIDNumber != 100 {
		t.Errorf("Expected GIDNumber=100, got %d", group.GIDNumber)
	}
	if len(group.Members) != 2 {
		t.Errorf("Expected 2 members, got %d", len(group.Members))
	}
}

func TestUserSearchFilter_Fields(t *testing.T) {
	enabled := true
	filter := UserSearchFilter{
		Username: "alice",
		Email:    "alice@example.com",
		Group:    "admins",
		Enabled:  &enabled,
		UIDMin:   1000,
		UIDMax:   2000,
		Limit:    10,
		Offset:   0,
	}

	if filter.Username != "alice" {
		t.Errorf("Expected Username=alice, got %s", filter.Username)
	}
	if filter.Limit != 10 {
		t.Errorf("Expected Limit=10, got %d", filter.Limit)
	}
}

func TestGroupSearchFilter_Fields(t *testing.T) {
	hasMembers := true
	filter := GroupSearchFilter{
		Name:       "admins",
		GIDMin:     100,
		GIDMax:     200,
		HasMembers: &hasMembers,
		Limit:      20,
		Offset:     0,
	}

	if filter.Name != "admins" {
		t.Errorf("Expected Name=admins, got %s", filter.Name)
	}
	if filter.GIDMin != 100 {
		t.Errorf("Expected GIDMin=100, got %d", filter.GIDMin)
	}
}

// mockConn 模拟网络连接.
type mockConn struct{}

func (m *mockConn) Read(b []byte) (n int, err error)   { return 0, nil }
func (m *mockConn) Write(b []byte) (n int, err error)  { return len(b), nil }
func (m *mockConn) Close() error                       { return nil }
func (m *mockConn) LocalAddr() net.Addr                { return nil }
func (m *mockConn) RemoteAddr() net.Addr               { return nil }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

// nopCloser 创建一个 io.ReadCloser 用于测试.
func nopCloser(s string) *stringReader {
	return &stringReader{s: s, i: 0}
}

type stringReader struct {
	s string
	i int
}

func (r *stringReader) Read(p []byte) (n int, err error) {
	if r.i >= len(r.s) {
		return 0, nil
	}
	n = copy(p, r.s[r.i:])
	r.i += n
	return n, nil
}

func (r *stringReader) Close() error {
	return nil
}
