package auth

import "time"

// MFAConfig 双因素认证配置
type MFAConfig struct {
	UserID          string    `json:"user_id"`
	Enabled         bool      `json:"enabled"`
	TOTPEnabled     bool      `json:"totp_enabled"`
	TOTPSecret      string    `json:"totp_secret,omitempty"` // 加密存储
	SMSEnabled      bool      `json:"sms_enabled"`
	Phone           string    `json:"phone,omitempty"`
	BackupCodes     []string  `json:"backup_codes,omitempty"` // 加密存储
	WebAuthnEnabled bool      `json:"webauthn_enabled"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// TOTPSetup TOTP 设置信息
type TOTPSetup struct {
	Secret      string `json:"secret"`
	URI         string `json:"uri"`
	QRCode      string `json:"qr_code"` // base64 编码的 QR 码图片
	Issuer      string `json:"issuer"`
	AccountName string `json:"account_name"`
}

// SMSCode 短信验证码
type SMSCode struct {
	Phone     string    `json:"phone"`
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
	Attempts  int       `json:"attempts"`
}

// BackupCode 备份码
type BackupCode struct {
	Code   string     `json:"code"`
	Used   bool       `json:"used"`
	UsedAt *time.Time `json:"used_at,omitempty"`
}

// WebAuthnCredential WebAuthn 凭据
type WebAuthnCredential struct {
	ID              string     `json:"id"`
	PublicKey       []byte     `json:"public_key"`
	AttestationType string     `json:"attestation_type"`
	Transport       []string   `json:"transport"`
	CreatedAt       time.Time  `json:"created_at"`
	LastUsedAt      *time.Time `json:"last_used_at,omitempty"`
}

// WebAuthnSetup WebAuthn 设置信息
type WebAuthnSetup struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

// MFALoginRequest 双因素登录请求
type MFALoginRequest struct {
	Username                string      `json:"username" binding:"required"`
	Password                string      `json:"password" binding:"required"`
	MFACode                 string      `json:"mfa_code,omitempty"`             // TOTP 或短信验证码
	BackupCode              string      `json:"backup_code,omitempty"`          // 备份码
	CredentialAssertionData interface{} `json:"credential_assertion,omitempty"` // WebAuthn 认证数据
}

// MFALoginResponse 双因素登录响应
type MFALoginResponse struct {
	Token       string `json:"token,omitempty"`
	ExpiresAt   string `json:"expires_at,omitempty"`
	MFARequired bool   `json:"mfa_required"`
	MFAType     string `json:"mfa_type,omitempty"`   // totp, sms, webauthn
	SessionID   string `json:"session_id,omitempty"` // 临时会话 ID，用于 MFA 验证
	User        *User  `json:"user,omitempty"`
}

// SMSRequest 短信验证码请求
type SMSRequest struct {
	Phone string `json:"phone" binding:"required"`
}

// SMSVerifyRequest 短信验证码验证请求
type SMSVerifyRequest struct {
	Phone string `json:"phone" binding:"required"`
	Code  string `json:"code" binding:"required"`
}

// BackupCodesResponse 备份码响应
type BackupCodesResponse struct {
	Codes []string `json:"codes"`
}

// 错误定义
var (
	ErrMFANotEnabled      = "双因素认证未启用"
	ErrMFAInvalidCode     = "验证码无效或已过期"
	ErrMFATooManyAttempts = "尝试次数过多，请稍后再试"
	ErrBackupCodeUsed     = "备份码已使用"
	ErrBackupCodeInvalid  = "备份码无效"
)

// User 用户信息（从 users 包复制，避免循环依赖）
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email,omitempty"`
	Role     string `json:"role"`
}
