package aitokenmeter

import (
	"sync"
	"testing"
	"time"
)

// ========== 滑动窗口测试 ==========

func TestSlidingWindow_BasicCheck(t *testing.T) {
	sw := newSlidingWindow(time.Minute, 1000, 10)

	// 初始应允许
	allowed, _ := sw.check(100)
	if !allowed {
		t.Fatal("initial check should allow")
	}
	sw.add(100)

	// 再次检查
	allowed, _ = sw.check(200)
	if !allowed {
		t.Fatal("second check should allow")
	}
	sw.add(200)

	// 窗口内当前用量
	usage := sw.currentUsage()
	if usage != 300 {
		t.Fatalf("expected 300, got %d", usage)
	}
}

func TestSlidingWindow_TokenLimit(t *testing.T) {
	sw := newSlidingWindow(time.Minute, 500, 0)

	// 消耗 400
	sw.add(400)

	// 再加 200 应该被拒
	allowed, _ := sw.check(200)
	if allowed {
		t.Fatal("should be rejected: token limit exceeded")
	}

	// 加 100 应该通过
	allowed, _ = sw.check(100)
	if !allowed {
		t.Fatal("should be allowed: within limit")
	}
}

func TestSlidingWindow_RequestLimit(t *testing.T) {
	sw := newSlidingWindow(time.Minute, 0, 3)

	sw.add(10)
	sw.add(10)
	sw.add(10)

	allowed, _ := sw.check(10)
	if allowed {
		t.Fatal("should be rejected: request limit exceeded")
	}
}

func TestSlidingWindow_Expiry(t *testing.T) {
	sw := newSlidingWindow(100*time.Millisecond, 1000, 10)

	sw.add(500)
	usage := sw.currentUsage()
	if usage != 500 {
		t.Fatalf("expected 500, got %d", usage)
	}

	// 等待窗口过期
	time.Sleep(150 * time.Millisecond)
	usage = sw.currentUsage()
	if usage != 0 {
		t.Fatalf("expected 0 after expiry, got %d", usage)
	}
}

// ========== Meter 测试 ==========

func TestMeter_CheckAndRecord(t *testing.T) {
	m := NewMeter(1000)
	limits := []RateLimit{
		{UserID: "user1", MaxTokens: 1000, Window: time.Minute, MaxRequests: 10},
	}

	usage := TokenUsage{
		UserID:           "user1",
		Provider:         ProviderOpenAI,
		Model:            "gpt-4",
		PromptTokens:     100,
		CompletionTokens: 200,
		TotalTokens:      300,
		Cost:             0.01,
		Timestamp:        time.Now(),
	}

	err := m.CheckAndRecord(usage, limits)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if m.UsageCount() != 1 {
		t.Fatalf("expected 1 usage record, got %d", m.UsageCount())
	}
}

func TestMeter_RateLimit(t *testing.T) {
	m := NewMeter(1000)
	limits := []RateLimit{
		{UserID: "user1", MaxTokens: 500, Window: time.Minute, MaxRequests: 100},
	}

	// 第一次应通过
	usage1 := TokenUsage{
		UserID:      "user1",
		Provider:    ProviderOpenAI,
		TotalTokens: 400,
		Timestamp:   time.Now(),
	}
	if err := m.CheckAndRecord(usage1, limits); err != nil {
		t.Fatalf("first should pass: %v", err)
	}

	// 第二次超限
	usage2 := TokenUsage{
		UserID:      "user1",
		Provider:    ProviderOpenAI,
		TotalTokens: 200,
		Timestamp:   time.Now(),
	}
	if err := m.CheckAndRecord(usage2, limits); err != ErrRateLimited {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
}

func TestMeter_RequestLimit(t *testing.T) {
	m := NewMeter(1000)
	limits := []RateLimit{
		{UserID: "user1", MaxTokens: 10000, Window: time.Minute, MaxRequests: 2},
	}

	for i := 0; i < 2; i++ {
		usage := TokenUsage{
			UserID:      "user1",
			TotalTokens: 10,
			Timestamp:   time.Now(),
		}
		if err := m.CheckAndRecord(usage, limits); err != nil {
			t.Fatalf("request %d should pass: %v", i, err)
		}
	}

	usage := TokenUsage{
		UserID:      "user1",
		TotalTokens: 10,
		Timestamp:   time.Now(),
	}
	if err := m.CheckAndRecord(usage, limits); err != ErrRateLimited {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
}

func TestMeter_GetUserUsage(t *testing.T) {
	m := NewMeter(1000)
	limits := []RateLimit{
		{UserID: "user1", MaxTokens: 10000, Window: time.Minute, MaxRequests: 100},
	}

	now := time.Now()
	m.CheckAndRecord(TokenUsage{UserID: "user1", TotalTokens: 100, Cost: 0.01, Timestamp: now}, limits)
	m.CheckAndRecord(TokenUsage{UserID: "user1", TotalTokens: 200, Cost: 0.02, Timestamp: now}, limits)
	m.CheckAndRecord(TokenUsage{UserID: "user2", TotalTokens: 500, Cost: 0.05, Timestamp: now}, limits)

	tokens, cost := m.GetUserUsage("user1", now.Add(-time.Hour))
	if tokens != 300 {
		t.Fatalf("expected 300 tokens, got %d", tokens)
	}
	if cost != 0.03 {
		t.Fatalf("expected 0.03 cost, got %f", cost)
	}

	tokens, _ = m.GetUserUsage("user2", now.Add(-time.Hour))
	if tokens != 500 {
		t.Fatalf("expected 500 tokens, got %d", tokens)
	}
}

func TestMeter_Concurrent(t *testing.T) {
	m := NewMeter(10000)
	limits := []RateLimit{
		{UserID: "user1", MaxTokens: 100000, Window: time.Minute, MaxRequests: 10000},
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			usage := TokenUsage{
				UserID:      "user1",
				TotalTokens: 10,
				Timestamp:   time.Now(),
			}
			m.CheckAndRecord(usage, limits)
		}()
	}
	wg.Wait()

	if m.UsageCount() != 100 {
		t.Fatalf("expected 100 records, got %d", m.UsageCount())
	}
}

// ========== QuotaManager 测试 ==========

func TestQuotaManager_SetAndGet(t *testing.T) {
	qm := NewQuotaManager()

	quota := UserQuota{
		UserID:  "user1",
		Limits:  map[QuotaPeriod]int{PeriodPerDay: 10000},
		Enabled: true,
	}
	qm.SetQuota(quota)

	got, ok := qm.GetQuota("user1")
	if !ok {
		t.Fatal("quota not found")
	}
	if got.Limits[PeriodPerDay] != 10000 {
		t.Fatalf("expected 10000, got %d", got.Limits[PeriodPerDay])
	}
}

func TestQuotaManager_CheckQuota(t *testing.T) {
	qm := NewQuotaManager()

	qm.SetQuota(UserQuota{
		UserID:  "user1",
		Limits:  map[QuotaPeriod]int{PeriodPerDay: 1000},
		Enabled: true,
	})

	// 未超限
	err := qm.CheckQuota("user1", 500, ProviderOpenAI, PeriodPerDay, 200, 0)
	if err != nil {
		t.Fatalf("should pass: %v", err)
	}

	// 超限
	err = qm.CheckQuota("user1", 500, ProviderOpenAI, PeriodPerDay, 800, 0)
	if err != ErrQuotaExceeded {
		t.Fatalf("expected ErrQuotaExceeded, got %v", err)
	}
}

func TestQuotaManager_CheckQuotaNoLimit(t *testing.T) {
	qm := NewQuotaManager()

	// 无配额应通过
	err := qm.CheckQuota("unknown", 999999, ProviderOpenAI, PeriodPerDay, 0, 0)
	if err != nil {
		t.Fatalf("no quota should pass: %v", err)
	}
}

func TestQuotaManager_AssignPlan(t *testing.T) {
	qm := NewQuotaManager()

	qm.SetPlan(Plan{
		ID:          "basic",
		Name:        "Basic Plan",
		TokenLimits: map[QuotaPeriod]int{PeriodPerDay: 5000, PeriodPerMonth: 100000},
		CostLimits:  map[QuotaPeriod]float64{PeriodPerDay: 1.0, PeriodPerMonth: 20.0},
	})

	err := qm.AssignPlan("user1", "basic")
	if err != nil {
		t.Fatalf("assign plan failed: %v", err)
	}

	quota, ok := qm.GetQuota("user1")
	if !ok {
		t.Fatal("quota not created after plan assignment")
	}
	if quota.PlanID != "basic" {
		t.Fatalf("expected plan basic, got %s", quota.PlanID)
	}
	if quota.Limits[PeriodPerDay] != 5000 {
		t.Fatalf("expected 5000 daily limit, got %d", quota.Limits[PeriodPerDay])
	}
}

func TestQuotaManager_AssignPlanNotFound(t *testing.T) {
	qm := NewQuotaManager()
	err := qm.AssignPlan("user1", "nonexistent")
	if err != ErrPlanNotFound {
		t.Fatalf("expected ErrPlanNotFound, got %v", err)
	}
}

func TestQuotaManager_ProviderQuota(t *testing.T) {
	qm := NewQuotaManager()

	qm.SetQuota(UserQuota{
		UserID:        "user1",
		ProviderQuota: map[Provider]int{ProviderOpenAI: 1000, ProviderClaude: 500},
		Enabled:       true,
	})

	// OpenAI 未超限
	err := qm.CheckQuota("user1", 500, ProviderOpenAI, PeriodPerDay, 200, 0)
	if err != nil {
		t.Fatalf("should pass: %v", err)
	}

	// Claude 超限
	err = qm.CheckQuota("user1", 200, ProviderClaude, PeriodPerDay, 400, 0)
	if err != ErrQuotaExceeded {
		t.Fatalf("expected ErrQuotaExceeded for Claude, got %v", err)
	}
}

// ========== BudgetManager 测试 ==========

func TestBudgetManager_SetAndGet(t *testing.T) {
	bm := NewBudgetManager(nil)

	budget := Budget{
		ID:     "b1",
		Name:   "Monthly Budget",
		Type:   BudgetTypeGlobal,
		Amount: 100.0,
		Period: PeriodPerMonth,
	}
	bm.SetBudget(budget)

	got, ok := bm.GetBudget("b1")
	if !ok {
		t.Fatal("budget not found")
	}
	if got.Amount != 100.0 {
		t.Fatalf("expected 100.0, got %f", got.Amount)
	}
}

func TestBudgetManager_Spend(t *testing.T) {
	alertCh := make(chan Alert, 10)
	bm := NewBudgetManager(func(alert Alert) {
		alertCh <- alert
	})

	bm.SetBudget(Budget{
		ID:             "b1",
		Type:           BudgetTypeGlobal,
		Amount:         100.0,
		AlertThreshold: 0.8,
		Enabled:        true,
	})

	// 正常消耗
	err := bm.Spend("b1", 50.0)
	if err != nil {
		t.Fatalf("spend should pass: %v", err)
	}

	// 触发告警阈值
	err = bm.Spend("b1", 35.0)
	if err != nil {
		t.Fatalf("spend should pass (alert only): %v", err)
	}

	// 等待告警
	select {
	case alert := <-alertCh:
		if alert.Level != AlertLevelWarning {
			t.Fatalf("expected warning alert, got %s", alert.Level)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for alert")
	}

	// 超限
	err = bm.Spend("b1", 20.0)
	if err != ErrBudgetExceeded {
		t.Fatalf("expected ErrBudgetExceeded, got %v", err)
	}

	// 等待超限告警
	select {
	case alert := <-alertCh:
		if alert.Level != AlertLevelCritical {
			t.Fatalf("expected critical alert, got %s", alert.Level)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for critical alert")
	}
}

func TestBudgetManager_Disabled(t *testing.T) {
	bm := NewBudgetHandler(nil)

	bm.SetBudget(Budget{
		ID:      "b1",
		Amount:  100.0,
		Enabled: false,
	})

	err := bm.Spend("b1", 999.0)
	if err != nil {
		t.Fatalf("disabled budget should not block: %v", err)
	}
}

func TestBudgetManager_CheckBudget(t *testing.T) {
	bm := NewBudgetHandler(nil)

	bm.SetBudget(Budget{
		ID:      "b1",
		Amount:  100.0,
		Enabled: true,
	})

	// 未超限
	err := bm.CheckBudget("b1", 50.0)
	if err != nil {
		t.Fatalf("check should pass: %v", err)
	}

	// 模拟已花费
	bm.Spend("b1", 80.0)

	// 超限
	err = bm.CheckBudget("b1", 30.0)
	if err != ErrBudgetExceeded {
		t.Fatalf("expected ErrBudgetExceeded, got %v", err)
	}
}

func TestBudgetManager_FindByTarget(t *testing.T) {
	bm := NewBudgetHandler(nil)

	bm.SetBudget(Budget{ID: "b1", Type: BudgetTypeUser, TargetID: "user1", Amount: 100, Enabled: true})
	bm.SetBudget(Budget{ID: "b2", Type: BudgetTypeUser, TargetID: "user1", Amount: 200, Enabled: true})
	bm.SetBudget(Budget{ID: "b3", Type: BudgetTypeUser, TargetID: "user2", Amount: 300, Enabled: true})
	bm.SetBudget(Budget{ID: "b4", Type: BudgetTypeGlobal, TargetID: "", Amount: 1000, Enabled: true})

	budgets := bm.FindBudgetsByTarget("user1", BudgetTypeUser)
	if len(budgets) != 2 {
		t.Fatalf("expected 2 budgets, got %d", len(budgets))
	}
}

// ========== Manager 集成测试 ==========

func TestManager_RecordUsage(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AuditLogSize = 100
	m := NewManager(cfg)

	limits := []RateLimit{
		{UserID: "user1", MaxTokens: 10000, Window: time.Minute, MaxRequests: 100},
	}

	usage := TokenUsage{
		UserID:           "user1",
		Provider:         ProviderOpenAI,
		Model:            "gpt-4",
		PromptTokens:     100,
		CompletionTokens: 200,
		TotalTokens:      300,
		Cost:             0.01,
	}

	err := m.RecordUsage(usage, limits)
	if err != nil {
		t.Fatalf("record usage failed: %v", err)
	}

	if m.UsageCount() != 1 {
		t.Fatalf("expected 1, got %d", m.UsageCount())
	}

	// 检查审计日志
	logs := m.RecentAuditLogs(10)
	found := false
	for _, l := range logs {
		if l.Action == AuditActionRecord {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("audit log not recorded")
	}
}

func TestManager_InvalidUsage(t *testing.T) {
	m := NewManager(DefaultConfig())

	err := m.RecordUsage(TokenUsage{UserID: "user1", TotalTokens: 0}, nil)
	if err != ErrInvalidParams {
		t.Fatalf("expected ErrInvalidParams, got %v", err)
	}
}

func TestManager_QuotaIntegration(t *testing.T) {
	m := NewManager(DefaultConfig())

	// 设置配额: 每日 1000
	m.SetQuota(UserQuota{
		UserID:  "user1",
		Limits:  map[QuotaPeriod]int{PeriodPerDay: 1000},
		Enabled: true,
	})

	limits := []RateLimit{
		{UserID: "user1", MaxTokens: 10000, Window: time.Minute, MaxRequests: 100},
	}

	// 用量应通过
	err := m.RecordUsage(TokenUsage{UserID: "user1", TotalTokens: 500, Timestamp: time.Now()}, limits)
	if err != nil {
		t.Fatalf("first record should pass: %v", err)
	}
}

func TestManager_PlanIntegration(t *testing.T) {
	m := NewManager(DefaultConfig())

	m.SetPlan(Plan{
		ID:          "pro",
		Name:        "Pro Plan",
		TokenLimits: map[QuotaPeriod]int{PeriodPerDay: 5000},
		Enabled:     true,
	})

	err := m.AssignPlan("user1", "pro")
	if err != nil {
		t.Fatalf("assign plan failed: %v", err)
	}

	quota, ok := m.GetQuota("user1")
	if !ok {
		t.Fatal("quota not created")
	}
	if quota.PlanID != "pro" {
		t.Fatalf("expected pro, got %s", quota.PlanID)
	}
}

func TestManager_BudgetIntegration(t *testing.T) {
	m := NewManager(DefaultConfig())

	m.SetBudget(Budget{
		ID:      "global",
		Type:    BudgetTypeGlobal,
		Amount:  10.0,
		Enabled: true,
	})

	limits := []RateLimit{
		{UserID: "user1", MaxTokens: 100000, Window: time.Minute, MaxRequests: 100},
	}

	// 正常消耗
	err := m.RecordUsage(TokenUsage{UserID: "user1", TotalTokens: 100, Cost: 1.0, Timestamp: time.Now()}, limits)
	if err != nil {
		t.Fatalf("should pass: %v", err)
	}

	// 超限
	err = m.RecordUsage(TokenUsage{UserID: "user1", TotalTokens: 100, Cost: 10.0, Timestamp: time.Now()}, limits)
	if err != ErrBudgetExceeded {
		t.Fatalf("expected ErrBudgetExceeded, got %v", err)
	}
}

func TestManager_ConcurrentRecord(t *testing.T) {
	m := NewManager(DefaultConfig())

	limits := []RateLimit{
		{UserID: "user1", MaxTokens: 1000000, Window: time.Minute, MaxRequests: 10000},
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.RecordUsage(TokenUsage{
				UserID:      "user1",
				TotalTokens: 10,
				Timestamp:   time.Now(),
			}, limits)
		}()
	}
	wg.Wait()

	if m.UsageCount() != 50 {
		t.Fatalf("expected 50, got %d", m.UsageCount())
	}
}

func TestManager_RecentUsage(t *testing.T) {
	m := NewManager(DefaultConfig())

	limits := []RateLimit{
		{UserID: "user1", MaxTokens: 100000, Window: time.Minute, MaxRequests: 100},
	}

	for i := 0; i < 5; i++ {
		m.RecordUsage(TokenUsage{UserID: "user1", TotalTokens: 100, Timestamp: time.Now()}, limits)
	}

	recent := m.RecentUsage(3)
	if len(recent) != 3 {
		t.Fatalf("expected 3, got %d", len(recent))
	}
}

func TestManager_AuditLogs(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AuditLogSize = 10
	m := NewManager(cfg)

	limits := []RateLimit{
		{UserID: "user1", MaxTokens: 100000, Window: time.Minute, MaxRequests: 100},
	}

	for i := 0; i < 15; i++ {
		m.RecordUsage(TokenUsage{UserID: "user1", TotalTokens: 10, Timestamp: time.Now()}, limits)
	}

	// 环形缓冲区应只保留最近 10 条
	logs := m.RecentAuditLogs(100)
	if len(logs) > 10 {
		t.Fatalf("expected at most 10 logs, got %d", len(logs))
	}
}

// ========== ringBuffer 测试 ==========

func TestRingBuffer_Basic(t *testing.T) {
	rb := newRingBuffer(3)

	rb.append(AuditLog{ID: "1"})
	rb.append(AuditLog{ID: "2"})
	rb.append(AuditLog{ID: "3"})

	if rb.count() != 3 {
		t.Fatalf("expected 3, got %d", rb.count())
	}

	recent := rb.recent(2)
	if len(recent) != 2 {
		t.Fatalf("expected 2, got %d", len(recent))
	}
	if recent[0].ID != "2" || recent[1].ID != "3" {
		t.Fatalf("unexpected order: %v", recent)
	}
}

func TestRingBuffer_Overflow(t *testing.T) {
	rb := newRingBuffer(3)

	for i := 0; i < 5; i++ {
		rb.append(AuditLog{ID: intToStr(i)})
	}

	if rb.count() != 3 {
		t.Fatalf("expected 3, got %d", rb.count())
	}

	recent := rb.recent(3)
	if recent[0].ID != "2" || recent[2].ID != "4" {
		t.Fatalf("unexpected overflow result: %v", recent)
	}
}

// ========== 辅助函数 ==========

func NewBudgetHandler(h AlertHandler) *BudgetManager {
	return NewBudgetManager(h)
}
