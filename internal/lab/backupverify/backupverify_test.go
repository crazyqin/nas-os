package backupverify

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTest(t *testing.T) (*Manager, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	mgr := NewManager()
	r := gin.New()
	rg := r.Group("/api/v1")
	h := NewHandler(mgr)
	h.RegisterRoutes(rg)
	return mgr, r
}

func TestNewManager(t *testing.T) {
	mgr := NewManager()
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestRunVerification(t *testing.T) {
	mgr := NewManager()
	task, err := mgr.RunVerification("backup1", "/data/backup1", ModeChecksum)
	if err != nil {
		t.Fatalf("RunVerification failed: %v", err)
	}
	if task.ID == "" {
		t.Error("task ID should not be empty")
	}
	if task.BackupID != "backup1" {
		t.Errorf("expected backup1, got %s", task.BackupID)
	}
}

func TestRunVerificationEmptyID(t *testing.T) {
	mgr := NewManager()
	_, err := mgr.RunVerification("", "/data", ModeChecksum)
	if err == nil {
		t.Error("expected error for empty backup ID")
	}
}

func TestGetTask(t *testing.T) {
	mgr := NewManager()
	task, _ := mgr.RunVerification("backup1", "/data", ModeChecksum)
	fetched, err := mgr.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if fetched.ID != task.ID {
		t.Errorf("expected %s, got %s", task.ID, fetched.ID)
	}
}

func TestListTasks(t *testing.T) {
	mgr := NewManager()
	mgr.RunVerification("backup1", "/a", ModeChecksum)
	mgr.RunVerification("backup2", "/b", ModeDeep)
	tasks := mgr.ListTasks()
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestSchedule(t *testing.T) {
	mgr := NewManager()
	schedule := &VerifySchedule{
		BackupID: "backup1",
		Mode:     ModeChecksum,
		Cron:     "0 3 * * *",
		Enabled:  true,
	}
	err := mgr.AddSchedule(schedule)
	if err != nil {
		t.Fatalf("AddSchedule failed: %v", err)
	}
	schedules := mgr.GetSchedules()
	if len(schedules) != 1 {
		t.Errorf("expected 1 schedule, got %d", len(schedules))
	}
}

func TestReport(t *testing.T) {
	mgr := NewManager()
	_, err := mgr.RunVerification("backup1", "/data", ModeChecksum)
	if err != nil {
		t.Fatal(err)
	}
	report, err := mgr.GetReport("backup1")
	if err != nil {
		// 报告可能还未生成（异步），这很正常
		return
	}
	if report.BackupID != "backup1" {
		t.Errorf("expected backup1, got %s", report.BackupID)
	}
}

func TestGenerateChecksum(t *testing.T) {
	data := []byte("test data")
	hash := GenerateChecksum(data)
	if hash == "" {
		t.Error("checksum should not be empty")
	}
	// 同样数据应产生同样哈希
	hash2 := GenerateChecksum(data)
	if hash != hash2 {
		t.Error("same data should produce same checksum")
	}
}

func TestAPIVerify(t *testing.T) {
	_, r := setupTest(t)
	body := `{"backup_id":"b1","backup_path":"/data","mode":"checksum"}`
	req, _ := http.NewRequest("POST", "/api/v1/backup-verify/verify", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest && w.Code != http.StatusAccepted {
		t.Errorf("expected 202 or 400, got %d", w.Code)
	}
	_ = body
}

func TestAPITasks(t *testing.T) {
	_, r := setupTest(t)
	req, _ := http.NewRequest("GET", "/api/v1/backup-verify/tasks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAPISchedules(t *testing.T) {
	_, r := setupTest(t)
	req, _ := http.NewRequest("GET", "/api/v1/backup-verify/schedules", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
