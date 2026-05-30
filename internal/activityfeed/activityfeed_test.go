package activityfeed

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewFeed(t *testing.T) {
	// 测试默认配置
	feed := NewFeed(nil)
	if feed == nil {
		t.Fatal("NewFeed returned nil")
	}
	if feed.config.MaxActivities != 10000 {
		t.Errorf("Expected MaxActivities 10000, got %d", feed.config.MaxActivities)
	}
	if feed.config.RetentionDays != 30 {
		t.Errorf("Expected RetentionDays 30, got %d", feed.config.RetentionDays)
	}

	// 测试自定义配置
	config := &FeedConfig{
		MaxActivities: 5000,
		RetentionDays: 7,
	}
	feed = NewFeed(config)
	if feed.config.MaxActivities != 5000 {
		t.Errorf("Expected MaxActivities 5000, got %d", feed.config.MaxActivities)
	}
}

func TestRecordActivity(t *testing.T) {
	feed := NewFeed(nil)

	// 测试正常记录
	activity := Activity{
		Service:     ServiceFileOps,
		Action:      "file_upload",
		Description: "User uploaded a file",
		Severity:    SeverityInfo,
		Actor: ActivityActor{
			ID:   "user123",
			Name: "Test User",
			Type: "user",
		},
		Resource: "/documents/test.pdf",
	}

	recorded, err := feed.RecordActivity(activity)
	if err != nil {
		t.Fatalf("RecordActivity failed: %v", err)
	}

	if recorded.ID == "" {
		t.Error("Expected non-empty ID")
	}
	if recorded.CreatedAt.IsZero() {
		t.Error("Expected non-zero CreatedAt")
	}
	if recorded.Severity != SeverityInfo {
		t.Errorf("Expected severity info, got %s", recorded.Severity)
	}

	// 测试缺少必填字段
	invalidActivity := Activity{
		Service: ServiceFileOps,
	}
	_, err = feed.RecordActivity(invalidActivity)
	if err == nil {
		t.Error("Expected error for missing actor ID")
	}
}

func TestQueryActivities(t *testing.T) {
	feed := NewFeed(nil)

	// 创建测试数据
	now := time.Now()
	activities := []Activity{
		{
			Service:  ServiceFileOps,
			Action:   "file_upload",
			Actor:    ActivityActor{ID: "user1", Name: "User 1"},
			Severity: SeverityInfo,
			Timestamp: now.Add(-2 * time.Hour),
		},
		{
			Service:  ServiceUserAuth,
			Action:   "login",
			Actor:    ActivityActor{ID: "user2", Name: "User 2"},
			Severity: SeverityInfo,
			Timestamp: now.Add(-1 * time.Hour),
		},
		{
			Service:  ServiceSystem,
			Action:   "backup_complete",
			Actor:    ActivityActor{ID: "system", Name: "System"},
			Severity: SeverityWarning,
			Timestamp: now,
		},
	}

	for _, act := range activities {
		_, err := feed.RecordActivity(act)
		if err != nil {
			t.Fatalf("Failed to record activity: %v", err)
		}
	}

	// 测试无过滤查询
	result, err := feed.QueryActivities(ActivityFilter{})
	if err != nil {
		t.Fatalf("QueryActivities failed: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("Expected 3 activities, got %d", len(result))
	}

	// 测试服务过滤
	result, err = feed.QueryActivities(ActivityFilter{
		Services: []ServiceType{ServiceFileOps},
	})
	if err != nil {
		t.Fatalf("QueryActivities with filter failed: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("Expected 1 activity, got %d", len(result))
	}

	// 测试执行者过滤
	result, err = feed.QueryActivities(ActivityFilter{
		ActorIDs: []string{"user1"},
	})
	if err != nil {
		t.Fatalf("QueryActivities with actor filter failed: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("Expected 1 activity, got %d", len(result))
	}

	// 测试严重级别过滤
	result, err = feed.QueryActivities(ActivityFilter{
		Severities: []Severity{SeverityWarning},
	})
	if err != nil {
		t.Fatalf("QueryActivities with severity filter failed: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("Expected 1 activity, got %d", len(result))
	}

	// 测试分页
	result, err = feed.QueryActivities(ActivityFilter{
		Limit:  2,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("QueryActivities with pagination failed: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("Expected 2 activities, got %d", len(result))
	}

	// 测试关键词搜索
	result, err = feed.QueryActivities(ActivityFilter{
		Keyword: "upload",
	})
	if err != nil {
		t.Fatalf("QueryActivities with keyword failed: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("Expected 1 activity for keyword 'upload', got %d", len(result))
	}
}

func TestGetSummary(t *testing.T) {
	feed := NewFeed(nil)

	// 创建测试数据
	now := time.Now()
	for i := 0; i < 10; i++ {
		_, err := feed.RecordActivity(Activity{
			Service:  ServiceFileOps,
			Action:   "file_upload",
			Actor:    ActivityActor{ID: fmt.Sprintf("user%d", i%3), Name: fmt.Sprintf("User %d", i%3)},
			Severity: SeverityInfo,
			Timestamp: now.Add(-time.Duration(i) * time.Hour),
		})
		if err != nil {
			t.Fatalf("Failed to record activity: %v", err)
		}
	}

	// 测试每日摘要
	startTime := now.Add(-24 * time.Hour)
	summary, err := feed.GetSummary("daily", startTime, now)
	if err != nil {
		t.Fatalf("GetSummary failed: %v", err)
	}

	if summary.Period != "daily" {
		t.Errorf("Expected period 'daily', got %s", summary.Period)
	}
	if summary.TotalActivities != 10 {
		t.Errorf("Expected 10 activities, got %d", summary.TotalActivities)
	}
	if summary.ByService[ServiceFileOps] != 10 {
		t.Errorf("Expected 10 file_ops activities, got %d", summary.ByService[ServiceFileOps])
	}
	if len(summary.TopActors) != 3 {
		t.Errorf("Expected 3 top actors, got %d", len(summary.TopActors))
	}
}

func TestSubscribe(t *testing.T) {
	feed := NewFeed(nil)

	// 测试创建订阅
	subID, ch, err := feed.Subscribe("http://example.com/webhook", ActivityFilter{
		Services: []ServiceType{ServiceFileOps},
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	if subID == "" {
		t.Error("Expected non-empty subscription ID")
	}
	if ch == nil {
		t.Error("Expected non-nil channel")
	}

	// 测试接收事件
	go func() {
		time.Sleep(100 * time.Millisecond)
		feed.RecordActivity(Activity{
			Service:  ServiceFileOps,
			Action:   "file_upload",
			Actor:    ActivityActor{ID: "user1", Name: "User 1"},
			Severity: SeverityInfo,
		})
	}()

	select {
	case event := <-ch:
		if event.Activity.Service != ServiceFileOps {
			t.Errorf("Expected service file_ops, got %s", event.Activity.Service)
		}
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for event")
	}

	// 测试取消订阅
	err = feed.Unsubscribe(subID)
	if err != nil {
		t.Fatalf("Unsubscribe failed: %v", err)
	}

	// 验证通道已关闭
	_, ok := <-ch
	if ok {
		t.Error("Expected channel to be closed")
	}
}

func TestExportFeed(t *testing.T) {
	feed := NewFeed(nil)

	// 创建测试数据
	for i := 0; i < 5; i++ {
		_, err := feed.RecordActivity(Activity{
			Service:     ServiceFileOps,
			Action:      "file_upload",
			Description: fmt.Sprintf("File %d uploaded", i),
			Actor:       ActivityActor{ID: "user1", Name: "User 1"},
			Severity:    SeverityInfo,
		})
		if err != nil {
			t.Fatalf("Failed to record activity: %v", err)
		}
	}

	// 测试JSON导出
	jsonExport, err := feed.ExportFeed(ActivityFilter{}, FormatJSON)
	if err != nil {
		t.Fatalf("ExportFeed JSON failed: %v", err)
	}

	if jsonExport.Format != FormatJSON {
		t.Errorf("Expected format json, got %s", jsonExport.Format)
	}
	if jsonExport.Count != 5 {
		t.Errorf("Expected 5 activities, got %d", jsonExport.Count)
	}
	if len(jsonExport.Content) == 0 {
		t.Error("Expected non-empty JSON content")
	}

	// 验证JSON格式正确性
	var activities []Activity
	if err := json.Unmarshal(jsonExport.Content, &activities); err != nil {
		t.Errorf("Invalid JSON export: %v", err)
	}

	// 测试CSV导出
	csvExport, err := feed.ExportFeed(ActivityFilter{}, FormatCSV)
	if err != nil {
		t.Fatalf("ExportFeed CSV failed: %v", err)
	}

	if csvExport.Format != FormatCSV {
		t.Errorf("Expected format csv, got %s", csvExport.Format)
	}
	if len(csvExport.Content) == 0 {
		t.Error("Expected non-empty CSV content")
	}

	// 验证CSV包含表头
	if !bytes.Contains(csvExport.Content, []byte("ID,Timestamp,Service")) {
		t.Error("CSV missing expected header")
	}
}

func TestActivityCorrelation(t *testing.T) {
	feed := NewFeed(nil)

	// 创建第一个活动
	act1, err := feed.RecordActivity(Activity{
		Service:  ServiceFileOps,
		Action:   "file_upload",
		Actor:    ActivityActor{ID: "user1", Name: "User 1"},
		Resource: "/documents/test.pdf",
	})
	if err != nil {
		t.Fatalf("Failed to record first activity: %v", err)
	}

	// 创建相关活动（同一用户、同一服务）
	act2, err := feed.RecordActivity(Activity{
		Service:  ServiceFileOps,
		Action:   "file_download",
		Actor:    ActivityActor{ID: "user1", Name: "User 1"},
		Resource: "/documents/test.pdf",
	})
	if err != nil {
		t.Fatalf("Failed to record second activity: %v", err)
	}

	// 验证关联
	if len(act2.RelatedIDs) == 0 {
		t.Error("Expected related IDs for correlated activity")
	}
	found := false
	for _, id := range act2.RelatedIDs {
		if id == act1.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected act1 to be in act2's related IDs")
	}
}

func TestHTTPHandlers(t *testing.T) {
	feed := NewFeed(nil)
	handler := NewHandler(feed)

	// 创建一些测试数据
	for i := 0; i < 3; i++ {
		feed.RecordActivity(Activity{
			Service:     ServiceFileOps,
			Action:      "file_upload",
			Description: fmt.Sprintf("File %d uploaded", i),
			Actor:       ActivityActor{ID: "user1", Name: "User 1"},
			Severity:    SeverityInfo,
		})
	}

	t.Run("GET /api/activities", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/activities", nil)
		w := httptest.NewRecorder()

		handler.handleActivities(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if response["count"].(float64) != 3 {
			t.Errorf("Expected 3 activities, got %v", response["count"])
		}
	})

	t.Run("POST /api/activities", func(t *testing.T) {
		activity := Activity{
			Service:     ServiceUserAuth,
			Action:      "login",
			Description: "User logged in",
			Actor:       ActivityActor{ID: "user2", Name: "User 2"},
		}

		body, _ := json.Marshal(activity)
		req := httptest.NewRequest(http.MethodPost, "/api/activities", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.handleActivities(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", w.Code)
		}

		var recorded Activity
		if err := json.Unmarshal(w.Body.Bytes(), &recorded); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if recorded.ID == "" {
			t.Error("Expected non-empty ID in response")
		}
	})

	t.Run("GET /api/activities/summary", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/activities/summary?period=daily", nil)
		w := httptest.NewRecorder()

		handler.handleSummary(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var summary ActivitySummary
		if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if summary.Period != "daily" {
			t.Errorf("Expected period 'daily', got %s", summary.Period)
		}
	})

	t.Run("GET /api/activities/export", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/activities/export?format=json", nil)
		w := httptest.NewRecorder()

		handler.handleExport(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		if w.Header().Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", w.Header().Get("Content-Type"))
		}
	})

	t.Run("POST /api/activities/subscribe", func(t *testing.T) {
		subReq := map[string]interface{}{
			"url": "http://example.com/webhook",
			"filter": map[string]interface{}{
				"services": []string{"file_ops"},
			},
		}

		body, _ := json.Marshal(subReq)
		req := httptest.NewRequest(http.MethodPost, "/api/activities/subscribe", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.handleSubscribe(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if response["subscription_id"] == nil {
			t.Error("Expected subscription_id in response")
		}
	})
}

func TestFilterMatching(t *testing.T) {
	feed := NewFeed(nil)

	now := time.Now()
	activity := Activity{
		Service:     ServiceFileOps,
		Action:      "file_upload",
		Description: "Test upload",
		Actor:       ActivityActor{ID: "user1", Name: "User 1"},
		Resource:    "/documents/test.pdf",
		Severity:    SeverityInfo,
		Timestamp:   now,
	}

	tests := []struct {
		name     string
		filter   ActivityFilter
		expected bool
	}{
		{
			name:     "Empty filter matches all",
			filter:   ActivityFilter{},
			expected: true,
		},
		{
			name:     "Matching service",
			filter:   ActivityFilter{Services: []ServiceType{ServiceFileOps}},
			expected: true,
		},
		{
			name:     "Non-matching service",
			filter:   ActivityFilter{Services: []ServiceType{ServiceBackup}},
			expected: false,
		},
		{
			name:     "Matching actor",
			filter:   ActivityFilter{ActorIDs: []string{"user1"}},
			expected: true,
		},
		{
			name:     "Matching severity",
			filter:   ActivityFilter{Severities: []Severity{SeverityInfo}},
			expected: true,
		},
		{
			name:     "Matching resource",
			filter:   ActivityFilter{Resource: "documents"},
			expected: true,
		},
		{
			name:     "Matching keyword",
			filter:   ActivityFilter{Keyword: "upload"},
			expected: true,
		},
		{
			name:     "Non-matching keyword",
			filter:   ActivityFilter{Keyword: "download"},
			expected: false,
		},
		{
			name: "Matching time range",
			filter: ActivityFilter{
				StartTime: func() *time.Time { t := now.Add(-1 * time.Hour); return &t }(),
				EndTime:   func() *time.Time { t := now.Add(1 * time.Hour); return &t }(),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := feed.matchesFilter(activity, tt.filter)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestConcurrentAccess(t *testing.T) {
	feed := NewFeed(nil)

	// 并发写入
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				feed.RecordActivity(Activity{
					Service:  ServiceFileOps,
					Action:   "test",
					Actor:    ActivityActor{ID: fmt.Sprintf("user%d", id)},
					Severity: SeverityInfo,
				})
			}
			done <- true
		}(i)
	}

	// 等待所有写入完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证数据完整性
	result, err := feed.QueryActivities(ActivityFilter{Limit: 10000})
	if err != nil {
		t.Fatalf("QueryActivities failed: %v", err)
	}

	if len(result) != 1000 {
		t.Errorf("Expected 1000 activities, got %d", len(result))
	}
}
