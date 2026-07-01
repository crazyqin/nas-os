package datasovereignty

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 错误定义 ==========

var (
	// ErrTagNotFound 标签未找到.
	ErrTagNotFound = errors.New("数据主权标签未找到")
	// ErrTagAlreadyExists 资源已存在标签.
	ErrTagAlreadyExists = errors.New("该资源已存在数据主权标签")
	// ErrTransferBlocked 传输被合规策略阻止.
	ErrTransferBlocked = errors.New("数据传输被合规策略阻止")
	// ErrInvalidRegion 无效区域.
	ErrInvalidRegion = errors.New("无效的数据区域")
	// ErrInvalidFramework 无效合规框架.
	ErrInvalidFramework = errors.New("无效的合规框架")
)

// ========== 合规规则 ==========

// frameworkRegions 合规框架对应的强制数据驻留区域.
var frameworkRegions = map[ComplianceFramework][]DataRegion{
	FrameworkGDPR: {RegionEU},           // GDPR 要求数据留在欧盟
	FrameworkPIPL: {RegionCN},           // PIPL 要求数据留在中国大陆
	FrameworkCCPA: {RegionUS, RegionCA}, // CCPA 适用于北美
	FrameworkLGPD: {RegionBR},           // LGPD 要求数据留在巴西
	FrameworkPDPA: {RegionSG},           // PDPA 适用于新加坡
}

// validRegions 有效的区域集合.
var validRegions = map[DataRegion]bool{
	RegionEU: true, RegionCN: true, RegionUS: true, RegionCA: true,
	RegionBR: true, RegionSG: true, RegionGlobal: true,
}

// validFrameworks 有效的合规框架集合.
var validFrameworks = map[ComplianceFramework]bool{
	FrameworkGDPR: true, FrameworkPIPL: true, FrameworkCCPA: true,
	FrameworkLGPD: true, FrameworkPDPA: true,
}

// ========== 服务定义 ==========

// Service 数据主权标签服务，管理标签、合规检查和审计日志.
type Service struct {
	mu      sync.RWMutex
	tags    map[string]*SovereigntyTag // tagID -> SovereigntyTag
	pathIdx map[string]string          // resourcePath -> tagID （快速路径查找）
	audit   []AuditEntry               // 审计日志（按时间顺序追加）
}

// NewService 创建数据主权标签服务.
func NewService() *Service {
	return &Service{
		tags:    make(map[string]*SovereigntyTag),
		pathIdx: make(map[string]string),
		audit:   make([]AuditEntry, 0),
	}
}

// ========== 标签管理 ==========

// CreateTag 创建数据主权标签.
func (s *Service) CreateTag(req TagRequest) (*SovereigntyTag, error) {
	// 验证合规框架
	for _, fw := range req.Frameworks {
		if !validFrameworks[fw] {
			return nil, fmt.Errorf("%w: %s", ErrInvalidFramework, fw)
		}
	}

	// 验证区域
	for _, r := range req.AllowedRegions {
		if !validRegions[r] {
			return nil, fmt.Errorf("%w: %s", ErrInvalidRegion, r)
		}
	}
	for _, r := range req.RestrictedRegions {
		if !validRegions[r] {
			return nil, fmt.Errorf("%w: %s", ErrInvalidRegion, r)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查资源是否已有标签
	if _, exists := s.pathIdx[req.ResourcePath]; exists {
		return nil, ErrTagAlreadyExists
	}

	// 根据合规框架自动补充强制区域
	allowed := make(map[DataRegion]bool)
	for _, r := range req.AllowedRegions {
		allowed[r] = true
	}
	for _, fw := range req.Frameworks {
		for _, r := range frameworkRegions[fw] {
			allowed[r] = true
		}
	}
	for _, r := range req.RestrictedRegions {
		delete(allowed, r)
	}

	// 构建最终允许区域列表
	allowedList := make([]DataRegion, 0, len(allowed))
	for r := range allowed {
		allowedList = append(allowedList, r)
	}

	now := time.Now()
	tag := &SovereigntyTag{
		ID:               uuid.New().String(),
		ResourcePath:     req.ResourcePath,
		ResourceType:     req.ResourceType,
		Frameworks:       req.Frameworks,
		AllowedRegions:   allowedList,
		RestrictedRegions: req.RestrictedRegions,
		DataSubject:      req.DataSubject,
		Description:      req.Description,
		CreatedAt:        now,
		UpdatedAt:        now,
		CreatedBy:        req.CreatedBy,
	}

	s.tags[tag.ID] = tag
	s.pathIdx[req.ResourcePath] = tag.ID

	return tag, nil
}

// DeleteTag 删除数据主权标签.
func (s *Service) DeleteTag(tagID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tag, exists := s.tags[tagID]
	if !exists {
		return ErrTagNotFound
	}

	delete(s.tags, tagID)
	delete(s.pathIdx, tag.ResourcePath)
	return nil
}

// GetTag 根据 ID 获取标签.
func (s *Service) GetTag(tagID string) (*SovereigntyTag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tag, exists := s.tags[tagID]
	if !exists {
		return nil, ErrTagNotFound
	}
	return tag, nil
}

// GetTagByPath 根据资源路径获取标签.
func (s *Service) GetTagByPath(path string) (*SovereigntyTag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tagID, exists := s.pathIdx[path]
	if !exists {
		return nil, ErrTagNotFound
	}
	return s.tags[tagID], nil
}

// ListTags 列出所有数据主权标签.
func (s *Service) ListTags() []*SovereigntyTag {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tags := make([]*SovereigntyTag, 0, len(s.tags))
	for _, t := range s.tags {
		tags = append(tags, t)
	}
	return tags
}

// ========== 合规检查 ==========

// CheckTransfer 检查数据传输是否符合合规要求.
func (s *Service) CheckTransfer(req CheckRequest) (*CheckResponse, error) {
	// 验证目标区域
	if !validRegions[req.TargetRegion] {
		return nil, fmt.Errorf("%w: %s", ErrInvalidRegion, req.TargetRegion)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 查找资源标签
	tagID, hasTag := s.pathIdx[req.ResourcePath]
	var tag *SovereigntyTag
	if hasTag {
		tag = s.tags[tagID]
	}

	// 无标签的资源不做限制（但记录审计日志）
	if tag == nil {
		entry := AuditEntry{
			ID:           uuid.New().String(),
			Timestamp:    time.Now(),
			ResourcePath: req.ResourcePath,
			ResourceType: ResourceFile,
			Action:       req.Action,
			SourceRegion: RegionGlobal,
			TargetRegion: req.TargetRegion,
			Status:       TransferAllowed,
			User:         req.User,
			ClientIP:     req.ClientIP,
			Reason:       "资源无主权标签，允许传输",
		}
		s.audit = append(s.audit, entry)

		return &CheckResponse{
			Allowed: true,
			Status:  TransferAllowed,
			Reason:  "资源无主权标签，允许传输",
			EntryID: entry.ID,
		}, nil
	}

	// 检查目标区域是否在允许列表中
	allowed := false
	for _, r := range tag.AllowedRegions {
		if r == req.TargetRegion || r == RegionGlobal {
			allowed = true
			break
		}
	}

	// 检查目标区域是否在禁止列表中
	for _, r := range tag.RestrictedRegions {
		if r == req.TargetRegion {
			allowed = false
			break
		}
	}

	// 检查合规框架强制区域
	for _, fw := range tag.Frameworks {
		fwRegions := frameworkRegions[fw]
		if len(fwRegions) > 0 {
			regionMatch := false
			for _, r := range fwRegions {
				if r == req.TargetRegion {
					regionMatch = true
					break
				}
			}
			if !regionMatch && req.TargetRegion != RegionGlobal {
				allowed = false
			}
		}
	}

	// 构建审计日志
	var status TransferStatus
	var reason string
	if allowed {
		status = TransferAllowed
		reason = "传输符合合规要求"
	} else {
		status = TransferBlocked
		fwNames := make([]string, 0, len(tag.Frameworks))
		for _, fw := range tag.Frameworks {
			fwNames = append(fwNames, string(fw))
		}
		reason = fmt.Sprintf("传输到区域 %s 违反合规框架 [%s] 的数据驻留要求",
			req.TargetRegion, strings.Join(fwNames, ", "))
	}

	entry := AuditEntry{
		ID:           uuid.New().String(),
		Timestamp:    time.Now(),
		ResourcePath: req.ResourcePath,
		ResourceType: tag.ResourceType,
		Action:       req.Action,
		SourceRegion: RegionGlobal, // 简化：实际应查询资源当前所在区域
		TargetRegion: req.TargetRegion,
		Status:       status,
		User:         req.User,
		ClientIP:     req.ClientIP,
		Reason:       reason,
		TagID:        tag.ID,
	}
	s.audit = append(s.audit, entry)

	return &CheckResponse{
		Allowed: allowed,
		Status:  status,
		Tag:     tag,
		Reason:  reason,
		EntryID: entry.ID,
	}, nil
}

// ========== 审计日志 ==========

// QueryAudit 查询审计日志.
func (s *Service) QueryAudit(query AuditQuery) []AuditEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []AuditEntry
	for i := len(s.audit) - 1; i >= 0; i-- { // 倒序，最新在前
		entry := s.audit[i]

		// 过滤条件
		if query.ResourcePath != "" && entry.ResourcePath != query.ResourcePath {
			continue
		}
		if query.Action != "" && entry.Action != query.Action {
			continue
		}
		if query.Status != "" && entry.Status != query.Status {
			continue
		}
		if query.User != "" && entry.User != query.User {
			continue
		}
		if query.StartTime != nil && entry.Timestamp.Before(*query.StartTime) {
			continue
		}
		if query.EndTime != nil && entry.Timestamp.After(*query.EndTime) {
			continue
		}

		results = append(results, entry)

		// 限制返回数量
		if query.Limit > 0 && len(results) >= query.Limit {
			break
		}
	}

	if results == nil {
		results = []AuditEntry{}
	}
	return results
}

// GetAuditCount 获取审计日志总数.
func (s *Service) GetAuditCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.audit)
}
