# 安全模块集成指南

## 概述

安全模块提供了完整的 NAS-OS 安全加固功能，包括防火墙管理、失败登录保护、安全审计和安全基线检查。

## 目录结构

```
nas-os/internal/security/
├── README.md          # 模块说明
├── types.go           # 类型定义
├── manager.go         # 安全管理器（统一入口）
├── firewall.go        # 防火墙管理
├── fail2ban.go        # 失败登录保护
├── audit.go           # 安全审计日志
├── baseline.go        # 安全基线检查
└── handlers.go        # HTTP 处理器
```

## 快速开始

### 1. 初始化安全管理器

在应用启动时初始化安全管理器：

```go
import "nas-os/internal/security"

// 创建安全管理器
securityMgr := security.NewSecurityManager()

// 设置通知回调（可选）
securityMgr.SetNotifyFunc(func(alert security.SecurityAlert) {
    // 发送通知（邮件、短信、Webhook 等）
    log.Printf("安全告警：%s - %s", alert.Title, alert.Description)
})
```

### 2. 注册 HTTP 路由

```go
import "nas-os/internal/security"

// 创建处理器
securityHandlers := security.NewHandlers(securityMgr)

// 注册路由（在 Gin 路由器中）
api := r.Group("/api")
{
    // 注册安全模块路由
    securityHandlers.RegisterRoutes(api)
}
```

### 3. 集成到登录流程

在用户登录时记录成功/失败：

```go
// 登录失败时
securityMgr.RecordFailedLogin(
    c.ClientIP(),
    username,
    c.UserAgent(),
    "密码错误",
)

// 登录成功时
securityMgr.RecordSuccessfulLogin(
    c.ClientIP(),
    username,
    c.UserAgent(),
    mfaMethod, // "totp", "sms", 或 ""
)
```

### 4. 访问控制中间件

```go
// 安全访问控制中间件
func SecurityMiddleware(mgr *security.SecurityManager) gin.HandlerFunc {
    return func(c *gin.Context) {
        ip := c.ClientIP()
        
        // 检查访问是否允许
        if !mgr.IsAccessAllowed(ip) {
            c.JSON(http.StatusForbidden, gin.H{
                "error": "访问被拒绝",
                "reason": "IP 地址已被封禁",
            })
            c.Abort()
            return
        }
        
        c.Next()
    }
}

// 使用中间件
r.Use(SecurityMiddleware(securityMgr))
```

### 5. 记录操作日志

```go
// 记录用户操作
securityMgr.RecordAction(
    userID,
    username,
    c.ClientIP(),
    "file",           // 资源类型
    "delete",         // 操作
    map[string]interface{}{
        "file_path": "/path/to/file",
    },
    "success",        // 状态
)
```

## API 端点

### 仪表板
- `GET /api/security/dashboard` - 获取安全仪表板数据

### 防火墙
- `GET /api/security/firewall/rules` - 获取防火墙规则列表
- `POST /api/security/firewall/rules` - 添加防火墙规则
- `PUT /api/security/firewall/rules/:id` - 更新防火墙规则
- `DELETE /api/security/firewall/rules/:id` - 删除防火墙规则
- `GET /api/security/firewall/blacklist` - 获取 IP 黑名单
- `POST /api/security/firewall/blacklist` - 添加 IP 到黑名单
- `DELETE /api/security/firewall/blacklist/:ip` - 从黑名单移除 IP
- `GET /api/security/firewall/whitelist` - 获取 IP 白名单

### 失败登录保护
- `GET /api/security/fail2ban/status` - 获取保护状态
- `PUT /api/security/fail2ban/config` - 更新配置
- `GET /api/security/fail2ban/banned` - 获取被封禁的 IP
- `POST /api/security/fail2ban/unban/:ip` - 解封 IP
- `GET /api/security/fail2ban/attempts/:ip` - 获取失败尝试记录

### 安全审计
- `GET /api/security/audit/logs` - 获取审计日志
- `GET /api/security/audit/login-logs` - 获取登录日志
- `GET /api/security/audit/alerts` - 获取安全告警
- `POST /api/security/audit/alerts/:id/acknowledge` - 确认告警
- `GET /api/security/audit/stats` - 获取统计信息
- `GET /api/security/audit/export` - 导出日志

### 安全基线
- `GET /api/security/baseline/checks` - 获取检查项列表
- `GET /api/security/baseline/check` - 运行基线检查
- `GET /api/security/baseline/report` - 获取检查报告
- `GET /api/security/baseline/categories` - 获取检查类别

## 配置示例

### 完整配置

```go
config := security.SecurityConfig{
    Firewall: security.FirewallConfig{
        Enabled:       true,
        DefaultPolicy: "deny",
        IPv6Enabled:   true,
        LogDropped:    true,
    },
    Fail2Ban: security.Fail2BanConfig{
        Enabled:            true,
        MaxAttempts:        5,
        WindowMinutes:      10,
        BanDurationMinutes: 60,
        AutoUnban:          true,
        NotifyOnBan:        true,
    },
    AuditEnabled: true,
    AlertEnabled: true,
}

securityMgr.UpdateConfig(config)
```

### 与现有用户系统集成

```go
// 在 users/handlers.go 的 login 函数中
func (h *Handlers) login(c *gin.Context) {
    var req LoginRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, Error(400, err.Error()))
        return
    }

    // 首先检查 IP 是否被封禁
    if !h.securityMgr.IsAccessAllowed(c.ClientIP()) {
        c.JSON(http.StatusForbidden, Error(403, "IP 地址已被封禁"))
        return
    }

    // 验证用户名和密码
    token, err := h.manager.Authenticate(req.Username, req.Password)
    if err != nil {
        // 记录失败登录
        h.securityMgr.RecordFailedLogin(
            c.ClientIP(),
            req.Username,
            c.UserAgent(),
            "密码错误",
        )
        
        c.JSON(http.StatusUnauthorized, Error(401, "用户名或密码错误"))
        return
    }

    user, _ := h.manager.GetUser(req.Username)

    // 记录成功登录
    h.securityMgr.RecordSuccessfulLogin(
        c.ClientIP(),
        req.Username,
        c.UserAgent(),
        "", // MFA 方法
    )

    // 返回令牌
    c.JSON(http.StatusOK, Success(LoginResponse{
        Token:       token.Token,
        ExpiresAt:   token.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
        User:        user,
    }))
}
```

## Web UI

安全中心的 Web UI 位于 `webui/pages/security.html`。

### 功能
- 安全仪表板（安全评分、状态概览）
- 防火墙规则管理
- IP 黑白名单管理
- 失败登录保护配置
- 安全审计日志查看
- 安全基线检查

### 访问
在浏览器中访问：`http://your-nas-ip/security.html`

## 安全基线检查项

### 认证安全 (AUTH)
- AUTH-001: 密码复杂度检查
- AUTH-002: MFA 启用状态
- AUTH-003: 默认密码检查
- AUTH-004: 账户锁定策略

### 网络安全 (NET)
- NET-001: 防火墙状态
- NET-002: SSH 配置检查
- NET-003: 开放端口检查
- NET-004: HTTPS 强制

### 系统安全 (SYS)
- SYS-001: 系统更新状态
- SYS-002: Root 登录检查
- SYS-003: 文件权限检查
- SYS-004: 日志记录状态

### 文件安全 (FILE)
- FILE-001: 敏感文件加密
- FILE-002: 共享权限检查

## 通知集成

### 配置通知回调

```go
securityMgr.SetNotifyFunc(func(alert security.SecurityAlert) {
    // 发送邮件通知
    sendEmail("admin@example.com", alert.Title, alert.Description)
    
    // 发送 Webhook
    sendWebhook("https://hooks.example.com/security", alert)
    
    // 发送短信（严重告警）
    if alert.Severity == "critical" {
        sendSMS("13800000000", alert.Title)
    }
})
```

## 最佳实践

1. **启用防火墙**: 设置默认策略为 deny，只开放必要的端口
2. **配置失败登录保护**: 建议 5 次失败后封禁 60 分钟
3. **启用 MFA**: 为所有管理员账户启用双因素认证
4. **定期基线检查**: 每周运行一次安全基线检查
5. **监控告警**: 及时处理未确认的安全告警
6. **日志审计**: 保留至少 90 天的审计日志

## 故障排除

### 防火墙规则不生效
- 检查是否以 root 权限运行
- 检查 iptables 是否可用：`iptables -L`
- 查看系统日志：`journalctl -u iptables`

### fail2ban 未工作
- 检查是否安装：`which fail2ban-client`
- 查看状态：`fail2ban-client status`
- 检查配置文件：`/etc/fail2ban/jail.local`

### 审计日志未记录
- 检查配置：`GET /api/security/config`
- 确保审计已启用：`audit_enabled: true`
- 检查日志目录权限

## 更新日志

### v1.0.0 (2026-03-12)
- 初始版本
- 防火墙管理（IPv4/IPv6）
- 失败登录保护
- 安全审计日志
- 安全基线检查
- Web UI 安全中心
