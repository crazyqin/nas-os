# 安全审计报告 v2.148.0

**审计日期**: 2026-03-17  
**审计工具**: gosec v2  
**代码库**: nas-os  
**审计版本**: v2.148.0 (基于 v2.147.0)

---

## 执行摘要

| 指标 | 数值 |
|------|------|
| 总问题数 | 260+ |
| 高危 (HIGH) | 180+ |
| 中危 (MEDIUM) | 80+ |
| 扫描文件数 | 300+ |

---

## 高危问题分布

### 1. 整数溢出 (G115) - CWE-190

**数量**: 70+ 处  
**严重性**: HIGH  
**置信度**: MEDIUM

**问题描述**:  
整数类型转换时未检查溢出，可能导致意外行为。

**主要受影响文件**:
- `internal/quota/optimizer/optimizer.go` - uint64→int64 转换
- `internal/optimizer/optimizer.go` - uint64→int64 转换
- `internal/search/engine.go` - uint64→int 转换
- `internal/monitor/metrics_collector.go` - uint64→int 转换
- `internal/disk/smart_monitor.go` - uint64→int 转换
- `internal/storage/distributed_storage.go` - rune→uint32 转换
- `internal/vm/*.go` - int64→uint64 转换
- `internal/quota/cleanup.go` - int64→uint64 转换
- `internal/photos/*.go` - int64→uint64 转换

**修复建议**:
```go
// 使用安全转换函数
func safeUint64ToInt64(v uint64) (int64, error) {
    if v > math.MaxInt64 {
        return 0, errors.New("overflow")
    }
    return int64(v), nil
}
```

---

### 2. 路径遍历 (G703)

**数量**: 50+ 处  
**严重性**: HIGH  
**置信度**: HIGH

**问题描述**:  
用户输入可能被用于构造文件路径，存在路径遍历攻击风险。

**主要受影响文件**:
- `internal/webdav/server.go` - 大量文件操作使用外部路径
- `internal/vm/snapshot.go` - 快照路径操作
- `internal/vm/manager.go` - VM 目录操作
- `internal/backup/*.go` - 备份路径操作
- `internal/security/v2/encryption.go` - 加密文件路径
- `plugins/filemanager-enhance/main.go` - 文件写入操作

**修复建议**:
```go
import "github.com/mikewhite/mlib/safepath"

// 使用安全路径库
safePath, err := safepath.Sanitize(userInput)
if err != nil {
    return err
}
fullPath := filepath.Join(baseDir, safePath)
```

---

### 3. 命令注入 (G702, G204) - CWE-78

**数量**: 80+ 处  
**严重性**: HIGH/MEDIUM  
**置信度**: HIGH

**问题描述**:  
使用外部输入构造系统命令，存在命令注入风险。

**主要受影响文件**:
- `internal/vm/manager.go` - virsh 命令执行
- `internal/vm/snapshot.go` - qemu-img, virsh 命令
- `pkg/btrfs/btrfs.go` - btrfs, mount, umount 命令
- `internal/security/v2/disk_encryption.go` - cryptsetup 命令
- `internal/security/firewall.go` - iptables 命令
- `internal/security/fail2ban.go` - fail2ban-client, iptables 命令
- `internal/network/*.go` - ip, iptables, ping, traceroute 命令
- `internal/container/*.go` - docker 命令
- `internal/backup/manager.go` - tar, openssl, scp 命令
- `internal/files/manager.go` - ffmpeg, unzip, tar 命令

**修复建议**:
```go
// 1. 使用参数化命令
cmd := exec.Command("virsh", "start", validatedVMName)

// 2. 输入验证
func validateVMName(name string) error {
    matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, name)
    if !matched {
        return errors.New("invalid VM name")
    }
    return nil
}

// 3. 避免使用 shell -c
// 危险: exec.Command("sh", "-c", script)
// 安全: exec.Command("cmd", args...)
```

---

### 4. TLS 不安全跳过验证 (G402) - CWE-295

**数量**: 3 处  
**严重性**: HIGH  
**置信度**: LOW

**受影响文件**:
- `internal/ldap/client.go:73` - InsecureSkipVerify
- `internal/ldap/client.go:103` - InsecureSkipVerify
- `internal/auth/ldap.go:153` - InsecureSkipVerify

**问题描述**:  
LDAP 连接配置中 `InsecureSkipVerify` 可能被设置为 true，跳过 TLS 证书验证。

**修复建议**:
```go
// 仅在测试环境允许跳过验证
if skipVerify && !isTestEnvironment() {
    return errors.New("TLS verification cannot be skipped in production")
}
tlsConfig := &tls.Config{
    InsecureSkipVerify: skipVerify,
}
```

---

### 5. 弱随机数生成器 (G404) - CWE-338

**数量**: 2 处  
**严重性**: HIGH  
**置信度**: MEDIUM

**受影响文件**:
- `internal/reports/cost_report.go:1220`
- `internal/budget/alert.go:936`

**问题描述**:  
使用 `math/rand` 生成随机字符串，不适合安全敏感场景。

**修复建议**:
```go
import "crypto/rand"

func generateSecureRandomString(n int) (string, error) {
    const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    b := make([]byte, n)
    if _, err := rand.Read(b); err != nil {
        return "", err
    }
    for i := range b {
        b[i] = letters[int(b[i])%len(letters)]
    }
    return string(b), nil
}
```

---

### 6. SMTP 注入 (G707) - CWE-93

**数量**: 1 处  
**严重性**: HIGH  
**置信度**: HIGH

**受影响文件**:
- `internal/automation/action/action.go:264`

**问题描述**:  
SMTP 邮件发送时可能存在头部注入风险。

**修复建议**:
```go
import "net/textproto"

// 使用 textproto 进行头部编码
from = textproto.QuoteString(from)
to = textproto.QuoteString(to)
```

---

### 7. 潜在硬编码凭据 (G101) - CWE-798

**数量**: 7 处  
**严重性**: HIGH  
**置信度**: LOW

**受影响文件**:
- `internal/auth/oauth2.go` - OAuth2 配置函数
- `internal/cloudsync/providers.go` - Token URL
- `internal/office/types.go` - 常量命名

**问题描述**:  
部分代码被误报为硬编码凭据，主要是 OAuth2 Provider 配置函数。

**修复建议**:
- 使用 `#nosec G101` 注释抑制误报
- 确保实际凭据从环境变量或配置文件读取

---

### 8. XSS (G705)

**数量**: 2 处  
**严重性**: MEDIUM  
**置信度**: HIGH

**受影响文件**:
- `internal/automation/api/handlers.go:409`
- `internal/automation/api/handlers.go:278`

**问题描述**:  
HTTP 响应中可能存在 XSS 风险。

**修复建议**:
```go
// 对 Content-Disposition 文件名进行编码
import "mime"

encodedFilename := mime.QEncoding.Encode("utf-8", sanitizedFilename)
w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", encodedFilename))
```

---

## 中危问题分布

### 9. Context 泄漏 (G118) - CWE-400

**数量**: 10+ 处  
**严重性**: HIGH/MEDIUM  
**置信度: HIGH/MEDIUM

**受影响文件**:
- `internal/scheduler/executor.go` - context.WithCancel/WithTimeout 未调用 cancel
- `internal/media/streaming.go` - context.WithCancel 未调用 cancel
- `internal/media/transcoder.go` - context.WithCancel 未调用 cancel
- `internal/compress/service.go` - context.WithCancel 未调用 cancel
- `internal/backup/manager.go` - context.WithTimeout 存储到 map
- `internal/performance/monitor.go` - Goroutine 使用 context.Background

**修复建议**:
```go
// 确保 cancel 函数被调用
ctx, cancel := context.WithTimeout(parentCtx, timeout)
defer cancel()

// 存储到 map 的情况需要确保清理
m.cancels[task.ID] = cancel
// 在任务完成或取消时调用
delete(m.cancels, task.ID)
cancel()
```

---

### 10. TOCTOU 竞争条件 (G122) - CWE-367

**数量**: 7 处  
**严重性**: HIGH  
**置信度**: MEDIUM

**受影响文件**:
- `internal/snapshot/replication.go`
- `internal/plugin/hotreload.go`
- `internal/files/manager.go`
- `internal/backup/manager.go`
- `internal/backup/encrypt.go`
- `internal/backup/config_backup.go`
- `internal/backup/advanced/verification.go`

**问题描述**:  
`filepath.Walk/WalkDir` 回调中的文件操作存在 Time-of-Check to Time-of-Use 竞争条件。

**修复建议**:
```go
// 使用 os.Root (Go 1.24+) 或安全操作模式
import "os"

// 方案1: 使用 O_NOFOLLOW 打开文件
f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)

// 方案2: 检查并原子操作
if err := os.Rename(src, dst); err != nil {
    // 处理错误
}
```

---

## 优先修复建议

### 第一优先级 (立即修复)

1. **命令注入 (G702/G204)** - 80+ 处
   - 添加严格的输入验证
   - 使用参数化命令而非 shell

2. **路径遍历 (G703)** - 50+ 处
   - 实现统一的路径安全验证函数
   - 限制文件操作范围

3. **SMTP 注入 (G707)** - 1 处
   - 对邮件内容进行严格过滤

### 第二优先级 (近期修复)

4. **TLS 不安全跳过验证 (G402)** - 3 处
   - 生产环境禁止跳过验证

5. **弱随机数生成器 (G404)** - 2 处
   - 替换为 crypto/rand

6. **Context 泄漏 (G118)** - 10+ 处
   - 确保 cancel 函数被正确调用

### 第三优先级 (逐步改进)

7. **整数溢出 (G115)** - 70+ 处
   - 添加边界检查
   - 使用安全转换函数

8. **TOCTOU 竞争条件 (G122)** - 7 处
   - 评估实际风险并逐步修复

---

## 误报分析

以下问题属于误报或已处理:

1. **OAuth2 配置函数 (G101)** - 设计模式，非实际凭据泄露
2. **virsh 命令 (G702)** - 已有 #nosec 注释，VM 名称已验证
3. **部分路径操作 (G703)** - 路径由内部生成，非用户输入

---

## 建议的后续行动

1. **建立安全编码规范**
   - 制定输入验证标准
   - 禁止危险函数的直接使用

2. **引入静态分析 CI**
   - 将 gosec 集成到 CI/CD 流程
   - 对新代码强制安全检查

3. **安全培训**
   - 针对 OWASP Top 10 进行开发培训
   - 重点讲解命令注入和路径遍历防护

4. **代码审计**
   - 对高危模块进行人工代码审计
   - 特别关注 WebDAV 和 VM 管理模块

---

**审计完成**  
刑部 v2.147.0