# Security Module v2 集成指南

## 快速开始

### 1. 在 main.go 中初始化

```go
package main

import (
    "github.com/gin-gonic/gin"
    "nas-os/internal/security/v2"
)

func main() {
    // 创建 Gin 路由
    r := gin.Default()
    api := r.Group("/api")
    
    // ========== 初始化安全模块 v2 ==========
    securityMgrV2 := securityv2.NewSecurityManagerV2()
    
    // 可选：初始化加密系统（需要主密码）
    // 主密码应从环境变量或安全存储中获取
    masterPassword := os.Getenv("NAS_OS_MASTER_PASSWORD")
    if masterPassword != "" {
        if err := securityMgrV2.Initialize(masterPassword); err != nil {
            log.Printf("警告：加密系统初始化失败：%v", err)
        }
    }
    
    // 设置通知回调函数
    setupSecurityNotifications(securityMgrV2)
    
    // 注册安全模块路由
    handlersV2 := securityv2.NewHandlersV2(securityMgrV2)
    handlersV2.RegisterRoutes(api)
    
    // 启动服务器
    r.Run(":8080")
}

// setupSecurityNotifications 配置安全通知
func setupSecurityNotifications(mgr *securityv2.SecurityManagerV2) {
    alertingMgr := mgr.GetAlertingManager()
    
    // 设置邮件发送函数
    alertingMgr.SetSendEmailFunc(func(to, subject, body string) error {
        return sendEmail(to, subject, body)
    })
    
    // 设置 Webhook 发送函数
    alertingMgr.SetSendWebhookFunc(func(url string, payload map[string]interface{}) error {
        return sendWebhook(url, payload)
    })
    
    // 设置短信发送函数
    mgr.GetMFAManager().SetSendSMSFunc(func(to, message string) error {
        return sendSMS(to, message)
    })
}
```

### 2. 与现有认证系统集成

在登录处理器中集成 MFA 验证：

```go
package auth

import (
    "nas-os/internal/security/v2"
)

type LoginHandler struct {
    securityMgr *securityv2.SecurityManagerV2
}

func (h *LoginHandler) Login(c *gin.Context) {
    var req LoginRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // 1. 验证用户名密码
    user, err := validateCredentials(req.Username, req.Password)
    if err != nil {
        // 记录失败登录
        h.securityMgr.GetMFAManager().RecordFailedLogin(
            c.ClientIP(),
            req.Username,
            req.Password,
        )
        
        c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
        return
    }
    
    // 2. 检查是否需要 MFA
    mfaStatus, err := h.securityMgr.GetMFAManager().GetMFAStatus(user.ID)
    if err == nil && mfaStatus["enabled"].(bool) {
        // 需要 MFA 验证，返回临时令牌
        tempToken := generateTempToken(user.ID)
        c.JSON(http.StatusOK, gin.H{
            "requires_mfa": true,
            "temporary_token": tempToken,
            "mfa_methods": []string{"totp", "sms", "email"},
        })
        return
    }
    
    // 3. 登录成功，生成正式令牌
    token := generateJWT(user)
    c.JSON(http.StatusOK, gin.H{"token": token})
}

func (h *LoginHandler) VerifyMFA(c *gin.Context) {
    var req VerifyMFARequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // 验证临时令牌
    userID := validateTempToken(req.TemporaryToken)
    if userID == "" {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "临时令牌无效"})
        return
    }
    
    // 验证 MFA 代码
    result, err := h.securityMgr.GetMFAManager().VerifyMFA(
        userID,
        req.Code,
        req.Method,
    )
    if err != nil || !result.Success {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "MFA 验证失败"})
        return
    }
    
    // MFA 验证通过，生成正式令牌
    user := getUserByID(userID)
    token := generateJWT(user)
    c.JSON(http.StatusOK, gin.H{"token": token})
}
```

### 3. 与 Fail2Ban 集成

在现有的 Fail2Ban 模块中集成告警：

```go
package security

import "nas-os/internal/security/v2"

type Fail2BanManager struct {
    // ... 现有字段 ...
    alertingMgr *securityv2.AlertingManager
}

func NewFail2BanManager(alertingMgr *securityv2.AlertingManager) *Fail2BanManager {
    return &Fail2BanManager{
        // ... 初始化现有字段 ...
        alertingMgr: alertingMgr,
    }
}

func (f2m *Fail2BanManager) banIPLocked(ip, username string, attempts int) error {
    // ... 现有封禁逻辑 ...
    
    // 发送安全告警
    if f2m.alertingMgr != nil {
        alert := securityv2.SecurityAlertV2{
            ID:          generateAlertIDV2(),
            Timestamp:   time.Now(),
            Severity:    "high",
            Type:        "ip_banned",
            Title:       "IP 地址已被封禁",
            Description: fmt.Sprintf("IP %s 因失败登录尝试次数过多 (%d 次) 已被封禁", ip, attempts),
            SourceIP:    ip,
            Username:    username,
            Details: map[string]interface{}{
                "attempts":             attempts,
                "ban_duration_minutes": f2m.config.BanDurationMinutes,
            },
        }
        f2m.alertingMgr.SendAlert(alert)
    }
    
    return nil
}
```

### 4. 与安全审计集成

在审计日志中记录 v2 模块的操作：

```go
package security

import "nas-os/internal/security/v2"

type AuditManager struct {
    // ... 现有字段 ...
    securityMgrV2 *securityv2.SecurityManagerV2
}

func (am *AuditManager) LogMFACreated(userID, username, method string) {
    am.Log(AuditLogEntry{
        Level:    "info",
        Category: "auth",
        Event:    "mfa_created",
        UserID:   userID,
        Username: username,
        Details: map[string]interface{}{
            "method": method,
        },
        Status: "success",
    })
}

func (am *AuditManager) LogEncryptedDirectoryCreated(userID, username, path, name string) {
    am.Log(AuditLogEntry{
        Level:    "info",
        Category: "encryption",
        Event:    "encrypted_directory_created",
        UserID:   userID,
        Username: username,
        Resource: path,
        Details: map[string]interface{}{
            "name": name,
        },
        Status: "success",
    })
}

func (am *AuditManager) LogSecurityAlertSent(alertID, severity, alertType string) {
    am.Log(AuditLogEntry{
        Level:    "info",
        Category: "alerting",
        Event:    "security_alert_sent",
        Details: map[string]interface{}{
            "alert_id":  alertID,
            "severity":  severity,
            "alert_type": alertType,
        },
        Status: "success",
    })
}
```

### 5. Web UI 集成

在现有 Web UI 中添加安全中心入口：

```html
<!-- 在导航栏或设置页面中添加链接 -->
<a href="/pages/security-v2.html" class="nav-item">
    <span class="icon">🔒</span>
    <span>安全中心</span>
</a>
```

### 6. 配置文件集成

在配置文件中添加 v2 模块配置：

```yaml
# config/security-v2.yaml
security_v2:
  mfa:
    enabled: true
    required_for:
      - admin
    totp_enabled: true
    sms_enabled: false
    email_enabled: true
    recovery_codes: 10
    code_length: 6
    validity_period: 30
  
  encryption:
    enabled: true
    algorithm: aes-256-gcm
    key_derivation: argon2id
    master_key_path: /var/lib/nas-os/security/master.key
    salt_path: /var/lib/nas-os/security/salt
  
  alerting:
    enabled: true
    email_enabled: false
    email_recipients: []
    wecom_enabled: false
    wecom_webhook: ""
    webhook_enabled: false
    webhook_urls: []
    min_severity: medium
    rate_limit: 10
    quiet_hours:
      enabled: false
      start_time: "23:00"
      end_time: "07:00"
```

加载配置：

```go
func LoadSecurityV2Config(configPath string) (*securityv2.SecurityConfigV2, error) {
    data, err := os.ReadFile(configPath)
    if err != nil {
        return nil, err
    }
    
    var config securityv2.SecurityConfigV2
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, err
    }
    
    return &config, nil
}

// 使用配置
config, err := LoadSecurityV2Config("/etc/nas-os/security-v2.yaml")
if err != nil {
    log.Printf("加载安全配置失败：%v", err)
} else {
    securityMgrV2.UpdateConfig(*config)
}
```

### 7. 数据库集成（可选）

如果需要持久化 MFA 和加密配置：

```go
package database

import (
    "gorm.io/gorm"
    "nas-os/internal/security/v2"
)

type MFASecret struct {
    ID              uint      `gorm:"primaryKey"`
    UserID          string    `gorm:"uniqueIndex"`
    TOTPSecret      string
    SMSPhone        string
    Email           string
    EncryptedRecoveryCodes string // JSON 格式存储加密的恢复码
    Enabled         bool
    PreferredMethod string
    CreatedAt       time.Time
    LastUsed        *time.Time
}

type EncryptedDirectory struct {
    ID          uint      `gorm:"primaryKey"`
    Path        string    `gorm:"uniqueIndex"`
    Name        string
    Description string
    EncryptedKey []byte   // 加密的目录密钥
    Status      string    // locked/unlocked
    CreatedAt   time.Time
}

// 从数据库加载 MFA 配置
func LoadMFASecrets(db *gorm.DB, mfaMgr *securityv2.MFAManager) error {
    var secrets []MFASecret
    if err := db.Find(&secrets).Error; err != nil {
        return err
    }
    
    // 将数据库中的配置加载到 MFA 管理器
    // （需要扩展 MFA 管理器支持从外部加载）
    
    return nil
}

// 保存 MFA 配置到数据库
func SaveMFASecret(db *gorm.DB, secret *securityv2.MFASecret) error {
    model := MFASecret{
        UserID:          secret.UserID,
        TOTPSecret:      secret.TOTPSecret,
        SMSPhone:        secret.SMSPhone,
        Email:           secret.Email,
        Enabled:         secret.Enabled,
        PreferredMethod: secret.PreferredMethod,
        CreatedAt:       secret.CreatedAt,
        LastUsed:        secret.LastUsed,
    }
    
    return db.Save(&model).Error
}
```

## 环境变量

```bash
# 主密码（用于加密系统）
NAS_OS_MASTER_PASSWORD=your-secure-password

# 邮件服务器配置
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USER=noreply@example.com
SMTP_PASSWORD=smtp-password

# 短信服务商配置
SMS_SERVICE=aliyun
SMS_ACCESS_KEY=your-access-key
SMS_ACCESS_SECRET=your-access-secret

# 企业微信 Webhook
WECOM_WEBHOOK_URL=https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx
```

## 测试

### 单元测试

```go
package securityv2_test

import (
    "testing"
    "nas-os/internal/security/v2"
)

func TestMFAManager(t *testing.T) {
    mgr := securityv2.NewMFAManager()
    
    // 测试 TOTP 生成和验证
    secret := mgr.GenerateTOTPSecret()
    code, err := mgr.GenerateTOTPCode(secret, time.Now())
    if err != nil {
        t.Fatal(err)
    }
    
    if !mgr.VerifyTOTPCode(secret, code) {
        t.Error("TOTP 验证失败")
    }
}

func TestEncryptionManager(t *testing.T) {
    mgr := securityv2.NewEncryptionManager()
    
    // 初始化
    if err := mgr.Initialize("test-password"); err != nil {
        t.Fatal(err)
    }
    
    // 测试加密解密
    plaintext := []byte("Hello, World!")
    ciphertext, err := mgr.EncryptData(plaintext)
    if err != nil {
        t.Fatal(err)
    }
    
    decrypted, err := mgr.DecryptData(ciphertext)
    if err != nil {
        t.Fatal(err)
    }
    
    if string(decrypted) != string(plaintext) {
        t.Error("解密结果不匹配")
    }
}

func TestAlertingManager(t *testing.T) {
    mgr := securityv2.NewAlertingManager()
    
    // 测试告警发送
    alert := securityv2.SecurityAlertV2{
        Severity:    "high",
        Type:        "test",
        Title:       "测试告警",
        Description: "这是一条测试告警",
    }
    
    if err := mgr.SendAlert(alert); err != nil {
        t.Fatal(err)
    }
}
```

## 迁移指南

### 从 v1 迁移到 v2

1. **备份现有配置**
   ```bash
   cp /var/lib/nas-os/security/* /backup/security-backup/
   ```

2. **安装 v2 模块**
   ```bash
   # v2 模块与 v1 并存，不会冲突
   ```

3. **逐步启用功能**
   - 先启用 MFA（不影响现有用户）
   - 再配置告警通知
   - 最后使用文件加密

4. **测试验证**
   - 测试登录流程
   - 测试告警通知
   - 测试加密解密

## 故障排除

### 常见问题

1. **MFA 验证码总是错误**
   - 检查服务器时间是否同步：`timedatectl status`
   - 确认客户端应用时间正确

2. **加密目录无法访问**
   - 确认主密码正确
   - 检查密钥文件权限：`ls -la /var/lib/nas-os/security/`

3. **告警通知不发送**
   - 检查通知渠道配置
   - 查看日志：`journalctl -u nas-os -f`

### 日志位置

```
/var/log/nas-os/security.log
/var/log/nas-os/audit.log
/var/log/nas-os/encryption.log
```

## 性能优化建议

1. **使用缓存**：将 MFA 验证结果缓存 5 分钟
2. **批量发送告警**：将相同类型的告警合并发送
3. **异步处理**：加密/解密操作使用后台 goroutine
4. **定期清理**：清理过期的审计日志和告警

## 安全建议

1. **定期更新主密码**：每 6 个月更换一次
2. **备份密钥**：将主密钥备份到离线存储
3. **监控异常**：设置告警阈值，及时发现异常
4. **审计日志**：定期审查安全审计日志
