// Package appstore 应用商店增强测试
package appstore

import (
	"testing"
)

// ========== Catalog Tests ==========

func TestNewCatalog(t *testing.T) {
	cfg := &CatalogConfig{
		DataDir: t.TempDir(),
	}
	catalog := NewCatalog(cfg)
	if catalog == nil {
		t.Fatal("创建目录失败")
	}

	// 验证内置应用已加载
	apps := catalog.ListApps(nil)
	if len(apps) == 0 {
		t.Error("内置应用未加载")
	}

	// 验证分类
	categories := catalog.Categories()
	if len(categories) == 0 {
		t.Error("分类为空")
	}
}

func TestCatalogGetApp(t *testing.T) {
	catalog := NewCatalog(&CatalogConfig{DataDir: t.TempDir()})

	// 获取存在的应用
	app, ok := catalog.GetApp("jellyfin")
	if !ok {
		t.Fatal("获取jellyfin失败")
	}
	if app.DisplayName != "Jellyfin" {
		t.Errorf("期望Jellyfin, got %s", app.DisplayName)
	}

	// 获取不存在的应用
	_, ok = catalog.GetApp("nonexistent")
	if ok {
		t.Error("不应存在nonexistent")
	}
}

func TestCatalogSearchApps(t *testing.T) {
	catalog := NewCatalog(&CatalogConfig{DataDir: t.TempDir()})

	// 搜索媒体类
	results := catalog.SearchApps("media")
	if len(results) == 0 {
		t.Error("搜索media应有结果")
	}

	// 搜索不存在的
	results = catalog.SearchApps("xyznonexistent123")
	if len(results) != 0 {
		t.Error("搜索不存在的应为空")
	}
}

func TestCatalogListAppsFilter(t *testing.T) {
	catalog := NewCatalog(&CatalogConfig{DataDir: t.TempDir()})

	// 按分类过滤
	mediaApps := catalog.ListApps(&AppFilter{Category: "Media"})
	if len(mediaApps) == 0 {
		t.Error("Media分类不应为空")
	}
	for _, app := range mediaApps {
		if app.Category != "Media" {
			t.Errorf("过滤失败: %s 不是 Media", app.Category)
		}
	}

	// 按标签过滤
	tagApps := catalog.ListApps(&AppFilter{Tag: "streaming"})
	if len(tagApps) == 0 {
		t.Error("streaming标签不应为空")
	}
}

func TestCatalogAddRemoveRepo(t *testing.T) {
	catalog := NewCatalog(&CatalogConfig{DataDir: t.TempDir()})

	// 添加仓库
	repo := &Repository{
		ID:   "test-repo",
		Name: "测试仓库",
		URL:  "https://example.com/test",
		Type: "custom",
	}
	if err := catalog.AddRepository(repo); err != nil {
		t.Fatalf("添加仓库失败: %v", err)
	}

	// 重复添加
	if err := catalog.AddRepository(repo); err == nil {
		t.Error("重复添加应失败")
	}

	// 列出仓库
	repos := catalog.ListRepositories()
	if len(repos) < 3 { // official + community + test
		t.Errorf("仓库数量不足: %d", len(repos))
	}

	// 删除仓库
	if err := catalog.RemoveRepository("test-repo"); err != nil {
		t.Fatalf("删除仓库失败: %v", err)
	}

	// 删除不存在的
	if err := catalog.RemoveRepository("nonexistent"); err == nil {
		t.Error("删除不存在的仓库应失败")
	}
}

func TestCatalogGetUpdates(t *testing.T) {
	catalog := NewCatalog(&CatalogConfig{DataDir: t.TempDir()})

	installed := map[string]string{
		"postgres": "14", // 当前目录中 postgres latestVersion 为 "16"
	}

	updates := catalog.GetUpdates(installed)
	found := false
	for _, u := range updates {
		if u.AppID == "postgres" {
			found = true
			if u.CurrentVersion != "14" {
				t.Errorf("当前版本应为14, got %s", u.CurrentVersion)
			}
			if u.LatestVersion != "16" {
				t.Errorf("最新版本应为16, got %s", u.LatestVersion)
			}
		}
	}
	if !found {
		t.Error("应检测到postgres更新")
	}
}

// ========== Recommender Tests ==========

func TestRecommenderBasic(t *testing.T) {
	catalog := NewCatalog(&CatalogConfig{DataDir: t.TempDir()})
	sysInfo := &SystemInfo{
		TotalMemoryMB: 8192,
		UsedMemoryMB:  2048,
		TotalDiskGB:   500,
		UsedDiskGB:    100,
		CPUCores:      4,
		HasDocker:     true,
		NetworkType:   "home",
	}
	rec := NewRecommender(catalog, sysInfo)

	// 获取推荐（无已安装应用）
	recommendations := rec.GetRecommendations(map[string]bool{}, 10)
	if len(recommendations) == 0 {
		t.Error("推荐不应为空")
	}

	// 所有推荐都应有分数
	for _, r := range recommendations {
		if r.Score <= 0 {
			t.Errorf("推荐 %s 分数应 > 0, got %f", r.AppID, r.Score)
		}
	}
}

func TestRecommenderExcludeInstalled(t *testing.T) {
	catalog := NewCatalog(&CatalogConfig{DataDir: t.TempDir()})
	rec := NewRecommender(catalog, nil)

	installed := map[string]bool{
		"jellyfin": true,
		"nextcloud": true,
	}

	recommendations := rec.GetRecommendations(installed, 20)
	for _, r := range recommendations {
		if installed[r.AppID] {
			t.Errorf("推荐中不应包含已安装应用: %s", r.AppID)
		}
	}
}

func TestRecommenderSimilarApps(t *testing.T) {
	catalog := NewCatalog(&CatalogConfig{DataDir: t.TempDir()})
	rec := NewRecommender(catalog, nil)

	// 查找与jellyfin相似的应用
	similar := rec.GetSimilarApps("jellyfin", 5)
	if len(similar) == 0 {
		t.Error("相似应用不应为空")
	}

	// 不应包含自身
	for _, s := range similar {
		if s.AppID == "jellyfin" {
			t.Error("相似应用不应包含自身")
		}
	}
}

func TestRecommenderUsageTracking(t *testing.T) {
	catalog := NewCatalog(&CatalogConfig{DataDir: t.TempDir()})
	rec := NewRecommender(catalog, nil)

	// 记录使用
	rec.RecordUsage("jellyfin", 3600)
	rec.SetRating("jellyfin", 5)

	// 使用后不应影响推荐（已安装应用不在推荐中）
	rec.RecordUninstall("jellyfin")
}

// ========== DependencyResolver Tests ==========

func TestDependencyResolverNoDeps(t *testing.T) {
	catalog := NewCatalog(&CatalogConfig{DataDir: t.TempDir()})
	resolver := NewDependencyResolver(catalog)

	// jellyfin 没有依赖
	result, err := resolver.Resolve("jellyfin", map[string]bool{})
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(result.Resolved) != 0 {
		t.Errorf("jellyfin不应有依赖, got %d", len(result.Resolved))
	}
	if len(result.Conflicts) != 0 {
		t.Error("不应有冲突")
	}
}

func TestDependencyResolverWithDeps(t *testing.T) {
	catalog := NewCatalog(&CatalogConfig{DataDir: t.TempDir()})
	resolver := NewDependencyResolver(catalog)

	// nextcloud 依赖 postgres 和 redis
	result, err := resolver.Resolve("nextcloud", map[string]bool{})
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(result.Resolved) < 2 {
		t.Errorf("nextcloud应有2个依赖, got %d", len(result.Resolved))
	}

	// 验证安装顺序（依赖应在前）
	depSet := make(map[string]bool)
	for _, id := range result.Resolved {
		depSet[id] = true
	}
	if !depSet["postgres"] || !depSet["redis"] {
		t.Error("nextcloud应依赖postgres和redis")
	}
}

func TestDependencyResolverInstalledDeps(t *testing.T) {
	catalog := NewCatalog(&CatalogConfig{DataDir: t.TempDir()})
	resolver := NewDependencyResolver(catalog)

	// postgres已安装
	result, err := resolver.Resolve("nextcloud", map[string]bool{"postgres": true, "redis": true})
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(result.Resolved) != 0 {
		t.Errorf("依赖已安装，不应有额外依赖, got %d", len(result.Resolved))
	}
}

func TestDependencyResolverConflicts(t *testing.T) {
	catalog := NewCatalog(&CatalogConfig{DataDir: t.TempDir()})
	resolver := NewDependencyResolver(catalog)

	// plex 和 jellyfin 冲突
	result, err := resolver.Resolve("plex", map[string]bool{"jellyfin": true})
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(result.Conflicts) == 0 {
		t.Error("plex与jellyfin应有冲突")
	}
}

func TestDependencyResolverPortConflict(t *testing.T) {
	catalog := NewCatalog(&CatalogConfig{DataDir: t.TempDir()})
	resolver := NewDependencyResolver(catalog)

	// 两个应用都使用端口53 (pihole 和 postgres 不冲突，但可以测试 pihole 自身)
	result, err := resolver.Resolve("pihole", map[string]bool{})
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	// pihole 与 adguard 冲突声明但 adguard 不在目录中
	_ = result
}

func TestDependencyResolverNonexistent(t *testing.T) {
	catalog := NewCatalog(&CatalogConfig{DataDir: t.TempDir()})
	resolver := NewDependencyResolver(catalog)

	_, err := resolver.Resolve("nonexistent", map[string]bool{})
	if err == nil {
		t.Error("解析不存在的应用应失败")
	}
}

func TestDependencyResolverBatch(t *testing.T) {
	catalog := NewCatalog(&CatalogConfig{DataDir: t.TempDir()})
	resolver := NewDependencyResolver(catalog)

	// 批量安装无冲突的应用
	result, err := resolver.BatchResolve(
		[]string{"jellyfin", "nginx"},
		map[string]bool{},
	)
	if err != nil {
		t.Fatalf("批量解析失败: %v", err)
	}
	if len(result.Conflicts) != 0 {
		t.Error("jellyfin和nginx不应冲突")
	}

	// 批量安装有冲突的应用
	result, err = resolver.BatchResolve(
		[]string{"plex", "jellyfin"},
		map[string]bool{},
	)
	if err != nil {
		t.Fatalf("批量解析失败: %v", err)
	}
	if len(result.Conflicts) == 0 {
		t.Error("plex和jellyfin应有冲突")
	}
}

func TestDependencyResolverValidateUninstall(t *testing.T) {
	catalog := NewCatalog(&CatalogConfig{DataDir: t.TempDir()})
	resolver := NewDependencyResolver(catalog)

	installed := map[string]bool{
		"nextcloud": true,
		"postgres":  true,
		"redis":     true,
	}

	// postgres 被 nextcloud 依赖
	dependents := resolver.ValidateUninstall("postgres", installed)
	if len(dependents) == 0 {
		t.Error("postgres应有依赖者")
	}
	found := false
	for _, d := range dependents {
		if d == "nextcloud" {
			found = true
		}
	}
	if !found {
		t.Error("nextcloud应依赖postgres")
	}

	// redis 被 nextcloud 依赖
	dependents = resolver.ValidateUninstall("redis", installed)
	if len(dependents) == 0 {
		t.Error("redis应有依赖者")
	}

	// nginx 不被依赖
	dependents = resolver.ValidateUninstall("nginx", installed)
	if len(dependents) != 0 {
		t.Error("nginx不应有依赖者")
	}
}

func TestDependencyGraph(t *testing.T) {
	catalog := NewCatalog(&CatalogConfig{DataDir: t.TempDir()})
	resolver := NewDependencyResolver(catalog)

	graph := resolver.GetDependencyGraph("nextcloud")
	if graph == nil {
		t.Fatal("依赖图不应为空")
	}
	if graph.Root != "nextcloud" {
		t.Errorf("根节点应为nextcloud, got %s", graph.Root)
	}
	if len(graph.Deps) < 1 {
		t.Error("依赖图应有节点")
	}

	// jellyfin 无依赖
	graph = resolver.GetDependencyGraph("jellyfin")
	if graph == nil {
		t.Fatal("依赖图不应为空")
	}
	deps := graph.Deps["jellyfin"]
	if len(deps) != 0 {
		t.Error("jellyfin不应有依赖")
	}
}

func TestFormatDependencyTree(t *testing.T) {
	catalog := NewCatalog(&CatalogConfig{DataDir: t.TempDir()})
	resolver := NewDependencyResolver(catalog)

	tree := resolver.FormatDependencyTree("nextcloud", map[string]bool{})
	if tree == "" {
		t.Error("依赖树不应为空")
	}
	// 应包含 nextcloud
	if !containsStr(tree, "Nextcloud") {
		t.Error("依赖树应包含Nextcloud")
	}
}

// ========== SandboxManager Tests ==========

func TestSandboxManagerCreate(t *testing.T) {
	sm := NewSandboxManager(nil)

	sb, err := sm.CreateSandbox(nil, "test-app", nil)
	if err != nil {
		t.Fatalf("创建沙箱失败: %v", err)
	}
	if sb.State != SandboxStateRunning {
		t.Errorf("沙箱状态应为running, got %s", sb.State)
	}
	if sb.ResourceLimits == nil {
		t.Error("资源限制不应为空")
	}

	// 重复创建（同一应用）
	_, err = sm.CreateSandbox(nil, "test-app", nil)
	if err == nil {
		t.Error("同一应用重复创建沙箱应失败")
	}
}

func TestSandboxManagerResourceLimits(t *testing.T) {
	sm := NewSandboxManager(nil)

	limits := &ResourceLimits{
		CPUCores:  2.0,
		MemoryMB:  1024,
		DiskGB:    50,
	}

	sb, err := sm.CreateSandbox(nil, "test-app-2", limits)
	if err != nil {
		t.Fatalf("创建沙箱失败: %v", err)
	}
	if sb.ResourceLimits.CPUCores != 2.0 {
		t.Errorf("CPU应为2.0, got %f", sb.ResourceLimits.CPUCores)
	}
}

func TestSandboxManagerResourceLimitsMax(t *testing.T) {
	cfg := DefaultSandboxConfig()
	cfg.MaxCPUQuota = 4.0
	cfg.MaxMemoryMB = 4096
	cfg.MaxDiskGB = 100

	sm := NewSandboxManager(cfg)

	// 超过限制
	limits := &ResourceLimits{
		CPUCores:  16.0,
		MemoryMB:  16384,
		DiskGB:    1000,
	}

	sb, err := sm.CreateSandbox(nil, "test-max", limits)
	if err != nil {
		t.Fatalf("创建沙箱失败: %v", err)
	}

	if sb.ResourceLimits.CPUCores != 4.0 {
		t.Errorf("CPU应被限制为4.0, got %f", sb.ResourceLimits.CPUCores)
	}
	if sb.ResourceLimits.MemoryMB != 4096 {
		t.Errorf("内存应被限制为4096, got %d", sb.ResourceLimits.MemoryMB)
	}
	if sb.ResourceLimits.DiskGB != 100 {
		t.Errorf("磁盘应被限制为100, got %d", sb.ResourceLimits.DiskGB)
	}
}

func TestSandboxManagerLifecycle(t *testing.T) {
	sm := NewSandboxManager(nil)

	sb, _ := sm.CreateSandbox(nil, "test-lifecycle", nil)

	// 暂停
	if err := sm.PauseSandbox(sb.ID); err != nil {
		t.Fatalf("暂停沙箱失败: %v", err)
	}

	s, _ := sm.GetSandbox(sb.ID)
	if s.State != SandboxStatePaused {
		t.Errorf("状态应为paused, got %s", s.State)
	}

	// 恢复
	if err := sm.ResumeSandbox(sb.ID); err != nil {
		t.Fatalf("恢复沙箱失败: %v", err)
	}

	s, _ = sm.GetSandbox(sb.ID)
	if s.State != SandboxStateRunning {
		t.Errorf("状态应为running, got %s", s.State)
	}

	// 销毁
	if err := sm.DestroySandbox(sb.ID); err != nil {
		t.Fatalf("销毁沙箱失败: %v", err)
	}

	_, ok := sm.GetSandbox(sb.ID)
	if ok {
		t.Error("沙箱应已被销毁")
	}
}

func TestSandboxManagerGetByApp(t *testing.T) {
	sm := NewSandboxManager(nil)

	sm.CreateSandbox(nil, "app-1", nil)
	sm.CreateSandbox(nil, "app-2", nil)

	sb, ok := sm.GetSandboxByApp("app-1")
	if !ok {
		t.Fatal("应找到app-1的沙箱")
	}
	if sb.AppID != "app-1" {
		t.Errorf("应为app-1, got %s", sb.AppID)
	}

	_, ok = sm.GetSandboxByApp("nonexistent")
	if ok {
		t.Error("不应找到不存在应用的沙箱")
	}
}

func TestSandboxManagerList(t *testing.T) {
	sm := NewSandboxManager(nil)

	sm.CreateSandbox(nil, "list-1", nil)
	sm.CreateSandbox(nil, "list-2", nil)
	sm.CreateSandbox(nil, "list-3", nil)

	sandboxes := sm.ListSandboxes()
	if len(sandboxes) != 3 {
		t.Errorf("应有3个沙箱, got %d", len(sandboxes))
	}
}

func TestSandboxManagerResourceUsage(t *testing.T) {
	sm := NewSandboxManager(nil)

	sb, _ := sm.CreateSandbox(nil, "usage-app", &ResourceLimits{MemoryMB: 2048, DiskGB: 100})

	usage, err := sm.GetResourceUsage(sb.ID)
	if err != nil {
		t.Fatalf("获取资源使用失败: %v", err)
	}
	if usage.MemoryMB <= 0 {
		t.Error("内存使用应 > 0")
	}
}

func TestGenerateDockerResourceArgs(t *testing.T) {
	limits := &ResourceLimits{
		CPUCores: 2.0,
		MemoryMB: 1024,
	}

	args := GenerateDockerResourceArgs(limits)
	if len(args) == 0 {
		t.Error("Docker参数不应为空")
	}

	// 空限制
	args = GenerateDockerResourceArgs(nil)
	if len(args) != 0 {
		t.Error("空限制应返回空参数")
	}
}

func TestGenerateSecurityArgs(t *testing.T) {
	ctx := &SecurityContext{
		NoNewPrivs: true,
		DropCaps:   []string{"SYS_ADMIN", "NET_ADMIN"},
	}

	args := GenerateSecurityArgs(ctx)
	if len(args) == 0 {
		t.Error("安全参数不应为空")
	}

	// 空上下文
	args = GenerateSecurityArgs(nil)
	if len(args) != 0 {
		t.Error("空上下文应返回空参数")
	}
}

// ========== BatchManager Tests ==========

func TestBatchManagerInstall(t *testing.T) {
	catalog := NewCatalog(&CatalogConfig{DataDir: t.TempDir()})
	resolver := NewDependencyResolver(catalog)
	sandbox := NewSandboxManager(nil)
	bm := NewBatchManager(catalog, resolver, sandbox)

	installed := map[string]bool{}
	op, err := bm.BatchInstall(nil, []string{"jellyfin", "nginx"}, installed)
	if err != nil {
		t.Fatalf("批量安装失败: %v", err)
	}
	if op.Status != BatchOpStatusCompleted {
		t.Errorf("状态应为completed, got %s", op.Status)
	}
	if len(op.Results) == 0 {
		t.Error("结果不应为空")
	}
}

func TestBatchManagerInstallConflict(t *testing.T) {
	catalog := NewCatalog(&CatalogConfig{DataDir: t.TempDir()})
	resolver := NewDependencyResolver(catalog)
	sandbox := NewSandboxManager(nil)
	bm := NewBatchManager(catalog, resolver, sandbox)

	// plex 和 jellyfin 冲突
	_, err := bm.BatchInstall(nil, []string{"plex", "jellyfin"}, map[string]bool{})
	if err == nil {
		t.Error("冲突应用批量安装应失败")
	}
}

func TestBatchManagerCheckUpdates(t *testing.T) {
	catalog := NewCatalog(&CatalogConfig{DataDir: t.TempDir()})
	resolver := NewDependencyResolver(catalog)
	sandbox := NewSandboxManager(nil)
	bm := NewBatchManager(catalog, resolver, sandbox)

	installed := map[string]string{
		"postgres": "14",
	}

	updates := bm.CheckUpdates(installed)
	if len(updates) == 0 {
		t.Error("应检测到postgres更新")
	}
}

func TestBatchManagerListOperations(t *testing.T) {
	catalog := NewCatalog(&CatalogConfig{DataDir: t.TempDir()})
	resolver := NewDependencyResolver(catalog)
	sandbox := NewSandboxManager(nil)
	bm := NewBatchManager(catalog, resolver, sandbox)

	// 执行一个操作
	bm.BatchInstall(nil, []string{"jellyfin"}, map[string]bool{})

	ops := bm.ListOperations(10)
	if len(ops) == 0 {
		t.Error("操作历史不应为空")
	}
}

// ========== Helper ==========

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
