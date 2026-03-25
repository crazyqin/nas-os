// Package monitor 提供构建和部署指标
// build_metrics.go - CI/CD 构建指标端点
//
// v2.89.0 工部创建
//
// 功能:
//   - 构建时间追踪
//   - 构建成功率统计
//   - 缓存命中率监控
//   - 部署健康检查

package monitor

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// BuildMetrics CI/CD 构建指标.
type BuildMetrics struct {
	// 构建时间指标
	BuildDuration *prometheus.HistogramVec
	BuildTotal    *prometheus.CounterVec
	BuildSuccess  *prometheus.CounterVec
	BuildFailed   *prometheus.CounterVec

	// 缓存指标
	CacheHits    *prometheus.CounterVec
	CacheMisses  *prometheus.CounterVec
	CacheHitRate prometheus.Gauge

	// 部署指标
	DeployDuration    *prometheus.HistogramVec
	DeployTotal       *prometheus.CounterVec
	DeploySuccess     *prometheus.CounterVec
	DeployFailed      *prometheus.CounterVec
	DeployRollback    *prometheus.CounterVec
	DeployHealthScore prometheus.Gauge

	// 测试指标
	TestDuration *prometheus.HistogramVec
	TestTotal    *prometheus.CounterVec
	TestPassed   *prometheus.CounterVec
	TestFailed   *prometheus.CounterVec
	TestCoverage *prometheus.GaugeVec
	TestSkipped  *prometheus.CounterVec

	// 安全扫描指标
	SecurityScanDuration    *prometheus.HistogramVec
	SecurityScanTotal       *prometheus.CounterVec
	SecurityVulnerabilities *prometheus.GaugeVec
	SecurityCriticalCount   prometheus.Gauge
	SecurityHighCount       prometheus.Gauge

	// 镜像构建指标
	ImageBuildDuration *prometheus.HistogramVec
	ImageBuildTotal    *prometheus.CounterVec
	ImageSize          *prometheus.GaugeVec
	ImageLayers        *prometheus.GaugeVec

	// 版本信息
	VersionInfo *prometheus.GaugeVec
	BuildInfo   *prometheus.GaugeVec

	// 统计数据
	stats *BuildStats
}

// BuildStats 构建统计.
type BuildStats struct {
	TotalBuilds      int64
	SuccessfulBuilds int64
	FailedBuilds     int64
	AvgBuildTime     float64
	LastBuildTime    time.Time
	LastBuildStatus  string
	CacheHitCount    int64
	CacheMissCount   int64
}

// NewBuildMetrics 创建构建指标.
func NewBuildMetrics(namespace string) *BuildMetrics {
	if namespace == "" {
		namespace = "nas_os"
	}

	return &BuildMetrics{
		BuildDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "build_duration_seconds",
			Help:      "Build duration in seconds",
			Buckets:   []float64{30, 60, 120, 180, 300, 600, 900, 1200, 1800},
		}, []string{"job", "platform", "branch"}),

		BuildTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "build_total",
			Help:      "Total number of builds",
		}, []string{"job", "platform", "branch"}),

		BuildSuccess: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "build_success_total",
			Help:      "Total number of successful builds",
		}, []string{"job", "platform", "branch"}),

		BuildFailed: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "build_failed_total",
			Help:      "Total number of failed builds",
		}, []string{"job", "platform", "branch", "reason"}),

		CacheHits: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "build_cache_hits_total",
			Help:      "Total number of cache hits",
		}, []string{"cache_type", "job"}),

		CacheMisses: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "build_cache_misses_total",
			Help:      "Total number of cache misses",
		}, []string{"cache_type", "job"}),

		CacheHitRate: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "build_cache_hit_rate",
			Help:      "Cache hit rate (0-1)",
		}),

		DeployDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "deploy_duration_seconds",
			Help:      "Deployment duration in seconds",
			Buckets:   []float64{10, 30, 60, 120, 180, 300, 600},
		}, []string{"environment", "version"}),

		DeployTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "deploy_total",
			Help:      "Total number of deployments",
		}, []string{"environment", "version"}),

		DeploySuccess: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "deploy_success_total",
			Help:      "Total number of successful deployments",
		}, []string{"environment", "version"}),

		DeployFailed: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "deploy_failed_total",
			Help:      "Total number of failed deployments",
		}, []string{"environment", "version", "reason"}),

		DeployRollback: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "deploy_rollback_total",
			Help:      "Total number of deployment rollbacks",
		}, []string{"environment", "from_version", "to_version"}),

		DeployHealthScore: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "deploy_health_score",
			Help:      "Deployment health score (0-100)",
		}),

		TestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "test_duration_seconds",
			Help:      "Test duration in seconds",
			Buckets:   []float64{30, 60, 120, 300, 600, 900, 1200, 1800},
		}, []string{"type", "suite"}),

		TestTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "test_total",
			Help:      "Total number of test runs",
		}, []string{"type", "suite"}),

		TestPassed: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "test_passed_total",
			Help:      "Total number of passed tests",
		}, []string{"type", "suite"}),

		TestFailed: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "test_failed_total",
			Help:      "Total number of failed tests",
		}, []string{"type", "suite"}),

		TestCoverage: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "test_coverage_percent",
			Help:      "Test coverage percentage",
		}, []string{"type", "package"}),

		TestSkipped: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "test_skipped_total",
			Help:      "Total number of skipped tests",
		}, []string{"type", "suite"}),

		SecurityScanDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "security_scan_duration_seconds",
			Help:      "Security scan duration in seconds",
			Buckets:   []float64{30, 60, 120, 300, 600},
		}, []string{"scanner", "type"}),

		SecurityScanTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "security_scan_total",
			Help:      "Total number of security scans",
		}, []string{"scanner", "type"}),

		SecurityVulnerabilities: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "security_vulnerabilities",
			Help:      "Number of security vulnerabilities by severity",
		}, []string{"severity", "scanner"}),

		SecurityCriticalCount: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "security_vulnerabilities_critical",
			Help:      "Number of critical security vulnerabilities",
		}),

		SecurityHighCount: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "security_vulnerabilities_high",
			Help:      "Number of high severity security vulnerabilities",
		}),

		ImageBuildDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "image_build_duration_seconds",
			Help:      "Docker image build duration in seconds",
			Buckets:   []float64{60, 120, 180, 300, 600, 900, 1200},
		}, []string{"image", "platform"}),

		ImageBuildTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "image_build_total",
			Help:      "Total number of Docker image builds",
		}, []string{"image", "platform"}),

		ImageSize: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "image_size_bytes",
			Help:      "Docker image size in bytes",
		}, []string{"image", "tag"}),

		ImageLayers: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "image_layers_count",
			Help:      "Number of Docker image layers",
		}, []string{"image", "tag"}),

		VersionInfo: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "version_info",
			Help:      "Version information",
		}, []string{"version", "commit", "branch"}),

		BuildInfo: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "build_info",
			Help:      "Build information",
		}, []string{"go_version", "platform", "built_at"}),

		stats: &BuildStats{},
	}
}

// RecordBuild 记录构建.
func (bm *BuildMetrics) RecordBuild(job, platform, branch string, duration float64, success bool) {
	bm.BuildDuration.WithLabelValues(job, platform, branch).Observe(duration)
	bm.BuildTotal.WithLabelValues(job, platform, branch).Inc()

	if success {
		bm.BuildSuccess.WithLabelValues(job, platform, branch).Inc()
		bm.stats.SuccessfulBuilds++
	} else {
		bm.BuildFailed.WithLabelValues(job, platform, branch, "unknown").Inc()
		bm.stats.FailedBuilds++
	}

	bm.stats.TotalBuilds++
	bm.stats.LastBuildTime = time.Now()
	bm.stats.LastBuildStatus = "success"
	if !success {
		bm.stats.LastBuildStatus = "failed"
	}
}

// RecordCacheHit 记录缓存命中.
func (bm *BuildMetrics) RecordCacheHit(cacheType, job string) {
	bm.CacheHits.WithLabelValues(cacheType, job).Inc()
	bm.stats.CacheHitCount++
	bm.updateCacheHitRate()
}

// RecordCacheMiss 记录缓存未命中.
func (bm *BuildMetrics) RecordCacheMiss(cacheType, job string) {
	bm.CacheMisses.WithLabelValues(cacheType, job).Inc()
	bm.stats.CacheMissCount++
	bm.updateCacheHitRate()
}

func (bm *BuildMetrics) updateCacheHitRate() {
	total := bm.stats.CacheHitCount + bm.stats.CacheMissCount
	if total > 0 {
		rate := float64(bm.stats.CacheHitCount) / float64(total)
		bm.CacheHitRate.Set(rate)
	}
}

// RecordDeploy 记录部署.
func (bm *BuildMetrics) RecordDeploy(environment, version string, duration float64, success bool) {
	bm.DeployDuration.WithLabelValues(environment, version).Observe(duration)
	bm.DeployTotal.WithLabelValues(environment, version).Inc()

	if success {
		bm.DeploySuccess.WithLabelValues(environment, version).Inc()
	} else {
		bm.DeployFailed.WithLabelValues(environment, version, "unknown").Inc()
	}
}

// RecordRollback 记录回滚.
func (bm *BuildMetrics) RecordRollback(environment, fromVersion, toVersion string) {
	bm.DeployRollback.WithLabelValues(environment, fromVersion, toVersion).Inc()
}

// RecordTest 记录测试.
func (bm *BuildMetrics) RecordTest(testType, suite string, duration float64, passed, failed, skipped int, coverage float64) {
	bm.TestDuration.WithLabelValues(testType, suite).Observe(duration)
	bm.TestTotal.WithLabelValues(testType, suite).Inc()
	bm.TestPassed.WithLabelValues(testType, suite).Add(float64(passed))
	bm.TestFailed.WithLabelValues(testType, suite).Add(float64(failed))
	bm.TestSkipped.WithLabelValues(testType, suite).Add(float64(skipped))
	if coverage > 0 {
		bm.TestCoverage.WithLabelValues(testType, suite).Set(coverage)
	}
}

// RecordSecurityScan 记录安全扫描.
func (bm *BuildMetrics) RecordSecurityScan(scanner, scanType string, duration float64, critical, high, medium, low int) {
	bm.SecurityScanDuration.WithLabelValues(scanner, scanType).Observe(duration)
	bm.SecurityScanTotal.WithLabelValues(scanner, scanType).Inc()
	bm.SecurityVulnerabilities.WithLabelValues("critical", scanner).Set(float64(critical))
	bm.SecurityVulnerabilities.WithLabelValues("high", scanner).Set(float64(high))
	bm.SecurityVulnerabilities.WithLabelValues("medium", scanner).Set(float64(medium))
	bm.SecurityVulnerabilities.WithLabelValues("low", scanner).Set(float64(low))
	bm.SecurityCriticalCount.Set(float64(critical))
	bm.SecurityHighCount.Set(float64(high))
}

// RecordImageBuild 记录镜像构建.
func (bm *BuildMetrics) RecordImageBuild(image, platform string, duration float64, size int64, layers int) {
	bm.ImageBuildDuration.WithLabelValues(image, platform).Observe(duration)
	bm.ImageBuildTotal.WithLabelValues(image, platform).Inc()
	bm.ImageSize.WithLabelValues(image, "latest").Set(float64(size))
	bm.ImageLayers.WithLabelValues(image, "latest").Set(float64(layers))
}

// SetVersionInfo 设置版本信息.
func (bm *BuildMetrics) SetVersionInfo(version, commit, branch string) {
	bm.VersionInfo.WithLabelValues(version, commit, branch).Set(1)
}

// SetBuildInfo 设置构建信息.
func (bm *BuildMetrics) SetBuildInfo(goVersion, platform, builtAt string) {
	bm.BuildInfo.WithLabelValues(goVersion, platform, builtAt).Set(1)
}

// SetDeployHealthScore 设置部署健康评分.
func (bm *BuildMetrics) SetDeployHealthScore(score float64) {
	bm.DeployHealthScore.Set(score)
}

// GetStats 获取构建统计.
func (bm *BuildMetrics) GetStats() BuildStats {
	return *bm.stats
}

// CalculateBuildSuccessRate 计算构建成功率.
func (bm *BuildMetrics) CalculateBuildSuccessRate() float64 {
	if bm.stats.TotalBuilds == 0 {
		return 0
	}
	return float64(bm.stats.SuccessfulBuilds) / float64(bm.stats.TotalBuilds) * 100
}
