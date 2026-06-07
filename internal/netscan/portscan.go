package netscan

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

// PortScanner 端口扫描器.
type PortScanner struct {
	config PortScanConfig
}

// NewPortScanner 创建端口扫描器.
func NewPortScanner(config PortScanConfig) *PortScanner {
	if config.Timeout == 0 {
		config.Timeout = 2 * time.Second
	}
	if config.Concurrent == 0 {
		config.Concurrent = 100
	}
	if config.Protocol == "" {
		config.Protocol = ProtocolTCP
	}
	return &PortScanner{config: config}
}

// Scan 扫描端口.
func (ps *PortScanner) Scan(ctx context.Context) ([]Port, error) {
	ports := ps.getTargetPorts()
	if len(ports) == 0 {
		return nil, fmt.Errorf("未指定扫描端口")
	}

	var results []Port
	var mu sync.Mutex
	var wg sync.WaitGroup

	sem := make(chan struct{}, ps.config.Concurrent)

	for _, port := range ports {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(port int) {
			defer wg.Done()
			defer func() { <-sem }()

			var p Port
			switch ps.config.Protocol {
			case ProtocolTCP:
				p = ps.scanTCP(ctx, ps.config.Target, port)
			case ProtocolUDP:
				p = ps.scanUDP(ctx, ps.config.Target, port)
			default:
				p = ps.scanTCP(ctx, ps.config.Target, port)
			}

			if p.State == PortStateOpen {
				mu.Lock()
				results = append(results, p)
				mu.Unlock()
			}
		}(port)
	}

	wg.Wait()
	return results, nil
}

// scanTCP TCP 端口扫描.
func (ps *PortScanner) scanTCP(ctx context.Context, host string, port int) Port {
	addr := fmt.Sprintf("%s:%d", host, port)

	ctx, cancel := context.WithTimeout(ctx, ps.config.Timeout)
	defer cancel()

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return Port{
			Number:   port,
			Protocol: ProtocolTCP,
			State:    PortStateClosed,
		}
	}
	defer conn.Close()

	p := Port{
		Number:   port,
		Protocol: ProtocolTCP,
		State:    PortStateOpen,
		Service:  GuessService(port, "tcp"),
	}

	// 尝试获取 Banner
	banner, err := ps.grabBanner(conn)
	if err == nil && banner != "" {
		p.Banner = banner
	}

	return p
}

// scanUDP UDP 端口扫描.
func (ps *PortScanner) scanUDP(ctx context.Context, host string, port int) Port {
	addr := fmt.Sprintf("%s:%d", host, port)

	ctx, cancel := context.WithTimeout(ctx, ps.config.Timeout)
	defer cancel()

	var d net.Dialer
	conn, err := d.DialContext(ctx, "udp", addr)
	if err != nil {
		return Port{
			Number:   port,
			Protocol: ProtocolUDP,
			State:    PortStateClosed,
		}
	}
	defer conn.Close()

	// UDP 扫描：发送数据包并等待响应
	_, err = conn.Write([]byte{0x00})
	if err != nil {
		return Port{
			Number:   port,
			Protocol: ProtocolUDP,
			State:    PortStateFiltered,
		}
	}

	// 等待响应
	buf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(ps.config.Timeout))
	_, err = conn.Read(buf)
	if err != nil {
		// UDP 无响应可能是 open|filtered
		return Port{
			Number:   port,
			Protocol: ProtocolUDP,
			State:    PortStateOpen,
			Service:  GuessService(port, "udp"),
		}
	}

	return Port{
		Number:   port,
		Protocol: ProtocolUDP,
		State:    PortStateOpen,
		Service:  GuessService(port, "udp"),
	}
}

// grabBanner 抓取 Banner.
func (ps *PortScanner) grabBanner(conn net.Conn) (string, error) {
	buf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	n, err := conn.Read(buf)
	if err != nil {
		return "", err
	}
	return string(buf[:n]), nil
}

// getTargetPorts 获取目标端口列表.
func (ps *PortScanner) getTargetPorts() []int {
	if len(ps.config.Ports) > 0 {
		return ps.config.Ports
	}

	if ps.config.TopPorts > 0 && ps.config.TopPorts <= len(TopPorts100) {
		return TopPorts100[:ps.config.TopPorts]
	}

	return CommonPorts
}

// GuessService 猜测端口对应的服务.
func GuessService(port int, protocol string) string {
	services := map[int]string{
		20:    "ftp-data",
		21:    "ftp",
		22:    "ssh",
		23:    "telnet",
		25:    "smtp",
		53:    "dns",
		80:    "http",
		110:   "pop3",
		143:   "imap",
		443:   "https",
		445:   "smb",
		993:   "imaps",
		995:   "pop3s",
		1723:  "pptp",
		3306:  "mysql",
		3389:  "rdp",
		5432:  "postgresql",
		5900:  "vnc",
		8080:  "http-alt",
		8443:  "https-alt",
		9090:  "web-console",
		27017: "mongodb",
		6379:  "redis",
		11211: "memcached",
		5000:  "upnp",
		8000:  "http-alt",
	}

	if svc, ok := services[port]; ok {
		return svc
	}

	if protocol == "udp" {
		udpServices := map[int]string{
			53:   "dns",
			67:   "dhcp",
			68:   "dhcp",
			69:   "tftp",
			123:  "ntp",
			161:  "snmp",
			162:  "snmptrap",
			500:  "ike",
			514:  "syslog",
			1900: "ssdp",
		}
		if svc, ok := udpServices[port]; ok {
			return svc
		}
	}

	return ""
}

// TCPConnectScan TCP Connect 扫描.
func TCPConnectScan(ctx context.Context, host string, ports []int) ([]Port, error) {
	config := PortScanConfig{
		Target:     host,
		Ports:      ports,
		Protocol:   ProtocolTCP,
		Timeout:    2 * time.Second,
		Concurrent: 100,
	}
	scanner := NewPortScanner(config)
	return scanner.Scan(ctx)
}

// UDPScan UDP 扫描.
func UDPScan(ctx context.Context, host string, ports []int) ([]Port, error) {
	config := PortScanConfig{
		Target:     host,
		Ports:      ports,
		Protocol:   ProtocolUDP,
		Timeout:    2 * time.Second,
		Concurrent: 50,
	}
	scanner := NewPortScanner(config)
	return scanner.Scan(ctx)
}
