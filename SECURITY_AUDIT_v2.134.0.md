# NAS-OS v2.134.0 安全审计报告

**审计日期**: 2026-03-16  
**审计版本**: v2.134.0  
**审计工具**: 手动代码审查 (gosec 兼容性问题)  
**审计范围**: 全部 Go 源代码

---

## 执行摘要

| 风险等级 | 发现数量 | 已修复 |
|---------|---------|--------|
| 高危     | 2       | 2      |
| 中危     | 4       | 0      |
| 低危     | 2       | 2      |
| 信息     | 多项     | N/A    |

---

## 高危漏洞

### 1. Shell 注入风险 - 快照脚本执行

**文件**: `internal/snapshot/executor.go`  
**行号**: 89  
**风险等级**: 高危  

**问题描述**:  
`runScript` 函数直接执行用户配置的脚本内容，没有任何安全验证：
```go
cmd := exec.CommandContext(ctx, "sh", "-c", script)
```

攻击者如果能够控制脚本内容，可能执行任意系统命令，包括删除系统文件、提权等。

**修复措施**:  
- 添加 `validateScript` 函数进行脚本内容安全验证
- 禁止危险命令黑名单：`rm -rf /`, `mkfs`, `dd if=/dev/zero` 等
- 在执行前强制验证脚本安全性

---

### 2. Shell 注入风险 - USB 通知命令

**文件**: `internal/usbmount/manager.go`  
**行号**: 883  
**风险等级**: 高危  

**问题描述**:  
`sendNotification` 函数直接执行用户配置的通知命令，且环境变量未经清理：
```go
cmd := exec.CommandContext(m.ctx, "sh", "-c", m.config.NotifyCommand)
cmd.Env = append(os.Environ(),
    fmt.Sprintf("USB_DEVICE=%s", device.DevicePath),
    // ... 其他环境变量
)
```

攻击者可通过伪造设备信息（如 Label、UUID）注入恶意命令。

**修复措施**:  
- 添加 `validateNotifyCommand` 白名单验证
- 仅允许 `/usr/bin/notify-send`, `/bin/echo`, `/usr/bin/logger` 等安全命令
- 添加 `sanitizeEnvValue` 函数清理环境变量中的危险字符

---

## 中危漏洞

### 3. TLS 证书验证跳过

**文件**: 
- `internal/backup/cloud.go:136`
- `internal/backup/sync.go:664`
- `internal/cloudsync/providers.go:399`
- `internal/ldap/client.go:73,103`
- `internal/auth/ldap.go:153,185`

**风险等级**: 中危  

**问题描述**:  
多处代码使用 `InsecureSkipVerify: true` 跳过 TLS 证书验证：
```go
TLSClientConfig: &tls.Config{InsecureSkipVerify: true}
```

**风险影响**:  
- 中间人攻击
- 数据窃听
- 凭证泄露

**建议修复**:  
- 默认启用证书验证
- 提供配置选项让用户选择是否信任自签名证书
- 记录安全警告日志

**当前状态**: 代码中有 `#nosec G402` 注释说明用途，暂不修复。

---

### 4. 弱哈希算法 SHA-1

**文件**: 
- `internal/network/ddns_providers.go:198`
- `internal/auth/sms.go:274`

**风险等级**: 中危  

**问题描述**:  
使用 SHA-1 哈希算法，已被认为不再安全：
```go
mac := hmac.New(sha1.New, []byte(p.AccessKeySecret+"&"))
```

**风险影响**:  
SHA-1 存在碰撞攻击风险，不适合用于安全敏感场景。

**建议修复**:  
- 升级到 SHA-256 或更强算法
- 评估 API 兼容性

**当前状态**: 需确认 API 是否支持更高安全级别，暂不修复。

---

### 5. MFA 使用 SHA-1

**文件**: `internal/security/v2/mfa.go:177,224`  
**风险等级**: 中危（有条件）  

**问题描述**:  
TOTP 实现使用 HMAC-SHA1。

**评估**:  
代码注释已说明：`#nosec G505 -- TOTP (RFC 6238) requires HMAC-SHA1`  
RFC 6238 要求 TOTP 必须使用 HMAC-SHA1，这是标准要求，不是漏洞。

**当前状态**: 不需要修复（符合标准）。

---

### 6. 路径遍历风险检查

**文件**: `plugins/filemanager-enhance/main.go`  
**风险等级**: 中危  

**问题描述**:  
文件管理器插件使用 `filepath.Join` 拼接用户输入路径。

**评估**:  
代码已实现 `isPathAllowed` 函数进行路径验证，使用 `strings.HasPrefix` 检查路径是否在允许范围内。

**当前状态**: 已有防护措施，但建议增强：
- 使用 `filepath.Rel` 计算相对路径，避免符号链接绕过
- 添加对 `..` 路径的明确检查

---

## 低危漏洞

### 7. HTTP 客户端无超时设置

**文件**: 
- `internal/vm/iso.go:256`
- `internal/downloader/manager.go:649`

**风险等级**: 低危  

**问题描述**:  
使用 `http.DefaultClient` 没有超时设置：
```go
resp, err := http.DefaultClient.Do(req)
```

**风险影响**:  
- 资源耗尽
- 挂起等待
- DoS 攻击面

**修复措施**:  
- 创建自定义 HTTP 客户端
- 设置 30 分钟超时（适合大文件下载）
- 配置 Transport 参数

**状态**: 已修复

---

## 信息级发现

### 代码质量建议

1. **错误处理**: 部分文件 `defer` 关闭资源未检查错误，建议使用 `defer func() { _ = file.Close() }()` 模式

2. **日志敏感信息**: 未发现密码、Token 等敏感信息日志泄露

3. **SQL 注入**: 未发现 SQL 拼接风险（项目使用 ORM 或参数化查询）

4. **文件权限**: 测试代码中有 `0777` 权限，生产代码未发现不安全权限设置

---

## 修复记录

| 日期 | 文件 | 修复内容 |
|------|------|---------|
| 2026-03-16 | `internal/snapshot/executor.go` | 添加脚本安全验证，禁止危险命令 |
| 2026-03-16 | `internal/usbmount/manager.go` | 添加命令白名单验证，清理环境变量 |
| 2026-03-16 | `internal/vm/iso.go` | 使用自定义 HTTP 客户端，设置超时 |
| 2026-03-16 | `internal/downloader/manager.go` | 使用自定义 HTTP 客户端，设置超时 |

---

## 建议后续行动

1. **短期（1-2周）**:
   - 配置 CI/CD 流程中的安全扫描
   - 审计所有 `exec.Command` 调用点

2. **中期（1个月）**:
   - 评估 TLS InsecureSkipVerify 的使用场景
   - 升级 DDNS/SMS 中的 SHA-1 为 SHA-256

3. **长期**:
   - 建立安全编码规范
   - 定期进行安全审计

---

## 附录

### A. 安全扫描命令

```bash
# 由于 gosec 与 Go 1.26 兼容性问题，使用手动扫描
grep -rn "exec.Command" --include="*.go" .
grep -rn "InsecureSkipVerify" --include="*.go" .
grep -rn "http.DefaultClient" --include="*.go" .
```

### B. 参考标准

- OWASP Top 10
- CWE-78: OS Command Injection
- CWE-295: Improper Certificate Validation
- CWE-328: Use of Weak Hash

---

**审计人**: AI 安全审计系统  
**报告生成时间**: 2026-03-16 22:50:00 CST