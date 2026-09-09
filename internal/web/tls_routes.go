// tls_routes.go - Web UI HTTPS 管理端点与明文跳转（Core 与 Full 双构建共用）.
package web

import (
	"context"
	"log"
	"net"
	"net/http"

	"nas-os/internal/webtls"

	"github.com/gin-gonic/gin"
)

// maxTLSImportBytes 导入接口请求体上限（证书+私钥远小于此值）.
const maxTLSImportBytes = 128 << 10

// tlsManager 返回（必要时惰性创建）Web TLS 管理器.
// 首次初始化发生在构造期路由注册（registerCoreIdentityAndDocs），非并发路径.
func (s *Server) tlsManager() *webtls.Manager {
	if s == nil {
		return nil
	}
	if s.tlsMgr == nil {
		dir := ""
		if s.cfg != nil {
			dir = s.cfg.DataPath("webtls")
		}
		mgr := webtls.NewManager(dir)
		if err := mgr.Load(); err != nil {
			log.Printf("⚠️ Web UI TLS 状态加载失败（按无证书处理）：%v", err)
		}
		s.tlsMgr = mgr
	}
	return s.tlsMgr
}

// registerTLSRoutes 挂载 /system/tls 端点组（admin 权限）.
//
//	GET    /system/tls                证书与跳转状态
//	POST   /system/tls/generate       生成自签证书 {hosts?: []string}
//	POST   /system/tls/import         导入用户证书 {certPEM, keyPEM}
//	DELETE /system/tls/certificate    删除证书（同时关闭跳转）
//	PUT    /system/tls/settings       跳转开关 {enabled}
func (s *Server) registerTLSRoutes(apiGroup *gin.RouterGroup) {
	if s == nil || s.cfg == nil || apiGroup == nil {
		return
	}
	mgr := s.tlsManager()

	g := apiGroup.Group("/system/tls")
	{
		g.GET("", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": mgr.Status()})
		})

		g.POST("/generate", func(c *gin.Context) {
			var body struct {
				Hosts []string `json:"hosts"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				// 允许空 body：走默认主机名/IP 收集
				body.Hosts = nil
			}
			st, err := mgr.Generate(body.Hosts)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "自签证书已生成", "data": st})
		})

		g.POST("/import", func(c *gin.Context) {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxTLSImportBytes)
			var body struct {
				CertPEM string `json:"certPEM" binding:"required"`
				KeyPEM  string `json:"keyPEM" binding:"required"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "无效的请求体（或超过 128KB 上限）"})
				return
			}
			st, err := mgr.Import([]byte(body.CertPEM), []byte(body.KeyPEM))
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "证书已导入", "data": st})
		})

		g.DELETE("/certificate", func(c *gin.Context) {
			st, err := mgr.Remove()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "证书已删除", "data": st})
		})

		g.PUT("/settings", func(c *gin.Context) {
			var body struct {
				Enabled bool `json:"enabled"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "无效的请求体"})
				return
			}
			st, err := mgr.SetEnabled(body.Enabled)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "已保存", "data": st})
		})
	}
}

// httpsRedirectMiddleware 返回明文 → HTTPS 301 跳转中间件.
// 仅当跳转开关开启且证书有效时生效；本机（loopback）明文请求豁免，
// 保证 Docker/K8s 探针与 nasctl 本地调用不受影响.
func (s *Server) httpsRedirectMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.TLS != nil { // 已是 TLS 连接
			c.Next()
			return
		}
		mgr := s.tlsManager()
		if mgr == nil || !mgr.RedirectActive() {
			c.Next()
			return
		}
		if isLoopbackClient(c) {
			c.Next()
			return
		}
		host := c.Request.Host
		if host == "" {
			host = c.Request.URL.Host
		}
		if host == "" {
			c.Next()
			return
		}
		c.Redirect(http.StatusMovedPermanently, "https://"+host+c.Request.URL.RequestURI())
		c.Abort()
	}
}

// isLoopbackClient 判断客户端是否来自本机.
func isLoopbackClient(c *gin.Context) bool {
	ip := net.ParseIP(c.ClientIP())
	return ip != nil && ip.IsLoopback()
}

// serveHTTPWithOptionalTLS 按证书状态选择服务方式：
// 无有效证书 → 普通 HTTP；有证书 → 同端口双协议（TLS + 明文回退）.
// 双协议模式下探针与明文客户端仍可正常访问（回退为明文，是否 301 由中间件决定）.
func (s *Server) serveHTTPWithOptionalTLS(addr string, httpSrv *http.Server) error {
	mgr := s.tlsManager()
	if mgr == nil || !mgr.HasValidCert() {
		return httpSrv.ListenAndServe()
	}
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", addr)
	if err != nil {
		return err
	}
	return httpSrv.Serve(webtls.NewListener(ln, mgr.TLSConfig()))
}

// TLSStatusHint 供启动日志展示 HTTPS 可用性（application 层使用）.
func (s *Server) TLSStatusHint() string {
	mgr := s.tlsManager()
	if mgr == nil {
		return ""
	}
	if mgr.HasValidCert() {
		return "（HTTPS 已启用，同端口兼容 HTTP）"
	}
	return ""
}
