# NAS-OS 安全审计报告 v2.74.0

**审计日期**: 2026-03-15  
**审计版本**: v2.73.0 → v2.74.0  
**审计部门**: 刑部  
**审计范围**: 代码安全漏洞、依赖安全性、敏感信息泄露、认证授权

---

## 一、审计概述

### 版本变更范围
v2.74.0 为版本发布完善版本，主要变更：
- `memory/six-ministries-dev-state.json` (开发状态更新)
- 文档同步 (CHANGELOG.md, README.md, docs/RELEASE-v2.74.0.md)

**代码层面无实质性变更**，本次审计聚焦于整体代码库安全状态评估。

---

## 二、依赖安全审计

### 2.1 Go 标准库漏洞 (govulncheck)

发现 **5 个标准库漏洞**，需升级 Go 版本修复：

| 漏洞编号 | 组件 | 严重性 | 影响 | 修复版本 |
|---------|------|--------|------|---------|
| GO-2026-4603 | html/template | 中 | meta content URL 未转义 | go1.26.1 |
| GO-2026-4602 | os | 低 | FileInfo 可逃逸 Root | go1.26.1 |
| GO-2026-4601 | net/url | 中 | IPv6 主机解析错误 | go1.26.1 |
| GO-2026-4600 | crypto/x509 | 高 | 证书名称约束检查可致 panic | go1.26.1 |
| GO-2026-4599 | crypto/x509 | 中 | 邮件约束验证不正确 | go1.26.1 |

**影响路径**:
- `internal/reports/enhanced_export.go:308` - HTML 模板执行
- `internal/ldap/client.go:81` - LDAP StartTLS 连接
- `internal/office/manager.go:1158` - URL 解析
- `internal/web/server.go:791` - HTTP 服务

**修复建议**: 升级 Go 版本至 `go1.26.1` 或更高版本

```bash
# go.mod 当前版本
go 1.26

# 建议修改为
go 1.26.1
```

### 2.2 第三方依赖状态

主要依赖版本检查：
- ✅ `gin-gonic/gin v1.11.0` - 最新稳定版
- ✅ `golang.org/x/crypto v0.48.0` - 安全
- ✅ `google/uuid v1.6.0` - 安全
- ✅ `prometheus/client_golang v1.23.2` - 安全
- ✅ `spf13/cobra v1.10.2` - 安全

---

## 三、代码安全审计

### 3.1 整数溢出风险 (gosec G115)

发现 **22 处整数转换潜在溢出**：

| 文件 | 行号 | 问题 |
|-----|------|-----|
| `internal/system/monitor.go` | 760-761 | uint64 → int64 网络速度累加 |
| `internal/snapshot/adapter.go` | 42 | uint64 → int64 快照大小 |
| `internal/quota/optimizer/optimizer.go` | 516, 934 | uint64 → int64 配额计算 |
| `internal/backup/smart_manager_v2.go` | 710 | uint64 → int64 存储空间 |
| `internal/storage/smart_monitor.go` | 420, 606, 617 | uint64 → int SMART 属性 |
| `internal/search/engine.go` | 539 | uint64 → int 搜索结果计数 |
| `internal/reports/datasource.go` | 116, 119, 331, 334, 337 | uint64 → int 数据汇总 |
| `internal/monitor/metrics_collector.go` | 844, 847 | uint64 → int 磁盘指标 |
| `internal/optimizer/optimizer.go` | 168, 170 | uint64 → time.Duration |

**风险评估**: 中等。NAS 系统存储容量可能超过 int64 范围 (9.2 EB)，但实际场景罕见。

**修复建议**: 
```go
// 不安全
netRX += int64(n.RXSpeed)

// 建议：添加溢出检查
if n.RXSpeed > math.MaxInt64 {
    netRX = math.MaxInt64
} else {
    netRX += int64(n.RXSpeed)
}
```

### 3.2 命令注入风险

#### 高风险: 脚本执行 (internal/snapshot/executor.go:89)

```go
cmd := exec.CommandContext(ctx, "sh", "-c", script)
```

**问题**: `script` 参数直接传入 shell 执行，如果来源不可信可能导致命令注入。

**缓解措施**: 
- 脚本来源于策略配置 (`policy.Scripts.PreSnapshotScript`)
- 需确保配置文件权限受限

**修复建议**:
```go
// 方案 1: 禁止 shell，使用直接执行
cmd := exec.CommandContext(ctx, script) // 脚本路径

// 方案 2: 输入验证
import "regexp"
var safeScriptPattern = regexp.MustCompile(`^[a-zA-Z0-9_\-./]+$`)
if !safeScriptPattern.MatchString(script) {
    return fmt.Errorf("invalid script path")
}
```

#### 高风险: 通知命令 (internal/usbmount/manager.go:883)

```go
cmd := exec.CommandContext(m.ctx, "sh", "-c", m.config.NotifyCommand)
```

**问题**: `NotifyCommand` 来自配置文件，通过环境变量注入设备信息。

**缓解措施**: 环境变量正确转义，但命令本身仍可执行任意代码。

**修复建议**: 添加命令白名单或限制可执行路径。

### 3.3 敏感信息泄露

#### 中风险: 初始密码输出到控制台 (internal/users/manager.go:183)

```go
fmt.Printf("   密码: %s\n", defaultPassword)
```

**问题**: 默认管理员密码输出到标准输出，可能被日志记录或进程监控捕获。

**缓解措施**: 
- 仅首次启动时输出
- 使用随机生成的 16 字符密码
- 密码不写入日志文件

**修复建议**:
```go
// 使用更安全的传递方式
// 1. 写入临时文件，设置 600 权限，首次登录后删除
passwordFile := "/tmp/nas-os-initial-password"
os.WriteFile(passwordFile, []byte(defaultPassword), 0600)

// 2. 或使用 systemd-ask-password 交互式输入
```

#### 低风险: 解密错误信息 (internal/auth/enhanced_mfa_manager.go:137)

```go
fmt.Printf("警告：解密用户 %s 的 TOTP Secret 失败：%v\n", userID, err)
```

**问题**: 错误信息包含用户 ID，建议使用日志级别控制。

### 3.4 SQL 注入检查

**结果**: ✅ 未发现 SQL 注入风险

检查要点：
- 所有 SQL 查询使用参数化查询 (`?` 占位符)
- `db.Exec(query, args...)` 正确使用
- `db.Query(query, args...)` 正确使用
- 无 `fmt.Sprintf` 构建 SQL 语句

### 3.5 认证授权检查

**结果**: ✅ 认证授权机制完善

发现的安全机制：
- ✅ bcrypt 密码哈希 (`bcrypt.DefaultCost`)
- ✅ argon2id 密钥派生
- ✅ pbkdf2 备份加密
- ✅ JWT Token 验证
- ✅ RBAC 权限控制
- ✅ IP 白名单/黑名单
- ✅ 会话缓存机制
- ✅ 审计日志记录

**中间件实现** (`internal/auth/rbac_middleware.go`):
- `RequireAuth()` - 强制认证
- `RequirePermission()` - 权限检查
- `RequireRole()` - 角色检查
- `RequireAdmin()` - 管理员检查
- `CheckIPAccess()` - IP 访问控制

---

## 四、安全评分

| 类别 | 得分 | 说明 |
|-----|------|-----|
| 依赖安全 | 70/100 | Go 标准库存在已知漏洞 |
| 代码安全 | 75/100 | 整数溢出和命令注入风险 |
| 认证授权 | 90/100 | 机制完善，实现规范 |
| 敏感信息 | 80/100 | 初始密码输出需改进 |
| **综合评分** | **78/100** | 中等安全水平 |

---

## 五、修复优先级

### 高优先级 (P0)

1. **升级 Go 版本至 1.26.1+**
   - 修复 5 个标准库安全漏洞
   - 特别是 crypto/x509 的 panic 漏洞

### 中优先级 (P1)

2. **命令注入风险缓解**
   - `internal/snapshot/executor.go` - 添加输入验证
   - `internal/usbmount/manager.go` - 添加命令白名单

3. **整数溢出检查**
   - 存储容量相关计算添加边界检查
   - 大数值场景添加溢出保护

### 低优先级 (P2)

4. **初始密码传递方式改进**
   - 避免输出到标准输出
   - 考虑更安全的传递机制

---

## 六、建议行动项

### 立即执行

```bash
# 1. 升级 Go 版本
# 修改 go.mod
go 1.26.1

# 2. 更新依赖
go mod tidy
```

### 短期改进 (1-2 周)

1. 为脚本执行添加输入验证
2. 添加命令执行白名单配置
3. 改进初始密码传递机制

### 长期规划

1. 引入静态安全扫描到 CI/CD
2. 定期执行 govulncheck 和 gosec
3. 建立 安全审计自动化流程

---

## 七、结论

NAS-OS v2.74.0 整体安全状态良好，认证授权机制完善，未发现 SQL 注入漏洞。主要风险集中在：

1. **Go 标准库漏洞** - 需升级 Go 版本修复
2. **命令注入风险** - 脚本执行需添加验证
3. **整数溢出** - 大容量存储场景需注意

建议优先修复 Go 版本漏洞，后续迭代逐步完善其他安全问题。

---

**审计人**: 刑部  
**审计时间**: 2026-03-15 22:46 CST  
**下次审计**: v2.75.0 发布前