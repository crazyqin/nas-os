# NAS-OS 安全审计报告

**审计日期**: 2026-03-14  
**审计范围**: v1.7.0 / v1.8.0 代码库  
**审计员**: 刑部安全审计组  
**风险等级**: 中等

---

## 一、执行摘要

本次安全审计对 NAS-OS 项目进行了全面的安全评估，涵盖代码安全、API 安全、依赖安全等方面。共发现 **15 个安全问题**，其中：

- **高危**: 4 个
- **中危**: 6 个
- **低危**: 5 个

主要风险集中在：
1. 默认凭据硬编码
2. 密码策略过弱
3. 命令注入风险
4. 路径遍历漏洞
5. 敏感数据处理不当

---

## 二、详细发现

### 2.1 高危问题

#### [HIGH-001] 默认管理员密码硬编码

**文件**: `internal/users/manager.go:82-86`

**代码**:
```go
// 默认密码：admin123（首次登录后应修改）
hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
```

**风险**: 默认密码 `admin123` 硬编码在源码中，攻击者可直接使用该凭据登录系统。

**修复建议**:
1. 首次启动时强制用户设置密码
2. 或生成随机临时密码并显示给用户
3. 添加首次登录强制修改密码机制

---

#### [HIGH-002] 密码强度要求过弱

**文件**: `internal/users/handlers.go:62`

**代码**:
```go
type ChangePasswordRequest struct {
    OldPassword string `json:"old_password" binding:"required"`
    NewPassword string `json:"new_password" binding:"required,min=6"`
}
```

**风险**: 密码最小长度仅 6 位，无复杂度要求，容易被暴力破解。

**修复建议**:
1. 密码最小长度提升至 12 位
2. 添加复杂度要求：大小写字母 + 数字 + 特殊字符
3. 实现密码强度检查器

---

#### [HIGH-003] 命令注入风险 - Docker 容器创建

**文件**: `internal/docker/manager.go:192-220`

**代码**:
```go
func (m *Manager) CreateContainer(name, image string, opts map[string]interface{}) (*Container, error) {
    args := []string{"run", "-d", "--name", name}
    // ...
    args = append(args, image)
    cmd := exec.Command("docker", args...)
    // ...
}
```

**风险**: `name`, `image`, `ports`, `volumes`, `env` 等参数未经验证直接拼接到命令行，存在命令注入风险。

**修复建议**:
1. 对所有输入进行白名单验证
2. 使用 Docker SDK 而非命令行调用
3. 对容器名、镜像名使用正则验证：`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`

---

#### [HIGH-004] 路径遍历漏洞

**文件**: `internal/backup/manager.go:644-648` (gosec 报告 G703)

**风险**: 备份文件路径未进行充分验证，攻击者可通过构造 `../` 访问任意文件。

**修复建议**:
1. 使用 `filepath.Clean()` 规范化路径
2. 验证路径在允许的目录范围内
3. 使用 `os.Root` (Go 1.24+) 防止 TOCTOU 攻击

---

### 2.2 中危问题

#### [MEDIUM-001] TOTP 密钥未加密存储

**文件**: `internal/auth/manager.go:103-106`

**代码**:
```go
// 存储密钥（实际应该加密存储）
m.configs[userID].TOTPSecret = setup.Secret
```

**风险**: TOTP 密钥以明文形式存储在配置文件中，一旦文件泄露，攻击者可生成有效验证码。

**修复建议**:
1. 使用 AES-GCM 加密存储密钥
2. 密钥应从硬件安全模块 (HSM) 或密钥管理服务获取
3. 或使用操作系统密钥链 (keyring)

---

#### [MEDIUM-002] MFA 会话存储在内存中

**文件**: `internal/auth/manager.go:310-325`

**风险**: MFA 会话存储在全局内存 map 中，服务重启后会话丢失，且无法跨实例共享。

**修复建议**:
1. 使用 Redis 等分布式缓存存储会话
2. 添加会话持久化机制
3. 设置合理的会话过期时间

---

#### [MEDIUM-003] 弱加密算法使用

**文件**: `internal/network/ddns_providers.go:559`

**代码**:
```go
h := sha1.New()
```

**风险**: SHA-1 已被证明存在碰撞攻击，不应用于安全敏感场景。

**修复建议**: 使用 SHA-256 或 SHA-3 替代。

---

#### [MEDIUM-004] CSRF 保护未完全实现

**文件**: `internal/web/middleware.go:163-180`

**代码**:
```go
// TODO: 验证 token (需要从 session 或 cookie 中获取期望的 token)
// if !validateCSRFToken(token) {
//     ...
// }
```

**风险**: CSRF 中间件框架存在但验证逻辑被注释，实际未提供保护。

**修复建议**:
1. 实现 CSRF token 生成和验证
2. 使用 `gorilla/csrf` 或类似库
3. 确保所有状态修改操作都受保护

---

#### [MEDIUM-005] 敏感信息泄露 - 错误消息

**文件**: 多处 handlers.go

**代码**:
```go
c.JSON(http.StatusInternalServerError, gin.H{
    "code": 500,
    "message": err.Error(),  // 可能泄露内部错误信息
})
```

**风险**: 错误消息直接返回给客户端，可能泄露内部实现细节。

**修复建议**:
1. 对外返回通用错误消息
2. 内部记录详细错误日志
3. 使用错误码而非错误文本

---

#### [MEDIUM-006] OpenSSL 命令行密码传递

**文件**: `internal/backup/encrypt.go:36-45`

**代码**:
```go
cmd := exec.Command(
    "openssl",
    // ...
    "-pass", "pass:"+e.password,
)
```

**风险**: 密码通过命令行参数传递，可通过 `ps` 命令或 `/proc` 查看。

**修复建议**:
1. 使用 Go 原生 AES-GCM 实现（已有）
2. 或通过文件描述符/环境变量传递密码
3. 优先使用 Go 实现，OpenSSL 作为降级选项

---

### 2.3 低危问题

#### [LOW-001] 硬编码 CSRF 密钥

**文件**: `internal/web/middleware.go:30`

**代码**:
```go
CSRFKey: []byte("change-this-to-a-32-byte-secret-key-now!"), // TODO: 从环境变量读取
```

**修复建议**: 从环境变量或配置文件读取。

---

#### [LOW-002] 配置文件权限

**文件**: `internal/users/manager.go:126`

**代码**:
```go
if err := os.WriteFile(m.configPath, data, 0600); err != nil {
```

**状态**: 已正确设置为 0600，仅所有者可读写。✅

---

#### [LOW-003] 缺少账户锁定机制

**风险**: 登录失败无锁定机制，可进行暴力破解。

**修复建议**:
1. 连续失败 5 次后锁定账户 15 分钟
2. 实现指数退避
3. 记录失败尝试日志

---

#### [LOW-004] WebSocket CORS 配置宽松

**文件**: `internal/system/handlers.go:18-22`

**代码**:
```go
var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        return true  // 允许所有来源
    },
}
```

**修复建议**: 验证 Origin 头，仅允许信任的域名。

---

#### [LOW-005] 缺少 API 版本控制

**风险**: API 缺少版本控制，未来更新可能破坏兼容性。

**修复建议**: 在路由中添加版本前缀（已有 `/api/v1`）。✅

---

## 三、API 安全评估

### 3.1 认证与授权

| 端点 | 认证要求 | 风险 |
|------|----------|------|
| `/api/v1/login` | 无 | 正常 |
| `/api/v1/users` | 需要 AuthMiddleware | ⚠️ 部分端点缺少认证 |
| `/api/v1/volumes` | 无认证 | 🔴 需添加认证 |
| `/api/v1/docker/*` | 无认证 | 🔴 需添加认证 |
| `/api/v1/files/*` | 无认证 | 🔴 需添加认证 |

**建议**: 为所有敏感 API 端点添加认证中间件。

### 3.2 输入验证

- ✅ 使用 Gin 的 `binding` 标签进行基本验证
- ⚠️ 部分输入缺少格式验证（如 IP 地址、端口、路径）
- ⚠️ 文件上传缺少类型和大小的严格限制

### 3.3 速率限制

- ✅ 已实现基础内存限流
- ⚠️ 生产环境应使用 Redis 分布式限流
- ⚠️ 登录接口应有单独的严格限流

---

## 四、依赖安全评估

运行 `go list -m -json all` 检查主要依赖：

| 依赖 | 版本 | 风险 |
|------|------|------|
| github.com/gin-gonic/gin | - | ✅ 活跃维护 |
| golang.org/x/crypto | - | ✅ 官方库 |
| github.com/aws/aws-sdk-go-v2 | v1.41.3 | ✅ 最新版本 |
| go.uber.org/zap | - | ✅ 活跃维护 |

**建议**: 定期运行 `go list -m -u all` 检查更新，或使用 Dependabot 自动化。

---

## 五、gosec 静态分析摘要

运行 gosec 发现的主要问题：

```
G703: Path traversal via taint analysis (HIGH)
G122: Symlink TOCTOU traversal (MEDIUM)  
G401: Weak cryptographic primitive (SHA-1) (MEDIUM)
G204: Subprocess launched with variable (MEDIUM)
```

建议将 gosec 集成到 CI/CD 流程中。

---

## 六、修复优先级建议

| 优先级 | 问题编号 | 描述 | 建议完成时间 |
|--------|----------|------|--------------|
| P0 | HIGH-001 | 默认密码 | 立即 |
| P0 | HIGH-004 | 路径遍历 | 1 周 |
| P1 | HIGH-002 | 密码策略 | 1 周 |
| P1 | HIGH-003 | 命令注入 | 2 周 |
| P1 | MEDIUM-001 | TOTP 密钥加密 | 2 周 |
| P2 | MEDIUM-004 | CSRF 实现 | 2 周 |
| P2 | MEDIUM-006 | OpenSSL 密码传递 | 3 周 |
| P3 | 其他低危问题 | - | 下一版本 |

---

## 七、最佳实践建议

1. **安全开发流程**
   - 在 CI/CD 中集成 gosec、golangci-lint
   - 代码审查必须包含安全检查
   - 定期进行渗透测试

2. **认证与授权**
   - 实现完整的 RBAC 权限控制
   - 所有敏感 API 都需要认证
   - 实现会话管理（JWT + 刷新机制）

3. **数据保护**
   - 敏感配置加密存储
   - 使用环境变量存储密钥
   - 实现审计日志

4. **网络安全**
   - 强制 HTTPS
   - 实现真正的 CSRF 保护
   - 添加 CSP 和其他安全头

---

## 八、总结

NAS-OS 是一个功能丰富的 NAS 操作系统，安全架构整体合理，但存在若干需要修复的安全问题。建议优先处理高危问题，特别是：

1. **移除默认密码**，实现首次启动强制设置密码
2. **加强输入验证**，防止命令注入和路径遍历
3. **完善认证中间件**，保护所有敏感 API

修复上述问题后，系统安全态势将显著提升。

---

**审计完成日期**: 2026-03-14  
**下次审计建议**: 修复完成后进行复查