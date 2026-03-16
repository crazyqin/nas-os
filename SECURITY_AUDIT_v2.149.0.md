# NAS-OS 安全审计报告 v2.149.0

**审计日期**: 2026-03-17  
**审计工具**: gosec v2  
**代码库**: nas-os  
**版本**: v2.149.0

---

## 📊 执行摘要

| 严重程度 | 数量 | 占比 |
|---------|------|------|
| **高危 (HIGH)** | 153 | 9.2% |
| **中危 (MEDIUM)** | 791 | 47.7% |
| **低危 (LOW)** | 701 | 42.3% |
| **总计** | **1645** | 100% |

### 整体评估
⚠️ **风险等级: 高** - 发现大量安全问题，需要优先处理高危和中危问题。

---

## 🔴 高危问题 (153个)

### 1. G115 - 整数溢出转换 (74个)
**CWE-190**: 整数溢出或回绕

**影响**: 在类型转换时可能发生溢出，导致数据损坏或安全边界被绕过。

**主要位置**:
- `internal/quota/optimizer/optimizer.go` - uint64 → int64 转换
- `internal/monitor/metrics_collector.go` - uint64 → int 转换
- `internal/storage/distributed_storage.go` - rune → uint32 转换
- `internal/vm/snapshot.go`, `internal/vm/iso.go` - int64 → uint64 转换
- `internal/photos/handlers.go` - int64 → uint64 转换

**建议**: 使用安全的类型转换函数，添加边界检查。

### 2. G703 - 路径遍历漏洞 (48个)
**CWE-22**: 路径遍历

**影响**: 攻击者可能通过构造恶意路径访问或修改系统文件。

**主要位置**:
- `internal/webdav/server.go` - 大量文件操作未经充分验证
- `internal/vm/snapshot.go` - 快照路径操作
- `internal/backup/*.go` - 备份文件路径操作
- `internal/security/v2/encryption.go` - 加密文件路径

**建议**: 使用 `filepath.Clean()` 和 `filepath.Rel()` 验证路径，确保不超出预期目录。

### 3. G702 - 命令注入 (10个)
**CWE-78**: OS命令注入

**影响**: 攻击者可能通过构造恶意输入执行任意系统命令。

**主要位置**:
- `internal/vm/manager.go` - virsh 命令执行
- `internal/vm/snapshot.go` - virsh/qemu-img 命令

**建议**: 严格验证所有外部输入，使用参数化命令而非字符串拼接。

### 4. G118 - Context泄漏 (10个)
**CWE-400**: 资源耗尽

**影响**: Context取消函数未调用可能导致goroutine泄漏。

**主要位置**:
- `internal/scheduler/executor.go`
- `internal/media/streaming.go`
- `internal/backup/manager.go`

**建议**: 确保所有 `context.WithCancel/WithTimeout` 返回的cancel函数都被调用。

### 5. G122 - TOCTOU竞态条件 (7个)
**CWE-367**: Time-of-check Time-of-use 竞态条件

**影响**: 在filepath.Walk回调中使用竞态路径可能导致安全问题。

**主要位置**:
- `internal/snapshot/replication.go`
- `internal/backup/manager.go`
- `internal/files/manager.go`

**建议**: 使用 `os.Root` API或原子性文件操作。

### 6. G101 - 硬编码凭证 (7个)
**CWE-798**: 使用硬编码凭证

**影响**: 潜在的敏感信息泄露风险。

**主要位置**:
- `internal/auth/oauth2.go` - OAuth2配置（URL，非凭证）
- `internal/cloudsync/providers.go` - Token URL（非凭证）

**评估**: 大部分是OAuth2 URL配置，风险较低。但需审查 `internal/office/types.go` 中的 `ErrInvalidToken`。

### 7. G404 - 弱随机数生成器 (2个)
**CWE-338**: 使用弱随机数生成器

**主要位置**:
- `internal/reports/cost_report.go:1220`
- `internal/budget/alert.go:936`

**建议**: 使用 `crypto/rand` 替代 `math/rand` 生成安全敏感的随机数。

### 8. G402 - TLS跳过验证 (3个)
**CWE-295**: 证书验证不当

**主要位置**:
- `internal/ldap/client.go`
- `internal/auth/ldap.go`

**建议**: 确保生产环境禁用 `InsecureSkipVerify`，或使用自定义证书验证。

### 9. G707 - SMTP注入 (1个)
**CWE-93**: SMTP命令/头部注入

**主要位置**:
- `internal/automation/action/action.go:264`

**建议**: 验证和转义邮件地址和内容中的特殊字符。

---

## 🟡 中危问题 (983个)

### 1. G304 - 文件路径注入 (230个)
**CWE-22**: 路径遍历

**影响**: 使用用户输入构建文件路径可能导致路径遍历。

**建议**: 验证和清理所有文件路径输入。

### 2. G204 - 子进程启动 (192个)
**CWE-78**: 命令注入

**影响**: 使用变量参数启动子进程可能导致命令注入。

**主要涉及**:
- Docker命令 (`docker`, `docker-compose`)
- 系统管理命令 (`mount`, `umount`, `btrfs`)
- 安全工具 (`iptables`, `cryptsetup`, `fail2ban-client`)
- 媒体工具 (`ffmpeg`, `smartctl`)

**建议**: 严格验证所有输入参数，使用白名单策略。

### 3. G301 - 目录权限过宽 (181个)
**CWE-276**: 权限设置不当

**影响**: 创建目录时使用0755或更宽松权限可能导致未授权访问。

**建议**: 根据实际需求设置最小权限，如0750或0700。

### 4. G306 - 文件权限过宽 (154个)
**CWE-276**: 权限设置不当

**影响**: 写入文件时使用0644或更宽松权限。

**建议**: 敏感文件使用0600或0640权限。

### 5. G302 - 文件权限过宽 (10个)
**CWE-276**: 权限设置不当

**建议**: 审查文件创建时的权限设置。

### 6. G110 - 潜在的DoS (3个)
**CWE-400**: 资源耗尽

**建议**: 添加资源限制和超时控制。

### 7. G107 - URL重定向 (6个)
**CWE-601**: URL重定向

**建议**: 验证重定向目标URL。

---

## 🔵 低危问题 (701个)

### G104 - 未检查错误返回值 (701个)

**影响**: 错误未处理可能导致程序状态不一致。

**建议**: 系统性审查并处理所有错误返回值。

---

## 🛠️ 修复建议优先级

### P0 - 立即修复 (安全关键)
1. **G702/G703** - 命令注入和路径遍历（WebDAV、VM管理）
2. **G404** - 弱随机数生成器（改用crypto/rand）
3. **G402** - TLS跳过验证（生产环境必须禁用）

### P1 - 短期修复 (1-2周)
1. **G115** - 整数溢出转换
2. **G118** - Context泄漏
3. **G122** - TOCTOU竞态条件
4. **G707** - SMTP注入

### P2 - 中期修复 (1个月)
1. **G204** - 子进程命令验证
2. **G304** - 文件路径验证
3. **G301/G306** - 文件/目录权限审查

### P3 - 长期改进
1. **G104** - 错误处理（持续改进）
2. **G101** - 审查潜在硬编码凭证

---

## 📝 已修复问题

### 本次修复

1. **G118 - Context泄漏** (`internal/scheduler/executor.go:108-114`)
   - 问题: `context.WithCancel` 返回的 cancel 函数在创建 `context.WithTimeout` 时被覆盖丢失
   - 修复: 使用组合 cancel 函数，确保两个 cancel 都能被调用
   - 提交: 已修复，代码编译通过

### 说明

部分 G118 问题经审查为误报：
- `internal/media/streaming.go` - cancel 保存在 session 结构中，在清理时调用
- `internal/backup/manager.go` - cancel 保存在 cancels map 中，在 CancelTask 方法中调用
- `internal/compress/service.go` - cancel 保存在 task.Cancel 中，供外部调用

G404 弱随机数问题已有 `#nosec` 注释，设计为先使用 crypto/rand，仅在失败时回退到 math/rand。

---

## 📋 附录

### 扫描命令
```bash
gosec -fmt json -out gosec-report-v2.149.0.json ./...
```

### 详细报告
- JSON格式报告: `gosec-report-v2.149.0.json`

---

**审计员**: 刑部 (安全审计Agent)  
**报告生成时间**: 2026-03-17 06:25 CST