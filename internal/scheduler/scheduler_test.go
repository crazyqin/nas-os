package scheduler

import (
	"context"
	"testing"
	"time"
)

// ========== Cron 表达式测试 ==========

func TestCronExpression_Parse(t *testing.T) {
	tests := []struct {
		expression string
		valid      bool
	}{
		{"0 * * * * *", true},    // 每分钟
		{"0 0 * * * *", true},    // 每小时
		{"0 0 0 * * *", true},    // 每天
		{"0 0 0 1 * *", true},    // 每月
		{"0 0 0 * * 0", true},    // 每周日
		{"*/5 * * * * *", true},  // 每5秒
		{"0 */15 * * * *", true}, // 每15分钟
		{"0 0 */2 * * *", true},  // 每2小时
		{"invalid", false},       // 无效
		{"* * *", false},         // 字段不够
	}

	for _, tt := range tests {
		t.Run(tt.expression, func(t *testing.T) {
			_, err := NewCronExpression(tt.expression, CronParseOptions{Second: true})
			valid := err == nil
			if valid != tt.valid {
				t.Errorf("NewCronExpression(%q) valid = %v, want %v", tt.expression, valid, tt.valid)
			}
		})
	}
}

func TestCronExpression_Next(t *testing.T) {
	// 测试每分钟执行
	expr, err := NewCronExpression("0 * * * * *", CronParseOptions{Second: true, Location: time.UTC})
	if err != nil {
		t.Fatalf("解析表达式失败: %v", err)
	}

	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	next := expr.Next(now)

	// 下一次执行应该是下一分钟
	expected := time.Date(2024, 1, 1, 12, 1, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("Next() = %v, want %v", next, expected)
	}
}

func TestCronExpression_NextN(t *testing.T) {
	expr, _ := NewCronExpression("0 */5 * * * *", CronParseOptions{Second: true, Location: time.UTC})

	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	times := expr.NextN(now, 3)

	if len(times) != 3 {
		t.Errorf("NextN() 返回 %d 个时间，期望 3", len(times))
	}

	// 验证时间间隔
	for i := 1; i < len(times); i++ {
		diff := times[i].Sub(times[i-1])
		if diff != 5*time.Minute {
			t.Errorf("时间间隔 = %v, 期望 5m", diff)
		}
	}
}

func TestCronExpression_Step(t *testing.T) {
	// 测试步长表达式
	expr, err := NewCronExpression("0 */10 * * * *", CronParseOptions{Second: true, Location: time.UTC})
	if err != nil {
		t.Fatalf("解析表达式失败: %v", err)
	}

	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	next := expr.Next(now)

	// 应该是 10 分钟后
	expected := time.Date(2024, 1, 1, 12, 10, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("Next() = %v, want %v", next, expected)
	}
}

func TestCronExpression_Range(t *testing.T) {
	// 测试范围表达式
	expr, err := NewCronExpression("0 0 9-17 * * 1-5", CronParseOptions{Second: true, Location: time.UTC})
	if err != nil {
		t.Fatalf("解析表达式失败: %v", err)
	}

	// 测试在工作日的工作时间
	now := time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC) // 周二 10:00
	next := expr.Next(now)

	// 下一次应该是 11:00
	if next.Hour() != 11 {
		t.Errorf("Next() hour = %d, want 11", next.Hour())
	}
}

func TestIsValidCron(t *testing.T) {
	if !IsValidCron("0 * * * * *", true) {
		t.Error("有效的表达式应该返回 true")
	}

	if IsValidCron("invalid", true) {
		t.Error("无效的表达式应该返回 false")
	}
}

// ========== 依赖管理测试 ==========

func TestDependencyManager_RegisterTask(t *testing.T) {
	dm := NewDependencyManager()

	task := &Task{
		ID:   "task1",
		Name: "任务1",
	}

	if err := dm.RegisterTask(task); err != nil {
		t.Errorf("注册任务失败: %v", err)
	}

	// 验证任务已注册
	if _, exists := dm.tasks[task.ID]; !exists {
		t.Error("任务未注册")
	}
}

func TestDependencyManager_CanRun(t *testing.T) {
	dm := NewDependencyManager()

	// 注册两个任务
	task1 := &Task{ID: "task1", Name: "任务1"}
	task2 := &Task{ID: "task2", Name: "任务2", Dependencies: []string{"task1"}}

	dm.RegisterTask(task1)
	dm.RegisterTask(task2)

	// task2 依赖 task1，在 task1 完成前不能运行
	canRun, _ := dm.CanRun("task2")
	if canRun {
		t.Error("依赖未满足时不应该能运行")
	}

	// 标记 task1 完成
	dm.MarkCompleted("task1")

	// 现在 task2 应该可以运行
	canRun, _ = dm.CanRun("task2")
	if !canRun {
		t.Error("依赖满足后应该能运行")
	}
}

func TestDependencyManager_HasCycle(t *testing.T) {
	dm := NewDependencyManager()

	// 创建循环依赖
	task1 := &Task{ID: "task1", Dependencies: []string{"task2"}}
	task2 := &Task{ID: "task2", Dependencies: []string{"task3"}}
	task3 := &Task{ID: "task3", Dependencies: []string{"task1"}}

	dm.RegisterTask(task1)
	dm.RegisterTask(task2)
	dm.RegisterTask(task3)

	if !dm.HasCycle() {
		t.Error("应该检测到循环依赖")
	}

	// 创建无循环依赖
	dm2 := NewDependencyManager()
	taskA := &Task{ID: "taskA"}
	taskB := &Task{ID: "taskB", Dependencies: []string{"taskA"}}
	taskC := &Task{ID: "taskC", Dependencies: []string{"taskB"}}

	dm2.RegisterTask(taskA)
	dm2.RegisterTask(taskB)
	dm2.RegisterTask(taskC)

	if dm2.HasCycle() {
		t.Error("不应该检测到循环依赖")
	}
}

func TestDependencyManager_GetExecutionOrder(t *testing.T) {
	dm := NewDependencyManager()

	task1 := &Task{ID: "task1"}
	task2 := &Task{ID: "task2", Dependencies: []string{"task1"}}
	task3 := &Task{ID: "task3", Dependencies: []string{"task1", "task2"}}

	dm.RegisterTask(task1)
	dm.RegisterTask(task2)
	dm.RegisterTask(task3)

	order, err := dm.GetExecutionOrder()
	if err != nil {
		t.Fatalf("获取执行顺序失败: %v", err)
	}

	// task1 应该在 task2 之前
	task1Idx := indexOf(order, "task1")
	task2Idx := indexOf(order, "task2")
	task3Idx := indexOf(order, "task3")

	if task1Idx > task2Idx {
		t.Error("task1 应该在 task2 之前执行")
	}
	if task2Idx > task3Idx {
		t.Error("task2 应该在 task3 之前执行")
	}
}

func indexOf(slice []string, item string) int {
	for i, v := range slice {
		if v == item {
			return i
		}
	}
	return -1
}

// ========== 执行器测试 ==========

func TestExecutor_RegisterHandler(t *testing.T) {
	exec := NewExecutor(10)

	handler := NewCommandHandler()
	if err := exec.RegisterHandler(handler); err != nil {
		t.Errorf("注册处理器失败: %v", err)
	}

	// 验证处理器已注册
	_, exists := exec.GetHandler("command")
	if !exists {
		t.Error("处理器未注册")
	}
}

func TestExecutor_IsRunning(t *testing.T) {
	exec := NewExecutor(10)

	if exec.IsRunning("nonexistent") {
		t.Error("不存在的任务不应该在运行")
	}
}

func TestExecutor_RunningCount(t *testing.T) {
	exec := NewExecutor(10)

	if exec.RunningCount() != 0 {
		t.Error("初始运行数量应该为 0")
	}
}

// ========== 重试测试 ==========

func TestRetryManager_ShouldRetry(t *testing.T) {
	rm := NewRetryManager()

	task := &Task{
		ID:          "task1",
		RetryPolicy: RetryPolicyFixed,
		MaxRetries:  3,
	}

	// 第一次应该可以重试
	if !rm.ShouldRetry(task, nil) {
		t.Error("第一次应该可以重试")
	}

	// 不设置重试策略的任务不应该重试
	taskNoRetry := &Task{
		ID:          "task2",
		RetryPolicy: RetryPolicyNone,
	}

	if rm.ShouldRetry(taskNoRetry, nil) {
		t.Error("None 策略不应该重试")
	}
}

func TestRetryManager_CalculateDelay(t *testing.T) {
	rm := NewRetryManager()

	// 固定延迟
	taskFixed := &Task{
		RetryPolicy:   RetryPolicyFixed,
		RetryInterval: "5m",
	}

	delay := rm.CalculateDelay(taskFixed, 1)
	if delay != 5*time.Minute {
		t.Errorf("固定延迟 = %v, want 5m", delay)
	}

	// 指数退避
	taskExp := &Task{
		RetryPolicy:   RetryPolicyExponential,
		RetryInterval: "1m",
	}

	delay1 := rm.CalculateDelay(taskExp, 1)
	delay2 := rm.CalculateDelay(taskExp, 2)

	if delay2 <= delay1 {
		t.Error("指数退避应该递增")
	}
}

// ========== 调度器测试 ==========

func TestScheduler_AddTask(t *testing.T) {
	s, err := NewScheduler(nil)
	if err != nil {
		t.Fatalf("创建调度器失败: %v", err)
	}

	task := &Task{
		Name:    "测试任务",
		Type:    TaskTypeOneTime,
		Handler: "command",
	}

	if err := s.AddTask(task); err != nil {
		t.Errorf("添加任务失败: %v", err)
	}

	if task.ID == "" {
		t.Error("任务 ID 应该自动生成")
	}
}

func TestScheduler_GetTask(t *testing.T) {
	s, _ := NewScheduler(nil)

	task := &Task{
		Name:    "测试任务",
		Type:    TaskTypeOneTime,
		Handler: "command",
	}
	_ = s.AddTask(task)

	got, err := s.GetTask(task.ID)
	if err != nil {
		t.Errorf("获取任务失败: %v", err)
	}

	if got.Name != task.Name {
		t.Errorf("任务名称 = %s, want %s", got.Name, task.Name)
	}
}

func TestScheduler_DeleteTask(t *testing.T) {
	s, _ := NewScheduler(nil)

	task := &Task{
		Name:    "测试任务",
		Type:    TaskTypeOneTime,
		Handler: "command",
	}
	_ = s.AddTask(task)

	if err := s.DeleteTask(task.ID); err != nil {
		t.Errorf("删除任务失败: %v", err)
	}

	_, err := s.GetTask(task.ID)
	if err == nil {
		t.Error("删除后应该找不到任务")
	}
}

func TestScheduler_EnableDisable(t *testing.T) {
	s, _ := NewScheduler(nil)

	task := &Task{
		Name:    "测试任务",
		Type:    TaskTypeCron,
		Handler: "command",
		Enabled: true,
	}
	_ = s.AddTask(task)

	// 禁用
	if err := s.DisableTask(task.ID); err != nil {
		t.Errorf("禁用任务失败: %v", err)
	}

	got, _ := s.GetTask(task.ID)
	if got.Enabled {
		t.Error("任务应该被禁用")
	}

	// 启用
	if err := s.EnableTask(task.ID); err != nil {
		t.Errorf("启用任务失败: %v", err)
	}

	got, _ = s.GetTask(task.ID)
	if !got.Enabled {
		t.Error("任务应该被启用")
	}
}

func TestScheduler_GetStats(t *testing.T) {
	s, _ := NewScheduler(nil)

	// 添加几个任务
	_ = s.AddTask(&Task{Name: "任务1", Type: TaskTypeOneTime, Handler: "command"})
	_ = s.AddTask(&Task{Name: "任务2", Type: TaskTypeCron, Handler: "command", Enabled: true})
	_ = s.AddTask(&Task{Name: "任务3", Type: TaskTypeInterval, Handler: "command"})

	stats := s.GetStats()

	if stats.TotalTasks != 3 {
		t.Errorf("总任务数 = %d, want 3", stats.TotalTasks)
	}
}

// ========== 日志管理测试 ==========

func TestLogManager_RecordExecution(t *testing.T) {
	lm, err := NewLogManager("", 1000, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("创建日志管理器失败: %v", err)
	}

	execution := &TaskExecution{
		ID:        "exec1",
		TaskID:    "task1",
		TaskName:  "测试任务",
		Status:    ExecutionStatusCompleted,
		StartedAt: time.Now(),
	}

	if err := lm.RecordExecution(execution); err != nil {
		t.Errorf("记录执行失败: %v", err)
	}

	// 验证记录已保存
	got, err := lm.GetExecution("exec1")
	if err != nil {
		t.Errorf("获取执行记录失败: %v", err)
	}

	if got.TaskID != "task1" {
		t.Errorf("TaskID = %s, want task1", got.TaskID)
	}
}

func TestLogManager_AddLog(t *testing.T) {
	lm, _ := NewLogManager("", 1000, 7*24*time.Hour)

	log := &TaskLog{
		ExecutionID: "exec1",
		TaskID:      "task1",
		Level:       "info",
		Message:     "测试日志",
	}

	if err := lm.AddLog(log); err != nil {
		t.Errorf("添加日志失败: %v", err)
	}

	logs := lm.GetLogs("exec1")
	if len(logs) != 1 {
		t.Errorf("日志数量 = %d, want 1", len(logs))
	}
}

func TestLogManager_QueryExecutions(t *testing.T) {
	lm, _ := NewLogManager("", 1000, 7*24*time.Hour)

	// 添加多条执行记录
	lm.RecordExecution(&TaskExecution{ID: "exec1", TaskID: "task1", Status: ExecutionStatusCompleted, StartedAt: time.Now()})
	lm.RecordExecution(&TaskExecution{ID: "exec2", TaskID: "task1", Status: ExecutionStatusFailed, StartedAt: time.Now()})
	lm.RecordExecution(&TaskExecution{ID: "exec3", TaskID: "task2", Status: ExecutionStatusCompleted, StartedAt: time.Now()})

	// 按任务过滤
	filter := &ExecutionFilter{TaskID: "task1"}
	executions := lm.QueryExecutions(filter)

	if len(executions) != 2 {
		t.Errorf("执行记录数量 = %d, want 2", len(executions))
	}

	// 按状态过滤
	filter = &ExecutionFilter{Status: ExecutionStatusFailed}
	executions = lm.QueryExecutions(filter)

	if len(executions) != 1 {
		t.Errorf("失败记录数量 = %d, want 1", len(executions))
	}
}

// ========== 类型定义测试 ==========

func TestTaskStatus_String(t *testing.T) {
	statuses := []TaskStatus{TaskStatusPending, TaskStatusRunning, TaskStatusCompleted, TaskStatusFailed, TaskStatusCancelled, TaskStatusPaused}

	for _, status := range statuses {
		if string(status) == "" {
			t.Errorf("状态字符串不应该为空")
		}
	}
}

func TestTaskType_String(t *testing.T) {
	types := []TaskType{TaskTypeCron, TaskTypeOneTime, TaskTypeInterval, TaskTypeEvent, TaskTypeDependent}

	for _, typ := range types {
		if string(typ) == "" {
			t.Errorf("类型字符串不应该为空")
		}
	}
}

func TestRetryPolicy_String(t *testing.T) {
	policies := []RetryPolicy{RetryPolicyNone, RetryPolicyFixed, RetryPolicyExponential}

	for _, policy := range policies {
		if string(policy) == "" {
			t.Errorf("策略字符串不应该为空")
		}
	}
}

// ========== ParseDuration 测试 ==========

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		hasError bool
	}{
		{"1s", time.Second, false},
		{"1m", time.Minute, false},
		{"1h", time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"1w", 7 * 24 * time.Hour, false},
		{"invalid", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ParseDuration(tt.input)

			if tt.hasError {
				if err == nil {
					t.Errorf("ParseDuration(%q) 应该返回错误", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("ParseDuration(%q) 返回错误: %v", tt.input, err)
				}
				if result != tt.expected {
					t.Errorf("ParseDuration(%q) = %v, want %v", tt.input, result, tt.expected)
				}
			}
		})
	}
}

// ========== 处理器测试 ==========

func TestCommandHandler_Name(t *testing.T) {
	handler := NewCommandHandler()
	if handler.Name() != "command" {
		t.Errorf("处理器名称 = %s, want command", handler.Name())
	}
}

func TestHTTPHandler_Name(t *testing.T) {
	handler := NewHTTPHandler()
	if handler.Name() != "http" {
		t.Errorf("处理器名称 = %s, want http", handler.Name())
	}
}

func TestScriptHandler_Name(t *testing.T) {
	handler := NewScriptHandler()
	if handler.Name() != "script" {
		t.Errorf("处理器名称 = %s, want script", handler.Name())
	}
}

// ========== 集成测试 ==========

func TestIntegration_FullWorkflow(t *testing.T) {
	// 创建调度器
	s, err := NewScheduler(&Config{
		MaxConcurrentTasks: 5,
		StoragePath:        "",
	})
	if err != nil {
		t.Fatalf("创建调度器失败: %v", err)
	}

	// 注册处理器
	_ = s.RegisterHandler(NewCommandHandler())

	// 创建 Cron 任务
	cronTask := &Task{
		Name:           "Cron 测试任务",
		Type:           TaskTypeCron,
		Handler:        "command",
		CronExpression: "0 */5 * * * *",
		Enabled:        true,
	}

	if err := s.AddTask(cronTask); err != nil {
		t.Errorf("添加 Cron 任务失败: %v", err)
	}

	// 创建一次性任务
	oneTimeTask := &Task{
		Name:        "一次性任务",
		Type:        TaskTypeOneTime,
		Handler:     "command",
		ScheduledAt: timePtr(time.Now().Add(time.Hour)),
		Enabled:     true,
	}

	if err := s.AddTask(oneTimeTask); err != nil {
		t.Errorf("添加一次性任务失败: %v", err)
	}

	// 创建依赖任务
	taskA := &Task{Name: "任务A", Type: TaskTypeOneTime, Handler: "command"}
	taskB := &Task{Name: "任务B", Type: TaskTypeDependent, Handler: "command", Dependencies: []string{taskA.ID}}

	_ = s.AddTask(taskA)
	_ = s.AddTask(taskB)

	// 验证统计
	stats := s.GetStats()
	if stats.TotalTasks != 4 {
		t.Errorf("总任务数 = %d, want 4", stats.TotalTasks)
	}

	// 获取依赖图
	graph := s.GetDependencyGraph()
	if len(graph.Nodes) == 0 {
		t.Error("依赖图节点不应该为空")
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}

// ========== DoRetry 测试 ==========

func TestDoRetry(t *testing.T) {
	attempts := 0

	err := DoRetry(context.Background(), 3, time.Millisecond, func(ctx context.Context, rctx *RetryContext) error {
		attempts++
		if attempts < 3 {
			return nil // 成功
		}
		return nil
	})

	if err != nil {
		t.Errorf("DoRetry 返回错误: %v", err)
	}

	if attempts != 1 {
		t.Errorf("执行次数 = %d, want 1", attempts)
	}
}

func TestDoRetry_MaxAttempts(t *testing.T) {
	attempts := 0

	err := DoRetry(context.Background(), 3, time.Millisecond, func(ctx context.Context, rctx *RetryContext) error {
		attempts++
		return nil // 这里返回 nil 表示成功
	})

	if err != nil {
		t.Errorf("DoRetry 返回错误: %v", err)
	}

	// 因为第一次就成功了，所以只执行一次
	if attempts != 1 {
		t.Errorf("执行次数 = %d, want 1", attempts)
	}
}
