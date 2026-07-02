package resmonpro

import (
	"github.com/gin-gonic/gin"
)

// ResmonPro 资源监控增强模块.
type ResmonPro struct {
	handlers *Handlers
}

// New 创建资源监控模块.
func New() *ResmonPro {
	return &ResmonPro{
		handlers: NewHandlers(),
	}
}

// RegisterRoutes 注册路由到 gin 路由组.
func (r *ResmonPro) RegisterRoutes(router *gin.RouterGroup) {
	r.handlers.RegisterRoutes(router)
}
