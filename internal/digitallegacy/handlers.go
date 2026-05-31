// Package digitallegacy 提供 REST API 处理器
package digitallegacy

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 数字遗产 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	legacy := r.Group("/legacy")
	{
		// 遗产计划管理
		legacy.GET("/plans", h.listPlans)
		legacy.POST("/plans", h.createPlan)
		legacy.GET("/plans/:id", h.getPlan)
		legacy.PUT("/plans/:id", h.updatePlan)
		legacy.DELETE("/plans/:id", h.deletePlan)
		legacy.POST("/plans/:id/activate", h.activatePlan)
		legacy.POST("/plans/:id/trigger", h.triggerPlan)

		// 受益人管理
		legacy.GET("/plans/:id/beneficiaries", h.listBeneficiaries)
		legacy.POST("/plans/:id/beneficiaries", h.addBeneficiary)
		legacy.PUT("/plans/:id/beneficiaries/:bid", h.updateBeneficiary)
		legacy.DELETE("/plans/:id/beneficiaries/:bid", h.removeBeneficiary)

		// 数字资产管理
		legacy.GET("/plans/:id/assets", h.listAssets)
		legacy.POST("/plans/:id/assets", h.addAsset)
		legacy.PUT("/plans/:id/assets/:aid", h.updateAsset)
		legacy.DELETE("/plans/:id/assets/:aid", h.removeAsset)
		legacy.GET("/plans/:id/assets/:aid/decrypt", h.decryptAsset)

		// 紧急联系人管理
		legacy.GET("/plans/:id/contacts", h.listEmergencyContacts)
		legacy.POST("/plans/:id/contacts", h.addEmergencyContact)
		legacy.DELETE("/plans/:id/contacts/:cid", h.removeEmergencyContact)

		// 遗嘱文档管理
		legacy.GET("/plans/:id/will", h.getWillDocument)
		legacy.POST("/plans/:id/will", h.setWillDocument)

		// 信任联系人管理
		legacy.GET("/contacts", h.listTrustContacts)
		legacy.POST("/contacts", h.addTrustContact)
		legacy.DELETE("/contacts/:id", h.removeTrustContact)

		// 访问授权管理
		legacy.GET("/plans/:id/access-grants", h.getAccessGrants)
		legacy.DELETE("/access-grants/:id", h.revokeAccessGrant)

		// 审计日志
		legacy.GET("/plans/:id/audit-logs", h.getAuditLogs)

		// 不活跃检查
		legacy.POST("/check-inactivity", h.checkInactivity)

		// 配置
		legacy.GET("/config", h.getConfig)
		legacy.PUT("/config", h.updateConfig)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// getUserID 获取用户ID（从上下文）
func getUserID(c *gin.Context) string {
	// 这里应该从认证中间件获取用户ID
	// 简化处理，返回默认值
	return "user-001"
}

// listPlans 列出遗产计划
func (h *Handlers) listPlans(c *gin.Context) {
	userID := getUserID(c)
	plans := h.manager.ListPlans(userID)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    plans,
	})
}

// createPlan 创建遗产计划
func (h *Handlers) createPlan(c *gin.Context) {
	var req LegacyPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	userID := getUserID(c)
	plan, err := h.manager.CreatePlan(&req, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "plan created",
		Data:    plan,
	})
}

// getPlan 获取遗产计划
func (h *Handlers) getPlan(c *gin.Context) {
	id := c.Param("id")
	plan, err := h.manager.GetPlan(id)
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
		Data:    plan,
	})
}

// updatePlan 更新遗产计划
func (h *Handlers) updatePlan(c *gin.Context) {
	id := c.Param("id")
	var req LegacyPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	plan, err := h.manager.UpdatePlan(id, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "plan updated",
		Data:    plan,
	})
}

// deletePlan 删除遗产计划
func (h *Handlers) deletePlan(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeletePlan(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "plan deleted",
	})
}

// activatePlan 激活遗产计划
func (h *Handlers) activatePlan(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.ActivatePlan(id); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "plan activated",
	})
}

// triggerPlan 触发遗产计划
func (h *Handlers) triggerPlan(c *gin.Context) {
	id := c.Param("id")
	var req TriggerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	req.PlanID = id
	if err := h.manager.TriggerPlan(c.Request.Context(), id, &req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "plan triggered",
	})
}

// listBeneficiaries 列出受益人
func (h *Handlers) listBeneficiaries(c *gin.Context) {
	planID := c.Param("id")
	plan, err := h.manager.GetPlan(planID)
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
		Data:    plan.Beneficiaries,
	})
}

// addBeneficiary 添加受益人
func (h *Handlers) addBeneficiary(c *gin.Context) {
	planID := c.Param("id")
	var req BeneficiaryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	beneficiary, err := h.manager.AddBeneficiary(planID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "beneficiary added",
		Data:    beneficiary,
	})
}

// updateBeneficiary 更新受益人
func (h *Handlers) updateBeneficiary(c *gin.Context) {
	planID := c.Param("id")
	beneficiaryID := c.Param("bid")
	var req BeneficiaryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	beneficiary, err := h.manager.UpdateBeneficiary(planID, beneficiaryID, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "beneficiary updated",
		Data:    beneficiary,
	})
}

// removeBeneficiary 移除受益人
func (h *Handlers) removeBeneficiary(c *gin.Context) {
	planID := c.Param("id")
	beneficiaryID := c.Param("bid")
	if err := h.manager.RemoveBeneficiary(planID, beneficiaryID); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "beneficiary removed",
	})
}

// listAssets 列出数字资产
func (h *Handlers) listAssets(c *gin.Context) {
	planID := c.Param("id")
	plan, err := h.manager.GetPlan(planID)
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
		Data:    plan.Assets,
	})
}

// addAsset 添加数字资产
func (h *Handlers) addAsset(c *gin.Context) {
	planID := c.Param("id")
	var req AssetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	asset, err := h.manager.AddAsset(planID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "asset added",
		Data:    asset,
	})
}

// updateAsset 更新数字资产
func (h *Handlers) updateAsset(c *gin.Context) {
	planID := c.Param("id")
	assetID := c.Param("aid")
	var req AssetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	asset, err := h.manager.UpdateAsset(planID, assetID, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "asset updated",
		Data:    asset,
	})
}

// removeAsset 移除数字资产
func (h *Handlers) removeAsset(c *gin.Context) {
	planID := c.Param("id")
	assetID := c.Param("aid")
	if err := h.manager.RemoveAsset(planID, assetID); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "asset removed",
	})
}

// decryptAsset 解密资产数据
func (h *Handlers) decryptAsset(c *gin.Context) {
	planID := c.Param("id")
	assetID := c.Param("aid")

	data, err := h.manager.DecryptAssetData(planID, assetID)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    map[string]string{"data": data},
	})
}

// listEmergencyContacts 列出紧急联系人
func (h *Handlers) listEmergencyContacts(c *gin.Context) {
	planID := c.Param("id")
	plan, err := h.manager.GetPlan(planID)
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
		Data:    plan.EmergencyContacts,
	})
}

// addEmergencyContact 添加紧急联系人
func (h *Handlers) addEmergencyContact(c *gin.Context) {
	planID := c.Param("id")
	var req EmergencyContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	contact, err := h.manager.AddEmergencyContact(planID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "emergency contact added",
		Data:    contact,
	})
}

// removeEmergencyContact 移除紧急联系人
func (h *Handlers) removeEmergencyContact(c *gin.Context) {
	planID := c.Param("id")
	contactID := c.Param("cid")
	if err := h.manager.RemoveEmergencyContact(planID, contactID); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "emergency contact removed",
	})
}

// getWillDocument 获取遗嘱文档
func (h *Handlers) getWillDocument(c *gin.Context) {
	planID := c.Param("id")
	decrypt := c.DefaultQuery("decrypt", "false") == "true"

	doc, err := h.manager.GetWillDocument(planID, decrypt)
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
		Data:    doc,
	})
}

// setWillDocument 设置遗嘱文档
func (h *Handlers) setWillDocument(c *gin.Context) {
	planID := c.Param("id")
	var req WillDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	doc, err := h.manager.SetWillDocument(planID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "will document set",
		Data:    doc,
	})
}

// listTrustContacts 列出信任联系人
func (h *Handlers) listTrustContacts(c *gin.Context) {
	userID := getUserID(c)
	contacts := h.manager.GetTrustContacts(userID)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    contacts,
	})
}

// addTrustContact 添加信任联系人
func (h *Handlers) addTrustContact(c *gin.Context) {
	var req TrustContact
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	userID := getUserID(c)
	contact, err := h.manager.AddTrustContact(userID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "trust contact added",
		Data:    contact,
	})
}

// removeTrustContact 移除信任联系人
func (h *Handlers) removeTrustContact(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.RemoveTrustContact(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "trust contact removed",
	})
}

// getAccessGrants 获取访问授权
func (h *Handlers) getAccessGrants(c *gin.Context) {
	planID := c.Param("id")
	beneficiaryID := c.Query("beneficiary_id")

	grants := h.manager.GetAccessGrants(planID, beneficiaryID)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    grants,
	})
}

// revokeAccessGrant 撤销访问授权
func (h *Handlers) revokeAccessGrant(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.RevokeAccessGrant(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "access grant revoked",
	})
}

// getAuditLogs 获取审计日志
func (h *Handlers) getAuditLogs(c *gin.Context) {
	planID := c.Param("id")
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	logs := h.manager.GetAuditLogs(planID, limit)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    logs,
	})
}

// checkInactivity 检查不活跃状态
func (h *Handlers) checkInactivity(c *gin.Context) {
	checks := h.manager.CheckInactivity(c.Request.Context())
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    checks,
	})
}

// getConfig 获取配置
func (h *Handlers) getConfig(c *gin.Context) {
	cfg := h.manager.GetConfig()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    cfg,
	})
}

// updateConfig 更新配置
func (h *Handlers) updateConfig(c *gin.Context) {
	var cfg DefaultLegacyConfig
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
