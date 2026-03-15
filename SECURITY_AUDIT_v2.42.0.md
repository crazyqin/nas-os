# NAS-OS v2.42.0 安全审计报告

## 扫描信息
- **扫描时间**: 2026-03-15 07:56 CST
- **扫描工具**: gosec v2
- **项目版本**: v2.42.0
- **项目路径**: /home/mrafter/clawd/nas-os

## 扫描结果摘要

**结论: 发现高危漏洞，需要修复**

共发现 **超过 150 个安全问题**，其中大部分为高危漏洞。

## 风险等级统计

| 规则ID | 严重程度 | 漏洞类型 | 数量 |
|--------|----------|----------|------|
| G702 | **严重** | 命令注入 | 10+ |
| G703 | **严重** | 路径遍历 | 50+ |
| G705 | **严重** | XSS跨站脚本 | 2 |
| G707 | **严重** | SMTP注入 | 1 |
| G204 | **高危** | 子进程变量执行 | 70+ |
| G402 | **高危** | TLS证书跳过验证 | 4 |
| G401 | **中危** | 弱加密算法(MD5) | 7 |
| G115 | **中危** | 整数溢出 | 50+ |
| G101 | **低危** | 潜在硬编码凭证 | 7 |
| G118 | **低危** | Context资源泄漏 | 6 |
| G122 | **低危** | 文件系统竞态条件 | 6 |

## 高危漏洞详情

### 1. 命令注入 (G702) - 严重
**影响文件:**
- `internal/vm/manager.go` - virsh 命令执行
- `internal/vm/snapshot.go` - qemu-img/virsh 命令执行

**风险:** 攻击者可通过 VM 名称、路径等参数注入任意命令

### 2. 路径遍历 (G703) - 严重
**影响文件:**
- `internal/webdav/server.go` - WebDAV 文件操作
- `internal/backup/manager.go` - 备份文件操作
- `internal/backup/encrypt.go` - 加密文件操作
- `internal/vm/snapshot.go` - VM 快照文件操作
- `internal/vm/manager.go` - VM 文件操作
- `plugins/filemanager-enhance/main.go` - 文件管理插件

**风险:** 攻击者可访问系统任意文件，读取敏感信息或写入恶意文件

### 3. XSS跨站脚本 (G705) - 严重
**影响文件:**
- `internal/automation/api/handlers.go`

**风险:** 未转义的用户输入直接写入响应，可能导致XSS攻击

### 4. SMTP注入 (G707) - 严重
**影响文件:**
- `internal/automation/action/action.go`

**风险:** SMTP命令/头注入，可能导致邮件欺骗或钓鱼攻击

### 5. 子进程变量执行 (G204) - 高危
**影响文件:**
- `pkg/btrfs/btrfs.go` - btrfs/mount/wipefs 命令
- `internal/vm/manager.go` - virsh/qemu-img 命令
- `internal/network/firewall.go` - iptables 命令
- `internal/network/interfaces.go` - ip/dhclient 命令
- `internal/security/firewall.go` - iptables 命令
- `internal/security/fail2ban.go` - fail2ban-client 命令
- `internal/security/v2/disk_encryption.go` - cryptsetup 命令
- `internal/files/manager.go` - tar/unzip/ffmpeg 命令
- 等多个文件...

**风险:** 外部输入传递给命令执行函数，存在命令注入风险

### 6. TLS证书跳过验证 (G402) - 高危
**影响文件:**
- `internal/auth/ldap.go` (行 151, 179)
- `internal/ldap/client.go` (行 71, 99)

**风险:** LDAP连接跳过TLS证书验证，可能导致中间人攻击

### 7. 弱加密算法 (G401) - 中危
**影响文件:**
- `internal/transfer/chunked.go`
- `internal/tiering/migrator.go`
- `internal/replication/conflict.go`
- `internal/cloudsync/sync_engine.go`
- `internal/backup/sync.go`

**风险:** 使用 MD5 进行哈希计算，存在碰撞攻击风险

### 8. 整数溢出 (G115) - 中危
**影响文件:**
- `internal/system/monitor.go`
- `internal/snapshot/adapter.go`
- `internal/optimizer/optimizer.go`
- `internal/search/engine.go`
- `internal/reports/datasource.go`
- `internal/ldap/client.go`
- `internal/vm/snapshot.go`
- `internal/vm/iso.go`
- 等多个文件...

**风险:** uint64/int64/int 类型转换可能导致整数溢出

### 9. 潜在硬编码凭证 (G101) - 低危
**影响文件:**
- `internal/office/types.go` - ErrInvalidToken
- `internal/cloudsync/providers.go` - OAuth TokenURL
- `internal/auth/oauth2.go` - OAuth2 配置

**风险:** 实际为OAuth端点URL和错误消息，非真实凭证，误报

## 修复建议

### 优先级1 - 立即修复
1. **路径遍历**: 对所有文件路径操作进行严格验证，使用 `filepath.Clean()` 和白名单校验
2. **命令注入**: 对所有命令参数进行严格转义，避免直接拼接用户输入

### 优先级2 - 尽快修复
1. **XSS/SMTP注入**: 对所有用户输入进行HTML实体编码和SMTP头部验证
2. **TLS跳过验证**: 移除 `InsecureSkipVerify: true`，配置正确的TLS证书验证

### 优先级3 - 计划修复
1. **弱加密算法**: 将 MD5 替换为 SHA-256 或更安全的哈希算法
2. **整数溢出**: 添加边界检查，使用安全的类型转换

### 优先级4 - 长期改进
1. **Context资源泄漏**: 确保所有 `context.WithCancel` 返回的 cancel 函数被调用
2. **子进程执行**: 使用 `exec.Command` 时对参数进行严格验证

## 附注
- 详细 SARIF 报告已生成: `gosec-report.sarif`
- 建议在 CI/CD 流程中集成 gosec 安全扫描
- 部分警告可能为误报（如 OAuth 端点 URL），需人工复核

---
*报告生成时间: 2026-03-15 07:58 CST*
*审计部门: 刑部（法务合规）*