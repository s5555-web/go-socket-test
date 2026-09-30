package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Claims struct {
	UserID  int64 `json:"uid"`
	Admin   bool  `json:"adm"`
	Expires int64 `json:"exp"`
}
type websocketTicket struct {
	userID  int64
	expires time.Time
}

type Manager struct {
	secret   []byte
	ticketMu sync.Mutex
	tickets  map[string]websocketTicket
}

func New(secret string) *Manager {
	return &Manager{secret: []byte(secret), tickets: make(map[string]websocketTicket)}
}
func (m *Manager) Sign(id int64, admin bool) string {
	c, _ := json.Marshal(Claims{id, admin, time.Now().Add(30 * 24 * time.Hour).Unix()})
	p := base64.RawURLEncoding.EncodeToString(c)
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(p))
	return p + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
func (m *Manager) Parse(token string) (Claims, error) {
	var c Claims
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return c, errors.New("invalid token")
	}
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(parts[0]))
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(sig, mac.Sum(nil)) {
		return c, errors.New("invalid signature")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return c, err
	}
	if err = json.Unmarshal(raw, &c); err != nil {
		return c, err
	}
	if c.Expires < time.Now().Unix() {
		return c, errors.New("expired")
	}
	return c, nil
}
func Bearer(header string) string {
	p := strings.Fields(header)
	if len(p) == 2 && strings.EqualFold(p[0], "bearer") {
		return p[1]
	}
	return ""
}
func ParseID(v string) (int64, error) { return strconv.ParseInt(v, 10, 64) }

// IssueWebSocketTicket creates a short-lived, single-use credential so the
// long-lived bearer token never appears in a WebSocket URL or proxy log.
func (m *Manager) IssueWebSocketTicket(userID int64) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	ticket := base64.RawURLEncoding.EncodeToString(raw)
	now := time.Now()
	m.ticketMu.Lock()
	for value, entry := range m.tickets {
		if !entry.expires.After(now) {
			delete(m.tickets, value)
		}
	}
	m.tickets[ticket] = websocketTicket{userID: userID, expires: now.Add(30 * time.Second)}
	m.ticketMu.Unlock()
	return ticket, nil
}

func (m *Manager) ConsumeWebSocketTicket(ticket string) (int64, error) {
	m.ticketMu.Lock()
	entry, ok := m.tickets[ticket]
	delete(m.tickets, ticket)
	m.ticketMu.Unlock()
	if !ok || !entry.expires.After(time.Now()) {
		return 0, errors.New("invalid or expired websocket ticket")
	}
	return entry.userID, nil
}
