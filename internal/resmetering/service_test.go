package resmetering

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := NewService()
	h := NewHandler(svc)
	router := gin.New()
	api := router.Group("/api/v1")
	h.RegisterRoutes(api)
	return router
}

// === 辅助函数 ===

func makeSample(userID, containerID string, ts time.Time) Sample {
	return Sample{
		Timestamp:   ts,
		UserID:      userID,
		ContainerID: containerID,
		ServiceName: "test-service",
		CPU: CPUUsage{
			Cores:       1.5,
			Percent:     50.0,
			UsedSeconds: 1800,
		},
		Memory: MemoryUsage{
			UsedBytes:  2 * 1024 * 1024 * 1024, // 2GB
			LimitBytes: 4 * 1024 * 1024 * 1024, // 4GB
			Percent:    50.0,
		},
		Storage: StorageUsage{
			UsedBytes:  100 * 1024 * 1024 * 1024, // 100GB
			TotalBytes: 500 * 1024 * 1024 * 1024, // 500GB
			Percent:    20.0,
		},
		Network: NetworkUsage{
			RxBytes: 1e9,
			TxBytes: 5e8,
			RxRate:  1e6,
			TxRate:  5e5,
		},
	}
}

// === 服务层测试 ===

func TestRecord(t *testing.T) {
	svc := NewService()
	svc.Record(makeSample("user1", "c1", time.Now()))

	if svc.GetSampleCount() != 1 {
		t.Errorf("期望1条采样, 实际%d", svc.GetSampleCount())
	}
}

func TestRecordBatch(t *testing.T) {
	svc := NewService()
	samples := []Sample{
		makeSample("user1", "c1", time.Now()),
		makeSample("user1", "c2", time.Now()),
		makeSample("user2", "c3", time.Now()),
	}
	svc.RecordSample(samples)

	if svc.GetSampleCount() != 3 {
		t.Errorf("期望3条采样, 实际%d", svc.GetSampleCount())
	}
}

func TestMaxSamplesLimit(t *testing.T) {
	svc := NewService()
	svc.maxSamples = 5

	for i := 0; i < 10; i++ {
		svc.Record(makeSample("user1", "c1", time.Now()))
	}

	if svc.GetSampleCount() != 5 {
		t.Errorf("期望5条采样（截断）, 实际%d", svc.GetSampleCount())
	}
}

func TestClear(t *testing.T) {
	svc := NewService()
	svc.Record(makeSample("user1", "c1", time.Now()))
	svc.Clear()

	if svc.GetSampleCount() != 0 {
		t.Error("清空后应为0条采样")
	}
}

func TestGetSummary(t *testing.T) {
	svc := NewService()
	now := time.Now()

	// 插入3条采样
	svc.Record(makeSample("user1", "c1", now))
	svc.Record(makeSample("user2", "c2", now))
	svc.Record(makeSample("user1", "c3", now))

	from := now.Add(-1 * time.Hour)
	to := now.Add(1 * time.Hour)
	summary := svc.GetSummary(PeriodDaily, from, to)

	if summary.SampleCount != 3 {
		t.Errorf("期望3条采样, 实际%d", summary.SampleCount)
	}

	if summary.UniqueUsers != 2 {
		t.Errorf("期望2个用户, 实际%d", summary.UniqueUsers)
	}

	if summary.UniqueContainers != 3 {
		t.Errorf("期望3个容器, 实际%d", summary.UniqueContainers)
	}

	// CPU核心数累加 = 1.5 × 3 = 4.5
	if summary.TotalCPU.Cores != 4.5 {
		t.Errorf("期望CPU核心4.5, 实际%.2f", summary.TotalCPU.Cores)
	}

	// CPU百分比平均 = 50.0
	if summary.TotalCPU.Percent != 50.0 {
		t.Errorf("期望CPU平均50%%, 实际%.2f", summary.TotalCPU.Percent)
	}
}

func TestGetSummaryEmpty(t *testing.T) {
	svc := NewService()
	now := time.Now()
	from := now.Add(-1 * time.Hour)
	to := now.Add(1 * time.Hour)

	summary := svc.GetSummary(PeriodDaily, from, to)

	if summary.SampleCount != 0 {
		t.Errorf("空服务应返回0采样, 实际%d", summary.SampleCount)
	}
}

func TestGetSummaryTimeFilter(t *testing.T) {
	svc := NewService()
	now := time.Now()

	// 插入不同时间的采样
	svc.Record(makeSample("user1", "c1", now.Add(-2*time.Hour))) // 不在范围内
	svc.Record(makeSample("user2", "c2", now))                   // 在范围内

	from := now.Add(-1 * time.Hour)
	to := now.Add(1 * time.Hour)
	summary := svc.GetSummary(PeriodDaily, from, to)

	if summary.SampleCount != 1 {
		t.Errorf("期望1条采样（时间过滤）, 实际%d", summary.SampleCount)
	}
}

func TestGetByUser(t *testing.T) {
	svc := NewService()
	now := time.Now()

	// user1: 2条采样, user2: 1条采样
	svc.Record(makeSample("user1", "c1", now))
	svc.Record(makeSample("user1", "c2", now))
	svc.Record(makeSample("user2", "c3", now))

	from := now.Add(-1 * time.Hour)
	to := now.Add(1 * time.Hour)
	report := svc.GetByUser(PeriodDaily, from, to)

	if len(report.Users) != 2 {
		t.Fatalf("期望2个用户, 实际%d", len(report.Users))
	}

	// 找到user1
	var user1 *AggregatedUsage
	for i := range report.Users {
		if report.Users[i].Key == "user1" {
			user1 = &report.Users[i]
			break
		}
	}
	if user1 == nil {
		t.Fatal("未找到user1")
	}

	if user1.SampleCount != 2 {
		t.Errorf("user1期望2条采样, 实际%d", user1.SampleCount)
	}

	// CPU核心累加 = 1.5 × 2 = 3.0
	if user1.CPU.Cores != 3.0 {
		t.Errorf("user1 CPU核心期望3.0, 实际%.2f", user1.CPU.Cores)
	}
}

func TestGetByUserEmpty(t *testing.T) {
	svc := NewService()
	now := time.Now()

	from := now.Add(-1 * time.Hour)
	to := now.Add(1 * time.Hour)
	report := svc.GetByUser(PeriodDaily, from, to)

	if len(report.Users) != 0 {
		t.Errorf("空服务应返回0个用户, 实际%d", len(report.Users))
	}
}

func TestGetByContainer(t *testing.T) {
	svc := NewService()
	now := time.Now()

	// c1: 2条采样, c2: 1条采样
	svc.Record(makeSample("user1", "c1", now))
	svc.Record(makeSample("user2", "c1", now)) // 同一容器不同用户
	svc.Record(makeSample("user3", "c2", now))

	from := now.Add(-1 * time.Hour)
	to := now.Add(1 * time.Hour)
	report := svc.GetByContainer(PeriodDaily, from, to)

	if len(report.Containers) != 2 {
		t.Fatalf("期望2个容器, 实际%d", len(report.Containers))
	}

	// 找到c1
	var c1 *AggregatedUsage
	for i := range report.Containers {
		if report.Containers[i].Key == "c1" {
			c1 = &report.Containers[i]
			break
		}
	}
	if c1 == nil {
		t.Fatal("未找到容器c1")
	}

	if c1.SampleCount != 2 {
		t.Errorf("c1期望2条采样, 实际%d", c1.SampleCount)
	}
}

func TestGetByContainerEmpty(t *testing.T) {
	svc := NewService()
	now := time.Now()

	from := now.Add(-1 * time.Hour)
	to := now.Add(1 * time.Hour)
	report := svc.GetByContainer(PeriodDaily, from, to)

	if len(report.Containers) != 0 {
		t.Errorf("空服务应返回0个容器, 实际%d", len(report.Containers))
	}
}

func TestNetworkAggregation(t *testing.T) {
	svc := NewService()
	now := time.Now()

	svc.Record(makeSample("user1", "c1", now))
	svc.Record(makeSample("user1", "c2", now))

	from := now.Add(-1 * time.Hour)
	to := now.Add(1 * time.Hour)
	report := svc.GetByUser(PeriodDaily, from, to)

	if len(report.Users) != 1 {
		t.Fatalf("期望1个用户, 实际%d", len(report.Users))
	}

	// 网络接收 = 1e9 × 2 = 2e9
	if report.Users[0].Network.RxBytes != 2e9 {
		t.Errorf("网络接收期望2e9, 实际%.0f", report.Users[0].Network.RxBytes)
	}

	// 网络发送 = 5e8 × 2 = 1e9
	if report.Users[0].Network.TxBytes != 1e9 {
		t.Errorf("网络发送期望1e9, 实际%.0f", report.Users[0].Network.TxBytes)
	}
}

func TestStorageAggregation(t *testing.T) {
	svc := NewService()
	now := time.Now()

	svc.Record(makeSample("user1", "c1", now))
	svc.Record(makeSample("user2", "c2", now))

	from := now.Add(-1 * time.Hour)
	to := now.Add(1 * time.Hour)
	summary := svc.GetSummary(PeriodDaily, from, to)

	// 存储已用 = 100GB × 2 = 200GB（以字节计）
	expectedUsed := uint64(100*1024*1024*1024) * 2
	if summary.TotalStorage.UsedBytes != expectedUsed {
		t.Errorf("存储已用期望%d, 实际%d", expectedUsed, summary.TotalStorage.UsedBytes)
	}

	// 存储总量取最大值 = 500GB
	if summary.TotalStorage.TotalBytes != 500*1024*1024*1024 {
		t.Errorf("存储总量期望500GB, 实际%d", summary.TotalStorage.TotalBytes)
	}
}

// === HTTP Handler 测试 ===

func TestHandlerSummary(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/resmetering/summary?period=daily", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码200, 实际%d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("响应应包含data字段")
	}

	if data["period"] != "daily" {
		t.Errorf("期望period=daily, 实际%v", data["period"])
	}
}

func TestHandlerByUser(t *testing.T) {
	router := setupTestRouter()

	// 先记录数据
	body, _ := json.Marshal(makeSample("user1", "c1", time.Now()))
	req, _ := http.NewRequest("POST", "/api/v1/resmetering/record", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("记录采样失败: %d", w.Code)
	}

	// 查询
	req, _ = http.NewRequest("GET", "/api/v1/resmetering/by-user?period=daily", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码200, 实际%d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("响应应包含data字段")
	}

	users := data["users"].([]interface{})
	if len(users) != 1 {
		t.Errorf("期望1个用户, 实际%d", len(users))
	}
}

func TestHandlerByContainer(t *testing.T) {
	router := setupTestRouter()

	// 先记录数据
	body, _ := json.Marshal(makeSample("user1", "c1", time.Now()))
	req, _ := http.NewRequest("POST", "/api/v1/resmetering/record", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("记录采样失败: %d", w.Code)
	}

	// 查询
	req, _ = http.NewRequest("GET", "/api/v1/resmetering/by-container?period=daily", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码200, 实际%d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("响应应包含data字段")
	}

	containers := data["containers"].([]interface{})
	if len(containers) != 1 {
		t.Errorf("期望1个容器, 实际%d", len(containers))
	}
}

func TestHandlerRecord(t *testing.T) {
	router := setupTestRouter()

	body, _ := json.Marshal(makeSample("user1", "c1", time.Now()))
	req, _ := http.NewRequest("POST", "/api/v1/resmetering/record", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码200, 实际%d, 响应: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("期望status=ok, 实际%v", resp["status"])
	}
}

func TestHandlerRecordBadJSON(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequest("POST", "/api/v1/resmetering/record", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码400, 实际%d", w.Code)
	}
}

func TestHandlerSummaryWithCustomTimeRange(t *testing.T) {
	router := setupTestRouter()

	now := time.Now()
	from := now.Add(-2 * time.Hour)
	to := now.Add(1 * time.Hour)
	url := "/api/v1/resmetering/summary?period=hourly&from=" + from.Format(time.RFC3339) + "&to=" + to.Format(time.RFC3339)

	req, _ := http.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码200, 实际%d, 响应: %s", w.Code, w.Body.String())
	}
}
