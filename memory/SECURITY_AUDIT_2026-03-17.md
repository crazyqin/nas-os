# 刑部安全审计报告

**项目**: nas-os  
**审计时间**: 2026-03-17 01:46  
**审计官**: 刑部（安全合规）

---

## 📊 扫描统计

| 指标 | 数值 |
|------|------|
| 扫描文件 | 452 |
| 代码行数 | 277,816 |
| 发现问题 | **163** |
| 已忽略 (nosec) | 30 |

---

## 🚨 风险等级分布

| 级别 | 数量 | 说明 |
|------|------|------|
| **HIGH** | 163 | 全部为高危问题 |

---

## 🔍 问题分类汇总

### 1. G115 - 整数溢出 (约 100+ 处) ⚠️

**CWE-190**: 整数溢出或回绕

**严重程度**: HIGH  
**置信度**: MEDIUM

**问题描述**:  
uint64/int64/int/uint32/rune/byte 之间的类型转换可能导致整数溢出。

**主要涉及文件**:
- `internal/quota/optimizer/optimizer.go` - quota 计算
- `internal/quota/api.go` - 配额限制计算
- `internal/storage/smart_monitor.go` - SMART 数据处理
- `internal/reports/datasource.go` - 报表汇总
- `internal/monitor/metrics_collector.go` - 指标采集
- `internal/disk/smart_monitor.go` - 磁盘健康监控
- `internal/performance/collector.go` - 性能采集
- `internal/health/health.go` - 健康检查

**风险评估**:  
- 对于存储系统，大文件/大容量场景下 uint64 转 int 可能溢出
- SMART 属性值通常较小，风险较低
- 配额计算涉及用户输入，需关注边界情况

**建议**:
1. 使用 `strconv.ParseUint` 等安全转换函数
2. 添加边界检查：`if val > math.MaxInt64 { return error }`
3. 对于关键计算路径，添加单元测试验证边界情况

---

### 2. G703 - 路径遍历 (约 50+ 处) 🔴

**CWE**: 路径遍历漏洞

**严重程度**: HIGH  
**置信度**: HIGH

**问题描述**:  
用户可控的路径参数可能被利用进行路径遍历攻击。

**重点文件**:
- `internal/webdav/server.go` - **50+ 处问题集中地**
- `internal/vm/snapshot.go` - 快照管理
- `internal/vm/manager.go` - VM 管理
- `internal/backup/*.go` - 备份恢复
- `internal/security/v2/encryption.go` - 加密文件操作
- `plugins/filemanager-enhance/main.go` - 文件管理插件

**风险评估**:  
- WebDAV 服务器直接处理用户路径，攻击面极大
- 备份恢复功能可能被利用读取/写入任意文件
- VM 快照路径拼接存在风险

**建议**:
1. **WebDAV 服务器重构** - 使用 `filepath.Rel` 和 `filepath.Clean` 净化路径
2. 添加路径前缀检查：`strings.HasPrefix(cleanedPath, allowedRoot)`
3. 禁止路径中的 `..` 和绝对路径
4. 考虑使用 `os.Root` (Go 1.24+) 进行沙箱化文件操作
5. 为所有文件操作添加审计日志

---

### 3. G702 - 命令注入 (10 处) 🔴

**CWE**: 命令注入

**严重程度**: HIGH  
**置信度**: HIGH

**涉及文件**:
- `internal/vm/manager.go` - virsh 命令执行
- `internal/vm/snapshot.go` - qemu-img 命令执行

**问题代码示例**:
```go
cmd := exec.CommandContext(ctx, "virsh", "-c", "qemu:///system", "start", vm.Name)
```

**风险评估**:  
- vm.Name 虽有验证注释，但需确认验证完整性
- 攻击者可能通过构造特殊 VM 名称注入命令

**建议**:
1. 强化 `validateConfig()` 验证，只允许 `[a-zA-Z0-9-_]` 字符
2. 考虑使用 libvirt Go 绑定替代命令行调用
3. 所有外部输入必须经过白名单验证

---

### 4. G404 - 弱随机数生成器 (2 处) ⚠️

**CWE-338**: 使用弱随机数生成器

**涉及文件**:
- `internal/reports/cost_report.go:1220`
- `internal/budget/alert.go:935`

**问题描述**:  
使用 `math/rand` 而非 `crypto/rand` 生成随机字符串。

**风险评估**:  
- 如果用于生成安全令牌/ID，存在可预测风险
- 报表场景风险较低，但应统一使用安全随机

**建议**:
```go
// 替换为
import "crypto/rand"
func randomString(n int) string {
    b := make([]byte, n)
    rand.Read(b)
    return base64.RawURLEncoding.EncodeToString(b)[:n]
}
```

---

### 5. G402 - TLS 不安全配置 (3 处) ⚠️

**CWE-295**: 证书验证不当

**涉及文件**:
- `internal/ldap/client.go` (2 处)
- `internal/auth/ldap.go` (1 处)

**问题描述**:  
`InsecureSkipVerify: skipVerify` 可能跳过 TLS 证书验证。

**风险评估**:  
- LDAP 认证场景，中间人攻击可窃取凭证
- 生产环境必须启用证书验证

**建议**:
1. 生产环境强制 `skipVerify = false`
2. 支持自定义 CA 证书配置
3. 添加安全警告日志

---

### 6. G101 - 硬编码凭证嫌疑 (6 处) ⚠️

**CWE-798**: 硬编码凭证

**涉及文件**:
- `internal/auth/oauth2.go` - OAuth2 配置函数
- `internal/cloudsync/providers.go` - 云同步 provider URLs
- `internal/office/types.go` - 错误常量

**问题描述**:  
OAuth2 配置函数中的 URL 字符串被误报为凭证。`ErrInvalidToken` 常量被误报。

**风险评估**:  
- 经人工审查，这些是配置模板，非硬编码凭证
- ClientID/ClientSecret 通过参数传入，符合安全要求

**结论**: **误报** - 可添加 `#nosec G101` 注释

---

### 7. G707 - SMTP 注入 (1 处) ⚠️

**CWE-93**: SMTP 命令/头注入

**涉及文件**:
- `internal/automation/action/action.go:264`

**问题描述**:  
邮件发送功能可能存在注入风险。

**建议**:
1. 验证邮件地址格式
2. 过滤邮件主题/内容中的换行符
3. 使用成熟的邮件库（如 `github.com/go-gomail/gomail`）

---

### 8. G122 - TOCTOU 竞态条件 (7 处) ⚠️

**CWE-367**: Time-of-check Time-of-use 竞态

**涉及文件**:
- `internal/backup/encrypt.go`
- `internal/backup/manager.go`
- `internal/backup/config_backup.go`
- `internal/backup/advanced/verification.go`
- `internal/files/manager.go`
- `internal/plugin/hotreload.go`
- `internal/snapshot/replication.go`

**问题描述**:  
`filepath.Walk/WalkDir` 回调中的文件操作存在竞态窗口。

**建议**:
1. 使用 `os.Root` (Go 1.24+) 防止 symlink TOCTOU
2. 对关键操作使用文件锁
3. 检查符号链接：`if info.Mode()&os.ModeSymlink != 0`

---

### 9. G118 - Context 使用不当 (1 处)

**涉及文件**:
- `internal/performance/monitor.go:759`

**问题描述**:  
Goroutine 中使用 `context.Background/TODO` 而非请求作用域 context。

**建议**: 传递请求 context 到 goroutine。

---

## 🎯 优先修复建议

### P0 - 立即修复
1. **WebDAV 路径遍历** - 50+ 处漏洞，攻击面极大
2. **命令注入** - VM 管理模块，验证输入白名单

### P1 - 本周修复
1. **TLS 证书验证** - LDAP 认证安全
2. **SMTP 注入** - 邮件功能安全

### P2 - 迭代修复
1. **整数溢出** - 逐步添加边界检查
2. **弱随机数** - 统一使用 crypto/rand
3. **TOCTOU** - 备份/文件操作模块

---

## 📝 已知忽略项 (nosec: 30)

项目中已有 30 处使用 `#nosec` 注释抑制的告警，需人工审查合理性。

---

## 🔒 合规建议

1. **定期扫描**: 每次发布前运行 `gosec ./...`
2. **安全编码培训**: 针对 OWASP Top 10
3. **代码审查**: 安全相关代码需要双人审查
4. **渗透测试**: WebDAV/VM 模块建议专业渗透测试

---

**审计官**: 刑部  
**报告状态**: 完成  
**下一步**: 提交兵部进行漏洞修复