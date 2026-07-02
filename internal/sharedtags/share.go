package sharedtags

import (
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
)

// TagShareManager manages tag sharing, permissions, and subscriptions.
type TagShareManager struct {
	mu          sync.RWMutex
	shares      map[string]*TagShare   // shareID -> TagShare
	tagShares   map[string][]*TagShare // tagID -> []*TagShare
	userShares  map[string][]*TagShare // userID -> []*TagShare
	subscribers map[string][]string    // tagID -> []userID (notification subscribers)
	manager     *TagManager
	nextID      int64
}

// NewTagShareManager creates a new TagShareManager instance.
func NewTagShareManager(manager *TagManager) *TagShareManager {
	sm := &TagShareManager{
		shares:      make(map[string]*TagShare),
		tagShares:   make(map[string][]*TagShare),
		userShares:  make(map[string][]*TagShare),
		subscribers: make(map[string][]string),
		manager:     manager,
	}
	log.Println("标签共享管理器已初始化")
	return sm
}

// ShareTag shares a tag with a user or group.
func (sm *TagShareManager) ShareTag(shareBy string, req ShareTagRequest) (*TagShare, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Verify tag exists
	tag, err := sm.manager.GetTag(req.TagID)
	if err != nil {
		return nil, err
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check for duplicate share
	for _, share := range sm.tagShares[req.TagID] {
		if share.SharedWith == req.SharedWith && share.TargetType == req.TargetType {
			return nil, ErrDuplicateShare
		}
	}

	sm.nextID++
	share := &TagShare{
		ID:         fmt.Sprintf("share-%d", sm.nextID),
		TagID:      req.TagID,
		TagName:    tag.Name,
		SharedBy:   shareBy,
		SharedWith: req.SharedWith,
		TargetType: req.TargetType,
		Permission: req.Permission,
		Notify:     req.Notify,
		CreatedAt:  time.Now(),
		ExpiresAt:  req.ExpiresAt,
	}

	sm.shares[share.ID] = share
	sm.tagShares[req.TagID] = append(sm.tagShares[req.TagID], share)
	sm.userShares[req.SharedWith] = append(sm.userShares[req.SharedWith], share)

	// Mark tag as shared
	tag.IsShared = true

	// Subscribe for notifications if requested
	if req.Notify {
		sm.addSubscriber(req.TagID, req.SharedWith)
	}

	log.Printf("标签已共享: %s -> %s (%s)", tag.Name, req.SharedWith, req.Permission)
	return share, nil
}

// RevokeShare revokes a tag share.
func (sm *TagShareManager) RevokeShare(shareID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	share, ok := sm.shares[shareID]
	if !ok {
		return ErrShareNotFound
	}

	// Remove from tag shares
	tagShares := sm.tagShares[share.TagID]
	var newTagShares []*TagShare
	for _, s := range tagShares {
		if s.ID != shareID {
			newTagShares = append(newTagShares, s)
		}
	}
	if len(newTagShares) == 0 {
		delete(sm.tagShares, share.TagID)
	} else {
		sm.tagShares[share.TagID] = newTagShares
	}

	// Remove from user shares
	userShares := sm.userShares[share.SharedWith]
	var newUserShares []*TagShare
	for _, s := range userShares {
		if s.ID != shareID {
			newUserShares = append(newUserShares, s)
		}
	}
	if len(newUserShares) == 0 {
		delete(sm.userShares, share.SharedWith)
	} else {
		sm.userShares[share.SharedWith] = newUserShares
	}

	// Remove subscriber
	sm.removeSubscriber(share.TagID, share.SharedWith)

	// Update tag shared status
	sm.updateTagSharedStatus(share.TagID)

	delete(sm.shares, shareID)

	log.Printf("共享已撤销: %s", shareID)
	return nil
}

// UpdateSharePermission updates the permission of a share.
func (sm *TagShareManager) UpdateSharePermission(shareID string, permission TagSharePermission) (*TagShare, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	share, ok := sm.shares[shareID]
	if !ok {
		return nil, ErrShareNotFound
	}

	share.Permission = permission
	log.Printf("共享权限已更新: %s -> %s", shareID, permission)
	return share, nil
}

// ToggleSubscription toggles notification subscription for a tag.
func (sm *TagShareManager) ToggleSubscription(tagID, userID string) (bool, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check if tag exists
	if _, err := sm.manager.GetTag(tagID); err != nil {
		return false, err
	}

	subs := sm.subscribers[tagID]
	for i, sub := range subs {
		if sub == userID {
			// Remove subscription
			sm.subscribers[tagID] = append(subs[:i], subs[i+1:]...)
			log.Printf("用户 %s 已取消订阅标签 %s", userID, tagID)
			return false, nil
		}
	}

	// Add subscription
	sm.subscribers[tagID] = append(sm.subscribers[tagID], userID)
	log.Printf("用户 %s 已订阅标签 %s", userID, tagID)
	return true, nil
}

// GetSubscribers returns all subscribers for a tag.
func (sm *TagShareManager) GetSubscribers(tagID string) []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	subs := sm.subscribers[tagID]
	result := make([]string, len(subs))
	copy(result, subs)
	return result
}

// GetUserSharedTags returns all tags shared with a user.
func (sm *TagShareManager) GetUserSharedTags(userID string) []*TagShare {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	shares := sm.userShares[userID]
	var result []*TagShare
	now := time.Now()

	for _, share := range shares {
		// Check expiration
		if share.ExpiresAt != nil && share.ExpiresAt.Before(now) {
			continue
		}
		result = append(result, share)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}

// GetTagShares returns all shares for a specific tag.
func (sm *TagShareManager) GetTagShares(tagID string) []*TagShare {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	shares := sm.tagShares[tagID]
	result := make([]*TagShare, len(shares))
	copy(result, shares)
	return result
}

// CheckPermission checks if a user has a specific permission on a tag.
func (sm *TagShareManager) CheckPermission(tagID, userID string, requiredPerm TagSharePermission) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Check if user is the tag owner
	tag, err := sm.manager.GetTag(tagID)
	if err == nil && tag.Owner == userID {
		return true
	}

	// Check shares
	shares := sm.tagShares[tagID]
	now := time.Now()
	for _, share := range shares {
		if share.SharedWith != userID {
			continue
		}
		// Check expiration
		if share.ExpiresAt != nil && share.ExpiresAt.Before(now) {
			continue
		}
		// Check permission level
		if sm.permissionLevel(share.Permission) >= sm.permissionLevel(requiredPerm) {
			return true
		}
	}

	return false
}

// permissionLevel returns numeric level for permission comparison.
func (sm *TagShareManager) permissionLevel(perm TagSharePermission) int {
	switch perm {
	case ShareView:
		return 1
	case ShareUse:
		return 2
	case ShareEdit:
		return 3
	case ShareManage:
		return 4
	default:
		return 0
	}
}

// addSubscriber adds a subscriber to a tag.
func (sm *TagShareManager) addSubscriber(tagID, userID string) {
	subs := sm.subscribers[tagID]
	for _, sub := range subs {
		if sub == userID {
			return // Already subscribed
		}
	}
	sm.subscribers[tagID] = append(sm.subscribers[tagID], userID)
}

// removeSubscriber removes a subscriber from a tag.
func (sm *TagShareManager) removeSubscriber(tagID, userID string) {
	subs := sm.subscribers[tagID]
	for i, sub := range subs {
		if sub == userID {
			sm.subscribers[tagID] = append(subs[:i], subs[i+1:]...)
			return
		}
	}
}

// updateTagSharedStatus updates the IsShared flag on a tag.
func (sm *TagShareManager) updateTagSharedStatus(tagID string) {
	shares := sm.tagShares[tagID]
	if tag, err := sm.manager.GetTag(tagID); err == nil {
		tag.IsShared = len(shares) > 0
	}
}

// GetShareStats returns sharing statistics.
func (sm *TagShareManager) GetShareStats() *ShareStats {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	stats := &ShareStats{
		TotalShares:   int64(len(sm.shares)),
		SharedTags:    int64(len(sm.tagShares)),
		ActiveUsers:   int64(len(sm.userShares)),
		Subscriptions: 0,
	}

	for _, subs := range sm.subscribers {
		stats.Subscriptions += int64(len(subs))
	}

	return stats
}

// ShareStats represents sharing statistics.
type ShareStats struct {
	TotalShares   int64 `json:"totalShares"`   // 总共享数
	SharedTags    int64 `json:"sharedTags"`    // 被共享的标签数
	ActiveUsers   int64 `json:"activeUsers"`   // 参与共享的用户数
	Subscriptions int64 `json:"subscriptions"` // 订阅通知数
}

// NotifySubscribers sends notification to subscribers (stub for integration).
func (sm *TagShareManager) NotifySubscribers(tagID, message string) []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	subs := sm.subscribers[tagID]
	if len(subs) == 0 {
		return nil
	}

	tag, err := sm.manager.GetTag(tagID)
	if err != nil {
		return nil
	}

	log.Printf("通知订阅者: 标签 %s - %s (共 %d 人)", tag.Name, message, len(subs))

	// Return list of notified users (actual notification delivery would be handled externally)
	notified := make([]string, len(subs))
	copy(notified, subs)
	return notified
}
