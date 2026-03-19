package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// SMSProvider 短信服务提供商接口
type SMSProvider interface {
	Send(phone, code string) error
}

// SMSManager 短信验证码管理器
type SMSManager struct {
	mu          sync.RWMutex
	codes       map[string]*SMSCode // phone -> SMSCode
	provider    SMSProvider
	codeLen     int
	validity    time.Duration
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
	RegionID        string // 区域 ID，如 cn-hangzhou
}

// AliyunSMSResponse 阿里云短信响应
type AliyunSMSResponse struct {
	Message   string `json:"Message"`
	RequestID string `json:"RequestId"`
	Code      string `json:"Code"`
	BizID     string `json:"BizId"`
}

func NewAliyunSMSProvider(accessKeyID, accessKeySecret, signName, templateCode string) *AliyunSMSProvider {
	return &AliyunSMSProvider{
		AccessKeyID:     accessKeyID,
		AccessKeySecret: accessKeySecret,
		SignName:        signName,
		TemplateCode:    templateCode,
		RegionID:        "cn-hangzhou",
	}
}

func (p *AliyunSMSProvider) SetRegion(regionID string) {
	p.RegionID = regionID
}

func (p *AliyunSMSProvider) Send(phone, code string) error {
	if p.AccessKeyID == "" || p.AccessKeySecret == "" {
		return fmt.Errorf("阿里云短信配置不完整：缺少 AccessKeyID 或 AccessKeySecret")
	}

	if p.SignName == "" || p.TemplateCode == "" {
		return fmt.Errorf("阿里云短信配置不完整：缺少签名名称或模板代码")
	}

	// 构建请求参数
	params := map[string]string{
		"AccessKeyId":      p.AccessKeyID,
		"Action":           "SendSms",
		"Format":           "JSON",
		"PhoneNumbers":     phone,
		"RegionId":         p.RegionID,
		"SignName":         p.SignName,
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureNonce":   generateNonce(),
		"SignatureVersion": "1.0",
		"TemplateCode":     p.TemplateCode,
		"TemplateParam":    fmt.Sprintf(`{"code":"%s"}`, code),
		"Timestamp":        time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"Version":          "2017-05-25",
	}

	// 计算签名
	signature := p.calculateSignature(params)
	params["Signature"] = signature

	// 发送 HTTP 请求
	endpoint := fmt.Sprintf("https://dysmsapi.aliyuncs.com/?%s", p.encodeParams(params))

	resp, err := http.Get(endpoint)
	if err != nil {
		return fmt.Errorf("阿里云短信请求失败：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败：%w", err)
	}

	var result AliyunSMSResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析响应失败：%w", err)
	}

	if result.Code != "OK" {
		return fmt.Errorf("阿里云短信发送失败：%s (Code: %s)", result.Message, result.Code)
	}

	return nil
}

// calculateSignature 计算阿里云 API 签名
func (p *AliyunSMSProvider) calculateSignature(params map[string]string) string {
	// 构造规范化的请求字符串
	var keys []string
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var pairs []string
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%s", percentEncode(k), percentEncode(params[k])))
	}
	canonicalizedQueryString := strings.Join(pairs, "&")

	// 构造待签名字符串
	stringToSign := "GET&%2F&" + percentEncode(canonicalizedQueryString)

	// HMAC-SHA1 签名
	h := hmac.New(sha1.New, []byte(p.AccessKeySecret+"&"))
	h.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	return signature
}

// encodeParams 编码请求参数
func (p *AliyunSMSProvider) encodeParams(params map[string]string) string {
	var keys []string
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var pairs []string
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%s", percentEncode(k), percentEncode(params[k])))
	}

	return strings.Join(pairs, "&")
}

// percentEncode URL 编码（阿里云特殊规则）
func percentEncode(s string) string {
	s = url.QueryEscape(s)
	s = strings.ReplaceAll(s, "+", "%20")
	s = strings.ReplaceAll(s, "*", "%2A")
	s = strings.ReplaceAll(s, "%7E", "~")
	return s
}

// generateNonce 生成随机字符串
func generateNonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand 失败是致命错误
	}
	return hex.EncodeToString(b)
}

// TencentSMSProvider 腾讯云短信提供商
type TencentSMSProvider struct {
	SecretID   string
	SecretKey  string
	AppID      string
	SignName   string
	TemplateID string
	Region     string // 区域，如 ap-guangzhou
	SdkAppID   string // 短信应用 ID
}

// TencentSMSResponse 腾讯云短信响应
type TencentSMSResponse struct {
	Response struct {
		SendStatusSet []struct {
			SerialNo       string `json:"SerialNo"`
			PhoneNumber    string `json:"PhoneNumber"`
			Fee            int    `json:"Fee"`
			SessionContext string `json:"SessionContext"`
			Code           string `json:"Code"`
			Message        string `json:"Message"`
			IsoCode        string `json:"IsoCode"`
		} `json:"SendStatusSet"`
		RequestID string `json:"RequestId"`
	} `json:"Response"`
}

// NewTencentSMSProvider 创建腾讯云短信提供商
func NewTencentSMSProvider(secretID, secretKey, sdkAppID, signName, templateID string) *TencentSMSProvider {
	return &TencentSMSProvider{
		SecretID:   secretID,
		SecretKey:  secretKey,
		SdkAppID:   sdkAppID,
		SignName:   signName,
		TemplateID: templateID,
		Region:     "ap-guangzhou",
	}
}

// SetRegion 设置区域
func (p *TencentSMSProvider) SetRegion(region string) {
	p.Region = region
}

func (p *TencentSMSProvider) Send(phone, code string) error {
	if p.SecretID == "" || p.SecretKey == "" {
		return fmt.Errorf("腾讯云短信配置不完整：缺少 SecretID 或 SecretKey")
	}

	if p.SdkAppID == "" || p.SignName == "" || p.TemplateID == "" {
		return fmt.Errorf("腾讯云短信配置不完整：缺少 AppID、签名名称或模板ID")
	}

	// 格式化手机号（添加 +86 前缀）
	phoneNumber := phone
	if !strings.HasPrefix(phone, "+") {
		phoneNumber = "+86" + phone
	}

	// 构建请求体
	requestBody := map[string]interface{}{
		"PhoneNumberSet":   []string{phoneNumber},
		"SmsSdkAppId":      p.SdkAppID,
		"SignName":         p.SignName,
		"TemplateId":       p.TemplateID,
		"TemplateParamSet": []string{code},
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("序列化请求失败：%w", err)
	}

	// 构建请求
	host := "sms.tencentcloudapi.com"
	uri := "/"
	method := "POST"
	contentType := "application/json"
	timestamp := time.Now().Unix()

	// 计算签名
	signature := p.calculateTencentSignature(host, method, uri, string(bodyBytes), timestamp)

	// 创建 HTTP 请求
	urlStr := fmt.Sprintf("https://%s%s", host, uri)
	req, err := http.NewRequest("POST", urlStr, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return fmt.Errorf("创建请求失败：%w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Host", host)
	req.Header.Set("X-TC-Action", "SendSms")
	req.Header.Set("X-TC-Version", "2021-01-11")
	req.Header.Set("X-TC-Timestamp", fmt.Sprintf("%d", timestamp))
	req.Header.Set("X-TC-Region", p.Region)
	req.Header.Set("Authorization", signature)
	req.Header.Set("X-TC-SecretId", p.SecretID)

	// 发送请求
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("腾讯云短信请求失败：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败：%w", err)
	}

	var result TencentSMSResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析响应失败：%w", err)
	}

	// 检查发送结果
	if len(result.Response.SendStatusSet) == 0 {
		return fmt.Errorf("腾讯云短信发送失败：无返回结果")
	}

	status := result.Response.SendStatusSet[0]
	if status.Code != "Ok" {
		return fmt.Errorf("腾讯云短信发送失败：%s (Code: %s)", status.Message, status.Code)
	}

	return nil
}

// calculateTencentSignature 计算腾讯云 API 签名
func (p *TencentSMSProvider) calculateTencentSignature(host, method, uri, payload string, timestamp int64) string {
	// 步骤1：拼接规范请求串
	httpRequestMethod := method
	canonicalUri := uri
	canonicalQueryString := "" // POST 请求为空
	canonicalHeaders := fmt.Sprintf("content-type:%s\nhost:%s\n", "application/json", host)
	signedHeaders := "content-type;host"

	// 计算请求体哈希
	h := sha256.New()
	h.Write([]byte(payload))
	hashedRequestPayload := fmt.Sprintf("%x", h.Sum(nil))

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		httpRequestMethod,
		canonicalUri,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		hashedRequestPayload,
	)

	// 步骤2：拼接待签名字符串
	algorithm := "TC3-HMAC-SHA256"
	service := "sms"
	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")

	// 计算规范请求的哈希
	h2 := sha256.New()
	h2.Write([]byte(canonicalRequest))
	hashedCanonicalRequest := fmt.Sprintf("%x", h2.Sum(nil))

	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, service)
	stringToSign := fmt.Sprintf("%s\n%d\n%s\n%s",
		algorithm,
		timestamp,
		credentialScope,
		hashedCanonicalRequest,
	)

	// 步骤3：计算签名
	secretDate := hmacSha256([]byte("TC3"+p.SecretKey), date)
	secretService := hmacSha256(secretDate, service)
	secretSigning := hmacSha256(secretService, "tc3_request")
	signature := fmt.Sprintf("%x", hmacSha256(secretSigning, stringToSign))

	// 步骤4：拼接 Authorization
	authorization := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm,
		p.SecretID,
		credentialScope,
		signedHeaders,
		signature,
	)

	return authorization
}

// hmacSha256 HMAC-SHA256 辅助函数
func hmacSha256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}
