package web

import (
	"testing"
)

func TestDefaultSecurityConfig(t *testing.T) {
	config := DefaultSecurityConfig()

	if config == nil {
		t.Fatal("DefaultSecurityConfig should not return nil")
	}

	// 验证默认配置值
	if config.RateLimitRPS <= 0 {
		t.Error("RateLimitRPS should be positive")
	}

	if len(config.CSRFKey) == 0 {
		t.Error("CSRFKey should not be empty")
	}
}

func TestSecurityConfigFields(t *testing.T) {
	config := DefaultSecurityConfig()

	// 验证必要字段
	if config.AllowedOrigins == nil {
		t.Error("AllowedOrigins should not be nil")
	}

	// 验证 CSRF 配置
	if len(config.CSRFKey) < 32 {
		t.Error("CSRFKey should be at least 32 bytes")
	}
}

func TestSecurityConfigOrigins(t *testing.T) {
	config := DefaultSecurityConfig()

	// 验证默认允许的源
	if len(config.AllowedOrigins) == 0 {
		t.Error("Should have default allowed origins")
	}

	// 默认应该包含 localhost
	found := false
	for _, origin := range config.AllowedOrigins {
		if origin == "http://localhost:8080" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Default origins should include localhost:8080")
	}
}

func TestRateLimitConfig(t *testing.T) {
	config := DefaultSecurityConfig()

	// 验证速率限制配置合理性
	if config.RateLimitRPS > 10000 {
		t.Error("RateLimitRPS seems too high")
	}

	if config.RateLimitRPS < 1 {
		t.Error("RateLimitRPS should be at least 1")
	}
}

func TestCSRFKeyLength(t *testing.T) {
	config := DefaultSecurityConfig()

	// CSRFKey 应该至少 32 字节用于 HMAC-SHA256
	if len(config.CSRFKey) < 32 {
		t.Errorf("CSRFKey should be at least 32 bytes, got %d", len(config.CSRFKey))
	}
}

func TestEnableRateLimit(t *testing.T) {
	config := DefaultSecurityConfig()

	// 默认应该启用速率限制
	if !config.EnableRateLimit {
		t.Error("EnableRateLimit should be true by default")
	}
}
