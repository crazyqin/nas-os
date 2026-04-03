// Package cloudsync provides cloud storage synchronization
// This file implements Dropbox cloud drive provider
package cloudsync

import (
	"bytes"
	"context"
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

// ==================== Dropbox Provider ====================

// DropboxProvider Dropbox 云存储提供商
// Dropbox API v2 参考: https://www.dropbox.com/developers/documentation/http/documentation
type DropboxProvider struct {
	client       *http.Client
	config       *ProviderConfig
	apiURL       string
	contentURL   string
	accessToken  string
	refreshToken string
	tokenExpiry  time.Time
}

// NewDropboxProvider 创建 Dropbox 提供商.
func NewDropboxProvider(cfg *ProviderConfig) (*DropboxProvider, error) {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	if cfg.Timeout > 0 {
		client.Timeout = time.Duration(cfg.Timeout) * time.Second
	}

	return &DropboxProvider{
		client:       client,
		config:       cfg,
		apiURL:       "https://api.dropboxapi.com/2",
		contentURL:   "https://content.dropboxapi.com/2",
		accessToken:  cfg.AccessToken,
		refreshToken: cfg.RefreshToken,
	}, nil
}

// refreshTokenIfNeeded 刷新访问令牌.
func (p *DropboxProvider) refreshTokenIfNeeded(ctx context.Context) error {
	// 如果token未过期，直接返回
	if p.accessToken != "" && time.Now().Before(p.tokenExpiry.Add(-5*time.Minute)) {
		return nil
	}

	if p.refreshToken == "" {
		return fmt.Errorf("Dropbox 需要 refresh_token，请先完成 OAuth2 授权")
	}

	// Dropbox OAuth2 token 刷新端点
	tokenURL := "https://api.dropboxapi.com/oauth2/token"

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {p.refreshToken},
	}

	// 如果有 client_id 和 client_secret，使用它们
	if p.config.ClientID != "" && p.config.ClientSecret != "" {
		data.Set("client_id", p.config.ClientID)
		data.Set("client_secret", p.config.ClientSecret)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("创建刷新请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("刷新token失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("刷新token失败: %s - %s", resp.Status, string(body))
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("解析token响应失败: %w", err)
	}

	p.accessToken = result.AccessToken
	if result.RefreshToken != "" {
		p.refreshToken = result.RefreshToken
	}
	p.tokenExpiry = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)

	return nil
}

// Upload 上传文件到 Dropbox
// 支持大文件分片上传和断点续传.
func (p *DropboxProvider) Upload(ctx context.Context, localPath, remotePath string) error {
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

	// Dropbox 路径格式
	dropboxPath := "/" + strings.TrimPrefix(remotePath, "/")

	// 小文件 (< 4MB) 使用简单上传
	if stat.Size() < 4*1024*1024 {
		return p.uploadSmallFile(ctx, file, dropboxPath, stat.Size())
	}

	// 大文件使用分片上传（支持断点续传）
	return p.uploadLargeFile(ctx, file, dropboxPath, stat.Size())
}

// uploadSmallFile 上传小文件.
func (p *DropboxProvider) uploadSmallFile(ctx context.Context, file *os.File, path string, size int64) error {
	apiURL := p.contentURL + "/files/upload"

	// 上传参数
	params := map[string]interface{}{
		"path":       path,
		"mode":       "overwrite", // 覆盖现有文件
		"autorename": false,
		"mute":       false,
	}

	paramsJSON, _ := json.Marshal(params)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, file)
	if err != nil {
		return fmt.Errorf("创建上传请求失败: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Dropbox-API-Arg", string(paramsJSON))
	req.ContentLength = size

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

// uploadLargeFile 上传大文件（分片上传，支持断点续传）.
func (p *DropboxProvider) uploadLargeFile(ctx context.Context, file *os.File, path string, size int64) error {
	// 1. 创建上传会话
	sessionID, err := p.createUploadSession(ctx, file, size)
	if err != nil {
		return fmt.Errorf("创建上传会话失败: %w", err)
	}

	// 2. 分片上传
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

		if err := p.uploadSessionAppend(ctx, sessionID, offset, buf[:n]); err != nil {
			return fmt.Errorf("上传分片失败 (offset %d): %w", offset, err)
		}

		offset += int64(n)
	}

	// 3. 完成上传
	return p.uploadSessionFinish(ctx, sessionID, path, size)
}

// dropboxUploadSession 上传会话信息.
type dropboxUploadSession struct {
	SessionID string `json:"session_id"`
}

// createUploadSession 创建上传会话.
func (p *DropboxProvider) createUploadSession(ctx context.Context, file *os.File, size int64) (string, error) {
	apiURL := p.contentURL + "/files/upload_session/start"

	// 读取第一个分片
	chunkSize := int64(4 * 1024 * 1024)
	buf := make([]byte, chunkSize)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("读取文件失败: %w", err)
	}

	// 如果文件小于 chunkSize，关闭会话
	closeSession := size <= chunkSize

	params := map[string]interface{}{
		"close": closeSession,
	}

	paramsJSON, _ := json.Marshal(params)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(buf[:n]))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Dropbox-API-Arg", string(paramsJSON))

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("创建会话失败: %s - %s", resp.Status, string(body))
	}

	var result dropboxUploadSession
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	return result.SessionID, nil
}

// uploadSessionAppend 添加分片到上传会话.
func (p *DropboxProvider) uploadSessionAppend(ctx context.Context, sessionID string, offset int64, data []byte) error {
	apiURL := p.contentURL + "/files/upload_session/append_v2"

	params := map[string]interface{}{
		"cursor": map[string]interface{}{
			"session_id": sessionID,
			"offset":     offset,
		},
		"close": false,
	}

	paramsJSON, _ := json.Marshal(params)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Dropbox-API-Arg", string(paramsJSON))

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("添加分片失败: %s - %s", resp.Status, string(body))
	}

	return nil
}

// uploadSessionFinish 完成上传会话.
func (p *DropboxProvider) uploadSessionFinish(ctx context.Context, sessionID, path string, size int64) error {
	apiURL := p.contentURL + "/files/upload_session/finish"

	params := map[string]interface{}{
		"cursor": map[string]interface{}{
			"session_id": sessionID,
			"offset":     size,
		},
		"commit": map[string]interface{}{
			"path":       path,
			"mode":       "overwrite",
			"autorename": false,
			"mute":       false,
		},
	}

	paramsJSON, _ := json.Marshal(params)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader([]byte{}))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Dropbox-API-Arg", string(paramsJSON))

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("完成上传失败: %s - %s", resp.Status, string(body))
	}

	return nil
}

// Download 从 Dropbox 下载文件.
func (p *DropboxProvider) Download(ctx context.Context, remotePath, localPath string) error {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return err
	}

	apiURL := p.contentURL + "/files/download"

	dropboxPath := "/" + strings.TrimPrefix(remotePath, "/")

	params := map[string]interface{}{
		"path": dropboxPath,
	}

	paramsJSON, _ := json.Marshal(params)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, nil)
	if err != nil {
		return fmt.Errorf("创建下载请求失败: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("Dropbox-API-Arg", string(paramsJSON))

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("文件不存在")
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("下载失败: %s - %s", resp.Status, string(body))
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

	_, err = io.Copy(file, resp.Body)
	return err
}

// Delete 删除 Dropbox 文件.
func (p *DropboxProvider) Delete(ctx context.Context, remotePath string) error {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return err
	}

	apiURL := p.apiURL + "/files/delete_v2"

	dropboxPath := "/" + strings.TrimPrefix(remotePath, "/")

	data := map[string]interface{}{
		"path": dropboxPath,
	}

	body, _ := json.Marshal(data)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("创建删除请求失败: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("删除失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("删除失败: %s - %s", resp.Status, string(body))
	}

	return nil
}

// List 列出 Dropbox 文件.
func (p *DropboxProvider) List(ctx context.Context, prefix string, recursive bool) ([]FileInfo, error) {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return nil, err
	}

	dropboxPath := "/" + strings.TrimPrefix(prefix, "/")

	return p.listFilesRecursive(ctx, dropboxPath, prefix, recursive)
}

// listFilesRecursive 递归列出文件.
func (p *DropboxProvider) listFilesRecursive(ctx context.Context, dropboxPath, prefix string, recursive bool) ([]FileInfo, error) {
	apiURL := p.apiURL + "/files/list_folder"

	data := map[string]interface{}{
		"path":                    dropboxPath,
		"recursive":               recursive,
		"include_deleted":         false,
		"include_has_explicit_shared_members": false,
		"include_mounted_folders": true,
		"limit":                   2000,
	}

	body, _ := json.Marshal(data)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建列表请求失败: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("列出文件失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return []FileInfo{}, nil // 目录不存在，返回空列表
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("列出文件失败: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Entries []struct {
			Tag         string `json:".tag"`
			Name        string `json:"name"`
			PathLower   string `json:"path_lower"`
			Size        int64  `json:"size"`
			ClientModified string `json:"client_modified"`
			ServerModified string `json:"server_modified"`
			ContentHash string `json:"content_hash,omitempty"`
		} `json:"entries"`
		HasMore bool   `json:"has_more"`
		Cursor  string `json:"cursor"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	var files []FileInfo
	for _, entry := range result.Entries {
		isDir := entry.Tag == "folder"
		modTime, _ := time.Parse(time.RFC3339, entry.ServerModified)

		// 构建相对路径
		relPath := strings.TrimPrefix(entry.PathLower, strings.TrimPrefix(dropboxPath, "/"))
		if relPath == "" {
			relPath = entry.Name
		} else {
			relPath = strings.TrimPrefix(relPath, "/")
		}

		files = append(files, FileInfo{
			Path:    filepath.Join(prefix, relPath),
			Size:    entry.Size,
			ModTime: modTime,
			IsDir:   isDir,
			Hash:    entry.ContentHash,
		})
	}

	// 处理分页
	if result.HasMore {
		moreFiles, err := p.listFolderContinue(ctx, result.Cursor, prefix)
		if err == nil {
			files = append(files, moreFiles...)
		}
	}

	return files, nil
}

// listFolderContinue 继续列出文件（分页）.
func (p *DropboxProvider) listFolderContinue(ctx context.Context, cursor, prefix string) ([]FileInfo, error) {
	apiURL := p.apiURL + "/files/list_folder/continue"

	data := map[string]interface{}{
		"cursor": cursor,
	}

	body, _ := json.Marshal(data)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Entries []struct {
			Tag         string `json:".tag"`
			Name        string `json:"name"`
			PathLower   string `json:"path_lower"`
			Size        int64  `json:"size"`
			ServerModified string `json:"server_modified"`
			ContentHash string `json:"content_hash,omitempty"`
		} `json:"entries"`
		HasMore bool   `json:"has_more"`
		Cursor  string `json:"cursor"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var files []FileInfo
	for _, entry := range result.Entries {
		isDir := entry.Tag == "folder"
		modTime, _ := time.Parse(time.RFC3339, entry.ServerModified)

		files = append(files, FileInfo{
			Path:    filepath.Join(prefix, entry.Name),
			Size:    entry.Size,
			ModTime: modTime,
			IsDir:   isDir,
			Hash:    entry.ContentHash,
		})
	}

	if result.HasMore {
		moreFiles, err := p.listFolderContinue(ctx, result.Cursor, prefix)
		if err == nil {
			files = append(files, moreFiles...)
		}
	}

	return files, nil
}

// Stat 获取 Dropbox 文件信息.
func (p *DropboxProvider) Stat(ctx context.Context, remotePath string) (*FileInfo, error) {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return nil, err
	}

	apiURL := p.apiURL + "/files/get_metadata"

	dropboxPath := "/" + strings.TrimPrefix(remotePath, "/")

	data := map[string]interface{}{
		"path": dropboxPath,
		"include_deleted": false,
	}

	body, _ := json.Marshal(data)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取文件信息失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, os.ErrNotExist
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("获取文件信息失败: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Tag         string `json:".tag"`
		Name        string `json:"name"`
		Size        int64  `json:"size"`
		ServerModified string `json:"server_modified"`
		ContentHash string `json:"content_hash,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	modTime, _ := time.Parse(time.RFC3339, result.ServerModified)

	return &FileInfo{
		Path:    remotePath,
		Size:    result.Size,
		ModTime: modTime,
		IsDir:   result.Tag == "folder",
		Hash:    result.ContentHash,
	}, nil
}

// CreateDir 在 Dropbox 创建目录.
func (p *DropboxProvider) CreateDir(ctx context.Context, remotePath string) error {
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return err
	}

	apiURL := p.apiURL + "/files/create_folder_v2"

	dropboxPath := "/" + strings.TrimPrefix(remotePath, "/")

	data := map[string]interface{}{
		"path": dropboxPath,
		"autorename": false,
	}

	body, _ := json.Marshal(data)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("创建目录请求失败: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// 目录已存在不算错误
	if resp.StatusCode == http.StatusConflict {
		return nil
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("创建目录失败: %s - %s", resp.Status, string(body))
	}

	return nil
}

// DeleteDir 删除 Dropbox 目录.
func (p *DropboxProvider) DeleteDir(ctx context.Context, remotePath string) error {
	return p.Delete(ctx, remotePath)
}

// TestConnection 测试 Dropbox 连接.
func (p *DropboxProvider) TestConnection(ctx context.Context) (*ConnectionTestResult, error) {
	start := time.Now()

	// 尝试刷新token
	if err := p.refreshTokenIfNeeded(ctx); err != nil {
		return &ConnectionTestResult{
			Provider: ProviderDropbox,
			Success:  false,
			Message:  fmt.Sprintf("认证失败: %v", err),
		}, nil
	}

	// 获取账户信息
	apiURL := p.apiURL + "/users/get_current_account"

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader([]byte("null")))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	latency := time.Since(start).Milliseconds()

	result := &ConnectionTestResult{
		Provider:  ProviderDropbox,
		Endpoint:  "api.dropboxapi.com",
		LatencyMs: latency,
	}

	if resp.StatusCode == http.StatusUnauthorized {
		result.Success = false
		result.Message = "授权已过期，请重新授权"
		return result, nil
	}

	if resp.StatusCode != http.StatusOK {
		result.Success = false
		result.Message = fmt.Sprintf("连接失败: %s", resp.Status)
		return result, nil
	}

	// 解析账户信息
	var account struct {
		Name struct {
			DisplayName string `json:"display_name"`
		} `json:"name"`
		Email string `json:"email"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&account); err != nil {
		result.Success = true
		result.Message = "连接成功"
		return result, nil
	}

	result.Success = true
	result.Message = fmt.Sprintf("连接成功 (%s)", account.Name.DisplayName)

	return result, nil
}

// Close 关闭连接.
func (p *DropboxProvider) Close() error {
	return nil
}

// GetType 返回提供商类型.
func (p *DropboxProvider) GetType() ProviderType {
	return ProviderDropbox
}

// GetCapabilities 返回支持的功能.
func (p *DropboxProvider) GetCapabilities() []string {
	return []string{"upload", "download", "delete", "list", "share", "multipart"}
}