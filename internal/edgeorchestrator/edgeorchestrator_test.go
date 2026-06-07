package edgeorchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestRouter(manager *Manager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	handlers := NewHandlers(manager)
	handlers.RegisterRoutes(v1)
	return r
}

func TestRegisterNode(t *testing.T) {
	manager := NewManager(nil)
	r := setupTestRouter(manager)

	body := `{
		"name": "edge-node-1",
		"ip_address": "192.168.1.100",
		"region": "cn-east",
		"zone": "zone-a",
		"cpu_cores": 8,
		"memory_mb": 16384,
		"gpu_count": 1,
		"gpu_model": "NVIDIA T4",
		"labels": {"env": "production"},
		"max_tasks": 20
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/edgeorch/nodes", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["code"].(float64) != 0 {
		t.Errorf("expected code 0, got %v", resp["code"])
	}

	data := resp["data"].(map[string]interface{})
	if data["name"] != "edge-node-1" {
		t.Errorf("expected name edge-node-1, got %v", data["name"])
	}
}

func TestListNodes(t *testing.T) {
	manager := NewManager(nil)
	r := setupTestRouter(manager)

	// 注册两个节点
	manager.RegisterNode(&RegisterNodeRequest{
		Name:      "node-1",
		IPAddress: "10.0.0.1",
		CPUCores:  4,
		MemoryMB:  8192,
	})
	manager.RegisterNode(&RegisterNodeRequest{
		Name:      "node-2",
		IPAddress: "10.0.0.2",
		CPUCores:  8,
		MemoryMB:  16384,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/edgeorch/nodes", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	if data["total"].(float64) != 2 {
		t.Errorf("expected 2 nodes, got %v", data["total"])
	}
}

func TestSubmitTask(t *testing.T) {
	manager := NewManager(nil)
	r := setupTestRouter(manager)

	body := `{
		"name": "test-task",
		"type": "ai_inference",
		"image": "tensorflow/serving:latest",
		"cpu_request": 2.0,
		"memory_request_mb": 4096,
		"gpu_request": 1,
		"timeout_sec": 300,
		"max_retries": 3,
		"labels": {"model": "resnet50"}
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/edgeorch/tasks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["code"].(float64) != 0 {
		t.Errorf("expected code 0, got %v", resp["code"])
	}

	data := resp["data"].(map[string]interface{})
	if data["name"] != "test-task" {
		t.Errorf("expected name test-task, got %v", data["name"])
	}
}

func TestScheduleAndStartTask(t *testing.T) {
	manager := NewManager(nil)
	r := setupTestRouter(manager)

	// 注册节点
	node, _ := manager.RegisterNode(&RegisterNodeRequest{
		Name:      "node-1",
		IPAddress: "10.0.0.1",
		CPUCores:  8,
		MemoryMB:  16384,
	})

	// 提交任务
	task, _ := manager.SubmitTask(&SubmitTaskRequest{
		Name:            "test-task",
		Image:           "nginx:latest",
		CPURequest:      1.0,
		MemoryRequestMB: 512,
	})

	// 调度任务
	scheduleReq := httptest.NewRequest(http.MethodPost, "/api/v1/edgeorch/tasks/"+task.ID+"/schedule", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, scheduleReq)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// 验证任务被分配到节点
	task, _ = manager.GetTask(task.ID)
	if task.AssignedNodeID != node.ID {
		t.Errorf("expected task assigned to node %s, got %s", node.ID, task.AssignedNodeID)
	}

	// 启动任务
	startReq := httptest.NewRequest(http.MethodPost, "/api/v1/edgeorch/tasks/"+task.ID+"/start", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, startReq)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	task, _ = manager.GetTask(task.ID)
	if task.Status != TaskStatusRunning {
		t.Errorf("expected task status running, got %s", task.Status)
	}
}

func TestCompleteTask(t *testing.T) {
	manager := NewManager(nil)
	r := setupTestRouter(manager)

	// 注册节点并创建任务
	manager.RegisterNode(&RegisterNodeRequest{
		Name:      "node-1",
		IPAddress: "10.0.0.1",
		CPUCores:  8,
		MemoryMB:  16384,
	})

	task, _ := manager.SubmitTask(&SubmitTaskRequest{
		Name:            "test-task",
		Image:           "nginx:latest",
		CPURequest:      1.0,
		MemoryRequestMB: 512,
	})

	manager.ScheduleTask(task.ID)
	manager.StartTask(task.ID)

	// 完成任务
	body := `{"exit_code": 0, "output": "success"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/edgeorch/tasks/"+task.ID+"/complete", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	task, _ = manager.GetTask(task.ID)
	if task.Status != TaskStatusCompleted {
		t.Errorf("expected task status completed, got %s", task.Status)
	}
}

func TestCancelTask(t *testing.T) {
	manager := NewManager(nil)
	r := setupTestRouter(manager)

	task, _ := manager.SubmitTask(&SubmitTaskRequest{
		Name:            "test-task",
		Image:           "nginx:latest",
		CPURequest:      1.0,
		MemoryRequestMB: 512,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/edgeorch/tasks/"+task.ID+"/cancel", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	task, _ = manager.GetTask(task.ID)
	if task.Status != TaskStatusCancelled {
		t.Errorf("expected task status cancelled, got %s", task.Status)
	}
}

func TestListTasks(t *testing.T) {
	manager := NewManager(nil)
	r := setupTestRouter(manager)

	manager.SubmitTask(&SubmitTaskRequest{
		Name:  "task-1",
		Image: "nginx:latest",
	})
	manager.SubmitTask(&SubmitTaskRequest{
		Name:  "task-2",
		Image: "redis:latest",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/edgeorch/tasks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	if data["total"].(float64) != 2 {
		t.Errorf("expected 2 tasks, got %v", data["total"])
	}
}

func TestNodeHeartbeat(t *testing.T) {
	manager := NewManager(nil)
	r := setupTestRouter(manager)

	node, _ := manager.RegisterNode(&RegisterNodeRequest{
		Name:      "node-1",
		IPAddress: "10.0.0.1",
		CPUCores:  8,
		MemoryMB:  16384,
	})

	body := `{"cpu_usage": 50.5, "mem_usage": 60.2, "running_tasks": 3}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/edgeorch/nodes/"+node.ID+"/heartbeat", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	node, _ = manager.GetNode(node.ID)
	if node.CurrentCPUUsage != 50.5 {
		t.Errorf("expected CPU usage 50.5, got %f", node.CurrentCPUUsage)
	}
}

func TestClusterMetrics(t *testing.T) {
	manager := NewManager(nil)
	r := setupTestRouter(manager)

	// 注册节点
	manager.RegisterNode(&RegisterNodeRequest{
		Name:      "node-1",
		IPAddress: "10.0.0.1",
		CPUCores:  8,
		MemoryMB:  16384,
		GPUCount:  1,
	})

	// 提交任务
	manager.SubmitTask(&SubmitTaskRequest{
		Name:  "task-1",
		Image: "nginx:latest",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/edgeorch/metrics", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	if data["total_nodes"].(float64) != 1 {
		t.Errorf("expected 1 node, got %v", data["total_nodes"])
	}
}

func TestSchedulingStrategies(t *testing.T) {
	strategies := []SchedulingStrategy{
		StrategyRoundRobin,
		StrategyLeastLoad,
		StrategyRandom,
		StrategyBinPack,
		StrategySpread,
	}

	for _, strategy := range strategies {
		t.Run(string(strategy), func(t *testing.T) {
			config := &SchedulerConfig{
				Strategy:        strategy,
				MaxTasksPerNode: 50,
			}
			manager := NewManager(config)

			// 注册多个节点
			manager.RegisterNode(&RegisterNodeRequest{
				Name:      "node-1",
				IPAddress: "10.0.0.1",
				CPUCores:  4,
				MemoryMB:  8192,
			})
			manager.RegisterNode(&RegisterNodeRequest{
				Name:      "node-2",
				IPAddress: "10.0.0.2",
				CPUCores:  8,
				MemoryMB:  16384,
			})

			// 提交任务
			task, _ := manager.SubmitTask(&SubmitTaskRequest{
				Name:            "test-task",
				Image:           "nginx:latest",
				CPURequest:      1.0,
				MemoryRequestMB: 512,
			})

			// 调度任务
			scheduled, err := manager.ScheduleTask(task.ID)
			if err != nil {
				t.Fatalf("failed to schedule task: %v", err)
			}

			if scheduled.AssignedNodeID == "" {
				t.Error("expected task to be assigned to a node")
			}
		})
	}
}

func TestAffinityRules(t *testing.T) {
	manager := NewManager(nil)

	// 注册节点
	node, _ := manager.RegisterNode(&RegisterNodeRequest{
		Name:      "node-1",
		IPAddress: "10.0.0.1",
		CPUCores:  8,
		MemoryMB:  16384,
		Zone:      "zone-a",
		Labels:    map[string]string{"gpu": "nvidia"},
	})

	// 提交带亲和性规则的任务
	task, _ := manager.SubmitTask(&SubmitTaskRequest{
		Name:            "affinity-task",
		Image:           "nginx:latest",
		CPURequest:      1.0,
		MemoryRequestMB: 512,
		Affinity: &AffinityRule{
			RequiredZones:  []string{"zone-a"},
			RequiredLabels: map[string]string{"gpu": "nvidia"},
		},
	})

	scheduled, err := manager.ScheduleTask(task.ID)
	if err != nil {
		t.Fatalf("failed to schedule task with affinity: %v", err)
	}

	if scheduled.AssignedNodeID != node.ID {
		t.Errorf("expected task assigned to node %s, got %s", node.ID, scheduled.AssignedNodeID)
	}
}

func TestNodeHealthCheck(t *testing.T) {
	manager := NewManager(nil)

	node, _ := manager.RegisterNode(&RegisterNodeRequest{
		Name:      "node-1",
		IPAddress: "10.0.0.1",
		CPUCores:  8,
		MemoryMB:  16384,
	})

	// 更新心跳
	manager.Heartbeat(node.ID, 75.0, 80.0, 5)

	health, err := manager.CheckNodeHealth(node.ID)
	if err != nil {
		t.Fatalf("failed to check health: %v", err)
	}

	if !health.Healthy {
		t.Error("expected node to be healthy")
	}

	if health.CPUPercent != 75.0 {
		t.Errorf("expected CPU 75.0, got %f", health.CPUPercent)
	}
}

func TestNotFoundErrors(t *testing.T) {
	manager := NewManager(nil)
	r := setupTestRouter(manager)

	// 获取不存在的节点
	req := httptest.NewRequest(http.MethodGet, "/api/v1/edgeorch/nodes/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}

	// 获取不存在的任务
	req = httptest.NewRequest(http.MethodGet, "/api/v1/edgeorch/tasks/nonexistent", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestAutoScheduler(t *testing.T) {
	manager := NewManager(&SchedulerConfig{
		Strategy:            StrategyLeastLoad,
		MaxTasksPerNode:     50,
		HeartbeatTimeout:    30e9,
		HealthCheckInterval: 1e9,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 注册节点
	manager.RegisterNode(&RegisterNodeRequest{
		Name:      "node-1",
		IPAddress: "10.0.0.1",
		CPUCores:  8,
		MemoryMB:  16384,
	})

	// 提交任务（应该在下一个调度周期自动调度）
	task, _ := manager.SubmitTask(&SubmitTaskRequest{
		Name:            "auto-task",
		Image:           "nginx:latest",
		CPURequest:      1.0,
		MemoryRequestMB: 512,
	})

	// 启动调度器
	manager.StartScheduler(ctx)

	// 手动触发一次调度
	manager.schedulePendingTasks()

	task, _ = manager.GetTask(task.ID)
	if task.Status != TaskStatusRunning {
		t.Errorf("expected task to be running after auto-schedule, got %s", task.Status)
	}
}
