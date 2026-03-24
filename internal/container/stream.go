// Package container provides Docker container management functionality
// File: stream.go - Container log streaming and real-time stats monitoring
package container

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// StreamConfig configures streaming behavior.
type StreamConfig struct {
	// Follow whether to follow log output
	Follow bool `json:"follow"`
	// Tail number of lines to show from the end
	Tail int `json:"tail"`
	// Since show logs since timestamp (e.g. "2023-01-01", "1h30m")
	Since string `json:"since"`
	// Until show logs before timestamp
	Until string `json:"until"`
	// Timestamps show timestamps
	Timestamps bool `json:"timestamps"`
	// Stdout show stdout
	Stdout bool `json:"stdout"`
	// Stderr show stderr
	Stderr bool `json:"stderr"`
}

// DefaultStreamConfig returns default streaming configuration.
func DefaultStreamConfig() StreamConfig {
	return StreamConfig{
		Follow:     true,
		Tail:       100,
		Timestamps: true,
		Stdout:     true,
		Stderr:     true,
	}
}

// LogMessage represents a log entry for streaming.
type LogMessage struct {
	Timestamp time.Time `json:"timestamp"`
	Line      string    `json:"line"`
	Source    string    `json:"source"` // "stdout" or "stderr"
	Container string    `json:"container,omitempty"`
}

// StatsMessage represents a stats update for streaming.
type StatsMessage struct {
	Timestamp   time.Time `json:"timestamp"`
	CPUUsage    float64   `json:"cpuUsage"`
	MemUsage    uint64    `json:"memUsage"`
	MemLimit    uint64    `json:"memLimit"`
	MemPercent  float64   `json:"memPercent"`
	NetRX       uint64    `json:"netRx"`
	NetTX       uint64    `json:"netTx"`
	BlockRead   uint64    `json:"blockRead"`
	BlockWrite  uint64    `json:"blockWrite"`
	PIDs        uint64    `json:"pids"`
	ContainerID string    `json:"containerId"`
	Container   string    `json:"container,omitempty"`
}

// LogStreamer manages log streaming for containers.
type LogStreamer struct {
	manager *Manager
	mu      sync.RWMutex
	active  map[string]context.CancelFunc
}

// NewLogStreamer creates a new log streamer.
func NewLogStreamer(manager *Manager) *LogStreamer {
	return &LogStreamer{
		manager: manager,
		active:  make(map[string]context.CancelFunc),
	}
}

// StreamLogs starts streaming logs for a container.
// Returns a channel that emits LogMessage entries.
func (s *LogStreamer) StreamLogs(ctx context.Context, containerID string, config StreamConfig) (<-chan LogMessage, error) {
	args := s.buildLogArgs(containerID, config)

	cmd := exec.CommandContext(ctx, "docker", args...)

	// Get both stdout and stderr pipes
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdout.Close()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start log stream: %w", err)
	}

	logChan := make(chan LogMessage, 256)

	// Track this stream
	s.mu.Lock()
	s.active[containerID] = func() { _ = cmd.Process.Kill() }
	s.mu.Unlock()

	// Cleanup function
	cleanup := func() {
		s.mu.Lock()
		delete(s.active, containerID)
		s.mu.Unlock()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		close(logChan)
	}

	// Read from stdout
	go func() {
		defer cleanup()
		s.readStream(stdout, "stdout", logChan)
	}()

	// Read from stderr if enabled
	if config.Stderr {
		go s.readStream(stderr, "stderr", logChan)
	}

	return logChan, nil
}

// buildLogArgs builds docker logs command arguments.
func (s *LogStreamer) buildLogArgs(containerID string, config StreamConfig) []string {
	args := []string{"logs"}

	if config.Follow {
		args = append(args, "-f")
	}

	if config.Tail > 0 {
		args = append(args, "--tail", fmt.Sprintf("%d", config.Tail))
	}

	if config.Since != "" {
		args = append(args, "--since", config.Since)
	}

	if config.Until != "" {
		args = append(args, "--until", config.Until)
	}

	if config.Timestamps {
		args = append(args, "-t")
	}

	args = append(args, containerID)
	return args
}

// readStream reads from a stream and sends log messages.
func (s *LogStreamer) readStream(reader io.Reader, source string, logChan chan<- LogMessage) {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		// Parse timestamp if present
		msg := LogMessage{
			Timestamp: time.Now(),
			Line:      line,
			Source:    source,
		}

		// Try to parse docker log timestamp format
		if len(line) > 30 && line[4] == '-' && line[7] == '-' {
			// Format: 2024-03-25T10:30:00.000000000Z message
			if t, err := time.Parse(time.RFC3339Nano, line[:30]); err == nil {
				msg.Timestamp = t
				msg.Line = strings.TrimSpace(line[30:])
			}
		}

		select {
		case logChan <- msg:
		default:
			// Channel full, skip message
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("[LogStreamer] Scanner error: %v", err)
	}
}

// Stop stops all active log streams.
func (s *LogStreamer) Stop(containerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cancel, ok := s.active[containerID]; ok {
		cancel()
		delete(s.active, containerID)
	}
}

// StopAll stops all active log streams.
func (s *LogStreamer) StopAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, cancel := range s.active {
		cancel()
		delete(s.active, id)
	}
}

// StatsStreamer manages real-time stats streaming for containers.
type StatsStreamer struct {
	manager    *Manager
	mu         sync.RWMutex
	active     map[string]context.CancelFunc
	updateChan chan StatsMessage
}

// NewStatsStreamer creates a new stats streamer.
func NewStatsStreamer(manager *Manager) *StatsStreamer {
	return &StatsStreamer{
		manager:    manager,
		active:     make(map[string]context.CancelFunc),
		updateChan: make(chan StatsMessage, 512),
	}
}

// StreamStats starts streaming stats for a container.
// Returns a channel that emits StatsMessage entries at regular intervals.
func (s *StatsStreamer) StreamStats(ctx context.Context, containerID string, _ time.Duration) (<-chan StatsMessage, error) {

	statsChan := make(chan StatsMessage, 64)

	// Use docker stats --no-stream=false for continuous streaming
	cmd := exec.CommandContext(ctx, "docker", "stats", "--format", "{{json .}}", containerID)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stats pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start stats stream: %w", err)
	}

	// Track this stream
	s.mu.Lock()
	s.active[containerID] = func() { _ = cmd.Process.Kill() }
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.active, containerID)
			s.mu.Unlock()
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			close(statsChan)
		}()

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			stats := s.parseStatsLine(line, containerID)

			select {
			case statsChan <- stats:
			case <-ctx.Done():
				return
			default:
				// Channel full, skip update
			}
		}
	}()

	return statsChan, nil
}

// StreamAllStats streams stats for all running containers
func (s *StatsStreamer) StreamAllStats(ctx context.Context, interval time.Duration) (<-chan StatsMessage, error) {
	if interval <= 0 {
		interval = 2 * time.Second
	}

	statsChan := make(chan StatsMessage, 256)

	go func() {
		defer close(statsChan)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				stats, err := s.getAllContainerStats(ctx)
				if err != nil {
					log.Printf("[StatsStreamer] Error getting stats: %v", err)
					continue
				}

				for _, stat := range stats {
					select {
					case statsChan <- stat:
					default:
						// Channel full, skip
					}
				}
			}
		}
	}()

	return statsChan, nil
}

// getAllContainerStats gets stats for all running containers
func (s *StatsStreamer) getAllContainerStats(ctx context.Context) ([]StatsMessage, error) {
	cmd := exec.CommandContext(ctx, "docker", "stats", "--no-stream", "--format", "{{json .}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	var stats []StatsMessage
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var raw struct {
			Container string `json:"Container"`
			CPUPerc   string `json:"CPUPerc"`
			MemUsage  string `json:"MemUsage"`
			MemPerc   string `json:"MemPerc"`
			NetIO     string `json:"NetIO"`
			BlockIO   string `json:"BlockIO"`
			PIDs      string `json:"PIDs"`
		}

		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		stats = append(stats, s.parseStatsFromRaw(raw))
	}

	return stats, nil
}

// parseStatsLine parses a docker stats JSON line
func (s *StatsStreamer) parseStatsLine(line string, containerID string) StatsMessage {
	var raw struct {
		CPUPerc  string `json:"CPUPerc"`
		MemUsage string `json:"MemUsage"`
		MemPerc  string `json:"MemPerc"`
		NetIO    string `json:"NetIO"`
		BlockIO  string `json:"BlockIO"`
		PIDs     string `json:"PIDs"`
	}

	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return StatsMessage{Timestamp: time.Now(), ContainerID: containerID}
	}

	return s.parseStatsFromRaw(struct {
		Container string `json:"Container"`
		CPUPerc   string `json:"CPUPerc"`
		MemUsage  string `json:"MemUsage"`
		MemPerc   string `json:"MemPerc"`
		NetIO     string `json:"NetIO"`
		BlockIO   string `json:"BlockIO"`
		PIDs      string `json:"PIDs"`
	}{
		Container: containerID,
		CPUPerc:   raw.CPUPerc,
		MemUsage:  raw.MemUsage,
		MemPerc:   raw.MemPerc,
		NetIO:     raw.NetIO,
		BlockIO:   raw.BlockIO,
		PIDs:      raw.PIDs,
	})
}

// parseStatsFromRaw parses stats from raw JSON struct
func (s *StatsStreamer) parseStatsFromRaw(raw struct {
	Container string `json:"Container"`
	CPUPerc   string `json:"CPUPerc"`
	MemUsage  string `json:"MemUsage"`
	MemPerc   string `json:"MemPerc"`
	NetIO     string `json:"NetIO"`
	BlockIO   string `json:"BlockIO"`
	PIDs      string `json:"PIDs"`
}) StatsMessage {
	stats := StatsMessage{
		Timestamp:   time.Now(),
		ContainerID: raw.Container,
		Container:   raw.Container,
	}

	// Parse CPU percentage
	cpuStr := strings.TrimSuffix(raw.CPUPerc, "%")
	if _, err := fmt.Sscanf(cpuStr, "%f", &stats.CPUUsage); err != nil {
		stats.CPUUsage = 0
	}

	// Parse memory usage
	memParts := strings.Split(raw.MemUsage, " / ")
	if len(memParts) >= 2 {
		stats.MemUsage = parseSize(memParts[0])
		stats.MemLimit = parseSize(memParts[1])
	}

	// Parse memory percentage
	memPercStr := strings.TrimSuffix(raw.MemPerc, "%")
	if _, err := fmt.Sscanf(memPercStr, "%f", &stats.MemPercent); err != nil {
		stats.MemPercent = 0
	}

	// Parse network I/O
	netParts := strings.Split(raw.NetIO, " / ")
	if len(netParts) >= 2 {
		stats.NetRX = parseSize(netParts[0])
		stats.NetTX = parseSize(netParts[1])
	}

	// Parse block I/O
	blockParts := strings.Split(raw.BlockIO, " / ")
	if len(blockParts) >= 2 {
		stats.BlockRead = parseSize(blockParts[0])
		stats.BlockWrite = parseSize(blockParts[1])
	}

	// Parse PIDs
	if _, err := fmt.Sscanf(raw.PIDs, "%d", &stats.PIDs); err != nil {
		stats.PIDs = 0
	}

	return stats
}

// Stop stops streaming stats for a container
func (s *StatsStreamer) Stop(containerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cancel, ok := s.active[containerID]; ok {
		cancel()
		delete(s.active, containerID)
	}
}

// StopAll stops all active stats streams
func (s *StatsStreamer) StopAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, cancel := range s.active {
		cancel()
		delete(s.active, id)
	}
}
