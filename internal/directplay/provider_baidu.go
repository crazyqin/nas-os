package directplay

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// BaiduPanProvider 百度网盘直链播放提供商
type BaiduPanProvider struct {
	client       *http.Client
	accessToken  string
	refreshToken string
	baseURL      string
}

// NewBaiduPanProvider 创建百度网盘提供商
func NewBaiduPanProvider() *BaiduPanProvider {
	return &BaiduPanProvider{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://pan.baidu.com/rest/2.0/xpan",
	}
}

// GetType 获取提供商类型
func (p *BaiduPanProvider) GetType() ProviderType {
	return ProviderBaiduPan
}

// SetAccessToken 设置访问令牌
func (p *BaiduPanProvider) SetAccessToken(accessToken, refreshToken, driveID string) {
	p.accessToken = accessToken
	p.refreshToken = refreshToken
}

// GetDirectLink 获取百度网盘直链
func (p *BaiduPanProvider) GetDirectLink(ctx context.Context, req *DirectPlayRequest) (*DirectLinkInfo, error) {
	if req.AccessToken == "" && p.accessToken == "" {
		return nil, fmt.Errorf("需要访问令牌")
	}

	accessToken := req.AccessToken
	if accessToken == "" {
		accessToken = p.accessToken
	}

	// 构建请求获取文件下载链接
	// 百度网盘API: /xpan/multimedia?method=filemetas
	apiURL := fmt.Sprintf("%s/multimedia?method=filemetas&access_token=%s", p.baseURL, accessToken)

	// 请求体
	body := map[string]interface{}{
		"fsids":    []string{req.FileID},
		"dlink":    1,
		"extra":    1, // 获取额外信息
		"thumb":    1, // 获取缩略图
	}

	bodyBytes, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Errno  int    `json:"errno"`
		ErrMsg string `json:"errmsg,omitempty"`
		List   []struct {
			FsID       int64  `json:"fs_id"`
			Path       string `json:"path"`
			Filename   string `json:"filename"`
			Size       int64  `json:"size"`
			Dlink      string `json:"dlink"`
			Md5        string `json:"md5"`
			ServerMtime int64 `json:"server_mtime"`
			Thumbs     struct {
				Icon string `json:"icon,omitempty"`
				URL3 string `json:"url3,omitempty"` // 大缩略图
			} `json:"thumbs"`
			MediaInfo struct {
				Duration   int    `json:"duration"`
				VideoCodec string `json:"video_codec"`
				AudioCodec string `json:"audio_codec"`
				Width      int    `json:"width"`
				Height     int    `json:"height"`
			} `json:"media_info"`
		} `json:"list"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if result.Errno != 0 {
		// 错误码处理
		switch result.Errno {
		case -6:
			return nil, fmt.Errorf("登录状态已过期，请重新授权")
		case 2:
			return nil, fmt.Errorf("参数错误")
		case 10:
			return nil, fmt.Errorf("文件不存在")
		case 119:
			return nil, fmt.Errorf("文件被禁止下载")
		default:
			return nil, fmt.Errorf("获取直链失败: %s (errno: %d)", result.ErrMsg, result.Errno)
		}
	}

	if len(result.List) == 0 {
		return nil, fmt.Errorf("文件不存在")
	}

	file := result.List[0]

	// 构建完整下载链接
	downloadURL := file.Dlink
	if downloadURL != "" {
		downloadURL += "&access_token=" + accessToken
	}

	link := &DirectLinkInfo{
		FileID:       fmt.Sprintf("%d", file.FsID),
		FileName:     file.Filename,
		FilePath:     file.Path,
		FileSize:     file.Size,
		URL:          downloadURL,
		DownloadURL:  downloadURL,
		StreamURL:    downloadURL, // 百度网盘下载链接可直接用于流播放
		ExpiresAt:    time.Now().Add(8 * time.Hour), // 百度网盘链接有效期约8小时
		ExpiresIn:    8 * 60 * 60,
		Headers: map[string]string{
			"User-Agent": "LogStatistic",
		},
		Provider:     ProviderBaiduPan,
		Duration:     int64(file.MediaInfo.Duration),
		VideoCodec:   file.MediaInfo.VideoCodec,
		AudioCodec:   file.MediaInfo.AudioCodec,
		Width:        file.MediaInfo.Width,
		Height:       file.MediaInfo.Height,
		ThumbnailURL: file.Thumbs.URL3,
	}

	return link, nil
}

// ListFiles 列出百度网盘文件
func (p *BaiduPanProvider) ListFiles(ctx context.Context, req *ListFilesRequest) (*ListFilesResponse, error) {
	accessToken := req.AccessToken
	if accessToken == "" {
		accessToken = p.accessToken
	}

	if accessToken == "" {
		return nil, fmt.Errorf("需要访问令牌")
	}

	dirPath := req.DirPath
	if dirPath == "" {
		dirPath = "/"
	}

	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 100
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}

	// 百度网盘API: /xpan/file?method=list
	apiURL := fmt.Sprintf("%s/file?method=list&access_token=%s&dir=%s&num=%d&order=time&desc=1",
		p.baseURL, accessToken, url.QueryEscape(dirPath), pageSize)

	if req.Recursive {
		apiURL += "&recursion=1"
	}

	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Errno int `json:"errno"`
		List  []struct {
			FsID        int64  `json:"fs_id"`
			Path        string `json:"path"`
			ServerMtime int64  `json:"server_mtime"`
			Size        int64  `json:"size"`
			Isdir       int    `json:"isdir"`
			Filename    string `json:"filename"`
			Md5         string `json:"md5"`
		} `json:"list"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if result.Errno != 0 {
		return nil, fmt.Errorf("列出文件失败: errno=%d", result.Errno)
	}

	files := make([]FileInfo, 0, len(result.List))
	for _, item := range result.List {
		files = append(files, FileInfo{
			FileID:   fmt.Sprintf("%d", item.FsID),
			FileName: item.Filename,
			FilePath: item.Path,
			FileSize: item.Size,
			IsDir:    item.Isdir == 1,
			ModTime:  time.Unix(item.ServerMtime, 0),
			Provider: ProviderBaiduPan,
		})
	}

	return &ListFilesResponse{
		Files:    files,
		HasMore:  len(result.List) >= pageSize,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetFileInfo 获取百度网盘文件信息
func (p *BaiduPanProvider) GetFileInfo(ctx context.Context, provider ProviderType, fileID, accessToken string) (*FileInfo, error) {
	if accessToken == "" {
		accessToken = p.accessToken
	}

	apiURL := fmt.Sprintf("%s/file?method=filemetas&access_token=%s&fsids=[%s]&dlink=1&extra=1",
		p.baseURL, accessToken, fileID)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Errno int `json:"errno"`
		List  []struct {
			FsID        int64  `json:"fs_id"`
			Path        string `json:"path"`
			Filename    string `json:"filename"`
			Size        int64  `json:"size"`
			Isdir       int    `json:"isdir"`
			ServerMtime int64  `json:"server_mtime"`
			Md5         string `json:"md5"`
		} `json:"list"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if result.Errno != 0 || len(result.List) == 0 {
		return nil, fmt.Errorf("文件不存在")
	}

	item := result.List[0]
	return &FileInfo{
		FileID:   fmt.Sprintf("%d", item.FsID),
		FileName: item.Filename,
		FilePath: item.Path,
		FileSize: item.Size,
		IsDir:    item.Isdir == 1,
		ModTime:  time.Unix(item.ServerMtime, 0),
		Provider: ProviderBaiduPan,
	}, nil
}

// TestConnection 测试百度网盘连接
func (p *BaiduPanProvider) TestConnection(ctx context.Context, accessToken, refreshToken string) (*ProviderInfo, error) {
	if accessToken == "" {
		accessToken = p.accessToken
	}

	info := &ProviderInfo{
		Type:    ProviderBaiduPan,
		Name:    "百度网盘",
		Enabled: true,
	}

	// 获取用户信息
	apiURL := fmt.Sprintf("https://pan.baidu.com/rest/2.0/xpan/nas?method=uinfo&access_token=%s", accessToken)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		info.Connected = false
		return info, nil
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Errno       int    `json:"errno"`
		BaiduName   string `json:"baidu_name"`
		TotalQuota  int64  `json:"total_quota"`
		UsedQuota   int64  `json:"used_quota"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		info.Connected = false
		return info, nil
	}

	if result.Errno != 0 {
		info.Connected = false
		info.Expired = result.Errno == -6
		return info, nil
	}

	info.Connected = true
	info.UserName = result.BaiduName
	info.TotalSpace = result.TotalQuota
	info.UsedSpace = result.UsedQuota

	return info, nil
}