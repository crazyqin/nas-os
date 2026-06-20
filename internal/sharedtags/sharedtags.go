package sharedtags

import (
	"log"
)

// SharedTagsSystem is the main entry point for the shared tags system
type SharedTagsSystem struct {
	Manager  *TagManager
	Tagger   *FileTagger
	Search   *TagSearch
	Stats    *TagStats
	Share    *TagShareManager
}

// NewSharedTagsSystem creates a new SharedTagsSystem
func NewSharedTagsSystem() *SharedTagsSystem {
	manager := NewTagManager()
	tagger := NewFileTagger(manager)
	search := NewTagSearch(tagger, manager)
	stats := NewTagStats(manager, tagger)
	share := NewTagShareManager(manager)

	log.Println("共享标签系统已初始化")

	return &SharedTagsSystem{
		Manager:  manager,
		Tagger:   tagger,
		Search:   search,
		Stats:    stats,
		Share:    share,
	}
}

// Init initializes the system with sample data (for demo/testing)
func (s *SharedTagsSystem) Init() {
	log.Println("共享标签系统正在初始化示例数据...")

	// Create sample tags
	sampleTags := []struct {
		name       string
		desc       string
		categoryID string
		color      string
	}{
		{"重要", "重要文件标记", "cat-system-priority", "#FF0000"},
		{"待审核", "待审核文件", "cat-system-priority", "#FFA500"},
		{"已归档", "已归档文件", "cat-system-custom", "#808080"},
		{"合同", "合同文件", "cat-system-project", "#0000FF"},
		{"设计稿", "设计相关文件", "cat-system-project", "#00FF00"},
		{"技术文档", "技术文档", "cat-system-department", "#800080"},
		{"会议纪要", "会议记录", "cat-system-department", "#FFC0CB"},
	}

	for _, st := range sampleTags {
		_, _ = s.Manager.CreateTag(CreateTagRequest{
			Name:        st.name,
			Description: st.desc,
			CategoryID:  st.categoryID,
			Color:       st.color,
			Owner:       "system",
		})
	}

	// Create sample auto tag rules
	s.Tagger.AddAutoTagRule(
		"PDF文档",
		[]string{".pdf"},
		"",
		[]string{}, // Would need actual tag IDs
		"system",
	)
	s.Tagger.AddAutoTagRule(
		"设计文件",
		[]string{".psd", ".ai", ".sketch", ".fig"},
		"/设计",
		[]string{},
		"system",
	)

	log.Println("共享标签系统示例数据初始化完成")
}

// GetSystemSummary returns a summary of the entire system
func (s *SharedTagsSystem) GetSystemSummary() *SystemSummary {
	return &SystemSummary{
		TagSummary:  s.Stats.GetTagSummary(),
		ShareStats:  s.Share.GetShareStats(),
		TopTags:     s.Stats.GetTopTags(10),
		CategoryStats: s.Stats.GetCategoryStats(),
	}
}

// SystemSummary represents overall system summary
type SystemSummary struct {
	TagSummary    *TagSummary                `json:"tagSummary"`    // 标签统计摘要
	ShareStats    *ShareStats                `json:"shareStats"`    // 共享统计
	TopTags       []*TagStatsResult          `json:"topTags"`       // 热门标签
	CategoryStats map[string]*CategoryStats  `json:"categoryStats"` // 分类统计
}
