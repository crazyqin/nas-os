package taskcenter

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestRouter() (*gin.Engine, *TaskCenter) {
	gin.SetMode(gin.TestMode)
	tc := NewTaskCenter()
	handler := NewHandler(tc)
	r := gin.New()
	api := r.Group("/api/v1")
	handler.RegisterRoutes(api)
	return r, tc
}

func TestCreateTaskAPI(t *testing.T) {
	r, _ := setupTestRouter()

	task := Task{
		ID:          "task-1",
		Name:        "备份任务",
		Description: "每日备份",
		Type:        TaskTypeBackup,
		Priority:    TaskPriorityNormal,
		MaxRetries:  3,
		Enabled:     true,
	}
	body, _ := json.Marshal(task)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/task-center/tasks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp["success"].(bool) {
		t.Error("Expected success=true")
	}
}

func TestGetTaskAPI(t *testing.T) {
	r, tc := setupTestRouter()

	tc.CreateTask(Task{ID: "task-1", Name: "测试任务", Type: TaskTypeBackup})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/task-center/tasks/task-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetTaskNotFound(t *testing.T) {
	r, _ := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/task-center/tasks/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestListTasksAPI(t *testing.T) {
	r, tc := setupTestRouter()

	tc.CreateTask(Task{ID: "task-1", Name: "任务1", Type: TaskTypeBackup})
	tc.CreateTask(Task{ID: "task-2", Name: "任务2", Type: TaskTypeSync})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/task-center/tasks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].([]interface{})
	if len(data) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(data))
	}
}

func TestListTasksWithFilter(t *testing.T) {
	r, tc := setupTestRouter()

	tc.CreateTask(Task{ID: "task-1", Name: "备份", Type: TaskTypeBackup})
	tc.CreateTask(Task{ID: "task-2", Name: "同步", Type: TaskTypeSync})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/task-center/tasks?type=backup", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].([]interface{})
	if len(data) != 1 {
		t.Errorf("Expected 1 task with type=backup, got %d", len(data))
	}
}

func TestUpdateTaskAPI(t *testing.T) {
	r, tc := setupTestRouter()

	tc.CreateTask(Task{ID: "task-1", Name: "原始名称", Type: TaskTypeBackup})

	task := Task{Name: "更新后名称", Type: TaskTypeSync}
	body, _ := json.Marshal(task)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/task-center/tasks/task-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteTaskAPI(t *testing.T) {
	r, tc := setupTestRouter()

	tc.CreateTask(Task{ID: "task-1", Name: "待删除", Type: TaskTypeBackup})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/task-center/tasks/task-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	_, err := tc.GetTask("task-1")
	if err == nil {
		t.Error("Expected task to be deleted")
	}
}

func TestStartTaskAPI(t *testing.T) {
	r, tc := setupTestRouter()

	tc.CreateTask(Task{ID: "task-1", Name: "待启动", Type: TaskTypeBackup})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/task-center/tasks/task-1/start", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	task, _ := tc.GetTask("task-1")
	if task.Status != TaskStatusRunning {
		t.Errorf("Expected status 'running', got '%s'", task.Status)
	}
}

func TestStartTaskWithDependencies(t *testing.T) {
	r, tc := setupTestRouter()

	tc.CreateTask(Task{ID: "dep-1", Name: "依赖", Type: TaskTypeBackup, Status: TaskStatusPending})
	tc.CreateTask(Task{ID: "task-1", Name: "主任务", Type: TaskTypeBackup, Dependencies: []string{"dep-1"}})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/task-center/tasks/task-1/start", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for unmet dependency, got %d", w.Code)
	}
}

func TestPauseTaskAPI(t *testing.T) {
	r, tc := setupTestRouter()

	tc.CreateTask(Task{ID: "task-1", Name: "运行中", Type: TaskTypeBackup})
	tc.ExecuteTask("task-1")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/task-center/tasks/task-1/pause", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	task, _ := tc.GetTask("task-1")
	if task.Status != TaskStatusWaiting {
		t.Errorf("Expected status 'waiting', got '%s'", task.Status)
	}
}

func TestPauseTaskNotRunning(t *testing.T) {
	r, tc := setupTestRouter()

	tc.CreateTask(Task{ID: "task-1", Name: "待执行", Type: TaskTypeBackup})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/task-center/tasks/task-1/pause", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestCancelTaskAPI(t *testing.T) {
	r, tc := setupTestRouter()

	tc.CreateTask(Task{ID: "task-1", Name: "待取消", Type: TaskTypeBackup})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/task-center/tasks/task-1/cancel", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	task, _ := tc.GetTask("task-1")
	if task.Status != TaskStatusCancelled {
		t.Errorf("Expected status 'cancelled', got '%s'", task.Status)
	}
}

func TestGetTaskLogsAPI(t *testing.T) {
	r, tc := setupTestRouter()

	tc.AddLog(TaskLog{ID: "log-1", TaskID: "task-1", Level: "info", Message: "任务开始"})
	tc.AddLog(TaskLog{ID: "log-2", TaskID: "task-1", Level: "info", Message: "进度50%"})
	tc.AddLog(TaskLog{ID: "log-3", TaskID: "task-2", Level: "info", Message: "其他任务"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/task-center/tasks/task-1/logs?limit=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].([]interface{})
	if len(data) != 2 {
		t.Errorf("Expected 2 logs for task-1, got %d", len(data))
	}
}

func TestGetStatsAPI(t *testing.T) {
	r, tc := setupTestRouter()

	tc.CreateTask(Task{ID: "t1", Name: "1", Type: TaskTypeBackup})
	tc.CreateTask(Task{ID: "t2", Name: "2", Type: TaskTypeSync})
	tc.ExecuteTask("t1")
	tc.CompleteTask("t1", TaskResult{Success: true})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/task-center/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	if int(data["total"].(float64)) != 2 {
		t.Errorf("Expected total=2, got %v", data["total"])
	}
	if int(data["completed"].(float64)) != 1 {
		t.Errorf("Expected completed=1, got %v", data["completed"])
	}
}
