// Package collabwhiteboard 提供协作白板功能
package collabwhiteboard

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// Renderer 渲染器.
type Renderer struct {
	mu sync.RWMutex
}

// NewRenderer 创建渲染器.
func NewRenderer() *Renderer {
	return &Renderer{}
}

// SVGElement SVG元素.
type SVGElement struct {
	Tag      string            `json:"tag"`
	Attrs    map[string]string `json:"attrs"`
	Content  string            `json:"content,omitempty"`
	Children []SVGElement      `json:"children,omitempty"`
}

// CanvasCommand Canvas命令.
type CanvasCommand struct {
	Action string      `json:"action"`
	Params interface{} `json:"params"`
}

// RenderToSVG 渲染为SVG.
func (r *Renderer) RenderToSVG(board *Board) (string, error) {
	if board == nil {
		return "", errors.New("白板不能为空")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`,
		board.Width, board.Height, board.Width, board.Height))
	sb.WriteString("\n")

	// 背景
	sb.WriteString(fmt.Sprintf(`<rect width="%d" height="%d" fill="white"/>`, board.Width, board.Height))
	sb.WriteString("\n")

	// 渲染元素
	for _, elem := range board.Elements {
		if !elem.Visible {
			continue
		}
		svg := r.renderElementToSVG(elem)
		sb.WriteString(svg)
		sb.WriteString("\n")
	}

	sb.WriteString("</svg>")

	return sb.String(), nil
}

// renderElementToSVG 渲染单个元素为SVG.
func (r *Renderer) renderElementToSVG(elem Element) string {
	var sb strings.Builder

	transform := ""
	if elem.Rotation != 0 {
		transform = fmt.Sprintf(` transform="rotate(%f %f %f)"`, elem.Rotation, elem.X+elem.Width/2, elem.Y+elem.Height/2)
	}

	switch elem.Type {
	case "stroke":
		sb.WriteString(fmt.Sprintf(`<g id="%s"%s>`, elem.ID, transform))
		// 简化的笔画渲染
		sb.WriteString(fmt.Sprintf(`<path d="M%f %f" stroke="%s" stroke-width="%f" fill="none" opacity="%f"/>`,
			elem.X, elem.Y,
			elem.Style.StrokeColor,
			elem.Style.StrokeWidth,
			elem.Style.Opacity))
		sb.WriteString("</g>")

	case "shape":
		sb.WriteString(fmt.Sprintf(`<g id="%s"%s>`, elem.ID, transform))
		sb.WriteString(r.renderShapeSVG(elem))
		sb.WriteString("</g>")

	case "text":
		sb.WriteString(fmt.Sprintf(`<g id="%s"%s>`, elem.ID, transform))
		sb.WriteString(fmt.Sprintf(`<text x="%f" y="%f" font-family="%s" font-size="%d" fill="%s"`,
			elem.X, elem.Y+float64(elem.Style.FontSize),
			elem.Style.FontFamily,
			elem.Style.FontSize,
			elem.Style.StrokeColor))
		if elem.Style.FontBold {
			sb.WriteString(` font-weight="bold"`)
		}
		if elem.Style.FontItalic {
			sb.WriteString(` font-style="italic"`)
		}
		sb.WriteString(">")
		// 需要从元素数据中获取文本内容
		sb.WriteString("</text>")
		sb.WriteString("</g>")

	case "image":
		sb.WriteString(fmt.Sprintf(`<g id="%s"%s>`, elem.ID, transform))
		sb.WriteString(fmt.Sprintf(`<rect x="%f" y="%f" width="%f" height="%f" fill="#f0f0f0" stroke="#ccc"/>`,
			elem.X, elem.Y, elem.Width, elem.Height))
		sb.WriteString("</g>")
	}

	return sb.String()
}

// renderShapeSVG 渲染形状为SVG.
func (r *Renderer) renderShapeSVG(elem Element) string {
	var sb strings.Builder

	style := fmt.Sprintf(`stroke="%s" stroke-width="%f" fill="%s" opacity="%f"`,
		elem.Style.StrokeColor,
		elem.Style.StrokeWidth,
		elem.Style.FillColor,
		elem.Style.Opacity)

	// 根据样式判断形状类型
	if elem.Width > 0 && elem.Height > 0 {
		if elem.Style.BorderRadius > 0 {
			sb.WriteString(fmt.Sprintf(`<rect x="%f" y="%f" width="%f" height="%f" rx="%f" %s/>`,
				elem.X, elem.Y, elem.Width, elem.Height, elem.Style.BorderRadius, style))
		} else {
			sb.WriteString(fmt.Sprintf(`<rect x="%f" y="%f" width="%f" height="%f" %s/>`,
				elem.X, elem.Y, elem.Width, elem.Height, style))
		}
	}

	return sb.String()
}

// RenderToCanvas 渲染为Canvas命令.
func (r *Renderer) RenderToCanvas(board *Board) ([]CanvasCommand, error) {
	if board == nil {
		return nil, errors.New("白板不能为空")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	commands := make([]CanvasCommand, 0)

	// 清空画布
	commands = append(commands, CanvasCommand{
		Action: "clear",
		Params: map[string]interface{}{
			"width":  board.Width,
			"height": board.Height,
		},
	})

	// 绘制背景
	commands = append(commands, CanvasCommand{
		Action: "fillRect",
		Params: map[string]interface{}{
			"x":      0,
			"y":      0,
			"width":  board.Width,
			"height": board.Height,
			"color":  "white",
		},
	})

	// 绘制元素
	for _, elem := range board.Elements {
		if !elem.Visible {
			continue
		}
		elemCommands := r.renderElementToCanvas(elem)
		commands = append(commands, elemCommands...)
	}

	return commands, nil
}

// renderElementToCanvas 渲染元素为Canvas命令.
func (r *Renderer) renderElementToCanvas(elem Element) []CanvasCommand {
	commands := make([]CanvasCommand, 0)

	// 保存状态
	commands = append(commands, CanvasCommand{Action: "save", Params: nil})

	// 应用变换
	if elem.Rotation != 0 {
		commands = append(commands, CanvasCommand{
			Action: "rotate",
			Params: map[string]interface{}{
				"angle": elem.Rotation,
				"x":     elem.X + elem.Width/2,
				"y":     elem.Y + elem.Height/2,
			},
		})
	}

	// 设置透明度
	if elem.Style.Opacity > 0 && elem.Style.Opacity < 1 {
		commands = append(commands, CanvasCommand{
			Action: "setAlpha",
			Params: map[string]interface{}{
				"alpha": elem.Style.Opacity,
			},
		})
	}

	switch elem.Type {
	case "stroke":
		commands = append(commands, CanvasCommand{
			Action: "beginPath",
			Params: nil,
		})
		commands = append(commands, CanvasCommand{
			Action: "moveTo",
			Params: map[string]interface{}{
				"x": elem.X,
				"y": elem.Y,
			},
		})
		commands = append(commands, CanvasCommand{
			Action: "stroke",
			Params: map[string]interface{}{
				"color":     elem.Style.StrokeColor,
				"lineWidth": elem.Style.StrokeWidth,
			},
		})

	case "shape":
		if elem.Style.FillColor != "" {
			commands = append(commands, CanvasCommand{
				Action: "fillRect",
				Params: map[string]interface{}{
					"x":      elem.X,
					"y":      elem.Y,
					"width":  elem.Width,
					"height": elem.Height,
					"color":  elem.Style.FillColor,
				},
			})
		}
		commands = append(commands, CanvasCommand{
			Action: "strokeRect",
			Params: map[string]interface{}{
				"x":         elem.X,
				"y":         elem.Y,
				"width":     elem.Width,
				"height":    elem.Height,
				"color":     elem.Style.StrokeColor,
				"lineWidth": elem.Style.StrokeWidth,
			},
		})

	case "text":
		commands = append(commands, CanvasCommand{
			Action: "fillText",
			Params: map[string]interface{}{
				"x":        elem.X,
				"y":        elem.Y,
				"font":     fmt.Sprintf("%s %dpx", elem.Style.FontFamily, elem.Style.FontSize),
				"color":    elem.Style.StrokeColor,
				"text":     "",
			},
		})
	}

	// 恢复状态
	commands = append(commands, CanvasCommand{Action: "restore", Params: nil})

	return commands
}

// ExportToJSON 导出为JSON.
func (r *Renderer) ExportToJSON(board *Board) ([]byte, error) {
	if board == nil {
		return nil, errors.New("白板不能为空")
	}

	return json.MarshalIndent(board, "", "  ")
}

// ExportToPDF 导出为PDF.
func (r *Renderer) ExportToPDF(board *Board) ([]byte, error) {
	if board == nil {
		return nil, errors.New("白板不能为空")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// 生成简化的PDF内容
	// 实际应用中需要使用PDF库
	var sb strings.Builder

	sb.WriteString("%PDF-1.4\n")
	sb.WriteString("1 0 obj\n")
	sb.WriteString("<< /Type /Catalog /Pages 2 0 R >>\n")
	sb.WriteString("endobj\n")

	sb.WriteString("2 0 obj\n")
	sb.WriteString(fmt.Sprintf("<< /Type /Pages /Kids [3 0 R] /Count 1 >>\n"))
	sb.WriteString("endobj\n")

	sb.WriteString("3 0 obj\n")
	sb.WriteString(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %d %d] >>\n", board.Width, board.Height))
	sb.WriteString("endobj\n")

	sb.WriteString("xref\n")
	sb.WriteString("0 4\n")
	sb.WriteString("trailer\n")
	sb.WriteString("<< /Size 4 /Root 1 0 R >>\n")
	sb.WriteString("%%EOF\n")

	return []byte(sb.String()), nil
}

// ExportToImage 导出为图片.
func (r *Renderer) ExportToImage(board *Board, format string) ([]byte, error) {
	if board == nil {
		return nil, errors.New("白板不能为空")
	}

	if format != "png" && format != "jpeg" {
		return nil, errors.New("不支持的图片格式")
	}

	// 先渲染为SVG
	svg, err := r.RenderToSVG(board)
	if err != nil {
		return nil, err
	}

	// 返回SVG内容（实际应用中需要转换为图片）
	return []byte(svg), nil
}

// ExportToSVGFile 导出为SVG文件.
func (r *Renderer) ExportToSVGFile(board *Board) ([]byte, error) {
	svg, err := r.RenderToSVG(board)
	if err != nil {
		return nil, err
	}

	return []byte(svg), nil
}

// RenderElementPreview 渲染元素预览.
func (r *Renderer) RenderElementPreview(elem Element, width, height int) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="%f %f %f %f">`,
		width, height, elem.X, elem.Y, elem.Width, elem.Height))

	sb.WriteString(r.renderElementToSVG(elem))
	sb.WriteString("</svg>")

	return sb.String()
}

// BatchRender 批量渲染.
func (r *Renderer) BatchRender(board *Board, format string) (interface{}, error) {
	if board == nil {
		return nil, errors.New("白板不能为空")
	}

	switch format {
	case "svg":
		return r.RenderToSVG(board)
	case "canvas":
		return r.RenderToCanvas(board)
	case "json":
		return r.ExportToJSON(board)
	default:
		return nil, errors.New("不支持的格式")
	}
}

// GetRenderStats 获取渲染统计.
func (r *Renderer) GetRenderStats(board *Board) map[string]int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := map[string]int{
		"total":     len(board.Elements),
		"visible":   0,
		"hidden":    0,
		"locked":    0,
		"unlocked":  0,
		"strokes":   0,
		"shapes":    0,
		"texts":     0,
		"images":    0,
	}

	for _, elem := range board.Elements {
		if elem.Visible {
			stats["visible"]++
		} else {
			stats["hidden"]++
		}

		if elem.Locked {
			stats["locked"]++
		} else {
			stats["unlocked"]++
		}

		switch elem.Type {
		case "stroke":
			stats["strokes"]++
		case "shape":
			stats["shapes"]++
		case "text":
			stats["texts"]++
		case "image":
			stats["images"]++
		}
	}

	return stats
}

// OptimizeSVG 优化SVG.
func (r *Renderer) OptimizeSVG(svg string) string {
	// 移除注释
	svg = strings.ReplaceAll(svg, "<!--", "")
	svg = strings.ReplaceAll(svg, "-->", "")

	// 移除多余空格
	svg = strings.Join(strings.Fields(svg), " ")

	// 移除空属性
	svg = strings.ReplaceAll(svg, `=""`, "")

	return svg
}
