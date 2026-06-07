package containerscanpro

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// ContainerScanPro 高级容器安全扫描系统
type ContainerScanPro struct {
	mu      sync.RWMutex
	config  *Config
	scanner *Scanner
	cveDB   *CVEDatabase
	runtime *RuntimeScanner
	alerter *Alerter
	stats   *ScanStats
	stopCh  chan struct{}
	running bool
}

// Config 总配置
type Config struct {
	Scan     ScanConfig  `json:"scan"`
	Alert    AlertConfig `json:"alert"`
	DataDir  string      `json:"data_dir"`
	HTTPAddr string      `json:"http_addr"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Scan: ScanConfig{
			EnableCVEScan:        true,
			EnableRuntimeMonitor: true,
			ScanInterval:         1 * time.Hour,
			AlertThreshold:       SeverityMedium,
			MaxConcurrent:        5,
			Timeout:              30 * time.Minute,
		},
		Alert: AlertConfig{
			Enabled:     true,
			MinLevel:    AlertLevelWarning,
			CooldownSec: 300,
		},
		DataDir:  "/var/lib/containerscanpro",
		HTTPAddr: ":8090",
	}
}

// New 创建 ContainerScanPro 实例
func New(config *Config) (*ContainerScanPro, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// 初始化 CVE 数据库
	cveDB, err := NewCVEDatabase(config.DataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to init CVE database: %w", err)
	}

	// 初始化告警器
	alerter := NewAlerter(&config.Alert)

	// 初始化运行时监控
	runtime := NewRuntimeScanner(&config.Scan)

	// 初始化扫描器
	scanner := NewScanner(&config.Scan, cveDB, runtime, alerter)

	return &ContainerScanPro{
		config:  config,
		scanner: scanner,
		cveDB:   cveDB,
		runtime: runtime,
		alerter: alerter,
		stats:   &ScanStats{},
		stopCh:  make(chan struct{}),
	}, nil
}

// Start 启动系统
func (csp *ContainerScanPro) Start(ctx context.Context) error {
	csp.mu.Lock()
	if csp.running {
		csp.mu.Unlock()
		return fmt.Errorf("system already running")
	}
	csp.running = true
	csp.mu.Unlock()

	log.Println("[ContainerScanPro] Starting system...")

	// 启动各组件
	if err := csp.runtime.Start(ctx); err != nil {
		return fmt.Errorf("failed to start runtime monitor: %w", err)
	}

	if err := csp.alerter.Start(ctx); err != nil {
		return fmt.Errorf("failed to start alerter: %w", err)
	}

	if err := csp.scanner.Start(ctx); err != nil {
		return fmt.Errorf("failed to start scanner: %w", err)
	}

	// 启动 HTTP API
	go csp.startHTTPServer(ctx)

	log.Println("[ContainerScanPro] System started successfully")
	return nil
}

// Stop 停止系统
func (csp *ContainerScanPro) Stop() {
	csp.mu.Lock()
	defer csp.mu.Unlock()

	if !csp.running {
		return
	}

	log.Println("[ContainerScanPro] Stopping system...")
	close(csp.stopCh)

	csp.scanner.Stop()
	csp.runtime.Stop()
	csp.alerter.Stop()

	csp.running = false
	log.Println("[ContainerScanPro] System stopped")
}

// ScanContainer 扫描单个容器
func (csp *ContainerScanPro) ScanContainer(ctx context.Context, containerID string) (*ScanResult, error) {
	csp.mu.RLock()
	if !csp.running {
		csp.mu.RUnlock()
		return nil, fmt.Errorf("system not running")
	}
	csp.mu.RUnlock()

	return csp.scanner.ScanContainer(ctx, containerID), nil
}

// ScanAllContainers 扫描所有容器
func (csp *ContainerScanPro) ScanAllContainers(ctx context.Context) ([]*ScanResult, error) {
	csp.mu.RLock()
	if !csp.running {
		csp.mu.RUnlock()
		return nil, fmt.Errorf("system not running")
	}
	csp.mu.RUnlock()

	return csp.scanner.ScanAllContainers(ctx)
}

// ScanImage 扫描镜像
func (csp *ContainerScanPro) ScanImage(ctx context.Context, imageName string) (*ScanResult, error) {
	csp.mu.RLock()
	if !csp.running {
		csp.mu.RUnlock()
		return nil, fmt.Errorf("system not running")
	}
	csp.mu.RUnlock()

	return csp.scanner.ScanImage(ctx, imageName), nil
}

// GetScanResult 获取扫描结果
func (csp *ContainerScanPro) GetScanResult(scanID string) (*ScanResult, bool) {
	return csp.scanner.GetResult(scanID)
}

// GetContainerResults 获取容器历史扫描结果
func (csp *ContainerScanPro) GetContainerResults(containerID string) []*ScanResult {
	return csp.scanner.GetResultsByContainer(containerID)
}

// GetStats 获取统计信息
func (csp *ContainerScanPro) GetStats() ScanStats {
	return csp.scanner.GetStats()
}

// GetAlerts 获取告警列表
func (csp *ContainerScanPro) GetAlerts() []Alert {
	return csp.alerter.GetAlerts()
}

// GetRuntimeAnomalies 获取运行时异常
func (csp *ContainerScanPro) GetRuntimeAnomalies() []RuntimeAnomaly {
	return csp.runtime.GetAnomalies()
}

// SearchCVE 搜索 CVE
func (csp *ContainerScanPro) SearchCVE(keyword string) []*CVEInfo {
	return csp.cveDB.SearchByKeyword(keyword)
}

// GetCVEDBStats 获取 CVE 数据库统计
func (csp *ContainerScanPro) GetCVEDBStats() map[string]int {
	return csp.cveDB.GetStats()
}

// startHTTPServer 启动 HTTP API 服务
func (csp *ContainerScanPro) startHTTPServer(ctx context.Context) {
	mux := http.NewServeMux()

	// 健康检查
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "healthy",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	// 扫描所有容器
	mux.HandleFunc("/api/v1/scan", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		results, err := csp.ScanAllContainers(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	})

	// 扫描单个容器
	mux.HandleFunc("/api/v1/scan/container/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		containerID := r.URL.Path[len("/api/v1/scan/container/"):]
		if containerID == "" {
			http.Error(w, "Container ID required", http.StatusBadRequest)
			return
		}

		result, err := csp.ScanContainer(r.Context(), containerID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	// 扫描镜像
	mux.HandleFunc("/api/v1/scan/image/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		imageName := r.URL.Path[len("/api/v1/scan/image/"):]
		if imageName == "" {
			http.Error(w, "Image name required", http.StatusBadRequest)
			return
		}

		result, err := csp.ScanImage(r.Context(), imageName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	// 获取扫描结果
	mux.HandleFunc("/api/v1/result/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		scanID := r.URL.Path[len("/api/v1/result/"):]
		if scanID == "" {
			http.Error(w, "Scan ID required", http.StatusBadRequest)
			return
		}

		result, ok := csp.GetScanResult(scanID)
		if !ok {
			http.Error(w, "Result not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	// 获取统计信息
	mux.HandleFunc("/api/v1/stats", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		stats := csp.GetStats()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	})

	// 获取告警
	mux.HandleFunc("/api/v1/alerts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		alerts := csp.GetAlerts()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(alerts)
	})

	// 获取运行时异常
	mux.HandleFunc("/api/v1/anomalies", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		anomalies := csp.GetRuntimeAnomalies()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(anomalies)
	})

	// 搜索 CVE
	mux.HandleFunc("/api/v1/cve/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		keyword := r.URL.Query().Get("q")
		if keyword == "" {
			http.Error(w, "Query parameter 'q' required", http.StatusBadRequest)
			return
		}

		results := csp.SearchCVE(keyword)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	})

	// CVE 数据库统计
	mux.HandleFunc("/api/v1/cve/stats", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		stats := csp.GetCVEDBStats()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	})

	server := &http.Server{
		Addr:    csp.config.HTTPAddr,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()

	log.Printf("[ContainerScanPro] HTTP API listening on %s", csp.config.HTTPAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("[ContainerScanPro] HTTP server error: %v", err)
	}
}

// Version 返回版本信息
func Version() string {
	return "1.0.0"
}
