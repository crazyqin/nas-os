package mpiofc

import (
	"testing"
	"time"
)

// ========== 类型与验证测试 ==========

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig() 返回 nil")
	}
	if cfg.SysFCBase == "" {
		t.Error("默认 SysFCBase 不应为空")
	}
	if cfg.DMMPBase == "" {
		t.Error("默认 DMMPBase 不应为空")
	}
	if cfg.MultiPathBin == "" {
		t.Error("默认 MultiPathBin 不应为空")
	}
}

func TestPathPolicyValidate(t *testing.T) {
	tests := []struct {
		name    string
		policy  PathPolicy
		wantErr bool
	}{
		{"轮询策略", PathPolicyRoundRobin, false},
		{"故障转移策略", PathPolicyFailover, false},
		{"最小队列策略", PathPolicyMinQueue, false},
		{"轮询16策略", PathPolicyRoundRobin16, false},
		{"空策略", "", true},
		{"无效策略", PathPolicy("invalid"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.policy.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("PathPolicy(%q).Validate() error = %v, wantErr %v", tt.policy, err, tt.wantErr)
			}
		})
	}
}

func TestValidateWWPN(t *testing.T) {
	tests := []struct {
		wwpn    string
		wantErr bool
	}{
		{"5001438023456789", false},
		{"0x5001438023456789", false},
		{"short", true},
		{"", true},
	}
	for _, tt := range tests {
		err := ValidateWWPN(tt.wwpn)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateWWPN(%q) error = %v, wantErr %v", tt.wwpn, err, tt.wantErr)
		}
	}
}

func TestPathStateConstants(t *testing.T) {
	if PathStateActive != "active" {
		t.Error("PathStateActive 应为 'active'")
	}
	if PathStateStandby != "standby" {
		t.Error("PathStateStandby 应为 'standby'")
	}
	if PathStateFailed != "failed" {
		t.Error("PathStateFailed 应为 'failed'")
	}
	if PathStateUnknown != "unknown" {
		t.Error("PathStateUnknown 应为 'unknown'")
	}
}

func TestPathPolicyConstants(t *testing.T) {
	if PathPolicyRoundRobin != "round-robin" {
		t.Error("PathPolicyRoundRobin 应为 'round-robin'")
	}
	if PathPolicyFailover != "failover" {
		t.Error("PathPolicyFailover 应为 'failover'")
	}
	if PathPolicyMinQueue != "min-queue" {
		t.Error("PathPolicyMinQueue 应为 'min-queue'")
	}
	if PathPolicyRoundRobin16 != "round-robin-16" {
		t.Error("PathPolicyRoundRobin16 应为 'round-robin-16'")
	}
}

// ========== Service 基础测试 ==========

func TestNewService(t *testing.T) {
	svc := NewService(nil)
	if svc == nil {
		t.Fatal("NewService(nil) 返回 nil")
	}
	if svc.config == nil {
		t.Error("config 未初始化")
	}
	if svc.ports == nil {
		t.Error("ports map 未初始化")
	}
	if svc.paths == nil {
		t.Error("paths map 未初始化")
	}
	if svc.targetPaths == nil {
		t.Error("targetPaths map 未初始化")
	}
	if svc.stats == nil {
		t.Error("stats map 未初始化")
	}
}

func TestNewServiceWithConfig(t *testing.T) {
	cfg := &Config{
		SysFCBase:    "/tmp/fc",
		DMMPBase:     "/tmp/multipath",
		MultiPathBin: "/tmp/multipath",
	}
	svc := NewService(cfg)
	if svc.config != cfg {
		t.Error("config 应为传入的配置")
	}
}

// ========== 端口检测测试 ==========

func TestDetectPortsNoSysFS(t *testing.T) {
	cfg := &Config{
		SysFCBase:    "/nonexistent/path",
		DMMPBase:     "/tmp/multipath",
		MultiPathBin: "/sbin/multipath",
	}
	svc := NewService(cfg)
	ports, err := svc.DetectPorts()
	if err != nil {
		t.Fatalf("DetectPorts() 不应返回错误: %v", err)
	}
	if ports == nil {
		t.Fatal("DetectPorts() 返回 nil")
	}
	if len(ports) != 0 {
		t.Errorf("期望 0 个端口，得到 %d 个", len(ports))
	}
}

func TestGetPortsEmpty(t *testing.T) {
	svc := NewService(nil)
	ports := svc.GetPorts()
	if ports == nil {
		t.Fatal("GetPorts() 返回 nil")
	}
	if len(ports) != 0 {
		t.Errorf("期望 0 个端口，得到 %d", len(ports))
	}
}

func TestGetPortNotFound(t *testing.T) {
	svc := NewService(nil)
	_, err := svc.GetPort("fc_host0")
	if err == nil {
		t.Error("不存在的端口应返回错误")
	}
}

// ========== 多路径配置测试 ==========

// injectPort 向 Service 注入测试端口.
func injectPort(svc *Service, portID string) *HBAPort {
	p := &HBAPort{
		ID:        portID,
		Name:      portID,
		WWPN:      "500143802345678" + portID[len(portID)-1:],
		WWNN:      "500143801234567" + portID[len(portID)-1:],
		Speed:     "16G",
		PortType:  "N_Port",
		State:     PathStateActive,
		Online:    true,
		Supported: true,
		UpdatedAt: time.Now(),
	}
	svc.ports[portID] = p
	return p
}

func TestConfigureMPIOSuccess(t *testing.T) {
	svc := NewService(nil)
	injectPort(svc, "fc_host0")
	injectPort(svc, "fc_host1")

	req := &MPIOConfig{
		TargetWWPN: "5001438023456789",
		Policy:     PathPolicyRoundRobin,
		Paths: []PathConfig{
			{HBAPortID: "fc_host0", Priority: 1},
			{HBAPortID: "fc_host1", Priority: 2},
		},
	}
	paths, err := svc.ConfigureMPIO(req)
	if err != nil {
		t.Fatalf("配置失败: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("期望 2 条路径，得到 %d", len(paths))
	}

	// 轮询模式下所有路径应为活跃
	for _, p := range paths {
		if p.PathState != PathStateActive {
			t.Errorf("轮询模式路径状态应为 active，得到 %s", p.PathState)
		}
		if !p.Active {
			t.Error("轮询模式路径应为活跃")
		}
	}

	// 验证状态
	status := svc.GetStatus()
	if status.TotalPaths != 2 {
		t.Errorf("期望 2 条路径，得到 %d", status.TotalPaths)
	}
	if status.ActivePaths != 2 {
		t.Errorf("期望 2 条活跃路径，得到 %d", status.ActivePaths)
	}
}

func TestConfigureMPIOFailoverPolicy(t *testing.T) {
	svc := NewService(nil)
	injectPort(svc, "fc_host0")
	injectPort(svc, "fc_host1")

	req := &MPIOConfig{
		TargetWWPN: "5001438023456789",
		Policy:     PathPolicyFailover,
		Paths: []PathConfig{
			{HBAPortID: "fc_host0", Priority: 1},
			{HBAPortID: "fc_host1", Priority: 2},
		},
	}
	paths, err := svc.ConfigureMPIO(req)
	if err != nil {
		t.Fatalf("配置失败: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("期望 2 条路径，得到 %d", len(paths))
	}

	// 故障转移模式：仅一个活跃，一个待机
	activeCount := 0
	standbyCount := 0
	for _, p := range paths {
		switch p.PathState {
		case PathStateActive:
			activeCount++
		case PathStateStandby:
			standbyCount++
		}
	}
	if activeCount != 1 {
		t.Errorf("期望 1 条活跃路径，得到 %d", activeCount)
	}
	if standbyCount != 1 {
		t.Errorf("期望 1 条待机路径，得到 %d", standbyCount)
	}
}

func TestConfigureMPIOInvalidPolicy(t *testing.T) {
	svc := NewService(nil)
	injectPort(svc, "fc_host0")

	req := &MPIOConfig{
		TargetWWPN: "5001438023456789",
		Policy:     PathPolicy("invalid"),
		Paths:      []PathConfig{{HBAPortID: "fc_host0"}},
	}
	_, err := svc.ConfigureMPIO(req)
	if err == nil {
		t.Error("无效策略应返回错误")
	}
}

func TestConfigureMPIOInvalidWWPN(t *testing.T) {
	svc := NewService(nil)
	injectPort(svc, "fc_host0")

	req := &MPIOConfig{
		TargetWWPN: "short",
		Policy:     PathPolicyRoundRobin,
		Paths:      []PathConfig{{HBAPortID: "fc_host0"}},
	}
	_, err := svc.ConfigureMPIO(req)
	if err == nil {
		t.Error("无效 WWPN 应返回错误")
	}
}

func TestConfigureMPIOInvalidPort(t *testing.T) {
	svc := NewService(nil)

	req := &MPIOConfig{
		TargetWWPN: "5001438023456789",
		Policy:     PathPolicyRoundRobin,
		Paths:      []PathConfig{{HBAPortID: "nonexistent"}},
	}
	_, err := svc.ConfigureMPIO(req)
	if err == nil {
		t.Error("不存在的端口应返回错误")
	}
}

func TestConfigureMPIOReconfigure(t *testing.T) {
	svc := NewService(nil)
	injectPort(svc, "fc_host0")
	injectPort(svc, "fc_host1")

	// 第一次配置
	req := &MPIOConfig{
		TargetWWPN: "5001438023456789",
		Policy:     PathPolicyRoundRobin,
		Paths:      []PathConfig{{HBAPortID: "fc_host0"}},
	}
	paths, err := svc.ConfigureMPIO(req)
	if err != nil {
		t.Fatalf("第一次配置失败: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("期望 1 条路径，得到 %d", len(paths))
	}

	// 重新配置（替换旧路径）
	req.Paths = []PathConfig{
		{HBAPortID: "fc_host0"},
		{HBAPortID: "fc_host1"},
	}
	paths, err = svc.ConfigureMPIO(req)
	if err != nil {
		t.Fatalf("重新配置失败: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("期望 2 条路径，得到 %d", len(paths))
	}

	// 旧路径应被清除
	status := svc.GetStatus()
	if status.TotalPaths != 2 {
		t.Errorf("期望 2 条路径（旧路径应被清除），得到 %d", status.TotalPaths)
	}
}

// ========== 状态查询测试 ==========

func TestGetStatusEmpty(t *testing.T) {
	svc := NewService(nil)
	status := svc.GetStatus()
	if status == nil {
		t.Fatal("GetStatus() 返回 nil")
	}
	if status.TotalPaths != 0 {
		t.Errorf("期望 0 条路径，得到 %d", status.TotalPaths)
	}
	if status.Paths == nil {
		t.Error("Paths 不应为 nil")
	}
}

func TestGetStatisticsEmpty(t *testing.T) {
	svc := NewService(nil)
	stats := svc.GetStatistics()
	if stats == nil {
		t.Fatal("GetStatistics() 返回 nil")
	}
	if len(stats) != 0 {
		t.Errorf("期望 0 条统计，得到 %d", len(stats))
	}
}

func TestGetPathsByTarget(t *testing.T) {
	svc := NewService(nil)
	injectPort(svc, "fc_host0")
	injectPort(svc, "fc_host1")

	// 未配置时返回空
	paths := svc.GetPathsByTarget("5001438023456789")
	if len(paths) != 0 {
		t.Errorf("期望 0 条路径，得到 %d", len(paths))
	}

	// 配置后返回路径
	req := &MPIOConfig{
		TargetWWPN: "5001438023456789",
		Policy:     PathPolicyRoundRobin,
		Paths: []PathConfig{
			{HBAPortID: "fc_host0"},
			{HBAPortID: "fc_host1"},
		},
	}
	_, _ = svc.ConfigureMPIO(req)
	paths = svc.GetPathsByTarget("5001438023456789")
	if len(paths) != 2 {
		t.Errorf("期望 2 条路径，得到 %d", len(paths))
	}
}

// ========== 故障切换测试 ==========

func TestHandlePathFailureFailover(t *testing.T) {
	svc := NewService(nil)
	injectPort(svc, "fc_host0")
	injectPort(svc, "fc_host1")

	// 配置故障转移模式
	req := &MPIOConfig{
		TargetWWPN: "5001438023456789",
		Policy:     PathPolicyFailover,
		Paths: []PathConfig{
			{HBAPortID: "fc_host0", Priority: 1},
			{HBAPortID: "fc_host1", Priority: 2},
		},
	}
	paths, err := svc.ConfigureMPIO(req)
	if err != nil {
		t.Fatalf("配置失败: %v", err)
	}

	// 找到活跃路径和待机路径
	var activePath, standbyPath *MPIOPath
	for _, p := range paths {
		switch p.PathState {
		case PathStateActive:
			activePath = p
		case PathStateStandby:
			standbyPath = p
		}
	}
	if activePath == nil || standbyPath == nil {
		t.Fatal("未找到活跃路径或待机路径")
	}

	// 模拟活跃路径故障
	err = svc.HandlePathFailure(activePath.ID)
	if err != nil {
		t.Fatalf("故障切换失败: %v", err)
	}

	// 验证原活跃路径已变为故障
	failedPath, _ := func() (*MPIOPath, error) {
		svc.mu.RLock()
		defer svc.mu.RUnlock()
		p, ok := svc.paths[activePath.ID]
		if !ok {
			return nil, nil
		}
		return p, nil
	}()
	if failedPath.PathState != PathStateFailed {
		t.Errorf("原活跃路径状态应为 failed，得到 %s", failedPath.PathState)
	}

	// 验证待机路径已激活
	activatedPath, _ := func() (*MPIOPath, error) {
		svc.mu.RLock()
		defer svc.mu.RUnlock()
		p, ok := svc.paths[standbyPath.ID]
		if !ok {
			return nil, nil
		}
		return p, nil
	}()
	if activatedPath.PathState != PathStateActive {
		t.Errorf("备用路径状态应为 active，得到 %s", activatedPath.PathState)
	}
	if !activatedPath.Active {
		t.Error("备用路径应已激活")
	}
	if activatedPath.FailoverCount != 1 {
		t.Errorf("故障切换计数期望 1，得到 %d", activatedPath.FailoverCount)
	}

	// 验证总故障切换事件
	status := svc.GetStatus()
	if status.FailoverEvents != 1 {
		t.Errorf("期望 1 次故障切换事件，得到 %d", status.FailoverEvents)
	}
}

func TestHandlePathFailureRoundRobin(t *testing.T) {
	svc := NewService(nil)
	injectPort(svc, "fc_host0")
	injectPort(svc, "fc_host1")

	// 配置轮询模式
	req := &MPIOConfig{
		TargetWWPN: "5001438023456789",
		Policy:     PathPolicyRoundRobin,
		Paths: []PathConfig{
			{HBAPortID: "fc_host0"},
			{HBAPortID: "fc_host1"},
		},
	}
	paths, _ := svc.ConfigureMPIO(req)

	// 标记一条路径故障
	err := svc.HandlePathFailure(paths[0].ID)
	if err != nil {
		t.Fatalf("故障处理失败: %v", err)
	}

	// 轮询模式下故障路径标记为 failed，但不会触发切换（其余本来就是活跃的）
	status := svc.GetStatus()
	if status.FailedPaths != 1 {
		t.Errorf("期望 1 条故障路径，得到 %d", status.FailedPaths)
	}
	if status.ActivePaths != 1 {
		t.Errorf("期望 1 条活跃路径，得到 %d", status.ActivePaths)
	}
}

func TestHandlePathFailureNotFound(t *testing.T) {
	svc := NewService(nil)
	err := svc.HandlePathFailure("nonexistent")
	if err == nil {
		t.Error("不存在的路径应返回错误")
	}
}

func TestHandlePathFailureAlreadyFailed(t *testing.T) {
	svc := NewService(nil)
	injectPort(svc, "fc_host0")

	req := &MPIOConfig{
		TargetWWPN: "5001438023456789",
		Policy:     PathPolicyFailover,
		Paths:      []PathConfig{{HBAPortID: "fc_host0"}},
	}
	paths, _ := svc.ConfigureMPIO(req)

	// 第一次标记故障
	_ = svc.HandlePathFailure(paths[0].ID)

	// 第二次标记故障（已是故障状态，不应报错）
	err := svc.HandlePathFailure(paths[0].ID)
	if err != nil {
		t.Errorf("重复标记故障状态不应返回错误: %v", err)
	}
}

func TestReactivatePath(t *testing.T) {
	svc := NewService(nil)
	injectPort(svc, "fc_host0")
	injectPort(svc, "fc_host1")

	req := &MPIOConfig{
		TargetWWPN: "5001438023456789",
		Policy:     PathPolicyFailover,
		Paths: []PathConfig{
			{HBAPortID: "fc_host0", Priority: 1},
			{HBAPortID: "fc_host1", Priority: 2},
		},
	}
	paths, _ := svc.ConfigureMPIO(req)

	// 标记活跃路径故障
	_ = svc.HandlePathFailure(paths[0].ID)

	// 恢复路径
	err := svc.ReactivatePath(paths[0].ID)
	if err != nil {
		t.Fatalf("恢复路径失败: %v", err)
	}

	// 验证路径已恢复为待机
	path, _ := func() (*MPIOPath, error) {
		svc.mu.RLock()
		defer svc.mu.RUnlock()
		p, ok := svc.paths[paths[0].ID]
		if !ok {
			return nil, nil
		}
		return p, nil
	}()

	// 故障转移策略下恢复后应为待机
	if path.PathState != PathStateStandby {
		t.Errorf("恢复后路径状态应为 standby，得到 %s", path.PathState)
	}
}

func TestReactivatePathNotFailed(t *testing.T) {
	svc := NewService(nil)
	injectPort(svc, "fc_host0")

	req := &MPIOConfig{
		TargetWWPN: "5001438023456789",
		Policy:     PathPolicyRoundRobin,
		Paths:      []PathConfig{{HBAPortID: "fc_host0"}},
	}
	paths, _ := svc.ConfigureMPIO(req)

	// 路径未故障，恢复应报错
	err := svc.ReactivatePath(paths[0].ID)
	if err == nil {
		t.Error("未故障的路径恢复应返回错误")
	}
}

func TestReactivatePathNotFound(t *testing.T) {
	svc := NewService(nil)
	err := svc.ReactivatePath("nonexistent")
	if err == nil {
		t.Error("不存在的路径应返回错误")
	}
}

// ========== 统计更新测试 ==========

func TestUpdatePathStats(t *testing.T) {
	svc := NewService(nil)
	injectPort(svc, "fc_host0")

	req := &MPIOConfig{
		TargetWWPN: "5001438023456789",
		Policy:     PathPolicyRoundRobin,
		Paths:      []PathConfig{{HBAPortID: "fc_host0"}},
	}
	paths, _ := svc.ConfigureMPIO(req)

	// 更新统计
	svc.UpdatePathStats(paths[0].ID, 100, 50, 10.5, 5.2, 2.3)

	stats := svc.GetStatistics()
	if len(stats) != 1 {
		t.Fatalf("期望 1 条统计，得到 %d", len(stats))
	}
	if stats[0].IOPSRead != 100 {
		t.Errorf("IOPSRead 期望 100，得到 %d", stats[0].IOPSRead)
	}
	if stats[0].IOPSWrite != 50 {
		t.Errorf("IOPSWrite 期望 50，得到 %d", stats[0].IOPSWrite)
	}
	if stats[0].IOPSTotal != 150 {
		t.Errorf("IOPSTotal 期望 150，得到 %d", stats[0].IOPSTotal)
	}
	if stats[0].LatencyAvgMs != 2.3 {
		t.Errorf("LatencyAvgMs 期望 2.3，得到 %f", stats[0].LatencyAvgMs)
	}
	if stats[0].LatencyMaxMs != 2.3 {
		t.Errorf("LatencyMaxMs 期望 2.3，得到 %f", stats[0].LatencyMaxMs)
	}
}

func TestUpdatePathStatsNonExist(t *testing.T) {
	svc := NewService(nil)
	// 不存在的路径，不应 panic
	svc.UpdatePathStats("nonexistent", 100, 50, 10.0, 5.0, 2.0)
}

// ========== Handler 测试 ==========

func TestNewHandler(t *testing.T) {
	svc := NewService(nil)
	handler := NewHandler(svc)
	if handler == nil {
		t.Fatal("NewHandler() 返回 nil")
	}
	if handler.service != svc {
		t.Error("Handler 的 service 应为传入的 Service")
	}
}

// ========== 辅助函数测试 ==========

func TestPortStateFromString(t *testing.T) {
	tests := []struct {
		input string
		want  PathState
	}{
		{"Online", PathStateActive},
		{"online", PathStateActive},
		{"Offline", PathStateFailed},
		{"offline", PathStateFailed},
		{"Standby", PathStateStandby},
		{"unknown", PathStateUnknown},
		{"", PathStateUnknown},
	}
	for _, tt := range tests {
		got := portStateFromString(tt.input)
		if got != tt.want {
			t.Errorf("portStateFromString(%q) = %v, 期望 %v", tt.input, got, tt.want)
		}
	}
}
