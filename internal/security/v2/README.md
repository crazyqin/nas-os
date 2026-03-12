# Security Module v2 - NAS-OS 安全加固增强版

## 概述

Security Module v2 是 NAS-OS v1.2.0 的安全加固增强模块，在现有安全模块基础上增加了以下核心功能：

### 新增功能

1. **增强的双因素认证 (MFA)**
   - TOTP 时间同步验证码（支持 Google Authenticator、Authy 等）
   - 短信验证码
   - 邮件验证码
   - 恢复码（紧急情况下使用）
   - 多验证方式切换

2. **文件加密系统**
   - AES-256-GCM 加密算法
   - Argon2id 密钥派生
   - 加密目录管理
   - 文件级加密/解密
   - 目录锁定/解锁

3. **告警通知系统**
   - 邮件通知
   - 企业微信机器人
   - 通用 Webhook
   - 告警分级（low/medium/high/critical）
   - 速率限制
   - 免打扰时段

## 目录结构

```
security/v2/
├── mfa.go           # 双因素认证管理
├── encryption.go    # 文件加密管理
├── alerting.go      # 告警通知管理
├── manager.go       # 统一安全管理器
├── handlers.go      # HTTP 处理器
└── README.md        # 本文档
```

## API 端点

### MFA 双因素认证

```
GET    /api/security/v2/mfa/status          # 获取 MFA 状态
POST   /api/security/v2/mfa/setup           # 设置 MFA
POST   /api/security/v2/mfa/enable          # 启用 MFA
POST   /api/security/v2/mfa/disable         # 禁用 MFA
POST   /api/security/v2/mfa/verify          # 验证 MFA 代码
GET    /api/security/v2/mfa/recovery-codes  # 获取恢复码
POST   /api/security/v2/mfa/recovery-codes/regenerate  # 重新生成恢复码
PUT    /api/security/v2/mfa/phone           # 更新手机号
PUT    /api/security/v2/mfa/email           # 更新邮箱
POST   /api/security/v2/mfa/sms-code        # 发送短信验证码
POST   /api/security/v2/mfa/email-code      # 发送邮件验证码
```

### 文件加密

```
GET    /api/security/v2/encryption/status              # 获取加密状态
POST   /api/security/v2/encryption/initialize          # 初始化加密系统
POST   /api/security/v2/encryption/directories         # 创建加密目录
GET    /api/security/v2/encryption/directories         # 获取加密目录列表
POST   /api/security/v2/encryption/directories/:path/unlock   # 解锁目录
POST   /api/security/v2/encryption/directories/:path/lock     # 锁定目录
DELETE /api/security/v2/encryption/directories/:path           # 删除加密目录
POST   /api/security/v2/encryption/files/encrypt        # 加密文件
POST   /api/security/v2/encryption/files/decrypt        # 解密文件
```

### 告警通知

```
GET    /api/security/v2/alerting/config         # 获取告警配置
PUT    /api/security/v2/alerting/config         # 更新告警配置
GET    /api/security/v2/alerting/alerts         # 获取告警列表
POST   /api/security/v2/alerting/alerts/:id/acknowledge  # 确认告警
GET    /api/security/v2/alerting/stats          # 获取告警统计
GET    /api/security/v2/alerting/subscribers    # 获取订阅者列表
POST   /api/security/v2/alerting/subscribers    # 添加订阅者
DELETE /api/security/v2/alerting/subscribers/:id # 删除订阅者
POST   /api/security/v2/alerting/test/:channel  # 测试通知渠道
```

## 使用示例

### 1. 初始化加密系统

```go
import "nas-os/internal/security/v2"

// 创建安全管理器
securityMgr := securityv2.NewSecurityManagerV2()

// 初始化加密系统（使用主密码）
err := securityMgr.Initialize("your-master-password")
if err != nil {
    log.Fatal(err)
}
```

### 2. 为用户设置 MFA

```go
mfaMgr := securityMgr.GetMFAManager()

// 设置 MFA
secret, err := mfaMgr.SetupMFA(userID, username, phone, email)
if err != nil {
    return err
}

// 显示 TOTP 密钥和恢复码给用户
fmt.Println("TOTP Secret:", secret.TOTPSecret)
fmt.Println("Recovery Codes:", secret.RecoveryCodes)

// 用户验证后启用 MFA
err = mfaMgr.EnableMFA(userID, "totp")
```

### 3. 创建加密目录

```go
encMgr := securityMgr.GetEncryptionManager()

// 创建加密目录
dir, err := encMgr.CreateEncryptedDirectory(
    "/data/encrypted/finance",
    "财务数据",
    "存储公司财务相关文件",
)
if err != nil {
    return err
}

// 解锁目录以访问文件
err = encMgr.UnlockDirectory(dir.Path, "directory-password")
```

### 4. 发送安全告警

```go
alertingMgr := securityMgr.GetAlertingManager()

// 配置通知渠道
alertingMgr.SetSendEmailFunc(sendEmail)
alertingMgr.SetSendWebhookFunc(sendWebhook)

// 发送告警
err := securityMgr.SendSecurityAlert(
    "high",                              // 级别
    "login_failure_multiple",            // 类型
    "多次登录失败",                        // 标题
    "IP 192.168.1.100 连续失败 5 次登录",  // 描述
    "192.168.1.100",                     // 来源 IP
    "admin",                             // 用户名
    map[string]interface{}{              // 详情
        "attempts": 5,
        "window_minutes": 10,
    },
)
```

### 5. 配置告警通知

```go
config := AlertingConfig{
    Enabled:         true,
    EmailEnabled:    true,
    EmailRecipients: []string{"admin@example.com", "security@example.com"},
    WeComEnabled:    true,
    WeComWebhook:    "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx",
    WebhookEnabled:  true,
    WebhookURLs:     []string{"https://example.com/webhook"},
    MinSeverity:     "medium",
    RateLimit:       10,
    QuietHours: QuietHours{
        Enabled:   true,
        StartTime: "23:00",
        EndTime:   "07:00",
    },
}

alertingMgr.UpdateConfig(config)
```

## 安全最佳实践

### MFA

1. **强制管理员启用 MFA**：所有管理员账户必须启用双因素认证
2. **恢复码安全存储**：恢复码只能查看一次，应打印或保存到安全位置
3. **定期更换恢复码**：建议每 6 个月重新生成恢复码
4. **多验证方式备份**：同时配置 TOTP 和邮件/短信验证

### 文件加密

1. **强主密码**：使用至少 16 位的复杂密码作为主密码
2. **定期备份密钥**：备份主密钥文件到安全位置
3. **最小权限原则**：仅加密真正敏感的文件
4. **及时锁定目录**：使用完毕后立即锁定加密目录

### 告警通知

1. **多渠道通知**：配置至少两个通知渠道（如邮件 + 企业微信）
2. **合理告警级别**：避免过多低级别告警导致告警疲劳
3. **设置免打扰时段**：非紧急告警在夜间延迟发送
4. **定期测试**：每月测试一次通知渠道是否正常工作

## 与现有模块集成

### 注册路由

在 main.go 或路由注册处添加：

```go
import "nas-os/internal/security/v2"

// 创建安全管理器 v2
securityMgrV2 := securityv2.NewSecurityManagerV2()

// 创建处理器并注册路由
handlersV2 := securityv2.NewHandlersV2(securityMgrV2)
handlersV2.RegisterRoutes(api)  // api 是 gin.RouterGroup
```

### 通知回调集成

```go
// 设置邮件发送函数
securityMgrV2.GetAlertingManager().SetSendEmailFunc(
    func(to, subject, body string) error {
        return notify.SendEmail(to, subject, body)
    },
)

// 设置 Webhook 发送函数
securityMgrV2.GetAlertingManager().SetSendWebhookFunc(
    func(url string, payload map[string]interface{}) error {
        return notify.SendWebhook(url, payload)
    },
)
```

### 与 Fail2Ban 集成

```go
// 在 Fail2Ban 封禁 IP 时发送告警
func (f2m *Fail2BanManager) banIPLocked(ip, username string, attempts int) error {
    // ... 封禁逻辑 ...
    
    // 发送安全告警
    if notifyFunc != nil {
        alert := SecurityAlertV2{
            ID:          generateAlertIDV2(),
            Severity:    "high",
            Type:        "ip_banned",
            Title:       "IP 地址已被封禁",
            Description: fmt.Sprintf("IP %s 因失败登录尝试次数过多 (%d 次) 已被封禁", ip, attempts),
            SourceIP:    ip,
            Username:    username,
        }
        notifyFunc(alert)
    }
}
```

## 性能考虑

1. **异步通知**：所有通知发送都是异步的，不会阻塞主流程
2. **速率限制**：默认每分钟最多 10 条告警，防止告警风暴
3. **内存管理**：告警列表限制在 1000 条以内，自动清理旧告警
4. **加密性能**：AES-256-GCM 加密速度快，适合大文件

## 故障排除

### MFA 验证码无效

- 检查服务器时间是否同步（TOTP 依赖准确时间）
- 确认用户输入的验证码是最新的（30 秒过期）
- 尝试使用恢复码登录

### 加密目录无法解锁

- 确认主密码正确
- 检查密钥文件是否存在且未损坏
- 查看系统日志获取详细错误信息

### 告警通知未发送

- 检查通知渠道配置是否正确
- 确认邮件服务器/Webhook 地址可访问
- 查看是否触发速率限制
- 检查是否在免打扰时段

## 版本历史

### v2.0.0 (2026-03-12)

- ✨ 新增 MFA 增强功能（TOTP/短信/邮件/恢复码）
- ✨ 新增文件加密系统（AES-256-GCM）
- ✨ 新增告警通知系统（邮件/企业微信/Webhook）
- 🎨 优化安全中心 UI
- 📝 完善 API 文档

## 参考资料

- [TOTP 算法规范 (RFC 6238)](https://tools.ietf.org/html/rfc6238)
- [Argon2 密钥派生](https://github.com/P-H-C/phc-winner-argon2)
- [AES-GCM 加密模式](https://csrc.nist.gov/publications/detail/sp/800-38d/final)
- [企业微信机器人 API](https://developer.work.weixin.qq.com/document/path/91770)
