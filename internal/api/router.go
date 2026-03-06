package api

import (
	"sket/internal/api/socket"
	"sket/internal/config"

	"github.com/gin-gonic/gin"
)

type Router struct {
	engine *gin.Engine
	cfg    *config.Config
	socket *socket.Handler
}

func NewRouter(cfg *config.Config, hub *socket.Hub) *Router {
	gin.SetMode(gin.ReleaseMode)
	e := gin.New()
	e.Use(gin.Recovery())

	sockHandler := &socket.Handler{
		Hub: hub,
		Config: struct {
			ReadBufferSize  int
			WriteBufferSize int
			PongWaitSec     int
			PingPeriodSec   int
		}{
			ReadBufferSize:  cfg.Socket.ReadBufferSize,
			WriteBufferSize: cfg.Socket.WriteBufferSize,
			PongWaitSec:     cfg.Socket.PongWaitSec,
			PingPeriodSec:   cfg.Socket.PingPeriodSec,
		},
	}

	r := &Router{engine: e, cfg: cfg, socket: sockHandler}
	r.routes()
	return r
}

func (r *Router) routes() {
	r.engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	r.engine.GET("/ws/socket", r.socket.Upgrade)

	// 后续扩展示例：聊天室等
	// v1 := r.engine.Group("/api/v1")
	// v1.GET("/rooms", ...)
	// v1.POST("/rooms/:id/messages", ...)
}

func (r *Router) Engine() *gin.Engine {
	return r.engine
}
