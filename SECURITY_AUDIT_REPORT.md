# NAS-OS 安全审计报告

**审计日期**: 2026-03-19  
**审计范围**: ~/clawd/nas-os  
**审计工具**: gosec v2, govulncheck  

---

## 执行摘要

本次安全审计发现了 **多个高危安全漏洞**，需要优先修复。主要问题集中在：

1. **命令注入/路径遍历** - 大量外部命令执行和文件操作存在输入验证不足
2. **整数溢出** - 大量类型转换可能导致溢出
3. **TLS 配置不当** - LDAP 客户端跳过证书验证
4. **SMTP 注入** - 邮件发送功能存在注入风险

---

## 关键安全问题 (需立即修复)

### 🔴 1. 命令注入漏洞 (G702/G204) - 高危

**风险等级**: 高  
**影响范围**: 60+ 处代码位置  

**问题描述**:  
大量使用 `exec.Command` 执行外部命令，参数可能来自用户输入或未充分验证的来源。

**典型示例**:
- `internal/vm/manager.go` - virsh 命令执行
- `internal/security/firewall.go` - iptables 命令执行
- `internal/backup/manager.go` - tar/openssl 命令执行
- `internal/container/*.go` - docker 命令执行

**修复建议**:
1. 使用 `internal/security/cmdsec` 包中的安全命令执行方法
2. 对所有外部输入进行严格验证和白名单过滤
3. 避免使用 `sh -c` 执行命令

---

### 🔴 2. 路径遍历漏洞 (G703) - 高危

**风险等级**: 高  
**影响范围**: 50+ 处代码位置  

**问题描述**:  
文件操作（os.Stat, os.Open, os.WriteFile 等）使用的路径可能来自用户输入，未进行充分的路径验证。

**典型示例**:
- `internal/webdav/server.go` - WebDAV 文件操作
- `internal/backup/*.go` - 备份文件操作
- `internal/vm/snapshot.go` - 快照文件操作
- `plugins/filemanager-enhance/main.go:549` - 文件写入

**修复建议**:
1. 使用 `pkg/safeguards` 包中的路径验证方法
2. 对所有用户提供的路径进行规范化并检查是否在允许的根目录内
3. 使用 `filepath.Clean` 和 `filepath.Rel` 验证路径

---

### 🔴 3. SMTP 命令注入 (G707) - 高危

**风险等级**: 高  
**位置**: `internal/automation/action/action.go:287`

**问题描述**:  
邮件发送功能中，邮件内容可能包含 SMTP 命令注入字符。

**修复建议**:
1. 对邮件头和内容进行严格过滤
2. 使用 Go 标准库的 `net/smtp` 安全方法
3. 验证收件人地址格式

---

### 🟠 4. TLS 证书验证跳过 (G402) - 中高危

**风险等级**: 中高  
**影响范围**: 3 处  
**位置**: 
- `internal/ldap/client.go:73, 107`
- `internal/auth/ldap.go:153`

**问题描述**:  
LDAP 客户端配置中 `InsecureSkipVerify: skipVerify` 可能被设置为 true，导致跳过 TLS 证书验证。

**修复建议**:
1. 默认禁止跳过证书验证
2. 如必须使用，应明确记录并限制使用场景
3. 考虑使用自定义证书池代替跳过验证

---

### 🟠 5. XSS 漏洞 (G705) - 中危

**风险等级**: 中  
**影响范围**: 2 处  
**位置**: `internal/automation/api/handlers.go:280, 416`

**问题描述**:  
Content-Disposition 头中的文件名可能包含恶意字符。

**修复建议**:
1. 对文件名进行 URL 编码或移除特殊字符
2. 使用 `mime.FormatMediaType` 安全设置头

---

## 中等风险问题

### 🟡 6. 整数溢出 (G115) - 中危

**风险等级**: 中  
**影响范围**: 70+ 处代码位置  

**问题描述**:  
大量类型转换（uint64↔int64, int↔uint64 等）可能导致整数溢出。

**典型示例**:
- `internal/quota/optimizer/optimizer.go` - 配额计算
- `internal/photos/handlers.go` - 文件大小处理
- `internal/container/volume.go` - 卷大小计算

**修复建议**:
1. 使用 `pkg/safeguards` 包中的安全转换方法
2. 在转换前检查边界条件

---

### 🟡 7. Context 泄漏 (G118) - 中危

**风险等级**: 中  
**影响范围**: 10+ 处  

**问题描述**:  
`context.WithCancel/WithTimeout` 返回的 cancel 函数未被调用，可能导致 goroutine 泄漏。

**修复建议**:
1. 使用 `defer cancel()` 确保资源释放
2. 审查所有 context 创建点

---

### 🟡 8. TOCTOU 竞态条件 (G122) - 中危

**风险等级**: 中  
**影响范围**: 6 处  

**问题描述**:  
`filepath.Walk/WalkDir` 回调中的文件操作可能存在 Time-Of-Check-Time-Of-Use 竞态条件。

**修复建议**:
1. 使用 `os.Root` API（Go 1.24+）
2. 或使用文件描述符操作代替路径操作

---

## 低风险问题

### ⚪ 9. 硬编码凭证误报 (G101)

**风险等级**: 低（误报）  
**影响范围**: 5 处  

**说明**:  
OAuth2 配置函数（`GetGoogleOAuth2Config` 等）被误报为硬编码凭证，实际这些是配置模板，实际凭证由参数传入。

---

## 统计摘要

| 风险等级 | 问题类型 | 数量 |
|---------|---------|------|
| 🔴 高危 | 命令注入/路径遍历 | 110+ |
| 🔴 高危 | SMTP 注入 | 1 |
| 🟠 中高 | TLS 跳过验证 | 3 |
| 🟠 中危 | XSS | 2 |
| 🟡 中危 | 整数溢出 | 70+ |
| 🟡 中危 | Context 泄漏 | 10+ |
| 🟡 中危 | TOCTOU | 6 |
| ⚪ 低危 | 硬编码凭证(误报) | 5 |

---

## 修复优先级建议

1. **P0 (立即修复)**:
   - SMTP 注入漏洞
   - WebDAV 路径遍历
   - VM/容器命令注入

2. **P1 (本周修复)**:
   - TLS 证书验证问题
   - 备份模块路径遍历
   - XSS 漏洞

3. **P2 (本月修复)**:
   - 整数溢出问题
   - Context 泄漏
   - TOCTOU 竞态条件

---

## 依赖漏洞检查

由于编译错误，govulncheck 未能完成检查。建议：
1. 先修复编译错误（`internal/health/health.go` 中的未定义类型）
2. 重新运行 `govulncheck ./...` 检查依赖漏洞

---

## 已有安全机制

项目已实现部分安全机制：
- `pkg/safeguards` - 安全类型转换和路径验证
- `internal/security/cmdsec` - 安全命令执行
- 部分代码已添加 `#nosec` 注释说明安全原因

**建议**: 扩展这些安全包的使用范围，替换不安全的直接操作。

---

**审计完成时间**: 2026-03-19 07:06  
**审计人**: 刑部 (安全审计子代理)