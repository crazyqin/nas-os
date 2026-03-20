# 刑部安全审计报告

**审计日期**: 2026-03-20  
**审计范围**: ~/nas-os 代码库  
**审计官**: 刑部

---

## 一、硬编码密码/密钥检查

### 检查结果: ✅ 通过

**检查命令**:
```bash
grep -rn -E "(password|passwd|secret|api_key|apikey|token)\s*[=:]\s*[\"'][^\"']{8,}[\"']" --include="*.go"
```

**发现**:
- 未发现硬编码的密码或密钥
- 敏感字段（password, token, secret）均为变量名或配置项
- 密码示例（如 `example:"secret123"`）仅用于 API 文档注释

**正面发现**:
- `internal/backup/credentials.go`: 实现 AES-256-GCM 加密存储凭证
- `internal/auth/secret_encryption.go`: 使用 PBKDF2 派生密钥（100000 迭代）
- 密钥文件权限设置为 0600（仅所有者可读写）

---

## 二、代码格式检查

### 检查结果: ✅ 通过

**检查命令**:
```bash
gofmt -l .
```

**输出**: 无（代码格式规范）

---

## 三、敏感文件权限检查

### 检查结果: ⚠️ 需改进

| 文件/目录 | 当前权限 | 建议权限 | 风险等级 |
|-----------|----------|----------|----------|
| `internal/backup/credentials.go` | 664 | 644 | 低 |
| `internal/auth/secret_encryption.go` | 664 | 644 | 低 |
| `configs/*.yaml` | 664 | 644 | 低 |
| `deploy/*.sh` | 775 | 755 | 低 |
| `monitoring/*.yml` | 664 | 644 | 低 |

**修复建议**:
```bash
chmod 644 internal/backup/credentials.go
chmod 644 internal/auth/secret_encryption.go
chmod 644 configs/*.yaml monitoring/*.yml
chmod 755 deploy/*.sh
```

**备注**: 当前权限风险较低，因为这些是源代码文件，不在生产运行时直接暴露。

---

## 四、RBAC 安全性检查

### 检查结果: ✅ 良好

**安全特性**:

1. **严格模式**: 默认配置 `StrictMode: true`，默认拒绝策略
   ```go
   // internal/rbac/manager.go:42
   StrictMode: true,  // 严格模式：默认拒绝
   ```

2. **权限检查流程**:
   - 管理员角色自动拥有所有权限
   - 先检查拒绝策略，再检查允许策略
   - 权限缓存机制（5分钟 TTL）
   - 审计日志记录

3. **无 SQL 注入风险**:
   - RBAC 模块使用内存存储（map），无数据库查询
   - 未发现 `fmt.Sprintf` 构造 SQL 语句

4. **权限粒度**:
   - 资源级别: system, user, storage, share, network, backup, audit 等
   - 操作级别: read, write, admin, execute
   - 支持权限依赖（DependsOn）

5. **中间件保护**:
   - Bearer Token 认证
   - 路径白名单（/health, /metrics）
   - 公开路径（/api/auth/login 等）

---

## 五、其他安全发现

### 正面发现 ✅

| 项目 | 状态 | 说明 |
|------|------|------|
| CSRF 保护 | ✅ 已实现 | `internal/web/middleware.go` |
| XSS 防护 | ✅ 已实现 | `internal/quota/handlers_v2.go` safeFilename |
| 敏感文件忽略 | ✅ 已配置 | `.gitignore` 包含敏感文件模式 |
| 密钥文件 | ✅ 无泄露 | 仓库中无 .key/.pem/.crt 文件 |
| 环境变量 | ✅ 正确使用 | 敏感配置通过环境变量/加密存储 |

### 注意事项 ⚠️

1. **TLS 跳过选项**:
   - `internal/backup/cloud.go` 存在 `Insecure bool` 字段
   - 用于特定场景（如自签名证书），需确保生产环境不启用

2. **CSRF 密钥警告**:
   - `internal/web/middleware.go:33` 提示 `NAS_CSRF_KEY` 未设置时生成临时密钥
   - 生产环境必须设置 `NAS_CSRF_KEY` 环境变量

---

## 六、修复建议汇总

### 高优先级
无

### 中优先级
1. 生产环境设置 `NAS_CSRF_KEY` 环境变量（至少32字节）

### 低优先级
1. 调整源代码文件权限为 644（组不可写）
2. 调整部署脚本权限为 755（其他用户不可写）

---

## 七、整体安全状态

### 评级: ✅ 安全

**总结**:
- 代码安全性良好，无明显漏洞
- RBAC 实现规范，严格模式默认拒绝
- 敏感数据加密存储，无硬编码凭证
- CSRF/XSS 防护已实现
- 文件权限问题为低风险

**建议**:
- 定期运行 `gosec` 进行自动化安全扫描
- 保持依赖库更新
- 生产部署前检查环境变量配置

---

*刑部 审核完毕*