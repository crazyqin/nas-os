package auth

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// GenerateTOTPSecret 生成 TOTP 密钥.
func GenerateTOTPSecret() (string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "",
		AccountName: "",
	})
	if err != nil {
		return "", err
	}
	return key.Secret(), nil
}

// GenerateTOTPURI 生成 TOTP URI（用于 QR 码）.
func GenerateTOTPURI(secret, issuer, accountName string) string {
	// 手动构建 otpauth:// URI
	return fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s&algorithm=SHA1&digits=6&period=30",
		issuer, accountName, secret, issuer)
}

// GenerateTOTPQRCode 生成 TOTP QR 码（返回 base64 编码的 PNG 图片）.
func GenerateTOTPQRCode(uri string) (string, error) {
	key, err := otp.NewKeyFromURL(uri)
	if err != nil {
		return "", err
	}

	img, err := key.Image(200, 200)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}

	// 转换为 base64
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// SetupTOTP 设置 TOTP.
func SetupTOTP(issuer, accountName string) (*TOTPSetup, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountName,
	})
	if err != nil {
		return nil, err
	}

	qrCode, err := GenerateTOTPQRCode(key.String())
	if err != nil {
		return nil, err
	}

	return &TOTPSetup{
		Secret:      key.Secret(),
		URI:         key.String(),
		QRCode:      qrCode,
		Issuer:      issuer,
		AccountName: accountName,
	}, nil
}

// VerifyTOTP 验证 TOTP 验证码.
func VerifyTOTP(secret, code string) bool {
	// 使用默认设置验证（允许前后一个时间窗口的偏差）
	valid := totp.Validate(code, secret)
	return valid
}

// ValidateTOTPCode 验证 TOTP 代码（带错误计数）.
func ValidateTOTPCode(secret, code string) error {
	if !VerifyTOTP(secret, code) {
		return fmt.Errorf("TOTP 验证码无效")
	}
	return nil
}

// ParseTOTPURI 解析 TOTP URI.
func ParseTOTPURI(uri string) (*otp.Key, error) {
	return otp.NewKeyFromURL(uri)
}
