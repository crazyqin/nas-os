package videoconf

import (
	"time"
)

// ParticipantRole 参与者角色
type ParticipantRole string

const (
	RoleHost  ParticipantRole = "host"
	RoleGuest ParticipantRole = "guest"
)

// MediaState 媒体状态
type MediaState struct {
	AudioEnabled    bool `json:"audio_enabled"`
	VideoEnabled    bool `json:"video_enabled"`
	ScreenSharing   bool `json:"screen_sharing"`
}

// Participant 会议参与者
type Participant struct {
	ID         string         `json:"id"`
	UserID     string         `json:"user_id"`
	Name       string         `json:"name"`
	Role       ParticipantRole `json:"role"`
	Media      MediaState     `json:"media"`
	IsMuted    bool           `json:"is_muted"`
	IsKicked   bool           `json:"is_kicked"`
	JoinedAt   time.Time      `json:"joined_at"`
}

// RoomStatus 房间状态
type RoomStatus string

const (
	RoomStatusWaiting  RoomStatus = "waiting"
	RoomStatusActive   RoomStatus = "active"
	RoomStatusEnded    RoomStatus = "ended"
)

// Room 会议室
type Room struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Password        string         `json:"password,omitempty"`
	HostID          string         `json:"host_id"`
	MaxParticipants int            `json:"max_participants"`
	Status          RoomStatus     `json:"status"`
	Participants    []*Participant `json:"participants"`
	CreatedAt       time.Time      `json:"created_at"`
	EndedAt         *time.Time     `json:"ended_at,omitempty"`
}

// RecordingStatus 录制状态
type RecordingStatus string

const (
	RecordingStatusIdle    RecordingStatus = "idle"
	RecordingStatusRunning RecordingStatus = "running"
	RecordingStatusDone    RecordingStatus = "done"
)

// Recording 会议录制
type Recording struct {
	ID        string          `json:"id"`
	RoomID    string          `json:"room_id"`
	Status    RecordingStatus `json:"status"`
	StartedAt time.Time       `json:"started_at"`
	EndedAt   *time.Time      `json:"ended_at,omitempty"`
	Duration  int             `json:"duration"`
	FileSize  int64           `json:"file_size"`
}

// SignalType 信令类型
type SignalType string

const (
	SignalOffer     SignalType = "offer"
	SignalAnswer    SignalType = "answer"
	SignalICE       SignalType = "ice-candidate"
	SignalBye       SignalType = "bye"
)

// Signal WebRTC信令
type Signal struct {
	Type       SignalType `json:"type"`
	FromUserID string     `json:"from_user_id"`
	ToUserID   string     `json:"to_user_id"`
	RoomID     string     `json:"room_id"`
	Payload    string     `json:"payload"`
	Timestamp  time.Time  `json:"timestamp"`
}

// ChatMessage 会议内聊天消息
type ChatMessage struct {
	ID        string    `json:"id"`
	RoomID    string    `json:"room_id"`
	UserID    string    `json:"user_id"`
	UserName  string    `json:"user_name"`
	Content   string    `json:"content"`
	IsPrivate bool      `json:"is_private"`
	ToUserID  string    `json:"to_user_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateRoomRequest 创建房间请求
type CreateRoomRequest struct {
	Name            string `json:"name"`
	Password        string `json:"password,omitempty"`
	HostID          string `json:"host_id"`
	MaxParticipants int    `json:"max_participants"`
}

// JoinRoomRequest 加入房间请求
type JoinRoomRequest struct {
	UserID   string `json:"user_id"`
	UserName string `json:"user_name"`
	Password string `json:"password,omitempty"`
}

// SignalRequest 信令请求
type SignalRequest struct {
	Type       SignalType `json:"type"`
	FromUserID string     `json:"from_user_id"`
	ToUserID   string     `json:"to_user_id"`
	RoomID     string     `json:"room_id"`
	Payload    string     `json:"payload"`
}

// SendChatRequest 发送聊天消息请求
type SendChatRequest struct {
	UserID    string `json:"user_id"`
	UserName  string `json:"user_name"`
	Content   string `json:"content"`
	IsPrivate bool   `json:"is_private,omitempty"`
	ToUserID  string `json:"to_user_id,omitempty"`
}

// ConferenceStats 会议统计
type ConferenceStats struct {
	TotalRooms       int `json:"total_rooms"`
	ActiveRooms      int `json:"active_rooms"`
	TotalParticipants int `json:"total_participants"`
	TotalRecordings  int `json:"total_recordings"`
	ActiveRecordings int `json:"active_recordings"`
}
