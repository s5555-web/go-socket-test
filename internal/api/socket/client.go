package socket

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const writeWait = 10 * time.Second

func (c *Client) ReadPump(readLimit int64, pongWait, pingPeriod time.Duration) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[socket] read pump panic recovered: %v", r)
		}
		c.Hub.Unregister(c)
		_ = c.Conn.Close()
	}()
	conn := c.Conn
	conn.SetReadLimit(readLimit)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		c.Hub.Broadcast(msg)
	}
}

func (c *Client) WritePump(pingPeriod time.Duration) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[socket] write pump panic recovered: %v", r)
		}
		ticker.Stop()
		_ = c.Conn.Close()
	}()
	conn := c.Conn
	for {
		select {
		case msg, ok := <-c.Send:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func GenClientID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
