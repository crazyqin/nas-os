// Package mobileapi 提供移动端远程管理API服务
package mobileapi

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWT相关错误.
var (
	ErrInvalidToken     = errors.New("invalid token")         // 无效令牌
	ErrTokenExpired     = errors.New("token expired")         // 令牌已过期
	ErrTokenRevoked     = errors.New("token revoked")         // 令牌已撤销
	ErrDeviceNotFound   = errors.New("device not found")      // 设备未找到
	ErrDeviceBlocked    = errors.New("device blocked")        // 设备已封禁
	ErrInvalidSignature = errors.New("invalid signature")     // 无效签名
	ErrRefreshExpired   = errors.New("refresh token expired") // 刷新令牌已过期
)

// AuthConfig 认证配置.
type AuthConfig struct {
	JWTSecret          string        `json:"-"`                  // JWT密钥
	AccessTokenExpiry  time.Duration `json:"accessTokenExpiry"`  // 访问令牌过期时间
	RefreshTokenExpiry time.Duration `json:"refreshTokenExpiry"` // 刷新令牌过期时间
	Issuer             string        `json:"issuer"`             // 签发者
	MaxSessions        int           `json:"maxSessions"`        // 最大会话数
}

// DefaultAuthConfig 返回默认认证配置.
func DefaultAuthConfig() *AuthConfig {
	return &AuthConfig{
		AccessTokenExpiry:  1 * time.Hour,       // 1小时
		RefreshTokenExpiry: 30 * 24 * time.Hour, // 30天
		Issuer:             "nas-os-mobile",
		MaxSessions:        10,
	}
}

// JWTClaims JWT声明.
type JWTClaims struct {
	UserID   string `json:"userId"`
	DeviceID string `json:"deviceId"`
	jwt.RegisteredClaims
}

// AuthService 认证服务.
type AuthService struct {
	mu            sync.RWMutex
	config        *AuthConfig
	devices       map[string]*MobileDevice // deviceID -> device
	sessions      map[string]*Session      // sessionID -> session
	refreshTokens map[string]*RefreshToken // token -> refreshToken
}

// NewAuthService 创建认证服务.
func NewAuthService(config *AuthConfig) *AuthService {
	if config == nil {
		config = DefaultAuthConfig()
	}
	return &AuthService{
		config:        config,
		devices:       make(map[string]*MobileDevice),
		sessions:      make(map[string]*Session),
		refreshTokens: make(map[string]*RefreshToken),
	}
}

// RegisterDevice 注册新设备.
func (s *AuthService) RegisterDevice(device *MobileDevice) (*AuthToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 设置默认值
	if device.ID == "" {
		device.ID = generateID()
	}
	device.RegisteredAt = time.Now()
	device.UpdatedAt = time.Now()
	device.LastSeenAt = time.Now()
	if device.Status == "" {
		device.Status = StatusOnline
	}

	// 检查设备是否已存在
	if existing, ok := s.devices[device.ID]; ok {
		// 更新现有设备信息
		existing.DeviceName = device.DeviceName
		existing.OSVersion = device.OSVersion
		existing.AppVersion = device.AppVersion
		existing.PushToken = device.PushToken
		existing.PushProvider = device.PushProvider
		existing.LastSeenAt = time.Now()
		existing.UpdatedAt = time.Now()
		device = existing
	} else {
		s.devices[device.ID] = device
	}

	// 生成令牌
	token, err := s.generateTokenLocked(device.UserID, device.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return token, nil
}

// Authenticate 设备认证.
func (s *AuthService) Authenticate(deviceID, userID string) (*AuthToken, error) {
	s.mu.RLock()
	device, ok := s.devices[deviceID]
	s.mu.RUnlock()

	if !ok {
		return nil, ErrDeviceNotFound
	}

	if device.Status == StatusBlocked {
		return nil, ErrDeviceBlocked
	}

	// 更新最后在线时间
	s.mu.Lock()
	device.LastSeenAt = time.Now()
	device.Status = StatusOnline
	s.mu.Unlock()

	// 生成令牌
	token, err := s.generateToken(userID, deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return token, nil
}

// ValidateToken 验证访问令牌.
func (s *AuthService) ValidateToken(tokenString string) (*JWTClaims, error) {
	claims := &JWTClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidSignature
		}
		return []byte(s.config.JWTSecret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	if !token.Valid {
		return nil, ErrInvalidToken
	}

	// 检查设备状态
	s.mu.RLock()
	device, ok := s.devices[claims.DeviceID]
	s.mu.RUnlock()

	if ok && device.Status == StatusBlocked {
		return nil, ErrDeviceBlocked
	}

	return claims, nil
}

// RefreshAccessToken 刷新访问令牌.
func (s *AuthService) RefreshAccessToken(refreshTokenStr string) (*AuthToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 查找刷新令牌
	refreshToken, ok := s.refreshTokens[refreshTokenStr]
	if !ok {
		return nil, ErrInvalidToken
	}

	if refreshToken.Revoked {
		return nil, ErrTokenRevoked
	}

	if time.Now().After(refreshToken.ExpiresAt) {
		return nil, ErrRefreshExpired
	}

	// 检查设备状态
	device, ok := s.devices[refreshToken.DeviceID]
	if !ok {
		return nil, ErrDeviceNotFound
	}
	if device.Status == StatusBlocked {
		return nil, ErrDeviceBlocked
	}

	// 生成新的访问令牌
	token, err := s.generateTokenLocked(refreshToken.UserID, refreshToken.DeviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// 撤销旧的刷新令牌
	refreshToken.Revoked = true

	return token, nil
}

// RevokeToken 撤销令牌.
func (s *AuthService) RevokeToken(refreshTokenStr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	refreshToken, ok := s.refreshTokens[refreshTokenStr]
	if !ok {
		return ErrInvalidToken
	}

	refreshToken.Revoked = true
	return nil
}

// RevokeAllDeviceTokens 撤销设备所有令牌.
func (s *AuthService) RevokeAllDeviceTokens(deviceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, rt := range s.refreshTokens {
		if rt.DeviceID == deviceID {
			rt.Revoked = true
		}
	}
}

// GetDevice 获取设备信息.
func (s *AuthService) GetDevice(deviceID string) (*MobileDevice, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	device, ok := s.devices[deviceID]
	return device, ok
}

// ListDevices 列出用户设备.
func (s *AuthService) ListDevices(userID string) []*MobileDevice {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var devices []*MobileDevice
	for _, d := range s.devices {
		if d.UserID == userID {
			devices = append(devices, d)
		}
	}
	return devices
}

// RemoveDevice 移除设备.
func (s *AuthService) RemoveDevice(deviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.devices[deviceID]; !ok {
		return ErrDeviceNotFound
	}

	// 撤销所有相关令牌
	for _, rt := range s.refreshTokens {
		if rt.DeviceID == deviceID {
			rt.Revoked = true
		}
	}

	// 删除设备
	delete(s.devices, deviceID)
	return nil
}

// BlockDevice 封禁设备.
func (s *AuthService) BlockDevice(deviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	device, ok := s.devices[deviceID]
	if !ok {
		return ErrDeviceNotFound
	}

	device.Status = StatusBlocked
	device.UpdatedAt = time.Now()

	// 撤销所有相关令牌
	for _, rt := range s.refreshTokens {
		if rt.DeviceID == deviceID {
			rt.Revoked = true
		}
	}

	return nil
}

// UnblockDevice 解封设备.
func (s *AuthService) UnblockDevice(deviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	device, ok := s.devices[deviceID]
	if !ok {
		return ErrDeviceNotFound
	}

	device.Status = StatusOffline
	device.UpdatedAt = time.Now()
	return nil
}

// CreateSession 创建会话.
func (s *AuthService) CreateSession(userID, deviceID, ipAddress, userAgent string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := &Session{
		ID:           generateID(),
		UserID:       userID,
		DeviceID:     deviceID,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		ExpiresAt:    time.Now().Add(s.config.RefreshTokenExpiry),
		LastActiveAt: time.Now(),
		CreatedAt:    time.Now(),
	}

	s.sessions[session.ID] = session
	return session
}

// GetSession 获取会话.
func (s *AuthService) GetSession(sessionID string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[sessionID]
	return session, ok
}

// ListSessions 列出用户会话.
func (s *AuthService) ListSessions(userID string) []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sessions []*Session
	for _, s := range s.sessions {
		if s.UserID == userID && !s.Revoked {
			sessions = append(sessions, s)
		}
	}
	return sessions
}

// RevokeSession 撤销会话.
func (s *AuthService) RevokeSession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if session, ok := s.sessions[sessionID]; ok {
		session.Revoked = true
	}
}

// generateToken 生成令牌（需要外部持有锁）.
func (s *AuthService) generateToken(userID, deviceID string) (*AuthToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.generateTokenLocked(userID, deviceID)
}

// generateTokenLocked 生成令牌（内部方法，调用者需持有锁）.
func (s *AuthService) generateTokenLocked(userID, deviceID string) (*AuthToken, error) {
	now := time.Now()
	expiresAt := now.Add(s.config.AccessTokenExpiry)

	// 创建JWT
	claims := &JWTClaims{
		UserID:   userID,
		DeviceID: deviceID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    s.config.Issuer,
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessToken, err := token.SignedString([]byte(s.config.JWTSecret))
	if err != nil {
		return nil, err
	}

	// 生成刷新令牌
	refreshTokenStr := generateID()
	refreshToken := &RefreshToken{
		Token:     refreshTokenStr,
		DeviceID:  deviceID,
		UserID:    userID,
		ExpiresAt: now.Add(s.config.RefreshTokenExpiry),
		CreatedAt: now,
	}
	s.refreshTokens[refreshTokenStr] = refreshToken

	// 创建或更新会话
	session := &Session{
		ID:           generateID(),
		UserID:       userID,
		DeviceID:     deviceID,
		AccessToken:  accessToken,
		RefreshToken: refreshTokenStr,
		ExpiresAt:    expiresAt,
		LastActiveAt: now,
		CreatedAt:    now,
	}
	s.sessions[session.ID] = session

	return &AuthToken{
		AccessToken:  accessToken,
		RefreshToken: refreshTokenStr,
		TokenType:    "Bearer",
		ExpiresAt:    expiresAt,
		DeviceID:     deviceID,
		CreatedAt:    now,
	}, nil
}

// generateID 生成随机ID.
func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
