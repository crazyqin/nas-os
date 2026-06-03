package datapipeline

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Connector 数据源连接器接口
type Connector interface {
	// Connect 连接到数据源
	Connect(ctx context.Context) error
	// Disconnect 断开连接
	Disconnect() error
	// Read 读取数据
	Read(ctx context.Context) ([]map[string]interface{}, error)
	// Write 写入数据
	Write(ctx context.Context, data []map[string]interface{}) error
	// IsConnected 是否已连接
	IsConnected() bool
	// Ping 测试连接
	Ping(ctx context.Context) error
}

// FileSystemConnector 文件系统连接器
type FileSystemConnector struct {
	config    DataSource
	basePath  string
	pattern   string
	recursive bool
	connected bool
	mu        sync.RWMutex
}

// SMBConnector SMB 连接器（模拟实现）
type SMBConnector struct {
	config    DataSource
	host      string
	share     string
	user      string
	password  string
	connected bool
	mu        sync.RWMutex
}

// S3Connector S3 连接器（模拟实现）
type S3Connector struct {
	config      DataSource
	endpoint    string
	bucket      string
	accessKey   string
	secretKey   string
	region      string
	prefix      string
	connected   bool
	mu          sync.RWMutex
}

// DatabaseConnector 数据库连接器（模拟实现）
type DatabaseConnector struct {
	config      DataSource
	driver      string
	dsn         string
	query       string
	connected   bool
	mu          sync.RWMutex
}

// HTTPConnector HTTP API 连接器
type HTTPConnector struct {
	config      DataSource
	url         string
	method      string
	headers     map[string]string
	connected   bool
	mu          sync.RWMutex
}

// OutputConnector 输出连接器接口
type OutputConnector interface {
	// Connect 连接到输出目标
	Connect(ctx context.Context) error
	// Disconnect 断开连接
	Disconnect() error
	// Write 写入数据
	Write(ctx context.Context, data []map[string]interface{}) error
	// IsConnected 是否已连接
	IsConnected() bool
}

// FileOutputConnector 文件输出连接器
type FileOutputConnector struct {
	config    OutputNode
	basePath  string
	format    string
	connected bool
	mu        sync.RWMutex
}

// WebhookOutputConnector Webhook 输出连接器
type WebhookOutputConnector struct {
	config    OutputNode
	url       string
	method    string
	headers   map[string]string
	connected bool
	mu        sync.RWMutex
}

// NotificationOutputConnector 通知输出连接器
type NotificationOutputConnector struct {
	config    OutputNode
	channel   string
	message   string
	connected bool
	mu        sync.RWMutex
}

// NewConnector 创建数据源连接器
func NewConnector(ds DataSource) (Connector, error) {
	switch ds.Type {
	case DataSourceFileSystem:
		return NewFileSystemConnector(ds)
	case DataSourceSMB:
		return NewSMBConnector(ds)
	case DataSourceS3:
		return NewS3Connector(ds)
	case DataSourceDatabase:
		return NewDatabaseConnector(ds)
	case DataSourceHTTP:
		return NewHTTPConnector(ds)
	default:
		return nil, fmt.Errorf("unsupported data source type: %s", ds.Type)
	}
}

// NewFileSystemConnector 创建文件系统连接器
func NewFileSystemConnector(ds DataSource) (*FileSystemConnector, error) {
	basePath, ok := ds.Connection["path"].(string)
	if !ok {
		return nil, fmt.Errorf("filesystem connector requires 'path' in connection config")
	}

	pattern, _ := ds.Connection["pattern"].(string)
	if pattern == "" {
		pattern = "*"
	}

	recursive, _ := ds.Connection["recursive"].(bool)

	return &FileSystemConnector{
		config:    ds,
		basePath:  basePath,
		pattern:   pattern,
		recursive: recursive,
	}, nil
}

// Connect 连接到文件系统
func (c *FileSystemConnector) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 检查路径是否存在
	info, err := os.Stat(c.basePath)
	if err != nil {
		return fmt.Errorf("failed to access path %s: %w", c.basePath, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path %s is not a directory", c.basePath)
	}

	c.connected = true
	return nil
}

// Disconnect 断开连接
func (c *FileSystemConnector) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	return nil
}

// Read 读取文件系统数据
func (c *FileSystemConnector) Read(ctx context.Context) ([]map[string]interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return nil, fmt.Errorf("connector not connected")
	}

	var files []map[string]interface{}

	walkFn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 跳过错误
		}

		// 检查 context
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 匹配模式
		matched, err := filepath.Match(c.pattern, d.Name())
		if err != nil || !matched {
			if !d.IsDir() {
				return nil
			}
		}

		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return nil
			}

			relPath, _ := filepath.Rel(c.basePath, path)
			files = append(files, map[string]interface{}{
				"path":         path,
				"relativePath": relPath,
				"name":         d.Name(),
				"size":         info.Size(),
				"modified":     info.ModTime(),
				"isDir":        false,
			})
		}

		// 非递归模式下只读取顶层目录
		if !c.recursive && d.IsDir() && path != c.basePath {
			return filepath.SkipDir
		}

		return nil
	}

	if err := filepath.WalkDir(c.basePath, walkFn); err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return files, nil
}

// Write 写入文件系统
func (c *FileSystemConnector) Write(ctx context.Context, data []map[string]interface{}) error {
	return fmt.Errorf("filesystem connector does not support write operations")
}

// IsConnected 是否已连接
func (c *FileSystemConnector) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// Ping 测试连接
func (c *FileSystemConnector) Ping(ctx context.Context) error {
	_, err := os.Stat(c.basePath)
	return err
}

// NewSMBConnector 创建 SMB 连接器
func NewSMBConnector(ds DataSource) (*SMBConnector, error) {
	host, _ := ds.Connection["host"].(string)
	share, _ := ds.Connection["share"].(string)
	user, _ := ds.Connection["user"].(string)
	password, _ := ds.Connection["password"].(string)

	if host == "" || share == "" {
		return nil, fmt.Errorf("SMB connector requires 'host' and 'share' in connection config")
	}

	return &SMBConnector{
		config:   ds,
		host:     host,
		share:    share,
		user:     user,
		password: password,
	}, nil
}

// Connect 连接到 SMB（模拟实现）
func (c *SMBConnector) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 模拟连接
	time.Sleep(50 * time.Millisecond)
	c.connected = true
	return nil
}

// Disconnect 断开连接
func (c *SMBConnector) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	return nil
}

// Read 读取 SMB 数据（模拟实现）
func (c *SMBConnector) Read(ctx context.Context) ([]map[string]interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return nil, fmt.Errorf("SMB connector not connected")
	}

	// 模拟返回数据
	return []map[string]interface{}{
		{
			"source": "smb",
			"host":   c.host,
			"share":  c.share,
			"path":   "/data/file1.txt",
			"size":   1024,
		},
	}, nil
}

// Write 写入 SMB
func (c *SMBConnector) Write(ctx context.Context, data []map[string]interface{}) error {
	return fmt.Errorf("SMB connector write not implemented")
}

// IsConnected 是否已连接
func (c *SMBConnector) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// Ping 测试连接
func (c *SMBConnector) Ping(ctx context.Context) error {
	return nil // 模拟实现
}

// NewS3Connector 创建 S3 连接器
func NewS3Connector(ds DataSource) (*S3Connector, error) {
	endpoint, _ := ds.Connection["endpoint"].(string)
	bucket, _ := ds.Connection["bucket"].(string)
	accessKey, _ := ds.Connection["accessKey"].(string)
	secretKey, _ := ds.Connection["secretKey"].(string)
	region, _ := ds.Connection["region"].(string)
	prefix, _ := ds.Connection["prefix"].(string)

	if bucket == "" {
		return nil, fmt.Errorf("S3 connector requires 'bucket' in connection config")
	}

	return &S3Connector{
		config:    ds,
		endpoint:  endpoint,
		bucket:    bucket,
		accessKey: accessKey,
		secretKey: secretKey,
		region:    region,
		prefix:    prefix,
	}, nil
}

// Connect 连接到 S3（模拟实现）
func (c *S3Connector) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	time.Sleep(100 * time.Millisecond)
	c.connected = true
	return nil
}

// Disconnect 断开连接
func (c *S3Connector) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	return nil
}

// Read 读取 S3 数据（模拟实现）
func (c *S3Connector) Read(ctx context.Context) ([]map[string]interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return nil, fmt.Errorf("S3 connector not connected")
	}

	return []map[string]interface{}{
		{
			"source":   "s3",
			"bucket":   c.bucket,
			"key":      c.prefix + "data.json",
			"size":     2048,
			"modified": time.Now(),
		},
	}, nil
}

// Write 写入 S3
func (c *S3Connector) Write(ctx context.Context, data []map[string]interface{}) error {
	return fmt.Errorf("S3 connector write not implemented in this version")
}

// IsConnected 是否已连接
func (c *S3Connector) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// Ping 测试连接
func (c *S3Connector) Ping(ctx context.Context) error {
	return nil // 模拟实现
}

// NewDatabaseConnector 创建数据库连接器
func NewDatabaseConnector(ds DataSource) (*DatabaseConnector, error) {
	driver, _ := ds.Connection["driver"].(string)
	dsn, _ := ds.Connection["dsn"].(string)
	query, _ := ds.Connection["query"].(string)

	if driver == "" || dsn == "" {
		return nil, fmt.Errorf("database connector requires 'driver' and 'dsn' in connection config")
	}

	return &DatabaseConnector{
		config: ds,
		driver: driver,
		dsn:    dsn,
		query:  query,
	}, nil
}

// Connect 连接到数据库（模拟实现）
func (c *DatabaseConnector) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	time.Sleep(150 * time.Millisecond)
	c.connected = true
	return nil
}

// Disconnect 断开连接
func (c *DatabaseConnector) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	return nil
}

// Read 读取数据库数据（模拟实现）
func (c *DatabaseConnector) Read(ctx context.Context) ([]map[string]interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return nil, fmt.Errorf("database connector not connected")
	}

	return []map[string]interface{}{
		{
			"source": "database",
			"driver": c.driver,
			"query":  c.query,
			"rows":   100,
		},
	}, nil
}

// Write 写入数据库
func (c *DatabaseConnector) Write(ctx context.Context, data []map[string]interface{}) error {
	return fmt.Errorf("database connector write not implemented in this version")
}

// IsConnected 是否已连接
func (c *DatabaseConnector) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// Ping 测试连接
func (c *DatabaseConnector) Ping(ctx context.Context) error {
	return nil // 模拟实现
}

// NewHTTPConnector 创建 HTTP 连接器
func NewHTTPConnector(ds DataSource) (*HTTPConnector, error) {
	url, _ := ds.Connection["url"].(string)
	method, _ := ds.Connection["method"].(string)
	headers, _ := ds.Connection["headers"].(map[string]string)

	if url == "" {
		return nil, fmt.Errorf("HTTP connector requires 'url' in connection config")
	}

	if method == "" {
		method = "GET"
	}

	return &HTTPConnector{
		config:  ds,
		url:     url,
		method:  strings.ToUpper(method),
		headers: headers,
	}, nil
}

// Connect 连接到 HTTP（模拟实现）
func (c *HTTPConnector) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.connected = true
	return nil
}

// Disconnect 断开连接
func (c *HTTPConnector) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	return nil
}

// Read 读取 HTTP 数据（模拟实现）
func (c *HTTPConnector) Read(ctx context.Context) ([]map[string]interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return nil, fmt.Errorf("HTTP connector not connected")
	}

	return []map[string]interface{}{
		{
			"source": "http",
			"url":    c.url,
			"method": c.method,
			"data":   "response_data",
		},
	}, nil
}

// Write 写入 HTTP
func (c *HTTPConnector) Write(ctx context.Context, data []map[string]interface{}) error {
	return fmt.Errorf("HTTP connector write not implemented in this version")
}

// IsConnected 是否已连接
func (c *HTTPConnector) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// Ping 测试连接
func (c *HTTPConnector) Ping(ctx context.Context) error {
	return nil // 模拟实现
}

// NewOutputConnector 创建输出连接器
func NewOutputConnector(on OutputNode) (OutputConnector, error) {
	switch on.Type {
	case OutputTypeFile:
		return NewFileOutputConnector(on)
	case OutputTypeWebhook:
		return NewWebhookOutputConnector(on)
	case OutputTypeNotification:
		return NewNotificationOutputConnector(on)
	case OutputTypeDatabase:
		// 数据库输出复用 DatabaseConnector
		return nil, fmt.Errorf("database output connector not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported output type: %s", on.Type)
	}
}

// NewFileOutputConnector 创建文件输出连接器
func NewFileOutputConnector(on OutputNode) (*FileOutputConnector, error) {
	basePath, _ := on.Config["path"].(string)
	format, _ := on.Config["format"].(string)

	if basePath == "" {
		return nil, fmt.Errorf("file output connector requires 'path' in config")
	}

	if format == "" {
		format = "json"
	}

	return &FileOutputConnector{
		config:   on,
		basePath: basePath,
		format:   format,
	}, nil
}

// Connect 连接到文件输出
func (c *FileOutputConnector) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 确保目录存在
	dir := filepath.Dir(c.basePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	c.connected = true
	return nil
}

// Disconnect 断开连接
func (c *FileOutputConnector) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	return nil
}

// Write 写入文件
func (c *FileOutputConnector) Write(ctx context.Context, data []map[string]interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return fmt.Errorf("file output connector not connected")
	}

	file, err := os.Create(c.basePath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// 简单写入 JSON 格式
	writer := io.Writer(file)
	writer.Write([]byte("[\n"))
	for i, item := range data {
		if i > 0 {
			writer.Write([]byte(",\n"))
		}
		// 简化：直接写入 map 的字符串表示
		writer.Write([]byte(fmt.Sprintf("  %v", item)))
	}
	writer.Write([]byte("\n]"))

	return nil
}

// IsConnected 是否已连接
func (c *FileOutputConnector) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// NewWebhookOutputConnector 创建 Webhook 输出连接器
func NewWebhookOutputConnector(on OutputNode) (*WebhookOutputConnector, error) {
	url, _ := on.Config["url"].(string)
	method, _ := on.Config["method"].(string)
	headers, _ := on.Config["headers"].(map[string]string)

	if url == "" {
		return nil, fmt.Errorf("webhook output connector requires 'url' in config")
	}

	if method == "" {
		method = "POST"
	}

	return &WebhookOutputConnector{
		config:  on,
		url:     url,
		method:  strings.ToUpper(method),
		headers: headers,
	}, nil
}

// Connect 连接到 Webhook
func (c *WebhookOutputConnector) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = true
	return nil
}

// Disconnect 断开连接
func (c *WebhookOutputConnector) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	return nil
}

// Write 发送 Webhook
func (c *WebhookOutputConnector) Write(ctx context.Context, data []map[string]interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return fmt.Errorf("webhook output connector not connected")
	}

	// 模拟发送 webhook
	time.Sleep(50 * time.Millisecond)
	return nil
}

// IsConnected 是否已连接
func (c *WebhookOutputConnector) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// NewNotificationOutputConnector 创建通知输出连接器
func NewNotificationOutputConnector(on OutputNode) (*NotificationOutputConnector, error) {
	channel, _ := on.Config["channel"].(string)
	message, _ := on.Config["message"].(string)

	return &NotificationOutputConnector{
		config:  on,
		channel: channel,
		message: message,
	}, nil
}

// Connect 连接到通知系统
func (c *NotificationOutputConnector) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = true
	return nil
}

// Disconnect 断开连接
func (c *NotificationOutputConnector) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	return nil
}

// Write 发送通知
func (c *NotificationOutputConnector) Write(ctx context.Context, data []map[string]interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return fmt.Errorf("notification output connector not connected")
	}

	// 模拟发送通知
	return nil
}

// IsConnected 是否已连接
func (c *NotificationOutputConnector) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}
