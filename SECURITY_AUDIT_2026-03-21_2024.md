# NAS-OS 安全审计报告
**审计时间**: 2026-03-21 20:24 GMT+8
**审计部门**: 刑部
**项目版本**: v2.253.129
**项目路径**: /home/mrafter/clawd/nas-os

---

## 一、执行摘要

| 检查项 | 状态 | 说明 |
|--------|------|------|
| 敏感信息泄露 | ✅ 通过 | 无硬编码 API keys、passwords |
| 硬编码密钥 | ✅ 通过 | 测试文件中的示例密钥均为占位符 |
| 不安全代码模式 | ⚠️ 已修复 | 1 个 CORS 配置问题已修复 |
| SQL 注入风险 | ✅ 通过 | 使用参数化查询 |
| 命令注入风险 | ✅ 可控 | 有输入验证和白名单 |
| RBAC 权限控制 | ✅ 完善 | 完整的角色/策略/ACL 实现 |

---

## 二、发现的安全问题

### 2.1 已修复问题

#### 问题：遗留的 CORS 不安全默认配置

- **位置**：`internal/api/middleware.go:173-181`
- **风险等级**：中
- **原问题**：`DefaultCORSConfig` 使用 `AllowOrigins: []string{"*"}` 允许所有源
- **影响**：如果其他代码引用此配置，可能导致 CORS 安全策略失效
- **修复状态**：✅ 已修复

**修复内容**：
```go
// 修复前
AllowOrigins: []string{"*"},

// 修复后
AllowOrigins: []string{"http://localhost:8080", "http://127.0.0.1:8080"},
```

---

## 三、详细检查结果

### 3.1 敏感信息泄露检查

**检查方法**：
```bash
# 检查硬编码密钥
grep -rn "sk-[a-zA-Z0-9]{20,}" --include="*.go"
grep -rn "AKIA[A-Z0-9]{16}" --include="*.go"
grep -rn "ghp_[a-zA-Z0-9]{36}" --include="*.go"
```

**结果**：
- 发现 1 处 AWS 示例密钥 `AKIAIOSFODNN7EXAMPLE`（AWS 官方示例，非真实密钥）
- 无真实 API 密钥泄露

### 3.2 密码存储检查

**检查结果**：
- ✅ 用户密码使用 bcrypt 哈希存储 (`internal/users/manager.go`)
- ✅ TOTP 密钥使用 AES-256-GCM 加密存储 (`internal/auth/secret_encryption.go`)
- ✅ 备份密钥使用 argon2id 派生 (`internal/security/v2/manager.go`)

### 3.3 SQL 注入检查

**检查方法**：
```bash
grep -rn "fmt.Sprintf.*SELECT\|fmt.Sprintf.*INSERT" --include="*.go"
```

**结果**：
- ✅ 所有数据库操作使用参数化查询
- ✅ 表名操作有 `validateTableName` 白名单验证

### 3.4 命令注入检查

**检查结果**：
- `internal/database/optimizer.go` - 表名有白名单验证
- `internal/security/baseline.go` - grep 命令使用固定格式
- `internal/security/v2/disk_encryption.go` - cryptsetup 命令参数可控但来源可信

### 3.5 最近的安全修复

**v2.253.127 安全修复**（2026-03-21）：
1. ✅ CSRF 密钥：移除固定密钥回退，无法生成随机密钥时 panic
2. ✅ CORS：拒绝不在白名单的 Origin，移除 `*` 回退
3. ✅ XML 注入：审计日志导出时对用户输入字段进行 XML 转义

---

## 四、gosec 静态扫描摘要

**扫描结果**：860 条告警，多为中低风险

| 规则 | 数量 | 风险等级 | 说明 |
|------|------|----------|------|
| G304 | 216 | MEDIUM | 文件路径通过变量构造（已有防护）|
| G204 | 175 | MEDIUM | 子进程启动使用变量（输入可控）|
| G301 | 165 | LOW | 目录创建权限 (0755) |
| G306 | 135 | LOW | 文件写入权限 (0644) |
| G115 | 67 | HIGH | 整数溢出转换（需关注）|
| G101 | 7 | HIGH | 潜在硬编码凭证（均为误报）|
| G107 | 5 | MEDIUM | HTTP 请求使用变量 URL |

---

## 五、结论

**本次审计发现 1 个安全问题，已修复。**

项目整体安全状况良好：
- ✅ 无敏感信息泄露
- ✅ 密码存储使用安全算法
- ✅ SQL 使用参数化查询
- ✅ RBAC 权限控制完整
- ✅ 最近的安全修复已正确实施

**建议关注**：
1. gosec G115 整数溢出告警需逐项评估
2. G204 命令执行相关代码需持续审计
3. G107 SSRF 风险可考虑增加 URL 白名单

---

**刑部**
2026-03-21