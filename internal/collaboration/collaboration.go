package collaboration

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Collaboration 团队协作模块
type Collaboration struct {
	mu         sync.RWMutex
	teams      map[string]*Team
	members    map[string]*Member
	shares     map[string]*Share
	tags       map[string]*Tag
	fileLocks  map[string]*FileLock
	requests   map[string]*FileRequest
	activities []*Activity
	config     *Config
}

// Team 团队
type Team struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	OwnerID     string    `json:"owner_id"`
	Members     []string  `json:"members"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Member 成员
type Member struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TeamID    string    `json:"team_id"`
	Role      string    `json:"role"`   // owner, admin, member, viewer
	Status    string    `json:"status"` // active, inactive, invited
	JoinedAt  time.Time `json:"joined_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Share 共享
type Share struct {
	ID            string     `json:"id"`
	FileID        string     `json:"file_id"`
	SharedBy      string     `json:"shared_by"`
	SharedWith    []string   `json:"shared_with"`
	Permission    string     `json:"permission"` // view, edit, admin
	ExpiresAt     *time.Time `json:"expires_at"`
	Password      string     `json:"password"`
	IsPublic      bool       `json:"is_public"`
	DownloadCount int        `json:"download_count"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// Tag 标签
type Tag struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	Files     []string  `json:"files"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// FileLock 文件锁
type FileLock struct {
	ID        string    `json:"id"`
	FileID    string    `json:"file_id"`
	LockedBy  string    `json:"locked_by"`
	Reason    string    `json:"reason"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// FileRequest 文件请求
type FileRequest struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	CreatedBy   string     `json:"created_by"`
	AssignedTo  []string   `json:"assigned_to"`
	Status      string     `json:"status"` // pending, in_progress, completed, cancelled
	Files       []string   `json:"files"`
	DueDate     *time.Time `json:"due_date"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Activity 活动记录
type Activity struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"` // share, lock, tag, request, comment
	UserID    string                 `json:"user_id"`
	FileID    string                 `json:"file_id"`
	Details   map[string]interface{} `json:"details"`
	CreatedAt time.Time              `json:"created_at"`
}

// Config 配置
type Config struct {
	MaxTeams             int           `json:"max_teams"`
	MaxMembers           int           `json:"max_members"`
	MaxShares            int           `json:"max_shares"`
	LockTimeout          time.Duration `json:"lock_timeout"`
	ActivityRetention    time.Duration `json:"activity_retention"`
	NotificationsEnabled bool          `json:"notifications_enabled"`
}

// NewCollaboration 创建协作模块
func NewCollaboration(config *Config) *Collaboration {
	return &Collaboration{
		teams:      make(map[string]*Team),
		members:    make(map[string]*Member),
		shares:     make(map[string]*Share),
		tags:       make(map[string]*Tag),
		fileLocks:  make(map[string]*FileLock),
		requests:   make(map[string]*FileRequest),
		activities: make([]*Activity, 0),
		config:     config,
	}
}

// CreateTeam 创建团队
func (c *Collaboration) CreateTeam(ctx context.Context, team *Team) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	team.CreatedAt = time.Now()
	team.UpdatedAt = time.Now()
	c.teams[team.ID] = team
	return nil
}

// GetTeam 获取团队
func (c *Collaboration) GetTeam(ctx context.Context, id string) (*Team, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	team, exists := c.teams[id]
	if !exists {
		return nil, fmt.Errorf("team not found: %s", id)
	}
	return team, nil
}

// ListTeams 列出团队
func (c *Collaboration) ListTeams(ctx context.Context, userID string) []*Team {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var teams []*Team
	for _, team := range c.teams {
		if containsMember(team.Members, userID) {
			teams = append(teams, team)
		}
	}
	return teams
}

// AddMember 添加成员
func (c *Collaboration) AddMember(ctx context.Context, member *Member) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	team, exists := c.teams[member.TeamID]
	if !exists {
		return fmt.Errorf("team not found: %s", member.TeamID)
	}

	member.JoinedAt = time.Now()
	member.UpdatedAt = time.Now()
	c.members[member.ID] = member

	team.Members = append(team.Members, member.UserID)
	team.UpdatedAt = time.Now()

	return nil
}

// RemoveMember 移除成员
func (c *Collaboration) RemoveMember(ctx context.Context, memberID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	member, exists := c.members[memberID]
	if !exists {
		return fmt.Errorf("member not found: %s", memberID)
	}

	team, exists := c.teams[member.TeamID]
	if exists {
		team.Members = removeMemberFromList(team.Members, member.UserID)
		team.UpdatedAt = time.Now()
	}

	delete(c.members, memberID)
	return nil
}

// UpdateMemberRole 更新成员角色
func (c *Collaboration) UpdateMemberRole(ctx context.Context, memberID, role string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	member, exists := c.members[memberID]
	if !exists {
		return fmt.Errorf("member not found: %s", memberID)
	}

	member.Role = role
	member.UpdatedAt = time.Now()
	return nil
}

// CreateShare 创建共享
func (c *Collaboration) CreateShare(ctx context.Context, share *Share) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	share.CreatedAt = time.Now()
	share.UpdatedAt = time.Now()
	c.shares[share.ID] = share

	// 记录活动
	c.addActivity("share", share.SharedBy, share.FileID, map[string]interface{}{
		"share_id":   share.ID,
		"permission": share.Permission,
	})

	return nil
}

// GetShare 获取共享
func (c *Collaboration) GetShare(ctx context.Context, id string) (*Share, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	share, exists := c.shares[id]
	if !exists {
		return nil, fmt.Errorf("share not found: %s", id)
	}
	return share, nil
}

// UpdateShare 更新共享
func (c *Collaboration) UpdateShare(ctx context.Context, shareID string, updates map[string]interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	share, exists := c.shares[shareID]
	if !exists {
		return fmt.Errorf("share not found: %s", shareID)
	}

	if permission, ok := updates["permission"].(string); ok {
		share.Permission = permission
	}
	if sharedWith, ok := updates["shared_with"].([]string); ok {
		share.SharedWith = sharedWith
	}

	share.UpdatedAt = time.Now()
	return nil
}

// DeleteShare 删除共享
func (c *Collaboration) DeleteShare(ctx context.Context, shareID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.shares, shareID)
	return nil
}

// AddTag 添加标签
func (c *Collaboration) AddTag(ctx context.Context, tag *Tag) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	tag.CreatedAt = time.Now()
	c.tags[tag.ID] = tag
	return nil
}

// TagFile 文件打标签
func (c *Collaboration) TagFile(ctx context.Context, tagID, fileID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	tag, exists := c.tags[tagID]
	if !exists {
		return fmt.Errorf("tag not found: %s", tagID)
	}

	tag.Files = append(tag.Files, fileID)

	// 记录活动
	c.addActivity("tag", tag.CreatedBy, fileID, map[string]interface{}{
		"tag_id":   tagID,
		"tag_name": tag.Name,
	})

	return nil
}

// UntagFile 移除文件标签
func (c *Collaboration) UntagFile(ctx context.Context, tagID, fileID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	tag, exists := c.tags[tagID]
	if !exists {
		return fmt.Errorf("tag not found: %s", tagID)
	}

	tag.Files = removeFileFromList(tag.Files, fileID)
	return nil
}

// GetTags 获取文件标签
func (c *Collaboration) GetTags(ctx context.Context, fileID string) []*Tag {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var tags []*Tag
	for _, tag := range c.tags {
		if containsFile(tag.Files, fileID) {
			tags = append(tags, tag)
		}
	}
	return tags
}

// LockFile 锁定文件
func (c *Collaboration) LockFile(ctx context.Context, fileID, userID, reason string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 检查是否已锁定
	if _, exists := c.fileLocks[fileID]; exists {
		return fmt.Errorf("file already locked: %s", fileID)
	}

	lock := &FileLock{
		ID:        generateID(),
		FileID:    fileID,
		LockedBy:  userID,
		Reason:    reason,
		ExpiresAt: time.Now().Add(c.config.LockTimeout),
		CreatedAt: time.Now(),
	}

	c.fileLocks[fileID] = lock

	// 记录活动
	c.addActivity("lock", userID, fileID, map[string]interface{}{
		"lock_id": lock.ID,
		"reason":  reason,
	})

	return nil
}

// UnlockFile 解锁文件
func (c *Collaboration) UnlockFile(ctx context.Context, fileID, userID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	lock, exists := c.fileLocks[fileID]
	if !exists {
		return fmt.Errorf("file not locked: %s", fileID)
	}

	if lock.LockedBy != userID {
		return fmt.Errorf("file locked by another user")
	}

	delete(c.fileLocks, fileID)

	// 记录活动
	c.addActivity("unlock", userID, fileID, map[string]interface{}{
		"lock_id": lock.ID,
	})

	return nil
}

// IsFileLocked 检查文件是否锁定
func (c *Collaboration) IsFileLocked(ctx context.Context, fileID string) (bool, string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	lock, exists := c.fileLocks[fileID]
	if !exists {
		return false, ""
	}

	// 检查是否过期
	if time.Now().After(lock.ExpiresAt) {
		delete(c.fileLocks, fileID)
		return false, ""
	}

	return true, lock.LockedBy
}

// CreateFileRequest 创建文件请求
func (c *Collaboration) CreateFileRequest(ctx context.Context, request *FileRequest) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	request.CreatedAt = time.Now()
	request.UpdatedAt = time.Now()
	request.Status = "pending"
	c.requests[request.ID] = request

	// 记录活动
	c.addActivity("request", request.CreatedBy, "", map[string]interface{}{
		"request_id": request.ID,
		"title":      request.Title,
	})

	return nil
}

// GetFileRequest 获取文件请求
func (c *Collaboration) GetFileRequest(ctx context.Context, id string) (*FileRequest, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	request, exists := c.requests[id]
	if !exists {
		return nil, fmt.Errorf("request not found: %s", id)
	}
	return request, nil
}

// UpdateFileRequestStatus 更新文件请求状态
func (c *Collaboration) UpdateFileRequestStatus(ctx context.Context, requestID, status string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	request, exists := c.requests[requestID]
	if !exists {
		return fmt.Errorf("request not found: %s", requestID)
	}

	request.Status = status
	request.UpdatedAt = time.Now()
	return nil
}

// AddFileToRequest 添加文件到请求
func (c *Collaboration) AddFileToRequest(ctx context.Context, requestID, fileID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	request, exists := c.requests[requestID]
	if !exists {
		return fmt.Errorf("request not found: %s", requestID)
	}

	request.Files = append(request.Files, fileID)
	request.UpdatedAt = time.Now()
	return nil
}

// GetActivities 获取活动记录
func (c *Collaboration) GetActivities(ctx context.Context, fileID string, limit int) []*Activity {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var activities []*Activity
	for _, activity := range c.activities {
		if fileID == "" || activity.FileID == fileID {
			activities = append(activities, activity)
		}
	}

	// 按时间倒序排序
	sortActivitiesByTime(activities)

	if len(activities) > limit {
		return activities[:limit]
	}
	return activities
}

// GetStats 获取统计信息
func (c *Collaboration) GetStats(ctx context.Context) map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return map[string]interface{}{
		"total_teams":      len(c.teams),
		"total_members":    len(c.members),
		"total_shares":     len(c.shares),
		"total_tags":       len(c.tags),
		"total_locks":      len(c.fileLocks),
		"total_requests":   len(c.requests),
		"total_activities": len(c.activities),
	}
}

// 内部方法
func (c *Collaboration) addActivity(activityType, userID, fileID string, details map[string]interface{}) {
	activity := &Activity{
		ID:        generateID(),
		Type:      activityType,
		UserID:    userID,
		FileID:    fileID,
		Details:   details,
		CreatedAt: time.Now(),
	}
	c.activities = append(c.activities, activity)
}

// 辅助函数
func containsMember(members []string, userID string) bool {
	for _, member := range members {
		if member == userID {
			return true
		}
	}
	return false
}

func removeMemberFromList(members []string, userID string) []string {
	var result []string
	for _, member := range members {
		if member != userID {
			result = append(result, member)
		}
	}
	return result
}

func containsFile(files []string, fileID string) bool {
	for _, file := range files {
		if file == fileID {
			return true
		}
	}
	return false
}

func removeFileFromList(files []string, fileID string) []string {
	var result []string
	for _, file := range files {
		if file != fileID {
			result = append(result, file)
		}
	}
	return result
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func sortActivitiesByTime(activities []*Activity) {
	for i := 0; i < len(activities); i++ {
		for j := i + 1; j < len(activities); j++ {
			if activities[i].CreatedAt.Before(activities[j].CreatedAt) {
				activities[i], activities[j] = activities[j], activities[i]
			}
		}
	}
}
