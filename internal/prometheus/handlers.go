// Package prometheus 提供 NAS-OS Prometheus 指标 HTTP 处理器.
package prometheus

import (
	"net/http"

	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Handler Prometheus HTTP 处理器.
type Handler struct {
	registry *promclient.Registry
}

// NewHandler 创建 Prometheus HTTP 处理器.
// 注册 Exporter 到独立 Registry，避免与默认全局注册表冲突。
func NewHandler(provider MetricsProvider) *Handler {
	registry := promclient.NewRegistry()
	exporter := NewExporter(provider)
	registry.MustRegister(exporter)

	return &Handler{
		registry: registry,
	}
}

// MetricsHandler 返回 /metrics 端点的 http.Handler.
func (h *Handler) MetricsHandler() http.Handler {
	return promhttp.HandlerFor(h.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// ServeHTTP 实现 http.Handler 接口，可直接用于 http.Handle 或 gin.WrapH.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.MetricsHandler().ServeHTTP(w, r)
}

// Registry 返回底层 prometheus Registry.
func (h *Handler) Registry() *promclient.Registry {
	return h.registry
}
