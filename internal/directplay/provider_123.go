package directplay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Pan123Provider 123云盘直链播放提供商
type Pan123Provider struct {
	client       *http.Client
	accessToken  string
	refreshToken string
	baseURL      string
}

// New123PanProvider 创建123云盘提供商
func New123PanProvider() *Pan123Provider {
	return &Pan123Provider{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://www.123pan.com/api",
	}
}

// GetType 获取提供商类型
func (p *Pan123Provider) GetType() ProviderType {
	return Provider123Pan
}

// SetAccessToken 设置访问令牌
func (p *Pan123Provider) SetAccessToken(accessToken, refreshToken, driveID string) {
	p.accessToken = accessToken
	p.refreshToken = refreshToken
}

// GetDirectLink 获取123云盘直链
func (p *Pan123Provider) GetDirectLink(ctx context.Context, req *DirectPlayRequest) (*DirectLinkInfo, error) {
	if req.AccessToken == "" && p.accessToken == "" {
		return nil, fmt.Errorf("需要访问令牌")
	}

	accessToken := req.AccessToken
	if accessToken == "" {
		accessToken = p.accessToken
	}

	// 123云盘API: /api/v1/worker/download/info
	apiURL := fmt.Sprintf("%s/v1/worker/download/info", p.baseURL)

	body := map[string]interface{}{
		"driveId": 0,
		"etag":    req.FileID,
	}

	bodyBytes, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"message"`
		Data struct {
			DownloadUrl string `json:"downloadUrl"`
			FileName    string `json:"name"`
			Size        int64  `json:"size"`
			Etag        string `json:"etag"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("获取直链失败: %s (code: %d)", result.Msg, result.Code)
	}

	link := &DirectLinkInfo{
		FileID:      result.Data.Etag,
		FileName:    result.Data.FileName,
		FileSize:    result.Data.Size,
		URL:         result.Data.DownloadUrl,
		DownloadURL: result.Data.DownloadUrl,
		StreamURL:   result.Data.DownloadUrl,
		ExpiresAt:   time.Now().Add(1 * time.Hour),
		ExpiresIn:   3600,
		Provider:    Provider123Pan,
	}

	return link, nil
}

// ListFiles 列出123云盘文件
func (p *Pan123Provider) ListFiles(ctx context.Context, req *ListFilesRequest) (*ListFilesResponse, error) {
	accessToken := req.AccessToken
	if accessToken == "" {
		accessToken = p.accessToken
	}

	if accessToken == "" {
		return nil, fmt.Errorf("需要访问令牌")
	}

	parentFileId := req.DirPath
	if parentFileId == "" {
		parentFileId = "0"
	}

	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 100
	}

	apiURL := fmt.Sprintf("%s/v1/worker/file/list", p.baseURL)

	body := map[string]interface{}{
		"driveId":      0,
		"parentFileId": parentFileId,
		"limit":        pageSize,
	}

	bodyBytes, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"message"`
		Data struct {
			InfoList []struct {
				FileId     string `json:"fileId"`
				FileName   string `json:"name"`
				Size       int64  `json:"size"`
				Type       int    `json:"type"` // 0=文件, 1=文件夹
				CreateTime int64  `json:"createTime"`
				UpdateTime int64  `json:"updateTime"`
				Etag       string `json:"etag"`
			} `json:"infoList"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("列出文件失败: %s", result.Msg)
	}

	files := make([]FileInfo, 0, len(result.Data.InfoList))
	for _, item := range result.Data.InfoList {
		files = append(files, FileInfo{
			FileID:   item.FileId,
			FileName: item.FileName,
			FileSize: item.Size,
			IsDir:    item.Type == 1,
			ModTime:  time.Unix(item.UpdateTime, 0),
			Provider: Provider123Pan,
		})
	}

	return &ListFilesResponse{
		Files:    files,
		HasMore:  len(result.Data.InfoList) >= pageSize,
		Page:     req.Page,
		PageSize: pageSize,
	}, nil
}

// GetFileInfo 获取123云盘文件信息
func (p *Pan123Provider) GetFileInfo(ctx context.Context, provider ProviderType, fileID, accessToken string) (*FileInfo, error) {
	// 通过列出文件API获取信息
	return &FileInfo{
		FileID:   fileID,
		Provider: Provider123Pan,
	}, nil
}

// TestConnection 测试123云盘连接
func (p *Pan123Provider) TestConnection(ctx context.Context, accessToken, refreshToken string) (*ProviderInfo, error) {
	if accessToken == "" {
		accessToken = p.accessToken
	}

	info := &ProviderInfo{
		Type:    Provider123Pan,
		Name:    "123云盘",
		Enabled: true,
	}

	apiURL := fmt.Sprintf("%s/v1/worker/user/info", p.baseURL)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		info.Connected = false
		return info, nil
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"message"`
		Data struct {
			NickName   string `json:"nickName"`
			TotalSpace int64  `json:"totalSpace"`
			UsedSpace  int64  `json:"usedSpace"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		info.Connected = false
		return info, nil
	}

	if result.Code != 0 {
		info.Connected = false
		return info, nil
	}

	info.Connected = true
	info.UserName = result.Data.NickName
	info.TotalSpace = result.Data.TotalSpace
	info.UsedSpace = result.Data.UsedSpace

	return info, nil
}
