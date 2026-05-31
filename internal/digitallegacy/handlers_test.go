// Package digitallegacy 提供单元测试
package digitallegacy

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func setupTestRouter() (*gin.Engine, *Manager) {
	gin.SetMode(gin.TestMode)

	logger := zap.NewNop()
	config := GetDefaultConfig()
	// AES-256 需要 32 字节的密钥
	encryptionKey := []byte("test-encryption-key-32bytes!") // 28 bytes
	// 填充到 32 字节
	key := make([]byte, 32)
	copy(key, encryptionKey)

	manager := NewManager(logger, config, key)
	handlers := NewHandlers(manager)

	r := gin.New()
	api := r.Group("/api/v1")
	handlers.RegisterRoutes(api)

	return r, manager
}

func TestCreatePlan(t *testing.T) {
	r, _ := setupTestRouter()

	reqBody := LegacyPlanRequest{
		Name:        "Test Legacy Plan",
		Description: "Test description",
		TriggerType: TriggerInactivity,
		TriggerConditions: &TriggerConditions{
			InactivityDays: 30,
		},
		IsEncrypted: true,
	}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/api/v1/legacy/plans", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "Test Legacy Plan", data["name"])
	assert.Equal(t, "draft", data["status"])
}

func TestGetPlan(t *testing.T) {
	r, manager := setupTestRouter()

	// 创建一个计划
	plan, _ := manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Test Plan",
		Description: "Test description",
		TriggerType: TriggerManual,
		IsEncrypted: false,
	}, "user-001")

	req, _ := http.NewRequest("GET", "/api/v1/legacy/plans/"+plan.ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "Test Plan", data["name"])
}

func TestUpdatePlan(t *testing.T) {
	r, manager := setupTestRouter()

	// 创建一个计划
	plan, _ := manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Original Plan",
		Description: "Original description",
		TriggerType: TriggerManual,
		IsEncrypted: false,
	}, "user-001")

	reqBody := LegacyPlanRequest{
		Name:        "Updated Plan",
		Description: "Updated description",
		TriggerType: TriggerInactivity,
		TriggerConditions: &TriggerConditions{
			InactivityDays: 60,
		},
		IsEncrypted: true,
	}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("PUT", "/api/v1/legacy/plans/"+plan.ID, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "Updated Plan", data["name"])
}

func TestDeletePlan(t *testing.T) {
	r, manager := setupTestRouter()

	// 创建一个计划
	plan, _ := manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Plan to Delete",
		TriggerType: TriggerManual,
	}, "user-001")

	req, _ := http.NewRequest("DELETE", "/api/v1/legacy/plans/"+plan.ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证计划已删除
	req, _ = http.NewRequest("GET", "/api/v1/legacy/plans/"+plan.ID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestActivatePlan(t *testing.T) {
	r, manager := setupTestRouter()

	// 创建一个计划
	plan, _ := manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Plan to Activate",
		TriggerType: TriggerManual,
	}, "user-001")

	// 添加受益人
	manager.AddBeneficiary(plan.ID, &BeneficiaryRequest{
		Name:              "Beneficiary 1",
		Email:             "ben1@example.com",
		Role:              RoleBeneficiary,
		AllocationPercent: 100,
		AccessLevel:       AccessFull,
	})

	// 添加资产
	manager.AddAsset(plan.ID, &AssetRequest{
		Name: "Test Asset",
		Type: AssetTypeAccount,
		Data: "sensitive-data",
	})

	req, _ := http.NewRequest("POST", "/api/v1/legacy/plans/"+plan.ID+"/activate", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAddBeneficiary(t *testing.T) {
	r, manager := setupTestRouter()

	// 创建一个计划
	plan, _ := manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Plan with Beneficiary",
		TriggerType: TriggerManual,
	}, "user-001")

	reqBody := BeneficiaryRequest{
		Name:              "John Doe",
		Email:             "john@example.com",
		Phone:             "+1234567890",
		Relationship:      "Son",
		Role:              RoleBeneficiary,
		AllocationPercent: 50,
		AccessLevel:       AccessFull,
	}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/api/v1/legacy/plans/"+plan.ID+"/beneficiaries", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "John Doe", data["name"])
}

func TestAddAsset(t *testing.T) {
	r, manager := setupTestRouter()

	// 创建一个计划
	plan, _ := manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Plan with Assets",
		TriggerType: TriggerManual,
		IsEncrypted: true,
	}, "user-001")

	reqBody := AssetRequest{
		Name:        "Bank Account",
		Type:        AssetTypeAccount,
		Description: "Main bank account",
		Data:        "account-number-and-password",
		Notes:       "Primary savings account",
	}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/api/v1/legacy/plans/"+plan.ID+"/assets", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "Bank Account", data["name"])
	assert.Equal(t, true, data["is_encrypted"])
}

func TestAddEmergencyContact(t *testing.T) {
	r, manager := setupTestRouter()

	// 创建一个计划
	plan, _ := manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Plan with Emergency Contacts",
		TriggerType: TriggerManual,
	}, "user-001")

	reqBody := EmergencyContactRequest{
		Name:            "Emergency Contact",
		Email:           "emergency@example.com",
		Phone:           "+0987654321",
		Relationship:    "Spouse",
		Role:            RoleEmergency,
		IsPrimary:       true,
		CanTriggerPlan:  true,
		NotifyOnTrigger: true,
	}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/api/v1/legacy/plans/"+plan.ID+"/contacts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "Emergency Contact", data["name"])
}

func TestSetWillDocument(t *testing.T) {
	r, manager := setupTestRouter()

	// 创建一个计划
	plan, _ := manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Plan with Will",
		TriggerType: TriggerManual,
		IsEncrypted: true,
	}, "user-001")

	reqBody := WillDocumentRequest{
		Title:   "My Last Will",
		Content: "I hereby bequeath all my digital assets to my family...",
	}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/api/v1/legacy/plans/"+plan.ID+"/will", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "My Last Will", data["title"])
	assert.Equal(t, true, data["is_encrypted"])
}

func TestGetWillDocument(t *testing.T) {
	r, manager := setupTestRouter()

	// 创建一个计划
	plan, _ := manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Plan with Will",
		TriggerType: TriggerManual,
		IsEncrypted: true,
	}, "user-001")

	// 设置遗嘱文档
	manager.SetWillDocument(plan.ID, &WillDocumentRequest{
		Title:   "Test Will",
		Content: "Test content",
	})

	// 获取加密版本
	req, _ := http.NewRequest("GET", "/api/v1/legacy/plans/"+plan.ID+"/will", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 获取解密版本
	req, _ = http.NewRequest("GET", "/api/v1/legacy/plans/"+plan.ID+"/will?decrypt=true", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTriggerPlan(t *testing.T) {
	r, manager := setupTestRouter()

	// 创建一个计划
	plan, _ := manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Plan to Trigger",
		TriggerType: TriggerManual,
		TriggerConditions: &TriggerConditions{
			EmergencyCode: "test-code-123",
		},
	}, "user-001")

	// 添加受益人
	manager.AddBeneficiary(plan.ID, &BeneficiaryRequest{
		Name:              "Beneficiary",
		Role:              RoleBeneficiary,
		AllocationPercent: 100,
		AccessLevel:       AccessFull,
	})

	// 添加资产
	manager.AddAsset(plan.ID, &AssetRequest{
		Name: "Asset",
		Type: AssetTypeAccount,
		Data: "data",
	})

	// 激活计划
	manager.ActivatePlan(plan.ID)

	// 触发计划
	reqBody := TriggerRequest{
		PlanID:        plan.ID,
		EmergencyCode: "test-code-123",
	}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/api/v1/legacy/plans/"+plan.ID+"/trigger", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCheckInactivity(t *testing.T) {
	r, _ := setupTestRouter()

	req, _ := http.NewRequest("POST", "/api/v1/legacy/check-inactivity", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

func TestGetConfig(t *testing.T) {
	r, _ := setupTestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/legacy/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, float64(365), data["inactivity_days"])
}

func TestUpdateConfig(t *testing.T) {
	r, _ := setupTestRouter()

	reqBody := DefaultLegacyConfig{
		InactivityDays:      180,
		GracePeriodDays:     14,
		RequiredWitnesses:   1,
		EnableEncryption:    true,
		EnableAuditLog:      true,
		MaxBeneficiaries:    5,
		MaxAssets:           50,
		NotifyBeforeTrigger: 3,
	}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("PUT", "/api/v1/legacy/config", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAddTrustContact(t *testing.T) {
	r, _ := setupTestRouter()

	reqBody := TrustContact{
		Name:         "Trust Contact",
		Email:        "trust@example.com",
		Phone:        "+1122334455",
		Relationship: "Friend",
		Role:         RoleWitness,
	}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/api/v1/legacy/contacts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "Trust Contact", data["name"])
}

func TestListPlans(t *testing.T) {
	r, manager := setupTestRouter()

	// 创建多个计划
	manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Plan 1",
		TriggerType: TriggerManual,
	}, "user-001")

	manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Plan 2",
		TriggerType: TriggerInactivity,
	}, "user-001")

	req, _ := http.NewRequest("GET", "/api/v1/legacy/plans", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.([]interface{})
	assert.True(t, ok)
	assert.Equal(t, 2, len(data))
}

func TestListBeneficiaries(t *testing.T) {
	r, manager := setupTestRouter()

	// 创建一个计划
	plan, _ := manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Plan with Beneficiaries",
		TriggerType: TriggerManual,
	}, "user-001")

	// 添加受益人
	manager.AddBeneficiary(plan.ID, &BeneficiaryRequest{
		Name:              "Beneficiary 1",
		Role:              RoleBeneficiary,
		AllocationPercent: 50,
		AccessLevel:       AccessFull,
	})

	manager.AddBeneficiary(plan.ID, &BeneficiaryRequest{
		Name:              "Beneficiary 2",
		Role:              RoleBeneficiary,
		AllocationPercent: 50,
		AccessLevel:       AccessRead,
	})

	req, _ := http.NewRequest("GET", "/api/v1/legacy/plans/"+plan.ID+"/beneficiaries", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.([]interface{})
	assert.True(t, ok)
	assert.Equal(t, 2, len(data))
}

func TestListAssets(t *testing.T) {
	r, manager := setupTestRouter()

	// 创建一个计划
	plan, _ := manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Plan with Assets",
		TriggerType: TriggerManual,
	}, "user-001")

	// 添加资产
	manager.AddAsset(plan.ID, &AssetRequest{
		Name: "Asset 1",
		Type: AssetTypeAccount,
	})

	manager.AddAsset(plan.ID, &AssetRequest{
		Name: "Asset 2",
		Type: AssetTypeFile,
	})

	req, _ := http.NewRequest("GET", "/api/v1/legacy/plans/"+plan.ID+"/assets", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.([]interface{})
	assert.True(t, ok)
	assert.Equal(t, 2, len(data))
}

func TestListEmergencyContacts(t *testing.T) {
	r, manager := setupTestRouter()

	// 创建一个计划
	plan, _ := manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Plan with Contacts",
		TriggerType: TriggerManual,
	}, "user-001")

	// 添加紧急联系人
	manager.AddEmergencyContact(plan.ID, &EmergencyContactRequest{
		Name: "Contact 1",
		Role: RoleEmergency,
	})

	manager.AddEmergencyContact(plan.ID, &EmergencyContactRequest{
		Name: "Contact 2",
		Role: RoleWitness,
	})

	req, _ := http.NewRequest("GET", "/api/v1/legacy/plans/"+plan.ID+"/contacts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.([]interface{})
	assert.True(t, ok)
	assert.Equal(t, 2, len(data))
}

func TestGetAuditLogs(t *testing.T) {
	r, manager := setupTestRouter()

	// 创建一个计划
	plan, _ := manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Plan with Audit Logs",
		TriggerType: TriggerManual,
	}, "user-001")

	// 添加一些审计日志
	manager.addAuditLog(&AuditLog{
		ID:        "audit-1",
		PlanID:    plan.ID,
		UserID:    "user-001",
		Action:    "create",
		Resource:  "plan",
	})

	manager.addAuditLog(&AuditLog{
		ID:        "audit-2",
		PlanID:    plan.ID,
		UserID:    "user-001",
		Action:    "update",
		Resource:  "plan",
	})

	req, _ := http.NewRequest("GET", "/api/v1/legacy/plans/"+plan.ID+"/audit-logs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.([]interface{})
	assert.True(t, ok)
	assert.Equal(t, 2, len(data))
}

func TestGetAccessGrants(t *testing.T) {
	r, manager := setupTestRouter()

	// 创建一个计划
	plan, _ := manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Plan with Access Grants",
		TriggerType: TriggerManual,
		TriggerConditions: &TriggerConditions{
			EmergencyCode: "test-code",
		},
	}, "user-001")

	// 添加受益人
	beneficiary, _ := manager.AddBeneficiary(plan.ID, &BeneficiaryRequest{
		Name:              "Beneficiary",
		Role:              RoleBeneficiary,
		AllocationPercent: 100,
		AccessLevel:       AccessFull,
	})

	// 添加资产
	_, _ = manager.AddAsset(plan.ID, &AssetRequest{
		Name: "Asset",
		Type: AssetTypeAccount,
		AssignedTo: []string{beneficiary.ID},
	})

	// 激活计划
	manager.ActivatePlan(plan.ID)

	// 触发计划
	err := manager.TriggerPlan(context.Background(), plan.ID, &TriggerRequest{
		PlanID:        plan.ID,
		EmergencyCode: "test-code",
	})
	assert.NoError(t, err)

	req, _ := http.NewRequest("GET", "/api/v1/legacy/plans/"+plan.ID+"/access-grants?beneficiary_id="+beneficiary.ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.([]interface{})
	assert.True(t, ok)
	assert.Equal(t, 1, len(data))
}

func TestRevokeAccessGrant(t *testing.T) {
	r, manager := setupTestRouter()

	// 模拟访问授权
	manager.accessGrants = append(manager.accessGrants, &AccessGrant{
		ID:       "grant-to-revoke",
		PlanID:   "plan-1",
		IsActive: true,
	})

	req, _ := http.NewRequest("DELETE", "/api/v1/legacy/access-grants/grant-to-revoke", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证授权已撤销
	for _, grant := range manager.accessGrants {
		if grant.ID == "grant-to-revoke" {
			assert.False(t, grant.IsActive)
			assert.NotNil(t, grant.RevokedAt)
		}
	}
}

func TestListTrustContacts(t *testing.T) {
	r, manager := setupTestRouter()

	// 添加信任联系人
	manager.AddTrustContact("user-001", &TrustContact{
		Name:  "Contact 1",
		Email: "contact1@example.com",
		Role:  RoleWitness,
	})

	manager.AddTrustContact("user-001", &TrustContact{
		Name:  "Contact 2",
		Email: "contact2@example.com",
		Role:  RoleEmergency,
	})

	req, _ := http.NewRequest("GET", "/api/v1/legacy/contacts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.([]interface{})
	assert.True(t, ok)
	assert.Equal(t, 2, len(data))
}

func TestRemoveTrustContact(t *testing.T) {
	r, manager := setupTestRouter()

	// 添加信任联系人
	contact, _ := manager.AddTrustContact("user-001", &TrustContact{
		Name:  "Contact to Remove",
		Email: "remove@example.com",
		Role:  RoleWitness,
	})

	req, _ := http.NewRequest("DELETE", "/api/v1/legacy/contacts/"+contact.ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证联系人已删除
	contacts := manager.GetTrustContacts("user-001")
	assert.Equal(t, 0, len(contacts))
}

func TestDecryptAsset(t *testing.T) {
	r, manager := setupTestRouter()

	// 创建一个计划
	plan, _ := manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Plan with Encrypted Asset",
		TriggerType: TriggerManual,
		IsEncrypted: true,
	}, "user-001")

	// 添加加密资产
	asset, _ := manager.AddAsset(plan.ID, &AssetRequest{
		Name: "Encrypted Asset",
		Type: AssetTypePassword,
		Data: "my-secret-password",
	})

	req, _ := http.NewRequest("GET", "/api/v1/legacy/plans/"+plan.ID+"/assets/"+asset.ID+"/decrypt", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "my-secret-password", data["data"])
}

func TestInvalidRequests(t *testing.T) {
	r, _ := setupTestRouter()

	// 测试无效的 JSON
	req, _ := http.NewRequest("POST", "/api/v1/legacy/plans", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 测试不存在的计划
	req, _ = http.NewRequest("GET", "/api/v1/legacy/plans/non-existent", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPlanStatusValidation(t *testing.T) {
	r, manager := setupTestRouter()

	// 创建一个计划
	plan, _ := manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Plan for Status Test",
		TriggerType: TriggerManual,
	}, "user-001")

	// 尝试激活没有受益人的计划
	req, _ := http.NewRequest("POST", "/api/v1/legacy/plans/"+plan.ID+"/activate", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAllocationPercentValidation(t *testing.T) {
	r, manager := setupTestRouter()

	// 创建一个计划
	plan, _ := manager.CreatePlan(&LegacyPlanRequest{
		Name:        "Plan for Allocation Test",
		TriggerType: TriggerManual,
	}, "user-001")

	// 添加受益人
	manager.AddBeneficiary(plan.ID, &BeneficiaryRequest{
		Name:              "Beneficiary 1",
		Role:              RoleBeneficiary,
		AllocationPercent: 60,
		AccessLevel:       AccessFull,
	})

	manager.AddBeneficiary(plan.ID, &BeneficiaryRequest{
		Name:              "Beneficiary 2",
		Role:              RoleBeneficiary,
		AllocationPercent: 60,
		AccessLevel:       AccessRead,
	})

	// 添加资产
	manager.AddAsset(plan.ID, &AssetRequest{
		Name: "Asset",
		Type: AssetTypeAccount,
	})

	// 尝试激活分配比例总和不为100的计划
	req, _ := http.NewRequest("POST", "/api/v1/legacy/plans/"+plan.ID+"/activate", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
