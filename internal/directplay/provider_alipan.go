package directplay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// AliyunPanProvider 阿里云盘直链播放提供商
type AliyunPanProvider struct {
	client       *http.Client
	accessToken  string
	refreshToken string
	driveID      string
	baseURL      string
}

// NewAliyunPanProvider 创建阿里云盘提供商
func NewAliyunPanProvider() *AliyunPanProvider {
	return &AliyunPanProvider{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://api.aliyundrive.com/v2",
	}
}

// GetType 获取提供商类型
func (p *AliyunPanProvider) GetType() ProviderType {
	return ProviderAliyunPan
}

// SetAccessToken 设置访问令牌
func (p *AliyunPanProvider) SetAccessToken(accessToken, refreshToken, driveID string) {
	p.accessToken = accessToken
	p.refreshToken = refreshToken
	p.driveID = driveID
}

// GetDirectLink 获取阿里云盘直链
func (p *AliyunPanProvider) GetDirectLink(ctx context.Context, req *DirectPlayRequest) (*DirectLinkInfo, error) {
	accessToken := req.AccessToken
	if accessToken == "" {
		accessToken = p.accessToken
	}

	driveID := req.DriveID
	if driveID == "" {
		driveID = p.driveID
	}

	if accessToken == "" {
		return nil, fmt.Errorf("需要访问令牌")
	}

	// 阿里云盘直链API
	apiURL := p.baseURL + "/file/get_download_url"

	body := map[string]interface{}{
		"drive_id":   driveID,
		"file_id":    req.FileID,
		"expire_sec": 14400, // 4小时有效期
	}

	bodyBytes, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	p.setHeaders(httpReq, accessToken)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Code        string `json:"code"`
		Message     string `json:"message"`
		URL         string `json:"url"`
		InternalURL string `json:"internal_url"`
		Expiration  string `json:"expiration"`
		RateLimit   struct {
			PartSpeed  int `json:"part_speed"`
			PartNumber int `json:"part_number"`
		} `json:"rate_limit"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if result.Code != "" && result.Code != "Success" {
		switch result.Code {
		case "AccessTokenInvalid":
			return nil, fmt.Errorf("登录状态已过期，请重新授权")
		case "ForbiddenFileInTheRecycleBin":
			return nil, fmt.Errorf("文件在回收站中")
		case "ForbiddenNoPermissionFile":
			return nil, fmt.Errorf("没有文件访问权限")
		case "NotFound.File":
			return nil, fmt.Errorf("文件不存在")
		default:
			return nil, fmt.Errorf("获取直链失败: %s", result.Message)
		}
	}

	// 获取文件详细信息
	fileInfo, _ := p.getFileInfo(ctx, accessToken, driveID, req.FileID)

	// 解析过期时间
	var expiresAt time.Time
	if result.Expiration != "" {
		expiresAt, _ = time.Parse(time.RFC3339, result.Expiration)
	} else {
		expiresAt = time.Now().Add(4 * time.Hour)
	}

	link := &DirectLinkInfo{
		FileID:      req.FileID,
		URL:         result.URL,
		DownloadURL: result.URL,
		StreamURL:   result.URL, // 阿里云盘链接支持流播放
		ExpiresAt:   expiresAt,
		ExpiresIn:   int64(time.Until(expiresAt).Seconds()),
		Headers: map[string]string{
			"Referer":    "https://www.aliyundrive.com/",
			"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		},
		Provider: ProviderAliyunPan,
	}

	// 填充文件信息
	if fileInfo != nil {
		link.FileName = fileInfo.FileName
		link.FilePath = fileInfo.FilePath
		link.FileSize = fileInfo.FileSize
		link.Duration = fileInfo.Duration
		link.VideoCodec = fileInfo.VideoCodec
		link.AudioCodec = fileInfo.AudioCodec
		link.Width = fileInfo.Width
		link.Height = fileInfo.Height
	}

	return link, nil
}

// getFileInfo 获取阿里云盘文件详细信息
func (p *AliyunPanProvider) getFileInfo(ctx context.Context, accessToken, driveID, fileID string) (*FileInfo, error) {
	apiURL := p.baseURL + "/file/get"

	body := map[string]interface{}{
		"drive_id": driveID,
		"file_id":  fileID,
		"fields":   "name,path,size,type,file_extension,video_media_metadata,image_media_metadata,created_at,updated_at",
	}

	bodyBytes, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	p.setHeaders(httpReq, accessToken)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Code             string `json:"code"`
		Message          string `json:"message"`
		Name             string `json:"name"`
		Path             string `json:"path"`
		Size             int64  `json:"size"`
		Type             string `json:"type"`
		FileExtension    string `json:"file_extension"`
		CreatedAt        string `json:"created_at"`
		UpdatedAt        string `json:"updated_at"`
		VideoMediaMetadata struct {
			Duration   string `json:"duration"`
			Width      int    `json:"width"`
			Height     int    `json:"height"`
			VideoCodec string `json:"video_media_stream.0.codec_name"`
			AudioCodec string `json:"audio_media_stream.0.codec_name"`
		} `json:"video_media_metadata"`
		ImageMediaMetadata struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"image_media_metadata"`
		Thumbnail string `json:"thumbnail"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Code != "" && result.Code != "Success" {
		return nil, fmt.Errorf("获取文件信息失败: %s", result.Message)
	}

	// 解析时长
	var duration int64
	if result.VideoMediaMetadata.Duration != "" {
		// 阿里云盘返回的duration格式可能是毫秒
		_, _ = fmt.Sscanf(result.VideoMediaMetadata.Duration, "%d", &duration)
	}

	// 解析时间
	modTime, _ := time.Parse(time.RFC3339, result.UpdatedAt)

	return &FileInfo{
		FileID:     fileID,
		FileName:   result.Name,
		FilePath:   result.Path,
		FileSize:   result.Size,
		IsDir:      result.Type == "folder",
		ModTime:    modTime,
		MimeType:   getMimeType(result.FileExtension),
		Duration:   duration,
		VideoCodec: result.VideoMediaMetadata.VideoCodec,
		AudioCodec: result.VideoMediaMetadata.AudioCodec,
		Width:      result.VideoMediaMetadata.Width,
		Height:     result.VideoMediaMetadata.Height,
		Provider:   ProviderAliyunPan,
	}, nil
}

// ListFiles 列出阿里云盘文件
func (p *AliyunPanProvider) ListFiles(ctx context.Context, req *ListFilesRequest) (*ListFilesResponse, error) {
	accessToken := req.AccessToken
	if accessToken == "" {
		accessToken = p.accessToken
	}

	driveID := req.DriveID
	if driveID == "" {
		driveID = p.driveID
	}

	if accessToken == "" {
		return nil, fmt.Errorf("需要访问令牌")
	}

	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 100
	}

	// 目录ID
	parentID := req.DirPath
	if parentID == "" || parentID == "/" {
		parentID = "root"
	}

	// 阿里云盘文件列表API
	apiURL := p.baseURL + "/file/list"

	body := map[string]interface{}{
		"drive_id":        driveID,
		"parent_file_id":  parentID,
		"limit":           pageSize,
		"order_by":        "updated_at",
		"order_direction": "DESC",
		"fields":          "name,file_id,size,type,updated_at,created_at,file_extension",
	}

	bodyBytes, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	p.setHeaders(httpReq, accessToken)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Code       string `json:"code"`
		Message    string `json:"message"`
		Items      []struct {
			FileId       string `json:"file_id"`
			Name         string `json:"name"`
			Size         int64  `json:"size"`
			Type         string `json:"type"`
			UpdatedAt    string `json:"updated_at"`
			CreatedAt    string `json:"created_at"`
			FileExtension string `json:"file_extension"`
		} `json:"items"`
		NextMarker string `json:"next_marker"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if result.Code != "" && result.Code != "Success" {
		return nil, fmt.Errorf("列出文件失败: %s", result.Message)
	}

	files := make([]FileInfo, 0, len(result.Items))
	for _, item := range result.Items {
		modTime, _ := time.Parse(time.RFC3339, item.UpdatedAt)
		files = append(files, FileInfo{
			FileID:   item.FileId,
			FileName: item.Name,
			FileSize: item.Size,
			IsDir:    item.Type == "folder",
			ModTime:  modTime,
			MimeType: getMimeType(item.FileExtension),
			Provider: ProviderAliyunPan,
		})
	}

	return &ListFilesResponse{
		Files:    files,
		HasMore:  result.NextMarker != "",
		PageSize: pageSize,
	}, nil
}

// GetFileInfo 获取阿里云盘文件信息
func (p *AliyunPanProvider) GetFileInfo(ctx context.Context, provider ProviderType, fileID, accessToken string) (*FileInfo, error) {
	if accessToken == "" {
		accessToken = p.accessToken
	}

	return p.getFileInfo(ctx, accessToken, p.driveID, fileID)
}

// TestConnection 测试阿里云盘连接
func (p *AliyunPanProvider) TestConnection(ctx context.Context, accessToken, refreshToken string) (*ProviderInfo, error) {
	if accessToken == "" {
		accessToken = p.accessToken
	}

	info := &ProviderInfo{
		Type:    ProviderAliyunPan,
		Name:    "阿里云盘",
		Enabled: true,
	}

	// 获取用户信息
	apiURL := p.baseURL + "/user/get"

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader([]byte("{}")))
	if err != nil {
		return nil, err
	}

	p.setHeaders(httpReq, accessToken)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		info.Connected = false
		return info, nil
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Code          string `json:"code"`
		Message       string `json:"message"`
		NickName      string `json:"nick_name"`
		UserName      string `json:"user_name"`
		DefaultDriveId string `json:"default_drive_id"`
		UsedSpaceSize int64  `json:"used_size"`
		TotalSpaceSize int64 `json:"total_size"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		info.Connected = false
		return info, nil
	}

	if result.Code != "" && result.Code != "Success" {
		info.Connected = false
		info.Expired = result.Code == "AccessTokenInvalid"
		return info, nil
	}

	info.Connected = true
	info.UserName = result.NickName
	if info.UserName == "" {
		info.UserName = result.UserName
	}
	info.TotalSpace = result.TotalSpaceSize
	info.UsedSpace = result.UsedSpaceSize

	// 更新driveID
	if result.DefaultDriveId != "" {
		p.driveID = result.DefaultDriveId
	}

	return info, nil
}

// GetOpenFileDirectLink 获取阿里云盘分享文件直链（优化版）
func (p *AliyunPanProvider) GetOpenFileDirectLink(ctx context.Context, accessToken, shareID, fileID string) (*DirectLinkInfo, error) {
	// 阿里云盘分享文件直链API
	apiURL := p.baseURL + "/file/get_share_link_download_url"

	body := map[string]interface{}{
		"share_id": shareID,
		"file_id":  fileID,
	}

	bodyBytes, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	p.setHeaders(httpReq, accessToken)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		DownloadURL string `json:"download_url"`
		Expiration  string `json:"expiration"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Code != "" && result.Code != "Success" {
		return nil, fmt.Errorf("获取分享链接失败: %s", result.Message)
	}

	var expiresAt time.Time
	if result.Expiration != "" {
		expiresAt, _ = time.Parse(time.RFC3339, result.Expiration)
	} else {
		expiresAt = time.Now().Add(4 * time.Hour)
	}

	return &DirectLinkInfo{
		FileID:      fileID,
		URL:         result.DownloadURL,
		DownloadURL: result.DownloadURL,
		StreamURL:   result.DownloadURL,
		ExpiresAt:   expiresAt,
		ExpiresIn:   int64(time.Until(expiresAt).Seconds()),
		Headers: map[string]string{
			"Referer": "https://www.aliyundrive.com/",
		},
		Provider: ProviderAliyunPan,
	}, nil
}

// setHeaders 设置请求头
func (p *AliyunPanProvider) setHeaders(req *http.Request, accessToken string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
}

// getMimeType 根据文件扩展名获取MIME类型
func getMimeType(ext string) string {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	mimeTypes := map[string]string{
		"mp4":  "video/mp4",
		"mkv":  "video/x-matroska",
		"avi":  "video/x-msvideo",
		"mov":  "video/quicktime",
		"wmv":  "video/x-ms-wmv",
		"flv":  "video/x-flv",
		"webm": "video/webm",
		"m3u8": "application/vnd.apple.mpegurl",
		"ts":   "video/mp2t",
		"mp3":  "audio/mpeg",
		"m4a":  "audio/mp4",
		"flac": "audio/flac",
		"wav":  "audio/wav",
		"aac":  "audio/aac",
		"jpg":  "image/jpeg",
		"jpeg": "image/jpeg",
		"png":  "image/png",
		"gif":  "image/gif",
		"webp": "image/webp",
		"bmp":  "image/bmp",
		"pdf":  "application/pdf",
		"doc":  "application/msword",
		"docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"xls":  "application/vnd.ms-excel",
		"xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"ppt":  "application/vnd.ms-powerpoint",
		"pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"zip":  "application/zip",
		"rar":  "application/x-rar-compressed",
		"7z":   "application/x-7z-compressed",
		"tar":  "application/x-tar",
		"gz":   "application/gzip",
	}

	if mt, ok := mimeTypes[ext]; ok {
		return mt
	}
	return "application/octet-stream"
}