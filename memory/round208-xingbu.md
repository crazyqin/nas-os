# 刑部第208轮安全审计报告

## 审计范围
1. govulncheck漏洞扫描
2. FRP隧道模块安全审计 (`internal/connect/frp/`)
3. 依赖安全检查

---

## 一、govulncheck扫描结果

### 发现漏洞（6个）

| ID | 漏洞描述 | 影响包 | 风险 | 修复版本 |
|----|----------|--------|------|----------|
| GO-2026-4947 | chain building异常工作 | crypto/x509 | 中 | go1.26.2 |
| GO-2026-4946 | policy验证效率问题 | crypto/x509 | 中 | go1.26.2 |
| GO-2026-4870 | TLS 1.3 KeyUpdate DoS | crypto/tls | **高** | go1.26.2 |
| GO-2026-4869 | GNU sparse无界分配 | archive/tar | **高** | go1.26.2 |
| GO-2026-4866 | name constraints Auth Bypass | crypto/x509 | **高** | go1.26.2 |
| GO-2026-4865 | XSS漏洞 | html/template | **高** | go1.26.2 |

### 调用路径分析

**crypto/x509** 影响路径：
- `internal/ldap/client.go:89` → LDAP StartTLS

**crypto/tls** 影响路径：
- `internal/connect/frp/client.go:171` → TLS连接
- `internal/tunnel/signaling.go:132` → WebSocket TLS
- `internal/natpierce/natpierce.go:137` → NAT穿透TLS

**archive/tar** 影响路径：
- `internal/backup/config_backup.go:452` → 备份解压

**html/template** 影响路径：
- `internal/reports/enhanced_export.go:329` → HTML导出
- `internal/web/server.go:934` → Web服务

### 修复方案
```bash
# 升级Go版本
go version  # 当前: go1.26.1
# 需要: go1.26.2
```

---

## 二、FRP隧道安全审计

### 文件清单
- `config.go` - 配置定义
- `client.go` - 客户端核心
- `manager.go` - 管理器
- `protocol.go` - 协议定义
- `free_nodes.go` - 节点配置

### TLS配置审计

| 检查项 | 状态 | 说明 |
|--------|------|------|
| TLS最低版本 | ✅ | `tls.VersionTLS12` |
| InsecureSkipVerify | ✅ | `false` |
| 客户端证书 | ✅ | 支持双向认证 |
| 证书加载 | ✅ | `tls.LoadX509KeyPair` |

```go
// client.go:165-175
tlsConfig := &tls.Config{
    InsecureSkipVerify: false,  // ✓ 安全
    MinVersion:         tls.VersionTLS12,  // ✓ 安全
}
```

### 认证机制审计

| 检查项 | 状态 | 说明 |
|--------|------|------|
| Token认证 | ✅ | 存在认证机制 |
| 心跳机制 | ✅ | 30秒间隔 |
| 重连机制 | ✅ | 自动重连 |
| 认证超时 | ⚠️ | 10秒固定 |

**认证流程**:
1. 发送AuthRequest (version, token, timestamp, run_id)
2. 等待AuthResponse
3. 验证error字段

### 敏感信息存储审计

| 检查项 | 状态 | 风险 |
|--------|------|------|
| Token存储 | ⚠️ | JSON明文 |
| HTTPPwd存储 | ⚠️ | JSON明文 |
| AdminPwd存储 | ⚠️ | JSON明文 |
| 文件权限 | ✅ | `0600` |
| Sk (STCP密钥) | ⚠️ | JSON明文 |

**问题代码** (`config.go:184-186`):
```go
data, err := json.MarshalIndent(c, "", "  ")  // 明文序列化
return os.WriteFile(c.ConfigPath, data, 0600)  // 权限OK
```

### 安全风险矩阵

| 风险项 | 等级 | 说明 |
|--------|------|------|
| Token泄露 | 中 | 配置文件明文 |
| DoS攻击 | 中 | 无消息大小上限检查 |
| 重连风暴 | 低 | LoginFailExit=false |
| MITM | 低 | TLS启用 |

---

## 三、依赖安全检查

### 关键依赖（高关注）

| 依赖 | 版本 | 说明 |
|------|------|------|
| golang.org/x/crypto | v0.49.0 | 加密库 |
| golang.org/x/net | v0.52.0 | 网络库 |
| gorilla/websocket | v1.5.3 | WebSocket |
| go-jose/go-jose/v4 | v4.1.3 | JWT/JOSE |
| golang-jwt/jwt/v5 | v5.3.0 | JWT |

### 依赖总数: 168个

---

## 四、安全评分

### 评分: **C**

**评分依据**:
- ✅ 基础TLS配置安全
- ✅ 认证机制存在
- ⚠️ 标准库有6个已知漏洞（可修复）
- ⚠️ 敏感配置明文存储
- ⚠️ 无加密存储机制

---

## 五、改进建议

### 紧急（P0）
1. **升级Go到1.26.2** - 修复所有标准库漏洞
   ```bash
   # 检查当前版本
   go version
   # 升级后重新编译
   go build ./...
   ```

### 重要（P1）
2. **敏感配置加密存储**
   - Token使用AES-256加密
   - AdminPwd使用bcrypt哈希
   - Sk密钥加密存储
   ```go
   // 建议: 配置保存前加密敏感字段
   func (c *ClientConfig) encryptSensitiveFields(key []byte) error {
       c.Common.Token = encryptAES(c.Common.Token, key)
       c.Common.AdminPwd = hashBcrypt(c.Common.AdminPwd)
       ...
   }
   ```

3. **消息大小限制**
   - 添加最大消息体限制
   - 当前已有1MB限制 ✓ (`client.go:276`)

### 一般（P2）
4. **认证增强**
   - 添加认证超时配置
   - 支持OAuth2/OIDC

5. **审计日志**
   - 添加连接审计日志
   - 隧道操作日志

---

## 六、总结

| 类别 | 发现数 | 风险分布 |
|------|--------|----------|
| 标准库漏洞 | 6 | 高4 中2 |
| FRP安全问题 | 3 | 中2 低1 |
| 依赖风险 | 0 | 无已知 |

**整体评估**: 项目安全基线合格，需升级Go版本修复标准库漏洞，建议加密敏感配置。

---
*刑部安全审计 - 第208轮*  
*审计时间: 2026-04-09*