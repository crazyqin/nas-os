package lock

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 冲突类型定义 ==========.

// ExtendedConflictType 扩展冲突类型.
type ExtendedConflictType string

const (
	// ExtendedConflictExclusiveLock 独占锁冲突.
	ExtendedConflictExclusiveLock ExtendedConflictType = "exclusive_lock"
	// ExtendedConflictSharedLock 共享锁冲突.
	ExtendedConflictSharedLock ExtendedConflictType = "shared_lock"
	// ExtendedConflictEditCollision 编辑碰撞（多人同时编辑）.
	ExtendedConflictEditCollision ExtendedConflictType = "edit_collision"
	// ExtendedConflictVersion 版本冲突.
	ExtendedConflictVersion ExtendedConflictType = "version_conflict"
	// ExtendedConflictPermission 权限冲突.
	ExtendedConflictPermission ExtendedConflictType = "permission_denied"
	// ExtendedConflictResource 资源耗尽.
	ExtendedConflictResource ExtendedConflictType = "resource_exhausted"
	// ExtendedConflictTimeout 超时冲突.
	ExtendedConflictTimeout ExtendedConflictType = "timeout"
	// ExtendedConflictOwnerChange 持有者变更.
	ExtendedConflictOwnerChange ExtendedConflictType = "owner_change"
)

// ConflictSeverity 冲突严重程度.
type ConflictSeverity int

const (
	// SeverityLow 低严重性（可忽略）.
	SeverityLow ConflictSeverity = iota
	// SeverityMedium 中等严重性（需要处理）.
	SeverityMedium
	// SeverityHigh 高严重性（必须处理）.
	SeverityHigh
	// SeverityCritical 关键严重性（可能导致数据丢失）.
	SeverityCritical
)

func (cs ConflictSeverity) String() string {
	switch cs {
	case SeverityLow:
		return "low"
	case SeverityMedium:
		return "medium"
	case SeverityHigh:
		return "high"
	case SeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// ========== 冲突记录 ==========.

// ConflictRecord 冲突记录.
type ConflictRecord struct {
	// ID 冲突ID
	ID string `json:"id"`
	// Type 冲突类型
	Type ExtendedConflictType `json:"type"`
	// Severity 严重程度
	Severity ConflictSeverity `json:"severity"`
	// Status 冲突状态
	Status ConflictRecordStatus `json:"status"`
	// FilePath 文件路径
	FilePath string `json:"filePath"`
	// FileName 文件名
	FileName string `json:"fileName"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"createdAt"`
	// ResolvedAt 解决时间
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`
	// ResolvedBy 解决者
	ResolvedBy string `json:"resolvedBy,omitempty"`
	// Resolution 解决方式
	Resolution string `json:"resolution,omitempty"`
	// Description 冲突描述
	Description string `json:"description"`
	// Participants 冲突参与者
	Participants []*ConflictParticipant `json:"participants"`
	// Suggestions 建议解决方案
	Suggestions []*ConflictSuggestion `json:"suggestions"`
	// RelatedLocks 相关锁ID
	RelatedLocks []string `json:"relatedLocks,omitempty"`
	// RelatedVersions 相关版本ID
	RelatedVersions []string `json:"relatedVersions,omitempty"`
	// Metadata 元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	mu sync.RWMutex
}

// ConflictRecordStatus 冲突记录状态.
type ConflictRecordStatus int

const (
	// ConflictStatusPending 待处理.
	ConflictStatusPending ConflictRecordStatus = iota
	// ConflictStatusInProgress 处理中.
	ConflictStatusInProgress
	// ConflictStatusResolved 已解决.
	ConflictStatusResolved
	// ConflictStatusEscalated 已升级.
	ConflictStatusEscalated
	// ConflictStatusIgnored 已忽略.
	ConflictStatusIgnored
)

func (s ConflictRecordStatus) String() string {
	switch s {
	case ConflictStatusPending:
		return "pending"
	case ConflictStatusInProgress:
		return "in_progress"
	case ConflictStatusResolved:
		return "resolved"
	case ConflictStatusEscalated:
		return "escalated"
	case ConflictStatusIgnored:
		return "ignored"
	default:
		return "unknown"
	}
}

// ConflictParticipant 冲突参与者.
type ConflictParticipant struct {
	// UserID 用户ID
	UserID string `json:"userId"`
	// UserName 用户名
	UserName string `json:"userName"`
	// ClientID 客户端ID
	ClientID string `json:"clientId,omitempty"`
	// Role 参与角色
	Role ConflictRole `json:"role"`
	// Actions 参与者的操作
	Actions []*ConflictAction `json:"actions,omitempty"`
	// NotifiedAt 通知时间
	NotifiedAt *time.Time `json:"notifiedAt,omitempty"`
	// ResponseAt 响应时间
	ResponseAt *time.Time `json:"responseAt,omitempty"`
	// Response 响应内容
	Response string `json:"response,omitempty"`
}

// ConflictRole 参与角色.
type ConflictRole string

const (
	// RoleInitiator 发起者.
	RoleInitiator ConflictRole = "initiator"
	// RoleOwner 当前持有者.
	RoleOwner ConflictRole = "owner"
	// RoleCollaborator 协作者.
	RoleCollaborator ConflictRole = "collaborator"
	// RoleMediator 调解者（管理员）.
	RoleMediator ConflictRole = "mediator"
)

// ConflictAction 冲突操作.
type ConflictAction struct {
	// Action 操作类型
	Action string `json:"action"`
	// Timestamp 时间戳
	Timestamp time.Time `json:"timestamp"`
	// Details 详情
	Details map[string]interface{} `json:"details,omitempty"`
}

// ConflictSuggestion 解决建议.
type ConflictSuggestion struct {
	// ID 建议ID
	ID string `json:"id"`
	// Type 建议类型
	Type SuggestionType `json:"type"`
	// Priority 优先级
	Priority int `json:"priority"`
	// Description 描述
	Description string `json:"description"`
	// AutoApplicable 是否可自动应用
	AutoApplicable bool `json:"autoApplicable"`
	// Params 参数
	Params map[string]interface{} `json:"params,omitempty"`
	// Risks 风险说明
	Risks []string `json:"risks,omitempty"`
}

// SuggestionType 建议类型.
type SuggestionType string

const (
	// SuggestionWait 等待.
	SuggestionWait SuggestionType = "wait"
	// SuggestionForce 强制解决.
	SuggestionForce SuggestionType = "force"
	// SuggestionMerge 合并.
	SuggestionMerge SuggestionType = "merge"
	// SuggestionRollback 回滚.
	SuggestionRollback SuggestionType = "rollback"
	// SuggestionNotify 通知.
	SuggestionNotify SuggestionType = "notify"
	// SuggestionEscalate 升级.
	SuggestionEscalate SuggestionType = "escalate"
	// SuggestionSplit 分割.
	SuggestionSplit SuggestionType = "split"
	// SuggestionVersion 创建版本.
	SuggestionVersion SuggestionType = "version"
)

// ========== 冲突检测器 ==========.

// ConflictDetector 冲突检测器.
type ConflictDetector struct {
	config ConflictDetectorConfig
	// lockManager 锁管理器
	lockManager *Manager
	// versionManager 版本管理器
	versionManager *VersionManager
	// conflicts 冲突记录
	conflicts sync.Map // map[string]*ConflictRecord
	// activeConflicts 活跃冲突索引
	activeConflicts sync.Map // map[string][]string (key: filePath)
	// notificationChan 通知通道
	notificationChan chan *ConflictNotification
	// stopCh 停止通道
	stopCh chan struct{}
	// wg 等待组
	wg sync.WaitGroup
}

// ConflictDetectorConfig 冲突检测器配置.
type ConflictDetectorConfig struct {
	// EnableAutoDetection 启用自动检测
	EnableAutoDetection bool `json:"enableAutoDetection"`
	// DetectionInterval 检测间隔
	DetectionInterval time.Duration `json:"detectionInterval"`
	// EnableAutoResolution 启用自动解决
	EnableAutoResolution bool `json:"enableAutoResolution"`
	// MaxConflictAge 冲突最大保留时间
	MaxConflictAge time.Duration `json:"maxConflictAge"`
	// NotificationEnabled 启用通知
	NotificationEnabled bool `json:"notificationEnabled"`
	// EscalationTimeout 升级超时时间
	EscalationTimeout time.Duration `json:"escalationTimeout"`
	// MaxActiveConflicts 最大活跃冲突数
	MaxActiveConflicts int `json:"maxActiveConflicts"`
}

// DefaultConflictDetectorConfig 默认配置.
func DefaultConflictDetectorConfig() ConflictDetectorConfig {
	return ConflictDetectorConfig{
		EnableAutoDetection:  true,
		DetectionInterval:    time.Minute,
		EnableAutoResolution: false,
		MaxConflictAge:       time.Hour * 24 * 7,
		NotificationEnabled:  true,
		EscalationTimeout:    time.Hour,
		MaxActiveConflicts:   1000,
	}
}

// NewConflictDetector 创建冲突检测器.
func NewConflictDetector(config ConflictDetectorConfig, lockManager *Manager, versionManager *VersionManager) *ConflictDetector {
	cd := &ConflictDetector{
		config:           config,
		lockManager:      lockManager,
		versionManager:   versionManager,
		notificationChan: make(chan *ConflictNotification, 100),
		stopCh:           make(chan struct{}),
	}

	// 启动后台检测
	if config.EnableAutoDetection {
		cd.wg.Add(1)
		go cd.detectionLoop()
	}

	// 启动自动解决
	if config.EnableAutoResolution {
		cd.wg.Add(1)
		go cd.resolutionLoop()
	}

	// 启动清理任务
	cd.wg.Add(1)
	go cd.cleanupLoop()

	return cd
}

// ========== 冲突检测 ==========.

// DetectConflict 检测冲突.
func (cd *ConflictDetector) DetectConflict(ctx context.Context, req *ConflictDetectionRequest) (*ConflictRecord, error) {
	// 检查锁冲突
	if cd.lockManager != nil {
		if lockConflict, canAcquire := cd.lockManager.CanAcquire(req.FilePath, req.LockType, req.UserID); !canAcquire && lockConflict != nil {
			return cd.createLockConflict(req, lockConflict), nil
		}
	}

	// 检查编辑碰撞
	if req.Operation == "edit" || req.Operation == "write" {
		if collision := cd.detectEditCollision(req); collision != nil {
			return collision, nil
		}
	}

	// 检查版本冲突
	if cd.versionManager != nil && req.BaseVersionID != "" {
		if versionConflict := cd.detectVersionConflict(req); versionConflict != nil {
			return versionConflict, nil
		}
	}

	return nil, nil
}

// DetectBatch 批量检测冲突.
func (cd *ConflictDetector) DetectBatch(ctx context.Context, requests []*ConflictDetectionRequest) ([]*ConflictRecord, error) {
	var conflicts []*ConflictRecord

	for _, req := range requests {
		conflict, err := cd.DetectConflict(ctx, req)
		if err != nil {
			return nil, err
		}
		if conflict != nil {
			conflicts = append(conflicts, conflict)
		}
	}

	return conflicts, nil
}

// ========== 冲突记录管理 ==========.

// CreateConflict 创建冲突记录.
func (cd *ConflictDetector) CreateConflict(conflictType ExtendedConflictType, filePath string, participants []*ConflictParticipant, description string) *ConflictRecord {
	conflict := &ConflictRecord{
		ID:           uuid.New().String(),
		Type:         conflictType,
		Severity:     cd.determineSeverity(conflictType),
		Status:       ConflictStatusPending,
		FilePath:     filePath,
		FileName:     extractFileName(filePath),
		CreatedAt:    time.Now(),
		Description:  description,
		Participants: participants,
		Suggestions:  cd.generateSuggestions(conflictType),
		Metadata:     make(map[string]interface{}),
	}

	// 存储冲突
	cd.conflicts.Store(conflict.ID, conflict)
	cd.addToActiveIndex(filePath, conflict.ID)

	// 发送通知
	if cd.config.NotificationEnabled {
		cd.notifyConflict(conflict)
	}

	return conflict
}

// GetConflict 获取冲突记录.
func (cd *ConflictDetector) GetConflict(conflictID string) (*ConflictRecord, error) {
	raw, ok := cd.conflicts.Load(conflictID)
	if !ok {
		return nil, errors.New("conflict not found")
	}
	conflict, ok := raw.(*ConflictRecord)
	if !ok {
		return nil, errors.New("invalid conflict record type")
	}
	return conflict, nil
}

// ListConflicts 列出冲突.
func (cd *ConflictDetector) ListConflicts(filter *ConflictFilter) []*ConflictRecord {
	var result []*ConflictRecord

	cd.conflicts.Range(func(key, value interface{}) bool {
		conflict, ok := value.(*ConflictRecord)
		if !ok {
			return true
		}

		if filter != nil {
			if filter.FilePath != "" && conflict.FilePath != filter.FilePath {
				return true
			}
			if filter.Type != "" && conflict.Type != filter.Type {
				return true
			}
			if filter.Status != 0 && conflict.Status != filter.Status {
				return true
			}
			if filter.Severity != 0 && conflict.Severity != filter.Severity {
				return true
			}
		}

		result = append(result, conflict)
		return true
	})

	// 按时间降序排序
	sortConflictsByTime(result)
	return result
}

// GetActiveConflicts 获取文件的活跃冲突.
func (cd *ConflictDetector) GetActiveConflicts(filePath string) []*ConflictRecord {
	raw, ok := cd.activeConflicts.Load(filePath)
	if !ok {
		return nil
	}

	ids, ok := raw.([]string)
	if !ok {
		return nil
	}

	var result []*ConflictRecord
	for _, conflictID := range ids {
		if conflict, err := cd.GetConflict(conflictID); err == nil {
			if conflict.Status == ConflictStatusPending || conflict.Status == ConflictStatusInProgress {
				result = append(result, conflict)
			}
		}
	}

	return result
}

// ========== 冲突解决 ==========.

// ResolveConflict 解决冲突.
func (cd *ConflictDetector) ResolveConflict(conflictID, resolvedBy, resolution string, params map[string]interface{}) error {
	conflict, err := cd.GetConflict(conflictID)
	if err != nil {
		return err
	}

	conflict.mu.Lock()
	defer conflict.mu.Unlock()

	// 应用解决方案
	switch resolution {
	case "force_acquire":
		if err := cd.resolveForceAcquire(conflict, resolvedBy); err != nil {
			return err
		}
	case "wait":
		cd.resolveWait(conflict, resolvedBy, params)
	case "rollback":
		if err := cd.resolveRollback(conflict, resolvedBy); err != nil {
			return err
		}
	case "version":
		if err := cd.resolveVersion(conflict, resolvedBy); err != nil {
			return err
		}
	case "ignore":
		cd.resolveIgnore(conflict, resolvedBy)
	default:
		return errors.New("unknown resolution type")
	}

	now := time.Now()
	conflict.ResolvedAt = &now
	conflict.ResolvedBy = resolvedBy
	conflict.Resolution = resolution
	conflict.Status = ConflictStatusResolved

	// 从活跃索引移除
	cd.removeFromActiveIndex(conflict.FilePath, conflictID)

	return nil
}

// EscalateConflict 升级冲突.
func (cd *ConflictDetector) EscalateConflict(conflictID, escalatedBy string, reason string) error {
	conflict, err := cd.GetConflict(conflictID)
	if err != nil {
		return err
	}

	conflict.mu.Lock()
	conflict.Status = ConflictStatusEscalated
	conflict.Severity = SeverityCritical
	conflict.mu.Unlock()

	// 通知管理员
	if cd.config.NotificationEnabled {
		cd.notifyEscalation(conflict, escalatedBy, reason)
	}

	return nil
}

// ========== 内部方法 ==========.

func (cd *ConflictDetector) createLockConflict(req *ConflictDetectionRequest, lockConflict *LockConflict) *ConflictRecord {
	participants := []*ConflictParticipant{
		{
			UserID:   req.UserID,
			UserName: req.UserName,
			ClientID: req.ClientID,
			Role:     RoleInitiator,
		},
	}

	if lockConflict.ExistingLock != nil {
		participants = append(participants, &ConflictParticipant{
			UserID:   lockConflict.ExistingLock.Owner,
			UserName: lockConflict.ExistingLock.OwnerName,
			Role:     RoleOwner,
		})
	}

	conflictType := ExtendedConflictExclusiveLock
	if lockConflict.ConflictType == ConflictTypeShared {
		conflictType = ExtendedConflictSharedLock
	}

	conflict := cd.CreateConflict(
		conflictType,
		req.FilePath,
		participants,
		lockConflict.Message,
	)

	if lockConflict.ExistingLock != nil {
		conflict.RelatedLocks = []string{lockConflict.ExistingLock.ID}
	}

	return conflict
}

func (cd *ConflictDetector) detectEditCollision(req *ConflictDetectionRequest) *ConflictRecord {
	// 检查是否有其他用户正在编辑
	if cd.lockManager == nil {
		return nil
	}

	info, err := cd.lockManager.GetLockByPath(req.FilePath)
	if err != nil {
		return nil
	}

	// 如果是共享锁，检查是否有多个协作者
	if info.LockType == "shared" && len(info.SharedOwners) > 1 {
		// 检测潜在编辑碰撞
		for _, owner := range info.SharedOwners {
			if owner.Owner != req.UserID {
				// 存在其他协作者，可能发生编辑碰撞
				return cd.CreateConflict(
					ExtendedConflictEditCollision,
					req.FilePath,
					[]*ConflictParticipant{
						{UserID: req.UserID, UserName: req.UserName, Role: RoleInitiator},
						{UserID: owner.Owner, UserName: owner.OwnerName, Role: RoleCollaborator},
					},
					fmt.Sprintf("Multiple users are editing %s simultaneously", extractFileName(req.FilePath)),
				)
			}
		}
	}

	return nil
}

func (cd *ConflictDetector) detectVersionConflict(req *ConflictDetectionRequest) *ConflictRecord {
	if cd.versionManager == nil {
		return nil
	}

	// 获取基础版本
	baseVersion, err := cd.versionManager.GetVersion(req.BaseVersionID)
	if err != nil {
		return nil
	}

	// 获取最新版本
	latestVersion := cd.versionManager.GetLatestVersion(req.FilePath)
	if latestVersion == nil || latestVersion.ID == baseVersion.ID {
		return nil
	}

	// 存在版本冲突
	return cd.CreateConflict(
		ExtendedConflictVersion,
		req.FilePath,
		[]*ConflictParticipant{
			{UserID: req.UserID, UserName: req.UserName, Role: RoleInitiator},
			{UserID: latestVersion.CreatedBy, UserName: latestVersion.CreatorName, Role: RoleCollaborator},
		},
		fmt.Sprintf("Version conflict: base version %d is outdated, latest is %d",
			baseVersion.VersionNumber, latestVersion.VersionNumber),
	)
}

func (cd *ConflictDetector) determineSeverity(conflictType ExtendedConflictType) ConflictSeverity {
	switch conflictType {
	case ExtendedConflictEditCollision:
		return SeverityMedium
	case ExtendedConflictVersion:
		return SeverityHigh
	case ExtendedConflictPermission:
		return SeverityLow
	case ExtendedConflictResource:
		return SeverityCritical
	default:
		return SeverityMedium
	}
}

func (cd *ConflictDetector) generateSuggestions(conflictType ExtendedConflictType) []*ConflictSuggestion {
	switch conflictType {
	case ExtendedConflictExclusiveLock:
		return []*ConflictSuggestion{
			{ID: "wait", Type: SuggestionWait, Priority: 1, Description: "Wait for the lock to be released", AutoApplicable: false},
			{ID: "notify", Type: SuggestionNotify, Priority: 2, Description: "Notify the lock owner", AutoApplicable: true},
			{ID: "force", Type: SuggestionForce, Priority: 3, Description: "Force acquire (admin only)", AutoApplicable: false, Risks: []string{"May cause data loss for the current lock owner"}},
		}
	case ExtendedConflictSharedLock:
		return []*ConflictSuggestion{
			{ID: "join", Type: SuggestionWait, Priority: 1, Description: "Join as a collaborator", AutoApplicable: true},
			{ID: "wait", Type: SuggestionWait, Priority: 2, Description: "Wait for exclusive access", AutoApplicable: false},
		}
	case ExtendedConflictEditCollision:
		return []*ConflictSuggestion{
			{ID: "version", Type: SuggestionVersion, Priority: 1, Description: "Create a new version before editing", AutoApplicable: true},
			{ID: "merge", Type: SuggestionMerge, Priority: 2, Description: "Merge changes", AutoApplicable: false},
			{ID: "split", Type: SuggestionSplit, Priority: 3, Description: "Split work into different sections", AutoApplicable: false},
		}
	case ExtendedConflictVersion:
		return []*ConflictSuggestion{
			{ID: "merge", Type: SuggestionMerge, Priority: 1, Description: "Merge with latest version", AutoApplicable: false},
			{ID: "rollback", Type: SuggestionRollback, Priority: 2, Description: "Rollback to base version", AutoApplicable: false, Risks: []string{"Will lose changes after base version"}},
		}
	default:
		return []*ConflictSuggestion{
			{ID: "wait", Type: SuggestionWait, Priority: 1, Description: "Wait and retry", AutoApplicable: false},
		}
	}
}

func (cd *ConflictDetector) resolveForceAcquire(conflict *ConflictRecord, resolvedBy string) error {
	if cd.lockManager == nil {
		return errors.New("lock manager not available")
	}

	// 记录解决者（用于审计）
	conflict.mu.Lock()
	if conflict.Metadata == nil {
		conflict.Metadata = make(map[string]interface{})
	}
	conflict.Metadata["force_acquired_by"] = resolvedBy
	conflict.mu.Unlock()

	// 强制释放现有锁
	for _, lockID := range conflict.RelatedLocks {
		if err := cd.lockManager.ForceUnlock(lockID, "resolved conflict"); err != nil {
			return err
		}
	}

	return nil
}

func (cd *ConflictDetector) resolveWait(conflict *ConflictRecord, resolvedBy string, params map[string]interface{}) {
	// 标记为处理中
	conflict.Status = ConflictStatusInProgress

	// 记录等待参数和解决者
	if conflict.Metadata == nil {
		conflict.Metadata = make(map[string]interface{})
	}
	conflict.Metadata["wait_resolved_by"] = resolvedBy
	if params != nil {
		conflict.Metadata["wait_params"] = params
	}
}

func (cd *ConflictDetector) resolveRollback(conflict *ConflictRecord, resolvedBy string) error {
	if cd.versionManager == nil {
		return errors.New("version manager not available")
	}

	// 找到基础版本
	if len(conflict.RelatedVersions) > 0 {
		ctx := context.Background()
		return cd.versionManager.RestoreVersion(ctx, conflict.RelatedVersions[0], resolvedBy)
	}

	return nil
}

func (cd *ConflictDetector) resolveVersion(conflict *ConflictRecord, resolvedBy string) error {
	if cd.versionManager == nil {
		return errors.New("version manager not available")
	}

	// 创建新版本
	ctx := context.Background()
	_, err := cd.versionManager.CreateVersion(ctx, conflict.FilePath, resolvedBy, "", VersionTypeManual, "Created to resolve conflict")
	return err
}

func (cd *ConflictDetector) resolveIgnore(conflict *ConflictRecord, resolvedBy string) {
	conflict.Status = ConflictStatusIgnored
	// 记录忽略者（用于审计）
	conflict.mu.Lock()
	if conflict.Metadata == nil {
		conflict.Metadata = make(map[string]interface{})
	}
	conflict.Metadata["ignored_by"] = resolvedBy
	conflict.mu.Unlock()
}

func (cd *ConflictDetector) addToActiveIndex(filePath, conflictID string) {
	raw, _ := cd.activeConflicts.LoadOrStore(filePath, &[]string{})
	ids, ok := raw.(*[]string)
	if ok {
		*ids = append(*ids, conflictID)
	}
}

func (cd *ConflictDetector) removeFromActiveIndex(filePath, conflictID string) {
	raw, ok := cd.activeConflicts.Load(filePath)
	if !ok {
		return
	}
	ids, ok := raw.(*[]string)
	if !ok {
		return
	}
	for i, id := range *ids {
		if id == conflictID {
			*ids = append((*ids)[:i], (*ids)[i+1:]...)
			return
		}
	}
}

func (cd *ConflictDetector) notifyConflict(conflict *ConflictRecord) {
	notification := &ConflictNotification{
		Type:        NotificationTypeConflictDetected,
		ConflictID:  conflict.ID,
		FilePath:    conflict.FilePath,
		FileName:    conflict.FileName,
		Severity:    conflict.Severity.String(),
		Description: conflict.Description,
		Timestamp:   time.Now(),
	}

	for _, participant := range conflict.Participants {
		notification.Recipients = append(notification.Recipients, participant.UserID)
	}

	select {
	case cd.notificationChan <- notification:
	default:
		// 通道满了，丢弃通知
	}
}

func (cd *ConflictDetector) notifyEscalation(conflict *ConflictRecord, escalatedBy, reason string) {
	notification := &ConflictNotification{
		Type:        NotificationTypeConflictEscalated,
		ConflictID:  conflict.ID,
		FilePath:    conflict.FilePath,
		FileName:    conflict.FileName,
		Severity:    "critical",
		Description: fmt.Sprintf("Conflict escalated: %s", reason),
		Timestamp:   time.Now(),
		Metadata: map[string]interface{}{
			"escalated_by": escalatedBy,
			"reason":       reason,
		},
	}

	select {
	case cd.notificationChan <- notification:
	default:
	}
}

// ========== 后台任务 ==========.

func (cd *ConflictDetector) detectionLoop() {
	defer cd.wg.Done()

	ticker := time.NewTicker(cd.config.DetectionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-cd.stopCh:
			return
		case <-ticker.C:
			cd.runDetection()
		}
	}
}

func (cd *ConflictDetector) runDetection() {
	// 检查活跃锁的状态
	if cd.lockManager != nil {
		locks := cd.lockManager.ListLocks(nil)
		for _, lock := range locks {
			// 检查等待队列中是否有长时间等待的请求
			if lock.WaitQueueSize > 0 {
				// 可能存在冲突
				activeConflicts := cd.GetActiveConflicts(lock.FilePath)
				if len(activeConflicts) == 0 {
					// 创建新的冲突记录
					cd.CreateConflict(
						ExtendedConflictExclusiveLock,
						lock.FilePath,
						[]*ConflictParticipant{
							{UserID: lock.Owner, UserName: lock.OwnerName, Role: RoleOwner},
						},
						fmt.Sprintf("File %s has %d pending requests in wait queue", lock.FileName, lock.WaitQueueSize),
					)
				}
			}
		}
	}
}

func (cd *ConflictDetector) resolutionLoop() {
	defer cd.wg.Done()

	ticker := time.NewTicker(time.Minute * 5)
	defer ticker.Stop()

	for {
		select {
		case <-cd.stopCh:
			return
		case <-ticker.C:
			cd.tryAutoResolve()
		}
	}
}

func (cd *ConflictDetector) tryAutoResolve() {
	if !cd.config.EnableAutoResolution {
		return
	}

	// 查找可自动解决的冲突
	conflicts := cd.ListConflicts(&ConflictFilter{Status: ConflictStatusPending})

	for _, conflict := range conflicts {
		for _, suggestion := range conflict.Suggestions {
			if suggestion.AutoApplicable && suggestion.Priority == 1 {
				// 尝试自动解决
				_ = cd.ResolveConflict(conflict.ID, "system", string(suggestion.Type), nil) //nolint:errcheck // 自动解决失败不影响流程
				break
			}
		}
	}
}

func (cd *ConflictDetector) cleanupLoop() {
	defer cd.wg.Done()

	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-cd.stopCh:
			return
		case <-ticker.C:
			cd.cleanupOldConflicts()
		}
	}
}

func (cd *ConflictDetector) cleanupOldConflicts() {
	cutoff := time.Now().Add(-cd.config.MaxConflictAge)

	cd.conflicts.Range(func(key, value interface{}) bool {
		conflict, ok := value.(*ConflictRecord)
		if !ok {
			return true
		}

		if conflict.CreatedAt.Before(cutoff) && conflict.Status == ConflictStatusResolved {
			cd.conflicts.Delete(key)
		}

		return true
	})
}

// Close 关闭检测器.
func (cd *ConflictDetector) Close() {
	close(cd.stopCh)
	cd.wg.Wait()
	close(cd.notificationChan)
}

// GetNotificationChannel 获取通知通道.
func (cd *ConflictDetector) GetNotificationChannel() <-chan *ConflictNotification {
	return cd.notificationChan
}

// ========== 统计信息 ==========.

// ConflictStats 冲突统计.
type ConflictStats struct {
	TotalConflicts    int64               `json:"totalConflicts"`
	ByType            map[string]int64    `json:"byType"`
	ByStatus          map[string]int64    `json:"byStatus"`
	BySeverity        map[string]int64    `json:"bySeverity"`
	ResolutionRate    float64             `json:"resolutionRate"`
	AvgResolutionTime int64               `json:"avgResolutionTime"` // ms
	TopFiles          []FileConflictCount `json:"topFiles"`
}

// FileConflictCount 文件冲突统计.
type FileConflictCount struct {
	FilePath string `json:"filePath"`
	FileName string `json:"fileName"`
	Count    int64  `json:"count"`
}

// GetConflictStats 获取冲突统计.
func (cd *ConflictDetector) GetConflictStats() *ConflictStats {
	stats := &ConflictStats{
		ByType:     make(map[string]int64),
		ByStatus:   make(map[string]int64),
		BySeverity: make(map[string]int64),
	}

	fileCounts := make(map[string]int64)
	var totalResolutionTime int64
	var resolvedCount int64

	cd.conflicts.Range(func(key, value interface{}) bool {
		conflict, ok := value.(*ConflictRecord)
		if !ok {
			return true
		}

		stats.TotalConflicts++
		stats.ByType[string(conflict.Type)]++
		stats.ByStatus[conflict.Status.String()]++
		stats.BySeverity[conflict.Severity.String()]++

		fileCounts[conflict.FilePath]++

		if conflict.ResolvedAt != nil {
			resolvedCount++
			duration := conflict.ResolvedAt.Sub(conflict.CreatedAt).Milliseconds()
			totalResolutionTime += duration
		}

		return true
	})

	// 计算解决率
	if stats.TotalConflicts > 0 {
		stats.ResolutionRate = float64(resolvedCount) / float64(stats.TotalConflicts) * 100
	}

	// 计算平均解决时间
	if resolvedCount > 0 {
		stats.AvgResolutionTime = totalResolutionTime / resolvedCount
	}

	// Top文件
	for path, count := range fileCounts {
		stats.TopFiles = append(stats.TopFiles, FileConflictCount{
			FilePath: path,
			FileName: extractFileName(path),
			Count:    count,
		})
	}

	sortFileConflictCounts(stats.TopFiles)
	if len(stats.TopFiles) > 10 {
		stats.TopFiles = stats.TopFiles[:10]
	}

	return stats
}

// ========== API 类型 ==========.

// ConflictDetectionRequest 冲突检测请求.
type ConflictDetectionRequest struct {
	FilePath      string   `json:"filePath" binding:"required"`
	Operation     string   `json:"operation"` // read/write/edit/delete
	LockType      LockType `json:"lockType"`
	UserID        string   `json:"userId" binding:"required"`
	UserName      string   `json:"userName"`
	ClientID      string   `json:"clientId"`
	BaseVersionID string   `json:"baseVersionId,omitempty"`
}

// ConflictResolutionRequest 冲突解决请求.
type ConflictResolutionRequest struct {
	ConflictID string                 `json:"conflictId" binding:"required"`
	Resolution string                 `json:"resolution" binding:"required"`
	Params     map[string]interface{} `json:"params,omitempty"`
}

// ConflictFilter 冲突过滤器.
type ConflictFilter struct {
	FilePath string               `json:"filePath,omitempty"`
	Type     ExtendedConflictType `json:"type,omitempty"`
	Status   ConflictRecordStatus `json:"status,omitempty"`
	Severity ConflictSeverity     `json:"severity,omitempty"`
}

// ConflictNotification 冲突通知.
type ConflictNotification struct {
	Type        NotificationTypeConflict `json:"type"`
	ConflictID  string                   `json:"conflictId"`
	FilePath    string                   `json:"filePath"`
	FileName    string                   `json:"fileName"`
	Severity    string                   `json:"severity"`
	Description string                   `json:"description"`
	Recipients  []string                 `json:"recipients,omitempty"`
	Timestamp   time.Time                `json:"timestamp"`
	Metadata    map[string]interface{}   `json:"metadata,omitempty"`
}

// NotificationTypeConflict 冲突通知类型.
type NotificationTypeConflict string

const (
	// NotificationTypeConflictDetected 冲突检测到.
	NotificationTypeConflictDetected NotificationTypeConflict = "conflict_detected"
	// NotificationTypeConflictResolved 冲突已解决.
	NotificationTypeConflictResolved NotificationTypeConflict = "conflict_resolved"
	// NotificationTypeConflictEscalated 冲突已升级.
	NotificationTypeConflictEscalated NotificationTypeConflict = "conflict_escalated"
	// NotificationTypeConflictExpired 冲突已过期.
	NotificationTypeConflictExpired NotificationTypeConflict = "conflict_expired"
)

// ========== 辅助函数 ==========.

func extractFileName(filePath string) string {
	for i := len(filePath) - 1; i >= 0; i-- {
		if filePath[i] == '/' {
			return filePath[i+1:]
		}
	}
	return filePath
}

func sortConflictsByTime(conflicts []*ConflictRecord) {
	for i := 0; i < len(conflicts)-1; i++ {
		for j := i + 1; j < len(conflicts); j++ {
			if conflicts[j].CreatedAt.After(conflicts[i].CreatedAt) {
				conflicts[i], conflicts[j] = conflicts[j], conflicts[i]
			}
		}
	}
}

func sortFileConflictCounts(counts []FileConflictCount) {
	for i := 0; i < len(counts)-1; i++ {
		for j := i + 1; j < len(counts); j++ {
			if counts[j].Count > counts[i].Count {
				counts[i], counts[j] = counts[j], counts[i]
			}
		}
	}
}

// ========== 集成管理器 ==========.

// CollaborativeLockManagerWithConflict 带冲突检测的协作锁管理器.
type CollaborativeLockManagerWithConflict struct {
	lockManager      *Manager
	conflictDetector *ConflictDetector
}

// NewCollaborativeLockManagerWithConflict 创建协作锁管理器.
func NewCollaborativeLockManagerWithConflict(lockManager *Manager, conflictDetector *ConflictDetector) *CollaborativeLockManagerWithConflict {
	return &CollaborativeLockManagerWithConflict{
		lockManager:      lockManager,
		conflictDetector: conflictDetector,
	}
}

// LockWithConflictDetection 带冲突检测的锁定.
func (m *CollaborativeLockManagerWithConflict) LockWithConflictDetection(ctx context.Context, req *LockRequest) (*FileLock, *ConflictRecord, error) {
	// 先检测冲突
	detectionReq := &ConflictDetectionRequest{
		FilePath:  req.FilePath,
		Operation: "lock",
		LockType:  req.LockType,
		UserID:    req.Owner,
		UserName:  req.OwnerName,
		ClientID:  req.ClientID,
	}

	conflict, err := m.conflictDetector.DetectConflict(ctx, detectionReq)
	if err != nil {
		return nil, nil, err
	}

	// 如果存在冲突，返回
	if conflict != nil {
		return nil, conflict, ErrLockConflict
	}

	// 尝试获取锁
	lock, lockConflict, err := m.lockManager.Lock(req)
	if err != nil && lockConflict != nil {
		// 创建冲突记录
		conflict = m.conflictDetector.CreateConflict(
			ExtendedConflictExclusiveLock,
			req.FilePath,
			[]*ConflictParticipant{
				{UserID: req.Owner, UserName: req.OwnerName, ClientID: req.ClientID, Role: RoleInitiator},
				{UserID: lockConflict.ExistingLock.Owner, UserName: lockConflict.ExistingLock.OwnerName, Role: RoleOwner},
			},
			lockConflict.Message,
		)
		return nil, conflict, err
	}

	return lock, nil, err
}
