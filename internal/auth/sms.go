package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
	"time"
)

// SMSProvider 短信服务提供商接口
type SMSProvider interface {
	Send(phone, code string) error
}

// SMSManager 短信验证码管理器
type SMSManager struct {
	mu       sync.RWMutex
	codes    map[string]*SMSCode // phone -> SMSCode
	provider SMSProvider
	codeLen  int
	validity time.Duration
	maxAttempts int
}

// NewSMSManager 创建短信管理器
func NewSMSManager(provider SMSProvider) *SMSManager {
	return &SMSManager{
		codes:       make(map[string]*SMSCode),
		provider:    provider,
		codeLen:     6,
		validity:    5 * time.Minute,
		maxAttempts: 3,
	}
}

// generateCode 生成随机验证码
func (m *SMSManager) generateCode() (string, error) {
	const digits = "0123456789"
	code := make([]byte, m.codeLen)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		code[i] = digits[n.Int64()]
	}
	return string(code), nil
}

// SendCode 发送短信验证码
func (m *SMSManager) SendCode(phone string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否正在冷却期
	if existing, ok := m.codes[phone]; ok {
		if time.Now().Before(existing.ExpiresAt) {
			// 还在有效期内，检查发送频率（30 秒内只能发送一次）
			if time.Since(existing.ExpiresAt.Add(-m.validity)) < 30*time.Second {
				return fmt.Errorf("验证码已发送，请稍后再试")
			}
		}
	}

	// 生成验证码
	code, err := m.generateCode()
	if err != nil {
		return err
	}

	// 发送短信
	if m.provider != nil {
		if err := m.provider.Send(phone, code); err != nil {
			return fmt.Errorf("发送短信失败：%w", err)
		}
	} else {
		// 开发环境：打印验证码到日志
		fmt.Printf("[SMS] 验证码 [%s]: %s\n", phone, code)
	}

	// 存储验证码
	m.codes[phone] = &SMSCode{
		Phone:     phone,
		Code:      code,
		ExpiresAt: time.Now().Add(m.validity),
		Attempts:  0,
	}

	// 清理过期验证码（每 10 分钟清理一次）
	go m.cleanupExpired()

	return nil
}

// VerifyCode 验证短信验证码
func (m *SMSManager) VerifyCode(phone, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	smsCode, ok := m.codes[phone]
	if !ok {
		return fmt.Errorf("请先获取验证码")
	}

	// 检查是否过期
	if time.Now().After(smsCode.ExpiresAt) {
		delete(m.codes, phone)
		return fmt.Errorf("验证码已过期")
	}

	// 检查尝试次数
	if smsCode.Attempts >= m.maxAttempts {
		delete(m.codes, phone)
		return fmt.Errorf("%s", ErrMFATooManyAttempts)
	}

	// 验证验证码
	if smsCode.Code != code {
		smsCode.Attempts++
		return fmt.Errorf("验证码错误")
	}

	// 验证成功，删除验证码
	delete(m.codes, phone)
	return nil
}

// cleanupExpired 清理过期的验证码
func (m *SMSManager) cleanupExpired() {
	time.Sleep(10 * time.Minute)
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for phone, smsCode := range m.codes {
		if now.After(smsCode.ExpiresAt) {
			delete(m.codes, phone)
		}
	}
}

// MockSMSProvider 模拟短信提供商（用于测试）
type MockSMSProvider struct {
	Codes map[string]string // phone -> code
}

func (p *MockSMSProvider) Send(phone, code string) error {
	if p.Codes == nil {
		p.Codes = make(map[string]string)
	}
	p.Codes[phone] = code
	fmt.Printf("[MockSMS] 发送到 %s 的验证码：%s\n", phone, code)
	return nil
}

// AliyunSMSProvider 阿里云短信提供商
type AliyunSMSProvider struct {
	AccessKeyID     string
	AccessKeySecret string
	SignName        string
	TemplateCode    string
}

func (p *AliyunSMSProvider) Send(phone, code string) error {
	// TODO: 实现阿里云短信 API 调用
	// 这里需要集成阿里云 SMS SDK
	fmt.Printf("[AliyunSMS] 发送到 %s 的验证码：%s\n", phone, code)
	return nil
}

// TencentSMSProvider 腾讯云短信提供商
type TencentSMSProvider struct {
	SecretID     string
	SecretKey    string
	AppID        string
	SignName     string
	TemplateID   string
}

func (p *TencentSMSProvider) Send(phone, code string) error {
	// TODO: 实现腾讯云短信 API 调用
	// 这里需要集成腾讯云 SMS SDK
	fmt.Printf("[TencentSMS] 发送到 %s 的验证码：%s\n", phone, code)
	return nil
}
