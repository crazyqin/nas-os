package downloader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TransmissionClient Transmission RPC 客户端
type TransmissionClient struct {
	url      string
	username string
	password string
	session  string
	client   *http.Client
}

// TransmissionRequest Transmission RPC 请求
type TransmissionRequest struct {
	Method    string      `json:"method"`
	Arguments interface{} `json:"arguments,omitempty"`
	Tag       int         `json:"tag,omitempty"`
}

// TransmissionResponse Transmission RPC 响应
type TransmissionResponse struct {
	Result    string          `json:"result"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
	Tag       int             `json:"tag,omitempty"`
}

// TransmissionTorrentAddRequest 添加种子请求
type TransmissionTorrentAddRequest struct {
	Filename    string `json:"filename,omitempty"`     // .torrent 文件 URL 或磁力链接
	Metainfo    string `json:"metainfo,omitempty"`     // base64 编码的 .torrent 文件内容
	DownloadDir string `json:"download-dir,omitempty"` // 下载目录
	Paused      bool   `json:"paused,omitempty"`       // 是否暂停
}

// TransmissionTorrentAddResponse 添加种子响应
type TransmissionTorrentAddResponse struct {
	TorrentDuplicate struct {
		HashString string `json:"hashString"`
		ID         int    `json:"id"`
		Name       string `json:"name"`
	} `json:"torrent-duplicate,omitempty"`
	TorrentAdded struct {
		HashString string `json:"hashString"`
		ID         int    `json:"id"`
		Name       string `json:"name"`
	} `json:"torrent-added,omitempty"`
}

// TransmissionTorrentGetRequest 获取种子信息请求
type TransmissionTorrentGetRequest struct {
	IDS    []int    `json:"ids,omitempty"`    // 种子 ID 列表，空表示所有
	Fields []string `json:"fields,omitempty"` // 需要获取的字段
}

// TransmissionTorrentGetResponse 获取种子信息响应
type TransmissionTorrentGetResponse struct {
	Torrents []TransmissionTorrent `json:"torrents"`
}

// TransmissionTorrent 种子信息
type TransmissionTorrent struct {
	ID             int     `json:"id"`
	Name           string  `json:"name"`
	HashString     string  `json:"hashString"`
	Status         int     `json:"status"`
	TotalSize      int64   `json:"totalSize"`
	DownloadedEver int64   `json:"downloadedEver"`
	UploadedEver   int64   `json:"uploadedEver"`
	PercentDone    float64 `json:"percentDone"`
	RateDownload   int64   `json:"rateDownload"` // 下载速度 (bytes/s)
	RateUpload     int64   `json:"rateUpload"`   // 上传速度 (bytes/s)
	PeersConnected int     `json:"peersConnected"`
	Seeders        int     `json:"seeders"`
	Leechers       int     `json:"leechers"`
	DownloadDir    string  `json:"downloadDir"`
	Error          int     `json:"error"`
	ErrorString    string  `json:"errorString"`
	DoneDate       int64   `json:"doneDate"`
	AddedDate      int64   `json:"addedDate"`
	ActivityDate   int64   `json:"activityDate"`
}

// TransmissionTorrentRemoveRequest 删除种子请求
type TransmissionTorrentRemoveRequest struct {
	IDS             []int `json:"ids"`
	DeleteLocalData bool  `json:"delete-local-data"` // 是否同时删除下载的数据
}

// TransmissionSessionStatsResponse 会话统计响应
type TransmissionSessionStatsResponse struct {
	ActiveTorrentCount int   `json:"activeTorrentCount"`
	CumulativeStats    Stats `json:"cumulativeStats"`
	CurrentStats       Stats `json:"currentStats"`
	PausedTorrentCount int   `json:"pausedTorrentCount"`
	TorrentCount       int   `json:"torrentCount"`
}

// Stats 统计信息
type Stats struct {
	DownloadedBytes int64 `json:"downloadedBytes"`
	UploadedBytes   int64 `json:"uploadedBytes"`
	FilesAdded      int   `json:"filesAdded"`
	SessionCount    int   `json:"sessionCount"`
	SecondsActive   int   `json:"secondsActive"`
}

// NewTransmissionClient 创建 Transmission 客户端
func NewTransmissionClient(url, username, password string) *TransmissionClient {
	return &TransmissionClient{
		url:      strings.TrimSuffix(url, "/") + "/transmission/rpc",
		username: username,
		password: password,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// getSession 获取 session ID
func (c *TransmissionClient) getSession() error {
	if c.session != "" {
		return nil
	}

	req, err := http.NewRequest("GET", c.url, nil)
	if err != nil {
		return err
	}

	if c.username != "" || c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	// Transmission 在 409 响应中返回 session ID
	if resp.StatusCode == http.StatusConflict {
		c.session = resp.Header.Get("X-Transmission-Session-Id")
		if c.session != "" {
			return nil
		}
	}

	// 尝试从响应头获取
	c.session = resp.Header.Get("X-Transmission-Session-Id")
	return nil
}

// doRequest 执行 RPC 请求
func (c *TransmissionClient) doRequest(method string, args interface{}) (*TransmissionResponse, error) {
	if err := c.getSession(); err != nil {
		return nil, err
	}

	reqBody := TransmissionRequest{
		Method:    method,
		Arguments: args,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Transmission-Session-Id", c.session)

	if c.username != "" || c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	// 如果 session 失效，重新获取
	if resp.StatusCode == http.StatusConflict {
		c.session = resp.Header.Get("X-Transmission-Session-Id")
		return c.doRequest(method, args)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result TransmissionResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	if result.Result != "success" {
		return nil, fmt.Errorf("transmission RPC 错误: %s", result.Result)
	}

	return &result, nil
}

// AddTorrent 添加种子
func (c *TransmissionClient) AddTorrent(torrentURL, downloadDir string) (string, int, error) {
	args := TransmissionTorrentAddRequest{
		Filename:    torrentURL,
		DownloadDir: downloadDir,
		Paused:      false,
	}

	resp, err := c.doRequest("torrent-add", args)
	if err != nil {
		return "", 0, err
	}

	var addResp TransmissionTorrentAddResponse
	if err := json.Unmarshal(resp.Arguments, &addResp); err != nil {
		return "", 0, err
	}

	// 返回 hash 和 ID
	if addResp.TorrentAdded.HashString != "" {
		return addResp.TorrentAdded.HashString, addResp.TorrentAdded.ID, nil
	}
	if addResp.TorrentDuplicate.HashString != "" {
		return addResp.TorrentDuplicate.HashString, addResp.TorrentDuplicate.ID, nil
	}

	return "", 0, fmt.Errorf("添加种子失败：未返回有效信息")
}

// GetTorrents 获取种子列表
func (c *TransmissionClient) GetTorrents(ids ...int) ([]TransmissionTorrent, error) {
	args := TransmissionTorrentGetRequest{
		IDS: ids,
		Fields: []string{
			"id", "name", "hashString", "status", "totalSize",
			"downloadedEver", "uploadedEver", "percentDone",
			"rateDownload", "rateUpload", "peersConnected",
			"seeders", "leechers", "downloadDir",
			"error", "errorString", "doneDate", "addedDate", "activityDate",
		},
	}

	resp, err := c.doRequest("torrent-get", args)
	if err != nil {
		return nil, err
	}

	var getResp TransmissionTorrentGetResponse
	if err := json.Unmarshal(resp.Arguments, &getResp); err != nil {
		return nil, err
	}

	return getResp.Torrents, nil
}

// GetTorrentByHash 通过 hash 获取种子信息
func (c *TransmissionClient) GetTorrentByHash(hash string) (*TransmissionTorrent, error) {
	// Transmission 不支持直接用 hash 查询，需要获取所有后过滤
	torrents, err := c.GetTorrents()
	if err != nil {
		return nil, err
	}

	for _, t := range torrents {
		if t.HashString == hash {
			return &t, nil
		}
	}

	return nil, fmt.Errorf("未找到种子: %s", hash)
}

// RemoveTorrent 删除种子
func (c *TransmissionClient) RemoveTorrent(id int, deleteData bool) error {
	args := TransmissionTorrentRemoveRequest{
		IDS:             []int{id},
		DeleteLocalData: deleteData,
	}

	_, err := c.doRequest("torrent-remove", args)
	return err
}

// StopTorrent 暂停种子
func (c *TransmissionClient) StopTorrent(id int) error {
	_, err := c.doRequest("torrent-stop", map[string]interface{}{"ids": []int{id}})
	return err
}

// StartTorrent 开始种子
func (c *TransmissionClient) StartTorrent(id int) error {
	_, err := c.doRequest("torrent-start", map[string]interface{}{"ids": []int{id}})
	return err
}

// GetSessionStats 获取会话统计
func (c *TransmissionClient) GetSessionStats() (*TransmissionSessionStatsResponse, error) {
	resp, err := c.doRequest("session-stats", nil)
	if err != nil {
		return nil, err
	}

	var stats TransmissionSessionStatsResponse
	if err := json.Unmarshal(resp.Arguments, &stats); err != nil {
		return nil, err
	}

	return &stats, nil
}

// ==================== qBittorrent 客户端 ====================

// QBittorrentClient qBittorrent Web API 客户端
type QBittorrentClient struct {
	url      string
	username string
	password string
	client   *http.Client
	cookie   string
}

// QBittorrentTorrentInfo 种子信息
type QBittorrentTorrentInfo struct {
	AddedOn           int64   `json:"added_on"`
	AmountLeft        int64   `json:"amount_left"`
	AutoTmm           bool    `json:"auto_tmm"`
	Availability      float64 `json:"availability"`
	Category          string  `json:"category"`
	Completed         int64   `json:"completed"`
	CompletionOn      int64   `json:"completion_on"`
	ContentPath       string  `json:"content_path"`
	DlLimit           int64   `json:"dl_limit"`
	Dlspeed           int64   `json:"dlspeed"`
	Downloaded        int64   `json:"downloaded"`
	DownloadedSession int64   `json:"downloaded_session"`
	Eta               int64   `json:"eta"`
	FLPiecePrio       bool    `json:"f_l_piece_prio"`
	ForceStart        bool    `json:"force_start"`
	Hash              string  `json:"hash"`
	LastActivity      int64   `json:"last_activity"`
	MagnetURI         string  `json:"magnet_uri"`
	MaxRatio          float64 `json:"max_ratio"`
	MaxSeedingTime    int64   `json:"max_seeding_time"`
	Name              string  `json:"name"`
	NumComplete       int     `json:"num_complete"`
	NumIncomplete     int     `json:"num_incomplete"`
	NumLeechs         int     `json:"num_leechs"`
	NumSeeds          int     `json:"num_seeds"`
	Priority          int     `json:"priority"`
	Progress          float64 `json:"progress"`
	Ratio             float64 `json:"ratio"`
	RatioLimit        float64 `json:"ratio_limit"`
	SavePath          string  `json:"save_path"`
	SeedingTimeLimit  int64   `json:"seeding_time_limit"`
	SeenComplete      int64   `json:"seen_complete"`
	SeqDl             bool    `json:"seq_dl"`
	Size              int64   `json:"size"`
	State             string  `json:"state"`
	SuperSeeding      bool    `json:"super_seeding"`
	Tags              string  `json:"tags"`
	TimeActive        int64   `json:"time_active"`
	TotalSize         int64   `json:"total_size"`
	Tracker           string  `json:"tracker"`
	TrackersCount     int     `json:"trackers_count"`
	UpLimit           int64   `json:"up_limit"`
	Uploaded          int64   `json:"uploaded"`
	UploadedSession   int64   `json:"uploaded_session"`
	Upspeed           int64   `json:"upspeed"`
}

// NewQBittorrentClient 创建 qBittorrent 客户端
func NewQBittorrentClient(url, username, password string) *QBittorrentClient {
	return &QBittorrentClient{
		url:      strings.TrimSuffix(url, "/") + "/api/v2",
		username: username,
		password: password,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// login 登录获取 cookie
func (c *QBittorrentClient) login() error {
	if c.cookie != "" {
		return nil
	}

	formData := url.Values{}
	formData.Set("username", c.username)
	formData.Set("password", c.password)

	req, err := http.NewRequest("POST", c.url+"/auth/login", strings.NewReader(formData.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("qBittorrent 登录失败: %d", resp.StatusCode)
	}

	// 从响应中获取 SID cookie
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "SID" {
			c.cookie = cookie.Value
			return nil
		}
	}

	// 尝试从 Set-Cookie header 获取
	setCookie := resp.Header.Get("Set-Cookie")
	if strings.Contains(setCookie, "SID=") {
		parts := strings.Split(setCookie, ";")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "SID=") {
				c.cookie = strings.TrimPrefix(part, "SID=")
				return nil
			}
		}
	}

	return fmt.Errorf("qBittorrent 登录失败：未获取到 SID")
}

// doRequest 执行 API 请求
func (c *QBittorrentClient) doRequest(method, endpoint string, data url.Values) ([]byte, error) {
	if err := c.login(); err != nil {
		return nil, err
	}

	var body io.Reader
	if data != nil {
		body = strings.NewReader(data.Encode())
	}

	req, err := http.NewRequest(method, c.url+endpoint, body)
	if err != nil {
		return nil, err
	}

	if data != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.Header.Set("Cookie", "SID="+c.cookie)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("qBittorrent API 错误: %d, %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// AddTorrent 添加种子
func (c *QBittorrentClient) AddTorrent(torrentURL, savePath string) error {
	data := url.Values{}
	data.Set("urls", torrentURL)
	if savePath != "" {
		data.Set("savepath", savePath)
	}

	_, err := c.doRequest("POST", "/torrents/add", data)
	return err
}

// AddTorrentFile 通过文件添加种子
func (c *QBittorrentClient) AddTorrentFile(torrentContent []byte, savePath string) error {
	// qBittorrent 需要使用 multipart/form-data 上传文件
	// 这里简化处理，使用 URL 方式
	return fmt.Errorf("暂不支持文件上传方式，请使用 URL 或磁力链接")
}

// GetTorrents 获取种子列表
func (c *QBittorrentClient) GetTorrents() ([]QBittorrentTorrentInfo, error) {
	respBody, err := c.doRequest("GET", "/torrents/info", nil)
	if err != nil {
		return nil, err
	}

	var torrents []QBittorrentTorrentInfo
	if err := json.Unmarshal(respBody, &torrents); err != nil {
		return nil, err
	}

	return torrents, nil
}

// GetTorrentByHash 通过 hash 获取种子信息
func (c *QBittorrentClient) GetTorrentByHash(hash string) (*QBittorrentTorrentInfo, error) {
	data := url.Values{}
	data.Set("hashes", hash)

	respBody, err := c.doRequest("GET", "/torrents/info?hashes="+hash, nil)
	if err != nil {
		return nil, err
	}

	var torrents []QBittorrentTorrentInfo
	if err := json.Unmarshal(respBody, &torrents); err != nil {
		return nil, err
	}

	if len(torrents) == 0 {
		return nil, fmt.Errorf("未找到种子: %s", hash)
	}

	return &torrents[0], nil
}

// PauseTorrent 暂停种子
func (c *QBittorrentClient) PauseTorrent(hash string) error {
	data := url.Values{}
	data.Set("hashes", hash)

	_, err := c.doRequest("POST", "/torrents/pause", data)
	return err
}

// ResumeTorrent 恢复种子
func (c *QBittorrentClient) ResumeTorrent(hash string) error {
	data := url.Values{}
	data.Set("hashes", hash)

	_, err := c.doRequest("POST", "/torrents/resume", data)
	return err
}

// DeleteTorrent 删除种子
func (c *QBittorrentClient) DeleteTorrent(hash string, deleteFiles bool) error {
	data := url.Values{}
	data.Set("hashes", hash)
	if deleteFiles {
		data.Set("deleteFiles", "true")
	}

	_, err := c.doRequest("POST", "/torrents/delete", data)
	return err
}

// SetDownloadLimit 设置下载限速
func (c *QBittorrentClient) SetDownloadLimit(hash string, limit int64) error {
	data := url.Values{}
	data.Set("hashes", hash)
	data.Set("limit", fmt.Sprintf("%d", limit))

	_, err := c.doRequest("POST", "/torrents/setDownloadLimit", data)
	return err
}

// SetUploadLimit 设置上传限速
func (c *QBittorrentClient) SetUploadLimit(hash string, limit int64) error {
	data := url.Values{}
	data.Set("hashes", hash)
	data.Set("limit", fmt.Sprintf("%d", limit))

	_, err := c.doRequest("POST", "/torrents/setUploadLimit", data)
	return err
}

// GetTransferInfo 获取传输信息
func (c *QBittorrentClient) GetTransferInfo() (map[string]interface{}, error) {
	respBody, err := c.doRequest("GET", "/transfer/info", nil)
	if err != nil {
		return nil, err
	}

	var info map[string]interface{}
	if err := json.Unmarshal(respBody, &info); err != nil {
		return nil, err
	}

	return info, nil
}

// Logout 登出
func (c *QBittorrentClient) Logout() error {
	_, err := c.doRequest("POST", "/auth/logout", nil)
	c.cookie = ""
	return err
}
