package iscsi

import (
	"context"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestHandlers(t *testing.T) (*Handlers, *Manager, string) {
	gin.SetMode(gin.TestMode)
	mgr, tmpDir := setupTestManager(t)
	handlers := NewHandlers(mgr)
	return handlers, mgr, tmpDir
}

func makeRequest(method, path string, body interface{}) (*http.Request, error) {
	var reqBody bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&reqBody).Encode(body); err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(context.Background(), method, path, &reqBody)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

// ========== Target Handler Tests ==========

func TestListTargetsHandler(t *testing.T) {
	h, mgr, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	// Create test targets
	mgr.CreateTarget(TargetInput{Name: "target1"})
	mgr.CreateTarget(TargetInput{Name: "target2"})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := makeRequest("GET", "/iscsi/targets", nil)
	c.Request = req

	h.listTargets(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 {
		t.Errorf("Expected code 0, got %d", resp.Code)
	}
}

func TestCreateTargetHandler(t *testing.T) {
	h, _, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := makeRequest("POST", "/iscsi/targets", TargetInput{Name: "new-target"})
	c.Request = req

	h.createTarget(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetTargetHandler(t *testing.T) {
	h, mgr, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	target, _ := mgr.CreateTarget(TargetInput{Name: "get-target"})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: target.ID}}
	req, _ := makeRequest("GET", "/iscsi/targets/"+target.ID, nil)
	c.Request = req

	h.getTarget(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetTargetHandlerNotFound(t *testing.T) {
	h, _, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "non-existent"}}
	req, _ := makeRequest("GET", "/iscsi/targets/non-existent", nil)
	c.Request = req

	h.getTarget(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestUpdateTargetHandler(t *testing.T) {
	h, mgr, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	target, _ := mgr.CreateTarget(TargetInput{Name: "update-target"})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: target.ID}}
	req, _ := makeRequest("PUT", "/iscsi/targets/"+target.ID, TargetInput{Name: "update-target", Alias: "Updated"})
	c.Request = req

	h.updateTarget(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestDeleteTargetHandler(t *testing.T) {
	h, mgr, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	target, _ := mgr.CreateTarget(TargetInput{Name: "delete-target"})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: target.ID}}
	req, _ := makeRequest("DELETE", "/iscsi/targets/"+target.ID, nil)
	c.Request = req

	h.deleteTarget(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// ========== LUN Handler Tests ==========

func TestAddLUNHandler(t *testing.T) {
	h, mgr, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	target, _ := mgr.CreateTarget(TargetInput{Name: "lun-target"})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: target.ID}}
	req, _ := makeRequest("POST", "/iscsi/targets/"+target.ID+"/luns", LUNInput{
		Name: "test-lun",
		Type: LUNTypeFile,
		Size: 1024 * 1024 * 100,
	})
	c.Request = req

	h.addLUN(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListLUNsHandler(t *testing.T) {
	h, mgr, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	target, _ := mgr.CreateTarget(TargetInput{Name: "list-lun-target"})
	mgr.AddLUN(target.ID, LUNInput{Name: "lun1", Type: LUNTypeFile, Size: 1024 * 1024 * 100})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: target.ID}}
	req, _ := makeRequest("GET", "/iscsi/targets/"+target.ID+"/luns", nil)
	c.Request = req

	h.listLUNs(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetLUNHandler(t *testing.T) {
	h, mgr, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	target, _ := mgr.CreateTarget(TargetInput{Name: "get-lun-target"})
	lun, _ := mgr.AddLUN(target.ID, LUNInput{Name: "get-lun", Type: LUNTypeFile, Size: 1024 * 1024 * 100})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: target.ID}, {Key: "lunId", Value: lun.ID}}
	req, _ := makeRequest("GET", "/iscsi/targets/"+target.ID+"/luns/"+lun.ID, nil)
	c.Request = req

	h.getLUN(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestRemoveLUNHandler(t *testing.T) {
	h, mgr, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	target, _ := mgr.CreateTarget(TargetInput{Name: "remove-lun-target"})
	lun, _ := mgr.AddLUN(target.ID, LUNInput{Name: "remove-lun", Type: LUNTypeFile, Size: 1024 * 1024 * 100})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: target.ID}, {Key: "lunId", Value: lun.ID}}
	req, _ := makeRequest("DELETE", "/iscsi/targets/"+target.ID+"/luns/"+lun.ID, nil)
	c.Request = req

	h.removeLUN(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestExpandLUNHandler(t *testing.T) {
	h, mgr, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	target, _ := mgr.CreateTarget(TargetInput{Name: "expand-lun-target"})
	lun, _ := mgr.AddLUN(target.ID, LUNInput{Name: "expand-lun", Type: LUNTypeFile, Size: 1024 * 1024 * 100})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: target.ID}, {Key: "lunId", Value: lun.ID}}
	req, _ := makeRequest("POST", "/iscsi/targets/"+target.ID+"/luns/"+lun.ID+"/expand", LUNExpandInput{Size: 1024 * 1024 * 200})
	c.Request = req

	h.expandLUN(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// ========== Snapshot Handler Tests ==========

func TestCreateLUNSnapshotHandler(t *testing.T) {
	h, mgr, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	target, _ := mgr.CreateTarget(TargetInput{Name: "snapshot-target"})
	lun, _ := mgr.AddLUN(target.ID, LUNInput{Name: "snapshot-lun", Type: LUNTypeFile, Size: 1024 * 1024 * 100})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: target.ID}, {Key: "lunId", Value: lun.ID}}
	req, _ := makeRequest("POST", "/iscsi/targets/"+target.ID+"/luns/"+lun.ID+"/snapshots", LUNSnapshotInput{Name: "test-snap"})
	c.Request = req

	h.createLUNSnapshot(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListLUNSnapshotsHandler(t *testing.T) {
	h, mgr, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	target, _ := mgr.CreateTarget(TargetInput{Name: "list-snap-target"})
	lun, _ := mgr.AddLUN(target.ID, LUNInput{Name: "list-snap-lun", Type: LUNTypeFile, Size: 1024 * 1024 * 100})
	mgr.CreateLUNSnapshot(target.ID, lun.ID, LUNSnapshotInput{Name: "snap1"})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: target.ID}, {Key: "lunId", Value: lun.ID}}
	req, _ := makeRequest("GET", "/iscsi/targets/"+target.ID+"/luns/"+lun.ID+"/snapshots", nil)
	c.Request = req

	h.listLUNSnapshots(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// ========== Service Handler Tests ==========

func TestGetServiceStatusHandler(t *testing.T) {
	h, _, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := makeRequest("GET", "/iscsi/status", nil)
	c.Request = req

	h.getServiceStatus(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestEnableTargetHandler(t *testing.T) {
	h, mgr, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	target, _ := mgr.CreateTarget(TargetInput{Name: "enable-target"})
	mgr.DisableTarget(target.ID)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: target.ID}}
	req, _ := makeRequest("POST", "/iscsi/targets/"+target.ID+"/enable", nil)
	c.Request = req

	h.enableTarget(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestDisableTargetHandler(t *testing.T) {
	h, mgr, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	target, _ := mgr.CreateTarget(TargetInput{Name: "disable-target"})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: target.ID}}
	req, _ := makeRequest("POST", "/iscsi/targets/"+target.ID+"/disable", nil)
	c.Request = req

	h.disableTarget(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetTargetStatusHandler(t *testing.T) {
	h, mgr, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	target, _ := mgr.CreateTarget(TargetInput{Name: "status-target"})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: target.ID}}
	req, _ := makeRequest("GET", "/iscsi/targets/"+target.ID+"/status", nil)
	c.Request = req

	h.getTargetStatus(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestDeleteLUNSnapshotHandler(t *testing.T) {
	h, mgr, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	target, _ := mgr.CreateTarget(TargetInput{Name: "del-snap-target"})
	lun, _ := mgr.AddLUN(target.ID, LUNInput{Name: "del-snap-lun", Type: LUNTypeFile, Size: 1024 * 1024 * 100})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: target.ID}, {Key: "lunId", Value: lun.ID}, {Key: "snapId", Value: "snap-id"}}
	req, _ := makeRequest("DELETE", "/iscsi/targets/"+target.ID+"/luns/"+lun.ID+"/snapshots/snap-id", nil)
	c.Request = req

	h.deleteLUNSnapshot(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// ========== Error Handler Tests ==========

func TestAddLUNHandlerNotFound(t *testing.T) {
	h, _, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "non-existent"}}
	req, _ := makeRequest("POST", "/iscsi/targets/non-existent/luns", LUNInput{Name: "test", Type: LUNTypeFile, Size: 1024 * 1024})
	c.Request = req

	h.addLUN(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestAddLUNHandlerInvalidInput(t *testing.T) {
	h, mgr, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	target, _ := mgr.CreateTarget(TargetInput{Name: "invalid-lun-target"})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: target.ID}}
	req, _ := makeRequest("POST", "/iscsi/targets/"+target.ID+"/luns", LUNInput{Name: "", Type: LUNTypeFile, Size: 1024})
	c.Request = req

	h.addLUN(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestExpandLUNHandlerInvalidSize(t *testing.T) {
	h, mgr, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	target, _ := mgr.CreateTarget(TargetInput{Name: "expand-invalid-target"})
	lun, _ := mgr.AddLUN(target.ID, LUNInput{Name: "expand-invalid-lun", Type: LUNTypeFile, Size: 1024 * 1024 * 100})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: target.ID}, {Key: "lunId", Value: lun.ID}}
	// Try to shrink (which is not allowed)
	req, _ := makeRequest("POST", "/iscsi/targets/"+target.ID+"/luns/"+lun.ID+"/expand", LUNExpandInput{Size: 1024 * 1024 * 50})
	c.Request = req

	h.expandLUN(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestCreateTargetHandlerDuplicateName(t *testing.T) {
	h, mgr, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	mgr.CreateTarget(TargetInput{Name: "dup-name"})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := makeRequest("POST", "/iscsi/targets", TargetInput{Name: "dup-name"})
	c.Request = req

	h.createTarget(c)

	if w.Code != http.StatusConflict {
		t.Errorf("Expected status 409, got %d", w.Code)
	}
}

func TestCreateTargetHandlerInvalidInput(t *testing.T) {
	h, _, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := makeRequest("POST", "/iscsi/targets", TargetInput{Name: ""}) // Empty name
	c.Request = req

	h.createTarget(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestRemoveLUNHandlerNotFound(t *testing.T) {
	h, mgr, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	target, _ := mgr.CreateTarget(TargetInput{Name: "remove-lun-notfound-target"})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: target.ID}, {Key: "lunId", Value: "non-existent"}}
	req, _ := makeRequest("DELETE", "/iscsi/targets/"+target.ID+"/luns/non-existent", nil)
	c.Request = req

	h.removeLUN(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestGetLUNHandlerNotFound(t *testing.T) {
	h, mgr, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	target, _ := mgr.CreateTarget(TargetInput{Name: "get-lun-notfound-target"})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: target.ID}, {Key: "lunId", Value: "non-existent"}}
	req, _ := makeRequest("GET", "/iscsi/targets/"+target.ID+"/luns/non-existent", nil)
	c.Request = req

	h.getLUN(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestListLUNsHandlerTargetNotFound(t *testing.T) {
	h, _, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "non-existent"}}
	req, _ := makeRequest("GET", "/iscsi/targets/non-existent/luns", nil)
	c.Request = req

	h.listLUNs(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestEnableTargetHandlerNotFound(t *testing.T) {
	h, _, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "non-existent"}}
	req, _ := makeRequest("POST", "/iscsi/targets/non-existent/enable", nil)
	c.Request = req

	h.enableTarget(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestDisableTargetHandlerNotFound(t *testing.T) {
	h, _, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "non-existent"}}
	req, _ := makeRequest("POST", "/iscsi/targets/non-existent/disable", nil)
	c.Request = req

	h.disableTarget(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestGetTargetStatusHandlerNotFound(t *testing.T) {
	h, _, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "non-existent"}}
	req, _ := makeRequest("GET", "/iscsi/targets/non-existent/status", nil)
	c.Request = req

	h.getTargetStatus(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestCreateLUNSnapshotHandlerNotFound(t *testing.T) {
	h, mgr, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	target, _ := mgr.CreateTarget(TargetInput{Name: "snap-notfound-target"})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: target.ID}, {Key: "lunId", Value: "non-existent"}}
	req, _ := makeRequest("POST", "/iscsi/targets/"+target.ID+"/luns/non-existent/snapshots", LUNSnapshotInput{Name: "test"})
	c.Request = req

	h.createLUNSnapshot(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestListLUNSnapshotsHandlerNotFound(t *testing.T) {
	h, mgr, tmpDir := setupTestHandlers(t)
	defer cleanupTestManager(tmpDir)

	target, _ := mgr.CreateTarget(TargetInput{Name: "list-snap-notfound-target"})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: target.ID}, {Key: "lunId", Value: "non-existent"}}
	req, _ := makeRequest("GET", "/iscsi/targets/"+target.ID+"/luns/non-existent/snapshots", nil)
	c.Request = req

	h.listLUNSnapshots(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}
