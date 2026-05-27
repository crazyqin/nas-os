// Package dataprovenance 提供数据溯源追踪功能
package dataprovenance

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestEngine_RecordOperation(t *testing.T) {
	engine := NewEngine(nil)

	record := &ProvenanceRecord{
		ID:          "record-1",
		FileID:      "file-1",
		FilePath:    "/data/test.txt",
		Operation:   OpCreate,
		UserID:      "user-1",
		UserName:    "testuser",
		Source:      SourceUpload,
		FileSize:    1024,
		CurrentHash: "abc123",
	}

	err := engine.RecordOperation(record)
	if err != nil {
		t.Fatalf("RecordOperation failed: %v", err)
	}

	// 验证记录已保存
	got, err := engine.GetRecord("record-1")
	if err != nil {
		t.Fatalf("GetRecord failed: %v", err)
	}
	if got.FileID != "file-1" {
		t.Errorf("expected file_id=file-1, got %s", got.FileID)
	}
}

func TestEngine_RecordOperation_InvalidInput(t *testing.T) {
	engine := NewEngine(nil)

	// 测试空记录
	err := engine.RecordOperation(nil)
	if err != ErrInvalidInput {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}

	// 测试缺少ID
	err = engine.RecordOperation(&ProvenanceRecord{FileID: "file-1"})
	if err != ErrInvalidInput {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

func TestEngine_GetRecord_NotFound(t *testing.T) {
	engine := NewEngine(nil)

	_, err := engine.GetRecord("nonexistent")
	if err != ErrRecordNotFound {
		t.Errorf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestEngine_GetFileHistory(t *testing.T) {
	engine := NewEngine(nil)

	// 创建多个操作记录
	records := []*ProvenanceRecord{
		{ID: "r1", FileID: "file-1", FilePath: "/test.txt", Operation: OpCreate, UserID: "user-1"},
		{ID: "r2", FileID: "file-1", FilePath: "/test.txt", Operation: OpModify, UserID: "user-1"},
		{ID: "r3", FileID: "file-1", FilePath: "/test.txt", Operation: OpModify, UserID: "user-2"},
	}

	for _, r := range records {
		if err := engine.RecordOperation(r); err != nil {
			t.Fatalf("RecordOperation failed: %v", err)
		}
	}

	history, err := engine.GetFileHistory("file-1")
	if err != nil {
		t.Fatalf("GetFileHistory failed: %v", err)
	}
	if len(history) != 3 {
		t.Errorf("expected 3 records, got %d", len(history))
	}
}

func TestEngine_GetFileHistory_NotFound(t *testing.T) {
	engine := NewEngine(nil)

	_, err := engine.GetFileHistory("nonexistent")
	if err != ErrFileNotFound {
		t.Errorf("expected ErrFileNotFound, got %v", err)
	}
}

func TestEngine_GetUserAudit(t *testing.T) {
	engine := NewEngine(nil)

	records := []*ProvenanceRecord{
		{ID: "r1", FileID: "file-1", Operation: OpCreate, UserID: "user-1", UserName: "Alice"},
		{ID: "r2", FileID: "file-2", Operation: OpModify, UserID: "user-1", UserName: "Alice"},
		{ID: "r3", FileID: "file-3", Operation: OpDelete, UserID: "user-2", UserName: "Bob"},
	}

	for _, r := range records {
		engine.RecordOperation(r)
	}

	entries := engine.GetUserAudit("user-1")
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

func TestEngine_QueryRecords(t *testing.T) {
	engine := NewEngine(nil)

	now := time.Now()
	records := []*ProvenanceRecord{
		{ID: "r1", FileID: "file-1", Operation: OpCreate, UserID: "user-1", Timestamp: now.Add(-2 * time.Hour)},
		{ID: "r2", FileID: "file-1", Operation: OpModify, UserID: "user-1", Timestamp: now.Add(-1 * time.Hour)},
		{ID: "r3", FileID: "file-2", Operation: OpCreate, UserID: "user-2", Timestamp: now},
	}

	for _, r := range records {
		engine.RecordOperation(r)
	}

	// 按文件ID查询
	filter := QueryFilter{FileID: "file-1"}
	results := engine.QueryRecords(filter)
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	// 按操作类型查询
	filter = QueryFilter{Operation: OpCreate}
	results = engine.QueryRecords(filter)
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestEngine_Lineage(t *testing.T) {
	engine := NewEngine(nil)

	// 创建血缘关系: file-1 -> file-2 -> file-3
	records := []*ProvenanceRecord{
		{ID: "r1", FileID: "file-1", FilePath: "/original.txt", Operation: OpCreate, UserID: "user-1"},
		{ID: "r2", FileID: "file-2", FilePath: "/copy.txt", Operation: OpCopy, UserID: "user-1", ParentID: "file-1"},
		{ID: "r3", FileID: "file-3", FilePath: "/derived.txt", Operation: OpCopy, UserID: "user-2", ParentID: "file-2"},
	}

	for _, r := range records {
		engine.RecordOperation(r)
	}

	lineage, err := engine.GetLineage("file-3")
	if err != nil {
		t.Fatalf("GetLineage failed: %v", err)
	}

	if len(lineage.Ancestors) != 1 {
		t.Errorf("expected 1 ancestor, got %d", len(lineage.Ancestors))
	}

	// 检查 file-1 的后代
	lineage1, _ := engine.GetLineage("file-1")
	if len(lineage1.Descendants) != 1 {
		t.Errorf("expected 1 descendant for file-1, got %d", len(lineage1.Descendants))
	}
}

func TestEngine_AnalyzeImpact(t *testing.T) {
	engine := NewEngine(nil)

	records := []*ProvenanceRecord{
		{ID: "r1", FileID: "file-1", FilePath: "/source.txt", Operation: OpCreate, UserID: "user-1"},
		{ID: "r2", FileID: "file-2", FilePath: "/derived1.txt", Operation: OpCopy, UserID: "user-1", ParentID: "file-1"},
		{ID: "r3", FileID: "file-3", FilePath: "/derived2.txt", Operation: OpCopy, UserID: "user-2", ParentID: "file-1"},
	}

	for _, r := range records {
		engine.RecordOperation(r)
	}

	impact, err := engine.AnalyzeImpact("file-1")
	if err != nil {
		t.Fatalf("AnalyzeImpact failed: %v", err)
	}

	if impact.TotalAffected != 2 {
		t.Errorf("expected 2 affected files, got %d", impact.TotalAffected)
	}
}

func TestEngine_VerifyIntegrity(t *testing.T) {
	engine := NewEngine(nil)

	record := &ProvenanceRecord{
		ID:          "r1",
		FileID:      "file-1",
		Operation:   OpCreate,
		CurrentHash: "abc123",
	}
	engine.RecordOperation(record)

	// 哈希匹配
	result, err := engine.VerifyIntegrity("file-1", "abc123")
	if err != nil {
		t.Fatalf("VerifyIntegrity failed: %v", err)
	}
	if !result.IsValid {
		t.Error("expected integrity check to pass")
	}

	// 哈希不匹配
	result, err = engine.VerifyIntegrity("file-1", "wronghash")
	if err != nil {
		t.Fatalf("VerifyIntegrity failed: %v", err)
	}
	if result.IsValid {
		t.Error("expected integrity check to fail")
	}
}

func TestEngine_ComplianceReport(t *testing.T) {
	engine := NewEngine(nil)

	now := time.Now()
	records := []*ProvenanceRecord{
		{ID: "r1", FileID: "file-1", Operation: OpCreate, UserID: "user-1", Timestamp: now.Add(-2 * time.Hour)},
		{ID: "r2", FileID: "file-1", Operation: OpModify, UserID: "user-1", Timestamp: now.Add(-1 * time.Hour)},
		{ID: "r3", FileID: "file-2", Operation: OpDelete, UserID: "user-2", Timestamp: now},
	}

	for _, r := range records {
		engine.records[r.ID] = r
		engine.fileIndex[r.FileID] = append(engine.fileIndex[r.FileID], r.ID)
		engine.userIndex[r.UserID] = append(engine.userIndex[r.UserID], r.ID)
		engine.operationIndex[r.Operation] = append(engine.operationIndex[r.Operation], r.ID)
	}

	report := engine.GenerateComplianceReport(now.Add(-3*time.Hour), now.Add(time.Hour))

	if report.TotalOperations != 3 {
		t.Errorf("expected 3 operations, got %d", report.TotalOperations)
	}
	if report.OperationsByType[OpCreate] != 1 {
		t.Errorf("expected 1 create operation, got %d", report.OperationsByType[OpCreate])
	}
	if report.OperationsByUser["user-1"] != 2 {
		t.Errorf("expected 2 operations for user-1, got %d", report.OperationsByUser["user-1"])
	}
}

func TestEngine_RetentionPolicy(t *testing.T) {
	engine := NewEngine(&RetentionPolicy{
		MaxAge:     1 * time.Hour,
		MaxRecords: 1000,
	})

	// 添加旧记录
	oldRecord := &ProvenanceRecord{
		ID:        "old-1",
		FileID:    "file-1",
		Operation: OpCreate,
		Timestamp: time.Now().Add(-2 * time.Hour),
	}
	engine.records[oldRecord.ID] = oldRecord

	// 添加新记录
	newRecord := &ProvenanceRecord{
		ID:        "new-1",
		FileID:    "file-2",
		Operation: OpCreate,
		Timestamp: time.Now(),
	}
	engine.records[newRecord.ID] = newRecord
	engine.fileIndex[newRecord.FileID] = []string{newRecord.ID}
	engine.operationIndex[newRecord.Operation] = []string{newRecord.ID}

	cleaned := engine.CleanupExpired()
	if cleaned != 1 {
		t.Errorf("expected 1 cleaned record, got %d", cleaned)
	}

	// 验证旧记录已删除
	_, err := engine.GetRecord("old-1")
	if err != ErrRecordNotFound {
		t.Errorf("expected old record to be deleted")
	}

	// 验证新记录仍在
	_, err = engine.GetRecord("new-1")
	if err != nil {
		t.Errorf("expected new record to exist")
	}
}

func TestCalculateHash(t *testing.T) {
	data := []byte("test data")
	hash := CalculateHash(data)
	if hash == "" {
		t.Error("expected non-empty hash")
	}

	// 相同数据应产生相同哈希
	hash2 := CalculateHash(data)
	if hash != hash2 {
		t.Error("expected same hash for same data")
	}

	// 不同数据应产生不同哈希
	hash3 := CalculateHash([]byte("different data"))
	if hash == hash3 {
		t.Error("expected different hash for different data")
	}
}

func TestHandlers_CreateRecord(t *testing.T) {
	engine := NewEngine(nil)
	h := NewHandlers(engine)

	body := `{
		"id": "r1",
		"file_id": "file-1",
		"file_path": "/test.txt",
		"operation": "create",
		"user_id": "user-1",
		"source": "upload"
	}`

	w := httptest.NewRecorder()
	c := createTestContext(w, "POST", "/api/v1/data-provenance/records", body)
	h.createRecord(c)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
}

func TestHandlers_GetRecord(t *testing.T) {
	engine := NewEngine(nil)
	engine.RecordOperation(&ProvenanceRecord{
		ID:        "r1",
		FileID:    "file-1",
		Operation: OpCreate,
	})

	h := NewHandlers(engine)

	w := httptest.NewRecorder()
	c := createTestContext(w, "GET", "/api/v1/data-provenance/records/r1", "")
	c.Params = []gin.Param{{Key: "id", Value: "r1"}}
	h.getRecord(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlers_GetRecord_NotFound(t *testing.T) {
	engine := NewEngine(nil)
	h := NewHandlers(engine)

	w := httptest.NewRecorder()
	c := createTestContext(w, "GET", "/api/v1/data-provenance/records/nonexistent", "")
	c.Params = []gin.Param{{Key: "id", Value: "nonexistent"}}
	h.getRecord(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandlers_GetFileHistory(t *testing.T) {
	engine := NewEngine(nil)
	engine.RecordOperation(&ProvenanceRecord{
		ID: "r1", FileID: "file-1", Operation: OpCreate,
	})
	engine.RecordOperation(&ProvenanceRecord{
		ID: "r2", FileID: "file-1", Operation: OpModify,
	})

	h := NewHandlers(engine)

	w := httptest.NewRecorder()
	c := createTestContext(w, "GET", "/api/v1/data-provenance/files/file-1/history", "")
	c.Params = []gin.Param{{Key: "fileId", Value: "file-1"}}
	h.getFileHistory(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlers_QueryRecords(t *testing.T) {
	engine := NewEngine(nil)
	engine.RecordOperation(&ProvenanceRecord{
		ID: "r1", FileID: "file-1", Operation: OpCreate, UserID: "user-1",
	})

	h := NewHandlers(engine)

	body := `{"file_id": "file-1"}`
	w := httptest.NewRecorder()
	c := createTestContext(w, "POST", "/api/v1/data-provenance/query", body)
	h.queryRecords(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlers_GetRetentionPolicy(t *testing.T) {
	engine := NewEngine(nil)
	h := NewHandlers(engine)

	w := httptest.NewRecorder()
	c := createTestContext(w, "GET", "/api/v1/data-provenance/retention", "")
	h.getRetentionPolicy(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlers_CleanupExpired(t *testing.T) {
	engine := NewEngine(&RetentionPolicy{MaxAge: 1 * time.Hour})
	engine.records["old"] = &ProvenanceRecord{
		ID: "old", Timestamp: time.Now().Add(-2 * time.Hour),
	}
	engine.records["new"] = &ProvenanceRecord{
		ID: "new", Timestamp: time.Now(),
	}

	h := NewHandlers(engine)

	w := httptest.NewRecorder()
	c := createTestContext(w, "POST", "/api/v1/data-provenance/retention/cleanup", "")
	h.cleanupExpired(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// createTestContext 创建测试用的 gin.Context.
func createTestContext(w *httptest.ResponseRecorder, method, path, body string) *gin.Context {
	gin.SetMode(gin.TestMode)

	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}

	c, _ := gin.CreateTestContext(w)
	c.Request = req
	return c
}
