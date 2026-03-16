package ldap

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/go-ldap/ldap/v3"
)

// Client LDAP 客户端
type Client struct {
	config Config
	conn   *ldap.Conn
	mu     sync.Mutex
}

// Pool 连接池
type Pool struct {
	config    Config
	clients   chan *Client
	mu        sync.Mutex
	createdAt time.Time
}

// NewClient 创建新的 LDAP 客户端
func NewClient(config Config) (*Client, error) {
	client := &Client{
		config: config,
	}
	return client, nil
}

// Connect 连接到 LDAP 服务器
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return nil
	}

	var conn *ldap.Conn
	var err error

	timeout := c.config.ConnectTimeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	// 解析 URL
	if c.config.UseTLS || len(c.config.URL) >= 5 && c.config.URL[:5] == "ldaps" {
		conn, err = c.dialTLS()
	} else {
		conn, err = c.dialPlain()
	}

	if err != nil {
		return fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}

	// 设置超时
	conn.SetTimeout(timeout)

	// StartTLS for plain LDAP
	if !c.config.UseTLS && len(c.config.URL) >= 5 && c.config.URL[:5] == "ldap:" {
		// 仅测试环境允许跳过 TLS 验证，生产环境必须验证证书
		skipVerify := c.config.SkipTLSVerify && os.Getenv("ENV") == "test"
		tlsConfig := &tls.Config{
			InsecureSkipVerify: skipVerify,
		}
		if c.config.CACertPath != "" {
			pool, err := c.loadCACert()
			if err != nil {
				conn.Close()
				return err
			}
			tlsConfig.RootCAs = pool
		}
		if err := conn.StartTLS(tlsConfig); err != nil {
			conn.Close()
			return fmt.Errorf("StartTLS 失败: %w", err)
		}
	}

	c.conn = conn
	return nil
}

// dialPlain 普通连接
func (c *Client) dialPlain() (*ldap.Conn, error) {
	return ldap.DialURL(c.config.URL)
}

// dialTLS TLS 连接
func (c *Client) dialTLS() (*ldap.Conn, error) {
	// 仅测试环境允许跳过 TLS 验证，生产环境必须验证证书
	skipVerify := c.config.SkipTLSVerify && os.Getenv("ENV") == "test"
	tlsConfig := &tls.Config{
		InsecureSkipVerify: skipVerify,
	}

	// 加载 CA 证书
	if c.config.CACertPath != "" {
		pool, err := c.loadCACert()
		if err != nil {
			return nil, err
		}
		tlsConfig.RootCAs = pool
	}

	// 加载客户端证书
	if c.config.ClientCertPath != "" && c.config.ClientKeyPath != "" {
		cert, err := tls.LoadX509KeyPair(c.config.ClientCertPath, c.config.ClientKeyPath)
		if err != nil {
			return nil, fmt.Errorf("%w: 加载客户端证书失败: %v", ErrTLSCertInvalid, err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	// 提取主机名
	host := c.config.URL
	if len(host) > 8 && host[:8] == "ldaps://" {
		host = host[8:]
	} else if len(host) > 7 && host[:7] == "ldap://" {
		host = host[7:]
	}
	tlsConfig.ServerName = host

	// 使用 DialURL 替代已弃用的 DialTLS
	return ldap.DialURL(c.config.URL, ldap.DialWithTLSConfig(tlsConfig))
}

// loadCACert 加载 CA 证书
func (c *Client) loadCACert() (*x509.CertPool, error) {
	caCert, err := os.ReadFile(c.config.CACertPath)
	if err != nil {
		return nil, fmt.Errorf("%w: 读取 CA 证书失败: %v", ErrTLSCertInvalid, err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("%w: 无效的 CA 证书格式", ErrTLSCertInvalid)
	}

	return pool, nil
}

// Bind 绑定到 LDAP 服务器
func (c *Client) Bind() error {
	if err := c.Connect(); err != nil {
		return err
	}

	if c.config.BindDN == "" {
		return nil // 匿名绑定
	}

	if err := c.conn.Bind(c.config.BindDN, c.config.BindPassword); err != nil {
		return fmt.Errorf("%w: %v", ErrBindFailed, err)
	}

	return nil
}

// BindWithCredential 使用指定凭据绑定
func (c *Client) BindWithCredential(dn, password string) error {
	if err := c.Connect(); err != nil {
		return err
	}

	if err := c.conn.Bind(dn, password); err != nil {
		return ErrAuthFailed
	}

	return nil
}

// Close 关闭连接
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

// IsConnected 检查是否已连接
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn != nil
}

// RawConn 获取原始连接（用于高级操作）
func (c *Client) RawConn() *ldap.Conn {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn
}

// Search 执行搜索
func (c *Client) Search(request *ldap.SearchRequest) (*ldap.SearchResult, error) {
	if err := c.Bind(); err != nil {
		return nil, err
	}

	result, err := c.conn.Search(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSearchFailed, err)
	}

	return result, nil
}

// Add 添加条目
func (c *Client) Add(request *ldap.AddRequest) error {
	if err := c.Bind(); err != nil {
		return err
	}

	if err := c.conn.Add(request); err != nil {
		return fmt.Errorf("%w: 添加条目失败: %v", ErrOperationFailed, err)
	}

	return nil
}

// Modify 修改条目
func (c *Client) Modify(request *ldap.ModifyRequest) error {
	if err := c.Bind(); err != nil {
		return err
	}

	if err := c.conn.Modify(request); err != nil {
		return fmt.Errorf("%w: 修改条目失败: %v", ErrOperationFailed, err)
	}

	return nil
}

// Delete 删除条目
func (c *Client) Delete(dn string) error {
	if err := c.Bind(); err != nil {
		return err
	}

	request := ldap.NewDelRequest(dn, nil)
	if err := c.conn.Del(request); err != nil {
		return fmt.Errorf("%w: 删除条目失败: %v", ErrOperationFailed, err)
	}

	return nil
}

// ModifyDN 修改条目 DN
func (c *Client) ModifyDN(dn, newRDN string, deleteOld bool, newSuperior string) error {
	if err := c.Bind(); err != nil {
		return err
	}

	request := ldap.NewModifyDNRequest(dn, newRDN, deleteOld, newSuperior)
	if err := c.conn.ModifyDN(request); err != nil {
		return fmt.Errorf("%w: 修改 DN 失败: %v", ErrOperationFailed, err)
	}

	return nil
}

// Compare 比较属性值
func (c *Client) Compare(dn, attribute, value string) (bool, error) {
	if err := c.Bind(); err != nil {
		return false, err
	}

	result, err := c.conn.Compare(dn, attribute, value)
	if err != nil {
		return false, fmt.Errorf("比较失败: %w", err)
	}

	return result, nil
}

// PasswordModify 修改密码
func (c *Client) PasswordModify(userDN, oldPassword, newPassword string) error {
	if err := c.Bind(); err != nil {
		return err
	}

	request := ldap.NewModifyRequest(userDN, nil)

	// 根据服务器类型使用不同的密码属性
	passwordAttr := "userPassword"
	if c.config.ServerType == ServerTypeAD {
		passwordAttr = "unicodePwd"
		// AD 需要特殊编码
		encodedPwd, err := encodeADPassword(newPassword)
		if err != nil {
			return err
		}
		request.Replace(passwordAttr, []string{string(encodedPwd)})
	} else {
		request.Replace(passwordAttr, []string{newPassword})
	}

	return c.Modify(request)
}

// encodeADPassword 编码 AD 密码
func encodeADPassword(password string) ([]byte, error) {
	// AD 密码需要 UTF-16LE 编码并加上双引号
	quoted := "\"" + password + "\""
	result := make([]byte, len(quoted)*2)
	for i, r := range quoted {
		result[i*2] = byte(r)
		result[i*2+1] = 0
	}
	return result, nil
}

// ========== 连接池 ==========

// NewPool 创建连接池
func NewPool(config Config) (*Pool, error) {
	size := config.PoolSize
	if size <= 0 {
		size = 10
	}

	pool := &Pool{
		config:  config,
		clients: make(chan *Client, size),
	}

	// 预创建连接
	for i := 0; i < size/2; i++ {
		client, err := NewClient(config)
		if err != nil {
			pool.Close()
			return nil, err
		}
		pool.clients <- client
	}

	pool.createdAt = time.Now()
	return pool, nil
}

// Get 获取连接
func (p *Pool) Get() (*Client, error) {
	select {
	case client := <-p.clients:
		// 检查连接是否有效
		if client.IsConnected() {
			return client, nil
		}
		// 连接无效，创建新的
		return NewClient(p.config)
	default:
		// 池中没有可用连接，创建新的
		return NewClient(p.config)
	}
}

// Put 放回连接
func (p *Pool) Put(client *Client) {
	select {
	case p.clients <- client:
		// 成功放回
	default:
		// 池已满，关闭连接
		client.Close()
	}
}

// Close 关闭连接池
func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	close(p.clients)
	for client := range p.clients {
		client.Close()
	}
}

// Stats 获取连接池统计
func (p *Pool) Stats() map[string]interface{} {
	return map[string]interface{}{
		"pool_size":   cap(p.clients),
		"available":   len(p.clients),
		"created_at":  p.createdAt,
		"config_name": p.config.Name,
	}
}
