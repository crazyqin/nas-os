package federatednas

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewFederation(t *testing.T) {
	f := NewFederation()
	if f == nil {
		t.Fatal("NewFederation returned nil")
	}
	if f.nodes == nil {
		t.Error("nodes map not initialized")
	}
	if f.syncJobs == nil {
		t.Error("syncJobs map not initialized")
	}
}

func TestRegisterNode(t *testing.T) {
	f := NewFederation()

	node := &FederationNode{
		ID:       "node-1",
		Name:     "NAS-1",
		Address:  "192.168.1.100",
		Port:     8080,
		Capacity: 1024 * 1024 * 1024 * 10, // 10GB
	}

	// Test successful registration
	if err := f.RegisterNode(node); err != nil {
		t.Fatalf("RegisterNode failed: %v", err)
	}

	if node.Status != NodeStatusOnline {
		t.Errorf("Expected status %s, got %s", NodeStatusOnline, node.Status)
	}

	if node.RegisteredAt.IsZero() {
		t.Error("RegisteredAt not set")
	}

	// Test duplicate registration
	if err := f.RegisterNode(node); err != ErrNodeAlreadyExists {
		t.Errorf("Expected ErrNodeAlreadyExists, got %v", err)
	}

	// Test empty ID
	invalidNode := &FederationNode{ID: ""}
	if err := f.RegisterNode(invalidNode); err != ErrInvalidNodeID {
		t.Errorf("Expected ErrInvalidNodeID, got %v", err)
	}
}

func TestGetNode(t *testing.T) {
	f := NewFederation()

	node := &FederationNode{
		ID:   "node-1",
		Name: "NAS-1",
	}
	f.RegisterNode(node)

	// Test get existing node
	got, err := f.GetNode("node-1")
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}
	if got.ID != "node-1" {
		t.Errorf("Expected ID node-1, got %s", got.ID)
	}

	// Test get non-existing node
	_, err = f.GetNode("non-existing")
	if err != ErrNodeNotFound {
		t.Errorf("Expected ErrNodeNotFound, got %v", err)
	}
}

func TestListNodes(t *testing.T) {
	f := NewFederation()

	f.RegisterNode(&FederationNode{ID: "node-1", Name: "NAS-1"})
	f.RegisterNode(&FederationNode{ID: "node-2", Name: "NAS-2"})

	nodes := f.ListNodes()
	if len(nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(nodes))
	}
}

func TestRemoveNode(t *testing.T) {
	f := NewFederation()

	f.RegisterNode(&FederationNode{ID: "node-1", Name: "NAS-1"})

	// Test remove existing node
	if err := f.RemoveNode("node-1"); err != nil {
		t.Fatalf("RemoveNode failed: %v", err)
	}

	if len(f.ListNodes()) != 0 {
		t.Error("Node not removed")
	}

	// Test remove non-existing node
	if err := f.RemoveNode("non-existing"); err != ErrNodeNotFound {
		t.Errorf("Expected ErrNodeNotFound, got %v", err)
	}
}

func TestStartSync(t *testing.T) {
	f := NewFederation()

	f.RegisterNode(&FederationNode{ID: "node-1", Name: "NAS-1", Address: "192.168.1.100", Port: 8080})
	f.RegisterNode(&FederationNode{ID: "node-2", Name: "NAS-2", Address: "192.168.1.101", Port: 8080})

	// Test successful sync
	job, err := f.StartSync("node-1", "node-2", true)
	if err != nil {
		t.Fatalf("StartSync failed: %v", err)
	}

	if job.SourceNodeID != "node-1" {
		t.Errorf("Expected source node-1, got %s", job.SourceNodeID)
	}
	if job.TargetNodeID != "node-2" {
		t.Errorf("Expected target node-2, got %s", job.TargetNodeID)
	}
	if !job.Incremental {
		t.Error("Expected incremental sync")
	}
	if job.Status != "running" {
		t.Errorf("Expected status running, got %s", job.Status)
	}

	// Wait for sync to complete
	time.Sleep(2 * time.Second)

	// Test sync with non-existing node
	_, err = f.StartSync("node-1", "non-existing", false)
	if err == nil {
		t.Error("Expected error for non-existing target node")
	}

	// Test sync with non-existing source
	_, err = f.StartSync("non-existing", "node-1", false)
	if err == nil {
		t.Error("Expected error for non-existing source node")
	}
}

func TestGetSyncJob(t *testing.T) {
	f := NewFederation()

	f.RegisterNode(&FederationNode{ID: "node-1", Name: "NAS-1", Address: "192.168.1.100", Port: 8080})
	f.RegisterNode(&FederationNode{ID: "node-2", Name: "NAS-2", Address: "192.168.1.101", Port: 8080})

	job, _ := f.StartSync("node-1", "node-2", false)

	got, err := f.GetSyncJob(job.ID)
	if err != nil {
		t.Fatalf("GetSyncJob failed: %v", err)
	}
	if got.ID != job.ID {
		t.Errorf("Expected job ID %s, got %s", job.ID, got.ID)
	}

	// Test non-existing job
	_, err = f.GetSyncJob("non-existing")
	if err == nil {
		t.Error("Expected error for non-existing job")
	}
}

func TestListSyncJobs(t *testing.T) {
	f := NewFederation()

	f.RegisterNode(&FederationNode{ID: "node-1", Name: "NAS-1", Address: "192.168.1.100", Port: 8080})
	f.RegisterNode(&FederationNode{ID: "node-2", Name: "NAS-2", Address: "192.168.1.101", Port: 8080})

	f.StartSync("node-1", "node-2", false)
	time.Sleep(100 * time.Millisecond)
	f.StartSync("node-2", "node-1", false)

	jobs := f.ListSyncJobs()
	if len(jobs) != 2 {
		t.Errorf("Expected 2 jobs, got %d", len(jobs))
	}
}

func TestConflictResolution(t *testing.T) {
	f := NewFederation()

	conflict := &ConflictRecord{
		SyncJobID:    "sync-1",
		FilePath:     "/data/test.txt",
		SourceNodeID: "node-1",
		TargetNodeID: "node-2",
	}
	f.AddConflict(conflict)

	if conflict.ID == "" {
		t.Error("Conflict ID not generated")
	}

	// Test resolve conflict
	if err := f.ResolveConflict(conflict.ID, "use-source", "admin"); err != nil {
		t.Fatalf("ResolveConflict failed: %v", err)
	}

	got, _ := f.GetConflict(conflict.ID)
	if got.Resolution != "use-source" {
		t.Errorf("Expected resolution use-source, got %s", got.Resolution)
	}
	if got.ResolvedBy != "admin" {
		t.Errorf("Expected resolved by admin, got %s", got.ResolvedBy)
	}
	if got.ResolvedAt == nil {
		t.Error("ResolvedAt not set")
	}

	// Test resolve non-existing conflict
	if err := f.ResolveConflict("non-existing", "use-source", "admin"); err == nil {
		t.Error("Expected error for non-existing conflict")
	}
}

func TestNodeHealth(t *testing.T) {
	f := NewFederation()

	f.RegisterNode(&FederationNode{ID: "node-1", Name: "NAS-1"})

	health := &NodeHealth{
		CPUUsage:    50.5,
		MemoryUsage: 75.0,
		DiskUsage:   60.0,
		LatencyMs:   15.5,
	}

	// Test update health
	if err := f.UpdateNodeHealth("node-1", health); err != nil {
		t.Fatalf("UpdateNodeHealth failed: %v", err)
	}

	got, err := f.GetNodeStatus("node-1")
	if err != nil {
		t.Fatalf("GetNodeStatus failed: %v", err)
	}
	if got.CPUUsage != 50.5 {
		t.Errorf("Expected CPU 50.5, got %f", got.CPUUsage)
	}

	// Test health for non-existing node
	_, err = f.GetNodeStatus("non-existing")
	if err != ErrNodeNotFound {
		t.Errorf("Expected ErrNodeNotFound, got %v", err)
	}
}

func TestPropagateChange(t *testing.T) {
	f := NewFederation()

	f.RegisterNode(&FederationNode{ID: "node-1", Name: "NAS-1"})
	f.RegisterNode(&FederationNode{ID: "node-2", Name: "NAS-2"})

	// Test propagate change
	if err := f.PropagateChange("node-1", "/data/test.txt", false); err != nil {
		t.Fatalf("PropagateChange failed: %v", err)
	}

	// Test propagate delete
	if err := f.PropagateChange("node-1", "/data/old.txt", true); err != nil {
		t.Fatalf("PropagateChange delete failed: %v", err)
	}

	// Test propagate from non-existing node
	if err := f.PropagateChange("non-existing", "/data/test.txt", false); err != ErrNodeNotFound {
		t.Errorf("Expected ErrNodeNotFound, got %v", err)
	}
}

func TestFederationStatus(t *testing.T) {
	f := NewFederation()

	f.RegisterNode(&FederationNode{ID: "node-1", Name: "NAS-1", Capacity: 1024 * 1024 * 1024 * 10})
	f.RegisterNode(&FederationNode{ID: "node-2", Name: "NAS-2", Capacity: 1024 * 1024 * 1024 * 20})
	f.RegisterNode(&FederationNode{ID: "node-3", Name: "NAS-3", Capacity: 1024 * 1024 * 1024 * 30})

	// Set one node offline
	node2, _ := f.GetNode("node-2")
	node2.UpdateStatus(NodeStatusOffline)

	status := f.GetFederationStatus()
	if status.TotalNodes != 3 {
		t.Errorf("Expected 3 total nodes, got %d", status.TotalNodes)
	}
	if status.OnlineNodes != 2 {
		t.Errorf("Expected 2 online nodes, got %d", status.OnlineNodes)
	}
	if status.OfflineNodes != 1 {
		t.Errorf("Expected 1 offline node, got %d", status.OfflineNodes)
	}
	if status.TotalCapacity != 1024*1024*1024*60 {
		t.Errorf("Expected total capacity 60GB, got %d", status.TotalCapacity)
	}
}

func TestPolicy(t *testing.T) {
	f := NewFederation()

	policy := &FederationPolicy{
		Name:               "default",
		SyncInterval:       time.Hour,
		ConflictResolution: ConflictResolutionAuto,
		AutoResolve:        true,
		RetryAttempts:      3,
	}

	f.AddPolicy(policy)
	if policy.ID == "" {
		t.Error("Policy ID not generated")
	}

	// Test get policy
	got, err := f.GetPolicy(policy.ID)
	if err != nil {
		t.Fatalf("GetPolicy failed: %v", err)
	}
	if got.Name != "default" {
		t.Errorf("Expected name default, got %s", got.Name)
	}

	// Test list policies
	policies := f.ListPolicies()
	if len(policies) != 1 {
		t.Errorf("Expected 1 policy, got %d", len(policies))
	}

	// Test non-existing policy
	_, err = f.GetPolicy("non-existing")
	if err == nil {
		t.Error("Expected error for non-existing policy")
	}
}

func TestDistributedNamespace(t *testing.T) {
	f := NewFederation()

	f.RegisterNode(&FederationNode{ID: "node-1", Name: "NAS-1"})
	f.RegisterNode(&FederationNode{ID: "node-2", Name: "NAS-2"})

	f.PropagateChange("node-1", "/data/file1.txt", false)
	f.PropagateChange("node-2", "/data/file2.txt", false)
	f.PropagateChange("node-1", "/data/subdir/file3.txt", false)

	ns := f.GetNamespace("/data")
	if len(ns) != 3 {
		t.Errorf("Expected 3 namespace entries, got %d", len(ns))
	}
}

func TestSyncJobProgress(t *testing.T) {
	job := &SyncJob{
		TotalFiles:  100,
		SyncedFiles: 50,
	}

	progress := job.Progress()
	if progress != 50.0 {
		t.Errorf("Expected progress 50.0, got %f", progress)
	}

	// Test zero total
	job2 := &SyncJob{TotalFiles: 0}
	if job2.Progress() != 0 {
		t.Error("Expected 0 progress for zero total files")
	}
}

func TestFederationNodeUpdateStatus(t *testing.T) {
	node := &FederationNode{
		ID:     "node-1",
		Status: NodeStatusOnline,
	}

	before := time.Now()
	node.UpdateStatus(NodeStatusSyncing)

	if node.Status != NodeStatusSyncing {
		t.Errorf("Expected status syncing, got %s", node.Status)
	}
	if node.LastSeen.Before(before) {
		t.Error("LastSeen not updated")
	}
}

// HTTP Handler Tests

func TestHandlerRegisterNode(t *testing.T) {
	f := NewFederation()
	handler := NewHandler(f)

	node := &FederationNode{
		ID:      "node-1",
		Name:    "NAS-1",
		Address: "192.168.1.100",
		Port:    8080,
	}
	body, _ := json.Marshal(node)

	req := httptest.NewRequest(http.MethodPost, "/api/federation/nodes", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.registerNode(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}

	var got FederationNode
	json.NewDecoder(w.Body).Decode(&got)
	if got.ID != "node-1" {
		t.Errorf("Expected ID node-1, got %s", got.ID)
	}
}

func TestHandlerListNodes(t *testing.T) {
	f := NewFederation()
	handler := NewHandler(f)

	f.RegisterNode(&FederationNode{ID: "node-1", Name: "NAS-1"})
	f.RegisterNode(&FederationNode{ID: "node-2", Name: "NAS-2"})

	req := httptest.NewRequest(http.MethodGet, "/api/federation/nodes", nil)
	w := httptest.NewRecorder()

	handler.listNodes(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var nodes []FederationNode
	json.NewDecoder(w.Body).Decode(&nodes)
	if len(nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(nodes))
	}
}

func TestHandlerStartSync(t *testing.T) {
	f := NewFederation()
	handler := NewHandler(f)

	f.RegisterNode(&FederationNode{ID: "node-1", Name: "NAS-1", Address: "192.168.1.100", Port: 8080})
	f.RegisterNode(&FederationNode{ID: "node-2", Name: "NAS-2", Address: "192.168.1.101", Port: 8080})

	reqBody := struct {
		SourceNodeID string `json:"source_node_id"`
		TargetNodeID string `json:"target_node_id"`
		Incremental  bool   `json:"incremental"`
	}{
		SourceNodeID: "node-1",
		TargetNodeID: "node-2",
		Incremental:  true,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/federation/sync", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.startSync(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}
}

func TestHandlerStatus(t *testing.T) {
	f := NewFederation()
	handler := NewHandler(f)

	f.RegisterNode(&FederationNode{ID: "node-1", Name: "NAS-1", Capacity: 1024 * 1024 * 1024 * 10})

	req := httptest.NewRequest(http.MethodGet, "/api/federation/status", nil)
	w := httptest.NewRecorder()

	handler.handleStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var status FederationStatus
	json.NewDecoder(w.Body).Decode(&status)
	if status.TotalNodes != 1 {
		t.Errorf("Expected 1 total node, got %d", status.TotalNodes)
	}
}

func TestHandlerMethodNotAllowed(t *testing.T) {
	f := NewFederation()
	handler := NewHandler(f)

	req := httptest.NewRequest(http.MethodPut, "/api/federation/nodes", nil)
	w := httptest.NewRecorder()

	handler.handleNodes(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandlerGetNonExistingNode(t *testing.T) {
	f := NewFederation()
	handler := NewHandler(f)

	req := httptest.NewRequest(http.MethodGet, "/api/federation/nodes/non-existing", nil)
	w := httptest.NewRecorder()

	handler.getNode(w, req, "non-existing")

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandlerConflicts(t *testing.T) {
	f := NewFederation()
	handler := NewHandler(f)

	conflict := &ConflictRecord{
		SyncJobID:    "sync-1",
		FilePath:     "/data/test.txt",
		SourceNodeID: "node-1",
		TargetNodeID: "node-2",
	}
	f.AddConflict(conflict)

	req := httptest.NewRequest(http.MethodGet, "/api/federation/conflicts", nil)
	w := httptest.NewRecorder()

	handler.handleConflicts(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var conflicts []ConflictRecord
	json.NewDecoder(w.Body).Decode(&conflicts)
	if len(conflicts) != 1 {
		t.Errorf("Expected 1 conflict, got %d", len(conflicts))
	}
}

func TestHandlerPolicies(t *testing.T) {
	f := NewFederation()
	handler := NewHandler(f)

	policy := FederationPolicy{
		Name:               "default",
		SyncInterval:       time.Hour,
		ConflictResolution: ConflictResolutionAuto,
	}
	body, _ := json.Marshal(policy)

	req := httptest.NewRequest(http.MethodPost, "/api/federation/policies", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.handlePolicies(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}
}

func TestHandlerHealth(t *testing.T) {
	f := NewFederation()
	handler := NewHandler(f)

	f.RegisterNode(&FederationNode{ID: "node-1", Name: "NAS-1"})
	f.UpdateNodeHealth("node-1", &NodeHealth{CPUUsage: 50.0})

	req := httptest.NewRequest(http.MethodGet, "/api/federation/health/node-1", nil)
	w := httptest.NewRecorder()

	handler.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlerNamespace(t *testing.T) {
	f := NewFederation()
	handler := NewHandler(f)

	f.RegisterNode(&FederationNode{ID: "node-1", Name: "NAS-1"})
	f.PropagateChange("node-1", "/data/test.txt", false)

	req := httptest.NewRequest(http.MethodGet, "/api/federation/namespace?path=/data", nil)
	w := httptest.NewRecorder()

	handler.handleNamespace(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlerPropagate(t *testing.T) {
	f := NewFederation()
	handler := NewHandler(f)

	f.RegisterNode(&FederationNode{ID: "node-1", Name: "NAS-1"})

	reqBody := struct {
		NodeID   string `json:"node_id"`
		FilePath string `json:"file_path"`
		IsDelete bool   `json:"is_delete"`
	}{
		NodeID:   "node-1",
		FilePath: "/data/test.txt",
		IsDelete: false,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/federation/propagate", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.handlePropagate(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestNodeStatusConstants(t *testing.T) {
	if NodeStatusOnline != "online" {
		t.Errorf("Expected NodeStatusOnline to be 'online', got %s", NodeStatusOnline)
	}
	if NodeStatusOffline != "offline" {
		t.Errorf("Expected NodeStatusOffline to be 'offline', got %s", NodeStatusOffline)
	}
	if NodeStatusSyncing != "syncing" {
		t.Errorf("Expected NodeStatusSyncing to be 'syncing', got %s", NodeStatusSyncing)
	}
	if NodeStatusError != "error" {
		t.Errorf("Expected NodeStatusError to be 'error', got %s", NodeStatusError)
	}
}

func TestConflictResolutionConstants(t *testing.T) {
	if ConflictResolutionAuto != "auto" {
		t.Errorf("Expected ConflictResolutionAuto to be 'auto', got %s", ConflictResolutionAuto)
	}
	if ConflictResolutionManual != "manual" {
		t.Errorf("Expected ConflictResolutionManual to be 'manual', got %s", ConflictResolutionManual)
	}
	if ConflictResolutionNewest != "newest" {
		t.Errorf("Expected ConflictResolutionNewest to be 'newest', got %s", ConflictResolutionNewest)
	}
}
