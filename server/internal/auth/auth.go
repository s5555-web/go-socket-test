package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

type Claims struct {
	UserID  int64 `json:"uid"`
	Admin   bool  `json:"adm"`
	Expires int64 `json:"exp"`
}
type Manager struct{ secret []byte }

func New(secret string) *Manager { return &Manager{secret: []byte(secret)} }
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
