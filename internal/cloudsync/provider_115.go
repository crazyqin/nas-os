// Package cloudsync provides cloud storage synchronization
// This file implements 115 cloud drive provider
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

// ==================== 115网盘 Provider ====================

// Provider115Impl 115网盘实现
// 115网盘API参考：https://github.com/erdone/web-115-doc
type Provider115Impl struct {
	client      *http.Client
	config      *ProviderConfig
	baseURL     string
	accessToken string
	userID      string
	rootDirID   string
}

// New115Provider 创建115网盘提供商.
func New115Provider(cfg *ProviderConfig) (*Provider115Impl, error) {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	if cfg.Timeout > 0 {
		client.Timeout = time.Duration(cfg.Timeout) * time.Second
	}

	return &Provider115Impl{
		client:      client,
		config:      cfg,
		baseURL:     "https://webapi.115.com",
		accessToken: cfg.AccessToken,
		userID:      cfg.UserID,
		rootDirID:   "0", // 115网盘根目录ID为0
	}, nil
}

// 115网盘API响应结构.
type api115Response struct {
	State    bool   `json:"state"`
	Error    string `json:"error"`
	Errno    int    `json:"errno"`
	ErrnoMsg string `json:"errno_msg,omitempty"`
}

// Upload 上传文件到115网盘
// 115网盘支持秒传功能，会先尝试秒传，失败后走普通上传流程.
func (p *Provider115Impl) Upload(ctx context.Context, localPath, remotePath string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer func() { _ = file.Close() }()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	// 计算文件哈希（用于秒传）
	hash, err := p.calculateFileHash(file)
	if err != nil {
		return fmt.Errorf("计算文件哈希失败: %w", err)
	}

	// 获取父目录ID
	parentID, err := p.getOrCreateDirID(ctx, filepath.Dir(remotePath))
	if err != nil {
		return fmt.Errorf("获取目录ID失败: %w", err)
	}

	fileName := filepath.Base(remotePath)

	// 1. 尝试秒传
	if err := p.tryInstantUpload(ctx, fileName, parentID, hash, stat.Size()); err == nil {
		return nil // 秒传成功
	}

	// 2. 获取上传地址
	uploadInfo, err := p.getUploadInfo(ctx, fileName, parentID, stat.Size(), hash)
	if err != nil {
		return fmt.Errorf("获取上传信息失败: %w", err)
	}

	// 3. 上传文件到OSS
	if err := p.uploadToOSS(ctx, uploadInfo.UploadURL, file, stat.Size()); err != nil {
		return fmt.Errorf("上传文件失败: %w", err)
	}

	// 4. 确认上传
	return p.confirmUpload(ctx, uploadInfo.UploadID, fileName, parentID, stat.Size(), hash)
}

// calculateFileHash 计算文件哈希（115网盘使用的哈希算法）.
func (p *Provider115Impl) calculateFileHash(file *os.File) (string, error) {
	// 重置文件指针
	if _, err := file.Seek(0, 0); err != nil {
		return "", err
	}

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	// 重置文件指针供后续使用
	if _, err := file.Seek(0, 0); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// tryInstantUpload 尝试秒传.
func (p *Provider115Impl) tryInstantUpload(ctx context.Context, fileName, parentID, hash string, size int64) error {
	apiURL := fmt.Sprintf("%s/files/upload_instant", p.baseURL)

	data := map[string]interface{}{
		"name":      fileName,
		"parent_id": parentID,
		"size":      size,
		"sha1":      hash,
	}

	body, _ := json.Marshal(data)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return err
	}

	p.setAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	var result api115Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if result.State && result.Errno == 0 {
		return nil // 秒传成功
	}

	return fmt.Errorf("秒传失败: %s", result.ErrnoMsg)
}

// uploadInfo115 上传信息.
type uploadInfo115 struct {
	UploadID  string `json:"upload_id"`
	UploadURL string `json:"upload_url"`
	ObjectID  string `json:"object_id"`
}

// getUploadInfo 获取上传信息.
func (p *Provider115Impl) getUploadInfo(ctx context.Context, fileName, parentID string, size int64, hash string) (*uploadInfo115, error) {
	apiURL := fmt.Sprintf("%s/files/upload_init", p.baseURL)

	data := map[string]interface{}{
		"name":      fileName,
		"parent_id": parentID,
		"size":      size,
		"sha1":      hash,
	}

	body, _ := json.Marshal(data)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	p.setAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		api115Response
		Data uploadInfo115 `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if !result.State {
		return nil, fmt.Errorf("获取上传信息失败: %s", result.ErrnoMsg)
	}

	return &result.Data, nil
}

// uploadToOSS 上传文件到OSS.
func (p *Provider115Impl) uploadToOSS(ctx context.Context, uploadURL string, file *os.File, size int64) error {
	req, err := http.NewRequestWithContext(ctx, "PUT", uploadURL, file)
	if err != nil {
		return err
	}

	req.ContentLength = size

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("上传失败: %s - %s", resp.Status, string(body))
	}

	return nil
}

// confirmUpload 确认上传.
func (p *Provider115Impl) confirmUpload(ctx context.Context, uploadID, fileName, parentID string, size int64, hash string) error {
	apiURL := fmt.Sprintf("%s/files/upload_complete", p.baseURL)

	data := map[string]interface{}{
		"upload_id": uploadID,
		"name":      fileName,
		"parent_id": parentID,
		"size":      size,
		"sha1":      hash,
	}

	body, _ := json.Marshal(data)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return err
	}

	p.setAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	var result api115Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if !result.State {
		return fmt.Errorf("确认上传失败: %s", result.ErrnoMsg)
	}

	return nil
}

// Download 从115网盘下载文件.
func (p *Provider115Impl) Download(ctx context.Context, remotePath, localPath string) error {
	// 获取文件ID
	fileID, err := p.getFileIDByPath(ctx, remotePath)
	if err != nil {
		return fmt.Errorf("获取文件ID失败: %w", err)
	}

	// 获取下载链接
	downloadURL, err := p.getDownloadURL(ctx, fileID)
	if err != nil {
		return fmt.Errorf("获取下载链接失败: %w", err)
	}

	// 创建本地目录
	if err := os.MkdirAll(filepath.Dir(localPath), 0750); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 创建本地文件
	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer func() { _ = file.Close() }()

	// 下载文件
	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return fmt.Errorf("创建下载请求失败: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败: %s", resp.Status)
	}

	_, err = io.Copy(file, resp.Body)
	return err
}

// getDownloadURL 获取下载链接.
func (p *Provider115Impl) getDownloadURL(ctx context.Context, fileID string) (string, error) {
	apiURL := fmt.Sprintf("%s/files/download?file_id=%s", p.baseURL, fileID)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", err
	}

	p.setAuthHeader(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		api115Response
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if !result.State {
		return "", fmt.Errorf("获取下载链接失败: %s", result.ErrnoMsg)
	}

	return result.Data.URL, nil
}

// Delete 删除115网盘文件.
func (p *Provider115Impl) Delete(ctx context.Context, remotePath string) error {
	fileID, err := p.getFileIDByPath(ctx, remotePath)
	if err != nil {
		return fmt.Errorf("获取文件ID失败: %w", err)
	}

	apiURL := fmt.Sprintf("%s/files/delete", p.baseURL)

	data := map[string]interface{}{
		"file_ids": []string{fileID},
	}

	body, _ := json.Marshal(data)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("创建删除请求失败: %w", err)
	}

	p.setAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("删除失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result api115Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if !result.State {
		return fmt.Errorf("删除失败: %s", result.ErrnoMsg)
	}

	return nil
}

// List 列出115网盘文件.
func (p *Provider115Impl) List(ctx context.Context, prefix string, recursive bool) ([]FileInfo, error) {
	dirID, err := p.getDirIDByPath(ctx, prefix)
	if err != nil {
		dirID = p.rootDirID
	}

	return p.listFiles(ctx, dirID, prefix, recursive)
}

// listFiles 列出目录下的文件.
func (p *Provider115Impl) listFiles(ctx context.Context, dirID, prefix string, recursive bool) ([]FileInfo, error) {
	apiURL := fmt.Sprintf("%s/files/list?parent_id=%s&limit=1000", p.baseURL, dirID)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建列表请求失败: %w", err)
	}

	p.setAuthHeader(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("列出文件失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		api115Response
		Data struct {
			Files []struct {
				FileID   string `json:"file_id"`
				Name     string `json:"name"`
				Size     int64  `json:"size"`
				IsDir    bool   `json:"is_dir"`
				Modified string `json:"modified"`
			} `json:"files"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	var files []FileInfo
	for _, f := range result.Data.Files {
		modTime, _ := time.Parse(time.RFC3339, f.Modified)
		path := filepath.Join(prefix, f.Name)
		files = append(files, FileInfo{
			Path:    path,
			Size:    f.Size,
			ModTime: modTime,
			IsDir:   f.IsDir,
		})

		// 递归列出子目录
		if recursive && f.IsDir {
			subFiles, err := p.listFiles(ctx, f.FileID, path, true)
			if err == nil {
				files = append(files, subFiles...)
			}
		}
	}

	return files, nil
}

// Stat 获取115网盘文件信息.
func (p *Provider115Impl) Stat(ctx context.Context, remotePath string) (*FileInfo, error) {
	fileID, err := p.getFileIDByPath(ctx, remotePath)
	if err != nil {
		return nil, fmt.Errorf("获取文件ID失败: %w", err)
	}

	apiURL := fmt.Sprintf("%s/files/stat?file_id=%s", p.baseURL, fileID)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	p.setAuthHeader(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取文件信息失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, os.ErrNotExist
	}

	var result struct {
		api115Response
		Data struct {
			Name     string `json:"name"`
			Size     int64  `json:"size"`
			IsDir    bool   `json:"is_dir"`
			Modified string `json:"modified"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	modTime, _ := time.Parse(time.RFC3339, result.Data.Modified)

	return &FileInfo{
		Path:    remotePath,
		Size:    result.Data.Size,
		ModTime: modTime,
		IsDir:   result.Data.IsDir,
	}, nil
}

// CreateDir 在115网盘创建目录.
func (p *Provider115Impl) CreateDir(ctx context.Context, remotePath string) error {
	_, err := p.getOrCreateDirID(ctx, remotePath)
	return err
}

// DeleteDir 删除115网盘目录.
func (p *Provider115Impl) DeleteDir(ctx context.Context, remotePath string) error {
	return p.Delete(ctx, remotePath)
}

// TestConnection 测试115网盘连接.
func (p *Provider115Impl) TestConnection(ctx context.Context) (*ConnectionTestResult, error) {
	start := time.Now()

	apiURL := fmt.Sprintf("%s/user/info", p.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	p.setAuthHeader(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	latency := time.Since(start).Milliseconds()

	result := &ConnectionTestResult{
		Provider:  Provider115,
		Endpoint:  p.baseURL,
		LatencyMs: latency,
	}

	var apiResult api115Response
	if err := json.NewDecoder(resp.Body).Decode(&apiResult); err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("解析响应失败: %v", err)
		return result, nil
	}

	if resp.StatusCode == http.StatusUnauthorized || !apiResult.State {
		result.Success = false
		result.Message = "授权已过期，请重新登录"
		return result, nil
	}

	result.Success = true
	result.Message = "连接成功"

	return result, nil
}

// Close 关闭连接.
func (p *Provider115Impl) Close() error {
	return nil
}

// GetType 返回提供商类型.
func (p *Provider115Impl) GetType() ProviderType {
	return Provider115
}

// GetCapabilities 返回支持的功能.
func (p *Provider115Impl) GetCapabilities() []string {
	return []string{"upload", "download", "delete", "list", "instant_upload", "offline_download"}
}

// ==================== 辅助方法 ====================

// setAuthHeader 设置认证头.
func (p *Provider115Impl) setAuthHeader(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("User-Agent", "NAS-OS/1.0")
}

// getOrCreateDirID 获取或创建目录ID.
func (p *Provider115Impl) getOrCreateDirID(ctx context.Context, path string) (string, error) {
	if path == "" || path == "/" {
		return p.rootDirID, nil
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	parentID := p.rootDirID

	for _, part := range parts {
		if part == "" {
			continue
		}

		// 查找子目录
		dirID, err := p.findDir(ctx, parentID, part)
		if err != nil {
			// 创建目录
			dirID, err = p.createDir(ctx, parentID, part)
			if err != nil {
				return "", fmt.Errorf("创建目录失败: %w", err)
			}
		}
		parentID = dirID
	}

	return parentID, nil
}

// getDirIDByPath 通过路径获取目录ID.
func (p *Provider115Impl) getDirIDByPath(ctx context.Context, path string) (string, error) {
	return p.getOrCreateDirID(ctx, path)
}

// findDir 查找目录.
func (p *Provider115Impl) findDir(ctx context.Context, parentID, name string) (string, error) {
	apiURL := fmt.Sprintf("%s/files/list?parent_id=%s&name=%s", p.baseURL, parentID, url.QueryEscape(name))

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", err
	}

	p.setAuthHeader(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		api115Response
		Data struct {
			Files []struct {
				FileID string `json:"file_id"`
				Name   string `json:"name"`
				IsDir  bool   `json:"is_dir"`
			} `json:"files"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	for _, f := range result.Data.Files {
		if f.Name == name && f.IsDir {
			return f.FileID, nil
		}
	}

	return "", os.ErrNotExist
}

// createDir 创建目录.
func (p *Provider115Impl) createDir(ctx context.Context, parentID, name string) (string, error) {
	apiURL := fmt.Sprintf("%s/files/mkdir", p.baseURL)

	data := map[string]interface{}{
		"parent_id": parentID,
		"name":      name,
	}

	body, _ := json.Marshal(data)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	p.setAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		api115Response
		Data struct {
			FileID string `json:"file_id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if !result.State {
		return "", fmt.Errorf("创建目录失败: %s", result.ErrnoMsg)
	}

	return result.Data.FileID, nil
}

// getFileIDByPath 通过路径获取文件ID.
func (p *Provider115Impl) getFileIDByPath(ctx context.Context, path string) (string, error) {
	dir := filepath.Dir(path)
	fileName := filepath.Base(path)

	dirID, err := p.getDirIDByPath(ctx, dir)
	if err != nil {
		return "", err
	}

	apiURL := fmt.Sprintf("%s/files/list?parent_id=%s&name=%s", p.baseURL, dirID, url.QueryEscape(fileName))

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", err
	}

	p.setAuthHeader(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		api115Response
		Data struct {
			Files []struct {
				FileID string `json:"file_id"`
				Name   string `json:"name"`
			} `json:"files"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	for _, f := range result.Data.Files {
		if f.Name == fileName {
			return f.FileID, nil
		}
	}

	return "", os.ErrNotExist
}
