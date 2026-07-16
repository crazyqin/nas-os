// Package emailserver 提供 REST API 处理器测试
package emailserver

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

// TestCreateAccount 测试创建邮件账户.
func TestCreateAccount(t *testing.T) {
	r, _ := setupTestRouter()

	reqBody := CreateAccountRequest{
		Username:    "testuser",
		Domain:      "example.com",
		Password:    "password123",
		DisplayName: "Test User",
		QuotaMB:     512,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/email/accounts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "testuser@example.com", data["email"])
	assert.Equal(t, "Test User", data["display_name"])
}

// TestSendEmail 测试发送邮件.
func TestSendEmail(t *testing.T) {
	r, mgr := setupTestRouter()

	// 先创建发件人账户
	mgr.CreateAccount(CreateAccountRequest{
		Username: "sender",
		Domain:   "example.com",
		Password: "pass",
	})

	reqBody := SendEmailRequest{
		From:    "sender@example.com",
		To:      []string{"recipient@example.com"},
		Subject: "Test Subject",
		Body:    "Hello, this is a test email.",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/email/send", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

// TestListMessages 测试获取邮件列表.
func TestListMessages(t *testing.T) {
	r, mgr := setupTestRouter()

	// 创建账户并发送邮件
	acct := mgr.CreateAccount(CreateAccountRequest{
		Username: "user1",
		Domain:   "example.com",
		Password: "pass",
	})

	mgr.SendEmail(SendEmailRequest{
		From:    "user1@example.com",
		To:      []string{"user2@example.com"},
		Subject: "Test Email 1",
		Body:    "Content 1",
	})

	mgr.SendEmail(SendEmailRequest{
		From:    "user1@example.com",
		To:      []string{"user2@example.com"},
		Subject: "Test Email 2",
		Body:    "Content 2",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/email/messages?account_id="+acct.ID+"&folder=sent", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, 2, int(data["total"].(float64)))
}

// TestCreateFilterRule 测试创建过滤规则.
func TestCreateFilterRule(t *testing.T) {
	r, _ := setupTestRouter()

	reqBody := CreateFilterRuleRequest{
		Name:        "Spam Filter",
		Priority:    1,
		Condition:   "subject",
		MatchType:   "contains",
		MatchValue:  "spam",
		Action:      "move",
		ActionValue: "spam",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/email/rules", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "Spam Filter", data["name"])
	assert.Equal(t, "subject", data["condition"])
}

// TestAntispamConfig 测试反垃圾邮件配置.
func TestAntispamConfig(t *testing.T) {
	r, mgr := setupTestRouter()

	// 更新配置
	enabled := true
	threshold := 75
	reqBody := UpdateAntispamRequest{
		Enabled:        &enabled,
		Threshold:      &threshold,
		BlacklistAddrs: []string{"spammer@evil.com"},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/email/antispam", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证配置已更新
	cfg := mgr.GetAntispamConfig()
	assert.Equal(t, 75, cfg.Threshold)
	assert.Contains(t, cfg.BlacklistAddrs, "spammer@evil.com")

	// 测试垃圾邮件检测
	assert.True(t, mgr.IsSpam("spammer@evil.com"))
	assert.False(t, mgr.IsSpam("legit@good.com"))
}
