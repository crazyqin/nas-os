// Package shares 提供共享管理 API 处理器测试
package shares

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nas-os/internal/nfs"
	"nas-os/internal/smb"

	"github.com/gin-gonic/gin"
)

// mockSMBManager 模拟 SMB 管理器
type mockSMBManager struct {
	shares map[string]*smb.Share
	config *smb.Config
	status bool
}

func newMockSMBManager() *mockSMBManager {
	return &mockSMBManager{
		shares: map[string]*smb.Share{
			"share1": {
				Name:        "share1",
				Path:        "/data/share1",
				Comment:     "Test Share 1",
				Browseable:  true,
				ReadOnly:    false,
				GuestOK:     false,
			},
			"share2": {
				Name:        "share2",
				Path:        "/data/share2",
				Comment:     "Test Share 2",
				Browseable:  true,
				ReadOnly:    true,
				GuestOK:     false,
			},
		},
		config: &smb.Config{
			Workgroup:      "WORKGROUP",
			ServerString:   "NAS Server",
			NetBIOSName:    "NAS",
			Security:       "user",
			EncryptPasswords: true,
		},
		status: true,
	}
}

func (m *mockSMBManager) ListShares() ([]*smb.Share, error) {
	shares := make([]*smb.Share, 0, len(m.shares))
	for _, s := range m.shares {
		shares = append(shares, s)
	}
	return shares, nil
}

func (m *mockSMBManager) GetShare(name string) (*smb.Share, error) {
	if s, ok := m.shares[name]; ok {
		return s, nil
	}
	return nil, nil
}

func (m *mockSMBManager) CreateShareFromInput(input smb.ShareInput) (*smb.Share, error) {
	share := &smb.Share{
		Name:       input.Name,
		Path:       input.Path,
		Comment:    input.Comment,
		Browseable: input.Browseable,
		ReadOnly:   input.ReadOnly,
		GuestOK:    input.GuestOK,
	}
	m.shares[input.Name] = share
	return share, nil
}

func (m *mockSMBManager) UpdateShareFromInput(name string, input smb.ShareInput) (*smb.Share, error) {
	if _, ok := m.shares[name]; !ok {
		return nil, nil
	}
	share := &smb.Share{
		Name:       name,
		Path:       input.Path,
		Comment:    input.Comment,
		Browseable: input.Browseable,
		ReadOnly:   input.ReadOnly,
		GuestOK:    input.GuestOK,
	}
	m.shares[name] = share
	return share, nil
}

func (m *mockSMBManager) DeleteShare(name string) error {
	delete(m.shares, name)
	return nil
}

func (m *mockSMBManager) SetSharePermission(shareName, username string, readWrite bool) error {
	return nil
}

func (m *mockSMBManager) RemoveSharePermission(shareName, username string) error {
	return nil
}

func (m *mockSMBManager) GetUserShares(username string) []*smb.Share {
	return []*smb.Share{}
}

func (m *mockSMBManager) Restart() error {
	return nil
}

func (m *mockSMBManager) GetStatus() (bool, error) {
	return m.status, nil
}

func (m *mockSMBManager) GetConfig() *smb.Config {
	return m.config
}

func (m *mockSMBManager) UpdateConfig(config *smb.Config) error {
	m.config = config
	return nil
}

func (m *mockSMBManager) ApplyConfig() error {
	return nil
}

func (m *mockSMBManager) TestConfig() (bool, string, error) {
	return true, "OK", nil
}

// mockNFSManager 模拟 NFS 管理器
type mockNFSManager struct {
	exports map[string]*nfs.Export
	config  *nfs.Config
	status  *nfs.ServiceStatus
}

func newMockNFSManager() *mockNFSManager {
	return &mockNFSManager{
		exports: map[string]*nfs.Export{
			"/data/nfs1": {
				Path: "/data/nfs1",
				Clients: []nfs.Client{{Host: "192.168.1.0/24", Options: []string{"rw"}}},
			},
		},
		config: &nfs.Config{
			EnableNFSv4: true,
			EnableNFSv3: true,
		},
		status: &nfs.ServiceStatus{
			Running: true,
			Status:  "running",
		},
	}
}

func (m *mockNFSManager) ListExports() ([]*nfs.Export, error) {
	exports := make([]*nfs.Export, 0, len(m.exports))
	for _, e := range m.exports {
		exports = append(exports, e)
	}
	return exports, nil
}

func (m *mockNFSManager) GetExport(path string) (*nfs.Export, error) {
	if e, ok := m.exports[path]; ok {
		return e, nil
	}
	return nil, nil
}

func (m *mockNFSManager) CreateExport(exp *nfs.Export) error {
	m.exports[exp.Path] = exp
	return nil
}

func (m *mockNFSManager) UpdateExport(oldPath string, exp *nfs.Export) error {
	delete(m.exports, oldPath)
	m.exports[exp.Path] = exp
	return nil
}

func (m *mockNFSManager) DeleteExport(path string) error {
	delete(m.exports, path)
	return nil
}

func (m *mockNFSManager) Restart() error {
	return nil
}

func (m *mockNFSManager) Reload() error {
	return nil
}

func (m *mockNFSManager) Status() (*nfs.ServiceStatus, error) {
	return m.status, nil
}

func (m *mockNFSManager) GetClients() ([]map[string]string, error) {
	return []map[string]string{}, nil
}

func (m *mockNFSManager) GetConfig() *nfs.Config {
	return m.config
}

func (m *mockNFSManager) UpdateConfig(config *nfs.Config) error {
	m.config = config
	return nil
}

// setupTestRouter 创建测试路由
func setupTestRouter() (*gin.Engine, *Handlers) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	smbMgr := newMockSMBManager()
	nfsMgr := newMockNFSManager()
	handlers := NewHandlers(smbMgr, nfsMgr)
	return router, handlers
}

func TestNewHandlers(t *testing.T) {
	smbMgr := newMockSMBManager()
	nfsMgr := newMockNFSManager()
	handlers := NewHandlers(smbMgr, nfsMgr)
	
	if handlers == nil {
		t.Fatal("NewHandlers should not return nil")
	}
	if handlers.smbManager == nil {
		t.Fatal("smbManager should not be nil")
	}
	if handlers.nfsManager == nil {
		t.Fatal("nfsManager should not be nil")
	}
}

func TestListAllShares(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequest("GET", "/api/shares", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("Expected code 0, got %d", resp.Code)
	}

	// 验证返回的共享数据
	data, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatal("Data should be an array")
	}
	if len(data) == 0 {
		t.Error("Expected at least one share")
	}
}

func TestGetStatus(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequest("GET", "/api/shares/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Data should be a map")
	}

	smbStatus, ok := data["smb"].(map[string]interface{})
	if !ok {
		t.Fatal("smb status should be present")
	}
	if !smbStatus["running"].(bool) {
		t.Error("SMB should be running")
	}

	nfsStatus, ok := data["nfs"].(map[string]interface{})
	if !ok {
		t.Fatal("nfs status should be present")
	}
	if !nfsStatus["running"].(bool) {
		t.Error("NFS should be running")
	}
}

func TestApplyAllConfig(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequest("POST", "/api/shares/apply", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// ========== SMB 共享测试 ==========

func TestListSMBShares(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequest("GET", "/api/shares/smb", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("Expected code 0, got %d", resp.Code)
	}
}

func TestCreateSMBShare(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	shareInput := map[string]interface{}{
		"name":       "testshare",
		"path":       "/data/testshare",
		"comment":    "Test Share",
		"browseable": true,
		"read_only":  false,
		"guest_ok":   false,
	}
	body, _ := json.Marshal(shareInput)

	req, _ := http.NewRequest("POST", "/api/shares/smb", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("Expected code 0, got %d", resp.Code)
	}
}

func TestGetSMBShare(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	// 测试存在的共享
	req, _ := http.NewRequest("GET", "/api/shares/smb/share1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// 测试不存在的共享
	req2, _ := http.NewRequest("GET", "/api/shares/smb/nonexistent", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 for non-existent share, got %d", w2.Code)
	}
}

func TestUpdateSMBShare(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	shareInput := map[string]interface{}{
		"path":       "/data/share1-updated",
		"comment":    "Updated Share",
		"browseable": true,
		"read_only":  true,
		"guest_ok":   false,
	}
	body, _ := json.Marshal(shareInput)

	req, _ := http.NewRequest("PUT", "/api/shares/smb/share1", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestDeleteSMBShare(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequest("DELETE", "/api/shares/smb/share1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestSetSMBPermission(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	permInput := map[string]interface{}{
		"username":   "testuser",
		"read_write": true,
	}
	body, _ := json.Marshal(permInput)

	req, _ := http.NewRequest("POST", "/api/shares/smb/share1/permission", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestRemoveSMBPermission(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequest("DELETE", "/api/shares/smb/share1/permission/testuser", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestRestartSMB(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequest("POST", "/api/shares/smb/restart", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetSMBStatus(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequest("GET", "/api/shares/smb/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Data should be a map")
	}
	if !data["running"].(bool) {
		t.Error("SMB should be running")
	}
}

func TestGetSMBConfig(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequest("GET", "/api/shares/smb/config", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("Expected code 0, got %d", resp.Code)
	}
}

func TestUpdateSMBConfig(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	config := map[string]interface{}{
		"workgroup":        "WORKGROUP",
		"server_string":    "Updated NAS Server",
		"netbios_name":     "NAS",
		"security":         "user",
		"encrypt_passwords": true,
	}
	body, _ := json.Marshal(config)

	req, _ := http.NewRequest("PUT", "/api/shares/smb/config", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestTestSMBConfig(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequest("POST", "/api/shares/smb/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Data should be a map")
	}
	if !data["valid"].(bool) {
		t.Error("Config should be valid")
	}
}

// ========== NFS 共享测试 ==========

func TestListNFSExports(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequest("GET", "/api/shares/nfs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("Expected code 0, got %d", resp.Code)
	}
}

func TestCreateNFSExport(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	exportInput := map[string]interface{}{
		"path":       "/data/nfs2",
		"options":    "rw,sync",
		"clients":    []map[string]interface{}{{"host": "192.168.1.0/24", "options": "rw"}},
		"anonymous":  false,
	}
	body, _ := json.Marshal(exportInput)

	req, _ := http.NewRequest("POST", "/api/shares/nfs", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}
}

func TestGetNFSExport(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	// 测试存在的导出
	req, _ := http.NewRequest("GET", "/api/shares/nfs//data/nfs1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestDeleteNFSExport(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequest("DELETE", "/api/shares/nfs//data/nfs1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestRestartNFS(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequest("POST", "/api/shares/nfs/restart", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetNFSStatus(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequest("GET", "/api/shares/nfs/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("Expected code 0, got %d", resp.Code)
	}
}

func TestGetNFSClients(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequest("GET", "/api/shares/nfs/clients", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetNFSConfig(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req, _ := http.NewRequest("GET", "/api/shares/nfs/config", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("Expected code 0, got %d", resp.Code)
	}
}

func TestUpdateNFSConfig(t *testing.T) {
	router, handlers := setupTestRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	config := map[string]interface{}{
		"enable_nfsv4": true,
		"enable_nfsv3": true,
	}
	body, _ := json.Marshal(config)

	req, _ := http.NewRequest("PUT", "/api/shares/nfs/config", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestSuccess(t *testing.T) {
	data := map[string]string{"key": "value"}
	resp := Success(data)

	if resp.Code != 0 {
		t.Errorf("Expected code 0, got %d", resp.Code)
	}
	if resp.Message != "success" {
		t.Errorf("Expected message 'success', got %s", resp.Message)
	}
	if resp.Data == nil {
		t.Error("Data should not be nil")
	}
}

func TestError(t *testing.T) {
	resp := Error(400, "Bad Request")

	if resp.Code != 400 {
		t.Errorf("Expected code 400, got %d", resp.Code)
	}
	if resp.Message != "Bad Request" {
		t.Errorf("Expected message 'Bad Request', got %s", resp.Message)
	}
	if resp.Data != nil {
		t.Error("Data should be nil for error response")
	}
}