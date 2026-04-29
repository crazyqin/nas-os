package emailmod

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"database/sql"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

// setupTestDB 创建测试用内存数据库.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	return db
}

// setupTestManager 创建测试用 Manager.
func setupTestManager(t *testing.T) (*Manager, *Store) {
	t.Helper()
	db := setupTestDB(t)
	store, err := NewStore(db)
	require.NoError(t, err)
	mgr := NewManagerWithStore(store)
	return mgr, store
}

// setupTestRouter 创建测试用 Gin 路由.
func setupTestRouter(h *Handlers) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api/v1")
	h.RegisterRoutes(api)
	return r
}

// createTestPolicy 创建测试策略.
func createTestPolicy(t *testing.T, mgr *Manager, name string, reviewers []Reviewer) *Policy {
	t.Helper()
	enabled := true
	p, err := mgr.CreatePolicy(PolicyInput{
		Name:    name,
		Enabled: &enabled,
		Reviewers: reviewers,
		MatchType: MatchExact,
	})
	require.NoError(t, err)
	return p
}

// ==================== Store 测试 ====================

func TestStore_PolicyCRUD(t *testing.T) {
	mgr, store := setupTestManager(t)
	defer store.Close()

	t.Run("CreatePolicy", func(t *testing.T) {
		p, err := mgr.CreatePolicy(PolicyInput{
			Name:     "测试策略",
			Reviewers: []Reviewer{{UserID: "u1", Username: "审核员1", Level: 1}},
			MatchType: MatchExact,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, p.ID)
		assert.Equal(t, "测试策略", p.Name)
		assert.True(t, p.Enabled)
		assert.Len(t, p.Reviewers, 1)
	})

	t.Run("GetPolicy", func(t *testing.T) {
		p := createTestPolicy(t, mgr, "获取测试", []Reviewer{{UserID: "u1", Username: "审核员1", Level: 1}})
		got, err := mgr.GetPolicy(p.ID)
		require.NoError(t, err)
		assert.Equal(t, p.ID, got.ID)
		assert.Equal(t, "获取测试", got.Name)
	})

	t.Run("GetPolicy_NotFound", func(t *testing.T) {
		_, err := mgr.GetPolicy("nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), ErrPolicyNotFound)
	})

	t.Run("ListPolicies", func(t *testing.T) {
		_, _ = mgr.CreatePolicy(PolicyInput{
			Name: "策略A",
			Reviewers: []Reviewer{{UserID: "u1", Username: "审核员", Level: 1}},
		})
		_, _ = mgr.CreatePolicy(PolicyInput{
			Name: "策略B",
			Reviewers: []Reviewer{{UserID: "u2", Username: "审核员2", Level: 1}},
		})

		policies, err := mgr.ListPolicies()
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(policies), 2)
	})

	t.Run("UpdatePolicy", func(t *testing.T) {
		p := createTestPolicy(t, mgr, "待更新", []Reviewer{{UserID: "u1", Username: "审核员", Level: 1}})
		enabled := false
		updated, err := mgr.UpdatePolicy(p.ID, PolicyInput{
			Name:     "已更新",
			Enabled:  &enabled,
			Reviewers: []Reviewer{{UserID: "u1", Username: "审核员", Level: 1}},
			Priority:  10,
		})
		require.NoError(t, err)
		assert.Equal(t, "已更新", updated.Name)
		assert.False(t, updated.Enabled)
		assert.Equal(t, 10, updated.Priority)
	})

	t.Run("DeletePolicy", func(t *testing.T) {
		p := createTestPolicy(t, mgr, "待删除", []Reviewer{{UserID: "u1", Username: "审核员", Level: 1}})
		err := mgr.DeletePolicy(p.ID)
		require.NoError(t, err)

		_, err = mgr.GetPolicy(p.ID)
		assert.Error(t, err)
	})

	t.Run("DeletePolicy_NotFound", func(t *testing.T) {
		err := mgr.DeletePolicy("nonexistent")
		assert.Error(t, err)
	})

	t.Run("CreatePolicy_EmptyReviewers", func(t *testing.T) {
		_, err := mgr.CreatePolicy(PolicyInput{
			Name:      "无审核人",
			Reviewers: []Reviewer{},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), ErrEmptyReviewers)
	})
}

func TestStore_QueueCRUD(t *testing.T) {
	mgr, store := setupTestManager(t)
	defer store.Close()

	// 先创建策略
	policy := createTestPolicy(t, mgr, "队列测试策略", []Reviewer{
		{UserID: "r1", Username: "审核员1", Level: 1},
		{UserID: "r2", Username: "审核员2", Level: 2},
	})

	t.Run("SubmitEmail_Match", func(t *testing.T) {
		// 启用策略的发件人匹配
		enabled := true
		_, err := mgr.UpdatePolicy(policy.ID, PolicyInput{
			Name:     "队列测试策略",
			Enabled:  &enabled,
			Reviewers: []Reviewer{
				{UserID: "r1", Username: "审核员1", Level: 1},
				{UserID: "r2", Username: "审核员2", Level: 2},
			},
			SenderPatterns: []string{"test@example.com"},
			MatchType:      MatchExact,
		})
		require.NoError(t, err)

		item, err := mgr.SubmitEmail(
			"test@example.com",
			[]string{"admin@company.com"},
			nil,
			"测试邮件",
			"邮件正文内容",
			nil,
		)
		require.NoError(t, err)
		require.NotNil(t, item)
		assert.Equal(t, StatusPending, item.Status)
		assert.Equal(t, 1, item.CurrentLevel)
		assert.Equal(t, 2, item.MaxLevel)
		assert.Equal(t, "test@example.com", item.From)
	})

	t.Run("SubmitEmail_NoMatch", func(t *testing.T) {
		item, err := mgr.SubmitEmail(
			"other@example.com",
			[]string{"admin@company.com"},
			nil,
			"无关邮件",
			"内容",
			nil,
		)
		require.NoError(t, err)
		assert.Nil(t, item) // 不匹配，放行
	})

	t.Run("GetQueueItem", func(t *testing.T) {
		// 提交一封匹配的邮件
		enabled := true
		_, _ = mgr.UpdatePolicy(policy.ID, PolicyInput{
			Name:     "队列测试策略",
			Enabled:  &enabled,
			Reviewers: []Reviewer{
				{UserID: "r1", Username: "审核员1", Level: 1},
				{UserID: "r2", Username: "审核员2", Level: 2},
			},
			SenderPatterns: []string{"submit@test.com"},
			MatchType:      MatchExact,
		})

		submitted, err := mgr.SubmitEmail("submit@test.com", []string{"to@test.com"}, nil, "主题", "正文", nil)
		require.NoError(t, err)
		require.NotNil(t, submitted)

		got, err := mgr.GetQueueItem(submitted.ID)
		require.NoError(t, err)
		assert.Equal(t, submitted.ID, got.ID)
	})

	t.Run("GetQueueItem_NotFound", func(t *testing.T) {
		_, err := mgr.GetQueueItem("nonexistent")
		assert.Error(t, err)
	})

	t.Run("QueryQueue", func(t *testing.T) {
		items, total, err := mgr.QueryQueue(QueueQueryOptions{Limit: 100})
		require.NoError(t, err)
		assert.Greater(t, total, 0)
		assert.NotEmpty(t, items)
	})

	t.Run("QueryQueue_FilterStatus", func(t *testing.T) {
		items, _, err := mgr.QueryQueue(QueueQueryOptions{Status: StatusPending, Limit: 100})
		require.NoError(t, err)
		for _, item := range items {
			assert.Equal(t, StatusPending, item.Status)
		}
	})
}

func TestStore_ReviewFlow(t *testing.T) {
	mgr, store := setupTestManager(t)
	defer store.Close()

	// 创建两级审核策略
	enabled := true
	_, err := mgr.CreatePolicy(PolicyInput{
		Name:     "两级审核",
		Enabled:  &enabled,
		Reviewers: []Reviewer{
			{UserID: "r1", Username: "一级审核员", Level: 1},
			{UserID: "r2", Username: "二级审核员", Level: 2},
		},
		SenderPatterns: []string{"multi@test.com"},
		MatchType:      MatchExact,
	})
	require.NoError(t, err)

	// 提交邮件
	item, err := mgr.SubmitEmail("multi@test.com", []string{"to@test.com"}, nil, "多级审核", "内容", nil)
	require.NoError(t, err)
	require.NotNil(t, item)
	assert.Equal(t, 1, item.CurrentLevel)
	assert.Equal(t, 2, item.MaxLevel)

	t.Run("Approve_Level1", func(t *testing.T) {
		approved, err := mgr.Approve(item.ID, "r1", "一级审核员", "同意")
		require.NoError(t, err)
		assert.Equal(t, StatusPending, approved.Status) // 还需二级审核
		assert.Equal(t, 2, approved.CurrentLevel)
		assert.Len(t, approved.Reviews, 1)
		assert.Equal(t, StatusApproved, approved.Reviews[0].Status)
	})

	t.Run("Approve_WrongReviewer", func(t *testing.T) {
		_, err := mgr.Approve(item.ID, "r1", "一级审核员", "再次审核")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), ErrNotCurrentReviewer)
	})

	t.Run("Approve_Level2_Final", func(t *testing.T) {
		approved, err := mgr.Approve(item.ID, "r2", "二级审核员", "最终批准")
		require.NoError(t, err)
		assert.Equal(t, StatusApproved, approved.Status)
		assert.Len(t, approved.Reviews, 2)
		assert.NotNil(t, approved.ReviewedAt)
	})

	t.Run("Approve_AlreadyReviewed", func(t *testing.T) {
		_, err := mgr.Approve(item.ID, "r2", "二级审核员", "重复审核")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), ErrAlreadyReviewed)
	})
}

func TestStore_RejectFlow(t *testing.T) {
	mgr, store := setupTestManager(t)
	defer store.Close()

	enabled := true
	_, err := mgr.CreatePolicy(PolicyInput{
		Name:     "拒绝测试",
		Enabled:  &enabled,
		Reviewers: []Reviewer{
			{UserID: "r1", Username: "审核员", Level: 1},
		},
		SenderPatterns: []string{"reject@test.com"},
		MatchType:      MatchExact,
	})
	require.NoError(t, err)

	item, err := mgr.SubmitEmail("reject@test.com", []string{"to@test.com"}, nil, "拒绝测试", "内容", nil)
	require.NoError(t, err)
	require.NotNil(t, item)

	t.Run("Reject", func(t *testing.T) {
		rejected, err := mgr.Reject(item.ID, "r1", "审核员", "不合规")
		require.NoError(t, err)
		assert.Equal(t, StatusRejected, rejected.Status)
		assert.NotNil(t, rejected.ReviewedAt)
		assert.Len(t, rejected.Reviews, 1)
		assert.Equal(t, "不合规", rejected.Reviews[0].Comment)
	})

	t.Run("Reject_AlreadyReviewed", func(t *testing.T) {
		_, err := mgr.Reject(item.ID, "r1", "审核员", "重复拒绝")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), ErrAlreadyReviewed)
	})
}

func TestStore_Audit(t *testing.T) {
	mgr, store := setupTestManager(t)
	defer store.Close()

	enabled := true
	_, _ = mgr.CreatePolicy(PolicyInput{
		Name:     "审计测试",
		Enabled:  &enabled,
		Reviewers: []Reviewer{{UserID: "r1", Username: "审核员", Level: 1}},
		SenderPatterns: []string{"audit@test.com"},
		MatchType:      MatchExact,
	})

	// 提交+审批生成审计记录
	item, _ := mgr.SubmitEmail("audit@test.com", []string{"to@test.com"}, nil, "审计邮件", "内容", nil)
	if item != nil {
		_, _ = mgr.Approve(item.ID, "r1", "审核员", "同意")
	}

	t.Run("QueryAudit", func(t *testing.T) {
		entries, total, err := mgr.QueryAudit(AuditQueryOptions{Limit: 100})
		require.NoError(t, err)
		assert.Greater(t, total, 0)
		assert.NotEmpty(t, entries)

		// 验证审计记录包含 submit 和 approve
		actions := make(map[string]bool)
		for _, e := range entries {
			actions[e.Action] = true
		}
		assert.True(t, actions["submit"])
		assert.True(t, actions["approve"])
	})

	t.Run("QueryAudit_FilterStatus", func(t *testing.T) {
		entries, _, err := mgr.QueryAudit(AuditQueryOptions{Status: StatusApproved, Limit: 100})
		require.NoError(t, err)
		for _, e := range entries {
			assert.Equal(t, StatusApproved, e.Status)
		}
	})
}

func TestStore_Stats(t *testing.T) {
	mgr, store := setupTestManager(t)
	defer store.Close()

	stats, err := mgr.GetStats()
	require.NoError(t, err)
	assert.NotNil(t, stats)
	assert.NotNil(t, stats.ByPolicy)
	assert.NotNil(t, stats.ByReviewer)
}

// ==================== 策略匹配测试 ====================

func TestMatchPolicy_SenderMatch(t *testing.T) {
	mgr, store := setupTestManager(t)
	defer store.Close()

	enabled := true
	p, err := mgr.CreatePolicy(PolicyInput{
		Name:           "发件人策略",
		Enabled:        &enabled,
		Reviewers:      []Reviewer{{UserID: "r1", Username: "审核员", Level: 1}},
		SenderPatterns: []string{"bad@example.com", "spam@test.org"},
		MatchType:      MatchExact,
	})
	require.NoError(t, err)

	t.Run("MatchExact_Hit", func(t *testing.T) {
		assert.True(t, mgr.matchPolicy(p, "bad@example.com", []string{"to@test.com"}, "主题", "正文", nil))
	})

	t.Run("MatchExact_Miss", func(t *testing.T) {
		assert.False(t, mgr.matchPolicy(p, "good@example.com", []string{"to@test.com"}, "主题", "正文", nil))
	})
}

func TestMatchPolicy_KeywordMatch(t *testing.T) {
	mgr, store := setupTestManager(t)
	defer store.Close()

	enabled := true
	p, err := mgr.CreatePolicy(PolicyInput{
		Name:      "关键词策略",
		Enabled:   &enabled,
		Reviewers: []Reviewer{{UserID: "r1", Username: "审核员", Level: 1}},
		Keywords:  []string{"机密", "confidential"},
		MatchType: MatchExact,
	})
	require.NoError(t, err)

	t.Run("Keyword_InSubject", func(t *testing.T) {
		assert.True(t, mgr.matchPolicy(p, "user@test.com", []string{"to@test.com"}, "机密文件", "请查收", nil))
	})

	t.Run("Keyword_InBody", func(t *testing.T) {
		assert.True(t, mgr.matchPolicy(p, "user@test.com", []string{"to@test.com"}, "普通邮件", "Contains CONFIDENTIAL info", nil))
	})

	t.Run("Keyword_NoMatch", func(t *testing.T) {
		assert.False(t, mgr.matchPolicy(p, "user@test.com", []string{"to@test.com"}, "普通邮件", "日常内容", nil))
	})
}

func TestMatchPolicy_AttachmentMatch(t *testing.T) {
	mgr, store := setupTestManager(t)
	defer store.Close()

	enabled := true
	p, err := mgr.CreatePolicy(PolicyInput{
		Name:            "附件策略",
		Enabled:         &enabled,
		Reviewers:       []Reviewer{{UserID: "r1", Username: "审核员", Level: 1}},
		AttachmentTypes: []string{".exe", ".bat", ".sh"},
		MatchType:       MatchExact,
	})
	require.NoError(t, err)

	t.Run("Attachment_Hit", func(t *testing.T) {
		attachments := []Attachment{
			{Name: "report.pdf", SizeMB: 1.5},
			{Name: "virus.exe", SizeMB: 0.5},
		}
		assert.True(t, mgr.matchPolicy(p, "user@test.com", []string{"to@test.com"}, "附件", "内容", attachments))
	})

	t.Run("Attachment_Miss", func(t *testing.T) {
		attachments := []Attachment{
			{Name: "report.pdf", SizeMB: 1.5},
			{Name: "photo.jpg", SizeMB: 2.0},
		}
		assert.False(t, mgr.matchPolicy(p, "user@test.com", []string{"to@test.com"}, "附件", "内容", attachments))
	})
}

func TestMatchPolicy_RecipientMatch(t *testing.T) {
	mgr, store := setupTestManager(t)
	defer store.Close()

	enabled := true
	p, err := mgr.CreatePolicy(PolicyInput{
		Name:              "收件人策略",
		Enabled:           &enabled,
		Reviewers:         []Reviewer{{UserID: "r1", Username: "审核员", Level: 1}},
		RecipientPatterns: []string{"ceo@company.com"},
		MatchType:         MatchExact,
	})
	require.NoError(t, err)

	t.Run("Recipient_Hit", func(t *testing.T) {
		assert.True(t, mgr.matchPolicy(p, "user@test.com", []string{"ceo@company.com"}, "报告", "内容", nil))
	})

	t.Run("Recipient_Miss", func(t *testing.T) {
		assert.False(t, mgr.matchPolicy(p, "user@test.com", []string{"intern@company.com"}, "报告", "内容", nil))
	})
}

func TestMatchPolicy_MatchType(t *testing.T) {
	mgr, store := setupTestManager(t)
	defer store.Close()

	t.Run("Domain_Match", func(t *testing.T) {
		enabled := true
		p, _ := mgr.CreatePolicy(PolicyInput{
			Name:           "域名策略",
			Enabled:        &enabled,
			Reviewers:      []Reviewer{{UserID: "r1", Username: "审核员", Level: 1}},
			SenderPatterns: []string{"@suspicious.com"},
			MatchType:      MatchDomain,
		})
		assert.True(t, mgr.matchPolicy(p, "anyone@suspicious.com", []string{"to@test.com"}, "主题", "正文", nil))
	})

	t.Run("Glob_Match", func(t *testing.T) {
		enabled := true
		p, _ := mgr.CreatePolicy(PolicyInput{
			Name:           "通配符策略",
			Enabled:        &enabled,
			Reviewers:      []Reviewer{{UserID: "r1", Username: "审核员", Level: 1}},
			SenderPatterns: []string{"spam*@example.com"},
			MatchType:      MatchGlob,
		})
		assert.True(t, mgr.matchPolicy(p, "spam123@example.com", []string{"to@test.com"}, "主题", "正文", nil))
	})

	t.Run("Regex_Match", func(t *testing.T) {
		enabled := true
		p, _ := mgr.CreatePolicy(PolicyInput{
			Name:           "正则策略",
			Enabled:        &enabled,
			Reviewers:      []Reviewer{{UserID: "r1", Username: "审核员", Level: 1}},
			SenderPatterns: []string{`^[a-z]+@example\.com$`},
			MatchType:      MatchRegex,
		})
		// Note: matchPatterns lowercases both value and pattern, so case-insensitive
		assert.True(t, mgr.matchPolicy(p, "test@example.com", []string{"to@test.com"}, "主题", "正文", nil))
		assert.True(t, mgr.matchPolicy(p, "TEST@EXAMPLE.COM", []string{"to@test.com"}, "主题", "正文", nil)) // lowercased before match
		assert.False(t, mgr.matchPolicy(p, "123@example.com", []string{"to@test.com"}, "主题", "正文", nil))
	})
}

func TestMatchPolicy_NoConditions(t *testing.T) {
	mgr, store := setupTestManager(t)
	defer store.Close()

	enabled := true
	p, _ := mgr.CreatePolicy(PolicyInput{
		Name:      "空条件策略",
		Enabled:   &enabled,
		Reviewers: []Reviewer{{UserID: "r1", Username: "审核员", Level: 1}},
		MatchType: MatchExact,
	})

	// 无任何条件时不应匹配
	assert.False(t, mgr.matchPolicy(p, "anyone@test.com", []string{"to@test.com"}, "主题", "正文", nil))
}

// ==================== HTTP Handler 测试 ====================

func TestHandlers_PolicyCRUD(t *testing.T) {
	mgr, store := setupTestManager(t)
	defer store.Close()
	h := NewHandlers(mgr)
	r := setupTestRouter(h)

	t.Run("CreatePolicy", func(t *testing.T) {
		body := PolicyInput{
			Name:      "HTTP测试策略",
			Reviewers: []Reviewer{{UserID: "r1", Username: "审核员", Level: 1}},
		}
		data, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/email-mod/policies", bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, float64(0), resp["code"])
	})

	t.Run("ListPolicies", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/email-mod/policies", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, float64(0), resp["code"])
	})

	t.Run("GetPolicy", func(t *testing.T) {
		p := createTestPolicy(t, mgr, "HTTP获取测试", []Reviewer{{UserID: "r1", Username: "审核员", Level: 1}})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/email-mod/policies/"+p.ID, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("GetPolicy_NotFound", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/email-mod/policies/nonexistent", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("UpdatePolicy", func(t *testing.T) {
		p := createTestPolicy(t, mgr, "HTTP更新测试", []Reviewer{{UserID: "r1", Username: "审核员", Level: 1}})
		body := PolicyInput{
			Name:      "HTTP已更新",
			Reviewers: []Reviewer{{UserID: "r1", Username: "审核员", Level: 1}},
		}
		data, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/email-mod/policies/"+p.ID, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("DeletePolicy", func(t *testing.T) {
		p := createTestPolicy(t, mgr, "HTTP删除测试", []Reviewer{{UserID: "r1", Username: "审核员", Level: 1}})
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/email-mod/policies/"+p.ID, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestHandlers_QueueAndReview(t *testing.T) {
	mgr, store := setupTestManager(t)
	defer store.Close()

	// 创建匹配策略，审核人用 "admin" (handler默认值)
	enabled := true
	_, _ = mgr.CreatePolicy(PolicyInput{
		Name:           "HTTP队列测试",
		Enabled:        &enabled,
		Reviewers:      []Reviewer{{UserID: "admin", Username: "管理员", Level: 1}},
		SenderPatterns: []string{"handler@test.com"},
		MatchType:      MatchExact,
	})

	h := NewHandlers(mgr)
	// 创建路由并在中间件注入 user 信息
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", "admin")
		c.Set("username", "管理员")
		c.Next()
	})
	api := r.Group("/api/v1")
	h.RegisterRoutes(api)

	t.Run("SubmitEmail", func(t *testing.T) {
		body := submitEmailRequest{
			From:    "handler@test.com",
			To:      []string{"to@test.com"},
			Subject: "HTTP测试",
			Body:    "内容",
		}
		data, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/email-mod/submit", bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		dataMap := resp["data"].(map[string]interface{})
		assert.Equal(t, true, dataMap["queued"])
	})

	t.Run("QueryQueue", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/email-mod/queue?status=pending", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Approve", func(t *testing.T) {
		// 先提交一封
		submitBody := submitEmailRequest{
			From:    "handler@test.com",
			To:      []string{"to@test.com"},
			Subject: "审批测试",
		}
		submitData, _ := json.Marshal(submitBody)
		submitReq := httptest.NewRequest(http.MethodPost, "/api/v1/email-mod/submit", bytes.NewReader(submitData))
		submitReq.Header.Set("Content-Type", "application/json")
		submitW := httptest.NewRecorder()
		r.ServeHTTP(submitW, submitReq)

		var submitResp map[string]interface{}
		_ = json.Unmarshal(submitW.Body.Bytes(), &submitResp)
		item := submitResp["data"].(map[string]interface{})["item"].(map[string]interface{})
		queueID := item["id"].(string)

		// 审批
		reviewBody := ReviewInput{Comment: "同意"}
		reviewData, _ := json.Marshal(reviewBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/email-mod/queue/"+queueID+"/approve", bytes.NewReader(reviewData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Reject_NonExistent", func(t *testing.T) {
		reviewBody := ReviewInput{Comment: "拒绝"}
		reviewData, _ := json.Marshal(reviewBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/email-mod/queue/nonexistent/reject", bytes.NewReader(reviewData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandlers_AuditAndStats(t *testing.T) {
	mgr, store := setupTestManager(t)
	defer store.Close()

	h := NewHandlers(mgr)
	r := setupTestRouter(h)

	t.Run("QueryAudit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/email-mod/audit", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, float64(0), resp["code"])
	})

	t.Run("GetStats", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/email-mod/stats", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("QueryAudit_WithFilters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/email-mod/audit?status=approved&limit=10&offset=0", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestHandlers_InvalidInput(t *testing.T) {
	mgr, store := setupTestManager(t)
	defer store.Close()
	h := NewHandlers(mgr)
	r := setupTestRouter(h)

	t.Run("CreatePolicy_BadJSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/email-mod/policies", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("SubmitEmail_MissingFields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/email-mod/submit", bytes.NewReader([]byte(`{"from":"test@test.com"}`)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// ==================== 辅助函数测试 ====================

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		s       string
		pattern string
		want    bool
	}{
		{"hello", "hello", true},
		{"hello", "h*", true},
		{"hello", "h?llo", true},
		{"hello", "world", false},
		{"test@example.com", "test@*.com", true},
		{"", "*", true},
		{"", "a", false},
	}
	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.pattern, func(t *testing.T) {
			assert.Equal(t, tt.want, matchGlob(tt.s, tt.pattern))
		})
	}
}

func TestTruncateString(t *testing.T) {
	assert.Equal(t, "hello", truncateString("hello", 10))
	assert.Equal(t, "hel...", truncateString("hello world", 3))
	assert.Equal(t, "", truncateString("", 5))
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()
	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
}

func TestBoolToInt(t *testing.T) {
	assert.Equal(t, 1, boolToInt(true))
	assert.Equal(t, 0, boolToInt(false))
}

// TestMain 清理测试文件.
func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
