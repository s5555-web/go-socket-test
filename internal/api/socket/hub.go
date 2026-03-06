package socket

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

type Client struct {
	Hub  *Hub
	Conn *websocket.Conn
	Send chan []byte
	ID   string
}

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	done       chan struct{}
	mu         sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		done:       make(chan struct{}),
	}
}

// Stop 关闭 Hub，Run() 会退出
func (h *Hub) Stop() {
	close(h.done)
}

func (h *Hub) Run() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[socket] hub panic recovered: %v", r)
		}
	}()
	for {
		select {
		case <-h.done:
			return
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = true
			h.mu.Unlock()
		case c := <-h.unregister:
			h.unregisterClient(c)
		case msg := <-h.broadcast:
			var toRemove []*Client
			h.mu.RLock()
			for c := range h.clients {
				select {
				case c.Send <- msg:
				default:
					toRemove = append(toRemove, c)
				}
			}
			h.mu.RUnlock()
			for _, c := range toRemove {
				h.unregisterClient(c)
			}
		}
	}
}

// unregisterClient 从 map 移除并关闭 Send，只关闭一次，避免重复 close 导致 panic
func (h *Hub) unregisterClient(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c]; !ok {
		return
	}
	delete(h.clients, c)
	close(c.Send)
}

func (h *Hub) Broadcast(msg []byte) {
	h.broadcast <- msg
}

func (h *Hub) BroadcastJSON(v interface{}) {
	data, _ := json.Marshal(v)
	h.Broadcast(data)
}

func (h *Hub) Register(c *Client)   { h.register <- c }
func (h *Hub) Unregister(c *Client) { h.unregister <- c }

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	n := len(h.clients)
	h.mu.RUnlock()
	return n
}
