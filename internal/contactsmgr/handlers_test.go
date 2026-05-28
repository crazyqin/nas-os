// Package contactsmgr 提供 REST API 处理器测试
package contactsmgr

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupTestRouter() (*gin.Engine, *Manager) {
	gin.SetMode(gin.TestMode)
	mgr := NewManager()
	h := NewHandlers(mgr)

	r := gin.New()
	api := r.Group("/api")
	h.RegisterRoutes(api)

	return r, mgr
}

// TestCreateContact 测试创建联系人.
func TestCreateContact(t *testing.T) {
	r, _ := setupTestRouter()

	reqBody := CreateContactRequest{
		FirstName: "John",
		LastName:  "Doe",
		Emails: []Email{
			{Type: "work", Address: "john@example.com", Primary: true},
		},
		Phones: []Phone{
			{Type: "mobile", Number: "+1234567890", Primary: true},
		},
		Company: "Example Corp",
		Title:   "Software Engineer",
		Tags:    []string{"colleague", "tech"},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/contacts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "John", data["first_name"])
	assert.Equal(t, "Doe", data["last_name"])
	assert.Equal(t, "John Doe", data["full_name"])
}

// TestUpdateContact 测试更新联系人.
func TestUpdateContact(t *testing.T) {
	r, mgr := setupTestRouter()

	// 先创建联系人
	contact := mgr.CreateContact(CreateContactRequest{
		FirstName: "Jane",
		LastName:  "Smith",
		Company:   "Old Company",
	})

	// 更新联系人
	newCompany := "New Company"
	newTitle := "CTO"
	reqBody := UpdateContactRequest{
		Company: &newCompany,
		Title:   &newTitle,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/contacts/"+contact.ID, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "New Company", data["company"])
	assert.Equal(t, "CTO", data["title"])
}

// TestSearchContacts 测试搜索联系人.
func TestSearchContacts(t *testing.T) {
	r, mgr := setupTestRouter()

	// 创建多个联系人
	mgr.CreateContact(CreateContactRequest{
		FirstName: "Alice",
		LastName:  "Johnson",
		Company:   "Tech Corp",
		Emails:    []Email{{Address: "alice@techcorp.com"}},
	})

	mgr.CreateContact(CreateContactRequest{
		FirstName: "Bob",
		LastName:  "Williams",
		Company:   "Design Studio",
		Emails:    []Email{{Address: "bob@design.com"}},
	})

	mgr.CreateContact(CreateContactRequest{
		FirstName: "Charlie",
		LastName:  "Brown",
		Company:   "Tech Solutions",
		Phones:    []Phone{{Number: "+1234567890"}},
	})

	// 搜索 "Tech"
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/contacts/search?q=Tech", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, 2, int(data["total"].(float64)))
}

// TestContactGroups 测试联系人组功能.
func TestContactGroups(t *testing.T) {
	r, mgr := setupTestRouter()

	// 创建组
	group := mgr.CreateGroup(CreateGroupRequest{
		Name:        "Work Friends",
		Description: "Colleagues from work",
		Color:       "#3498db",
	})

	// 创建联系人
	contact1 := mgr.CreateContact(CreateContactRequest{
		FirstName: "Colleague1",
		LastName:  "Test",
	})
	contact2 := mgr.CreateContact(CreateContactRequest{
		FirstName: "Colleague2",
		LastName:  "Test",
	})

	// 添加联系人到组
	addReq := AddContactsToGroupRequest{
		ContactIDs: []string{contact1.ID, contact2.ID},
	}
	body, _ := json.Marshal(addReq)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/contacts/groups/"+group.ID+"/contacts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 列出组成员
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/contacts/groups/"+group.ID+"/contacts", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, 2, int(data["total"].(float64)))
}

// TestDeduplicateContacts 测试联系人去重.
func TestDeduplicateContacts(t *testing.T) {
	r, mgr := setupTestRouter()

	// 创建重复联系人
	mgr.CreateContact(CreateContactRequest{
		FirstName: "John",
		LastName:  "Duplicate",
		Emails:    []Email{{Address: "john@example.com"}},
	})
	mgr.CreateContact(CreateContactRequest{
		FirstName: "John",
		LastName:  "Duplicate",
		Phones:    []Phone{{Number: "+1111111111"}},
	})

	// 查找重复
	reqBody := DeduplicateRequest{Field: "name"}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/contacts/deduplicate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.GreaterOrEqual(t, int(data["total"].(float64)), 1)
}

// TestVCardExport 测试 vCard 导出.
func TestVCardExport(t *testing.T) {
	r, mgr := setupTestRouter()

	// 创建联系人
	contact := mgr.CreateContact(CreateContactRequest{
		FirstName: "Export",
		LastName:  "Test",
		Emails:    []Email{{Type: "work", Address: "export@test.com", Primary: true}},
		Phones:    []Phone{{Type: "mobile", Number: "+9876543210", Primary: true}},
		Company:   "Export Corp",
	})

	// 导出单个联系人
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/contacts/export?contact_id="+contact.ID, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "Export", data["first_name"])
	assert.Equal(t, "Export Corp", data["organization"])
}
