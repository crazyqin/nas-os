package multinasmanager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	config := Config{
		NodeID:            "test-node-1",
		Name:              "Test Cluster",
		HeartbeatInterval: 60,
		HeartbeatTimeout:  120,
		DataDir:           t.TempDir(),
		MaxAlerts:         100,
		MaxEvents:         100,
	}
	m, err := NewManager(config, logger)
	if err != nil {
		t.Fatalf("创建管理器失败：%v", err)
	}
	return m
}

func setupTestAPI(t *testing.T) (*API, *Manager) {
	t.Helper()
	m := setupTestManager(t)
	logger, _ := zap.NewDevelopment()
	api := NewAPI(m, logger)
	return api, m
}

func setupTestRouter(t *testing.T) (*gin.Engine, *API, *Manager) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	api, m := setupTestAPI(t)
	router := gin.New()
	api.RegisterRoutes(router.Group("/api/v1/multinas"))
	return router, api, m
}

func TestNewManager(t *testing.T) {
	m := setupTestManager(t)
	defer m.Shutdown()

	if m.config.NodeID != "test-node-1" {
		t.Errorf("期望 NodeID 为 test-node-1，实际为 %s", m.config.NodeID)
	}
	if m.config.Name != "Test Cluster" {
		t.Errorf("期望 Name 为 Test Cluster，实际为 %s", m.config.Name)
	}
}

func TestNewManagerDefaults(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := Config{
		DataDir: t.TempDir(),
	}
	m, err := NewManager(config, logger)
	if err != nil {
		t.Fatalf("创建管理器失败：%v", err)
	}
	defer m.Shutdown()

	if m.config.HeartbeatInterval != 10 {
		t.Errorf("期望 HeartbeatInterval 为 10，实际为 %d", m.config.HeartbeatInterval)
	}
	if m.config.HeartbeatTimeout != 30 {
		t.Errorf("期望 HeartbeatTimeout 为 30，实际为 %d", m.config.HeartbeatTimeout)
	}
}

func TestRegisterNode(t *testing.T) {
	m := setupTestManager(t)
	defer m.Shutdown()

	node := &NASNode{
		ID:           "node-1",
		Name:         "NAS-1",
		Hostname:     "nas1.local",
		IP:           "192.168.1.100",
		Port:         8080,
		Status:       NodeStatusOnline,
		TotalStorage: 1024 * 1024 * 1024 * 1024, // 1TB
	}

	err := m.RegisterNode(node)
	if err != nil {
		t.Fatalf("注册节点失败：%v", err)
	}

	nodes := m.GetNodes()
	if len(nodes) != 1 {
		t.Fatalf("期望 1 个节点，实际有 %d 个", len(nodes))
	}
	if nodes[0].ID != "node-1" {
		t.Errorf("期望节点 ID 为 node-1，实际为 %s", nodes[0].ID)
	}
}

func TestUnregisterNode(t *testing.T) {
	m := setupTestManager(t)
	defer m.Shutdown()

	node := &NASNode{
		ID:   "node-1",
		Name: "NAS-1",
	}
	m.RegisterNode(node)

	err := m.UnregisterNode("node-1")
	if err != nil {
		t.Fatalf("注销节点失败：%v", err)
	}

	nodes := m.GetNodes()
	if len(nodes) != 0 {
		t.Errorf("期望 0 个节点，实际有 %d 个", len(nodes))
	}
}

func TestGetNode(t *testing.T) {
	m := setupTestManager(t)
	defer m.Shutdown()

	node := &NASNode{
		ID:   "node-1",
		Name: "NAS-1",
		IP:   "192.168.1.100",
	}
	m.RegisterNode(node)

	result, err := m.GetNode("node-1")
	if err != nil {
		t.Fatalf("获取节点失败：%v", err)
	}
	if result.Name != "NAS-1" {
		t.Errorf("期望节点名称为 NAS-1，实际为 %s", result.Name)
	}

	_, err = m.GetNode("nonexistent")
	if err == nil {
		t.Error("期望获取不存在的节点时返回错误")
	}
}

func TestUpdateNodeStatus(t *testing.T) {
	m := setupTestManager(t)
	defer m.Shutdown()

	node := &NASNode{
		ID:     "node-1",
		Name:   "NAS-1",
		Status: NodeStatusOnline,
	}
	m.RegisterNode(node)

	err := m.UpdateNodeStatus("node-1", NodeStatusDegraded)
	if err != nil {
		t.Fatalf("更新节点状态失败：%v", err)
	}

	result, _ := m.GetNode("node-1")
	if result.Status != NodeStatusDegraded {
		t.Errorf("期望状态为 degraded，实际为 %s", result.Status)
	}
}

func TestUpdateNodeMetrics(t *testing.T) {
	m := setupTestManager(t)
	defer m.Shutdown()

	node := &NASNode{
		ID:           "node-1",
		Name:         "NAS-1",
		Status:       NodeStatusOnline,
		TotalStorage: 1000,
	}
	m.RegisterNode(node)

	err := m.UpdateNodeMetrics("node-1", 50.5, 60.2, 500)
	if err != nil {
		t.Fatalf("更新节点指标失败：%v", err)
	}

	result, _ := m.GetNode("node-1")
	if result.CPUUsage != 50.5 {
		t.Errorf("期望 CPU 为 50.5，实际为 %.1f", result.CPUUsage)
	}
	if result.FreeStorage != 500 {
		t.Errorf("期望 FreeStorage 为 500，实际为 %d", result.FreeStorage)
	}
}

func TestRegisterPool(t *testing.T) {
	m := setupTestManager(t)
	defer m.Shutdown()

	m.RegisterNode(&NASNode{ID: "node-1", Name: "NAS-1"})

	pool := &StoragePool{
		ID:        "pool-1",
		Name:      "Main Pool",
		NodeID:    "node-1",
		NodeName:  "NAS-1",
		TotalSize: 1024 * 1024 * 1024 * 1024,
		UsedSize:  512 * 1024 * 1024 * 1024,
		FreeSize:  512 * 1024 * 1024 * 1024,
		Health:    "healthy",
		RaidLevel: "raidz1",
	}

	err := m.RegisterPool(pool)
	if err != nil {
		t.Fatalf("注册存储池失败：%v", err)
	}

	pools := m.GetPools()
	if len(pools) != 1 {
		t.Fatalf("期望 1 个存储池，实际有 %d 个", len(pools))
	}
}

func TestGetAggregatedPools(t *testing.T) {
	m := setupTestManager(t)
	defer m.Shutdown()

	m.RegisterNode(&NASNode{ID: "node-1", Name: "NAS-1"})
	m.RegisterNode(&NASNode{ID: "node-2", Name: "NAS-2"})

	pool1 := &StoragePool{
		ID:        "pool-1",
		Name:      "Main Pool",
		NodeID:    "node-1",
		TotalSize: 1000,
		UsedSize:  500,
		FreeSize:  500,
	}
	pool2 := &StoragePool{
		ID:        "pool-2",
		Name:      "Main Pool",
		NodeID:    "node-2",
		TotalSize: 2000,
		UsedSize:  800,
		FreeSize:  1200,
	}

	m.RegisterPool(pool1)
	m.RegisterPool(pool2)

	aggregated := m.GetAggregatedPools()
	if len(aggregated) != 1 {
		t.Fatalf("期望 1 个聚合存储池，实际有 %d 个", len(aggregated))
	}
	if aggregated[0].TotalSize != 3000 {
		t.Errorf("期望聚合总大小为 3000，实际为 %d", aggregated[0].TotalSize)
	}
	if aggregated[0].UsedSize != 1300 {
		t.Errorf("期望聚合已用为 1300，实际为 %d", aggregated[0].UsedSize)
	}
}

func TestAlerts(t *testing.T) {
	m := setupTestManager(t)
	defer m.Shutdown()

	// 通过更新高CPU来触发告警.
	m.RegisterNode(&NASNode{
		ID:           "node-1",
		Name:         "NAS-1",
		Status:       NodeStatusOnline,
		TotalStorage: 10000,
	})
	m.UpdateNodeMetrics("node-1", 95.0, 50.0, 1000)

	alerts := m.GetAlerts("", nil, 100)
	if len(alerts) == 0 {
		t.Fatal("期望有告警，实际为 0")
	}

	// 确认告警.
	err := m.AckAlert(alerts[0].ID, "admin")
	if err != nil {
		t.Fatalf("确认告警失败：%v", err)
	}

	// 验证已确认.
	acked := true
	ackedAlerts := m.GetAlerts("", &acked, 100)
	if len(ackedAlerts) != 1 {
		t.Errorf("期望 1 个已确认告警，实际有 %d 个", len(ackedAlerts))
	}
}

func TestEvents(t *testing.T) {
	m := setupTestManager(t)
	defer m.Shutdown()

	m.RegisterNode(&NASNode{ID: "node-1", Name: "NAS-1"})
	m.RegisterNode(&NASNode{ID: "node-2", Name: "NAS-2"})

	events := m.GetEvents("", "", 100)
	if len(events) < 2 {
		t.Fatalf("期望至少 2 个事件，实际有 %d 个", len(events))
	}

	// 按节点过滤.
	node1Events := m.GetEvents("node-1", "", 100)
	for _, e := range node1Events {
		if e.NodeID != "node-1" {
			t.Errorf("期望节点ID为 node-1，实际为 %s", e.NodeID)
		}
	}
}

func TestMigration(t *testing.T) {
	m := setupTestManager(t)
	defer m.Shutdown()

	m.RegisterNode(&NASNode{ID: "node-1", Name: "NAS-1"})
	m.RegisterNode(&NASNode{ID: "node-2", Name: "NAS-2"})

	task, err := m.CreateMigration("node-1", "node-2", "/data/file1", "/data/file1", 1024)
	if err != nil {
		t.Fatalf("创建迁移任务失败：%v", err)
	}

	if task.Status != MigrationStatusPending {
		t.Errorf("期望状态为 pending，实际为 %s", task.Status)
	}

	// 更新进度.
	err = m.UpdateMigrationProgress(task.ID, 512, MigrationStatusRunning, "")
	if err != nil {
		t.Fatalf("更新迁移进度失败：%v", err)
	}

	result, _ := m.GetMigration(task.ID)
	if result.CopiedBytes != 512 {
		t.Errorf("期望 CopiedBytes 为 512，实际为 %d", result.CopiedBytes)
	}

	// 完成迁移.
	m.UpdateMigrationProgress(task.ID, 1024, MigrationStatusCompleted, "")

	migrations := m.GetMigrations(MigrationStatusCompleted)
	if len(migrations) != 1 {
		t.Errorf("期望 1 个已完成迁移，实际有 %d 个", len(migrations))
	}
}

func TestTopology(t *testing.T) {
	m := setupTestManager(t)
	defer m.Shutdown()

	m.RegisterNode(&NASNode{ID: "node-1", Name: "NAS-1", Status: NodeStatusOnline})
	m.RegisterNode(&NASNode{ID: "node-2", Name: "NAS-2", Status: NodeStatusOnline})
	m.RegisterNode(&NASNode{ID: "node-3", Name: "NAS-3", Status: NodeStatusOffline})

	topology := m.GetTopology()
	if topology.TotalNodes != 3 {
		t.Errorf("期望总节点数为 3，实际为 %d", topology.TotalNodes)
	}
	if topology.OnlineNodes != 2 {
		t.Errorf("期望在线节点数为 2，实际为 %d", topology.OnlineNodes)
	}
	if topology.LeaderID != "test-node-1" {
		t.Errorf("期望领导节点为 test-node-1，实际为 %s", topology.LeaderID)
	}
}

func TestSetLeader(t *testing.T) {
	m := setupTestManager(t)
	defer m.Shutdown()

	m.RegisterNode(&NASNode{ID: "node-1", Name: "NAS-1"})
	m.RegisterNode(&NASNode{ID: "node-2", Name: "NAS-2"})

	m.SetLeader("node-2")
	topology := m.GetTopology()
	if topology.LeaderID != "node-2" {
		t.Errorf("期望领导节点为 node-2，实际为 %s", topology.LeaderID)
	}
}

// HTTP API 测试.

func TestAPIListNodes(t *testing.T) {
	router, _, m := setupTestRouter(t)
	defer m.Shutdown()

	m.RegisterNode(&NASNode{ID: "node-1", Name: "NAS-1", Status: NodeStatusOnline})

	req, _ := http.NewRequest("GET", "/api/v1/multinas/nodes", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["total"].(float64) != 1 {
		t.Errorf("期望 total 为 1，实际为 %v", resp["total"])
	}
}

func TestAPIGetNode(t *testing.T) {
	router, _, m := setupTestRouter(t)
	defer m.Shutdown()

	m.RegisterNode(&NASNode{ID: "node-1", Name: "NAS-1", Status: NodeStatusOnline})

	req, _ := http.NewRequest("GET", "/api/v1/multinas/nodes/node-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["name"] != "NAS-1" {
		t.Errorf("期望 name 为 NAS-1，实际为 %v", resp["name"])
	}
}

func TestAPIRegisterNode(t *testing.T) {
	router, _, m := setupTestRouter(t)
	defer m.Shutdown()

	body := `{"id":"node-1","name":"NAS-1","ip":"192.168.1.100","port":8080,"status":"online"}`
	req, _ := http.NewRequest("POST", "/api/v1/multinas/nodes", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("期望状态码 201，实际为 %d", w.Code)
	}

	nodes := m.GetNodes()
	if len(nodes) != 1 {
		t.Errorf("期望 1 个节点，实际有 %d 个", len(nodes))
	}
}

func TestAPIUpdateNodeStatus(t *testing.T) {
	router, _, m := setupTestRouter(t)
	defer m.Shutdown()

	m.RegisterNode(&NASNode{ID: "node-1", Name: "NAS-1", Status: NodeStatusOnline})

	body := `{"status":"degraded"}`
	req, _ := http.NewRequest("PUT", "/api/v1/multinas/nodes/node-1/status", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", w.Code)
	}
}

func TestAPIDeleteNode(t *testing.T) {
	router, _, m := setupTestRouter(t)
	defer m.Shutdown()

	m.RegisterNode(&NASNode{ID: "node-1", Name: "NAS-1"})

	req, _ := http.NewRequest("DELETE", "/api/v1/multinas/nodes/node-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", w.Code)
	}

	if len(m.GetNodes()) != 0 {
		t.Errorf("期望 0 个节点，实际有 %d 个", len(m.GetNodes()))
	}
}

func TestAPIListPools(t *testing.T) {
	router, _, m := setupTestRouter(t)
	defer m.Shutdown()

	m.RegisterNode(&NASNode{ID: "node-1", Name: "NAS-1"})
	m.RegisterPool(&StoragePool{ID: "pool-1", Name: "Pool 1", NodeID: "node-1", TotalSize: 1000})

	req, _ := http.NewRequest("GET", "/api/v1/multinas/pools", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", w.Code)
	}
}

func TestAPIAggregatedPools(t *testing.T) {
	router, _, m := setupTestRouter(t)
	defer m.Shutdown()

	m.RegisterNode(&NASNode{ID: "node-1", Name: "NAS-1"})
	m.RegisterNode(&NASNode{ID: "node-2", Name: "NAS-2"})
	m.RegisterPool(&StoragePool{ID: "pool-1", Name: "Main Pool", NodeID: "node-1", TotalSize: 1000, UsedSize: 500, FreeSize: 500})
	m.RegisterPool(&StoragePool{ID: "pool-2", Name: "Main Pool", NodeID: "node-2", TotalSize: 2000, UsedSize: 800, FreeSize: 1200})

	req, _ := http.NewRequest("GET", "/api/v1/multinas/pools/aggregated", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["total"].(float64) != 1 {
		t.Errorf("期望 1 个聚合池，实际为 %v", resp["total"])
	}
}

func TestAPIListAlerts(t *testing.T) {
	router, _, m := setupTestRouter(t)
	defer m.Shutdown()

	// 触发一个告警.
	m.RegisterNode(&NASNode{ID: "node-1", Name: "NAS-1", Status: NodeStatusOnline, TotalStorage: 10000})
	m.UpdateNodeMetrics("node-1", 95.0, 50.0, 1000)

	req, _ := http.NewRequest("GET", "/api/v1/multinas/alerts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", w.Code)
	}
}

func TestAPITopology(t *testing.T) {
	router, _, m := setupTestRouter(t)
	defer m.Shutdown()

	m.RegisterNode(&NASNode{ID: "node-1", Name: "NAS-1", Status: NodeStatusOnline})

	req, _ := http.NewRequest("GET", "/api/v1/multinas/topology", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", w.Code)
	}
}

func TestAPICreateMigration(t *testing.T) {
	router, _, m := setupTestRouter(t)
	defer m.Shutdown()

	m.RegisterNode(&NASNode{ID: "node-1", Name: "NAS-1"})
	m.RegisterNode(&NASNode{ID: "node-2", Name: "NAS-2"})

	body := `{"source_node_id":"node-1","target_node_id":"node-2","source_path":"/data/file1","target_path":"/data/file1","total_bytes":1024}`
	req, _ := http.NewRequest("POST", "/api/v1/multinas/migrations", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("期望状态码 201，实际为 %d", w.Code)
	}
}

func TestAPIOverview(t *testing.T) {
	router, _, m := setupTestRouter(t)
	defer m.Shutdown()

	m.RegisterNode(&NASNode{ID: "node-1", Name: "NAS-1", Status: NodeStatusOnline, TotalStorage: 1000, UsedStorage: 500})
	m.RegisterNode(&NASNode{ID: "node-2", Name: "NAS-2", Status: NodeStatusOffline, TotalStorage: 2000, UsedStorage: 1000})

	req, _ := http.NewRequest("GET", "/api/v1/multinas/overview", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	cluster := resp["cluster"].(map[string]interface{})
	if cluster["total_nodes"].(float64) != 2 {
		t.Errorf("期望总节点数为 2，实际为 %v", cluster["total_nodes"])
	}
	if cluster["online_nodes"].(float64) != 1 {
		t.Errorf("期望在线节点数为 1，实际为 %v", cluster["online_nodes"])
	}
}

func TestNotFoundNode(t *testing.T) {
	router, _, m := setupTestRouter(t)
	defer m.Shutdown()

	req, _ := http.NewRequest("GET", "/api/v1/multinas/nodes/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("期望状态码 404，实际为 %d", w.Code)
	}
}

func TestInvalidRequest(t *testing.T) {
	router, _, m := setupTestRouter(t)
	defer m.Shutdown()

	body := `{"invalid json`
	req, _ := http.NewRequest("POST", "/api/v1/multinas/nodes", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 400，实际为 %d", w.Code)
	}
}

func TestMultipleNodesAndPools(t *testing.T) {
	m := setupTestManager(t)
	defer m.Shutdown()

	// 注册多个节点.
	for i := 0; i < 5; i++ {
		m.RegisterNode(&NASNode{
			ID:           fmt.Sprintf("node-%d", i),
			Name:         fmt.Sprintf("NAS-%d", i),
			Status:       NodeStatusOnline,
			TotalStorage: int64(1000 * (i + 1)),
			UsedStorage:  int64(500 * (i + 1)),
		})
	}

	nodes := m.GetNodes()
	if len(nodes) != 5 {
		t.Fatalf("期望 5 个节点，实际有 %d 个", len(nodes))
	}

	// 注册存储池.
	for i := 0; i < 5; i++ {
		m.RegisterPool(&StoragePool{
			ID:        fmt.Sprintf("pool-%d", i),
			Name:      "Shared Pool",
			NodeID:    fmt.Sprintf("node-%d", i),
			TotalSize: int64(1000 * (i + 1)),
			UsedSize:  int64(500 * (i + 1)),
			FreeSize:  int64(500 * (i + 1)),
		})
	}

	// 验证聚合视图.
	aggregated := m.GetAggregatedPools()
	if len(aggregated) != 1 {
		t.Fatalf("期望 1 个聚合池，实际有 %d 个", len(aggregated))
	}

	// 总大小：1000+2000+3000+4000+5000 = 15000.
	if aggregated[0].TotalSize != 15000 {
		t.Errorf("期望聚合总大小为 15000，实际为 %d", aggregated[0].TotalSize)
	}
}

func TestMigrationInvalidNodes(t *testing.T) {
	m := setupTestManager(t)
	defer m.Shutdown()

	_, err := m.CreateMigration("nonexistent", "node-2", "/src", "/dst", 100)
	if err == nil {
		t.Error("期望创建迁移时返回错误（源节点不存在）")
	}

	m.RegisterNode(&NASNode{ID: "node-1", Name: "NAS-1"})
	_, err = m.CreateMigration("node-1", "nonexistent", "/src", "/dst", 100)
	if err == nil {
		t.Error("期望创建迁移时返回错误（目标节点不存在）")
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := setupTestManager(t)
	defer m.Shutdown()

	// 并发注册节点.
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			m.RegisterNode(&NASNode{
				ID:     fmt.Sprintf("node-%d", id),
				Name:   fmt.Sprintf("NAS-%d", id),
				Status: NodeStatusOnline,
			})
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	nodes := m.GetNodes()
	if len(nodes) != 10 {
		t.Errorf("期望 10 个节点，实际有 %d 个", len(nodes))
	}
}
