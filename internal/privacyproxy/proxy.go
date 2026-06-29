package privacyproxy

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// =========================================================================
// AI 隐私代理服务器
// 设计理念：拦截出站 AI API 请求 → 本地脱敏 → 转发至目标 API → 响应回传
// =========================================================================

// ProxyServer AI 隐私代理服务器
type ProxyServer struct {
	config   *MaskConfig
	masker   *Masker
	auditor  *Auditor
	server   *http.Server
	mu       sync.RWMutex
	running  bool
	client   *http.Client
}

// NewProxyServer 创建代理服务器
func NewProxyServer(cfg *MaskConfig, masker *Masker, auditor *Auditor) *ProxyServer {
	if cfg == nil {
		cfg = DefaultMaskConfig()
	}
	if masker == nil {
		masker = NewMasker(cfg)
	}
	if auditor == nil {
		auditor = NewAuditor(cfg.AuditMaxEntries)
	}

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	return &ProxyServer{
		config:  cfg,
		masker:  masker,
		auditor: auditor,
		client: &http.Client{
			Transport: transport,
			Timeout:   0, // 不设全局超时，流式响应需要手动控制
		},
	}
}

// Start 启动代理服务器
func (p *ProxyServer) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		return fmt.Errorf("代理服务器已在运行")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", p.handleProxy)
	mux.HandleFunc("/health", p.handleHealth)
	mux.HandleFunc("/rules", p.handleRules)

	p.server = &http.Server{
		Addr:    p.config.ListenAddr,
		Handler: mux,
	}

	p.running = true
	go func() {
		log.Printf("[privacyproxy] 代理服务器启动于 %s", p.config.ListenAddr)
		if err := p.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[privacyproxy] 服务器错误: %v", err)
		}
	}()
	return nil
}

// Stop 停止代理服务器
func (p *ProxyServer) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.running {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	p.running = false
	return p.server.Shutdown(ctx)
}

// IsRunning 返回服务器运行状态
func (p *ProxyServer) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// =========================================================================
// HTTP 处理函数
// =========================================================================

// handleHealth 健康检查
func (p *ProxyServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok","service":"privacyproxy"}`))
}

// handleRules 规则管理接口
func (p *ProxyServer) handleRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rules := p.masker.ListRules()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("["))
		for i, rule := range rules {
			if i > 0 {
				w.Write([]byte(","))
			}
			w.Write([]byte(fmt.Sprintf(`{"id":"%s","name":"%s","enabled":%t}`, rule.ID, rule.Name, rule.Enabled)))
		}
		w.Write([]byte("]"))
	default:
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
	}
}

// handleProxy 核心代理处理
func (p *ProxyServer) handleProxy(w http.ResponseWriter, r *http.Request) {
	// 1. 解析目标 URL
	targetURL, err := p.extractTargetURL(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("目标 URL 解析失败: %v", err), http.StatusBadRequest)
		return
	}

	// 2. 域名白名单/黑名单检查
	if !p.isDomainAllowed(targetURL.Host) {
		http.Error(w, fmt.Sprintf("目标域名 %s 不在允许列表中", targetURL.Host), http.StatusForbidden)
		return
	}

	// 3. 读取请求体
	body, err := io.ReadAll(io.LimitReader(r.Body, p.config.MaxBodyBytes))
	if err != nil {
		http.Error(w, fmt.Sprintf("读取请求体失败: %v", err), http.StatusBadRequest)
		return
	}
	r.Body.Close()

	// 4. 对请求体执行脱敏
	outcome := p.masker.Mask(string(body))
	maskedBody := outcome.MaskedText

	// 5. 记录审计日志（如果命中了敏感数据）
	if p.config.AuditEnabled && outcome.TotalHits > 0 {
		for _, match := range outcome.Matches {
			entry := AuditEntry{
				ID:            generateID(),
				Timestamp:     time.Now(),
				RuleID:        match.RuleID,
				RuleName:      match.RuleName,
				Original:      match.Original,
				Masked:        match.Masked,
				TargetAPI:     targetURL.String(),
				MatchCount:    outcome.TotalHits,
				RequestMethod: r.Method,
				RequestPath:   r.URL.Path,
				Success:       true,
			}
			p.auditor.Log(entry)
		}
	}

	// 6. 创建转发请求
	forwardReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL.String(), strings.NewReader(maskedBody))
	if err != nil {
		http.Error(w, fmt.Sprintf("创建转发请求失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 复制请求头
	copyHeaders(forwardReq.Header, r.Header)
	forwardReq.Header.Set("Host", targetURL.Host)
	forwardReq.ContentLength = int64(len(maskedBody))

	// 7. 发送请求
	resp, err := p.client.Do(forwardReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("转发请求失败: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// 8. 处理响应（流式或非流式）
	// 检查是否为流式响应
	isStream := isStreamingResponse(resp)

	// 复制响应头
	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	if isStream {
		// 流式转发：逐块读取并转发
		p.streamResponse(w, resp)
	} else {
		// 非流式：读取完整响应体再转发
		respBody, err := io.ReadAll(io.LimitReader(resp.Body, p.config.MaxBodyBytes))
		if err != nil {
			log.Printf("[privacyproxy] 读取响应体失败: %v", err)
			return
		}
		w.Write(respBody)
	}
}

// extractTargetURL 从请求中提取目标 URL
// 支持两种模式：
//   1. 标准 HTTP 代理模式：请求行为完整 URL
//   2. 反向代理模式：通过 X-Target-URL 头指定目标
func (p *ProxyServer) extractTargetURL(r *http.Request) (*url.URL, error) {
	// 模式 1：X-Target-URL 头
	if target := r.Header.Get("X-Target-URL"); target != "" {
		u, err := url.Parse(target)
		if err != nil {
			return nil, fmt.Errorf("X-Target-URL 解析失败: %w", err)
		}
		if u.Scheme == "" {
			u.Scheme = "https"
		}
		if u.Host == "" {
			return nil, fmt.Errorf("X-Target-URL 缺少主机名")
		}
		return u, nil
	}

	// 模式 2：标准 HTTP 代理（请求行为完整 URL）
	if r.URL.IsAbs() {
		return r.URL, nil
	}

	// 模式 3：通过路径前缀指定目标，如 /api/openai/...
	// 路径格式: /<provider>/<path...>
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/"), "/", 2)
	if len(parts) >= 1 {
		provider := parts[0]
		if target := p.providerToHost(provider); target != "" {
			path := "/"
			if len(parts) > 1 {
				path += parts[1]
			}
			u := &url.URL{
				Scheme: "https",
				Host:   target,
				Path:   path,
				RawQuery: r.URL.RawQuery,
			}
			return u, nil
		}
	}

	return nil, fmt.Errorf("无法确定目标 URL，请使用 X-Target-URL 头或标准代理模式")
}

// providerToHost 将提供商名称映射到 API 主机
func (p *ProxyServer) providerToHost(provider string) string {
	switch strings.ToLower(provider) {
	case "openai":
		return "api.openai.com"
	case "anthropic":
		return "api.anthropic.com"
	case "google":
		return "generativelanguage.googleapis.com"
	case "dashscope", "aliyun":
		return "dashscope.aliyuncs.com"
	case "deepseek":
		return "api.deepseek.com"
	case "siliconflow":
		return "api.siliconflow.cn"
	default:
		return ""
	}
}

// isDomainAllowed 检查域名是否允许转发
func (p *ProxyServer) isDomainAllowed(host string) bool {
	// 去掉端口号
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	// 检查黑名单
	for _, blocked := range p.config.BlockedDomains {
		if matchDomain(host, blocked) {
			return false
		}
	}

	// 如果白名单为空，允许所有（除黑名单外）
	if len(p.config.AllowedDomains) == 0 {
		return true
	}

	for _, allowed := range p.config.AllowedDomains {
		if matchDomain(host, allowed) {
			return true
		}
	}
	return false
}

// matchDomain 域名匹配（支持通配符前缀 *.example.com）
func matchDomain(host, pattern string) bool {
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // .example.com
		return strings.HasSuffix(host, suffix)
	}
	return host == pattern
}

// isStreamingResponse 判断是否为流式响应
func isStreamingResponse(resp *http.Response) bool {
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/event-stream") {
		return true
	}
	if strings.Contains(contentType, "application/x-ndjson") {
		return true
	}
	return false
}

// streamResponse 流式转发响应体
func (p *ProxyServer) streamResponse(w http.ResponseWriter, resp *http.Response) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		// 不支持 Flush，降级为非流式
		io.Copy(w, resp.Body)
		return
	}

	buf := make([]byte, 4096)
	ctx, cancel := context.WithTimeout(context.Background(), p.config.StreamTimeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[privacyproxy] 流式响应超时")
			return
		default:
		}
		n, err := resp.Body.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
			flusher.Flush()
		}
		if err != nil {
			if err != io.EOF {
				log.Printf("[privacyproxy] 流式读取错误: %v", err)
			}
			return
		}
	}
}

// copyHeaders 复制 HTTP 头
func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		for _, v := range values {
			dst.Add(key, v)
		}
	}
}

// generateID 生成简单唯一 ID（时间戳 + 计数器）
var idCounter uint64
var idMu sync.Mutex

func generateID() string {
	idMu.Lock()
	idCounter++
	id := idCounter
	idMu.Unlock()
	return fmt.Sprintf("audit-%d-%d", time.Now().UnixNano(), id)
}
