// update_routes.go - 系统更新检查路由（Core 与 Full 双构建共用，挂 admin 组）.
package web

import (
	"net/http"

	"nas-os/internal/sysupdate"

	"github.com/gin-gonic/gin"
)

// registerUpdateRoutes 挂载 /system/updates 与 /system/update-settings.
// 更新检查只读上游 Release 元数据，不执行任何升级动作.
func (s *Server) registerUpdateRoutes(apiGroup *gin.RouterGroup) {
	if s == nil || s.cfg == nil || apiGroup == nil {
		return
	}
	checker := sysupdate.NewChecker()
	store := sysupdate.NewSettingsStore(s.cfg.Paths.DataDir)

	g := apiGroup.Group("/system")
	{
		g.GET("/updates", func(c *gin.Context) {
			status := checker.Check(c.Request.Context())
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": status})
		})

		g.GET("/update-settings", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": store.Load()})
		})

		g.PUT("/update-settings", func(c *gin.Context) {
			var set sysupdate.Settings
			if err := c.ShouldBindJSON(&set); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "无效的请求体"})
				return
			}
			if err := store.Save(set); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": "保存失败: " + err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "已保存", "data": set})
		})
	}
}
