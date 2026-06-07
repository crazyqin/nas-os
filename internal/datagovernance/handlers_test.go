package datagovernance

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ========== 管理器基础测试 ==========

func TestNewManager(t *testing.T) {
	cfg := Config{Enabled: true, DefaultRegion: RegionChina}
	m := NewManager(cfg)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.IsRunning() {
		t.Error("new manager should not be running")
	}
}

func TestStartStop(t *testing.T) {
	m := NewManager(Config{})
	if err := m.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if !m.IsRunning() {
		t.Error("expected running")
	}
	if err := m.Start(); err != ErrAlreadyRunning {
		t.Errorf("double Start should return ErrAlreadyRunning, got %v", err)
	}
	if err := m.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if m.IsRunning() {
		t.Error("expected stopped")
	}
}

// ========== 数据分类标签测试 ==========

func TestClassifyAsset(t *testing.T) {
	m := NewManager(Config{})
	m.Start()

	// 注册资产
	err := m.RegisterAsset(DataAsset{
		ID:       "asset-1",
		Name:     "financial-report.xlsx",
		FilePath: "/data/finance/report.xlsx",
		FileType: "xlsx",
		Region:   RegionChina,
		OwnerID:  "user1",
	})
	if err != nil {
		t.Fatalf("RegisterAsset failed: %v", err)
	}

	// 分类为机密
	err = m.ClassifyAsset("asset-1", LevelConfidential, "manual")
	if err != nil {
		t.Fatalf("ClassifyAsset failed: %v", err)
	}

	asset, _ := m.GetAsset("asset-1")
	if asset.Sensitivity != LevelConfidential {
		t.Errorf("expected confidential, got %s", asset.Sensitivity)
	}
	if asset.ClassifiedBy != "manual" {
		t.Errorf("expected manual, got %s", asset.ClassifiedBy)
	}
	if asset.PolicyID == "" {
		t.Error("expected policy to be auto-matched")
	}

	// 分类不存在的资产
	err = m.ClassifyAsset("nonexistent", LevelPublic, "ai")
	if err != ErrRecordNotFound {
		t.Errorf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestAutoClassify(t *testing.T) {
	m := NewManager(Config{AutoClassify: true})
	m.Start()

	// 注册多个未分类资产
	assets := []DataAsset{
		{ID: "a1", Name: "secret.key", FilePath: "/secrets/secret.key", FileType: "key", Region: RegionChina},
		{ID: "a2", Name: "finance.pdf", FilePath: "/data/finance/budget.pdf", FileType: "pdf", Region: RegionChina},
		{ID: "a3", Name: "readme.md", FilePath: "/docs/readme.md", FileType: "md", Region: RegionChina},
		{ID: "a4", Name: "server.go", FilePath: "/internal/server.go", FileType: "go", Region: RegionChina},
	}
	for _, a := range assets {
		m.RegisterAsset(a)
	}

	count := m.AutoClassify()
	if count != 4 {
		t.Errorf("expected 4 classified, got %d", count)
	}

	a1, _ := m.GetAsset("a1")
	if a1.Sensitivity != LevelTopSecret {
		t.Errorf("secret.key should be top_secret, got %s", a1.Sensitivity)
	}

	a2, _ := m.GetAsset("a2")
	if a2.Sensitivity != LevelConfidential {
		t.Errorf("finance.pdf should be confidential, got %s", a2.Sensitivity)
	}

	a4, _ := m.GetAsset("a4")
	if a4.Sensitivity != LevelInternal {
		t.Errorf("server.go should be internal, got %s", a4.Sensitivity)
	}
}

// ========== 数据驻留合规测试 ==========

func TestResidencyCompliance(t *testing.T) {
	m := NewManager(Config{
		DefaultRegion:  RegionChina,
		AllowedRegions: []GeoRegion{RegionChina, RegionAPAC},
	})
	m.Start()

	// 注册合规资产
	m.RegisterAsset(DataAsset{ID: "cn-1", Name: "cn-data", Region: RegionChina})
	m.RegisterAsset(DataAsset{ID: "apac-1", Name: "apac-data", Region: RegionAPAC})

	// 注册违规资产
	m.RegisterAsset(DataAsset{ID: "us-1", Name: "us-data", Region: RegionUSEast})

	// 检查单个资产
	ok, err := m.CheckResidency("cn-1")
	if err != nil {
		t.Fatalf("CheckResidency failed: %v", err)
	}
	if !ok {
		t.Error("cn-1 should be compliant")
	}

	ok, _ = m.CheckResidency("us-1")
	if ok {
		t.Error("us-1 should not be compliant")
	}

	// 检查全部违规
	violations := m.CheckAllResidency()
	if len(violations) != 1 {
		t.Errorf("expected 1 violation, got %d", len(violations))
	}
}

func TestRelocateAsset(t *testing.T) {
	m := NewManager(Config{
		AllowedRegions: []GeoRegion{RegionChina},
	})
	m.Start()

	m.RegisterAsset(DataAsset{ID: "asset-r1", Name: "data", Region: RegionUSEast})

	// 迁移前违规
	ok, _ := m.CheckResidency("asset-r1")
	if ok {
		t.Error("should be non-compliant before relocate")
	}

	// 迁移到中国
	err := m.RelocateAsset("asset-r1", RegionChina)
	if err != nil {
		t.Fatalf("RelocateAsset failed: %v", err)
	}

	// 迁移后合规
	ok, _ = m.CheckResidency("asset-r1")
	if !ok {
		t.Error("should be compliant after relocate")
	}
}

// ========== 保留策略测试 ==========

func TestRetentionPolicyCRUD(t *testing.T) {
	m := NewManager(Config{})
	m.Start()

	// 应有默认策略
	policies := m.ListPolicies()
	if len(policies) < 4 {
		t.Errorf("expected at least 4 default policies, got %d", len(policies))
	}

	// 获取策略
	p, err := m.GetPolicy("policy-public")
	if err != nil {
		t.Fatalf("GetPolicy failed: %v", err)
	}
	if p.RetentionDays != 365 {
		t.Errorf("expected 365 days, got %d", p.RetentionDays)
	}

	// 创建自定义策略
	err = m.CreatePolicy(RetentionPolicy{
		ID:               "policy-custom",
		Name:             "自定义策略",
		RetentionDays:    30,
		ExpirationAction: RetentionActionDestroy,
		Enabled:          true,
	})
	if err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}

	// 更新策略
	err = m.UpdatePolicy("policy-custom", RetentionPolicy{
		Name:          "更新后的策略",
		RetentionDays: 60,
	})
	if err != nil {
		t.Fatalf("UpdatePolicy failed: %v", err)
	}
	p, _ = m.GetPolicy("policy-custom")
	if p.RetentionDays != 60 {
		t.Errorf("expected 60, got %d", p.RetentionDays)
	}

	// 删除策略
	err = m.DeletePolicy("policy-custom")
	if err != nil {
		t.Fatalf("DeletePolicy failed: %v", err)
	}
	_, err = m.GetPolicy("policy-custom")
	if err != ErrPolicyNotFound {
		t.Error("expected ErrPolicyNotFound after delete")
	}
}

func TestEnforceRetention(t *testing.T) {
	m := NewManager(Config{})
	m.Start()

	// 注册一个已过期的资产
	pastTime := time.Now().Add(-24 * time.Hour)
	m.RegisterAsset(DataAsset{
		ID:                "expired-1",
		Name:              "old-report.pdf",
		Region:            RegionChina,
		PolicyID:          "policy-public",
		RetentionDeadline: &pastTime,
	})

	// 注册一个未过期的资产
	futureTime := time.Now().Add(365 * 24 * time.Hour)
	m.RegisterAsset(DataAsset{
		ID:                "valid-1",
		Name:              "new-report.pdf",
		Region:            RegionChina,
		PolicyID:          "policy-public",
		RetentionDeadline: &futureTime,
	})

	expired := m.EnforceRetention()
	if len(expired) != 1 {
		t.Errorf("expected 1 expired asset, got %d", len(expired))
	}
	if len(expired) > 0 && expired[0].ID != "expired-1" {
		t.Errorf("expected expired-1, got %s", expired[0].ID)
	}
}

// ========== 审计追踪测试 ==========

func TestAuditLog(t *testing.T) {
	m := NewManager(Config{})
	m.Start()

	// 注册资产会自动生成审计记录
	m.RegisterAsset(DataAsset{ID: "aud-1", Name: "test", Region: RegionChina})
	m.RegisterAsset(DataAsset{ID: "aud-2", Name: "test2", Region: RegionChina})

	// 手动记录审计事件
	m.LogAudit(AuditRecord{
		UserID:    "admin",
		UserName:  "Admin",
		Action:    ActionRead,
		AssetID:   "aud-1",
		Result:    "success",
		RiskLevel: "low",
	})

	// 查询所有
	records, total := m.GetAuditLog("", "", "", 1, 10)
	if total < 3 {
		t.Errorf("expected at least 3 audit records, got %d", total)
	}

	// 按用户过滤
	records, _ = m.GetAuditLog("admin", "", "", 1, 10)
	if len(records) != 1 {
		t.Errorf("expected 1 admin record, got %d", len(records))
	}

	// 按操作过滤
	records, _ = m.GetAuditLog("", "read", "", 1, 10)
	if len(records) != 1 {
		t.Errorf("expected 1 read record, got %d", len(records))
	}
}

// ========== 合规报告生成测试 ==========

func TestGenerateReport(t *testing.T) {
	m := NewManager(Config{
		AllowedRegions: []GeoRegion{RegionChina},
	})
	m.Start()

	// 注册一些资产
	m.RegisterAsset(DataAsset{ID: "rpt-1", Name: "doc1", Region: RegionChina, Sensitivity: LevelInternal})
	m.RegisterAsset(DataAsset{ID: "rpt-2", Name: "doc2", Region: RegionChina, Sensitivity: LevelPublic})
	m.RegisterAsset(DataAsset{ID: "rpt-3", Name: "doc3", Region: RegionUSEast, Sensitivity: LevelConfidential})

	// 生成 GDPR 报告
	report := m.GenerateReport(FrameworkGDPR)
	if report == nil {
		t.Fatal("GenerateReport returned nil")
	}
	if report.Framework != FrameworkGDPR {
		t.Errorf("expected GDPR, got %s", report.Framework)
	}
	if report.TotalChecks == 0 {
		t.Error("expected some checks")
	}
	if report.OverallScore <= 0 {
		t.Error("expected positive score")
	}
	if report.GeneratedAt.IsZero() {
		t.Error("expected non-zero GeneratedAt")
	}

	// 获取报告
	got, err := m.GetReport(report.ID)
	if err != nil {
		t.Fatalf("GetReport failed: %v", err)
	}
	if got.ID != report.ID {
		t.Error("report ID mismatch")
	}
}

// ========== 数据血缘追踪测试 ==========

func TestDataLineage(t *testing.T) {
	m := NewManager(Config{})
	m.Start()

	// 注册资产
	m.RegisterAsset(DataAsset{ID: "src-1", Name: "原始数据.csv", Region: RegionChina})
	m.RegisterAsset(DataAsset{ID: "mid-1", Name: "清洗数据.csv", Region: RegionChina})
	m.RegisterAsset(DataAsset{ID: "dst-1", Name: "分析报告.pdf", Region: RegionChina})

	// 添加血缘: mid-1 派生自 src-1
	err := m.AddLineage(LineageRecord{
		AssetID:         "mid-1",
		AssetName:       "清洗数据.csv",
		SourceAssetID:   "src-1",
		SourceAssetName: "原始数据.csv",
		Relation:        RelationDerivedFrom,
		Description:     "数据清洗",
		OperatorID:      "user1",
		OperatorName:    "数据工程师",
	})
	if err != nil {
		t.Fatalf("AddLineage failed: %v", err)
	}

	// 添加血缘: dst-1 派生自 mid-1
	m.AddLineage(LineageRecord{
		AssetID:         "dst-1",
		AssetName:       "分析报告.pdf",
		SourceAssetID:   "mid-1",
		SourceAssetName: "清洗数据.csv",
		Relation:        RelationTransformedFrom,
		Description:     "数据分析",
		OperatorID:      "user2",
		OperatorName:    "分析师",
	})

	// 查询直接血缘
	records, err := m.GetLineage("mid-1")
	if err != nil {
		t.Fatalf("GetLineage failed: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("expected 1 lineage record for mid-1, got %d", len(records))
	}

	// 查询上游血缘链
	chain := m.GetLineageUpstream("dst-1")
	if len(chain) < 2 {
		t.Errorf("expected at least 2 upstream records, got %d", len(chain))
	}
}

// ========== 统计概览测试 ==========

func TestGetStats(t *testing.T) {
	m := NewManager(Config{
		AllowedRegions: []GeoRegion{RegionChina},
	})
	m.Start()

	m.RegisterAsset(DataAsset{ID: "s1", Name: "a", Region: RegionChina, Sensitivity: LevelPublic})
	m.RegisterAsset(DataAsset{ID: "s2", Name: "b", Region: RegionChina, Sensitivity: LevelConfidential})
	m.RegisterAsset(DataAsset{ID: "s3", Name: "c", Region: RegionEU, Sensitivity: LevelTopSecret})

	stats := m.GetStats()
	if stats.TotalAssets != 3 {
		t.Errorf("expected 3 assets, got %d", stats.TotalAssets)
	}
	if stats.Classifications[LevelPublic] != 1 {
		t.Errorf("expected 1 public, got %d", stats.Classifications[LevelPublic])
	}
	if stats.ResidencyViolations != 1 {
		t.Errorf("expected 1 violation, got %d", stats.ResidencyViolations)
	}
	if stats.TotalPolicies < 4 {
		t.Errorf("expected at least 4 policies, got %d", stats.TotalPolicies)
	}
}

// ========== REST API Handler 测试 ==========

func TestHandlerStatus(t *testing.T) {
	m := NewManager(Config{})
	m.Start()
	h := NewHandler(m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/governance/status", nil)
	w := httptest.NewRecorder()
	h.handleStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["running"] != true {
		t.Error("expected running=true")
	}
}

func TestHandlerRegisterAndGetAsset(t *testing.T) {
	m := NewManager(Config{})
	m.Start()
	h := NewHandler(m)

	// 注册资产
	body, _ := json.Marshal(DataAsset{
		ID:       "api-asset-1",
		Name:     "test.txt",
		FilePath: "/data/test.txt",
		FileType: "txt",
		Region:   RegionChina,
		OwnerID:  "user1",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/governance/asset/register", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.handleRegisterAsset(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("register: expected 200, got %d", w.Code)
	}

	// 获取资产
	req = httptest.NewRequest(http.MethodGet, "/api/v1/governance/asset?id=api-asset-1", nil)
	w = httptest.NewRecorder()
	h.handleAsset(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("get asset: expected 200, got %d", w.Code)
	}

	var asset DataAsset
	json.NewDecoder(w.Body).Decode(&asset)
	if asset.Name != "test.txt" {
		t.Errorf("expected test.txt, got %s", asset.Name)
	}
}

func TestHandlerClassifyAndAutoClassify(t *testing.T) {
	m := NewManager(Config{AutoClassify: true})
	m.Start()
	h := NewHandler(m)

	// 注册资产
	m.RegisterAsset(DataAsset{ID: "cls-1", Name: "data.csv", FilePath: "/data/data.csv", FileType: "csv", Region: RegionChina})

	// 手动分类
	body, _ := json.Marshal(map[string]string{
		"assetId":      "cls-1",
		"sensitivity":  "confidential",
		"classifiedBy": "manual",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/governance/asset/classify", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.handleClassifyAsset(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("classify: expected 200, got %d", w.Code)
	}

	// 自动分类
	req = httptest.NewRequest(http.MethodPost, "/api/v1/governance/classify/auto", nil)
	w = httptest.NewRecorder()
	h.handleAutoClassify(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("auto classify: expected 200, got %d", w.Code)
	}
}

func TestHandlerResidencyCheck(t *testing.T) {
	m := NewManager(Config{
		AllowedRegions: []GeoRegion{RegionChina},
	})
	m.Start()
	h := NewHandler(m)

	m.RegisterAsset(DataAsset{ID: "res-1", Name: "data", Region: RegionChina})
	m.RegisterAsset(DataAsset{ID: "res-2", Name: "data-us", Region: RegionUSEast})

	// 合规检查
	req := httptest.NewRequest(http.MethodGet, "/api/v1/governance/residency/check?id=res-1", nil)
	w := httptest.NewRecorder()
	h.handleResidencyCheck(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["compliant"] != true {
		t.Error("res-1 should be compliant")
	}

	// 违规列表
	req = httptest.NewRequest(http.MethodGet, "/api/v1/governance/residency/violations", nil)
	w = httptest.NewRecorder()
	h.handleResidencyViolations(w, req)

	json.NewDecoder(w.Body).Decode(&resp)
	if int(resp["count"].(float64)) != 1 {
		t.Errorf("expected 1 violation, got %v", resp["count"])
	}
}

func TestHandlerPolicies(t *testing.T) {
	m := NewManager(Config{})
	m.Start()
	h := NewHandler(m)

	// 列出策略
	req := httptest.NewRequest(http.MethodGet, "/api/v1/governance/policies", nil)
	w := httptest.NewRecorder()
	h.handlePolicies(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var policies []RetentionPolicy
	json.NewDecoder(w.Body).Decode(&policies)
	if len(policies) < 4 {
		t.Errorf("expected at least 4 default policies, got %d", len(policies))
	}

	// 获取单个策略
	req = httptest.NewRequest(http.MethodGet, "/api/v1/governance/policy?id=policy-public", nil)
	w = httptest.NewRecorder()
	h.handlePolicy(w, req)

	var p RetentionPolicy
	json.NewDecoder(w.Body).Decode(&p)
	if p.Name == "" {
		t.Error("expected non-empty policy name")
	}
}

func TestHandlerAuditLog(t *testing.T) {
	m := NewManager(Config{})
	m.Start()
	h := NewHandler(m)

	// 记录审计事件
	body, _ := json.Marshal(AuditRecord{
		UserID:    "api-user",
		UserName:  "API测试",
		Action:    ActionCreate,
		AssetID:   "test",
		Result:    "success",
		RiskLevel: "low",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/governance/audit/log", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.handleLogAudit(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("log audit: expected 200, got %d", w.Code)
	}

	// 查询审计日志
	req = httptest.NewRequest(http.MethodGet, "/api/v1/governance/audit?userId=api-user", nil)
	w = httptest.NewRecorder()
	h.handleAuditLog(w, req)

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if int(resp["total"].(float64)) != 1 {
		t.Errorf("expected 1 audit record, got %v", resp["total"])
	}
}

func TestHandlerGenerateReport(t *testing.T) {
	m := NewManager(Config{
		AllowedRegions: []GeoRegion{RegionChina},
	})
	m.Start()
	h := NewHandler(m)

	m.RegisterAsset(DataAsset{ID: "gdpr-1", Name: "data", Region: RegionChina, Sensitivity: LevelInternal})

	body, _ := json.Marshal(map[string]string{"framework": "GDPR"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/governance/report/generate", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.handleGenerateReport(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var report ComplianceReport
	json.NewDecoder(w.Body).Decode(&report)
	if report.Framework != FrameworkGDPR {
		t.Errorf("expected GDPR, got %s", report.Framework)
	}
}

func TestHandlerLineage(t *testing.T) {
	m := NewManager(Config{})
	m.Start()
	h := NewHandler(m)

	m.RegisterAsset(DataAsset{ID: "lineage-src", Name: "source", Region: RegionChina})
	m.RegisterAsset(DataAsset{ID: "lineage-dst", Name: "dest", Region: RegionChina})

	// 添加血缘
	body, _ := json.Marshal(LineageRecord{
		AssetID:         "lineage-dst",
		AssetName:       "dest",
		SourceAssetID:   "lineage-src",
		SourceAssetName: "source",
		Relation:        RelationDerivedFrom,
		OperatorID:      "user1",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/governance/lineage/add", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.handleAddLineage(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("add lineage: expected 200, got %d", w.Code)
	}

	// 查询血缘
	req = httptest.NewRequest(http.MethodGet, "/api/v1/governance/lineage?assetId=lineage-dst", nil)
	w = httptest.NewRecorder()
	h.handleGetLineage(w, req)

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if int(resp["count"].(float64)) != 1 {
		t.Errorf("expected 1 lineage record, got %v", resp["count"])
	}
}

func TestHandlerMethodNotAllowed(t *testing.T) {
	m := NewManager(Config{})
	m.Start()
	h := NewHandler(m)

	// GET 到只接受 POST 的端点
	req := httptest.NewRequest(http.MethodGet, "/api/v1/governance/asset/register", nil)
	w := httptest.NewRecorder()
	h.handleRegisterAsset(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandlerDeleteAsset(t *testing.T) {
	m := NewManager(Config{})
	m.Start()
	h := NewHandler(m)

	m.RegisterAsset(DataAsset{ID: "del-1", Name: "to-delete", Region: RegionChina})

	// 删除资产
	req := httptest.NewRequest(http.MethodPost, "/api/v1/governance/asset/delete?id=del-1", nil)
	w := httptest.NewRecorder()
	h.handleDeleteAsset(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	// 确认已删除
	_, err := m.GetAsset("del-1")
	if err != ErrRecordNotFound {
		t.Error("expected asset to be deleted")
	}
}

func TestHandlerEnforceRetention(t *testing.T) {
	m := NewManager(Config{})
	m.Start()
	h := NewHandler(m)

	pastTime := time.Now().Add(-24 * time.Hour)
	m.RegisterAsset(DataAsset{
		ID:                "enf-1",
		Name:              "expired",
		Region:            RegionChina,
		PolicyID:          "policy-public",
		RetentionDeadline: &pastTime,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/governance/retention/enforce", nil)
	w := httptest.NewRecorder()
	h.handleEnforceRetention(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if int(resp["count"].(float64)) != 1 {
		t.Errorf("expected 1 expired, got %v", resp["count"])
	}
}
