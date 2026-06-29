package mediascraper

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ========== 文件名解析测试 ==========

func TestParseFileName_MovieWithYear(t *testing.T) {
	tests := []struct {
		filename  string
		wantTitle string
		wantYear  int
		wantType  MediaType
	}{
		{"Inception 2010 1080p BluRay x264.mkv", "Inception", 2010, MediaTypeMovie},
		{"The.Dark.Knight.2008.1080p.BluRay.mkv", "The Dark Knight", 2008, MediaTypeMovie},
		{"interstellar_2014_720p_web-dl.mp4", "interstellar", 2014, MediaTypeMovie},
		{"Parasite 2019 REMUX 2160p.mkv", "Parasite", 2019, MediaTypeMovie},
		{"Inception(2010).mkv", "Inception", 2010, MediaTypeMovie},
		{"Inception.[2010].1080p.mkv", "Inception", 2010, MediaTypeMovie},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			title, year, mt, err := ParseFileName(tt.filename)
			if err != nil {
				t.Fatalf("ParseFileName(%q) 返回错误: %v", tt.filename, err)
			}
			if title != tt.wantTitle {
				t.Errorf("标题: got %q, want %q", title, tt.wantTitle)
			}
			if year != tt.wantYear {
				t.Errorf("年份: got %d, want %d", year, tt.wantYear)
			}
			if mt != tt.wantType {
				t.Errorf("类型: got %v, want %v", mt, tt.wantType)
			}
		})
	}
}

func TestParseFileName_TVSeries(t *testing.T) {
	tests := []struct {
		filename  string
		wantTitle string
		wantYear  int
		wantType  MediaType
	}{
		{"Breaking.Bad.S01E02.2008.1080p.mkv", "Breaking Bad", 2008, MediaTypeTVSeries},
		{"Game.of.Thrones.s01e01.2011.mkv", "Game of Thrones", 2011, MediaTypeTVSeries},
		{"Stranger Things S02E05 2016 WEB-DL.mkv", "Stranger Things", 2016, MediaTypeTVSeries},
		{"breaking_bad_s01e01.mp4", "breaking bad", 0, MediaTypeTVSeries},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			title, year, mt, err := ParseFileName(tt.filename)
			if err != nil {
				t.Fatalf("ParseFileName(%q) 返回错误: %v", tt.filename, err)
			}
			if title != tt.wantTitle {
				t.Errorf("标题: got %q, want %q", title, tt.wantTitle)
			}
			if year != tt.wantYear {
				t.Errorf("年份: got %d, want %d", year, tt.wantYear)
			}
			if mt != tt.wantType {
				t.Errorf("类型: got %v, want %v", mt, tt.wantType)
			}
		})
	}
}

func TestParseFileName_EmptyFilename(t *testing.T) {
	_, _, _, err := ParseFileName("")
	if err == nil {
		t.Error("空文件名应返回错误")
	}
}

func TestParseFileName_NoExtension(t *testing.T) {
	title, year, mt, err := ParseFileName("Inception 2010")
	if err != nil {
		t.Fatalf("无扩展名文件应正常解析: %v", err)
	}
	if title != "Inception" {
		t.Errorf("标题: got %q, want %q", title, "Inception")
	}
	if year != 2010 {
		t.Errorf("年份: got %d, want %d", year, 2010)
	}
	if mt != MediaTypeMovie {
		t.Errorf("类型: got %v, want %v", mt, MediaTypeMovie)
	}
}

// ========== 刮削器测试 ==========

func TestNewScraper(t *testing.T) {
	s := NewScraper()
	if s == nil {
		t.Fatal("NewScraper 返回 nil")
	}
	if len(s.movieDB) == 0 {
		t.Error("电影数据库不应为空")
	}
	if len(s.tvDB) == 0 {
		t.Error("电视剧数据库不应为空")
	}
	if len(s.subtitleDB) == 0 {
		t.Error("字幕数据库不应为空")
	}
}

func TestScraper_ScrapeMovie(t *testing.T) {
	s := NewScraper()

	tests := []struct {
		name      string
		filePath  string
		wantTitle string
		wantYear  int
		wantFound bool
	}{
		{"精确匹配", "/media/movies/Inception 2010 1080p BluRay.mkv", "Inception", 2010, true},
		{"点分隔", "/media/movies/The.Dark.Knight.2008.1080p.BluRay.mkv", "The Dark Knight", 2008, true},
		{"下划线分隔", "/media/movies/interstellar_2014_720p_web-dl.mp4", "Interstellar", 2014, true},
		{"寄生虫", "/media/movies/Parasite 2019 REMUX 2160p.mkv", "Parasite", 2019, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.Scrape(tt.filePath)
			if result.Error != nil {
				t.Fatalf("Scrape 返回错误: %v", result.Error)
			}
			if !result.Found {
				t.Fatal("应找到匹配的元数据")
			}
			if result.Item == nil {
				t.Fatal("Item 不应为 nil")
			}
			if result.Item.Title != tt.wantTitle {
				t.Errorf("标题: got %q, want %q", result.Item.Title, tt.wantTitle)
			}
			if result.Item.Year != tt.wantYear {
				t.Errorf("年份: got %d, want %d", result.Item.Year, tt.wantYear)
			}
			if result.Item.Type != MediaTypeMovie {
				t.Errorf("类型: got %v, want %v", result.Item.Type, MediaTypeMovie)
			}
			if result.Item.Rating <= 0 || result.Item.Rating > 10 {
				t.Errorf("评分应在 0-10 范围内, got %f", result.Item.Rating)
			}
			if len(result.Item.Cast) == 0 {
				t.Error("演员列表不应为空")
			}
			if result.Item.Overview == "" {
				t.Error("简介不应为空")
			}
			if result.Item.PosterPath == "" {
				t.Error("海报路径不应为空")
			}
			if result.Confidence < 0 || result.Confidence > 1 {
				t.Errorf("置信度应在 0-1 范围, got %f", result.Confidence)
			}
		})
	}
}

func TestScraper_ScrapeTV(t *testing.T) {
	s := NewScraper()

	tests := []struct {
		name      string
		filePath  string
		wantTitle string
		wantType  MediaType
		wantFound bool
	}{
		{"绝命毒师", "/media/tv/Breaking.Bad.S01E02.2008.1080p.mkv", "Breaking Bad", MediaTypeTVSeries, true},
		{"权力的游戏", "/media/tv/Game.of.Thrones.s01e01.2011.mkv", "Game of Thrones", MediaTypeTVSeries, true},
		{"怪奇物语", "/media/tv/Stranger Things S02E05 2016 WEB-DL.mkv", "Stranger Things", MediaTypeTVSeries, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.Scrape(tt.filePath)
			if result.Error != nil {
				t.Fatalf("Scrape 返回错误: %v", result.Error)
			}
			if !result.Found {
				t.Fatal("应找到匹配的元数据")
			}
			if result.Item == nil {
				t.Fatal("Item 不应为 nil")
			}
			if result.Item.Title != tt.wantTitle {
				t.Errorf("标题: got %q, want %q", result.Item.Title, tt.wantTitle)
			}
			if result.Item.Type != tt.wantType {
				t.Errorf("类型: got %v, want %v", result.Item.Type, tt.wantType)
			}
		})
	}
}

func TestScraper_ScrapeNotFound(t *testing.T) {
	s := NewScraper()
	result := s.Scrape("/media/movies/Unknown.Movie.2099.1080p.mkv")
	if result.Found {
		t.Error("不应找到匹配的元数据")
	}
	if result.Item != nil {
		t.Error("未找到时 Item 应为 nil")
	}
}

func TestScraper_ScrapeBatch(t *testing.T) {
	s := NewScraper()
	files := []string{
		"/media/movies/Inception 2010 1080p BluRay.mkv",
		"/media/movies/The.Dark.Knight.2008.1080p.BluRay.mkv",
		"/media/tv/Breaking.Bad.S01E02.2008.1080p.mkv",
		"/media/movies/Unknown.Movie.2099.mkv",
	}

	results := s.ScrapeBatch(files)
	if len(results) != len(files) {
		t.Fatalf("结果数量: got %d, want %d", len(results), len(files))
	}

	foundCount := 0
	for _, r := range results {
		if r.Found {
			foundCount++
		}
	}
	if foundCount != 3 {
		t.Errorf("成功匹配数: got %d, want %d", foundCount, 3)
	}
}

func TestScraper_ScrapeEmptyFilename(t *testing.T) {
	s := NewScraper()
	result := s.Scrape("")
	if result.Found {
		t.Error("空文件名不应找到匹配")
	}
	if result.Error == nil {
		t.Error("空文件名应返回错误")
	}
}

func TestBuildKey(t *testing.T) {
	tests := []struct {
		title string
		year  int
		want  string
	}{
		{"Inception", 2010, "inception_2010"},
		{"The Dark Knight", 2008, "the_dark_knight_2008"},
		{"Breaking Bad", 2008, "breaking_bad_2008"},
		{"Parasite", 2019, "parasite_2019"},
		{"Unknown", 0, "unknown"},
	}
	for _, tt := range tests {
		got := buildKey(tt.title, tt.year)
		if got != tt.want {
			t.Errorf("buildKey(%q, %d) = %q, want %q", tt.title, tt.year, got, tt.want)
		}
	}
}

// ========== 海报墙测试 ==========

func TestPosterWallBuilder_Build(t *testing.T) {
	s := NewScraper()
	b := NewPosterWallBuilder()

	files := []string{
		"/media/movies/Inception 2010 1080p BluRay.mkv",
		"/media/movies/The.Dark.Knight.2008.1080p.BluRay.mkv",
		"/media/movies/interstellar_2014_720p_web-dl.mp4",
		"/media/tv/Breaking.Bad.S01E02.2008.1080p.mkv",
		"/media/tv/Game.of.Thrones.s01e01.2011.mkv",
	}

	results := s.ScrapeBatch(files)
	wall := b.BuildFromResults(results)

	if wall.Total != 5 {
		t.Errorf("总数: got %d, want %d", wall.Total, 5)
	}
	if len(wall.Groups) != 2 {
		t.Errorf("分组数: got %d, want %d", len(wall.Groups), 2)
	}
	if wall.Groups[0].Type != MediaTypeMovie {
		t.Errorf("第一组应为电影, got %v", wall.Groups[0].Type)
	}
	if wall.Groups[0].Count != 3 {
		t.Errorf("电影数: got %d, want %d", wall.Groups[0].Count, 3)
	}
	if wall.Groups[1].Type != MediaTypeTVSeries {
		t.Errorf("第二组应为电视剧, got %v", wall.Groups[1].Type)
	}
	if wall.Groups[1].Count != 2 {
		t.Errorf("电视剧数: got %d, want %d", wall.Groups[1].Count, 2)
	}
}

func TestPosterWallBuilder_SortByRating(t *testing.T) {
	s := NewScraper()
	b := NewPosterWallBuilder()

	files := []string{
		"/media/movies/Inception 2010 1080p BluRay.mkv",
		"/media/movies/The.Dark.Knight.2008.1080p.BluRay.mkv",
		"/media/movies/interstellar_2014_720p_web-dl.mp4",
	}

	results := s.ScrapeBatch(files)
	wall := b.BuildFromResults(results)

	if wall.Groups[0].Items[0].Title != "The Dark Knight" {
		t.Errorf("最高评分应为 The Dark Knight, got %s", wall.Groups[0].Items[0].Title)
	}
}

func TestPosterWallBuilder_SortByYear(t *testing.T) {
	s := NewScraper()
	b := NewPosterWallBuilder()

	files := []string{
		"/media/movies/Inception 2010 1080p BluRay.mkv",
		"/media/movies/The.Dark.Knight.2008.1080p.BluRay.mkv",
		"/media/movies/interstellar_2014_720p_web-dl.mp4",
	}

	results := s.ScrapeBatch(files)
	wall := b.BuildFromResults(results)
	b.SortByYear(wall)

	if wall.Groups[0].Items[0].Title != "Interstellar" {
		t.Errorf("最新年份应为 Interstellar, got %s", wall.Groups[0].Items[0].Title)
	}
}

func TestPosterWallBuilder_SortByTitle(t *testing.T) {
	s := NewScraper()
	b := NewPosterWallBuilder()

	files := []string{
		"/media/movies/The.Dark.Knight.2008.1080p.BluRay.mkv",
		"/media/movies/Inception 2010 1080p BluRay.mkv",
		"/media/movies/interstellar_2014_720p_web-dl.mp4",
	}

	results := s.ScrapeBatch(files)
	wall := b.BuildFromResults(results)
	b.SortByTitle(wall)

	if wall.Groups[0].Items[0].Title != "Inception" {
		t.Errorf("字母序第一应为 Inception, got %s", wall.Groups[0].Items[0].Title)
	}
}

func TestPosterWallBuilder_FilterByGenre(t *testing.T) {
	s := NewScraper()
	b := NewPosterWallBuilder()

	files := []string{
		"/media/movies/Inception 2010 1080p BluRay.mkv",
		"/media/movies/The.Dark.Knight.2008.1080p.BluRay.mkv",
		"/media/movies/interstellar_2014_720p_web-dl.mp4",
		"/media/movies/Parasite 2019 REMUX 2160p.mkv",
	}

	results := s.ScrapeBatch(files)
	wall := b.BuildFromResults(results)

	filtered := b.FilterByGenre(wall, "科幻")
	if filtered.Total == 0 {
		t.Fatal("过滤结果不应为空")
	}
	for _, g := range filtered.Groups {
		for _, item := range g.Items {
			found := false
			for _, genre := range item.Genres {
				if genre == "科幻" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("过滤结果中 %s 不包含科幻标签", item.Title)
			}
		}
	}
}

func TestPosterWallBuilder_EmptyInput(t *testing.T) {
	b := NewPosterWallBuilder()
	wall := b.Build(nil)

	if wall.Total != 0 {
		t.Errorf("空输入总数应为0, got %d", wall.Total)
	}
	if len(wall.Groups) != 0 {
		t.Errorf("空输入分组数应为0, got %d", len(wall.Groups))
	}
}

func TestPosterWallBuilder_BuildFromResultsWithFailures(t *testing.T) {
	s := NewScraper()
	b := NewPosterWallBuilder()

	files := []string{
		"/media/movies/Inception 2010 1080p BluRay.mkv",
		"/media/movies/Unknown.Movie.2099.mkv",
		"/media/tv/Breaking.Bad.S01E02.2008.1080p.mkv",
	}

	results := s.ScrapeBatch(files)
	wall := b.BuildFromResults(results)

	if wall.Total != 2 {
		t.Errorf("总数: got %d, want %d", wall.Total, 2)
	}
}

// ========== 字幕管理测试 ==========

func TestSubtitleManager_Search(t *testing.T) {
	s := NewScraper()
	m := NewSubtitleManager(s, t.TempDir())

	subs, err := m.Search("inception_2010", "zh-CN")
	if err != nil {
		t.Fatalf("搜索中文字幕失败: %v", err)
	}
	if len(subs) == 0 {
		t.Fatal("应至少有一条中文字幕")
	}
	if subs[0].Language != "zh-CN" {
		t.Errorf("字幕语言: got %q, want %q", subs[0].Language, "zh-CN")
	}
	if subs[0].Content == "" {
		t.Error("字幕内容不应为空")
	}

	subs, err = m.Search("inception_2010", "en-US")
	if err != nil {
		t.Fatalf("搜索英文字幕失败: %v", err)
	}
	if len(subs) == 0 {
		t.Fatal("应至少有一条英文字幕")
	}

	subs, err = m.Search("inception_2010", "")
	if err != nil {
		t.Fatalf("搜索所有字幕失败: %v", err)
	}
	if len(subs) != 2 {
		t.Errorf("字幕总数: got %d, want %d", len(subs), 2)
	}
}

func TestSubtitleManager_SearchNotFound(t *testing.T) {
	s := NewScraper()
	m := NewSubtitleManager(s, t.TempDir())

	_, err := m.Search("unknown_movie_2099", "zh-CN")
	if err == nil {
		t.Error("不存在的媒体应返回错误")
	}

	_, err = m.Search("interstellar_2014", "zh-CN")
	if err == nil {
		t.Error("无字幕的媒体应返回错误")
	}
}

func TestSubtitleManager_Download(t *testing.T) {
	s := NewScraper()
	saveDir := t.TempDir()
	m := NewSubtitleManager(s, saveDir)

	result := m.Download("inception_2010", "zh-CN")
	if result.Error != nil {
		t.Fatalf("下载字幕失败: %v", result.Error)
	}
	if result.FilePath == "" {
		t.Error("字幕文件路径不应为空")
	}
	if result.Language != "zh-CN" {
		t.Errorf("字幕语言: got %q, want %q", result.Language, "zh-CN")
	}
	if result.Source == "" {
		t.Error("字幕来源不应为空")
	}

	info, err := os.Stat(result.FilePath)
	if err != nil {
		t.Fatalf("字幕文件不存在: %v", err)
	}
	if info.Size() == 0 {
		t.Error("字幕文件不应为空")
	}
}

func TestSubtitleManager_DownloadNotFound(t *testing.T) {
	s := NewScraper()
	m := NewSubtitleManager(s, t.TempDir())

	result := m.Download("unknown_movie_2099", "zh-CN")
	if result.Error == nil {
		t.Error("不存在的媒体应返回错误")
	}
	if result.FilePath != "" {
		t.Error("失败时文件路径应为空")
	}
}

func TestSubtitleManager_DownloadNoSubtitle(t *testing.T) {
	s := NewScraper()
	m := NewSubtitleManager(s, t.TempDir())

	result := m.Download("interstellar_2014", "zh-CN")
	if result.Error == nil {
		t.Error("无字幕的媒体应返回错误")
	}
}

func TestSubtitleManager_DownloadMulti(t *testing.T) {
	s := NewScraper()
	m := NewSubtitleManager(s, t.TempDir())

	langs := []string{"zh-CN", "en-US"}
	results := m.DownloadMulti("inception_2010", langs)
	if len(results) != 2 {
		t.Fatalf("结果数: got %d, want %d", len(results), 2)
	}

	for i, r := range results {
		if r.Error != nil {
			t.Errorf("语言 %s 下载失败: %v", langs[i], r.Error)
		}
		if r.Language != langs[i] {
			t.Errorf("语言: got %q, want %q", r.Language, langs[i])
		}
	}
}

func TestSubtitleManager_SearchByItem(t *testing.T) {
	s := NewScraper()
	m := NewSubtitleManager(s, t.TempDir())

	result := s.Scrape("/media/movies/Inception 2010 1080p BluRay.mkv")
	if !result.Found {
		t.Fatal("应找到匹配的元数据")
	}

	subs, err := m.SearchByItem(result.Item, "zh-CN")
	if err != nil {
		t.Fatalf("通过MediaItem搜索字幕失败: %v", err)
	}
	if len(subs) == 0 {
		t.Fatal("应至少有一条中文字幕")
	}
}

func TestSubtitleManager_DownloadByItem(t *testing.T) {
	s := NewScraper()
	m := NewSubtitleManager(s, t.TempDir())

	result := s.Scrape("/media/movies/The.Dark.Knight.2008.1080p.BluRay.mkv")
	if !result.Found {
		t.Fatal("应找到匹配的元数据")
	}

	subResult := m.DownloadByItem(result.Item, "en-US")
	if subResult.Error != nil {
		t.Fatalf("下载字幕失败: %v", subResult.Error)
	}
	if subResult.Language != "en-US" {
		t.Errorf("字幕语言: got %q, want %q", subResult.Language, "en-US")
	}

	if _, err := os.Stat(subResult.FilePath); err != nil {
		t.Fatalf("字幕文件不存在: %v", err)
	}
}

func TestSubtitleManager_ListAvailableLanguages(t *testing.T) {
	s := NewScraper()
	m := NewSubtitleManager(s, t.TempDir())

	langs, err := m.ListAvailableLanguages("inception_2010")
	if err != nil {
		t.Fatalf("列出可用语言失败: %v", err)
	}
	if len(langs) != 2 {
		t.Errorf("语言数: got %d, want %d", len(langs), 2)
	}

	hasZh, hasEn := false, false
	for _, l := range langs {
		if l == "zh-CN" {
			hasZh = true
		}
		if l == "en-US" {
			hasEn = true
		}
	}
	if !hasZh {
		t.Error("应包含中文 zh-CN")
	}
	if !hasEn {
		t.Error("应包含英文 en-US")
	}
}

func TestSubtitleManager_ListAvailableLanguagesNotFound(t *testing.T) {
	s := NewScraper()
	m := NewSubtitleManager(s, t.TempDir())

	_, err := m.ListAvailableLanguages("unknown_movie_2099")
	if err == nil {
		t.Error("不存在的媒体应返回错误")
	}
}

func TestSubtitleManager_GetSetSaveDir(t *testing.T) {
	s := NewScraper()
	m := NewSubtitleManager(s, "/custom/subs")

	if m.GetSaveDir() != "/custom/subs" {
		t.Errorf("保存目录: got %q, want %q", m.GetSaveDir(), "/custom/subs")
	}

	m.SetSaveDir("/new/path")
	if m.GetSaveDir() != "/new/path" {
		t.Errorf("保存目录: got %q, want %q", m.GetSaveDir(), "/new/path")
	}
}

func TestSubtitleManager_DefaultSaveDir(t *testing.T) {
	s := NewScraper()
	m := NewSubtitleManager(s, "")

	if m.GetSaveDir() != "/tmp/subtitles" {
		t.Errorf("默认保存目录: got %q, want %q", m.GetSaveDir(), "/tmp/subtitles")
	}
}

func TestSubtitleManager_SearchByItemNil(t *testing.T) {
	s := NewScraper()
	m := NewSubtitleManager(s, t.TempDir())

	_, err := m.SearchByItem(nil, "zh-CN")
	if err == nil {
		t.Error("nil 媒体项应返回错误")
	}
}

func TestSubtitleManager_DownloadByItemNil(t *testing.T) {
	s := NewScraper()
	m := NewSubtitleManager(s, t.TempDir())

	result := m.DownloadByItem(nil, "zh-CN")
	if result.Error == nil {
		t.Error("nil 媒体项应返回错误")
	}
}

// ========== 集成测试 ==========

func TestIntegration_FullScrapeFlow(t *testing.T) {
	s := NewScraper()
	b := NewPosterWallBuilder()
	m := NewSubtitleManager(s, t.TempDir())

	files := []string{
		"/media/movies/Inception 2010 1080p BluRay.mkv",
		"/media/movies/The.Dark.Knight.2008.1080p.BluRay.mkv",
		"/media/movies/interstellar_2014_720p_web-dl.mp4",
		"/media/movies/Parasite 2019 REMUX 2160p.mkv",
		"/media/tv/Breaking.Bad.S01E02.2008.1080p.mkv",
		"/media/tv/Game.of.Thrones.s01e01.2011.mkv",
		"/media/tv/Stranger Things S02E05 2016 WEB-DL.mkv",
		"/media/movies/Unknown.Movie.2099.mkv",
	}

	results := s.ScrapeBatch(files)
	wall := b.BuildFromResults(results)

	if wall.Total != 7 {
		t.Errorf("海报墙总数: got %d, want %d", wall.Total, 7)
	}
	if len(wall.Groups) != 2 {
		t.Errorf("分组数: got %d, want %d", len(wall.Groups), 2)
	}

	movieGroup := wall.Groups[0]
	if movieGroup.Type != MediaTypeMovie {
		t.Errorf("第一组类型应为电影")
	}
	if movieGroup.Count != 4 {
		t.Errorf("电影数: got %d, want %d", movieGroup.Count, 4)
	}

	tvGroup := wall.Groups[1]
	if tvGroup.Type != MediaTypeTVSeries {
		t.Errorf("第二组类型应为电视剧")
	}
	if tvGroup.Count != 3 {
		t.Errorf("电视剧数: got %d, want %d", tvGroup.Count, 3)
	}

	// 为每部电影下载中文字幕
	for _, item := range movieGroup.Items {
		subResult := m.DownloadByItem(item, "zh-CN")
		if subResult.Error != nil {
			continue // 有些电影可能没有字幕数据
		}
		if subResult.FilePath == "" {
			t.Errorf("%s 字幕文件路径不应为空", item.Title)
		}
		if _, err := os.Stat(subResult.FilePath); err != nil {
			t.Errorf("%s 字幕文件不存在: %v", item.Title, err)
		}
	}

	// 按年份重新排序
	b.SortByYear(wall)
	if movieGroup.Items[0].Year < movieGroup.Items[1].Year {
		t.Error("年份排序应为降序")
	}

	// 按类型过滤
	filtered := b.FilterByGenre(wall, "科幻")
	if filtered.Total == 0 {
		t.Error("科幻类过滤结果不应为空")
	}
}

func TestIntegration_MediaItemFields(t *testing.T) {
	s := NewScraper()

	result := s.Scrape("/media/movies/Inception 2010 1080p BluRay.mkv")
	if !result.Found || result.Item == nil {
		t.Fatal("刮削失败")
	}

	item := result.Item

	assert.NotEmpty(t, item.ID, "ID不应为空")
	assert.NotEmpty(t, item.Title, "标题不应为空")
	assert.True(t, item.Year > 0, "年份应大于0")
	assert.True(t, item.Type == MediaTypeMovie || item.Type == MediaTypeTVSeries, "类型应有效")
	assert.NotEmpty(t, item.PosterPath, "海报路径不应为空")
	assert.NotEmpty(t, item.Overview, "简介不应为空")
	assert.True(t, len(item.Cast) > 0, "演员列表不应为空")
	assert.True(t, item.Rating > 0 && item.Rating <= 10, "评分应在有效范围")
	assert.True(t, len(item.Genres) > 0, "类型标签不应为空")
	assert.NotEmpty(t, item.FilePath, "文件路径不应为空")
	assert.False(t, item.ScrapedAt.IsZero(), "刮削时间不应为零值")
	assert.False(t, item.ScrapedAt.After(time.Now().Add(time.Second)), "刮削时间不应是未来")
}
