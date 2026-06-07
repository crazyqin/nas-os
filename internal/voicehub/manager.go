// Package voicehub 提供语音助手核心管理逻辑
package voicehub

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 语音助手管理器
type Manager struct {
	mu              sync.RWMutex
	logger          *zap.Logger
	config          *VoiceHubConfig
	commands        map[string]*CustomCommand
	scenes          map[string]*SceneInfo
	templates       map[string]*ReplyTemplate
	wakeWords       map[string]*WakeWordConfig
	history         []*CommandHistory
	sessionContexts map[string]*SessionContext
	stopChan        chan struct{}
	running         bool
}

// SessionContext 会话上下文
type SessionContext struct {
	SessionID  string                 `json:"session_id"`
	Platform   VoicePlatform          `json:"platform"`
	Language   VoiceLanguage          `json:"language"`
	LastActive time.Time              `json:"last_active"`
	Context    map[string]interface{} `json:"context,omitempty"`
}

// NewManager 创建语音助手管理器
func NewManager(logger *zap.Logger, config *VoiceHubConfig) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultVoiceHubConfig()
	}

	m := &Manager{
		logger:          logger,
		config:          config,
		commands:        make(map[string]*CustomCommand),
		scenes:          make(map[string]*SceneInfo),
		templates:       make(map[string]*ReplyTemplate),
		wakeWords:       make(map[string]*WakeWordConfig),
		history:         make([]*CommandHistory, 0),
		sessionContexts: make(map[string]*SessionContext),
		stopChan:        make(chan struct{}),
	}

	// 初始化默认场景
	m.initDefaultScenes()
	// 初始化默认模板
	m.initDefaultTemplates()
	// 初始化唤醒词
	m.initWakeWords()

	return m
}

// generateID 生成唯一 ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// initDefaultScenes 初始化默认场景
func (m *Manager) initDefaultScenes() {
	defaultScenes := []*SceneInfo{
		{
			ID:          "scene-movie",
			Name:        "观影模式",
			Type:        SceneMovieMode,
			Description: "调暗灯光、关闭窗帘、打开投影仪、设置音响环绕声",
			Devices: []DeviceAction{
				{DeviceID: "light-001", DeviceName: "客厅灯光", CommandType: CmdSetVolume, Parameters: map[string]interface{}{"brightness": 20}},
				{DeviceID: "curtain-001", DeviceName: "窗帘", CommandType: CmdPowerOff},
				{DeviceID: "projector-001", DeviceName: "投影仪", CommandType: CmdPowerOn},
				{DeviceID: "speaker-001", DeviceName: "音响", CommandType: CmdPowerOn},
			},
			IsActive:  true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "scene-security",
			Name:        "安防模式",
			Type:        SceneSecurityMode,
			Description: "开启监控摄像头、锁定门窗、启动报警系统",
			Devices: []DeviceAction{
				{DeviceID: "camera-001", DeviceName: "前门摄像头", CommandType: CmdPowerOn},
				{DeviceID: "camera-002", DeviceName: "后门摄像头", CommandType: CmdPowerOn},
				{DeviceID: "lock-001", DeviceName: "大门智能锁", CommandType: CmdPowerOn},
				{DeviceID: "alarm-001", DeviceName: "报警器", CommandType: CmdPowerOn},
			},
			IsActive:  true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "scene-energy",
			Name:        "节能模式",
			Type:        SceneEnergySaving,
			Description: "关闭非必要设备、降低空调温度、关闭待机设备",
			Devices: []DeviceAction{
				{DeviceID: "ac-001", DeviceName: "空调", CommandType: CmdSetVolume, Parameters: map[string]interface{}{"temperature": 26}},
				{DeviceID: "light-001", DeviceName: "客厅灯光", CommandType: CmdPowerOff},
				{DeviceID: "tv-001", DeviceName: "电视", CommandType: CmdPowerOff},
			},
			IsActive:  true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "scene-sleep",
			Name:        "睡眠模式",
			Type:        SceneSleepMode,
			Description: "关闭所有灯光、设置空调睡眠、开启勿扰",
			Devices: []DeviceAction{
				{DeviceID: "light-001", DeviceName: "所有灯光", CommandType: CmdPowerOff},
				{DeviceID: "ac-001", DeviceName: "空调", CommandType: CmdSetVolume, Parameters: map[string]interface{}{"mode": "sleep"}},
			},
			IsActive:  true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "scene-party",
			Name:        "派对模式",
			Type:        ScenePartyMode,
			Description: "开启彩色灯光、播放音乐、设置音响音量",
			Devices: []DeviceAction{
				{DeviceID: "light-001", DeviceName: "彩色灯光", CommandType: CmdPowerOn},
				{DeviceID: "speaker-001", DeviceName: "音响", CommandType: CmdPlay},
			},
			IsActive:  true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	for _, s := range defaultScenes {
		m.scenes[s.ID] = s
	}
}

// initDefaultTemplates 初始化默认回复模板
func (m *Manager) initDefaultTemplates() {
	defaultTemplates := []*ReplyTemplate{
		{
			ID:        "tpl-greeting-zh",
			Name:      "问候语-中文",
			Language:  LangChinese,
			Template:  "你好！我是智能语音助手，有什么可以帮您的吗？",
			Category:  "greeting",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "tpl-greeting-en",
			Name:      "greeting-english",
			Language:  LangEnglish,
			Template:  "Hello! I'm your voice assistant. How can I help you?",
			Category:  "greeting",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "tpl-greeting-ja",
			Name:      "greeting-japanese",
			Language:  LangJapanese,
			Template:  "こんにちは！音声アシスタントです。何かお手伝いできますか？",
			Category:  "greeting",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "tpl-scene-activated",
			Name:      "场景激活",
			Language:  LangChinese,
			Template:  "已为您激活{{scene_name}}",
			Category:  "scene",
			Variables: []string{"scene_name"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "tpl-command-success",
			Name:      "命令成功",
			Language:  LangChinese,
			Template:  "已执行{{device_name}}的{{command}}操作",
			Category:  "command",
			Variables: []string{"device_name", "command"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "tpl-command-failed",
			Name:      "命令失败",
			Language:  LangChinese,
			Template:  "抱歉，{{device_name}}的{{command}}操作失败：{{error}}",
			Category:  "command",
			Variables: []string{"device_name", "command", "error"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	for _, t := range defaultTemplates {
		m.templates[t.ID] = t
	}
}

// initWakeWords 初始化唤醒词
func (m *Manager) initWakeWords() {
	for _, w := range m.config.WakeWords {
		m.wakeWords[w.ID] = &w
	}
}

// ProcessVoiceCommand 处理语音命令
func (m *Manager) ProcessVoiceCommand(ctx context.Context, cmd *VoiceCommand) (*VoiceResponse, error) {
	if !m.config.Enabled {
		return nil, fmt.Errorf("voice assistant is disabled")
	}

	start := time.Now()
	respID := generateID()

	// 设置默认值
	if cmd.Language == "" {
		cmd.Language = m.config.DefaultLanguage
	}
	if cmd.Platform == "" {
		cmd.Platform = m.config.DefaultPlatform
	}
	if cmd.Timestamp.IsZero() {
		cmd.Timestamp = start
	}
	if cmd.ID == "" {
		cmd.ID = generateID()
	}

	// 更新会话上下文
	m.updateSessionContext(cmd)

	response := &VoiceResponse{
		ID:        respID,
		CommandID: cmd.ID,
		Status:    CommandStatusProcessing,
		CreatedAt: start,
	}

	// 解析语音命令
	intent := m.parseIntent(cmd.Query, cmd.Language)

	// 根据意图执行操作
	var err error
	switch intent.Type {
	case "scene":
		err = m.handleSceneCommand(ctx, intent, response)
	case "device":
		err = m.handleDeviceCommand(ctx, intent, response)
	case "tts":
		err = m.handleTTSCommand(ctx, intent, response)
	case "custom":
		err = m.handleCustomCommand(ctx, intent, cmd, response)
	default:
		err = m.handleGeneralCommand(ctx, intent, cmd, response)
	}

	response.Duration = time.Since(start)

	if err != nil {
		response.Status = CommandStatusFailed
		response.TextReply = m.getLocalizedError(err.Error(), cmd.Language)
		m.logger.Error("voice command failed",
			zap.String("id", cmd.ID),
			zap.String("query", cmd.Query),
			zap.Error(err))
	} else {
		response.Status = CommandStatusSuccess
	}

	// 记录历史
	history := &CommandHistory{
		ID:        generateID(),
		Command:   *cmd,
		Response:  response,
		Success:   err == nil,
		CreatedAt: start,
	}
	if err != nil {
		history.Error = err.Error()
	}
	m.addHistory(history)

	return response, nil
}

// Intent 意图解析结果
type Intent struct {
	Type     string                 `json:"type"`
	Action   string                 `json:"action"`
	Target   string                 `json:"target"`
	Params   map[string]interface{} `json:"params,omitempty"`
	Original string                 `json:"original"`
}

// parseIntent 解析语音命令意图
func (m *Manager) parseIntent(query string, lang VoiceLanguage) *Intent {
	query = strings.TrimSpace(query)
	queryLower := strings.ToLower(query)

	intent := &Intent{
		Original: query,
		Params:   make(map[string]interface{}),
	}

	// 场景匹配
	sceneKeywords := map[string]string{
		"观影模式": "scene-movie", "movie mode": "scene-movie", "看电影": "scene-movie",
		"安防模式": "scene-security", "security mode": "scene-security", "布防": "scene-security",
		"节能模式": "scene-energy", "energy saving": "scene-energy", "省电": "scene-energy",
		"睡眠模式": "scene-sleep", "sleep mode": "scene-sleep", "睡觉": "scene-sleep",
		"派对模式": "scene-party", "party mode": "scene-party", "聚会": "scene-party",
	}

	for keyword, sceneID := range sceneKeywords {
		if strings.Contains(queryLower, strings.ToLower(keyword)) {
			intent.Type = "scene"
			intent.Action = "activate"
			intent.Target = sceneID
			return intent
		}
	}

	// 设备控制匹配
	deviceCommands := map[string]CommandType{
		"开机": CmdPowerOn, "打开": CmdPowerOn, "开启": CmdPowerOn, "turn on": CmdPowerOn, "power on": CmdPowerOn,
		"关机": CmdPowerOff, "关闭": CmdPowerOff, "关掉": CmdPowerOff, "turn off": CmdPowerOff, "power off": CmdPowerOff,
		"播放": CmdPlay, "放": CmdPlay, "play": CmdPlay,
		"暂停": CmdPause, "停一下": CmdPause, "pause": CmdPause,
		"停止": CmdStop, "stop": CmdStop,
		"上一个": CmdPrevious, "上一首": CmdPrevious, "previous": CmdPrevious,
		"下一个": CmdNext, "下一首": CmdNext, "next": CmdNext,
		"静音": CmdMute, "mute": CmdMute,
		"取消静音": CmdUnmute, "unmute": CmdUnmute,
	}

	for keyword, cmdType := range deviceCommands {
		if strings.Contains(queryLower, strings.ToLower(keyword)) {
			intent.Type = "device"
			intent.Action = string(cmdType)
			// 提取设备名称
			intent.Target = m.extractDeviceName(query, keyword)
			return intent
		}
	}

	// 音量控制
	volumePattern := regexp.MustCompile(`(音量|volume)\s*(调到|设为|设为|set to)\s*(\d+)`)
	if matches := volumePattern.FindStringSubmatch(queryLower); len(matches) > 3 {
		intent.Type = "device"
		intent.Action = string(CmdSetVolume)
		intent.Params["volume"] = matches[3]
		return intent
	}

	if strings.Contains(queryLower, "音量") || strings.Contains(queryLower, "volume") {
		if strings.Contains(queryLower, "大") || strings.Contains(queryLower, "up") || strings.Contains(queryLower, "高") {
			intent.Type = "device"
			intent.Action = string(CmdVolumeUp)
			return intent
		}
		if strings.Contains(queryLower, "小") || strings.Contains(queryLower, "down") || strings.Contains(queryLower, "低") {
			intent.Type = "device"
			intent.Action = string(CmdVolumeDown)
			return intent
		}
	}

	// TTS 请求
	ttsKeywords := []string{"说", "播报", "念", "读", "say", "speak", "announce", "read"}
	for _, kw := range ttsKeywords {
		if strings.HasPrefix(queryLower, kw) {
			intent.Type = "tts"
			intent.Action = "speak"
			intent.Target = strings.TrimSpace(strings.TrimPrefix(query, kw))
			return intent
		}
	}

	// 通用命令
	intent.Type = "general"
	intent.Action = "query"
	intent.Target = query
	return intent
}

// extractDeviceName 提取设备名称
func (m *Manager) extractDeviceName(query, command string) string {
	// 移除命令关键词，提取设备名
	queryClean := strings.ReplaceAll(query, command, "")
	queryClean = strings.TrimSpace(queryClean)

	// 常见设备关键词
	devices := []string{"电视", "灯", "空调", "音响", "投影仪", "窗帘", "摄像头", "音箱", "风扇"}
	for _, d := range devices {
		if strings.Contains(queryClean, d) {
			return d
		}
	}

	if queryClean != "" {
		return queryClean
	}
	return "default"
}

// handleSceneCommand 处理场景命令
func (m *Manager) handleSceneCommand(ctx context.Context, intent *Intent, resp *VoiceResponse) error {
	m.mu.RLock()
	scene, ok := m.scenes[intent.Target]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("scene not found: %s", intent.Target)
	}

	// 模拟执行场景中的设备动作
	actions := make([]DeviceAction, 0, len(scene.Devices))
	for _, action := range scene.Devices {
		newAction := action
		newAction.Status = CommandStatusSuccess
		actions = append(actions, newAction)
	}

	sceneCopy := *scene
	sceneCopy.IsActive = true
	sceneCopy.UpdatedAt = time.Now()

	resp.Scene = &sceneCopy
	resp.Actions = actions
	resp.TextReply = fmt.Sprintf("已为您激活%s", scene.Name)

	m.logger.Info("scene activated",
		zap.String("scene_id", scene.ID),
		zap.String("scene_name", scene.Name))

	return nil
}

// handleDeviceCommand 处理设备控制命令
func (m *Manager) handleDeviceCommand(ctx context.Context, intent *Intent, resp *VoiceResponse) error {
	action := DeviceAction{
		DeviceID:    fmt.Sprintf("device-%s", intent.Target),
		DeviceName:  intent.Target,
		CommandType: CommandType(intent.Action),
		Parameters:  intent.Params,
		Status:      CommandStatusSuccess,
	}

	resp.Actions = []DeviceAction{action}
	resp.TextReply = fmt.Sprintf("已执行%s的%s操作", intent.Target, m.getCommandName(CommandType(intent.Action)))

	return nil
}

// handleTTSCommand 处理 TTS 命令
func (m *Manager) handleTTSCommand(ctx context.Context, intent *Intent, resp *VoiceResponse) error {
	if !m.config.TTSEnabled {
		resp.TextReply = intent.Target
		return nil
	}

	// 生成 SSML
	ssml := fmt.Sprintf(`<speak version="1.0" xmlns="http://www.w3.org/2001/10/synthesis" xml:lang="zh-CN">
		<voice name="%s">
			<prosody rate="%f" pitch="%f">%s</prosody>
		</voice>
	</speak>`, m.config.TTSVoice, m.config.TTSSpeed, m.config.TTSPitch, intent.Target)

	resp.TextReply = intent.Target
	resp.SsmlReply = ssml

	return nil
}

// handleCustomCommand 处理自定义命令
func (m *Manager) handleCustomCommand(ctx context.Context, intent *Intent, cmd *VoiceCommand, resp *VoiceResponse) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, custom := range m.commands {
		if !custom.IsActive {
			continue
		}

		matched, _ := regexp.MatchString(strings.ToLower(custom.Pattern), strings.ToLower(cmd.Query))
		if matched {
			resp.TextReply = custom.Response
			if len(custom.Actions) > 0 {
				actions := make([]DeviceAction, len(custom.Actions))
				copy(actions, custom.Actions)
				for i := range actions {
					actions[i].Status = CommandStatusSuccess
				}
				resp.Actions = actions
			}
			return nil
		}
	}

	// 没有匹配的自定义命令，走通用处理
	return m.handleGeneralCommand(ctx, intent, cmd, resp)
}

// handleGeneralCommand 处理通用命令
func (m *Manager) handleGeneralCommand(ctx context.Context, intent *Intent, cmd *VoiceCommand, resp *VoiceResponse) error {
	// 根据语言返回通用回复
	switch cmd.Language {
	case LangEnglish:
		resp.TextReply = fmt.Sprintf("I heard: %s. I can help you control devices, activate scenes, or answer questions.", cmd.Query)
	case LangJapanese:
		resp.TextReply = fmt.Sprintf("%s と聞こえました。デバイスの制御、シーンの有効化、または質問にお答えできます。", cmd.Query)
	default:
		resp.TextReply = fmt.Sprintf("我听到了：%s。我可以帮您控制设备、激活场景或回答问题。", cmd.Query)
	}

	resp.Suggestions = []string{
		"打开电视", "关闭灯光",
		"激活观影模式", "音量调大",
	}

	return nil
}

// getCommandName 获取命令名称
func (m *Manager) getCommandName(cmd CommandType) string {
	names := map[CommandType]string{
		CmdPowerOn:    "开机",
		CmdPowerOff:   "关机",
		CmdVolumeUp:   "调高音量",
		CmdVolumeDown: "调低音量",
		CmdSetVolume:  "设置音量",
		CmdPlay:       "播放",
		CmdPause:      "暂停",
		CmdStop:       "停止",
		CmdNext:       "下一个",
		CmdPrevious:   "上一个",
		CmdMute:       "静音",
		CmdUnmute:     "取消静音",
	}

	if name, ok := names[cmd]; ok {
		return name
	}
	return string(cmd)
}

// getLocalizedError 获取本地化错误消息
func (m *Manager) getLocalizedError(err string, lang VoiceLanguage) string {
	switch lang {
	case LangEnglish:
		return fmt.Sprintf("Sorry, an error occurred: %s", err)
	case LangJapanese:
		return fmt.Sprintf("申し訳ありません、エラーが発生しました: %s", err)
	default:
		return fmt.Sprintf("抱歉，出现错误：%s", err)
	}
}

// updateSessionContext 更新会话上下文
func (m *Manager) updateSessionContext(cmd *VoiceCommand) {
	if cmd.SessionID == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	ctx, ok := m.sessionContexts[cmd.SessionID]
	if !ok {
		ctx = &SessionContext{
			SessionID: cmd.SessionID,
			Platform:  cmd.Platform,
			Language:  cmd.Language,
			Context:   make(map[string]interface{}),
		}
		m.sessionContexts[cmd.SessionID] = ctx
	}

	ctx.LastActive = time.Now()
	ctx.Platform = cmd.Platform
	ctx.Language = cmd.Language
}

// addHistory 添加历史记录
func (m *Manager) addHistory(h *CommandHistory) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.history = append(m.history, h)

	// 限制历史大小
	if len(m.history) > m.config.MaxHistory {
		m.history = m.history[len(m.history)-m.config.MaxHistory:]
	}
}

// GetCommandHistory 获取命令历史
func (m *Manager) GetCommandHistory(limit int) []*CommandHistory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.history) {
		limit = len(m.history)
	}

	start := len(m.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*CommandHistory, limit)
	copy(result, m.history[start:])
	return result
}

// CreateScene 创建场景
func (m *Manager) CreateScene(req *SceneRequest) (*SceneInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	scene := &SceneInfo{
		ID:          generateID(),
		Name:        req.Name,
		Type:        req.Type,
		Description: req.Description,
		Devices:     req.Devices,
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.scenes[scene.ID] = scene
	return scene, nil
}

// GetScene 获取场景
func (m *Manager) GetScene(id string) (*SceneInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	scene, ok := m.scenes[id]
	if !ok {
		return nil, fmt.Errorf("scene not found: %s", id)
	}
	return scene, nil
}

// ListScenes 列出所有场景
func (m *Manager) ListScenes() []*SceneInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	scenes := make([]*SceneInfo, 0, len(m.scenes))
	for _, s := range m.scenes {
		scenes = append(scenes, s)
	}
	return scenes
}

// UpdateScene 更新场景
func (m *Manager) UpdateScene(id string, req *SceneRequest) (*SceneInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	scene, ok := m.scenes[id]
	if !ok {
		return nil, fmt.Errorf("scene not found: %s", id)
	}

	scene.Name = req.Name
	scene.Type = req.Type
	scene.Description = req.Description
	scene.Devices = req.Devices
	scene.UpdatedAt = time.Now()

	return scene, nil
}

// DeleteScene 删除场景
func (m *Manager) DeleteScene(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.scenes[id]; !ok {
		return fmt.Errorf("scene not found: %s", id)
	}
	delete(m.scenes, id)
	return nil
}

// ActivateScene 激活场景
func (m *Manager) ActivateScene(ctx context.Context, id string) (*VoiceResponse, error) {
	scene, err := m.GetScene(id)
	if err != nil {
		return nil, err
	}

	resp := &VoiceResponse{
		ID:        generateID(),
		Status:    CommandStatusProcessing,
		CreatedAt: time.Now(),
	}

	actions := make([]DeviceAction, 0, len(scene.Devices))
	for _, action := range scene.Devices {
		newAction := action
		newAction.Status = CommandStatusSuccess
		actions = append(actions, newAction)
	}

	scene.UpdatedAt = time.Now()
	scene.IsActive = true

	resp.Status = CommandStatusSuccess
	resp.Scene = scene
	resp.Actions = actions
	resp.TextReply = fmt.Sprintf("已激活场景：%s", scene.Name)

	return resp, nil
}

// RegisterCustomCommand 注册自定义命令
func (m *Manager) RegisterCustomCommand(req *CustomCommandRequest) (*CustomCommand, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证正则表达式
	if _, err := regexp.Compile(req.Pattern); err != nil {
		return nil, fmt.Errorf("invalid pattern: %w", err)
	}

	if req.Language == "" {
		req.Language = m.config.DefaultLanguage
	}

	cmd := &CustomCommand{
		ID:        generateID(),
		Name:      req.Name,
		Pattern:   req.Pattern,
		Language:  req.Language,
		Response:  req.Response,
		Actions:   req.Actions,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.commands[cmd.ID] = cmd
	return cmd, nil
}

// GetCustomCommand 获取自定义命令
func (m *Manager) GetCustomCommand(id string) (*CustomCommand, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cmd, ok := m.commands[id]
	if !ok {
		return nil, fmt.Errorf("custom command not found: %s", id)
	}
	return cmd, nil
}

// ListCustomCommands 列出所有自定义命令
func (m *Manager) ListCustomCommands() []*CustomCommand {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cmds := make([]*CustomCommand, 0, len(m.commands))
	for _, c := range m.commands {
		cmds = append(cmds, c)
	}
	return cmds
}

// UpdateCustomCommand 更新自定义命令
func (m *Manager) UpdateCustomCommand(id string, req *CustomCommandRequest) (*CustomCommand, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cmd, ok := m.commands[id]
	if !ok {
		return nil, fmt.Errorf("custom command not found: %s", id)
	}

	if _, err := regexp.Compile(req.Pattern); err != nil {
		return nil, fmt.Errorf("invalid pattern: %w", err)
	}

	cmd.Name = req.Name
	cmd.Pattern = req.Pattern
	cmd.Response = req.Response
	cmd.Actions = req.Actions
	cmd.UpdatedAt = time.Now()

	return cmd, nil
}

// DeleteCustomCommand 删除自定义命令
func (m *Manager) DeleteCustomCommand(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.commands[id]; !ok {
		return fmt.Errorf("custom command not found: %s", id)
	}
	delete(m.commands, id)
	return nil
}

// RegisterWakeWord 注册唤醒词
func (m *Manager) RegisterWakeWord(config *WakeWordConfig) *WakeWordConfig {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.ID == "" {
		config.ID = generateID()
	}
	config.IsActive = true
	config.CreatedAt = time.Now()

	m.wakeWords[config.ID] = config
	return config
}

// ListWakeWords 列出所有唤醒词
func (m *Manager) ListWakeWords() []*WakeWordConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	words := make([]*WakeWordConfig, 0, len(m.wakeWords))
	for _, w := range m.wakeWords {
		words = append(words, w)
	}
	return words
}

// DeleteWakeWord 删除唤醒词
func (m *Manager) DeleteWakeWord(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.wakeWords[id]; !ok {
		return fmt.Errorf("wake word not found: %s", id)
	}
	delete(m.wakeWords, id)
	return nil
}

// CreateReplyTemplate 创建回复模板
func (m *Manager) CreateReplyTemplate(req *ReplyTemplateRequest) *ReplyTemplate {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Language == "" {
		req.Language = m.config.DefaultLanguage
	}

	// 从模板中提取变量
	vars := m.extractVariables(req.Template)

	tpl := &ReplyTemplate{
		ID:        generateID(),
		Name:      req.Name,
		Language:  req.Language,
		Template:  req.Template,
		Category:  req.Category,
		Variables: vars,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.templates[tpl.ID] = tpl
	return tpl
}

// extractVariables 从模板中提取变量
func (m *Manager) extractVariables(template string) []string {
	re := regexp.MustCompile(`\{\{(\w+)\}\}`)
	matches := re.FindAllStringSubmatch(template, -1)

	varSet := make(map[string]bool)
	vars := make([]string, 0)
	for _, match := range matches {
		if len(match) > 1 && !varSet[match[1]] {
			varSet[match[1]] = true
			vars = append(vars, match[1])
		}
	}
	return vars
}

// GetReplyTemplate 获取回复模板
func (m *Manager) GetReplyTemplate(id string) (*ReplyTemplate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tpl, ok := m.templates[id]
	if !ok {
		return nil, fmt.Errorf("template not found: %s", id)
	}
	return tpl, nil
}

// ListReplyTemplates 列出所有回复模板
func (m *Manager) ListReplyTemplates() []*ReplyTemplate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tpls := make([]*ReplyTemplate, 0, len(m.templates))
	for _, t := range m.templates {
		tpls = append(tpls, t)
	}
	return tpls
}

// UpdateReplyTemplate 更新回复模板
func (m *Manager) UpdateReplyTemplate(id string, req *ReplyTemplateRequest) (*ReplyTemplate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tpl, ok := m.templates[id]
	if !ok {
		return nil, fmt.Errorf("template not found: %s", id)
	}

	tpl.Name = req.Name
	tpl.Language = req.Language
	tpl.Template = req.Template
	tpl.Category = req.Category
	tpl.Variables = m.extractVariables(req.Template)
	tpl.UpdatedAt = time.Now()

	return tpl, nil
}

// DeleteReplyTemplate 删除回复模板
func (m *Manager) DeleteReplyTemplate(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.templates[id]; !ok {
		return fmt.Errorf("template not found: %s", id)
	}
	delete(m.templates, id)
	return nil
}

// RenderTemplate 渲染回复模板
func (m *Manager) RenderTemplate(id string, vars map[string]string) (string, error) {
	tpl, err := m.GetReplyTemplate(id)
	if err != nil {
		return "", err
	}

	result := tpl.Template
	for k, v := range vars {
		result = strings.ReplaceAll(result, fmt.Sprintf("{{%s}}", k), v)
	}

	return result, nil
}

// ConvertTTS 文本转语音
func (m *Manager) ConvertTTS(req *TTSRequest) (*TTSResponse, error) {
	if !m.config.TTSEnabled {
		return nil, fmt.Errorf("TTS is disabled")
	}

	if req.Language == "" {
		req.Language = m.config.DefaultLanguage
	}
	if req.Speed == 0 {
		req.Speed = m.config.TTSSpeed
	}
	if req.Pitch == 0 {
		req.Pitch = m.config.TTSPitch
	}
	if req.Format == "" {
		req.Format = "mp3"
	}

	// 生成 SSML
	voice := m.config.TTSVoice
	if req.Voice != "" {
		voice = req.Voice
	}

	_ = fmt.Sprintf(`<speak version="1.0" xmlns="http://www.w3.org/2001/10/synthesis" xml:lang="%s">
		<voice name="%s">
			<prosody rate="%f" pitch="%f">%s</prosody>
		</voice>
	</speak>`, req.Language, voice, req.Speed, req.Pitch, req.Text)

	return &TTSResponse{
		ID:        generateID(),
		AudioURL:  fmt.Sprintf("/api/v1/voicehub/tts/%s.%s", generateID(), req.Format),
		Duration:  time.Duration(len(req.Text)/5) * time.Second, // 粗略估算
		Format:    req.Format,
		CreatedAt: time.Now(),
	}, nil
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *VoiceHubConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.config
	return &cfg
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(cfg *VoiceHubConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg != nil {
		m.config = cfg
	}
}

// GetSupportedPlatforms 获取支持的平台列表
func (m *Manager) GetSupportedPlatforms() []VoicePlatform {
	return SupportedPlatforms()
}

// GetSupportedLanguages 获取支持的语言列表
func (m *Manager) GetSupportedLanguages() []VoiceLanguage {
	return SupportedLanguages()
}
