package cloudsyncmgr

import (
	"context"
	"io"
	"sync"
	"time"
)

// BandwidthLimiter 带宽限制器 (令牌桶算法).
type BandwidthLimiter struct {
	mu         sync.Mutex
	rate       int64     // 字节/秒
	tokens     float64   // 当前可用令牌
	maxTokens  float64   // 最大令牌数
	lastRefill time.Time // 上次补充时间
}

// NewBandwidthLimiter 创建带宽限制器.
// rate 为 0 表示不限速.
func NewBandwidthLimiter(rate int64) *BandwidthLimiter {
	if rate <= 0 {
		return &BandwidthLimiter{rate: 0}
	}
	return &BandwidthLimiter{
		rate:       rate,
		tokens:     float64(rate),
		maxTokens:  float64(rate),
		lastRefill: time.Now(),
	}
}

// SetRate 更新限速值 (0 表示不限速).
func (bl *BandwidthLimiter) SetRate(rate int64) {
	bl.mu.Lock()
	defer bl.mu.Unlock()
	bl.rate = rate
	if rate > 0 {
		bl.maxTokens = float64(rate)
		bl.tokens = float64(rate)
		bl.lastRefill = time.Now()
	}
}

// GetRate 获取当前限速值.
func (bl *BandwidthLimiter) GetRate() int64 {
	bl.mu.Lock()
	defer bl.mu.Unlock()
	return bl.rate
}

// Acquire 等待直到获得 n 字节的传输许可.
func (bl *BandwidthLimiter) Acquire(ctx context.Context, n int64) error {
	if bl.rate <= 0 {
		return nil // 不限速
	}

	for {
		bl.mu.Lock()
		bl.refill()

		if bl.tokens >= float64(n) {
			bl.tokens -= float64(n)
			bl.mu.Unlock()
			return nil
		}

		// 计算等待时间
		deficit := float64(n) - bl.tokens
		waitTime := time.Duration(deficit/float64(bl.rate)*1000) * time.Millisecond
		if waitTime < time.Millisecond {
			waitTime = time.Millisecond
		}
		bl.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// 继续重试
		}
	}
}

// refill 补充令牌（调用时需要持有锁）.
func (bl *BandwidthLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(bl.lastRefill).Seconds()
	bl.tokens += elapsed * float64(bl.rate)
	if bl.tokens > bl.maxTokens {
		bl.tokens = bl.maxTokens
	}
	bl.lastRefill = now
}

// LimitedReader 带限速的读取器.
type LimitedReader struct {
	reader  io.Reader
	limiter *BandwidthLimiter
	ctx     context.Context
}

// NewLimitedReader 创建带限速的读取器.
func NewLimitedReader(ctx context.Context, reader io.Reader, limiter *BandwidthLimiter) *LimitedReader {
	return &LimitedReader{
		reader:  reader,
		limiter: limiter,
		ctx:     ctx,
	}
}

// Read 实现 io.Reader 接口，带限速.
func (lr *LimitedReader) Read(p []byte) (int, error) {
	if lr.limiter == nil || lr.limiter.GetRate() <= 0 {
		return lr.reader.Read(p)
	}

	// 限制单次读取大小不超过令牌桶的 1/4
	maxRead := int(lr.limiter.GetRate() / 4)
	if maxRead < 1024 {
		maxRead = 1024
	}
	if len(p) > maxRead {
		p = p[:maxRead]
	}

	// 先获取令牌
	toRead := int64(len(p))
	if err := lr.limiter.Acquire(lr.ctx, toRead); err != nil {
		return 0, err
	}

	return lr.reader.Read(p)
}
