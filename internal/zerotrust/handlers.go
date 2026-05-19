// Package zerotrust 提供零信任网络架构实现
package zerotrust

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 零信任 API 处理器.
type Handlers struct {
	engine *Engine
}

// NewHandlers 创建处理器.
func NewHandlers(engine *Engine) *Handlers {
	return &Handlers{engine: engine}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	zt := r.Group("/zero-trust")
	{
		// 访问控制
		zt.POST("/access/evaluate", h.evaluateAccess)
		zt.POST("/access/revoke/:sessionId", h.revokeSession)

		// 策略管理
		zt.GET("/policies", h.listPolicies)
		zt.GET("/policies/:id", h.getPolicy)
		zt.POST("/policies", h.createPolicy)
		zt.PUT("/policies/:id", h.updatePolicy)
		zt.DELETE("/policies/:id", h.deletePolicy)

		// 网络分段管理
		zt.GET("/segments", h.listSegments)
		zt.GET("/segments/:id", h.getSegment)
		zt.POST("/segments", h.createSegment)
		zt.PUT("/segments/:id", h.updateSegment)
		zt.DELETE("/segments/:id", h.deleteSegment)

		// 访问规则管理
		zt.GET("/rules", h.listRules)
		zt.GET("/rules/:id", h.getRule)
		zt.POST("/rules", h.createRule)
		zt.PUT("/rules/:id", h.updateRule)
		zt.DELETE("/rules/:id", h.deleteRule)

		// 身份管理
		zt.GET("/identities", h.listIdentities)
		zt.GET("/identities/:id", h.getIdentity)
		zt.POST("/identities", h.createIdentity)
		zt.PUT("/identities/:id", h.updateIdentity)
		zt.DELETE("/identities/:id", h.deleteIdentity)

		// 会话管理
		zt.GET("/sessions", h.listSessions)
		zt.GET("/sessions/:id", h.getSession)

		// 统计信息
		zt.GET("/stats", h.getStats)

		// 审计日志
		zt.GET("/audit/logs", h.getAuditLogs)

		// WireGuard 管理
		zt.GET("/wireguard/status", h.wireguardStatus)
		zt.GET("/wireguard/peers", h.wireguardPeers)
		zt.POST("/wireguard/peers", h.wireguardAddPeer)
		zt.DELETE("/wireguard/peers/:publicKey", h.wireguardRemovePeer)
		zt.POST("/wireguard/peers/:publicKey/restart", h.wireguardRestartPeer)
	}
}

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// evaluateAccess 评估访问请求.
func (h *Handlers) evaluateAccess(c *gin.Context) {
	var req AccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	// 设置默认值
	if req.SourceIP == "" {
		req.SourceIP = c.ClientIP()
	}

	decision, err := h.engine.EvaluateAccess(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	statusCode := http.StatusOK
	if !decision.Allowed {
		statusCode = http.StatusForbidden
	}

	c.JSON(statusCode, response{
		Code:    0,
		Message: decision.Reason,
		Data:    decision,
	})
}

// revokeSession 撤销会话.
func (h *Handlers) revokeSession(c *gin.Context) {
	sessionID := c.Param("sessionId")

	if err := h.engine.RevokeSession(sessionID); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "session revoked",
	})
}

// listPolicies 列出策略.
func (h *Handlers) listPolicies(c *gin.Context) {
	policies := h.engine.ListPolicies()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(policies),
			"policies": policies,
		},
	})
}

// getPolicy 获取策略.
func (h *Handlers) getPolicy(c *gin.Context) {
	id := c.Param("id")
	policy, ok := h.engine.GetPolicy(id)
	if !ok {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: "policy not found",
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    policy,
	})
}

// createPolicy 创建策略.
func (h *Handlers) createPolicy(c *gin.Context) {
	var policy TrustPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.engine.AddPolicy(&policy); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "policy created",
		Data:    policy,
	})
}

// updatePolicy 更新策略.
func (h *Handlers) updatePolicy(c *gin.Context) {
	id := c.Param("id")

	var policy TrustPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	policy.ID = id
	if err := h.engine.UpdatePolicy(&policy); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "policy updated",
		Data:    policy,
	})
}

// deletePolicy 删除策略.
func (h *Handlers) deletePolicy(c *gin.Context) {
	id := c.Param("id")

	if err := h.engine.DeletePolicy(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "policy deleted",
	})
}

// listSegments 列出网络分段.
func (h *Handlers) listSegments(c *gin.Context) {
	segments := h.engine.ListSegments()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(segments),
			"segments": segments,
		},
	})
}

// getSegment 获取网络分段.
func (h *Handlers) getSegment(c *gin.Context) {
	id := c.Param("id")
	segment, ok := h.engine.GetSegment(id)
	if !ok {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: "segment not found",
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    segment,
	})
}

// createSegment 创建网络分段.
func (h *Handlers) createSegment(c *gin.Context) {
	var segment NetworkSegment
	if err := c.ShouldBindJSON(&segment); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.engine.AddSegment(&segment); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "segment created",
		Data:    segment,
	})
}

// updateSegment 更新网络分段.
func (h *Handlers) updateSegment(c *gin.Context) {
	id := c.Param("id")

	var segment NetworkSegment
	if err := c.ShouldBindJSON(&segment); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	segment.ID = id
	if err := h.engine.UpdateSegment(&segment); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "segment updated",
		Data:    segment,
	})
}

// deleteSegment 删除网络分段.
func (h *Handlers) deleteSegment(c *gin.Context) {
	id := c.Param("id")

	if err := h.engine.DeleteSegment(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "segment deleted",
	})
}

// listRules 列出访问规则.
func (h *Handlers) listRules(c *gin.Context) {
	rules := h.engine.ListRules()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(rules),
			"rules": rules,
		},
	})
}

// getRule 获取访问规则.
func (h *Handlers) getRule(c *gin.Context) {
	id := c.Param("id")
	rule, ok := h.engine.GetRule(id)
	if !ok {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: "rule not found",
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    rule,
	})
}

// createRule 创建访问规则.
func (h *Handlers) createRule(c *gin.Context) {
	var rule AccessRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.engine.AddRule(&rule); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "rule created",
		Data:    rule,
	})
}

// updateRule 更新访问规则.
func (h *Handlers) updateRule(c *gin.Context) {
	id := c.Param("id")

	var rule AccessRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	rule.ID = id
	if err := h.engine.UpdateRule(&rule); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "rule updated",
		Data:    rule,
	})
}

// deleteRule 删除访问规则.
func (h *Handlers) deleteRule(c *gin.Context) {
	id := c.Param("id")

	if err := h.engine.DeleteRule(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "rule deleted",
	})
}

// listIdentities 列出身份.
func (h *Handlers) listIdentities(c *gin.Context) {
	identities := h.engine.ListIdentities()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":      len(identities),
			"identities": identities,
		},
	})
}

// getIdentity 获取身份.
func (h *Handlers) getIdentity(c *gin.Context) {
	id := c.Param("id")
	identity, ok := h.engine.GetIdentity(id)
	if !ok {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: "identity not found",
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    identity,
	})
}

// createIdentity 创建身份.
func (h *Handlers) createIdentity(c *gin.Context) {
	var identity Identity
	if err := c.ShouldBindJSON(&identity); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.engine.AddIdentity(&identity); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "identity created",
		Data:    identity,
	})
}

// updateIdentity 更新身份.
func (h *Handlers) updateIdentity(c *gin.Context) {
	id := c.Param("id")

	var identity Identity
	if err := c.ShouldBindJSON(&identity); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	identity.ID = id
	if err := h.engine.UpdateIdentity(&identity); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "identity updated",
		Data:    identity,
	})
}

// deleteIdentity 删除身份.
func (h *Handlers) deleteIdentity(c *gin.Context) {
	id := c.Param("id")

	if err := h.engine.DeleteIdentity(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "identity deleted",
	})
}

// listSessions 列出会话.
func (h *Handlers) listSessions(c *gin.Context) {
	sessions := h.engine.ListSessions()

	// 支持按状态过滤
	status := c.Query("status")
	if status != "" {
		filtered := make([]*AccessSession, 0)
		for _, s := range sessions {
			if string(s.Status) == status {
				filtered = append(filtered, s)
			}
		}
		sessions = filtered
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(sessions),
			"sessions": sessions,
		},
	})
}

// getSession 获取会话.
func (h *Handlers) getSession(c *gin.Context) {
	id := c.Param("id")
	session, ok := h.engine.GetSession(id)
	if !ok {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: "session not found",
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    session,
	})
}

// getStats 获取统计信息.
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.engine.GetStats()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// getAuditLogs 获取审计日志.
func (h *Handlers) getAuditLogs(c *gin.Context) {
	// 解析分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	// 解析过滤参数
	eventType := c.Query("eventType")
	severity := c.Query("severity")
	subjectID := c.Query("subjectId")
	allowed := c.Query("allowed")

	auditLogger := h.engine.GetAuditLog()
	if auditLogger == nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Code:    503,
			Message: "audit logger not available",
		})
		return
	}

	logs := auditLogger.GetLogs(page, pageSize, eventType, severity, subjectID, allowed)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    logs,
	})
}

// wireguardStatus 获取 WireGuard 状态.
func (h *Handlers) wireguardStatus(c *gin.Context) {
	wgMgr := h.engine.GetWireGuardManager()
	if wgMgr == nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Code:    503,
			Message: "wireguard not enabled",
		})
		return
	}

	status, err := wgMgr.GetStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    status,
	})
}

// wireguardPeers 获取 WireGuard 对等端列表.
func (h *Handlers) wireguardPeers(c *gin.Context) {
	wgMgr := h.engine.GetWireGuardManager()
	if wgMgr == nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Code:    503,
			Message: "wireguard not enabled",
		})
		return
	}

	peers := wgMgr.ListPeers()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(peers),
			"peers":  peers,
		},
	})
}

// wireguardAddPeerRequest 添加 WireGuard 对等端请求.
type wireguardAddPeerRequest struct {
	Name         string   `json:"name"`
	PublicKey    string   `json:"publicKey"`
	AllowedIPs   []string `json:"allowedIPs"`
	Endpoint     string   `json:"endpoint,omitempty"`
	TrustLevel   TrustLevel `json:"trustLevel,omitempty"`
}

// wireguardAddPeer 添加 WireGuard 对等端.
func (h *Handlers) wireguardAddPeer(c *gin.Context) {
	wgMgr := h.engine.GetWireGuardManager()
	if wgMgr == nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Code:    503,
			Message: "wireguard not enabled",
		})
		return
	}

	var req wireguardAddPeerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	peer := &WireGuardPeer{
		Name:       req.Name,
		PublicKey:  req.PublicKey,
		AllowedIPs: req.AllowedIPs,
		Endpoint:   req.Endpoint,
		TrustLevel: req.TrustLevel,
	}

	if peer.TrustLevel == "" {
		peer.TrustLevel = TrustLevelLow
	}

	if err := wgMgr.AddPeer(peer); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "peer added",
		Data:    peer,
	})
}

// wireguardRemovePeer 移除 WireGuard 对等端.
func (h *Handlers) wireguardRemovePeer(c *gin.Context) {
	wgMgr := h.engine.GetWireGuardManager()
	if wgMgr == nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Code:    503,
			Message: "wireguard not enabled",
		})
		return
	}

	publicKey := c.Param("publicKey")
	if err := wgMgr.RemovePeer(publicKey); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "peer removed",
	})
}

// wireguardRestartPeer 重启 WireGuard 对等端.
func (h *Handlers) wireguardRestartPeer(c *gin.Context) {
	wgMgr := h.engine.GetWireGuardManager()
	if wgMgr == nil {
		c.JSON(http.StatusServiceUnavailable, response{
			Code:    503,
			Message: "wireguard not enabled",
		})
		return
	}

	publicKey := c.Param("publicKey")
	if err := wgMgr.RestartPeer(publicKey); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "peer restarted",
	})
}
