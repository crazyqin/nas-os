// Package aifilesum 提供AI智能文件摘要生成功能
package aifilesum

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

// Summarizer AI摘要引擎
type Summarizer struct {
	config     *SummarizerConfig
	httpClient *http.Client
}

// NewSummarizer 创建摘要引擎
func NewSummarizer(config *SummarizerConfig) *Summarizer {
	if config == nil {
		config = &SummarizerConfig{
			AIEndpoint:            "http://localhost:11434",
			AIModel:               "llama3.2",
			MaxConcurrent:         3,
			MaxQueueSize:          100,
			CacheTTL:              3600,
			SupportedLanguages:    []Language{LanguageAuto, LanguageChinese, LanguageEnglish},
			MaxFileSizeMB:         100,
			VideoFrameIntervalSec: 10,
		}
	}

	return &Summarizer{
		config: config,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// aiRequest AI请求结构
type aiRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// aiResponse AI响应结构
type aiResponse struct {
	Response string `json:"response"`
}

// SummarizeDocument 文档摘要生成
func (s *Summarizer) SummarizeDocument(ctx context.Context, filePath string, content string, opts *SummarizeOptions) (*Summary, error) {
	startTime := time.Now()

	// 构建提示词
	prompt := s.buildDocumentPrompt(content, opts)

	// 调用AI生成摘要
	response, err := s.callAI(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("AI调用失败: %w", err)
	}

	// 解析响应
	summary := s.parseDocumentResponse(response, filePath, content, opts)
	summary.Duration = time.Since(startTime).Milliseconds()
	summary.ProcessedAt = time.Now()

	return summary, nil
}

// SummarizeImage 图片描述生成
func (s *Summarizer) SummarizeImage(ctx context.Context, filePath string, opts *SummarizeOptions) (*Summary, error) {
	startTime := time.Now()

	// 构建提示词
	prompt := s.buildImagePrompt(filePath, opts)

	// 调用AI生成描述
	response, err := s.callAI(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("AI调用失败: %w", err)
	}

	// 解析响应
	summary := s.parseImageResponse(response, filePath, opts)
	summary.Duration = time.Since(startTime).Milliseconds()
	summary.ProcessedAt = time.Now()

	return summary, nil
}

// SummarizeVideo 视频摘要生成
func (s *Summarizer) SummarizeVideo(ctx context.Context, filePath string, opts *SummarizeOptions) (*Summary, error) {
	startTime := time.Now()

	// 构建提示词
	prompt := s.buildVideoPrompt(filePath, opts)

	// 调用AI生成摘要
	response, err := s.callAI(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("AI调用失败: %w", err)
	}

	// 解析响应
	summary := s.parseVideoResponse(response, filePath, opts)
	summary.Duration = time.Since(startTime).Milliseconds()
	summary.ProcessedAt = time.Now()

	return summary, nil
}

// buildDocumentPrompt 构建文档摘要提示词
func (s *Summarizer) buildDocumentPrompt(content string, opts *SummarizeOptions) string {
	langPrompt := s.getLanguagePrompt(opts.Language)
	maxLen := opts.MaxSummaryLength
	if maxLen <= 0 {
		maxLen = 200
	}

	prompt := fmt.Sprintf(`请对以下文档内容进行摘要。要求：
1. 生成一个简洁的标题
2. 生成一段%d字以内的摘要
3. 提取5个关键词
4. 生成3-5个标签
%s

文档内容：
%s

请按以下JSON格式返回：
{
  "title": "标题",
  "summary": "摘要内容",
  "keywords": ["关键词1", "关键词2"],
  "tags": ["标签1", "标签2"],
  "language": "检测到的语言"
}`, maxLen, langPrompt, content)

	return prompt
}

// buildImagePrompt 构建图片描述提示词
func (s *Summarizer) buildImagePrompt(filePath string, opts *SummarizeOptions) string {
	langPrompt := s.getLanguagePrompt(opts.Language)

	prompt := fmt.Sprintf(`请对图片进行详细描述。文件路径: %s
要求：
1. 详细描述图片内容
2. 生成3-5个标签
3. 提取图片中的文字（如果有的话）
%s

请按以下JSON格式返回：
{
  "description": "详细描述",
  "tags": ["标签1", "标签2"],
  "text_content": "图片中的文字（如果有）",
  "language": "检测到的语言"
}`, filePath, langPrompt)

	return prompt
}

// buildVideoPrompt 构建视频摘要提示词
func (s *Summarizer) buildVideoPrompt(filePath string, opts *SummarizeOptions) string {
	langPrompt := s.getLanguagePrompt(opts.Language)

	prompt := fmt.Sprintf(`请对视频内容进行摘要。文件路径: %s
要求：
1. 生成视频内容的总体描述
2. 生成3-5个标签
3. 描述关键场景
%s

请按以下JSON格式返回：
{
  "summary": "视频内容摘要",
  "tags": ["标签1", "标签2"],
  "key_scenes": [
    {"timestamp": 0, "description": "场景描述"}
  ],
  "language": "检测到的语言"
}`, filePath, langPrompt)

	return prompt
}

// getLanguagePrompt 获取语言提示
func (s *Summarizer) getLanguagePrompt(lang Language) string {
	switch lang {
	case LanguageChinese:
		return "请使用中文回复。"
	case LanguageEnglish:
		return "Please respond in English."
	case LanguageJapanese:
		return "日本語で回答してください。"
	case LanguageKorean:
		return "한국어로 답변해 주세요."
	default:
		return "请使用与原文相同的语言回复。"
	}
}

// callAI 调用AI服务
func (s *Summarizer) callAI(ctx context.Context, prompt string) (string, error) {
	reqBody := aiRequest{
		Model:  s.config.AIModel,
		Prompt: prompt,
		Stream: false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("请求序列化失败: %w", err)
	}

	url := fmt.Sprintf("%s/api/generate", s.config.AIEndpoint)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if s.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.config.APIKey)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求发送失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("AI服务返回错误: %d - %s", resp.StatusCode, string(body))
	}

	var aiResp aiResponse
	if err := json.NewDecoder(resp.Body).Decode(&aiResp); err != nil {
		return "", fmt.Errorf("响应解析失败: %w", err)
	}

	return aiResp.Response, nil
}

// parseDocumentResponse 解析文档摘要响应
func (s *Summarizer) parseDocumentResponse(response string, filePath string, content string, opts *SummarizeOptions) *Summary {
	var parsed struct {
		Title    string   `json:"title"`
		Summary  string   `json:"summary"`
		Keywords []string `json:"keywords"`
		Tags     []string `json:"tags"`
		Language string   `json:"language"`
	}

	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		log.Printf("⚠️ JSON解析失败，使用原始响应: %v", err)
		parsed.Summary = response
		parsed.Title = filepath.Base(filePath)
	}

	// 计算词数
	wordCount := len(strings.Fields(content))
	summaryWordCount := len(strings.Fields(parsed.Summary))
	compressionRatio := 0.0
	if wordCount > 0 {
		compressionRatio = float64(summaryWordCount) / float64(wordCount)
	}

	lang := LanguageAuto
	if parsed.Language != "" {
		lang = Language(parsed.Language)
	}

	summary := &Summary{
		ID:               generateID(),
		FileID:           generateFileID(filePath),
		FileInfo:         getFileInfo(filePath),
		ContentType:      "document",
		SummaryText:      parsed.Summary,
		Title:            parsed.Title,
		Language:         lang,
		WordCount:        wordCount,
		SummaryWordCount: summaryWordCount,
		CompressionRatio: compressionRatio,
		CreatedAt:        time.Now(),
	}

	if opts.ExtractKeywords {
		summary.Keywords = parsed.Keywords
	}
	if opts.ExtractTags {
		summary.Tags = parsed.Tags
	}

	return summary
}

// parseImageResponse 解析图片描述响应
func (s *Summarizer) parseImageResponse(response string, filePath string, opts *SummarizeOptions) *Summary {
	var parsed struct {
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
		TextContent string   `json:"text_content"`
		Language    string   `json:"language"`
	}

	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		log.Printf("⚠️ JSON解析失败，使用原始响应: %v", err)
		parsed.Description = response
	}

	lang := LanguageAuto
	if parsed.Language != "" {
		lang = Language(parsed.Language)
	}

	summary := &Summary{
		ID:               generateID(),
		FileID:           generateFileID(filePath),
		FileInfo:         getFileInfo(filePath),
		ContentType:      "image",
		SummaryText:      parsed.Description,
		ImageDescription: parsed.Description,
		Language:         lang,
		CreatedAt:        time.Now(),
	}

	if opts.ExtractTags {
		summary.Tags = parsed.Tags
	}

	return summary
}

// parseVideoResponse 解析视频摘要响应
func (s *Summarizer) parseVideoResponse(response string, filePath string, opts *SummarizeOptions) *Summary {
	var parsed struct {
		Summary   string   `json:"summary"`
		Tags      []string `json:"tags"`
		KeyScenes []struct {
			Timestamp   float64 `json:"timestamp"`
			Description string  `json:"description"`
		} `json:"key_scenes"`
		Language string `json:"language"`
	}

	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		log.Printf("⚠️ JSON解析失败，使用原始响应: %v", err)
		parsed.Summary = response
	}

	lang := LanguageAuto
	if parsed.Language != "" {
		lang = Language(parsed.Language)
	}

	// 转换关键帧
	keyFrames := make([]VideoKeyFrame, len(parsed.KeyScenes))
	for i, scene := range parsed.KeyScenes {
		keyFrames[i] = VideoKeyFrame{
			Timestamp:   scene.Timestamp,
			Description: scene.Description,
		}
	}

	summary := &Summary{
		ID:             generateID(),
		FileID:         generateFileID(filePath),
		FileInfo:       getFileInfo(filePath),
		ContentType:    "video",
		SummaryText:    parsed.Summary,
		VideoKeyFrames: keyFrames,
		Language:       lang,
		CreatedAt:      time.Now(),
	}

	if opts.ExtractTags {
		summary.Tags = parsed.Tags
	}

	return summary
}

// generateID 生成唯一ID
func generateID() string {
	return fmt.Sprintf("sum_%d", time.Now().UnixNano())
}

// generateFileID 生成文件ID
func generateFileID(filePath string) string {
	return fmt.Sprintf("file_%x", filePath)
}

// getFileInfo 获取文件信息
func getFileInfo(filePath string) *FileInfo {
	ext := strings.ToLower(filepath.Ext(filePath))
	name := filepath.Base(filePath)

	return &FileInfo{
		Path:      filePath,
		Name:      name,
		Extension: ext,
		FileType:  classifyFileType(ext),
		MimeType:  getMimeType(ext),
	}
}

// classifyFileType 根据扩展名分类文件类型
func classifyFileType(ext string) FileType {
	docExts := map[string]bool{
		".pdf": true, ".doc": true, ".docx": true, ".txt": true,
		".md": true, ".rtf": true, ".odt": true, ".html": true,
	}

	imgExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
		".bmp": true, ".webp": true, ".svg": true, ".tiff": true,
	}

	vidExts := map[string]bool{
		".mp4": true, ".mkv": true, ".avi": true, ".mov": true,
		".wmv": true, ".flv": true, ".webm": true, ".m4v": true,
	}

	if docExts[ext] {
		return FileTypeDocument
	}
	if imgExts[ext] {
		return FileTypeImage
	}
	if vidExts[ext] {
		return FileTypeVideo
	}
	return FileTypeUnknown
}

// getMimeType 获取MIME类型
func getMimeType(ext string) string {
	mimeMap := map[string]string{
		".pdf":  "application/pdf",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".txt":  "text/plain",
		".md":   "text/markdown",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".mp4":  "video/mp4",
		".mkv":  "video/x-matroska",
		".avi":  "video/x-msvideo",
	}

	if mime, ok := mimeMap[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}

// IsSupported 检查文件是否支持
func (s *Summarizer) IsSupported(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	ft := classifyFileType(ext)
	return ft != FileTypeUnknown
}

// GetSupportedExtensions 获取支持的扩展名列表
func (s *Summarizer) GetSupportedExtensions() []string {
	return []string{
		".pdf", ".doc", ".docx", ".txt", ".md", ".rtf", ".odt", ".html",
		".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".svg",
		".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm",
	}
}
