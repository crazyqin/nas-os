package concurrency

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
)

var (
	ErrPoolExhausted = errors.New("connection pool exhausted")
	ErrConnClosed    = errors.New("connection is closed")
)

// Connection represents a pooled connection
type Connection interface {
	Close() error
	IsHealthy() bool
}

// ConnectionFactory creates new connections
type ConnectionFactory func() (Connection, error)

// ConnectionPool manages a pool of reusable connections
type ConnectionPool struct {
	factory     ConnectionFactory
	maxSize     int
	minSize     int
	idleTimeout time.Duration

	conns     chan Connection
	mu        sync.Mutex
	closed    bool
	openCount int
	ctx       context.Context
	cancel    context.CancelFunc

	// Statistics
	created      int64
	reused       int64
	closed_count int64
	waitCount    int64

	logger *zap.Logger
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(
	factory ConnectionFactory,
	maxSize, minSize int,
	idleTimeout time.Duration,
	logger *zap.Logger,
) *ConnectionPool {
	ctx, cancel := context.WithCancel(context.Background())

	pool := &ConnectionPool{
		factory:     factory,
		maxSize:     maxSize,
		minSize:     minSize,
		idleTimeout: idleTimeout,
		conns:       make(chan Connection, maxSize),
		ctx:         ctx,
		cancel:      cancel,
		logger:      logger,
	}

	// Pre-create minimum connections
	for i := 0; i < minSize; i++ {
		if conn, err := factory(); err == nil {
			pool.conns <- conn
			pool.openCount++
			pool.created++
		} else if logger != nil {
			logger.Error("Failed to create initial connection", zap.Error(err))
		}
	}

	// Start connection health checker
	go pool.healthCheck()

	return pool
}

// Get retrieves a connection from the pool
func (p *ConnectionPool) Get(timeout time.Duration) (Connection, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, ErrConnClosed
	}
	p.mu.Unlock()

	// Try to get existing connection
	select {
	case conn := <-p.conns:
		if conn.IsHealthy() {
			p.mu.Lock()
			p.reused++
			p.mu.Unlock()
			return conn, nil
		}
		// Connection unhealthy, close and create new one
		_ = conn.Close()
		p.mu.Lock()
		p.closed_count++
		p.mu.Unlock()

	default:
		// No available connections
	}

	// Try to create new connection if under limit
	p.mu.Lock()
	if p.openCount < p.maxSize {
		p.openCount++
		p.mu.Unlock()

		conn, err := p.factory()
		if err != nil {
			p.mu.Lock()
			p.openCount--
			p.mu.Unlock()
			return nil, err
		}
		p.mu.Lock()
		p.created++
		p.mu.Unlock()
		return conn, nil
	}
	p.mu.Unlock()

	// Pool exhausted, wait for available connection
	p.mu.Lock()
	p.waitCount++
	p.mu.Unlock()

	select {
	case conn := <-p.conns:
		if conn.IsHealthy() {
			p.mu.Lock()
			p.reused++
			p.mu.Unlock()
			return conn, nil
		}
		_ = conn.Close()
		p.mu.Lock()
		p.closed_count++
		p.openCount--
		p.mu.Unlock()
		return p.Get(timeout)
	case <-time.After(timeout):
		return nil, ErrPoolExhausted
	}
}

// Put returns a connection to the pool
func (p *ConnectionPool) Put(conn Connection) {
	if conn == nil {
		return
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		_ = conn.Close()
		p.mu.Lock()
		p.closed_count++
		p.openCount--
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()

	if !conn.IsHealthy() {
		_ = conn.Close()
		p.mu.Lock()
		p.closed_count++
		p.openCount--
		p.mu.Unlock()
		return
	}

	select {
	case p.conns <- conn:
	default:
		// Pool full, close connection
		_ = conn.Close()
		p.mu.Lock()
		p.closed_count++
		p.openCount--
		p.mu.Unlock()
	}
}

// Close closes all connections in the pool
func (p *ConnectionPool) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	p.mu.Unlock()

	// Signal healthCheck goroutine to stop
	p.cancel()

	close(p.conns)

	for conn := range p.conns {
		_ = conn.Close()
		p.mu.Lock()
		p.closed_count++
		p.openCount--
		p.mu.Unlock()
	}
}

// Stats returns pool statistics
func (p *ConnectionPool) Stats() ConnPoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()

	return ConnPoolStats{
		MaxSize:   p.maxSize,
		MinSize:   p.minSize,
		OpenCount: p.openCount,
		Available: len(p.conns),
		Created:   p.created,
		Reused:    p.reused,
		Closed:    p.closed_count,
		WaitCount: p.waitCount,
	}
}

// ConnPoolStats holds connection pool statistics
type ConnPoolStats struct {
	MaxSize   int   `json:"max_size"`
	MinSize   int   `json:"min_size"`
	OpenCount int   `json:"open_count"`
	Available int   `json:"available"`
	Created   int64 `json:"created"`
	Reused    int64 `json:"reused"`
	Closed    int64 `json:"closed"`
	WaitCount int64 `json:"wait_count"`
}

// healthCheck periodically checks and removes unhealthy connections
func (p *ConnectionPool) healthCheck() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.mu.Lock()
			if p.closed {
				p.mu.Unlock()
				return
			}
			p.mu.Unlock()

			// Check idle connections
			p.checkIdleConnections()
		case <-p.ctx.Done():
			return
		}
	}
}

// checkIdleConnections checks and removes unhealthy idle connections
func (p *ConnectionPool) checkIdleConnections() {
	select {
	case conn := <-p.conns:
		if !conn.IsHealthy() {
			_ = conn.Close()
			p.mu.Lock()
			p.closed_count++
			p.openCount--
			p.mu.Unlock()

			// Create replacement if below minSize
			if p.openCount < p.minSize {
				if newConn, err := p.factory(); err == nil {
					p.conns <- newConn
					p.openCount++
					p.created++
				}
			}
		} else {
			p.conns <- conn
		}
	default:
		// No idle connections
	}
}
