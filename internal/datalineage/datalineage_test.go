package datalineage

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTest(t *testing.T) (*Manager, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	tmpDir := t.TempDir()
	mgr := NewManager(filepath.Join(tmpDir, "data.json"))
	require.NoError(t, mgr.Initialize())
	r := gin.New()
	grp := r.Group("")
	NewHandlers(mgr).RegisterRoutes(grp)
	return mgr, r
}

func createTestNodes(t *testing.T, mgr *Manager) {
	t.Helper()
	nodes := []*DataNode{
		{ID: "db-users", Name: "Users DB", Type: SourceDatabase, Classification: ClassPII, Table: "users"},
		{ID: "api-register", Name: "Register API", Type: SourceAPI, Classification: ClassConfidential},
		{ID: "etl-user-report", Name: "User Report ETL", Type: SourceETL, Classification: ClassInternal},
		{ID: "file-report", Name: "Report File", Type: SourceFile, Classification: ClassInternal},
	}
	for _, n := range nodes {
		require.NoError(t, mgr.CreateNode(n))
	}
}

func TestCreateAndListNode(t *testing.T) {
	_, r := setupTest(t)

	node := DataNode{ID: "node-1", Name: "Test DB", Type: SourceDatabase, Classification: ClassInternal}
	body, _ := json.Marshal(node)
	req := httptest.NewRequest(http.MethodPost, "/datalineage/nodes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	req2 := httptest.NewRequest(http.MethodGet, "/datalineage/nodes", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp))
	assert.Equal(t, float64(1), resp["total"])
}

func TestDuplicateNode(t *testing.T) {
	mgr, _ := setupTest(t)
	require.NoError(t, mgr.CreateNode(&DataNode{ID: "dup-1", Name: "A", Type: SourceDatabase}))
	err := mgr.CreateNode(&DataNode{ID: "dup-1", Name: "B", Type: SourceDatabase})
	assert.ErrorIs(t, err, ErrNodeExists)
}

func TestNodeLifecycle(t *testing.T) {
	mgr, r := setupTest(t)
	require.NoError(t, mgr.CreateNode(&DataNode{ID: "node-2", Name: "Redis", Type: SourceDatabase, Classification: ClassInternal}))

	req := httptest.NewRequest(http.MethodGet, "/datalineage/nodes/node-2", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	update := DataNode{Name: "Redis Updated", Classification: ClassConfidential}
	body, _ := json.Marshal(update)
	req2 := httptest.NewRequest(http.MethodPut, "/datalineage/nodes/node-2", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	node, _ := mgr.GetNode("node-2")
	assert.Equal(t, "Redis Updated", node.Name)

	req3 := httptest.NewRequest(http.MethodDelete, "/datalineage/nodes/node-2", nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)
}

func TestEdgeCRUD(t *testing.T) {
	mgr, r := setupTest(t)
	require.NoError(t, mgr.CreateNode(&DataNode{ID: "src", Name: "Source", Type: SourceDatabase}))
	require.NoError(t, mgr.CreateNode(&DataNode{ID: "dst", Name: "Target", Type: SourceAPI}))

	edge := LineageEdge{ID: "e1", SourceID: "src", TargetID: "dst", Type: EdgeDirect, Process: "sync"}
	body, _ := json.Marshal(edge)
	req := httptest.NewRequest(http.MethodPost, "/datalineage/edges", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	req2 := httptest.NewRequest(http.MethodGet, "/datalineage/edges?node_id=src", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	req3 := httptest.NewRequest(http.MethodDelete, "/datalineage/edges/e1", nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)
}

func TestCircularDetection(t *testing.T) {
	mgr, _ := setupTest(t)
	require.NoError(t, mgr.CreateNode(&DataNode{ID: "a", Name: "A", Type: SourceDatabase}))
	require.NoError(t, mgr.CreateNode(&DataNode{ID: "b", Name: "B", Type: SourceAPI}))
	require.NoError(t, mgr.CreateEdge(&LineageEdge{ID: "e1", SourceID: "a", TargetID: "b", Type: EdgeDirect}))

	err := mgr.CreateEdge(&LineageEdge{ID: "e2", SourceID: "b", TargetID: "a", Type: EdgeDirect})
	assert.ErrorIs(t, err, ErrCircularLineage)
}

func TestImpactAnalysis(t *testing.T) {
	mgr, r := setupTest(t)
	createTestNodes(t, mgr)
	require.NoError(t, mgr.CreateEdge(&LineageEdge{ID: "e1", SourceID: "db-users", TargetID: "api-register", Type: EdgeDirect}))
	require.NoError(t, mgr.CreateEdge(&LineageEdge{ID: "e2", SourceID: "api-register", TargetID: "etl-user-report", Type: EdgeTransform}))
	require.NoError(t, mgr.CreateEdge(&LineageEdge{ID: "e3", SourceID: "etl-user-report", TargetID: "file-report", Type: EdgeDirect}))

	req := httptest.NewRequest(http.MethodGet, "/datalineage/impact/db-users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(3), data["total_count"])
}

func TestTraceSource(t *testing.T) {
	mgr, r := setupTest(t)
	createTestNodes(t, mgr)
	require.NoError(t, mgr.CreateEdge(&LineageEdge{ID: "e1", SourceID: "db-users", TargetID: "api-register", Type: EdgeDirect}))
	require.NoError(t, mgr.CreateEdge(&LineageEdge{ID: "e2", SourceID: "api-register", TargetID: "etl-user-report", Type: EdgeTransform}))

	req := httptest.NewRequest(http.MethodGet, "/datalineage/trace/etl-user-report", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(2), data["total_count"])
}

func TestLineageGraph(t *testing.T) {
	mgr, r := setupTest(t)
	createTestNodes(t, mgr)
	require.NoError(t, mgr.CreateEdge(&LineageEdge{ID: "e1", SourceID: "db-users", TargetID: "api-register", Type: EdgeDirect}))
	require.NoError(t, mgr.CreateEdge(&LineageEdge{ID: "e2", SourceID: "api-register", TargetID: "etl-user-report", Type: EdgeTransform}))

	req := httptest.NewRequest(http.MethodGet, "/datalineage/graph/api-register?depth=5", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestComplianceAudit(t *testing.T) {
	mgr, r := setupTest(t)
	createTestNodes(t, mgr)

	record := ProcessingRecord{
		ID:              "rec-1",
		NodeID:          "db-users",
		Operation:       PurposeStorage,
		Regulation:      RegGDPR,
		Purpose:         "用户数据存储",
		Processor:       "data-engineer",
		LegalBasis:      "consent",
		CrossBorder:     false,
		ConsentObtained: true,
	}
	body, _ := json.Marshal(record)
	req := httptest.NewRequest(http.MethodPost, "/datalineage/records", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	req2 := httptest.NewRequest(http.MethodGet, "/datalineage/records?node_id=db-users&regulation=gdpr", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp))
	assert.Equal(t, float64(1), resp["total"])
}

func TestComplianceReport(t *testing.T) {
	mgr, r := setupTest(t)
	createTestNodes(t, mgr)
	require.NoError(t, mgr.AddProcessingRecord(&ProcessingRecord{
		ID: "rec-1", NodeID: "db-users", Operation: PurposeStorage, Regulation: RegGDPR, ConsentObtained: true,
	}))
	require.NoError(t, mgr.AddProcessingRecord(&ProcessingRecord{
		ID: "rec-2", NodeID: "db-users", Operation: PurposeTransfer, Regulation: RegGDPR, CrossBorder: true, DestCountry: "US",
	}))

	req := httptest.NewRequest(http.MethodGet, "/datalineage/compliance/report?regulation=gdpr", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(2), data["total_records"])
	assert.Equal(t, float64(1), data["cross_border_count"])
}

func TestManageClassification(t *testing.T) {
	mgr, r := setupTest(t)
	createTestNodes(t, mgr)

	reqBody := map[string]interface{}{"classification": "restricted", "tags": []string{"pii", "eu-data"}}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPut, "/datalineage/nodes/db-users/classification", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	node, _ := mgr.GetNode("db-users")
	assert.Equal(t, ClassRestricted, node.Classification)
}

func TestAutoCollectLineage(t *testing.T) {
	mgr, r := setupTest(t)
	createTestNodes(t, mgr)

	records := []AutoCollectRecord{
		{SourceID: "db-users", TargetID: "etl-user-report", EdgeType: EdgeTransform, Process: "spark-job", SQL: "SELECT * FROM users"},
	}
	body, _ := json.Marshal(records)
	req := httptest.NewRequest(http.MethodPost, "/datalineage/collect", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(1), resp["collected"])
}

func TestStats(t *testing.T) {
	mgr, r := setupTest(t)
	createTestNodes(t, mgr)
	require.NoError(t, mgr.CreateEdge(&LineageEdge{ID: "e1", SourceID: "db-users", TargetID: "api-register", Type: EdgeDirect}))

	req := httptest.NewRequest(http.MethodGet, "/datalineage/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(4), data["total_nodes"])
	assert.Equal(t, float64(1), data["total_edges"])
}

func TestDeleteNodeCascadesEdges(t *testing.T) {
	mgr, _ := setupTest(t)
	require.NoError(t, mgr.CreateNode(&DataNode{ID: "src", Name: "Source", Type: SourceDatabase}))
	require.NoError(t, mgr.CreateNode(&DataNode{ID: "dst", Name: "Target", Type: SourceAPI}))
	require.NoError(t, mgr.CreateEdge(&LineageEdge{ID: "e1", SourceID: "src", TargetID: "dst", Type: EdgeDirect}))

	require.NoError(t, mgr.DeleteNode("src"))
	_, err := mgr.GetEdge("e1")
	assert.ErrorIs(t, err, ErrEdgeNotFound)
}
