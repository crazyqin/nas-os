package notify

import (
	"encoding/json"
	"net/http"
	"os"
	"sync"

	"github.com/gin-gonic/gin"
)

// Config 通知配置
type Config struct {
	Email    EmailConfig    `json:"email,omitempty"`
	WeChat   WeChatConfig   `json:"wechat,omitempty"`
	Webhooks []WebhookConfig `json:"webhooks,omitempty"`
}

// EmailConfig 邮件配置
type EmailConfig struct {
	Enabled  bool     `json:"enabled"`
	SMTP     string   `json:"smtp_server"`
	Port     int      `json:"port"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	From     string   `json:"from"`
	To       []string `json:"to"`
}

// WeChatConfig 企业微信配置
type WeChatConfig struct {
	Enabled    bool   `json:"enabled"`
	WebhookURL string `json:"webhook_url"`
}

// WebhookConfig Webhook 配置
type WebhookConfig struct {
	Enabled bool   `json:"enabled"`
	Name    string `json:"name"`
	URL     string `json:"url"`
}

// Handlers 通知处理器
type Handlers struct {
	manager *Manager
	config  *Config
	configPath string
	mu      sync.RWMutex
}

// NewHandlers 创建通知处理器
func NewHandlers(manager *Manager, configPath string) *Handlers {
	h := &Handlers{
		manager:    manager,
		configPath: configPath,
		config:     &Config{},
	}
	h.loadConfig()
	return h
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	notify := r.Group("/notify")
	{
		notify.GET("/config", h.getConfig)
		notify.PUT("/config", h.updateConfig)
		notify.POST("/test", h.testNotification)
		notify.GET("/channels", h.getChannels)
	}
}

// loadConfig 加载配置
func (h *Handlers) loadConfig() {
	data, err := os.ReadFile(h.configPath)
	if err != nil {
		// 配置文件不存在，使用默认配置
		return
	}
	json.Unmarshal(data, h.config)
	h.applyConfig()
}

// saveConfig 保存配置
func (h *Handlers) saveConfig() error {
	data, err := json.MarshalIndent(h.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(h.configPath, data, 0644)
}

// applyConfig 应用配置
func (h *Handlers) applyConfig() {
	h.manager.notifiers = make([]Notifier, 0)

	if h.config.Email.Enabled {
		emailNotif := NewEmailNotifier(
			h.config.Email.SMTP,
			h.config.Email.Port,
			h.config.Email.Username,
			h.config.Email.Password,
			h.config.Email.From,
			h.config.Email.To,
		)
		h.manager.AddNotifier(emailNotif)
	}

	if h.config.WeChat.Enabled {
		wechatNotif := NewWeChatNotifier(h.config.WeChat.WebhookURL)
		h.manager.AddNotifier(wechatNotif)
	}

	for _, wh := range h.config.Webhooks {
		if wh.Enabled {
			webhookNotif := NewWebhookNotifier(wh.URL)
			h.manager.AddNotifier(webhookNotif)
		}
	}
}

// getConfig 获取配置
func (h *Handlers) getConfig(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// 不返回密码
	safeConfig := &Config{
		Email: EmailConfig{
			Enabled:  h.config.Email.Enabled,
			SMTP:     h.config.Email.SMTP,
			Port:     h.config.Email.Port,
			Username: h.config.Email.Username,
			From:     h.config.Email.From,
			To:       h.config.Email.To,
		},
		WeChat: WeChatConfig{
			Enabled:    h.config.WeChat.Enabled,
			WebhookURL: h.config.WeChat.WebhookURL,
		},
		Webhooks: h.config.Webhooks,
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    safeConfig,
	})
}

// updateConfig 更新配置
func (h *Handlers) updateConfig(c *gin.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()

	var newConfig Config
	if err := c.ShouldBindJSON(&newConfig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	h.config = &newConfig
	if err := h.saveConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "保存配置失败：" + err.Error(),
		})
		return
	}

	h.applyConfig()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "配置已更新",
	})
}

// testNotification 测试通知
func (h *Handlers) testNotification(c *gin.Context) {
	var req struct {
		Channel string `json:"channel"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	notif := &Notification{
		Title:   "测试通知",
		Message: "这是一条测试通知，如果您收到此消息，说明通知配置正确。",
		Level:   LevelInfo,
		Source:  "NAS-OS",
	}

	switch req.Channel {
	case "email":
		if h.config.Email.Enabled {
			notifier := NewEmailNotifier(
				h.config.Email.SMTP,
				h.config.Email.Port,
				h.config.Email.Username,
				h.config.Email.Password,
				h.config.Email.From,
				h.config.Email.To,
			)
			if err := notifier.Send(notif); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":    500,
					"message": "发送失败：" + err.Error(),
				})
				return
			}
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "邮件通知未启用",
			})
			return
		}

	case "wechat":
		if h.config.WeChat.Enabled {
			notifier := NewWeChatNotifier(h.config.WeChat.WebhookURL)
			if err := notifier.Send(notif); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":    500,
					"message": "发送失败：" + err.Error(),
				})
				return
			}
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "微信通知未启用",
			})
			return
		}

	default:
		// 发送到所有渠道
		h.manager.Send(notif)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "测试通知已发送",
	})
}

// getChannels 获取已配置的通知渠道
func (h *Handlers) getChannels(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	channels := []gin.H{}

	if h.config.Email.Enabled {
		channels = append(channels, gin.H{
			"name":    "email",
			"type":    "邮件",
			"enabled": true,
		})
	}

	if h.config.WeChat.Enabled {
		channels = append(channels, gin.H{
			"name":    "wechat",
			"type":    "企业微信",
			"enabled": true,
		})
	}

	for _, wh := range h.config.Webhooks {
		if wh.Enabled {
			channels = append(channels, gin.H{
				"name":    "webhook:" + wh.Name,
				"type":    "Webhook",
				"enabled": true,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    channels,
	})
}
