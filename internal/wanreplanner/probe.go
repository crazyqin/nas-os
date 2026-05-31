package wanreplanner

import (
	"net"
	"net/http"
	"time"
)

// ExecuteProbe 执行链路探测
func ExecuteProbe(link *WANLink, target ProbeTarget, timeout time.Duration) ProbeResult {
	start := time.Now()
	result := ProbeResult{
		LinkID:    link.ID,
		Target:    target,
		Timestamp: start,
	}

	switch target.Type {
	case ProbePing:
		result = probePing(link, target, timeout)
	case ProbeTCP:
		result = probeTCP(link, target, timeout)
	case ProbeHTTP:
		result = probeHTTP(link, target, timeout)
	default:
		result.Error = "unknown probe type"
	}

	return result
}

// probePing 模拟 ping 探测
func probePing(link *WANLink, target ProbeTarget, timeout time.Duration) ProbeResult {
	start := time.Now()
	result := ProbeResult{
		LinkID:    link.ID,
		Target:    target,
		Timestamp: start,
	}

	// 使用 TCP 连接模拟 ping（避免需要 root 权限）
	addr := target.Host + ":80"
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	conn.Close()
	result.Success = true
	result.Latency = time.Since(start)
	return result
}

// probeTCP TCP 探测
func probeTCP(link *WANLink, target ProbeTarget, timeout time.Duration) ProbeResult {
	start := time.Now()
	result := ProbeResult{
		LinkID:    link.ID,
		Target:    target,
		Timestamp: start,
	}

	port := target.Port
	if port == 0 {
		port = 80
	}
	addr := net.JoinHostPort(target.Host, intToStr(port))
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	conn.Close()
	result.Success = true
	result.Latency = time.Since(start)
	return result
}

// probeHTTP HTTP 探测
func probeHTTP(link *WANLink, target ProbeTarget, timeout time.Duration) ProbeResult {
	start := time.Now()
	result := ProbeResult{
		LinkID:    link.ID,
		Target:    target,
		Timestamp: start,
	}

	scheme := "http"
	if target.Port == 443 {
		scheme = "https"
	}
	port := target.Port
	if port == 0 {
		port = 80
	}
	path := target.Path
	if path == "" {
		path = "/"
	}
	url := scheme + "://" + target.Host + ":" + intToStr(port) + path

	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()

	expected := target.Expected
	if expected == 0 {
		expected = 200
	}
	if resp.StatusCode == expected {
		result.Success = true
	} else {
		result.Error = "unexpected status: " + resp.Status
	}
	result.Latency = time.Since(start)
	return result
}

// intToStr int 转 string
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	negative := false
	if n < 0 {
		negative = true
		n = -n
	}
	for n > 0 {
		digit := n % 10
		result = string(rune('0'+digit)) + result
		n /= 10
	}
	if negative {
		result = "-" + result
	}
	return result
}
