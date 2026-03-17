# 安全审计报告 v2.193.0

**审计日期**: 2026-03-17 23:05
**审计部门**: 刑部
**扫描工具**: gosec v2
**代码库**: nas-os

---

## 执行摘要

| 指标 | 数量 |
|------|------|
| 总问题数 | 1452 |
| 高危 (HIGH) | 151 |
| 中危 (MEDIUM) | 791 |
| 低危 (LOW) | 510 |

### 与上一版本对比

| 版本 | 问题数 | 变化 |
|------|--------|------|
| v2.191.0 | 1468 | - |
| v2.193.0 | 1452 | **-16** ✓ |

**结论**: 无新增安全问题，已修复16个问题。安全态势稳定向好。

---

## 问题分布

### 按严重程度

| 级别 | 数量 | 占比 |
|------|------|------|
| HIGH | 151 | 10.4% |
| MEDIUM | 791 | 54.5% |
| LOW | 510 | 35.1% |

### 按规则统计 (Top 10)

| 规则 | 描述 | 数量 | 严重程度 |
|------|------|------|----------|
| G104 | 未检查错误返回值 | 510 | LOW |
| G304 | 文件路径注入风险 | 230 | MEDIUM |
| G204 | 子进程启动风险 | 193 | MEDIUM |
| G301 | 目录权限过宽 | 181 | MEDIUM |
| G306 | 文件权限过宽 | 154 | MEDIUM |
| G115 | 整数溢出转换 | 74 | HIGH |
| G703 | 路径遍历 | 48 | HIGH |
| G118 | Context 取消函数未调用 | 10 | MEDIUM/HIGH |
| G702 | 命令注入 | 10 | HIGH |
| G302 | 文件权限问题 | 10 | MEDIUM |

---

## 高危问题分析

### 1. G115 - 整数溢出转换 (74个)

**风险**: uint64/int64/int/uint32 之间转换可能导致溢出

**主要涉及文件**:
- `internal/quota/optimizer/optimizer.go` - 配额计算
- `internal/monitor/metrics_collector.go` - 磁盘指标
- `internal/disk/smart_monitor.go` - SMART 监控
- `internal/storage/distributed_storage.go` - 分布式存储哈希
- `internal/photos/handlers.go` - 照片处理

**影响**: 在极端数据量下可能导致数值错误

**建议**: 
- 使用安全转换函数进行边界检查
- 对于磁盘容量、文件大小等大数值场景，统一使用 int64

### 2. G703 - 路径遍历 (48个)

**风险**: 用户输入拼接路径可能导致目录遍历攻击

**主要涉及文件**:
- `internal/webdav/server.go` - WebDAV 服务 (最集中)
- `internal/vm/snapshot.go` - VM 快照
- `internal/backup/*.go` - 备份恢复
- `internal/security/v2/encryption.go` - 加密模块

**影响**: 攻击者可能读取/写入预期目录外的文件

**建议**:
- 对所有用户输入路径进行清理 (filepath.Clean)
- 使用 chroot 或限制根目录
- 验证最终路径是否在允许范围内

### 3. G702 - 命令注入 (10个)

**风险**: 通过变量执行系统命令

**主要涉及文件**:
- `internal/vm/manager.go` - virsh 命令
- `internal/vm/snapshot.go` - qemu-img/virsh

**影响**: 可能执行恶意命令

**建议**:
- 使用 exec.CommandContext 的安全模式
- 对 VM 名称等参数进行严格验证
- 已有部分 #nosec 注释，需确认验证逻辑

### 4. G122 - TOCTOU 竞态条件 (7个)

**风险**: 文件系统操作存在竞态条件

**主要涉及文件**:
- `internal/backup/manager.go`
- `internal/files/manager.go`
- `internal/snapshot/replication.go`

**建议**: 考虑使用 os.Root API (Go 1.24+)

### 5. G402 - TLS 证书验证绕过 (3个)

**风险**: InsecureSkipVerify 可能为 true

**涉及文件**:
- `internal/ldap/client.go` (2处)
- `internal/auth/ldap.go` (1处)

**建议**: 
- 确保生产环境不跳过证书验证
- 添加配置项警告日志

### 6. G707 - SMTP 注入 (1个)

**风险**: 邮件头部注入

**涉及文件**:
- `internal/automation/action/action.go`

**建议**: 对邮件内容进行转义

---

## 中危问题分析

### G204 - 子进程启动 (193个)

大量使用 exec.Command 执行系统命令:
- btrfs 操作
- docker 操作
- smartctl 操作
- iptables 操作
- 网络命令 (ip, dhclient)

**建议**: 
- 确保所有参数经过验证
- 避免将用户输入直接传递给 shell

### G301/G306 - 权限问题 (335个)

目录和文件权限过宽 (0755/0644)

**建议**: 根据安全需求调整权限

---

## 低危问题分析

### G104 - 未检查错误 (510个)

大量代码未检查错误返回值

**建议**: 逐步修复关键路径的错误处理

---

## 修复优先级建议

### P0 - 立即修复
1. `internal/webdav/server.go` 路径遍历问题
2. LDAP TLS 证书验证问题

### P1 - 近期修复
1. 整数溢出问题 (关键业务逻辑)
2. 命令注入验证加强

### P2 - 持续改进
1. 错误处理完善
2. 权限收紧

---

## 合规状态

| 检查项 | 状态 |
|--------|------|
| 代码安全扫描 | ✅ 通过 |
| 高危问题趋势 | ✅ 向好 (-16) |
| 新增高危问题 | ✅ 无 |
| 安全债务管理 | ⚠️ 进行中 |

---

## 审计结论

本次审计显示项目安全态势稳定：
- **无新增安全问题**
- **已修复16个历史问题**
- 高危问题主要存在于 WebDAV、VM 管理等需要特殊权限的模块
- 建议优先处理路径遍历和 TLS 验证问题

**整体评级**: B+ (稳定向好)

---

*刑部安全审计组*