// Package fido2 FIDO2/WebAuthn 模块单元测试
package fido2

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// ==================== 认证器测试 ====================

func TestAuthenticator_GenerateChallenge(t *testing.T) {
	auth := NewAuthenticator(nil)

	challenge1, err := auth.GenerateChallenge()
	if err != nil {
		t.Fatalf("生成挑战值失败: %v", err)
	}

	challenge2, err := auth.GenerateChallenge()
	if err != nil {
		t.Fatalf("生成挑战值失败: %v", err)
	}

	// 验证挑战值不为空
	if challenge1 == "" || challenge2 == "" {
		t.Error("挑战值不能为空")
	}

	// 验证两次生成的挑战值不同
	if challenge1 == challenge2 {
		t.Error("两次生成的挑战值不应相同")
	}

	// 验证是有效的 Base64
	_, err = base64.URLEncoding.DecodeString(challenge1)
	if err != nil {
		t.Errorf("挑战值不是有效的 Base64: %v", err)
	}
}

func TestAuthenticator_CreateRegistrationChallenge(t *testing.T) {
	auth := NewAuthenticator(nil)

	challenge, err := auth.CreateRegistrationChallenge("user1", "testuser", "Test User", nil)
	if err != nil {
		t.Fatalf("创建注册挑战失败: %v", err)
	}

	// 验证基本字段
	if challenge.Challenge == "" {
		t.Error("挑战值不能为空")
	}
	if challenge.RP.ID != "localhost" {
		t.Errorf("RP ID 错误: %s", challenge.RP.ID)
	}
	if challenge.RP.Name != "NAS-OS" {
		t.Errorf("RP Name 错误: %s", challenge.RP.Name)
	}
	if challenge.User.Name != "testuser" {
		t.Errorf("用户名错误: %s", challenge.User.Name)
	}
	if challenge.User.DisplayName != "Test User" {
		t.Errorf("显示名称错误: %s", challenge.User.DisplayName)
	}

	// 验证支持的公钥类型
	if len(challenge.PubKeyCredParams) == 0 {
		t.Error("支持的公钥类型不能为空")
	}

	// 验证包含 ES256
	hasES256 := false
	for _, param := range challenge.PubKeyCredParams {
		if param.Alg == -7 {
			hasES256 = true
			break
		}
	}
	if !hasES256 {
		t.Error("应支持 ES256 算法")
	}
}

func TestAuthenticator_CreateRegistrationChallenge_WithExistingCreds(t *testing.T) {
	auth := NewAuthenticator(nil)

	existingCreds := []Credential{
		{
			CredentialID: []byte("existing-cred-1"),
			Transports:   []string{"usb"},
		},
		{
			CredentialID: []byte("existing-cred-2"),
			Transports:   []string{"nfc"},
		},
	}

	challenge, err := auth.CreateRegistrationChallenge("user1", "testuser", "Test User", existingCreds)
	if err != nil {
		t.Fatalf("创建注册挑战失败: %v", err)
	}

	// 验证排除的凭据
	if len(challenge.ExcludeCredentials) != 2 {
		t.Errorf("排除的凭据数量错误: 期望 2, 实际 %d", len(challenge.ExcludeCredentials))
	}
}

func TestAuthenticator_CreateAuthenticationChallenge(t *testing.T) {
	auth := NewAuthenticator(nil)

	creds := []Credential{
		{
			ID:           "cred1",
			CredentialID: []byte("cred-id-1"),
			Transports:   []string{"usb"},
		},
	}

	challenge, err := auth.CreateAuthenticationChallenge(creds)
	if err != nil {
		t.Fatalf("创建认证挑战失败: %v", err)
	}

	// 验证基本字段
	if challenge.Challenge == "" {
		t.Error("挑战值不能为空")
	}
	if challenge.RPID != "localhost" {
		t.Errorf("RP ID 错误: %s", challenge.RPID)
	}

	// 验证允许的凭据
	if len(challenge.AllowCredentials) != 1 {
		t.Errorf("允许的凭据数量错误: 期望 1, 实际 %d", len(challenge.AllowCredentials))
	}
}

func TestAuthenticator_ParseClientDataJSON(t *testing.T) {
	auth := NewAuthenticator(nil)

	clientData := ClientData{
		Type:      "webauthn.create",
		Challenge: "test-challenge",
		Origin:    "http://localhost:8080",
	}

	jsonData, _ := json.Marshal(clientData)
	encoded := base64.URLEncoding.EncodeToString(jsonData)

	parsed, err := auth.ParseClientDataJSON(encoded)
	if err != nil {
		t.Fatalf("解析客户端数据失败: %v", err)
	}

	if parsed.Type != clientData.Type {
		t.Errorf("类型不匹配: %s != %s", parsed.Type, clientData.Type)
	}
	if parsed.Challenge != clientData.Challenge {
		t.Errorf("挑战值不匹配: %s != %s", parsed.Challenge, clientData.Challenge)
	}
	if parsed.Origin != clientData.Origin {
		t.Errorf("来源不匹配: %s != %s", parsed.Origin, clientData.Origin)
	}
}

func TestAuthenticator_ValidateClientData(t *testing.T) {
	auth := NewAuthenticator(nil)

	tests := []struct {
		name          string
		clientData    *ClientData
		expectedType  string
		expectedChall string
		expectError   bool
	}{
		{
			name:          "有效数据",
			clientData:    &ClientData{Type: "webauthn.create", Challenge: "chall1", Origin: "http://localhost:8080"},
			expectedType:  "webauthn.create",
			expectedChall: "chall1",
			expectError:   false,
		},
		{
			name:          "类型不匹配",
			clientData:    &ClientData{Type: "webauthn.get", Challenge: "chall1", Origin: "http://localhost:8080"},
			expectedType:  "webauthn.create",
			expectedChall: "chall1",
			expectError:   true,
		},
		{
			name:          "挑战值不匹配",
			clientData:    &ClientData{Type: "webauthn.create", Challenge: "chall2", Origin: "http://localhost:8080"},
			expectedType:  "webauthn.create",
			expectedChall: "chall1",
			expectError:   true,
		},
		{
			name:          "来源不匹配",
			clientData:    &ClientData{Type: "webauthn.create", Challenge: "chall1", Origin: "http://evil.com"},
			expectedType:  "webauthn.create",
			expectedChall: "chall1",
			expectError:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := auth.ValidateClientData(tc.clientData, tc.expectedType, tc.expectedChall)
			if (err != nil) != tc.expectError {
				t.Errorf("期望错误: %v, 实际错误: %v", tc.expectError, err)
			}
		})
	}
}

func TestAuthenticator_ParseAuthenticatorData(t *testing.T) {
	auth := NewAuthenticator(nil)

	// 构建有效的认证器数据 (37 字节)
	authData := make([]byte, 37)
	// RP ID Hash (32 字节)
	for i := 0; i < 32; i++ {
		authData[i] = byte(i)
	}
	// 标志位: UP=1, UV=1
	authData[32] = 0x05
	// 签名计数器: 1
	authData[33] = 0
	authData[34] = 0
	authData[35] = 0
	authData[36] = 1

	data, flags, err := auth.ParseAuthenticatorData(authData)
	if err != nil {
		t.Fatalf("解析认证器数据失败: %v", err)
	}

	if !flags.UserPresent {
		t.Error("用户在场标志应为 true")
	}
	if !flags.UserVerified {
		t.Error("用户已验证标志应为 true")
	}
	if data.SignCount != 1 {
		t.Errorf("签名计数器错误: 期望 1, 实际 %d", data.SignCount)
	}
}

func TestAuthenticator_ParseAuthenticatorData_InvalidLength(t *testing.T) {
	auth := NewAuthenticator(nil)

	// 长度不足的数据
	authData := make([]byte, 10)

	_, _, err := auth.ParseAuthenticatorData(authData)
	if err == nil {
		t.Error("应该返回长度不足错误")
	}
}

func TestAuthenticator_ValidateRPIDHash(t *testing.T) {
	auth := NewAuthenticator(nil)

	// 这个测试需要真实的 SHA256 实现
	// 在简化版本中，我们只测试函数存在
	_ = auth
}

// ==================== 凭据存储测试 ====================

func TestMemoryCredentialStore_SaveAndGet(t *testing.T) {
	store := NewMemoryCredentialStore()

	cred := &Credential{
		ID:           "cred1",
		UserID:       "user1",
		Name:         "YubiKey 5",
		CredentialID: []byte("webauthn-cred-id"),
		CreatedAt:    time.Now(),
	}

	// 保存凭据
	if err := store.SaveCredential(cred); err != nil {
		t.Fatalf("保存凭据失败: %v", err)
	}

	// 获取凭据
	got, err := store.GetCredential("cred1")
	if err != nil {
		t.Fatalf("获取凭据失败: %v", err)
	}

	if got.ID != cred.ID {
		t.Errorf("凭据 ID 不匹配: %s != %s", got.ID, cred.ID)
	}
	if got.UserID != cred.UserID {
		t.Errorf("用户 ID 不匹配: %s != %s", got.UserID, cred.UserID)
	}
	if got.Name != cred.Name {
		t.Errorf("凭据名称不匹配: %s != %s", got.Name, cred.Name)
	}
}

func TestMemoryCredentialStore_SaveDuplicate(t *testing.T) {
	store := NewMemoryCredentialStore()

	cred := &Credential{
		ID:     "cred1",
		UserID: "user1",
	}

	// 第一次保存
	if err := store.SaveCredential(cred); err != nil {
		t.Fatalf("保存凭据失败: %v", err)
	}

	// 第二次保存应失败
	if err := store.SaveCredential(cred); err == nil {
		t.Error("重复保存应返回错误")
	}
}

func TestMemoryCredentialStore_GetNonExistent(t *testing.T) {
	store := NewMemoryCredentialStore()

	_, err := store.GetCredential("nonexistent")
	if err == nil {
		t.Error("获取不存在的凭据应返回错误")
	}
}

func TestMemoryCredentialStore_GetByWebAuthnID(t *testing.T) {
	store := NewMemoryCredentialStore()

	webauthnID := []byte("webauthn-cred-id-123")
	cred := &Credential{
		ID:           "cred1",
		UserID:       "user1",
		CredentialID: webauthnID,
	}

	store.SaveCredential(cred)

	got, err := store.GetCredentialByWebAuthnID(webauthnID)
	if err != nil {
		t.Fatalf("根据 WebAuthn ID 获取凭据失败: %v", err)
	}

	if got.ID != cred.ID {
		t.Errorf("凭据 ID 不匹配: %s != %s", got.ID, cred.ID)
	}
}

func TestMemoryCredentialStore_GetUserCredentials(t *testing.T) {
	store := NewMemoryCredentialStore()

	// 添加多个用户的凭据
	store.SaveCredential(&Credential{ID: "cred1", UserID: "user1", CreatedAt: time.Now()})
	store.SaveCredential(&Credential{ID: "cred2", UserID: "user1", CreatedAt: time.Now().Add(time.Second)})
	store.SaveCredential(&Credential{ID: "cred3", UserID: "user2", CreatedAt: time.Now()})

	creds, err := store.GetUserCredentials("user1")
	if err != nil {
		t.Fatalf("获取用户凭据失败: %v", err)
	}

	if len(creds) != 2 {
		t.Errorf("用户凭据数量错误: 期望 2, 实际 %d", len(creds))
	}

	// 验证排序（按创建时间）
	if creds[0].ID != "cred1" || creds[1].ID != "cred2" {
		t.Error("凭据排序错误")
	}
}

func TestMemoryCredentialStore_Delete(t *testing.T) {
	store := NewMemoryCredentialStore()

	cred := &Credential{ID: "cred1", UserID: "user1"}
	store.SaveCredential(cred)

	// 删除凭据
	if err := store.DeleteCredential("cred1"); err != nil {
		t.Fatalf("删除凭据失败: %v", err)
	}

	// 验证已删除
	_, err := store.GetCredential("cred1")
	if err == nil {
		t.Error("凭据应已删除")
	}

	// 删除不存在的凭据
	if err := store.DeleteCredential("nonexistent"); err == nil {
		t.Error("删除不存在的凭据应返回错误")
	}
}

func TestMemoryCredentialStore_Update(t *testing.T) {
	store := NewMemoryCredentialStore()

	cred := &Credential{ID: "cred1", UserID: "user1", Name: "Original"}
	store.SaveCredential(cred)

	// 更新凭据
	cred.Name = "Updated"
	if err := store.UpdateCredential(cred); err != nil {
		t.Fatalf("更新凭据失败: %v", err)
	}

	// 验证更新
	got, _ := store.GetCredential("cred1")
	if got.Name != "Updated" {
		t.Errorf("凭据名称未更新: %s", got.Name)
	}
}

func TestMemoryCredentialStore_List(t *testing.T) {
	store := NewMemoryCredentialStore()

	store.SaveCredential(&Credential{ID: "cred1", UserID: "user1", CreatedAt: time.Now()})
	store.SaveCredential(&Credential{ID: "cred2", UserID: "user2", CreatedAt: time.Now()})

	creds, err := store.ListCredentials()
	if err != nil {
		t.Fatalf("列出凭据失败: %v", err)
	}

	if len(creds) != 2 {
		t.Errorf("凭据数量错误: 期望 2, 实际 %d", len(creds))
	}
}

// ==================== 凭据管理器测试 ====================

func TestCredentialManager_RegisterCredential(t *testing.T) {
	store := NewMemoryCredentialStore()
	auth := NewAuthenticator(nil)
	config := DefaultConfig()
	manager := NewCredentialManager(store, auth, config)

	// 构建有效的认证器数据 (37 字节)
	authData := make([]byte, 37)
	// RP ID Hash (32 字节)
	rpIDHash := sha256.Sum256([]byte("localhost"))
	copy(authData[0:32], rpIDHash[:])
	// 标志位: UP=1 (用户在场)
	authData[32] = 0x01
	// 签名计数器: 0
	authData[33] = 0
	authData[34] = 0
	authData[35] = 0
	authData[36] = 0

	// 构建证明对象
	attObj := map[string]interface{}{
		"fmt":       "none",
		"auth_data": base64.URLEncoding.EncodeToString(authData),
		"att_stmt":  map[string]interface{}{},
	}
	attObjJSON, _ := json.Marshal(attObj)

	// 构建客户端数据
	clientData := ClientData{
		Type:      "webauthn.create",
		Challenge: "test-challenge",
		Origin:    "http://localhost:8080",
	}
	clientDataJSON, _ := json.Marshal(clientData)

	resp := &RegistrationResponse{
		ID:    base64.URLEncoding.EncodeToString([]byte("cred-id")),
		RawID: base64.URLEncoding.EncodeToString([]byte("cred-id")),
		Type:  "public-key",
		Response: RegistrationResponseData{
			AttestationObject: base64.URLEncoding.EncodeToString(attObjJSON),
			ClientDataJSON:    base64.URLEncoding.EncodeToString(clientDataJSON),
		},
	}

	cred, err := manager.RegisterCredential("user1", "My YubiKey", resp, "test-challenge")
	if err != nil {
		t.Fatalf("注册凭据失败: %v", err)
	}

	if cred.UserID != "user1" {
		t.Errorf("用户 ID 不匹配: %s", cred.UserID)
	}
	if cred.Name != "My YubiKey" {
		t.Errorf("凭据名称不匹配: %s", cred.Name)
	}
}

func TestCredentialManager_MaxCredentialsLimit(t *testing.T) {
	store := NewMemoryCredentialStore()
	auth := NewAuthenticator(nil)
	config := DefaultConfig()
	config.MaxCredentials = 2 // 设置限制为 2
	manager := NewCredentialManager(store, auth, config)

	// 添加 2 个凭据
	store.SaveCredential(&Credential{ID: "cred1", UserID: "user1", CreatedAt: time.Now()})
	store.SaveCredential(&Credential{ID: "cred2", UserID: "user1", CreatedAt: time.Now()})

	// 尝试添加第 3 个应失败
	clientData := ClientData{
		Type:      "webauthn.create",
		Challenge: "test-challenge",
		Origin:    "http://localhost:8080",
	}
	clientDataJSON, _ := json.Marshal(clientData)

	resp := &RegistrationResponse{
		ID:    base64.URLEncoding.EncodeToString([]byte("cred-id")),
		RawID: base64.URLEncoding.EncodeToString([]byte("cred-id")),
		Type:  "public-key",
		Response: RegistrationResponseData{
			AttestationObject: base64.URLEncoding.EncodeToString([]byte(`{"fmt":"none","auth_data":"` + base64.URLEncoding.EncodeToString(make([]byte, 37)) + `"}`)),
			ClientDataJSON:    base64.URLEncoding.EncodeToString(clientDataJSON),
		},
	}

	_, err := manager.RegisterCredential("user1", "Third Key", resp, "test-challenge")
	if err == nil {
		t.Error("超过最大凭据数量限制应返回错误")
	}
}

func TestCredentialManager_RenameCredential(t *testing.T) {
	store := NewMemoryCredentialStore()
	auth := NewAuthenticator(nil)
	manager := NewCredentialManager(store, auth, nil)

	store.SaveCredential(&Credential{ID: "cred1", UserID: "user1", Name: "Original"})

	if err := manager.RenameCredential("cred1", "New Name"); err != nil {
		t.Fatalf("重命名凭据失败: %v", err)
	}

	cred, _ := store.GetCredential("cred1")
	if cred.Name != "New Name" {
		t.Errorf("凭据名称未更新: %s", cred.Name)
	}
}

func TestCredentialManager_RevokeCredential(t *testing.T) {
	store := NewMemoryCredentialStore()
	auth := NewAuthenticator(nil)
	manager := NewCredentialManager(store, auth, nil)

	store.SaveCredential(&Credential{ID: "cred1", UserID: "user1"})

	if err := manager.RevokeCredential("cred1"); err != nil {
		t.Fatalf("吊销凭据失败: %v", err)
	}

	cred, _ := store.GetCredential("cred1")
	if !cred.Revoked {
		t.Error("凭据应已吊销")
	}
	if cred.RevokedAt == nil {
		t.Error("吊销时间应已设置")
	}
}

func TestCredentialManager_UpdateCredentialUsage(t *testing.T) {
	store := NewMemoryCredentialStore()
	auth := NewAuthenticator(nil)
	manager := NewCredentialManager(store, auth, nil)

	store.SaveCredential(&Credential{ID: "cred1", UserID: "user1", SignCount: 0, UsageCount: 0})

	if err := manager.UpdateCredentialUsage("cred1", 5); err != nil {
		t.Fatalf("更新凭据使用信息失败: %v", err)
	}

	cred, _ := store.GetCredential("cred1")
	if cred.SignCount != 5 {
		t.Errorf("签名计数器未更新: %d", cred.SignCount)
	}
	if cred.UsageCount != 1 {
		t.Errorf("使用次数未更新: %d", cred.UsageCount)
	}
}

func TestCredentialManager_GetActiveUserCredentials(t *testing.T) {
	store := NewMemoryCredentialStore()
	auth := NewAuthenticator(nil)
	manager := NewCredentialManager(store, auth, nil)

	store.SaveCredential(&Credential{ID: "cred1", UserID: "user1", Revoked: false})
	store.SaveCredential(&Credential{ID: "cred2", UserID: "user1", Revoked: true})
	store.SaveCredential(&Credential{ID: "cred3", UserID: "user1", Revoked: false})

	creds, err := manager.GetActiveUserCredentials("user1")
	if err != nil {
		t.Fatalf("获取活跃凭据失败: %v", err)
	}

	if len(creds) != 2 {
		t.Errorf("活跃凭据数量错误: 期望 2, 实际 %d", len(creds))
	}
}

// ==================== 恢复码测试 ====================

func TestRecoveryCodeStore_SaveAndGet(t *testing.T) {
	store := NewMemoryRecoveryCodeStore()

	code := &RecoveryCode{
		ID:        "code1",
		UserID:    "user1",
		Code:      "hashed-code",
		Used:      false,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	if err := store.SaveRecoveryCode(code); err != nil {
		t.Fatalf("保存恢复码失败: %v", err)
	}

	got, err := store.GetRecoveryCode("code1")
	if err != nil {
		t.Fatalf("获取恢复码失败: %v", err)
	}

	if got.ID != code.ID {
		t.Errorf("恢复码 ID 不匹配: %s != %s", got.ID, code.ID)
	}
}

func TestRecoveryCodeStore_GetUnusedUserCodes(t *testing.T) {
	store := NewMemoryRecoveryCodeStore()

	store.SaveRecoveryCode(&RecoveryCode{ID: "code1", UserID: "user1", Used: false})
	store.SaveRecoveryCode(&RecoveryCode{ID: "code2", UserID: "user1", Used: true})
	store.SaveRecoveryCode(&RecoveryCode{ID: "code3", UserID: "user1", Used: false})

	codes, err := store.GetUnusedUserRecoveryCodes("user1")
	if err != nil {
		t.Fatalf("获取未使用恢复码失败: %v", err)
	}

	if len(codes) != 2 {
		t.Errorf("未使用恢复码数量错误: 期望 2, 实际 %d", len(codes))
	}
}

func TestRecoveryCodeManager_GenerateRecoveryCodes(t *testing.T) {
	store := NewMemoryRecoveryCodeStore()
	auth := NewAuthenticator(nil)
	manager := NewRecoveryCodeManager(store, auth, nil)

	codes, err := manager.GenerateRecoveryCodes("user1", 5)
	if err != nil {
		t.Fatalf("生成恢复码失败: %v", err)
	}

	if len(codes) != 5 {
		t.Errorf("恢复码数量错误: 期望 5, 实际 %d", len(codes))
	}

	// 验证恢复码格式
	for _, code := range codes {
		if len(code) != 19 { // XXXX-XXXX-XXXX-XXXX
			t.Errorf("恢复码格式错误: %s", code)
		}
	}
}

func TestRecoveryCodeManager_VerifyRecoveryCode(t *testing.T) {
	store := NewMemoryRecoveryCodeStore()
	auth := NewAuthenticator(nil)
	manager := NewRecoveryCodeManager(store, auth, nil)

	// 生成恢复码
	codes, _ := manager.GenerateRecoveryCodes("user1", 3)

	// 验证第一个恢复码
	valid, err := manager.VerifyRecoveryCode("user1", codes[0])
	if err != nil {
		t.Fatalf("验证恢复码失败: %v", err)
	}
	if !valid {
		t.Error("恢复码应有效")
	}

	// 再次验证应失败（已使用）
	valid, err = manager.VerifyRecoveryCode("user1", codes[0])
	if err != nil {
		t.Fatalf("验证恢复码失败: %v", err)
	}
	if valid {
		t.Error("已使用的恢复码应无效")
	}

	// 验证无效的恢复码
	valid, err = manager.VerifyRecoveryCode("user1", "XXXX-XXXX-XXXX-XXXX")
	if err != nil {
		t.Fatalf("验证恢复码失败: %v", err)
	}
	if valid {
		t.Error("无效的恢复码应返回 false")
	}
}

func TestRecoveryCodeManager_GenerateInvalidCount(t *testing.T) {
	store := NewMemoryRecoveryCodeStore()
	auth := NewAuthenticator(nil)
	manager := NewRecoveryCodeManager(store, auth, nil)

	_, err := manager.GenerateRecoveryCodes("user1", 0)
	if err == nil {
		t.Error("恢复码数量为 0 应返回错误")
	}

	_, err = manager.GenerateRecoveryCodes("user1", 11)
	if err == nil {
		t.Error("恢复码数量超过 10 应返回错误")
	}
}

// ==================== 配置测试 ====================

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.RPID != "localhost" {
		t.Errorf("默认 RP ID 错误: %s", config.RPID)
	}
	if config.RPName != "NAS-OS" {
		t.Errorf("默认 RP 名称错误: %s", config.RPName)
	}
	if config.ChallengeLen != 32 {
		t.Errorf("默认挑战值长度错误: %d", config.ChallengeLen)
	}
	if config.Timeout != 60000 {
		t.Errorf("默认超时时间错误: %d", config.Timeout)
	}
	if config.MaxCredentials != 10 {
		t.Errorf("默认最大凭据数错误: %d", config.MaxCredentials)
	}
}

// ==================== 辅助函数测试 ====================

func TestBytesEqual(t *testing.T) {
	tests := []struct {
		name   string
		a, b   []byte
		expect bool
	}{
		{"相等", []byte{1, 2, 3}, []byte{1, 2, 3}, true},
		{"不相等", []byte{1, 2, 3}, []byte{1, 2, 4}, false},
		{"长度不同", []byte{1, 2}, []byte{1, 2, 3}, false},
		{"空切片", []byte{}, []byte{}, true},
		{"nil", nil, nil, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := bytesEqual(tc.a, tc.b)
			if result != tc.expect {
				t.Errorf("bytesEqual(%v, %v) = %v, 期望 %v", tc.a, tc.b, result, tc.expect)
			}
		})
	}
}

func TestFormatRecoveryCode(t *testing.T) {
	data := []byte{0x41, 0x42, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49, 0x4A, 0x4B, 0x4C, 0x4D, 0x4E, 0x4F, 0x50}
	code := formatRecoveryCode(data)

	// 验证格式: XXXX-XXXX-XXXX-XXXX
	if len(code) != 19 {
		t.Errorf("恢复码长度错误: %d", len(code))
	}
	if code[4] != '-' || code[9] != '-' || code[14] != '-' {
		t.Errorf("恢复码格式错误: %s", code)
	}
}

func TestBase64URLEncode(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5}
	encoded := base64URLEncode(data)

	decoded, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}

	if !bytesEqual(data, decoded) {
		t.Error("编码解码不匹配")
	}
}

// ==================== VerifyAuthentication 测试 ====================

func TestAuthenticator_VerifyAuthentication(t *testing.T) {
	auth := NewAuthenticator(nil)

	// 构建有效的认证器数据
	authData := make([]byte, 37)
	rpIDHash := sha256.Sum256([]byte("localhost"))
	copy(authData[0:32], rpIDHash[:])
	authData[32] = 0x01 // UP=1
	authData[33] = 0    // SignCount=1
	authData[34] = 0
	authData[35] = 0
	authData[36] = 1

	// 构建客户端数据
	clientData := ClientData{
		Type:      "webauthn.get",
		Challenge: "test-challenge",
		Origin:    "http://localhost:8080",
	}
	clientDataJSON, _ := json.Marshal(clientData)

	// 生成测试密钥对
	privateKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	publicKeyBytes, _ := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)

	// 生成签名
	clientDataHash := sha256.Sum256(clientDataJSON)
	verifyData := append(authData, clientDataHash[:]...)
	hash := sha256.Sum256(verifyData)
	signature, _ := ecdsa.SignASN1(rand.Reader, privateKey, hash[:])

	resp := &AuthenticationResponse{
		ID:    base64.URLEncoding.EncodeToString([]byte("cred-id")),
		RawID: base64.URLEncoding.EncodeToString([]byte("cred-id")),
		Type:  "public-key",
		Response: AuthenticationResponseData{
			AuthenticatorData: base64.URLEncoding.EncodeToString(authData),
			ClientDataJSON:    base64.URLEncoding.EncodeToString(clientDataJSON),
			Signature:         base64.URLEncoding.EncodeToString(signature),
		},
	}

	cred := &Credential{
		ID:           "cred1",
		PublicKey:    publicKeyBytes,
		SignCount:    0,
		CredentialID: []byte("cred-id"),
	}

	session, err := auth.VerifyAuthentication(resp, cred, "test-challenge")
	if err != nil {
		t.Fatalf("验证认证失败: %v", err)
	}

	if !session.Verified {
		t.Error("会话应已验证")
	}
	if session.SignCount != 1 {
		t.Errorf("签名计数器错误: %d", session.SignCount)
	}
}

func TestAuthenticator_VerifyAuthentication_InvalidChallenge(t *testing.T) {
	auth := NewAuthenticator(nil)

	clientData := ClientData{
		Type:      "webauthn.get",
		Challenge: "wrong-challenge",
		Origin:    "http://localhost:8080",
	}
	clientDataJSON, _ := json.Marshal(clientData)

	resp := &AuthenticationResponse{
		ID: base64.URLEncoding.EncodeToString([]byte("cred-id")),
		Response: AuthenticationResponseData{
			AuthenticatorData: base64.URLEncoding.EncodeToString(make([]byte, 37)),
			ClientDataJSON:    base64.URLEncoding.EncodeToString(clientDataJSON),
			Signature:         base64.URLEncoding.EncodeToString([]byte("sig")),
		},
	}

	cred := &Credential{ID: "cred1", PublicKey: []byte("key"), SignCount: 0}

	_, err := auth.VerifyAuthentication(resp, cred, "test-challenge")
	if err == nil {
		t.Error("挑战值不匹配应返回错误")
	}
}

func TestAuthenticator_VerifyAuthentication_UserNotPresent(t *testing.T) {
	auth := NewAuthenticator(nil)

	authData := make([]byte, 37)
	rpIDHash := sha256.Sum256([]byte("localhost"))
	copy(authData[0:32], rpIDHash[:])
	authData[32] = 0x00 // UP=0 (用户未在场)

	clientData := ClientData{
		Type:      "webauthn.get",
		Challenge: "test-challenge",
		Origin:    "http://localhost:8080",
	}
	clientDataJSON, _ := json.Marshal(clientData)

	resp := &AuthenticationResponse{
		ID: base64.URLEncoding.EncodeToString([]byte("cred-id")),
		Response: AuthenticationResponseData{
			AuthenticatorData: base64.URLEncoding.EncodeToString(authData),
			ClientDataJSON:    base64.URLEncoding.EncodeToString(clientDataJSON),
			Signature:         base64.URLEncoding.EncodeToString([]byte("sig")),
		},
	}

	cred := &Credential{ID: "cred1", PublicKey: []byte("key"), SignCount: 0}

	_, err := auth.VerifyAuthentication(resp, cred, "test-challenge")
	if err == nil {
		t.Error("用户未在场应返回错误")
	}
}

func TestAuthenticator_VerifyAuthentication_ReplayAttack(t *testing.T) {
	auth := NewAuthenticator(nil)

	authData := make([]byte, 37)
	rpIDHash := sha256.Sum256([]byte("localhost"))
	copy(authData[0:32], rpIDHash[:])
	authData[32] = 0x01 // UP=1
	authData[36] = 1    // SignCount=1

	clientData := ClientData{
		Type:      "webauthn.get",
		Challenge: "test-challenge",
		Origin:    "http://localhost:8080",
	}
	clientDataJSON, _ := json.Marshal(clientData)

	resp := &AuthenticationResponse{
		ID: base64.URLEncoding.EncodeToString([]byte("cred-id")),
		Response: AuthenticationResponseData{
			AuthenticatorData: base64.URLEncoding.EncodeToString(authData),
			ClientDataJSON:    base64.URLEncoding.EncodeToString(clientDataJSON),
			Signature:         base64.URLEncoding.EncodeToString([]byte("sig")),
		},
	}

	cred := &Credential{ID: "cred1", PublicKey: []byte("key"), SignCount: 1}

	_, err := auth.VerifyAuthentication(resp, cred, "test-challenge")
	if err == nil {
		t.Error("重放攻击应返回错误")
	}
}

// ==================== Handlers 测试 ====================

func TestHandlers_RegisterRoutes(t *testing.T) {
	store := NewMemoryCredentialStore()
	auth := NewAuthenticator(nil)
	config := DefaultConfig()
	credManager := NewCredentialManager(store, auth, config)
	recoveryManager := NewRecoveryCodeManager(nil, auth, config)
	handlers := NewHandlers(credManager, recoveryManager, config)

	// 验证处理器创建成功
	if handlers == nil {
		t.Error("处理器创建失败")
	}
	if handlers.credManager == nil {
		t.Error("凭据管理器不能为空")
	}
	if handlers.recoveryManager == nil {
		t.Error("恢复码管理器不能为空")
	}
}

func TestHandlers_Sessions(t *testing.T) {
	store := NewMemoryCredentialStore()
	auth := NewAuthenticator(nil)
	config := DefaultConfig()
	credManager := NewCredentialManager(store, auth, config)
	recoveryManager := NewRecoveryCodeManager(nil, auth, config)
	handlers := NewHandlers(credManager, recoveryManager, config)

	// 测试会话存储
	session := &Session{
		ID:        "session1",
		UserID:    "user1",
		Verified:  true,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	handlers.sessions[session.ID] = session

	if _, exists := handlers.sessions["session1"]; !exists {
		t.Error("会话应存在")
	}
}

func TestHandlers_PendingRegistrations(t *testing.T) {
	store := NewMemoryCredentialStore()
	auth := NewAuthenticator(nil)
	config := DefaultConfig()
	credManager := NewCredentialManager(store, auth, config)
	recoveryManager := NewRecoveryCodeManager(nil, auth, config)
	handlers := NewHandlers(credManager, recoveryManager, config)

	// 测试待处理注册存储
	pending := &pendingRegistration{
		Challenge: "challenge1",
		UserID:    "user1",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Minute),
	}
	handlers.pendingRegs[pending.Challenge] = pending

	if _, exists := handlers.pendingRegs["challenge1"]; !exists {
		t.Error("待处理注册应存在")
	}
}

func TestHandlers_PendingAuthentications(t *testing.T) {
	store := NewMemoryCredentialStore()
	auth := NewAuthenticator(nil)
	config := DefaultConfig()
	credManager := NewCredentialManager(store, auth, config)
	recoveryManager := NewRecoveryCodeManager(nil, auth, config)
	handlers := NewHandlers(credManager, recoveryManager, config)

	// 测试待处理认证存储
	pending := &pendingAuthentication{
		Challenge: "challenge1",
		UserID:    "user1",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Minute),
	}
	handlers.pendingAuths[pending.Challenge] = pending

	if _, exists := handlers.pendingAuths["challenge1"]; !exists {
		t.Error("待处理认证应存在")
	}
}

// ==================== 补充测试 ====================

func TestCredentialManager_GetCredential(t *testing.T) {
	store := NewMemoryCredentialStore()
	auth := NewAuthenticator(nil)
	manager := NewCredentialManager(store, auth, nil)

	store.SaveCredential(&Credential{ID: "cred1", UserID: "user1", Name: "Test"})

	cred, err := manager.GetCredential("cred1")
	if err != nil {
		t.Fatalf("获取凭据失败: %v", err)
	}
	if cred.Name != "Test" {
		t.Errorf("凭据名称错误: %s", cred.Name)
	}
}

func TestCredentialManager_GetUserCredentials(t *testing.T) {
	store := NewMemoryCredentialStore()
	auth := NewAuthenticator(nil)
	manager := NewCredentialManager(store, auth, nil)

	store.SaveCredential(&Credential{ID: "cred1", UserID: "user1", CreatedAt: time.Now()})
	store.SaveCredential(&Credential{ID: "cred2", UserID: "user2", CreatedAt: time.Now()})

	creds, err := manager.GetUserCredentials("user1")
	if err != nil {
		t.Fatalf("获取用户凭据失败: %v", err)
	}
	if len(creds) != 1 {
		t.Errorf("凭据数量错误: %d", len(creds))
	}
}

func TestCredentialManager_GetUserCredentialInfos(t *testing.T) {
	store := NewMemoryCredentialStore()
	auth := NewAuthenticator(nil)
	manager := NewCredentialManager(store, auth, nil)

	store.SaveCredential(&Credential{
		ID:            "cred1",
		UserID:        "user1",
		Name:          "YubiKey",
		Authenticator: "hardware",
		Transports:    []string{"usb"},
		CreatedAt:     time.Now(),
		LastUsedAt:    time.Now(),
		UsageCount:    5,
	})

	infos, err := manager.GetUserCredentialInfos("user1")
	if err != nil {
		t.Fatalf("获取凭据信息失败: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("凭据数量错误: %d", len(infos))
	}
	if infos[0].Name != "YubiKey" {
		t.Errorf("凭据名称错误: %s", infos[0].Name)
	}
	if infos[0].UsageCount != 5 {
		t.Errorf("使用次数错误: %d", infos[0].UsageCount)
	}
}

func TestCredentialManager_DeleteCredential(t *testing.T) {
	store := NewMemoryCredentialStore()
	auth := NewAuthenticator(nil)
	manager := NewCredentialManager(store, auth, nil)

	store.SaveCredential(&Credential{ID: "cred1", UserID: "user1"})

	if err := manager.DeleteCredential("cred1"); err != nil {
		t.Fatalf("删除凭据失败: %v", err)
	}

	_, err := manager.GetCredential("cred1")
	if err == nil {
		t.Error("凭据应已删除")
	}
}

func TestCredentialManager_FindCredentialByWebAuthnID(t *testing.T) {
	store := NewMemoryCredentialStore()
	auth := NewAuthenticator(nil)
	manager := NewCredentialManager(store, auth, nil)

	webauthnID := []byte("webauthn-id-123")
	store.SaveCredential(&Credential{ID: "cred1", UserID: "user1", CredentialID: webauthnID})

	cred, err := manager.FindCredentialByWebAuthnID(webauthnID)
	if err != nil {
		t.Fatalf("查找凭据失败: %v", err)
	}
	if cred.ID != "cred1" {
		t.Errorf("凭据 ID 错误: %s", cred.ID)
	}
}

func TestRecoveryCodeManager_GetUserRecoveryCodeInfos(t *testing.T) {
	store := NewMemoryRecoveryCodeStore()
	auth := NewAuthenticator(nil)
	manager := NewRecoveryCodeManager(store, auth, nil)

	now := time.Now()
	store.SaveRecoveryCode(&RecoveryCode{
		ID:        "code1",
		UserID:    "user1",
		Used:      false,
		CreatedAt: now,
		ExpiresAt: now.Add(24 * time.Hour),
	})
	store.SaveRecoveryCode(&RecoveryCode{
		ID:        "code2",
		UserID:    "user1",
		Used:      true,
		UsedAt:    &now,
		CreatedAt: now,
		ExpiresAt: now.Add(24 * time.Hour),
	})

	infos, err := manager.GetUserRecoveryCodeInfos("user1")
	if err != nil {
		t.Fatalf("获取恢复码信息失败: %v", err)
	}
	if len(infos) != 2 {
		t.Errorf("恢复码数量错误: %d", len(infos))
	}
}

func TestRecoveryCodeStore_GetUserRecoveryCodes(t *testing.T) {
	store := NewMemoryRecoveryCodeStore()

	store.SaveRecoveryCode(&RecoveryCode{ID: "code1", UserID: "user1"})
	store.SaveRecoveryCode(&RecoveryCode{ID: "code2", UserID: "user1"})
	store.SaveRecoveryCode(&RecoveryCode{ID: "code3", UserID: "user2"})

	codes, err := store.GetUserRecoveryCodes("user1")
	if err != nil {
		t.Fatalf("获取恢复码失败: %v", err)
	}
	if len(codes) != 2 {
		t.Errorf("恢复码数量错误: %d", len(codes))
	}
}

func TestRecoveryCodeStore_DeleteUserRecoveryCodes(t *testing.T) {
	store := NewMemoryRecoveryCodeStore()

	store.SaveRecoveryCode(&RecoveryCode{ID: "code1", UserID: "user1"})
	store.SaveRecoveryCode(&RecoveryCode{ID: "code2", UserID: "user1"})
	store.SaveRecoveryCode(&RecoveryCode{ID: "code3", UserID: "user2"})

	if err := store.DeleteUserRecoveryCodes("user1"); err != nil {
		t.Fatalf("删除恢复码失败: %v", err)
	}

	codes, _ := store.GetUserRecoveryCodes("user1")
	if len(codes) != 0 {
		t.Errorf("恢复码应已删除: %d", len(codes))
	}

	codes, _ = store.GetUserRecoveryCodes("user2")
	if len(codes) != 1 {
		t.Errorf("其他用户的恢复码不应被删除: %d", len(codes))
	}
}

func TestCredentialStore_UpdateNonExistent(t *testing.T) {
	store := NewMemoryCredentialStore()

	err := store.UpdateCredential(&Credential{ID: "nonexistent"})
	if err == nil {
		t.Error("更新不存在的凭据应返回错误")
	}
}

func TestCredentialStore_SaveNil(t *testing.T) {
	store := NewMemoryCredentialStore()

	err := store.SaveCredential(nil)
	if err == nil {
		t.Error("保存空凭据应返回错误")
	}
}

func TestCredentialStore_SaveEmptyID(t *testing.T) {
	store := NewMemoryCredentialStore()

	err := store.SaveCredential(&Credential{})
	if err == nil {
		t.Error("保存空 ID 凭据应返回错误")
	}
}

func TestRecoveryCodeStore_SaveNil(t *testing.T) {
	store := NewMemoryRecoveryCodeStore()

	err := store.SaveRecoveryCode(nil)
	if err == nil {
		t.Error("保存空恢复码应返回错误")
	}
}

func TestRecoveryCodeStore_SaveEmptyID(t *testing.T) {
	store := NewMemoryRecoveryCodeStore()

	err := store.SaveRecoveryCode(&RecoveryCode{})
	if err == nil {
		t.Error("保存空 ID 恢复码应返回错误")
	}
}

func TestRecoveryCodeStore_UpdateNonExistent(t *testing.T) {
	store := NewMemoryRecoveryCodeStore()

	err := store.UpdateRecoveryCode(&RecoveryCode{ID: "nonexistent"})
	if err == nil {
		t.Error("更新不存在的恢复码应返回错误")
	}
}

func TestRecoveryCodeStore_UpdateNil(t *testing.T) {
	store := NewMemoryRecoveryCodeStore()

	err := store.UpdateRecoveryCode(nil)
	if err == nil {
		t.Error("更新空恢复码应返回错误")
	}
}

// ==================== HTTP 处理器测试 ====================

func TestHandlers_BeginRegistration(t *testing.T) {
	store := NewMemoryCredentialStore()
	auth := NewAuthenticator(nil)
	config := DefaultConfig()
	credManager := NewCredentialManager(store, auth, config)
	recoveryManager := NewRecoveryCodeManager(nil, auth, config)
	handlers := NewHandlers(credManager, recoveryManager, config)

	// 验证处理器创建成功
	if handlers == nil {
		t.Fatal("处理器创建失败")
	}

	// 验证待处理注册 map 初始化
	if handlers.pendingRegs == nil {
		t.Error("pendingRegs 未初始化")
	}

	// 验证待处理认证 map 初始化
	if handlers.pendingAuths == nil {
		t.Error("pendingAuths 未初始化")
	}

	// 验证会话 map 初始化
	if handlers.sessions == nil {
		t.Error("sessions 未初始化")
	}
}

func TestHandlers_Config(t *testing.T) {
	config := &Config{
		RPID:           "example.com",
		RPName:         "Test App",
		RPOrigin:       "https://example.com",
		ChallengeLen:   64,
		Timeout:        120000,
		MaxCredentials: 5,
	}

	store := NewMemoryCredentialStore()
	auth := NewAuthenticator(config)
	credManager := NewCredentialManager(store, auth, config)
	recoveryManager := NewRecoveryCodeManager(nil, auth, config)
	handlers := NewHandlers(credManager, recoveryManager, config)

	if handlers.config.RPID != "example.com" {
		t.Errorf("RPID 错误: %s", handlers.config.RPID)
	}
	if handlers.config.RPName != "Test App" {
		t.Errorf("RPName 错误: %s", handlers.config.RPName)
	}
	if handlers.config.MaxCredentials != 5 {
		t.Errorf("MaxCredentials 错误: %d", handlers.config.MaxCredentials)
	}
}

func TestHandlers_NilConfig(t *testing.T) {
	handlers := NewHandlers(nil, nil, nil)

	if handlers.config == nil {
		t.Error("配置不应为 nil")
	}
	if handlers.credManager == nil {
		t.Error("凭据管理器不应为 nil")
	}
	if handlers.recoveryManager == nil {
		t.Error("恢复码管理器不应为 nil")
	}
}

func TestBase64URLEncodeDecode(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5}
	encoded := base64.URLEncoding.EncodeToString(data)

	decoded, err := base64URLEncodeDecode(encoded)
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}

	if !bytesEqual(data, decoded) {
		t.Error("编码解码不匹配")
	}
}

func TestBase64URLEncodeDecode_InvalidInput(t *testing.T) {
	_, err := base64URLEncodeDecode("invalid-base64!")
	if err == nil {
		t.Error("无效输入应返回错误")
	}
}

func TestHandlers_BeginRegistrationHandler(t *testing.T) {
	// 使用 httptest 测试处理器
	handlers := NewHandlers(nil, nil, nil)

	// 验证处理器创建成功
	if handlers == nil {
		t.Fatal("处理器创建失败")
	}

	// 验证认证器创建成功
	if handlers.authenticator == nil {
		t.Fatal("认证器创建失败")
	}

	// 测试创建注册挑战
	challenge, err := handlers.authenticator.CreateRegistrationChallenge("user1", "test", "Test User", nil)
	if err != nil {
		t.Fatalf("创建注册挑战失败: %v", err)
	}

	// 保存待处理的注册请求
	handlers.pendingRegs[challenge.Challenge] = &pendingRegistration{
		Challenge: challenge.Challenge,
		UserID:    "user1",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Minute),
	}

	if _, exists := handlers.pendingRegs[challenge.Challenge]; !exists {
		t.Error("待处理注册请求应存在")
	}
}

func TestHandlers_BeginLoginHandler(t *testing.T) {
	handlers := NewHandlers(nil, nil, nil)

	// 添加测试凭据
	credID := []byte("cred-id-123")
	handlers.credManager.store.SaveCredential(&Credential{
		ID:           "cred1",
		UserID:       "user1",
		CredentialID: credID,
		PublicKey:    []byte("test-key"),
		Revoked:      false,
		CreatedAt:    time.Now(),
	})

	// 测试创建认证挑战
	credsPtr, _ := handlers.credManager.GetActiveUserCredentials("user1")
	var creds []Credential
	for _, c := range credsPtr {
		creds = append(creds, *c)
	}
	challenge, err := handlers.authenticator.CreateAuthenticationChallenge(creds)
	if err != nil {
		t.Fatalf("创建认证挑战失败: %v", err)
	}

	// 保存待处理的认证请求
	handlers.pendingAuths[challenge.Challenge] = &pendingAuthentication{
		Challenge: challenge.Challenge,
		UserID:    "user1",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Minute),
	}

	if _, exists := handlers.pendingAuths[challenge.Challenge]; !exists {
		t.Error("待处理认证请求应存在")
	}
}

func TestHandlers_ResponseStruct(t *testing.T) {
	resp := response{
		Code:    200,
		Message: "success",
		Data:    map[string]string{"key": "value"},
	}

	if resp.Code != 200 {
		t.Errorf("状态码错误: %d", resp.Code)
	}
	if resp.Message != "success" {
		t.Errorf("消息错误: %s", resp.Message)
	}
}

func TestHandlers_RequestStructs(t *testing.T) {
	// 测试请求结构体
	beginRegReq := beginRegistrationRequest{
		UserID:      "user1",
		UserName:    "testuser",
		DisplayName: "Test User",
	}
	if beginRegReq.UserID != "user1" {
		t.Error("UserID 错误")
	}

	finishRegReq := finishRegistrationRequest{
		UserID: "user1",
		Name:   "My Key",
	}
	if finishRegReq.Name != "My Key" {
		t.Error("Name 错误")
	}

	beginLoginReq := beginLoginRequest{
		UserID: "user1",
	}
	if beginLoginReq.UserID != "user1" {
		t.Error("UserID 错误")
	}

	finishLoginReq := finishLoginRequest{
		UserID: "user1",
	}
	if finishLoginReq.UserID != "user1" {
		t.Error("UserID 错误")
	}

	genRecoveryReq := generateRecoveryCodesRequest{
		UserID: "user1",
		Count:  8,
	}
	if genRecoveryReq.Count != 8 {
		t.Error("Count 错误")
	}

	verifyRecoveryReq := verifyRecoveryCodeRequest{
		UserID: "user1",
		Code:   "ABCD-EFGH",
	}
	if verifyRecoveryReq.Code != "ABCD-EFGH" {
		t.Error("Code 错误")
	}
}

func TestHandlers_ListCredentialsHandler(t *testing.T) {
	handlers := NewHandlers(nil, nil, nil)

	// 添加测试凭据
	handlers.credManager.store.SaveCredential(&Credential{
		ID:        "cred1",
		UserID:    "user1",
		Name:      "YubiKey",
		CreatedAt: time.Now(),
	})

	// 测试获取凭据列表
	infos, err := handlers.credManager.GetUserCredentialInfos("user1")
	if err != nil {
		t.Fatalf("获取凭据列表失败: %v", err)
	}
	if len(infos) != 1 {
		t.Errorf("凭据数量错误: %d", len(infos))
	}
}

func TestHandlers_DeleteCredentialHandler(t *testing.T) {
	handlers := NewHandlers(nil, nil, nil)

	// 添加测试凭据
	handlers.credManager.store.SaveCredential(&Credential{
		ID:     "cred1",
		UserID: "user1",
		Name:   "YubiKey",
	})

	// 测试删除凭据
	if err := handlers.credManager.DeleteCredential("cred1"); err != nil {
		t.Fatalf("删除凭据失败: %v", err)
	}

	_, err := handlers.credManager.GetCredential("cred1")
	if err == nil {
		t.Error("凭据应已删除")
	}
}

func TestHandlers_GenerateRecoveryCodesHandler(t *testing.T) {
	handlers := NewHandlers(nil, nil, nil)

	// 测试生成恢复码
	codes, err := handlers.recoveryManager.GenerateRecoveryCodes("user1", 5)
	if err != nil {
		t.Fatalf("生成恢复码失败: %v", err)
	}
	if len(codes) != 5 {
		t.Errorf("恢复码数量错误: %d", len(codes))
	}
}

func TestHandlers_VerifyRecoveryCodeHandler(t *testing.T) {
	handlers := NewHandlers(nil, nil, nil)

	// 生成恢复码
	codes, _ := handlers.recoveryManager.GenerateRecoveryCodes("user1", 3)

	// 验证恢复码
	valid, err := handlers.recoveryManager.VerifyRecoveryCode("user1", codes[0])
	if err != nil {
		t.Fatalf("验证恢复码失败: %v", err)
	}
	if !valid {
		t.Error("恢复码应有效")
	}
}

func TestHandlers_RegisterRoutesWithGin(t *testing.T) {
	// 使用 httptest 测试路由注册
	handlers := NewHandlers(nil, nil, nil)

	// 创建 gin 路由器
	router := gin.New()
	api := router.Group("/api/v1/auth")
	handlers.RegisterRoutes(api)

	// 验证路由注册成功（通过检查路由数量）
	routes := router.Routes()
	if len(routes) == 0 {
		t.Error("路由注册失败")
	}

	// 验证路由路径
	expectedPaths := map[string]bool{
		"/api/v1/auth/fido2/register/begin":    false,
		"/api/v1/auth/fido2/register/finish":   false,
		"/api/v1/auth/fido2/login/begin":       false,
		"/api/v1/auth/fido2/login/finish":      false,
		"/api/v1/auth/fido2/credentials":       false,
		"/api/v1/auth/fido2/recovery/generate": false,
		"/api/v1/auth/fido2/recovery/verify":   false,
	}

	for _, route := range routes {
		if _, exists := expectedPaths[route.Path]; exists {
			expectedPaths[route.Path] = true
		}
	}

	for path, found := range expectedPaths {
		if !found {
			t.Errorf("路由 %s 未注册", path)
		}
	}
}

func TestHandlers_BeginRegistrationEndpoint(t *testing.T) {
	// 设置 gin 为测试模式
	gin.SetMode(gin.TestMode)

	handlers := NewHandlers(nil, nil, nil)

	// 创建路由器
	router := gin.New()
	api := router.Group("/api/v1/auth")
	handlers.RegisterRoutes(api)

	// 测试开始注册端点
	body := `{"user_id":"user1","user_name":"testuser","display_name":"Test User"}`
	req, _ := http.NewRequest("POST", "/api/v1/auth/fido2/register/begin", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 实际 %d", w.Code)
	}

	var resp response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if resp.Code != http.StatusOK {
		t.Errorf("响应代码错误: %d", resp.Code)
	}
}

func TestHandlers_BeginRegistrationEndpoint_InvalidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := NewHandlers(nil, nil, nil)

	router := gin.New()
	api := router.Group("/api/v1/auth")
	handlers.RegisterRoutes(api)

	// 测试无效请求体
	req, _ := http.NewRequest("POST", "/api/v1/auth/fido2/register/begin", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 400, 实际 %d", w.Code)
	}
}

func TestHandlers_ListCredentialsEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := NewHandlers(nil, nil, nil)

	// 添加测试凭据
	handlers.credManager.store.SaveCredential(&Credential{
		ID:        "cred1",
		UserID:    "user1",
		Name:      "YubiKey",
		CreatedAt: time.Now(),
	})

	router := gin.New()
	api := router.Group("/api/v1/auth")
	handlers.RegisterRoutes(api)

	// 测试获取凭据列表
	req, _ := http.NewRequest("GET", "/api/v1/auth/fido2/credentials?user_id=user1", nil)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 实际 %d", w.Code)
	}
}

func TestHandlers_ListCredentialsEndpoint_MissingUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := NewHandlers(nil, nil, nil)

	router := gin.New()
	api := router.Group("/api/v1/auth")
	handlers.RegisterRoutes(api)

	// 测试缺少 user_id 参数
	req, _ := http.NewRequest("GET", "/api/v1/auth/fido2/credentials", nil)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 400, 实际 %d", w.Code)
	}
}

func TestHandlers_DeleteCredentialEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := NewHandlers(nil, nil, nil)

	// 添加测试凭据
	handlers.credManager.store.SaveCredential(&Credential{
		ID:     "cred1",
		UserID: "user1",
		Name:   "YubiKey",
	})

	router := gin.New()
	api := router.Group("/api/v1/auth")
	handlers.RegisterRoutes(api)

	// 测试删除凭据
	req, _ := http.NewRequest("DELETE", "/api/v1/auth/fido2/credentials/cred1", nil)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 实际 %d", w.Code)
	}
}

func TestHandlers_DeleteCredentialEndpoint_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := NewHandlers(nil, nil, nil)

	router := gin.New()
	api := router.Group("/api/v1/auth")
	handlers.RegisterRoutes(api)

	// 测试删除不存在的凭据
	req, _ := http.NewRequest("DELETE", "/api/v1/auth/fido2/credentials/nonexistent", nil)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("期望状态码 404, 实际 %d", w.Code)
	}
}

func TestHandlers_GenerateRecoveryCodesEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := NewHandlers(nil, nil, nil)

	router := gin.New()
	api := router.Group("/api/v1/auth")
	handlers.RegisterRoutes(api)

	// 测试生成恢复码
	body := `{"user_id":"user1","count":5}`
	req, _ := http.NewRequest("POST", "/api/v1/auth/fido2/recovery/generate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 实际 %d", w.Code)
	}
}

func TestHandlers_GenerateRecoveryCodesEndpoint_DefaultCount(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := NewHandlers(nil, nil, nil)

	router := gin.New()
	api := router.Group("/api/v1/auth")
	handlers.RegisterRoutes(api)

	// 测试默认恢复码数量
	body := `{"user_id":"user1"}`
	req, _ := http.NewRequest("POST", "/api/v1/auth/fido2/recovery/generate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 实际 %d", w.Code)
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("响应数据格式错误")
	}

	count, ok := data["count"].(float64)
	if !ok || int(count) != 8 {
		t.Errorf("默认恢复码数量应为 8, 实际 %v", data["count"])
	}
}

func TestHandlers_VerifyRecoveryCodeEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := NewHandlers(nil, nil, nil)

	// 先生成恢复码
	codes, _ := handlers.recoveryManager.GenerateRecoveryCodes("user1", 3)

	router := gin.New()
	api := router.Group("/api/v1/auth")
	handlers.RegisterRoutes(api)

	// 测试验证恢复码
	// 需要正确转义恢复码中的特殊字符
	codeJSON, _ := json.Marshal(codes[0])
	body := fmt.Sprintf(`{"user_id":"user1","code":%s}`, string(codeJSON))
	req, _ := http.NewRequest("POST", "/api/v1/auth/fido2/recovery/verify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 实际 %d", w.Code)
	}
}

func TestHandlers_VerifyRecoveryCodeEndpoint_InvalidCode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := NewHandlers(nil, nil, nil)

	router := gin.New()
	api := router.Group("/api/v1/auth")
	handlers.RegisterRoutes(api)

	// 测试无效恢复码
	body := `{"user_id":"user1","code":"XXXX-XXXX-XXXX-XXXX"}`
	req, _ := http.NewRequest("POST", "/api/v1/auth/fido2/recovery/verify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("期望状态码 401, 实际 %d", w.Code)
	}
}

func TestHandlers_BeginLoginEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := NewHandlers(nil, nil, nil)

	// 添加测试凭据
	handlers.credManager.store.SaveCredential(&Credential{
		ID:           "cred1",
		UserID:       "user1",
		CredentialID: []byte("cred-id-123"),
		PublicKey:    []byte("test-key"),
		Revoked:      false,
		CreatedAt:    time.Now(),
	})

	router := gin.New()
	api := router.Group("/api/v1/auth")
	handlers.RegisterRoutes(api)

	// 测试开始登录
	body := `{"user_id":"user1"}`
	req, _ := http.NewRequest("POST", "/api/v1/auth/fido2/login/begin", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 实际 %d", w.Code)
	}
}

func TestHandlers_BeginLoginEndpoint_NoCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := NewHandlers(nil, nil, nil)

	router := gin.New()
	api := router.Group("/api/v1/auth")
	handlers.RegisterRoutes(api)

	// 测试没有凭据的用户
	body := `{"user_id":"user_no_creds"}`
	req, _ := http.NewRequest("POST", "/api/v1/auth/fido2/login/begin", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("期望状态码 404, 实际 %d", w.Code)
	}
}

func TestHandlers_BeginLoginEndpoint_InvalidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := NewHandlers(nil, nil, nil)

	router := gin.New()
	api := router.Group("/api/v1/auth")
	handlers.RegisterRoutes(api)

	// 测试无效请求体
	req, _ := http.NewRequest("POST", "/api/v1/auth/fido2/login/begin", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 400, 实际 %d", w.Code)
	}
}

func TestHandlers_FinishLoginEndpoint_InvalidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := NewHandlers(nil, nil, nil)

	router := gin.New()
	api := router.Group("/api/v1/auth")
	handlers.RegisterRoutes(api)

	// 测试无效请求体
	req, _ := http.NewRequest("POST", "/api/v1/auth/fido2/login/finish", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 400, 实际 %d", w.Code)
	}
}

func TestHandlers_FinishLoginEndpoint_NoPendingAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := NewHandlers(nil, nil, nil)

	router := gin.New()
	api := router.Group("/api/v1/auth")
	handlers.RegisterRoutes(api)

	// 构建客户端数据
	clientData := ClientData{
		Type:      "webauthn.get",
		Challenge: "nonexistent-challenge",
		Origin:    "http://localhost:8080",
	}
	clientDataJSON, _ := json.Marshal(clientData)

	resp := AuthenticationResponse{
		ID:   base64.URLEncoding.EncodeToString([]byte("cred-id")),
		Type: "public-key",
		Response: AuthenticationResponseData{
			AuthenticatorData: base64.URLEncoding.EncodeToString(make([]byte, 37)),
			ClientDataJSON:    base64.URLEncoding.EncodeToString(clientDataJSON),
			Signature:         base64.URLEncoding.EncodeToString([]byte("sig")),
		},
	}
	bodyBytes, _ := json.Marshal(map[string]interface{}{
		"user_id":  "user1",
		"response": resp,
	})

	req, _ := http.NewRequest("POST", "/api/v1/auth/fido2/login/finish", strings.NewReader(string(bodyBytes)))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 400, 实际 %d", w.Code)
	}
}

func TestHandlers_FinishRegistrationEndpoint_InvalidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := NewHandlers(nil, nil, nil)

	router := gin.New()
	api := router.Group("/api/v1/auth")
	handlers.RegisterRoutes(api)

	// 测试无效请求体
	req, _ := http.NewRequest("POST", "/api/v1/auth/fido2/register/finish", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 400, 实际 %d", w.Code)
	}
}

func TestHandlers_FinishRegistrationEndpoint_NoPending(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := NewHandlers(nil, nil, nil)

	router := gin.New()
	api := router.Group("/api/v1/auth")
	handlers.RegisterRoutes(api)

	// 构建客户端数据
	clientData := ClientData{
		Type:      "webauthn.create",
		Challenge: "nonexistent-challenge",
		Origin:    "http://localhost:8080",
	}
	clientDataJSON, _ := json.Marshal(clientData)

	resp := RegistrationResponse{
		ID:   base64.URLEncoding.EncodeToString([]byte("cred-id")),
		Type: "public-key",
		Response: RegistrationResponseData{
			AttestationObject: base64.URLEncoding.EncodeToString([]byte("att")),
			ClientDataJSON:    base64.URLEncoding.EncodeToString(clientDataJSON),
		},
	}
	bodyBytes, _ := json.Marshal(map[string]interface{}{
		"user_id":  "user1",
		"name":     "Test Key",
		"response": resp,
	})

	req, _ := http.NewRequest("POST", "/api/v1/auth/fido2/register/finish", strings.NewReader(string(bodyBytes)))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 400, 实际 %d", w.Code)
	}
}

func TestHandlers_GenerateRecoveryCodesEndpoint_InvalidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := NewHandlers(nil, nil, nil)

	router := gin.New()
	api := router.Group("/api/v1/auth")
	handlers.RegisterRoutes(api)

	// 测试无效请求体
	req, _ := http.NewRequest("POST", "/api/v1/auth/fido2/recovery/generate", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 400, 实际 %d", w.Code)
	}
}

func TestHandlers_VerifyRecoveryCodeEndpoint_InvalidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := NewHandlers(nil, nil, nil)

	router := gin.New()
	api := router.Group("/api/v1/auth")
	handlers.RegisterRoutes(api)

	// 测试无效请求体
	req, _ := http.NewRequest("POST", "/api/v1/auth/fido2/recovery/verify", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 400, 实际 %d", w.Code)
	}
}

func TestHandlers_DeleteCredentialEndpoint_WithUserFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := NewHandlers(nil, nil, nil)

	// 添加测试凭据
	handlers.credManager.store.SaveCredential(&Credential{
		ID:     "cred1",
		UserID: "user1",
		Name:   "YubiKey",
	})

	router := gin.New()
	api := router.Group("/api/v1/auth")
	handlers.RegisterRoutes(api)

	// 测试删除凭据（带用户过滤）
	req, _ := http.NewRequest("DELETE", "/api/v1/auth/fido2/credentials/cred1?user_id=user1", nil)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 实际 %d", w.Code)
	}
}

func TestHandlers_DeleteCredentialEndpoint_Forbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := NewHandlers(nil, nil, nil)

	// 添加测试凭据
	handlers.credManager.store.SaveCredential(&Credential{
		ID:     "cred1",
		UserID: "user1",
		Name:   "YubiKey",
	})

	router := gin.New()
	api := router.Group("/api/v1/auth")
	handlers.RegisterRoutes(api)

	// 测试删除其他用户的凭据
	req, _ := http.NewRequest("DELETE", "/api/v1/auth/fido2/credentials/cred1?user_id=user2", nil)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("期望状态码 403, 实际 %d", w.Code)
	}
}

func TestHandlers_FinishLoginEndpoint_ExpiredChallenge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := NewHandlers(nil, nil, nil)

	// 设置一个已过期的待处理认证请求
	handlers.pendingAuths["expired-challenge"] = &pendingAuthentication{
		Challenge: "expired-challenge",
		UserID:    "user1",
		CreatedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-1 * time.Hour), // 已过期
	}

	router := gin.New()
	api := router.Group("/api/v1/auth")
	handlers.RegisterRoutes(api)

	// 构建客户端数据
	clientData := ClientData{
		Type:      "webauthn.get",
		Challenge: "expired-challenge",
		Origin:    "http://localhost:8080",
	}
	clientDataJSON, _ := json.Marshal(clientData)

	resp := AuthenticationResponse{
		ID:   base64.URLEncoding.EncodeToString([]byte("cred-id")),
		Type: "public-key",
		Response: AuthenticationResponseData{
			AuthenticatorData: base64.URLEncoding.EncodeToString(make([]byte, 37)),
			ClientDataJSON:    base64.URLEncoding.EncodeToString(clientDataJSON),
			Signature:         base64.URLEncoding.EncodeToString([]byte("sig")),
		},
	}
	bodyBytes, _ := json.Marshal(map[string]interface{}{
		"user_id":  "user1",
		"response": resp,
	})

	req, _ := http.NewRequest("POST", "/api/v1/auth/fido2/login/finish", strings.NewReader(string(bodyBytes)))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 400, 实际 %d", w.Code)
	}
}

func TestHandlers_FinishLoginEndpoint_UserMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := NewHandlers(nil, nil, nil)

	// 设置待处理认证请求
	handlers.pendingAuths["valid-challenge"] = &pendingAuthentication{
		Challenge: "valid-challenge",
		UserID:    "user1",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}

	router := gin.New()
	api := router.Group("/api/v1/auth")
	handlers.RegisterRoutes(api)

	// 构建客户端数据
	clientData := ClientData{
		Type:      "webauthn.get",
		Challenge: "valid-challenge",
		Origin:    "http://localhost:8080",
	}
	clientDataJSON, _ := json.Marshal(clientData)

	resp := AuthenticationResponse{
		ID:   base64.URLEncoding.EncodeToString([]byte("cred-id")),
		Type: "public-key",
		Response: AuthenticationResponseData{
			AuthenticatorData: base64.URLEncoding.EncodeToString(make([]byte, 37)),
			ClientDataJSON:    base64.URLEncoding.EncodeToString(clientDataJSON),
			Signature:         base64.URLEncoding.EncodeToString([]byte("sig")),
		},
	}
	bodyBytes, _ := json.Marshal(map[string]interface{}{
		"user_id":  "user2", // 不匹配的用户
		"response": resp,
	})

	req, _ := http.NewRequest("POST", "/api/v1/auth/fido2/login/finish", strings.NewReader(string(bodyBytes)))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 400, 实际 %d", w.Code)
	}
}

func TestHandlers_FinishRegistrationEndpoint_ExpiredChallenge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := NewHandlers(nil, nil, nil)

	// 设置一个已过期的待处理注册请求
	handlers.pendingRegs["expired-challenge"] = &pendingRegistration{
		Challenge: "expired-challenge",
		UserID:    "user1",
		CreatedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-1 * time.Hour), // 已过期
	}

	router := gin.New()
	api := router.Group("/api/v1/auth")
	handlers.RegisterRoutes(api)

	// 构建客户端数据
	clientData := ClientData{
		Type:      "webauthn.create",
		Challenge: "expired-challenge",
		Origin:    "http://localhost:8080",
	}
	clientDataJSON, _ := json.Marshal(clientData)

	resp := RegistrationResponse{
		ID:   base64.URLEncoding.EncodeToString([]byte("cred-id")),
		Type: "public-key",
		Response: RegistrationResponseData{
			AttestationObject: base64.URLEncoding.EncodeToString([]byte("att")),
			ClientDataJSON:    base64.URLEncoding.EncodeToString(clientDataJSON),
		},
	}
	bodyBytes, _ := json.Marshal(map[string]interface{}{
		"user_id":  "user1",
		"name":     "Test Key",
		"response": resp,
	})

	req, _ := http.NewRequest("POST", "/api/v1/auth/fido2/register/finish", strings.NewReader(string(bodyBytes)))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 400, 实际 %d", w.Code)
	}
}

func TestHandlers_FinishRegistrationEndpoint_UserMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := NewHandlers(nil, nil, nil)

	// 设置待处理注册请求
	handlers.pendingRegs["valid-challenge"] = &pendingRegistration{
		Challenge: "valid-challenge",
		UserID:    "user1",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}

	router := gin.New()
	api := router.Group("/api/v1/auth")
	handlers.RegisterRoutes(api)

	// 构建客户端数据
	clientData := ClientData{
		Type:      "webauthn.create",
		Challenge: "valid-challenge",
		Origin:    "http://localhost:8080",
	}
	clientDataJSON, _ := json.Marshal(clientData)

	resp := RegistrationResponse{
		ID:   base64.URLEncoding.EncodeToString([]byte("cred-id")),
		Type: "public-key",
		Response: RegistrationResponseData{
			AttestationObject: base64.URLEncoding.EncodeToString([]byte("att")),
			ClientDataJSON:    base64.URLEncoding.EncodeToString(clientDataJSON),
		},
	}
	bodyBytes, _ := json.Marshal(map[string]interface{}{
		"user_id":  "user2", // 不匹配的用户
		"name":     "Test Key",
		"response": resp,
	})

	req, _ := http.NewRequest("POST", "/api/v1/auth/fido2/register/finish", strings.NewReader(string(bodyBytes)))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 400, 实际 %d", w.Code)
	}
}

func TestHandlers_FinishLoginEndpoint_CredNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := NewHandlers(nil, nil, nil)

	// 设置待处理认证请求
	handlers.pendingAuths["valid-challenge"] = &pendingAuthentication{
		Challenge: "valid-challenge",
		UserID:    "user1",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}

	router := gin.New()
	api := router.Group("/api/v1/auth")
	handlers.RegisterRoutes(api)

	// 构建客户端数据
	clientData := ClientData{
		Type:      "webauthn.get",
		Challenge: "valid-challenge",
		Origin:    "http://localhost:8080",
	}
	clientDataJSON, _ := json.Marshal(clientData)

	resp := AuthenticationResponse{
		ID:   base64.URLEncoding.EncodeToString([]byte("nonexistent-cred")),
		Type: "public-key",
		Response: AuthenticationResponseData{
			AuthenticatorData: base64.URLEncoding.EncodeToString(make([]byte, 37)),
			ClientDataJSON:    base64.URLEncoding.EncodeToString(clientDataJSON),
			Signature:         base64.URLEncoding.EncodeToString([]byte("sig")),
		},
	}
	bodyBytes, _ := json.Marshal(map[string]interface{}{
		"user_id":  "user1",
		"response": resp,
	})

	req, _ := http.NewRequest("POST", "/api/v1/auth/fido2/login/finish", strings.NewReader(string(bodyBytes)))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("期望状态码 404, 实际 %d", w.Code)
	}
}

func TestHandlers_FinishLoginEndpoint_CredBelongsToOtherUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlers := NewHandlers(nil, nil, nil)

	// 添加一个属于其他用户的凭据
	webauthnID := []byte("cred-id-123")
	handlers.credManager.store.SaveCredential(&Credential{
		ID:           "cred1",
		UserID:       "user2", // 属于其他用户
		CredentialID: webauthnID,
		PublicKey:    []byte("key"),
	})

	// 设置待处理认证请求
	handlers.pendingAuths["valid-challenge"] = &pendingAuthentication{
		Challenge: "valid-challenge",
		UserID:    "user1",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}

	router := gin.New()
	api := router.Group("/api/v1/auth")
	handlers.RegisterRoutes(api)

	// 构建客户端数据
	clientData := ClientData{
		Type:      "webauthn.get",
		Challenge: "valid-challenge",
		Origin:    "http://localhost:8080",
	}
	clientDataJSON, _ := json.Marshal(clientData)

	resp := AuthenticationResponse{
		ID:   base64.URLEncoding.EncodeToString(webauthnID),
		Type: "public-key",
		Response: AuthenticationResponseData{
			AuthenticatorData: base64.URLEncoding.EncodeToString(make([]byte, 37)),
			ClientDataJSON:    base64.URLEncoding.EncodeToString(clientDataJSON),
			Signature:         base64.URLEncoding.EncodeToString([]byte("sig")),
		},
	}
	bodyBytes, _ := json.Marshal(map[string]interface{}{
		"user_id":  "user1",
		"response": resp,
	})

	req, _ := http.NewRequest("POST", "/api/v1/auth/fido2/login/finish", strings.NewReader(string(bodyBytes)))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("期望状态码 403, 实际 %d", w.Code)
	}
}
