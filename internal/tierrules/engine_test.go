package tierrules

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ========== 辅助函数 ==========

func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	return NewEngine(logger)
}

func newTestHandlers(t *testing.T, engine *Engine) *Handlers {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	return NewHandlers(engine, logger)
}

func setupRouter(h *Handlers) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/api/v1")
	h.RegisterRoutes(rg)
	return r
}

func makeTestFile(name string, tier StorageTier, size int64, accessDaysAgo, ageDaysAgo int) FileInfo {
	return FileInfo{
		Path:        "/data/" + name,
		Name:        name,
		Size:        size,
		ModTime:     time.Now().AddDate(0, 0, -ageDaysAgo),
		AccessTime:  time.Now().AddDate(0, 0, -accessDaysAgo),
		CurrentTier: tier,
	}
}

// ========== types.go 测试 ==========

func TestValidTiers(t *testing.T) {
	tiers := []StorageTier{TierNVMe, TierSSD, TierHDD, TierCloud, TierArchive}
	for _, tier := range tiers {
		if !ValidTiers[tier] {
			t.Errorf("Tier %s should be valid", tier)
		}
	}
	if ValidTiers["invalid"] {
		t.Error("invalid tier should not be valid")
	}
}

func TestTierOrder(t *testing.T) {
	if TierOrder[TierNVMe] >= TierOrder[TierSSD] {
		t.Error("NVMe should be faster than SSD")
	}
	if TierOrder[TierSSD] >= TierOrder[TierHDD] {
		t.Error("SSD should be faster than HDD")
	}
	if TierOrder[TierHDD] >= TierOrder[TierCloud] {
		t.Error("HDD should be faster than Cloud")
	}
	if TierOrder[TierCloud] >= TierOrder[TierArchive] {
		t.Error("Cloud should be faster than Archive")
	}
}

// ========== engine.go 测试 ==========

func TestNewEngine(t *testing.T) {
	e := newTestEngine(t)
	if e == nil {
		t.Fatal("NewEngine returned nil")
	}
	if len(e.rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(e.rules))
	}
}

func TestAddRule_Success(t *testing.T) {
	e := newTestEngine(t)
	rule := TierRule{
		Name:       "cold-to-hdd",
		SourceTier: TierSSD,
		TargetTier: TierHDD,
		Priority:   10,
		Enabled:    true,
		Conditions: TierConditions{MaxAccessDays: 30},
	}
	if err := e.AddRule(rule); err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}
	if len(e.rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(e.rules))
	}
}

func TestAddRule_EmptyName(t *testing.T) {
	e := newTestEngine(t)
	rule := TierRule{SourceTier: TierSSD, TargetTier: TierHDD}
	if err := e.AddRule(rule); !errors.Is(err, ErrRuleNameEmpty) {
		t.Errorf("expected ErrRuleNameEmpty, got %v", err)
	}
}

func TestAddRule_DuplicateName(t *testing.T) {
	e := newTestEngine(t)
	rule := TierRule{Name: "r1", SourceTier: TierSSD, TargetTier: TierHDD, Enabled: true}
	e.AddRule(rule)
	if err := e.AddRule(rule); !errors.Is(err, ErrRuleNameDuplicate) {
		t.Errorf("expected ErrRuleNameDuplicate, got %v", err)
	}
}

func TestAddRule_InvalidTier(t *testing.T) {
	e := newTestEngine(t)
	rule := TierRule{Name: "bad", SourceTier: "tape", TargetTier: TierHDD}
	if err := e.AddRule(rule); !errors.Is(err, ErrInvalidTier) {
		t.Errorf("expected ErrInvalidTier, got %v", err)
	}
}

func TestAddRule_SameTier(t *testing.T) {
	e := newTestEngine(t)
	rule := TierRule{Name: "same", SourceTier: TierSSD, TargetTier: TierSSD}
	if err := e.AddRule(rule); !errors.Is(err, ErrSameTier) {
		t.Errorf("expected ErrSameTier, got %v", err)
	}
}

func TestRemoveRule_Success(t *testing.T) {
	e := newTestEngine(t)
	e.AddRule(TierRule{Name: "r1", SourceTier: TierSSD, TargetTier: TierHDD, Enabled: true})
	if err := e.RemoveRule("r1"); err != nil {
		t.Fatalf("RemoveRule failed: %v", err)
	}
	if len(e.rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(e.rules))
	}
}

func TestRemoveRule_NotFound(t *testing.T) {
	e := newTestEngine(t)
	if err := e.RemoveRule("nope"); !errors.Is(err, ErrRuleNotFound) {
		t.Errorf("expected ErrRuleNotFound, got %v", err)
	}
}

func TestListRules_SortedByPriority(t *testing.T) {
	e := newTestEngine(t)
	e.AddRule(TierRule{Name: "low", SourceTier: TierSSD, TargetTier: TierHDD, Priority: 1, Enabled: true})
	e.AddRule(TierRule{Name: "high", SourceTier: TierSSD, TargetTier: TierHDD, Priority: 100, Enabled: true})
	e.AddRule(TierRule{Name: "mid", SourceTier: TierSSD, TargetTier: TierHDD, Priority: 50, Enabled: true})

	rules := e.ListRules()
	if len(rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(rules))
	}
	if rules[0].Name != "high" || rules[1].Name != "mid" || rules[2].Name != "low" {
		t.Errorf("rules not sorted by priority: %s, %s, %s", rules[0].Name, rules[1].Name, rules[2].Name)
	}
}

func TestEvaluate_AccessDays(t *testing.T) {
	e := newTestEngine(t)
	e.AddRule(TierRule{
		Name:       "archive-old",
		SourceTier: TierSSD,
		TargetTier: TierArchive,
		Priority:   10,
		Enabled:    true,
		Conditions: TierConditions{MaxAccessDays: 30},
	})

	// 文件40天未访问 → 应匹配
	file := makeTestFile("old.txt", TierSSD, 1024, 40, 100)
	tier, err := e.Evaluate(file)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if StorageTier(tier) != TierArchive {
		t.Errorf("expected TierArchive, got %s", tier)
	}

	// 文件10天前访问 → 不应匹配
	file2 := makeTestFile("recent.txt", TierSSD, 1024, 10, 100)
	_, err = e.Evaluate(file2)
	if !errors.Is(err, ErrNoMatchingRule) {
		t.Errorf("expected ErrNoMatchingRule, got %v", err)
	}
}

func TestEvaluate_MinAge(t *testing.T) {
	e := newTestEngine(t)
	e.AddRule(TierRule{
		Name:       "old-files",
		SourceTier: TierHDD,
		TargetTier: TierCloud,
		Priority:   10,
		Enabled:    true,
		Conditions: TierConditions{MinAgeDays: 90},
	})

	// 文件100天前修改 → 匹配
	file := makeTestFile("archive.zip", TierHDD, 5000, 5, 100)
	tier, err := e.Evaluate(file)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if StorageTier(tier) != TierCloud {
		t.Errorf("expected TierCloud, got %s", tier)
	}

	// 文件30天前修改 → 不匹配
	file2 := makeTestFile("new.zip", TierHDD, 5000, 5, 30)
	_, err = e.Evaluate(file2)
	if !errors.Is(err, ErrNoMatchingRule) {
		t.Errorf("expected ErrNoMatchingRule, got %v", err)
	}
}

func TestEvaluate_FilePatterns(t *testing.T) {
	e := newTestEngine(t)
	e.AddRule(TierRule{
		Name:       "logs-to-archive",
		SourceTier: TierHDD,
		TargetTier: TierArchive,
		Priority:   10,
		Enabled:    true,
		Conditions: TierConditions{FilePatterns: []string{"*.log", "*.tmp"}},
	})

	file := makeTestFile("app.log", TierHDD, 1024, 60, 60)
	tier, err := e.Evaluate(file)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if StorageTier(tier) != TierArchive {
		t.Errorf("expected TierArchive, got %s", tier)
	}

	// 不匹配的文件
	file2 := makeTestFile("data.csv", TierHDD, 1024, 60, 60)
	_, err = e.Evaluate(file2)
	if !errors.Is(err, ErrNoMatchingRule) {
		t.Errorf("expected ErrNoMatchingRule, got %v", err)
	}
}

func TestEvaluate_DotPattern(t *testing.T) {
	e := newTestEngine(t)
	e.AddRule(TierRule{
		Name:       "dot-log",
		SourceTier: TierHDD,
		TargetTier: TierArchive,
		Priority:   10,
		Enabled:    true,
		Conditions: TierConditions{FilePatterns: []string{".log"}},
	})

	file := makeTestFile("test.log", TierHDD, 1024, 60, 60)
	tier, err := e.Evaluate(file)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if StorageTier(tier) != TierArchive {
		t.Errorf("expected TierArchive, got %s", tier)
	}
}

func TestEvaluate_FileSize(t *testing.T) {
	e := newTestEngine(t)
	e.AddRule(TierRule{
		Name:       "large-files",
		SourceTier: TierSSD,
		TargetTier: TierHDD,
		Priority:   10,
		Enabled:    true,
		Conditions: TierConditions{MinSizeBytes: 1024 * 1024},
	})

	// 大文件 → 匹配
	file := makeTestFile("video.mp4", TierSSD, 5*1024*1024, 60, 60)
	tier, err := e.Evaluate(file)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if StorageTier(tier) != TierHDD {
		t.Errorf("expected TierHDD, got %s", tier)
	}

	// 小文件 → 不匹配
	file2 := makeTestFile("tiny.txt", TierSSD, 100, 60, 60)
	_, err = e.Evaluate(file2)
	if !errors.Is(err, ErrNoMatchingRule) {
		t.Errorf("expected ErrNoMatchingRule, got %v", err)
	}
}

func TestEvaluate_SizeRange(t *testing.T) {
	e := newTestEngine(t)
	e.AddRule(TierRule{
		Name:       "medium-files",
		SourceTier: TierSSD,
		TargetTier: TierHDD,
		Priority:   10,
		Enabled:    true,
		Conditions: TierConditions{MinSizeBytes: 1000, MaxSizeBytes: 5000},
	})

	file := makeTestFile("mid.dat", TierSSD, 3000, 60, 60)
	tier, err := e.Evaluate(file)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if StorageTier(tier) != TierHDD {
		t.Errorf("expected TierHDD, got %s", tier)
	}

	// 超出范围
	file2 := makeTestFile("big.dat", TierSSD, 10000, 60, 60)
	_, err = e.Evaluate(file2)
	if !errors.Is(err, ErrNoMatchingRule) {
		t.Errorf("expected ErrNoMatchingRule for out-of-range size, got %v", err)
	}
}

func TestEvaluate_SourceTierMismatch(t *testing.T) {
	e := newTestEngine(t)
	e.AddRule(TierRule{
		Name:       "ssd-only",
		SourceTier: TierSSD,
		TargetTier: TierHDD,
		Priority:   10,
		Enabled:    true,
		Conditions: TierConditions{MaxAccessDays: 1},
	})

	// HDD 文件不应该匹配 SSD→HDD 规则
	file := makeTestFile("data.bin", TierHDD, 1024, 60, 60)
	_, err := e.Evaluate(file)
	if !errors.Is(err, ErrNoMatchingRule) {
		t.Errorf("expected ErrNoMatchingRule for tier mismatch, got %v", err)
	}
}

func TestEvaluate_DisabledRule(t *testing.T) {
	e := newTestEngine(t)
	e.AddRule(TierRule{
		Name:       "disabled",
		SourceTier: TierSSD,
		TargetTier: TierHDD,
		Priority:   10,
		Enabled:    false,
		Conditions: TierConditions{MaxAccessDays: 1},
	})

	file := makeTestFile("data.bin", TierSSD, 1024, 60, 60)
	_, err := e.Evaluate(file)
	if !errors.Is(err, ErrNoMatchingRule) {
		t.Errorf("expected ErrNoMatchingRule for disabled rule, got %v", err)
	}
}

func TestEvaluate_PriorityOrder(t *testing.T) {
	e := newTestEngine(t)
	// 低优先级：SSD → HDD
	e.AddRule(TierRule{
		Name:       "low",
		SourceTier: TierSSD,
		TargetTier: TierHDD,
		Priority:   1,
		Enabled:    true,
		Conditions: TierConditions{MaxAccessDays: 5},
	})
	// 高优先级：SSD → Archive
	e.AddRule(TierRule{
		Name:       "high",
		SourceTier: TierSSD,
		TargetTier: TierArchive,
		Priority:   100,
		Enabled:    true,
		Conditions: TierConditions{MaxAccessDays: 5},
	})

	file := makeTestFile("data.bin", TierSSD, 1024, 60, 60)
	tier, err := e.Evaluate(file)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	// 高优先级规则应该先匹配
	if StorageTier(tier) != TierArchive {
		t.Errorf("expected TierArchive from high priority rule, got %s", tier)
	}
}

func TestEvaluate_EmptyCurrentTier(t *testing.T) {
	e := newTestEngine(t)
	file := FileInfo{Path: "/test", Name: "test.txt", Size: 100}
	_, err := e.Evaluate(file)
	if err == nil {
		t.Error("expected error for empty current tier")
	}
}

func TestEvaluate_CombinedConditions(t *testing.T) {
	e := newTestEngine(t)
	e.AddRule(TierRule{
		Name:       "combined",
		SourceTier: TierSSD,
		TargetTier: TierCloud,
		Priority:   10,
		Enabled:    true,
		Conditions: TierConditions{
			MaxAccessDays: 30,
			MinAgeDays:    60,
			FilePatterns:  []string{"*.bak"},
			MinSizeBytes:  100,
			MaxSizeBytes:  10000,
		},
	})

	// 满足所有条件
	file := makeTestFile("data.bak", TierSSD, 5000, 40, 80)
	tier, err := e.Evaluate(file)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if StorageTier(tier) != TierCloud {
		t.Errorf("expected TierCloud, got %s", tier)
	}

	// 不满足文件名模式
	file2 := makeTestFile("data.log", TierSSD, 5000, 40, 80)
	_, err = e.Evaluate(file2)
	if !errors.Is(err, ErrNoMatchingRule) {
		t.Errorf("expected ErrNoMatchingRule, got %v", err)
	}
}

// ========== RunBatch 测试 ==========

func TestRunBatch_Success(t *testing.T) {
	e := newTestEngine(t)
	e.AddRule(TierRule{
		Name:       "archive-cold",
		SourceTier: TierSSD,
		TargetTier: TierArchive,
		Priority:   10,
		Enabled:    true,
		Conditions: TierConditions{MaxAccessDays: 30},
	})

	files := []FileInfo{
		makeTestFile("old1.log", TierSSD, 1024, 60, 100),
		makeTestFile("old2.log", TierSSD, 2048, 90, 200),
		makeTestFile("new.log", TierSSD, 512, 5, 10), // 不满足条件
	}

	e.SetFileListFunc(func(ctx context.Context) ([]FileInfo, error) {
		return files, nil
	})

	migrated := 0
	e.SetMigrateFunc(func(ctx context.Context, file FileInfo, target StorageTier) error {
		migrated++
		if target != TierArchive {
			t.Errorf("expected target TierArchive, got %s", target)
		}
		return nil
	})

	stats, err := e.RunBatch(context.Background())
	if err != nil {
		t.Fatalf("RunBatch failed: %v", err)
	}
	if stats.TotalMoved != 2 {
		t.Errorf("expected 2 moved, got %d", stats.TotalMoved)
	}
	if stats.TotalBytes != 3072 {
		t.Errorf("expected 3072 bytes, got %d", stats.TotalBytes)
	}
	if stats.ErrorCount != 0 {
		t.Errorf("expected 0 errors, got %d", stats.ErrorCount)
	}
}

func TestRunBatch_MigrateError(t *testing.T) {
	e := newTestEngine(t)
	e.AddRule(TierRule{
		Name:       "fail-rule",
		SourceTier: TierSSD,
		TargetTier: TierHDD,
		Priority:   10,
		Enabled:    true,
		Conditions: TierConditions{MaxAccessDays: 1},
	})

	e.SetFileListFunc(func(ctx context.Context) ([]FileInfo, error) {
		return []FileInfo{makeTestFile("fail.bin", TierSSD, 1024, 60, 60)}, nil
	})
	e.SetMigrateFunc(func(ctx context.Context, file FileInfo, target StorageTier) error {
		return errors.New("disk full")
	})

	stats, err := e.RunBatch(context.Background())
	if err != nil {
		t.Fatalf("RunBatch should not return error for individual failures: %v", err)
	}
	if stats.ErrorCount != 1 {
		t.Errorf("expected 1 error, got %d", stats.ErrorCount)
	}
	if stats.TotalMoved != 0 {
		t.Errorf("expected 0 moved, got %d", stats.TotalMoved)
	}
}

func TestRunBatch_ContextCancelled(t *testing.T) {
	e := newTestEngine(t)
	e.AddRule(TierRule{
		Name:       "cancel",
		SourceTier: TierSSD,
		TargetTier: TierHDD,
		Priority:   10,
		Enabled:    true,
		Conditions: TierConditions{MaxAccessDays: 1},
	})

	e.SetFileListFunc(func(ctx context.Context) ([]FileInfo, error) {
		return []FileInfo{makeTestFile("data.bin", TierSSD, 1024, 60, 60)}, nil
	})
	e.SetMigrateFunc(func(ctx context.Context, file FileInfo, target StorageTier) error {
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	_, err := e.RunBatch(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestRunBatch_NoListFunc(t *testing.T) {
	e := newTestEngine(t)
	_, err := e.RunBatch(context.Background())
	if err == nil {
		t.Error("expected error when no list func set")
	}
}

func TestRunBatch_NoMigrateFunc(t *testing.T) {
	e := newTestEngine(t)
	e.SetFileListFunc(func(ctx context.Context) ([]FileInfo, error) {
		return nil, nil
	})
	_, err := e.RunBatch(context.Background())
	if err == nil {
		t.Error("expected error when no migrate func set")
	}
}

func TestRunBatch_ListError(t *testing.T) {
	e := newTestEngine(t)
	e.SetFileListFunc(func(ctx context.Context) ([]FileInfo, error) {
		return nil, errors.New("permission denied")
	})
	e.SetMigrateFunc(func(ctx context.Context, file FileInfo, target StorageTier) error {
		return nil
	})
	_, err := e.RunBatch(context.Background())
	if err == nil || !contains(err.Error(), "permission denied") {
		t.Errorf("expected list error, got %v", err)
	}
}

func TestRunBatchDryRun(t *testing.T) {
	e := newTestEngine(t)
	e.AddRule(TierRule{
		Name:       "dry",
		SourceTier: TierSSD,
		TargetTier: TierHDD,
		Priority:   10,
		Enabled:    true,
		Conditions: TierConditions{MaxAccessDays: 1},
	})

	e.SetFileListFunc(func(ctx context.Context) ([]FileInfo, error) {
		return []FileInfo{makeTestFile("test.bin", TierSSD, 1024, 60, 60)}, nil
	})
	// Dry run 不需要 migrateFunc

	stats, err := e.RunBatchDryRun(context.Background())
	if err != nil {
		t.Fatalf("RunBatchDryRun failed: %v", err)
	}
	if stats.TotalMoved != 1 {
		t.Errorf("expected 1 moved (dry run), got %d", stats.TotalMoved)
	}
}

func TestGetStats(t *testing.T) {
	e := newTestEngine(t)
	stats := e.GetStats()
	if stats.TotalMoved != 0 || stats.ErrorCount != 0 {
		t.Error("initial stats should be zero")
	}
}

// ========== handlers.go 测试 ==========

func TestCreateRule_Success(t *testing.T) {
	e := newTestEngine(t)
	h := newTestHandlers(t, e)
	r := setupRouter(h)

	body := CreateRuleRequest{
		Name:       "test-rule",
		SourceTier: TierSSD,
		TargetTier: TierHDD,
		Priority:   10,
		Enabled:    true,
		Conditions: TierConditions{MaxAccessDays: 30},
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tierrules", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateRule_InvalidBody(t *testing.T) {
	e := newTestEngine(t)
	h := newTestHandlers(t, e)
	r := setupRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tierrules", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateRule_Conflict(t *testing.T) {
	e := newTestEngine(t)
	e.AddRule(TierRule{Name: "dup", SourceTier: TierSSD, TargetTier: TierHDD, Enabled: true})
	h := newTestHandlers(t, e)
	r := setupRouter(h)

	body := CreateRuleRequest{Name: "dup", SourceTier: TierSSD, TargetTier: TierHDD, Enabled: true}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tierrules", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListRules(t *testing.T) {
	e := newTestEngine(t)
	e.AddRule(TierRule{Name: "r1", SourceTier: TierSSD, TargetTier: TierHDD, Priority: 10, Enabled: true})
	e.AddRule(TierRule{Name: "r2", SourceTier: TierHDD, TargetTier: TierCloud, Priority: 20, Enabled: true})
	h := newTestHandlers(t, e)
	r := setupRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tierrules", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if int(resp["total"].(float64)) != 2 {
		t.Errorf("expected 2 rules, got %v", resp["total"])
	}
}

func TestDeleteRule_Success(t *testing.T) {
	e := newTestEngine(t)
	e.AddRule(TierRule{Name: "to-delete", SourceTier: TierSSD, TargetTier: TierHDD, Enabled: true})
	h := newTestHandlers(t, e)
	r := setupRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/tierrules/to-delete", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteRule_NotFound(t *testing.T) {
	e := newTestEngine(t)
	h := newTestHandlers(t, e)
	r := setupRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/tierrules/nonexistent", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestEvaluateFile_Match(t *testing.T) {
	e := newTestEngine(t)
	e.AddRule(TierRule{
		Name:       "ssd-to-hdd",
		SourceTier: TierSSD,
		TargetTier: TierHDD,
		Priority:   10,
		Enabled:    true,
		Conditions: TierConditions{MaxAccessDays: 30},
	})
	h := newTestHandlers(t, e)
	r := setupRouter(h)

	body := EvaluateRequest{
		File: makeTestFile("old.dat", TierSSD, 2048, 60, 100),
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tierrules/evaluate", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp EvaluateResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.ShouldMigrate {
		t.Error("expected ShouldMigrate=true")
	}
	if resp.RecommendedTier != TierHDD {
		t.Errorf("expected TierHDD, got %s", resp.RecommendedTier)
	}
}

func TestEvaluateFile_NoMatch(t *testing.T) {
	e := newTestEngine(t)
	e.AddRule(TierRule{
		Name:       "ssd-to-hdd",
		SourceTier: TierSSD,
		TargetTier: TierHDD,
		Priority:   10,
		Enabled:    true,
		Conditions: TierConditions{MaxAccessDays: 90},
	})
	h := newTestHandlers(t, e)
	r := setupRouter(h)

	body := EvaluateRequest{
		File: makeTestFile("new.dat", TierSSD, 2048, 5, 10),
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tierrules/evaluate", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp EvaluateResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.ShouldMigrate {
		t.Error("expected ShouldMigrate=false")
	}
	if resp.RecommendedTier != TierSSD {
		t.Errorf("expected current tier TierSSD, got %s", resp.RecommendedTier)
	}
}

func TestRunBatch_Handler(t *testing.T) {
	e := newTestEngine(t)
	e.AddRule(TierRule{
		Name:       "rule1",
		SourceTier: TierSSD,
		TargetTier: TierHDD,
		Priority:   10,
		Enabled:    true,
		Conditions: TierConditions{MaxAccessDays: 1},
	})
	e.SetFileListFunc(func(ctx context.Context) ([]FileInfo, error) {
		return []FileInfo{makeTestFile("a.bin", TierSSD, 1024, 60, 60)}, nil
	})
	e.SetMigrateFunc(func(ctx context.Context, file FileInfo, target StorageTier) error {
		return nil
	})

	h := newTestHandlers(t, e)
	r := setupRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tierrules/run", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRunBatch_DryRun_Handler(t *testing.T) {
	e := newTestEngine(t)
	e.AddRule(TierRule{
		Name:       "rule1",
		SourceTier: TierSSD,
		TargetTier: TierHDD,
		Priority:   10,
		Enabled:    true,
		Conditions: TierConditions{MaxAccessDays: 1},
	})
	e.SetFileListFunc(func(ctx context.Context) ([]FileInfo, error) {
		return []FileInfo{makeTestFile("a.bin", TierSSD, 1024, 60, 60)}, nil
	})

	h := newTestHandlers(t, e)
	r := setupRouter(h)

	body := RunRequest{DryRun: true}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tierrules/run", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetStats_Handler(t *testing.T) {
	e := newTestEngine(t)
	h := newTestHandlers(t, e)
	r := setupRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tierrules/stats", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var stats TierStats
	json.Unmarshal(w.Body.Bytes(), &stats)
	if stats.TotalMoved != 0 {
		t.Errorf("expected 0 total moved, got %d", stats.TotalMoved)
	}
}

// ========== 辅助函数 ==========

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
