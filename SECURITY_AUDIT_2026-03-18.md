# NAS-OS 安全审计报告

**审计日期**: 2026-03-18  
**项目**: /home/mrafter/clawd/workspace/nas-os  
**审计部门**: 刑部  
**审计范围**: 代码安全风险、RBAC 配置、internal/security 模块

---

## 一、审计结论

**整体安全状况**: ⚠️ **中等风险**

| 检查项 | 状态 | 说明 |
|-------|------|------|
| 硬编码凭证 | ✅ 通过 | 无实际凭证泄露，OAuth2 URL 为误报 |
| RBAC 配置 | ✅ 良好 | 实现完整，默认严格模式 |
| 加密实现 | ✅ 良好 | AES-256-GCM + Argon2id |
| 命令注入防护 | ⚠️ 需关注 | 部分命令参数来自变量 |
| 整数溢出 | ⚠️ 需关注 | 74处 uint64→int 转换 |
| 路径遍历 | ⚠️ 需关注 | 230处潜在风险 |

---

## 二、详细发现

### 2.1 硬编码密钥检查 ✅ 通过

**检查结果**: 无实际硬编码凭证泄露

**gosec G101 报告分析**:
- `internal/auth/oauth2.go` - OAuth2 配置 URL，非凭证
- `internal/cloudsync/providers.go` - OAuth2 Token URL，非凭证
- `internal/office/types.go` - 错误消息字符串，误报

**实际凭证管理方式**:
- 密码通过环境变量传递（`NAS_BACKUP_KEY`）
- 敏感配置通过配置文件管理
- iSCSI CHAP 密码通过 stdin 传递（避免命令行泄露）

### 2.2 RBAC 配置检查 ✅ 良好

**角色定义**:
```
admin     - 完全控制权限 (*:*)
operator  - 系统操作，无用户管理
readonly  - 只读访问
guest     - 最小权限
```

**安全特性**:
- ✅ 默认启用严格模式（StrictMode: true）
- ✅ 默认拒绝策略
- ✅ 权限缓存（TTL 5分钟）
- ✅ 审计回调支持
- ✅ 策略优先级和 deny 优先

**代码片段** (`internal/rbac/manager.go`):
```go
func DefaultConfig() Config {
    return Config{
        CacheEnabled: true,
        CacheTTL:     time.Minute * 5,
        StrictMode:   true,   // 默认拒绝
        AuditEnabled: true,   // 审计日志
    }
}
```

### 2.3 internal/security 模块检查 ✅ 良好

**加密实现** (`internal/security/v2/encryption.go`):
- ✅ AES-256-GCM 加密（AEAD）
- ✅ Argon2id 密钥派生（抗暴力破解）
- ✅ 随机 nonce 生成
- ✅ 目录级独立密钥

**MFA 实现** (`internal/security/v2/mfa.go`):
- ✅ TOTP (RFC 6238)
- ✅ SMS 验证
- ✅ WebAuthn 支持
- ✅ 备用恢复码

**敏感数据扫描** (`internal/security/scanner/types.go`):
- AWS Access Key 模式检测
- 私钥模式检测
- API Token 模式检测

### 2.4 命令注入风险 ⚠️ 需关注

**高风险位置**:

| 文件 | 行号 | 风险描述 |
|-----|------|---------|
| `internal/iscsi/manager.go` | 503 | 命令参数来自变量 |
| `pkg/btrfs/btrfs.go` | 331, 483, 503 | Btrfs 命令参数 |
| `internal/files/manager.go` | 837 | zip 命令参数拼接 |

**良好实践**:
```go
// iSCSI 密码通过 stdin 传递（良好）
execCmd := exec.Command("targetcli", "/iscsi/"+iqn+"/tpg1/auth", "set", "password=-")
execCmd.Stdin = strings.NewReader(secret)

// 备份密钥通过环境变量传递（良好）
cmd := exec.CommandContext(ctx, "openssl", "enc", "-aes-256-cbc", ...)
cmd.Env = append(os.Environ(), "NAS_BACKUP_KEY="+encryptKey)
```

**建议**: 
- 对外部输入进行白名单验证
- 使用固定参数而非字符串拼接

### 2.5 整数溢出风险 ⚠️ 需关注

**G115 问题**: uint64 → int64/int 转换可能溢出

**主要位置**:
- `internal/monitor/metrics_collector.go` - SMART 属性值
- `internal/search/engine.go` - 搜索结果计数
- `internal/quota/optimizer/optimizer.go` - 配额计算

**建议**: 添加边界检查函数

### 2.6 文件路径遍历风险 ⚠️ 需关注

**G304 问题**: 用户输入构造文件路径

**主要位置**:
- `plugins/filemanager-enhance/main.go` - 文件管理器
- `internal/files/manager.go` - 文件操作

**建议**:
- 使用 `filepath.Clean()` 清理路径
- 验证路径在预期目录范围内

---

## 三、安全配置文件

**`.gosec.json` 已配置规则豁免**:
```json
{
  "G101": "disabled - OAuth2 URLs trigger false positives",
  "G104": "disabled - cleanup/logging error handling",
  "G115": "disabled - NAS context conversions are safe"
}
```

**注意**: G304（路径遍历）和 G204（命令注入）未豁免，需实际修复。

---

## 四、修复优先级建议

### P0 - 本周内

1. 文件管理器路径验证加固
2. 命令注入风险审查

### P1 - 两周内

1. 整数溢出边界检查
2. 错误处理完善

### P2 - 持续改进

1. 权限配置收紧（0640/0750）
2. 依赖漏洞扫描集成

---

## 五、总结

| 方面 | 评估 |
|-----|------|
| 凭证管理 | ✅ 安全，无硬编码泄露 |
| RBAC 实现 | ✅ 完整，遵循最小权限原则 |
| 加密方案 | ✅ 安全，使用现代算法 |
| 命令执行 | ⚠️ 需加强输入验证 |
| 文件操作 | ⚠️ 需加强路径验证 |

**整体评估**: 项目安全架构设计合理，RBAC 和加密实现符合安全最佳实践。主要风险集中在外部输入验证环节，建议优先加固文件路径和命令参数验证。

---

**审计人**: 刑部安全审计系统  
**报告生成时间**: 2026-03-18 05:05 CST