package aiconsole

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动
)

// newTestDB 创建测试用内存数据库.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

// newTestStore 创建测试用 Store.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	db := newTestDB(t)
	store, err := NewStore(db)
	require.NoError(t, err)
	return store
}

// ==================== 模型 CRUD 测试 ====================

func TestStore_CreateAndGetModel(t *testing.T) {
	store := newTestStore(t)

	m := &AIModel{
		ID:          "model-1",
		Name:        "GPT-4",
		Provider:    ProviderOpenAI,
		Endpoint:    "https://api.openai.com",
		APIKey:      "sk-test",
		ModelName:   "gpt-4",
		MaxTokens:   4096,
		Temperature: 0.7,
		Status:      ModelStatusActive,
		IsDefault:   true,
		Enabled:     true,
		Description: "OpenAI GPT-4",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err := store.CreateModel(m)
	require.NoError(t, err)

	got, err := store.GetModel("model-1")
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, "model-1", got.ID)
	assert.Equal(t, "GPT-4", got.Name)
	assert.Equal(t, ProviderOpenAI, got.Provider)
	assert.Equal(t, "https://api.openai.com", got.Endpoint)
	assert.Equal(t, "sk-test", got.APIKey)
	assert.Equal(t, "gpt-4", got.ModelName)
	assert.Equal(t, 4096, got.MaxTokens)
	assert.True(t, got.IsDefault)
	assert.True(t, got.Enabled)
}

func TestStore_GetModel_NotFound(t *testing.T) {
	store := newTestStore(t)

	got, err := store.GetModel("nonexistent")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestStore_ListModels(t *testing.T) {
	store := newTestStore(t)

	// 创建多个模型
	for i, name := range []string{"Model-A", "Model-B", "Model-C"} {
		m := &AIModel{
			ID:        "m-" + name,
			Name:      name,
			Provider:  ProviderOpenAI,
			Endpoint:  "https://api.test.com",
			ModelName: "test",
			Status:    ModelStatusActive,
			Enabled:   true,
			CreatedAt: time.Now().Add(time.Duration(i) * time.Second),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, store.CreateModel(m))
	}

	models, err := store.ListModels()
	require.NoError(t, err)
	assert.Len(t, models, 3)
}

func TestStore_ListModels_Empty(t *testing.T) {
	store := newTestStore(t)

	models, err := store.ListModels()
	require.NoError(t, err)
	assert.Empty(t, models)
}

func TestStore_UpdateModel(t *testing.T) {
	store := newTestStore(t)

	m := &AIModel{
		ID: "m-1", Name: "Old", Provider: ProviderLocal,
		Endpoint: "http://localhost:11434", ModelName: "llama3",
		Status: ModelStatusActive, Enabled: true,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	require.NoError(t, store.CreateModel(m))

	m.Name = "New"
	m.Description = "Updated"
	m.Temperature = 0.5
	err := store.UpdateModel(m)
	require.NoError(t, err)

	got, _ := store.GetModel("m-1")
	assert.Equal(t, "New", got.Name)
	assert.Equal(t, "Updated", got.Description)
	assert.InDelta(t, 0.5, got.Temperature, 0.001)
}

func TestStore_UpdateModel_NotFound(t *testing.T) {
	store := newTestStore(t)

	m := &AIModel{ID: "nonexistent", Name: "X", Provider: ProviderLocal,
		Endpoint: "http://x", ModelName: "x",
		CreatedAt: time.Now(), UpdatedAt: time.Now()}
	err := store.UpdateModel(m)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestStore_DeleteModel(t *testing.T) {
	store := newTestStore(t)

	m := &AIModel{
		ID: "m-del", Name: "Del", Provider: ProviderLocal,
		Endpoint: "http://localhost", ModelName: "test",
		Status: ModelStatusActive, Enabled: true,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	require.NoError(t, store.CreateModel(m))

	err := store.DeleteModel("m-del")
	require.NoError(t, err)

	got, _ := store.GetModel("m-del")
	assert.Nil(t, got)
}

func TestStore_DeleteModel_NotFound(t *testing.T) {
	store := newTestStore(t)

	err := store.DeleteModel("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestStore_GetDefaultModel(t *testing.T) {
	store := newTestStore(t)

	// 无默认模型
	got, err := store.GetDefaultModel()
	require.NoError(t, err)
	assert.Nil(t, got)

	// 创建一个默认模型
	m := &AIModel{
		ID: "default-1", Name: "Default", Provider: ProviderOpenAI,
		Endpoint: "https://api.test.com", ModelName: "gpt-4",
		Status: ModelStatusActive, IsDefault: true, Enabled: true,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	require.NoError(t, store.CreateModel(m))

	got, err = store.GetDefaultModel()
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "default-1", got.ID)
	assert.True(t, got.IsDefault)
}

func TestStore_ClearDefault(t *testing.T) {
	store := newTestStore(t)

	// 创建两个默认模型（手动插入）
	for _, id := range []string{"a", "b"} {
		m := &AIModel{
			ID: id, Name: id, Provider: ProviderLocal,
			Endpoint: "http://localhost", ModelName: "test",
			Status: ModelStatusActive, IsDefault: true, Enabled: true,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		require.NoError(t, store.CreateModel(m))
	}

	err := store.ClearDefault()
	require.NoError(t, err)

	// 都不是默认了
	got, _ := store.GetDefaultModel()
	assert.Nil(t, got)
}

// ==================== 脱敏规则 CRUD 测试 ====================

func TestStore_CreateAndGetRule(t *testing.T) {
	store := newTestStore(t)

	r := &RedactRule{
		ID:          "rule-1",
		Name:        "邮箱规则",
		PIIType:     PIIEmail,
		Pattern:     `[\w.\-]+@[\w.\-]+\.\w+`,
		Strategy:    StrategyPartial,
		MaskChar:    "*",
		ShowFirst:   2,
		Enabled:     true,
		Priority:    80,
		Description: "电子邮箱脱敏",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	err := store.CreateRule(r)
	require.NoError(t, err)

	got, err := store.GetRule("rule-1")
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, "邮箱规则", got.Name)
	assert.Equal(t, PIIEmail, got.PIIType)
	assert.Equal(t, StrategyPartial, got.Strategy)
	assert.True(t, got.Enabled)
}

func TestStore_GetRule_NotFound(t *testing.T) {
	store := newTestStore(t)

	got, err := store.GetRule("nonexistent")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestStore_ListRules(t *testing.T) {
	store := newTestStore(t)

	for i, name := range []string{"R1", "R2", "R3"} {
		r := &RedactRule{
			ID: "r-" + name, Name: name, PIIType: PIICustom,
			Pattern: `\d+`, Strategy: StrategyMask,
			Enabled: true, Priority: 100 - i,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		require.NoError(t, store.CreateRule(r))
	}

	rules, err := store.ListRules()
	require.NoError(t, err)
	assert.Len(t, rules, 3)

	// 验证按优先级降序
	assert.GreaterOrEqual(t, rules[0].Priority, rules[1].Priority)
}

func TestStore_ListEnabledRules(t *testing.T) {
	store := newTestStore(t)

	// 创建一个启用和一个禁用的规则
	r1 := &RedactRule{
		ID: "enabled", Name: "Enabled", PIIType: PIICustom,
		Pattern: `\d+`, Strategy: StrategyMask,
		Enabled: true, Priority: 100,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	r2 := &RedactRule{
		ID: "disabled", Name: "Disabled", PIIType: PIICustom,
		Pattern: `\d+`, Strategy: StrategyMask,
		Enabled: false, Priority: 90,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	require.NoError(t, store.CreateRule(r1))
	require.NoError(t, store.CreateRule(r2))

	rules, err := store.ListEnabledRules()
	require.NoError(t, err)
	assert.Len(t, rules, 1)
	assert.Equal(t, "enabled", rules[0].ID)
}

func TestStore_UpdateRule(t *testing.T) {
	store := newTestStore(t)

	r := &RedactRule{
		ID: "r-upd", Name: "Old", PIIType: PIICustom,
		Pattern: `\d+`, Strategy: StrategyMask,
		Enabled: true, Priority: 50,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	require.NoError(t, store.CreateRule(r))

	r.Name = "New"
	r.Priority = 200
	r.Enabled = false
	err := store.UpdateRule(r)
	require.NoError(t, err)

	got, _ := store.GetRule("r-upd")
	assert.Equal(t, "New", got.Name)
	assert.Equal(t, 200, got.Priority)
	assert.False(t, got.Enabled)
}

func TestStore_UpdateRule_NotFound(t *testing.T) {
	store := newTestStore(t)

	r := &RedactRule{ID: "nonexistent", Name: "X", PIIType: PIICustom,
		Pattern: `\d+`, Strategy: StrategyMask,
		CreatedAt: time.Now(), UpdatedAt: time.Now()}
	err := store.UpdateRule(r)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestStore_DeleteRule(t *testing.T) {
	store := newTestStore(t)

	r := &RedactRule{
		ID: "r-del", Name: "Del", PIIType: PIICustom,
		Pattern: `\d+`, Strategy: StrategyMask,
		Enabled: true, Priority: 50,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	require.NoError(t, store.CreateRule(r))

	err := store.DeleteRule("r-del")
	require.NoError(t, err)

	got, _ := store.GetRule("r-del")
	assert.Nil(t, got)
}

func TestStore_DeleteRule_NotFound(t *testing.T) {
	store := newTestStore(t)

	err := store.DeleteRule("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

// ==================== 审计日志测试 ====================

func TestStore_CreateAndQueryAuditEntry(t *testing.T) {
	store := newTestStore(t)

	e := &AuditEntry{
		ID:               "audit-1",
		Timestamp:        time.Now(),
		UserID:           "user1",
		Username:         "testuser",
		ModelID:          "model-1",
		ModelName:        "GPT-4",
		Action:           "chat",
		RequestSummary:   "你好",
		ResponseSummary:  "你好！",
		PromptTokens:     10,
		CompletionTokens: 20,
		TotalTokens:      30,
		DurationMs:       500,
		Success:          true,
		Redacted:         true,
		RedactCount:      2,
		IPAddress:        "127.0.0.1",
		Metadata:         map[string]interface{}{"key": "value"},
	}

	err := store.CreateAuditEntry(e)
	require.NoError(t, err)

	// 查询
	entries, total, err := store.QueryAuditLogs(AuditQueryFilter{
		UserID: "user1",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, entries, 1)

	got := entries[0]
	assert.Equal(t, "user1", got.UserID)
	assert.Equal(t, "chat", got.Action)
	assert.True(t, got.Success)
	assert.True(t, got.Redacted)
	assert.Equal(t, 2, got.RedactCount)
	assert.Equal(t, "127.0.0.1", got.IPAddress)
	assert.Equal(t, "value", got.Metadata["key"])
}

func TestStore_QueryAuditLogs_Pagination(t *testing.T) {
	store := newTestStore(t)

	// 创建 5 条审计日志
	for i := 0; i < 5; i++ {
		e := &AuditEntry{
			ID:        "audit-" + string(rune('A'+i)),
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			UserID:    "user1",
			Action:    "chat",
			Success:   true,
		}
		require.NoError(t, store.CreateAuditEntry(e))
	}

	// 第1页，2条
	entries, total, err := store.QueryAuditLogs(AuditQueryFilter{
		Page:     1,
		PageSize: 2,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, entries, 2)

	// 第3页，2条（应只有1条）
	entries, total, err = store.QueryAuditLogs(AuditQueryFilter{
		Page:     3,
		PageSize: 2,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, entries, 1)
}

func TestStore_QueryAuditLogs_FilterByAction(t *testing.T) {
	store := newTestStore(t)

	e1 := &AuditEntry{ID: "a1", Timestamp: time.Now(), Action: "chat", Success: true}
	e2 := &AuditEntry{ID: "a2", Timestamp: time.Now(), Action: "model_add", Success: true}
	require.NoError(t, store.CreateAuditEntry(e1))
	require.NoError(t, store.CreateAuditEntry(e2))

	entries, total, err := store.QueryAuditLogs(AuditQueryFilter{Action: "chat"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, entries, 1)
	assert.Equal(t, "chat", entries[0].Action)
}

func TestStore_QueryAuditLogs_FilterBySuccess(t *testing.T) {
	store := newTestStore(t)

	e1 := &AuditEntry{ID: "s1", Timestamp: time.Now(), Action: "chat", Success: true}
	e2 := &AuditEntry{ID: "s2", Timestamp: time.Now(), Action: "chat", Success: false}
	require.NoError(t, store.CreateAuditEntry(e1))
	require.NoError(t, store.CreateAuditEntry(e2))

	success := false
	entries, total, err := store.QueryAuditLogs(AuditQueryFilter{Success: &success})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, entries, 1)
	assert.False(t, entries[0].Success)
}

func TestStore_QueryAuditLogs_Empty(t *testing.T) {
	store := newTestStore(t)

	entries, total, err := store.QueryAuditLogs(AuditQueryFilter{})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, entries)
}

func TestStore_QueryAuditLogs_DefaultPagination(t *testing.T) {
	store := newTestStore(t)

	// 不设置分页参数，应使用默认值
	entries, total, err := store.QueryAuditLogs(AuditQueryFilter{
		Page:     0, // 无效值
		PageSize: 0, // 无效值
	})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, entries)
}

func TestStore_Migrate_Idempotent(t *testing.T) {
	db := newTestDB(t)

	// 多次迁移应该幂等
	_, err := NewStore(db)
	require.NoError(t, err)
	_, err = NewStore(db)
	require.NoError(t, err)
}
