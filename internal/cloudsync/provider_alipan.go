// Package cloudsync provides cloud storage synchronization
// This file implements Aliyun (阿里云盘) cloud drive provider
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
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ==================== 阿里云盘 Provider ====================

// ProviderAliyunPanImpl 阿里云盘实现
// 阿里云盘API参考：https://www.yuque.com/aliyundrive/zpfszx
type ProviderAliyunPanImpl struct {
	client       *http.Client
	config       *ProviderConfig
	baseURL      string
	accessToken  string
	refreshToken string
	driveID      string
	tokenExpiry  time.Time
}

// NewAliyunPanProvider 创建阿里云盘提供商
func NewAliyunPanProvider(cfg *ProviderConfig) (*ProviderAliyunPanImpl, error) {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	if cfg.Timeout > 0 {
		client.Timeout = time.Duration(cfg.Timeout) * time.Second
	}

	return &ProviderAliyunPanImpl{
		client:       client,
		config:       cfg,
		baseURL:      "https://api.aliyundrive.com/v2",
		accessToken:  cfg.AccessToken,
		refreshToken: cfg.RefreshToken,
		driveID:      cfg.DriveID,
	}, nil
}

// 阿里云盘API响应结构
type alipanAPIResponse struct {
	Code          string `json:"code"`
	Message       string `json:"message"`
	NextPageToken string `json:"next_page_token,omitempty"`
}

// refreshTokenIfNeeded 刷新访问令牌
func (p *ProviderAliyunPanImpl) refreshTokenIfNeeded(ctx context.Context) error {
	// 如果token未过期，直接返回
	if p.accessToken != "" && time.Now().Before(p.tokenExpiry.Add(-5*time.Minute)) {
		return nil
	}

	if p.refreshToken == "" {
		return fmt.Errorf("阿里云盘需要 refresh_token，请先完成授权")
	}

	tokenURL := "https://api.aliyundrive.com/token/refresh"

	data := map[string]string{
		"refresh_token": p.refreshToken,
	}

	body, _ := json.Marshal(data)
	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("创建刷新请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

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
		DriveID      string `json:"default_drive_id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("解析token响应失败: %w", err)
	}

	p.accessToken = result.AccessToken
	p.refreshToken = result.RefreshToken
	p.tokenExpiry = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)

	if p.driveID == "" && result.DriveID != "" {
		p.driveID = result.DriveID
	}

	return nil
}

// Upload 上传文件到阿里云盘
// 阿里云盘支持秒传功能，会先尝试秒传，失败后走普通上传流程
func (p *ProviderAliyunPanImpl) Upload(ctx context.Context, localPath, remotePath string) error {
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

	// 1. 创建文件（尝试秒传）
	uploadInfo, err := p.createFile(ctx, fileName, parentID, stat.Size(), hash)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}

	// 检查是否秒传成功
	if uploadInfo.Exist {
		return nil // 秒传成功
	}

	// 2. 上传文件到OSS
	if uploadInfo.UploadURL != "" {
		if err := p.uploadToOSS(ctx, uploadInfo.UploadURL, file, stat.Size()); err != nil {
			return fmt.Errorf("上传文件失败: %w", err)
		}
	}

	// 3. 完成上传
	return p.completeUpload(ctx, uploadInfo)
}

// alipanUploadInfo 上传信息
type alipanUploadInfo struct {
	FileID    string `json:"file_id"`
	UploadID  string `json:"upload_id"`
	UploadURL string `json:"upload_url"`
	Exist     bool   `json:"exist"` // 秒传成功标志
	PartInfo  []struct {
		PartNumber int    `json:"part_number"`
		UploadURL  string `json:"upload_url"`
	} `json:"part_info_list"`
}

// calculateFileHash 计算文件哈希（阿里云盘使用sha1）
func (p *ProviderAliyunPanImpl) calculateFileHash(file *os.File) (string, error) {
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

// createFile 创建文件（尝试秒传）
func (p *ProviderAliyunPanImpl) createFile(ctx context.Context, fileName, parentID string, size int64, hash string) (*alipanUploadInfo, error) {
	apiURL := fmt.Sprintf("%s/file/create", p.baseURL)

	// 分片信息
	partInfo := []map[string]interface{}{}
	if size > 0 {
		chunkSize := int64(10 * 1024 * 1024) // 10MB chunks
		partNum := (size + chunkSize - 1) / chunkSize
		for i := int64(1); i <= partNum; i++ {
			partInfo = append(partInfo, map[string]interface{}{
				"part_number": i,
			})
		}
	}

	data := map[string]interface{}{
		"drive_id":          p.driveID,
		"parent_file_id":    parentID,
		"name":              fileName,
		"type":              "file",
		"size":              size,
		"check_name_mode":   "refuse",
		"content_hash":      hash,
		"content_hash_name": "sha1",
		"part_info_list":    partInfo,
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
		alipanAPIResponse
		FileID       string `json:"file_id"`
		UploadID     string `json:"upload_id"`
		Exist        bool   `json:"exist"`
		PartInfoList []struct {
			PartNumber int    `json:"part_number"`
			UploadURL  string `json:"upload_url"`
		} `json:"part_info_list"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Code != "" && result.Code != "Success" {
		return nil, fmt.Errorf("创建文件失败: %s", result.Message)
	}

	uploadURL := ""
	if len(result.PartInfoList) > 0 {
		uploadURL = result.PartInfoList[0].UploadURL
	}

	return &alipanUploadInfo{
		FileID:    result.FileID,
		UploadID:  result.UploadID,
		UploadURL: uploadURL,
		Exist:     result.Exist,
		PartInfo:  result.PartInfoList,
	}, nil
}

// uploadToOSS 上传文件到OSS
func (p *ProviderAliyunPanImpl) uploadToOSS(ctx context.Context, uploadURL string, file *os.File, size int64) error {
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

// completeUpload 完成上传
func (p *ProviderAliyunPanImpl) completeUpload(ctx context.Context, uploadInfo *alipanUploadInfo) error {
	apiURL := fmt.Sprintf("%s/file/complete", p.baseURL)

	data := map[string]interface{}{
		"drive_id":  p.driveID,
		"file_id":   uploadInfo.FileID,
		"upload_id": uploadInfo.UploadID,
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

	var result alipanAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if result.Code != "" && result.Code != "Success" {
		return fmt.Errorf("完成上传失败: %s", result.Message)
	}

	return nil
}

// Download 从阿里云盘下载文件
func (p *ProviderAliyunPanImpl) Download(ctx context.Context, remotePath, localPath string) error {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return err
	}

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

// getDownloadURL 获取下载链接
func (p *ProviderAliyunPanImpl) getDownloadURL(ctx context.Context, fileID string) (string, error) {
	apiURL := fmt.Sprintf("%s/file/get_download_url", p.baseURL)

	data := map[string]interface{}{
		"drive_id": p.driveID,
		"file_id":  fileID,
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
		alipanAPIResponse
		URL string `json:"url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.Code != "" && result.Code != "Success" {
		return "", fmt.Errorf("获取下载链接失败: %s", result.Message)
	}

	return result.URL, nil
}

// Delete 删除阿里云盘文件
func (p *ProviderAliyunPanImpl) Delete(ctx context.Context, remotePath string) error {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return err
	}

	fileID, err := p.getFileIDByPath(ctx, remotePath)
	if err != nil {
		return fmt.Errorf("获取文件ID失败: %w", err)
	}

	apiURL := fmt.Sprintf("%s/file/delete", p.baseURL)

	data := map[string]interface{}{
		"drive_id":  p.driveID,
		"file_id":   fileID,
		"permanent": false, // 放入回收站，而不是永久删除
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

	var result alipanAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if result.Code != "" && result.Code != "Success" {
		return fmt.Errorf("删除失败: %s", result.Message)
	}

	return nil
}

// List 列出阿里云盘文件
func (p *ProviderAliyunPanImpl) List(ctx context.Context, prefix string, recursive bool) ([]FileInfo, error) {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return nil, err
	}

	dirID, err := p.getDirIDByPath(ctx, prefix)
	if err != nil {
		dirID = "root"
	}

	return p.listFiles(ctx, dirID, prefix, recursive)
}

// listFiles 列出目录下的文件
func (p *ProviderAliyunPanImpl) listFiles(ctx context.Context, dirID, prefix string, recursive bool) ([]FileInfo, error) {
	apiURL := fmt.Sprintf("%s/file/list", p.baseURL)

	data := map[string]interface{}{
		"drive_id":       p.driveID,
		"parent_file_id": dirID,
		"limit":          100,
		"fields":         "*",
	}

	body, _ := json.Marshal(data)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建列表请求失败: %w", err)
	}

	p.setAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("列出文件失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		alipanAPIResponse
		Items []struct {
			FileID      string    `json:"file_id"`
			Name        string    `json:"name"`
			Size        int64     `json:"size"`
			Type        string    `json:"type"`
			UpdatedAt   time.Time `json:"updated_at"`
			ContentHash string    `json:"content_hash"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	var files []FileInfo
	for _, item := range result.Items {
		path := filepath.Join(prefix, item.Name)
		isDir := item.Type == "folder"

		files = append(files, FileInfo{
			Path:    path,
			Size:    item.Size,
			ModTime: item.UpdatedAt,
			IsDir:   isDir,
			Hash:    item.ContentHash,
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

// Stat 获取阿里云盘文件信息
func (p *ProviderAliyunPanImpl) Stat(ctx context.Context, remotePath string) (*FileInfo, error) {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return nil, err
	}

	fileID, err := p.getFileIDByPath(ctx, remotePath)
	if err != nil {
		return nil, fmt.Errorf("获取文件ID失败: %w", err)
	}

	apiURL := fmt.Sprintf("%s/file/get", p.baseURL)

	data := map[string]interface{}{
		"drive_id": p.driveID,
		"file_id":  fileID,
		"fields":   "*",
	}

	body, _ := json.Marshal(data)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	p.setAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取文件信息失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, os.ErrNotExist
	}

	var result struct {
		alipanAPIResponse
		Name        string    `json:"name"`
		Size        int64     `json:"size"`
		Type        string    `json:"type"`
		UpdatedAt   time.Time `json:"updated_at"`
		ContentHash string    `json:"content_hash"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &FileInfo{
		Path:    remotePath,
		Size:    result.Size,
		ModTime: result.UpdatedAt,
		IsDir:   result.Type == "folder",
		Hash:    result.ContentHash,
	}, nil
}

// CreateDir 在阿里云盘创建目录
func (p *ProviderAliyunPanImpl) CreateDir(ctx context.Context, remotePath string) error {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return err
	}

	_, err := p.getOrCreateDirID(ctx, remotePath)
	return err
}

// DeleteDir 删除阿里云盘目录
func (p *ProviderAliyunPanImpl) DeleteDir(ctx context.Context, remotePath string) error {
	return p.Delete(ctx, remotePath)
}

// TestConnection 测试阿里云盘连接
func (p *ProviderAliyunPanImpl) TestConnection(ctx context.Context) (*ConnectionTestResult, error) {
	start := time.Now()

	// 尝试刷新token
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return &ConnectionTestResult{
			Provider: ProviderAliyunPan,
			Success:  false,
			Message:  fmt.Sprintf("认证失败: %v", err),
		}, nil
	}

	// 获取用户信息
	apiURL := fmt.Sprintf("%s/user/get", p.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader([]byte("{}")))
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

	latency := time.Since(start).Milliseconds()

	result := &ConnectionTestResult{
		Provider:  ProviderAliyunPan,
		Endpoint:  p.baseURL,
		LatencyMs: latency,
	}

	var apiResult alipanAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResult); err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("解析响应失败: %v", err)
		return result, nil
	}

	if resp.StatusCode == http.StatusUnauthorized || apiResult.Code == "AccessTokenInvalid" {
		result.Success = false
		result.Message = "授权已过期，请重新登录"
		return result, nil
	}

	result.Success = apiResult.Code == "" || apiResult.Code == "Success"
	if result.Success {
		result.Message = "连接成功"
	} else {
		result.Message = fmt.Sprintf("连接失败: %s", apiResult.Message)
	}

	return result, nil
}

// Close 关闭连接
func (p *ProviderAliyunPanImpl) Close() error {
	return nil
}

// GetType 返回提供商类型
func (p *ProviderAliyunPanImpl) GetType() ProviderType {
	return ProviderAliyunPan
}

// GetCapabilities 返回支持的功能
func (p *ProviderAliyunPanImpl) GetCapabilities() []string {
	return []string{"upload", "download", "delete", "list", "instant_upload", "share"}
}

// ==================== 辅助方法 ====================

// setAuthHeader 设置认证头
func (p *ProviderAliyunPanImpl) setAuthHeader(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("User-Agent", "NAS-OS/1.0")
}

// getOrCreateDirID 获取或创建目录ID
func (p *ProviderAliyunPanImpl) getOrCreateDirID(ctx context.Context, path string) (string, error) {
	if path == "" || path == "/" {
		return "root", nil
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	parentID := "root"

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

// getDirIDByPath 通过路径获取目录ID
func (p *ProviderAliyunPanImpl) getDirIDByPath(ctx context.Context, path string) (string, error) {
	return p.getOrCreateDirID(ctx, path)
}

// findDir 查找目录
func (p *ProviderAliyunPanImpl) findDir(ctx context.Context, parentID, name string) (string, error) {
	apiURL := fmt.Sprintf("%s/file/search", p.baseURL)

	data := map[string]interface{}{
		"drive_id": p.driveID,
		"query":    fmt.Sprintf("name = '%s' and parent_file_id = '%s'", name, parentID),
		"limit":    100,
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
		alipanAPIResponse
		Items []struct {
			FileID string `json:"file_id"`
			Name   string `json:"name"`
			Type   string `json:"type"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	for _, item := range result.Items {
		if item.Name == name && item.Type == "folder" {
			return item.FileID, nil
		}
	}

	return "", os.ErrNotExist
}

// createDir 创建目录
func (p *ProviderAliyunPanImpl) createDir(ctx context.Context, parentID, name string) (string, error) {
	apiURL := fmt.Sprintf("%s/file/create_folder", p.baseURL)

	data := map[string]interface{}{
		"drive_id":        p.driveID,
		"parent_file_id":  parentID,
		"name":            name,
		"check_name_mode": "refuse",
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
		alipanAPIResponse
		FileID string `json:"file_id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.Code != "" && result.Code != "Success" {
		return "", fmt.Errorf("创建目录失败: %s", result.Message)
	}

	return result.FileID, nil
}

// getFileIDByPath 通过路径获取文件ID
func (p *ProviderAliyunPanImpl) getFileIDByPath(ctx context.Context, path string) (string, error) {
	dir := filepath.Dir(path)
	fileName := filepath.Base(path)

	dirID, err := p.getDirIDByPath(ctx, dir)
	if err != nil {
		return "", err
	}

	apiURL := fmt.Sprintf("%s/file/search", p.baseURL)

	data := map[string]interface{}{
		"drive_id": p.driveID,
		"query":    fmt.Sprintf("name = '%s' and parent_file_id = '%s'", fileName, dirID),
		"limit":    100,
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
		alipanAPIResponse
		Items []struct {
			FileID string `json:"file_id"`
			Name   string `json:"name"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	for _, item := range result.Items {
		if item.Name == fileName {
			return item.FileID, nil
		}
	}

	return "", os.ErrNotExist
}
