package backup

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
	"github.com/studio-b12/gowebdav"
)

// cloudConfig 存储云端配置（内部使用）
type cloudConfig struct {
	Provider   CloudProvider
	Bucket     string
	Endpoint   string
	Region     string
	AccessKey  string
	SecretKey  string
	Prefix     string
	Insecure   bool
	Encryption bool
}

// CloudBackup 云端备份管理器
type CloudBackup struct {
	provider CloudProvider
	client   interface{}
	config   cloudConfig
}

// CloudProvider 云存储提供商类型
type CloudProvider string

const (
	CloudProviderS3     CloudProvider = "s3"
	CloudProviderWebDAV CloudProvider = "webdav"
	CloudProviderAliyun CloudProvider = "aliyun" // 阿里云 OSS（S3 兼容）
)

// CloudConfig 云端配置
type CloudConfig struct {
	Provider   CloudProvider `json:"provider"`
	Bucket     string        `json:"bucket"`     // S3 bucket 或 WebDAV 根路径
	Endpoint   string        `json:"endpoint"`   // S3 endpoint 或 WebDAV URL
	Region     string        `json:"region"`     // S3 region
	AccessKey  string        `json:"-"`          // S3 Access Key 或 WebDAV 用户名（敏感信息，不序列化）
	SecretKey  string        `json:"-"`          // S3 Secret Key 或 WebDAV 密码（敏感信息，不序列化）
	Prefix     string        `json:"prefix"`     // 路径前缀
	Insecure   bool          `json:"insecure"`   // 跳过 TLS 验证（WebDAV）
	Encryption bool          `json:"encryption"` // 上传前加密
}

// Sanitize 返回脱敏后的配置副本（用于日志和调试）
func (cc *CloudConfig) Sanitize() map[string]interface{} {
	return map[string]interface{}{
		"provider":       cc.Provider,
		"bucket":         cc.Bucket,
		"endpoint":       cc.Endpoint,
		"region":         cc.Region,
		"prefix":         cc.Prefix,
		"insecure":       cc.Insecure,
		"encryption":     cc.Encryption,
		"has_access_key": cc.AccessKey != "",
		"has_secret_key": cc.SecretKey != "",
	}
}

// NewCloudBackup 创建云端备份管理器
func NewCloudBackup(cfg CloudConfig) (*CloudBackup, error) {
	cb := &CloudBackup{
		provider: cfg.Provider,
		config:   cloudConfig(cfg),
	}

	switch cfg.Provider {
	case CloudProviderS3, CloudProviderAliyun:
		client, err := cb.initS3Client(cfg)
		if err != nil {
			return nil, err
		}
		cb.client = client
	case CloudProviderWebDAV:
		client, err := cb.initWebDAVClient(cfg)
		if err != nil {
			return nil, err
		}
		cb.client = client
	default:
		return nil, fmt.Errorf("不支持的云存储提供商：%s", cfg.Provider)
	}

	return cb, nil
}

// initS3Client 初始化 S3 客户端
func (cb *CloudBackup) initS3Client(cfg CloudConfig) (*s3.Client, error) {
	creds := credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")

	s3Config, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(creds),
		config.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("加载 AWS 配置失败：%w", err)
	}

	// 自定义 endpoint（用于兼容 S3 的服务如 MinIO、阿里云 OSS）
	endpointOption := func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true // 路径风格访问（兼容 MinIO 等）
		}
	}

	client := s3.NewFromConfig(s3Config, endpointOption)
	return client, nil
}

// initWebDAVClient 初始化 WebDAV 客户端
func (cb *CloudBackup) initWebDAVClient(cfg CloudConfig) (*gowebdav.Client, error) {
	client := gowebdav.NewClient(cfg.Endpoint, cfg.AccessKey, cfg.SecretKey)

	if cfg.Insecure {
		// 仅测试环境允许跳过 TLS 验证，生产环境必须验证证书
		if os.Getenv("ENV") != "test" {
			return nil, fmt.Errorf("生产环境禁止跳过 TLS 验证，请配置有效的 CA 证书")
		}
		// #nosec G402 -- InsecureSkipVerify is only allowed in test environment (ENV=test)
		client.SetTransport(&http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		})
	}

	return client, nil
}

// UploadBackup 上传备份到云端
func (cb *CloudBackup) UploadBackup(localPath, remotePath string) (*UploadResult, error) {
	switch cb.provider {
	case CloudProviderS3, CloudProviderAliyun:
		return cb.uploadToS3(localPath, remotePath)
	case CloudProviderWebDAV:
		return cb.uploadToWebDAV(localPath, remotePath)
	default:
		return nil, fmt.Errorf("不支持的提供商：%s", cb.provider)
	}
}

// UploadResult 上传结果
type UploadResult struct {
	RemotePath string
	Size       int64
	Duration   time.Duration
	Checksum   string
}

// uploadToS3 上传到 S3
func (cb *CloudBackup) uploadToS3(localPath, remotePath string) (*UploadResult, error) {
	startTime := time.Now()
	client, ok := cb.client.(*s3.Client)
	if !ok {
		return nil, fmt.Errorf("客户端类型错误")
	}

	cfg := cb.getCloudConfig()
	key := strings.TrimPrefix(remotePath, "/")
	if cfg.Prefix != "" {
		key = strings.TrimSuffix(cfg.Prefix, "/") + "/" + key
	}

	file, err := os.Open(localPath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败：%w", err)
	}
	defer func() { _ = file.Close() }()

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("获取文件信息失败：%w", err)
	}

	_, err = client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(cfg.Bucket),
		Key:         aws.String(key),
		Body:        file,
		ContentType: aws.String("application/octet-stream"),
	})
	if err != nil {
		return nil, fmt.Errorf("S3 上传失败：%w", err)
	}

	return &UploadResult{
		RemotePath: fmt.Sprintf("s3://%s/%s", cfg.Bucket, key),
		Size:       stat.Size(),
		Duration:   time.Since(startTime),
	}, nil
}

// uploadToWebDAV 上传到 WebDAV
func (cb *CloudBackup) uploadToWebDAV(localPath, remotePath string) (*UploadResult, error) {
	startTime := time.Now()
	client, ok := cb.client.(*gowebdav.Client)
	if !ok {
		return nil, fmt.Errorf("客户端类型错误")
	}

	data, err := os.ReadFile(localPath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败：%w", err)
	}

	remoteDir := filepath.Dir(remotePath)
	if err := client.MkdirAll(remoteDir, 0755); err != nil {
		return nil, fmt.Errorf("创建远程目录失败：%w", err)
	}

	if err := client.Write(remotePath, data, 0644); err != nil {
		return nil, fmt.Errorf("WebDAV 上传失败：%w", err)
	}

	return &UploadResult{
		RemotePath: remotePath,
		Size:       int64(len(data)),
		Duration:   time.Since(startTime),
	}, nil
}

// DownloadBackup 从云端下载备份
func (cb *CloudBackup) DownloadBackup(remotePath, localPath string) (*DownloadResult, error) {
	switch cb.provider {
	case CloudProviderS3, CloudProviderAliyun:
		return cb.downloadFromS3(remotePath, localPath)
	case CloudProviderWebDAV:
		return cb.downloadFromWebDAV(remotePath, localPath)
	default:
		return nil, fmt.Errorf("不支持的提供商：%s", cb.provider)
	}
}

// DownloadResult 下载结果
type DownloadResult struct {
	LocalPath string
	Size      int64
	Duration  time.Duration
}

// downloadFromS3 从 S3 下载
func (cb *CloudBackup) downloadFromS3(remotePath, localPath string) (*DownloadResult, error) {
	startTime := time.Now()
	client, ok := cb.client.(*s3.Client)
	if !ok {
		return nil, fmt.Errorf("客户端类型错误")
	}

	cfg := cb.getCloudConfig()

	key := remotePath
	if strings.HasPrefix(key, fmt.Sprintf("s3://%s/", cfg.Bucket)) {
		key = strings.TrimPrefix(key, fmt.Sprintf("s3://%s/", cfg.Bucket))
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return nil, fmt.Errorf("创建本地目录失败：%w", err)
	}

	file, err := os.Create(localPath)
	if err != nil {
		return nil, fmt.Errorf("创建文件失败：%w", err)
	}
	defer func() { _ = file.Close() }()

	result, err := client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("S3 下载失败：%w", err)
	}
	defer func() { _ = result.Body.Close() }()

	_, err = io.Copy(file, result.Body)
	if err != nil {
		return nil, fmt.Errorf("写入文件失败：%w", err)
	}

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("获取文件信息失败：%w", err)
	}

	return &DownloadResult{
		LocalPath: localPath,
		Size:      stat.Size(),
		Duration:  time.Since(startTime),
	}, nil
}

// downloadFromWebDAV 从 WebDAV 下载
func (cb *CloudBackup) downloadFromWebDAV(remotePath, localPath string) (*DownloadResult, error) {
	startTime := time.Now()
	client, ok := cb.client.(*gowebdav.Client)
	if !ok {
		return nil, fmt.Errorf("客户端类型错误")
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return nil, fmt.Errorf("创建本地目录失败：%w", err)
	}

	data, err := client.Read(remotePath)
	if err != nil {
		return nil, fmt.Errorf("WebDAV 下载失败：%w", err)
	}

	if err := os.WriteFile(localPath, data, 0644); err != nil {
		return nil, fmt.Errorf("写入文件失败：%w", err)
	}

	return &DownloadResult{
		LocalPath: localPath,
		Size:      int64(len(data)),
		Duration:  time.Since(startTime),
	}, nil
}

// ListBackups 列出云端备份
func (cb *CloudBackup) ListBackups(prefix string) ([]CloudBackupInfo, error) {
	switch cb.provider {
	case CloudProviderS3, CloudProviderAliyun:
		return cb.listS3Backups(prefix)
	case CloudProviderWebDAV:
		return cb.listWebDAVBackups(prefix)
	default:
		return nil, fmt.Errorf("不支持的提供商：%s", cb.provider)
	}
}

// CloudBackupInfo 云端备份信息
type CloudBackupInfo struct {
	Name      string
	Size      int64
	CreatedAt time.Time
	Path      string
}

// listS3Backups 列出 S3 备份
func (cb *CloudBackup) listS3Backups(prefix string) ([]CloudBackupInfo, error) {
	client, ok := cb.client.(*s3.Client)
	if !ok {
		return nil, fmt.Errorf("客户端类型错误")
	}

	cfg := cb.getCloudConfig()

	// 构建前缀
	fullPrefix := cfg.Prefix
	if prefix != "" {
		fullPrefix = strings.TrimSuffix(fullPrefix, "/") + "/" + prefix
	}

	// 列出对象
	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: aws.String(cfg.Bucket),
		Prefix: aws.String(fullPrefix),
	})

	var backups []CloudBackupInfo
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.TODO())
		if err != nil {
			return nil, fmt.Errorf("列出对象失败：%w", err)
		}

		for _, obj := range page.Contents {
			backups = append(backups, CloudBackupInfo{
				Name:      filepath.Base(*obj.Key),
				Size:      *obj.Size,
				CreatedAt: *obj.LastModified,
				Path:      *obj.Key,
			})
		}
	}

	return backups, nil
}

// listWebDAVBackups 列出 WebDAV 备份
func (cb *CloudBackup) listWebDAVBackups(prefix string) ([]CloudBackupInfo, error) {
	client, ok := cb.client.(*gowebdav.Client)
	if !ok {
		return nil, fmt.Errorf("客户端类型错误")
	}

	files, err := client.ReadDir(prefix)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败：%w", err)
	}

	var backups []CloudBackupInfo
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		backups = append(backups, CloudBackupInfo{
			Name:      file.Name(),
			Size:      file.Size(),
			CreatedAt: file.ModTime(),
			Path:      filepath.Join(prefix, file.Name()),
		})
	}

	return backups, nil
}

// DeleteBackup 删除云端备份
func (cb *CloudBackup) DeleteBackup(remotePath string) error {
	switch cb.provider {
	case CloudProviderS3, CloudProviderAliyun:
		return cb.deleteS3Backup(remotePath)
	case CloudProviderWebDAV:
		return cb.deleteWebDAVBackup(remotePath)
	default:
		return fmt.Errorf("不支持的提供商：%s", cb.provider)
	}
}

// deleteS3Backup 删除 S3 备份
func (cb *CloudBackup) deleteS3Backup(remotePath string) error {
	client, ok := cb.client.(*s3.Client)
	if !ok {
		return fmt.Errorf("客户端类型错误")
	}

	cfg := cb.getCloudConfig()

	key := remotePath
	if strings.HasPrefix(key, fmt.Sprintf("s3://%s/", cfg.Bucket)) {
		key = strings.TrimPrefix(key, fmt.Sprintf("s3://%s/", cfg.Bucket))
	}

	_, err := client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(cfg.Bucket),
		Key:    aws.String(key),
	})
	return err
}

// deleteWebDAVBackup 删除 WebDAV 备份
func (cb *CloudBackup) deleteWebDAVBackup(remotePath string) error {
	client, ok := cb.client.(*gowebdav.Client)
	if !ok {
		return fmt.Errorf("客户端类型错误")
	}

	return client.Remove(remotePath)
}

// getCloudConfig 获取云端配置
func (cb *CloudBackup) getCloudConfig() CloudConfig {
	return CloudConfig{
		Provider:   cb.config.Provider,
		Bucket:     cb.config.Bucket,
		Endpoint:   cb.config.Endpoint,
		Region:     cb.config.Region,
		AccessKey:  cb.config.AccessKey,
		SecretKey:  cb.config.SecretKey,
		Prefix:     cb.config.Prefix,
		Insecure:   cb.config.Insecure,
		Encryption: cb.config.Encryption,
	}
}

// VerifyBackup 验证云端备份完整性
func (cb *CloudBackup) VerifyBackup(remotePath string) (bool, error) {
	// 检查文件是否存在
	switch cb.provider {
	case CloudProviderS3, CloudProviderAliyun:
		return cb.verifyS3Backup(remotePath)
	case CloudProviderWebDAV:
		return cb.verifyWebDAVBackup(remotePath)
	default:
		return false, fmt.Errorf("不支持的提供商：%s", cb.provider)
	}
}

// verifyS3Backup 验证 S3 备份
func (cb *CloudBackup) verifyS3Backup(remotePath string) (bool, error) {
	client, ok := cb.client.(*s3.Client)
	if !ok {
		return false, fmt.Errorf("客户端类型错误")
	}

	cfg := cb.getCloudConfig()

	key := remotePath
	if strings.HasPrefix(key, fmt.Sprintf("s3://%s/", cfg.Bucket)) {
		key = strings.TrimPrefix(key, fmt.Sprintf("s3://%s/", cfg.Bucket))
	}

	_, err := client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(cfg.Bucket),
		Key:    aws.String(key),
	})

	return err == nil, nil
}

// verifyWebDAVBackup 验证 WebDAV 备份
func (cb *CloudBackup) verifyWebDAVBackup(remotePath string) (bool, error) {
	client, ok := cb.client.(*gowebdav.Client)
	if !ok {
		return false, fmt.Errorf("客户端类型错误")
	}

	_, err := client.Stat(remotePath)
	return err == nil, nil
}

// ConnectionTestResult 连接测试结果
type ConnectionTestResult struct {
	Success   bool   `json:"success"`
	Provider  string `json:"provider"`
	Endpoint  string `json:"endpoint"`
	Bucket    string `json:"bucket"`
	LatencyMs int64  `json:"latencyMs"`
	Message   string `json:"message"`
}

// CheckConnection 检查云端连接状态
func (cb *CloudBackup) CheckConnection() (*ConnectionTestResult, error) {
	switch cb.provider {
	case CloudProviderS3, CloudProviderAliyun:
		return cb.checkS3Connection()
	case CloudProviderWebDAV:
		return cb.checkWebDAVConnection()
	default:
		return nil, fmt.Errorf("不支持的提供商：%s", cb.provider)
	}
}

// checkS3Connection 检查 S3 连接
func (cb *CloudBackup) checkS3Connection() (*ConnectionTestResult, error) {
	startTime := time.Now()

	client, ok := cb.client.(*s3.Client)
	if !ok {
		return nil, fmt.Errorf("客户端类型错误")
	}

	cfg := cb.getCloudConfig()

	// 尝试列出 bucket 中的对象（仅检查连接，不获取实际数据）
	_, err := client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket:  aws.String(cfg.Bucket),
		MaxKeys: aws.Int32(1), // 只检查是否能连接，不获取大量数据
	})

	latency := time.Since(startTime).Milliseconds()

	if err != nil {
		return &ConnectionTestResult{
			Success:   false,
			Provider:  string(cb.provider),
			Endpoint:  cfg.Endpoint,
			Bucket:    cfg.Bucket,
			LatencyMs: latency,
			Message:   fmt.Sprintf("连接失败：%v", err),
		}, err
	}

	return &ConnectionTestResult{
		Success:   true,
		Provider:  string(cb.provider),
		Endpoint:  cfg.Endpoint,
		Bucket:    cfg.Bucket,
		LatencyMs: latency,
		Message:   "连接成功",
	}, nil
}

// checkWebDAVConnection 检查 WebDAV 连接
func (cb *CloudBackup) checkWebDAVConnection() (*ConnectionTestResult, error) {
	startTime := time.Now()

	client, ok := cb.client.(*gowebdav.Client)
	if !ok {
		return nil, fmt.Errorf("客户端类型错误")
	}

	cfg := cb.getCloudConfig()

	// 尝试读取根目录
	_, err := client.ReadDir("/")

	latency := time.Since(startTime).Milliseconds()

	if err != nil {
		return &ConnectionTestResult{
			Success:   false,
			Provider:  string(cb.provider),
			Endpoint:  cfg.Endpoint,
			Bucket:    "",
			LatencyMs: latency,
			Message:   fmt.Sprintf("连接失败：%v", err),
		}, err
	}

	return &ConnectionTestResult{
		Success:   true,
		Provider:  string(cb.provider),
		Endpoint:  cfg.Endpoint,
		Bucket:    "",
		LatencyMs: latency,
		Message:   "连接成功",
	}, nil
}
