# NAS-OS v2.18.0 安全审计报告

**审计日期**: 2026-03-14  
**审计版本**: v2.17.1 → v2.18.0  
**审计部门**: 刑部（法务合规）

---

## 一、扫描摘要

| 扫描工具 | 结果 |
|---------|------|
| gosec | 969 个问题 (主要是 G104 未处理错误) |
| govulncheck | ✅ 无依赖漏洞 |

---

## 二、发现的安全问题

### 🔴 高危 (4)

#### 1. CSRF 密钥硬编码
**位置**: `internal/web/middleware.go:30`  
**代码**:
```go
CSRFKey: []byte("change-this-to-a-32-byte-secret-key-now!"), // TODO: 从环境变量读取
```
**风险**: 硬编码密钥可被攻击者获取，导致 CSRF 保护失效  
**修复建议**: 从环境变量或安全配置文件读取密钥

#### 2. CSRF 保护未真正实现
**位置**: `internal/web/middleware.go:145-157`  
**代码**:
```go
// TODO: 验证 token (需要从 session 或 cookie 中获取期望的 token)
// 这里提供框架，具体实现需要配合认证系统
```
**风险**: CSRF 中间件只是占位符，实际不验证 token  
**修复建议**: 实现完整的 CSRF token 生成和验证逻辑

#### 3. 命令行传递加密密钥
**位置**: `internal/backup/manager.go:366`  
**代码**:
```go
cmd := exec.Command("openssl", "enc", "-aes-256-cbc", "-salt", "-in", backupPath, "-out", encryptedPath, "-pass", "pass:"+encryptKey)
```
**风险**: 密钥通过命令行参数传递，会在进程列表 (`ps aux`) 中暴露  
**修复建议**: 使用 openssl 的 `-passin` 从文件或环境变量读取密钥

#### 4. 脚本命令注入风险
**位置**: `internal/snapshot/executor.go:89`  
**代码**:
```go
cmd := exec.CommandContext(ctx, "sh", "-c", script)
```
**风险**: 用户提供的脚本直接传给 `sh -c` 执行，可能被恶意利用  
**修复建议**: 
- 限制可执行的命令白名单
- 对脚本内容进行严格验证
- 使用非 root 权限执行

---

### 🟠 中危 (3)

#### 5. 默认密码打印到控制台
**位置**: `internal/users/manager.go:158`  
**代码**:
```go
fmt.Printf("   密码: %s\n", defaultPassword)
```
**风险**: 密码可能被日志收集系统记录或被旁观者看到  
**修复建议**: 仅提示"请查看系统初始化日志获取密码"，或将密码写入仅 root 可读的文件

#### 6. 大量未处理错误 (G104)
**位置**: 全项目 969 处  
**示例**: `internal/api/validator.go:14-20` 注册验证器时忽略返回值  
**风险**: 错误被静默忽略，可能导致程序在无效状态下运行  
**修复建议**: 审查并处理关键路径的错误返回值，至少记录日志

#### 7. API 路由可能缺少认证保护
**位置**: `internal/web/server.go:440-570`  
**问题**: 全局中间件链包含安全头、限流、CSRF 等，但未见认证中间件应用到各路由组  
**风险**: 敏感 API 可能被未授权访问  
**修复建议**: 
- 对敏感操作应用 `RequireAuth()` 或 `RequireRole()` 中间件
- 确认所有写入操作都有认证保护

---

### 🟡 低危 (2)

#### 8. 密码强度验证较弱
**位置**: `internal/api/validator.go:35`  
**代码**:
```go
func validatePassword(fl validator.FieldLevel) bool {
    password := fl.Field().String()
    return len(password) >= 6
}
```
**风险**: 仅验证长度 ≥6，无复杂度要求  
**修复建议**: 添加复杂度要求（大小写、数字、特殊字符）

#### 9. 敏感字段可能泄露到日志
**位置**: 多处结构体包含 `json:"password"` 字段  
**风险**: 如果这些结构体被序列化记录到日志，密码会明文泄露  
**修复建议**: 对敏感字段使用 `json:"-"` 排除序列化，或使用专门的响应结构体

---

## 三、安全亮点 ✅

1. **密码存储安全** - 使用 bcrypt 进行密码哈希 (`internal/users/manager.go`)
2. **随机数生成安全** - 使用 `crypto/rand` 生成令牌和 ID
3. **SQL 注入防护** - 使用参数化查询，未发现字符串拼接 SQL
4. **路径遍历防护** - 有检测 `..` 路径的逻辑 (`internal/backup/manager.go:681`)
5. **安全头配置** - 设置了 CSP、X-Frame-Options、HSTS 等安全头

---

## 四、修复优先级建议

| 优先级 | 问题编号 | 预计工时 |
|--------|---------|---------|
| P0 (立即修复) | 1, 2, 3, 4 | 2-3 天 |
| P1 (本版本修复) | 5, 7 | 1 天 |
| P2 (后续版本) | 6, 8, 9 | 2-3 天 |

---

## 五、结论

本次审计发现 **4 个高危漏洞**，主要集中在 CSRF 保护和命令执行安全。建议在 v2.18.0 发布前修复所有高危问题。

依赖安全状态良好，govulncheck 未发现已知漏洞。

---

*刑部 安全审计组*