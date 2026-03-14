package monitor

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	nooptrace "go.opentelemetry.io/otel/trace/noop"
)

// TracingConfig 分布式追踪配置
type TracingConfig struct {
	Enabled     bool   `json:"enabled"`
	ServiceName string `json:"serviceName"`
	Endpoint    string `json:"endpoint"`
	Protocol    string `json:"protocol"` // grpc or http
	SampleRate  float64 `json:"sampleRate"`
	Insecure    bool   `json:"insecure"`
}

// DefaultTracingConfig 默认追踪配置
func DefaultTracingConfig() *TracingConfig {
	return &TracingConfig{
		Enabled:     false,
		ServiceName: "nas-os",
		Endpoint:    "localhost:4317",
		Protocol:    "grpc",
		SampleRate:  0.1, // 10% 采样率
		Insecure:    true,
	}
}

// TracingManager 追踪管理器
type TracingManager struct {
	config     *TracingConfig
	tracer     trace.Tracer
	provider   *sdktrace.TracerProvider
	shutdown   func(context.Context) error
	enabled    bool
}

// NewTracingManager 创建追踪管理器
func NewTracingManager(config *TracingConfig) (*TracingManager, error) {
	if config == nil {
		config = DefaultTracingConfig()
	}

	tm := &TracingManager{
		config: config,
	}

	if !config.Enabled {
		tm.enabled = false
		noopProvider := nooptrace.NewTracerProvider()
		tm.tracer = noopProvider.Tracer(config.ServiceName)
		return tm, nil
	}

	// 创建资源
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(config.ServiceName),
			semconv.ServiceVersion("2.29.0"),
			attribute.String("service.instance.id", "nas-os-main"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("创建追踪资源失败: %w", err)
	}

	// 创建导出器
	var exporter sdktrace.SpanExporter
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	switch config.Protocol {
	case "http":
		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(config.Endpoint),
		}
		if config.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		exporter, err = otlptrace.New(ctx, otlptracehttp.NewClient(opts...))
	default: // grpc
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(config.Endpoint),
		}
		if config.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		exporter, err = otlptrace.New(ctx, otlptracegrpc.NewClient(opts...))
	}

	if err != nil {
		return nil, fmt.Errorf("创建追踪导出器失败: %w", err)
	}

	// 创建采样器
	var sampler sdktrace.Sampler
	if config.SampleRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if config.SampleRate <= 0 {
		sampler = sdktrace.NeverSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(config.SampleRate)
	}

	// 创建 TracerProvider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	tm.provider = tp
	tm.tracer = tp.Tracer(config.ServiceName)
	tm.shutdown = tp.Shutdown
	tm.enabled = true

	// 设置全局 TracerProvider
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tm, nil
}

// Tracer 获取追踪器
func (tm *TracingManager) Tracer() trace.Tracer {
	return tm.tracer
}

// Enabled 检查是否启用
func (tm *TracingManager) Enabled() bool {
	return tm.enabled
}

// Shutdown 关闭追踪
func (tm *TracingManager) Shutdown(ctx context.Context) error {
	if tm.shutdown != nil {
		return tm.shutdown(ctx)
	}
	return nil
}

// StartSpan 开始追踪 Span
func (tm *TracingManager) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return tm.tracer.Start(ctx, name, opts...)
}

// ========== 追踪辅助函数 ==========

// TraceOperation 追踪操作
func (tm *TracingManager) TraceOperation(ctx context.Context, operationName string, fn func(ctx context.Context) error) error {
	if !tm.enabled {
		return fn(ctx)
	}

	ctx, span := tm.tracer.Start(ctx, operationName)
	defer span.End()

	err := fn(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error", err.Error()))
	}

	return err
}

// TraceOperationWithResult 追踪带返回值的操作
func TraceOperationWithResult[T any](tm *TracingManager, ctx context.Context, operationName string, fn func(ctx context.Context) (T, error)) (T, error) {
	if !tm.enabled {
		return fn(ctx)
	}

	ctx, span := tm.tracer.Start(ctx, operationName)
	defer span.End()

	result, err := fn(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error", err.Error()))
	}

	return result, err
}

// ========== 预定义的追踪操作 ==========

// TraceBackupOperation 追踪备份操作
func (tm *TracingManager) TraceBackupOperation(ctx context.Context, jobID, jobName string, fn func(ctx context.Context) error) error {
	if !tm.enabled {
		return fn(ctx)
	}

	ctx, span := tm.tracer.Start(ctx, "backup.execute",
		trace.WithAttributes(
			attribute.String("backup.job_id", jobID),
			attribute.String("backup.job_name", jobName),
		),
	)
	defer span.End()

	err := fn(ctx)
	if err != nil {
		span.RecordError(err)
	}

	return err
}

// TraceSnapshotOperation 追踪快照操作
func (tm *TracingManager) TraceSnapshotOperation(ctx context.Context, pool, snapshotID string, fn func(ctx context.Context) error) error {
	if !tm.enabled {
		return fn(ctx)
	}

	ctx, span := tm.tracer.Start(ctx, "snapshot.execute",
		trace.WithAttributes(
			attribute.String("snapshot.pool", pool),
			attribute.String("snapshot.id", snapshotID),
		),
	)
	defer span.End()

	err := fn(ctx)
	if err != nil {
		span.RecordError(err)
	}

	return err
}

// TraceStorageOperation 追踪存储操作
func (tm *TracingManager) TraceStorageOperation(ctx context.Context, operation, pool string, fn func(ctx context.Context) error) error {
	if !tm.enabled {
		return fn(ctx)
	}

	ctx, span := tm.tracer.Start(ctx, "storage."+operation,
		trace.WithAttributes(
			attribute.String("storage.operation", operation),
			attribute.String("storage.pool", pool),
		),
	)
	defer span.End()

	err := fn(ctx)
	if err != nil {
		span.RecordError(err)
	}

	return err
}

// TraceAPIRequest 追踪 API 请求
func (tm *TracingManager) TraceAPIRequest(ctx context.Context, method, path string, fn func(ctx context.Context) error) error {
	if !tm.enabled {
		return fn(ctx)
	}

	ctx, span := tm.tracer.Start(ctx, "api.request",
		trace.WithAttributes(
			attribute.String("http.method", method),
			attribute.String("http.path", path),
		),
	)
	defer span.End()

	err := fn(ctx)
	if err != nil {
		span.RecordError(err)
	}

	return err
}

// ========== Span 属性辅助 ==========

// SetBackupSpanAttributes 设置备份 Span 属性
func SetBackupSpanAttributes(span trace.Span, jobID, jobName, backupType string, size int64, duration time.Duration) {
	span.SetAttributes(
		attribute.String("backup.job_id", jobID),
		attribute.String("backup.job_name", jobName),
		attribute.String("backup.type", backupType),
		attribute.Int64("backup.size_bytes", size),
		attribute.Int64("backup.duration_ms", duration.Milliseconds()),
	)
}

// SetSnapshotSpanAttributes 设置快照 Span 属性
func SetSnapshotSpanAttributes(span trace.Span, pool, snapshotID string, size int64, exclusive bool) {
	span.SetAttributes(
		attribute.String("snapshot.pool", pool),
		attribute.String("snapshot.id", snapshotID),
		attribute.Int64("snapshot.size_bytes", size),
		attribute.Bool("snapshot.exclusive", exclusive),
	)
}

// SetStorageSpanAttributes 设置存储 Span 属性
func SetStorageSpanAttributes(span trace.Span, pool, operation string, success bool) {
	span.SetAttributes(
		attribute.String("storage.pool", pool),
		attribute.String("storage.operation", operation),
		attribute.Bool("storage.success", success),
	)
}

// ========== 全局追踪管理器 ==========

var globalTracingManager *TracingManager

// InitGlobalTracing 初始化全局追踪
func InitGlobalTracing(config *TracingConfig) error {
	tm, err := NewTracingManager(config)
	if err != nil {
		return err
	}
	globalTracingManager = tm
	return nil
}

// GlobalTracing 获取全局追踪管理器
func GlobalTracing() *TracingManager {
	if globalTracingManager == nil {
		globalTracingManager, _ = NewTracingManager(nil)
	}
	return globalTracingManager
}