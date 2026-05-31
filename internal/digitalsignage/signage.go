package digitalsignage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Manager 数字标牌管理器
type Manager struct {
	mu        sync.RWMutex
	contents  map[string]*Content
	playlists map[string]*Playlist
	schedules map[string]*Schedule
	devices   map[string]*Device
	groups    map[string]*DeviceGroup
	templates map[string]*Template
	playback  map[string]*PlaybackStatus // deviceID -> status
	logger    Logger
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// Logger 日志接口
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}

// NewManager 创建数字标牌管理器
func NewManager(logger Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		contents:  make(map[string]*Content),
		playlists: make(map[string]*Playlist),
		schedules: make(map[string]*Schedule),
		devices:   make(map[string]*Device),
		groups:    make(map[string]*DeviceGroup),
		templates: make(map[string]*Template),
		playback:  make(map[string]*PlaybackStatus),
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
	}

	// 初始化默认模板
	m.initDefaultTemplates()

	// 启动排程调度器
	m.wg.Add(1)
	go m.scheduleLoop()

	// 启动设备心跳检查
	m.wg.Add(1)
	go m.deviceHealthCheck()

	return m
}

// initDefaultTemplates 初始化默认模板
func (m *Manager) initDefaultTemplates() {
	defaultTemplates := []*Template{
		{
			ID:          "tpl_fullscreen",
			Name:        "全屏",
			Description: "单区域全屏显示",
			Type:        LayoutTypeFullscreen,
			Zones:       []LayoutZone{{ID: "zone1", X: 0, Y: 0, Width: 100, Height: 100}},
			IsDefault:   true,
		},
		{
			ID:          "tpl_split2",
			Name:        "双分屏",
			Description: "左右两区域分屏",
			Type:        LayoutTypeSplit2,
			Zones: []LayoutZone{
				{ID: "zone1", X: 0, Y: 0, Width: 50, Height: 100},
				{ID: "zone2", X: 50, Y: 0, Width: 50, Height: 100},
			},
		},
		{
			ID:          "tpl_split4",
			Name:        "四分屏",
			Description: "四区域分屏",
			Type:        LayoutTypeSplit4,
			Zones: []LayoutZone{
				{ID: "zone1", X: 0, Y: 0, Width: 50, Height: 50},
				{ID: "zone2", X: 50, Y: 0, Width: 50, Height: 50},
				{ID: "zone3", X: 0, Y: 50, Width: 50, Height: 50},
				{ID: "zone4", X: 50, Y: 50, Width: 50, Height: 50},
			},
		},
		{
			ID:          "tpl_pip",
			Name:        "画中画",
			Description: "主区域+小窗口",
			Type:        LayoutTypePIP,
			Zones: []LayoutZone{
				{ID: "zone1", X: 0, Y: 0, Width: 100, Height: 100},
				{ID: "zone2", X: 65, Y: 60, Width: 30, Height: 30},
			},
		},
	}

	for _, tpl := range defaultTemplates {
		tpl.CreatedAt = time.Now()
		m.templates[tpl.ID] = tpl
	}
}

// ==================== 内容管理 ====================

// CreateContent 创建内容
func (m *Manager) CreateContent(content *Content) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if content.ID == "" {
		content.ID = generateID("content")
	}
	content.Status = ContentStatusActive
	content.CreatedAt = time.Now()
	content.UpdatedAt = time.Now()

	m.contents[content.ID] = content
	m.logger.Info("内容创建成功: %s (%s)", content.Name, content.ID)
	return nil
}

// UpdateContent 更新内容
func (m *Manager) UpdateContent(content *Content) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.contents[content.ID]
	if !ok {
		return fmt.Errorf("内容不存在: %s", content.ID)
	}

	content.CreatedAt = existing.CreatedAt
	content.UpdatedAt = time.Now()
	m.contents[content.ID] = content
	m.logger.Info("内容更新成功: %s (%s)", content.Name, content.ID)
	return nil
}

// DeleteContent 删除内容
func (m *Manager) DeleteContent(contentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.contents[contentID]; !ok {
		return fmt.Errorf("内容不存在: %s", contentID)
	}

	delete(m.contents, contentID)
	m.logger.Info("内容删除成功: %s", contentID)
	return nil
}

// GetContent 获取内容
func (m *Manager) GetContent(contentID string) (*Content, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	content, ok := m.contents[contentID]
	if !ok {
		return nil, fmt.Errorf("内容不存在: %s", contentID)
	}
	return content, nil
}

// ListContents 列出所有内容
func (m *Manager) ListContents(contentType *ContentType) []*Content {
	m.mu.RLock()
	defer m.mu.RUnlock()

	contents := make([]*Content, 0)
	for _, content := range m.contents {
		if contentType != nil && content.Type != *contentType {
			continue
		}
		contents = append(contents, content)
	}
	return contents
}

// ==================== 播放列表管理 ====================

// CreatePlaylist 创建播放列表
func (m *Manager) CreatePlaylist(playlist *Playlist) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if playlist.ID == "" {
		playlist.ID = generateID("playlist")
	}
	playlist.Status = PlaylistStatusActive
	playlist.CreatedAt = time.Now()
	playlist.UpdatedAt = time.Now()

	m.playlists[playlist.ID] = playlist
	m.logger.Info("播放列表创建成功: %s (%s)", playlist.Name, playlist.ID)
	return nil
}

// UpdatePlaylist 更新播放列表
func (m *Manager) UpdatePlaylist(playlist *Playlist) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.playlists[playlist.ID]
	if !ok {
		return fmt.Errorf("播放列表不存在: %s", playlist.ID)
	}

	playlist.CreatedAt = existing.CreatedAt
	playlist.UpdatedAt = time.Now()
	m.playlists[playlist.ID] = playlist
	m.logger.Info("播放列表更新成功: %s (%s)", playlist.Name, playlist.ID)
	return nil
}

// DeletePlaylist 删除播放列表
func (m *Manager) DeletePlaylist(playlistID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.playlists[playlistID]; !ok {
		return fmt.Errorf("播放列表不存在: %s", playlistID)
	}

	delete(m.playlists, playlistID)
	m.logger.Info("播放列表删除成功: %s", playlistID)
	return nil
}

// GetPlaylist 获取播放列表
func (m *Manager) GetPlaylist(playlistID string) (*Playlist, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	playlist, ok := m.playlists[playlistID]
	if !ok {
		return nil, fmt.Errorf("播放列表不存在: %s", playlistID)
	}
	return playlist, nil
}

// ListPlaylists 列出所有播放列表
func (m *Manager) ListPlaylists() []*Playlist {
	m.mu.RLock()
	defer m.mu.RUnlock()

	playlists := make([]*Playlist, 0, len(m.playlists))
	for _, playlist := range m.playlists {
		playlists = append(playlists, playlist)
	}
	return playlists
}

// ==================== 排程管理 ====================

// CreateSchedule 创建排程
func (m *Manager) CreateSchedule(schedule *Schedule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if schedule.ID == "" {
		schedule.ID = generateID("schedule")
	}
	schedule.Enabled = true
	schedule.CreatedAt = time.Now()
	schedule.UpdatedAt = time.Now()

	m.schedules[schedule.ID] = schedule
	m.logger.Info("排程创建成功: %s (%s)", schedule.Name, schedule.ID)
	return nil
}

// UpdateSchedule 更新排程
func (m *Manager) UpdateSchedule(schedule *Schedule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.schedules[schedule.ID]
	if !ok {
		return fmt.Errorf("排程不存在: %s", schedule.ID)
	}

	schedule.CreatedAt = existing.CreatedAt
	schedule.UpdatedAt = time.Now()
	m.schedules[schedule.ID] = schedule
	m.logger.Info("排程更新成功: %s (%s)", schedule.Name, schedule.ID)
	return nil
}

// DeleteSchedule 删除排程
func (m *Manager) DeleteSchedule(scheduleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.schedules[scheduleID]; !ok {
		return fmt.Errorf("排程不存在: %s", scheduleID)
	}

	delete(m.schedules, scheduleID)
	m.logger.Info("排程删除成功: %s", scheduleID)
	return nil
}

// GetSchedule 获取排程
func (m *Manager) GetSchedule(scheduleID string) (*Schedule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	schedule, ok := m.schedules[scheduleID]
	if !ok {
		return nil, fmt.Errorf("排程不存在: %s", scheduleID)
	}
	return schedule, nil
}

// ListSchedules 列出所有排程
func (m *Manager) ListSchedules() []*Schedule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	schedules := make([]*Schedule, 0, len(m.schedules))
	for _, schedule := range m.schedules {
		schedules = append(schedules, schedule)
	}
	return schedules
}

// UrgentInsert 紧急插播
func (m *Manager) UrgentInsert(playlistID string, deviceIDs []string, duration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	playlist, ok := m.playlists[playlistID]
	if !ok {
		return fmt.Errorf("播放列表不存在: %s", playlistID)
	}

	endTime := time.Now().Add(duration)
	schedule := &Schedule{
		ID:         generateID("urgent"),
		Name:       fmt.Sprintf("紧急插播-%s", playlist.Name),
		PlaylistID: playlistID,
		DeviceIDs:  deviceIDs,
		Type:       ScheduleTypeUrgent,
		Enabled:    true,
		StartTime:  time.Now(),
		EndTime:    &endTime,
		Priority:   100,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	m.schedules[schedule.ID] = schedule
	m.logger.Info("紧急插播创建成功: %s, 设备: %v", playlist.Name, deviceIDs)

	go m.executeSchedule(schedule)
	return nil
}

// ==================== 设备管理 ====================

// RegisterDevice 注册设备
func (m *Manager) RegisterDevice(device *Device) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if device.ID == "" {
		device.ID = generateID("device")
	}
	device.Status = DeviceStatusOnline
	now := time.Now()
	device.LastSeen = &now
	device.CreatedAt = time.Now()
	device.UpdatedAt = time.Now()

	m.devices[device.ID] = device
	m.logger.Info("设备注册成功: %s (%s)", device.Name, device.ID)
	return nil
}

// UpdateDevice 更新设备
func (m *Manager) UpdateDevice(device *Device) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.devices[device.ID]
	if !ok {
		return fmt.Errorf("设备不存在: %s", device.ID)
	}

	device.CreatedAt = existing.CreatedAt
	device.UpdatedAt = time.Now()
	m.devices[device.ID] = device
	m.logger.Info("设备更新成功: %s (%s)", device.Name, device.ID)
	return nil
}

// DeleteDevice 删除设备
func (m *Manager) DeleteDevice(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.devices[deviceID]; !ok {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	delete(m.devices, deviceID)
	delete(m.playback, deviceID)
	m.logger.Info("设备删除成功: %s", deviceID)
	return nil
}

// GetDevice 获取设备
func (m *Manager) GetDevice(deviceID string) (*Device, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, ok := m.devices[deviceID]
	if !ok {
		return nil, fmt.Errorf("设备不存在: %s", deviceID)
	}
	return device, nil
}

// ListDevices 列出所有设备
func (m *Manager) ListDevices(group *string) []*Device {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*Device, 0)
	for _, device := range m.devices {
		if group != nil && device.Group != *group {
			continue
		}
		devices = append(devices, device)
	}
	return devices
}

// CreateDeviceGroup 创建设备组
func (m *Manager) CreateDeviceGroup(group *DeviceGroup) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if group.ID == "" {
		group.ID = generateID("group")
	}
	group.CreatedAt = time.Now()
	group.UpdatedAt = time.Now()

	m.groups[group.ID] = group
	m.logger.Info("设备组创建成功: %s (%s)", group.Name, group.ID)
	return nil
}

// ListDeviceGroups 列出所有设备组
func (m *Manager) ListDeviceGroups() []*DeviceGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groups := make([]*DeviceGroup, 0, len(m.groups))
	for _, group := range m.groups {
		groups = append(groups, group)
	}
	return groups
}

// DeviceHeartbeat 设备心跳
func (m *Manager) DeviceHeartbeat(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.devices[deviceID]
	if !ok {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	now := time.Now()
	device.LastSeen = &now
	device.Status = DeviceStatusOnline
	device.UpdatedAt = now
	return nil
}

// ==================== 模板管理 ====================

// GetTemplate 获取模板
func (m *Manager) GetTemplate(templateID string) (*Template, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tpl, ok := m.templates[templateID]
	if !ok {
		return nil, fmt.Errorf("模板不存在: %s", templateID)
	}
	return tpl, nil
}

// ListTemplates 列出所有模板
func (m *Manager) ListTemplates() []*Template {
	m.mu.RLock()
	defer m.mu.RUnlock()

	templates := make([]*Template, 0, len(m.templates))
	for _, tpl := range m.templates {
		templates = append(templates, tpl)
	}
	return templates
}

// ==================== 播放控制 ====================

// PushToDevice 推送内容到设备
func (m *Manager) PushToDevice(deviceID string, playlistID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.devices[deviceID]
	if !ok {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	playlist, ok := m.playlists[playlistID]
	if !ok {
		return fmt.Errorf("播放列表不存在: %s", playlistID)
	}

	device.CurrentPlaylist = playlistID
	device.UpdatedAt = time.Now()

	m.playback[deviceID] = &PlaybackStatus{
		DeviceID:     deviceID,
		PlaylistID:   playlistID,
		ContentIndex: 0,
		StartedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if len(playlist.Items) > 0 {
		device.CurrentContent = playlist.Items[0].ContentID
		m.playback[deviceID].ContentID = playlist.Items[0].ContentID
	}

	m.logger.Info("推送播放列表到设备: %s -> %s", playlist.Name, device.Name)
	return nil
}

// StopDevice 停止设备播放
func (m *Manager) StopDevice(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.devices[deviceID]
	if !ok {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	device.CurrentPlaylist = ""
	device.CurrentContent = ""
	device.UpdatedAt = time.Now()
	delete(m.playback, deviceID)

	m.logger.Info("停止设备播放: %s", device.Name)
	return nil
}

// GetPlaybackStatus 获取播放状态
func (m *Manager) GetPlaybackStatus(deviceID string) (*PlaybackStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status, ok := m.playback[deviceID]
	if !ok {
		return nil, fmt.Errorf("设备无播放状态: %s", deviceID)
	}
	return status, nil
}

// SetDeviceVolume 设置设备音量
func (m *Manager) SetDeviceVolume(deviceID string, volume int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.devices[deviceID]
	if !ok {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	if volume < 0 || volume > 100 {
		return fmt.Errorf("音量范围 0-100")
	}

	device.Volume = volume
	device.UpdatedAt = time.Now()
	m.logger.Info("设置设备音量: %s -> %d", device.Name, volume)
	return nil
}

// SetDeviceBrightness 设置设备亮度
func (m *Manager) SetDeviceBrightness(deviceID string, brightness int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.devices[deviceID]
	if !ok {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	if brightness < 0 || brightness > 100 {
		return fmt.Errorf("亮度范围 0-100")
	}

	device.Brightness = brightness
	device.UpdatedAt = time.Now()
	m.logger.Info("设置设备亮度: %s -> %d", device.Name, brightness)
	return nil
}

// ==================== 内部调度 ====================

func (m *Manager) scheduleLoop() {
	defer m.wg.Done()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkSchedules()
		}
	}
}

func (m *Manager) checkSchedules() {
	m.mu.RLock()
	schedules := make([]*Schedule, 0)
	for _, schedule := range m.schedules {
		if schedule.Enabled {
			schedules = append(schedules, schedule)
		}
	}
	m.mu.RUnlock()

	now := time.Now()
	for _, schedule := range schedules {
		if now.Before(schedule.StartTime) {
			continue
		}
		if schedule.EndTime != nil && now.After(*schedule.EndTime) {
			continue
		}

		if schedule.Type == ScheduleTypeFixed && now.Sub(schedule.StartTime) < time.Minute {
			go m.executeSchedule(schedule)
		}
	}
}

func (m *Manager) executeSchedule(schedule *Schedule) {
	m.mu.RLock()
	playlist, ok := m.playlists[schedule.PlaylistID]
	if !ok {
		m.mu.RUnlock()
		m.logger.Error("播放列表不存在: %s", schedule.PlaylistID)
		return
	}

	deviceIDs := make([]string, 0)
	if len(schedule.DeviceIDs) > 0 {
		deviceIDs = schedule.DeviceIDs
	} else if schedule.DeviceGroup != "" {
		if group, ok := m.groups[schedule.DeviceGroup]; ok {
			deviceIDs = group.DeviceIDs
		}
	}
	m.mu.RUnlock()

	for _, deviceID := range deviceIDs {
		if err := m.PushToDevice(deviceID, schedule.PlaylistID); err != nil {
			m.logger.Error("推送失败 %s: %v", deviceID, err)
		}
	}

	m.logger.Info("排程执行完成: %s, 播放列表: %s, 设备数: %d", schedule.Name, playlist.Name, len(deviceIDs))
}

func (m *Manager) deviceHealthCheck() {
	defer m.wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkDeviceHealth()
		}
	}
}

func (m *Manager) checkDeviceHealth() {
	m.mu.Lock()
	defer m.mu.Unlock()

	offlineThreshold := 2 * time.Minute
	now := time.Now()

	for _, device := range m.devices {
		if device.LastSeen != nil && now.Sub(*device.LastSeen) > offlineThreshold {
			device.Status = DeviceStatusOffline
		}
	}
}

// Stop 停止管理器
func (m *Manager) Stop() {
	m.cancel()
	m.wg.Wait()
}

// ==================== 工具函数 ====================

func generateID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
