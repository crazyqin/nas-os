// Package gameserver 提供游戏服务器托管管理功能
// 支持多游戏服务器管理、自动备份、性能监控、模组管理、玩家管理等
package gameserver

import "time"

// GameType 游戏类型
type GameType string

const (
	GameMinecraft      GameType = "minecraft"
	GameValheim        GameType = "valheim"
	GameFactorio       GameType = "factorio"
	GameSatisfactory   GameType = "satisfactory"
	GamePalworld       GameType = "palworld"
	GameRust           GameType = "rust"
	GameTerraria       GameType = "terraria"
	GameARK            GameType = "ark"
	GameCS2            GameType = "cs2"
	GameGarryMod       GameType = "gmod"
	GameTeamFortress   GameType = "tf2"
	GameLeft4Dead      GameType = "l4d2"
	GameDontStarve     GameType = "dst"
	GameProjectZomboid GameType = "pz"
	GameUnturned       GameType = "unturned"
	GameSevenDays      GameType = "7d2d"
	GameConanExiles    GameType = "conan"
	GameCustom         GameType = "custom"
)

// ServerStatus 服务器状态
type ServerStatus string

const (
	StatusStopped   ServerStatus = "stopped"
	StatusStarting  ServerStatus = "starting"
	StatusRunning   ServerStatus = "running"
	StatusStopping  ServerStatus = "stopping"
	StatusCrashed   ServerStatus = "crashed"
	StatusUpdating  ServerStatus = "updating"
	StatusBackingUp ServerStatus = "backing_up"
	StatusError     ServerStatus = "error"
)

// ModSource 模组来源
type ModSource string

const (
	ModSourceSteam      ModSource = "steam_workshop"
	ModSourceCurseForge ModSource = "curseforge"
	ModSourceModrinth   ModSource = "modrinth"
	ModSourceCustom     ModSource = "custom"
)

// GameServer 游戏服务器配置
type GameServer struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	GameType       GameType          `json:"game_type"`
	Version        string            `json:"version"`
	Status         ServerStatus      `json:"status"`
	IP             string            `json:"ip"`
	Port           int               `json:"port"`
	QueryPort      int               `json:"query_port,omitempty"`
	RCONPort       int               `json:"rcon_port,omitempty"`
	MaxPlayers     int               `json:"max_players"`
	CurrentPlayers int               `json:"current_players"`
	Map            string            `json:"map,omitempty"`
	Gamemode       string            `json:"gamemode,omitempty"`
	Difficulty     string            `json:"difficulty,omitempty"`
	Password       string            `json:"password,omitempty"`
	IsPublic       bool              `json:"is_public"`
	AutoStart      bool              `json:"auto_start"`
	AutoUpdate     bool              `json:"auto_update"`
	AutoBackup     bool              `json:"auto_backup"`
	BackupInterval int               `json:"backup_interval"`
	MaxBackups     int               `json:"max_backups"`
	WorkingDir     string            `json:"working_dir"`
	ExecPath       string            `json:"exec_path"`
	LaunchArgs     string            `json:"launch_args"`
	Environment    map[string]string `json:"environment,omitempty"`
	ResourceLimits ResourceLimits    `json:"resource_limits"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	LastStarted    *time.Time        `json:"last_started,omitempty"`
	LastStopped    *time.Time        `json:"last_stopped,omitempty"`
	Uptime         int64             `json:"uptime"`
}

// ResourceLimits 资源限制
type ResourceLimits struct {
	MaxMemoryMB    int `json:"max_memory_mb"`
	MaxCPU         int `json:"max_cpu"`
	MaxDiskGB      int `json:"max_disk_gb"`
	MaxBandwidthMB int `json:"max_bandwidth_mb"`
}

// Player 玩家信息
type Player struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	SteamID     string    `json:"steam_id,omitempty"`
	IP          string    `json:"ip"`
	Connected   bool      `json:"connected"`
	PlayTime    int64     `json:"play_time"`
	LastSeen    time.Time `json:"last_seen"`
	FirstJoined time.Time `json:"firstjoined"`
	IsBanned    bool      `json:"is_banned"`
	IsAdmin     bool      `json:"is_admin"`
	Notes       string    `json:"notes,omitempty"`
}

// Mod 模组信息
type Mod struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Version      string    `json:"version"`
	Author       string    `json:"author"`
	Source       ModSource `json:"source"`
	SourceID     string    `json:"source_id"`
	Enabled      bool      `json:"enabled"`
	InstalledAt  time.Time `json:"installed_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	FileSize     int64     `json:"file_size"`
	Dependencies []string  `json:"dependencies,omitempty"`
}

// WorldSave 世界存档
type WorldSave struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Size        int64     `json:"size"`
	CreatedAt   time.Time `json:"created_at"`
	IsAuto      bool      `json:"is_auto"`
	Checksum    string    `json:"checksum"`
}

// ServerSchedule 定时任务
type ServerSchedule struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Type     string     `json:"type"`
	CronExpr string     `json:"cron_expr"`
	Command  string     `json:"command"`
	Enabled  bool       `json:"enabled"`
	LastRun  *time.Time `json:"last_run,omitempty"`
	NextRun  *time.Time `json:"next_run,omitempty"`
}

// ServerMetrics 服务器指标
type ServerMetrics struct {
	ServerID    string    `json:"server_id"`
	Timestamp   time.Time `json:"timestamp"`
	CPUUsage    float64   `json:"cpu_usage"`
	MemoryUsage float64   `json:"memory_usage"`
	DiskUsage   float64   `json:"disk_usage"`
	NetworkIn   int64     `json:"network_in"`
	NetworkOut  int64     `json:"network_out"`
	Players     int       `json:"players"`
	TPS         float64   `json:"tps,omitempty"`
	WorldSize   int64     `json:"world_size"`
}

// ServerAlert 服务器告警
type ServerAlert struct {
	ID        string    `json:"id"`
	ServerID  string    `json:"server_id"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Severity  string    `json:"severity"`
	Timestamp time.Time `json:"timestamp"`
	Resolved  bool      `json:"resolved"`
}

// ServerTemplate 服务器模板
type ServerTemplate struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	GameType    GameType          `json:"game_type"`
	Version     string            `json:"version"`
	LaunchArgs  string            `json:"launch_args"`
	Environment map[string]string `json:"environment"`
	DefaultMods []string          `json:"default_mods"`
	MinMemory   int               `json:"min_memory"`
	MinDisk     int               `json:"min_disk"`
}

// ServerStats 服务器统计
type ServerStats struct {
	TotalServers   int     `json:"total_servers"`
	RunningServers int     `json:"running_servers"`
	StoppedServers int     `json:"stopped_servers"`
	TotalPlayers   int     `json:"total_players"`
	MaxPlayers     int     `json:"max_players"`
	TotalUptime    int64   `json:"total_uptime"`
	AvgUptime      float64 `json:"avg_uptime"`
	TotalBackups   int     `json:"total_backups"`
	TotalMods      int     `json:"total_mods"`
	TotalWorldSize int64   `json:"total_world_size"`
}

// ConsoleLog 控制台日志
type ConsoleLog struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Source    string    `json:"source"`
}

// RCONCommand RCON 命令
type RCONCommand struct {
	Command   string    `json:"command"`
	Response  string    `json:"response"`
	Timestamp time.Time `json:"timestamp"`
}

// BanEntry 封禁记录
type BanEntry struct {
	PlayerID   string     `json:"player_id"`
	PlayerName string     `json:"player_name"`
	Reason     string     `json:"reason"`
	BannedBy   string     `json:"banned_by"`
	BannedAt   time.Time  `json:"banned_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	Permanent  bool       `json:"permanent"`
}

// ServerEvent 服务器事件
type ServerEvent struct {
	ID        string                 `json:"id"`
	ServerID  string                 `json:"server_id"`
	Type      string                 `json:"type"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}
