// Package face 外部AI服务客户端
package face

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ==================== 外部服务客户端 ====================

// ExternalClient 外部AI服务客户端
type ExternalClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewExternalClient 创建外部服务客户端
func NewExternalClient(baseURL, apiKey string) *ExternalClient {
	return &ExternalClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// DetectFacesRequest 人脸检测请求
type DetectFacesRequest struct {
	Image   string `json:"image"`   // Base64编码图像
	Width   int    `json:"width"`   // 图像宽度
	Height  int    `json:"height"`  // 图像高度
	MinSize int    `json:"minSize"` // 最小人脸尺寸
}

// DetectFacesResponse 人脸检测响应
type DetectFacesResponse struct {
	Faces []ExternalFace `json:"faces"`
	Error string         `json:"error,omitempty"`
}

// ExternalFace 外部服务人脸结果
type ExternalFace struct {
	BoundingBox BoundingBox `json:"boundingBox"`
	Confidence  float64     `json:"confidence"`
	Landmarks   []Landmark  `json:"landmarks,omitempty"`
}

// ExtractEmbeddingRequest 嵌入向量提取请求
type ExtractEmbeddingRequest struct {
	Image       string      `json:"image"`
	Face        ExternalFace `json:"face"`
	Width       int         `json:"width"`
	Height      int         `json:"height"`
}

// ExtractEmbeddingResponse 嵌入向量提取响应
type ExtractEmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
	Error     string    `json:"error,omitempty"`
}

// DetectFaces 调用外部服务检测人脸
func (c *ExternalClient) DetectFaces(ctx context.Context, rgba []byte, width, height int) ([]Face, error) {
	// Base64编码
	imageBase64 := encodeBase64(rgba)

	req := DetectFacesRequest{
		Image:   imageBase64,
		Width:   width,
		Height:  height,
		MinSize: 30,
	}

	resp, err := c.post(ctx, "/api/v1/face/detect", req)
	if err != nil {
		return nil, fmt.Errorf("检测请求失败: %w", err)
	}

	var result DetectFacesResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if result.Error != "" {
		return nil, fmt.Errorf("检测失败: %s", result.Error)
	}

	// 转换结果
	faces := make([]Face, len(result.Faces))
	for i, ef := range result.Faces {
		faces[i] = Face{
			ID:          generateID("face"),
			BoundingBox: ef.BoundingBox,
			Confidence:  ef.Confidence,
			Quality:     ef.Confidence,
			Landmarks:   ef.Landmarks,
			CreatedAt:   time.Now(),
		}
	}

	return faces, nil
}

// ExtractFaceEmbedding 调用外部服务提取嵌入向量
func (c *ExternalClient) ExtractFaceEmbedding(ctx context.Context, rgba []byte, width, height int, face *Face) ([]float32, error) {
	imageBase64 := encodeBase64(rgba)

	req := ExtractEmbeddingRequest{
		Image: imageBase64,
		Face: ExternalFace{
			BoundingBox: face.BoundingBox,
			Confidence:  face.Confidence,
			Landmarks:   face.Landmarks,
		},
		Width:  width,
		Height: height,
	}

	resp, err := c.post(ctx, "/api/v1/face/embedding", req)
	if err != nil {
		return nil, fmt.Errorf("嵌入向量请求失败: %w", err)
	}

	var result ExtractEmbeddingResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if result.Error != "" {
		return nil, fmt.Errorf("提取失败: %s", result.Error)
	}

	return result.Embedding, nil
}

// post 发送POST请求
func (c *ExternalClient) post(ctx context.Context, path string, body interface{}) ([]byte, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("请求失败: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	return io.ReadAll(resp.Body)
}

// ==================== OpenAI兼容API ====================

// OpenAICompatibleClient OpenAI兼容API客户端
type OpenAICompatibleClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewOpenAICompatibleClient 创建OpenAI兼容客户端
func NewOpenAICompatibleClient(baseURL, apiKey string) *OpenAICompatibleClient {
	return &OpenAICompatibleClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// VisionRequest OpenAI Vision请求
type VisionRequest struct {
	Model    string          `json:"model"`
	Messages []VisionMessage `json:"messages"`
	MaxTokens int            `json:"max_tokens"`
}

// VisionMessage Vision消息
type VisionMessage struct {
	Role    string        `json:"role"`
	Content []VisionContent `json:"content"`
}

// VisionContent Vision内容
type VisionContent struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *VisionImage  `json:"image_url,omitempty"`
}

// VisionImage Vision图像
type VisionImage struct {
	URL string `json:"url"`
}

// VisionResponse Vision响应
type VisionResponse struct {
	Choices []VisionChoice `json:"choices"`
	Error   *APIError      `json:"error,omitempty"`
}

// VisionChoice Vision选择
type VisionChoice struct {
	Message VisionMessage `json:"message"`
}

// APIError API错误
type APIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// DetectFacesWithVision 使用Vision API检测人脸
func (c *OpenAICompatibleClient) DetectFacesWithVision(ctx context.Context, imageBase64 string) ([]Face, error) {
	req := VisionRequest{
		Model: "gpt-4-vision-preview",
		Messages: []VisionMessage{
			{
				Role: "user",
				Content: []VisionContent{
					{
						Type: "text",
						Text: "检测这张图片中的人脸。返回JSON格式的人脸位置信息，包括边界框(x, y, width, height，归一化到0-1)和置信度。",
					},
					{
						Type: "image_url",
						ImageURL: &VisionImage{
							URL: "data:image/jpeg;base64," + imageBase64,
						},
					},
				},
			},
		},
		MaxTokens: 1000,
	}

	jsonBody, _ := json.Marshal(req)
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/chat/completions", bytes.NewReader(jsonBody))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result VisionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Error != nil {
		return nil, fmt.Errorf("API错误: %s", result.Error.Message)
	}

	// 解析返回的人脸信息
	// TODO: 解析JSON格式的响应
	_ = result.Choices

	return []Face{}, nil
}

// ==================== Azure Face API ====================

// AzureFaceClient Azure Face API客户端
type AzureFaceClient struct {
	endpoint   string
	apiKey     string
	httpClient *http.Client
}

// NewAzureFaceClient 创建Azure Face客户端
func NewAzureFaceClient(endpoint, apiKey string) *AzureFaceClient {
	return &AzureFaceClient{
		endpoint: endpoint,
		apiKey:   apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// AzureFaceResult Azure Face结果
type AzureFaceResult struct {
	FaceID            string                 `json:"faceId"`
	FaceRectangle     AzureFaceRectangle     `json:"faceRectangle"`
	FaceAttributes    AzureFaceAttributes    `json:"faceAttributes,omitempty"`
	FaceLandmarks     AzureFaceLandmarks     `json:"faceLandmarks,omitempty"`
}

// AzureFaceRectangle Azure人脸矩形
type AzureFaceRectangle struct {
	Top    int `json:"top"`
	Left   int `json:"left"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// AzureFaceAttributes Azure人脸属性
type AzureFaceAttributes struct {
	Age    float64 `json:"age"`
	Gender string  `json:"gender"`
	Smile  float64 `json:"smile"`
}

// AzureFaceLandmarks Azure人脸特征点
type AzureFaceLandmarks struct {
	PupilLeft       AzurePoint `json:"pupilLeft"`
	PupilRight      AzurePoint `json:"pupilRight"`
	NoseTip         AzurePoint `json:"noseTip"`
	MouthLeft       AzurePoint `json:"mouthLeft"`
	MouthRight      AzurePoint `json:"mouthRight"`
}

// AzurePoint Azure坐标点
type AzurePoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// DetectFaces Azure人脸检测
func (c *AzureFaceClient) DetectFaces(ctx context.Context, imageData []byte, width, height int) ([]Face, error) {
	url := fmt.Sprintf("%s/face/v1.0/detect?returnFaceId=true&returnFaceLandmarks=true&returnFaceAttributes=age,gender,smile", c.endpoint)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(imageData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Ocp-Apim-Subscription-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var results []AzureFaceResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}

	// 转换结果
	faces := make([]Face, len(results))
	for i, r := range results {
		faces[i] = Face{
			ID: r.FaceID,
			BoundingBox: BoundingBox{
				X:      float64(r.FaceRectangle.Left) / float64(width),
				Y:      float64(r.FaceRectangle.Top) / float64(height),
				Width:  float64(r.FaceRectangle.Width) / float64(width),
				Height: float64(r.FaceRectangle.Height) / float64(height),
			},
			Confidence: 1.0,
			Quality:    1.0,
			CreatedAt:  time.Now(),
		}

		// 添加特征点
		if r.FaceLandmarks.PupilLeft.X > 0 {
			faces[i].Landmarks = []Landmark{
				{Type: "left_eye", X: r.FaceLandmarks.PupilLeft.X / float64(width), Y: r.FaceLandmarks.PupilLeft.Y / float64(height)},
				{Type: "right_eye", X: r.FaceLandmarks.PupilRight.X / float64(width), Y: r.FaceLandmarks.PupilRight.Y / float64(height)},
				{Type: "nose", X: r.FaceLandmarks.NoseTip.X / float64(width), Y: r.FaceLandmarks.NoseTip.Y / float64(height)},
				{Type: "left_mouth", X: r.FaceLandmarks.MouthLeft.X / float64(width), Y: r.FaceLandmarks.MouthLeft.Y / float64(height)},
				{Type: "right_mouth", X: r.FaceLandmarks.MouthRight.X / float64(width), Y: r.FaceLandmarks.MouthRight.Y / float64(height)},
			}
		}
	}

	return faces, nil
}

// ==================== 工具函数 ====================

func encodeBase64(data []byte) string {
	// 简单的Base64编码
	const base64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

	result := make([]byte, 0, (len(data)+2)/3*4)

	for i := 0; i < len(data); i += 3 {
		var n uint32
		remaining := len(data) - i

		if remaining >= 3 {
			n = uint32(data[i])<<16 | uint32(data[i+1])<<8 | uint32(data[i+2])
			result = append(result, base64Chars[n>>18&0x3F], base64Chars[n>>12&0x3F], base64Chars[n>>6&0x3F], base64Chars[n&0x3F])
		} else if remaining == 2 {
			n = uint32(data[i])<<16 | uint32(data[i+1])<<8
			result = append(result, base64Chars[n>>18&0x3F], base64Chars[n>>12&0x3F], base64Chars[n>>6&0x3F], '=')
		} else {
			n = uint32(data[i]) << 16
			result = append(result, base64Chars[n>>18&0x3F], base64Chars[n>>12&0x3F], '=', '=')
		}
	}

	return string(result)
}