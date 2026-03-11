# 双因素认证 (2FA/MFA) 模块

本模块为 NAS-OS 提供完整的双因素认证支持。

## 功能特性

### 1. TOTP 支持 (Google Authenticator)
- 生成 TOTP 密钥和 QR 码
- 支持 Google Authenticator、Authy 等应用
- 时间窗口容错（前后 30 秒）

### 2. 短信验证码
- 支持阿里云、腾讯云短信服务
- 验证码有效期 5 分钟
- 防暴力破解（最多 3 次尝试）
- 发送频率限制（30 秒内只能发送一次）

### 3. 备份码生成
- 生成 10 个一次性使用的备份码
- 格式：XXXX-XXXX（易读易记）
- 使用后立即失效
- 支持重新生成

### 4. 安全密钥支持 (WebAuthn/FIDO2)
- 支持物理安全密钥（YubiKey 等）
- 支持平台认证器（Touch ID、Windows Hello）
- 无密码登录体验

## API 接口

### 状态查询
```
GET /api/mfa/status
```

### TOTP 设置
```
POST /api/mfa/totp/setup
POST /api/mfa/totp/enable  {"code": "123456"}
POST /api/mfa/totp/disable {"code": "123456"}
```

### 短信验证码
```
POST /api/mfa/sms/send    {"phone": "+8613800138000"}
POST /api/mfa/sms/enable  {"phone": "+8613800138000", "code": "123456"}
POST /api/mfa/sms/disable {"code": "123456"}
```

### 备份码
```
POST /api/mfa/backup/generate
GET  /api/mfa/backup/status
```

### WebAuthn
```
POST /api/mfa/webauthn/register/start   {"display_name": "My Key"}
POST /api/mfa/webauthn/register/finish  (WebAuthn 响应数据)
POST /api/mfa/webauthn/authenticate/start
POST /api/mfa/webauthn/authenticate/finish (WebAuthn 响应数据)
GET  /api/mfa/webauthn/credentials
DELETE /api/mfa/webauthn/credentials/:id
```

### MFA 验证
```
POST /api/mfa/verify
{
  "mfa_type": "totp",  // 或 sms, webauthn
  "code": "123456"
}
```

## 使用流程

### 启用 TOTP
1. 调用 `POST /api/mfa/totp/setup` 获取 QR 码
2. 用户使用 Google Authenticator 扫描 QR 码
3. 调用 `POST /api/mfa/totp/enable` 提交 6 位验证码
4. 启用成功后，建议生成备份码

### 启用短信验证
1. 调用 `POST /api/mfa/sms/send` 发送验证码到手机
2. 用户收到短信后，调用 `POST /api/mfa/sms/enable` 提交验证码
3. 启用成功

### 生成备份码
1. 调用 `POST /api/mfa/backup/generate`
2. **立即保存备份码**（只显示一次）
3. 建议打印或存储在安全的地方

### 启用安全密钥
1. 调用 `POST /api/mfa/webauthn/register/start`
2. 前端使用返回的 options 调用 WebAuthn API
3. 用户触摸安全密钥完成注册
4. 调用 `POST /api/mfa/webauthn/register/finish` 提交响应

### 登录流程
1. 用户输入用户名和密码
2. 如果启用了 MFA，返回 `mfa_required: true` 和 `mfa_type`
3. 根据 mfa_type 提示用户输入验证码或使用安全密钥
4. 再次调用登录接口，带上 `mfa_code` 或 `backup_code`
5. 验证成功，返回访问令牌

## 配置示例

```go
// 创建 MFA 管理器
smsProvider := &auth.AliyunSMSProvider{
    AccessKeyID:     "your-access-key-id",
    AccessKeySecret: "your-access-key-secret",
    SignName:        "您的签名",
    TemplateCode:    "SMS_123456789",
}

mfaMgr, err := auth.NewMFAManager(
    "/path/to/mfa-config.json", // 配置文件路径
    "NAS-OS",                    // 发行者名称
    smsProvider,                 // 短信提供商
)

// 创建处理器并注册路由
mfaHandlers := auth.NewHandlers(mfaMgr)
mfaHandlers.RegisterRoutes(api) // api 是 gin.RouterGroup
```

## 安全建议

1. **强制 MFA**: 对管理员账户强制启用 MFA
2. **备份码**: 生成后备份码必须安全存储
3. **短信限制**: 生产环境配置短信发送频率限制
4. **WebAuthn 配置**: 正确配置 RPID 和 RPOrigins
5. **日志审计**: 记录所有 MFA 相关操作

## 依赖

- `github.com/pquerna/otp` - TOTP 生成和验证
- `github.com/go-webauthn/webauthn` - WebAuthn/FIDO2 支持

## 测试

```bash
cd nas-os
go test ./internal/auth/...
```
