# 安全审计报告 - 第227轮

**版本**: v2.455.0
**日期**: 2026-04-15
**审计人员**: 刑部
**工作目录**: `/home/mrafter/clawd/nas-os`

## 1. 审计范围

| 模块 | 文件 | 备注 |
|------|------|------|
| internal/smb/stateful | manager.go, registry.go, manager_test.go | Phase2 核心实现 |

**Phase3 提示**: `loadbalancer.go` 和 `failover.go` 尚未创建，审计范围仅限现有代码。

## 2. 静态分析

### 2.1 go vet
```
cd internal/smb/stateful/... && go vet
```
**结果**: ✅ 无问题

### 2.2 govulncheck
```
govulncheck ./internal/smb/stateful/...
```
**结果**: ✅ 代码调用的符号中无已知漏洞

```
No vulnerabilities found.
Your code is affected by 0 vulnerabilities.
This scan also found 1 vulnerability in packages you import
and 6 vulnerabilities in modules you require, but your code
doesn't appear to call these vulnerabilities.
```

> 注: 6个标准库漏洞（GO-2026-4947/4946/4870/4869/4866/4865）存在于项目依赖链中，
> 但 smb/stateful 模块未调用受影响代码路径。当前 Go 版本仍为 go1.26.1，
> 建议升级至 go1.26.2（见 Round223 报告 P0）。

## 3. 代码安全审计

### 3.1 manager.go 审计结果

| 检查项 | 状态 | 说明 |
|--------|------|------|
| 空指针解引用 | ✅ | NewStatefulFailoverManager 有 nil 检查 |
| 竞态条件 | ✅ | 使用 sync.RWMutex 保护共享状态 |
| 资源泄漏 | ✅ | defer ticker.Stop(), conn.Close() |
| 输入验证 | ✅ | ValidateSessionState 检查 SessionID/ClientIP/NodeID |
| 权限提升 | ✅ | 无提权路径 |
| 注入风险 | ✅ | SessionState 从内部注册，非外部直接注入 |
| DoS 风险 | ⚠️ | 见 3.2 |

### 3.2 发现的问题

#### 🔶 中等风险: 快照文件无限积累 (manager.go:281)

`takeSnapshot()` 每 `SnapshotInterval`(默认5秒) 创建快照文件，永不清理：

```go
snapshotFile := filepath.Join(m.snapshotDir, fmt.Sprintf("snapshot-%d.json", time.Now().UnixNano()))
return os.WriteFile(snapshotFile, data, 0640)
```

磁盘耗尽风险。建议添加快照轮转或 TTL 清理机制。

**建议修复**:
```go
// 保留最近N个快照
const MaxSnapshots = 10
// takeSnapshot 后清理过期快照
```

#### 🔶 中等风险: eventCh 可能阻塞 TriggerFailover (manager.go:197)

`TriggerFailover` 向 `eventCh` 发送多个事件（每个会话2条），channel 容量 256。
大规模故障转移时若 `eventProcessor` 处理不及时，可能导致发送端阻塞。

goroutine 中发送事件不返回错误但可能永久阻塞：
```go
go func(nodeID string) {
    _ = m.TriggerFailover(nodeID)  // 可能在 full channel 上永久阻塞
}(node.NodeID)
```

**建议修复**:
```go
select {
case m.eventCh <- event:
default: // 非阻塞，避免死锁
    log.Printf("event channel full, dropped event")
}
```

#### 🔶 低风险: checkClientReachable 直接连接客户端 (manager.go:271)

```go
conn, err := d.DialContext(ctx, "tcp", clientIP+":445")
```

SMB 服务器主动连接客户端 IP:445，在某些网络拓扑下可能有问题（如严格入站防火墙）。
当前代码有超时保护 (3s)，影响有限。

#### 🔶 低风险: 快照文件权限 0640 (manager.go:282)

SMB 会话数据含敏感信息（用户名、客户端IP），0640 对非 root 用户可读。
建议收紧至 0600 或在特定场景下限制目录访问。

### 3.3 registry.go 审计结果

| 检查项 | 状态 | 说明 |
|--------|------|------|
| 竞态条件 | ✅ | 所有操作经 RWMutex 保护 |
| 内存泄漏 | ✅ | Remove 后无残留引用 |
| 边界检查 | ✅ | GetByNode/GetByShare 等安全遍历 |
| 序列化安全 | ✅ | 使用标准 json 库，无反序列化漏洞 |

## 4. 测试覆盖

```
go test ./internal/smb/stateful/... -cover
```

| 指标 | 值 |
|------|-----|
| 通过测试 | 8/8 |
| 语句覆盖率 | 59.7% |
| 失败测试 | 0 |

**测试覆盖文件**:
- TestNewStatefulFailoverManager
- TestRegisterAndUnregisterSession
- TestFindBestTarget
- TestGetStatus
- TestSessionStateRegistry
- TestCleanupExpiredSessions
- TestValidateSessionState (含4子测试)
- TestManagerStartStop

**未覆盖的关键路径**:
- `TriggerFailover` 完整路径（涉及并发会话迁移）
- `migrateSession` 失败路径
- `syncStateToPeers` 完整路径
- `checkPeerHealth` 触发故障转移路径
- `loadPersistedState` 快照恢复路径
- 快照文件清理逻辑（尚未实现）

## 5. Phase3 安全要求预告

Phase3 `loadbalancer.go` 和 `failover.go` 尚未实现，建议后续审计关注：

1. **负载均衡器**: 权重/健康分来源是否可控，防止恶意节点注入虚假健康数据
2. **Failover 触发器**: 权限检查（谁可以触发 failover？）
3. **跨节点通信**: 状态传输是否加密（smb/stateful 模块无 TLS 配置）
4. **速率限制**: failover 触发频率限制，防止频繁切换

## 6. 总结

| 维度 | 评估 |
|------|------|
| 代码质量 | 良好 - 并发安全，边界清晰 |
| 漏洞情况 | 无新增漏洞（go vet ✅, govulncheck ✅） |
| 测试覆盖 | 59.7% - 核心逻辑已覆盖，并发路径缺失 |
| 安全风险 | 中等 - 快照积累 + eventCh 阻塞为已知问题 |
| Go 版本 | 1.26.1 (仍需升级至 1.26.2) |

**本轮结论**: ✅ 代码通过审计。建议尽快修复快照积累和 eventCh 阻塞问题后再进入 Phase3。

## 7. 修复建议优先级

| 优先级 | 问题 | 文件 |
|--------|------|------|
| P1 | 快照文件 TTL 清理 | manager.go:takeSnapshot |
| P1 | eventCh 非阻塞发送 | manager.go:eventProcessor, TriggerFailover |
| P2 | 快照文件权限收紧 | manager.go:takeSnapshot |
| P2 | 补全 TriggerFailover/migrateSession 测试 | manager_test.go |
| P3 | 完善 checkClientReachable 网络策略 | manager.go:migrateSession |

---
*审计完成时间: 2026-04-15 03:39 CST*
