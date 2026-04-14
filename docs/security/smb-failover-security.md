# SMB Stateful Failover 安全设计文档

> **版本**: v1.0
> **日期**: 2026-04-15
> **负责部门**: 刑部（安全审计部）
> **对标**: TrueNAS Enterprise SMB HA Security / ISO 27001 / NIST CSF

---

## 1. 概述

### 1.1 文档目的

本文档为 nas-os SMB Stateful Failover 功能提供系统性的安全设计依据，涵盖威胁建模、安全设计要点、审计要求、权限控制和与 WriteOnce 不可变存储的联动机制。

### 1.2 适用范围

- SMB HA 双节点集群（Active/Passive）
- 所有跨节点 SMB 会话状态同步通道
- VIP 漂移机制中的网络安全
- 故障转移全流程中的安全事件记录

### 1.3 参考文档

| 文档 | 说明 |
|------|------|
| `docs/smb-stateful-failover-design.md` | SMB HA 架构设计 |
| `docs/security/smb-security-audit.md` | SMB 安全审计规范 |
| `docs/writeonce-guide.md` | WriteOnce 不可变存储指南 |
| `docs/security/kmip-design.md` | KMIP 密钥管理设计 |

---

## 2. 威胁模型分析

### 2.1 资产识别

SMB Stateful Failover 引入以下关键资产：

| 资产 | 敏感性 | 威胁影响 |
|------|--------|----------|
| SMB SessionKey（会话密钥） | **极高** | 泄露后攻击者可冒充任意会话 |
| NTLM/LM Hash | **极高** | 可用于 Pass-the-Hash 横向移动 |
| 文件锁状态（byte-range lock） | 高 | 状态篡改可导致数据不一致 |
| 打开的文件句柄 | 高 | 句柄劫持可导致未授权文件访问 |
| 跨节点同步数据流 | 高 | 中间人攻击可破坏状态完整性 |
| VIP 漂移控制权 | 高 | VIP 劫持可使攻击者截获所有 SMB 流量 |
| WriteOnce 快照元数据 | 中 | 元数据篡改可绕过不可变性保护 |

### 2.2 威胁场景

#### T1: 会话劫持（Session Hijacking）

**攻击描述**：攻击者通过获取目标会话的 SessionID，在故障转移期间或之后冒充合法客户端，接管已恢复的 SMB 会话。

**攻击路径**：
1. 攻击者获取 SessionID（通过网络嗅探、日志泄露或内部人员）
2. 主节点故障触发故障转移
3. 攻击者在新节点上使用窃取的 SessionID 发起重连请求
4. 服务端验证 SessionID 有效，恢复文件句柄
5. 攻击者获得对原用户文件的完全访问权

**风险等级**：**极高**
**影响范围**：所有活跃 SMB 会话

**缓解措施**：
- SessionKey 加密存储（AES-256-GCM），仅内存中解密
- 故障转移后强制 SessionKey 轮换（仅当客户端支持时）
- 添加客户端证书双向认证（可选）
- 审计所有会话恢复事件

```go
// 故障转移后会话验证
func (h *SMBRecoveryHandler) HandleReconnect(clientIP string, sessionID string) (*RecoveryResult, error) {
    session, err := h.sessionStore.GetSession(sessionID)
    if err != nil {
        h.auditLog.Warn("session_not_found", zap.String("session_id", sessionID))
        return nil, ErrSessionNotFound
    }

    // 验证客户端 IP 与原记录匹配（可选严格模式）
    if h.config.StrictIPCheck && session.ClientIP != clientIP {
        h.auditLog.Security("ip_mismatch_on_reconnect",
            zap.String("session_id", sessionID),
            zap.String("original_ip", session.ClientIP),
            zap.String("reconnect_ip", clientIP),
        )
        return nil, ErrIPMismatch
    }

    // 验证会话未超时
    if time.Since(session.LastActive) > h.config.SessionTimeout {
        h.sessionStore.DeleteSession(sessionID)
        return nil, ErrSessionExpired
    }

    // 验证会话在故障转移后才被允许恢复
    if session.FailoverRestricted {
        return nil, ErrSessionRestricted
    }

    return h.recoverSession(session, clientIP)
}
```

#### T2: 重放攻击（Replay Attack）

**攻击描述**：攻击者记录合法客户端的 SMB 认证请求或状态同步包，故障转移期间在链路上重放，冒充客户端或节点。

**攻击路径**：
1. 攻击者嗅探主节点到备节点的同步数据流
2. 攻击者捕获并保存加密的会话状态同步包
3. 攻击者在故障转移后重放旧的同步包
4. 备节点接受旧状态，导致会话状态回滚或分叉

**风险等级**：**高**
**影响范围**：跨节点状态同步通道

**缓解措施**：
- 状态同步包使用带时间戳的 HMAC-SHA256 签名
- 使用 ChaCha20-Poly1305 或 AES-256-GCM 加密，确保密文不可重放
- 引入单调递增的 Sequence Number，丢弃旧包
- 同步状态存储 Redis 使用带 TTL 的键，键值包含版本号

```go
// 同步包防重放设计
type SyncPacket struct {
    SequenceNumber  uint64    `json:"seq"`
    Timestamp       time.Time `json:"ts"`
    SourceNodeID    string    `json:"src"`
    StateData       []byte    `json:"data"` // AES-256-GCM 加密
    HMAC            []byte    `json:"hmac"` // HMAC-SHA256(Seq|Timestamp|Src|StateData)
}

func (e *StateSyncEngine) ValidatePacket(pkt *SyncPacket) error {
    // 1. 检查时间戳窗口（容忍 ±5 分钟）
    now := time.Now()
    if math.Abs(now.Sub(pkt.Timestamp).Seconds()) > 300 {
        return ErrTimestampExpired
    }

    // 2. 检查序列号（单调递增）
    lastSeq, ok := e.lastSequence[pkt.SourceNodeID]
    if ok && pkt.SequenceNumber <= lastSeq {
        return ErrSequenceReplayed
    }

    // 3. 验证 HMAC
    expectedMAC := e.computeHMAC(pkt)
    if !hmac.Equal(pkt.HMAC, expectedMAC) {
        return ErrHMACInvalid
    }

    e.lastSequence[pkt.SourceNodeID] = pkt.SequenceNumber
    return nil
}
```

#### T3: 中间人攻击（Man-in-the-Middle）

**攻击描述**：攻击者位于主备节点之间的网络路径上，拦截、篡改或注入状态同步流量。

**攻击路径**：
1. 攻击者获得主节点和备节点之间链路的中间位置
2. 攻击者截获加密的同步流量
3. 攻击者篡改同步数据（如修改文件锁状态）
4. 备节点应用被篡改的状态，导致数据不一致或安全策略绕过

**风险等级**：**高**
**影响范围**：节点间网络通道

**缓解措施**：
- 强制 TLS 1.3 加密所有节点间通信（mTLS 双向认证）
- 使用独立的专用网络（VLAN）隔离 HA 流量
- 所有同步数据加密存储（AES-256-GCM）
- 网络路径完整性监控（HBM 入侵检测）

```go
// 节点间 mTLS 配置
func (e *StateSyncEngine) setupMTLS() error {
    certPool := x509.NewCertPool()
    if !certPool.AppendCertsFromPEM(e.config.CACert) {
        return ErrCAInvalid
    }

    tlsConfig := &tls.Config{
        MinVersion:         tls.VersionTLS13,
        ClientCerts:        []tls.Certificate{e.nodeCert},
        RootCAs:            certPool,
        VerifyConnection:    e.verifyNodeCertificate,
        VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
            // 验证备用节点证书 CN 属于已知节点列表
            if len(verifiedChains) == 0 {
                return ErrCertNotVerified
            }
            nodeCN := verifiedChains[0][0].Subject.CommonName
            if !e.isAllowedNode(nodeCN) {
                return ErrNodeNotAllowed
            }
            return nil
        },
    }
    e.grpcServer = grpc.NewServer(grpc.Creds(credentials.NewTLS(tlsConfig)))
    return nil
}
```

#### T4: VIP 劫持（VIP Hijacking）

**攻击描述**：恶意节点（非集群成员）通过 ARP 欺骗宣称拥有 VIP，截获 SMB 客户端流量。

**风险等级**：**高**
**影响范围**：SMB 服务可用性和保密性

**缓解措施**：
- 使用静态 ARP 条目（备节点预先配置主节点的 MAC）
- 启用 802.1X 端口安全，限制可参与 VIP 竞争的端口
- 故障转移后立即发送 grat ARP（强制更新全网 ARP 表）
- 监控 ARP 表变更，检测异常 ARP 响应

```go
// VIP 劫持防护
func (v *VIPManager) AcquireVIP() error {
    v.mu.Lock()
    defer v.mu.Unlock()

    // 1. 前置检查：验证调用者是否为当前合法备节点
    if !v.isClusterMember(v.callerNodeID) {
        return ErrUnauthorizedNode
    }

    // 2. 执行 VIP 获取（带 gratuitous ARP）
    if err := v.netLink.AddIP(v.vip, v.iface); err != nil {
        return err
    }
    v.arpUtil.SendGratuitousARP(v.vip, v.iface)

    // 3. 验证 ARP 表中没有其他节点声称拥有此 VIP
    otherMAC, err := v.arpUtil.QueryVIPOwner(v.vip)
    if err == nil && otherMAC != v.localMAC {
        v.logger.Warn("potential_vip_hijack", zap.String("vip", v.vip),
            zap.String("expected_mac", v.localMAC),
            zap.String("actual_mac", otherMAC))
        // 触发安全告警
        h.auditLog.Security("vip_hijack_attempt",
            zap.String("vip", v.vip),
            zap.String("attacker_mac", otherMAC))
        return ErrVIPConflict
    }

    v.acquired = true
    return nil
}
```

#### T5: 凭证缓存投毒（Credential Cache Poisoning）

**攻击描述**：攻击者通过篡改存储的 NTLM Hash 或 SessionKey，使故障转移后的会话恢复使用攻击者提供的凭证。

**风险等级**：**极高**
**影响范围**：所有故障转移后的会话

**缓解措施**：
- 凭证数据 AES-256-GCM 加密存储，密钥来自 KMIP 或本地 TPM
- 加密密钥在每次故障转移后轮换
- 凭证存储仅对 `root:nas-os` 可读（chmod 600）
- 恢复会话时重新验证凭证（Kerberos ticket re-auth 可选）

#### T6: 拒绝服务（Denial of Service）

**攻击描述**：攻击者通过大量伪造心跳、快速触发故障转移、耗尽状态存储资源等方式，使 SMB 服务不可用。

**风险等级**：**中**
**影响范围**：SMB 服务可用性

**缓解措施**：
- 心跳来源必须通过 mTLS 认证，无效节点的心跳包直接丢弃
- 状态同步限速（单节点最大同步频率 100req/s）
- Redis 状态存储设置内存上限和 TTL
- 故障转移冷却期：同一 VIP 至少间隔 30 秒才允许再次转移

### 2.3 威胁矩阵

| 威胁 | 攻击向量 | 风险等级 | 攻击复杂度 | 影响维度 | 缓解措施 |
|------|----------|----------|-----------|----------|----------|
| T1 会话劫持 | 网络嗅探/日志泄露 | **极高** | 中 | 机密性+完整性 | SessionKey 加密+轮换 |
| T2 重放攻击 | 网络嗅探 | **高** | 中 | 完整性 | 序列号+HMAC+时间戳 |
| T3 中间人 | 网络路径 | **高** | 高 | 机密性+完整性 | mTLS+专用网络 |
| T4 VIP 劫持 | ARP 欺骗 | **高** | 中 | 可用性+机密性 | 静态 ARP+ARP 监控 |
| T5 凭证投毒 | 存储篡改 | **极高** | 高 | 机密性 | 凭证加密+密钥轮换 |
| T6 拒绝服务 | 协议滥用 | **中** | 低 | 可用性 | mTLS 认证+限速 |

---

## 3. 安全设计要点

### 3.1 认证强化

#### 3.1.1 节点间双向认证（mTLS）

所有跨节点通信强制使用 TLS 1.3 + 双向证书认证：

```yaml
# internal/ha/smb_failover_security.yaml
node_auth:
  tls_version: "1.3"              # 强制 TLS 1.3
  require_mtls: true               # 双向证书认证
  ca_cert_path: "/etc/nas-os/pki/ha-ca.pem"
  node_cert_path: "/etc/nas-os/pki/node.pem"
  node_key_path: "/etc/nas-os/pki/node-key.pem"
  cert_renewal_window: 720h       # 证书提前30天续期
  allowed_node_cns:
    - "nas-os-node-a"
    - "nas-os-node-b"
```

#### 3.1.2 SMB 会话密钥安全

```go
// 凭证加密存储
type SecureCredentialStore struct {
    encryptor encryptor.AES256GCMEncryptor
    store     StateStore
}

// SessionKey 在传输和存储时全程加密
// 密钥来自 KMS（KMIP）或本地 TPM
// 故障转移后：若客户端支持，触发 SessionKey 重新协商
```

#### 3.1.3 故障转移后凭证重验证

```
策略规则：
- 若故障转移前会话使用 Kerberos 认证：自动恢复（Kerberos ticket 仍有效）
- 若故障转移前会话使用 NTLM 认证：
  - 客户端主动重连：验证 NTLM 握手，重新生成 SessionKey
  - 超时未重连（> 60s）：清除缓存凭证
- 强制重认证阈值：同一会话经历 3 次故障转移后强制重认证
```

### 3.2 TLS 加密

#### 3.2.1 节点间通信加密矩阵

| 通道 | 加密算法 | 密钥长度 | 认证方式 | 说明 |
|------|----------|----------|----------|------|
| 状态同步（gRPC） | TLS 1.3 + AES-256-GCM | 256bit | mTLS 双向 | 推荐 |
| Redis 状态存储 | TLS 1.3 | 256bit | 证书 | Redis over TLS |
| 心跳（Keepalived） | IPSec / VRRP | 256bit | 预共享密钥 | 网络层加密 |
| VIP ARP 通告 | — | — | — | L2 层，依赖网络隔离 |

#### 3.2.2 SMB 会话加密（客户端到服务端）

| 配置项 | 安全值 | 说明 |
|--------|--------|------|
| `smb encrypt = required` | ✅ 强制 | 拒绝未加密连接 |
| `smb3 encryption algorithm` | AES-256-GCM | 最高加密强度 |
| `require stronge encryption` | true | 拒绝弱算法（SMB1 DES/RC4） |
| `reject md5 sessions` | true | 拒绝 MD5 签名 |

```bash
# SMB 全局加密配置（smb.conf）
[global]
    smb encrypt = required
    require strong crypto = yes
    reject md5 sessions = yes
    smb3 encryption algorithm = aes-256-gcm
```

### 3.3 IP 白名单

#### 3.3.1 节点管理平面 IP 白名单

仅允许来自已知管理 IP 的 HA 控制命令：

```yaml
# internal/ha/ip_allowlist.yaml
management_access:
  allowed_source_ips:
    - "10.0.0.1/32"    # Node A 管理IP
    - "10.0.0.2/32"    # Node B 管理IP
    - "10.0.0.254/32"  # HA 虚拟管理IP
  block_other_sources: true

failover_triggers:
  allowed_sources:
    - "10.0.0.1/32"
    - "10.0.0.2/32"
  # 仅集群节点可触发故障转移
```

#### 3.3.2 SMB 客户端连接控制

```
SMB 客户端访问控制：
- 使用标准 SMB ACL 和 IP 限制
- 故障转移期间：新 VIP 上的连接需要通过正常 SMB 认证
- VIP 漂移前已在连接的客户端：SMB3 会话恢复，无需重新输入密码
- 未认证的客户端尝试连接新 VIP：返回 access_denied
```

### 3.4 安全配置清单

```yaml
# SMB Failover Security Configuration
smb_failover_security:
  # === 认证与加密 ===
  node_mtls:
    enabled: true
    min_tls_version: "1.3"
    cert_renewal_hours: 720

  session_key_handling:
    encrypt_at_rest: true        # SessionKey 加密存储
    rotate_on_failover: true     # 故障转移后轮换
    max_key_age_hours: 24        # 密钥最大生命周期

  # === 网络安全 ===
  ip_whitelist:
    enabled: true
    ha_control_plane:
      - "10.0.0.1/32"
      - "10.0.0.2/32"
    smb_clients:
      enforced: false            # SMB 客户端不强制 IP 白名单

  network_isolation:
    dedicated_ha_vlan: "VLAN 100"  # 专用 HA VLAN
    reject_external_sync: true

  # === VIP 安全 ===
  vip_protection:
    static_arp_entries: true
    arp_guard_enabled: true
    failover_cooldown_seconds: 30

  # === 凭证安全 ===
  credential_storage:
    encryption: "AES-256-GCM"
    key_source: "kms"              # kmip 或 local
    storage_permission: "0600"
    max_cache_age_hours: 24
    forced_reauth_after_failovers: 3

  # === DoS 防护 ===
  dos_protection:
    sync_rate_limit_per_node: 100  # req/s
    heartbeat_timeout_seconds: 3
    heartbeat_interval_seconds: 1
    max_failover_per_hour: 10
```

---

## 4. 审计日志要求

### 4.1 故障转移事件审计

所有 SMB HA 故障转移相关事件必须记录到审计日志：

#### 4.1.1 审计事件类型

| 事件ID | 事件类型 | 严重性 | 说明 |
|--------|----------|--------|------|
| `smbha-001` | `failover_initiated` | INFO | 故障转移开始 |
| `smbha-002` | `failover_completed` | INFO | 故障转移完成 |
| `smbha-003` | `failover_failed` | ERROR | 故障转移失败 |
| `smbha-004` | `session_state_synced` | DEBUG | 会话状态同步 |
| `smbha-005` | `session_state_restore` | INFO | 会话状态恢复 |
| `smbha-006` | `session_recovery_mismatch` | WARN | 会话恢复 IP 不匹配 |
| `smbha-007` | `vip_acquired` | INFO | VIP 接管成功 |
| `smbha-008` | `vip_released` | INFO | VIP 释放 |
| `smbha-009` | `vip_hijack_detected` | **CRITICAL** | VIP 劫持检测 |
| `smbha-010` | `sync_packet_invalid` | WARN | 无效同步包 |
| `smbha-011` | `node_auth_failed` | **CRITICAL** | 节点认证失败 |
| `smbha-012` | `credential_restored` | WARN | 凭证被恢复 |
| `smbha-013` | `lock_state_recovered` | DEBUG | 文件锁状态恢复 |
| `smbha-014` | `denied_reconnect` | WARN | 未授权重连尝试 |
| `smbha-015` | `failover_cooldown_active` | INFO | 故障转移冷却期 |

#### 4.1.2 审计日志字段

```go
// SMBHAuditEvent SMB HA 审计事件
type SMBHAuditEvent struct {
    EventID        string                 `json:"event_id"`         // smbha-{timestamp}-{random}
    Timestamp      time.Time              `json:"timestamp"`        // UTC
    EventType      string                 `json:"event_type"`       // 见上表
    Severity       string                 `json:"severity"`         // INFO/WARN/ERROR/CRITICAL
    SourceNode     string                 `json:"source_node"`      // 源节点ID
    TargetNode     string                 `json:"target_node"`      // 目标节点ID
    SessionID      string                 `json:"session_id"`       // 关联会话（如有）
    VIP            string                 `json:"vip"`              // 相关VIP
    ClientIP       string                 `json:"client_ip"`        // 客户端IP
    DurationMs     int64                  `json:"duration_ms"`      // 操作耗时
    Result         string                 `json:"result"`            // success/failure
    ErrorMessage   string                 `json:"error_message"`     // 错误详情
    SecurityContext map[string]interface{} `json:"security_context"` // 安全上下文
    Metadata       map[string]string      `json:"metadata"`          // 附加信息
}
```

#### 4.1.3 日志存储要求

```
存储路径：/var/log/nas-os/audit/smb-ha/
文件名格式：smb-ha-audit-{YYYY-MM-DD}.log
格式：JSON Lines（每行一个事件）
权限：root:nas-os 640
轮转：每日轮转，保留 180 天
压缩：7 天后 gzip 压缩
完整性：使用 SHA-256 对每条日志行签名
```

#### 4.1.4 关键审计场景

**场景 1：故障转移完整流程记录**
```
[timestamp] failover_initiated
  - 触发原因（心跳超时/手动/API）
  - 受影响会话数
  - VIP 信息
  - 节点切换时间线（精确到毫秒）

[timestamp] session_state_synced (每会话一条)
  - SessionID、用户名、客户端IP
  - 打开文件数、持有锁数
  - 状态数据哈希（完整性验证）

[timestamp] vip_acquired
  - 新主节点、VIP、新 MAC

[timestamp] session_state_restore (每会话一条)
  - 恢复结果（成功/部分/失败）
  - 恢复的文件句柄数
  - 恢复的锁数

[timestamp] failover_completed
  - 总耗时
  - 成功恢复会话数
  - 失败会话数及原因
```

**场景 2：安全事件记录（VIP 劫持）**
```
[CRITICAL] vip_hijack_detected
  - 攻击者 MAC
  - 预期 MAC
  - VIP
  - 检测时间
  - 自动处置动作（阻断/告警）
```

**场景 3：未授权重连**
```
[WARN] denied_reconnect
  - SessionID
  - 重连客户端IP
  - 原会话客户端IP
  - 不匹配原因
  - 处置（拒绝/允许需重新认证）
```

### 4.2 安全告警规则

| 告警规则 | 阈值 | 严重性 | 自动处置 |
|----------|------|--------|----------|
| 节点认证失败 | > 1次/分钟 | CRITICAL | 封禁源IP 10分钟 |
| VIP 劫持检测 | > 0次 | CRITICAL | 立即断开并告警 |
| 无效同步包 | > 10次/分钟 | WARN | 触发网络诊断 |
| 故障转移频繁 | > 3次/小时 | HIGH | 锁定 HA 自动切换 |
| 会话恢复 IP 不匹配 | > 1次 | HIGH | 标记高风险会话 |
| 凭证缓存异常访问 | > 5次/分钟 | HIGH | 触发安全审计 |

---

## 5. 权限控制设计

### 5.1 状态存储访问控制

```
Redis/SQLite 状态存储权限矩阵：

| 角色 | 会话读取 | 会话写入 | 会话删除 | 凭证读取 | 锁操作 |
|------|----------|----------|----------|----------|--------|
| SMB 服务进程 | ✅ | ✅ | ✅ | ✅ | ✅ |
| HA Manager | ✅ | ✅ | ✅ | ❌ | ✅ |
| 审计模块 | ✅ | ❌ | ❌ | ❌ | ❌ |
| API (admin) | ✅ | ✅ | ✅ | ❌ | ✅ |
| 普通用户 | ❌ | ❌ | ❌ | ❌ | ❌ |
```

### 5.2 文件锁权限一致性

故障转移期间和之后，必须保证文件锁语义一致：

```
权限一致性要求：
1. 所有节点使用同一套 ZFS ACL 规则
2. 文件锁状态通过 StateSyncEngine 实时同步
3. 故障转移后重新验证文件锁的有效性：
   - 检查文件是否仍存在
   - 检查用户是否仍有相应权限
   - 检查锁是否未超时
4. 持有锁的会话在故障节点上过期时，自动释放锁
5. 拒绝恢复已失效的文件锁
```

### 5.3 API 权限控制

```yaml
# SMB HA API 权限
smb_ha_api_permissions:
  # 会话管理
  GET  /api/ha/smb/sessions          # admin, operator
  GET  /api/ha/smb/sessions/{id}     # admin, operator
  POST /api/ha/smb/flush             # admin only

  # 故障转移控制
  POST /api/ha/smb/failover          # admin only
  POST /api/ha/smb/failover/abort    # admin only
  GET  /api/ha/smb/failover/status   # admin, operator

  # VIP 管理
  GET  /api/ha/smb/vip/status        # admin, operator
  POST /api/ha/smb/vip/force-acquire # admin only

  # 安全配置
  GET  /api/ha/smb/security/config   # admin only
  PUT  /api/ha/smb/security/config   # admin only

  # 审计日志
  GET  /api/audit/smb-ha/events      # admin, auditor
  GET  /api/audit/smb-ha/statistics  # admin, auditor
```

### 5.4 POSIX 权限加固

```bash
# 状态存储文件权限
chmod 600 /var/lib/nas-os/smb-ha/sessions.db          # 仅 root 可读写
chmod 600 /var/lib/nas-os/smb-ha/credentials.enc     # 加密凭证
chown root:nas-os /var/lib/nas-os/smb-ha/ -R
chmod 700 /var/lib/nas-os/smb-ha/                    # 目录禁止其他用户访问

# 审计日志权限
chown root:nas-os /var/log/nas-os/audit/smb-ha/ -R
chmod 640 /var/log/nas-os/audit/smb-ha/*.log

# SMB HA 配置文件权限
chmod 600 /etc/nas-os/ha/smb-failover-security.yaml
chown root:nas-os /etc/nas-os/ha/smb-failover-security.yaml
```

---

## 6. 与 WriteOnce 不可变存储联动

### 6.1 联动设计目标

当 SMB Stateful Failover 发生故障转移时，需要与 WriteOnce 不可变存储联动，确保：

1. **不可变快照在故障转移后仍然有效**：快照元数据在节点间同步
2. **故障转移期间不破坏 WriteOnce 保护**：锁定的目录在故障转移后继续保持只读
3. **支持 WriteOnce 策略的跨节点一致性**：锁定/解锁操作在所有节点上保持一致
4. **防勒索联动**：SMB HA 故障不影响勒索防护保护的有效性

### 6.2 快照元数据同步

```go
// WriteOnce 元数据（随 SMB 状态同步）
type WriteOnceMetadata struct {
    SnapshotPath    string    `json:"snapshot_path"`     // 快照挂载路径
    OriginalPath   string    `json:"original_path"`     // 原目录路径
    LockedAt       time.Time `json:"locked_at"`          // 锁定时间
    LockedUntil    time.Time `json:"locked_until"`       // 锁定截止
    IsPermanent    bool      `json:"is_permanent"`       // 永久锁定
    ImmutableFlag  bool      `json:"immutable_flag"`     // chattr +i 已设置
    RansomwareProtected bool `json:"ransomware_protected"` // 防勒索保护
    Tags           []string  `json:"tags"`               // 标签
}

// 快照元数据与 SMB 状态一起同步
// 备节点在故障转移后自动恢复 WriteOnce 保护
```

### 6.3 联动流程

```
故障转移阶段与 WriteOnce 联动：

Phase 1 (检测): 
  ├─ HA 检测到节点故障
  └─ 记录当前 WriteOnce 锁定状态快照

Phase 2 (状态转移):
  ├─ SMB 会话状态同步（含 WriteOnce 元数据）
  ├─ WriteOnce 快照元数据同步到备节点
  └─ 验证备节点上的 btrfs 快照只读属性

Phase 3 (VIP 接管):
  ├─ VIP 漂移到备节点
  ├─ 恢复 SMB 会话
  └─ 重新应用 WriteOnce 保护（如有必要）:
       - 确认 btrfs 快照只读属性仍有效
       - 确认 chattr +i 属性仍存在
       - 如属性丢失（节点间不同步），重新设置

Phase 4 (验证):
  ├─ 验证所有 WriteOnce 目录仍为只读
  └─ 验证 SMB 只读访问已正确映射

Phase 5 (审计):
  ├─ 记录故障转移中的 WriteOnce 状态
  └─ 告警任何保护异常（如快照被意外修改）
```

### 6.4 防勒索联动机制

```
SMB HA 故障转移期间，勒索防护保护不中断：

1. btrfs 快照只读属性由内核强制，不依赖 SMB 服务
2. chattr +i 属性由文件系统强制，不依赖任何服务
3. SMB HA 仅负责恢复文件锁和会话，不修改快照元数据
4. 故障转移后 WriteOnce 保护自动延续：
   - btrfs 快照层：内核自动维护
   - SMB 访问控制：HA 恢复后 SMB ACL 重新生效
   - 防勒索监控：audit 模块独立运行，不受影响
```

### 6.5 场景化联动策略

| 场景 | WriteOnce 行为 | SMB HA 行为 |
|------|---------------|-------------|
| 主节点故障，备节点接管 | 快照只读属性不变（内核层） | 恢复 SMB 会话，限制到只读共享 |
| 备节点 WriteOnce 快照损坏 | 尝试从 ZFS 快照链恢复 | 拒绝接管，触发告警 |
| 故障转移期间有人尝试解锁 WriteOnce | 锁定操作记录到审计日志 | SMB HA 继续，不干预 |
| 快速连续故障转移（< 30s） | 冷却期拒绝解锁操作 | 冷却期拒绝再次转移 |
| WriteOnce 永久锁定 + 备节点接管 | 快照永久只读，不受影响 | 恢复 SMB 只读会话 |

### 6.6 安全注意事项

> **警告**：WriteOnce 快照元数据中包含快照路径和锁定策略，属于中等敏感信息，随 SMB 状态一起通过 mTLS 加密通道同步。若快照路径本身包含敏感信息（如 `/data/并购计划/`），建议同时对快照目录名进行混淆处理，或将快照存储在独立加密存储池中。

---

## 7. 安全测试计划

### 7.1 渗透测试用例

| 测试ID | 测试场景 | 预期结果 | 严重性 |
|--------|----------|----------|--------|
| ST-01 | 使用过期 SessionID 尝试会话恢复 | 返回 `ErrSessionExpired` | 高 |
| ST-02 | 使用其他客户端 IP 尝试恢复会话 | 记录安全事件，可选拒绝 | 高 |
| ST-03 | 重放旧的同步数据包 | HMAC 验证失败，包被丢弃 | 高 |
| ST-04 | 使用伪造节点证书连接同步通道 | mTLS 握手失败，连接拒绝 | 高 |
| ST-05 | 模拟 ARP 欺骗宣告 VIP | 检测到 VIP 冲突，触发告警 | 高 |
| ST-06 | 大量并发同步请求触发 DoS | 限速生效，部分请求被拒绝 | 中 |
| ST-07 | 故障转移后尝试修改 WriteOnce 快照 | 拒绝操作，返回只读错误 | 高 |
| ST-08 | 未授权用户调用 HA API | 返回 403 Forbidden | 高 |

### 7.2 回归测试

- 故障转移后 SMB 加密连接不受影响
- 节点重启后 SMB HA 安全配置不丢失
- 日志轮转不影响审计完整性
- WriteOnce 快照在多次故障转移后仍保持只读

---

## 8. 合规映射

### 8.1 ISO 27001

| 控制项 | SMB HA 安全设计支持 |
|--------|---------------------|
| A.9.4.3 口令使用策略 | ✅ SessionKey 加密存储，凭证不泄露 |
| A.10.1 加密策略 | ✅ TLS 1.3 + AES-256-GCM，mTLS |
| A.12.4.1 事件日志记录 | ✅ 完整故障转移审计事件 |
| A.12.4.2 管理员日志 | ✅ 所有 HA 操作均有记录 |
| A.13.2.1 信息传输策略 | ✅ 节点间 mTLS 加密传输 |
| A.18.1.3 资产管理 | ✅ 会话状态作为敏感资产保护 |

### 8.2 NIST CSF

| 功能 | SMB HA 安全设计支持 |
|------|---------------------|
| Identify（识别） | ✅ 资产识别，敏感数据分类 |
| Protect（防护） | ✅ 加密、认证、访问控制 |
| Detect（检测） | ✅ 异常检测、VIP 劫持检测 |
| Respond（响应） | ✅ 告警、故障转移冷却期 |
| Recover（恢复） | ✅ 有状态会话恢复 |

---

## 9. 版本记录

| 版本 | 日期 | 变更 | 作者 |
|------|------|------|------|
| v1.0 | 2026-04-15 | 初始版本（威胁模型+安全设计+审计+联动） | 刑部 |

---

*本文档由刑部（NAS-OS 安全审计部）编制，遵循 ISO 27001 和 NIST CSF 标准*
