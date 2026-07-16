package smartquota

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func setupTestManager() *QuotaManager {
	mgr := NewQuotaManager()
	return mgr
}

func setupTestRouter(mgr *QuotaManager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/quota/create", func(c *gin.Context) {
		var cfg QuotaConfig
		if err := c.ShouldBindJSON(&cfg); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		created, err := mgr.CreateQuota(cfg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, created)
	})
	r.GET("/api/v1/quota/list", func(c *gin.Context) {
		quotas := mgr.ListQuotas()
		c.JSON(http.StatusOK, gin.H{"quotas": quotas, "total": len(quotas)})
	})
	return r
}

func TestCreateAndListQuota(t *testing.T) {
	mgr := setupTestManager()
	r := setupTestRouter(mgr)

	// 创建配额
	body := `{"name":"test-user","level":"user","ownerId":"u1","limitBytes":1073741824,"policy":"soft"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/quota/create", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created QuotaConfig
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if created.Name != "test-user" {
		t.Errorf("expected name 'test-user', got %s", created.Name)
	}

	// 列出配额
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/quota/list", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w2.Code)
	}

	var listResp map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &listResp)
	if int(listResp["total"].(float64)) != 1 {
		t.Errorf("expected 1 quota in list, got %v", listResp["total"])
	}
}

func TestQuotaAlerts(t *testing.T) {
	mgr := setupTestManager()

	// 创建配额
	q, err := mgr.CreateQuota(QuotaConfig{
		Name:       "alert-test",
		Level:      LevelUser,
		OwnerID:    "u2",
		LimitBytes: 1000,
		Policy:     PolicySoft,
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	alertTriggered := false
	mgr.SetAlertCallback(func(a *Alert) {
		alertTriggered = true
	})

	// 更新使用量到80%（应触发50%和75%告警）
	mgr.UpdateUsage(q.ID, 800)

	alerts := mgr.GetAlerts(nil)
	if len(alerts) < 2 {
		t.Errorf("expected at least 2 alerts, got %d", len(alerts))
	}
	if !alertTriggered {
		t.Error("expected alert callback to be triggered")
	}

	// 更新到100%（应触发90%和100%告警）
	mgr.UpdateUsage(q.ID, 1000)
	alerts = mgr.GetAlerts(nil)
	if len(alerts) < 4 {
		t.Errorf("expected at least 4 alerts, got %d", len(alerts))
	}
}

func TestPredictUsage(t *testing.T) {
	mgr := setupTestManager()

	q, _ := mgr.CreateQuota(QuotaConfig{
		Name:       "predict-test",
		Level:      LevelUser,
		OwnerID:    "u3",
		LimitBytes: 10 * 1024 * 1024 * 1024, // 10GB
		Policy:     PolicySoft,
	})

	// 模拟历史数据：每天增长100MB
	baseTime := time.Now().Add(-7 * 24 * time.Hour)
	for i := 0; i <= 7; i++ {
		q.History = append(q.History, UsageRecord{
			Timestamp: baseTime.Add(time.Duration(i) * 24 * time.Hour),
			UsedBytes: int64(i) * 100 * 1024 * 1024,
		})
	}
	q.UsedBytes = 700 * 1024 * 1024

	pred, err := mgr.PredictUsage(q.ID)
	if err != nil {
		t.Fatalf("predict failed: %v", err)
	}

	if pred.Trend != "increasing" {
		t.Errorf("expected trend 'increasing', got %s", pred.Trend)
	}
	if pred.DailyGrowthRate <= 0 {
		t.Errorf("expected positive daily growth rate, got %f", pred.DailyGrowthRate)
	}
	if pred.DaysRemaining <= 0 {
		t.Errorf("expected positive days remaining, got %f", pred.DaysRemaining)
	}
	if pred.ExhaustDate == nil {
		t.Error("expected non-nil exhaust date")
	}
}

func TestQuotaInheritance(t *testing.T) {
	mgr := setupTestManager()

	// 创建组配额 100GB
	groupQ, _ := mgr.CreateQuota(QuotaConfig{
		Name:       "engineering-group",
		Level:      LevelGroup,
		OwnerID:    "g1",
		LimitBytes: 100 * 1024 * 1024 * 1024,
		Policy:     PolicySoft,
	})

	// 分配子配额给用户
	childQ, err := mgr.InheritQuota(groupQ.ID, "alice-quota", "u-alice", 30*1024*1024*1024)
	if err != nil {
		t.Fatalf("inherit failed: %v", err)
	}

	if childQ.ParentID != groupQ.ID {
		t.Errorf("expected parentID %s, got %s", groupQ.ID, childQ.ParentID)
	}
	if childQ.Level != LevelUser {
		t.Errorf("expected level 'user', got %s", childQ.Level)
	}
	if childQ.LimitBytes != 30*1024*1024*1024 {
		t.Errorf("expected limitBytes 30GB, got %d", childQ.LimitBytes)
	}

	// 再分配一个
	_, err = mgr.InheritQuota(groupQ.ID, "bob-quota", "u-bob", 40*1024*1024*1024)
	if err != nil {
		t.Fatalf("second inherit failed: %v", err)
	}

	// 超额分配应该失败
	_, err = mgr.InheritQuota(groupQ.ID, "charlie-quota", "u-charlie", 40*1024*1024*1024)
	if err == nil {
		t.Error("expected error for over-allocation")
	}
}

func TestApplyTemplate(t *testing.T) {
	mgr := setupTestManager()

	q, err := mgr.ApplyTemplate("family_user", "u-family", "家庭存储")
	if err != nil {
		t.Fatalf("apply template failed: %v", err)
	}

	if q.LimitBytes != 1024*1024*1024*1024 {
		t.Errorf("expected 1TB limit, got %d", q.LimitBytes)
	}
	if q.Policy != PolicySoft {
		t.Errorf("expected soft policy, got %s", q.Policy)
	}

	// 不存在的模板
	_, err = mgr.ApplyTemplate("nonexistent", "u1", "test")
	if err == nil {
		t.Error("expected error for nonexistent template")
	}
}

func TestCleanupSuggestions(t *testing.T) {
	mgr := setupTestManager()

	q, _ := mgr.CreateQuota(QuotaConfig{
		Name:       "cleanup-test",
		Level:      LevelUser,
		OwnerID:    "u4",
		LimitBytes: 100 * 1024 * 1024 * 1024,
		Policy:     PolicySoft,
	})

	// 使用率低时无建议
	suggestions, _ := mgr.GetCleanupSuggestions(q.ID)
	if len(suggestions) != 0 {
		t.Errorf("expected 0 suggestions at low usage, got %d", len(suggestions))
	}

	// 使用率80%
	q.UsedBytes = 80 * 1024 * 1024 * 1024
	suggestions, _ = mgr.GetCleanupSuggestions(q.ID)
	if len(suggestions) < 1 {
		t.Errorf("expected at least 1 suggestion at 80%% usage, got %d", len(suggestions))
	}

	// 使用率95%
	q.UsedBytes = 95 * 1024 * 1024 * 1024
	suggestions, _ = mgr.GetCleanupSuggestions(q.ID)
	if len(suggestions) < 3 {
		t.Errorf("expected at least 3 suggestions at 95%% usage, got %d", len(suggestions))
	}
}
