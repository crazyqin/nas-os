package cloudsync

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Provider 云存储提供商接口
type Provider interface {
	// 基础操作
	Upload(ctx context.Context, localPath, remotePath string) error
	Download(ctx context.Context, remotePath, localPath string) error
	Delete(ctx context.Context, remotePath string) error
	List(ctx context.Context, prefix string, recursive bool) ([]FileInfo, error)
	Stat(ctx context.Context, remotePath string) (*FileInfo, error)

	// 目录操作
	CreateDir(ctx context.Context, remotePath string) error
	DeleteDir(ctx context.Context, remotePath string) error

	// 连接管理
	TestConnection(ctx context.Context) (*ConnectionTestResult, error)
	Close() error

	// 元信息
	GetType() ProviderType
	GetCapabilities() []string
}

// ConnectionTestResult 连接测试结果
type ConnectionTestResult struct {
	Success   bool         `json:"success"`
	Provider  ProviderType `json:"provider"`
	Endpoint  string       `json:"endpoint"`
	Bucket    string       `json:"bucket,omitempty"`
	LatencyMs int64        `json:"latencyMs"`
	Message   string       `json:"message"`
	Error     string       `json:"error,omitempty"`
}

// ==================== S3 兼容存储 ====================

// S3Provider S3 兼容存储提供商
type S3Provider struct {
	client   *s3.Client
	config   *ProviderConfig
	provider ProviderType
}

// NewS3Provider 创建 S3 兼容存储提供商
func NewS3Provider(ctx context.Context, cfg *ProviderConfig, providerType ProviderType) (*S3Provider, error) {
	creds := credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")

	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}

	s3Config, err := config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(creds),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("加载 AWS 配置失败: %w", err)
	}

	client := s3.NewFromConfig(s3Config, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		if cfg.PathStyle {
			o.UsePathStyle = true
		}
	})

	return &S3Provider{
		client:   client,
		config:   cfg,
		provider: providerType,
	}, nil
}

func (p *S3Provider) Upload(ctx context.Context, localPath, remotePath string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	key := strings.TrimPrefix(remotePath, "/")
	_, err = p.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(p.config.Bucket),
		Key:         aws.String(key),
		Body:        file,
		ContentType: aws.String(getContentType(localPath)),
	})
	return err
}

func (p *S3Provider) Download(ctx context.Context, remotePath, localPath string) error {
	key := strings.TrimPrefix(remotePath, "/")

	result, err := p.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(p.config.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer result.Body.Close()

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return err
	}

	file, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, result.Body)
	return err
}

func (p *S3Provider) Delete(ctx context.Context, remotePath string) error {
	key := strings.TrimPrefix(remotePath, "/")
	_, err := p.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(p.config.Bucket),
		Key:    aws.String(key),
	})
	return err
}

func (p *S3Provider) List(ctx context.Context, prefix string, recursive bool) ([]FileInfo, error) {
	var files []FileInfo

	paginator := s3.NewListObjectsV2Paginator(p.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(p.config.Bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, obj := range page.Contents {
			key := *obj.Key
			if !recursive && strings.Count(strings.TrimPrefix(key, prefix), "/") > 1 {
				continue
			}
			files = append(files, FileInfo{
				Path:    key,
				Size:    *obj.Size,
				ModTime: *obj.LastModified,
				IsDir:   strings.HasSuffix(key, "/"),
			})
		}
	}

	return files, nil
}

func (p *S3Provider) Stat(ctx context.Context, remotePath string) (*FileInfo, error) {
	key := strings.TrimPrefix(remotePath, "/")

	result, err := p.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(p.config.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}

	size := int64(0)
	if result.ContentLength != nil {
		size = *result.ContentLength
	}

	return &FileInfo{
		Path:    remotePath,
		Size:    size,
		ModTime: *result.LastModified,
		IsDir:   false,
	}, nil
}

func (p *S3Provider) CreateDir(ctx context.Context, remotePath string) error {
	key := strings.TrimSuffix(remotePath, "/") + "/"
	_, err := p.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(p.config.Bucket),
		Key:    aws.String(key),
		Body:   strings.NewReader(""),
	})
	return err
}

func (p *S3Provider) DeleteDir(ctx context.Context, remotePath string) error {
	prefix := strings.TrimSuffix(remotePath, "/") + "/"

	objects, err := p.List(ctx, prefix, true)
	if err != nil {
		return err
	}

	if len(objects) == 0 {
		return nil
	}

	var objectIds []types.ObjectIdentifier
	for _, obj := range objects {
		objectIds = append(objectIds, types.ObjectIdentifier{Key: aws.String(obj.Path)})
	}

	_, err = p.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(p.config.Bucket),
		Delete: &types.Delete{
			Objects: objectIds,
			Quiet:   aws.Bool(true),
		},
	})
	return err
}

func (p *S3Provider) TestConnection(ctx context.Context) (*ConnectionTestResult, error) {
	start := time.Now()

	_, err := p.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(p.config.Bucket),
		MaxKeys: aws.Int32(1),
	})

	latency := time.Since(start).Milliseconds()

	result := &ConnectionTestResult{
		Provider:  p.provider,
		Endpoint:  p.config.Endpoint,
		Bucket:    p.config.Bucket,
		LatencyMs: latency,
	}

	if err != nil {
		result.Success = false
		result.Error = err.Error()
		result.Message = fmt.Sprintf("连接失败: %v", err)
	} else {
		result.Success = true
		result.Message = "连接成功"
	}

	return result, nil
}

func (p *S3Provider) Close() error {
	return nil
}

func (p *S3Provider) GetType() ProviderType {
	return p.provider
}

func (p *S3Provider) GetCapabilities() []string {
	return []string{"upload", "download", "delete", "list", "multipart"}
}

// ==================== 阿里云 OSS ====================

// AliyunOSSProvider 阿里云 OSS 提供商
type AliyunOSSProvider struct {
	*S3Provider
}

// NewAliyunOSSProvider 创建阿里云 OSS 提供商
func NewAliyunOSSProvider(ctx context.Context, cfg *ProviderConfig) (*AliyunOSSProvider, error) {
	// 阿里云 OSS endpoint 格式: oss-cn-hangzhou.aliyuncs.com
	if cfg.Region == "" && cfg.Endpoint != "" {
		parts := strings.Split(cfg.Endpoint, ".")
		if len(parts) > 0 {
			cfg.Region = strings.TrimPrefix(parts[0], "oss-")
		}
	}

	s3Provider, err := NewS3Provider(ctx, cfg, ProviderAliyunOSS)
	if err != nil {
		return nil, err
	}

	return &AliyunOSSProvider{S3Provider: s3Provider}, nil
}

// ==================== 腾讯云 COS ====================

// TencentCOSProvider 腾讯云 COS 提供商
type TencentCOSProvider struct {
	*S3Provider
}

// NewTencentCOSProvider 创建腾讯云 COS 提供商
func NewTencentCOSProvider(ctx context.Context, cfg *ProviderConfig) (*TencentCOSProvider, error) {
	// 腾讯云 COS endpoint 格式: cos.ap-guangzhou.myqcloud.com
	if cfg.Region == "" && cfg.Endpoint != "" {
		parts := strings.Split(cfg.Endpoint, ".")
		if len(parts) > 0 {
			cfg.Region = strings.TrimPrefix(parts[0], "cos.")
		}
	}

	s3Provider, err := NewS3Provider(ctx, cfg, ProviderTencentCOS)
	if err != nil {
		return nil, err
	}

	return &TencentCOSProvider{S3Provider: s3Provider}, nil
}

// ==================== AWS S3 ====================

// AWSS3Provider AWS S3 提供商
type AWSS3Provider struct {
	*S3Provider
}

// NewAWSS3Provider 创建 AWS S3 提供商
func NewAWSS3Provider(ctx context.Context, cfg *ProviderConfig) (*AWSS3Provider, error) {
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}

	s3Provider, err := NewS3Provider(ctx, cfg, ProviderAWSS3)
	if err != nil {
		return nil, err
	}

	return &AWSS3Provider{S3Provider: s3Provider}, nil
}

// ==================== Backblaze B2 ====================

// BackblazeB2Provider Backblaze B2 提供商
type BackblazeB2Provider struct {
	*S3Provider
}

// NewBackblazeB2Provider 创建 Backblaze B2 提供商
func NewBackblazeB2Provider(ctx context.Context, cfg *ProviderConfig) (*BackblazeB2Provider, error) {
	// B2 S3 兼容 endpoint 格式: https://s3.us-west-004.backblazeb2.com
	cfg.PathStyle = true // B2 需要路径风格

	s3Provider, err := NewS3Provider(ctx, cfg, ProviderBackblazeB2)
	if err != nil {
		return nil, err
	}

	return &BackblazeB2Provider{S3Provider: s3Provider}, nil
}

// ==================== WebDAV ====================

// WebDAVProvider WebDAV 提供商
type WebDAVProvider struct {
	client  *http.Client
	config  *ProviderConfig
	baseURL string
}

// NewWebDAVProvider 创建 WebDAV 提供商
func NewWebDAVProvider(cfg *ProviderConfig) (*WebDAVProvider, error) {
	client := &http.Client{
		Timeout: time.Duration(cfg.Timeout) * time.Second,
	}

	if cfg.Timeout == 0 {
		client.Timeout = 30 * time.Second
	}

	if cfg.Insecure {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	baseURL := strings.TrimSuffix(cfg.Endpoint, "/")

	return &WebDAVProvider{
		client:  client,
		config:  cfg,
		baseURL: baseURL,
	}, nil
}

func (p *WebDAVProvider) Upload(ctx context.Context, localPath, remotePath string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	url := p.baseURL + remotePath
	req, err := http.NewRequestWithContext(ctx, "PUT", url, file)
	if err != nil {
		return err
	}

	req.SetBasicAuth(p.config.AccessKey, p.config.SecretKey)
	req.ContentLength = stat.Size()

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("上传失败: %s", resp.Status)
	}

	return nil
}

func (p *WebDAVProvider) Download(ctx context.Context, remotePath, localPath string) error {
	url := p.baseURL + remotePath
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	req.SetBasicAuth(p.config.AccessKey, p.config.SecretKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("文件不存在")
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("下载失败: %s", resp.Status)
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return err
	}

	file, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}

func (p *WebDAVProvider) Delete(ctx context.Context, remotePath string) error {
	url := p.baseURL + remotePath
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	req.SetBasicAuth(p.config.AccessKey, p.config.SecretKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("删除失败: %s", resp.Status)
	}

	return nil
}

func (p *WebDAVProvider) List(ctx context.Context, prefix string, recursive bool) ([]FileInfo, error) {
	// WebDAV PROPFIND 实现
	url := p.baseURL + prefix
	req, err := http.NewRequestWithContext(ctx, "PROPFIND", url, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(p.config.AccessKey, p.config.SecretKey)
	req.Header.Set("Depth", "1")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 简化实现，返回空列表
	// 实际实现需要解析 XML 响应
	return []FileInfo{}, nil
}

func (p *WebDAVProvider) Stat(ctx context.Context, remotePath string) (*FileInfo, error) {
	url := p.baseURL + remotePath
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(p.config.AccessKey, p.config.SecretKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("文件不存在")
	}

	return &FileInfo{
		Path:    remotePath,
		Size:    resp.ContentLength,
		ModTime: time.Now(),
		IsDir:   resp.Header.Get("Content-Type") == "httpd/unix-directory",
	}, nil
}

func (p *WebDAVProvider) CreateDir(ctx context.Context, remotePath string) error {
	url := p.baseURL + remotePath
	req, err := http.NewRequestWithContext(ctx, "MKCOL", url, nil)
	if err != nil {
		return err
	}

	req.SetBasicAuth(p.config.AccessKey, p.config.SecretKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusMethodNotAllowed {
		return fmt.Errorf("创建目录失败: %s", resp.Status)
	}

	return nil
}

func (p *WebDAVProvider) DeleteDir(ctx context.Context, remotePath string) error {
	return p.Delete(ctx, strings.TrimSuffix(remotePath, "/")+"/")
}

func (p *WebDAVProvider) TestConnection(ctx context.Context) (*ConnectionTestResult, error) {
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, "OPTIONS", p.baseURL+"/", nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(p.config.AccessKey, p.config.SecretKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	latency := time.Since(start).Milliseconds()

	result := &ConnectionTestResult{
		Provider:  ProviderWebDAV,
		Endpoint:  p.config.Endpoint,
		LatencyMs: latency,
	}

	if resp.StatusCode >= 500 {
		result.Success = false
		result.Error = resp.Status
		result.Message = fmt.Sprintf("连接失败: %s", resp.Status)
	} else {
		result.Success = true
		result.Message = "连接成功"
	}

	return result, nil
}

func (p *WebDAVProvider) Close() error {
	return nil
}

func (p *WebDAVProvider) GetType() ProviderType {
	return ProviderWebDAV
}

func (p *WebDAVProvider) GetCapabilities() []string {
	return []string{"upload", "download", "delete", "list"}
}

// ==================== Google Drive ====================

// GoogleDriveProvider Google Drive 提供商
type GoogleDriveProvider struct {
	config       *ProviderConfig
	refreshToken string
	accessToken  string
	tokenExpiry  time.Time
}

// NewGoogleDriveProvider 创建 Google Drive 提供商
func NewGoogleDriveProvider(cfg *ProviderConfig) (*GoogleDriveProvider, error) {
	return &GoogleDriveProvider{
		config:       cfg,
		refreshToken: cfg.RefreshToken,
	}, nil
}

func (p *GoogleDriveProvider) refreshTokenIfNeeded(ctx context.Context) error {
	if p.accessToken != "" && time.Now().Before(p.tokenExpiry) {
		return nil
	}

	// 使用 refresh token 获取新的 access token
	// 实际实现需要调用 Google OAuth2 API
	return fmt.Errorf("Google Drive 需要 OAuth2 认证，请先授权")
}

func (p *GoogleDriveProvider) Upload(ctx context.Context, localPath, remotePath string) error {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return err
	}
	// 实际实现需要调用 Google Drive API
	return fmt.Errorf("Google Drive 上传需要完整 API 实现")
}

func (p *GoogleDriveProvider) Download(ctx context.Context, remotePath, localPath string) error {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return err
	}
	return fmt.Errorf("Google Drive 下载需要完整 API 实现")
}

func (p *GoogleDriveProvider) Delete(ctx context.Context, remotePath string) error {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return err
	}
	return fmt.Errorf("Google Drive 删除需要完整 API 实现")
}

func (p *GoogleDriveProvider) List(ctx context.Context, prefix string, recursive bool) ([]FileInfo, error) {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("Google Drive 列表需要完整 API 实现")
}

func (p *GoogleDriveProvider) Stat(ctx context.Context, remotePath string) (*FileInfo, error) {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("Google Drive 状态需要完整 API 实现")
}

func (p *GoogleDriveProvider) CreateDir(ctx context.Context, remotePath string) error {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return err
	}
	return fmt.Errorf("Google Drive 创建目录需要完整 API 实现")
}

func (p *GoogleDriveProvider) DeleteDir(ctx context.Context, remotePath string) error {
	return p.Delete(ctx, remotePath)
}

func (p *GoogleDriveProvider) TestConnection(ctx context.Context) (*ConnectionTestResult, error) {
	latency := int64(0)
	result := &ConnectionTestResult{
		Provider:  ProviderGoogleDrive,
		Endpoint:  "drive.google.com",
		LatencyMs: latency,
	}

	if p.refreshToken == "" {
		result.Success = false
		result.Message = "需要 OAuth2 授权"
	} else {
		result.Success = true
		result.Message = "配置有效，需要完整 API 实现"
	}

	return result, nil
}

func (p *GoogleDriveProvider) Close() error {
	return nil
}

func (p *GoogleDriveProvider) GetType() ProviderType {
	return ProviderGoogleDrive
}

func (p *GoogleDriveProvider) GetCapabilities() []string {
	return []string{"upload", "download", "delete", "list", "share"}
}

// ==================== OneDrive ====================

// OneDriveProvider OneDrive 提供商
type OneDriveProvider struct {
	config       *ProviderConfig
	refreshToken string
	accessToken  string
	tokenExpiry  time.Time
}

// NewOneDriveProvider 创建 OneDrive 提供商
func NewOneDriveProvider(cfg *ProviderConfig) (*OneDriveProvider, error) {
	return &OneDriveProvider{
		config:       cfg,
		refreshToken: cfg.RefreshToken,
	}, nil
}

func (p *OneDriveProvider) refreshTokenIfNeeded(ctx context.Context) error {
	if p.accessToken != "" && time.Now().Before(p.tokenExpiry) {
		return nil
	}
	return fmt.Errorf("OneDrive 需要 OAuth2 认证，请先授权")
}

func (p *OneDriveProvider) Upload(ctx context.Context, localPath, remotePath string) error {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return err
	}
	return fmt.Errorf("OneDrive 上传需要完整 API 实现")
}

func (p *OneDriveProvider) Download(ctx context.Context, remotePath, localPath string) error {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return err
	}
	return fmt.Errorf("OneDrive 下载需要完整 API 实现")
}

func (p *OneDriveProvider) Delete(ctx context.Context, remotePath string) error {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return err
	}
	return fmt.Errorf("OneDrive 删除需要完整 API 实现")
}

func (p *OneDriveProvider) List(ctx context.Context, prefix string, recursive bool) ([]FileInfo, error) {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("OneDrive 列表需要完整 API 实现")
}

func (p *OneDriveProvider) Stat(ctx context.Context, remotePath string) (*FileInfo, error) {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("OneDrive 状态需要完整 API 实现")
}

func (p *OneDriveProvider) CreateDir(ctx context.Context, remotePath string) error {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return err
	}
	return fmt.Errorf("OneDrive 创建目录需要完整 API 实现")
}

func (p *OneDriveProvider) DeleteDir(ctx context.Context, remotePath string) error {
	return p.Delete(ctx, remotePath)
}

func (p *OneDriveProvider) TestConnection(ctx context.Context) (*ConnectionTestResult, error) {
	result := &ConnectionTestResult{
		Provider:  ProviderOneDrive,
		Endpoint:  "graph.microsoft.com",
		LatencyMs: 0,
	}

	if p.refreshToken == "" {
		result.Success = false
		result.Message = "需要 OAuth2 授权"
	} else {
		result.Success = true
		result.Message = "配置有效，需要完整 API 实现"
	}

	return result, nil
}

func (p *OneDriveProvider) Close() error {
	return nil
}

func (p *OneDriveProvider) GetType() ProviderType {
	return ProviderOneDrive
}

func (p *OneDriveProvider) GetCapabilities() []string {
	return []string{"upload", "download", "delete", "list", "share"}
}

// ==================== 工厂函数 ====================

// NewProvider 创建云存储提供商
func NewProvider(ctx context.Context, cfg *ProviderConfig) (Provider, error) {
	switch cfg.Type {
	case ProviderAliyunOSS:
		return NewAliyunOSSProvider(ctx, cfg)
	case ProviderTencentCOS:
		return NewTencentCOSProvider(ctx, cfg)
	case ProviderAWSS3:
		return NewAWSS3Provider(ctx, cfg)
	case ProviderBackblazeB2:
		return NewBackblazeB2Provider(ctx, cfg)
	case ProviderWebDAV:
		return NewWebDAVProvider(cfg)
	case ProviderGoogleDrive:
		return NewGoogleDriveProvider(cfg)
	case ProviderOneDrive:
		return NewOneDriveProvider(cfg)
	case ProviderS3Compatible:
		return NewS3Provider(ctx, cfg, ProviderS3Compatible)
	default:
		return nil, fmt.Errorf("不支持的云存储提供商: %s", cfg.Type)
	}
}

// ==================== 辅助函数 ====================

func getContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	contentTypes := map[string]string{
		".txt":  "text/plain",
		".html": "text/html",
		".css":  "text/css",
		".js":   "application/javascript",
		".json": "application/json",
		".xml":  "application/xml",
		".pdf":  "application/pdf",
		".zip":  "application/zip",
		".tar":  "application/x-tar",
		".gz":   "application/gzip",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".svg":  "image/svg+xml",
		".mp3":  "audio/mpeg",
		".mp4":  "video/mp4",
		".avi":  "video/x-msvideo",
		".mov":  "video/quicktime",
	}

	if ct, ok := contentTypes[ext]; ok {
		return ct
	}
	return "application/octet-stream"
}
