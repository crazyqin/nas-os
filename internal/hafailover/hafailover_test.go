// Package hafailover 高可用故障转移模块
package hafailover

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestManager(t *testing.T) (*Manager, string) {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "ha_config.json")
	mgr := NewManager(configPath)
	return mgr, configPath
}

func setupTestRouter(mgr *Manager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	group := r.Group("/api/v1/ha")
	RegisterRoutes(group, mgr)
	return r
}

// ========== 配置管理测试 ==========

func TestGetConfig_DefaultConfig(t *testing.T) {
	mgr, _ := setupTestManager(t)
	config := mgr.GetConfig()

	assert.NotNil(t, config)
	assert.Equal(t, "nas-os-ha", config.ClusterName)
	assert.True(t, config.AutoFailover)
	assert.Equal(t, 5, config.FailoverDelay)
	assert.Len(t, config.Heartbeats, 3)
}

func TestUpdateConfig_Success(t *testing.T) {
	mgr, _ := setupTestManager(t)

	req := &HAConfig{
		ClusterName:   "test-cluster",
		LocalNodeID:   "node-1",
		PeerNodeID:    "node-2",
		AutoFailover:  true,
		FailoverDelay: 10,
		Heartbeats: map[HeartbeatLevel]HeartbeatConfig{
			HeartbeatNetwork: {Interval: 5, Timeout: 15, MaxRetries: 3},
		},
		VIP: VIPConfig{
			Enabled:   true,
			VIP:       "192.168.1.100",
			Interface: "eth0",
			Netmask:   "255.255.255.0",
		},
		Sync: SyncConfig{
			StorageSync:  true,
			ServiceSync:  true,
			SyncInterval: 60,
		},
	}

	config, err := mgr.UpdateConfig(req)
	require.NoError(t, err)
	assert.Equal(t, "test-cluster", config.ClusterName)
	assert.Equal(t, "node-1", config.LocalNodeID)
	assert.Equal(t, "node-2", config.PeerNodeID)
	assert.True(t, config.VIP.Enabled)
	assert.Equal(t, "192.168.1.100", config.VIP.VIP)
}

func TestUpdateConfig_EmptyClusterName(t *testing.T) {
	mgr, _ := setupTestManager(t)

	req := &HAConfig{
		LocalNodeID: "node-1",
	}

	_, err := mgr.UpdateConfig(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "集群名称不能为空")
}

func TestUpdateConfig_EmptyLocalNodeID(t *testing.T) {
	mgr, _ := setupTestManager(t)

	req := &HAConfig{
		ClusterName: "test-cluster",
	}

	_, err := mgr.UpdateConfig(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "本节点ID不能为空")
}

// ========== 节点管理测试 ==========

func TestRegisterNode_Success(t *testing.T) {
	mgr, _ := setupTestManager(t)

	req := &NodeInfo{
		Name:     "node-1",
		Hostname: "nas-1",
		IP:       "192.168.1.10",
		Role:     RoleActive,
	}

	node, err := mgr.RegisterNode(req)
	require.NoError(t, err)
	assert.NotEmpty(t, node.ID)
	assert.Equal(t, "node-1", node.Name)
	assert.Equal(t, "192.168.1.10", node.IP)
	assert.Equal(t, RoleActive, node.Role)
	assert.Equal(t, StatusOnline, node.Status)
}

func TestRegisterNode_SecondNode(t *testing.T) {
	mgr, _ := setupTestManager(t)

	_, err := mgr.RegisterNode(&NodeInfo{
		Name: "node-1", IP: "192.168.1.10", Role: RoleActive,
	})
	require.NoError(t, err)

	node2, err := mgr.RegisterNode(&NodeInfo{
		Name: "node-2", IP: "192.168.1.11", Role: RoleStandby,
	})
	require.NoError(t, err)
	assert.Equal(t, "node-2", node2.Name)
}

func TestRegisterNode_ClusterFull(t *testing.T) {
	mgr, _ := setupTestManager(t)

	_, err := mgr.RegisterNode(&NodeInfo{
		Name: "node-1", IP: "192.168.1.10", Role: RoleActive,
	})
	require.NoError(t, err)

	_, err = mgr.RegisterNode(&NodeInfo{
		Name: "node-2", IP: "192.168.1.11", Role: RoleStandby,
	})
	require.NoError(t, err)

	_, err = mgr.RegisterNode(&NodeInfo{
		Name: "node-3", IP: "192.168.1.12", Role: RoleStandby,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "集群已满")
}

func TestRegisterNode_EmptyName(t *testing.T) {
	mgr, _ := setupTestManager(t)

	_, err := mgr.RegisterNode(&NodeInfo{
		IP: "192.168.1.10",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "节点名称不能为空")
}

func TestListNodes_Empty(t *testing.T) {
	mgr, _ := setupTestManager(t)

	nodes := mgr.ListNodes()
	assert.Empty(t, nodes)
}

func TestListNodes_WithNodes(t *testing.T) {
	mgr, _ := setupTestManager(t)

	_, _ = mgr.RegisterNode(&NodeInfo{
		Name: "node-1", IP: "192.168.1.10", Role: RoleActive,
	})
	_, _ = mgr.RegisterNode(&NodeInfo{
		Name: "node-2", IP: "192.168.1.11", Role: RoleStandby,
	})

	nodes := mgr.ListNodes()
	assert.Len(t, nodes, 2)
}

func TestGetNode_Found(t *testing.T) {
	mgr, _ := setupTestManager(t)

	node, _ := mgr.RegisterNode(&NodeInfo{
		Name: "node-1", IP: "192.168.1.10", Role: RoleActive,
	})

	found, err := mgr.GetNode(node.ID)
	require.NoError(t, err)
	assert.Equal(t, node.ID, found.ID)
	assert.Equal(t, "node-1", found.Name)
}

func TestGetNode_NotFound(t *testing.T) {
	mgr, _ := setupTestManager(t)

	_, err := mgr.GetNode("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

// ========== HA状态测试 ==========

func TestGetHAStatus_NoNodes(t *testing.T) {
	mgr, _ := setupTestManager(t)

	status := mgr.GetHAStatus()
	assert.NotNil(t, status)
	assert.Nil(t, status.ActiveNode)
	assert.Nil(t, status.StandbyNode)
	assert.Equal(t, 100, status.HealthScore)
}

func TestGetHAStatus_WithNodes(t *testing.T) {
	mgr, _ := setupTestManager(t)

	_, _ = mgr.RegisterNode(&NodeInfo{
		Name: "node-1", IP: "192.168.1.10", Role: RoleActive,
	})
	_, _ = mgr.RegisterNode(&NodeInfo{
		Name: "node-2", IP: "192.168.1.11", Role: RoleStandby,
	})

	status := mgr.GetHAStatus()
	assert.NotNil(t, status.ActiveNode)
	assert.NotNil(t, status.StandbyNode)
	assert.Equal(t, RoleActive, status.ActiveNode.Role)
	assert.Equal(t, RoleStandby, status.StandbyNode.Role)
}

// ========== 故障切换测试 ==========

func TestManualFailover_Success(t *testing.T) {
	mgr, _ := setupTestManager(t)

	_, _ = mgr.RegisterNode(&NodeInfo{
		Name: "node-1", IP: "192.168.1.10", Role: RoleActive,
	})
	_, _ = mgr.RegisterNode(&NodeInfo{
		Name: "node-2", IP: "192.168.1.11", Role: RoleStandby,
	})

	req := &FailoverRequest{
		Reason: "手动测试切换",
	}

	event, err := mgr.ManualFailover(req)
	require.NoError(t, err)
	assert.True(t, event.Success)
	assert.Equal(t, TriggerManual, event.Trigger)
	assert.NotEmpty(t, event.ID)
	assert.NotNil(t, event.CompletedAt)

	// 验证角色已切换
	nodes := mgr.ListNodes()
	for _, n := range nodes {
		if n.Name == "node-1" {
			assert.Equal(t, RoleStandby, n.Role)
		}
		if n.Name == "node-2" {
			assert.Equal(t, RoleActive, n.Role)
		}
	}
}

func TestManualFailover_EmptyReason(t *testing.T) {
	mgr, _ := setupTestManager(t)

	_, _ = mgr.RegisterNode(&NodeInfo{
		Name: "node-1", IP: "192.168.1.10", Role: RoleActive,
	})
	_, _ = mgr.RegisterNode(&NodeInfo{
		Name: "node-2", IP: "192.168.1.11", Role: RoleStandby,
	})

	_, err := mgr.ManualFailover(&FailoverRequest{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "请提供切换原因")
}

func TestManualFailover_IncompleteNodes(t *testing.T) {
	mgr, _ := setupTestManager(t)

	_, _ = mgr.RegisterNode(&NodeInfo{
		Name: "node-1", IP: "192.168.1.10", Role: RoleActive,
	})

	_, err := mgr.ManualFailover(&FailoverRequest{Reason: "测试"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "节点信息不完整")
}

func TestGetFailoverHistory_Empty(t *testing.T) {
	mgr, _ := setupTestManager(t)

	events := mgr.GetFailoverHistory(10)
	assert.Empty(t, events)
}

func TestGetFailoverHistory_WithEvents(t *testing.T) {
	mgr, _ := setupTestManager(t)

	_, _ = mgr.RegisterNode(&NodeInfo{
		Name: "node-1", IP: "192.168.1.10", Role: RoleActive,
	})
	_, _ = mgr.RegisterNode(&NodeInfo{
		Name: "node-2", IP: "192.168.1.11", Role: RoleStandby,
	})

	_, _ = mgr.ManualFailover(&FailoverRequest{Reason: "测试1"})
	_, _ = mgr.ManualFailover(&FailoverRequest{Reason: "测试2"})

	events := mgr.GetFailoverHistory(10)
	assert.Len(t, events, 2)
}

func TestGetFailoverHistory_WithLimit(t *testing.T) {
	mgr, _ := setupTestManager(t)

	_, _ = mgr.RegisterNode(&NodeInfo{
		Name: "node-1", IP: "192.168.1.10", Role: RoleActive,
	})
	_, _ = mgr.RegisterNode(&NodeInfo{
		Name: "node-2", IP: "192.168.1.11", Role: RoleStandby,
	})

	for i := 0; i < 5; i++ {
		_, _ = mgr.ManualFailover(&FailoverRequest{Reason: "测试"})
	}

	events := mgr.GetFailoverHistory(3)
	assert.Len(t, events, 3)
}

// ========== 同步测试 ==========

func TestTriggerSync_Success(t *testing.T) {
	mgr, _ := setupTestManager(t)

	status, err := mgr.TriggerSync()
	require.NoError(t, err)
	assert.Equal(t, SyncStateSyncing, status.State)
}

func TestTriggerSync_AlreadySyncing(t *testing.T) {
	mgr, _ := setupTestManager(t)

	_, err := mgr.TriggerSync()
	require.NoError(t, err)

	_, err = mgr.TriggerSync()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "同步正在进行中")
}

func TestGetSyncStatus_Idle(t *testing.T) {
	mgr, _ := setupTestManager(t)

	status := mgr.GetSyncStatus()
	assert.Equal(t, SyncStateIdle, status.State)
}

// ========== 心跳测试 ==========

func TestStartHeartbeat_Success(t *testing.T) {
	mgr, _ := setupTestManager(t)

	// 先设置配置
	_, _ = mgr.UpdateConfig(&HAConfig{
		ClusterName: "test",
		LocalNodeID: "node-1",
		Heartbeats: map[HeartbeatLevel]HeartbeatConfig{
			HeartbeatNetwork: {Interval: 1, Timeout: 3, MaxRetries: 1},
		},
	})

	err := mgr.StartHeartbeat(HeartbeatNetwork)
	require.NoError(t, err)

	// 清理
	_ = mgr.StopHeartbeat(HeartbeatNetwork)
}

func TestStartHeartbeat_AlreadyRunning(t *testing.T) {
	mgr, _ := setupTestManager(t)

	_, _ = mgr.UpdateConfig(&HAConfig{
		ClusterName: "test",
		LocalNodeID: "node-1",
		Heartbeats: map[HeartbeatLevel]HeartbeatConfig{
			HeartbeatNetwork: {Interval: 1, Timeout: 3, MaxRetries: 1},
		},
	})

	_ = mgr.StartHeartbeat(HeartbeatNetwork)
	defer mgr.StopHeartbeat(HeartbeatNetwork)

	err := mgr.StartHeartbeat(HeartbeatNetwork)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已在运行")
}

func TestStopHeartbeat_NotRunning(t *testing.T) {
	mgr, _ := setupTestManager(t)

	err := mgr.StopHeartbeat(HeartbeatNetwork)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未在运行")
}

func TestGetHeartbeatStatus(t *testing.T) {
	mgr, _ := setupTestManager(t)

	_, _ = mgr.UpdateConfig(&HAConfig{
		ClusterName: "test",
		LocalNodeID: "node-1",
		Heartbeats: map[HeartbeatLevel]HeartbeatConfig{
			HeartbeatNetwork: {Interval: 1, Timeout: 3, MaxRetries: 1},
		},
	})

	_ = mgr.StartHeartbeat(HeartbeatNetwork)
	defer mgr.StopHeartbeat(HeartbeatNetwork)

	status := mgr.GetHeartbeatStatus()
	assert.Len(t, status, 1)
}

// ========== API 测试 ==========

func TestAPI_GetConfig(t *testing.T) {
	mgr, _ := setupTestManager(t)
	r := setupTestRouter(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ha/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

func TestAPI_UpdateConfig(t *testing.T) {
	mgr, _ := setupTestManager(t)
	r := setupTestRouter(mgr)

	config := HAConfig{
		ClusterName:   "api-test",
		LocalNodeID:   "node-1",
		AutoFailover:  true,
		FailoverDelay: 5,
		Heartbeats: map[HeartbeatLevel]HeartbeatConfig{
			HeartbeatNetwork: {Interval: 5, Timeout: 15, MaxRetries: 3},
		},
	}
	body, _ := json.Marshal(config)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/ha/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

func TestAPI_ListNodes(t *testing.T) {
	mgr, _ := setupTestManager(t)
	r := setupTestRouter(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ha/nodes", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPI_RegisterNode(t *testing.T) {
	mgr, _ := setupTestManager(t)
	r := setupTestRouter(mgr)

	node := NodeInfo{
		Name:     "api-node",
		Hostname: "nas-api",
		IP:       "192.168.1.20",
		Role:     RoleActive,
	}
	body, _ := json.Marshal(node)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ha/nodes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

func TestAPI_GetHAStatus(t *testing.T) {
	mgr, _ := setupTestManager(t)
	r := setupTestRouter(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ha/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

func TestAPI_ManualFailover(t *testing.T) {
	mgr, _ := setupTestManager(t)
	r := setupTestRouter(mgr)

	// 注册节点
	_, _ = mgr.RegisterNode(&NodeInfo{
		Name: "node-1", IP: "192.168.1.10", Role: RoleActive,
	})
	_, _ = mgr.RegisterNode(&NodeInfo{
		Name: "node-2", IP: "192.168.1.11", Role: RoleStandby,
	})

	failoverReq := FailoverRequest{
		Reason: "API测试切换",
	}
	body, _ := json.Marshal(failoverReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ha/failover", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

func TestAPI_GetFailoverHistory(t *testing.T) {
	mgr, _ := setupTestManager(t)
	r := setupTestRouter(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ha/failover/history?limit=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPI_TriggerSync(t *testing.T) {
	mgr, _ := setupTestManager(t)
	r := setupTestRouter(mgr)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ha/sync", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPI_GetSyncStatus(t *testing.T) {
	mgr, _ := setupTestManager(t)
	r := setupTestRouter(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ha/sync/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAPI_GetHeartbeatStatus(t *testing.T) {
	mgr, _ := setupTestManager(t)
	r := setupTestRouter(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ha/heartbeat/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ========== 配置持久化测试 ==========

func TestConfigPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "ha_config.json")

	mgr1 := NewManager(configPath)
	_, _ = mgr1.UpdateConfig(&HAConfig{
		ClusterName: "persist-test",
		LocalNodeID: "node-1",
		PeerNodeID:  "node-2",
	})

	// 重新加载
	mgr2 := NewManager(configPath)
	config := mgr2.GetConfig()
	assert.Equal(t, "persist-test", config.ClusterName)
	assert.Equal(t, "node-1", config.LocalNodeID)
	assert.Equal(t, "node-2", config.PeerNodeID)
}

func TestMain(m *testing.M) {
	// 清理测试文件
	os.Exit(m.Run())
}
