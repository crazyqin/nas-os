// Package e2e 提供 NAS-OS 端到端测试
// HTTP 客户端和辅助工具
package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// TestClient HTTP 测试客户端
type TestClient struct {
	BaseURL    string
	AuthToken  string
	Timeout    time.Duration
	HTTPClient *http.Client
}

// NewTestClient 创建测试客户端
func NewTestClient(baseURL string) *TestClient {
	return &TestClient{
		BaseURL: baseURL,
		Timeout: 30 * time.Second,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Request 发送 HTTP 请求
func (c *TestClient) Request(method, path string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	}

	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: c.Timeout}
	}

	return c.HTTPClient.Do(req)
}

// Get 发送 GET 请求
func (c *TestClient) Get(path string) (*http.Response, error) {
	return c.Request(http.MethodGet, path, nil)
}

// Post 发送 POST 请求
func (c *TestClient) Post(path string, body interface{}) (*http.Response, error) {
	return c.Request(http.MethodPost, path, body)
}

// Put 发送 PUT 请求
func (c *TestClient) Put(path string, body interface{}) (*http.Response, error) {
	return c.Request(http.MethodPut, path, body)
}

// Delete 发送 DELETE 请求
func (c *TestClient) Delete(path string) (*http.Response, error) {
	return c.Request(http.MethodDelete, path, nil)
}

// Patch 发送 PATCH 请求
func (c *TestClient) Patch(path string, body interface{}) (*http.Response, error) {
	return c.Request(http.MethodPatch, path, body)
}

// SetAuth 设置认证令牌
func (c *TestClient) SetAuth(token string) {
	c.AuthToken = token
}

// ClearAuth 清除认证令牌
func (c *TestClient) ClearAuth() {
	c.AuthToken = ""
}

// ParseJSON 解析 JSON 响应
func ParseJSON(resp *http.Response, out interface{}) error {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

// ReadBody 读取响应体
func ReadBody(resp *http.Response) (string, error) {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// AssertStatus 断言 HTTP 状态码
func AssertStatus(t interface{ Fatalf(string, ...interface{}) }, expected, actual int, msg ...interface{}) {
	if expected != actual {
		t.Fatalf("状态码不匹配: 期望 %d, 实际 %d. %v", expected, actual, msg)
	}
}

// AssertEqual 断言相等
func AssertEqual(t interface{ Fatalf(string, ...interface{}) }, expected, actual interface{}, msg ...interface{}) {
	if expected != actual {
		t.Fatalf("断言失败: 期望 %v, 实际 %v. %v", expected, actual, msg)
	}
}

// AssertNotEmpty 断言非空字符串
func AssertNotEmpty(t interface{ Fatalf(string, ...interface{}) }, value string, msg ...interface{}) {
	if value == "" {
		t.Fatalf("断言失败: 值不应为空. %v", msg)
	}
}

// AssertNotNil 断言非 nil
func AssertNotNil(t interface{ Fatalf(string, ...interface{}) }, value interface{}, msg ...interface{}) {
	if value == nil {
		t.Fatalf("断言失败: 值不应为 nil. %v", msg)
	}
}

// AssertNil 断言为 nil
func AssertNil(t interface{ Fatalf(string, ...interface{}) }, value interface{}, msg ...interface{}) {
	if value != nil {
		t.Fatalf("断言失败: 值应为 nil, 实际 %v. %v", value, msg)
	}
}

// AssertContains 断言字符串包含
func AssertContains(t interface{ Fatalf(string, ...interface{}) }, str, substr string, msg ...interface{}) {
	if !bytes.Contains([]byte(str), []byte(substr)) {
		t.Fatalf("断言失败: 字符串 '%s' 不包含 '%s'. %v", str, substr, msg)
	}
}

// AssertLen 断言长度
func AssertLen(t interface{ Fatalf(string, ...interface{}) }, value interface{}, expectedLen int, msg ...interface{}) {
	switch v := value.(type) {
	case []interface{}:
		if len(v) != expectedLen {
			t.Fatalf("断言失败: 长度期望 %d, 实际 %d. %v", expectedLen, len(v), msg)
		}
	case string:
		if len(v) != expectedLen {
			t.Fatalf("断言失败: 长度期望 %d, 实际 %d. %v", expectedLen, len(v), msg)
		}
	case map[string]interface{}:
		if len(v) != expectedLen {
			t.Fatalf("断言失败: 长度期望 %d, 实际 %d. %v", expectedLen, len(v), msg)
		}
	default:
		t.Fatalf("不支持的类型: %T", value)
	}
}
