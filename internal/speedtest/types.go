// Package speedtest 提供网络测速功能
package speedtest

import (
	"time"
)

// TestResult 测速结果.
type TestResult struct {
	ID           string    `json:"id"`
	ServerName   string    `json:"server_name"`
	ServerURL    string    `json:"server_url"`
	DownloadSpeed float64  `json:"download_speed"` // Mbps
	UploadSpeed   float64  `json:"upload_speed"`   // Mbps
	Latency       float64  `json:"latency"`         // ms
	Jitter        float64  `json:"jitter"`          // ms
	PacketLoss    float64  `json:"packet_loss"`     // percentage
	Timestamp     time.Time `json:"timestamp"`
}

// TestServer 测速服务器.
type TestServer struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	URL      string  `json:"url"`
	Location string  `json:"location"`
	Distance float64 `json:"distance"` // km
}

// SpeedStats 统计信息.
type SpeedStats struct {
	AvgDownload  float64   `json:"avg_download"` // Mbps
	AvgUpload    float64   `json:"avg_upload"`   // Mbps
	AvgLatency   float64   `json:"avg_latency"`  // ms
	TestCount    int       `json:"test_count"`
	LastTestTime time.Time `json:"last_test_time"`
}

// RunTestRequest 运行测速请求.
type RunTestRequest struct {
	ServerID string `json:"server_id,omitempty"`
}

// AddServerRequest 添加服务器请求.
type AddServerRequest struct {
	Name     string  `json:"name" binding:"required"`
	URL      string  `json:"url" binding:"required"`
	Location string  `json:"location"`
	Distance float64 `json:"distance"`
}
