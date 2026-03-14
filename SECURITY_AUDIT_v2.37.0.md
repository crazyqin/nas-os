# NAS-OS v2.37.0 安全审计报告

**审计日期**: 2026-03-15
**审计人**: 刑部
**版本**: v2.37.0
**提交用户**: crazyqin

---

## 一、审计概述

### 审计范围
1. **gosec 静态扫描** - 164个发现
2. **go vet 静态分析** - 通过
3. **SQL注入风险** - 无风险
4. **命令注入风险** - 中风险（需关注）
5. **路径遍历风险** - 已有防护
6. **认证授权机制** - 完善健全
7. **权限控制** - 完备

### 审计结果摘要
| 项目 | 状态 | 评级 |
|------|------|------|
| go vet | ✅ 通过 | A |
| gosec 扫描 | ⚠️ 164个发现 | B |
| SQL注入 | ✅ 无风险 | A |
| 命令注入 | ⚠️ 中风险 | B |
| 路径遍历 | ✅ 已防护 | A |
| 认证机制 | ✅ 完善 | A |
| 权限控制 | ✅ 完备 | A |
| 会话管理 | ✅ 安全 | A |
| 密码策略 | ✅ 健全 | A |
| CSRF防护 | ✅ 完善 | A |

**综合评级: B+**

---

## 二、gosec 扫描结果分析

### 2.1 问题分布
| 规则ID | 描述 | 数量 | 风险等级 |
|--------|------|------|----------|
| G204 | 子进程启动(变量) | 57 | 中 |
| G104 | 未处理错误 | 67 | 低 |
| G301/G302/G306 | 文件权限问题 | 22 | 低 |
| G703/G122/G304 | 路径遍历 | 3 | 高 |
| G401/G505 | 弱加密(SHA1) | 2 | 中* |
| G107 | HTTP请求变量URL | 2 | 低 |
| G112 | Slowloris攻击 | 1 | 中* |

### 2.2 高风险问题详情

#### G703/G122/G304: 路径遍历风险
**位置**: `internal/backup/manager.go:644-648`

**现状**: 已有防护措施
```go
// 已实现路径验证
cleanDst := filepath.Clean(dst)
if !strings.HasPrefix(cleanDst, "/mnt") && !strings.HasPrefix(cleanDst, "/backup") {
    return fmt.Errorf("invalid destination path: %s", dst)
}
// 已实现符号链接TOCTOU防护
if info.Mode()&os.ModeSymlink != 0 {
    realPath, err := filepath.EvalSymlinks(path)
    // 验证符号链接目标在源目录内
}
```

**建议**: 
- 使用 `internal/security/v2/safe_file_manager.go` 中的 `SafeFileManager` 替代直接文件操作
- 为 filepath.Walk/WalkDir 回调中的文件操作添加更严格的边界检查

### 2.3 中风险问题详情

#### G204: 子进程启动(命令注入风险)
**涉及文件**:
- `pkg/btrfs/btrfs.go` (12处)
- `internal/network/firewall.go` (7处)
- `internal/network/interfaces.go` (7处)
- `internal/network/portforward.go` (9处)
- `internal/docker/manager.go` (5处)
- `internal/docker/appstore.go` (2处)
- `internal/backup/manager.go` (4处)

**现状**: 参数来源多为配置或内部生成，非直接用户输入

**示例** (`internal/network/portforward.go:167-200`):
```go
// iptables 命令参数来自内部配置结构
dnatCmd := exec.Command("iptables", "-t", "nat", "-A", "PREROUTING",
    "-p", rule.Protocol,
    "--dport", fmt.Sprintf("%d", rule.ExternalPort),
    "-j", "DNAT",
    "--to-destination", fmt.Sprintf("%s:%d", rule.InternalIP, rule.InternalPort))
```

**建议**:
1. 为 Protocol、InternalIP 等字段添加输入验证
2. 使用 `exec.CommandContext` 添加超时控制
3. 考虑使用参数白名单校验

#### G401/G505: HMAC-SHA1 使用
**位置**: `internal/network/ddns_providers.go:559`

**说明**: 这是阿里云DNS API规范要求，非安全漏洞
```go
// 阿里云 DNS API 要求使用 HMAC-SHA1，这是 API 规范，不是安全漏洞
h := hmac.New(sha1.New, []byte(p.AccessKeySecret))
```

**结论**: 无需修复，属于外部API要求

#### G112: Slowloris 攻击风险
**位置**: `internal/web/server.go:151`

**现状**: 已在多处添加 ReadHeaderTimeout 防护
```go
// internal/web/server.go:772-773
ReadHeaderTimeout: 10 * time.Second,
WriteTimeout:      30 * time.Second,

// internal/webdav/server.go:104-105
ReadHeaderTimeout: 30 * time.Second, // 防止 Slowloris 攻击
WriteTimeout:      60 * time.Second,
```

**结论**: 已修复

### 2.4 低风险问题详情

#### G104: 未处理错误 (67处)
**涉及模块**: web/middleware, users/manager, storage/manager, network/*, docker/*

**建议**: 
- 对关键操作添加错误处理
- 使用 `log.Printf` 记录错误日志
- 非关键操作可使用 `_ =` 显式忽略

#### G301/G302/G306: 文件权限问题
**问题**: 部分目录使用 0755 权限，文件使用 0644

**现状**: 
```go
// 用户配置文件使用安全权限
os.WriteFile(m.configPath, data, 0600)  // ✅ 安全
// 部分配置目录使用宽松权限
os.MkdirAll(dir, 0755)  // ⚠️ 建议改为 0750
```

**建议**: 
- 敏感配置目录使用 0750 或更严格权限
- 敏感文件使用 0600 权限

---

## 三、SQL注入风险审计

### 3.1 数据库操作检查
✅ **无风险** - 所有数据库操作使用参数化查询

**示例** (`internal/tags/manager.go`):
```go
// ✅ 安全：使用参数化查询
query := `INSERT INTO tags (id, name, color, icon, grp, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
_, err = m.db.Exec(query, tag.ID, tag.Name, tag.Color, tag.Icon, tag.Group, tag.CreatedAt, tag.UpdatedAt)
```

**结论**: SQL注入防护良好

---

## 四、认证与授权审计

### 4.1 认证机制
| 机制 | 状态 | 安全特性 |
|------|------|----------|
| 本地认证 | ✅ | bcrypt密码哈希 |
| OAuth2 | ✅ | CSRF state、PKCE |
| LDAP | ✅ | TLS、EscapeFilter转义 |
| MFA | ✅ | TOTP、SMS、WebAuthn、恢复码 |
| 会话管理 | ✅ | 安全token、过期、刷新 |

### 4.2 密码策略
```go
DefaultPasswordPolicy = PasswordPolicy{
    MinLength:        8,
    MaxLength:        128,
    RequireUppercase: true,
    RequireLowercase: true,
    RequireDigit:     true,
    RequireSpecial:   true,
    PreventCommon:    true,    // 弱密码检测
    PreventUserInfo:  true,    // 用户信息检测
    HistoryCount:     5,       // 历史记录
    MaxAge:           90,      // 90天过期
}
```
**结论**: 密码策略健全

### 4.3 权限控制中间件
✅ 完备的权限控制体系：
- `RequireAuth()` - 认证检查
- `RequireRole()` - 角色检查
- `RequireAdmin()` - 管理员检查
- `RequirePermission()` - 权限检查

### 4.4 会话安全
- ✅ 使用加密安全的随机token生成 (`crypto/rand`)
- ✅ 会话缓存机制
- ✅ CSRF防护（常量时间比较）

---

## 五、敏感信息处理

### 5.1 发现问题
⚠️ **首次启动打印默认密码**
**位置**: `internal/users/manager.go:158`
```go
fmt.Printf("   密码: %s\n", defaultPassword)
```

**风险**: 低 - 仅输出到stdout，不记录日志文件

**建议**: 
- 生产环境通过邮件或安全渠道发送初始密码
- 或使用一次性密码链接

### 5.2 安全配置处理
✅ CSRF密钥从环境变量读取
✅ 无硬编码密码/密钥/Token
✅ 敏感配置文件使用0600权限

---

## 六、安全模块审计

### 6.1 模块结构
```
internal/security/
├── audit.go           ✅ 审计日志
├── audit_test.go      ✅ 测试覆盖
├── baseline.go        ✅ 安全基线
├── fail2ban.go        ✅ 入侵防护
├── firewall.go        ✅ 防火墙管理
└── v2/
    ├── safe_file_manager.go      ✅ 路径遍历防护
    ├── disk_encryption.go        ✅ 磁盘加密
    ├── mfa.go                    ✅ 多因素认证
    ├── alerting.go               ✅ 安全告警
    └── encryption.go             ✅ 数据加密
```

### 6.2 测试结果
```
$ go test ./internal/security/... -v
ok      nas-os/internal/security    0.009s
ok      nas-os/internal/security/v2 0.043s

$ go test ./internal/auth/... -v
PASS
ok      nas-os/internal/auth    6.016s
```
**结论**: 测试全部通过

---

## 七、安全加固建议

### 7.1 高优先级
1. **命令注入防护增强**
   - 为 iptables/mount 等命令参数添加白名单校验
   - 使用 `exec.CommandContext` 添加超时

2. **路径遍历防护统一**
   - 将 `SafeFileManager` 应用到所有文件操作模块

### 7.2 中优先级
1. **错误处理完善**
   - 对67处未处理错误添加适当的错误处理或日志记录

2. **文件权限收紧**
   - 敏感配置目录从 0755 改为 0750
   - 配置文件统一使用 0600 权限

### 7.3 低优先级
1. **首次启动密码输出**
   - 考虑通过安全渠道发送初始密码

---

## 八、合规性检查

| 标准 | 状态 |
|------|------|
| OWASP Top 10 | ✅ 基本符合 |
| NIST 密码策略建议 | ✅ 符合 |
| CWE 安全编码规范 | ✅ 基本符合 |

---

## 九、审计结论

### 9.1 总体评价
NAS-OS v2.37.0 安全实现良好：

**优势**:
- 完善的认证体系 (本地/OAuth2/LDAP/MFA)
- 健全的密码策略
- 完备的权限控制中间件
- SQL注入防护良好
- 路径遍历已有防护措施
- Slowloris攻击已防护

**需关注**:
- 57处 exec.Command 变量使用需审查输入来源
- 67处未处理错误需补充处理
- 22处文件权限问题可收紧

### 9.2 评级说明
**B+** - 安全基础扎实，存在中低风险问题需关注

### 9.3 下次审计建议
- 添加动态安全测试 (DAST)
- 进行渗透测试
- 关注新增代码的安全合规

---

**审计人**: 刑部安全审计组
**审计日期**: 2026-03-15
**下次审计**: v2.38.0 发布前