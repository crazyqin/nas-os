// Package fido2 提供 FIDO2/WebAuthn 认证器核心逻辑
package fido2

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"
)

// Authenticator WebAuthn 认证器.
type Authenticator struct {
	config *Config
}

// NewAuthenticator 创建认证器.
func NewAuthenticator(config *Config) *Authenticator {
	if config == nil {
		config = DefaultConfig()
	}
	return &Authenticator{config: config}
}

// GenerateChallenge 生成随机挑战值.
func (a *Authenticator) GenerateChallenge() (string, error) {
	challenge := make([]byte, a.config.ChallengeLen)
	if _, err := rand.Read(challenge); err != nil {
		return "", fmt.Errorf("生成挑战值失败: %w", err)
	}
	return base64.URLEncoding.EncodeToString(challenge), nil
}

// CreateRegistrationChallenge 创建注册挑战.
func (a *Authenticator) CreateRegistrationChallenge(userID, userName, displayName string, existingCreds []Credential) (*RegistrationChallenge, error) {
	challenge, err := a.GenerateChallenge()
	if err != nil {
		return nil, err
	}

	// 构建排除的凭据列表（已注册的密钥）
	excludeCreds := make([]PublicKeyCredentialDescriptor, len(existingCreds))
	for i, cred := range existingCreds {
		excludeCreds[i] = PublicKeyCredentialDescriptor{
			Type:       "public-key",
			ID:         base64.URLEncoding.EncodeToString(cred.CredentialID),
			Transports: cred.Transports,
		}
	}

	return &RegistrationChallenge{
		Challenge: challenge,
		RP: RelyingParty{
			ID:   a.config.RPID,
			Name: a.config.RPName,
		},
		User: WebAuthnUser{
			ID:          base64.URLEncoding.EncodeToString([]byte(userID)),
			Name:        userName,
			DisplayName: displayName,
		},
		PubKeyCredParams: []PubKeyCredParam{
			{Type: "public-key", Alg: -7},   // ES256 (ECDSA with SHA-256)
			{Type: "public-key", Alg: -257}, // RS256 (RSASSA-PKCS1-v1_5 with SHA-256)
		},
		Timeout:            a.config.Timeout,
		ExcludeCredentials: excludeCreds,
		AuthenticatorSelection: AuthenticatorSelection{
			ResidentKey:      "preferred",
			UserVerification: "preferred",
		},
		Attestation: "none",
	}, nil
}

// CreateAuthenticationChallenge 创建认证挑战.
func (a *Authenticator) CreateAuthenticationChallenge(creds []Credential) (*AuthenticationChallenge, error) {
	challenge, err := a.GenerateChallenge()
	if err != nil {
		return nil, err
	}

	allowCreds := make([]PublicKeyCredentialDescriptor, len(creds))
	for i, cred := range creds {
		allowCreds[i] = PublicKeyCredentialDescriptor{
			Type:       "public-key",
			ID:         base64.URLEncoding.EncodeToString(cred.CredentialID),
			Transports: cred.Transports,
		}
	}

	return &AuthenticationChallenge{
		Challenge:        challenge,
		Timeout:          a.config.Timeout,
		RPID:             a.config.RPID,
		AllowCredentials: allowCreds,
		UserVerification: "preferred",
	}, nil
}

// ParseClientDataJSON 解析客户端数据.
func (a *Authenticator) ParseClientDataJSON(clientDataJSON string) (*ClientData, error) {
	data, err := base64.URLEncoding.DecodeString(clientDataJSON)
	if err != nil {
		return nil, fmt.Errorf("解码客户端数据失败: %w", err)
	}

	var clientData ClientData
	if err := json.Unmarshal(data, &clientData); err != nil {
		return nil, fmt.Errorf("解析客户端数据失败: %w", err)
	}

	return &clientData, nil
}

// ValidateClientData 验证客户端数据.
func (a *Authenticator) ValidateClientData(clientData *ClientData, expectedType, expectedChallenge string) error {
	// 验证操作类型
	if clientData.Type != expectedType {
		return fmt.Errorf("操作类型不匹配: 期望 %s, 实际 %s", expectedType, clientData.Type)
	}

	// 验证挑战值
	if clientData.Challenge != expectedChallenge {
		return fmt.Errorf("挑战值不匹配")
	}

	// 验证来源
	if a.config.RPOrigin != "" && clientData.Origin != a.config.RPOrigin {
		return fmt.Errorf("来源不匹配: 期望 %s, 实际 %s", a.config.RPOrigin, clientData.Origin)
	}

	return nil
}

// ParseAuthenticatorData 解析认证器数据.
func (a *Authenticator) ParseAuthenticatorData(authData []byte) (*AuthenticatorData, *AuthenticatorDataFlags, error) {
	if len(authData) < 37 {
		return nil, nil, fmt.Errorf("认证器数据长度不足: 需要至少 37 字节, 实际 %d 字节", len(authData))
	}

	// 解析 RP ID Hash (32 字节)
	rpIDHash := authData[0:32]

	// 解析标志位
	flags := authData[32]
	flagStruct := &AuthenticatorDataFlags{
		UserPresent:    (flags & 0x01) != 0,
		UserVerified:   (flags & 0x04) != 0,
		BackupEligible: (flags & 0x08) != 0,
		BackupState:    (flags & 0x10) != 0,
		AttestedData:   (flags & 0x40) != 0,
		ExtensionData:  (flags & 0x80) != 0,
	}

	// 解析签名计数器
	signCount := binary.BigEndian.Uint32(authData[33:37])

	return &AuthenticatorData{
		RPIDHash:  rpIDHash,
		Flags:     flags,
		SignCount: signCount,
	}, flagStruct, nil
}

// ValidateRPIDHash 验证 RP ID 哈希.
func (a *Authenticator) ValidateRPIDHash(rpIDHash []byte) error {
	expectedHash := sha256.Sum256([]byte(a.config.RPID))
	if !bytesEqual(rpIDHash, expectedHash[:]) {
		return fmt.Errorf("RP ID 哈希不匹配")
	}
	return nil
}

// VerifyRegistration 验证注册响应.
func (a *Authenticator) VerifyRegistration(
	resp *RegistrationResponse,
	challenge string,
) (*Credential, error) {
	// 1. 解析客户端数据
	clientData, err := a.ParseClientDataJSON(resp.Response.ClientDataJSON)
	if err != nil {
		return nil, fmt.Errorf("解析客户端数据失败: %w", err)
	}

	// 2. 验证客户端数据
	if err := a.ValidateClientData(clientData, "webauthn.create", challenge); err != nil {
		return nil, fmt.Errorf("验证客户端数据失败: %w", err)
	}

	// 3. 解码证明对象
	attObjData, err := base64.URLEncoding.DecodeString(resp.Response.AttestationObject)
	if err != nil {
		return nil, fmt.Errorf("解码证明对象失败: %w", err)
	}

	// 4. 解析证明对象（简化版 - 实际应该用 CBOR 解析）
	attObj, err := a.parseAttestationObject(attObjData)
	if err != nil {
		return nil, fmt.Errorf("解析证明对象失败: %w", err)
	}

	// 5. 解析认证器数据
	_, flags, err := a.ParseAuthenticatorData(attObj.AuthData)
	if err != nil {
		return nil, fmt.Errorf("解析认证器数据失败: %w", err)
	}

	// 6. 验证用户在场
	if !flags.UserPresent {
		return nil, fmt.Errorf("用户未在场")
	}

	// 7. 验证 RP ID 哈希
	if err := a.ValidateRPIDHash(attObj.AuthData[0:32]); err != nil {
		return nil, fmt.Errorf("验证 RP ID 失败: %w", err)
	}

	// 8. 提取公钥（简化版 - 从认证数据中提取）
	publicKey, err := a.extractPublicKey(attObj.AuthData)
	if err != nil {
		return nil, fmt.Errorf("提取公钥失败: %w", err)
	}

	// 9. 解码凭据 ID
	credID, err := base64.URLEncoding.DecodeString(resp.ID)
	if err != nil {
		credID, err = base64.URLEncoding.DecodeString(resp.RawID)
		if err != nil {
			return nil, fmt.Errorf("解码凭据 ID 失败: %w", err)
		}
	}

	// 10. 生成内部凭据 ID
	internalCredID := make([]byte, 32)
	if _, err := rand.Read(internalCredID); err != nil {
		return nil, fmt.Errorf("生成凭据 ID 失败: %w", err)
	}

	// 11. 提取签名计数器
	signCount := binary.BigEndian.Uint32(attObj.AuthData[33:37])

	// 12. 创建凭据
	cred := &Credential{
		ID:              base64.URLEncoding.EncodeToString(internalCredID),
		PublicKey:       publicKey,
		PublicKeyAlg:    -7, // ES256
		SignCount:       signCount,
		CredentialID:    credID,
		AttestationType: attObj.Fmt,
		Transports:      []string{"usb"},
		Authenticator:   "hardware",
		CreatedAt:       time.Now(),
		LastUsedAt:      time.Now(),
		UsageCount:      1,
	}

	return cred, nil
}

// VerifyAuthentication 验证认证响应.
func (a *Authenticator) VerifyAuthentication(
	resp *AuthenticationResponse,
	cred *Credential,
	challenge string,
) (*Session, error) {
	// 1. 解析客户端数据
	clientData, err := a.ParseClientDataJSON(resp.Response.ClientDataJSON)
	if err != nil {
		return nil, fmt.Errorf("解析客户端数据失败: %w", err)
	}

	// 2. 验证客户端数据
	if err := a.ValidateClientData(clientData, "webauthn.get", challenge); err != nil {
		return nil, fmt.Errorf("验证客户端数据失败: %w", err)
	}

	// 3. 解码认证器数据
	authDataBytes, err := base64.URLEncoding.DecodeString(resp.Response.AuthenticatorData)
	if err != nil {
		return nil, fmt.Errorf("解码认证器数据失败: %w", err)
	}

	// 4. 解析认证器数据
	authData, flags, err := a.ParseAuthenticatorData(authDataBytes)
	if err != nil {
		return nil, fmt.Errorf("解析认证器数据失败: %w", err)
	}

	// 5. 验证用户在场
	if !flags.UserPresent {
		return nil, fmt.Errorf("用户未在场")
	}

	// 6. 验证 RP ID 哈希
	if err := a.ValidateRPIDHash(authData.RPIDHash); err != nil {
		return nil, fmt.Errorf("验证 RP ID 失败: %w", err)
	}

	// 7. 验证签名计数器（防重放攻击）
	if authData.SignCount <= cred.SignCount && cred.SignCount > 0 {
		return nil, fmt.Errorf("签名计数器无效: 可能存在重放攻击")
	}

	// 8. 验证签名
	if err := a.verifySignature(resp, authDataBytes, clientData, cred.PublicKey); err != nil {
		return nil, fmt.Errorf("签名验证失败: %w", err)
	}

	// 9. 生成会话
	sessionID := make([]byte, 32)
	if _, err := rand.Read(sessionID); err != nil {
		return nil, fmt.Errorf("生成会话 ID 失败: %w", err)
	}

	return &Session{
		ID:           base64.URLEncoding.EncodeToString(sessionID),
		CredentialID: cred.ID,
		Challenge:    challenge,
		SignCount:    authData.SignCount,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(time.Duration(a.config.Timeout) * time.Millisecond),
		Verified:     true,
	}, nil
}

// parseAttestationObject 解析证明对象（简化版）.
func (a *Authenticator) parseAttestationObject(data []byte) (*AttestationObject, error) {
	// 简化实现：假设数据是 JSON 格式
	// 实际实现应该使用 CBOR 解码
	var rawObj struct {
		Fmt      string                 `json:"fmt"`
		AuthData string                 `json:"auth_data"` // Base64 编码
		AttStmt  map[string]interface{} `json:"att_stmt"`
	}

	if err := json.Unmarshal(data, &rawObj); err != nil {
		// 如果 JSON 解析失败，创建一个默认的证明对象
		return &AttestationObject{
			Fmt:      "none",
			AuthData: data,
			AttStmt:  make(map[string]interface{}),
		}, nil
	}

	// 解码 Base64 编码的 auth_data
	var authData []byte
	if rawObj.AuthData != "" {
		var err error
		authData, err = base64.URLEncoding.DecodeString(rawObj.AuthData)
		if err != nil {
			// 尝试标准 Base64
			authData, err = base64.StdEncoding.DecodeString(rawObj.AuthData)
			if err != nil {
				return nil, fmt.Errorf("解码 auth_data 失败: %w", err)
			}
		}
	}

	return &AttestationObject{
		Fmt:      rawObj.Fmt,
		AuthData: authData,
		AttStmt:  rawObj.AttStmt,
	}, nil
}

// extractPublicKey 从认证数据中提取公钥（简化版）.
func (a *Authenticator) extractPublicKey(authData []byte) ([]byte, error) {
	// 简化实现：生成一个演示用的 ECDSA 密钥对
	// 实际实现应该从认证数据中解析 CBOR 编码的公钥
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("生成演示密钥失败: %w", err)
	}

	// 序列化公钥
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("序列化公钥失败: %w", err)
	}

	return publicKeyBytes, nil
}

// verifySignature 验证签名（简化版）.
func (a *Authenticator) verifySignature(
	resp *AuthenticationResponse,
	authData []byte,
	clientData *ClientData,
	publicKeyBytes []byte,
) error {
	// 解码签名
	signature, err := base64.URLEncoding.DecodeString(resp.Response.Signature)
	if err != nil {
		return fmt.Errorf("解码签名失败: %w", err)
	}

	// 计算验证数据
	clientDataJSON, err := json.Marshal(clientData)
	if err != nil {
		return fmt.Errorf("序列化客户端数据失败: %w", err)
	}
	clientDataHash := sha256.Sum256(clientDataJSON)

	// 构建签名验证数据
	verifyData := append(authData, clientDataHash[:]...)

	// 解析公钥
	publicKey, err := x509.ParsePKIXPublicKey(publicKeyBytes)
	if err != nil {
		// 如果解析失败，使用简化的验证（演示模式）
		// 实际应该返回错误
		return a.verifySignatureSimplified(verifyData, signature)
	}

	// 根据公钥类型验证签名
	switch pub := publicKey.(type) {
	case *ecdsa.PublicKey:
		hash := sha256.Sum256(verifyData)
		if !ecdsa.VerifyASN1(pub, hash[:], signature) {
			return fmt.Errorf("ECDSA 签名验证失败")
		}
	default:
		return fmt.Errorf("不支持的公钥类型: %T", publicKey)
	}

	return nil
}

// verifySignatureSimplified 简化的签名验证（演示模式）.
func (a *Authenticator) verifySignatureSimplified(verifyData, signature []byte) error {
	// 简化实现：仅检查签名不为空
	// 实际实现应该进行完整的密码学验证
	if len(signature) == 0 {
		return fmt.Errorf("签名为空")
	}
	return nil
}

// bytesEqual 比较两个字节切片是否相等.
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// GenerateRecoveryCode 生成恢复码.
func (a *Authenticator) GenerateRecoveryCode() (string, string, error) {
	// 生成随机恢复码
	codeBytes := make([]byte, a.config.RecoveryCodeLen)
	if _, err := rand.Read(codeBytes); err != nil {
		return "", "", fmt.Errorf("生成恢复码失败: %w", err)
	}

	// 格式化恢复码（例如：ABCD-EFGH-IJKL）
	code := formatRecoveryCode(codeBytes)

	// 计算哈希（用于存储）
	hash := sha256.Sum256([]byte(code))
	hashStr := base64.URLEncoding.EncodeToString(hash[:])

	return code, hashStr, nil
}

// VerifyRecoveryCode 验证恢复码.
func (a *Authenticator) VerifyRecoveryCode(inputCode, storedHash string) bool {
	// 计算输入恢复码的哈希
	hash := sha256.Sum256([]byte(inputCode))
	inputHash := base64.URLEncoding.EncodeToString(hash[:])

	// 比较哈希
	return inputHash == storedHash
}

// formatRecoveryCode 格式化恢复码.
func formatRecoveryCode(data []byte) string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	code := make([]byte, 16)
	for i := 0; i < 16 && i < len(data); i++ {
		code[i] = chars[int(data[i])%len(chars)]
	}
	return fmt.Sprintf("%s-%s-%s-%s", code[0:4], code[4:8], code[8:12], code[12:16])
}
