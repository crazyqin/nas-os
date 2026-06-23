// Package iptvservice IPTV 直播服务
// 对标飞牛 fnOS 局域网直播源功能，支持 IPTV 直播流接入与管理
package iptvservice

import (
	"fmt"
	"sync"
	"time"
)

// ChannelType 频道类型
type ChannelType string

const (
	ChannelTypeLive   ChannelType = "live"   // 直播频道
	ChannelTypeRadio  ChannelType = "radio"  // 电台
	ChannelTypeCustom ChannelType = "custom" // 自定义源
)

// StreamProtocol 流媒体协议
type StreamProtocol string

const (
	ProtocolHLS  StreamProtocol = "hls"   // HLS
	ProtocolRTMP StreamProtocol = "rtmp"  // RTMP
	ProtocolRTSP StreamProtocol = "rtsp"  // RTSP
	ProtocolUDP  StreamProtocol = "udp"   // UDP/RTP
	ProtocolHTTP StreamProtocol = "http"  // HTTP-FLV
)

// ChannelStatus 频道状态
type ChannelStatus string

const (
	StatusOnline  ChannelStatus = "online"
	StatusOffline ChannelStatus = "offline"
	StatusTesting ChannelStatus = "testing"
)

// Channel 直播频道
type Channel struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Logo        string          `json:"logo,omitempty"`
	Group       string          `json:"group"`
	Type        ChannelType     `json:"type"`
	Protocol    StreamProtocol  `json:"protocol"`
	URL         string          `json:"url"`
	BackupURLs  []string        `json:"backupUrls,omitempty"`
	EPGID       string          `json:"epgId,omitempty"`
	Resolution  string          `json:"resolution,omitempty"`
	Language    string          `json:"language,omitempty"`
	Country     string          `json:"country,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
	Status      ChannelStatus   `json:"status"`
	LastCheck   time.Time       `json:"lastCheck,omitempty"`
	ViewCount   int64           `json:"viewCount"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

// ChannelGroup 频道分组
type ChannelGroup struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Icon      string     `json:"icon,omitempty"`
	Order     int        `json:"order"`
	Channels  []string   `json:"channels"` // 频道ID列表
}

// EPGProgram EPG节目单
type EPGProgram struct {
	ID        string    `json:"id"`
	ChannelID string    `json:"channelId"`
	Title     string    `json:"title"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
	Desc      string    `json:"desc,omitempty"`
	Category  string    `json:"category,omitempty"`
}

// M3UPlaylist M3U播放列表
type M3UPlaylist struct {
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Channels  int       `json:"channels"`
	UpdatedAt time.Time `json:"updatedAt"`
	AutoRefresh bool    `json:"autoRefresh"`
	RefreshInterval int `json:"refreshInterval"` // 分钟
}

// IPTVService IPTV 服务
type IPTVService struct {
	mu         sync.RWMutex
	channels   map[string]*Channel
	groups     map[string]*ChannelGroup
	playlists  map[string]*M3UPlaylist
	epgData    map[string][]EPGProgram
}

// NewIPTVService 创建 IPTV 服务
func NewIPTVService() *IPTVService {
	return &IPTVService{
		channels:  make(map[string]*Channel),
		groups:    make(map[string]*ChannelGroup),
		playlists: make(map[string]*M3UPlaylist),
		epgData:   make(map[string][]EPGProgram),
	}
}

// AddChannel 添加频道
func (s *IPTVService) AddChannel(ch *Channel) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ch.ID == "" {
		ch.ID = fmt.Sprintf("ch-%d", time.Now().UnixNano())
	}
	if ch.Name == "" {
		return fmt.Errorf("频道名称不能为空")
	}
	if ch.URL == "" {
		return fmt.Errorf("频道 URL 不能为空")
	}

	ch.CreatedAt = time.Now()
	ch.UpdatedAt = time.Now()
	ch.Status = StatusOnline

	s.channels[ch.ID] = ch
	return nil
}

// UpdateChannel 更新频道
func (s *IPTVService) UpdateChannel(id string, update *Channel) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch, exists := s.channels[id]
	if !exists {
		return fmt.Errorf("频道 %s 不存在", id)
	}

	if update.Name != "" {
		ch.Name = update.Name
	}
	if update.URL != "" {
		ch.URL = update.URL
	}
	if update.Group != "" {
		ch.Group = update.Group
	}
	ch.UpdatedAt = time.Now()

	return nil
}

// DeleteChannel 删除频道
func (s *IPTVService) DeleteChannel(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.channels[id]; !exists {
		return fmt.Errorf("频道 %s 不存在", id)
	}

	delete(s.channels, id)
	return nil
}

// GetChannel 获取频道
func (s *IPTVService) GetChannel(id string) (*Channel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ch, exists := s.channels[id]
	if !exists {
		return nil, fmt.Errorf("频道 %s 不存在", id)
	}

	return ch, nil
}

// ListChannels 列出所有频道
func (s *IPTVService) ListChannels(group string) []*Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Channel
	for _, ch := range s.channels {
		if group == "" || ch.Group == group {
			result = append(result, ch)
		}
	}
	return result
}

// AddGroup 添加分组
func (s *IPTVService) AddGroup(group *ChannelGroup) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if group.ID == "" {
		group.ID = fmt.Sprintf("grp-%d", time.Now().UnixNano())
	}

	s.groups[group.ID] = group
	return nil
}

// ListGroups 列出所有分组
func (s *IPTVService) ListGroups() []*ChannelGroup {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ChannelGroup
	for _, g := range s.groups {
		result = append(result, g)
	}
	return result
}

// ImportM3U 导入 M3U 播放列表
func (s *IPTVService) ImportM3U(name, url string) (*M3UPlaylist, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	playlist := &M3UPlaylist{
		Name:      name,
		URL:       url,
		UpdatedAt: time.Now(),
	}

	s.playlists[name] = playlist
	return playlist, nil
}

// ListPlaylists 列出播放列表
func (s *IPTVService) ListPlaylists() []*M3UPlaylist {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*M3UPlaylist
	for _, p := range s.playlists {
		result = append(result, p)
	}
	return result
}

// RecordChannel 记录频道观看
func (s *IPTVService) RecordChannel(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch, exists := s.channels[id]
	if !exists {
		return fmt.Errorf("频道 %s 不存在", id)
	}

	ch.ViewCount++
	return nil
}

// SearchChannels 搜索频道
func (s *IPTVService) SearchChannels(keyword string) []*Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Channel
	for _, ch := range s.channels {
		if containsIgnoreCase(ch.Name, keyword) || containsIgnoreCase(ch.Group, keyword) {
			result = append(result, ch)
		}
	}
	return result
}

// GetStats 获取统计信息
func (s *IPTVService) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	online := 0
	for _, ch := range s.channels {
		if ch.Status == StatusOnline {
			online++
		}
	}

	return map[string]interface{}{
		"totalChannels":  len(s.channels),
		"onlineChannels": online,
		"totalGroups":    len(s.groups),
		"totalPlaylists": len(s.playlists),
	}
}

func containsIgnoreCase(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	// 简单的子串搜索
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			sc := s[i+j]
			tc := substr[j]
			// 大小写不敏感比较
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if tc >= 'A' && tc <= 'Z' {
				tc += 32
			}
			if sc != tc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
