// Package voicehub 提供 REST API 处理器
package voicehub

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 语音助手 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	voice := r.Group("/voicehub")
	{
		// 语音命令处理
		voice.POST("/command", h.processCommand)
		voice.POST("/command/batch", h.processBatchCommands)

		// 场景管理
		voice.GET("/scenes", h.listScenes)
		voice.POST("/scenes", h.createScene)
		voice.GET("/scenes/:id", h.getScene)
		voice.PUT("/scenes/:id", h.updateScene)
		voice.DELETE("/scenes/:id", h.deleteScene)
		voice.POST("/scenes/:id/activate", h.activateScene)

		// 自定义命令管理
		voice.GET("/commands", h.listCustomCommands)
		voice.POST("/commands", h.registerCustomCommand)
		voice.GET("/commands/:id", h.getCustomCommand)
		voice.PUT("/commands/:id", h.updateCustomCommand)
		voice.DELETE("/commands/:id", h.deleteCustomCommand)

		// 唤醒词管理
		voice.GET("/wake-words", h.listWakeWords)
		voice.POST("/wake-words", h.registerWakeWord)
		voice.DELETE("/wake-words/:id", h.deleteWakeWord)

		// 回复模板管理
		voice.GET("/templates", h.listTemplates)
		voice.POST("/templates", h.createTemplate)
		voice.GET("/templates/:id", h.getTemplate)
		voice.PUT("/templates/:id", h.updateTemplate)
		voice.DELETE("/templates/:id", h.deleteTemplate)
		voice.POST("/templates/:id/render", h.renderTemplate)

		// TTS
		voice.POST("/tts", h.textToSpeech)

		// 历史记录
		voice.GET("/history", h.getHistory)

		// 配置
		voice.GET("/config", h.getConfig)
		voice.PUT("/config", h.updateConfig)

		// 平台和语言支持
		voice.GET("/platforms", h.getPlatforms)
		voice.GET("/languages", h.getLanguages)
	}
}

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// processCommand 处理语音命令.
func (h *Handlers) processCommand(c *gin.Context) {
	var cmd VoiceCommand
	if err := c.ShouldBindJSON(&cmd); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	resp, err := h.manager.ProcessVoiceCommand(c.Request.Context(), &cmd)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    resp,
	})
}

// processBatchCommands 批量处理语音命令.
func (h *Handlers) processBatchCommands(c *gin.Context) {
	var cmds []VoiceCommand
	if err := c.ShouldBindJSON(&cmds); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if len(cmds) > 10 {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "batch size exceeds maximum (10)",
		})
		return
	}

	results := make([]*VoiceResponse, 0, len(cmds))
	for _, cmd := range cmds {
		resp, err := h.manager.ProcessVoiceCommand(c.Request.Context(), &cmd)
		if err != nil {
			resp = &VoiceResponse{
				CommandID: cmd.ID,
				Status:    CommandStatusFailed,
				TextReply: err.Error(),
			}
		}
		results = append(results, resp)
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    results,
	})
}

// listScenes 列出场景.
func (h *Handlers) listScenes(c *gin.Context) {
	scenes := h.manager.ListScenes()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    scenes,
	})
}

// createScene 创建场景.
func (h *Handlers) createScene(c *gin.Context) {
	var req SceneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	scene, err := h.manager.CreateScene(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "scene created",
		Data:    scene,
	})
}

// getScene 获取场景.
func (h *Handlers) getScene(c *gin.Context) {
	id := c.Param("id")
	scene, err := h.manager.GetScene(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    scene,
	})
}

// updateScene 更新场景.
func (h *Handlers) updateScene(c *gin.Context) {
	id := c.Param("id")
	var req SceneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	scene, err := h.manager.UpdateScene(id, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "scene updated",
		Data:    scene,
	})
}

// deleteScene 删除场景.
func (h *Handlers) deleteScene(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteScene(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "scene deleted",
	})
}

// activateScene 激活场景.
func (h *Handlers) activateScene(c *gin.Context) {
	id := c.Param("id")
	resp, err := h.manager.ActivateScene(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "scene activated",
		Data:    resp,
	})
}

// listCustomCommands 列出自定义命令.
func (h *Handlers) listCustomCommands(c *gin.Context) {
	cmds := h.manager.ListCustomCommands()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    cmds,
	})
}

// registerCustomCommand 注册自定义命令.
func (h *Handlers) registerCustomCommand(c *gin.Context) {
	var req CustomCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	cmd, err := h.manager.RegisterCustomCommand(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "command registered",
		Data:    cmd,
	})
}

// getCustomCommand 获取自定义命令.
func (h *Handlers) getCustomCommand(c *gin.Context) {
	id := c.Param("id")
	cmd, err := h.manager.GetCustomCommand(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    cmd,
	})
}

// updateCustomCommand 更新自定义命令.
func (h *Handlers) updateCustomCommand(c *gin.Context) {
	id := c.Param("id")
	var req CustomCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	cmd, err := h.manager.UpdateCustomCommand(id, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "command updated",
		Data:    cmd,
	})
}

// deleteCustomCommand 删除自定义命令.
func (h *Handlers) deleteCustomCommand(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteCustomCommand(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "command deleted",
	})
}

// listWakeWords 列出唤醒词.
func (h *Handlers) listWakeWords(c *gin.Context) {
	words := h.manager.ListWakeWords()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    words,
	})
}

// registerWakeWord 注册唤醒词.
func (h *Handlers) registerWakeWord(c *gin.Context) {
	var config WakeWordConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	result := h.manager.RegisterWakeWord(&config)
	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "wake word registered",
		Data:    result,
	})
}

// deleteWakeWord 删除唤醒词.
func (h *Handlers) deleteWakeWord(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteWakeWord(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "wake word deleted",
	})
}

// listTemplates 列出回复模板.
func (h *Handlers) listTemplates(c *gin.Context) {
	tpls := h.manager.ListReplyTemplates()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    tpls,
	})
}

// createTemplate 创建回复模板.
func (h *Handlers) createTemplate(c *gin.Context) {
	var req ReplyTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	tpl := h.manager.CreateReplyTemplate(&req)
	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "template created",
		Data:    tpl,
	})
}

// getTemplate 获取回复模板.
func (h *Handlers) getTemplate(c *gin.Context) {
	id := c.Param("id")
	tpl, err := h.manager.GetReplyTemplate(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    tpl,
	})
}

// updateTemplate 更新回复模板.
func (h *Handlers) updateTemplate(c *gin.Context) {
	id := c.Param("id")
	var req ReplyTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	tpl, err := h.manager.UpdateReplyTemplate(id, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "template updated",
		Data:    tpl,
	})
}

// deleteTemplate 删除回复模板.
func (h *Handlers) deleteTemplate(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteReplyTemplate(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "template deleted",
	})
}

// renderTemplate 渲染回复模板.
func (h *Handlers) renderTemplate(c *gin.Context) {
	id := c.Param("id")
	var vars map[string]string
	if err := c.ShouldBindJSON(&vars); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	result, err := h.manager.RenderTemplate(id, vars)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    map[string]string{"rendered": result},
	})
}

// textToSpeech 文本转语音.
func (h *Handlers) textToSpeech(c *gin.Context) {
	var req TTSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	ttsResp, err := h.manager.ConvertTTS(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    ttsResp,
	})
}

// getHistory 获取命令历史.
func (h *Handlers) getHistory(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	history := h.manager.GetCommandHistory(limit)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    history,
	})
}

// getConfig 获取配置.
func (h *Handlers) getConfig(c *gin.Context) {
	cfg := h.manager.GetConfig()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    cfg,
	})
}

// updateConfig 更新配置.
func (h *Handlers) updateConfig(c *gin.Context) {
	var cfg VoiceHubConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	h.manager.UpdateConfig(&cfg)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "config updated",
	})
}

// getPlatforms 获取支持的平台.
func (h *Handlers) getPlatforms(c *gin.Context) {
	platforms := h.manager.GetSupportedPlatforms()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    platforms,
	})
}

// getLanguages 获取支持的语言.
func (h *Handlers) getLanguages(c *gin.Context) {
	languages := h.manager.GetSupportedLanguages()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    languages,
	})
}
