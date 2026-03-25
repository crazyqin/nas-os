// Package cloudsync provides cloud storage synchronization
// This file implements Quark cloud drive provider
package cloudsync

import (
	"bytes"
	"context"
	"crypto/sha1"
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

// ==================== 夸克网盘 Provider ====================

// ProviderQuarkImpl 夸克网盘实现
// 夸克网盘API参考：https://github.com/alist-org/alist
type ProviderQuarkImpl struct {
	client      *http.Client
	config      *ProviderConfig
	baseURL     string
	accessToken string
	rootDirID   string
}

// NewQuarkProvider 创建夸克网盘提供商.
func NewQuarkProvider(cfg *ProviderConfig) (*ProviderQuarkImpl, error) {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	if cfg.Timeout > 0 {
		client.Timeout = time.Duration(cfg.Timeout) * time.Second
	}

	return &ProviderQuarkImpl{
		client:      client,
		config:      cfg,
		baseURL:     "https://pan.quark.cn/api",
		accessToken: cfg.AccessToken,
		rootDirID:   "0", // 夸克网盘根目录ID为0
	}, nil
}

// 夸克网盘API响应结构.
type quarkAPIResponse struct {
	Status   int    `json:"status"`
	Code     int    `json:"code"`
	Message  string `json:"message"`
	Errno    int    `json:"errno"`
	ErrnoMsg string `json:"errno_msg,omitempty"`
}

// Upload 上传文件到夸克网盘.
func (p *ProviderQuarkImpl) Upload(ctx context.Context, localPath, remotePath string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer func() { _ = file.Close() }()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	// 计算文件哈希
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

	// 1. 预上传
	uploadInfo, err := p.preUpload(ctx, fileName, parentID, stat.Size(), hash)
	if err != nil {
		return fmt.Errorf("预上传失败: %w", err)
	}

	// 检查是否秒传成功
	if uploadInfo.Finish {
		return nil
	}

	// 2. 上传分片
	if err := p.uploadParts(ctx, uploadInfo, file, stat.Size()); err != nil {
		return fmt.Errorf("上传分片失败: %w", err)
	}

	// 3. 完成上传
	return p.completeUpload(ctx, uploadInfo, fileName, parentID)
}

// quarkUploadInfo 上传信息.
type quarkUploadInfo struct {
	UploadID  string `json:"upload_id"`
	UploadURL string `json:"upload_url"`
	Finish    bool   `json:"finish"` // 秒传成功标志
}

// calculateFileHash 计算文件哈希.
func (p *ProviderQuarkImpl) calculateFileHash(file *os.File) (string, error) {
	if _, err := file.Seek(0, 0); err != nil {
		return "", err
	}

	hash := sha1.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	if _, err := file.Seek(0, 0); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// preUpload 预上传.
func (p *ProviderQuarkImpl) preUpload(ctx context.Context, fileName, parentID string, size int64, hash string) (*quarkUploadInfo, error) {
	apiURL := fmt.Sprintf("%s/file/upload/pre", p.baseURL)

	data := map[string]interface{}{
		"parent_file_id": parentID,
		"file_name":      fileName,
		"file_size":      size,
		"content_hash":   hash,
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
		quarkAPIResponse
		Data quarkUploadInfo `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("预上传失败: %s", result.Message)
	}

	return &result.Data, nil
}

// uploadParts 上传分片.
func (p *ProviderQuarkImpl) uploadParts(ctx context.Context, uploadInfo *quarkUploadInfo, file *os.File, size int64) error {
	chunkSize := int64(4 * 1024 * 1024) // 4MB chunks
	buf := make([]byte, chunkSize)

	for offset := int64(0); offset < size; {
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			return fmt.Errorf("读取文件失败: %w", err)
		}
		if n == 0 {
			break
		}

		// 上传分片
		uploadURL := fmt.Sprintf("%s/file/upload?upload_id=%s&part_number=%d",
			p.baseURL, uploadInfo.UploadID, offset/chunkSize+1)

		req, err := http.NewRequestWithContext(ctx, "PUT", uploadURL, bytes.NewReader(buf[:n]))
		if err != nil {
			return fmt.Errorf("创建上传请求失败: %w", err)
		}

		p.setAuthHeader(req)
		req.ContentLength = int64(n)

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

// completeUpload 完成上传.
func (p *ProviderQuarkImpl) completeUpload(ctx context.Context, uploadInfo *quarkUploadInfo, fileName, parentID string) error {
	apiURL := fmt.Sprintf("%s/file/upload/complete", p.baseURL)

	data := map[string]interface{}{
		"upload_id":      uploadInfo.UploadID,
		"parent_file_id": parentID,
		"file_name":      fileName,
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

	var result quarkAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if result.Code != 0 {
		return fmt.Errorf("完成上传失败: %s", result.Message)
	}

	return nil
}

// Download 从夸克网盘下载文件.
func (p *ProviderQuarkImpl) Download(ctx context.Context, remotePath, localPath string) error {
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
func (p *ProviderQuarkImpl) getDownloadURL(ctx context.Context, fileID string) (string, error) {
	apiURL := fmt.Sprintf("%s/file/download?file_id=%s", p.baseURL, fileID)

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
		quarkAPIResponse
		Data struct {
			DownloadURL string `json:"download_url"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.Code != 0 {
		return "", fmt.Errorf("获取下载链接失败: %s", result.Message)
	}

	return result.Data.DownloadURL, nil
}

// Delete 删除夸克网盘文件.
func (p *ProviderQuarkImpl) Delete(ctx context.Context, remotePath string) error {
	fileID, err := p.getFileIDByPath(ctx, remotePath)
	if err != nil {
		return fmt.Errorf("获取文件ID失败: %w", err)
	}

	apiURL := fmt.Sprintf("%s/file/delete", p.baseURL)

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

	var result quarkAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if result.Code != 0 {
		return fmt.Errorf("删除失败: %s", result.Message)
	}

	return nil
}

// List 列出夸克网盘文件.
func (p *ProviderQuarkImpl) List(ctx context.Context, prefix string, recursive bool) ([]FileInfo, error) {
	dirID, err := p.getDirIDByPath(ctx, prefix)
	if err != nil {
		dirID = p.rootDirID
	}

	return p.listFiles(ctx, dirID, prefix, recursive)
}

// listFiles 列出目录下的文件.
func (p *ProviderQuarkImpl) listFiles(ctx context.Context, dirID, prefix string, recursive bool) ([]FileInfo, error) {
	apiURL := fmt.Sprintf("%s/file/list?parent_file_id=%s&limit=1000", p.baseURL, dirID)

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
		quarkAPIResponse
		Data struct {
			Items []struct {
				FileID    string    `json:"file_id"`
				Name      string    `json:"name"`
				Size      int64     `json:"size"`
				Type      string    `json:"type"`
				UpdatedAt time.Time `json:"updated_at"`
			} `json:"items"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	var files []FileInfo
	for _, item := range result.Data.Items {
		path := filepath.Join(prefix, item.Name)
		isDir := item.Type == "folder"

		files = append(files, FileInfo{
			Path:    path,
			Size:    item.Size,
			ModTime: item.UpdatedAt,
			IsDir:   isDir,
		})

		// 递归列出子目录
		if recursive && isDir {
			subFiles, err := p.listFiles(ctx, item.FileID, path, true)
			if err == nil {
				files = append(files, subFiles...)
			}
		}
	}

	return files, nil
}

// Stat 获取夸克网盘文件信息.
func (p *ProviderQuarkImpl) Stat(ctx context.Context, remotePath string) (*FileInfo, error) {
	fileID, err := p.getFileIDByPath(ctx, remotePath)
	if err != nil {
		return nil, fmt.Errorf("获取文件ID失败: %w", err)
	}

	apiURL := fmt.Sprintf("%s/file/stat?file_id=%s", p.baseURL, fileID)

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
		quarkAPIResponse
		Data struct {
			Name      string    `json:"name"`
			Size      int64     `json:"size"`
			Type      string    `json:"type"`
			UpdatedAt time.Time `json:"updated_at"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &FileInfo{
		Path:    remotePath,
		Size:    result.Data.Size,
		ModTime: result.Data.UpdatedAt,
		IsDir:   result.Data.Type == "folder",
	}, nil
}

// CreateDir 在夸克网盘创建目录.
func (p *ProviderQuarkImpl) CreateDir(ctx context.Context, remotePath string) error {
	_, err := p.getOrCreateDirID(ctx, remotePath)
	return err
}

// DeleteDir 删除夸克网盘目录.
func (p *ProviderQuarkImpl) DeleteDir(ctx context.Context, remotePath string) error {
	return p.Delete(ctx, remotePath)
}

// TestConnection 测试夸克网盘连接.
func (p *ProviderQuarkImpl) TestConnection(ctx context.Context) (*ConnectionTestResult, error) {
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
		Provider:  ProviderQuark,
		Endpoint:  p.baseURL,
		LatencyMs: latency,
	}

	var apiResult quarkAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResult); err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("解析响应失败: %v", err)
		return result, nil
	}

	if resp.StatusCode == http.StatusUnauthorized || apiResult.Code == 401 {
		result.Success = false
		result.Message = "授权已过期，请重新登录"
		return result, nil
	}

	result.Success = apiResult.Code == 0
	if result.Success {
		result.Message = "连接成功"
	} else {
		result.Message = fmt.Sprintf("连接失败: %s", apiResult.Message)
	}

	return result, nil
}

// Close 关闭连接.
func (p *ProviderQuarkImpl) Close() error {
	return nil
}

// GetType 返回提供商类型.
func (p *ProviderQuarkImpl) GetType() ProviderType {
	return ProviderQuark
}

// GetCapabilities 返回支持的功能.
func (p *ProviderQuarkImpl) GetCapabilities() []string {
	return []string{"upload", "download", "delete", "list"}
}

// ==================== 辅助方法 ====================

// setAuthHeader 设置认证头.
func (p *ProviderQuarkImpl) setAuthHeader(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("User-Agent", "NAS-OS/1.0")
}

// getOrCreateDirID 获取或创建目录ID.
func (p *ProviderQuarkImpl) getOrCreateDirID(ctx context.Context, path string) (string, error) {
	if path == "" || path == "/" {
		return p.rootDirID, nil
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	parentID := p.rootDirID

	for _, part := range parts {
		if part == "" {
			continue
		}

		dirID, err := p.findDir(ctx, parentID, part)
		if err != nil {
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
func (p *ProviderQuarkImpl) getDirIDByPath(ctx context.Context, path string) (string, error) {
	return p.getOrCreateDirID(ctx, path)
}

// findDir 查找目录.
func (p *ProviderQuarkImpl) findDir(ctx context.Context, parentID, name string) (string, error) {
	apiURL := fmt.Sprintf("%s/file/list?parent_file_id=%s&name=%s", p.baseURL, parentID, url.QueryEscape(name))

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
		quarkAPIResponse
		Data struct {
			Items []struct {
				FileID string `json:"file_id"`
				Name   string `json:"name"`
				Type   string `json:"type"`
			} `json:"items"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	for _, item := range result.Data.Items {
		if item.Name == name && item.Type == "folder" {
			return item.FileID, nil
		}
	}

	return "", os.ErrNotExist
}

// createDir 创建目录.
func (p *ProviderQuarkImpl) createDir(ctx context.Context, parentID, name string) (string, error) {
	apiURL := fmt.Sprintf("%s/file/create_folder", p.baseURL)

	data := map[string]interface{}{
		"parent_file_id": parentID,
		"name":           name,
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
		quarkAPIResponse
		Data struct {
			FileID string `json:"file_id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.Code != 0 {
		return "", fmt.Errorf("创建目录失败: %s", result.Message)
	}

	return result.Data.FileID, nil
}

// getFileIDByPath 通过路径获取文件ID.
func (p *ProviderQuarkImpl) getFileIDByPath(ctx context.Context, path string) (string, error) {
	dir := filepath.Dir(path)
	fileName := filepath.Base(path)

	dirID, err := p.getDirIDByPath(ctx, dir)
	if err != nil {
		return "", err
	}

	apiURL := fmt.Sprintf("%s/file/list?parent_file_id=%s&name=%s", p.baseURL, dirID, url.QueryEscape(fileName))

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
		quarkAPIResponse
		Data struct {
			Items []struct {
				FileID string `json:"file_id"`
				Name   string `json:"name"`
			} `json:"items"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	for _, item := range result.Data.Items {
		if item.Name == fileName {
			return item.FileID, nil
		}
	}

	return "", os.ErrNotExist
}
