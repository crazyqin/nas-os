package teamfile

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 团队文件协作 HTTP 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	teams := r.Group("/teams")
	{
		teams.POST("", h.createTeam)
		teams.GET("", h.listTeams)
		teams.GET("/:id", h.getTeam)

		// 成员管理
		teams.POST("/:id/members", h.addMember)
		teams.DELETE("/:id/members/:uid", h.removeMember)
		teams.PUT("/:id/members/:uid/role", h.updateMemberRole)

		// 文件共享
		teams.POST("/:id/files", h.shareFile)
		teams.POST("/:id/files/:fid/lock", h.lockFile)
		teams.POST("/:id/files/:fid/unlock", h.unlockFile)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// createTeam 创建团队
// POST /teams
func (h *Handlers) createTeam(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, response{Code: 401, Message: "unauthorized"})
		return
	}

	var req CreateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 400, Message: err.Error()})
		return
	}

	team, err := h.manager.CreateTeam(userID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 400, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 201, Message: "created", Data: team})
}

// listTeams 列出用户参与的所有团队
// GET /teams
func (h *Handlers) listTeams(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, response{Code: 401, Message: "unauthorized"})
		return
	}

	teams := h.manager.ListUserTeams(userID)
	c.JSON(http.StatusOK, response{Code: 200, Message: "ok", Data: teams})
}

// getTeam 获取团队详情
// GET /teams/:id
func (h *Handlers) getTeam(c *gin.Context) {
	teamID := c.Param("id")

	team, err := h.manager.GetTeam(teamID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 404, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 200, Message: "ok", Data: team})
}

// addMember 添加团队成员
// POST /teams/:id/members
func (h *Handlers) addMember(c *gin.Context) {
	operatorID := c.GetString("user_id")
	teamID := c.Param("id")

	var req AddMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 400, Message: err.Error()})
		return
	}

	if err := h.manager.AddMember(operatorID, teamID, &req); err != nil {
		status := http.StatusBadRequest
		if err == ErrPermissionDenied {
			status = http.StatusForbidden
		} else if err == ErrTeamNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, response{Code: status, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 200, Message: "member added"})
}

// removeMember 移除团队成员
// DELETE /teams/:id/members/:uid
func (h *Handlers) removeMember(c *gin.Context) {
	operatorID := c.GetString("user_id")
	teamID := c.Param("id")
	memberUID := c.Param("uid")

	if err := h.manager.RemoveMember(operatorID, teamID, memberUID); err != nil {
		status := http.StatusBadRequest
		switch err {
		case ErrPermissionDenied:
			status = http.StatusForbidden
		case ErrTeamNotFound, ErrMemberNotFound:
			status = http.StatusNotFound
		}
		c.JSON(status, response{Code: status, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 200, Message: "member removed"})
}

// updateMemberRole 更新成员角色
// PUT /teams/:id/members/:uid/role
func (h *Handlers) updateMemberRole(c *gin.Context) {
	operatorID := c.GetString("user_id")
	teamID := c.Param("id")
	memberUID := c.Param("uid")

	var body struct {
		Role Role `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 400, Message: err.Error()})
		return
	}

	if err := h.manager.UpdateMemberRole(operatorID, teamID, memberUID, body.Role); err != nil {
		status := http.StatusBadRequest
		switch err {
		case ErrPermissionDenied:
			status = http.StatusForbidden
		case ErrTeamNotFound, ErrMemberNotFound:
			status = http.StatusNotFound
		}
		c.JSON(status, response{Code: status, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 200, Message: "role updated"})
}

// shareFile 在团队中共享文件
// POST /teams/:id/files
func (h *Handlers) shareFile(c *gin.Context) {
	userID := c.GetString("user_id")
	teamID := c.Param("id")

	var req ShareFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 400, Message: err.Error()})
		return
	}

	sf, err := h.manager.ShareFile(userID, teamID, &req)
	if err != nil {
		status := http.StatusBadRequest
		if err == ErrPermissionDenied {
			status = http.StatusForbidden
		} else if err == ErrTeamNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, response{Code: status, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 201, Message: "shared", Data: sf})
}

// lockFile 锁定文件
// POST /teams/:id/files/:fid/lock
func (h *Handlers) lockFile(c *gin.Context) {
	userID := c.GetString("user_id")
	teamID := c.Param("id")
	fileID := c.Param("fid")

	var req LockFileRequest
	// Duration 是可选参数，不强制绑定
	_ = c.ShouldBindJSON(&req)

	if err := h.manager.LockFile(userID, teamID, fileID, &req); err != nil {
		status := http.StatusBadRequest
		switch err {
		case ErrPermissionDenied:
			status = http.StatusForbidden
		case ErrFileLocked:
			status = http.StatusConflict
		case ErrFileNotFound, ErrTeamNotFound:
			status = http.StatusNotFound
		}
		c.JSON(status, response{Code: status, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 200, Message: "locked"})
}

// unlockFile 解锁文件
// POST /teams/:id/files/:fid/unlock
func (h *Handlers) unlockFile(c *gin.Context) {
	userID := c.GetString("user_id")
	teamID := c.Param("id")
	fileID := c.Param("fid")

	if err := h.manager.UnlockFile(userID, teamID, fileID); err != nil {
		status := http.StatusBadRequest
		switch err {
		case ErrPermissionDenied, ErrFileLocked:
			status = http.StatusForbidden
		case ErrFileNotFound, ErrTeamNotFound:
			status = http.StatusNotFound
		}
		c.JSON(status, response{Code: status, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 200, Message: "unlocked"})
}
