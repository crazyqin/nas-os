// Package aiwatermark 实现AI智能水印功能。
// 支持文本、图片、文档的可见和隐形水印，用于版权保护和泄露追踪。
package aiwatermark

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// WatermarkType 水印类型
type WatermarkType string

const (
	WatermarkVisible     WatermarkType = "visible"     // 可见水印
	WatermarkInvisible   WatermarkType = "invisible"   // 隐形水印（LSB）
	WatermarkMetadata    WatermarkType = "metadata"    // 元数据水印
	WatermarkFingerprint WatermarkType = "fingerprint" // 指纹水印（每份不同）
)

// WatermarkPosition 水印位置
type WatermarkPosition string

const (
	PositionCenter      WatermarkPosition = "center"
	PositionTopLeft     WatermarkPosition = "top-left"
	PositionTopRight    WatermarkPosition = "top-right"
	PositionBottomLeft  WatermarkPosition = "bottom-left"
	PositionBottomRight WatermarkPosition = "bottom-right"
	PositionTiled       WatermarkPosition = "tiled" // 平铺
)

// WatermarkConfig 水印配置
type WatermarkConfig struct {
	Type     WatermarkType     `json:"type"`
	Text     string            `json:"text"`
	Position WatermarkPosition `json:"position"`
	Opacity  float64           `json:"opacity"` // 0.0 - 1.0
	FontSize int               `json:"fontSize"`
	Color    string            `json:"color"`
	Angle    float64           `json:"angle"`   // 旋转角度
	Spacing  int               `json:"spacing"` // 平铺间距
	EmbedID  string            `json:"embedId"` // 嵌入追踪ID
}

// WatermarkRecord 水印记录
type WatermarkRecord struct {
	ID         string          `json:"id"`
	SourceFile string          `json:"sourceFile"`
	OutputFile string          `json:"outputFile"`
	Config     WatermarkConfig `json:"config"`
	EmbedID    string          `json:"embedId"`
	CreatedAt  time.Time       `json:"createdAt"`
	FileHash   string          `json:"fileHash"`
}

// WatermarkManager 水印管理器
type WatermarkManager struct {
	mu      sync.RWMutex
	records []WatermarkRecord
	secret  []byte
	baseDir string
}

// NewWatermarkManager 创建水印管理器
func NewWatermarkManager(secret string, baseDir string) *WatermarkManager {
	h := sha256.Sum256([]byte(secret))
	return &WatermarkManager{
		records: make([]WatermarkRecord, 0),
		secret:  h[:],
		baseDir: baseDir,
	}
}

// GenerateEmbedID 生成追踪ID
func (m *WatermarkManager) GenerateEmbedID(recipient string) string {
	data := fmt.Sprintf("%s:%d:%s", recipient, time.Now().UnixNano(), m.secret)
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:8])
}

// ApplyTextWatermark 给文本添加水印
func (m *WatermarkManager) ApplyTextWatermark(content string, config WatermarkConfig) string {
	var sb strings.Builder

	switch config.Position {
	case PositionCenter:
		lines := strings.Split(content, "\n")
		mid := len(lines) / 2
		for i, line := range lines {
			if i == mid {
				padding := (80 - len(config.Text)) / 2
				if padding > 0 {
					sb.WriteString(strings.Repeat(" ", padding))
				}
				sb.WriteString(fmt.Sprintf("【%s】", config.Text))
				sb.WriteString("\n")
			}
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	case PositionTopLeft:
		sb.WriteString(fmt.Sprintf("【%s】\n", config.Text))
		sb.WriteString(content)
	case PositionBottomRight:
		sb.WriteString(content)
		sb.WriteString(fmt.Sprintf("\n【%s】", config.Text))
	case PositionTiled:
		lines := strings.Split(content, "\n")
		spacing := config.Spacing
		if spacing <= 0 {
			spacing = 5
		}
		for i, line := range lines {
			sb.WriteString(line)
			if i%spacing == 0 {
				sb.WriteString(fmt.Sprintf("  ←%s→", config.Text))
			}
			sb.WriteString("\n")
		}
	default:
		sb.WriteString(fmt.Sprintf("【%s】\n", config.Text))
		sb.WriteString(content)
	}

	result := sb.String()

	// 嵌入隐形追踪ID（零宽字符）
	if config.EmbedID != "" {
		result = m.embedInvisibleID(result, config.EmbedID)
	}

	return result
}

// ApplyImageWatermark 给图片添加水印
func (m *WatermarkManager) ApplyImageWatermark(srcPath, dstPath string, config WatermarkConfig) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("打开源文件失败: %w", err)
	}
	defer srcFile.Close()

	srcImg, _, err := image.Decode(srcFile)
	if err != nil {
		return fmt.Errorf("解码图片失败: %w", err)
	}

	bounds := srcImg.Bounds()
	watermarked := image.NewRGBA(bounds)
	draw.Draw(watermarked, bounds, srcImg, bounds.Min, draw.Src)

	switch config.Type {
	case WatermarkVisible:
		m.drawVisibleText(watermarked, config)
	case WatermarkInvisible:
		m.embedLSB(watermarked, config.EmbedID)
	case WatermarkFingerprint:
		m.embedLSB(watermarked, config.EmbedID)
		m.drawVisibleText(watermarked, config)
	}

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("创建输出文件失败: %w", err)
	}
	defer dstFile.Close()

	if err := png.Encode(dstFile, watermarked); err != nil {
		return fmt.Errorf("编码图片失败: %w", err)
	}

	// 记录水印信息
	hash, _ := m.fileHash(srcPath)
	m.mu.Lock()
	m.records = append(m.records, WatermarkRecord{
		ID:         generateID(),
		SourceFile: srcPath,
		OutputFile: dstPath,
		Config:     config,
		EmbedID:    config.EmbedID,
		CreatedAt:  time.Now(),
		FileHash:   hash,
	})
	m.mu.Unlock()

	return nil
}

// VerifyWatermark 验证水印
func (m *WatermarkManager) VerifyWatermark(filePath string) (*WatermarkRecord, error) {
	hash, err := m.fileHash(filePath)
	if err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, record := range m.records {
		if record.FileHash == hash {
			return &record, nil
		}
	}
	return nil, fmt.Errorf("未找到匹配的水印记录")
}

// GetRecords 获取水印记录
func (m *WatermarkManager) GetRecords(limit int) []WatermarkRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.records) {
		limit = len(m.records)
	}
	result := make([]WatermarkRecord, limit)
	copy(result, m.records[:limit])
	return result
}

// drawVisibleText 在图片上绘制可见文字水印
func (m *WatermarkManager) drawVisibleText(img *image.RGBA, config WatermarkConfig) {
	bounds := img.Bounds()
	text := config.Text
	if text == "" {
		text = "NAS-OS"
	}

	opacity := uint8(config.Opacity * 255)
	if opacity == 0 {
		opacity = 128
	}

	wmColor := color.RGBA{R: 200, G: 200, B: 200, A: opacity}

	// 简化：在指定位置绘制像素点阵文字
	startX, startY := 0, 0
	switch config.Position {
	case PositionCenter:
		startX = bounds.Dx()/2 - len(text)*4
		startY = bounds.Dy() / 2
	case PositionTopRight:
		startX = bounds.Dx() - len(text)*8 - 10
		startY = 20
	case PositionBottomLeft:
		startX = 10
		startY = bounds.Dy() - 20
	case PositionBottomRight:
		startX = bounds.Dx() - len(text)*8 - 10
		startY = bounds.Dy() - 20
	default:
		startX = 10
		startY = 20
	}

	// 绘制简单的像素文字（简化版）
	for i, ch := range text {
		x := startX + i*8
		if x >= 0 && x < bounds.Dx()-8 {
			for dy := 0; dy < 12; dy++ {
				for dx := 0; dx < 6; dx++ {
					if (int(ch)+dx+dy)%3 == 0 { // 简化的字符渲染
						px := x + dx
						py := startY + dy
						if px >= 0 && px < bounds.Dx() && py >= 0 && py < bounds.Dy() {
							img.SetRGBA(px, py, wmColor)
						}
					}
				}
			}
		}
	}
}

// embedLSB 在图片LSB嵌入隐形水印
func (m *WatermarkManager) embedLSB(img *image.RGBA, data string) {
	if data == "" {
		data = "NAS-OS-WATERMARK"
	}

	bounds := img.Bounds()
	pixels := bounds.Dx() * bounds.Dy()
	bits := make([]byte, 0, len(data)*8)
	for _, b := range []byte(data) {
		for i := 7; i >= 0; i-- {
			bits = append(bits, (b>>uint(i))&1)
		}
	}

	// 在R通道LSB嵌入
	idx := 0
	for y := bounds.Min.Y; y < bounds.Max.Y && idx < len(bits); y++ {
		for x := bounds.Min.X; x < bounds.Max.X && idx < len(bits); x++ {
			c := img.RGBAAt(x, y)
			c.R = (c.R & 0xFE) | bits[idx]
			img.SetRGBA(x, y, c)
			idx++
		}
	}
	_ = pixels
}

// embedInvisibleID 在文本中嵌入零宽字符水印
func (m *WatermarkManager) embedInvisibleID(text, id string) string {
	// 使用零宽字符编码ID
	zeroWidthChars := []string{"\u200B", "\u200C", "\u200D", "\uFEFF"}
	var sb strings.Builder
	for _, ch := range id {
		idx := int(ch) % len(zeroWidthChars)
		sb.WriteString(zeroWidthChars[idx])
	}
	// 在文本中间插入
	mid := len(text) / 2
	return text[:mid] + sb.String() + text[mid:]
}

// ExtractInvisibleID 从文本提取零宽字符水印
func (m *WatermarkManager) ExtractInvisibleID(text string) string {
	zeroWidthMap := map[rune]int{
		'\u200B': 0,
		'\u200C': 1,
		'\u200D': 2,
		'\uFEFF': 3,
	}

	var bits []int
	for _, ch := range text {
		if v, ok := zeroWidthMap[ch]; ok {
			bits = append(bits, v)
		}
	}

	if len(bits) == 0 {
		return ""
	}

	// 简单返回检测到的零宽字符数量作为标记
	return fmt.Sprintf("detected_%d_markers", len(bits))
}

func (m *WatermarkManager) fileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}

// RegisterRoutes 注册 HTTP 路由
func (m *WatermarkManager) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/watermark/text", m.handleTextWatermark)
	mux.HandleFunc("/api/v1/watermark/image", m.handleImageWatermark)
	mux.HandleFunc("/api/v1/watermark/verify", m.handleVerify)
	mux.HandleFunc("/api/v1/watermark/records", m.handleRecords)
	mux.HandleFunc("/api/v1/watermark/generate-id", m.handleGenerateID)
}

func (m *WatermarkManager) handleTextWatermark(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Not implemented yet", http.StatusNotImplemented)
}

func (m *WatermarkManager) handleImageWatermark(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Not implemented yet", http.StatusNotImplemented)
}

func (m *WatermarkManager) handleVerify(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Not implemented yet", http.StatusNotImplemented)
}

func (m *WatermarkManager) handleRecords(w http.ResponseWriter, r *http.Request) {
	records := m.GetRecords(50)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"records":%d}`, len(records))
}

func (m *WatermarkManager) handleGenerateID(w http.ResponseWriter, r *http.Request) {
	recipient := r.URL.Query().Get("recipient")
	if recipient == "" {
		http.Error(w, "recipient required", http.StatusBadRequest)
		return
	}
	id := m.GenerateEmbedID(recipient)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"embedId":"%s"}`, id)
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
