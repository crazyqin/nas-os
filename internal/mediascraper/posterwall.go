package mediascraper

import (
	"sort"
	"time"
)

// PosterWallBuilder 海报墙构建器
// 负责将刮削后的媒体项按类型分组、排序，生成海报墙数据结构
type PosterWallBuilder struct{}

// NewPosterWallBuilder 创建海报墙构建器
func NewPosterWallBuilder() *PosterWallBuilder {
	return &PosterWallBuilder{}
}

// Build 从一组媒体项构建海报墙
// 按媒体类型分组，组内按评分降序排列，组间按电影→电视剧排列
func (b *PosterWallBuilder) Build(items []*MediaItem) *PosterWall {
	wall := &PosterWall{
		Groups:    make([]*PosterWallGroup, 0),
		UpdatedAt: time.Now(),
	}

	// 按类型分组
	groupMap := make(map[MediaType][]*MediaItem)
	for _, item := range items {
		if item == nil {
			continue
		}
		groupMap[item.Type] = append(groupMap[item.Type], item)
	}

	// 定义分组顺序：电影在前，电视剧在后
	order := []MediaType{MediaTypeMovie, MediaTypeTVSeries}
	titles := map[MediaType]string{
		MediaTypeMovie:    "电影",
		MediaTypeTVSeries: "电视剧",
	}

	for _, mt := range order {
		groupItems, ok := groupMap[mt]
		if !ok || len(groupItems) == 0 {
			continue
		}

		// 组内按评分降序排列
		sort.Slice(groupItems, func(i, j int) bool {
			return groupItems[i].Rating > groupItems[j].Rating
		})

		group := &PosterWallGroup{
			Type:  mt,
			Title: titles[mt],
			Items: groupItems,
			Count: len(groupItems),
		}
		wall.Groups = append(wall.Groups, group)
		wall.Total += len(groupItems)
	}

	return wall
}

// BuildFromResults 从刮削结果构建海报墙（仅包含成功刮削的项）
func (b *PosterWallBuilder) BuildFromResults(results []*ScraperResult) *PosterWall {
	items := make([]*MediaItem, 0, len(results))
	for _, r := range results {
		if r != nil && r.Found && r.Item != nil {
			items = append(items, r.Item)
		}
	}
	return b.Build(items)
}

// FilterByGenre 按类型标签过滤海报墙中的媒体项
func (b *PosterWallBuilder) FilterByGenre(wall *PosterWall, genre string) *PosterWall {
	filtered := &PosterWall{
		Groups:    make([]*PosterWallGroup, 0),
		UpdatedAt: time.Now(),
	}

	for _, group := range wall.Groups {
		filteredItems := make([]*MediaItem, 0)
		for _, item := range group.Items {
			for _, g := range item.Genres {
				if g == genre {
					filteredItems = append(filteredItems, item)
					break
				}
			}
		}
		if len(filteredItems) > 0 {
			filtered.Groups = append(filtered.Groups, &PosterWallGroup{
				Type:  group.Type,
				Title: group.Title,
				Items: filteredItems,
				Count: len(filteredItems),
			})
			filtered.Total += len(filteredItems)
		}
	}

	return filtered
}

// SortByYear 按年份降序重新排列海报墙中各组的媒体项
func (b *PosterWallBuilder) SortByYear(wall *PosterWall) {
	for _, group := range wall.Groups {
		sort.Slice(group.Items, func(i, j int) bool {
			return group.Items[i].Year > group.Items[j].Year
		})
	}
}

// SortByRating 按评分降序重新排列（默认行为，但提供显式方法）
func (b *PosterWallBuilder) SortByRating(wall *PosterWall) {
	for _, group := range wall.Groups {
		sort.Slice(group.Items, func(i, j int) bool {
			return group.Items[i].Rating > group.Items[j].Rating
		})
	}
}

// SortByTitle 按标题字母序排列
func (b *PosterWallBuilder) SortByTitle(wall *PosterWall) {
	for _, group := range wall.Groups {
		sort.Slice(group.Items, func(i, j int) bool {
			return group.Items[i].Title < group.Items[j].Title
		})
	}
}
