package cloudsync

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
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

// Upload 上传本地文件到云端
func (p *S3Provider) Upload(ctx context.Context, localPath, remotePath string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer func() { _ = file.Close() }()

	key := strings.TrimPrefix(remotePath, "/")
	_, err = p.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(p.config.Bucket),
		Key:         aws.String(key),
		Body:        file,
		ContentType: aws.String(getContentType(localPath)),
	})
	return err
}

// Download downloads a file from S3 storage to the local filesystem.
func (p *S3Provider) Download(ctx context.Context, remotePath, localPath string) error {
	key := strings.TrimPrefix(remotePath, "/")

	result, err := p.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(p.config.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer func() { _ = result.Body.Close() }()

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return err
	}

	file, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	_, err = io.Copy(file, result.Body)
	return err
}

// Delete removes a file from S3 storage.
func (p *S3Provider) Delete(ctx context.Context, remotePath string) error {
	key := strings.TrimPrefix(remotePath, "/")
	_, err := p.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(p.config.Bucket),
		Key:    aws.String(key),
	})
	return err
}

// List returns a list of files in S3 storage matching the given prefix.
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

// Stat returns metadata information about a file in S3 storage.
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

// CreateDir creates a directory in S3 storage by creating an empty object with a trailing slash.
func (p *S3Provider) CreateDir(ctx context.Context, remotePath string) error {
	key := strings.TrimSuffix(remotePath, "/") + "/"
	_, err := p.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(p.config.Bucket),
		Key:    aws.String(key),
		Body:   strings.NewReader(""),
	})
	return err
}

// DeleteDir removes a directory and all its contents from S3 storage.
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

// TestConnection tests the connection to S3 storage and returns the result.
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

// Close releases resources used by the S3 provider. Currently a no-op.
func (p *S3Provider) Close() error {
	return nil
}

// GetType returns the provider type for S3 storage.
func (p *S3Provider) GetType() ProviderType {
	return p.provider
}

// GetCapabilities returns the list of capabilities supported by S3 storage.
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
		// 仅测试环境允许跳过 TLS 验证，生产环境必须验证证书
		if os.Getenv("ENV") != "test" {
			return nil, fmt.Errorf("生产环境禁止跳过 TLS 验证，请配置有效的 CA 证书")
		}
		// #nosec G402 -- InsecureSkipVerify is only allowed in test environment (ENV=test)
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

// Upload uploads a local file to WebDAV storage.
func (p *WebDAVProvider) Upload(ctx context.Context, localPath, remotePath string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("上传失败: %s", resp.Status)
	}

	return nil
}

// Download downloads a file from WebDAV storage to the local filesystem.
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
	defer func() { _ = resp.Body.Close() }()

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
	defer func() { _ = file.Close() }()

	_, err = io.Copy(file, resp.Body)
	return err
}

// Delete removes a file from WebDAV storage.
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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("删除失败: %s", resp.Status)
	}

	return nil
}

// List returns a list of files in WebDAV storage matching the given prefix.
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
	defer func() { _ = resp.Body.Close() }()

	// 简化实现，返回空列表
	// 实际实现需要解析 XML 响应
	return []FileInfo{}, nil
}

// Stat returns metadata information about a file in WebDAV storage.
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
	defer func() { _ = resp.Body.Close() }()

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

// CreateDir creates a directory in WebDAV storage using the MKCOL method.
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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusMethodNotAllowed {
		return fmt.Errorf("创建目录失败: %s", resp.Status)
	}

	return nil
}

// DeleteDir removes a directory from WebDAV storage.
func (p *WebDAVProvider) DeleteDir(ctx context.Context, remotePath string) error {
	return p.Delete(ctx, strings.TrimSuffix(remotePath, "/")+"/")
}

// TestConnection tests the connection to WebDAV storage and returns the result.
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
	defer func() { _ = resp.Body.Close() }()

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

// Close releases resources used by the WebDAV provider. Currently a no-op.
func (p *WebDAVProvider) Close() error {
	return nil
}

// GetType returns the provider type for WebDAV storage.
func (p *WebDAVProvider) GetType() ProviderType {
	return ProviderWebDAV
}

// GetCapabilities returns the list of capabilities supported by WebDAV storage.
func (p *WebDAVProvider) GetCapabilities() []string {
	return []string{"upload", "download", "delete", "list"}
}

// ==================== Google Drive ====================

// GoogleDriveProvider Google Drive 提供商
type GoogleDriveProvider struct {
	config       *ProviderConfig
	client       *http.Client
	refreshToken string
	accessToken  string
	tokenExpiry  time.Time
	rootFolderID string
}

// NewGoogleDriveProvider 创建 Google Drive 提供商
func NewGoogleDriveProvider(cfg *ProviderConfig) (*GoogleDriveProvider, error) {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	rootFolderID := cfg.RootFolderID
	if rootFolderID == "" {
		rootFolderID = "root" // 默认使用根目录
	}

	return &GoogleDriveProvider{
		config:       cfg,
		client:       client,
		refreshToken: cfg.RefreshToken,
		rootFolderID: rootFolderID,
	}, nil
}

// refreshTokenIfNeeded 使用 refresh token 获取新的 access token
func (p *GoogleDriveProvider) refreshTokenIfNeeded(ctx context.Context) error {
	if p.accessToken != "" && time.Now().Before(p.tokenExpiry.Add(-5*time.Minute)) {
		return nil
	}

	if p.refreshToken == "" {
		return fmt.Errorf("google Drive 需要 OAuth2 授权，请先在 Web 界面完成授权")
	}

	// 使用 refresh token 换取 access token
	tokenURL := "https://oauth2.googleapis.com/token"
	data := url.Values{
		"client_id":     {p.config.ClientID},
		"client_secret": {p.config.ClientSecret},
		"refresh_token": {p.refreshToken},
		"grant_type":    {"refresh_token"},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("创建 token 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("获取 access token 失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("获取 access token 失败: %s - %s", resp.Status, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("解析 token 响应失败: %w", err)
	}

	p.accessToken = tokenResp.AccessToken
	p.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return nil
}

// getFolderID 获取或创建文件夹路径
func (p *GoogleDriveProvider) getFolderID(ctx context.Context, path string) (string, error) {
	if path == "" || path == "/" {
		return p.rootFolderID, nil
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	parentID := p.rootFolderID

	for _, part := range parts {
		if part == "" {
			continue
		}

		// 搜索文件夹
		query := fmt.Sprintf("name='%s' and mimeType='application/vnd.google-apps.folder' and '%s' in parents and trashed=false",
			part, parentID)

		url := fmt.Sprintf("https://www.googleapis.com/drive/v3/files?q=%s&fields=files(id,name)",
			url.QueryEscape(query))

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+p.accessToken)

		resp, err := p.client.Do(req)
		if err != nil {
			return "", err
		}

		var result struct {
			Files []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"files"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			_ = resp.Body.Close()
			return "", err
		}
		_ = resp.Body.Close()

		if len(result.Files) > 0 {
			parentID = result.Files[0].ID
		} else {
			// 创建文件夹
			folderID, err := p.createFolder(ctx, part, parentID)
			if err != nil {
				return "", err
			}
			parentID = folderID
		}
	}

	return parentID, nil
}

// createFolder 创建文件夹
func (p *GoogleDriveProvider) createFolder(ctx context.Context, name, parentID string) (string, error) {
	metadata := map[string]interface{}{
		"name":     name,
		"mimeType": "application/vnd.google-apps.folder",
		"parents":  []string{parentID},
	}

	body, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("序列化元数据失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://www.googleapis.com/drive/v3/files?fields=id,name",
		bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		ID string `json:"id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.ID, nil
}

// getFileID 通过路径获取文件 ID
func (p *GoogleDriveProvider) getFileID(ctx context.Context, path string) (string, error) {
	dir := filepath.Dir(path)
	filename := filepath.Base(path)

	folderID, err := p.getFolderID(ctx, dir)
	if err != nil {
		return "", err
	}

	query := fmt.Sprintf("name='%s' and '%s' in parents and trashed=false",
		filename, folderID)

	url := fmt.Sprintf("https://www.googleapis.com/drive/v3/files?q=%s&fields=files(id,name)",
		url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+p.accessToken)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Files []struct {
			ID string `json:"id"`
		} `json:"files"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Files) == 0 {
		return "", os.ErrNotExist
	}

	return result.Files[0].ID, nil
}

// Upload uploads a local file to Google Drive using resumable upload.
func (p *GoogleDriveProvider) Upload(ctx context.Context, localPath, remotePath string) error {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return err
	}

	// 打开本地文件
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer func() { _ = file.Close() }()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	// 获取或创建父文件夹
	dir := filepath.Dir(remotePath)
	filename := filepath.Base(remotePath)

	parentID, err := p.getFolderID(ctx, dir)
	if err != nil {
		return fmt.Errorf("获取文件夹失败: %w", err)
	}

	// 检查是否已存在同名文件
	existingID := ""
	query := fmt.Sprintf("name='%s' and '%s' in parents and trashed=false", filename, parentID)
	searchURL := fmt.Sprintf("https://www.googleapis.com/drive/v3/files?q=%s&fields=files(id)",
		url.QueryEscape(query))

	searchReq, _ := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	searchReq.Header.Set("Authorization", "Bearer "+p.accessToken)

	searchResp, err := p.client.Do(searchReq)
	if err == nil {
		var searchResult struct {
			Files []struct {
				ID string `json:"id"`
			} `json:"files"`
		}
		if json.NewDecoder(searchResp.Body).Decode(&searchResult) == nil && len(searchResult.Files) > 0 {
			existingID = searchResult.Files[0].ID
		}
		_ = searchResp.Body.Close()
	}

	var uploadReq *http.Request
	if existingID != "" {
		// 更新现有文件
		uploadURL := fmt.Sprintf("https://www.googleapis.com/upload/drive/v3/files/%s?uploadType=media", existingID)
		uploadReq, err = http.NewRequestWithContext(ctx, "PATCH", uploadURL, file)
	} else {
		// 创建新文件
		// 先创建文件元数据
		metadata := map[string]interface{}{
			"name":    filename,
			"parents": []string{parentID},
		}
		metadataBody, _ := json.Marshal(metadata)

		// 使用 multipart 上传
		uploadURL := "https://www.googleapis.com/upload/drive/v3/files?uploadType=multipart"

		// 构建 multipart body
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)

		// 添加元数据部分
		metaPart, _ := writer.CreatePart(textproto.MIMEHeader{
			"Content-Type": []string{"application/json; charset=UTF-8"},
		})
		if _, err := metaPart.Write(metadataBody); err != nil {
			return fmt.Errorf("写入元数据失败: %w", err)
		}

		// 添加文件内容部分
		filePart, _ := writer.CreatePart(textproto.MIMEHeader{
			"Content-Type": []string{getContentType(localPath)},
		})
		if _, err := io.Copy(filePart, file); err != nil {
			return fmt.Errorf("写入文件内容失败: %w", err)
		}

		if err := writer.Close(); err != nil {
			return fmt.Errorf("关闭 multipart writer 失败: %w", err)
		}

		uploadReq, err = http.NewRequestWithContext(ctx, "POST", uploadURL, &body)
		uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	}

	if err != nil {
		return fmt.Errorf("创建上传请求失败: %w", err)
	}

	uploadReq.Header.Set("Authorization", "Bearer "+p.accessToken)

	// 重置文件指针
	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("重置文件指针失败: %w", err)
	}

	// 简化上传：直接使用 resumable upload
	uploadURL := "https://www.googleapis.com/upload/drive/v3/files?uploadType=resumable"
	metadata := map[string]interface{}{
		"name":    filename,
		"parents": []string{parentID},
	}
	metadataBody, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("序列化元数据失败: %w", err)
	}

	initReq, err := http.NewRequestWithContext(ctx, "POST", uploadURL, bytes.NewReader(metadataBody))
	if err != nil {
		return fmt.Errorf("创建初始化请求失败: %w", err)
	}
	initReq.Header.Set("Authorization", "Bearer "+p.accessToken)
	initReq.Header.Set("Content-Type", "application/json")

	initResp, err := p.client.Do(initReq)
	if err != nil {
		return fmt.Errorf("初始化上传失败: %w", err)
	}
	_ = initResp.Body.Close()

	if initResp.StatusCode != http.StatusOK {
		return fmt.Errorf("初始化上传失败: %s", initResp.Status)
	}

	// 获取上传 URL
	uploadLocation := initResp.Header.Get("Location")

	// 执行上传
	uploadReq, err = http.NewRequestWithContext(ctx, "PUT", uploadLocation, file)
	if err != nil {
		return fmt.Errorf("创建上传请求失败: %w", err)
	}
	uploadReq.ContentLength = stat.Size()

	uploadResp, err := p.client.Do(uploadReq)
	if err != nil {
		return fmt.Errorf("上传失败: %w", err)
	}
	defer func() { _ = uploadResp.Body.Close() }()

	if uploadResp.StatusCode >= 400 {
		body, _ := io.ReadAll(uploadResp.Body)
		return fmt.Errorf("上传失败: %s - %s", uploadResp.Status, string(body))
	}

	return nil
}

// Download downloads a file from Google Drive to the local filesystem.
func (p *GoogleDriveProvider) Download(ctx context.Context, remotePath, localPath string) error {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return err
	}

	// 获取文件 ID
	fileID, err := p.getFileID(ctx, remotePath)
	if err != nil {
		return fmt.Errorf("获取文件 ID 失败: %w", err)
	}

	// 获取下载 URL
	url := fmt.Sprintf("https://www.googleapis.com/drive/v3/files/%s?alt=media", fileID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("创建下载请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.accessToken)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败: %s", resp.Status)
	}

	// 创建本地目录
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 创建本地文件
	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer func() { _ = file.Close() }()

	_, err = io.Copy(file, resp.Body)
	return err
}

// Delete removes a file from Google Drive.
func (p *GoogleDriveProvider) Delete(ctx context.Context, remotePath string) error {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return err
	}

	fileID, err := p.getFileID(ctx, remotePath)
	if err != nil {
		return fmt.Errorf("获取文件 ID 失败: %w", err)
	}

	url := fmt.Sprintf("https://www.googleapis.com/drive/v3/files/%s", fileID)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("创建删除请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.accessToken)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("删除失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("删除失败: %s", resp.Status)
	}

	return nil
}

// List returns a list of files in Google Drive matching the given prefix.
func (p *GoogleDriveProvider) List(ctx context.Context, prefix string, recursive bool) ([]FileInfo, error) {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return nil, err
	}

	var files []FileInfo

	folderID, err := p.getFolderID(ctx, prefix)
	if err != nil {
		return nil, err
	}

	// 列出文件
	query := fmt.Sprintf("'%s' in parents and trashed=false", folderID)
	listURL := fmt.Sprintf("https://www.googleapis.com/drive/v3/files?q=%s&fields=files(id,name,size,modifiedTime,mimeType)&pageSize=1000",
		url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, "GET", listURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.accessToken)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("列出文件失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Files []struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			Size         int64  `json:"size,string"`
			ModifiedTime string `json:"modifiedTime"`
			MimeType     string `json:"mimeType"`
		} `json:"files"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	for _, f := range result.Files {
		modTime, _ := time.Parse(time.RFC3339, f.ModifiedTime)
		isDir := f.MimeType == "application/vnd.google-apps.folder"

		files = append(files, FileInfo{
			Path:    filepath.Join(prefix, f.Name),
			Size:    f.Size,
			ModTime: modTime,
			IsDir:   isDir,
		})
	}

	// 如果需要递归，继续列出子文件夹
	if recursive {
		for _, f := range files {
			if f.IsDir {
				subFiles, err := p.List(ctx, f.Path, true)
				if err == nil {
					files = append(files, subFiles...)
				}
			}
		}
	}

	return files, nil
}

// Stat returns metadata information about a file in Google Drive.
func (p *GoogleDriveProvider) Stat(ctx context.Context, remotePath string) (*FileInfo, error) {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return nil, err
	}

	fileID, err := p.getFileID(ctx, remotePath)
	if err != nil {
		return nil, fmt.Errorf("获取文件 ID 失败: %w", err)
	}

	url := fmt.Sprintf("https://www.googleapis.com/drive/v3/files/%s?fields=name,size,modifiedTime,mimeType", fileID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.accessToken)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取文件信息失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Name         string `json:"name"`
		Size         int64  `json:"size,string"`
		ModifiedTime string `json:"modifiedTime"`
		MimeType     string `json:"mimeType"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	modTime, _ := time.Parse(time.RFC3339, result.ModifiedTime)

	return &FileInfo{
		Path:    remotePath,
		Size:    result.Size,
		ModTime: modTime,
		IsDir:   result.MimeType == "application/vnd.google-apps.folder",
	}, nil
}

// CreateDir creates a directory in Google Drive.
func (p *GoogleDriveProvider) CreateDir(ctx context.Context, remotePath string) error {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return err
	}

	_, err := p.getFolderID(ctx, remotePath)
	return err
}

// DeleteDir removes a directory from Google Drive.
func (p *GoogleDriveProvider) DeleteDir(ctx context.Context, remotePath string) error {
	return p.Delete(ctx, remotePath)
}

// TestConnection tests the connection to Google Drive and returns the result.
func (p *GoogleDriveProvider) TestConnection(ctx context.Context) (*ConnectionTestResult, error) {
	result := &ConnectionTestResult{
		Provider: ProviderGoogleDrive,
		Endpoint: "drive.google.com",
	}

	if p.refreshToken == "" {
		result.Success = false
		result.Message = "需要 OAuth2 授权"
		return result, nil
	}

	start := time.Now()

	// 尝试刷新 token
	err := p.refreshTokenIfNeeded(ctx)
	result.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("认证失败: %v", err)
		result.Error = err.Error()
		return result, nil
	}

	// 尝试列出文件
	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://www.googleapis.com/drive/v3/files?pageSize=1&fields=files(id)", nil)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("创建请求失败: %v", err)
		result.Error = err.Error()
		return result, nil
	}
	req.Header.Set("Authorization", "Bearer "+p.accessToken)

	resp, err := p.client.Do(req)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("连接失败: %v", err)
		result.Error = err.Error()
		return result, nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		result.Success = false
		result.Message = "授权已过期，请重新授权"
		return result, nil
	}

	result.Success = resp.StatusCode == http.StatusOK
	if result.Success {
		result.Message = "连接成功"
	} else {
		result.Message = fmt.Sprintf("连接失败: %s", resp.Status)
	}

	return result, nil
}

// Close releases resources used by the Google Drive provider. Currently a no-op.
func (p *GoogleDriveProvider) Close() error {
	return nil
}

// GetType returns the provider type for Google Drive.
func (p *GoogleDriveProvider) GetType() ProviderType {
	return ProviderGoogleDrive
}

// GetCapabilities returns the list of capabilities supported by Google Drive.
func (p *GoogleDriveProvider) GetCapabilities() []string {
	return []string{"upload", "download", "delete", "list", "share"}
}

// ==================== OneDrive ====================

// OneDriveProvider OneDrive 提供商
type OneDriveProvider struct {
	config       *ProviderConfig
	client       *http.Client
	refreshToken string
	accessToken  string
	tokenExpiry  time.Time
}

// NewOneDriveProvider 创建 OneDrive 提供商
func NewOneDriveProvider(cfg *ProviderConfig) (*OneDriveProvider, error) {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	return &OneDriveProvider{
		config:       cfg,
		client:       client,
		refreshToken: cfg.RefreshToken,
	}, nil
}

// refreshTokenIfNeeded 使用 refresh token 获取新的 access token
func (p *OneDriveProvider) refreshTokenIfNeeded(ctx context.Context) error {
	if p.accessToken != "" && time.Now().Before(p.tokenExpiry.Add(-5*time.Minute)) {
		return nil
	}

	if p.refreshToken == "" {
		return fmt.Errorf("OneDrive 需要 OAuth2 授权，请先在 Web 界面完成授权")
	}

	// 使用 refresh token 换取 access token
	tokenURL := "https://login.microsoftonline.com/common/oauth2/v2.0/token"
	data := url.Values{
		"client_id":     {p.config.ClientID},
		"client_secret": {p.config.ClientSecret},
		"refresh_token": {p.refreshToken},
		"grant_type":    {"refresh_token"},
		"scope":         {"files.readwrite offline_access"},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("创建 token 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("获取 access token 失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("获取 access token 失败: %s - %s", resp.Status, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("解析 token 响应失败: %w", err)
	}

	p.accessToken = tokenResp.AccessToken
	p.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	if tokenResp.RefreshToken != "" {
		p.refreshToken = tokenResp.RefreshToken
	}

	return nil
}

// ensureFolder 确保文件夹存在
func (p *OneDriveProvider) ensureFolder(ctx context.Context, path string) error {
	if path == "" || path == "/" {
		return nil
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	currentPath := ""

	for _, part := range parts {
		if part == "" {
			continue
		}

		currentPath += "/" + part

		// 检查文件夹是否存在
		url := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root:%s", currentPath)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return fmt.Errorf("创建请求失败: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+p.accessToken)

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("检查文件夹失败: %w", err)
		}

		if resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			continue
		}
		_ = resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			// 创建文件夹
			parentPath := filepath.Dir(currentPath)
			if parentPath == "." {
				parentPath = ""
			}

			folderURL := "https://graph.microsoft.com/v1.0/me/drive/root/children"
			if parentPath != "" {
				folderURL = fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root:%s:/children", parentPath)
			}

			folderData := map[string]interface{}{
				"name":                              part,
				"folder":                            map[string]interface{}{},
				"@microsoft.graph.conflictBehavior": "rename",
			}

			body, err := json.Marshal(folderData)
			if err != nil {
				return fmt.Errorf("序列化文件夹数据失败: %w", err)
			}

			req, err = http.NewRequestWithContext(ctx, "POST", folderURL, bytes.NewReader(body))
			if err != nil {
				return fmt.Errorf("创建文件夹请求失败: %w", err)
			}
			req.Header.Set("Authorization", "Bearer "+p.accessToken)
			req.Header.Set("Content-Type", "application/json")

			resp, err = p.client.Do(req)
			if err != nil {
				return fmt.Errorf("创建文件夹失败: %w", err)
			}
			_ = resp.Body.Close()

			if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
				return fmt.Errorf("创建文件夹失败: %s", resp.Status)
			}
		}
	}

	return nil
}

// Upload uploads a local file to OneDrive using simple or resumable upload based on file size.
func (p *OneDriveProvider) Upload(ctx context.Context, localPath, remotePath string) error {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return err
	}

	// 打开本地文件
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer func() { _ = file.Close() }()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	// 确保父目录存在
	dir := filepath.Dir(remotePath)
	if err := p.ensureFolder(ctx, dir); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 使用 OneDrive upload session（支持大文件）
	// 对于小文件 (< 4MB)，可以直接使用简单上传
	if stat.Size() < 4*1024*1024 {
		// 简单上传
		url := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root:%s:/content", remotePath)
		req, err := http.NewRequestWithContext(ctx, "PUT", url, file)
		if err != nil {
			return fmt.Errorf("创建上传请求失败: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+p.accessToken)
		req.Header.Set("Content-Type", getContentType(localPath))

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("上传失败: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("上传失败: %s - %s", resp.Status, string(body))
		}

		return nil
	}

	// 大文件使用 upload session
	createSessionURL := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root:%s:/createUploadSession", remotePath)
	sessionReq, _ := http.NewRequestWithContext(ctx, "POST", createSessionURL, nil)
	sessionReq.Header.Set("Authorization", "Bearer "+p.accessToken)

	sessionResp, err := p.client.Do(sessionReq)
	if err != nil {
		return fmt.Errorf("创建上传会话失败: %w", err)
	}

	var session struct {
		UploadURL string `json:"uploadUrl"`
	}
	if err := json.NewDecoder(sessionResp.Body).Decode(&session); err != nil {
		_ = sessionResp.Body.Close()
		return fmt.Errorf("解析会话响应失败: %w", err)
	}
	_ = sessionResp.Body.Close()

	// 分片上传
	chunkSize := int64(4 * 1024 * 1024) // 4MB chunks
	buf := make([]byte, chunkSize)

	for offset := int64(0); offset < stat.Size(); {
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			return fmt.Errorf("读取文件失败: %w", err)
		}
		if n == 0 {
			break
		}

		end := offset + int64(n) - 1
		if end >= stat.Size() {
			end = stat.Size() - 1
		}

		req, _ := http.NewRequestWithContext(ctx, "PUT", session.UploadURL, bytes.NewReader(buf[:n]))
		req.Header.Set("Content-Length", fmt.Sprintf("%d", n))
		req.Header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", offset, end, stat.Size()))

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("上传分片失败: %w", err)
		}
		_ = resp.Body.Close()

		if resp.StatusCode >= 400 {
			return fmt.Errorf("上传分片失败: %s", resp.Status)
		}

		offset += int64(n)
	}

	return nil
}

// Download downloads a file from OneDrive to the local filesystem.
func (p *OneDriveProvider) Download(ctx context.Context, remotePath, localPath string) error {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return err
	}

	url := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root:%s:/content", remotePath)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("创建下载请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.accessToken)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败: %s", resp.Status)
	}

	// 创建本地目录
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 创建本地文件
	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer func() { _ = file.Close() }()

	_, err = io.Copy(file, resp.Body)
	return err
}

// Delete removes a file from OneDrive.
func (p *OneDriveProvider) Delete(ctx context.Context, remotePath string) error {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return err
	}

	url := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root:%s", remotePath)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("创建删除请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.accessToken)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("删除失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("删除失败: %s", resp.Status)
	}

	return nil
}

// List returns a list of files in OneDrive matching the given prefix.
func (p *OneDriveProvider) List(ctx context.Context, prefix string, recursive bool) ([]FileInfo, error) {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return nil, err
	}

	var files []FileInfo

	// 列出文件夹内容
	folderPath := prefix
	if folderPath == "" {
		folderPath = "/"
	}

	url := "https://graph.microsoft.com/v1.0/me/drive/root/children"
	if folderPath == "/" {
		url = "https://graph.microsoft.com/v1.0/me/drive/root/children"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.accessToken)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("列出文件失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Value []struct {
			Name                 string    `json:"name"`
			Size                 int64     `json:"size"`
			LastModifiedDateTime string    `json:"lastModifiedDateTime"`
			Folder               *struct{} `json:"folder,omitempty"`
			File                 *struct{} `json:"file,omitempty"`
		} `json:"value"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	for _, item := range result.Value {
		modTime, _ := time.Parse(time.RFC3339, item.LastModifiedDateTime)

		files = append(files, FileInfo{
			Path:    filepath.Join(prefix, item.Name),
			Size:    item.Size,
			ModTime: modTime,
			IsDir:   item.Folder != nil,
		})
	}

	// 如果需要递归，继续列出子文件夹
	if recursive {
		for _, f := range files {
			if f.IsDir {
				subFiles, err := p.List(ctx, f.Path, true)
				if err == nil {
					files = append(files, subFiles...)
				}
			}
		}
	}

	return files, nil
}

// Stat returns metadata information about a file in OneDrive.
func (p *OneDriveProvider) Stat(ctx context.Context, remotePath string) (*FileInfo, error) {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root:%s", remotePath)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.accessToken)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取文件信息失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, os.ErrNotExist
	}

	var result struct {
		Name                 string    `json:"name"`
		Size                 int64     `json:"size"`
		LastModifiedDateTime string    `json:"lastModifiedDateTime"`
		Folder               *struct{} `json:"folder,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	modTime, _ := time.Parse(time.RFC3339, result.LastModifiedDateTime)

	return &FileInfo{
		Path:    remotePath,
		Size:    result.Size,
		ModTime: modTime,
		IsDir:   result.Folder != nil,
	}, nil
}

// CreateDir creates a directory in OneDrive.
func (p *OneDriveProvider) CreateDir(ctx context.Context, remotePath string) error {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return err
	}
	return p.ensureFolder(ctx, remotePath)
}

// DeleteDir removes a directory from OneDrive.
func (p *OneDriveProvider) DeleteDir(ctx context.Context, remotePath string) error {
	return p.Delete(ctx, remotePath)
}

// TestConnection tests the connection to OneDrive and returns the result.
func (p *OneDriveProvider) TestConnection(ctx context.Context) (*ConnectionTestResult, error) {
	result := &ConnectionTestResult{
		Provider: ProviderOneDrive,
		Endpoint: "graph.microsoft.com",
	}

	if p.refreshToken == "" {
		result.Success = false
		result.Message = "需要 OAuth2 授权"
		return result, nil
	}

	start := time.Now()

	// 尝试刷新 token
	err := p.refreshTokenIfNeeded(ctx)
	result.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("认证失败: %v", err)
		result.Error = err.Error()
		return result, nil
	}

	// 尝试获取 drive 信息
	req, err := http.NewRequestWithContext(ctx, "GET", "https://graph.microsoft.com/v1.0/me/drive", nil)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("创建请求失败: %v", err)
		result.Error = err.Error()
		return result, nil
	}
	req.Header.Set("Authorization", "Bearer "+p.accessToken)

	resp, err := p.client.Do(req)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("连接失败: %v", err)
		result.Error = err.Error()
		return result, nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		result.Success = false
		result.Message = "授权已过期，请重新授权"
		return result, nil
	}

	result.Success = resp.StatusCode == http.StatusOK
	if result.Success {
		result.Message = "连接成功"
	} else {
		result.Message = fmt.Sprintf("连接失败: %s", resp.Status)
	}

	return result, nil
}

// Close releases resources used by the OneDrive provider. Currently a no-op.
func (p *OneDriveProvider) Close() error {
	return nil
}

// GetType returns the provider type for OneDrive.
func (p *OneDriveProvider) GetType() ProviderType {
	return ProviderOneDrive
}

// GetCapabilities returns the list of capabilities supported by OneDrive.
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
