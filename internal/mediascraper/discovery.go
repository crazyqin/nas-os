package mediascraper

import (
	"sort"
	"strings"
	"time"
)

const (
	ShelfTopRated    = "top_rated"
	ShelfRecentlyAdd = "recently_added"
	ShelfNewRelease  = "new_release"
	ShelfGenrePrefix = "genre:"
)

// DiscoveryDigest 面向首页的媒体发现摘要，融合飞牛影视墙的推荐位与群晖 Photos 的智能聚合体验.
type DiscoveryDigest struct {
	Hero       *MediaItem        `json:"hero,omitempty"` // 首页主推海报
	Shelves    []*DiscoveryShelf `json:"shelves"`        // 智能横向内容架
	GenreStats []GenreStat       `json:"genre_stats"`    // 类型分布，用于筛选 chips/侧边栏
	Stats      DiscoveryStats    `json:"stats"`          // 内容库概览
	UpdatedAt  time.Time         `json:"updated_at"`     // 生成时间
}

// DiscoveryShelf 智能内容架.
type DiscoveryShelf struct {
	ID       string       `json:"id"`       // 稳定 ID，便于前端缓存和埋点
	Title    string       `json:"title"`    // 展示标题
	Subtitle string       `json:"subtitle"` // 简短解释推荐理由
	Items    []*MediaItem `json:"items"`    // 内容项
}

// GenreStat 类型标签统计.
type GenreStat struct {
	Genre string `json:"genre"`
	Count int    `json:"count"`
}

// DiscoveryStats 内容库统计.
type DiscoveryStats struct {
	Total       int     `json:"total"`
	Movies      int     `json:"movies"`
	TVSeries    int     `json:"tv_series"`
	AverageRate float64 `json:"average_rate"`
	LatestYear  int     `json:"latest_year"`
}

// BuildDiscoveryDigest 从海报墙生成首页发现摘要.
// limit 控制每个内容架最多展示多少项；小于等于 0 时默认展示 8 项.
func (b *PosterWallBuilder) BuildDiscoveryDigest(wall *PosterWall, limit int) *DiscoveryDigest {
	if limit <= 0 {
		limit = 8
	}

	items := flattenWallItems(wall)
	digest := &DiscoveryDigest{
		Shelves:    make([]*DiscoveryShelf, 0),
		GenreStats: make([]GenreStat, 0),
		UpdatedAt:  time.Now(),
	}
	if len(items) == 0 {
		return digest
	}

	digest.Stats = buildDiscoveryStats(items)
	digest.GenreStats = buildGenreStats(items)
	digest.Hero = pickHero(items)

	if shelf := newShelf(ShelfTopRated, "高分佳片", "按评分精选，适合首页优先展示", sortedCopy(items, byRatingDesc), limit); shelf != nil {
		digest.Shelves = append(digest.Shelves, shelf)
	}
	if shelf := newShelf(ShelfRecentlyAdd, "最近入库", "按刮削/入库时间展示新内容", sortedCopy(items, byScrapedAtDesc), limit); shelf != nil {
		digest.Shelves = append(digest.Shelves, shelf)
	}
	if shelf := newShelf(ShelfNewRelease, "新片新剧", "按上映年份发现较新的内容", sortedCopy(items, byYearDesc), limit); shelf != nil {
		digest.Shelves = append(digest.Shelves, shelf)
	}

	for _, stat := range digest.GenreStats {
		genreItems := filterByGenre(items, stat.Genre)
		if len(genreItems) < 2 {
			continue
		}
		shelf := newShelf(ShelfGenrePrefix+safeShelfID(stat.Genre), stat.Genre+"精选", "基于类型标签自动聚合", sortedCopy(genreItems, byRatingDesc), limit)
		if shelf != nil {
			digest.Shelves = append(digest.Shelves, shelf)
		}
		// 首页摘要保持克制，只生成最主要的 3 个类型架，避免前端首屏过长.
		if countGenreShelves(digest.Shelves) >= 3 {
			break
		}
	}

	return digest
}

func flattenWallItems(wall *PosterWall) []*MediaItem {
	if wall == nil {
		return nil
	}
	seen := make(map[string]struct{})
	items := make([]*MediaItem, 0, wall.Total)
	for _, group := range wall.Groups {
		if group == nil {
			continue
		}
		for _, item := range group.Items {
			if item == nil {
				continue
			}
			key := item.ID
			if key == "" {
				key = item.FilePath
			}
			if key == "" {
				key = string(item.Type) + ":" + strings.TrimSpace(item.Title) + ":" + itoaDiscovery(item.Year)
			}
			if key != "" {
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
			}
			items = append(items, item)
		}
	}
	return items
}

func buildDiscoveryStats(items []*MediaItem) DiscoveryStats {
	var stats DiscoveryStats
	var ratingSum float64
	for _, item := range items {
		stats.Total++
		switch item.Type {
		case MediaTypeMovie:
			stats.Movies++
		case MediaTypeTVSeries:
			stats.TVSeries++
		}
		if item.Rating > 0 {
			ratingSum += item.Rating
		}
		if item.Year > stats.LatestYear {
			stats.LatestYear = item.Year
		}
	}
	ratedCount := 0
	for _, item := range items {
		if item.Rating > 0 {
			ratedCount++
		}
	}
	if ratedCount > 0 {
		stats.AverageRate = round1(ratingSum / float64(ratedCount))
	}
	return stats
}

func buildGenreStats(items []*MediaItem) []GenreStat {
	counts := make(map[string]int)
	for _, item := range items {
		for _, genre := range item.Genres {
			genre = strings.TrimSpace(genre)
			if genre == "" {
				continue
			}
			counts[genre]++
		}
	}
	stats := make([]GenreStat, 0, len(counts))
	for genre, count := range counts {
		stats = append(stats, GenreStat{Genre: genre, Count: count})
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Count == stats[j].Count {
			return stats[i].Genre < stats[j].Genre
		}
		return stats[i].Count > stats[j].Count
	})
	return stats
}

func pickHero(items []*MediaItem) *MediaItem {
	sorted := sortedCopy(items, func(a, b *MediaItem) bool {
		if a.Rating == b.Rating {
			if a.Year == b.Year {
				return a.Title < b.Title
			}
			return a.Year > b.Year
		}
		return a.Rating > b.Rating
	})
	if len(sorted) == 0 {
		return nil
	}
	return sorted[0]
}

func newShelf(id, title, subtitle string, items []*MediaItem, limit int) *DiscoveryShelf {
	if len(items) == 0 {
		return nil
	}
	if len(items) > limit {
		items = items[:limit]
	}
	return &DiscoveryShelf{ID: id, Title: title, Subtitle: subtitle, Items: items}
}

func sortedCopy(items []*MediaItem, less func(a, b *MediaItem) bool) []*MediaItem {
	out := append([]*MediaItem(nil), items...)
	sort.SliceStable(out, func(i, j int) bool { return less(out[i], out[j]) })
	return out
}

func byRatingDesc(a, b *MediaItem) bool {
	if a.Rating == b.Rating {
		return a.Title < b.Title
	}
	return a.Rating > b.Rating
}

func byScrapedAtDesc(a, b *MediaItem) bool {
	if a.ScrapedAt.Equal(b.ScrapedAt) {
		return a.Title < b.Title
	}
	return a.ScrapedAt.After(b.ScrapedAt)
}

func byYearDesc(a, b *MediaItem) bool {
	if a.Year == b.Year {
		return byRatingDesc(a, b)
	}
	return a.Year > b.Year
}

func filterByGenre(items []*MediaItem, genre string) []*MediaItem {
	filtered := make([]*MediaItem, 0)
	for _, item := range items {
		for _, g := range item.Genres {
			if strings.TrimSpace(g) == genre {
				filtered = append(filtered, item)
				break
			}
		}
	}
	return filtered
}

func countGenreShelves(shelves []*DiscoveryShelf) int {
	count := 0
	for _, shelf := range shelves {
		if shelf != nil && strings.HasPrefix(shelf.ID, ShelfGenrePrefix) {
			count++
		}
	}
	return count
}

func round1(v float64) float64 {
	if v < 0 {
		return float64(int(v*10-0.5)) / 10
	}
	return float64(int(v*10+0.5)) / 10
}

func safeShelfID(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	var b strings.Builder
	lastDash := false
	for _, r := range v {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r > 127 {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func itoaDiscovery(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
