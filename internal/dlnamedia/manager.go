package dlnamedia

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager DLNA 媒体服务器管理器.
type Manager struct {
	mu sync.RWMutex

	// 设备管理
	devices     map[string]*DLNADevice
	deviceGroup map[string]*DeviceGroup

	// 媒体库
	libraries  map[string]*MediaLibrary
	mediaItems map[string]*MediaItem

	// 播放会话
	sessions map[string]*PlaybackSession
	queues   map[string]*PlayQueue

	// SSDP
	ssdpListener *net.UDPConn
	ssdpRunning  bool

	// 配置
	mediaRoot  string
	enableSSDP bool
	scanTicker *time.Ticker
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewManager 创建新的 DLNA 管理器.
func NewManager(mediaRoot string, enableSSDP bool) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		devices:     make(map[string]*DLNADevice),
		deviceGroup: make(map[string]*DeviceGroup),
		libraries:   make(map[string]*MediaLibrary),
		mediaItems:  make(map[string]*MediaItem),
		sessions:    make(map[string]*PlaybackSession),
		queues:      make(map[string]*PlayQueue),
		mediaRoot:   mediaRoot,
		enableSSDP:  enableSSDP,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start 启动管理器.
func (m *Manager) Start() error {
	log.Println("[DLNA] Starting DLNA media manager...")

	if m.enableSSDP {
		if err := m.startSSDP(); err != nil {
			return fmt.Errorf("failed to start SSDP: %w", err)
		}
	}

	// 启动定时扫描
	m.scanTicker = time.NewTicker(5 * time.Minute)
	go m.autoScanLoop()

	log.Println("[DLNA] DLNA media manager started")
	return nil
}

// Stop 停止管理器.
func (m *Manager) Stop() {
	log.Println("[DLNA] Stopping DLNA media manager...")
	m.cancel()

	if m.scanTicker != nil {
		m.scanTicker.Stop()
	}

	if m.ssdpListener != nil {
		m.ssdpListener.Close()
	}

	log.Println("[DLNA] DLNA media manager stopped")
}

// ========== SSDP 设备发现 ==========

// startSSDP 启动 SSDP 监听.
func (m *Manager) startSSDP() error {
	addr, err := net.ResolveUDPAddr("udp4", "239.255.255.250:1900")
	if err != nil {
		return err
	}

	conn, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err != nil {
		return err
	}

	conn.SetReadBuffer(65535)
	m.ssdpListener = conn
	m.ssdpRunning = true

	go m.ssdpListenLoop()

	// 发送搜索请求
	go m.sendSSDPSearch()

	return nil
}

// ssdpListenLoop SSDP 监听循环.
func (m *Manager) ssdpListenLoop() {
	buf := make([]byte, 65535)
	for {
		select {
		case <-m.ctx.Done():
			return
		default:
			n, addr, err := m.ssdpListener.ReadFromUDP(buf)
			if err != nil {
				if m.ctx.Err() != nil {
					return
				}
				continue
			}
			m.handleSSDPMessage(string(buf[:n]), addr)
		}
	}
}

// sendSSDPSearch 发送 SSDP 搜索请求.
func (m *Manager) sendSSDPSearch() {
	msg := "M-SEARCH * HTTP/1.1\r\n" +
		"HOST: 239.255.255.250:1900\r\n" +
		"MAN: \"ssdp:discover\"\r\n" +
		"MX: 3\r\n" +
		"ST: ssdp:all\r\n\r\n"

	addr, _ := net.ResolveUDPAddr("udp4", "239.255.255.250:1900")
	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		log.Printf("[DLNA] Failed to send SSDP search: %v", err)
		return
	}
	defer conn.Close()

	conn.Write([]byte(msg))
}

// handleSSDPMessage 处理 SSDP 消息.
func (m *Manager) handleSSDPMessage(msg string, addr *net.UDPAddr) {
	lines := strings.Split(msg, "\r\n")
	if len(lines) == 0 {
		return
	}

	// 解析响应
	if strings.Contains(lines[0], "200 OK") || strings.Contains(lines[0], "NOTIFY") {
		headers := make(map[string]string)
		for _, line := range lines[1:] {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}

		if location, ok := headers["LOCATION"]; ok {
			m.discoverDevice(location, addr.IP.String())
		}
	}
}

// discoverDevice 从 LOCATION URL 获取设备信息.
func (m *Manager) discoverDevice(location, ip string) {
	// 简化实现：创建基本设备记录
	device := &DLNADevice{
		ID:                  uuid.New().String(),
		UDN:                 fmt.Sprintf("uuid:%s", uuid.New().String()),
		FriendlyName:        fmt.Sprintf("DLNA Device (%s)", ip),
		DeviceType:          DeviceTypeRenderer,
		IPAddress:           ip,
		Location:            location,
		IsOnline:            true,
		LastSeenAt:          time.Now(),
		SupportedMediaTypes: []MediaType{MediaTypeVideo, MediaTypeAudio, MediaTypePhoto},
	}

	m.mu.Lock()
	// 检查是否已存在
	for _, d := range m.devices {
		if d.IPAddress == ip {
			d.LastSeenAt = time.Now()
			d.IsOnline = true
			m.mu.Unlock()
			return
		}
	}
	m.devices[device.ID] = device
	m.mu.Unlock()

	log.Printf("[DLNA] Discovered device: %s (%s)", device.FriendlyName, ip)
}

// DiscoverDevices 手动触发设备发现.
func (m *Manager) DiscoverDevices(timeout int) []*DLNADevice {
	m.sendSSDPSearch()

	// 等待响应
	time.Sleep(time.Duration(timeout) * time.Second)

	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*DLNADevice, 0, len(m.devices))
	for _, d := range m.devices {
		result = append(result, d)
	}
	return result
}

// GetDevice 获取设备.
func (m *Manager) GetDevice(id string) (*DLNADevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, ok := m.devices[id]
	if !ok {
		return nil, fmt.Errorf("device not found: %s", id)
	}
	return device, nil
}

// ListDevices 列出所有设备.
func (m *Manager) ListDevices() []*DLNADevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*DLNADevice, 0, len(m.devices))
	for _, d := range m.devices {
		result = append(result, d)
	}
	return result
}

// ========== 媒体库管理 ==========

// CreateLibrary 创建媒体库.
func (m *Manager) CreateLibrary(req CreateLibraryRequest) (*MediaLibrary, error) {
	if err := m.validatePath(req.Path); err != nil {
		return nil, err
	}

	lib := &MediaLibrary{
		ID:           uuid.New().String(),
		Name:         req.Name,
		Path:         req.Path,
		MediaType:    req.MediaType,
		Recursive:    req.Recursive,
		AutoScan:     req.AutoScan,
		ScanInterval: req.ScanInterval,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	m.mu.Lock()
	m.libraries[lib.ID] = lib
	m.mu.Unlock()

	log.Printf("[DLNA] Created library: %s (%s)", lib.Name, lib.Path)
	return lib, nil
}

// UpdateLibrary 更新媒体库.
func (m *Manager) UpdateLibrary(id string, req UpdateLibraryRequest) (*MediaLibrary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lib, ok := m.libraries[id]
	if !ok {
		return nil, fmt.Errorf("library not found: %s", id)
	}

	if req.Name != nil {
		lib.Name = *req.Name
	}
	if req.Recursive != nil {
		lib.Recursive = *req.Recursive
	}
	if req.AutoScan != nil {
		lib.AutoScan = *req.AutoScan
	}
	if req.ScanInterval != nil {
		lib.ScanInterval = *req.ScanInterval
	}
	lib.UpdatedAt = time.Now()

	return lib, nil
}

// DeleteLibrary 删除媒体库.
func (m *Manager) DeleteLibrary(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.libraries[id]; !ok {
		return fmt.Errorf("library not found: %s", id)
	}

	// 删除关联的媒体项
	for mediaID, item := range m.mediaItems {
		if item.LibraryID == id {
			delete(m.mediaItems, mediaID)
		}
	}

	delete(m.libraries, id)
	log.Printf("[DLNA] Deleted library: %s", id)
	return nil
}

// GetLibrary 获取媒体库.
func (m *Manager) GetLibrary(id string) (*MediaLibrary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lib, ok := m.libraries[id]
	if !ok {
		return nil, fmt.Errorf("library not found: %s", id)
	}
	return lib, nil
}

// ListLibraries 列出所有媒体库.
func (m *Manager) ListLibraries() []*MediaLibrary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*MediaLibrary, 0, len(m.libraries))
	for _, lib := range m.libraries {
		result = append(result, lib)
	}
	return result
}

// ScanLibrary 扫描媒体库.
func (m *Manager) ScanLibrary(libraryID string, force bool) error {
	m.mu.RLock()
	lib, ok := m.libraries[libraryID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("library not found: %s", libraryID)
	}

	log.Printf("[DLNA] Scanning library: %s (force=%v)", lib.Name, force)

	now := time.Now()
	err := filepath.Walk(lib.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过错误
		}

		if info.IsDir() {
			if !lib.Recursive && path != lib.Path {
				return filepath.SkipDir
			}
			return nil
		}

		mediaType := m.detectMediaType(path)
		if mediaType == "" {
			return nil
		}

		// 检查是否已存在
		m.mu.RLock()
		for _, item := range m.mediaItems {
			if item.FilePath == path {
				m.mu.RUnlock()
				return nil
			}
		}
		m.mu.RUnlock()

		item := &MediaItem{
			ID:         uuid.New().String(),
			Title:      info.Name(),
			FilePath:   path,
			MediaType:  mediaType,
			Size:       info.Size(),
			LibraryID:  libraryID,
			FolderPath: filepath.Dir(path),
			CreatedAt:  now,
			UpdatedAt:  now,
			ScannedAt:  now,
		}

		m.mu.Lock()
		m.mediaItems[item.ID] = item
		lib.ItemCount++
		m.mu.Unlock()

		return nil
	})

	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	m.mu.Lock()
	lib.LastScanAt = &now
	lib.UpdatedAt = now
	m.mu.Unlock()

	log.Printf("[DLNA] Library scan complete: %d items", lib.ItemCount)
	return nil
}

// autoScanLoop 自动扫描循环.
func (m *Manager) autoScanLoop() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-m.scanTicker.C:
			m.autoScan()
		}
	}
}

// autoScan 自动扫描启用了自动扫描的媒体库.
func (m *Manager) autoScan() {
	m.mu.RLock()
	libraries := make([]*MediaLibrary, 0)
	for _, lib := range m.libraries {
		if lib.AutoScan {
			libraries = append(libraries, lib)
		}
	}
	m.mu.RUnlock()

	for _, lib := range libraries {
		if lib.LastScanAt == nil || time.Since(*lib.LastScanAt) > time.Duration(lib.ScanInterval)*time.Minute {
			if err := m.ScanLibrary(lib.ID, false); err != nil {
				log.Printf("[DLNA] Auto scan failed for %s: %v", lib.Name, err)
			}
		}
	}
}

// detectMediaType 检测媒体类型.
func (m *Manager) detectMediaType(path string) MediaType {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".ts":
		return MediaTypeVideo
	case ".mp3", ".flac", ".wav", ".aac", ".ogg", ".wma", ".m4a", ".opus":
		return MediaTypeAudio
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".svg":
		return MediaTypePhoto
	default:
		return ""
	}
}

// validatePath 验证路径.
func (m *Manager) validatePath(path string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	// 检查路径是否存在
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("path does not exist: %s", path)
		}
		return fmt.Errorf("cannot access path: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}

	return nil
}

// ========== 媒体搜索 ==========

// SearchMedia 搜索媒体.
func (m *Manager) SearchMedia(req SearchMediaRequest) ([]*MediaItem, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*MediaItem
	for _, item := range m.mediaItems {
		if !m.matchesSearch(item, req) {
			continue
		}
		results = append(results, item)
	}

	// 排序
	m.sortMedia(results, req.SortBy, req.SortOrder)

	total := len(results)

	// 分页
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	start := (req.Page - 1) * req.PageSize
	if start >= len(results) {
		return nil, total
	}

	end := start + req.PageSize
	if end > len(results) {
		end = len(results)
	}

	return results[start:end], total
}

// matchesSearch 检查是否匹配搜索条件.
func (m *Manager) matchesSearch(item *MediaItem, req SearchMediaRequest) bool {
	if req.Query != "" {
		query := strings.ToLower(req.Query)
		if !strings.Contains(strings.ToLower(item.Title), query) {
			return false
		}
	}

	if req.MediaType != "" && item.MediaType != req.MediaType {
		return false
	}

	if req.LibraryID != "" && item.LibraryID != req.LibraryID {
		return false
	}

	if req.Tags != "" {
		tags := strings.Split(req.Tags, ",")
		found := false
		for _, tag := range tags {
			for _, itemTag := range item.Tags {
				if strings.TrimSpace(tag) == itemTag {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// sortMedia 排序媒体.
func (m *Manager) sortMedia(items []*MediaItem, sortBy, sortOrder string) {
	// 简化实现：使用标准库排序
	// 实际实现需要更复杂的排序逻辑
}

// GetMediaItem 获取媒体项.
func (m *Manager) GetMediaItem(id string) (*MediaItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, ok := m.mediaItems[id]
	if !ok {
		return nil, fmt.Errorf("media item not found: %s", id)
	}
	return item, nil
}

// ========== 投屏和播放控制 ==========

// PushMedia 推送媒体到设备播放.
func (m *Manager) PushMedia(req PushMediaRequest) (*PlaybackSession, error) {
	m.mu.RLock()
	device, deviceOk := m.devices[req.DeviceID]
	item, itemOk := m.mediaItems[req.MediaID]
	m.mu.RUnlock()

	if !deviceOk {
		return nil, fmt.Errorf("device not found: %s", req.DeviceID)
	}
	if !itemOk {
		return nil, fmt.Errorf("media item not found: %s", req.MediaID)
	}

	if !device.IsOnline {
		return nil, fmt.Errorf("device is offline: %s", device.FriendlyName)
	}

	// 创建播放会话
	session := &PlaybackSession{
		ID:          uuid.New().String(),
		DeviceID:    req.DeviceID,
		CurrentItem: item,
		State:       PlayStatePlaying,
		Position:    req.Position,
		Duration:    item.Duration,
		Volume:      50,
		StartedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.mu.Lock()
	m.sessions[session.ID] = session
	m.mu.Unlock()

	log.Printf("[DLNA] Pushed media %s to device %s", item.Title, device.FriendlyName)
	return session, nil
}

// ControlPlayback 控制播放.
func (m *Manager) ControlPlayback(sessionID string, req ControlPlaybackRequest) (*PlaybackSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	switch req.Action {
	case "play":
		session.State = PlayStatePlaying
	case "pause":
		session.State = PlayStatePaused
	case "stop":
		session.State = PlayStateStopped
		session.Position = 0
	case "seek":
		session.Position = req.Position
	case "next":
		// TODO: 从队列中获取下一个
	case "prev":
		// TODO: 从队列中获取上一个
	default:
		return nil, fmt.Errorf("unknown action: %s", req.Action)
	}

	session.UpdatedAt = time.Now()
	return session, nil
}

// SetVolume 设置音量.
func (m *Manager) SetVolume(sessionID string, level int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if level < 0 || level > 100 {
		return fmt.Errorf("volume must be between 0 and 100")
	}

	session.Volume = level
	session.UpdatedAt = time.Now()
	return nil
}

// GetPlaybackStatus 获取播放状态.
func (m *Manager) GetPlaybackStatus(sessionID string) (*PlaybackSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	return session, nil
}

// ListSessions 列出所有播放会话.
func (m *Manager) ListSessions() []*PlaybackSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*PlaybackSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, s)
	}
	return result
}

// StopSession 停止播放会话.
func (m *Manager) StopSession(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.State = PlayStateStopped
	session.UpdatedAt = time.Now()

	delete(m.sessions, sessionID)
	return nil
}

// ========== 播放队列 ==========

// ManageQueue 管理播放队列.
func (m *Manager) ManageQueue(deviceID string, req ManageQueueRequest) (*PlayQueue, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	queue, ok := m.queues[deviceID]
	if !ok {
		queue = &PlayQueue{
			ID:        uuid.New().String(),
			DeviceID:  deviceID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		m.queues[deviceID] = queue
	}

	switch req.Action {
	case "add":
		for _, mediaID := range req.MediaIDs {
			item := QueueItem{
				Index:   len(queue.Items),
				MediaID: mediaID,
				AddedAt: time.Now(),
			}
			queue.Items = append(queue.Items, item)
		}
	case "remove":
		if req.Index >= 0 && req.Index < len(queue.Items) {
			queue.Items = append(queue.Items[:req.Index], queue.Items[req.Index+1:]...)
			// 重新编号
			for i := range queue.Items {
				queue.Items[i].Index = i
			}
		}
	case "clear":
		queue.Items = nil
		queue.CurrentIndex = 0
	case "reorder":
		if req.Index >= 0 && req.Index < len(queue.Items) &&
			req.TargetIndex >= 0 && req.TargetIndex < len(queue.Items) {
			item := queue.Items[req.Index]
			queue.Items = append(queue.Items[:req.Index], queue.Items[req.Index+1:]...)
			queue.Items = append(queue.Items[:req.TargetIndex], append([]QueueItem{item}, queue.Items[req.TargetIndex:]...)...)
			// 重新编号
			for i := range queue.Items {
				queue.Items[i].Index = i
			}
		}
	default:
		return nil, fmt.Errorf("unknown queue action: %s", req.Action)
	}

	queue.UpdatedAt = time.Now()
	return queue, nil
}

// GetQueue 获取播放队列.
func (m *Manager) GetQueue(deviceID string) (*PlayQueue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	queue, ok := m.queues[deviceID]
	if !ok {
		return nil, fmt.Errorf("queue not found for device: %s", deviceID)
	}
	return queue, nil
}

// ========== 设备分组 ==========

// CreateGroup 创建设备分组.
func (m *Manager) CreateGroup(req CreateGroupRequest) (*DeviceGroup, error) {
	// 验证设备存在
	m.mu.RLock()
	for _, deviceID := range req.DeviceIDs {
		if _, ok := m.devices[deviceID]; !ok {
			m.mu.RUnlock()
			return nil, fmt.Errorf("device not found: %s", deviceID)
		}
	}
	m.mu.RUnlock()

	group := &DeviceGroup{
		ID:        uuid.New().String(),
		Name:      req.Name,
		DeviceIDs: req.DeviceIDs,
		IsSync:    req.IsSync,
		Volume:    50,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.mu.Lock()
	m.deviceGroup[group.ID] = group

	// 更新设备的 GroupID
	for _, deviceID := range req.DeviceIDs {
		if device, ok := m.devices[deviceID]; ok {
			device.GroupID = group.ID
		}
	}
	m.mu.Unlock()

	log.Printf("[DLNA] Created device group: %s with %d devices", group.Name, len(group.DeviceIDs))
	return group, nil
}

// UpdateGroup 更新设备分组.
func (m *Manager) UpdateGroup(groupID string, req UpdateGroupRequest) (*DeviceGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, ok := m.deviceGroup[groupID]
	if !ok {
		return nil, fmt.Errorf("group not found: %s", groupID)
	}

	if req.Name != nil {
		group.Name = *req.Name
	}
	if req.DeviceIDs != nil {
		// 清除旧设备的 GroupID
		for _, deviceID := range group.DeviceIDs {
			if device, ok := m.devices[deviceID]; ok {
				device.GroupID = ""
			}
		}

		group.DeviceIDs = req.DeviceIDs

		// 设置新设备的 GroupID
		for _, deviceID := range req.DeviceIDs {
			if device, ok := m.devices[deviceID]; ok {
				device.GroupID = group.ID
			}
		}
	}
	if req.IsSync != nil {
		group.IsSync = *req.IsSync
	}
	if req.Volume != nil {
		group.Volume = *req.Volume
	}

	group.UpdatedAt = time.Now()
	return group, nil
}

// DeleteGroup 删除设备分组.
func (m *Manager) DeleteGroup(groupID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, ok := m.deviceGroup[groupID]
	if !ok {
		return fmt.Errorf("group not found: %s", groupID)
	}

	// 清除设备的 GroupID
	for _, deviceID := range group.DeviceIDs {
		if device, ok := m.devices[deviceID]; ok {
			device.GroupID = ""
		}
	}

	delete(m.deviceGroup, groupID)
	log.Printf("[DLNA] Deleted device group: %s", group.Name)
	return nil
}

// GetGroup 获取设备分组.
func (m *Manager) GetGroup(groupID string) (*DeviceGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	group, ok := m.deviceGroup[groupID]
	if !ok {
		return nil, fmt.Errorf("group not found: %s", groupID)
	}
	return group, nil
}

// ListGroups 列出所有设备分组.
func (m *Manager) ListGroups() []*DeviceGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*DeviceGroup, 0, len(m.deviceGroup))
	for _, g := range m.deviceGroup {
		result = append(result, g)
	}
	return result
}

// ========== 内容目录 ==========

// GetContentDirectory 获取内容目录.
func (m *Manager) GetContentDirectory(parentID string) []*ContentDirectoryItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var items []*ContentDirectoryItem

	if parentID == "" || parentID == "0" {
		// 根目录：返回媒体库
		for _, lib := range m.libraries {
			items = append(items, &ContentDirectoryItem{
				ID:          lib.ID,
				ParentID:    "0",
				Title:       lib.Name,
				IsContainer: true,
				ItemCount:   lib.ItemCount,
			})
		}
	} else {
		// 库目录：返回媒体项
		for _, item := range m.mediaItems {
			if item.LibraryID == parentID {
				items = append(items, &ContentDirectoryItem{
					ID:        item.ID,
					ParentID:  parentID,
					Title:     item.Title,
					MediaItem: item,
				})
			}
		}
	}

	return items
}
