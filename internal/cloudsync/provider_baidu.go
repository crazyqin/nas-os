// Package cloudsync provides cloud storage synchronization
// This file implements Baidu (百度网盘) cloud drive provider
package cloudsync

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ==================== 百度网盘 Provider ====================

// ProviderBaiduPanImpl 百度网盘实现
// 百度网盘API参考：https://pan.baidu.com/union/doc/
type ProviderBaiduPanImpl struct {
	client       *http.Client
	config       *ProviderConfig
	baseURL      string
	accessToken  string
	refreshToken string
	tokenExpiry  time.Time
}

// NewBaiduPanProvider 创建百度网盘提供商
func NewBaiduPanProvider(cfg *ProviderConfig) (*ProviderBaiduPanImpl, error) {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	if cfg.Timeout > 0 {
		client.Timeout = time.Duration(cfg.Timeout) * time.Second
	}

	return &ProviderBaiduPanImpl{
		client:       client,
		config:       cfg,
		baseURL:      "https://pan.baidu.com/rest/2.0/xpan",
		accessToken:  cfg.AccessToken,
		refreshToken: cfg.RefreshToken,
	}, nil
}

// baiduAPIResponse 已移除 - 使用内联结构体

// refreshTokenIfNeeded 刷新访问令牌
func (p *ProviderBaiduPanImpl) refreshTokenIfNeeded(ctx context.Context) error {
	// 如果token未过期，直接返回
	if p.accessToken != "" && time.Now().Before(p.tokenExpiry.Add(-5*time.Minute)) {
		return nil
	}

	if p.refreshToken == "" {
		return fmt.Errorf("百度网盘需要 refresh_token，请先完成授权")
	}

	// 百度网盘刷新token需要 client_id 和 client_secret
	// 这里使用配置中的值
	tokenURL := "https://openapi.baidu.com/oauth/2.0/token"

	params := url.Values{}
	params.Set("grant_type", "refresh_token")
	params.Set("refresh_token", p.refreshToken)
	// 注意：实际使用时需要配置 client_id 和 client_secret
	// params.Set("client_id", p.config.ClientID)
	// params.Set("client_secret", p.config.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, "GET", tokenURL+"?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("创建刷新请求失败: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("刷新token失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("刷新token失败: %s", resp.Status)
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Error        string `json:"error,omitempty"`
		ErrorDesc    string `json:"error_description,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("解析token响应失败: %w", err)
	}

	if result.Error != "" {
		return fmt.Errorf("刷新token失败: %s - %s", result.Error, result.ErrorDesc)
	}

	p.accessToken = result.AccessToken
	p.refreshToken = result.RefreshToken
	p.tokenExpiry = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)

	return nil
}

// Upload 上传文件到百度网盘
// 百度网盘支持秒传功能，会先尝试秒传，失败后走普通上传流程
func (p *ProviderBaiduPanImpl) Upload(ctx context.Context, localPath, remotePath string) error {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return err
	}

	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer func() { _ = file.Close() }()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	// 计算文件MD5（用于秒传）
	md5Hash, err := p.calculateFileMD5(file)
	if err != nil {
		return fmt.Errorf("计算文件MD5失败: %w", err)
	}

	// 百度网盘要求远程路径以 / 开头
	if !strings.HasPrefix(remotePath, "/") {
		remotePath = "/" + remotePath
	}

	// 1. 预上传
	preUploadInfo, err := p.preUpload(ctx, remotePath, stat.Size(), md5Hash)
	if err != nil {
		return fmt.Errorf("预上传失败: %w", err)
	}

	// 检查是否秒传成功
	if preUploadInfo.UploadID == "" && preUploadInfo.ReturnType == 1 {
		return nil // 秒传成功
	}

	// 2. 分片上传
	if err := p.uploadParts(ctx, preUploadInfo, file, stat.Size()); err != nil {
		return fmt.Errorf("上传分片失败: %w", err)
	}

	// 3. 合并文件
	return p.mergeFile(ctx, remotePath, preUploadInfo.UploadID, stat.Size(), md5Hash)
}

// baiduPreUploadInfo 预上传信息
type baiduPreUploadInfo struct {
	UploadID   string `json:"uploadid"`
	ReturnType int    `json:"return_type"` // 1=秒传成功, 2=需要上传
	BlockList  []int  `json:"block_list"`  // 需要上传的分片索引
	Errno      int    `json:"errno"`
}

// preUpload 预上传
func (p *ProviderBaiduPanImpl) preUpload(ctx context.Context, remotePath string, fileSize int64, md5Hash string) (*baiduPreUploadInfo, error) {
	// 百度网盘分片大小为 4MB
	blockSize := int64(4 * 1024 * 1024)
	blockCount := int((fileSize + blockSize - 1) / blockSize)

	// 生成分片MD5列表
	blockList := make([]string, blockCount)
	// 简化处理：实际需要计算每个分片的MD5
	for i := 0; i < blockCount; i++ {
		blockList[i] = md5Hash // 使用整个文件的MD5作为近似
	}

	apiURL := fmt.Sprintf("%s/file?method=precreate&access_token=%s", p.baseURL, p.accessToken)

	body := map[string]interface{}{
		"path":       remotePath,
		"size":       fileSize,
		"isdir":      0,
		"autoinit":   1,
		"block_list": blockList,
	}

	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result baiduPreUploadInfo
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Errno != 0 {
		return nil, fmt.Errorf("预上传失败，错误码: %d", result.Errno)
	}

	return &result, nil
}

// uploadParts 上传分片
func (p *ProviderBaiduPanImpl) uploadParts(ctx context.Context, info *baiduPreUploadInfo, file *os.File, fileSize int64) error {
	blockSize := int64(4 * 1024 * 1024)

	// 如果 block_list 为空，需要上传所有分片
	blockList := info.BlockList
	if len(blockList) == 0 && info.ReturnType == 2 {
		blockCount := int((fileSize + blockSize - 1) / blockSize)
		blockList = make([]int, blockCount)
		for i := 0; i < blockCount; i++ {
			blockList[i] = i
		}
	}

	for _, blockIdx := range blockList {
		offset := int64(blockIdx) * blockSize
		size := blockSize
		if offset+size > fileSize {
			size = fileSize - offset
		}

		// 读取分片数据
		if _, err := file.Seek(offset, 0); err != nil {
			return fmt.Errorf("定位文件失败: %w", err)
		}

		partData := make([]byte, size)
		if _, err := io.ReadFull(file, partData); err != nil {
			return fmt.Errorf("读取分片失败: %w", err)
		}

		// 上传分片
		if err := p.uploadSinglePart(ctx, info.UploadID, blockIdx, partData); err != nil {
			return fmt.Errorf("上传分片 %d 失败: %w", blockIdx, err)
		}
	}

	return nil
}

// uploadSinglePart 上传单个分片
func (p *ProviderBaiduPanImpl) uploadSinglePart(ctx context.Context, uploadID string, partIndex int, data []byte) error {
	apiURL := fmt.Sprintf("https://d.pcs.baidu.com/rest/2.0/pcs/superfile2?method=upload&access_token=%s&type=tmpfile&uploadid=%s&partseq=%d",
		p.accessToken, uploadID, partIndex)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("上传分片失败: %s", resp.Status)
	}

	var result struct {
		MD5   string `json:"md5"`
		Errno int    `json:"errno"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if result.Errno != 0 {
		return fmt.Errorf("上传分片失败，错误码: %d", result.Errno)
	}

	return nil
}

// mergeFile 合并文件
func (p *ProviderBaiduPanImpl) mergeFile(ctx context.Context, remotePath, uploadID string, fileSize int64, md5Hash string) error {
	apiURL := fmt.Sprintf("%s/file?method=create&access_token=%s", p.baseURL, p.accessToken)

	body := map[string]interface{}{
		"path":       remotePath,
		"size":       fileSize,
		"isdir":      0,
		"uploadid":   uploadID,
		"block_list": []string{md5Hash},
	}

	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Fsid  int64 `json:"fs_id"`
		Errno int   `json:"errno"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if result.Errno != 0 {
		return fmt.Errorf("合并文件失败，错误码: %d", result.Errno)
	}

	return nil
}

// Download 从百度网盘下载文件
func (p *ProviderBaiduPanImpl) Download(ctx context.Context, remotePath, localPath string) error {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return err
	}

	// 百度网盘要求远程路径以 / 开头
	if !strings.HasPrefix(remotePath, "/") {
		remotePath = "/" + remotePath
	}
	_ = remotePath // TODO: 实现通过remotePath获取文件ID

	// 获取下载链接
	apiURL := fmt.Sprintf("%s/multimedia?method=filemetas&access_token=%s", p.baseURL, p.accessToken)

	body := map[string]interface{}{
		"fsids": []int64{}, // 需要先获取文件ID
		"dlink": 1,
	}

	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		List []struct {
			FsID     int64  `json:"fs_id"`
			Dlink    string `json:"dlink"`
			Filename string `json:"filename"`
		} `json:"list"`
		Errno int `json:"errno"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if result.Errno != 0 {
		return fmt.Errorf("获取下载链接失败，错误码: %d", result.Errno)
	}

	if len(result.List) == 0 || result.List[0].Dlink == "" {
		return fmt.Errorf("未找到文件或获取下载链接失败")
	}

	// 使用下载链接下载文件
	dlink := result.List[0].Dlink + "&access_token=" + p.accessToken

	// 创建本地目录
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 下载文件
	downloadResp, err := p.client.Get(dlink)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer func() { _ = downloadResp.Body.Close() }()

	if downloadResp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败: %s", downloadResp.Status)
	}

	// 创建本地文件
	outFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("创建本地文件失败: %w", err)
	}
	defer func() { _ = outFile.Close() }()

	if _, err := io.Copy(outFile, downloadResp.Body); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}

// Delete 删除百度网盘文件
func (p *ProviderBaiduPanImpl) Delete(ctx context.Context, remotePath string) error {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return err
	}

	if !strings.HasPrefix(remotePath, "/") {
		remotePath = "/" + remotePath
	}

	apiURL := fmt.Sprintf("%s/file?method=filemanager&access_token=%s&opera=delete", p.baseURL, p.accessToken)

	body := map[string]interface{}{
		"filelist": []string{remotePath},
	}

	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Errno int `json:"errno"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if result.Errno != 0 {
		return fmt.Errorf("删除失败，错误码: %d", result.Errno)
	}

	return nil
}

// List 列出百度网盘目录
func (p *ProviderBaiduPanImpl) List(ctx context.Context, prefix string, recursive bool) ([]FileInfo, error) {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return nil, err
	}

	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}

	apiURL := fmt.Sprintf("%s/file?method=list&access_token=%s&dir=%s", p.baseURL, p.accessToken, url.QueryEscape(prefix))

	if recursive {
		apiURL += "&recursion=1"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		List []struct {
			Path     string `json:"path"`
			Filename string `json:"filename"`
			Size     int64  `json:"size"`
			Isdir    int    `json:"isdir"`
			Mtime    int64  `json:"server_mtime"`
		} `json:"list"`
		Errno int `json:"errno"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Errno != 0 {
		return nil, fmt.Errorf("列出目录失败，错误码: %d", result.Errno)
	}

	files := make([]FileInfo, 0, len(result.List))
	for _, item := range result.List {
		files = append(files, FileInfo{
			Path:    item.Path,
			Size:    item.Size,
			ModTime: time.Unix(item.Mtime, 0),
			IsDir:   item.Isdir == 1,
		})
	}

	return files, nil
}

// Stat 获取文件信息
func (p *ProviderBaiduPanImpl) Stat(ctx context.Context, remotePath string) (*FileInfo, error) {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return nil, err
	}

	if !strings.HasPrefix(remotePath, "/") {
		remotePath = "/" + remotePath
	}

	apiURL := fmt.Sprintf("%s/file?method=filemetas&access_token=%s&path=%s", p.baseURL, p.accessToken, url.QueryEscape(remotePath))

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		List []struct {
			Path     string `json:"path"`
			Filename string `json:"filename"`
			Size     int64  `json:"size"`
			Isdir    int    `json:"isdir"`
			Mtime    int64  `json:"server_mtime"`
			MD5      string `json:"md5"`
		} `json:"list"`
		Errno int `json:"errno"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Errno != 0 {
		return nil, fmt.Errorf("获取文件信息失败，错误码: %d", result.Errno)
	}

	if len(result.List) == 0 {
		return nil, fmt.Errorf("文件不存在: %s", remotePath)
	}

	item := result.List[0]
	return &FileInfo{
		Path:    item.Path,
		Size:    item.Size,
		ModTime: time.Unix(item.Mtime, 0),
		IsDir:   item.Isdir == 1,
		Hash:    item.MD5,
	}, nil
}

// CreateDir 创建目录
func (p *ProviderBaiduPanImpl) CreateDir(ctx context.Context, remotePath string) error {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return err
	}

	if !strings.HasPrefix(remotePath, "/") {
		remotePath = "/" + remotePath
	}

	apiURL := fmt.Sprintf("%s/file?method=create&access_token=%s", p.baseURL, p.accessToken)

	body := map[string]interface{}{
		"path":  remotePath,
		"isdir": 1,
	}

	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Errno int `json:"errno"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if result.Errno != 0 && result.Errno != -8 { // -8 表示目录已存在
		return fmt.Errorf("创建目录失败，错误码: %d", result.Errno)
	}

	return nil
}

// DeleteDir 删除目录
func (p *ProviderBaiduPanImpl) DeleteDir(ctx context.Context, remotePath string) error {
	return p.Delete(ctx, remotePath)
}

// TestConnection 测试连接
func (p *ProviderBaiduPanImpl) TestConnection(ctx context.Context) (*ConnectionTestResult, error) {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return &ConnectionTestResult{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	start := time.Now()

	// 获取用户信息来验证连接
	apiURL := fmt.Sprintf("https://pan.baidu.com/rest/2.0/xpan/nas?method=uinfo&access_token=%s", p.accessToken)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return &ConnectionTestResult{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return &ConnectionTestResult{
			Success: false,
			Error:   err.Error(),
		}, err
	}
	defer func() { _ = resp.Body.Close() }()

	latency := time.Since(start).Milliseconds()

	var result struct {
		Errno   int    `json:"errno"`
		Errmsg  string `json:"errmsg,omitempty"`
		BaiduID string `json:"baidu_name,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return &ConnectionTestResult{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	if result.Errno != 0 {
		return &ConnectionTestResult{
			Success: false,
			Error:   fmt.Sprintf("错误码: %d, 消息: %s", result.Errno, result.Errmsg),
		}, fmt.Errorf("连接失败: %s", result.Errmsg)
	}

	return &ConnectionTestResult{
		Success:   true,
		Provider:  ProviderBaiduPan,
		Endpoint:  "pan.baidu.com",
		LatencyMs: latency,
		Message:   fmt.Sprintf("连接成功，用户: %s", result.BaiduID),
	}, nil
}

// Close 关闭连接
func (p *ProviderBaiduPanImpl) Close() error {
	return nil
}

// GetType 获取类型
func (p *ProviderBaiduPanImpl) GetType() ProviderType {
	return ProviderBaiduPan
}

// GetCapabilities 获取功能
func (p *ProviderBaiduPanImpl) GetCapabilities() []string {
	return []string{"upload", "download", "delete", "list", "instant_upload", "share"}
}

// calculateFileMD5 计算文件MD5
func (p *ProviderBaiduPanImpl) calculateFileMD5(file *os.File) (string, error) {
	if _, err := file.Seek(0, 0); err != nil {
		return "", err
	}

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	if _, err := file.Seek(0, 0); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// getOrCreateDir 已移除 - 未使用
