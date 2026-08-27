package api

import (
	"net/http"
	"os"
	"path/filepath"
	"sket/internal/api/socket"
	"sket/internal/auth"
	"sket/internal/config"
	"sket/internal/store"

	"github.com/gin-gonic/gin"
)

type API struct {
	store         *store.Store
	auth          *auth.Manager
	hub           *socket.Hub
	push          config.PushConfig
	attachmentDir string
	clientDir     string
	adminDir      string
}

func firstExistingDir(paths ...string) string {
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
	}
	return paths[0]
}

func New(cfg *config.Config, db *store.Store, hub *socket.Hub) *API {
	attachmentDir := filepath.Join("data", "attachments")
	_ = os.MkdirAll(attachmentDir, 0700)
	clientDir := firstExistingDir(filepath.Join("web", "client"), filepath.Join("..", "web", "client"))
	adminDir := firstExistingDir("admin", filepath.Join("..", "admin"))
	return &API{store: db, auth: auth.New(cfg.Auth.Secret), hub: hub, push: cfg.Push, attachmentDir: attachmentDir, clientDir: clientDir, adminDir: adminDir}
}

func (a *API) ClientEngine(cfg *config.Config) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	e := gin.New()
	e.Use(gin.Logger(), gin.Recovery())
	sock := &socket.Handler{Hub: a.hub, Authorize: func(t string) (int64, error) { c, err := a.auth.Parse(t); return c.UserID, err }}
	sock.Config.ReadBufferSize = cfg.Socket.ReadBufferSize
	sock.Config.WriteBufferSize = cfg.Socket.WriteBufferSize
	sock.Config.PongWaitSec = cfg.Socket.PongWaitSec
	sock.Config.PingPeriodSec = cfg.Socket.PingPeriodSec
	e.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok", "service": "client"}) })
	e.GET("/ws", sock.Upgrade)
	v := e.Group("/api")
	v.POST("/register", a.register)
	v.POST("/login", a.login)
	u := v.Group("")
	u.Use(a.requireUser(false))
	u.GET("/me", a.me)
	u.PUT("/me/profile", a.updateProfile)
	u.PUT("/me/key", a.setPublicKey)
	u.GET("/users", a.users)
	u.GET("/friends", a.friends)
	u.GET("/friend-requests", a.friendRequests)
	u.POST("/friend-requests", a.requestFriend)
	u.PUT("/friend-requests/:id/accept", a.acceptFriend)
	u.DELETE("/friends/:id", a.removeFriend)
	u.GET("/conversations", a.conversations)
	u.POST("/conversations", a.createConversation)
	u.DELETE("/conversations/:id", a.deleteConversation)
	u.PUT("/conversations/:id/state", a.updateConversationState)
	u.GET("/conversations/:id/messages", a.messages)
	u.DELETE("/conversations/:id/messages", a.clearConversationMessages)
	u.GET("/conversations/:id/members", a.conversationMembers)
	u.POST("/conversations/:id/messages", a.sendMessage)
	u.POST("/conversations/:id/attachments", a.uploadAttachment)
	u.DELETE("/conversations/:id/attachments/:attachment", a.deleteAttachment)
	u.GET("/conversations/:id/attachments/:attachment", a.downloadAttachment)
	u.POST("/conversations/:id/read", a.readConversation)
	u.GET("/push/config", a.pushConfig)
	u.POST("/push/subscriptions", a.savePushSubscription)
	u.DELETE("/push/subscriptions", a.deletePushSubscription)
	e.Static("/assets", filepath.Join(a.clientDir, "assets"))
	e.GET("/manifest.webmanifest", func(c *gin.Context) { c.File(filepath.Join(a.clientDir, "manifest.webmanifest")) })
	e.GET("/sw.js", func(c *gin.Context) {
		c.Header("Service-Worker-Allowed", "/")
		c.File(filepath.Join(a.clientDir, "sw.js"))
	})
	e.GET("/", func(c *gin.Context) { c.File(filepath.Join(a.clientDir, "index.html")) })
	e.NoRoute(func(c *gin.Context) {
		if c.Request.Method == http.MethodGet {
			c.File(filepath.Join(a.clientDir, "index.html"))
			return
		}
		c.JSON(404, gin.H{"error": "not found"})
	})
	return e
}

func (a *API) AdminEngine() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	e := gin.New()
	e.Use(gin.Logger(), gin.Recovery())
	e.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok", "service": "admin"}) })
	e.POST("/api/login", a.login)
	g := e.Group("/api")
	g.Use(a.requireUser(true))
	g.GET("/stats", a.adminStats)
	g.GET("/users", a.adminUsers)
	g.DELETE("/users/:id", a.deleteUser)
	e.GET("/", func(c *gin.Context) { c.File(filepath.Join(a.adminDir, "index.html")) })
	e.NoRoute(func(c *gin.Context) { c.File(filepath.Join(a.adminDir, "index.html")) })
	return e
}
