# SMART Cron 安全测试方案

**版本**: v2.419.0
**日期**: 2026-04-07
**负责**: 刑部安全合规

---

## 一、测试范围

### 1.1 相关模块

| 模块 | 文件位置 | 类型 |
|------|----------|------|
| 前端 | `webui/js/smart-cron.js` | JavaScript |
| 后端核心 | `internal/storage/smart/smart-flexible.go` | Go (新) |
| SMART监控 | `internal/disk/smart_monitor.go` | Go (现有) |
| API处理器 | `internal/disk/handlers.go` | Go |
| 前端测试 | `web/src/hooks/useNVMe.ts` | TypeScript |

### 1.2 API端点

```
/api/v1/smart-cron/tasks          # 任务列表 CRUD
/api/v1/smart-cron/executions     # 执行状态
/api/v1/smart-cron/results        # 检查结果
/api/v1/smart-cron/alerts         # 告警管理
/api/v1/disks                     # 设备列表（现有）
```

---

## 二、API单元测试设计

### 2.1 测试用例清单

#### 任务管理API测试

```go
// 文件: internal/storage/smart/smart_test.go

func TestSmartCronAPI_CreateTask(t *testing.T) {
    tests := []struct {
        name       string
        payload    map[string]interface{}
        expectCode int
        expectErr  bool
    }{
        // 正常场景
        {
            name: "正常创建任务",
            payload: map[string]interface{}{
                "name":     "每日SMART检测",
                "testType": "short",
                "schedule": "0 3 * * *",
                "devices":  []string{"/dev/sda"},
                "enabled":  true,
            },
            expectCode: 200,
            expectErr:  false,
        },
        
        // 边界条件 - 空名称
        {
            name: "空任务名称",
            payload: map[string]interface{}{
                "name":     "",
                "testType": "short",
                "schedule": "0 3 * * *",
                "devices":  []string{"/dev/sda"},
            },
            expectCode: 400,
            expectErr:  true,
        },
        
        // 边界条件 - 无效cron表达式
        {
            name: "无效cron表达式",
            payload: map[string]interface{}{
                "name":     "测试任务",
                "testType": "short",
                "schedule": "invalid-cron",
                "devices":  []string{"/dev/sda"},
            },
            expectCode: 400,
            expectErr:  true,
        },
        
        // 边界条件 - 空设备列表
        {
            name: "空设备列表",
            payload: map[string]interface{}{
                "name":     "测试任务",
                "testType": "short",
                "schedule": "0 3 * * *",
                "devices":  []string{},
            },
            expectCode: 400,
            expectErr:  true,
        },
        
        // 安全测试 - 路径遍历设备名
        {
            name: "路径遍历设备名",
            payload: map[string]interface{}{
                "name":     "测试任务",
                "testType": "short",
                "schedule": "0 3 * * *",
                "devices":  []string{"/dev/../etc/passwd"},
            },
            expectCode: 400,
            expectErr:  true,
        },
        
        // 安全测试 - 命令注入cron表达式
        {
            name: "命令注入cron表达式",
            payload: map[string]interface{}{
                "name":     "测试任务",
                "testType": "short",
                "schedule": "0 3 * * * ; rm -rf /",
                "devices":  []string{"/dev/sda"},
            },
            expectCode: 400,
            expectErr:  true,
        },
        
        // 边界条件 - 无效测试类型
        {
            name: "无效测试类型",
            payload: map[string]interface{}{
                "name":     "测试任务",
                "testType": "invalid-type",
                "schedule": "0 3 * * *",
                "devices":  []string{"/dev/sda"},
            },
            expectCode: 400,
            expectErr:  true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // 测试实现
        })
    }
}
```

#### 执行状态API测试

```go
func TestSmartCronAPI_GetExecutions(t *testing.T) {
    tests := []struct {
        name       string
        taskId     string
        expectCode int
    }{
        {"正常查询", "task-001", 200},
        {"无效任务ID", "task-../../etc", 400},
        {"不存在任务", "task-nonexistent", 404},
    }
}
```

#### 结果查询API测试

```go
func TestSmartCronAPI_GetResults(t *testing.T) {
    tests := []struct {
        name       string
        filters    map[string]string
        expectCode int
    }{
        // 正常过滤
        {
            name: "按设备过滤",
            filters: map[string]string{"device": "/dev/sda"},
            expectCode: 200,
        },
        
        // 安全测试 - 路径遍历过滤
        {
            name: "路径遍历设备过滤",
            filters: map[string]string{"device": "/dev/../etc"},
            expectCode: 400,
        },
        
        // 边界条件 - 无效状态
        {
            name: "无效状态过滤",
            filters: map[string]string{"status": "invalid"},
            expectCode: 400,
        },
    }
}
```

---

## 三、边界条件测试

### 3.1 Cron表达式验证

```go
func TestCronExpressionValidation(t *testing.T) {
    validExpressions := []string{
        "0 3 * * *",       // 每日3点
        "0 2 * * 0",       // 每周日2点
        "0 0 1 * *",       // 每月1号
        "0 */6 * * *",     // 每6小时
    }
    
    invalidExpressions := []string{
        "",                // 空
        "invalid",         // 无效格式
        "0 3 * * * ; rm",  // 命令注入
        "99 99 * * *",     // 超范围值
        "* * * *",         // 字段不足
        "* * * * * *",     // 字段过多
    }
    
    for _, expr := range validExpressions {
        t.Run("valid_"+expr, func(t *testing.T) {
            err := validateCronExpression(expr)
            assert.NoError(t, err)
        })
    }
    
    for _, expr := range invalidExpressions {
        t.Run("invalid_"+expr, func(t *testing.T) {
            err := validateCronExpression(expr)
            assert.Error(t, err)
        })
    }
}
```

### 3.2 设备路径验证

```go
func TestDevicePathValidation(t *testing.T) {
    validDevices := []string{
        "/dev/sda",
        "/dev/sdb1",
        "/dev/nvme0n1",
        "/dev/nvme1n1p2",
    }
    
    invalidDevices := []string{
        "",                    // 空
        "/dev/../etc/passwd",  // 路径遍历
        "/etc/passwd",         // 非设备路径
        "/dev/sda;rm",         // 命令注入
        "../../../etc/shadow", // 相对路径遍历
        "/dev/loop0",          // loop设备(排除)
    }
    
    for _, dev := range validDevices {
        t.Run("valid_"+dev, func(t *testing.T) {
            err := validateDevicePath(dev)
            assert.NoError(t, err)
        })
    }
    
    for _, dev := range invalidDevices {
        t.Run("invalid_"+dev, func(t *testing.T) {
            err := validateDevicePath(dev)
            assert.Error(t, err)
        })
    }
}
```

### 3.3 整数范围测试

```go
func TestIntegerBoundaryConditions(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        min      int
        max      int
        expected int
        hasError bool
    }{
        {"正常值", "50", 0, 100, 50, false},
        {"最小边界", "0", 0, 100, 0, false},
        {"最大边界", "100", 0, 100, 100, false},
        {"超出最小", "-1", 0, 100, 0, true},
        {"超出最大", "101", 0, 100, 100, true},
        {"非数字", "abc", 0, 100, 0, true},
        {"空值", "", 0, 100, 0, true},
        {"超大数", "999999999999", 0, 100, 100, true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := parseBoundedInt(tt.input, tt.min, tt.max)
            if tt.hasError {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tt.expected, result)
            }
        })
    }
}
```

---

## 四、权限验证测试

### 4.1 RBAC权限矩阵

| 操作 | admin | operator | viewer | guest |
|------|-------|----------|--------|-------|
| 创建任务 | ✅ | ✅ | ❌ | ❌ |
| 编辑任务 | ✅ | ✅ | ❌ | ❌ |
| 删除任务 | ✅ | ❌ | ❌ | ❌ |
| 查看任务 | ✅ | ✅ | ✅ | ❌ |
| 运行任务 | ✅ | ✅ | ❌ | ❌ |
| 查看结果 | ✅ | ✅ | ✅ | ✅ |

### 4.2 权限测试用例

```go
func TestSmartCronPermissions(t *testing.T) {
    tests := []struct {
        role      string
        endpoint  string
        method    string
        expectCode int
    }{
        // Admin权限
        {"admin", "/api/v1/smart-cron/tasks", "POST", 200},
        {"admin", "/api/v1/smart-cron/tasks/task-1", "DELETE", 200},
        {"admin", "/api/v1/smart-cron/tasks/task-1/run", "POST", 200},
        
        // Operator权限
        {"operator", "/api/v1/smart-cron/tasks", "POST", 200},
        {"operator", "/api/v1/smart-cron/tasks/task-1", "DELETE", 403},
        {"operator", "/api/v1/smart-cron/tasks/task-1/run", "POST", 200},
        
        // Viewer权限
        {"viewer", "/api/v1/smart-cron/tasks", "GET", 200},
        {"viewer", "/api/v1/smart-cron/tasks", "POST", 403},
        {"viewer", "/api/v1/smart-cron/tasks/task-1", "DELETE", 403},
        
        // Guest权限
        {"guest", "/api/v1/smart-cron/tasks", "GET", 403},
        {"guest", "/api/v1/smart-cron/results", "GET", 200}, // 仅可看结果
        {"guest", "/api/v1/smart-cron/alerts", "GET", 403},
        
        // 未认证用户
        {"", "/api/v1/smart-cron/tasks", "GET", 401},
        {"", "/api/v1/smart-cron/tasks", "POST", 401},
    }
    
    for _, tt := range tests {
        t.Run(tt.role+"_"+tt.method+"_"+tt.endpoint, func(t *testing.T) {
            // 权限测试实现
        })
    }
}
```

### 4.3 敏感操作审计

```go
func TestSensitiveOperationAudit(t *testing.T) {
    sensitiveOps := []struct {
        operation string
        requireAudit bool
    }{
        {"create_task", true},
        {"delete_task", true},
        {"toggle_task", true},
        {"run_task", true},
        {"view_results", false},
        {"view_alerts", false},
    }
    
    for _, op := range sensitiveOps {
        t.Run(op.operation, func(t *testing.T) {
            if op.requireAudit {
                // 验证审计日志写入
            }
        })
    }
}
```

---

## 五、前端安全测试

### 5.1 JavaScript输入验证

```javascript
// 文件: webui/js/smart-cron.test.js

describe('SmartCron Input Validation', () => {
    test('设备路径验证', () => {
        expect(validateDevice('/dev/sda')).toBe(true)
        expect(validateDevice('/dev/../etc')).toBe(false)
        expect(validateDevice('')).toBe(false)
    })
    
    test('Cron表达式验证', () => {
        expect(validateCron('0 3 * * *')).toBe(true)
        expect(validateCron('invalid')).toBe(false)
        expect(validateCron('0 3 * * * ; rm')).toBe(false)
    })
    
    test('任务名称验证', () => {
        expect(validateName('每日检查')).toBe(true)
        expect(validateName('')).toBe(false)
        expect(validateName('<script>alert(1)</script>')).toBe(false)
    })
})
```

### 5.2 XSS防护测试

```javascript
describe('XSS Protection', () => {
    test('任务名称XSS过滤', () => {
        const malicious = '<img src=x onerror=alert(1)>'
        const sanitized = sanitizeTaskName(malicious)
        expect(sanitized).not.toContain('<')
        expect(sanitized).not.toContain('onerror')
    })
    
    test('Cron预览XSS过滤', () => {
        const malicious = '0 3 * * *<script>alert(1)</script>'
        const sanitized = sanitizeCronPreview(malicious)
        expect(sanitized).not.toContain('<script>')
    })
})
```

---

## 六、测试执行计划

### 6.1 执行顺序

```
Phase 1: API单元测试 (Day 1)
├── 创建测试文件 smart_test.go
├── 实现输入验证测试
├── 实现边界条件测试
└── 实现安全注入测试

Phase 2: 权限验证测试 (Day 1-2)
├── 配置测试角色
├── 实现RBAC测试矩阵
├── 验证审计日志
└── 端到端权限测试

Phase 3: 前端安全测试 (Day 2)
├── JavaScript单元测试
├── XSS防护验证
├── CSRF验证
└── 输入过滤测试

Phase 4: 集成测试 (Day 3)
├── 完整流程测试
├── 错误处理测试
├── 性能边界测试
└── 报告生成
```

### 6.2 测试覆盖率目标

| 类型 | 目标覆盖率 |
|------|-----------|
| API单元测试 | 80% |
| 边界条件测试 | 90% |
| 权限验证测试 | 100% |
| 前端安全测试 | 70% |

---

## 七、测试环境要求

### 7.1 Go测试依赖

```bash
go test -v -race -cover ./internal/storage/smart/...
go test -v -race -cover ./internal/disk/...
```

### 7.2 前端测试依赖

```bash
npm test webui/js/smart-cron.test.js
```

### 7.3 Mock设备

测试需Mock以下设备避免依赖真实硬件：

```go
mockDevices := []string{
    "/dev/mock-sda",
    "/dev/mock-nvme0n1",
}
```

---

**测试负责人**: 刑部
**预计完成**: 2026-04-07
**文档位置**: memory/round187-security-test-plan.md