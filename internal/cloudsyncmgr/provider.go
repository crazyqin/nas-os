package cloudsyncmgr

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"
)

// FileInfo 云端文件信息.
type FileInfo struct {
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	ModTime      time.Time `json:"mod_time"`
	IsDir        bool      `json:"is_dir"`
	ETag         string    `json:"etag,omitempty"`
	ContentType  string    `json:"content_type,omitempty"`
}

// CloudProvider 云存储后端抽象接口.
type CloudProvider interface {
	// Type 返回提供商类型.
	Type() ProviderType

	// List 列出远程目录下的文件.
	List(ctx context.Context, remotePath string) ([]FileInfo, error)

	// Get 获取远程文件的读取流.
	Get(ctx context.Context, remotePath string) (io.ReadCloser, error)

	// Put 上传文件到远程.
	Put(ctx context.Context, remotePath string, reader io.Reader, size int64) error

	// Delete 删除远程文件.
	Delete(ctx context.Context, remotePath string) error

	// Stat 获取远程文件信息.
	Stat(ctx context.Context, remotePath string) (*FileInfo, error)

	// Mkdir 创建远程目录.
	Mkdir(ctx context.Context, remotePath string) error

	// TestConnection 测试连接.
	TestConnection(ctx context.Context) error
}

// ProviderFactory 创建云存储提供商.
type ProviderFactory func(config map[string]string) (CloudProvider, error)

var providerFactories = map[ProviderType]ProviderFactory{}

// RegisterProvider 注册提供商工厂.
func RegisterProvider(ptype ProviderType, factory ProviderFactory) {
	providerFactories[ptype] = factory
}

// CreateProvider 根据配置创建提供商.
func CreateProvider(ptype ProviderType, config map[string]string) (CloudProvider, error) {
	factory, ok := providerFactories[ptype]
	if !ok {
		return nil, fmt.Errorf("不支持的提供商类型: %s", ptype)
	}
	return factory(config)
}

// SupportedProviders 返回支持的提供商类型列表.
func SupportedProviders() []ProviderType {
	types := make([]ProviderType, 0, len(providerFactories))
	for t := range providerFactories {
		types = append(types, t)
	}
	return types
}

func init() {
	RegisterProvider(ProviderS3, newS3Provider)
	RegisterProvider(ProviderOSS, newOSSProvider)
	RegisterProvider(ProviderB2, newB2Provider)
	RegisterProvider(ProviderOneDrive, newOneDriveProvider)
}

// ============================================================
// S3 Provider
// ============================================================

// s3Provider Amazon S3 兼容存储.
type s3Provider struct {
	endpoint  string
	bucket    string
	region    string
	accessKey string
	secretKey string
}

func newS3Provider(config map[string]string) (CloudProvider, error) {
	if config["bucket"] == "" {
		return nil, fmt.Errorf("S3: bucket 不能为空")
	}
	if config["access_key"] == "" || config["secret_key"] == "" {
		return nil, fmt.Errorf("S3: access_key 和 secret_key 不能为空")
	}
	return &s3Provider{
		endpoint:  config["endpoint"],
		bucket:    config["bucket"],
		region:    config["region"],
		accessKey: config["access_key"],
		secretKey: config["secret_key"],
	}, nil
}

func (p *s3Provider) Type() ProviderType { return ProviderS3 }

func (p *s3Provider) List(ctx context.Context, remotePath string) ([]FileInfo, error) {
	return nil, fmt.Errorf("S3 List: 未连接实际后端 (演示模式)")
}

func (p *s3Provider) Get(ctx context.Context, remotePath string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("S3 Get: 未连接实际后端 (演示模式)")
}

func (p *s3Provider) Put(ctx context.Context, remotePath string, reader io.Reader, size int64) error {
	return fmt.Errorf("S3 Put: 未连接实际后端 (演示模式)")
}

func (p *s3Provider) Delete(ctx context.Context, remotePath string) error {
	return fmt.Errorf("S3 Delete: 未连接实际后端 (演示模式)")
}

func (p *s3Provider) Stat(ctx context.Context, remotePath string) (*FileInfo, error) {
	return nil, fmt.Errorf("S3 Stat: 未连接实际后端 (演示模式)")
}

func (p *s3Provider) Mkdir(ctx context.Context, remotePath string) error {
	return nil // S3 没有真正的目录概念
}

func (p *s3Provider) TestConnection(ctx context.Context) error {
	if p.bucket == "" {
		return fmt.Errorf("bucket 未配置")
	}
	return nil
}

// ============================================================
// OSS Provider (阿里云)
// ============================================================

type ossProvider struct {
	endpoint  string
	bucket    string
	accessKey string
	secretKey string
}

func newOSSProvider(config map[string]string) (CloudProvider, error) {
	if config["bucket"] == "" {
		return nil, fmt.Errorf("OSS: bucket 不能为空")
	}
	if config["access_key"] == "" || config["secret_key"] == "" {
		return nil, fmt.Errorf("OSS: access_key 和 secret_key 不能为空")
	}
	return &ossProvider{
		endpoint:  config["endpoint"],
		bucket:    config["bucket"],
		accessKey: config["access_key"],
		secretKey: config["secret_key"],
	}, nil
}

func (p *ossProvider) Type() ProviderType { return ProviderOSS }

func (p *ossProvider) List(ctx context.Context, remotePath string) ([]FileInfo, error) {
	return nil, fmt.Errorf("OSS List: 未连接实际后端 (演示模式)")
}

func (p *ossProvider) Get(ctx context.Context, remotePath string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("OSS Get: 未连接实际后端 (演示模式)")
}

func (p *ossProvider) Put(ctx context.Context, remotePath string, reader io.Reader, size int64) error {
	return fmt.Errorf("OSS Put: 未连接实际后端 (演示模式)")
}

func (p *ossProvider) Delete(ctx context.Context, remotePath string) error {
	return fmt.Errorf("OSS Delete: 未连接实际后端 (演示模式)")
}

func (p *ossProvider) Stat(ctx context.Context, remotePath string) (*FileInfo, error) {
	return nil, fmt.Errorf("OSS Stat: 未连接实际后端 (演示模式)")
}

func (p *ossProvider) Mkdir(ctx context.Context, remotePath string) error {
	return nil
}

func (p *ossProvider) TestConnection(ctx context.Context) error {
	if p.bucket == "" {
		return fmt.Errorf("bucket 未配置")
	}
	return nil
}

// ============================================================
// B2 Provider (Backblaze B2)
// ============================================================

type b2Provider struct {
	keyID     string
	appKey    string
	bucketID  string
	bucket    string
}

func newB2Provider(config map[string]string) (CloudProvider, error) {
	if config["key_id"] == "" || config["app_key"] == "" {
		return nil, fmt.Errorf("B2: key_id 和 app_key 不能为空")
	}
	return &b2Provider{
		keyID:    config["key_id"],
		appKey:   config["app_key"],
		bucketID: config["bucket_id"],
		bucket:   config["bucket"],
	}, nil
}

func (p *b2Provider) Type() ProviderType { return ProviderB2 }

func (p *b2Provider) List(ctx context.Context, remotePath string) ([]FileInfo, error) {
	return nil, fmt.Errorf("B2 List: 未连接实际后端 (演示模式)")
}

func (p *b2Provider) Get(ctx context.Context, remotePath string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("B2 Get: 未连接实际后端 (演示模式)")
}

func (p *b2Provider) Put(ctx context.Context, remotePath string, reader io.Reader, size int64) error {
	return fmt.Errorf("B2 Put: 未连接实际后端 (演示模式)")
}

func (p *b2Provider) Delete(ctx context.Context, remotePath string) error {
	return fmt.Errorf("B2 Delete: 未连接实际后端 (演示模式)")
}

func (p *b2Provider) Stat(ctx context.Context, remotePath string) (*FileInfo, error) {
	return nil, fmt.Errorf("B2 Stat: 未连接实际后端 (演示模式)")
}

func (p *b2Provider) Mkdir(ctx context.Context, remotePath string) error {
	return nil
}

func (p *b2Provider) TestConnection(ctx context.Context) error {
	if p.keyID == "" {
		return fmt.Errorf("key_id 未配置")
	}
	return nil
}

// ============================================================
// OneDrive Provider
// ============================================================

type oneDriveProvider struct {
	accessToken  string
	refreshToken string
	clientID     string
	clientSecret string
}

func newOneDriveProvider(config map[string]string) (CloudProvider, error) {
	if config["client_id"] == "" || config["client_secret"] == "" {
		return nil, fmt.Errorf("OneDrive: client_id 和 client_secret 不能为空")
	}
	return &oneDriveProvider{
		accessToken:  config["access_token"],
		refreshToken: config["refresh_token"],
		clientID:     config["client_id"],
		clientSecret: config["client_secret"],
	}, nil
}

func (p *oneDriveProvider) Type() ProviderType { return ProviderOneDrive }

func (p *oneDriveProvider) List(ctx context.Context, remotePath string) ([]FileInfo, error) {
	return nil, fmt.Errorf("OneDrive List: 未连接实际后端 (演示模式)")
}

func (p *oneDriveProvider) Get(ctx context.Context, remotePath string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("OneDrive Get: 未连接实际后端 (演示模式)")
}

func (p *oneDriveProvider) Put(ctx context.Context, remotePath string, reader io.Reader, size int64) error {
	return fmt.Errorf("OneDrive Put: 未连接实际后端 (演示模式)")
}

func (p *oneDriveProvider) Delete(ctx context.Context, remotePath string) error {
	return fmt.Errorf("OneDrive Delete: 未连接实际后端 (演示模式)")
}

func (p *oneDriveProvider) Stat(ctx context.Context, remotePath string) (*FileInfo, error) {
	return nil, fmt.Errorf("OneDrive Stat: 未连接实际后端 (演示模式)")
}

func (p *oneDriveProvider) Mkdir(ctx context.Context, remotePath string) error {
	return fmt.Errorf("OneDrive Mkdir: 未连接实际后端 (演示模式)")
}

func (p *oneDriveProvider) TestConnection(ctx context.Context) error {
	if p.clientID == "" {
		return fmt.Errorf("client_id 未配置")
	}
	return nil
}

// ============================================================
// 辅助函数
// ============================================================

// joinPath 拼接路径，保证路径分隔符正确.
func joinPath(base, sub string) string {
	base = strings.TrimRight(base, "/")
	sub = strings.TrimLeft(sub, "/")
	return base + "/" + sub
}

// cleanPath 清理路径.
func cleanPath(p string) string {
	return path.Clean(p)
}
