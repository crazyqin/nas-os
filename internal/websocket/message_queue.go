// Package websocket 提供增强的 WebSocket 消息队列功能
// Version: v2.45.0 - 消息优先级队列、背压控制、消息去重
package websocket

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"time"
)

// MessagePriority 消息优先级
type MessagePriority int

const (
	// PriorityLow 低优先级消息，最后处理
	PriorityLow MessagePriority = 0
	// PriorityNormal 普通优先级消息，默认优先级
	PriorityNormal MessagePriority = 1
	// PriorityHigh 高优先级消息，优先处理
	PriorityHigh MessagePriority = 2
	// PriorityCritical 关键优先级消息，最高优先级，永不丢弃
	PriorityCritical MessagePriority = 3
)

// String 返回优先级字符串
func (p MessagePriority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityNormal:
		return "normal"
	case PriorityHigh:
		return "high"
	case PriorityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// Message 消息结构
type Message struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	Priority    MessagePriority `json:"priority"`
	Data        json.RawMessage `json:"data"`
	Timestamp   time.Time       `json:"timestamp"`
	Expiration  time.Time       `json:"expiration,omitempty"`
	Retries     int             `json:"retries"`
	MaxRetries  int             `json:"maxRetries"`
	Hash        string          `json:"hash"` // 去重哈希
	From        string          `json:"from,omitempty"`
	To          string          `json:"to,omitempty"`          // 目标客户端/房间
	Correlation string          `json:"correlation,omitempty"` // 关联 ID
}

// MessageQueueConfig 消息队列配置
type MessageQueueConfig struct {
	// MaxSize 队列最大容量
	MaxSize int `json:"maxSize"`
	// HighPrioritySize 高优先级队列容量
	HighPrioritySize int `json:"highPrioritySize"`
	// NormalPrioritySize 普通优先级队列容量
	NormalPrioritySize int `json:"normalPrioritySize"`
	// LowPrioritySize 低优先级队列容量
	LowPrioritySize int `json:"lowPrioritySize"`
	// EnableDedup 是否启用消息去重
	EnableDedup bool `json:"enableDedup"`
	// DedupTTL 去重缓存过期时间
	DedupTTL time.Duration `json:"dedupTTL"`
	// EnableBackpressure 是否启用背压控制
	EnableBackpressure bool `json:"enableBackpressure"`
	// BackpressureThreshold 背压触发阈值（队列使用率）
	BackpressureThreshold float64 `json:"backpressureThreshold"`
	// BackpressureStrategy 背压策略: drop_oldest, drop_low_priority, reject
	BackpressureStrategy string `json:"backpressureStrategy"`
	// DefaultTTL 消息默认过期时间
	DefaultTTL time.Duration `json:"defaultTTL"`
	// MaxRetries 最大重试次数
	MaxRetries int `json:"maxRetries"`
	// RetryDelay 重试延迟
	RetryDelay time.Duration `json:"retryDelay"`
}

// DefaultMessageQueueConfig 默认消息队列配置
var DefaultMessageQueueConfig = &MessageQueueConfig{
	MaxSize:               10000,
	HighPrioritySize:      1000,
	NormalPrioritySize:    5000,
	LowPrioritySize:       4000,
	EnableDedup:           true,
	DedupTTL:              5 * time.Minute,
	EnableBackpressure:    true,
	BackpressureThreshold: 0.8,
	BackpressureStrategy:  "drop_low_priority",
	DefaultTTL:            5 * time.Minute,
	MaxRetries:            3,
	RetryDelay:            100 * time.Millisecond,
}

// PriorityQueue 优先级队列
type PriorityQueue struct {
	high     chan *Message
	normal   chan *Message
	low      chan *Message
	critical chan *Message
	config   *MessageQueueConfig
}

// NewPriorityQueue 创建优先级队列
func NewPriorityQueue(config *MessageQueueConfig) *PriorityQueue {
	if config == nil {
		config = DefaultMessageQueueConfig
	}

	return &PriorityQueue{
		high:     make(chan *Message, config.HighPrioritySize),
		normal:   make(chan *Message, config.NormalPrioritySize),
		low:      make(chan *Message, config.LowPrioritySize),
		critical: make(chan *Message, config.HighPrioritySize/2),
		config:   config,
	}
}

// Push 推送消息
func (pq *PriorityQueue) Push(msg *Message) error {
	var targetChan chan *Message

	switch msg.Priority {
	case PriorityCritical:
		targetChan = pq.critical
	case PriorityHigh:
		targetChan = pq.high
	case PriorityLow:
		targetChan = pq.low
	default:
		targetChan = pq.normal
	}

	select {
	case targetChan <- msg:
		return nil
	default:
		return ErrQueueFull
	}
}

// Pop 弹出消息（按优先级）
func (pq *PriorityQueue) Pop() *Message {
	// 按优先级顺序检查
	select {
	case msg := <-pq.critical:
		return msg
	default:
		select {
		case msg := <-pq.high:
			return msg
		default:
			select {
			case msg := <-pq.normal:
				return msg
			default:
				select {
				case msg := <-pq.low:
					return msg
				default:
					return nil
				}
			}
		}
	}
}

// PopWithTimeout 带超时的弹出
func (pq *PriorityQueue) PopWithTimeout(timeout time.Duration) *Message {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	// 先检查高优先级
	select {
	case msg := <-pq.critical:
		return msg
	case msg := <-pq.high:
		return msg
	default:
	}

	// 等待任意优先级的消息
	select {
	case msg := <-pq.critical:
		return msg
	case msg := <-pq.high:
		return msg
	case msg := <-pq.normal:
		return msg
	case msg := <-pq.low:
		return msg
	case <-timer.C:
		return nil
	}
}

// Len 获取队列长度
func (pq *PriorityQueue) Len() int {
	return len(pq.critical) + len(pq.high) + len(pq.normal) + len(pq.low)
}

// Stats 获取队列统计
func (pq *PriorityQueue) Stats() QueueStats {
	return QueueStats{
		CriticalCount: len(pq.critical),
		CriticalCap:   cap(pq.critical),
		HighCount:     len(pq.high),
		HighCap:       cap(pq.high),
		NormalCount:   len(pq.normal),
		NormalCap:     cap(pq.normal),
		LowCount:      len(pq.low),
		LowCap:        cap(pq.low),
		TotalCount:    pq.Len(),
		TotalCap:      cap(pq.critical) + cap(pq.high) + cap(pq.normal) + cap(pq.low),
	}
}

// QueueStats 队列统计
type QueueStats struct {
	CriticalCount int `json:"criticalCount"`
	CriticalCap   int `json:"criticalCap"`
	HighCount     int `json:"highCount"`
	HighCap       int `json:"highCap"`
	NormalCount   int `json:"normalCount"`
	NormalCap     int `json:"normalCap"`
	LowCount      int `json:"lowCount"`
	LowCap        int `json:"lowCap"`
	TotalCount    int `json:"totalCount"`
	TotalCap      int `json:"totalCap"`
}

// Deduplicator 消息去重器
type Deduplicator struct {
	cache     map[string]*dedupEntry
	mu        sync.RWMutex
	ttl       time.Duration
	maxSize   int
	hitCount  int64
	missCount int64
}

type dedupEntry struct {
	timestamp time.Time
	count     int
}

// NewDeduplicator 创建去重器
func NewDeduplicator(ttl time.Duration, maxSize int) *Deduplicator {
	if maxSize <= 0 {
		maxSize = 100000
	}

	d := &Deduplicator{
		cache:   make(map[string]*dedupEntry),
		ttl:     ttl,
		maxSize: maxSize,
	}

	// 启动清理协程
	go d.cleanup()

	return d
}

// Check 检查消息是否重复
func (d *Deduplicator) Check(msg *Message) bool {
	if msg.Hash == "" {
		msg.Hash = d.computeHash(msg)
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	entry, exists := d.cache[msg.Hash]
	if exists && time.Since(entry.timestamp) < d.ttl {
		entry.count++
		atomic.AddInt64(&d.hitCount, 1)
		return true // 重复
	}

	// 添加到缓存
	if len(d.cache) >= d.maxSize {
		d.evictOldest()
	}
	d.cache[msg.Hash] = &dedupEntry{
		timestamp: time.Now(),
		count:     1,
	}
	atomic.AddInt64(&d.missCount, 1)
	return false
}

// computeHash 计算消息哈希
func (d *Deduplicator) computeHash(msg *Message) string {
	// 使用消息类型、数据和来源计算哈希
	hasher := sha256.New()
	hasher.Write([]byte(msg.Type))
	hasher.Write(msg.Data)
	if msg.From != "" {
		hasher.Write([]byte(msg.From))
	}
	if msg.To != "" {
		hasher.Write([]byte(msg.To))
	}
	return hex.EncodeToString(hasher.Sum(nil))[:16]
}

// evictOldest 清理最旧的条目
func (d *Deduplicator) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for k, v := range d.cache {
		if oldestKey == "" || v.timestamp.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.timestamp
		}
	}

	if oldestKey != "" {
		delete(d.cache, oldestKey)
	}
}

// cleanup 定期清理过期条目
func (d *Deduplicator) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		d.mu.Lock()
		now := time.Now()
		for k, v := range d.cache {
			if now.Sub(v.timestamp) > d.ttl {
				delete(d.cache, k)
			}
		}
		d.mu.Unlock()
	}
}

// Stats 获取去重统计
func (d *Deduplicator) Stats() DedupStats {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return DedupStats{
		CacheSize: len(d.cache),
		MaxSize:   d.maxSize,
		HitCount:  atomic.LoadInt64(&d.hitCount),
		MissCount: atomic.LoadInt64(&d.missCount),
	}
}

// DedupStats 去重统计
type DedupStats struct {
	CacheSize int   `json:"cacheSize"`
	MaxSize   int   `json:"maxSize"`
	HitCount  int64 `json:"hitCount"`
	MissCount int64 `json:"missCount"`
}

// BackpressureController 背压控制器
type BackpressureController struct {
	config           *MessageQueueConfig
	currentLoad      float64
	state            BackpressureState
	mu               sync.RWMutex
	onStateChange    func(old, new BackpressureState)
	droppedCount     int64
	throttledCount   int64
	throttleDuration time.Duration
}

// BackpressureState 背压状态
type BackpressureState int

const (
	// StateNormal 正常状态，队列负载正常
	StateNormal BackpressureState = iota
	// StateWarning 警告状态，队列负载较高
	StateWarning
	// StateCritical 临界状态，队列即将满载
	StateCritical
	// StateThrottled 限流状态，需要限流处理
	StateThrottled
)

func (s BackpressureState) String() string {
	switch s {
	case StateNormal:
		return "normal"
	case StateWarning:
		return "warning"
	case StateCritical:
		return "critical"
	case StateThrottled:
		return "throttled"
	default:
		return "unknown"
	}
}

// NewBackpressureController 创建背压控制器
func NewBackpressureController(config *MessageQueueConfig) *BackpressureController {
	return &BackpressureController{
		config:           config,
		state:            StateNormal,
		throttleDuration: 10 * time.Millisecond,
	}
}

// Check 检查背压状态
func (bc *BackpressureController) Check(queueLen, queueCap int) BackpressureState {
	if !bc.config.EnableBackpressure {
		return StateNormal
	}

	load := float64(queueLen) / float64(queueCap)
	bc.currentLoad = load

	bc.mu.Lock()
	defer bc.mu.Unlock()

	var newState BackpressureState
	switch {
	case load >= 0.95:
		newState = StateCritical
	case load >= bc.config.BackpressureThreshold:
		newState = StateWarning
	default:
		newState = StateNormal
	}

	if newState != bc.state {
		oldState := bc.state
		bc.state = newState
		if bc.onStateChange != nil {
			go bc.onStateChange(oldState, newState)
		}
	}

	return bc.state
}

// ShouldDrop 是否应该丢弃消息
func (bc *BackpressureController) ShouldDrop(msg *Message) bool {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	if bc.state == StateNormal {
		return false
	}

	// 关键消息永不丢弃
	if msg.Priority == PriorityCritical {
		return false
	}

	// 根据策略决定
	switch bc.config.BackpressureStrategy {
	case "drop_oldest":
		return bc.state == StateCritical
	case "drop_low_priority":
		return msg.Priority == PriorityLow || (bc.state == StateCritical && msg.Priority == PriorityNormal)
	case "reject":
		return bc.state >= StateWarning
	default:
		return false
	}
}

// Throttle 限流等待
func (bc *BackpressureController) Throttle() {
	bc.mu.RLock()
	state := bc.state
	bc.mu.RUnlock()

	if state >= StateWarning {
		atomic.AddInt64(&bc.throttledCount, 1)
		time.Sleep(bc.throttleDuration)
	}
}

// RecordDrop 记录丢弃
func (bc *BackpressureController) RecordDrop() {
	atomic.AddInt64(&bc.droppedCount, 1)
}

// OnStateChange 设置状态变更回调
func (bc *BackpressureController) OnStateChange(fn func(old, new BackpressureState)) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.onStateChange = fn
}

// Stats 获取背压统计
func (bc *BackpressureController) Stats() BackpressureStats {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	return BackpressureStats{
		State:            bc.state.String(),
		CurrentLoad:      bc.currentLoad,
		DroppedCount:     atomic.LoadInt64(&bc.droppedCount),
		ThrottledCount:   atomic.LoadInt64(&bc.throttledCount),
		ThrottleDuration: bc.throttleDuration.String(),
	}
}

// BackpressureStats 背压统计
type BackpressureStats struct {
	State            string  `json:"state"`
	CurrentLoad      float64 `json:"currentLoad"`
	DroppedCount     int64   `json:"droppedCount"`
	ThrottledCount   int64   `json:"throttledCount"`
	ThrottleDuration string  `json:"throttleDuration"`
}

// EnhancedMessageQueue 增强消息队列
type EnhancedMessageQueue struct {
	config        *MessageQueueConfig
	priorityQueue *PriorityQueue
	deduplicator  *Deduplicator
	backpressure  *BackpressureController
	pending       map[string]*Message // 待确认消息
	mu            sync.RWMutex
	stopChan      chan struct{}
	onMessage     func(*Message)
	onDropped     func(*Message, string)
	sentCount     int64
	droppedCount  int64
	dedupedCount  int64
}

// NewEnhancedMessageQueue 创建增强消息队列
func NewEnhancedMessageQueue(config *MessageQueueConfig) *EnhancedMessageQueue {
	if config == nil {
		config = DefaultMessageQueueConfig
	}

	emq := &EnhancedMessageQueue{
		config:        config,
		priorityQueue: NewPriorityQueue(config),
		pending:       make(map[string]*Message),
		stopChan:      make(chan struct{}),
	}

	if config.EnableDedup {
		emq.deduplicator = NewDeduplicator(config.DedupTTL, config.MaxSize)
	}

	if config.EnableBackpressure {
		emq.backpressure = NewBackpressureController(config)
	}

	return emq
}

// Push 推送消息
func (emq *EnhancedMessageQueue) Push(msg *Message) error {
	// 设置默认值
	if msg.ID == "" {
		msg.ID = generateMessageID()
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	if msg.Expiration.IsZero() && emq.config.DefaultTTL > 0 {
		msg.Expiration = msg.Timestamp.Add(emq.config.DefaultTTL)
	}
	if msg.MaxRetries == 0 {
		msg.MaxRetries = emq.config.MaxRetries
	}

	// 检查消息是否过期
	if !msg.Expiration.IsZero() && time.Now().After(msg.Expiration) {
		atomic.AddInt64(&emq.droppedCount, 1)
		if emq.onDropped != nil {
			emq.onDropped(msg, "expired")
		}
		return ErrMessageExpired
	}

	// 背压检查
	if emq.backpressure != nil {
		stats := emq.priorityQueue.Stats()
		state := emq.backpressure.Check(stats.TotalCount, stats.TotalCap)

		if emq.backpressure.ShouldDrop(msg) {
			atomic.AddInt64(&emq.droppedCount, 1)
			emq.backpressure.RecordDrop()
			if emq.onDropped != nil {
				emq.onDropped(msg, "backpressure")
			}
			return ErrBackpressureDrop
		}

		if state >= StateWarning {
			emq.backpressure.Throttle()
		}
	}

	// 去重检查
	if emq.deduplicator != nil && emq.config.EnableDedup {
		if emq.deduplicator.Check(msg) {
			atomic.AddInt64(&emq.dedupedCount, 1)
			return ErrDuplicateMessage
		}
	}

	// 推入队列
	if err := emq.priorityQueue.Push(msg); err != nil {
		atomic.AddInt64(&emq.droppedCount, 1)
		if emq.onDropped != nil {
			emq.onDropped(msg, "queue_full")
		}
		return err
	}

	return nil
}

// Pop 弹出消息
func (emq *EnhancedMessageQueue) Pop() *Message {
	msg := emq.priorityQueue.Pop()
	if msg == nil {
		return nil
	}

	// 检查过期
	if !msg.Expiration.IsZero() && time.Now().After(msg.Expiration) {
		atomic.AddInt64(&emq.droppedCount, 1)
		return emq.Pop() // 递归获取下一个
	}

	atomic.AddInt64(&emq.sentCount, 1)

	// 添加到待确认列表
	emq.mu.Lock()
	emq.pending[msg.ID] = msg
	emq.mu.Unlock()

	return msg
}

// PopWithTimeout 带超时弹出
func (emq *EnhancedMessageQueue) PopWithTimeout(timeout time.Duration) *Message {
	msg := emq.priorityQueue.PopWithTimeout(timeout)
	if msg == nil {
		return nil
	}

	// 检查过期
	if !msg.Expiration.IsZero() && time.Now().After(msg.Expiration) {
		atomic.AddInt64(&emq.droppedCount, 1)
		return emq.PopWithTimeout(timeout)
	}

	atomic.AddInt64(&emq.sentCount, 1)

	emq.mu.Lock()
	emq.pending[msg.ID] = msg
	emq.mu.Unlock()

	return msg
}

// Ack 确认消息
func (emq *EnhancedMessageQueue) Ack(messageID string) {
	emq.mu.Lock()
	defer emq.mu.Unlock()
	delete(emq.pending, messageID)
}

// Nack 否认消息（重新入队）
func (emq *EnhancedMessageQueue) Nack(messageID string) error {
	emq.mu.Lock()
	msg, exists := emq.pending[messageID]
	if !exists {
		emq.mu.Unlock()
		return ErrMessageNotFound
	}
	delete(emq.pending, messageID)
	emq.mu.Unlock()

	// 检查重试次数
	if msg.Retries >= msg.MaxRetries {
		atomic.AddInt64(&emq.droppedCount, 1)
		if emq.onDropped != nil {
			emq.onDropped(msg, "max_retries")
		}
		return ErrMaxRetriesExceeded
	}

	// 延迟重试
	msg.Retries++
	time.Sleep(emq.config.RetryDelay)

	return emq.Push(msg)
}

// Start 启动消息处理
func (emq *EnhancedMessageQueue) Start() {
	go emq.process()
}

// Stop 停止消息处理
func (emq *EnhancedMessageQueue) Stop() {
	close(emq.stopChan)
}

// process 消息处理循环
func (emq *EnhancedMessageQueue) process() {
	for {
		select {
		case <-emq.stopChan:
			return
		default:
			msg := emq.PopWithTimeout(100 * time.Millisecond)
			if msg != nil && emq.onMessage != nil {
				emq.onMessage(msg)
			}
		}
	}
}

// OnMessage 设置消息处理回调
func (emq *EnhancedMessageQueue) OnMessage(fn func(*Message)) {
	emq.onMessage = fn
}

// OnDropped 设置丢弃回调
func (emq *EnhancedMessageQueue) OnDropped(fn func(*Message, string)) {
	emq.onDropped = fn
}

// Stats 获取队列统计
func (emq *EnhancedMessageQueue) Stats() EnhancedQueueStats {
	emq.mu.RLock()
	pendingCount := len(emq.pending)
	emq.mu.RUnlock()

	stats := EnhancedQueueStats{
		Queue:        emq.priorityQueue.Stats(),
		SentCount:    atomic.LoadInt64(&emq.sentCount),
		DroppedCount: atomic.LoadInt64(&emq.droppedCount),
		DedupedCount: atomic.LoadInt64(&emq.dedupedCount),
		PendingCount: pendingCount,
	}

	if emq.deduplicator != nil {
		s := emq.deduplicator.Stats()
		stats.Dedup = &s
	}

	if emq.backpressure != nil {
		s := emq.backpressure.Stats()
		stats.Backpressure = &s
	}

	return stats
}

// EnhancedQueueStats 增强队列统计
type EnhancedQueueStats struct {
	Queue        QueueStats         `json:"queue"`
	SentCount    int64              `json:"sentCount"`
	DroppedCount int64              `json:"droppedCount"`
	DedupedCount int64              `json:"dedupedCount"`
	PendingCount int                `json:"pendingCount"`
	Dedup        *DedupStats        `json:"dedup,omitempty"`
	Backpressure *BackpressureStats `json:"backpressure,omitempty"`
}

// BatchPush 批量推送消息
func (emq *EnhancedMessageQueue) BatchPush(messages []*Message) (int, []error) {
	var errors []error
	successCount := 0

	for _, msg := range messages {
		if err := emq.Push(msg); err != nil {
			errors = append(errors, err)
		} else {
			successCount++
		}
	}

	return successCount, errors
}

// BatchPop 批量弹出消息
func (emq *EnhancedMessageQueue) BatchPop(maxCount int) []*Message {
	var messages []*Message

	for i := 0; i < maxCount; i++ {
		msg := emq.priorityQueue.Pop()
		if msg == nil {
			break
		}

		// 检查过期
		if !msg.Expiration.IsZero() && time.Now().After(msg.Expiration) {
			atomic.AddInt64(&emq.droppedCount, 1)
			continue
		}

		messages = append(messages, msg)
		atomic.AddInt64(&emq.sentCount, 1)

		emq.mu.Lock()
		emq.pending[msg.ID] = msg
		emq.mu.Unlock()
	}

	return messages
}

// Flush 清空队列
func (emq *EnhancedMessageQueue) Flush() int {
	count := 0
	for {
		msg := emq.priorityQueue.Pop()
		if msg == nil {
			break
		}
		count++
	}
	return count
}

// generateMessageID 生成消息 ID
func generateMessageID() string {
	return fmt.Sprintf("msg-%d-%s", time.Now().UnixNano(), randomString(8))
}

// randomString 生成随机字符串（使用 crypto/rand 安全随机数）
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			// 如果 crypto/rand 失败，使用时间作为回退
			b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
			continue
		}
		b[i] = letters[num.Int64()]
	}
	return string(b)
}

// 错误定义
var (
	ErrQueueFull          = fmt.Errorf("队列已满")
	ErrMessageExpired     = fmt.Errorf("消息已过期")
	ErrDuplicateMessage   = fmt.Errorf("重复消息")
	ErrBackpressureDrop   = fmt.Errorf("背压丢弃")
	ErrMessageNotFound    = fmt.Errorf("消息不存在")
	ErrMaxRetriesExceeded = fmt.Errorf("超过最大重试次数")
)
