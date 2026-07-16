package netscan

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ServiceDetector 服务识别器.
type ServiceDetector struct {
	config ServiceDetectConfig
}

// NewServiceDetector 创建服务识别器.
func NewServiceDetector(config ServiceDetectConfig) *ServiceDetector {
	if config.Timeout == 0 {
		config.Timeout = 3 * time.Second
	}
	return &ServiceDetector{config: config}
}

// Detect 识别服务.
func (sd *ServiceDetector) Detect(ctx context.Context) ([]Service, error) {
	if len(sd.config.Ports) == 0 {
		return nil, fmt.Errorf("未指定端口")
	}

	var services []Service
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, port := range sd.config.Ports {
		select {
		case <-ctx.Done():
			return services, ctx.Err()
		default:
		}

		wg.Add(1)
		go func(port int) {
			defer wg.Done()

			svc := sd.detectService(ctx, sd.config.Target, port)
			if svc != nil {
				mu.Lock()
				services = append(services, *svc)
				mu.Unlock()
			}
		}(port)
	}

	wg.Wait()
	return services, nil
}

// detectService 检测单个端口的服务.
func (sd *ServiceDetector) detectService(ctx context.Context, host string, port int) *Service {
	addr := fmt.Sprintf("%s:%d", host, port)

	ctx, cancel := context.WithTimeout(ctx, sd.config.Timeout)
	defer cancel()

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil
	}
	defer conn.Close()

	svc := &Service{
		Port:  port,
		Proto: "tcp",
		Name:  GuessService(port, "tcp"),
	}

	// 特定服务深度探测
	probe := sd.getProbe(port)
	if probe != "" {
		conn.Write([]byte(probe))
	}

	if sd.config.BannerGrab {
		banner := sd.grabBanner(conn)
		if banner != "" {
			svc.Banner = banner
			svc.Version = sd.parseVersion(port, banner)
		}
	}

	return svc
}

// getProbe 获取服务探测字符串.
func (sd *ServiceDetector) getProbe(port int) string {
	probes := map[int]string{
		80:   "GET / HTTP/1.0\r\n\r\n",
		443:  "", // HTTPS 需要 TLS
		22:   "", // SSH 会主动发送 banner
		21:   "", // FTP 会主动发送 banner
		25:   "EHLO probe\r\n",
		110:  "", // POP3 会主动发送 banner
		143:  "", // IMAP 会主动发送 banner
		3306: "", // MySQL 会主动发送 banner
	}

	if probe, ok := probes[port]; ok {
		return probe
	}

	return ""
}

// grabBanner 抓取 Banner.
func (sd *ServiceDetector) grabBanner(conn net.Conn) string {
	buf := make([]byte, 4096)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buf)
	if err != nil || n == 0 {
		return ""
	}
	return strings.TrimSpace(string(buf[:n]))
}

// parseVersion 解析服务版本.
func (sd *ServiceDetector) parseVersion(port int, banner string) string {
	banner = strings.TrimSpace(banner)

	switch port {
	case 22:
		return parseSSHVersion(banner)
	case 80, 443, 8080, 8443:
		return parseHTTPServer(banner)
	case 21:
		return parseFTPVersion(banner)
	case 25:
		return parseSMTPVersion(banner)
	case 3306:
		return parseMySQLVersion(banner)
	case 5432:
		return parsePostgresVersion(banner)
	}

	return ""
}

// parseSSHVersion 解析 SSH 版本.
func parseSSHVersion(banner string) string {
	re := regexp.MustCompile(`SSH-[\d.]+-(\S+)`)
	matches := re.FindStringSubmatch(banner)
	if len(matches) > 1 {
		return matches[1]
	}
	return banner
}

// parseHTTPServer 解析 HTTP Server.
func parseHTTPServer(banner string) string {
	lines := strings.Split(banner, "\n")
	for _, line := range lines {
		if strings.HasPrefix(strings.ToLower(line), "server:") {
			return strings.TrimSpace(line[7:])
		}
	}
	return ""
}

// parseFTPVersion 解析 FTP 版本.
func parseFTPVersion(banner string) string {
	re := regexp.MustCompile(`^220[\s-]+(.+)`)
	matches := re.FindStringSubmatch(banner)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return banner
}

// parseSMTPVersion 解析 SMTP 版本.
func parseSMTPVersion(banner string) string {
	re := regexp.MustCompile(`^220[\s]+(\S+)`)
	matches := re.FindStringSubmatch(banner)
	if len(matches) > 1 {
		return matches[1]
	}
	return banner
}

// parseMySQLVersion 解析 MySQL 版本.
func parseMySQLVersion(banner string) string {
	re := regexp.MustCompile(`([\d.]+[-]\w+)`)
	matches := re.FindStringSubmatch(banner)
	if len(matches) > 1 {
		return matches[1]
	}
	return banner
}

// parsePostgresVersion 解析 PostgreSQL 版本.
func parsePostgresVersion(banner string) string {
	re := regexp.MustCompile(`PostgreSQL\s+([\d.]+)`)
	matches := re.FindStringSubmatch(banner)
	if len(matches) > 1 {
		return "PostgreSQL " + matches[1]
	}
	return banner
}

// DetectHTTPService 检测 HTTP 服务.
func DetectHTTPService(ctx context.Context, host string, port int) (*Service, error) {
	config := ServiceDetectConfig{
		Target:     host,
		Ports:      []int{port},
		Timeout:    3 * time.Second,
		BannerGrab: true,
	}
	detector := NewServiceDetector(config)
	services, err := detector.Detect(ctx)
	if err != nil {
		return nil, err
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("未检测到 HTTP 服务")
	}
	return &services[0], nil
}

// DetectSSHService 检测 SSH 服务.
func DetectSSHService(ctx context.Context, host string, port int) (*Service, error) {
	config := ServiceDetectConfig{
		Target:     host,
		Ports:      []int{port},
		Timeout:    3 * time.Second,
		BannerGrab: true,
	}
	detector := NewServiceDetector(config)
	services, err := detector.Detect(ctx)
	if err != nil {
		return nil, err
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("未检测到 SSH 服务")
	}
	return &services[0], nil
}

// DetectSMBService 检测 SMB 服务.
func DetectSMBService(ctx context.Context, host string) (*Service, error) {
	config := ServiceDetectConfig{
		Target:     host,
		Ports:      []int{445},
		Timeout:    3 * time.Second,
		BannerGrab: true,
	}
	detector := NewServiceDetector(config)
	services, err := detector.Detect(ctx)
	if err != nil {
		return nil, err
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("未检测到 SMB 服务")
	}
	return &services[0], nil
}
