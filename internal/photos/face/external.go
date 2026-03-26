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
	Image   string `json:"image"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	MinSize int    `json:"minSize"`
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
	Image  string        `json:"image"`
	Face   ExternalFace  `json:"face"`
	Width  int           `json:"width"`
	Height int           `json:"height"`
}

// ExtractEmbeddingResponse 嵌入向量提取响应
type ExtractEmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
	Error     string    `json:"error,omitempty"`
}

// DetectFaces 调用外部服务检测人脸
func (c *ExternalClient) DetectFaces(ctx context.Context, rgba []byte, width, height int) ([]Face, error) {
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

// encodeBase64 Base64编码
func encodeBase64(data []byte) string {
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