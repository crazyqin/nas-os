// Package e2e 提供 NAS-OS 端到端测试框架
// 测试完整的 API 端到端流程，模拟真实用户操作场景
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"nas-os/internal/auth"
	"nas-os/internal/cluster"
	"nas-os/internal/docker"
	"nas-os/internal/files"
	"nas-os/internal/network"
	"nas-os/internal/nfs"
	"nas-os/internal/perf"
	"nas-os/internal/plugin"
	"nas-os/internal/quota"
	"nas-os/internal/shares"
	"nas-os/internal/smb"
	"nas-os/internal/storage"
	"nas-os/internal/users"
	"nas-os/internal/web"
)

// TestConfig E2E 测试配置
type TestConfig struct {
	BaseURL      string
	Timeout      time.Duration
	CleanupAfter bool
	Parallel     int
}

// DefaultTestConfig 默认测试配置
var DefaultTestConfig = TestConfig{
	BaseURL:      "http://localhost:8080",
	Timeout:      30 * time.Second,
	CleanupAfter: true,
	Parallel:     4,
}

// TestSuite E2E 测试套件
type TestSuite struct {
	t          *testing.T
	config     TestConfig
	server     *web.Server
	httpClient *http.Client
	authToken  string
	ctx        context.Context
	cancel     context.CancelFunc
	cleanup    []func()
	mu         sync.Mutex
}

// NewTestSuite 创建测试套件
func NewTestSuite(t *testing.T, config TestConfig) *TestSuite {
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	return &TestSuite{
		t:      t,
		config: config,
		ctx:    ctx,
		cancel: cancel,
		cleanup: make([]func(), 0),
	}
}

// Setup 初始化测试环境
func (s *TestSuite) Setup() error {
	s.t.Helper()

	// 初始化 HTTP 客户端
	s.httpClient = &http.Client{
		Timeout: s.config.Timeout,
	}

	// 初始化模拟存储管理器
	storageMgr := storage.NewMockManager()
	userMgr := users.NewMockManager()
	smbMgr := smb.NewMockManager()
	nfsMgr := nfs.NewMockManager()
	networkMgr := network.NewMockManager()

	// 创建测试服务器
	s.server = web.NewServer(storageMgr, userMgr, smbMgr, nfsMgr, networkMgr)

	// 注册清理函数
	s.AddCleanup(func() {
		s.cancel()
	})

	return nil
}

// Teardown 清理测试环境
func (s *TestSuite) Teardown() {
	s.t.Helper()
	for _, fn := range s.cleanup {
		fn()
	}
}

// AddCleanup 添加清理函数
func (s *TestSuite) AddCleanup(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanup = append(s.cleanup, fn)
}

// ========== HTTP 辅助方法 ==========

// Request 发送 HTTP 请求
func (s *TestSuite) Request(method, path string, body interface{}) (*http.Response, error) {
	s.t.Helper()

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(s.ctx, method, s.config.BaseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if s.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.authToken)
	}

	return s.httpClient.Do(req)
}

// Get 发送 GET 请求
func (s *TestSuite) Get(path string) (*http.Response, error) {
	return s.Request(http.MethodGet, path, nil)
}

// Post 发送 POST 请求
func (s *TestSuite) Post(path string, body interface{}) (*http.Response, error) {
	return s.Request(http.MethodPost, path, body)
}

// Put 发送 PUT 请求
func (s *TestSuite) Put(path string, body interface{}) (*http.Response, error) {
	return s.Request(http.MethodPut, path, body)
}

// Delete 发送 DELETE 请求
func (s *TestSuite) Delete(path string) (*http.Response, error) {
	return s.Request(http.MethodDelete, path, nil)
}

// ParseResponse 解析响应
func (s *TestSuite) ParseResponse(resp *http.Response, out interface{}) error {
	s.t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}
	return nil
}

// AssertStatus 断言 HTTP 状态码
func (s *TestSuite) AssertStatus(expected, actual int, msgAndArgs ...interface{}) {
	s.t.Helper()
	if expected != actual {
		s.t.Fatalf("status code mismatch: expected %d, got %d. %v", expected, actual, msgAndArgs)
	}
}

// AssertEqual 断言相等
func (s *TestSuite) AssertEqual(expected, actual interface{}, msgAndArgs ...interface{}) {
	s.t.Helper()
	if expected != actual {
		s.t.Fatalf("assertion failed: expected %v, got %v. %v", expected, actual, msgAndArgs)
	}
}

// AssertNotEmpty 断言非空
func (s *TestSuite) AssertNotEmpty(value string, msgAndArgs ...interface{}) {
	s.t.Helper()
	if value == "" {
		s.t.Fatalf("assertion failed: value should not be empty. %v", msgAndArgs)
	}
}

// ========== 认证辅助方法 ==========

// Login 用户登录
func (s *TestSuite) Login(username, password string) error {
	s.t.Helper()

	resp, err := s.Post("/api/v1/auth/login", map[string]string{
		"username": username,
		"password": password,
	})
	if err != nil {
		return fmt.Errorf("login request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed: status %d", resp.StatusCode)
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := s.ParseResponse(resp, &result); err != nil {
		return fmt.Errorf("parse login response: %w", err)
	}

	s.authToken = result.Token
	return nil
}

// Logout 用户登出
func (s *TestSuite) Logout() {
	s.authToken = ""
}

// ========== 测试数据生成 ==========

// MockManager 模拟管理器
type MockManager struct {
	data map[string]interface{}
	mu   sync.RWMutex
}

// NewMockManager 创建模拟管理器
func NewMockManager() *MockManager {
	return &MockManager{
		data: make(map[string]interface{}),
	}
}

// ========== 测试入口 ==========

// TestMain E2E 测试入口
func TestMain(m *testing.M) {
	fmt.Println("🚀 NAS-OS E2E 测试套件 v1.0")
	fmt.Println("=====================================")

	// 检查环境
	if os.Getenv("NAS_OS_E2E") == "" {
		fmt.Println("⚠️  设置 NAS_OS_E2E=1 启用 E2E 测试")
		os.Exit(0)
	}

	code := m.Run()
	fmt.Println("=====================================")
	fmt.Println("✅ E2E 测试完成")
	os.Exit(code)
}

// ========== 模拟接口 ==========

// MockStorageManager 模拟存储管理器
type MockStorageManager struct {
	volumes map[string]*storage.Volume
	mu      sync.RWMutex
}

func NewMockStorageManager() *MockStorageManager {
	return &MockStorageManager{
		volumes: make(map[string]*storage.Volume),
	}
}

func (m *MockStorageManager) CreateVolume(name string, devices []string, profile string) (*storage.Volume, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	vol := &storage.Volume{
		Name:        name,
		Devices:     devices,
		DataProfile: profile,
		MetaProfile: profile,
		Size:        1000000000000, // 1TB
		Used:        0,
		Free:        1000000000000,
		Status: storage.VolumeStatus{
			Healthy: true,
		},
	}
	m.volumes[name] = vol
	return vol, nil
}

func (m *MockStorageManager) ListVolumes() ([]*storage.Volume, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vols := make([]*storage.Volume, 0, len(m.volumes))
	for _, v := range m.volumes {
		vols = append(vols, v)
	}
	return vols, nil
}

func (m *MockStorageManager) GetVolume(name string) (*storage.Volume, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vol, ok := m.volumes[name]
	if !ok {
		return nil, fmt.Errorf("volume not found: %s", name)
	}
	return vol, nil
}

func (m *MockStorageManager) DeleteVolume(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.volumes, name)
	return nil
}

// MockUserManager 模拟用户管理器
type MockUserManager struct {
	users map[string]*users.User
	mu    sync.RWMutex
}

func NewMockUserManager() *MockUserManager {
	return &MockUserManager{
		users: make(map[string]*users.User),
	}
}

func (m *MockUserManager) Create(username, password string) (*users.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	user := &users.User{
		Username: username,
		Role:     "user",
	}
	m.users[username] = user
	return user, nil
}

func (m *MockUserManager) Authenticate(username, password string) (*users.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	user, ok := m.users[username]
	if !ok {
		return nil, fmt.Errorf("user not found")
	}
	return user, nil
}

// ========== 类型声明 ==========

// User 用户结构
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// ========== 额外导入 ==========

var _ = auth.MFAManager{}
var _ = cluster.Manager{}
var _ = docker.Manager{}
var _ = files.Manager{}
var _ = perf.Manager{}
var _ = plugin.Manager{}
var _ = quota.Manager{}
var _ = shares.Manager{}

// ========== 测试服务器 ==========

// NewTestServer 创建测试服务器
func NewTestServer() *httptest.Server {
	// 初始化模拟管理器
	storageMgr := NewMockStorageManager()
	userMgr := NewMockUserManager()

	// 创建 Gin 引擎
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	// 注册路由
	api := engine.Group("/api/v1")
	{
		// 健康检查
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		// 卷管理
		volumes := api.Group("/volumes")
		{
			volumes.GET("", func(c *gin.Context) {
				vols, _ := storageMgr.ListVolumes()
				c.JSON(http.StatusOK, vols)
			})
			volumes.POST("", func(c *gin.Context) {
				var req struct {
					Name    string   `json:"name"`
					Devices []string `json:"devices"`
					Profile string   `json:"profile"`
				}
				if err := c.BindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				vol, err := storageMgr.CreateVolume(req.Name, req.Devices, req.Profile)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusCreated, vol)
			})
			volumes.GET("/:name", func(c *gin.Context) {
				name := c.Param("name")
				vol, err := storageMgr.GetVolume(name)
				if err != nil {
					c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, vol)
			})
			volumes.DELETE("/:name", func(c *gin.Context) {
				name := c.Param("name")
				if err := storageMgr.DeleteVolume(name); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusNoContent, nil)
			})
		}

		// 认证
		auth := api.Group("/auth")
		{
			auth.POST("/login", func(c *gin.Context) {
				var req struct {
					Username string `json:"username"`
					Password string `json:"password"`
				}
				if err := c.BindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				user, err := userMgr.Authenticate(req.Username, req.Password)
				if err != nil {
					c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
					return
				}
				c.JSON(http.StatusOK, gin.H{
					"token": "test-token-" + user.Username,
					"user":  user,
				})
			})
		}
	}

	return httptest.NewServer(engine)
}