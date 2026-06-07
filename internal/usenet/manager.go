// Package usenet 提供 Usenet 下载管理核心业务逻辑
package usenet

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager Usenet 下载管理器
type Manager struct {
	mu         sync.RWMutex
	servers    map[string]*Server
	nzbs       map[string]*NZB
	downloads  map[string]*Download
	categories map[string]*Category
	indexers   map[string]*Indexer
	queue      []*QueueItem
}

// NewManager 创建 Usenet 下载管理器
func NewManager() *Manager {
	m := &Manager{
		servers:    make(map[string]*Server),
		nzbs:       make(map[string]*NZB),
		downloads:  make(map[string]*Download),
		categories: make(map[string]*Category),
		indexers:   make(map[string]*Indexer),
		queue:      make([]*QueueItem, 0),
	}

	// 初始化预置服务器
	m.initDefaultServers()

	return m
}

// initDefaultServers 初始化预置服务器
func (m *Manager) initDefaultServers() {
	for _, s := range DefaultServers {
		server := s
		m.servers[server.ID] = &server
	}
}

// generateID 生成唯一 ID
func generateID() string {
	return uuid.New().String()
}

// AddServer 添加 Usenet 服务器
func (m *Manager) AddServer(server *Server) (*Server, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if server.Host == "" {
		return nil, fmt.Errorf("服务器主机不能为空")
	}
	if server.Port <= 0 || server.Port > 65535 {
		return nil, fmt.Errorf("服务器端口无效: %d", server.Port)
	}
	if server.Connections <= 0 {
		return nil, fmt.Errorf("连接数必须大于 0")
	}

	server.ID = generateID()
	if server.RetentionDays <= 0 {
		server.RetentionDays = 30
	}

	m.servers[server.ID] = server
	return server, nil
}

// UpdateServer 更新 Usenet 服务器配置
func (m *Manager) UpdateServer(id string, server *Server) (*Server, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.servers[id]
	if !ok {
		return nil, fmt.Errorf("服务器不存在: %s", id)
	}

	if server.Host != "" {
		existing.Host = server.Host
	}
	if server.Port > 0 {
		existing.Port = server.Port
	}
	if server.Connections > 0 {
		existing.Connections = server.Connections
	}
	if server.Username != "" {
		existing.Username = server.Username
	}
	if server.Password != "" {
		existing.Password = server.Password
	}
	existing.SSL = server.SSL
	existing.Enabled = server.Enabled
	existing.Priority = server.Priority
	if server.RetentionDays > 0 {
		existing.RetentionDays = server.RetentionDays
	}

	return existing, nil
}

// DeleteServer 删除 Usenet 服务器
func (m *Manager) DeleteServer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.servers[id]; !ok {
		return fmt.Errorf("服务器不存在: %s", id)
	}

	delete(m.servers, id)
	return nil
}

// TestServer 测试服务器连接
func (m *Manager) TestServer(id string) (time.Duration, error) {
	m.mu.RLock()
	server, ok := m.servers[id]
	m.mu.RUnlock()

	if !ok {
		return 0, fmt.Errorf("服务器不存在: %s", id)
	}

	// 模拟连接测试
	start := time.Now()
	time.Sleep(time.Millisecond * 100) // 模拟网络延迟
	duration := time.Since(start)

	_ = server // 使用服务器配置进行实际测试

	return duration, nil
}

// GetServer 获取服务器配置
func (m *Manager) GetServer(id string) (*Server, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	server, ok := m.servers[id]
	if !ok {
		return nil, fmt.Errorf("服务器不存在: %s", id)
	}
	return server, nil
}

// ListServers 列出所有服务器
func (m *Manager) ListServers() []*Server {
	m.mu.RLock()
	defer m.mu.RUnlock()

	servers := make([]*Server, 0, len(m.servers))
	for _, s := range m.servers {
		servers = append(servers, s)
	}
	return servers
}

// AddNZB 添加 NZB
func (m *Manager) AddNZB(nzb *NZB) (*NZB, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if nzb.Name == "" {
		return nil, fmt.Errorf("NZB 名称不能为空")
	}

	nzb.ID = generateID()
	nzb.AddedAt = time.Now()
	if nzb.Status == "" {
		nzb.Status = NZBStatusPending
	}

	m.nzbs[nzb.ID] = nzb
	return nzb, nil
}

// AddNZBFromFile 从文件添加 NZB
func (m *Manager) AddNZBFromFile(path string) (*NZB, error) {
	// 检查文件是否存在
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("文件不存在: %s", path)
	}

	// 创建 NZB 记录
	nzb := &NZB{
		Name:     info.Name(),
		Size:     info.Size(),
		FilePath: path,
		Status:   NZBStatusPending,
		AddedAt:  time.Now(),
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	nzb.ID = generateID()
	m.nzbs[nzb.ID] = nzb
	return nzb, nil
}

// GetNZB 获取 NZB 信息
func (m *Manager) GetNZB(id string) (*NZB, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nzb, ok := m.nzbs[id]
	if !ok {
		return nil, fmt.Errorf("NZB 不存在: %s", id)
	}
	return nzb, nil
}

// ListNZBs 列出 NZB，可按状态过滤
func (m *Manager) ListNZBs(status NZBStatus) ([]NZB, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []NZB
	for _, nzb := range m.nzbs {
		if status == "" || nzb.Status == status {
			result = append(result, *nzb)
		}
	}
	return result, nil
}

// DeleteNZB 删除 NZB
func (m *Manager) DeleteNZB(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.nzbs[id]; !ok {
		return fmt.Errorf("NZB 不存在: %s", id)
	}

	// 同时删除相关的下载任务
	for dlID, dl := range m.downloads {
		if dl.NZBID == id {
			delete(m.downloads, dlID)
		}
	}

	// 从队列中移除
	newQueue := make([]*QueueItem, 0, len(m.queue))
	for _, item := range m.queue {
		if item.NZBID != id {
			newQueue = append(newQueue, item)
		}
	}
	m.queue = newQueue

	delete(m.nzbs, id)
	return nil
}

// StartDownload 启动下载
func (m *Manager) StartDownload(nzbID string) (*Download, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	nzb, ok := m.nzbs[nzbID]
	if !ok {
		return nil, fmt.Errorf("NZB 不存在: %s", nzbID)
	}

	// 检查是否已有下载任务
	for _, dl := range m.downloads {
		if dl.NZBID == nzbID && dl.Status == DownloadStatusActive {
			return nil, fmt.Errorf("该 NZB 已有活跃的下载任务")
		}
	}

	// 找到一个可用的服务器
	var server *Server
	for _, s := range m.servers {
		if s.Enabled {
			server = s
			break
		}
	}
	if server == nil {
		return nil, fmt.Errorf("没有可用的服务器")
	}

	// 创建下载任务
	download := &Download{
		ID:          generateID(),
		NZBID:       nzbID,
		Size:        nzb.Size,
		Status:      DownloadStatusActive,
		Connections: server.Connections,
		Server:      server.ID,
		StartedAt:   time.Now(),
	}

	m.downloads[download.ID] = download

	// 更新 NZB 状态
	nzb.Status = NZBStatusDownloading

	return download, nil
}

// PauseDownload 暂停下载
func (m *Manager) PauseDownload(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dl, ok := m.downloads[id]
	if !ok {
		return fmt.Errorf("下载任务不存在: %s", id)
	}

	if dl.Status != DownloadStatusActive {
		return fmt.Errorf("只能暂停活跃的下载任务")
	}

	dl.Status = DownloadStatusPaused

	// 更新 NZB 状态
	if nzb, ok := m.nzbs[dl.NZBID]; ok {
		nzb.Status = NZBStatusPaused
	}

	return nil
}

// ResumeDownload 恢复下载
func (m *Manager) ResumeDownload(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dl, ok := m.downloads[id]
	if !ok {
		return fmt.Errorf("下载任务不存在: %s", id)
	}

	if dl.Status != DownloadStatusPaused {
		return fmt.Errorf("只能恢复暂停的下载任务")
	}

	dl.Status = DownloadStatusActive

	// 更新 NZB 状态
	if nzb, ok := m.nzbs[dl.NZBID]; ok {
		nzb.Status = NZBStatusDownloading
	}

	return nil
}

// CancelDownload 取消下载
func (m *Manager) CancelDownload(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dl, ok := m.downloads[id]
	if !ok {
		return fmt.Errorf("下载任务不存在: %s", id)
	}

	if dl.Status == DownloadStatusCompleted || dl.Status == DownloadStatusCancelled {
		return fmt.Errorf("已完成或已取消的任务不能取消")
	}

	dl.Status = DownloadStatusCancelled
	dl.CompletedAt = time.Now()

	// 更新 NZB 状态
	if nzb, ok := m.nzbs[dl.NZBID]; ok {
		nzb.Status = NZBStatusFailed
	}

	return nil
}

// GetDownload 获取下载任务信息
func (m *Manager) GetDownload(id string) (*Download, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dl, ok := m.downloads[id]
	if !ok {
		return nil, fmt.Errorf("下载任务不存在: %s", id)
	}
	return dl, nil
}

// ListDownloads 列出所有下载任务
func (m *Manager) ListDownloads() ([]Download, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	downloads := make([]Download, 0, len(m.downloads))
	for _, dl := range m.downloads {
		downloads = append(downloads, *dl)
	}
	return downloads, nil
}

// GetQueue 获取下载队列
func (m *Manager) GetQueue() ([]QueueItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	queue := make([]QueueItem, len(m.queue))
	for i, item := range m.queue {
		queue[i] = *item
	}
	return queue, nil
}

// ReorderQueue 重新排序队列
func (m *Manager) ReorderQueue(ids []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(ids) != len(m.queue) {
		return fmt.Errorf("队列长度不匹配")
	}

	newQueue := make([]*QueueItem, len(ids))
	for i, id := range ids {
		found := false
		for _, item := range m.queue {
			if item.ID == id {
				newItem := *item
				newItem.Position = i
				newQueue[i] = &newItem
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("队列项不存在: %s", id)
		}
	}

	m.queue = newQueue
	return nil
}

// AddIndexer 添加索引器
func (m *Manager) AddIndexer(indexer *Indexer) (*Indexer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if indexer.Name == "" {
		return nil, fmt.Errorf("索引器名称不能为空")
	}
	if indexer.URL == "" {
		return nil, fmt.Errorf("索引器 URL 不能为空")
	}

	indexer.ID = generateID()
	m.indexers[indexer.ID] = indexer
	return indexer, nil
}

// SearchIndexer 搜索索引器
func (m *Manager) SearchIndexer(indexerID, query string) ([]NZB, error) {
	m.mu.RLock()
	indexer, ok := m.indexers[indexerID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("索引器不存在: %s", indexerID)
	}

	if !indexer.Enabled {
		return nil, fmt.Errorf("索引器已禁用: %s", indexerID)
	}

	// 模拟搜索结果
	results := []NZB{
		{
			ID:      generateID(),
			Name:    fmt.Sprintf("搜索结果: %s", query),
			Size:    1024 * 1024 * 100, // 100MB
			Files:   10,
			Status:  NZBStatusPending,
			AddedAt: time.Now(),
		},
	}

	return results, nil
}

// ListIndexers 列出所有索引器
func (m *Manager) ListIndexers() []*Indexer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	indexers := make([]*Indexer, 0, len(m.indexers))
	for _, idx := range m.indexers {
		indexers = append(indexers, idx)
	}
	return indexers
}

// GetIndexer 获取索引器
func (m *Manager) GetIndexer(id string) (*Indexer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	idx, ok := m.indexers[id]
	if !ok {
		return nil, fmt.Errorf("索引器不存在: %s", id)
	}
	return idx, nil
}

// DeleteIndexer 删除索引器
func (m *Manager) DeleteIndexer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.indexers[id]; !ok {
		return fmt.Errorf("索引器不存在: %s", id)
	}
	delete(m.indexers, id)
	return nil
}

// CreateCategory 创建分类
func (m *Manager) CreateCategory(cat *Category) (*Category, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cat.Name == "" {
		return nil, fmt.Errorf("分类名称不能为空")
	}
	if cat.DestPath == "" {
		return nil, fmt.Errorf("目标路径不能为空")
	}

	cat.ID = generateID()
	m.categories[cat.ID] = cat
	return cat, nil
}

// ListCategories 列出所有分类
func (m *Manager) ListCategories() ([]Category, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cats := make([]Category, 0, len(m.categories))
	for _, cat := range m.categories {
		cats = append(cats, *cat)
	}
	return cats, nil
}

// GetCategory 获取分类
func (m *Manager) GetCategory(id string) (*Category, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cat, ok := m.categories[id]
	if !ok {
		return nil, fmt.Errorf("分类不存在: %s", id)
	}
	return cat, nil
}

// DeleteCategory 删除分类
func (m *Manager) DeleteCategory(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.categories[id]; !ok {
		return fmt.Errorf("分类不存在: %s", id)
	}
	delete(m.categories, id)
	return nil
}

// GetStats 获取统计信息
func (m *Manager) GetStats() (*Stats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &Stats{
		ServerStats: make([]ServerStats, 0),
	}

	for _, dl := range m.downloads {
		stats.TotalSize += dl.Size
		stats.TotalDownloaded += dl.Downloaded
		if dl.Status == DownloadStatusActive {
			stats.ActiveDownloads++
			stats.CurrentSpeed += dl.Speed
		}
	}

	stats.QueuedItems = len(m.queue)

	// 收集服务器统计
	for _, s := range m.servers {
		serverStats := ServerStats{
			ServerID:   s.ID,
			ServerHost: s.Host,
		}
		for _, dl := range m.downloads {
			if dl.Server == s.ID && dl.Status == DownloadStatusActive {
				serverStats.ConnectionsUsed += dl.Connections
				serverStats.CurrentSpeed += dl.Speed
				serverStats.TotalDownloaded += dl.Downloaded
			}
		}
		stats.ServerStats = append(stats.ServerStats, serverStats)
	}

	return stats, nil
}

// ProcessCompleted 处理已完成的下载
func (m *Manager) ProcessCompleted(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dl, ok := m.downloads[id]
	if !ok {
		return fmt.Errorf("下载任务不存在: %s", id)
	}

	if dl.Status != DownloadStatusCompleted {
		return fmt.Errorf("下载任务未完成")
	}

	// 更新 NZB 状态
	if nzb, ok := m.nzbs[dl.NZBID]; ok {
		nzb.Status = NZBStatusCompleted
	}

	// 从队列中移除
	newQueue := make([]*QueueItem, 0, len(m.queue))
	for _, item := range m.queue {
		if item.NZBID != dl.NZBID {
			newQueue = append(newQueue, item)
		}
	}
	m.queue = newQueue

	return nil
}
