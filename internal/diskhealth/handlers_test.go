// Package diskhealthai2 - 测试文件
package diskhealth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// 创建测试用的 SMART 数据
func createTestData(device string, reallocated uint64, pending uint64, temp int, poh uint64, isSSD bool) SMARTData {
	attrs := []SMARTAttribute{
		{ID: SMARTIDReallocatedSectorCt, Name: "Reallocated_Sector_Ct", RawValue: reallocated, Value: 100, Threshold: 10, IsCritical: true},
		{ID: SMARTIDCurrentPendingSector, Name: "Current_Pending_Sector", RawValue: pending, Value: 100, Threshold: 0, IsCritical: true},
		{ID: SMARTIDTemperatureCelsius, Name: "Temperature_Celsius", RawValue: uint64(temp), Value: 100, Threshold: 0},
		{ID: SMARTIDPowerOnHours, Name: "Power_On_Hours", RawValue: poh, Value: 100, Threshold: 0},
		{ID: SMARTIDSeekErrorRate, Name: "Seek_Error_Rate", RawValue: 0, Value: 100, Threshold: 0},
		{ID: SMARTIDSpinRetryCount, Name: "Spin_Retry_Count", RawValue: 0, Value: 100, Threshold: 0},
		{ID: SMARTIDUDMAErrorCount, Name: "UDMA_Error_Count", RawValue: 0, Value: 100, Threshold: 0},
		{ID: SMARTIDOfflineUncorrectable, Name: "Offline_Uncorrectable", RawValue: 0, Value: 100, Threshold: 0},
		{ID: SMARTIDPowerCycleCount, Name: "Power_Cycle_Count", RawValue: 100, Value: 100, Threshold: 0},
		{ID: SMARTIDLoadUnloadCycleCount, Name: "Load_Unload_Cycle_Count", RawValue: 1000, Value: 100, Threshold: 0},
		{ID: SMARTIDUnsafeShutdownCount, Name: "Unsafe_Shutdown_Count", RawValue: 5, Value: 100, Threshold: 0},
		{ID: SMARTIDMultiZoneErrorRate, Name: "Multi_Zone_Error_Rate", RawValue: 0, Value: 100, Threshold: 0},
		{ID: SMARTIDGSENSEErrorRate, Name: "G_Sense_Error_Rate", RawValue: 0, Value: 100, Threshold: 0},
		{ID: SMARTIDHardwareECCRecovered, Name: "Hardware_ECC_Recovered", RawValue: 0, Value: 100, Threshold: 0},
	}

	if isSSD {
		attrs = append(attrs,
			SMARTAttribute{ID: SMARTIDWearLevelingCount, Name: "Wear_Leveling_Count", RawValue: 80, Value: 80, Threshold: 10},
			SMARTAttribute{ID: SMARTIDNANDWrites, Name: "NAND_Writes", RawValue: 100, Value: 100, Threshold: 0},
			SMARTAttribute{ID: SMARTIDSSDLifeLeft, Name: "SSD_Life_Left", RawValue: 80, Value: 80, Threshold: 10},
		)
	}

	return SMARTData{
		Device:          device,
		Model:           "TestDisk Model",
		Serial:          "TEST123",
		Firmware:        "1.0",
		Interface:       "SATA",
		CapacityBytes:   1e12,
		Temperature:     temp,
		PowerOnHours:    poh,
		PowerCycleCount: 100,
		Attributes:      attrs,
		CollectedAt:     time.Now(),
		IsSSD:           isSSD,
	}
}

// 创建测试服务（使用简单设备名）
func createTestService() *DiskHealthService {
	svc := NewDiskHealthService(100)

	// 添加正常磁盘
	for i := 0; i < 10; i++ {
		svc.Analyzer.RecordData(createTestData("sda", 0, 0, 35, uint64(1000+i*10), false))
	}

	// 添加有问题的磁盘
	for i := 0; i < 5; i++ {
		svc.Analyzer.RecordData(createTestData("sdb", 10, 5, 60, uint64(35000+i*100), false))
	}

	// 添加 SSD
	for i := 0; i < 5; i++ {
		svc.Analyzer.RecordData(createTestData("nvme0n1", 0, 0, 40, uint64(5000+i*50), true))
	}

	// 创建磁盘组
	svc.GroupMgr.CreateGroup("raid1", "RAID-1 组", "RAID", []string{"sda", "sdb"})
	svc.GroupMgr.CreateGroup("pool1", "存储池1", "存储池", []string{"nvme0n1"})

	return svc
}

// ============================================================
// 测试用例
// ============================================================

func TestSmartAnalyzer_RecordAndGetLatest(t *testing.T) {
	analyzer := NewSMARTAnalyzer(100)
	testData := createTestData("sda", 0, 0, 35, 1000, false)

	analyzer.RecordData(testData)

	latest, err := analyzer.GetLatestData("sda")
	if err != nil {
		t.Fatalf("获取最新数据失败: %v", err)
	}

	if latest.Device != "sda" {
		t.Errorf("设备名不匹配: got %s, want sda", latest.Device)
	}

	if latest.Temperature != 35 {
		t.Errorf("温度不匹配: got %d, want 35", latest.Temperature)
	}
}

func TestSmartAnalyzer_GetDevices(t *testing.T) {
	analyzer := NewSMARTAnalyzer(100)
	analyzer.RecordData(createTestData("sda", 0, 0, 35, 1000, false))
	analyzer.RecordData(createTestData("sdb", 0, 0, 40, 2000, false))

	devices := analyzer.GetDevices()
	if len(devices) != 2 {
		t.Errorf("设备数量不匹配: got %d, want 2", len(devices))
	}
}

func TestSmartAnalyzer_Analyze(t *testing.T) {
	analyzer := NewSMARTAnalyzer(100)

	for i := 0; i < 10; i++ {
		analyzer.RecordData(createTestData("sda", 0, 0, 35, uint64(1000+i*100), false))
	}

	result, err := analyzer.Analyze("sda")
	if err != nil {
		t.Fatalf("分析失败: %v", err)
	}

	if result.Device != "sda" {
		t.Errorf("设备名不匹配: got %s", result.Device)
	}

	if len(result.Attributes) == 0 {
		t.Error("属性分析结果为空")
	}
}

func TestHealthScore_Calculate(t *testing.T) {
	svc := createTestService()

	score, err := svc.ScoreSys.Calculate("sda")
	if err != nil {
		t.Fatalf("计算评分失败: %v", err)
	}

	if score.Score < 60 {
		t.Errorf("正常磁盘评分过低: %.1f", score.Score)
	}

	if score.Grade != GradeA && score.Grade != GradeB {
		t.Errorf("正常磁盘等级不匹配: got %s", score.Grade)
	}
}

func TestHealthScore_DegradedDisk(t *testing.T) {
	svc := createTestService()

	score, err := svc.ScoreSys.Calculate("sdb")
	if err != nil {
		t.Fatalf("计算评分失败: %v", err)
	}

	if score.Score > 80 {
		t.Errorf("问题磁盘评分过高: %.1f", score.Score)
	}

	if score.Grade == GradeA {
		t.Error("问题磁盘不应为 A 级")
	}

	if len(score.CorrelationPenalty) == 0 {
		t.Error("问题磁盘应有关联惩罚")
	}
}

func TestHealthScore_SSDDisk(t *testing.T) {
	svc := createTestService()

	score, err := svc.ScoreSys.Calculate("nvme0n1")
	if err != nil {
		t.Fatalf("计算评分失败: %v", err)
	}

	if score.Score < 50 {
		t.Errorf("SSD 评分过低: %.1f", score.Score)
	}
}

func TestFailurePredictor_Predict(t *testing.T) {
	svc := createTestService()

	prediction, err := svc.Predictor.Predict("sda")
	if err != nil {
		t.Fatalf("预测失败: %v", err)
	}

	if prediction.FailureProbability < 0 || prediction.FailureProbability > 1 {
		t.Errorf("故障概率超出范围: %.4f", prediction.FailureProbability)
	}

	if prediction.Confidence < 0 || prediction.Confidence > 1 {
		t.Errorf("置信度超出范围: %.4f", prediction.Confidence)
	}

	if prediction.EstimatedLifeDays < 0 {
		t.Errorf("剩余寿命不能为负: %d", prediction.EstimatedLifeDays)
	}
}

func TestFailurePredictor_DegradedDisk(t *testing.T) {
	svc := createTestService()

	prediction, err := svc.Predictor.Predict("sdb")
	if err != nil {
		t.Fatalf("预测失败: %v", err)
	}

	normalPred, _ := svc.Predictor.Predict("sda")
	if prediction.FailureProbability < normalPred.FailureProbability {
		t.Errorf("问题磁盘故障概率应高于正常磁盘: %.4f vs %.4f", prediction.FailureProbability, normalPred.FailureProbability)
	}

	if len(prediction.RiskFactors) == 0 {
		t.Error("问题磁盘应有风险因素")
	}
}

func TestDiskGroupManager_CreateAndList(t *testing.T) {
	svc := createTestService()

	groups := svc.GroupMgr.ListGroups()
	if len(groups) != 2 {
		t.Errorf("磁盘组数量不匹配: got %d, want 2", len(groups))
	}
}

func TestDiskGroupManager_EvaluateGroup(t *testing.T) {
	svc := createTestService()

	group, err := svc.GroupMgr.EvaluateGroup("raid1")
	if err != nil {
		t.Fatalf("评估磁盘组失败: %v", err)
	}

	if group.GroupScore <= 0 {
		t.Errorf("磁盘组评分应大于 0: %.1f", group.GroupScore)
	}

	if group.WeakestDisk == "" {
		t.Error("最弱磁盘不应为空")
	}

	if group.WeakestDisk != "sdb" {
		t.Errorf("最弱磁盘不匹配: got %s, want sdb", group.WeakestDisk)
	}
}

func TestMaintenanceAdvisor_GenerateAdvice(t *testing.T) {
	svc := createTestService()

	advices, err := svc.Advisor.GenerateAdvice()
	if err != nil {
		t.Fatalf("生成建议失败: %v", err)
	}

	hasSDBAdvice := false
	for _, advice := range advices {
		if advice.Device == "sdb" {
			hasSDBAdvice = true
			break
		}
	}

	if !hasSDBAdvice {
		t.Error("应为 sdb 磁盘生成维护建议")
	}
}

func TestHandlers_ListDisks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := createTestService()
	handler := NewHandler(svc)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	req, _ := http.NewRequest("GET", "/api/v1/diskhealthai2/disks", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码不匹配: got %d, want %d", w.Code, http.StatusOK)
	}

	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if !resp.Success {
		t.Error("响应应为成功")
	}
}

func TestHandlers_GetScore(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := createTestService()
	handler := NewHandler(svc)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	req, _ := http.NewRequest("GET", "/api/v1/diskhealthai2/disks/sda/score", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码不匹配: got %d, want %d", w.Code, http.StatusOK)
	}

	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if !resp.Success {
		t.Error("响应应为成功")
	}
}

func TestHandlers_Dashboard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := createTestService()
	handler := NewHandler(svc)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	req, _ := http.NewRequest("GET", "/api/v1/diskhealthai2/dashboard", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码不匹配: got %d, want %d", w.Code, http.StatusOK)
	}

	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if !resp.Success {
		t.Error("响应应为成功")
	}
}

func TestHandlers_TriggerScan(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := createTestService()
	handler := NewHandler(svc)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	req, _ := http.NewRequest("POST", "/api/v1/diskhealthai2/scan", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码不匹配: got %d, want %d", w.Code, http.StatusOK)
	}

	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if !resp.Success {
		t.Error("响应应为成功")
	}
}

func TestHandlers_GetSMART(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := createTestService()
	handler := NewHandler(svc)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	req, _ := http.NewRequest("GET", "/api/v1/diskhealthai2/disks/sda/smart", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码不匹配: got %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandlers_Predict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := createTestService()
	handler := NewHandler(svc)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	req, _ := http.NewRequest("GET", "/api/v1/diskhealthai2/disks/sda/predict", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码不匹配: got %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandlers_Advice(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := createTestService()
	handler := NewHandler(svc)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	req, _ := http.NewRequest("GET", "/api/v1/diskhealthai2/advice", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码不匹配: got %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandlers_Groups(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := createTestService()
	handler := NewHandler(svc)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	req, _ := http.NewRequest("GET", "/api/v1/diskhealthai2/groups", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码不匹配: got %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandlers_GroupHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := createTestService()
	handler := NewHandler(svc)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	req, _ := http.NewRequest("GET", "/api/v1/diskhealthai2/groups/raid1/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码不匹配: got %d, want %d", w.Code, http.StatusOK)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		want  HealthGrade
	}{
		{95, GradeA},
		{80, GradeB},
		{60, GradeC},
		{40, GradeD},
		{20, GradeF},
	}

	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.want {
			t.Errorf("scoreToGrade(%v) = %v, want %v", tt.score, got, tt.want)
		}
	}
}

func TestScoreToStatus(t *testing.T) {
	tests := []struct {
		score float64
		want  DiskStatus
	}{
		{90, StatusHealthy},
		{60, StatusWarning},
		{40, StatusCritical},
		{20, StatusFailed},
	}

	for _, tt := range tests {
		got := scoreToStatus(tt.score)
		if got != tt.want {
			t.Errorf("scoreToStatus(%v) = %v, want %v", tt.score, got, tt.want)
		}
	}
}

func TestGetAttributeName(t *testing.T) {
	tests := []struct {
		id   SMARTAttributeID
		want string
	}{
		{SMARTIDReallocatedSectorCt, "Reallocated_Sector_Ct"},
		{SMARTIDTemperatureCelsius, "Temperature_Celsius"},
		{SMARTIDPowerOnHours, "Power_On_Hours"},
		{SMARTIDSSDLifeLeft, "SSD_Life_Left"},
	}

	for _, tt := range tests {
		got := GetAttributeName(tt.id)
		if got != tt.want {
			t.Errorf("GetAttributeName(%d) = %s, want %s", tt.id, got, tt.want)
		}
	}
}

func TestBayesianPrediction_Values(t *testing.T) {
	svc := createTestService()

	prediction, err := svc.Predictor.Predict("sda")
	if err != nil {
		t.Fatalf("预测失败: %v", err)
	}

	if prediction.PriorProbability < 0 || prediction.PriorProbability > 1 {
		t.Errorf("先验概率超出范围: %.4f", prediction.PriorProbability)
	}

	if prediction.Likelihood < 0 || prediction.Likelihood > 1 {
		t.Errorf("似然超出范围: %.4f", prediction.Likelihood)
	}

	if prediction.PosteriorProbability < 0 || prediction.PosteriorProbability > 1 {
		t.Errorf("后验概率超出范围: %.4f", prediction.PosteriorProbability)
	}
}

func TestDiskGroupManager_NotFound(t *testing.T) {
	svc := createTestService()

	_, err := svc.GroupMgr.GetGroup("nonexistent")
	if err == nil {
		t.Error("不存在的磁盘组应返回错误")
	}
}

func TestSMARTAnalyzer_NoData(t *testing.T) {
	analyzer := NewSMARTAnalyzer(100)

	_, err := analyzer.GetLatestData("nonexistent")
	if err == nil {
		t.Error("无数据设备应返回错误")
	}

	_, err = analyzer.Analyze("nonexistent")
	if err == nil {
		t.Error("无数据设备应返回错误")
	}
}

func TestDiskGroupManager_EvaluateAllGroups(t *testing.T) {
	svc := createTestService()

	groups := svc.GroupMgr.EvaluateAllGroups()
	if len(groups) != 2 {
		t.Errorf("磁盘组数量不匹配: got %d, want 2", len(groups))
	}

	if len(groups) >= 2 && groups[0].GroupScore > groups[1].GroupScore {
		t.Error("磁盘组应按评分升序排列")
	}
}

func TestDiskGroupManager_RemoveGroup(t *testing.T) {
	svc := createTestService()

	svc.GroupMgr.CreateGroup("temp", "临时组", "临时", []string{"sda"})

	err := svc.GroupMgr.RemoveGroup("temp")
	if err != nil {
		t.Fatalf("删除磁盘组失败: %v", err)
	}

	err = svc.GroupMgr.RemoveGroup("temp")
	if err == nil {
		t.Error("删除不存在的磁盘组应返回错误")
	}
}

func TestLinearRegression(t *testing.T) {
	analyzer := NewSMARTAnalyzer(100)

	for i := 0; i < 20; i++ {
		analyzer.RecordData(SMARTData{
			Device: "test",
			Attributes: []SMARTAttribute{
				{ID: SMARTIDTemperatureCelsius, RawValue: uint64(30 + i)},
			},
			CollectedAt: time.Now(),
		})
	}

	result, err := analyzer.Analyze("test")
	if err != nil {
		t.Fatalf("分析失败: %v", err)
	}

	for _, attr := range result.Attributes {
		if attr.AttributeID == SMARTIDTemperatureCelsius {
			if attr.Regression == nil {
				t.Error("应有回归分析结果")
			}
			if attr.Regression.Slope <= 0 {
				t.Errorf("斜率应为正: %.4f", attr.Regression.Slope)
			}
			break
		}
	}
}

func TestHandlers_History(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := createTestService()
	handler := NewHandler(svc)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	req, _ := http.NewRequest("GET", "/api/v1/diskhealthai2/disks/sda/history", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码不匹配: got %d, want %d", w.Code, http.StatusOK)
	}
}
