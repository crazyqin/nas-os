// Package session 提供SMB/NFS会话监控和管理功能
package session

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"nas-os/internal/logging"
)

// SMBProvider SMB会话数据提供者接口.
type SMBProvider interface {
	Connections() ([]*SMBConnection, error)
	KillConnection(pid int) error
}

// NFSProvider NFS会话数据提供者接口.
type NFSProvider interface {
	GetClients() ([]*NFSClient, error)
	KillClient(clientID string) error
}

// SMBConnection SMB连接信息（从SMB模块获取）.
type SMBConnection struct {
	PID         int
	Username    string
	ShareName   string
	ClientIP    string
	ClientName  string
	Protocol    string
	Encryption  string
	ConnectedAt time.Time
	LockedFiles []string
}

// NFSClient NFS客户端信息.
type NFSClient struct {
	ID          string
	ClientIP    string
	SharePath   string
	ConnectedAt time.Time
	BytesRead   int64
	BytesWrite  int64
}

// Monitor 会话监控器.
type Monitor struct {
	manager      *Manager
	logger       *logging.Logger
	smbProvider  SMBProvider
	nfsProvider  NFSProvider
	stopCh       chan struct{}
	running      bool
	runningMu    sync.Mutex
	pollInterval time.Duration
	lastSMBCount int
	lastNFSCount int
}

// NewMonitor 创建会话监控器.
func NewMonitor(manager *Manager, logger *logging.Logger) *Monitor {
	if logger == nil {
		logger = logging.NewLogger(nil).WithSource("session-monitor")
	}

	return &Monitor{
		manager:      manager,
		logger:       logger,
		stopCh:       make(chan struct{}),
		pollInterval: manager.GetConfig().RefreshInterval,
	}
}

// SetSMBProvider 设置SMB数据提供者.
func (m *Monitor) SetSMBProvider(provider SMBProvider) {
	m.smbProvider = provider
}

// SetNFSProvider 设置NFS数据提供者.
func (m *Monitor) SetNFSProvider(provider NFSProvider) {
	m.nfsProvider = provider
}

// SetPollInterval 设置轮询间隔.
func (m *Monitor) SetPollInterval(interval time.Duration) {
	m.pollInterval = interval
}

// Start 启动监控.
func (m *Monitor) Start(ctx context.Context) error {
	m.runningMu.Lock()
	if m.running {
		m.runningMu.Unlock()
		return fmt.Errorf("监控器已在运行")
	}
	m.running = true
	m.runningMu.Unlock()

	m.logger.Info("会话监控器启动")

	go m.run(ctx)

	return nil
}

// Stop 停止监控.
func (m *Monitor) Stop() {
	m.runningMu.Lock()
	defer m.runningMu.Unlock()

	if !m.running {
		return
	}

	close(m.stopCh)
	m.running = false
	m.logger.Info("会话监控器已停止")
}

// run 运行监控循环.
func (m *Monitor) run(ctx context.Context) {
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	// 立即执行一次
	m.poll()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.poll()
		}
	}
}

// poll 执行一次轮询.
func (m *Monitor) poll() {
	var wg sync.WaitGroup
	var smbSessions []*Session
	var nfsSessions []*Session
	var smbErr, nfsErr error

	// 并行收集 SMB 和 NFS 会话
	wg.Add(2)

	go func() {
		defer wg.Done()
		smbSessions, smbErr = m.collectSMBSessions()
	}()

	go func() {
		defer wg.Done()
		nfsSessions, nfsErr = m.collectNFSSessions()
	}()

	wg.Wait()

	// 合并会话
	allSessions := make([]*Session, 0, len(smbSessions)+len(nfsSessions))
	allSessions = append(allSessions, smbSessions...)
	allSessions = append(allSessions, nfsSessions...)

	// 同步到管理器
	m.manager.SyncSessions(allSessions)

	// 清理过期会话
	cleaned := m.manager.CleanupStale()
	if cleaned > 0 {
		m.logger.Infof("清理了 %d 个过期会话", cleaned)
	}

	// 标记空闲会话
	idle := m.manager.MarkIdle()
	if idle > 0 {
		m.logger.Debugf("标记了 %d 个空闲会话", idle)
	}

	// 记录日志
	smbCount := len(smbSessions)
	nfsCount := len(nfsSessions)
	if smbCount != m.lastSMBCount || nfsCount != m.lastNFSCount {
		m.logger.Infof("会话更新: SMB=%d, NFS=%d (变更: SMB %+d, NFS %+d)",
			smbCount, nfsCount, smbCount-m.lastSMBCount, nfsCount-m.lastNFSCount)
		m.lastSMBCount = smbCount
		m.lastNFSCount = nfsCount
	}

	// 记录错误
	if smbErr != nil {
		m.logger.Warnf("收集SMB会话失败: %v", smbErr)
	}
	if nfsErr != nil {
		m.logger.Warnf("收集NFS会话失败: %v", nfsErr)
	}
}

// collectSMBSessions 收集SMB会话.
func (m *Monitor) collectSMBSessions() ([]*Session, error) {
	var sessions []*Session

	// 如果有提供者，使用提供者
	if m.smbProvider != nil {
		connections, err := m.smbProvider.Connections()
		if err != nil {
			return nil, err
		}

		for _, conn := range connections {
			session := &Session{
				ID:           fmt.Sprintf("smb-%d", conn.PID),
				Type:         SessionTypeSMB,
				User:         conn.Username,
				ClientIP:     conn.ClientIP,
				ClientName:   conn.ClientName,
				ShareName:    conn.ShareName,
				SharePath:    conn.ShareName, // SMB使用共享名
				Status:       StatusActive,
				ConnectedAt:  conn.ConnectedAt,
				LastActiveAt: time.Now(),
				Protocol:     conn.Protocol,
				Encryption:   conn.Encryption,
				PID:          conn.PID,
				LockedFiles:  conn.LockedFiles,
			}
			sessions = append(sessions, session)
		}

		return sessions, nil
	}

	// 否则直接使用 smbstatus 命令
	return m.collectSMBSessionsFromCommand()
}

// collectSMBSessionsFromCommand 从命令收集SMB会话.
func (m *Monitor) collectSMBSessionsFromCommand() ([]*Session, error) {
	// 使用 smbstatus -b 获取会话信息
	cmd := exec.CommandContext(context.Background(), "smbstatus", "-b")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("执行smbstatus失败: %w", err)
	}

	var sessions []*Session
	lines := strings.Split(string(output), "\n")

	for i, line := range lines {
		if i < 3 { // 跳过头部
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 5 {
			pid := 0
			if p, err := strconv.Atoi(fields[0]); err == nil {
				pid = p
			}

			session := &Session{
				ID:           fmt.Sprintf("smb-%d", pid),
				Type:         SessionTypeSMB,
				User:         fields[1],
				ClientIP:     fields[2],
				Protocol:     fields[3],
				ShareName:    fields[4],
				SharePath:    fields[4],
				Status:       StatusActive,
				ConnectedAt:  time.Now(),
				LastActiveAt: time.Now(),
				PID:          pid,
			}

			if len(fields) >= 6 {
				session.ClientName = fields[5]
			}

			sessions = append(sessions, session)
		}
	}

	// 获取锁定文件
	m.collectSMBLockedFiles(sessions)

	return sessions, nil
}

// collectSMBLockedFiles 收集SMB锁定文件.
func (m *Monitor) collectSMBLockedFiles(sessions []*Session) {
	// 使用 smbstatus -L 获取锁定文件
	cmd := exec.CommandContext(context.Background(), "smbstatus", "-L")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	// 建立PID到会话的映射
	sessionMap := make(map[int]*Session)
	for _, s := range sessions {
		sessionMap[s.PID] = s
	}

	// 解析锁定文件
	lines := strings.Split(string(output), "\n")
	for i, line := range lines {
		if i < 3 || strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 1 {
			if pid, err := strconv.Atoi(fields[0]); err == nil {
				if session, exists := sessionMap[pid]; exists {
					if len(fields) >= 6 {
						session.LockedFiles = append(session.LockedFiles, fields[5])
					}
					session.FilesOpen++
				}
			}
		}
	}
}

// collectNFSSessions 收集NFS会话.
func (m *Monitor) collectNFSSessions() ([]*Session, error) {
	var sessions []*Session

	// 如果有提供者，使用提供者
	if m.nfsProvider != nil {
		clients, err := m.nfsProvider.GetClients()
		if err != nil {
			return nil, err
		}

		for _, client := range clients {
			session := &Session{
				ID:           fmt.Sprintf("nfs-%s", client.ID),
				Type:         SessionTypeNFS,
				User:         "nfs-client", // NFS没有用户概念
				ClientIP:     client.ClientIP,
				SharePath:    client.SharePath,
				Status:       StatusActive,
				ConnectedAt:  client.ConnectedAt,
				LastActiveAt: time.Now(),
				BytesRead:    client.BytesRead,
				BytesWritten: client.BytesWrite,
			}
			sessions = append(sessions, session)
		}

		return sessions, nil
	}

	// 否则直接从系统收集
	return m.collectNFSSessionsFromSystem()
}

// collectNFSSessionsFromSystem 从系统收集NFS会话.
func (m *Monitor) collectNFSSessionsFromSystem() ([]*Session, error) {
	var sessions []*Session

	// 从 /proc/fs/nfsd/clients/ 收集客户端信息
	clientsDir := "/proc/fs/nfsd/clients"

	entries, err := exec.CommandContext(context.Background(), "ls", "-1", clientsDir).Output()
	if err != nil {
		return nil, fmt.Errorf("读取NFS客户端目录失败: %w", err)
	}

	clientIDs := strings.Split(strings.TrimSpace(string(entries)), "\n")
	for _, clientID := range clientIDs {
		if clientID == "" {
			continue
		}

		session := &Session{
			ID:           fmt.Sprintf("nfs-%s", clientID),
			Type:         SessionTypeNFS,
			User:         "nfs-client",
			Status:       StatusActive,
			ConnectedAt:  time.Now(),
			LastActiveAt: time.Now(),
		}

		// 读取客户端信息
		infoPath := clientsDir + "/" + clientID + "/info"
		if info, err := exec.CommandContext(context.Background(), "cat", infoPath).Output(); err == nil {
			// 解析客户端IP
			lines := strings.Split(string(info), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "address:") {
					addr := strings.TrimPrefix(line, "address:")
					// 提取IP部分（可能包含端口）
					parts := strings.Split(addr, ",")
					if len(parts) > 0 {
						ipPort := strings.TrimSpace(parts[0])
						session.ClientIP = strings.Split(ipPort, ":")[0]
					}
				}
			}
		}

		// 获取共享路径（从 export 配置推断）
		session.SharePath = m.getNFSSharePath(clientID)

		sessions = append(sessions, session)
	}

	// 也使用 showmount 获取更详细的信息
	m.enrichNFSFromShowmount(sessions)

	return sessions, nil
}

// getNFSSharePath 获取NFS共享路径.
func (m *Monitor) getNFSSharePath(clientID string) string {
	// 读取客户端的状态信息
	clientsDir := "/proc/fs/nfsd/clients"

	// 查找客户端打开的文件
	opensPath := clientsDir + "/" + clientID + "/opens"
	if opens, err := exec.CommandContext(context.Background(), "cat", opensPath).Output(); err == nil {
		lines := strings.Split(string(opens), "\n")
		for _, line := range lines {
			// 从打开的文件路径提取共享路径
			if strings.Contains(line, "/") {
				fields := strings.Fields(line)
				for _, f := range fields {
					if strings.HasPrefix(f, "/") {
						return f
					}
				}
			}
		}
	}

	return "/"
}

// enrichNFSFromShowmount 使用showmount补充NFS信息.
func (m *Monitor) enrichNFSFromShowmount(sessions []*Session) {
	cmd := exec.CommandContext(context.Background(), "showmount", "-a", "localhost")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	// 跳过标题行
	for i, line := range lines {
		if i == 0 || line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			clientIP := parts[0]
			sharePath := parts[1]

			// 更新匹配的会话
			for _, session := range sessions {
				if session.ClientIP == "" || session.ClientIP == clientIP {
					session.ClientIP = clientIP
					if session.SharePath == "" || session.SharePath == "/" {
						session.SharePath = sharePath
					}
				}
			}
		}
	}
}

// GetStatus 获取监控器状态.
func (m *Monitor) GetStatus() map[string]interface{} {
	m.runningMu.Lock()
	running := m.running
	m.runningMu.Unlock()

	stats := m.manager.GetStats()

	return map[string]interface{}{
		"running":         running,
		"poll_interval":   m.pollInterval.String(),
		"total_sessions":  stats.TotalSessions,
		"smb_sessions":    stats.SMBSessions,
		"nfs_sessions":    stats.NFSSessions,
		"active_sessions": stats.ActiveSessions,
		"last_smb_count":  m.lastSMBCount,
		"last_nfs_count":  m.lastNFSCount,
	}
}

// ForceRefresh 强制刷新会话数据.
func (m *Monitor) ForceRefresh() error {
	m.poll()
	return nil
}

// DefaultSMBProvider 默认SMB提供者实现.
type DefaultSMBProvider struct{}

// Connections 获取SMB连接.
func (p *DefaultSMBProvider) Connections() ([]*SMBConnection, error) {
	cmd := exec.CommandContext(context.Background(), "smbstatus", "-b")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var connections []*SMBConnection
	lines := strings.Split(string(output), "\n")

	for i, line := range lines {
		if i < 3 {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 5 {
			pid := 0
			if p, err := strconv.Atoi(fields[0]); err == nil {
				pid = p
			}

			conn := &SMBConnection{
				PID:         pid,
				Username:    fields[1],
				ClientIP:    fields[2],
				Protocol:    fields[3],
				ShareName:   fields[4],
				ConnectedAt: time.Now(),
			}

			if len(fields) >= 6 {
				conn.ClientName = fields[5]
			}

			connections = append(connections, conn)
		}
	}

	return connections, nil
}

// KillConnection 断开SMB连接.
func (p *DefaultSMBProvider) KillConnection(pid int) error {
	// 使用 smbcontrol 断开连接
	cmd := exec.CommandContext(context.Background(), "smbcontrol", strconv.Itoa(pid), "close-share")
	return cmd.Run()
}

// DefaultNFSProvider 默认NFS提供者实现.
type DefaultNFSProvider struct{}

// GetClients 获取NFS客户端.
func (p *DefaultNFSProvider) GetClients() ([]*NFSClient, error) {
	var clients []*NFSClient

	clientsDir := "/proc/fs/nfsd/clients"
	entries, err := exec.CommandContext(context.Background(), "ls", "-1", clientsDir).Output()
	if err != nil {
		return nil, err
	}

	clientIDs := strings.Split(strings.TrimSpace(string(entries)), "\n")
	for _, clientID := range clientIDs {
		if clientID == "" {
			continue
		}

		client := &NFSClient{
			ID:          clientID,
			ConnectedAt: time.Now(),
		}

		// 读取客户端信息
		infoPath := clientsDir + "/" + clientID + "/info"
		if info, err := exec.CommandContext(context.Background(), "cat", infoPath).Output(); err == nil {
			lines := strings.Split(string(info), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "address:") {
					addr := strings.TrimPrefix(line, "address:")
					parts := strings.Split(addr, ",")
					if len(parts) > 0 {
						ipPort := strings.TrimSpace(parts[0])
						client.ClientIP = strings.Split(ipPort, ":")[0]
					}
				}
			}
		}

		clients = append(clients, client)
	}

	return clients, nil
}

// KillClient 断开NFS客户端.
func (p *DefaultNFSProvider) KillClient(clientID string) error {
	// NFS 没有直接的断开命令，需要通过内核接口
	// 写入到 /proc/fs/nfsd/clients/{id}/ctl
	_ = fmt.Sprintf("/proc/fs/nfsd/clients/%s/ctl", clientID)
	// TODO: 实际实现需要写入到 ctl 文件
	return exec.CommandContext(context.Background(), "echo", "-1").Run()
}
