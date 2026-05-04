package smartalert

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	return NewEngine(zap.NewNop())
}

// ========== Ingest 测试 ==========

func TestIngest_CreatesAlert(t *testing.T) {
	e := newTestEngine(t)

	alert := e.Ingest(
		"磁盘SMART检测异常",
		"/dev/sda SMART指标异常",
		CategoryDisk,
		SeverityCritical,
		"smartctl",
		"/dev/sda",
		nil,
	)

	assert.NotEmpty(t, alert.ID)
	assert.Equal(t, "磁盘SMART检测异常", alert.Title)
	assert.Equal(t, SeverityCritical, alert.Severity)
	assert.Equal(t, CategoryDisk, alert.Category)
	assert.Equal(t, StateActive, alert.State)
	assert.Equal(t, SeverityCritical, alert.OriginalSeverity)
}

func TestIngest_DeduplicatesAlerts(t *testing.T) {
	e := newTestEngine(t)

	a1 := e.Ingest("磁盘故障", "sda坏了", CategoryDisk, SeverityWarning, "test", "/dev/sda", nil)
	a2 := e.Ingest("磁盘故障", "sda还是坏的", CategoryDisk, SeverityCritical, "test", "/dev/sda", nil)

	assert.Equal(t, a1.ID, a2.ID, "相同title+resource应去重")
	assert.Equal(t, "sda还是坏的", a2.Description, "应更新描述")
}

func TestIngest_KnowledgeMatch(t *testing.T) {
	e := newTestEngine(t)

	alert := e.Ingest(
		"SMART检测异常",
		"磁盘健康指标下降",
		CategoryDisk,
		SeverityCritical,
		"smartctl",
		"/dev/sda",
		nil,
	)

	assert.NotEmpty(t, alert.TroubleshootSteps, "应匹配知识库获得排查步骤")
	assert.NotEmpty(t, alert.FixCommands, "应匹配知识库获得修复命令")
	assert.NotEmpty(t, alert.References, "应匹配知识库获得参考链接")
	assert.Equal(t, "disk-hardware-failure", alert.RootCauseID)
}

func TestIngest_KnowledgeMatchSpace(t *testing.T) {
	e := newTestEngine(t)

	alert := e.Ingest(
		"存储空间不足",
		"使用率超过90%",
		CategorySpace,
		SeverityWarning,
		"monitor",
		"/pool",
		nil,
	)

	assert.NotEmpty(t, alert.TroubleshootSteps, "空间不足应匹配排查步骤")
	assert.Equal(t, "storage-capacity", alert.RootCauseID)
}

func TestIngest_KnowledgeMatchSecurity(t *testing.T) {
	e := newTestEngine(t)

	alert := e.Ingest(
		"暴力破解检测",
		"检测到来自192.168.1.100的大量SSH登录失败",
		CategorySecurity,
		SeverityCritical,
		"fail2ban",
		"ssh",
		nil,
	)

	assert.NotEmpty(t, alert.TroubleshootSteps)
	assert.Equal(t, "security-attack", alert.RootCauseID)
}

func TestIngest_CorrelatesAlerts(t *testing.T) {
	e := newTestEngine(t)

	// 两条告警共享同一根因
	e.Ingest("SMART异常1", "磁盘健康指标下降", CategoryDisk, SeverityWarning, "test", "/dev/sda", nil)
	e.Ingest("SMART异常2", "磁盘健康指标下降", CategoryDisk, SeverityWarning, "test", "/dev/sdb", nil)

	guide, err := e.GetGuide(e.List(ListQuery{})[0].ID)
	require.NoError(t, err)
	if guide.Correlation != nil {
		assert.GreaterOrEqual(t, len(guide.Correlation.RelatedAlertIDs), 1)
	}
}

// ========== List 测试 ==========

func TestList_All(t *testing.T) {
	e := newTestEngine(t)
	e.Ingest("告警1", "desc1", CategoryDisk, SeverityCritical, "test", "r1", nil)
	e.Ingest("告警2", "desc2", CategoryNetwork, SeverityWarning, "test", "r2", nil)

	all := e.List(ListQuery{})
	assert.Len(t, all, 2)
}

func TestList_FilterByCategory(t *testing.T) {
	e := newTestEngine(t)
	e.Ingest("磁盘告警", "desc", CategoryDisk, SeverityCritical, "test", "r1", nil)
	e.Ingest("网络告警", "desc", CategoryNetwork, SeverityWarning, "test", "r2", nil)

	disk := e.List(ListQuery{Category: CategoryDisk})
	assert.Len(t, disk, 1)
	assert.Equal(t, CategoryDisk, disk[0].Category)
}

func TestList_FilterBySeverity(t *testing.T) {
	e := newTestEngine(t)
	e.Ingest("严重告警", "desc", CategoryDisk, SeverityCritical, "test", "r1", nil)
	e.Ingest("普通告警", "desc", CategoryDisk, SeverityInfo, "test", "r2", nil)

	crit := e.List(ListQuery{Severity: SeverityCritical})
	assert.Len(t, crit, 1)
}

func TestList_SortedBySeverity(t *testing.T) {
	e := newTestEngine(t)
	e.Ingest("信息", "desc", CategoryDisk, SeverityInfo, "test", "r1", nil)
	e.Ingest("严重", "desc", CategoryDisk, SeverityCritical, "test", "r2", nil)
	e.Ingest("警告", "desc", CategoryDisk, SeverityWarning, "test", "r3", nil)

	list := e.List(ListQuery{})
	require.Len(t, list, 3)
	assert.Equal(t, SeverityCritical, list[0].Severity, "严重告警应排在最前")
	assert.Equal(t, SeverityWarning, list[1].Severity)
	assert.Equal(t, SeverityInfo, list[2].Severity)
}

// ========== Guide 测试 ==========

func TestGetGuide_Existing(t *testing.T) {
	e := newTestEngine(t)
	alert := e.Ingest("磁盘告警", "desc", CategoryDisk, SeverityCritical, "test", "/dev/sda", nil)

	guide, err := e.GetGuide(alert.ID)
	require.NoError(t, err)
	assert.Equal(t, alert.ID, guide.Alert.ID)
	assert.NotEmpty(t, guide.Summary)
}

func TestGetGuide_NotFound(t *testing.T) {
	e := newTestEngine(t)

	_, err := e.GetGuide("nonexistent-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ========== Acknowledge 测试 ==========

func TestAcknowledge_Success(t *testing.T) {
	e := newTestEngine(t)
	alert := e.Ingest("告警", "desc", CategoryDisk, SeverityWarning, "test", "r1", nil)

	err := e.Acknowledge(alert.ID, "admin")
	require.NoError(t, err)

	guide, _ := e.GetGuide(alert.ID)
	assert.Equal(t, StateAcknowledged, guide.Alert.State)
	assert.Equal(t, "admin", guide.Alert.AcknowledgedBy)
	assert.NotNil(t, guide.Alert.AcknowledgedAt)
}

func TestAcknowledge_AlreadyResolved(t *testing.T) {
	e := newTestEngine(t)
	alert := e.Ingest("告警", "desc", CategoryDisk, SeverityWarning, "test", "r1", nil)

	_ = e.Resolve(alert.ID)
	err := e.Acknowledge(alert.ID, "admin")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already resolved")
}

func TestAcknowledge_NotFound(t *testing.T) {
	e := newTestEngine(t)
	err := e.Acknowledge("nonexistent", "admin")
	assert.Error(t, err)
}

// ========== Silence 测试 ==========

func TestAddSilence_Success(t *testing.T) {
	e := newTestEngine(t)

	rule := e.AddSilence(SilenceRequest{
		Name:        "静默磁盘告警",
		Description: "维护期间静默",
		Category:    CategoryDisk,
		DurationMin: 60,
		CreatedBy:   "admin",
	})

	assert.NotEmpty(t, rule.ID)
	assert.Equal(t, "静默磁盘告警", rule.Name)
	assert.True(t, rule.Enabled)
	assert.True(t, rule.EndTime.After(time.Now()))
}

func TestListSilences(t *testing.T) {
	e := newTestEngine(t)

	e.AddSilence(SilenceRequest{Name: "规则1", DurationMin: 30})
	e.AddSilence(SilenceRequest{Name: "规则2", DurationMin: 60})

	rules := e.ListSilences()
	assert.Len(t, rules, 2)
}

func TestRemoveSilence_Success(t *testing.T) {
	e := newTestEngine(t)

	rule := e.AddSilence(SilenceRequest{Name: "temp", DurationMin: 30})
	err := e.RemoveSilence(rule.ID)
	assert.NoError(t, err)

	rules := e.ListSilences()
	assert.Len(t, rules, 0)
}

func TestRemoveSilence_NotFound(t *testing.T) {
	e := newTestEngine(t)
	err := e.RemoveSilence("nonexistent")
	assert.Error(t, err)
}

// ========== Escalation 测试 ==========

func TestRunEscalation_UpgradeAlert(t *testing.T) {
	e := newTestEngine(t)
	// 设置短升级时间方便测试
	e.escPolicy.UpgradeAfter = 0 // 立即升级

	alert := e.Ingest("告警", "desc", CategoryDisk, SeverityInfo, "test", "r1", nil)
	// 手动设置 LastSeen 为很久之前
	e.mu.Lock()
	e.alerts[alert.ID].LastSeen = time.Now().Add(-time.Hour)
	e.mu.Unlock()

	count := e.RunEscalation()
	assert.Equal(t, 1, count)

	updated := e.List(ListQuery{})[0]
	assert.Equal(t, SeverityWarning, updated.Severity, "info应升级为warning")
	assert.Equal(t, StateEscalated, updated.State)
	assert.NotNil(t, updated.EscalatedAt)
}

func TestRunEscalation_NoUpgradeForCritical(t *testing.T) {
	e := newTestEngine(t)
	e.escPolicy.UpgradeAfter = 0

	alert := e.Ingest("告警", "desc", CategoryDisk, SeverityCritical, "test", "r1", nil)
	e.mu.Lock()
	e.alerts[alert.ID].LastSeen = time.Now().Add(-time.Hour)
	e.mu.Unlock()

	count := e.RunEscalation()
	assert.Equal(t, 0, count, "critical已经是最高级不应再升级")
}

func TestRunEscalation_CleansExpiredSilences(t *testing.T) {
	e := newTestEngine(t)

	// 创建一个已过期的静默规则
	e.mu.Lock()
	e.silences["expired"] = &SilenceRule{
		ID:        "expired",
		Name:      "过期规则",
		StartTime: time.Now().Add(-2 * time.Hour),
		EndTime:   time.Now().Add(-1 * time.Hour), // 已过期
		Enabled:   true,
	}
	e.mu.Unlock()

	e.RunEscalation()

	e.mu.RLock()
	_, exists := e.silences["expired"]
	e.mu.RUnlock()

	assert.False(t, exists, "过期的静默规则应被清除")
}

// ========== Resolve 测试 ==========

func TestResolve_Success(t *testing.T) {
	e := newTestEngine(t)
	alert := e.Ingest("告警", "desc", CategoryDisk, SeverityWarning, "test", "r1", nil)

	err := e.Resolve(alert.ID)
	require.NoError(t, err)

	guide, _ := e.GetGuide(alert.ID)
	assert.Equal(t, StateResolved, guide.Alert.State)
	assert.Equal(t, SeverityResolved, guide.Alert.Severity)
	assert.NotNil(t, guide.Alert.ResolvedAt)
}

func TestResolve_NotFound(t *testing.T) {
	e := newTestEngine(t)
	err := e.Resolve("nonexistent")
	assert.Error(t, err)
}

// ========== escalateSeverity 测试 ==========

func TestEscalateSeverity(t *testing.T) {
	assert.Equal(t, SeverityWarning, escalateSeverity(SeverityInfo))
	assert.Equal(t, SeverityCritical, escalateSeverity(SeverityWarning))
	assert.Equal(t, SeverityCritical, escalateSeverity(SeverityCritical))
}
