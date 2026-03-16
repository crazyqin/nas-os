# nas-os 安全审计报告 v2.108.0

**审计日期**: 2026-03-16  
**审计者**: 刑部  
**项目**: nas-os  
**版本**: v2.108.0  

---

## 执行摘要

| 指标 | 数值 |
|------|------|
| 扫描文件数 | 443 |
| 代码行数 | 272,004 |
| **问题总数** | **1,666** |
| 高危 (HIGH) | 169 |
| 中危 (MEDIUM) | 796 |
| 低危 (LOW) | 701 |

### 风险评估

**整体风险等级**: 🔴 **高风险**

本次审计发现大量安全问题，主要集中在：
1. **命令注入风险** (G204/G702): 202处
2. **路径遍历风险** (G304/G703): 278处
3. **整数溢出风险** (G115): 91处
4. **权限控制不当** (G301/G306/G302): 345处

---

## 问题分类统计

| 规则ID | 严重性 | 数量 | 描述 |
|--------|--------|------|------|
| G104 | LOW | 701 | 错误返回值未检查 |
| G304 | MEDIUM | 230 | 文件路径包含变量输入 |
| G204 | MEDIUM | 192 | 子进程启动使用变量 |
| G301 | MEDIUM | 181 | mkdir 权限可能过宽 |
| G306 | MEDIUM | 154 | 文件写入权限过宽 |
| G115 | HIGH | 91 | 整数溢出转换 |
| G703 | HIGH | 48 | 路径遍历（污点分析） |
| G118 | HIGH | 10 | Context 取消函数未调用 |
| G702 | HIGH | 10 | 命令注入（污点分析） |
| G101 | HIGH | 7 | 潜在硬编码凭证 |
| G107 | MEDIUM | 7 | URL 重定向风险 |
| G122 | HIGH | 7 | TOCTOU 竞态条件 |
| G110 | MEDIUM | 4 | 潜在拒绝服务 |
| G402 | HIGH | 4 | TLS InsecureSkipVerify |
| G505 | MEDIUM | 3 | 弱哈希算法 (MD5/SHA1) |
| G117 | MEDIUM | 2 | 无效内存分配 |
| G705 | MEDIUM | 2 | XSS 风险 |
| G120 | MEDIUM | 1 | 子进程启动 |
| G305 | MEDIUM | 1 | 文件路径包含变量 |
| G707 | HIGH | 1 | SMTP 注入 |

---

## 高危问题详情

### 1. TLS 证书验证绕过 (G402) - 4处

**CWE-295**: LDAP 连接时设置 `InsecureSkipVerify: true`，跳过 TLS 证书验证。

**影响位置**:
- `internal/auth/ldap.go:154,184`
- `internal/ldap/client.go:74,104`

**风险**: 中间人攻击、凭证窃取、数据泄露

**建议修复**:
```go
// 仅在测试环境允许跳过验证
skipVerify := config.SkipTLSVerify && os.Getenv("NAS_ENV") == "test"
tlsConfig := &tls.Config{
    InsecureSkipVerify: skipVerify,
}
```

---

### 2. 命令注入风险 (G204/G702) - 202处

**CWE-78**: 大量使用 `exec.Command` 执行外部命令，变量作为参数。

**主要涉及命令**:
- `btrfs`, `mount`, `umount`, `wipefs`, `mkfs.btrfs`
- `docker`, `docker-compose`
- `smartctl`, `cryptsetup`
- `iptables`, `ip6tables`, `fail2ban-client`
- `ffmpeg`, `ffprobe`
- `virsh`, `qemu-img`

**高风险文件**:
- `pkg/btrfs/btrfs.go` - 14处
- `internal/vm/manager.go` - 8处
- `internal/docker/manager.go` - 15处
- `internal/security/v2/disk_encryption.go` - 12处
- `internal/network/firewall.go` - 12处

**建议修复**:
1. 对所有用户输入进行严格验证
2. 使用白名单限制可执行的命令和参数
3. 避免使用 shell 解释器 (`sh -c`)
4. 使用参数化传递而非字符串拼接

---

### 3. 路径遍历风险 (G304/G703) - 278处

**CWE-22**: 文件操作使用用户可控的路径变量。

**高风险文件**:
- `internal/webdav/server.go` - 48处
- `internal/backup/manager.go` - 多处
- `internal/vm/snapshot.go` - 多处

**建议修复**:
```go
func sanitizePath(basePath, userPath string) (string, error) {
    fullPath := filepath.Join(basePath, userPath)
    // 确保最终路径在基础目录内
    if !strings.HasPrefix(fullPath, basePath) {
        return "", errors.New("路径遍历攻击检测")
    }
    return fullPath, nil
}
```

---

### 4. 整数溢出风险 (G115) - 91处

**CWE-190**: uint64 到 int64/int 的转换可能导致溢出。

**影响位置**:
- `internal/system/monitor.go` - 网络流量统计
- `internal/storage/smart_monitor.go` - SMART 数据处理
- `internal/quota/optimizer/optimizer.go` - 配额计算
- `internal/reports/datasource.go` - 报告数据汇总

**建议修复**:
```go
// 使用安全的转换函数
func safeUint64ToInt64(v uint64) (int64, error) {
    if v > math.MaxInt64 {
        return 0, errors.New("整数溢出")
    }
    return int64(v), nil
}
```

---

### 5. 硬编码凭证风险 (G101) - 7处

**CWE-798**: 代码中可能存在硬编码的敏感信息。

**影响位置**:
- `internal/office/types.go:581` - ErrInvalidToken
- `internal/cloudsync/providers.go:670,1320` - OAuth Token URL
- `internal/auth/oauth2.go:334-389` - OAuth 配置函数

**评估**: 大部分为 OAuth 配置函数的 URL 常量，非实际凭证。建议人工复核确认。

---

### 6. TOCTOU 竞态条件 (G122) - 7处

**CWE-367**: filepath.Walk 回调中的文件操作存在竞态条件。

**影响文件**:
- `internal/snapshot/replication.go`
- `internal/plugin/hotreload.go`
- `internal/files/manager.go`
- `internal/backup/manager.go`
- `internal/backup/encrypt.go`
- `internal/backup/config_backup.go`
- `internal/backup/advanced/verification.go`

**建议**: 使用 `os.Root` API 或在打开文件时使用 `O_NOFOLLOW` 标志。

---

### 7. Context 资源泄漏 (G118) - 10处

**CWE-400**: context 取消函数未调用，可能导致资源泄漏。

**影响文件**:
- `internal/scheduler/executor.go` - 2处
- `internal/media/transcoder.go` - 1处
- `internal/media/streaming.go` - 3处
- `internal/compress/service.go` - 1处
- `internal/backup/manager.go` - 2处
- `internal/performance/monitor.go` - 1处

**建议修复**:
```go
// 立即调用 defer
ctx, cancel := context.WithTimeout(parentCtx, timeout)
defer cancel()
```

---

### 8. SMTP 注入风险 (G707) - 1处

**CWE-93**: `internal/automation/action/action.go:264`

**建议**: 对邮件内容进行严格转义，避免 CRLF 注入。

---

### 9. XSS 风险 (G705) - 2处

**影响文件**: `internal/automation/api/handlers.go:278,409`

Content-Disposition header 中使用了未转义的 ID。

---

## 中危问题摘要

| 问题类型 | 数量 | 描述 |
|----------|------|------|
| 目录权限过宽 (G301) | 181 | mkdir 使用 0755 或更宽权限 |
| 文件权限过宽 (G306) | 154 | 文件写入使用 0644 或更宽权限 |
| URL 重定向 (G107) | 7 | HTTP 重定向使用变量 URL |
| 潜在 DoS (G110) | 4 | 可能的无限循环或资源耗尽 |
| 弱哈希算法 (G505) | 3 | 使用 MD5/SHA1 |

---

## 修复优先级建议

### P0 - 立即修复
1. **TLS 证书验证绕过** - 修复 LDAP 连接安全问题
2. **命令注入高风险点** - 重点修复用户输入直接传递到命令的位置
3. **路径遍历高风险点** - WebDAV 和文件管理模块

### P1 - 短期修复
1. 整数溢出转换添加边界检查
2. Context 取消函数泄漏
3. TOCTOU 竞态条件

### P2 - 中期优化
1. 文件/目录权限收紧
2. 硬编码凭证人工复核
3. 错误处理完善

---

## 依赖安全建议

建议执行以下命令检查依赖漏洞：
```bash
cd ~/clawd/nas-os
govulncheck ./...
```

---

## 合规状态

| 检查项 | 状态 |
|--------|------|
| 许可证合规 | ✅ PASS |
| 安全合规 | ❌ NEEDS_ATTENTION |
| 数据保护 | ⚠️ NEEDS_REVIEW |

---

## 审计结论

**整体评估**: ⚠️ **条件通过**

项目安全状况存在较大风险，主要问题集中在：
1. 大量命令注入风险点（NAS 系统特性，需调用系统命令）
2. 路径遍历防护不足
3. 证书验证绕过问题

**建议**:
- 优先修复 P0 级别问题后再发布生产版本
- 建立安全编码规范，对新代码进行安全审查
- 集成安全扫描到 CI/CD 流程

---

**下次审计日期**: 2026-04-16

*报告生成时间: 2026-03-16T11:35:00+08:00*