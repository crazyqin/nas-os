// Package antivirus - ClamAV 扫描引擎
package antivirus

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ClamAVClient ClamAV 客户端.
type ClamAVClient struct {
	config ClamAVConfig
}

// NewClamAVClient 创建 ClamAV 客户端.
func NewClamAVClient(config ClamAVConfig) *ClamAVClient {
	return &ClamAVClient{config: config}
}

// Ping 检查 clamd 是否可达.
func (c *ClamAVClient) Ping() error {
	conn, err := c.dial()
	if err != nil {
		return ErrClamAVNotReady
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(c.config.Timeout))
	fmt.Fprintf(conn, "PING\n")
	reply, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return ErrClamAVNotReady
	}
	if strings.TrimSpace(reply) != "PONG" {
		return ErrClamAVNotReady
	}
	return nil
}

// Version 获取 ClamAV 版本信息.
func (c *ClamAVClient) Version() (*ClamAVVersion, error) {
	conn, err := c.dial()
	if err != nil {
		return nil, ErrClamAVNotReady
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(c.config.Timeout))
	fmt.Fprintf(conn, "VERSION\n")
	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		return nil, ErrClamAVNotReady
	}
	line := scanner.Text()
	return &ClamAVVersion{
		Version: line,
	}, nil
}

// ScanFile 扫描单个文件.
func (c *ClamAVClient) ScanFile(path string) (*ScanResult, error) {
	conn, err := c.dial()
	if err != nil {
		return nil, ErrClamAVNotReady
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(c.config.Timeout))
	fmt.Fprintf(conn, "SCAN %s\n", path)

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		return nil, fmt.Errorf("clamd 无响应")
	}

	reply := scanner.Text()
	info, _ := os.Stat(path)
	result := &ScanResult{
		FilePath:  path,
		ScannedAt: time.Now(),
	}
	if info != nil {
		result.FileSize = info.Size()
	}

	if strings.HasSuffix(reply, "OK") {
		result.IsInfected = false
	} else if strings.Contains(reply, "FOUND") {
		result.IsInfected = true
		// 提取病毒名: "path: VirusName FOUND"
		parts := strings.Split(reply, ": ")
		if len(parts) >= 2 {
			threat := strings.TrimSuffix(parts[len(parts)-1], " FOUND")
			result.ThreatName = threat
		}
	} else {
		return nil, fmt.Errorf("扫描错误: %s", reply)
	}

	return result, nil
}

// Reload 重新加载病毒库.
func (c *ClamAVClient) Reload() error {
	conn, err := c.dial()
	if err != nil {
		return ErrClamAVNotReady
	}
	defer conn.Close()
	fmt.Fprintf(conn, "RELOAD\n")
	scanner := bufio.NewScanner(conn)
	if scanner.Scan() && strings.TrimSpace(scanner.Text()) == "RELOADING" {
		return nil
	}
	return fmt.Errorf("reload 失败")
}

func (c *ClamAVClient) dial() (net.Conn, error) {
	if c.config.Transport == TransportSocket {
		return net.Dial("unix", c.config.Socket)
	}
	addr := net.JoinHostPort(c.config.Host, fmt.Sprintf("%d", c.config.Port))
	return net.Dial("tcp", addr)
}

// ScanDirectory 扫描目录.
func ScanDirectory(client *ClamAVClient, dir string, recursive bool) ([]ScanResult, error) {
	var results []ScanResult
	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && !recursive && path != dir {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}
		r, err := client.ScanFile(path)
		if err != nil {
			return nil
		}
		results = append(results, *r)
		return nil
	}
	if err := filepath.Walk(dir, walkFn); err != nil {
		return results, err
	}
	return results, nil
}
