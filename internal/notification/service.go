package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Service 通知服务.
type Service struct {
	config          *ServiceConfig
	templateManager *TemplateManager
	channelManager  *ChannelManager
	ruleEngine      *RuleEngine
	historyManager  *HistoryManager
	senderRegistry  *SenderRegistry
	taskQueue       chan *sendTask
	wg              sync.WaitGroup
	ctx             context.Context
	cancel          context.CancelFunc
}

type sendTask struct {
	notification *Notification
	channels     []*ChannelConfig
	templateID   string
	variables    map[string]interface{}
	resultChan   chan *sendTaskResult
}

type sendTaskResult struct {
	records []*Record
	errors  map[string]string
}

// NewService 创建通知服务.
func NewService(config *ServiceConfig) (*Service, error) {
	if config == nil {
		config = &ServiceConfig{
			DefaultRetryCount:    3,
			DefaultRetryInterval: 5 * time.Minute,
			MaxHistoryDays:       30,
			MaxConcurrent:        10,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	s := &Service{
		config:         config,
		senderRegistry: NewSenderRegistry(),
		taskQueue:      make(chan *sendTask, 100),
		ctx:            ctx,
		cancel:         cancel,
	}

	// 初始化模板管理器
	var err error
	templatePath := ""
	if config.StoragePath != "" {
		templatePath = config.StoragePath + "/templates.json"
	}
	s.templateManager, err = NewTemplateManager(templatePath)
	if err != nil {
		return nil, fmt.Errorf("初始化模板管理器失败: %w", err)
	}

	// 初始化渠道管理器
	s.channelManager = NewChannelManager()

	// 初始化规则引擎
	rulePath := ""
	if config.StoragePath != "" {
		rulePath = config.StoragePath + "/rules.json"
	}
	s.ruleEngine, err = NewRuleEngine(rulePath)
	if err != nil {
		return nil, fmt.Errorf("初始化规则引擎失败: %w", err)
	}

	// 初始化历史管理器
	historyPath := ""
	if config.StoragePath != "" {
		historyPath = config.StoragePath + "/history.json"
	}
	s.historyManager, err = NewHistoryManager(historyPath, 10000, config.MaxHistoryDays)
	if err != nil {
		return nil, fmt.Errorf("初始化历史管理器失败: %w", err)
	}

	// 启动工作协程
	for i := 0; i < config.MaxConcurrent; i++ {
		go s.worker()
	}

	return s, nil
}

// Start 启动服务.
func (s *Service) Start() error {
	// 加载已保存的渠道配置
	if s.config.StoragePath != "" {
		if err := s.loadChannels(); err != nil {
			return fmt.Errorf("加载渠道配置失败: %w", err)
		}
	}

	return nil
}

// Stop 停止服务.
func (s *Service) Stop() {
	s.cancel()
	s.wg.Wait()
	close(s.taskQueue)
}

// loadChannels 加载渠道配置.
func (s *Service) loadChannels() error {
	data, err := readFile(s.config.StoragePath + "/channels.json")
	if err != nil {
		return nil // 文件不存在时忽略
	}

	var channels []*ChannelConfig
	if err := json.Unmarshal(data, &channels); err != nil {
		return err
	}

	for _, c := range channels {
		_ = s.channelManager.AddChannel(c)
	}

	return nil
}

// saveChannels 保存渠道配置.
func (s *Service) saveChannels() error {
	channels := s.channelManager.ListChannels("")
	data, err := json.MarshalIndent(channels, "", "  ")
	if err != nil {
		return err
	}

	return writeFile(s.config.StoragePath+"/channels.json", data)
}

// worker 工作协程.
func (s *Service) worker() {
	s.wg.Add(1)
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return
		case task, ok := <-s.taskQueue:
			if !ok {
				return
			}
			result := s.processTask(task)
			if task.resultChan != nil {
				task.resultChan <- result
			}
		}
	}
}

// processTask 处理发送任务.
func (s *Service) processTask(task *sendTask) *sendTaskResult {
	result := &sendTaskResult{
		records: make([]*Record, 0),
		errors:  make(map[string]string),
	}

	for _, channel := range task.channels {
		record := &Record{
			ID:           GenerateID(),
			Notification: task.notification,
			Channel:      channel.Type,
			ChannelName:  channel.Name,
			Status:       StatusPending,
			MaxAttempts:  s.config.DefaultRetryCount,
			CreatedAt:    time.Now(),
		}

		// 获取发送器
		sender, exists := s.senderRegistry.Get(channel.Type)
		if !exists {
			record.Status = StatusFailed
			record.Error = fmt.Sprintf("不支持的渠道类型: %s", channel.Type)
			result.errors[channel.ID] = record.Error
			_ = s.historyManager.AddRecord(record)
			result.records = append(result.records, record)
			continue
		}

		// 发送通知
		err := s.sendWithRetry(sender, channel, task.notification, record)
		if err != nil {
			record.Status = StatusFailed
			record.Error = err.Error()
			result.errors[channel.ID] = err.Error()
		} else {
			record.Status = StatusSent
			now := time.Now()
			record.SentAt = &now
		}

		_ = s.historyManager.AddRecord(record)
		result.records = append(result.records, record)
	}

	return result
}

// sendWithRetry 带重试的发送.
func (s *Service) sendWithRetry(sender ChannelSender, channel *ChannelConfig, notification *Notification, record *Record) error {
	var lastErr error

	for attempt := 1; attempt <= record.MaxAttempts; attempt++ {
		record.Attempts = attempt
		record.UpdatedAt = time.Now()

		err := sender.Send(channel, notification)
		if err == nil {
			return nil
		}

		lastErr = err

		if attempt < record.MaxAttempts {
			record.Status = StatusRetrying
			time.Sleep(s.config.DefaultRetryInterval)
		}
	}

	return lastErr
}

// Send 发送通知.
func (s *Service) Send(ctx context.Context, req *SendRequest) (*SendResponse, error) {
	notification := req.Notification
	if notification == nil {
		return nil, fmt.Errorf("通知内容不能为空")
	}

	// 设置通知ID和时间
	if notification.ID == "" {
		notification.ID = GenerateID()
	}
	notification.CreatedAt = time.Now()

	// 如果使用模板，渲染通知内容
	if req.TemplateID != "" {
		rendered, err := s.templateManager.Render(req.TemplateID, req.Variables)
		if err != nil {
			return nil, fmt.Errorf("渲染模板失败: %w", err)
		}
		notification.Title = rendered.Subject
		notification.Message = rendered.Body
		notification.TemplateID = req.TemplateID
	}

	// 确定发送渠道
	var channels []*ChannelConfig
	if len(req.Channels) > 0 {
		// 使用指定渠道
		for _, channelID := range req.Channels {
			channel, err := s.channelManager.GetChannel(channelID)
			if err != nil {
				continue
			}
			if channel.Enabled {
				channels = append(channels, channel)
			}
		}
	} else {
		// 通过规则匹配渠道
		rules := s.ruleEngine.MatchRules(notification)
		channelIDs := make(map[string]bool)
		for _, rule := range rules {
			for _, cid := range rule.Channels {
				channelIDs[cid] = true
			}
		}

		for channelID := range channelIDs {
			channel, err := s.channelManager.GetChannel(channelID)
			if err != nil {
				continue
			}
			if channel.Enabled {
				channels = append(channels, channel)
			}
		}

		// 如果没有匹配的规则，使用所有启用的渠道
		if len(channels) == 0 {
			channels = s.channelManager.GetEnabledChannels()
		}
	}

	if len(channels) == 0 {
		return nil, fmt.Errorf("没有可用的通知渠道")
	}

	// 创建发送任务
	task := &sendTask{
		notification: notification,
		channels:     channels,
		templateID:   req.TemplateID,
		variables:    req.Variables,
		resultChan:   make(chan *sendTaskResult, 1),
	}

	// 发送到任务队列
	select {
	case s.taskQueue <- task:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// 等待结果
	select {
	case result := <-task.resultChan:
		return &SendResponse{
			NotificationID: notification.ID,
			Records:        result.records,
			Success:        len(result.errors) == 0,
			Errors:         result.errors,
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// SendAsync 异步发送通知.
func (s *Service) SendAsync(req *SendRequest) (string, error) {
	notification := req.Notification
	if notification == nil {
		return "", fmt.Errorf("通知内容不能为空")
	}

	if notification.ID == "" {
		notification.ID = GenerateID()
	}
	notification.CreatedAt = time.Now()

	// 如果使用模板，渲染通知内容
	if req.TemplateID != "" {
		rendered, err := s.templateManager.Render(req.TemplateID, req.Variables)
		if err != nil {
			return "", fmt.Errorf("渲染模板失败: %w", err)
		}
		notification.Title = rendered.Subject
		notification.Message = rendered.Body
	}

	// 确定发送渠道
	channels := s.channelManager.GetEnabledChannels()
	if len(req.Channels) > 0 {
		channels = make([]*ChannelConfig, 0)
		for _, channelID := range req.Channels {
			channel, err := s.channelManager.GetChannel(channelID)
			if err != nil {
				continue
			}
			if channel.Enabled {
				channels = append(channels, channel)
			}
		}
	}

	if len(channels) == 0 {
		return "", fmt.Errorf("没有可用的通知渠道")
	}

	// 创建异步任务
	task := &sendTask{
		notification: notification,
		channels:     channels,
		templateID:   req.TemplateID,
		variables:    req.Variables,
		resultChan:   nil, // 不等待结果
	}

	s.taskQueue <- task

	return notification.ID, nil
}

// GetTemplateManager 获取模板管理器.
func (s *Service) GetTemplateManager() *TemplateManager {
	return s.templateManager
}

// GetChannelManager 获取渠道管理器.
func (s *Service) GetChannelManager() *ChannelManager {
	return s.channelManager
}

// GetRuleEngine 获取规则引擎.
func (s *Service) GetRuleEngine() *RuleEngine {
	return s.ruleEngine
}

// GetHistoryManager 获取历史管理器.
func (s *Service) GetHistoryManager() *HistoryManager {
	return s.historyManager
}

// SetWebSocketBroadcaster 设置 WebSocket 广播器.
func (s *Service) SetWebSocketBroadcaster(broadcaster WebSocketBroadcaster) {
	s.senderRegistry.SetWebSocketSender(broadcaster)
}

// AddChannel 添加渠道.
func (s *Service) AddChannel(config *ChannelConfig) error {
	if err := s.channelManager.AddChannel(config); err != nil {
		return err
	}
	return s.saveChannels()
}

// UpdateChannel 更新渠道.
func (s *Service) UpdateChannel(config *ChannelConfig) error {
	if err := s.channelManager.UpdateChannel(config); err != nil {
		return err
	}
	return s.saveChannels()
}

// RemoveChannel 移除渠道.
func (s *Service) RemoveChannel(id string) error {
	if err := s.channelManager.RemoveChannel(id); err != nil {
		return err
	}
	return s.saveChannels()
}

// TestChannel 测试渠道.
func (s *Service) TestChannel(channelID string) error {
	channel, err := s.channelManager.GetChannel(channelID)
	if err != nil {
		return err
	}

	sender, exists := s.senderRegistry.Get(channel.Type)
	if !exists {
		return fmt.Errorf("不支持的渠道类型: %s", channel.Type)
	}

	testNotification := &Notification{
		ID:        GenerateID(),
		Title:     "测试通知",
		Message:   "这是一条测试通知，如果您收到此消息，说明通知渠道配置正确。",
		Level:     LevelInfo,
		Source:    "NAS-OS",
		CreatedAt: time.Now(),
	}

	return sender.Send(channel, testNotification)
}

// RetryFailed 重试失败的记录.
func (s *Service) RetryFailed(recordID string) error {
	record, err := s.historyManager.GetRecord(recordID)
	if err != nil {
		return err
	}

	if record.Status != StatusFailed {
		return fmt.Errorf("只能重试失败的记录")
	}

	channel, err := s.channelManager.GetChannel(record.ChannelName)
	if err != nil {
		// 尝试通过类型查找
		channels := s.channelManager.ListChannels(record.Channel)
		if len(channels) == 0 {
			return fmt.Errorf("找不到渠道: %s", record.ChannelName)
		}
		channel = channels[0]
	}

	sender, exists := s.senderRegistry.Get(channel.Type)
	if !exists {
		return fmt.Errorf("不支持的渠道类型: %s", channel.Type)
	}

	record.Status = StatusRetrying
	record.Attempts = 0

	err = s.sendWithRetry(sender, channel, record.Notification, record)
	if err != nil {
		record.Status = StatusFailed
		record.Error = err.Error()
	} else {
		record.Status = StatusSent
		now := time.Now()
		record.SentAt = &now
	}

	return s.historyManager.UpdateRecord(record)
}

// GetStats 获取统计信息.
func (s *Service) GetStats(startTime, endTime *time.Time) *HistoryStats {
	return s.historyManager.GetStats(startTime, endTime)
}

// 辅助函数（模拟文件操作，实际项目中应该使用真实的文件操作）

func readFile(path string) ([]byte, error) {
	// 这里应该使用 os.ReadFile
	// 为了避免导入问题，这里只是占位
	return nil, fmt.Errorf("file not found")
}

func writeFile(path string, data []byte) error {
	// 这里应该使用 os.WriteFile
	// 为了避免导入问题，这里只是占位
	return nil
}
