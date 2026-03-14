package media

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// SubtitleFormat 字幕格式
type SubtitleFormat string

const (
	SubtitleFormatSRT SubtitleFormat = "srt"
	SubtitleFormatVTT SubtitleFormat = "vtt"
	SubtitleFormatASS SubtitleFormat = "ass"
	SubtitleFormatSSA SubtitleFormat = "ssa"
)

// SubtitleItem 字幕项
type SubtitleItem struct {
	Index     int           `json:"index"`
	StartTime time.Duration `json:"startTime"`
	EndTime   time.Duration `json:"endTime"`
	Text      string        `json:"text"`
	// ASS/SSA 特有字段
	Style   string `json:"style,omitempty"`
	Name    string `json:"name,omitempty"`
	MarginL int    `json:"marginL,omitempty"`
	MarginR int    `json:"marginR,omitempty"`
	MarginV int    `json:"marginV,omitempty"`
	Effect  string `json:"effect,omitempty"`
}

// Subtitle 字幕文档
type Subtitle struct {
	Format   SubtitleFormat `json:"format"`
	Items    []SubtitleItem `json:"items"`
	Language string         `json:"language"`
	Title    string         `json:"title"`
	// VTT 特有
	Regions []VTTRegion `json:"regions,omitempty"`
	// ASS/SSA 特有
	Styles     []ASSStyle    `json:"styles,omitempty"`
	ScriptInfo ASSScriptInfo `json:"scriptInfo,omitempty"`
}

// VTTRegion VTT 区域定义
type VTTRegion struct {
	ID        string  `json:"id"`
	Width     float64 `json:"width"`
	Height    float64 `json:"height"`
	AnchorX   float64 `json:"anchorX"`
	AnchorY   float64 `json:"anchorY"`
	ViewportX float64 `json:"viewportX"`
	ViewportY float64 `json:"viewportY"`
	Scroll    string  `json:"scroll"`
}

// ASSStyle ASS 样式定义
type ASSStyle struct {
	Name           string  `json:"name"`
	FontName       string  `json:"fontName"`
	FontSize       float64 `json:"fontSize"`
	PrimaryColor   string  `json:"primaryColor"`
	SecondaryColor string  `json:"secondaryColor"`
	OutlineColor   string  `json:"outlineColor"`
	BackColour     string  `json:"backColor"`
	Bold           bool    `json:"bold"`
	Italic         bool    `json:"italic"`
	Underline      bool    `json:"underline"`
	StrikeOut      bool    `json:"strikeOut"`
	ScaleX         float64 `json:"scaleX"`
	ScaleY         float64 `json:"scaleY"`
	Spacing        float64 `json:"spacing"`
	Angle          float64 `json:"angle"`
	BorderStyle    int     `json:"borderStyle"`
	Outline        float64 `json:"outline"`
	Shadow         float64 `json:"shadow"`
	Alignment      int     `json:"alignment"`
	MarginL        int     `json:"marginL"`
	MarginR        int     `json:"marginR"`
	MarginV        int     `json:"marginV"`
	Encoding       int     `json:"encoding"`
}

// ASSScriptInfo ASS 脚本信息
type ASSScriptInfo struct {
	Title                 string  `json:"title"`
	OriginalScript        string  `json:"originalScript"`
	Translation           string  `json:"translation"`
	Editing               string  `json:"editing"`
	Timing                string  `json:"timing"`
	SyncPoint             string  `json:"syncPoint"`
	UpdatedBy             string  `json:"updatedBy"`
	UpdateDetails         string  `json:"updateDetails"`
	ScriptType            string  `json:"scriptType"`
	Collisions            string  `json:"collisions"`
	PlayDepth             int     `json:"playDepth"`
	PlayResX              int     `json:"playResX"`
	PlayResY              int     `json:"playResY"`
	Timer                 float64 `json:"timer"`
	WrapStyle             string  `json:"wrapStyle"`
	ScaledBorderAndShadow bool    `json:"scaledBorderAndShadow"`
}

// SubtitleManager 字幕管理器
type SubtitleManager struct {
}

// NewSubtitleManager 创建字幕管理器
func NewSubtitleManager() *SubtitleManager {
	return &SubtitleManager{}
}

// ParseSubtitle 解析字幕文件
func (sm *SubtitleManager) ParseSubtitle(path string) (*Subtitle, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开字幕文件失败: %w", err)
	}
	defer file.Close()

	// 根据扩展名选择解析器
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".srt":
		return sm.ParseSRT(file)
	case ".vtt":
		return sm.ParseVTT(file)
	case ".ass", ".ssa":
		return sm.ParseASS(file)
	default:
		return nil, fmt.Errorf("不支持的字幕格式: %s", ext)
	}
}

// ParseSRT 解析 SRT 字幕
func (sm *SubtitleManager) ParseSRT(reader io.Reader) (*Subtitle, error) {
	subtitle := &Subtitle{
		Format: SubtitleFormatSRT,
		Items:  make([]SubtitleItem, 0),
	}

	scanner := bufio.NewScanner(reader)
	var currentItem *SubtitleItem
	var textLines []string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			// 空行表示字幕项结束
			if currentItem != nil && len(textLines) > 0 {
				currentItem.Text = strings.Join(textLines, "\n")
				subtitle.Items = append(subtitle.Items, *currentItem)
				currentItem = nil
				textLines = nil
			}
			continue
		}

		if currentItem == nil {
			// 尝试解析序号
			if _, err := strconv.Atoi(line); err == nil {
				currentItem = &SubtitleItem{
					Index: len(subtitle.Items) + 1,
				}
				textLines = make([]string, 0)
			}
		} else if currentItem.StartTime == 0 && strings.Contains(line, "-->") {
			// 时间轴
			times := strings.Split(line, "-->")
			if len(times) == 2 {
				currentItem.StartTime = parseSRTTime(strings.TrimSpace(times[0]))
				currentItem.EndTime = parseSRTTime(strings.TrimSpace(times[1]))
			}
		} else {
			// 文本内容
			textLines = append(textLines, line)
		}
	}

	// 处理最后一项
	if currentItem != nil && len(textLines) > 0 {
		currentItem.Text = strings.Join(textLines, "\n")
		subtitle.Items = append(subtitle.Items, *currentItem)
	}

	return subtitle, nil
}

// parseSRTTime 解析 SRT 时间格式 (00:00:00,000)
func parseSRTTime(s string) time.Duration {
	// 格式: HH:MM:SS,mmm 或 HH:MM:SS.mmm
	s = strings.ReplaceAll(s, ",", ".")

	pattern := regexp.MustCompile(`(\d+):(\d+):(\d+)[.,](\d+)`)
	matches := pattern.FindStringSubmatch(s)
	if len(matches) != 5 {
		return 0
	}

	hours, _ := strconv.Atoi(matches[1])
	minutes, _ := strconv.Atoi(matches[2])
	seconds, _ := strconv.Atoi(matches[3])
	millis, _ := strconv.Atoi(matches[4])

	return time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second +
		time.Duration(millis)*time.Millisecond
}

// ParseVTT 解析 VTT 字幕
func (sm *SubtitleManager) ParseVTT(reader io.Reader) (*Subtitle, error) {
	subtitle := &Subtitle{
		Format:  SubtitleFormatVTT,
		Items:   make([]SubtitleItem, 0),
		Regions: make([]VTTRegion, 0),
	}

	scanner := bufio.NewScanner(reader)
	var currentItem *SubtitleItem
	var textLines []string
	inHeader := true

	for scanner.Scan() {
		line := scanner.Text()

		// 检查文件头
		if inHeader {
			if strings.HasPrefix(line, "WEBVTT") {
				inHeader = false
				continue
			}
			// 解析区域定义等其他头部信息
			if strings.HasPrefix(line, "Region:") {
				region := parseVTTRegion(line[7:])
				subtitle.Regions = append(subtitle.Regions, region)
				continue
			}
			if strings.TrimSpace(line) == "" {
				inHeader = false
				continue
			}
			continue
		}

		line = strings.TrimSpace(line)

		if line == "" {
			if currentItem != nil && len(textLines) > 0 {
				currentItem.Text = strings.Join(textLines, "\n")
				subtitle.Items = append(subtitle.Items, *currentItem)
				currentItem = nil
				textLines = nil
			}
			continue
		}

		if currentItem == nil {
			// 检查是否是时间轴（NOTE: 标识符可以省略）
			if strings.Contains(line, "-->") {
				currentItem = &SubtitleItem{
					Index: len(subtitle.Items) + 1,
				}
				textLines = make([]string, 0)

				// 解析时间轴
				times := strings.Split(line, "-->")
				if len(times) >= 2 {
					timePart := strings.TrimSpace(times[0])
					// 可能有位置设置
					endParts := strings.Fields(times[1])
					currentItem.StartTime = parseVTTTime(timePart)
					if len(endParts) > 0 {
						currentItem.EndTime = parseVTTTime(endParts[0])
					}
				}
			}
		} else {
			textLines = append(textLines, line)
		}
	}

	if currentItem != nil && len(textLines) > 0 {
		currentItem.Text = strings.Join(textLines, "\n")
		subtitle.Items = append(subtitle.Items, *currentItem)
	}

	return subtitle, nil
}

// parseVTTTime 解析 VTT 时间格式 (00:00:00.000 或 00:00.000)
func parseVTTTime(s string) time.Duration {
	s = strings.TrimSpace(s)

	// 尝试 HH:MM:SS.mmm 格式
	pattern := regexp.MustCompile(`(\d+):(\d+):(\d+)\.(\d+)`)
	matches := pattern.FindStringSubmatch(s)
	if len(matches) == 5 {
		hours, _ := strconv.Atoi(matches[1])
		minutes, _ := strconv.Atoi(matches[2])
		seconds, _ := strconv.Atoi(matches[3])
		millis, _ := strconv.Atoi(matches[4])
		return time.Duration(hours)*time.Hour +
			time.Duration(minutes)*time.Minute +
			time.Duration(seconds)*time.Second +
			time.Duration(millis)*time.Millisecond
	}

	// 尝试 MM:SS.mmm 格式
	pattern = regexp.MustCompile(`(\d+):(\d+)\.(\d+)`)
	matches = pattern.FindStringSubmatch(s)
	if len(matches) == 4 {
		minutes, _ := strconv.Atoi(matches[1])
		seconds, _ := strconv.Atoi(matches[2])
		millis, _ := strconv.Atoi(matches[3])
		return time.Duration(minutes)*time.Minute +
			time.Duration(seconds)*time.Second +
			time.Duration(millis)*time.Millisecond
	}

	return 0
}

// parseVTTRegion 解析 VTT 区域
func parseVTTRegion(s string) VTTRegion {
	region := VTTRegion{}
	params := strings.Split(s, ",")

	for _, p := range params {
		p = strings.TrimSpace(p)
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			continue
		}

		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])

		switch key {
		case "id":
			region.ID = value
		case "width":
			if v, err := strconv.ParseFloat(strings.TrimSuffix(value, "%"), 64); err == nil {
				region.Width = v
			}
		case "height":
			if v, err := strconv.ParseFloat(strings.TrimSuffix(value, "%"), 64); err == nil {
				region.Height = v
			}
		case "regionanchor":
			coords := strings.Split(value, ",")
			if len(coords) == 2 {
				if x, err := strconv.ParseFloat(strings.TrimSuffix(coords[0], "%"), 64); err == nil {
					region.AnchorX = x
				}
				if y, err := strconv.ParseFloat(strings.TrimSuffix(coords[1], "%"), 64); err == nil {
					region.AnchorY = y
				}
			}
		case "viewportanchor":
			coords := strings.Split(value, ",")
			if len(coords) == 2 {
				if x, err := strconv.ParseFloat(strings.TrimSuffix(coords[0], "%"), 64); err == nil {
					region.ViewportX = x
				}
				if y, err := strconv.ParseFloat(strings.TrimSuffix(coords[1], "%"), 64); err == nil {
					region.ViewportY = y
				}
			}
		case "scroll":
			region.Scroll = value
		}
	}

	return region
}

// ParseASS 解析 ASS/SSA 字幕
func (sm *SubtitleManager) ParseASS(reader io.Reader) (*Subtitle, error) {
	subtitle := &Subtitle{
		Format: SubtitleFormatASS,
		Items:  make([]SubtitleItem, 0),
		Styles: make([]ASSStyle, 0),
	}

	scanner := bufio.NewScanner(reader)
	section := ""

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}

		// 检查区块
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line[1 : len(line)-1]
			continue
		}

		switch strings.ToLower(section) {
		case "script info":
			sm.parseASSScriptInfoLine(subtitle, line)
		case "v4+ styles", "v4 styles":
			sm.parseASSStyleLine(subtitle, line)
		case "events":
			sm.parseASSEventLine(subtitle, line)
		}
	}

	return subtitle, nil
}

// parseASSScriptInfoLine 解析脚本信息行
func (sm *SubtitleManager) parseASSScriptInfoLine(subtitle *Subtitle, line string) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	if subtitle.ScriptInfo.Title == "" {
		subtitle.ScriptInfo = ASSScriptInfo{}
	}

	switch strings.ToLower(key) {
	case "title":
		subtitle.ScriptInfo.Title = value
		subtitle.Title = value
	case "original script":
		subtitle.ScriptInfo.OriginalScript = value
	case "script type":
		subtitle.ScriptInfo.ScriptType = value
	case "collisions":
		subtitle.ScriptInfo.Collisions = value
	case "playdepth":
		if v, err := strconv.Atoi(value); err == nil {
			subtitle.ScriptInfo.PlayDepth = v
		}
	case "playresx":
		if v, err := strconv.Atoi(value); err == nil {
			subtitle.ScriptInfo.PlayResX = v
		}
	case "playresy":
		if v, err := strconv.Atoi(value); err == nil {
			subtitle.ScriptInfo.PlayResY = v
		}
	case "timer":
		if v, err := strconv.ParseFloat(value, 64); err == nil {
			subtitle.ScriptInfo.Timer = v
		}
	case "wrapstyle":
		subtitle.ScriptInfo.WrapStyle = value
	case "scaledborderandshadow":
		subtitle.ScriptInfo.ScaledBorderAndShadow = strings.ToLower(value) == "yes"
	}
}

// parseASSStyleLine 解析样式行
func (sm *SubtitleManager) parseASSStyleLine(subtitle *Subtitle, line string) {
	if strings.HasPrefix(line, "Format:") {
		return // 跳过格式定义行
	}

	if !strings.HasPrefix(line, "Style:") {
		return
	}

	values := strings.Split(line[6:], ",")
	if len(values) < 10 {
		return
	}

	style := ASSStyle{
		Name:     strings.TrimSpace(values[0]),
		FontName: strings.TrimSpace(values[1]),
	}

	if v, err := strconv.ParseFloat(strings.TrimSpace(values[2]), 64); err == nil {
		style.FontSize = v
	}

	style.PrimaryColor = strings.TrimSpace(values[3])
	if len(values) > 4 {
		style.SecondaryColor = strings.TrimSpace(values[4])
	}
	if len(values) > 5 {
		style.OutlineColor = strings.TrimSpace(values[5])
	}
	if len(values) > 6 {
		style.BackColour = strings.TrimSpace(values[6])
	}
	if len(values) > 7 {
		style.Bold = strings.TrimSpace(values[7]) == "-1"
	}
	if len(values) > 8 {
		style.Italic = strings.TrimSpace(values[8]) == "-1"
	}

	subtitle.Styles = append(subtitle.Styles, style)
}

// parseASSEventLine 解析事件行
func (sm *SubtitleManager) parseASSEventLine(subtitle *Subtitle, line string) {
	if strings.HasPrefix(line, "Format:") {
		return
	}

	if !strings.HasPrefix(line, "Dialogue:") {
		return
	}

	values := strings.Split(line[9:], ",")
	if len(values) < 10 {
		return
	}

	item := SubtitleItem{
		Index:     len(subtitle.Items) + 1,
		Style:     strings.TrimSpace(values[0]),
		Name:      strings.TrimSpace(values[1]),
		StartTime: parseASSTime(strings.TrimSpace(values[2])),
		EndTime:   parseASSTime(strings.TrimSpace(values[3])),
	}

	if len(values) > 4 {
		style := values[4]
		item.MarginL, _ = strconv.Atoi(strings.TrimSpace(style))
	}
	if len(values) > 5 {
		item.MarginR, _ = strconv.Atoi(strings.TrimSpace(values[5]))
	}
	if len(values) > 6 {
		item.MarginV, _ = strconv.Atoi(strings.TrimSpace(values[6]))
	}
	if len(values) > 7 {
		item.Effect = strings.TrimSpace(values[7])
	}
	if len(values) > 8 {
		item.Text = strings.Join(values[8:], ",")
		// 清理 ASS 格式代码
		item.Text = cleanASSText(item.Text)
	}

	subtitle.Items = append(subtitle.Items, item)
}

// parseASSTime 解析 ASS 时间格式 (H:MM:SS.cc)
func parseASSTime(s string) time.Duration {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return 0
	}

	hours, _ := strconv.Atoi(parts[0])
	minutes, _ := strconv.Atoi(parts[1])

	secParts := strings.Split(parts[2], ".")
	seconds, _ := strconv.Atoi(secParts[0])
	var centisec int
	if len(secParts) > 1 {
		centisec, _ = strconv.Atoi(secParts[1])
	}

	return time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second +
		time.Duration(centisec)*10*time.Millisecond
}

// cleanASSText 清理 ASS 文本格式代码
func cleanASSText(text string) string {
	// 移除常见的 ASS 标签
	re := regexp.MustCompile(`\{[^}]*\}`)
	text = re.ReplaceAllString(text, "")

	// 替换 \N 为换行
	text = strings.ReplaceAll(text, `\N`, "\n")
	text = strings.ReplaceAll(text, `\n`, "\n")

	return strings.TrimSpace(text)
}

// GenerateSRT 生成 SRT 格式字幕
func (sm *SubtitleManager) GenerateSRT(subtitle *Subtitle) string {
	var sb strings.Builder

	for i, item := range subtitle.Items {
		sb.WriteString(fmt.Sprintf("%d\n", i+1))
		sb.WriteString(fmt.Sprintf("%s --> %s\n",
			formatSRTTime(item.StartTime),
			formatSRTTime(item.EndTime)))
		sb.WriteString(item.Text)
		sb.WriteString("\n\n")
	}

	return sb.String()
}

// formatSRTTime 格式化为 SRT 时间
func formatSRTTime(d time.Duration) string {
	d = d.Round(time.Millisecond)

	hours := d / time.Hour
	d -= hours * time.Hour
	minutes := d / time.Minute
	d -= minutes * time.Minute
	seconds := d / time.Second
	d -= seconds * time.Second
	millis := d / time.Millisecond

	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, seconds, millis)
}

// GenerateVTT 生成 VTT 格式字幕
func (sm *SubtitleManager) GenerateVTT(subtitle *Subtitle) string {
	var sb strings.Builder

	sb.WriteString("WEBVTT\n\n")

	// 添加区域定义
	for _, region := range subtitle.Regions {
		sb.WriteString(fmt.Sprintf("Region: id=%s", region.ID))
		if region.Width > 0 {
			sb.WriteString(fmt.Sprintf(", width=%.0f%%", region.Width))
		}
		if region.Height > 0 {
			sb.WriteString(fmt.Sprintf(", height=%.0f%%", region.Height))
		}
		sb.WriteString("\n")
	}
	if len(subtitle.Regions) > 0 {
		sb.WriteString("\n")
	}

	// 添加字幕项
	for _, item := range subtitle.Items {
		sb.WriteString(fmt.Sprintf("%s --> %s\n",
			formatVTTTime(item.StartTime),
			formatVTTTime(item.EndTime)))
		sb.WriteString(item.Text)
		sb.WriteString("\n\n")
	}

	return sb.String()
}

// formatVTTTime 格式化为 VTT 时间
func formatVTTTime(d time.Duration) string {
	d = d.Round(time.Millisecond)

	hours := d / time.Hour
	d -= hours * time.Hour
	minutes := d / time.Minute
	d -= minutes * time.Minute
	seconds := d / time.Second
	d -= seconds * time.Second
	millis := d / time.Millisecond

	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, millis)
	}
	return fmt.Sprintf("%02d:%02d.%03d", minutes, seconds, millis)
}

// SaveSubtitle 保存字幕文件
func (sm *SubtitleManager) SaveSubtitle(subtitle *Subtitle, path string) error {
	var content string

	switch subtitle.Format {
	case SubtitleFormatSRT:
		content = sm.GenerateSRT(subtitle)
	case SubtitleFormatVTT:
		content = sm.GenerateVTT(subtitle)
	default:
		content = sm.GenerateSRT(subtitle)
	}

	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	return os.WriteFile(path, []byte(content), 0644)
}

// ConvertSubtitle 转换字幕格式
func (sm *SubtitleManager) ConvertSubtitle(inputPath, outputPath string) error {
	subtitle, err := sm.ParseSubtitle(inputPath)
	if err != nil {
		return err
	}

	// 根据输出文件扩展名设置格式
	ext := strings.ToLower(filepath.Ext(outputPath))
	switch ext {
	case ".srt":
		subtitle.Format = SubtitleFormatSRT
	case ".vtt":
		subtitle.Format = SubtitleFormatVTT
	default:
		return fmt.Errorf("不支持的目标格式: %s", ext)
	}

	return sm.SaveSubtitle(subtitle, outputPath)
}

// MergeSubtitles 合并多个字幕文件
func (sm *SubtitleManager) MergeSubtitles(paths []string, outputPath string) error {
	merged := &Subtitle{
		Format: SubtitleFormatSRT,
		Items:  make([]SubtitleItem, 0),
	}

	for _, path := range paths {
		subtitle, err := sm.ParseSubtitle(path)
		if err != nil {
			return fmt.Errorf("解析 %s 失败: %w", path, err)
		}

		// 调整索引并添加
		for _, item := range subtitle.Items {
			item.Index = len(merged.Items) + 1
			merged.Items = append(merged.Items, item)
		}
	}

	// 按开始时间排序
	for i := 0; i < len(merged.Items)-1; i++ {
		for j := i + 1; j < len(merged.Items); j++ {
			if merged.Items[i].StartTime > merged.Items[j].StartTime {
				merged.Items[i], merged.Items[j] = merged.Items[j], merged.Items[i]
			}
		}
	}

	// 重新编号
	for i := range merged.Items {
		merged.Items[i].Index = i + 1
	}

	return sm.SaveSubtitle(merged, outputPath)
}

// ShiftTime 时间偏移
func (sm *SubtitleManager) ShiftTime(subtitle *Subtitle, offset time.Duration) {
	for i := range subtitle.Items {
		subtitle.Items[i].StartTime += offset
		subtitle.Items[i].EndTime += offset

		// 确保不为负数
		if subtitle.Items[i].StartTime < 0 {
			subtitle.Items[i].StartTime = 0
		}
		if subtitle.Items[i].EndTime < 0 {
			subtitle.Items[i].EndTime = 0
		}
	}
}

// ExtractSubtitleFromMKV 从 MKV 文件提取字幕（需要 ffmpeg）
func (sm *SubtitleManager) ExtractSubtitleFromMKV(mkvPath, outputPath string, streamIndex int) error {
	// 使用 ffmpeg 提取字幕流
	args := []string{
		"-i", mkvPath,
		"-map", fmt.Sprintf("0:s:%d", streamIndex),
		"-y", outputPath,
	}

	cmd := execCommand("ffmpeg", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("提取字幕失败: %s", string(output))
	}

	return nil
}

// execCommand 可测试的命令执行
var execCommand = func(name string, args ...string) interface {
	CombinedOutput() ([]byte, error)
} {
	return execCommandReal(name, args...)
}

func execCommandReal(name string, args ...string) interface {
	CombinedOutput() ([]byte, error)
} {
	return exec.Command(name, args...)
}
