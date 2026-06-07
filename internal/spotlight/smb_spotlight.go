// Package spotlight 提供 macOS Spotlight 协议兼容的 SMB 搜索支持
// 实现 Netatalk Spotlight 协议格式，支持 kMDQuery 查询语法
package spotlight

import (
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"time"
)

// SMBSpotlightProtocol SMB Spotlight 协议处理器
// 基于 Netatalk Spotlight 协议实现 macOS 原生搜索兼容
type SMBSpotlightProtocol struct {
	engine *Engine
}

// NewSMBSpotlightProtocol 创建 SMB Spotlight 协议处理器
func NewSMBSpotlightProtocol(engine *Engine) *SMBSpotlightProtocol {
	return &SMBSpotlightProtocol{
		engine: engine,
	}
}

// SpotlightMessage Spotlight 协议消息类型
type SpotlightMessage struct {
	Command    SpotlightCommand
	Flags      uint32
	QueryID    uint64
	Query      string
	Attributes []string
	MaxResults uint32
}

// SpotlightCommand Spotlight 命令类型
type SpotlightCommand uint32

const (
	CmdQuery      SpotlightCommand = 0x01 // 查询请求
	CmdFetch      SpotlightCommand = 0x02 // 获取结果
	CmdClose      SpotlightCommand = 0x03 // 关闭查询
	CmdPing       SpotlightCommand = 0x04 // 心跳
	CmdRegister   SpotlightCommand = 0x05 // 注册通知
	CmdUnregister SpotlightCommand = 0x06 // 取消注册
)

// SpotlightResponse Spotlight 协议响应
type SpotlightResponse struct {
	Command    SpotlightCommand
	Status     uint32
	QueryID    uint64
	Results    []SpotlightResult
	TotalCount uint32
	HasMore    bool
}

// SpotlightResult Spotlight 单条结果
type SpotlightResult struct {
	Path       string
	Attributes map[string]interface{}
}

// SpotlightAttribute Spotlight 文件属性
type SpotlightAttribute struct {
	Name      string
	ValueType uint32
	Value     interface{}
}

// kMDQuery 属性常量
const (
	kMDItemDisplayName         = "kMDItemDisplayName"
	kMDItemPath                = "kMDItemPath"
	kMDItemFSSize              = "kMDItemFSSize"
	kMDItemFSCreationDate      = "kMDItemFSCreationDate"
	kMDItemFSContentChangeDate = "kMDItemFSContentChangeDate"
	kMDItemContentType         = "kMDItemContentType"
	kMDItemKind                = "kMDItemKind"
	kMDItemTextContent         = "kMDItemTextContent"
	kMDItemKeywords            = "kMDItemKeywords"
	kMDItemFSName              = "kMDItemFSName"
	kMDItemContentTypeTree     = "kMDItemContentTypeTree"
	kMDItemUsedDates           = "kMDItemUsedDates"
	kMDItemLastUsedDate        = "kMDItemLastUsedDate"
	kMDItemDurationSeconds     = "kMDItemDurationSeconds"
	kMDItemPixelHeight         = "kMDItemPixelHeight"
	kMDItemPixelWidth          = "kMDItemPixelWidth"
)

// HandleRequest 处理 Spotlight 请求
func (s *SMBSpotlightProtocol) HandleRequest(reader io.Reader) (*SpotlightResponse, error) {
	// 读取消息头
	header := make([]byte, 16)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, fmt.Errorf("读取消息头失败: %w", err)
	}

	// 解析命令
	cmd := SpotlightCommand(binary.BigEndian.Uint32(header[0:4]))
	flags := binary.BigEndian.Uint32(header[4:8])
	queryID := binary.BigEndian.Uint64(header[8:16])

	switch cmd {
	case CmdQuery:
		return s.handleQuery(reader, flags, queryID)
	case CmdFetch:
		return s.handleFetch(reader, queryID)
	case CmdClose:
		return s.handleClose(queryID)
	case CmdPing:
		return s.handlePing(), nil
	default:
		return nil, fmt.Errorf("未知命令: %d", cmd)
	}
}

// handleQuery 处理查询请求
func (s *SMBSpotlightProtocol) handleQuery(reader io.Reader, flags uint32, queryID uint64) (*SpotlightResponse, error) {
	// 读取查询长度
	queryLen := make([]byte, 4)
	if _, err := io.ReadFull(reader, queryLen); err != nil {
		return nil, fmt.Errorf("读取查询长度失败: %w", err)
	}
	length := binary.BigEndian.Uint32(queryLen)

	// 读取查询字符串
	queryBuf := make([]byte, length)
	if _, err := io.ReadFull(reader, queryBuf); err != nil {
		return nil, fmt.Errorf("读取查询内容失败: %w", err)
	}
	query := string(queryBuf)

	// 读取最大结果数
	maxResultsBuf := make([]byte, 4)
	if _, err := io.ReadFull(reader, maxResultsBuf); err != nil {
		return nil, fmt.Errorf("读取最大结果数失败: %w", err)
	}
	maxResults := binary.BigEndian.Uint32(maxResultsBuf)

	// 读取属性数量
	attrCountBuf := make([]byte, 4)
	if _, err := io.ReadFull(reader, attrCountBuf); err != nil {
		return nil, fmt.Errorf("读取属性数量失败: %w", err)
	}
	attrCount := binary.BigEndian.Uint32(attrCountBuf)

	// 读取请求的属性列表
	attributes := make([]string, 0, attrCount)
	for i := uint32(0); i < attrCount; i++ {
		attrLenBuf := make([]byte, 4)
		if _, err := io.ReadFull(reader, attrLenBuf); err != nil {
			return nil, fmt.Errorf("读取属性长度失败: %w", err)
		}
		attrLen := binary.BigEndian.Uint32(attrLenBuf)

		attrBuf := make([]byte, attrLen)
		if _, err := io.ReadFull(reader, attrBuf); err != nil {
			return nil, fmt.Errorf("读取属性名称失败: %w", err)
		}
		attributes = append(attributes, string(attrBuf))
	}

	// 转换为内部查询格式
	engineReq := s.convertQuery(query, maxResults)

	// 执行搜索
	ctx := s.engine.ctx
	result, err := s.engine.Search(ctx, engineReq)
	if err != nil {
		return &SpotlightResponse{
			Command: CmdQuery,
			Status:  1, // 错误
			QueryID: queryID,
		}, nil
	}

	// 转换为 Spotlight 响应格式
	spotlightResults := s.convertResults(result, attributes)

	return &SpotlightResponse{
		Command:    CmdQuery,
		Status:     0,
		QueryID:    queryID,
		Results:    spotlightResults,
		TotalCount: uint32(result.Total),
		HasMore:    len(spotlightResults) < result.Total,
	}, nil
}

// handleFetch 处理获取更多结果
func (s *SMBSpotlightProtocol) handleFetch(reader io.Reader, queryID uint64) (*SpotlightResponse, error) {
	// 读取偏移量和限制
	offsetBuf := make([]byte, 8)
	if _, err := io.ReadFull(reader, offsetBuf); err != nil {
		return nil, fmt.Errorf("读取偏移量失败: %w", err)
	}
	offset := binary.BigEndian.Uint64(offsetBuf)

	limitBuf := make([]byte, 4)
	if _, err := io.ReadFull(reader, limitBuf); err != nil {
		return nil, fmt.Errorf("读取限制数量失败: %w", err)
	}
	limit := binary.BigEndian.Uint32(limitBuf)

	// TODO: 实现查询结果缓存和分页
	_ = offset
	_ = limit

	return &SpotlightResponse{
		Command: CmdFetch,
		Status:  0,
		QueryID: queryID,
	}, nil
}

// handleClose 处理关闭查询
func (s *SMBSpotlightProtocol) handleClose(queryID uint64) (*SpotlightResponse, error) {
	// TODO: 清理查询资源
	return &SpotlightResponse{
		Command: CmdClose,
		Status:  0,
		QueryID: queryID,
	}, nil
}

// handlePing 处理心跳
func (s *SMBSpotlightProtocol) handlePing() *SpotlightResponse {
	return &SpotlightResponse{
		Command: CmdPing,
		Status:  0,
	}
}

// convertQuery 将 kMDQuery 转换为内部查询格式
func (s *SMBSpotlightProtocol) convertQuery(kmdQuery string, maxResults uint32) EngineSearchRequest {
	req := EngineSearchRequest{
		Query: kmdQuery,
		Limit: int(maxResults),
	}

	// 解析 kMDQuery 格式
	// 示例: "kMDItemDisplayName == "test*" && kMDItemContentType == "public.text""
	query := strings.TrimSpace(kmdQuery)

	// 处理属性查询
	attributes := parseKMDQueryAttributes(query)
	for attr, value := range attributes {
		switch attr {
		case kMDItemDisplayName, kMDItemFSName:
			req.Query = strings.Trim(value, "\"*")
		case kMDItemPath:
			req.Path = strings.Trim(value, "\"")
		case kMDItemContentType:
			req.FileTypes = append(req.FileTypes, strings.Trim(value, "\""))
		case kMDItemFSSize:
			// 解析大小范围
			size := parseSizeValue(value)
			if strings.HasPrefix(value, ">") {
				req.SizeMin = size
			} else if strings.HasPrefix(value, "<") {
				req.SizeMax = size
			}
		case kMDItemFSCreationDate, kMDItemFSContentChangeDate:
			// 解析日期范围
			date := parseDateValue(value)
			if !date.IsZero() {
				if strings.HasPrefix(value, ">") {
					req.DateStart = date
				} else if strings.HasPrefix(value, "<") {
					req.DateEnd = date
				}
			}
		}
	}

	return req
}

// parseKMDQueryAttributes 解析 kMDQuery 属性
func parseKMDQueryAttributes(query string) map[string]string {
	attributes := make(map[string]string)

	// 简单的属性解析（支持 == 和 != 操作符）
	// 格式: kMDItemDisplayName == "value"
	parts := strings.Split(query, "&&")
	for _, part := range parts {
		part = strings.TrimSpace(part)

		// 查找 == 操作符
		if idx := strings.Index(part, "=="); idx >= 0 {
			key := strings.TrimSpace(part[:idx])
			value := strings.TrimSpace(part[idx+2:])
			if isValidKMDAttribute(key) {
				attributes[key] = value
			}
		}

		// 查找 != 操作符
		if idx := strings.Index(part, "!="); idx >= 0 {
			key := strings.TrimSpace(part[:idx])
			value := strings.TrimSpace(part[idx+2:])
			if isValidKMDAttribute(key) {
				// 否定查询，暂不处理
				_ = value
			}
		}
	}

	return attributes
}

// isValidKMDAttribute 检查是否为有效的 kMD 属性
func isValidKMDAttribute(attr string) bool {
	validAttrs := map[string]bool{
		kMDItemDisplayName:         true,
		kMDItemPath:                true,
		kMDItemFSSize:              true,
		kMDItemFSCreationDate:      true,
		kMDItemFSContentChangeDate: true,
		kMDItemContentType:         true,
		kMDItemKind:                true,
		kMDItemTextContent:         true,
		kMDItemKeywords:            true,
		kMDItemFSName:              true,
		kMDItemContentTypeTree:     true,
	}
	return validAttrs[attr]
}

// parseSizeValue 解析大小值
func parseSizeValue(value string) int64 {
	value = strings.TrimPrefix(value, ">")
	value = strings.TrimPrefix(value, "<")
	value = strings.TrimSpace(value)

	var multiplier int64 = 1
	if strings.HasSuffix(value, "KB") {
		multiplier = 1024
		value = strings.TrimSuffix(value, "KB")
	} else if strings.HasSuffix(value, "MB") {
		multiplier = 1024 * 1024
		value = strings.TrimSuffix(value, "MB")
	} else if strings.HasSuffix(value, "GB") {
		multiplier = 1024 * 1024 * 1024
		value = strings.TrimSuffix(value, "GB")
	}

	var size int64
	fmt.Sscanf(value, "%d", &size)
	return size * multiplier
}

// parseDateValue 解析日期值
func parseDateValue(value string) time.Time {
	value = strings.TrimPrefix(value, ">")
	value = strings.TrimPrefix(value, "<")
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "\"")

	// 尝试多种日期格式
	formats := []string{
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02",
		time.RFC3339,
	}

	for _, format := range formats {
		if t, err := time.Parse(format, value); err == nil {
			return t
		}
	}

	return time.Time{}
}

// convertResults 将搜索结果转换为 Spotlight 格式
func (s *SMBSpotlightProtocol) convertResults(resp *SearchResponse, requestedAttrs []string) []SpotlightResult {
	results := make([]SpotlightResult, 0, len(resp.Results))

	for _, entry := range resp.Results {
		result := SpotlightResult{
			Path:       entry.Path,
			Attributes: make(map[string]interface{}),
		}

		// 填充请求的属性
		for _, attr := range requestedAttrs {
			switch attr {
			case kMDItemDisplayName, kMDItemFSName:
				result.Attributes[attr] = entry.Name
			case kMDItemPath:
				result.Attributes[attr] = entry.Path
			case kMDItemFSSize:
				result.Attributes[attr] = entry.Size
			case kMDItemFSCreationDate:
				result.Attributes[attr] = entry.CreateTime.Unix()
			case kMDItemFSContentChangeDate:
				result.Attributes[attr] = entry.ModTime.Unix()
			case kMDItemContentType:
				result.Attributes[attr] = entry.MimeType
			case kMDItemKind:
				result.Attributes[attr] = getFileKind(entry.Ext)
			case kMDItemTextContent:
				if len(entry.Content) > 500 {
					result.Attributes[attr] = entry.Content[:500]
				} else {
					result.Attributes[attr] = entry.Content
				}
			case kMDItemKeywords:
				result.Attributes[attr] = entry.Keywords
			}
		}

		// 默认属性
		if _, ok := result.Attributes[kMDItemDisplayName]; !ok {
			result.Attributes[kMDItemDisplayName] = entry.Name
		}
		if _, ok := result.Attributes[kMDItemPath]; !ok {
			result.Attributes[kMDItemPath] = entry.Path
		}

		results = append(results, result)
	}

	return results
}

// getFileKind 获取文件类型描述
func getFileKind(ext string) string {
	kindMap := map[string]string{
		".txt":  "纯文本文档",
		".pdf":  "PDF 文档",
		".doc":  "Word 文档",
		".docx": "Word 文档",
		".xls":  "Excel 表格",
		".xlsx": "Excel 表格",
		".ppt":  "PowerPoint 演示文稿",
		".pptx": "PowerPoint 演示文稿",
		".jpg":  "JPEG 图像",
		".jpeg": "JPEG 图像",
		".png":  "PNG 图像",
		".gif":  "GIF 图像",
		".mp4":  "MP4 视频",
		".avi":  "AVI 视频",
		".mp3":  "MP3 音频",
		".flac": "FLAC 音频",
		".zip":  "ZIP 压缩包",
		".tar":  "TAR 归档文件",
		".gz":   "GZIP 压缩文件",
		".go":   "Go 源代码",
		".py":   "Python 源代码",
		".js":   "JavaScript 源代码",
		".java": "Java 源代码",
		".html": "HTML 文档",
		".css":  "CSS 样式表",
		".json": "JSON 数据",
		".xml":  "XML 文档",
		".md":   "Markdown 文档",
	}

	if kind, ok := kindMap[strings.ToLower(ext)]; ok {
		return kind
	}
	return "文件"
}

// EncodeResponse 编码 Spotlight 响应为二进制格式
func EncodeResponse(resp *SpotlightResponse) ([]byte, error) {
	buf := make([]byte, 0, 1024)

	// 写入响应头
	header := make([]byte, 24)
	binary.BigEndian.PutUint32(header[0:4], uint32(resp.Command))
	binary.BigEndian.PutUint32(header[4:8], resp.Status)
	binary.BigEndian.PutUint64(header[8:16], resp.QueryID)
	binary.BigEndian.PutUint32(header[16:20], resp.TotalCount)
	if resp.HasMore {
		binary.BigEndian.PutUint32(header[20:24], 1)
	} else {
		binary.BigEndian.PutUint32(header[20:24], 0)
	}
	buf = append(buf, header...)

	// 写入结果数量
	resultCount := make([]byte, 4)
	binary.BigEndian.PutUint32(resultCount, uint32(len(resp.Results)))
	buf = append(buf, resultCount...)

	// 写入每个结果
	for _, result := range resp.Results {
		// 写入路径
		pathBytes := []byte(result.Path)
		pathLen := make([]byte, 4)
		binary.BigEndian.PutUint32(pathLen, uint32(len(pathBytes)))
		buf = append(buf, pathLen...)
		buf = append(buf, pathBytes...)

		// 写入属性数量
		attrCount := make([]byte, 4)
		binary.BigEndian.PutUint32(attrCount, uint32(len(result.Attributes)))
		buf = append(buf, attrCount...)

		// 写入每个属性
		for key, value := range result.Attributes {
			// 属性名
			keyBytes := []byte(key)
			keyLen := make([]byte, 4)
			binary.BigEndian.PutUint32(keyLen, uint32(len(keyBytes)))
			buf = append(buf, keyLen...)
			buf = append(buf, keyBytes...)

			// 属性值
			valueStr := fmt.Sprintf("%v", value)
			valueBytes := []byte(valueStr)
			valueLen := make([]byte, 4)
			binary.BigEndian.PutUint32(valueLen, uint32(len(valueBytes)))
			buf = append(buf, valueLen...)
			buf = append(buf, valueBytes...)
		}
	}

	return buf, nil
}

// DecodeRequest 解码 Spotlight 请求
func DecodeRequest(data []byte) (*SpotlightMessage, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("数据太短")
	}

	msg := &SpotlightMessage{
		Command: SpotlightCommand(binary.BigEndian.Uint32(data[0:4])),
		Flags:   binary.BigEndian.Uint32(data[4:8]),
		QueryID: binary.BigEndian.Uint64(data[8:16]),
	}

	return msg, nil
}
