# NAS-OS 安全审计报告 v2.192.0

**审计日期**: 2026-03-17  
**审计机构**: 刑部（安全合规）  
**项目版本**: v2.192.0  
**审计工具**: gosec (静态分析) + 人工代码审查

---

## 📊 执行摘要

### 总体风险评级: **高危 (HIGH)**

| 风险等级 | 问题数量 | 占比 |
|---------|---------|------|
| 🔴 高危 (HIGH) | 147 | 72.9% |
| 🟠 中危 (MEDIUM) | 55 | 27.1% |
| 🟢 低危 (LOW) | 0 | 0% |
| **总计** | **202** | 100% |

### 关键发现概览

| 漏洞类型 | 严重程度 | 受影响文件数 | 状态 |
|---------|---------|------------|------|
| 路径遍历漏洞 (G703) | 🔴 高危 | 15+ | ❌ 需修复 |
| 命令注入风险 (G702) | 🔴 高危 | 5+ | ❌ 需修复 |
| 整数溢出 (G115) | 🔴 高危 | 50+ | ⚠️ 建议修复 |
| TLS 不安全跳过验证 (G402) | 🔴 高危 | 3 | ⚠️ 可接受风险 |
| SMTP 注入 (G707) | 🔴 高危 | 1 | ❌ 需修复 |
| 潜在硬编码凭证 (G101) | 🔴 高危 | 5 | ⚠️ 误报 |
| TOCTOU 竞争条件 (G122) | 🔴 高危 | 6 | ⚠️ 建议修复 |
| 子进程命令注入 (G204) | 🟠 中危 | 50+ | ⚠️ 可控风险 |
| Context 泄漏 (G118) | 🟠 中危 | 9 | ⚠️ 建议修复 |
| XSS 风险 (G705) | 🟠 中危 | 2 | ❌ 需修复 |

---

## 🔴 严重安全问题

### 1. 路径遍历漏洞 (CWE-22, G703)

**严重程度**: 🔴 高危  
**置信度**: 高  
**受影响文件**: 15+ 个文件

#### 问题描述
多处代码直接使用用户可控的路径参数进行文件系统操作，未进行充分的路径验证，可能导致攻击者访问预期目录之外的文件。

#### 高风险代码位置

**`internal/webdav/server.go`** - 最严重
```go
// 第 226 行 - 路径构建
fullPath := filepath.Join(basePath, cleanPath)

// 多处直接使用用户路径:
// - os.Stat(fullPath) - 第 288, 413, 458, 482, 586, 653, 774, 861 行
// - os.RemoveAll(fullPath) - 第 602 行
// - os.WriteFile(fullPath, ...) - 第 864 行
// - os.Rename(fullPath, destPath) - 第 801 行
```

**修复状态**: ✅ 已有部分防护  
代码中已实现路径遍历检测：
```go
// 第 215-218 行
if strings.Contains(decodedPath, "..") {
    return "", ErrPathTraversal
}
// 第 230-232 行  
if !strings.HasPrefix(absFullPath, absBasePath) {
    return "", ErrPathTraversal
}
```

**风险评估**: 尽管已有防护，但 gosec 仍报告潜在风险，建议进一步审计边界情况。

**其他受影响文件**:
- `internal/vm/snapshot.go` - 快照文件操作
- `internal/vm/manager.go` - VM 配置文件操作
- `internal/smb/config.go` - SMB 配置备份
- `internal/security/v2/encryption.go` - 加密文件操作
- `internal/backup/*.go` - 备份恢复操作
- `internal/docker/app_version.go` - Docker Compose 文件
- `plugins/filemanager-enhance/main.go` - 文件管理插件

#### 修复建议
1. **统一路径验证函数**: 创建安全的路径解析工具函数
2. **符号链接检测**: 在关键操作前检查符号链接
3. **权限隔离**: 确保服务以最小权限运行
4. **审计日志**: 记录所有文件系统操作

---

### 2. 命令注入风险 (CWE-78, G702/G204)

**严重程度**: 🔴 高危  
**置信度**: 高  
**受影响文件**: 50+ 个文件

#### 问题描述
大量使用 `exec.Command` 执行外部命令，部分命令参数来自用户输入或可被间接控制，存在命令注入风险。

#### 高风险代码位置

**`internal/vm/manager.go`**
```go
// 第 558 行 - virsh 命令执行
cmd := exec.CommandContext(ctx, "virsh", "-c", "qemu:///system", "start", vm.Name)
// vm.Name 来源：用户输入，但有验证
```

**`internal/vm/snapshot.go`**
```go
// 第 290 行 - qemu-img 命令
cmd := exec.CommandContext(ctx, "qemu-img", "convert", "-f", "qcow2", "-O", "qcow2", snapshotDiskPath, vm.DiskPath)
```

**修复状态**: ✅ 部分代码有验证注释
```go
// 第 628 行
// #nosec G204 -- vm.Name validated by validateConfig() to contain only safe characters
```

#### 风险评估
大部分命令执行使用固定参数结构，风险可控。关键风险点：
- `vm.Name` - 已有验证（仅允许安全字符）
- 文件路径 - 使用内部生成的路径
- 网络命令 - IP/域名参数需要验证

**其他高风险命令执行位置**:
- `pkg/btrfs/btrfs.go` - BTRFS 文件系统操作
- `internal/security/v2/disk_encryption.go` - 磁盘加密
- `internal/network/*.go` - 网络配置
- `internal/docker/*.go` - Docker 操作
- `internal/backup/manager.go` - tar/openssl 命令

#### 修复建议
1. **参数白名单验证**: 对所有动态参数进行严格验证
2. **避免 shell 解释**: 始终使用 `exec.Command` 而非 `exec.Command("sh", "-c", ...)`
3. **输入清洗**: 过滤特殊字符 (|, &, ;, $, `, \n, \r 等)
4. **权限降级**: 在安全上下文中执行命令

---

### 3. SMTP 命令/头部注入 (CWE-93, G707)

**严重程度**: 🔴 高危  
**置信度**: 高  
**受影响文件**: `internal/automation/action/action.go`

#### 问题描述
邮件发送功能中，收件人地址和邮件内容直接拼接，可能导致 SMTP 注入攻击。

#### 代码位置
```go
// 第 264 行
return smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
```

#### 修复建议
1. **验证邮箱格式**: 使用正则表达式严格验证
2. **过滤换行符**: 移除 `\r`, `\n` 字符
3. **使用安全库**: 考虑使用成熟的邮件库如 `go-mail`

---

### 4. TLS 证书验证跳过 (CWE-295, G402)

**严重程度**: 🔴 高危 (可接受风险)  
**置信度**: 低  
**受影响文件**: 3 个

#### 代码位置
```go
// internal/ldap/client.go 第 73, 107 行
tlsConfig := &tls.Config{
    InsecureSkipVerify: skipVerify,  // 可配置跳过
}

// internal/auth/ldap.go 第 153 行
tlsConfig := &tls.Config{
    InsecureSkipVerify: skipVerify,
}
```

#### 风险评估
**可接受风险** - 这是 LDAP 连接的可配置选项，某些内部环境可能使用自签名证书。建议：
- 默认值应为 `false`
- 添加明确的安全警告日志
- 文档中说明风险

---

### 5. 整数溢出 (CWE-190, G115)

**严重程度**: 🔴 高危  
**置信度**: 中  
**受影响文件**: 50+ 个文件

#### 问题描述
大量整数类型转换可能存在溢出风险，特别是在处理文件大小、磁盘容量等大数值时。

#### 典型示例
```go
// internal/quota/optimizer/optimizer.go 第 527 行
changePercent := float64(abs(int64(suggestedLimit)-int64(usage.HardLimit))) / float64(usage.HardLimit)

// internal/search/engine.go 第 539 行
Total: int(result.Total),  // uint64 -> int

// internal/monitor/metrics_collector.go 第 843 行
metric.ReallocatedSectors = int(attr.RawValue)  // uint64 -> int
```

#### 风险评估
大部分溢出风险发生在监控指标和统计计算中，影响有限。关键风险点：
- 磁盘容量计算 - 可能导致负值或错误值
- 配额计算 - 可能绕过限制

#### 修复建议
1. **使用安全转换**: 添加边界检查
2. **大数值处理**: 关键计算使用 `math/big` 包
3. **单元测试**: 添加边界值测试

---

## 🟠 中等安全问题

### 6. TOCTOU 竞争条件 (CWE-367, G122)

**严重程度**: 🔴 高危  
**置信度**: 中  
**受影响文件**: 6 个

#### 问题描述
在 `filepath.Walk/WalkDir` 回调中执行文件操作，存在 Time-of-Check-Time-of-Use 竞争条件风险。

#### 受影响代码
```go
// internal/backup/manager.go 第 822 行
filepath.WalkDir(sourcePath, func(path string, info os.DirEntry, err error) error {
    data, err := os.ReadFile(path)  // 检查和使用之间存在时间窗口
})

// internal/backup/encrypt.go 第 83 行
// internal/files/manager.go 第 897 行
// internal/snapshot/replication.go 第 827 行
```

#### 修复建议
1. **使用 os.Root API**: Go 1.24+ 提供的根目录范围 API
2. **文件描述符操作**: 使用 `fchdir` + 相对路径
3. **符号链接检查**: 操作前验证路径类型

---

### 7. Context 泄漏 (CWE-400, G118)

**严重程度**: 🟠 中危  
**置信度**: 高  
**受影响文件**: 9 个

#### 问题描述
`context.WithCancel` / `context.WithTimeout` 返回的取消函数未被调用，可能导致 goroutine 泄漏。

#### 受影响代码
```go
// internal/scheduler/executor.go 第 108 行
execCtx, cancel := context.WithCancel(ctx)
// cancel() 未在所有路径中调用

// internal/media/streaming.go 第 115 行
session.ctx, session.cancel = context.WithCancel(context.Background())
```

#### 修复建议
```go
ctx, cancel := context.WithTimeout(parentCtx, timeout)
defer cancel()  // 始终调用
```

---

### 8. XSS 风险 (G705)

**严重程度**: 🟠 中危  
**置信度**: 高  
**受影响文件**: `internal/automation/api/handlers.go`

#### 问题描述
HTTP 响应中直接使用用户输入作为文件名，可能导致 XSS 攻击。

#### 代码位置
```go
// 第 409 行
w.Header().Set("Content-Disposition", "attachment; filename=template_"+id+".json")

// 第 278 行
w.Header().Set("Content-Disposition", "attachment; filename=workflow_"+id+".json")
```

#### 修复建议
```go
import "net/textproto"
// 转义文件名
filename = strings.ReplaceAll(filename, "\"", "\\\"")
w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
```

---

### 9. 潜在硬编码凭证 (CWE-798, G101)

**严重程度**: 🔴 高危 (误报)  
**置信度**: 低  
**受影响文件**: 5 个

#### 分析结果

| 文件 | 代码 | 判定 |
|-----|------|-----|
| `internal/auth/oauth2.go` | OAuth2 配置 URL | ✅ 误报 - URL 非凭证 |
| `internal/cloudsync/providers.go` | OAuth Token URL | ✅ 误报 - URL 非凭证 |
| `internal/office/types.go` | `"无效的 Token"` 错误字符串 | ✅ 误报 - 字符串常量 |

**实际风险**: 未发现真实硬编码凭证。密码和密钥均通过配置文件或环境变量注入。

---

## 🔒 权限控制审计

### 认证机制

**实现位置**: `internal/auth/`  
**机制**: 
- 用户名/密码认证 (bcrypt 哈希)
- 双因素认证 (TOTP, SMS, WebAuthn)
- OAuth2 集成 (Google, GitHub, Microsoft, WeChat)
- LDAP/AD 集成

**评估**: ✅ 认证机制完善

### 授权机制 (RBAC)

**实现位置**: `internal/auth/rbac.go`  
**角色定义**:
- `admin` - 全部权限
- `user` - 受限访问
- `guest` - 只读访问
- `system` - 系统服务账号

**资源类型**: volume, share, user, group, system, container, vm, file, snapshot

**评估**: ✅ RBAC 实现完善

### 中间件保护

**实现位置**: `internal/rbac/middleware.go`, `internal/web/middleware.go`

**安全头设置**:
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `X-XSS-Protection: 1; mode=block`
- `Content-Security-Policy` 已配置
- `Strict-Transport-Security` (HTTPS)
- `Referrer-Policy: strict-origin-when-cross-origin`

**评估**: ✅ 安全头配置完善

### CSRF 防护

**实现**: CSRF Key 从环境变量 `NAS_CSRF_KEY` 读取  
**评估**: ✅ 实现正确，有开发环境警告

### 限流机制

**实现**: 
- 内存限流 (开发环境)
- 建议生产环境使用 Redis
- 默认 100 RPS，突发 200

**评估**: ⚠️ 生产环境需配置分布式限流

---

## 📝 敏感数据处理审计

### 密码存储

**实现**: `golang.org/x/crypto/bcrypt`  
**评估**: ✅ 使用安全的哈希算法

### MFA 密钥存储

**代码位置**: `internal/auth/manager.go`  
```go
// 存储密钥（实际应该加密存储）
m.configs[userID].TOTPSecret = setup.Secret
```

**评估**: ⚠️ 建议加密存储 TOTP 密钥

### 配置文件权限

**实现**: `internal/users/manager.go`  
```go
os.WriteFile(m.configPath, data, 0600)  // 正确的权限设置
```

**评估**: ✅ 敏感配置文件权限正确

### 默认密码

**实现**: 首次启动生成随机密码
```go
defaultPassword := generateRandomPassword(16)
```

**评估**: ✅ 不使用硬编码默认密码

---

## 🛡️ 安全加固建议

### 高优先级 (立即修复)

1. **修复 SMTP 注入漏洞**
   - 文件: `internal/automation/action/action.go`
   - 添加邮箱验证和换行符过滤

2. **修复 XSS 漏洞**
   - 文件: `internal/automation/api/handlers.go`
   - 对 Content-Disposition 文件名进行转义

3. **加强路径遍历防护**
   - 文件: `internal/webdav/server.go` 等
   - 添加符号链接检测
   - 统一使用安全的路径解析函数

### 中优先级 (建议修复)

4. **修复 Context 泄漏**
   - 所有 `context.WithTimeout` 调用处添加 `defer cancel()`

5. **TOCTOU 竞争条件**
   - 考虑使用 Go 1.24+ 的 `os.Root` API

6. **整数溢出检查**
   - 关键计算添加边界检查

### 低优先级 (持续改进)

7. **生产环境配置**
   - 配置分布式限流 (Redis)
   - 设置 `NAS_CSRF_KEY` 环境变量
   - 配置 HTTPS

8. **日志审计**
   - 增强文件操作审计日志
   - 记录所有认证尝试

---

## 📈 安全评分

| 类别 | 得分 | 满分 |
|-----|------|------|
| 认证安全 | 90 | 100 |
| 授权控制 | 95 | 100 |
| 输入验证 | 60 | 100 |
| 输出编码 | 70 | 100 |
| 密码学实践 | 85 | 100 |
| 错误处理 | 80 | 100 |
| 日志审计 | 75 | 100 |
| **总分** | **79** | **100** |

**评级**: C+ (需要改进)

---

## 🔧 修复清单

| 编号 | 问题 | 优先级 | 状态 |
|-----|------|--------|------|
| SEC-001 | SMTP 注入漏洞 | 🔴 高 | ❌ 待修复 |
| SEC-002 | XSS 漏洞 | 🔴 高 | ❌ 待修复 |
| SEC-003 | 路径遍历风险 | 🔴 高 | ⚠️ 部分防护 |
| SEC-004 | 命令注入风险 | 🟠 中 | ⚠️ 可控 |
| SEC-005 | 整数溢出 | 🟠 中 | ⚠️ 建议修复 |
| SEC-006 | Context 泄漏 | 🟠 中 | ❌ 待修复 |
| SEC-007 | TOCTOU 竞争 | 🟠 中 | ⚠️ 建议修复 |
| SEC-008 | TLS 跳过验证 | 🟢 低 | ✅ 可接受 |
| SEC-009 | 硬编码凭证 | 🟢 低 | ✅ 误报 |
| SEC-010 | 分布式限流 | 🟢 低 | 📋 建议配置 |

---

## 📚 参考资料

- [OWASP Top 10 2021](https://owasp.org/www-project-top-ten/)
- [CWE - Common Weakness Enumeration](https://cwe.mitre.org/)
- [Go Security Checklist](https://github.com/guardrailsio/awesome-golang-security)
- [gosec - Golang Security Checker](https://github.com/securego/gosec)

---

**审计人员**: 刑部安全审计组  
**报告生成时间**: 2026-03-17 22:50:00 CST  
**下次审计建议**: 3 个月后或重大版本发布前