// Package fido2 提供 FIDO2/WebAuthn 硬件密钥认证功能，支持 YubiKey、TouchID、Windows Hello 等设备。
package fido2

import (
	"time"
)

// ==================== 凭据相关 ====================

// Credential WebAuthn 凭据
type Credential struct {
	ID              string    `json:"id"`               // 凭据唯一标识
	UserID          string    `json:"user_id"`          // 所属用户 ID
	Name            string    `json:"name"`             // 凭据名称（如 "YubiKey 5"）
	PublicKey        []byte    `json:"public_key"`       // 公钥数据
	PublicKeyAlg     int       `json:"public_key_alg"`   // 公钥算法（ES256 = -7）
	SignCount       uint32    `json:"sign_count"`       // 签名计数器（防重放）
	AAGUID          []byte    `json:"aaguid"`           // 认证器全局唯一标识
	CredentialID    []byte    `json:"credential_id"`    // WebAuthn 凭据 ID
	AttestationType string    `json:"attestation_type"` // 证明类型（packed, none 等）
	Transports      []string  `json:"transports"`       // 支持的传输方式（usb, nfc, ble, internal）
	Authenticator   string    `json:"authenticator"`    // 认证器类型描述
	CreatedAt       time.Time `json:"created_at"`       // 创建时间
	LastUsedAt      time.Time `json:"last_used_at"`     // 最后使用时间
	UsageCount      int64     `json:"usage_count"`      // 使用次数
	Revoked         bool      `json:"revoked"`          // 是否已吊销
	RevokedAt       *time.Time `json:"revoked_at,omitempty"` // 吊销时间
}

// CredentialInfo 凭据简要信息（用于列表展示）
type CredentialInfo struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Authenticator string    `json:"authenticator"`
	Transports    []string  `json:"transports"`
	CreatedAt     time.Time `json:"created_at"`
	LastUsedAt    time.Time `json:"last_used_at"`
	UsageCount    int64     `json:"usage_count"`
	Revoked       bool      `json:"revoked"`
}

// ==================== 注册相关 ====================

// RegistrationChallenge 注册挑战
type RegistrationChallenge struct {
	Challenge        string   `json:"challenge"`          // Base64 编码的挑战值
	RP               RelyingParty `json:"rp"`              // 依赖方信息
	User             WebAuthnUser `json:"user"`            // 用户信息
	PubKeyCredParams []PubKeyCredParam `json:"pub_key_cred_params"` // 支持的公钥类型
	Timeout          int      `json:"timeout"`            // 超时时间（毫秒）
	ExcludeCredentials []PublicKeyCredentialDescriptor `json:"exclude_credentials,omitempty"` // 排除的凭据
	AuthenticatorSelection AuthenticatorSelection `json:"authenticator_selection"` // 认证器选择
	Attestation      string   `json:"attestation"`        // 证明要求（none, indirect, direct）
}

// RelyingParty 依赖方
type RelyingParty struct {
	ID   string `json:"id"`   // 依赖方 ID（域名）
	Name string `json:"name"` // 依赖方名称
}

// WebAuthnUser WebAuthn 用户信息
type WebAuthnUser struct {
	ID          string `json:"id"`          // 用户唯一标识（Base64）
	Name        string `json:"name"`        // 用户名
	DisplayName string `json:"display_name"` // 显示名称
}

// PubKeyCredParam 公钥凭据参数
type PubKeyCredParam struct {
	Type string `json:"type"` // "public-key"
	Alg  int    `json:"alg"`  // 算法标识（-7 = ES256, -257 = RS256）
}

// PublicKeyCredentialDescriptor 公钥凭据描述符
type PublicKeyCredentialDescriptor struct {
	Type       string   `json:"type"`       // "public-key"
	ID         string   `json:"id"`         // 凭据 ID（Base64）
	Transports []string `json:"transports"` // 传输方式
}

// AuthenticatorSelection 认证器选择条件
type AuthenticatorSelection struct {
	AuthenticatorAttachment string `json:"authenticator_attachment,omitempty"` // platform, cross-platform
	ResidentKey             string `json:"resident_key"`                      // required, preferred, discouraged
	UserVerification        string `json:"user_verification"`                 // required, preferred, discouraged
}

// RegistrationResponse 注册响应（客户端返回）
type RegistrationResponse struct {
	ID       string `json:"id"`        // 凭据 ID（Base64）
	RawID    string `json:"raw_id"`    // 原始凭据 ID（Base64）
	Type     string `json:"type"`      // "public-key"
	Response RegistrationResponseData `json:"response"` // 注册响应数据
	ClientData ClientData `json:"client_data"` // 客户端数据（内部解析）
}

// RegistrationResponseData 注册响应数据
type RegistrationResponseData struct {
	AttestationObject string `json:"attestation_object"` // 证明对象（Base64）
	ClientDataJSON    string `json:"client_data_json"`   // 客户端数据 JSON（Base64）
}

// AttestationObject 证明对象（解析后）
type AttestationObject struct {
	Fmt      string                 `json:"fmt"`      // 证明格式
	AuthData []byte                 `json:"auth_data"` // 认证数据
	AttStmt  map[string]interface{} `json:"att_stmt"`  // 证明语句
}

// ==================== 认证相关 ====================

// AuthenticationChallenge 认证挑战
type AuthenticationChallenge struct {
	Challenge        string                          `json:"challenge"`          // Base64 编码的挑战值
	Timeout          int                             `json:"timeout"`            // 超时时间（毫秒）
	RPID             string                          `json:"rpId"`              // 依赖方 ID
	AllowCredentials []PublicKeyCredentialDescriptor  `json:"allow_credentials"` // 允许的凭据
	UserVerification string                          `json:"user_verification"` // 用户验证要求
}

// AuthenticationResponse 认证响应（客户端返回）
type AuthenticationResponse struct {
	ID       string `json:"id"`        // 凭据 ID（Base64）
	RawID    string `json:"raw_id"`    // 原始凭据 ID（Base64）
	Type     string `json:"type"`      // "public-key"
	Response AuthenticationResponseData `json:"response"` // 认证响应数据
}

// AuthenticationResponseData 认证响应数据
type AuthenticationResponseData struct {
	AuthenticatorData string `json:"authenticator_data"` // 认证器数据（Base64）
	ClientDataJSON    string `json:"client_data_json"`   // 客户端数据 JSON（Base64）
	Signature         string `json:"signature"`          // 签名（Base64）
	UserHandle        string `json:"user_handle"`        // 用户句柄（Base64，可选）
}

// AuthenticatorData 认证器数据（解析后）
type AuthenticatorData struct {
	RPIDHash []byte `json:"rp_id_hash"` // 依赖方 ID 哈希（32字节）
	Flags    byte   `json:"flags"`      // 标志位
	SignCount uint32 `json:"sign_count"` // 签名计数器
}

// AuthenticatorDataFlags 认证器数据标志位
type AuthenticatorDataFlags struct {
	UserPresent       bool `json:"user_present"`       // 用户在场（UP）
	UserVerified      bool `json:"user_verified"`      // 用户已验证（UV）
	BackupEligible    bool `json:"backup_eligible"`    // 备份资格（BE）
	BackupState       bool `json:"backup_state"`       // 备份状态（BS）
	AttestedData      bool `json:"attested_data"`      // 包含证明数据（AT）
	ExtensionData     bool `json:"extension_data"`     // 包含扩展数据（ED）
}

// ==================== 会话相关 ====================

// Session FIDO2 认证会话
type Session struct {
	ID           string    `json:"id"`            // 会话 ID
	UserID       string    `json:"user_id"`       // 用户 ID
	CredentialID string    `json:"credential_id"` // 使用的凭据 ID
	Challenge    string    `json:"challenge"`     // 挑战值
	SignCount    uint32    `json:"sign_count"`    // 签名计数器
	CreatedAt    time.Time `json:"created_at"`    // 创建时间
	ExpiresAt    time.Time `json:"expires_at"`    // 过期时间
	Verified     bool      `json:"verified"`      // 是否已验证
	IPAddress    string    `json:"ip_address"`    // 客户端 IP
	UserAgent    string    `json:"user_agent"`    // 客户端 User-Agent
}

// ==================== 恢复码相关 ====================

// RecoveryCode 恢复码
type RecoveryCode struct {
	ID        string     `json:"id"`         // 恢复码 ID
	UserID    string     `json:"user_id"`    // 用户 ID
	Code      string     `json:"code"`       // 恢复码（哈希存储）
	Used      bool       `json:"used"`       // 是否已使用
	UsedAt    *time.Time `json:"used_at,omitempty"` // 使用时间
	CreatedAt time.Time  `json:"created_at"` // 创建时间
	ExpiresAt time.Time  `json:"expires_at"` // 过期时间
}

// RecoveryCodeInfo 恢复码信息（用于展示）
type RecoveryCodeInfo struct {
	ID        string     `json:"id"`
	Used      bool       `json:"used"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `json:"expires_at"`
}

// ==================== 客户端数据 ====================

// ClientData 客户端数据
type ClientData struct {
	Type        string `json:"type"`        // 操作类型（webauthn.create 或 webauthn.get）
	Challenge   string `json:"challenge"`   // 挑战值（Base64）
	Origin      string `json:"origin"`      // 来源
	CrossOrigin bool   `json:"crossOrigin"` // 是否跨域
}

// ==================== 内部类型 ====================

// pendingRegistration 待处理的注册请求
type pendingRegistration struct {
	Challenge   string    `json:"challenge"`
	UserID      string    `json:"user_id"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	CredOptions []PubKeyCredParam `json:"cred_options"`
}

// pendingAuthentication 待处理的认证请求
type pendingAuthentication struct {
	Challenge    string    `json:"challenge"`
	UserID       string    `json:"user_id"`
	CredentialID string    `json:"credential_id"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// ==================== 配置 ====================

// Config FIDO2 配置
type Config struct {
	RPID          string `json:"rp_id"`          // 依赖方 ID（域名）
	RPName        string `json:"rp_name"`        // 依赖方名称
	RPOrigin      string `json:"rp_origin"`      // 依赖方来源 URL
	ChallengeLen  int    `json:"challenge_len"`  // 挑战值长度（字节）
	Timeout       int    `json:"timeout"`        // 超时时间（毫秒）
	RecoveryCodeLen int  `json:"recovery_code_len"` // 恢复码长度
	MaxCredentials int   `json:"max_credentials"` // 每用户最大凭据数
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		RPID:            "localhost",
		RPName:          "NAS-OS",
		RPOrigin:        "http://localhost:8080",
		ChallengeLen:    32,
		Timeout:         60000,
		RecoveryCodeLen: 10,
		MaxCredentials:  10,
	}
}

// ==================== 内部辅助函数 ====================
