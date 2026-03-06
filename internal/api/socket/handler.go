package socket

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(*http.Request) bool { return true },
}

type Handler struct {
	Hub    *Hub
	Config struct {
		ReadBufferSize  int
		WriteBufferSize int
		PongWaitSec     int
		PingPeriodSec   int
	}
}

func (h *Handler) Upgrade(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	client := &Client{
		Hub:  h.Hub,
		Conn: conn,
		Send: make(chan []byte, 256),
		ID:   GenClientID(),
	}
	h.Hub.Register(client)
	pongWait := time.Duration(h.Config.PongWaitSec) * time.Second
	pingPeriod := time.Duration(h.Config.PingPeriodSec) * time.Second
	go client.WritePump(pingPeriod)
	go client.ReadPump(512, pongWait, pingPeriod)
}
