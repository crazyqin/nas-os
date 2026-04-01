# NAS-OS 安全审计报告

**审计日期**: 2026-03-14  
**审计范围**: 安全审计与权限审查  
**项目路径**: `/home/mrafter/clawd/nas-os`

---

## 一、审计摘要

| 类别 | 状态 | 发现问题数 |
|------|------|-----------|
| API 安全性 | ⚠️ 需改进 | 4 |
| XSS/CSRF 防护 | ⚠️ 部分实现 | 2 |
| 敏感数据处理 | ❌ 高风险 | 3 |
| 权限控制 | ❌ 高风险 | 2 |
| 审计日志 | ⚠️ 需改进 | 3 |

**总体评估**: 🔴 **存在较高安全风险**

---

## 二、详细发现

### 2.1 API 安全性（输入验证）

#### 🔴 高危：路径遍历防护不足

**位置**: `internal/files/manager.go`

**问题描述**:  
文件操作 API 的路径遍历检查不够严格，仅检查 `..` 字符串，存在以下漏洞：

1. **未检查绝对路径攻击**：攻击者可传入 `/etc/passwd` 等系统敏感文件
2. **未检查符号链接攻击**：攻击者可创建符号链接绕过检查
3. **未使用 `filepath.Clean()` 规范化路径**
4. **未限制在安全根目录范围内**

**漏洞代码示例**:
```go
// 当前实现 - 仅检查 ".."
if strings.Contains(path, "..") {
    c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效路径"})
    return
}
```

**攻击示例**:
```
GET /api/v1/files/preview?path=/etc/passwd        # 绕过 ".." 检查
GET /api/v1/files/download?path=/etc/shadow       # 读取敏感文件
```

**修复建议**:
```go
func sanitizePath(baseDir, userPath string) (string, error) {
    // 1. 规范化路径
    cleanPath := filepath.Clean(userPath)
    
    // 2. 检查是否为绝对路径
    if filepath.IsAbs(cleanPath) {
        return "", errors.New("不允许使用绝对路径")
    }
    
    // 3. 拼接基础目录
    fullPath := filepath.Join(baseDir, cleanPath)
    
    // 4. 解析符号链接并检查
    realPath, err := filepath.EvalSymlinks(fullPath)
    if err != nil {
        return "", err
    }
    
    // 5. 确保最终路径在安全目录内
    if !strings.HasPrefix(realPath, baseDir) {
        return "", errors.New("路径越界")
    }
    
    return realPath, nil
}
```

---

#### ⚠️ 中危：输入验证不完整

**位置**: 多个 handlers 文件

**问题描述**:
- `createDir` 的 `name` 字段未检查特殊字符
- `createVolume` 的 `name` 字段未限制长度和字符
- `compressFile` 的 `Name` 字段未验证

---

### 2.2 XSS/CSRF 防护

#### ⚠️ 中危：CSRF 保护未完全实现

**位置**: `internal/web/middleware.go`

**问题描述**:  
CSRF 中间件存在但验证逻辑为空：

```go
// TODO: 验证 token (需要从 session 或 cookie 中获取期望的 token)
// 这里提供框架，具体实现需要配合认证系统
```

**风险**: 所有状态修改操作（POST/PUT/DELETE）没有 CSRF 保护。

---

#### ⚠️ 低危：CSP 策略不够严格

**位置**: `internal/web/middleware.go`

```go
c.Header("Content-Security-Policy", 
    "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; ...")
```

**问题**: `style-src` 允许 `'unsafe-inline'`，可能导致 CSS 注入攻击。

---

### 2.3 敏感数据处理

#### 🔴 高危：密钥硬编码

**位置**: `internal/web/middleware.go`

```go
CSRFKey: []byte("change-this-to-a-32-byte-secret-key-now!"), // TODO: 从环境变量读取
```

**风险**: CSRF 密钥硬编码，攻击者可伪造请求。

---

#### 🔴 高危：MFA 密钥明文存储

**位置**: `internal/auth/manager.go`

```go
// 存储密钥（实际应该加密存储）
m.configs[userID].TOTPSecret = setup.Secret
```

**风险**: TOTP 密钥以明文存储在配置文件中，若配置文件泄露，攻击者可生成有效验证码。

---

#### 🔴 高危：备份文件可能包含敏感信息

**位置**: `internal/backup/manager.go`

备份操作未对敏感文件（如密钥、密码文件）进行特殊处理。

---

### 2.4 权限控制

#### 🔴 **最高风险**：API 路由缺少认证中间件

**位置**: `internal/web/server.go`

**问题描述**:  
虽然存在完善的认证和 RBAC 中间件，但**大部分敏感 API 路由未应用**：

**未保护的敏感端点**:
| 端点 | 风险等级 | 说明 |
|------|---------|------|
| `/api/v1/volumes` | 高 | 卷管理，可删除数据 |
| `/api/v1/volumes/:name/snapshots` | 高 | 快照管理 |
| `/api/v1/backup/*` | 高 | 备份/恢复操作 |
| `/api/v1/files/*` | 高 | 文件操作 |
| `/api/v1/system/*` | 中 | 系统信息 |
| `/api/v1/docker/*` | 高 | 容器管理 |

**证据** (`server.go`):
```go
// ========== 卷管理 ==========
api.GET("/volumes", s.listVolumes)        // 无中间件
api.POST("/volumes", s.createVolume)      // 无中间件
api.DELETE("/volumes/:name", s.deleteVolume) // 无中间件
```

**对比** (`users/handlers.go` 注释):
```go
// 注意：调用方应在应用此路由组前添加认证和权限中间件
// 示例：api.Group("/users", authMiddleware, adminMiddleware)
```

**风险**:
- 任何人可删除卷和快照
- 任何人可创建/删除备份
- 任何人可访问文件系统

---

### 2.5 审计日志

#### ⚠️ 中危：审计日志不完整

**问题**:
1. 只记录请求开始，未记录响应结果
2. 未记录操作用户（因认证中间件未应用）
3. 日志文件路径硬编码，可能因权限问题写入失败

---

#### ⚠️ 中危：敏感操作日志不足

**位置**: `internal/web/middleware.go`

**当前覆盖**:
```go
sensitivePaths := []string{
    "/api/v1/volumes",
    "/api/v1/users",
    "/api/v1/shares",
    "/api/v1/raid",
}
```

**缺少**:
- `/api/v1/backup/*` - 备份操作
- `/api/v1/files/*` - 文件操作
- `/api/v1/docker/*` - 容器操作
- `/api/v1/vms/*` - 虚拟机操作

---

### 2.6 其他安全问题

#### ⚠️ 中危：WebSocket 认证缺失

**位置**: `internal/system/handlers.go`

```go
var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        return true  // 允许所有来源
    },
}
```

**风险**: WebSocket 连接无认证，任何人可连接获取系统信息。

---

#### ⚠️ 低危：错误信息泄露

**位置**: `internal/users/handlers.go`

```go
if err == ErrUserNotFound || err == ErrInvalidPassword {
    c.JSON(http.StatusUnauthorized, Error(401, "用户名或密码错误"))
    return
}
```

**问题**: 登录失败返回统一错误信息，但内部错误详情可能通过 `err.Error()` 泄露。

---

#### ⚠️ 低危：缺少登录失败限制

**位置**: `internal/users/handlers.go`

**问题**: 登录接口没有失败次数限制，存在暴力破解风险。

---

## 三、安全问题汇总

| 编号 | 严重程度 | 问题 | 位置 |
|------|---------|------|------|
| SEC-001 | 🔴 高危 | API 路由缺少认证中间件 | web/server.go |
| SEC-002 | 🔴 高危 | 路径遍历防护不足 | files/manager.go |
| SEC-003 | 🔴 高危 | 密钥硬编码 | web/middleware.go |
| SEC-004 | 🔴 高危 | MFA 密钥明文存储 | auth/manager.go |
| SEC-005 | ⚠️ 中危 | CSRF 保护未实现 | web/middleware.go |
| SEC-006 | ⚠️ 中危 | WebSocket 认证缺失 | system/handlers.go |
| SEC-007 | ⚠️ 中危 | 审计日志不完整 | web/middleware.go |
| SEC-008 | ⚠️ 中危 | 输入验证不完整 | 多个 handlers |
| SEC-009 | ⚠️ 低危 | CSP 策略不严格 | web/middleware.go |
| SEC-010 | ⚠️ 低危 | 缺少登录失败限制 | users/handlers.go |

---

## 四、修复优先级建议

### 立即修复（高危）

1. **为所有敏感 API 添加认证中间件**
   ```go
   // 修复示例
   api.GET("/volumes", 
       auth.AuthMiddleware(s.userMgr),  // 添加认证
       s.listVolumes,
   )
   ```

2. **修复路径遍历漏洞**
   - 实现安全的路径验证函数
   - 限制文件操作在安全根目录内

3. **移除硬编码密钥**
   - 从环境变量读取敏感配置
   - 使用 Vault 或类似方案管理密钥

4. **加密存储 MFA 密钥**
   - 使用 AES-256-GCM 加密 TOTP 密钥

### 短期修复（中危）

5. **实现 CSRF 验证逻辑**
6. **添加 WebSocket 认证**
7. **完善审计日志**
8. **增强输入验证**

### 长期改进（低危）

9. **收紧 CSP 策略**
10. **添加登录失败限制**

---

## 五、合规性检查

| 检查项 | 状态 | 说明 |
|--------|------|------|
| 认证机制 | ⚠️ 存在但未应用 | 有完整实现，但未在路由中启用 |
| 授权机制 | ⚠️ 存在但未应用 | RBAC 实现完善，但未在路由中启用 |
| 输入验证 | ⚠️ 部分 | 存在中间件但不够完善 |
| 输出编码 | ✅ 正常 | 使用 JSON 序列化 |
| 会话管理 | ✅ 正常 | JWT 实现合理 |
| 加密存储 | ❌ 缺失 | 敏感数据明文存储 |
| 审计日志 | ⚠️ 部分 | 框架存在，实现不完整 |
| 错误处理 | ⚠️ 部分 | 可能泄露内部信息 |

---

## 六、结论

NAS-OS 项目存在**较高的安全风险**，主要问题是：

1. **认证和授权机制虽已实现，但未在 API 路由中应用**，导致大部分敏感操作无需认证即可执行
2. **路径遍历防护不足**，可能导致任意文件读取
3. **敏感数据（密钥、MFA 密钥）存储不安全**

建议在发布前优先修复高危问题，特别是 API 认证和路径遍历问题。

---

**审计人**: 刑部  
**报告日期**: 2026-03-14