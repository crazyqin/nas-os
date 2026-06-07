// Package gamepreloader 提供游戏资源预加速功能
// 学习飞牛 fnOS 游戏资源预下载特性：
// - NAS 端预缓存游戏资源包
// - 局域网设备自动发现和推送
// - 智能调度（低带宽时段预下载）
// - 游戏更新自动检测和增量同步
// - 多平台支持（PC/手机/主机）
package gamepreloader

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Platform 游戏平台
type Platform string

const (
	PlatformPC      Platform = "pc"
	PlatformMobile  Platform = "mobile"
	PlatformConsole Platform = "console"
	PlatformSteam   Platform = "steam"
	PlatformEpic    Platform = "epic"
	PlatformWeGame  Platform = "wegame"
	PlatformPS5     Platform = "ps5"
	PlatformXbox    Platform = "xbox"
	PlatformSwitch  Platform = "switch"
)

// GameStatus 游戏状态
type GameStatus string

const (
	StatusIdle       GameStatus = "idle"
	StatusPreloading GameStatus = "preloading"
	StatusReady      GameStatus = "ready"
	StatusSyncing    GameStatus = "syncing"
	StatusError      GameStatus = "error"
)

// ScheduleMode 调度模式
type ScheduleMode string

const (
	ScheduleImmediate ScheduleMode = "immediate" // 立即下载
	ScheduleOffPeak   ScheduleMode = "off_peak"  // 低峰时段
	ScheduleManual    ScheduleMode = "manual"    // 手动触发
	ScheduleSmart     ScheduleMode = "smart"     // AI 智能调度
)

// DeviceType 设备类型
type DeviceType string

const (
	DevicePC     DeviceType = "pc"
	DevicePhone  DeviceType = "phone"
	DeviceTablet DeviceType = "tablet"
	DeviceTV     DeviceType = "tv"
)

// Game 游戏信息
type Game struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Platform    Platform   `json:"platform"`
	Size        int64      `json:"size"` // 字节
	Version     string     `json:"version"`
	Status      GameStatus `json:"status"`
	PreloadPath string     `json:"preloadPath"` // NAS 存储路径
	SourceURL   string     `json:"sourceUrl"`   // 下载源
	LastSync    time.Time  `json:"lastSync"`
	NextSync    time.Time  `json:"nextSync"`
	Progress    float64    `json:"progress"` // 0-100
	Speed       int64      `json:"speed"`    // bytes/sec
	ETag        string     `json:"etag"`     // 用于增量检测
	IconURL     string     `json:"iconUrl"`
	Tags        []string   `json:"tags"`
	PlayCount   int        `json:"playCount"`
	LastPlay    time.Time  `json:"lastPlay"`
	Priority    int        `json:"priority"` // 1-10, 越高越优先
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// Device 局域网设备
type Device struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Type      DeviceType `json:"type"`
	IP        string     `json:"ip"`
	MAC       string     `json:"mac"`
	Platform  Platform   `json:"platform"`
	Online    bool       `json:"online"`
	LastSeen  time.Time  `json:"lastSeen"`
	Bandwidth int64      `json:"bandwidth"` // 可用带宽 bytes/sec
	Games     []string   `json:"games"`     // 已安装的游戏 ID
}

// PreloadTask 预加载任务
type PreloadTask struct {
	ID          string       `json:"id"`
	GameID      string       `json:"gameId"`
	DeviceID    string       `json:"deviceId,omitempty"` // 空表示仅 NAS 缓存
	Status      GameStatus   `json:"status"`
	Progress    float64      `json:"progress"`
	Speed       int64        `json:"speed"`
	TotalBytes  int64        `json:"totalBytes"`
	LoadedBytes int64        `json:"loadedBytes"`
	Schedule    ScheduleMode `json:"schedule"`
	StartTime   time.Time    `json:"startTime"`
	EndTime     time.Time    `json:"endTime"`
	Error       string       `json:"error,omitempty"`
}

// ScheduleConfig 调度配置
type ScheduleConfig struct {
	Mode          ScheduleMode `json:"mode"`
	OffPeakStart  string       `json:"offPeakStart"`  // "02:00"
	OffPeakEnd    string       `json:"offPeakEnd"`    // "06:00"
	MaxBandwidth  int64        `json:"maxBandwidth"`  // 最大带宽限制
	MaxConcurrent int          `json:"maxConcurrent"` // 最大并发数
	SmartEnabled  bool         `json:"smartEnabled"`  // AI 智能调度
}

// Manager 预加载管理器
type Manager struct {
	mu       sync.RWMutex
	config   *Config
	games    map[string]*Game
	devices  map[string]*Device
	tasks    []*PreloadTask
	schedule *ScheduleConfig
	stopCh   chan struct{}
}

// Config 管理器配置
type Config struct {
	Enabled             bool            `json:"enabled"`
	StoragePath         string          `json:"storagePath"` // NAS 存储根路径
	MaxStorage          int64           `json:"maxStorage"`  // 最大存储空间
	ScheduleConfig      *ScheduleConfig `json:"scheduleConfig"`
	AutoDiscovery       bool            `json:"autoDiscovery"`       // 自动发现设备
	UpdateCheckInterval time.Duration   `json:"updateCheckInterval"` // 更新检查间隔
}

// NewManager 创建管理器
func NewManager(config *Config) *Manager {
	if config.ScheduleConfig == nil {
		config.ScheduleConfig = &ScheduleConfig{
			Mode:          ScheduleOffPeak,
			OffPeakStart:  "02:00",
			OffPeakEnd:    "06:00",
			MaxBandwidth:  50 * 1024 * 1024, // 50MB/s
			MaxConcurrent: 3,
		}
	}
	if config.UpdateCheckInterval == 0 {
		config.UpdateCheckInterval = 1 * time.Hour
	}
	return &Manager{
		config:   config,
		games:    make(map[string]*Game),
		devices:  make(map[string]*Device),
		tasks:    make([]*PreloadTask, 0),
		schedule: config.ScheduleConfig,
		stopCh:   make(chan struct{}),
	}
}

// Start 启动管理器
func (m *Manager) Start() error {
	if !m.config.Enabled {
		return nil
	}
	go m.updateCheckLoop()
	go m.deviceDiscoveryLoop()
	go m.schedulerLoop()
	return nil
}

// Stop 停止管理器
func (m *Manager) Stop() {
	close(m.stopCh)
}

// AddGame 添加游戏
func (m *Manager) AddGame(game *Game) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if game.ID == "" {
		game.ID = fmt.Sprintf("game-%d", time.Now().UnixNano())
	}
	if game.Status == "" {
		game.Status = StatusIdle
	}
	if game.Priority == 0 {
		game.Priority = 5
	}
	game.CreatedAt = time.Now()
	game.UpdatedAt = time.Now()

	m.games[game.ID] = game
	return nil
}

// RemoveGame 移除游戏
func (m *Manager) RemoveGame(gameID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.games[gameID]; !ok {
		return fmt.Errorf("game %s not found", gameID)
	}
	delete(m.games, gameID)
	return nil
}

// RegisterDevice 注册设备
func (m *Manager) RegisterDevice(device *Device) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if device.ID == "" {
		device.ID = fmt.Sprintf("device-%d", time.Now().UnixNano())
	}
	device.LastSeen = time.Now()
	device.Online = true

	m.devices[device.ID] = device
	return nil
}

// StartPreload 启动预加载
func (m *Manager) StartPreload(gameID, deviceID string, schedule ScheduleMode) (*PreloadTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	game, ok := m.games[gameID]
	if !ok {
		return nil, fmt.Errorf("game %s not found", gameID)
	}

	task := &PreloadTask{
		ID:         fmt.Sprintf("task-%d", time.Now().UnixNano()),
		GameID:     gameID,
		DeviceID:   deviceID,
		Status:     StatusPreloading,
		Schedule:   schedule,
		TotalBytes: game.Size,
		StartTime:  time.Now(),
	}

	m.tasks = append(m.tasks, task)
	game.Status = StatusPreloading

	// 异步执行预加载
	go m.executePreload(task)

	return task, nil
}

// GetGame 获取游戏信息
func (m *Manager) GetGame(gameID string) (*Game, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	game, ok := m.games[gameID]
	if !ok {
		return nil, fmt.Errorf("game %s not found", gameID)
	}
	return game, nil
}

// ListGames 列出所有游戏
func (m *Manager) ListGames() []*Game {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Game, 0, len(m.games))
	for _, g := range m.games {
		result = append(result, g)
	}
	return result
}

// ListDevices 列出所有设备
func (m *Manager) ListDevices() []*Device {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Device, 0, len(m.devices))
	for _, d := range m.devices {
		result = append(result, d)
	}
	return result
}

// GetTasks 获取任务列表
func (m *Manager) GetTasks() []*PreloadTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*PreloadTask, len(m.tasks))
	copy(result, m.tasks)
	return result
}

// GetStorageUsage 获取存储使用情况
func (m *Manager) GetStorageUsage() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var totalUsed int64
	gameCount := 0
	for _, g := range m.games {
		if g.Status == StatusReady {
			totalUsed += g.Size
			gameCount++
		}
	}

	return map[string]interface{}{
		"usedBytes":    totalUsed,
		"maxBytes":     m.config.MaxStorage,
		"usagePercent": float64(totalUsed) / float64(m.config.MaxStorage) * 100,
		"gameCount":    gameCount,
		"totalGames":   len(m.games),
	}
}

// GetSmartRecommendations 获取智能推荐
func (m *Manager) GetSmartRecommendations() []map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	recommendations := make([]map[string]interface{}, 0)

	// 基于游玩频率推荐预加载
	for _, g := range m.games {
		if g.Status == StatusIdle && g.PlayCount > 5 {
			recommendations = append(recommendations, map[string]interface{}{
				"gameId":    g.ID,
				"gameName":  g.Name,
				"reason":    fmt.Sprintf("高频游戏（已玩 %d 次），建议预缓存", g.PlayCount),
				"priority":  "high",
				"sizeBytes": g.Size,
			})
		}
	}

	// 基于最近游玩时间推荐
	for _, g := range m.games {
		if g.Status == StatusIdle && !g.LastPlay.IsZero() {
			daysSincePlay := time.Since(g.LastPlay).Hours() / 24
			if daysSincePlay < 7 {
				recommendations = append(recommendations, map[string]interface{}{
					"gameId":    g.ID,
					"gameName":  g.Name,
					"reason":    fmt.Sprintf("%.0f 天前玩过，可能近期还会玩", daysSincePlay),
					"priority":  "medium",
					"sizeBytes": g.Size,
				})
			}
		}
	}

	return recommendations
}

func (m *Manager) executePreload(task *PreloadTask) {
	// 模拟预加载过程
	game, ok := m.games[task.GameID]
	if !ok {
		return
	}

	// 模拟进度更新
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for task.Progress < 100 {
		select {
		case <-m.stopCh:
			task.Status = StatusError
			task.Error = "任务被取消"
			return
		case <-ticker.C:
			task.Progress += 5
			task.LoadedBytes = int64(float64(task.TotalBytes) * task.Progress / 100)
			task.Speed = 10 * 1024 * 1024 // 10MB/s 模拟速度

			if task.Progress >= 100 {
				task.Progress = 100
				task.Status = StatusReady
				task.EndTime = time.Now()
				game.Status = StatusReady
				game.Progress = 100
				game.LastSync = time.Now()
			}
		}
	}
}

func (m *Manager) updateCheckLoop() {
	ticker := time.NewTicker(m.config.UpdateCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkUpdates()
		}
	}
}

func (m *Manager) checkUpdates() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, game := range m.games {
		if game.SourceURL != "" && game.Status == StatusReady {
			// 检查更新（HEAD 请求 ETag）
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Head(game.SourceURL)
			if err == nil {
				resp.Body.Close()
				newETag := resp.Header.Get("ETag")
				if newETag != "" && newETag != game.ETag {
					game.Status = StatusSyncing
					game.ETag = newETag
					// 触发增量同步
					task := &PreloadTask{
						ID:         fmt.Sprintf("sync-%d", time.Now().UnixNano()),
						GameID:     game.ID,
						Status:     StatusSyncing,
						Schedule:   ScheduleImmediate,
						TotalBytes: game.Size,
						StartTime:  time.Now(),
					}
					m.tasks = append(m.tasks, task)
				}
			}
		}
	}
}

func (m *Manager) deviceDiscoveryLoop() {
	if !m.config.AutoDiscovery {
		return
	}
	// 设备发现逻辑（mDNS/SSDP）
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			// 扫描局域网设备
		}
	}
}

func (m *Manager) schedulerLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.scheduleCheck()
		}
	}
}

func (m *Manager) scheduleCheck() {
	if m.schedule.Mode != ScheduleOffPeak {
		return
	}

	now := time.Now()
	currentTime := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())

	if currentTime >= m.schedule.OffPeakStart && currentTime <= m.schedule.OffPeakEnd {
		// 在低峰时段内，执行待处理的任务
		m.mu.Lock()
		defer m.mu.Unlock()

		running := 0
		for _, task := range m.tasks {
			if task.Status == StatusPreloading {
				running++
			}
		}

		if running < m.schedule.MaxConcurrent {
			for _, task := range m.tasks {
				if task.Status == StatusIdle && task.Schedule == ScheduleOffPeak {
					task.Status = StatusPreloading
					task.StartTime = time.Now()
					go m.executePreload(task)
					running++
					if running >= m.schedule.MaxConcurrent {
						break
					}
				}
			}
		}
	}
}

// Handler HTTP 处理器
type Handler struct {
	manager *Manager
}

func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	group := router.Group("/game-preloader")
	{
		// 游戏管理
		group.POST("/games", h.AddGame)
		group.GET("/games", h.ListGames)
		group.GET("/games/:id", h.GetGame)
		group.DELETE("/games/:id", h.RemoveGame)

		// 预加载
		group.POST("/preload/:gameId", h.StartPreload)
		group.GET("/tasks", h.GetTasks)

		// 设备
		group.POST("/devices", h.RegisterDevice)
		group.GET("/devices", h.ListDevices)

		// 智能推荐
		group.GET("/recommendations", h.GetRecommendations)
		group.GET("/storage", h.GetStorageUsage)
	}
}

func (h *Handler) AddGame(c *gin.Context) {
	var game Game
	if err := c.ShouldBindJSON(&game); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if err := h.manager.AddGame(&game); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": game})
}

func (h *Handler) ListGames(c *gin.Context) {
	games := h.manager.ListGames()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": games, "total": len(games)})
}

func (h *Handler) GetGame(c *gin.Context) {
	id := c.Param("id")
	game, err := h.manager.GetGame(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": game})
}

func (h *Handler) RemoveGame(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.RemoveGame(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "游戏已移除"})
}

func (h *Handler) StartPreload(c *gin.Context) {
	gameID := c.Param("gameId")
	var req struct {
		DeviceID string       `json:"deviceId"`
		Schedule ScheduleMode `json:"schedule"`
	}
	c.ShouldBindJSON(&req)
	if req.Schedule == "" {
		req.Schedule = ScheduleImmediate
	}

	task, err := h.manager.StartPreload(gameID, req.DeviceID, req.Schedule)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": task})
}

func (h *Handler) GetTasks(c *gin.Context) {
	tasks := h.manager.GetTasks()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": tasks, "total": len(tasks)})
}

func (h *Handler) RegisterDevice(c *gin.Context) {
	var device Device
	if err := c.ShouldBindJSON(&device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if err := h.manager.RegisterDevice(&device); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": device})
}

func (h *Handler) ListDevices(c *gin.Context) {
	devices := h.manager.ListDevices()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": devices, "total": len(devices)})
}

func (h *Handler) GetRecommendations(c *gin.Context) {
	recs := h.manager.GetSmartRecommendations()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": recs, "total": len(recs)})
}

func (h *Handler) GetStorageUsage(c *gin.Context) {
	usage := h.manager.GetStorageUsage()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": usage})
}
