# 安全审计报告 - 第227轮

**版本**: v2.455.0
**日期**: 2026-04-15
**审计人员**: 刑部

## 1. 审计范围

- **模块**: `internal/smb/stateful/`
- **文件**: `manager.go`, `registry.go`, `manager_test.go`
- **本次目标**: Phase3 代码审计（loadbalancer.go, failover.go 尚未创建）
- **前序报告**: Round223

## 2. 静态分析结果

### 2.1 go vet ✅

```
cd /home/mrafter/clawd/nas-os && go vet ./internal/smb/stateful/...
```
**结果**: 无警告，代码通过标准 vet 检查。

### 2.2 govulncheck ✅

```
govulncheck ./internal/smb/stateful/...
```
**结果**: 直接调用 0 个漏洞。模块依赖发现 7 个间接漏洞（crypto/x509、html/template 等，Round223 已记录），本次代码未引入新的直接调用路径。

### 2.3 测试覆盖

| 测试用例 | 覆盖 | 结果 |
|---------|------|------|
| TestNewStatefulFailoverManager | ✅ | PASS |
| TestRegisterAndUnregisterSession | ✅ | PASS |
| TestFindBestTarget | ✅ | PASS |
| TestGetStatus | ✅ | PASS |
| TestSessionStateRegistry | ✅ | PASS |
| TestCleanupExpiredSessions | ✅ | PASS |
| TestValidateSessionState | ✅ | PASS |
| TestManagerStartStop | ✅ | PASS |
| **整体覆盖率** | **59.7%** | **8/8 PASS** |

**未覆盖路径**（代码审计发现）:
- `TriggerFailover` 全部失败分支
- `migrateSession` 中的 `checkClientReachable` 失败路径
- `checkPeerHealth` 中的 `NodeDown` 事件触发
- `loadPersistedState` 快照损坏时的错误路径
- `eventProcessor` 文件写入失败路径

## 3. 代码安全审计

### 3.1 Phase3 现状

`loadbalancer.go` 和 `failover.go` **尚未创建**。Phase3 代码审计待后续轮次执行。

### 3.2 已发现安全问题

#### P2 - 快照文件无限增长（磁盘耗尽风险）

**文件**: `manager.go:takeSnapshot()`
**问题**: 每 `SnapshotInterval`（默认5秒）写入一个快照文件，无清理机制。长期运行磁盘必然填满。

**证据**:
```go
// manager.go:takeSnapshot()
snapshotFile := filepath.Join(m.snapshotDir, fmt.Sprintf("snapshot-%d.json", time.Now().UnixNano()))
return os.WriteFile(snapshotFile, data, 0640)
// ↑ 无任何清理逻辑
```

**建议**: 添加快照保留策略（最多保留 N 个或保留最近 X 小时），在 `takeSnapshot` 前清理过期文件。

#### P2 - 事件日志无限增长

**文件**: `manager.go:eventProcessor()`
**问题**: 所有 FailoverEvent 都追加写入 `events.log`，无轮转、无大小限制。大量故障转移场景下日志快速增长。

**建议**: 集成日志轮转（`logrotate`），或改为结构化日志输出到 stdout，由系统日志管理。

#### P3 - 客户端可达性检查暴露节点 IP

**文件**: `manager.go:checkClientReachable()`
**问题**: 故障转移时主动 TCP 连接到客户端 IP 的 445 端口，将节点 IP 暴露给客户端，且可能被恶意客户端利用。

```go
conn, err := d.DialContext(ctx, "tcp", clientIP+":445")
```

**建议**: 改为单向检测（ICMP ping 或 ARP 表检查），或仅在管理员配置的白名单 IP 范围内执行。

#### P3 - JSON 反序列化无大小限制

**文件**: `manager.go:loadPersistedState()`
**问题**: 读取快照文件后直接 `json.Unmarshal`，无数据大小验证。恶意构造的大文件可导致内存耗尽。

**建议**: 读取前检查 `info.Size()`，设置最大文件大小阈值（如 100MB）。

#### P3 - 测试覆盖率不足

**覆盖率**: 59.7%，关键路径缺失：
- 故障转移全部失败路径（`failed > 0 && recovered == 0`）
- 客户端不可达时的会话清理路径
- 对等节点心跳超时触发故障转移路径
- 损坏快照的恢复失败路径

**建议**: 补充边界条件测试，目标覆盖率提升至 80%。

### 3.3 安全设计评价

**做得好的方面**:
- ✅ SessionState 验证函数 `ValidateSessionState` 完整（SessionID/ClientIP/NodeID 非空检查）
- ✅ 快照文件权限 `0640`，防止其他用户读取会话状态
- ✅ 使用 `sync.RWMutex` 保护共享状态，无明显竞态条件
- ✅ `RecoveryConcurrency` 并发限制防止过载
- ✅ JSON 序列化外部数据无 `eval` 或动态代码执行
- ✅ 事件日志与状态分离存储

**潜在风险点**:
- ⚠️ 会话状态中 `Username`、`OpenedFiles`、`.Metadata` 字段可能包含敏感信息，写入快照文件需确认文件系统加密
- ⚠️ `StateSyncMessage` 中的会话数据通过 `_ = data` 模拟传输，Phase3 实现时需确保传输加密（TLS）和认证

## 4. 已知漏洞复查（来自 Round223）

| 漏洞ID | 严重性 | 组件 | 状态 |
|--------|--------|------|------|
| GO-2026-4947 | 高 | crypto/x509 | 未修复（Go 1.26.1） |
| GO-2026-4946 | 高 | crypto/x509 | 未修复（Go 1.26.1） |
| GO-2026-4870 | 高 | crypto/tls | 未修复（Go 1.26.1） |
| GO-2026-4869 | 中 | archive/tar | 未修复（Go 1.26.1） |
| GO-2026-4866 | 高 | crypto/x509 | 未修复（Go 1.26.1） |
| GO-2026-4865 | 中 | html/template | 未修复（Go 1.26.1） |

**结论**: 6 个已知漏洞无变化，建议尽快升级 Go 至 1.26.2。

## 5. 修复建议

### P2（建议本轮修复）

1. **快照轮转**：`takeSnapshot` 前清理超过 24 小时或超过 100 个的旧快照文件
2. **JSON 大小限制**：`loadPersistedState` 读取前检查文件大小上限

### P3（下轮计划）

1. 客户端可达性检查改为非 TCP 连接方式
2. 事件日志接入日志轮转系统
3. 补充边界条件测试用例

### 无 P0/P1 问题

本次审计未发现高危或紧急安全问题。

## 6. 总结

| 指标 | 值 |
|------|-----|
| Go版本 | 1.26.1（需升级至1.26.2） |
| 标准库漏洞 | 6个（来自依赖，非直接调用） |
| 代码 vet | ✅ 通过 |
| govulncheck | ✅ 直接调用0漏洞 |
| 测试通过率 | 8/8 (100%) |
| 测试覆盖率 | 59.7% |
| 高危问题 | 0 |
| 中危问题 | 0 |
| 低危问题 | 4 |
| Phase3 代码 | 未创建，待后续审计 |

**整体评估**: 低风险。代码质量良好，无高危问题。Phase3 实现时需关注传输加密和传输层认证。

---
*审计完成时间: 2026-04-15 03:40 CST*
