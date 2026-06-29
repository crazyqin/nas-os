package mediascraper

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Scraper 核心刮削器，负责识别文件名、查询元数据
type Scraper struct {
	movieDB   map[string]*metadataRecord // 电影元数据库（模拟）
	tvDB      map[string]*metadataRecord // 电视剧元数据库（模拟）
	subtitleDB map[string][]subtitleRecord // 字幕库（模拟）
}

// NewScraper 创建一个新的刮削器，初始化模拟数据
func NewScraper() *Scraper {
	s := &Scraper{
		movieDB:    make(map[string]*metadataRecord),
		tvDB:       make(map[string]*metadataRecord),
		subtitleDB: make(map[string][]subtitleRecord),
	}
	s.seedMockData()
	return s
}

// seedMockData 填充模拟的元数据，模拟飞牛 fnOS 的元数据API
func (s *Scraper) seedMockData() {
	// === 电影元数据 ===
	s.movieDB["inception_2010"] = &metadataRecord{
		Title:         "Inception",
		OriginalTitle: "Inception",
		Year:          2010,
		Overview:      "一名专业的盗梦贼多姆·科布拥有在人们梦境中窃取秘密的特殊能力。然而，这一次他必须完成一项不可能的任务——将一个想法植入目标的梦中。",
		Cast:          []string{"Leonardo DiCaprio", "Joseph Gordon-Levitt", "Ellen Page", "Tom Hardy"},
		Director:      "Christopher Nolan",
		Rating:        8.8,
		Genres:        []string{"动作", "科幻", "悬疑"},
		PosterURL:     "/posters/inception_2010.jpg",
	}
	s.movieDB["the_dark_knight_2008"] = &metadataRecord{
		Title:         "The Dark Knight",
		OriginalTitle: "The Dark Knight",
		Year:          2008,
		Overview:      "蝙蝠侠与新兴犯罪领袖小丑之间的斗争升级为对哥谭市民众的终极考验。",
		Cast:          []string{"Christian Bale", "Heath Ledger", "Aaron Eckhart"},
		Director:      "Christopher Nolan",
		Rating:        9.0,
		Genres:        []string{"动作", "犯罪", "剧情"},
		PosterURL:     "/posters/the_dark_knight_2008.jpg",
	}
	s.movieDB["interstellar_2014"] = &metadataRecord{
		Title:         "Interstellar",
		OriginalTitle: "Interstellar",
		Year:          2014,
		Overview:      "在地球资源即将枯竭的未来，一群宇航员穿越虫洞寻找人类新的栖息地。",
		Cast:          []string{"Matthew McConaughey", "Anne Hathaway", "Jessica Chastain"},
		Director:      "Christopher Nolan",
		Rating:        8.6,
		Genres:        []string{"科幻", "冒险", "剧情"},
		PosterURL:     "/posters/interstellar_2014.jpg",
	}
	s.movieDB["parasite_2019"] = &metadataRecord{
		Title:         "Parasite",
		OriginalTitle: "기생충",
		Year:          2019,
		Overview:      "贫穷的金家四口和富有的朴家之间的故事，一场关于阶级的黑色幽默。",
		Cast:          []string{"Song Kang-ho", "Lee Sun-kyun", "Cho Yeo-jeong"},
		Director:      "Bong Joon-ho",
		Rating:        8.6,
		Genres:        []string{"剧情", "悬疑", "喜剧"},
		PosterURL:     "/posters/parasite_2019.jpg",
	}

	// === 电视剧元数据 ===
	s.tvDB["breaking_bad_2008"] = &metadataRecord{
		Title:         "Breaking Bad",
		OriginalTitle: "Breaking Bad",
		Year:          2008,
		Overview:      "一位高中化学老师被诊断出肺癌后，决定与昔日学生一起制造冰毒，为家人留下财产。",
		Cast:          []string{"Bryan Cranston", "Aaron Paul", "Anna Gunn"},
		Director:      "Vince Gilligan",
		Rating:        9.5,
		Genres:        []string{"犯罪", "剧情", "悬疑"},
		PosterURL:     "/posters/breaking_bad_2008.jpg",
	}
	s.tvDB["game_of_thrones_2011"] = &metadataRecord{
		Title:         "Game of Thrones",
		OriginalTitle: "Game of Thrones",
		Year:          2011,
		Overview:      "在维斯特洛大陆上，七大家族为争夺铁王座展开了史诗般的权力斗争。",
		Cast:          []string{"Emilia Clarke", "Kit Harington", "Peter Dinklage"},
		Director:      "David Benioff",
		Rating:        9.2,
		Genres:        []string{"奇幻", "剧情", "冒险"},
		PosterURL:     "/posters/game_of_thrones_2011.jpg",
	}
	s.tvDB["stranger_things_2016"] = &metadataRecord{
		Title:         "Stranger Things",
		OriginalTitle: "Stranger Things",
		Year:          2016,
		Overview:      "一群孩子在小镇上遭遇超自然力量，展开了一段充满悬疑和冒险的故事。",
		Cast:          []string{"Millie Bobby Brown", "Finn Wolfhard", "Winona Ryder"},
		Director:      "The Duffer Brothers",
		Rating:        8.7,
		Genres:        []string{"科幻", "悬疑", "恐怖"},
		PosterURL:     "/posters/stranger_things_2016.jpg",
	}

	// === 字幕数据 ===
	s.subtitleDB["inception_2010"] = []subtitleRecord{
		{Language: "zh-CN", Source: "字幕组", Content: "1\n00:01:00,000 --> 00:01:05,000\n盗梦空间中文字幕\n"},
		{Language: "en-US", Source: "OpenSubtitles", Content: "1\n00:01:00,000 --> 00:01:05,000\nInception English subtitle\n"},
	}
	s.subtitleDB["the_dark_knight_2008"] = []subtitleRecord{
		{Language: "zh-CN", Source: "字幕组", Content: "1\n00:01:00,000 --> 00:01:05,000\n蝙蝠侠中文字幕\n"},
		{Language: "en-US", Source: "OpenSubtitles", Content: "1\n00:01:00,000 --> 00:01:05,000\nDark Knight English subtitle\n"},
	}
	s.subtitleDB["breaking_bad_2008"] = []subtitleRecord{
		{Language: "zh-CN", Source: "射手网", Content: "1\n00:01:00,000 --> 00:01:05,000\n绝命毒师中文字幕\n"},
		{Language: "en-US", Source: "OpenSubtitles", Content: "1\n00:01:00,000 --> 00:01:05,000\nBreaking Bad English subtitle\n"},
	}
}

// 文件名解析正则表达式
var (
	// 匹配 "Title 2010 1080p BluRay" 或 "Title.2010.1080p.BluRay" 等格式
	moviePattern = regexp.MustCompile(`(?i)^(.+?)[\.\s\_\-\(\[]+(\d{4})[\.\s\_\-\)\]]*`)
	// 匹配电视剧 "Title S01E02" 或 "Title s01e02"
	tvPattern = regexp.MustCompile(`(?i)^(.+?)[\.\s\_\-\(\[]+[Ss](\d{1,2})[Ee](\d{1,2})`)
	// 匹配带年份的电视剧 "Title 2008 S01E02"
	tvWithYearPattern = regexp.MustCompile(`(?i)^(.+?)[\.\s\_\-\(\[]+(\d{4})[\.\s\_\-\)\]]*[Ss](\d{1,2})[Ee](\d{1,2})`)
	// 匹配电视剧 "Title S01E02 2008"（年份在季集号之后）
	tvWithYearAfterPattern = regexp.MustCompile(`(?i)^(.+?)[\.\s\_\-\(\[]+[Ss](\d{1,2})[Ee](\d{1,2})[\.\s\_\-\)\]]+(\d{4})`)
)

// ParseFileName 解析文件名，提取标题、年份和媒体类型
// 支持多种常见命名格式，类似飞牛 fnOS 的智能识别
func ParseFileName(filename string) (title string, year int, mediaType MediaType, err error) {
	// 去除文件扩展名
	name := filename
	if ext := filepath.Ext(name); ext != "" {
		name = strings.TrimSuffix(name, ext)
	}

	// 在 cleanup 之前，先从括号中提取年份（避免被清理掉）
	yearFromBrackets := 0
	yearInBracketsRe := regexp.MustCompile(`[\(\[](\d{4})[\)\]]`)
	if m := yearInBracketsRe.FindStringSubmatch(name); m != nil {
		yearFromBrackets, _ = strconv.Atoi(m[1])
	}

	// 去除分辨率、编码等常见标记（1080p, 720p, x264, H264, BluRay, WEB-DL 等）
	cleanupPatterns := []string{
		`(?i)\b(1080[pi]|720p|480p|4K|2160p)\b`,
		`(?i)\b(x264|x265|h264|h265|hevc|avc)\b`,
		`(?i)\b(BluRay|BRRip|DVDRip|WEB-DL|WEBRip|HDRip|CAM|TS|TC)\b`,
		`(?i)\b(REMUX|PROPER|REPACK|EXTENDED|UNCUT|IMAX)\b`,
		`(?i)\b(DTS|AC3|AAC|FLAC|DTS-HD|TrueHD|Atmos)\b`,
		`(?i)\b(10bit|10-bit|HDR|SDR|Dolby)\b`,
		`(?i)\b(GROUP|RARBG|YTS|EZTV|NTb|CMRG)\b`, // 发布组
		`(?i)\b(DVD|Blu-ray|UHD|Remux)\b`,
		`(?i)[\[\(\{][^\]\)\}]*[\]\)\}]`, // 方括号/圆括号内容
	}
	for _, p := range cleanupPatterns {
		re := regexp.MustCompile(p)
		name = re.ReplaceAllString(name, " ")
	}

	// 统一分隔符为空格
	name = strings.NewReplacer(".", " ", "_", " ", "-", " ").Replace(name)
	// 压缩多余空格
	name = strings.Join(strings.Fields(name), " ")
	name = strings.TrimSpace(name)

	if name == "" {
		return "", 0, "", errors.New("无法从文件名中提取有效信息")
	}

	// 先尝试匹配电视剧模式（带 SxxExx）
	if m := tvWithYearPattern.FindStringSubmatch(name); m != nil {
		title = strings.TrimSpace(m[1])
		year, _ = strconv.Atoi(m[2])
		return title, year, MediaTypeTVSeries, nil
	}
	// 匹配 "Title S01E02 2008" 格式（年份在季集号之后）
	if m := tvWithYearAfterPattern.FindStringSubmatch(name); m != nil {
		title = strings.TrimSpace(m[1])
		year, _ = strconv.Atoi(m[3])
		return title, year, MediaTypeTVSeries, nil
	}
	if m := tvPattern.FindStringSubmatch(name); m != nil {
		title = strings.TrimSpace(m[1])
		// 电视剧没年份时，尝试用括号中提取的年份
		if yearFromBrackets > 0 {
			return title, yearFromBrackets, MediaTypeTVSeries, nil
		}
		return title, year, MediaTypeTVSeries, nil
	}

	// 再匹配电影模式（带年份）
	if m := moviePattern.FindStringSubmatch(name); m != nil {
		title = strings.TrimSpace(m[1])
		year, _ = strconv.Atoi(m[2])
		return title, year, MediaTypeMovie, nil
	}

	// 无法确定类型，但如果有括号中提取到的年份，使用它
	if yearFromBrackets > 0 {
		return name, yearFromBrackets, MediaTypeMovie, nil
	}

	// 无法确定类型，默认按电影处理，年份未知
	return name, 0, MediaTypeMovie, nil
}

// buildKey 构建元数据库查找键
func buildKey(title string, year int) string {
	// 标准化：小写、去除空格和标点
	t := strings.ToLower(title)
	t = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return '_'
	}, t)
	t = strings.Trim(t, "_")
	// 去除连续下划线
	for strings.Contains(t, "__") {
		t = strings.ReplaceAll(t, "__", "_")
	}
	if year > 0 {
		return fmt.Sprintf("%s_%d", t, year)
	}
	return t
}

// SearchMovie 在电影元数据库中搜索
func (s *Scraper) SearchMovie(title string, year int) (*metadataRecord, float64, bool) {
	key := buildKey(title, year)
	// 精确匹配
	if rec, ok := s.movieDB[key]; ok {
		return rec, 1.0, true
	}
	// 模糊匹配：去掉年份再试
	if year > 0 {
		keyNoYear := buildKey(title, 0)
		for k, rec := range s.movieDB {
			if k == keyNoYear {
				return rec, 0.85, true
			}
			// 部分匹配
			if strings.Contains(k, keyNoYear) || strings.Contains(keyNoYear, k) {
				return rec, 0.7, true
			}
		}
	}
	// 全模糊匹配
	keyNoYear := buildKey(title, 0)
	for k, rec := range s.movieDB {
		if strings.Contains(k, keyNoYear) || strings.Contains(keyNoYear, k) {
			return rec, 0.6, true
		}
	}
	return nil, 0, false
}

// SearchTV 在电视剧元数据库中搜索
func (s *Scraper) SearchTV(title string, year int) (*metadataRecord, float64, bool) {
	key := buildKey(title, year)
	if rec, ok := s.tvDB[key]; ok {
		return rec, 1.0, true
	}
	// 模糊匹配
	keyNoYear := buildKey(title, 0)
	for k, rec := range s.tvDB {
		if strings.Contains(k, keyNoYear) || strings.Contains(keyNoYear, k) {
			return rec, 0.65, true
		}
	}
	return nil, 0, false
}

// Scrape 执行刮削：解析文件名 → 查询元数据 → 构建 MediaItem
// 这是核心入口方法，类似飞牛 fnOS 的自动刮削流程
func (s *Scraper) Scrape(filePath string) *ScraperResult {
	filename := filepath.Base(filePath)
	title, year, mediaType, err := ParseFileName(filename)
	if err != nil {
		return &ScraperResult{
			Found: false,
			Error: fmt.Errorf("文件名解析失败: %w", err),
		}
	}

	var record *metadataRecord
	var confidence float64
	var found bool

	switch mediaType {
	case MediaTypeTVSeries:
		record, confidence, found = s.SearchTV(title, year)
	default:
		record, confidence, found = s.SearchMovie(title, year)
	}

	if !found {
		return &ScraperResult{
			Found:      false,
			Confidence: 0,
			Error:      fmt.Errorf("未找到匹配的元数据: %s (%d)", title, year),
		}
	}

	item := &MediaItem{
		ID:            buildKey(title, year),
		Title:         record.Title,
		OriginalTitle: record.OriginalTitle,
		Year:          record.Year,
		Type:          mediaType,
		PosterPath:    record.PosterURL,
		Overview:      record.Overview,
		Cast:          record.Cast,
		Director:      record.Director,
		Rating:        record.Rating,
		Genres:        record.Genres,
		FilePath:      filePath,
		ScrapedAt:     time.Now(),
	}

	return &ScraperResult{
		Item:       item,
		Found:      true,
		Confidence: confidence,
		Error:      nil,
	}
}

// ScrapeBatch 批量刮削多个文件
func (s *Scraper) ScrapeBatch(filePaths []string) []*ScraperResult {
	results := make([]*ScraperResult, 0, len(filePaths))
	for _, fp := range filePaths {
		results = append(results, s.Scrape(fp))
	}
	return results
}
