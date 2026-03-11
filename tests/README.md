# NAS-OS 测试套件 v1.0

完整的集成测试框架，包含 E2E 测试、功能覆盖、性能基准和报告生成。

## 目录结构

```
tests/
├── e2e/                    # 端到端测试
│   ├── suite.go           # 测试套件框架
│   ├── client.go          # HTTP 测试客户端
│   ├── storage_test.go    # 存储模块 E2E 测试
│   ├── auth_test.go       # 认证模块 E2E 测试
│   └── system_test.go     # 系统状态 E2E 测试
├── integration/            # 集成测试
│   ├── integration_test.go # 基础集成测试
│   └── full_coverage_test.go # 全功能覆盖测试
├── benchmark/              # 性能基准测试
│   └── benchmark_test.go  # 性能基准测试
├── reports/                # 测试报告生成
│   ├── generator.go       # 报告生成器
│   └── output/            # 报告输出目录
└── fixtures/               # 测试固件
    └── fixtures.go        # 测试数据
```

## 快速开始

### 运行所有测试
```bash
make test-suite
# 或
./scripts/test.sh all
```

### 运行特定测试
```bash
# 单元测试
make test
./scripts/test.sh unit

# 集成测试
make test-integration
./scripts/test.sh integration

# E2E 测试
make test-e2e
./scripts/test.sh e2e

# 性能基准测试
make test-benchmark
./scripts/test.sh benchmark
```

### 生成覆盖率报告
```bash
make test-coverage
# 输出：tests/reports/output/coverage.html
```

### 生成测试报告
```bash
make test-report
# 输出：tests/reports/output/test-report.{html,md,json}
```

## 测试类型

### 1. E2E 测试 (端到端测试)

模拟真实用户操作流程，测试完整的 API 端到端功能。

**测试场景:**
- 存储卷生命周期（创建/查询/删除）
- 用户认证流程（登录/登出）
- 系统健康检查
- 完整工作流测试

**运行:**
```bash
NAS_OS_E2E=1 go test -v ./tests/e2e/...
```

### 2. 集成测试

测试各模块之间的交互和数据流。

**测试覆盖:**
- RAID 配置验证
- 卷生命周期测试
- 子卷操作测试
- 快照操作测试
- 并发安全测试
- 数据一致性测试

**运行:**
```bash
go test -v ./tests/integration/...
```

### 3. 性能基准测试

测试关键操作的性能指标。

**测试项目:**
- 存储操作性能（创建/查询/并发访问）
- 数据结构性能
- 内存分配性能
- 并发性能（Mutex/RWMutex）
- JSON 序列化性能

**运行:**
```bash
go test -bench=. -benchmem ./tests/benchmark/...
```

### 4. 测试报告

生成多种格式的测试报告。

**输出格式:**
- JSON: `test-report.json`
- Markdown: `test-report.md`
- HTML: `test-report.html`

## 测试工具

### 测试客户端

```go
client := NewTestClient("http://localhost:8080")

// 发送请求
resp, err := client.Get("/api/v1/volumes")
resp, err := client.Post("/api/v1/volumes", map[string]interface{}{
    "name": "test-vol",
    "devices": []string{"/dev/sda1"},
})

// 解析响应
var volume map[string]interface{}
ParseJSON(resp, &volume)
```

### 断言函数

```go
AssertStatus(t, http.StatusOK, resp.StatusCode)
AssertEqual(t, "expected", actual)
AssertNotEmpty(t, value)
AssertContains(t, str, substr)
```

### 测试固件

```go
// 使用预定义的测试数据
vols := fixtures.VolumeFixtures.ValidVolumes
users := fixtures.UserFixtures.ValidUsers

// 创建测试卷
vol := fixtures.CreateTestVolume("test")
```

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `NAS_OS_E2E` | 启用 E2E 测试 | 空（禁用） |
| `NAS_OS_TEST_TIMEOUT` | 测试超时时间 | 30s |

## CI/CD 集成

### GitHub Actions

```yaml
- name: Run Tests
  run: |
    make test-suite
    make test-coverage

- name: Upload Coverage
  uses: codecov/codecov-action@v3
  with:
    files: ./tests/reports/output/coverage.out
```

### 测试命令速查

| 命令 | 说明 |
|------|------|
| `make test` | 运行单元测试 |
| `make test-integration` | 运行集成测试 |
| `make test-e2e` | 运行 E2E 测试 |
| `make test-benchmark` | 运行性能测试 |
| `make test-coverage` | 生成覆盖率报告 |
| `make test-report` | 生成测试报告 |
| `make test-suite` | 运行完整测试套件 |
| `make test-race` | 竞态条件检测 |

## 编写测试

### E2E 测试示例

```go
func TestE2E_Storage_CreateVolume(t *testing.T) {
    if testing.Short() {
        t.Skip("跳过 E2E 测试")
    }

    server := NewTestServer()
    defer server.Close()

    client := NewTestClient(server.URL)

    resp, err := client.Post("/api/v1/volumes", map[string]interface{}{
        "name":    "test-volume",
        "devices": []string{"/dev/sda1"},
        "profile": "single",
    })
    if err != nil {
        t.Fatalf("请求失败: %v", err)
    }
    defer resp.Body.Close()

    AssertStatus(t, http.StatusCreated, resp.StatusCode)
}
```

### 集成测试示例

```go
func TestFull_Storage_VolumeLifecycle(t *testing.T) {
    mgr := storage.NewMockManager()

    // 创建
    vol, err := mgr.CreateVolume("test", []string{"/dev/sda1"}, "single")
    if err != nil {
        t.Fatalf("创建失败: %v", err)
    }

    // 查询
    vols, _ := mgr.ListVolumes()
    if len(vols) != 1 {
        t.Errorf("期望 1 个卷，实际 %d", len(vols))
    }

    // 删除
    mgr.DeleteVolume("test")
}
```

### 性能测试示例

```go
func BenchmarkStorage_CreateVolume(b *testing.B) {
    mgr := storage.NewMockManager()
    b.ResetTimer()

    for i := 0; i < b.N; i++ {
        name := fmt.Sprintf("bench-vol-%d", i)
        mgr.CreateVolume(name, []string{"/dev/sda1"}, "single")
    }
}
```

## 最佳实践

1. **使用测试固件**: 预定义测试数据，确保一致性
2. **清理测试环境**: 使用 `defer` 确保资源释放
3. **并行测试**: 使用 `t.Parallel()` 加速测试
4. **有意义的断言**: 提供清晰的错误消息
5. **覆盖边界条件**: 测试正常和异常场景

## 故障排查

### 测试失败

```bash
# 详细输出
./scripts/test.sh unit -v

# 单个测试
go test -v -run TestStorage_CreateVolume ./internal/storage/
```

### 竞态条件

```bash
# 启用竞态检测
make test-race
# 或
go test -race ./...
```

### 清理测试产物

```bash
./scripts/test.sh clean
```

---

**版本**: v1.0.0  
**更新日期**: 2026-03-11