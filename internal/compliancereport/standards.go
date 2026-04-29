// Package compliancereport 提供合规标准管理功能
package compliancereport

// StandardsManager 合规标准管理器.
type StandardsManager struct {
	standards map[ComplianceStandard]StandardInfo
}

// NewStandardsManager 创建合规标准管理器.
func NewStandardsManager() *StandardsManager {
	sm := &StandardsManager{
		standards: make(map[ComplianceStandard]StandardInfo),
	}
	sm.registerDefaults()
	return sm
}

// registerDefaults 注册默认合规标准.
func (sm *StandardsManager) registerDefaults() {
	sm.standards[StandardGDPR] = StandardInfo{
		ID:          StandardGDPR,
		Name:        "GDPR (通用数据保护条例)",
		Description: "欧盟通用数据保护条例，保护自然人在个人数据处理方面的基本权利和自由",
		Version:     "2016/679",
		Categories:  []CheckCategory{CategoryAccessControl, CategoryDataEncryption, CategoryLogIntegrity},
	}

	sm.standards[StandardSOC2] = StandardInfo{
		ID:          StandardSOC2,
		Name:        "SOC 2 (服务组织控制)",
		Description: "基于 AICPA 信任服务标准的服务组织内部控制报告框架",
		Version:     "2017 Trust Services Criteria",
		Categories:  []CheckCategory{CategoryAccessControl, CategoryDataEncryption, CategoryLogIntegrity, CategoryBackup, CategoryNetwork},
	}

	sm.standards[StandardDJBH] = StandardInfo{
		ID:          StandardDJBH,
		Name:        "等保 2.0 (信息安全等级保护)",
		Description: "中国信息安全等级保护制度，网络安全等级保护基本要求",
		Version:     "GB/T 22239-2019",
		Categories:  []CheckCategory{CategoryAccessControl, CategoryDataEncryption, CategoryLogIntegrity, CategoryBackup, CategoryNetwork},
	}

	sm.standards[StandardISO27001] = StandardInfo{
		ID:          StandardISO27001,
		Name:        "ISO/IEC 27001",
		Description: "信息安全管理体系国际标准，建立、实施、维护和持续改进信息安全管理体系",
		Version:     "2022",
		Categories:  []CheckCategory{CategoryAccessControl, CategoryDataEncryption, CategoryLogIntegrity, CategoryBackup, CategoryNetwork},
	}

	sm.standards[StandardHIPAA] = StandardInfo{
		ID:          StandardHIPAA,
		Name:        "HIPAA (健康保险可携性与责任法案)",
		Description: "美国保护个人健康信息隐私和安全的联邦法律",
		Version:     "HITECH Act",
		Categories:  []CheckCategory{CategoryAccessControl, CategoryDataEncryption, CategoryLogIntegrity},
	}
}

// ListStandards 列出所有支持的合规标准.
func (sm *StandardsManager) ListStandards() []StandardInfo {
	standards := make([]StandardInfo, 0, len(sm.standards))
	for _, s := range sm.standards {
		standards = append(standards, s)
	}
	return standards
}

// GetStandard 获取指定合规标准信息.
func (sm *StandardsManager) GetStandard(id ComplianceStandard) (StandardInfo, bool) {
	s, ok := sm.standards[id]
	return s, ok
}

// IsSupported 检查标准是否被支持.
func (sm *StandardsManager) IsSupported(id ComplianceStandard) bool {
	_, ok := sm.standards[id]
	return ok
}
