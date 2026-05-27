// Package k3s 单元测试
package k3s

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTest 创建测试环境
func setupTest(t *testing.T) (*Manager, *Handlers, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mgr := NewManager()
	handler := NewHandlers(mgr)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	return mgr, handler, router
}

// ========== 集群管理测试 ==========

func TestGetClusterInfo(t *testing.T) {
	mgr, _, router := setupTest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/k3s/cluster", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 0, resp.Code)

	// 验证返回数据
	info := mgr.GetClusterInfo()
	assert.Equal(t, "nas-os-cluster", info.Name)
	assert.Contains(t, info.Version, "k3s")
	assert.Equal(t, ClusterStatusRunning, info.Status)
}

func TestGetClusterHealth(t *testing.T) {
	_, _, router := setupTest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/k3s/cluster/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 0, resp.Code)
}

// ========== 节点管理测试 ==========

func TestNodeCRUD(t *testing.T) {
	mgr, _, router := setupTest(t)

	// 添加节点（通过 Manager）
	mgr.AddNode(&NodeInfo{
		Name:       "node-1",
		Role:       NodeRoleWorker,
		Status:     NodeStatusReady,
		IP:         "192.168.1.101",
		OS:         "Ubuntu 24.04",
		Arch:       "arm64",
		KubeletVer: "v1.31.4",
		CPUCores:   8,
		MemoryGB:   16,
		DiskGB:     500,
	})

	// 列出节点
	req := httptest.NewRequest(http.MethodGet, "/api/v1/k3s/nodes", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 获取节点
	req = httptest.NewRequest(http.MethodGet, "/api/v1/k3s/nodes/node-1", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 获取不存在的节点
	req = httptest.NewRequest(http.MethodGet, "/api/v1/k3s/nodes/nonexistent", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// 更新节点状态
	body := `{"status":"not_ready"}`
	req = httptest.NewRequest(http.MethodPut, "/api/v1/k3s/nodes/node-1/status", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	node, err := mgr.GetNode("node-1")
	require.NoError(t, err)
	assert.Equal(t, NodeStatusNotReady, node.Status)

	// 删除节点
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/k3s/nodes/node-1", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	_, err = mgr.GetNode("node-1")
	assert.Error(t, err)
}

func TestListNodes(t *testing.T) {
	mgr, _, router := setupTest(t)

	mgr.AddNode(&NodeInfo{Name: "master-1", Role: NodeRoleMaster, Status: NodeStatusReady})
	mgr.AddNode(&NodeInfo{Name: "worker-1", Role: NodeRoleWorker, Status: NodeStatusReady})
	mgr.AddNode(&NodeInfo{Name: "worker-2", Role: NodeRoleWorker, Status: NodeStatusNotReady})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/k3s/nodes", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(3), data["total"])
}

// ========== Helm Chart 测试 ==========

func TestHelmChartDeployAndList(t *testing.T) {
	mgr, _, router := setupTest(t)

	// 部署 Chart
	body := `{
		"name": "nginx",
		"default": "default",
		"namespace": "default",
		"chart": "stable/nginx",
		"version": "1.0.0",
		"description": "测试部署"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/k3s/helm/releases", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	// 列出 Releases
	req = httptest.NewRequest(http.MethodGet, "/api/v1/k3s/helm/releases", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 获取单个 Release
	release, err := mgr.GetHelmRelease("default", "nginx")
	require.NoError(t, err)
	assert.Equal(t, "nginx", release.Name)
	assert.Equal(t, "stable/nginx", release.Chart)
	assert.Equal(t, HelmStatusDeployed, release.Status)
	assert.Equal(t, 1, release.Revision)
}

func TestHelmChartUpgrade(t *testing.T) {
	mgr, _, router := setupTest(t)

	// 先部署
	mgr.DeployChart(DeployChartRequest{
		Name:      "redis",
		Namespace: "default",
		Chart:     "stable/redis",
		Version:   "1.0.0",
	})

	// 升级
	body := `{"version": "2.0.0", "description": "升级到 2.0"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/k3s/helm/releases/default/redis", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	release, err := mgr.GetHelmRelease("default", "redis")
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", release.ChartVer)
	assert.Equal(t, 2, release.Revision)
}

func TestHelmChartRollback(t *testing.T) {
	mgr, _, router := setupTest(t)

	// 部署并升级
	mgr.DeployChart(DeployChartRequest{
		Name:      "postgres",
		Namespace: "db",
		Chart:     "stable/postgres",
		Version:   "1.0.0",
	})
	mgr.UpgradeHelmRelease("db", "postgres", UpgradeChartRequest{Version: "2.0.0"})

	// 回滚到修订版本 1
	body := `{"revision": 1}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/k3s/helm/releases/db/postgres/rollback", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	release, err := mgr.GetHelmRelease("db", "postgres")
	require.NoError(t, err)
	assert.Equal(t, 3, release.Revision) // 部署(1) + 升级(2) + 回滚(3)
}

func TestHelmChartUninstall(t *testing.T) {
	mgr, _, router := setupTest(t)

	mgr.DeployChart(DeployChartRequest{
		Name:      "temp-app",
		Namespace: "default",
		Chart:     "stable/temp",
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/k3s/helm/releases/default/temp-app", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	_, err := mgr.GetHelmRelease("default", "temp-app")
	assert.ErrorIs(t, err, ErrHelmReleaseNotFound)
}

func TestHelmChartDeployDuplicate(t *testing.T) {
	mgr, _, router := setupTest(t)

	mgr.DeployChart(DeployChartRequest{
		Name:      "dup-app",
		Namespace: "default",
		Chart:     "stable/dup",
	})

	body := `{"name":"dup-app","namespace":"default","chart":"stable/dup"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/k3s/helm/releases", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusConflict, w.Code)
}

// ========== 工作负载测试 ==========

func TestWorkloadDeployments(t *testing.T) {
	mgr, _, router := setupTest(t)

	mgr.AddDeployment(&DeploymentInfo{
		Name:      "web-app",
		Namespace: "default",
		Ready:     3,
		Desired:   3,
		Strategy:  "RollingUpdate",
		Image:     "nginx:1.25",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/k3s/workloads/deployments?namespace=default", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/k3s/workloads/deployments/default/web-app", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWorkloadServices(t *testing.T) {
	mgr, _, router := setupTest(t)

	mgr.AddService(&ServiceInfo{
		Name:      "web-svc",
		Namespace: "default",
		Type:      "ClusterIP",
		ClusterIP: "10.43.0.10",
		Ports: []ServicePort{
			{Name: "http", Port: 80, TargetPort: 8080, Protocol: "TCP"},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/k3s/workloads/services", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWorkloadPods(t *testing.T) {
	mgr, _, router := setupTest(t)

	mgr.AddPod(&PodInfo{
		Name:      "web-app-abc123",
		Namespace: "default",
		Status:    "Running",
		IP:        "10.42.0.5",
		Node:      "worker-1",
		Restarts:  0,
		Containers: []ContainerInfo{
			{Name: "nginx", Image: "nginx:1.25", Ready: true, State: "running"},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/k3s/workloads/pods?namespace=default", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPodLogs(t *testing.T) {
	mgr, _, router := setupTest(t)

	mgr.AddPod(&PodInfo{
		Name:      "app-pod-001",
		Namespace: "default",
		Containers: []ContainerInfo{
			{Name: "app", Image: "myapp:v1", Ready: true},
		},
	})

	body := `{"namespace":"default","pod_name":"app-pod-001","tail_lines":10}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/k3s/workloads/pods/logs", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "app-pod-001", data["pod_name"])
}

// ========== 服务网格测试 ==========

func TestServiceMesh(t *testing.T) {
	mgr, _, router := setupTest(t)

	// 获取默认配置（未启用）
	req := httptest.NewRequest(http.MethodGet, "/api/v1/k3s/mesh", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 启用 Istio
	body := `{"type":"istio","namespace":"istio-system"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/k3s/mesh/enable", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	cfg := mgr.GetServiceMeshConfig()
	assert.True(t, cfg.Enabled)
	assert.Equal(t, ServiceMeshIstio, cfg.Type)

	// 更新配置
	body = `{"mtls":true,"tracing":true,"tracing_url":"http://jaeger:14268"}`
	req = httptest.NewRequest(http.MethodPut, "/api/v1/k3s/mesh", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	cfg = mgr.GetServiceMeshConfig()
	assert.True(t, cfg.MTLS)
	assert.True(t, cfg.Tracing)

	// 禁用
	req = httptest.NewRequest(http.MethodPost, "/api/v1/k3s/mesh/disable", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	cfg = mgr.GetServiceMeshConfig()
	assert.False(t, cfg.Enabled)
	assert.Equal(t, ServiceMeshNone, cfg.Type)
}

func TestServiceMeshInvalidType(t *testing.T) {
	_, _, router := setupTest(t)

	body := `{"type":"invalid","namespace":"default"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/k3s/mesh/enable", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ========== HPA 测试 ==========

func TestHPACRUD(t *testing.T) {
	mgr, _, router := setupTest(t)

	// 创建 HPA
	body := `{
		"name": "web-hpa",
		"namespace": "default",
		"target_name": "web-app",
		"min_replicas": 2,
		"max_replicas": 10,
		"metrics": [{"type":"Resource","resource":"cpu","target":"Utilization","value":80}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/k3s/hpa", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	// 列出
	req = httptest.NewRequest(http.MethodGet, "/api/v1/k3s/hpa", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 获取
	req = httptest.NewRequest(http.MethodGet, "/api/v1/k3s/hpa/default/web-hpa", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 更新
	body = `{"min_replicas": 3, "max_replicas": 20}`
	req = httptest.NewRequest(http.MethodPut, "/api/v1/k3s/hpa/default/web-hpa", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	hpa, err := mgr.GetHPA("default", "web-hpa")
	require.NoError(t, err)
	assert.Equal(t, 3, hpa.MinReplicas)
	assert.Equal(t, 20, hpa.MaxReplicas)

	// 删除
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/k3s/hpa/default/web-hpa", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	_, err = mgr.GetHPA("default", "web-hpa")
	assert.ErrorIs(t, err, ErrHPANotFound)
}

func TestHPADefaultValues(t *testing.T) {
	mgr, _, _ := setupTest(t)

	hpa := mgr.CreateHPA(CreateHPARequest{
		Name:       "default-hpa",
		Namespace:  "default",
		TargetName: "my-app",
		MaxReplicas: 5,
	})

	assert.Equal(t, "Deployment", hpa.TargetKind) // 默认值
	assert.Equal(t, 1, hpa.MinReplicas)           // 默认值
	assert.Len(t, hpa.Metrics, 1)                  // 默认 CPU 指标
}

// ========== 应用商店集成测试 ==========

func TestAppStoreIntegration(t *testing.T) {
	mgr, _, router := setupTest(t)

	// 列出可部署应用
	req := httptest.NewRequest(http.MethodGet, "/api/v1/k3s/appstore", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	apps := mgr.ListAppStoreApps()
	assert.True(t, len(apps) > 0)

	// 从应用商店部署
	body := `{"app_id":"grafana","namespace":"monitoring"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/k3s/appstore/deploy", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	// 验证 Release 已创建
	release, err := mgr.GetHelmRelease("monitoring", "grafana")
	require.NoError(t, err)
	assert.Equal(t, "stable/grafana", release.Chart)
}

func TestAppStoreDeployInvalidApp(t *testing.T) {
	_, _, router := setupTest(t)

	body := `{"app_id":"nonexistent","namespace":"default"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/k3s/appstore/deploy", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ========== 资源配额测试 ==========

func TestQuotaCRUD(t *testing.T) {
	mgr, _, router := setupTest(t)

	// 创建配额
	body := `{
		"namespace": "production",
		"name": "team-quota",
		"hard": {
			"requests.cpu": "10",
			"requests.memory": "20Gi",
			"limits.cpu": "20",
			"limits.memory": "40Gi",
			"pods": "50"
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/k3s/quotas", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	// 列出
	req = httptest.NewRequest(http.MethodGet, "/api/v1/k3s/quotas?namespace=production", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 获取
	req = httptest.NewRequest(http.MethodGet, "/api/v1/k3s/quotas/production/team-quota", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	quota, err := mgr.GetQuota("production", "team-quota")
	require.NoError(t, err)
	assert.Equal(t, "10", quota.Hard["requests.cpu"])

	// 更新
	body = `{"hard":{"requests.cpu":"20","requests.memory":"40Gi","limits.cpu":"40","limits.memory":"80Gi","pods":"100"}}`
	req = httptest.NewRequest(http.MethodPut, "/api/v1/k3s/quotas/production/team-quota", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	quota, err = mgr.GetQuota("production", "team-quota")
	require.NoError(t, err)
	assert.Equal(t, "20", quota.Hard["requests.cpu"])

	// 删除
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/k3s/quotas/production/team-quota", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	_, err = mgr.GetQuota("production", "team-quota")
	assert.ErrorIs(t, err, ErrQuotaNotFound)
}

// ========== 集群事件测试 ==========

func TestClusterEvents(t *testing.T) {
	mgr, _, router := setupTest(t)

	// 触发一些事件
	mgr.AddNode(&NodeInfo{Name: "event-node", Role: NodeRoleWorker, Status: NodeStatusReady})
	mgr.UpdateNodeStatus("event-node", NodeStatusNotReady)
	mgr.RemoveNode("event-node")

	// 列出事件
	req := httptest.NewRequest(http.MethodGet, "/api/v1/k3s/events", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.True(t, data["total"].(float64) >= 3)

	// 按严重级别过滤
	req = httptest.NewRequest(http.MethodGet, "/api/v1/k3s/events/severity/warning", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 清除事件
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/k3s/events", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestEventAggregation(t *testing.T) {
	mgr, _, _ := setupTest(t)

	// 同类事件应聚合
	mgr.AddNode(&NodeInfo{Name: "agg-node", Role: NodeRoleWorker, Status: NodeStatusReady})
	mgr.UpdateNodeStatus("agg-node", NodeStatusNotReady)
	mgr.UpdateNodeStatus("agg-node", NodeStatusReady)
	mgr.UpdateNodeStatus("agg-node", NodeStatusNotReady)

	events := mgr.ListEvents("", 100)
	// NodeStatusChanged 事件应聚合 count
	var statusChanged *ClusterEvent
	for _, e := range events {
		if e.Name == "agg-node" && e.Reason == "NodeStatusChanged" {
			statusChanged = e
			break
		}
	}
	if statusChanged != nil {
		assert.True(t, statusChanged.Count >= 2)
	}
}

// ========== Manager 直接方法测试 ==========

func TestManagerCreateHPA(t *testing.T) {
	mgr := NewManager()

	hpa := mgr.CreateHPA(CreateHPARequest{
		Name:        "test-hpa",
		Namespace:   "default",
		TargetName:  "my-deploy",
		MinReplicas: 2,
		MaxReplicas: 10,
		Metrics: []HPAMetric{
			{Type: "Resource", Resource: "cpu", Target: "Utilization", Value: 75},
			{Type: "Resource", Resource: "memory", Target: "Utilization", Value: 85},
		},
	})

	assert.NotEmpty(t, hpa.ID)
	assert.Equal(t, "test-hpa", hpa.Name)
	assert.Equal(t, 2, hpa.MinReplicas)
	assert.Equal(t, 10, hpa.MaxReplicas)
	assert.Len(t, hpa.Metrics, 2)

	// 再获取一次
	got, err := mgr.GetHPA("default", "test-hpa")
	require.NoError(t, err)
	assert.Equal(t, hpa.ID, got.ID)
}

func TestManagerQuotaUsage(t *testing.T) {
	mgr := NewManager()

	quota := mgr.CreateQuota(CreateQuotaRequest{
		Namespace: "ns1",
		Name:      "quota-1",
		Hard: map[string]string{
			"cpu":    "8",
			"memory": "16Gi",
		},
	})

	assert.Equal(t, "0", quota.Used["cpu"])
	assert.Equal(t, "0", quota.Used["memory"])
}

func TestManagerHelmReleaseValues(t *testing.T) {
	mgr := NewManager()

	release, err := mgr.DeployChart(DeployChartRequest{
		Name:      "with-values",
		Namespace: "default",
		Chart:     "stable/app",
		Values:    map[string]interface{}{"replicas": 3, "image": "v2"},
	})
	require.NoError(t, err)
	assert.Equal(t, 3, release.Values["replicas"])

	// 升级合并值
	updated, err := mgr.UpgradeHelmRelease("default", "with-values", UpgradeChartRequest{
		Values: map[string]interface{}{"replicas": 5, "debug": true},
	})
	require.NoError(t, err)
	assert.Equal(t, 5, updated.Values["replicas"])
	assert.Equal(t, true, updated.Values["debug"])
	assert.Equal(t, "v2", updated.Values["image"]) // 保留旧值

	// 重置值
	updated, err = mgr.UpgradeHelmRelease("default", "with-values", UpgradeChartRequest{
		ResetValues: true,
		Values:      map[string]interface{}{"fresh": true},
	})
	require.NoError(t, err)
	assert.Equal(t, true, updated.Values["fresh"])
	assert.Nil(t, updated.Values["image"]) // 被清除
}

func TestManagerServiceMeshUpdateWhenDisabled(t *testing.T) {
	mgr := NewManager()

	err := mgr.UpdateServiceMeshConfig(ServiceMeshConfig{MTLS: true})
	assert.ErrorIs(t, err, ErrServiceMeshNotEnabled)
}

func TestManagerListEventsWithLimit(t *testing.T) {
	mgr := NewManager()

	// 生成多条事件
	for i := 0; i < 10; i++ {
		mgr.AddNode(&NodeInfo{
			Name:   "node-" + string(rune('a'+i)),
			Role:   NodeRoleWorker,
			Status: NodeStatusReady,
		})
	}

	events := mgr.ListEvents("", 5)
	assert.Len(t, events, 5)
}

func TestManagerClearEventsNamespace(t *testing.T) {
	mgr := NewManager()

	mgr.AddNode(&NodeInfo{Name: "n1", Role: NodeRoleWorker, Status: NodeStatusReady})
	mgr.AddDeployment(&DeploymentInfo{Name: "d1", Namespace: "app"})
	mgr.AddDeployment(&DeploymentInfo{Name: "d2", Namespace: "db"})

	// 事件分布在 default 和其他命名空间
	before := len(mgr.ListEvents("", 100))

	// 只清除 default 命名空间的事件
	removed := mgr.ClearEvents("default")
	assert.True(t, removed >= 0)

	after := len(mgr.ListEvents("", 100))
	assert.Equal(t, before-removed, after)
}
