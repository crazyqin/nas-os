package concurrency

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
)

var (
	// ErrPoolClosed 工作池已关闭错误.
	ErrPoolClosed = errors.New("worker pool is closed")
	// ErrPoolTimeout 工作池超时错误.
	ErrPoolTimeout = errors.New("worker pool timeout")
	// ErrQueueFull 工作队列已满错误.
	ErrQueueFull = errors.New("worker queue is full")
)

// Task represents a unit of work.
type Task func() error

// WorkerPool manages a pool of worker goroutines.
type WorkerPool struct {
	workers  int
	maxQueue int
	taskChan chan Task
	errChan  chan error
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	closed   bool
	mu       sync.Mutex

	// Statistics
	submitted int64
	completed int64
	failed    int64

	logger *zap.Logger
}

// NewWorkerPool creates a new worker pool.
func NewWorkerPool(workers, maxQueue int, logger *zap.Logger) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	pool := &WorkerPool{
		workers:  workers,
		maxQueue: maxQueue,
		taskChan: make(chan Task, maxQueue),
		errChan:  make(chan error, workers),
		ctx:      ctx,
		cancel:   cancel,
		logger:   logger,
	}

	// Start workers
	for i := 0; i < workers; i++ {
		pool.wg.Add(1)
		go pool.worker(i)
	}

	return pool
}

// worker is the main worker loop.
func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		case task, ok := <-p.taskChan:
			if !ok {
				return
			}

			if err := task(); err != nil {
				p.mu.Lock()
				p.failed++
				p.mu.Unlock()

				select {
				case p.errChan <- err:
				default:
					// Error channel full, log and continue
					if p.logger != nil {
						p.logger.Error("Worker task error", zap.Int("worker", id), zap.Error(err))
					}
				}
			} else {
				p.mu.Lock()
				p.completed++
				p.mu.Unlock()
			}
		}
	}
}

// Submit adds a task to the pool.
func (p *WorkerPool) Submit(task Task) error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return ErrPoolClosed
	}
	p.mu.Unlock()

	select {
	case p.taskChan <- task:
		p.mu.Lock()
		p.submitted++
		p.mu.Unlock()
		return nil
	case <-time.After(5 * time.Second):
		return ErrQueueFull
	case <-p.ctx.Done():
		return ErrPoolClosed
	}
}

// SubmitWait submits a task and waits for completion.
func (p *WorkerPool) SubmitWait(task Task, timeout time.Duration) error {
	done := make(chan error, 1)

	err := p.Submit(func() error {
		err := task()
		done <- err
		return err
	})

	if err != nil {
		return err
	}

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return ErrPoolTimeout
	case <-p.ctx.Done():
		return ErrPoolClosed
	}
}

// Close gracefully shuts down the pool.
func (p *WorkerPool) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	p.mu.Unlock()

	p.cancel()
	close(p.taskChan)
	p.wg.Wait()
	close(p.errChan)
}

// Stats returns pool statistics.
func (p *WorkerPool) Stats() PoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()

	return PoolStats{
		Workers:   p.workers,
		QueueSize: len(p.taskChan),
		MaxQueue:  p.maxQueue,
		Submitted: p.submitted,
		Completed: p.completed,
		Failed:    p.failed,
		Pending:   int64(len(p.taskChan)),
	}
}

// PoolStats holds worker pool statistics.
type PoolStats struct {
	Workers   int   `json:"workers"`
	QueueSize int   `json:"queue_size"`
	MaxQueue  int   `json:"max_queue"`
	Submitted int64 `json:"submitted"`
	Completed int64 `json:"completed"`
	Failed    int64 `json:"failed"`
	Pending   int64 `json:"pending"`
}

// ErrorChan returns the error channel for monitoring.
func (p *WorkerPool) ErrorChan() <-chan error {
	return p.errChan
}
