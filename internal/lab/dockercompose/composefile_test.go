package dockercompose

import (
	"testing"
	"time"
)

// ===== Compose 文件解析测试 =====

func TestParseComposeFile(t *testing.T) {
	data := map[string]interface{}{
		"version": "3.8",
		"services": map[string]interface{}{
			"web": map[string]interface{}{
				"image": "nginx:latest",
				"ports": []interface{}{"8080:80"},
			},
		},
		"networks": map[string]interface{}{
			"frontend": map[string]interface{}{
				"driver": "bridge",
			},
		},
		"volumes": map[string]interface{}{
			"data": map[string]interface{}{
				"driver": "local",
			},
		},
	}

	cf, err := ParseComposeFile(data)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if cf.Version != "3.8" {
		t.Errorf("版本应为 3.8，实际 %s", cf.Version)
	}
	if len(cf.Services) != 1 {
		t.Errorf("期望 1 个服务，实际 %d", len(cf.Services))
	}
	if _, ok := cf.Services["web"]; !ok {
		t.Error("缺少 web 服务")
	}
	if len(cf.Networks) != 1 {
		t.Errorf("期望 1 个网络，实际 %d", len(cf.Networks))
	}
	if len(cf.Volumes) != 1 {
		t.Errorf("期望 1 个卷，实际 %d", len(cf.Volumes))
	}
}

func TestParseComposeFileNil(t *testing.T) {
	_, err := ParseComposeFile(nil)
	if err == nil {
		t.Error("空数据应返回错误")
	}
}

func TestParseComposeFileWithEnv(t *testing.T) {
	data := map[string]interface{}{
		"services": map[string]interface{}{
			"app": map[string]interface{}{
				"image": "myapp:v1",
				"environment": map[string]interface{}{
					"DB_HOST": "localhost",
					"DB_PORT": "5432",
				},
			},
		},
	}
	cf, err := ParseComposeFile(data)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	svc := cf.Services["app"]
	if svc.Environment == nil {
		t.Fatal("环境变量不应为 nil")
	}
	if svc.Environment["DB_HOST"] != "localhost" {
		t.Errorf("DB_HOST 应为 localhost，实际 %s", svc.Environment["DB_HOST"])
	}
}

func TestParseComposeFileWithDependencies(t *testing.T) {
	data := map[string]interface{}{
		"services": map[string]interface{}{
			"web": map[string]interface{}{
				"image":      "nginx",
				"depends_on": []interface{}{"app"},
			},
			"app": map[string]interface{}{
				"image": "myapp",
			},
		},
	}
	cf, err := ParseComposeFile(data)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	web := cf.Services["web"]
	if len(web.DependsOn) != 1 || web.DependsOn[0] != "app" {
		t.Errorf("web 应依赖 app，实际 %v", web.DependsOn)
	}
}

// ===== 验证测试 =====

func TestValidateComposeFileValid(t *testing.T) {
	cf := &ComposeFile{
		Services: map[string]CompService{
			"web": {Image: "nginx:latest"},
		},
	}
	result := ValidateComposeFile(cf)
	if !result.Valid {
		t.Errorf("应为有效，实际错误: %v", result.Errors)
	}
}

func TestValidateComposeFileNoServices(t *testing.T) {
	cf := &ComposeFile{
		Services: map[string]CompService{},
	}
	result := ValidateComposeFile(cf)
	if result.Valid {
		t.Error("无服务应验证失败")
	}
}

func TestValidateComposeFileNil(t *testing.T) {
	result := ValidateComposeFile(nil)
	if result.Valid {
		t.Error("nil 应验证失败")
	}
}

func TestValidateComposeFileNoImageNoBuild(t *testing.T) {
	cf := &ComposeFile{
		Services: map[string]CompService{
			"broken": {},
		},
	}
	result := ValidateComposeFile(cf)
	if result.Valid {
		t.Error("无 image 和 build 应验证失败")
	}
}

func TestValidateComposeFileMissingDependency(t *testing.T) {
	cf := &ComposeFile{
		Services: map[string]CompService{
			"web": {Image: "nginx", DependsOn: []string{"missing"}},
		},
	}
	result := ValidateComposeFile(cf)
	if result.Valid {
		t.Error("依赖不存在的服务应验证失败")
	}
}

func TestValidateComposeFileCyclicDependency(t *testing.T) {
	cf := &ComposeFile{
		Services: map[string]CompService{
			"a": {Image: "img", DependsOn: []string{"b"}},
			"b": {Image: "img", DependsOn: []string{"a"}},
		},
	}
	result := ValidateComposeFile(cf)
	if result.Valid {
		t.Error("循环依赖应验证失败")
	}
}

func TestValidateComposeFileNetworkWarning(t *testing.T) {
	cf := &ComposeFile{
		Services: map[string]CompService{
			"web": {Image: "nginx", Networks: []string{"custom"}},
		},
		Networks: map[string]CompNetwork{},
	}
	result := ValidateComposeFile(cf)
	if !result.Valid {
		t.Errorf("未定义网络应为警告而非错误，但失败: %v", result.Errors)
	}
	if len(result.Warnings) == 0 {
		t.Error("应产生警告")
	}
}

// ===== 日志管理测试 =====

func TestLogManagerAddAndGet(t *testing.T) {
	lm := NewLogManager(100)

	lm.AddLog("proj1", "web", "启动完成", LogLevelInfo)
	lm.AddLog("proj1", "web", "收到请求", LogLevelDebug)
	lm.AddLog("proj1", "db", "连接建立", LogLevelInfo)

	logs := lm.GetLogs("proj1", "web", LogOptions{})
	if len(logs) != 2 {
		t.Errorf("期望 2 条 web 日志，实际 %d", len(logs))
	}

	dbLogs := lm.GetLogs("proj1", "db", LogOptions{})
	if len(dbLogs) != 1 {
		t.Errorf("期望 1 条 db 日志，实际 %d", len(dbLogs))
	}
}

func TestLogManagerTail(t *testing.T) {
	lm := NewLogManager(100)
	for i := 0; i < 50; i++ {
		lm.AddLog("p", "s", "msg", LogLevelInfo)
	}

	logs := lm.GetLogs("p", "s", LogOptions{Tail: 10})
	if len(logs) != 10 {
		t.Errorf("tail=10 应返回 10 条，实际 %d", len(logs))
	}
}

func TestLogManagerLevelFilter(t *testing.T) {
	lm := NewLogManager(100)
	lm.AddLog("p", "s", "debug", LogLevelDebug)
	lm.AddLog("p", "s", "info", LogLevelInfo)
	lm.AddLog("p", "s", "error", LogLevelError)

	logs := lm.GetLogs("p", "s", LogOptions{Level: LogLevelError})
	if len(logs) != 1 {
		t.Errorf("error 级别过滤应返回 1 条，实际 %d", len(logs))
	}
	if logs[0].Message != "error" {
		t.Errorf("应为 error 消息，实际 %s", logs[0].Message)
	}
}

func TestLogManagerTimeFilter(t *testing.T) {
	lm := NewLogManager(100)
	lm.AddLog("p", "s", "old", LogLevelInfo)
	time.Sleep(10 * time.Millisecond)
	middle := time.Now()
	time.Sleep(10 * time.Millisecond)
	lm.AddLog("p", "s", "new", LogLevelInfo)

	logs := lm.GetLogs("p", "s", LogOptions{Since: middle})
	if len(logs) != 1 {
		t.Errorf("时间过滤应返回 1 条，实际 %d", len(logs))
	}
}

func TestLogManagerMaxSize(t *testing.T) {
	lm := NewLogManager(5)
	for i := 0; i < 10; i++ {
		lm.AddLog("p", "s", "msg", LogLevelInfo)
	}
	logs := lm.GetLogs("p", "s", LogOptions{})
	if len(logs) != 5 {
		t.Errorf("超过上限应截断，期望 5 条，实际 %d", len(logs))
	}
}

func TestLogManagerClear(t *testing.T) {
	lm := NewLogManager(100)
	lm.AddLog("p", "s", "msg", LogLevelInfo)
	lm.ClearLogs("p", "s")
	logs := lm.GetLogs("p", "s", LogOptions{})
	if len(logs) != 0 {
		t.Errorf("清除后应为空，实际 %d 条", len(logs))
	}
}

func TestLogManagerProjectLogs(t *testing.T) {
	lm := NewLogManager(100)
	lm.AddLog("proj", "web", "web log", LogLevelInfo)
	lm.AddLog("proj", "db", "db log", LogLevelInfo)
	lm.AddLog("other", "web", "other log", LogLevelInfo)

	logs := lm.GetProjectLogs("proj", LogOptions{})
	if len(logs) != 2 {
		t.Errorf("项目日志应为 2 条，实际 %d", len(logs))
	}
}

// ===== 健康检查管理测试 =====

func TestHealthManagerRegisterAndCheck(t *testing.T) {
	hm := NewHealthManager()

	hm.RegisterCheck("proj", "web", &HealthCheck{
		Test:     []string{"curl", "http://localhost"},
		Interval: "10s",
		Retries:  3,
	})

	// 首次健康检查
	hm.RunCheck("proj", "web", true, "ok")
	status, err := hm.GetStatus("proj", "web")
	if err != nil {
		t.Fatalf("获取状态失败: %v", err)
	}
	if status.Status != HealthHealthy {
		t.Errorf("应为 healthy，实际 %s", status.Status)
	}

	// 连续失败
	hm.RunCheck("proj", "web", false, "timeout")
	status, _ = hm.GetStatus("proj", "web")
	if status.Status != HealthStarting {
		t.Errorf("失败 1 次应为 starting，实际 %s", status.Status)
	}

	hm.RunCheck("proj", "web", false, "timeout")
	hm.RunCheck("proj", "web", false, "timeout")
	status, _ = hm.GetStatus("proj", "web")
	if status.Status != HealthUnhealthy {
		t.Errorf("失败 3 次应为 unhealthy，实际 %s", status.Status)
	}

	// 恢复
	hm.RunCheck("proj", "web", true, "recovered")
	status, _ = hm.GetStatus("proj", "web")
	if status.Status != HealthHealthy {
		t.Errorf("恢复后应为 healthy，实际 %s", status.Status)
	}
	if status.FailCount != 0 {
		t.Errorf("恢复后失败计数应为 0，实际 %d", status.FailCount)
	}
}

func TestHealthManagerGetStatusNotFound(t *testing.T) {
	hm := NewHealthManager()
	_, err := hm.GetStatus("no", "svc")
	if err == nil {
		t.Error("未注册的服务应返回错误")
	}
}

func TestHealthManagerProjectHealth(t *testing.T) {
	hm := NewHealthManager()
	hm.RegisterCheck("proj", "web", nil)
	hm.RegisterCheck("proj", "db", nil)
	hm.RegisterCheck("other", "svc", nil)

	statuses := hm.GetProjectHealth("proj")
	if len(statuses) != 2 {
		t.Errorf("应有 2 个服务，实际 %d", len(statuses))
	}
}

func TestHealthManagerResults(t *testing.T) {
	hm := NewHealthManager()
	hm.RegisterCheck("p", "s", nil)

	for i := 0; i < 5; i++ {
		hm.RunCheck("p", "s", true, "ok")
	}

	results := hm.GetResults("p", "s", 3)
	if len(results) != 3 {
		t.Errorf("limit=3 应返回 3 条，实际 %d", len(results))
	}
}

func TestHealthManagerResultsLimit150(t *testing.T) {
	hm := NewHealthManager()
	hm.RegisterCheck("p", "s", nil)

	for i := 0; i < 150; i++ {
		hm.RunCheck("p", "s", i%2 == 0, "check")
	}

	results := hm.GetResults("p", "s", 0)
	if len(results) != 100 {
		t.Errorf("最多保留 100 条，实际 %d", len(results))
	}
}

func TestHealthManagerUnregister(t *testing.T) {
	hm := NewHealthManager()
	hm.RegisterCheck("p", "s", nil)
	hm.RunCheck("p", "s", true, "ok")

	hm.UnregisterCheck("p", "s")
	_, err := hm.GetStatus("p", "s")
	if err == nil {
		t.Error("注销后应返回错误")
	}
}

func TestHealthManagerDefaultRetries(t *testing.T) {
	hm := NewHealthManager()
	// 不指定 Retries，默认 3 次
	hm.RegisterCheck("p", "s", nil)

	hm.RunCheck("p", "s", false, "fail1")
	hm.RunCheck("p", "s", false, "fail2")
	status, _ := hm.GetStatus("p", "s")
	if status.Status != HealthStarting {
		t.Errorf("2 次失败应为 starting，实际 %s", status.Status)
	}

	hm.RunCheck("p", "s", false, "fail3")
	status, _ = hm.GetStatus("p", "s")
	if status.Status != HealthUnhealthy {
		t.Errorf("3 次失败应为 unhealthy，实际 %s", status.Status)
	}
}
