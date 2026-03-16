package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestRBACHandlers(t *testing.T) (*RBACHandlers, *RBACManager) {
	mgr := NewRBACManager(nil)
	return NewRBACHandlers(mgr), mgr
}

func setupTestRBACRouter() *gin.Engine {
	router := gin.New()
	return router
}

// ========== 响应函数测试 ==========

func TestSuccess_RBAC(t *testing.T) {
	data := map[string]string{"key": "value"}
	resp := success(data)

	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
	if resp.Message != "success" {
		t.Errorf("expected message 'success', got '%s'", resp.Message)
	}
}

func TestAPIError(t *testing.T) {
	resp := apiError(404, "not found")

	if resp.Code != 404 {
		t.Errorf("expected code 404, got %d", resp.Code)
	}
	if resp.Message != "not found" {
		t.Errorf("expected message 'not found', got '%s'", resp.Message)
	}
}

// ========== RBACHandlers 构造函数测试 ==========

func TestNewRBACHandlers(t *testing.T) {
	handlers, _ := setupTestRBACHandlers(t)

	if handlers == nil {
		t.Fatal("NewRBACHandlers returned nil")
	}
	if handlers.manager == nil {
		t.Error("RBACHandlers.manager is nil")
	}
}

// ========== 路由注册测试 ==========

func TestRBACHandlers_RegisterRoutes(t *testing.T) {
	handlers, _ := setupTestRBACHandlers(t)

	router := setupTestRBACRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	routes := router.Routes()
	if len(routes) == 0 {
		t.Error("No routes registered")
	}

	// 验证关键路由存在
	expectedRoutes := []string{
		"/api/rbac/roles",
		"/api/rbac/check",
	}

	registeredPaths := make(map[string]bool)
	for _, r := range routes {
		registeredPaths[r.Path] = true
	}

	for _, expected := range expectedRoutes {
		if !registeredPaths[expected] {
			t.Errorf("Route %s not found", expected)
		}
	}
}

// ========== getRoles 测试 ==========

func TestGetRoles(t *testing.T) {
	handlers, _ := setupTestRBACHandlers(t)

	router := setupTestRBACRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/rbac/roles", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("expected response code 0, got %d", resp.Code)
	}
}

// ========== createRole 测试 ==========

func TestCreateRole_Valid(t *testing.T) {
	handlers, _ := setupTestRBACHandlers(t)

	router := setupTestRBACRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	reqBody := CreateRoleRequest{
		Name:        "test-role",
		Description: "Test role",
		Permissions: []Permission{PermissionRead},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/rbac/roles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestCreateRole_Duplicate(t *testing.T) {
	handlers, _ := setupTestRBACHandlers(t)

	router := setupTestRBACRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	reqBody := CreateRoleRequest{
		Name:        "admin",
		Description: "Duplicate admin",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/rbac/roles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", w.Code)
	}
}

func TestCreateRole_InvalidJSON(t *testing.T) {
	handlers, _ := setupTestRBACHandlers(t)

	router := setupTestRBACRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("POST", "/api/rbac/roles", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// ========== getRole 测试 ==========

func TestGetRole_Exists(t *testing.T) {
	handlers, _ := setupTestRBACHandlers(t)

	router := setupTestRBACRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/rbac/roles/admin", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestGetRole_NotExists(t *testing.T) {
	handlers, _ := setupTestRBACHandlers(t)

	router := setupTestRBACRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/rbac/roles/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// ========== deleteRole 测试 ==========

func TestDeleteRole_Exists(t *testing.T) {
	handlers, mgr := setupTestRBACHandlers(t)

	// 创建一个自定义角色
	_ = mgr.AddRole("test-role", "Test role", []Permission{PermissionRead}, nil)

	router := setupTestRBACRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("DELETE", "/api/rbac/roles/test-role", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestDeleteRole_NotExists(t *testing.T) {
	handlers, _ := setupTestRBACHandlers(t)

	router := setupTestRBACRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("DELETE", "/api/rbac/roles/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// ========== checkPermission 测试 ==========

func TestCheckPermission_Granted(t *testing.T) {
	handlers, mgr := setupTestRBACHandlers(t)

	// 分配角色给用户
	_ = mgr.AssignRoleToUser("test-user", RoleAdmin)

	router := setupTestRBACRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	reqBody := CheckPermissionRequest{
		UserID:     "test-user",
		Permission: string(PermissionAdmin),
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/rbac/check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestCheckPermission_InvalidJSON(t *testing.T) {
	handlers, _ := setupTestRBACHandlers(t)

	router := setupTestRBACRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("POST", "/api/rbac/check", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// ========== getUserRoles 测试 ==========

func TestGetUserRoles(t *testing.T) {
	handlers, mgr := setupTestRBACHandlers(t)

	// 分配角色给用户
	_ = mgr.AssignRoleToUser("test-user", RoleAdmin)

	router := setupTestRBACRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/rbac/users/test-user/roles", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// ========== assignUserRole 测试 ==========

func TestAssignUserRole_Valid(t *testing.T) {
	handlers, _ := setupTestRBACHandlers(t)

	router := setupTestRBACRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	reqBody := AssignRoleRequest{Role: "admin"}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/rbac/users/test-user/roles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestAssignUserRole_InvalidRole(t *testing.T) {
	handlers, _ := setupTestRBACHandlers(t)

	router := setupTestRBACRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	reqBody := AssignRoleRequest{Role: "nonexistent-role"}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/rbac/users/test-user/roles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// ========== removeUserRole 测试 ==========

func TestRemoveUserRole_Valid(t *testing.T) {
	handlers, mgr := setupTestRBACHandlers(t)

	// 先分配角色
	_ = mgr.AssignRoleToUser("test-user", RoleAdmin)

	router := setupTestRBACRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("DELETE", "/api/rbac/users/test-user/roles/admin", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// ========== getUserPermissions 测试 ==========

func TestGetUserPermissions(t *testing.T) {
	handlers, mgr := setupTestRBACHandlers(t)

	// 分配角色给用户
	_ = mgr.AssignRoleToUser("test-user", RoleAdmin)

	router := setupTestRBACRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/rbac/users/test-user/permissions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// ========== getGroupRoles 测试 ==========

func TestGetGroupRoles(t *testing.T) {
	handlers, _ := setupTestRBACHandlers(t)

	router := setupTestRBACRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/rbac/groups/test-group/roles", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// ========== assignGroupRole 测试 ==========

func TestAssignGroupRole_Valid(t *testing.T) {
	handlers, _ := setupTestRBACHandlers(t)

	router := setupTestRBACRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	reqBody := AssignRoleRequest{Role: "user"}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/rbac/groups/test-group/roles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// ========== removeGroupRole 测试 ==========

func TestRemoveGroupRole_Valid(t *testing.T) {
	handlers, mgr := setupTestRBACHandlers(t)

	// 先分配角色
	_ = mgr.AssignRoleToGroup("test-group", RoleUser)

	router := setupTestRBACRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("DELETE", "/api/rbac/groups/test-group/roles/user", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// ========== getResourceACL 测试 ==========

func TestGetResourceACL_NotFound(t *testing.T) {
	handlers, _ := setupTestRBACHandlers(t)

	router := setupTestRBACRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("GET", "/api/rbac/resources/test-resource/acl", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// ========== setResourceACL 测试 ==========

func TestSetResourceACL_Valid(t *testing.T) {
	handlers, _ := setupTestRBACHandlers(t)

	router := setupTestRBACRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	reqBody := SetResourceACLRequest{
		ResourceID:   "test-resource",
		ResourceType: "file",
		OwnerID:      "test-user",
		Permissions:  []Permission{PermissionRead, PermissionWrite},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("PUT", "/api/rbac/resources/test-resource/acl", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestSetResourceACL_InvalidJSON(t *testing.T) {
	handlers, _ := setupTestRBACHandlers(t)

	router := setupTestRBACRouter()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	req := httptest.NewRequest("PUT", "/api/rbac/resources/test-resource/acl", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// ========== CreateRoleRequest 测试 ==========

func TestCreateRoleRequest_Binding(t *testing.T) {
	req := CreateRoleRequest{
		Name:        "test-role",
		Description: "Test description",
		Permissions: []Permission{PermissionRead, PermissionWrite},
		Inherits:    []string{"user"},
	}

	if req.Name != "test-role" {
		t.Errorf("Name mismatch: %s", req.Name)
	}
	if len(req.Permissions) != 2 {
		t.Errorf("Expected 2 permissions, got %d", len(req.Permissions))
	}
}

// ========== AssignRoleRequest 测试 ==========

func TestAssignRoleRequest_Binding(t *testing.T) {
	req := AssignRoleRequest{Role: "admin"}

	if req.Role != "admin" {
		t.Errorf("Role mismatch: %s", req.Role)
	}
}

// ========== CheckPermissionRequest 测试 ==========

func TestCheckPermissionRequest_Binding(t *testing.T) {
	req := CheckPermissionRequest{
		UserID:     "test-user",
		Permission: "read",
		ResourceID: "test-resource",
	}

	if req.UserID != "test-user" {
		t.Errorf("UserID mismatch: %s", req.UserID)
	}
}

// ========== SetResourceACLRequest 测试 ==========

func TestSetResourceACLRequest_Binding(t *testing.T) {
	req := SetResourceACLRequest{
		ResourceID:   "test-resource",
		ResourceType: "file",
		OwnerID:      "test-user",
		Permissions:  []Permission{PermissionRead},
	}

	if req.ResourceID != "test-resource" {
		t.Errorf("ResourceID mismatch: %s", req.ResourceID)
	}
}