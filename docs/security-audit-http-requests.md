# NAS-OS 安全审计简报

**审计部门**: 刑部（法务合规与知识产权）
**审计日期**: 2026-03-25
**审计范围**: `internal/docker/app_custom.go`, `internal/docker/app_discovery.go`
**审计类型**: HTTP 请求安全性、敏感信息泄露风险

---

## 📋 执行摘要

| 严重级别 | 数量 |
|----------|------|
| 🔴 高危 | 2 |
| 🟠 中危 | 3 |
| 🟡 低危 | 2 |

**整体风险评级**: ⚠️ **需要立即修复**

---

## 🔴 高危漏洞

### 1. SSRF 漏洞 (app_custom.go:62-85)

**位置**: `ImportFromURL` 函数

```go
resp, err := http.Get(url)  // 直接使用用户输入的 URL
```

**问题**:
- 未对 URL 进行任何验证
- 攻击者可访问内部服务（`http://localhost:xxxx`, `http://169.254.169.254`）
- 未限制协议（可能被利用访问 `file://` 等）
- 未验证目标域名白名单

**影响**: 攻击者可探测内网服务、访问云元数据服务、读取内部 API

**建议修复**:
```go
func (ctm *CustomTemplateManager) ImportFromURL(urlStr, name, displayName, description, category string) (*CustomTemplate, error) {
    // 1. 解析并验证 URL
    parsedURL, err := url.Parse(urlStr)
    if err != nil {
        return nil, fmt.Errorf("无效的 URL：%w", err)
    }
    
    // 2. 限制协议
    if parsedURL.Scheme != "https" {
        return nil, fmt.Errorf("只允许 HTTPS 协议")
    }
    
    // 3. 阻止私有 IP 和内部地址
    host := parsedURL.Hostname()
    if isPrivateIP(host) {
        return nil, fmt.Errorf("不允许访问内部地址")
    }
    
    // 4. 使用带超时的 client
    client := &http.Client{
        Timeout: 30 * time.Second,
        Transport: &http.Transport{
            DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
                // 二次验证目标 IP
                ...
            },
        },
    }
    ...
}
```

---

### 2. 代码 Bug 导致功能失效 (app_discovery.go:345-367)

**位置**: `FetchComposeFile` 函数

```go
var content []byte
_, err = resp.Body.Read(content)  // content 是 nil slice，永远不会读取数据
```

**问题**:
- `content` 未初始化，`Read` 无法写入数据
- 函数永远返回空字符串

**建议修复**:
```go
content, err := io.ReadAll(resp.Body)
if err != nil {
    continue
}
return string(content), nil
```

---

## 🟠 中危漏洞

### 3. 无超时设置 (app_custom.go:64)

**问题**: 使用 `http.Get` 默认 client，无超时

**对比**: `app_discovery.go` 正确使用了带 30 秒超时的 `httpClient` ✅

**建议**: 启用注释掉的 `getHTTPClient()` 或统一使用共享 client

---

### 4. 无响应大小限制 (app_custom.go:76-79)

```go
buf := new(strings.Builder)
_, err = io.Copy(buf, resp.Body)  // 无大小限制
```

**问题**: 攻击者可发送超大响应耗尽内存

**建议修复**:
```go
// 限制最大 10MB
limitedReader := io.LimitReader(resp.Body, 10*1024*1024)
content, err := io.ReadAll(limitedReader)
if int64(len(content)) >= 10*1024*1024 {
    return nil, fmt.Errorf("响应超过最大限制")
}
```

---

### 5. 资源泄漏风险 (app_discovery.go:351-355)

```go
for _, filename := range filenames {
    resp, err := ad.httpClient.Get(url)
    if err != nil {
        continue
    }
    defer func() { _ = resp.Body.Close() }()  // defer 在循环内
}
```

**问题**: 多次迭代中 defer 不会立即执行，可能累积未关闭的连接

**建议修复**:
```go
for _, filename := range filenames {
    resp, err := ad.httpClient.Get(url)
    if err != nil {
        continue
    }
    
    content, readErr := io.ReadAll(resp.Body)
    _ = resp.Body.Close()  // 立即关闭
    
    if readErr != nil || resp.StatusCode != 200 {
        continue
    }
    
    return string(content), nil
}
```

---

## 🟡 低危问题

### 6. 敏感信息泄露风险

**位置**: 错误信息、日志输出

```go
// app_custom.go:72
return nil, fmt.Errorf("URL 返回状态码：%d", resp.StatusCode)

// app_discovery.go:144
fmt.Printf("GitHub 搜索失败 (%s): %v\n", query, err)
```

**建议**: 生产环境避免详细错误信息，使用结构化日志而非 fmt.Printf

---

### 7. GitHub API 速率限制

**位置**: `app_discovery.go` 多处 GitHub API 调用

**问题**: 未实现客户端速率限制，高频调用可能触发 API 限制

**建议**: 实现 rate limiting，缓存 API 响应

---

## ✅ 安全实践亮点

| 文件 | 安全实践 |
|------|----------|
| app_discovery.go | 正确使用带超时的 `httpClient` (30s) ✅ |
| app_discovery.go | GitHub Token 从环境变量读取 ✅ |
| app_discovery.go | 文件权限 0640 相对安全 ✅ |
| app_discovery.go | mutex 保护并发访问 ✅ |

---

## 📌 修复优先级

1. **立即修复** (高危):
   - [ ] SSRF 漏洞防护
   - [ ] 修复 `FetchComposeFile` Bug

2. **短期修复** (中危):
   - [ ] 添加 HTTP 超时设置
   - [ ] 添加响应大小限制
   - [ ] 修复 defer 循环泄漏

3. **中期改进** (低危):
   - [ ] 优化错误信息
   - [ ] 实现 API 速率限制

---

## 📚 参考资料

- [OWASP SSRF Prevention Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html)
- [Go HTTP Client Security Best Practices](https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/)

---

**审计员**: 刑部 AI 审计官  
**审计状态**: 待修复验证