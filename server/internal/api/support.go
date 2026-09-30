package api

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"sket/internal/auth"
)

type supportSite struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	SiteKey        string    `json:"site_key"`
	AllowedOrigin  string    `json:"allowed_origin"`
	WelcomeMessage string    `json:"welcome_message"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
}

type supportMessage struct {
	ID           int64     `json:"id"`
	ThreadID     int64     `json:"thread_id"`
	SenderType   string    `json:"sender_type"`
	SenderUserID *int64    `json:"sender_user_id,omitempty"`
	SenderName   string    `json:"sender_name"`
	Body         string    `json:"body"`
	CreatedAt    time.Time `json:"created_at"`
}

type supportThread struct {
	ID               int64     `json:"id"`
	SiteID           int64     `json:"site_id"`
	SiteName         string    `json:"site_name"`
	VisitorID        string    `json:"visitor_id"`
	VisitorName      string    `json:"visitor_name"`
	AssignedUserID   *int64    `json:"assigned_user_id,omitempty"`
	AssignedUserName string    `json:"assigned_user_name"`
	Status           string    `json:"status"`
	LastMessage      string    `json:"last_message"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func randomHex(bytes int) (string, error) {
	value := make([]byte, bytes)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func tokenHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func originAllowed(allowed, origin string) bool {
	if origin == "" {
		return true
	}
	for _, candidate := range strings.FieldsFunc(allowed, func(r rune) bool { return r == ',' || r == '\n' || r == ' ' }) {
		candidate = strings.TrimRight(strings.TrimSpace(candidate), "/")
		if candidate == "*" || strings.EqualFold(candidate, strings.TrimRight(origin, "/")) {
			return true
		}
	}
	return false
}

func (a *API) widgetCORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
		}
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Max-Age", "86400")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func (a *API) siteByKey(key string) (supportSite, error) {
	var site supportSite
	err := a.store.DB.QueryRow(`SELECT id,name,site_key,allowed_origin,welcome_message,enabled,created_at FROM support_sites WHERE site_key=?`, key).Scan(&site.ID, &site.Name, &site.SiteKey, &site.AllowedOrigin, &site.WelcomeMessage, &site.Enabled, &site.CreatedAt)
	return site, err
}

func (a *API) widgetConfig(c *gin.Context) {
	site, err := a.siteByKey(strings.TrimSpace(c.Query("site_key")))
	if err != nil || !site.Enabled {
		fail(c, 404, "客服站点不存在或已停用")
		return
	}
	if !originAllowed(site.AllowedOrigin, c.GetHeader("Origin")) {
		fail(c, 403, "当前网站未获准使用此客服插件")
		return
	}
	c.JSON(200, gin.H{"name": site.Name, "welcome_message": site.WelcomeMessage})
}

func (a *API) createWidgetSession(c *gin.Context) {
	var in struct {
		SiteKey     string `json:"site_key"`
		DisplayName string `json:"display_name"`
	}
	if c.ShouldBindJSON(&in) != nil {
		fail(c, 400, "参数错误")
		return
	}
	site, err := a.siteByKey(strings.TrimSpace(in.SiteKey))
	if err != nil || !site.Enabled || !originAllowed(site.AllowedOrigin, c.GetHeader("Origin")) {
		fail(c, 403, "客服插件不可用")
		return
	}
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	if in.DisplayName == "" {
		in.DisplayName = "游客"
	}
	if len([]rune(in.DisplayName)) > 80 {
		fail(c, 400, "访客名称不能超过80字")
		return
	}
	visitorID, err := randomHex(16)
	if err != nil {
		fail(c, 500, "创建访客会话失败")
		return
	}
	token, err := randomHex(32)
	if err != nil {
		fail(c, 500, "创建访客会话失败")
		return
	}
	tx, err := a.store.DB.Begin()
	if err != nil {
		fail(c, 500, "创建访客会话失败")
		return
	}
	if _, err = tx.Exec(`INSERT INTO support_visitors(id,site_id,token_hash,display_name) VALUES(?,?,?,?)`, visitorID, site.ID, tokenHash(token), in.DisplayName); err != nil {
		_ = tx.Rollback()
		fail(c, 500, "创建访客会话失败")
		return
	}
	result, err := tx.Exec(`INSERT INTO support_threads(site_id,visitor_id) VALUES(?,?)`, site.ID, visitorID)
	if err != nil {
		_ = tx.Rollback()
		fail(c, 500, "创建访客会话失败")
		return
	}
	threadID, _ := result.LastInsertId()
	if strings.TrimSpace(site.WelcomeMessage) != "" {
		_, err = tx.Exec(`INSERT INTO support_messages(thread_id,sender_type,body) VALUES(?,'system',?)`, threadID, site.WelcomeMessage)
	}
	if err != nil || tx.Commit() != nil {
		_ = tx.Rollback()
		fail(c, 500, "创建访客会话失败")
		return
	}
	c.JSON(201, gin.H{"visitor_token": token, "thread_id": threadID, "site_name": site.Name})
}

func visitorToken(c *gin.Context) string {
	return auth.Bearer(c.GetHeader("Authorization"))
}

func (a *API) visitorThread(c *gin.Context) (supportThread, error) {
	var thread supportThread
	err := a.store.DB.QueryRow(`SELECT t.id,t.site_id,s.name,t.visitor_id,v.display_name,t.assigned_user_id,COALESCE(u.display_name,''),t.status,'',t.created_at,t.updated_at FROM support_threads t JOIN support_visitors v ON v.id=t.visitor_id JOIN support_sites s ON s.id=t.site_id LEFT JOIN users u ON u.id=t.assigned_user_id WHERE v.token_hash=? ORDER BY t.id DESC LIMIT 1`, tokenHash(visitorToken(c))).Scan(&thread.ID, &thread.SiteID, &thread.SiteName, &thread.VisitorID, &thread.VisitorName, &thread.AssignedUserID, &thread.AssignedUserName, &thread.Status, &thread.LastMessage, &thread.CreatedAt, &thread.UpdatedAt)
	if err == nil {
		_, _ = a.store.DB.Exec(`UPDATE support_visitors SET last_seen=NOW() WHERE id=?`, thread.VisitorID)
	}
	return thread, err
}

func (a *API) readSupportMessages(threadID int64, after int64) ([]supportMessage, error) {
	rows, err := a.store.DB.Query(`SELECT m.id,m.thread_id,m.sender_type,m.sender_user_id,COALESCE(u.display_name,IF(m.sender_type='visitor','游客','客服')),m.body,m.created_at FROM support_messages m LEFT JOIN users u ON u.id=m.sender_user_id WHERE m.thread_id=? AND m.id>? ORDER BY m.id LIMIT 200`, threadID, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []supportMessage{}
	for rows.Next() {
		var message supportMessage
		var senderID sql.NullInt64
		if err := rows.Scan(&message.ID, &message.ThreadID, &message.SenderType, &senderID, &message.SenderName, &message.Body, &message.CreatedAt); err != nil {
			return nil, err
		}
		if senderID.Valid {
			message.SenderUserID = &senderID.Int64
		}
		out = append(out, message)
	}
	return out, rows.Err()
}

func (a *API) widgetMessages(c *gin.Context) {
	thread, err := a.visitorThread(c)
	if err != nil {
		fail(c, 401, "访客会话已失效")
		return
	}
	site, err := a.siteByKeyForID(thread.SiteID)
	if err != nil || !site.Enabled || !originAllowed(site.AllowedOrigin, c.GetHeader("Origin")) {
		fail(c, 403, "当前网站未获准使用此客服插件")
		return
	}
	after, _ := strconv.ParseInt(c.Query("after"), 10, 64)
	messages, err := a.readSupportMessages(thread.ID, after)
	if err != nil {
		fail(c, 500, "读取消息失败")
		return
	}
	c.JSON(200, gin.H{"thread": thread, "messages": messages})
}

func (a *API) siteByKeyForID(id int64) (supportSite, error) {
	var site supportSite
	err := a.store.DB.QueryRow(`SELECT id,name,site_key,allowed_origin,welcome_message,enabled,created_at FROM support_sites WHERE id=?`, id).Scan(&site.ID, &site.Name, &site.SiteKey, &site.AllowedOrigin, &site.WelcomeMessage, &site.Enabled, &site.CreatedAt)
	return site, err
}

func (a *API) sendWidgetMessage(c *gin.Context) {
	thread, err := a.visitorThread(c)
	if err != nil || thread.Status != "open" {
		fail(c, 409, "客服会话已关闭，请重新发起咨询")
		return
	}
	site, siteErr := a.siteByKeyForID(thread.SiteID)
	if siteErr != nil || !site.Enabled || !originAllowed(site.AllowedOrigin, c.GetHeader("Origin")) {
		fail(c, 403, "当前网站未获准使用此客服插件")
		return
	}
	var in struct {
		Body string `json:"body"`
	}
	if c.ShouldBindJSON(&in) != nil {
		fail(c, 400, "参数错误")
		return
	}
	in.Body = strings.TrimSpace(in.Body)
	if in.Body == "" || len([]rune(in.Body)) > 2000 {
		fail(c, 400, "消息不能为空且不能超过2000字")
		return
	}
	result, err := a.store.DB.Exec(`INSERT INTO support_messages(thread_id,sender_type,body) VALUES(?,'visitor',?)`, thread.ID, in.Body)
	if err != nil {
		fail(c, 500, "发送失败")
		return
	}
	_, _ = a.store.DB.Exec(`UPDATE support_threads SET updated_at=NOW() WHERE id=?`, thread.ID)
	messageID, _ := result.LastInsertId()
	message := supportMessage{ID: messageID, ThreadID: thread.ID, SenderType: "visitor", SenderName: thread.VisitorName, Body: in.Body, CreatedAt: time.Now()}
	rows, _ := a.store.DB.Query(`SELECT id FROM users WHERE is_support=TRUE`)
	agents := []int64{}
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			if rows.Scan(&id) == nil {
				agents = append(agents, id)
			}
		}
	}
	a.hub.SendTo(agents, gin.H{"type": "support_message", "data": message, "thread_id": thread.ID})
	c.JSON(201, message)
}

func (a *API) requireSupport() gin.HandlerFunc {
	return func(c *gin.Context) {
		var allowed bool
		if a.store.DB.QueryRow(`SELECT is_support FROM users WHERE id=?`, uid(c)).Scan(&allowed) != nil || !allowed {
			fail(c, 403, "需要客服权限")
			return
		}
		c.Next()
	}
}

func (a *API) supportThreads(c *gin.Context) {
	rows, err := a.store.DB.Query(`SELECT t.id,t.site_id,s.name,t.visitor_id,v.display_name,t.assigned_user_id,COALESCE(u.display_name,''),t.status,COALESCE((SELECT body FROM support_messages WHERE thread_id=t.id ORDER BY id DESC LIMIT 1),''),t.created_at,t.updated_at FROM support_threads t JOIN support_sites s ON s.id=t.site_id JOIN support_visitors v ON v.id=t.visitor_id LEFT JOIN users u ON u.id=t.assigned_user_id ORDER BY (t.status='open') DESC,t.updated_at DESC LIMIT 200`)
	if err != nil {
		fail(c, 500, "读取客服会话失败")
		return
	}
	defer rows.Close()
	out := []supportThread{}
	for rows.Next() {
		var thread supportThread
		var assigned sql.NullInt64
		if rows.Scan(&thread.ID, &thread.SiteID, &thread.SiteName, &thread.VisitorID, &thread.VisitorName, &assigned, &thread.AssignedUserName, &thread.Status, &thread.LastMessage, &thread.CreatedAt, &thread.UpdatedAt) == nil {
			if assigned.Valid {
				thread.AssignedUserID = &assigned.Int64
			}
			out = append(out, thread)
		}
	}
	c.JSON(200, out)
}

func (a *API) supportThreadAllowed(threadID int64) bool {
	var count int
	return a.store.DB.QueryRow(`SELECT COUNT(*) FROM support_threads WHERE id=?`, threadID).Scan(&count) == nil && count == 1
}

func (a *API) supportThreadMessages(c *gin.Context) {
	threadID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || !a.supportThreadAllowed(threadID) {
		fail(c, 404, "客服会话不存在")
		return
	}
	after, _ := strconv.ParseInt(c.Query("after"), 10, 64)
	messages, err := a.readSupportMessages(threadID, after)
	if err != nil {
		fail(c, 500, "读取消息失败")
		return
	}
	c.JSON(200, messages)
}

func (a *API) sendSupportMessage(c *gin.Context) {
	threadID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	var status string
	if err != nil || a.store.DB.QueryRow(`SELECT status FROM support_threads WHERE id=?`, threadID).Scan(&status) != nil {
		fail(c, 404, "客服会话不存在")
		return
	}
	if status != "open" {
		fail(c, 409, "客服会话已关闭")
		return
	}
	var in struct {
		Body string `json:"body"`
	}
	if c.ShouldBindJSON(&in) != nil {
		fail(c, 400, "参数错误")
		return
	}
	in.Body = strings.TrimSpace(in.Body)
	if in.Body == "" || len([]rune(in.Body)) > 2000 {
		fail(c, 400, "消息不能为空且不能超过2000字")
		return
	}
	tx, err := a.store.DB.Begin()
	if err != nil {
		fail(c, 500, "发送失败")
		return
	}
	_, err = tx.Exec(`UPDATE support_threads SET assigned_user_id=COALESCE(assigned_user_id,?),updated_at=NOW() WHERE id=?`, uid(c), threadID)
	var result sql.Result
	if err == nil {
		result, err = tx.Exec(`INSERT INTO support_messages(thread_id,sender_type,sender_user_id,body) VALUES(?,'agent',?,?)`, threadID, uid(c), in.Body)
	}
	if err != nil || tx.Commit() != nil {
		_ = tx.Rollback()
		fail(c, 500, "发送失败")
		return
	}
	messageID, _ := result.LastInsertId()
	var name string
	_ = a.store.DB.QueryRow(`SELECT display_name FROM users WHERE id=?`, uid(c)).Scan(&name)
	c.JSON(201, supportMessage{ID: messageID, ThreadID: threadID, SenderType: "agent", SenderUserID: func() *int64 { id := uid(c); return &id }(), SenderName: name, Body: in.Body, CreatedAt: time.Now()})
}

func (a *API) updateSupportThreadStatus(c *gin.Context) {
	threadID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	var in struct {
		Status string `json:"status"`
	}
	if err != nil || c.ShouldBindJSON(&in) != nil || (in.Status != "open" && in.Status != "closed") {
		fail(c, 400, "状态无效")
		return
	}
	result, err := a.store.DB.Exec(`UPDATE support_threads SET status=?,assigned_user_id=COALESCE(assigned_user_id,?),updated_at=NOW() WHERE id=?`, in.Status, uid(c), threadID)
	if err != nil {
		fail(c, 500, "更新失败")
		return
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		fail(c, 404, "客服会话不存在")
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *API) setSupportUser(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	var in struct {
		IsSupport bool `json:"is_support"`
	}
	if err != nil || c.ShouldBindJSON(&in) != nil {
		fail(c, 400, "参数错误")
		return
	}
	result, err := a.store.DB.Exec(`UPDATE users SET is_support=? WHERE id=?`, in.IsSupport, userID)
	if err != nil {
		fail(c, 500, "更新客服权限失败")
		return
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		var count int
		_ = a.store.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE id=?`, userID).Scan(&count)
		if count == 0 {
			fail(c, 404, "用户不存在")
			return
		}
	}
	c.Status(http.StatusNoContent)
}

func (a *API) adminSupportSites(c *gin.Context) {
	rows, err := a.store.DB.Query(`SELECT id,name,site_key,allowed_origin,welcome_message,enabled,created_at FROM support_sites ORDER BY id DESC`)
	if err != nil {
		fail(c, 500, "读取站点失败")
		return
	}
	defer rows.Close()
	out := []supportSite{}
	for rows.Next() {
		var site supportSite
		if rows.Scan(&site.ID, &site.Name, &site.SiteKey, &site.AllowedOrigin, &site.WelcomeMessage, &site.Enabled, &site.CreatedAt) == nil {
			out = append(out, site)
		}
	}
	c.JSON(200, out)
}

func (a *API) createSupportSite(c *gin.Context) {
	var in struct {
		Name           string `json:"name"`
		AllowedOrigin  string `json:"allowed_origin"`
		WelcomeMessage string `json:"welcome_message"`
	}
	if c.ShouldBindJSON(&in) != nil {
		fail(c, 400, "参数错误")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.AllowedOrigin = strings.TrimSpace(in.AllowedOrigin)
	in.WelcomeMessage = strings.TrimSpace(in.WelcomeMessage)
	if in.Name == "" || len([]rune(in.Name)) > 100 || in.AllowedOrigin == "" || len(in.AllowedOrigin) > 255 || len([]rune(in.WelcomeMessage)) > 500 {
		fail(c, 400, "站点名称、授权域名或欢迎语无效")
		return
	}
	key, err := randomHex(16)
	if err != nil {
		fail(c, 500, "创建站点失败")
		return
	}
	result, err := a.store.DB.Exec(`INSERT INTO support_sites(name,site_key,allowed_origin,welcome_message) VALUES(?,?,?,?)`, in.Name, key, in.AllowedOrigin, in.WelcomeMessage)
	if err != nil {
		fail(c, 500, "创建站点失败")
		return
	}
	id, _ := result.LastInsertId()
	c.JSON(201, gin.H{"id": id, "site_key": key})
}

func (a *API) updateSupportSite(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	var in struct {
		Name           string `json:"name"`
		AllowedOrigin  string `json:"allowed_origin"`
		WelcomeMessage string `json:"welcome_message"`
		Enabled        bool   `json:"enabled"`
	}
	if err != nil || c.ShouldBindJSON(&in) != nil {
		fail(c, 400, "参数错误")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.AllowedOrigin = strings.TrimSpace(in.AllowedOrigin)
	in.WelcomeMessage = strings.TrimSpace(in.WelcomeMessage)
	if in.Name == "" || in.AllowedOrigin == "" || len([]rune(in.Name)) > 100 || len(in.AllowedOrigin) > 255 || len([]rune(in.WelcomeMessage)) > 500 {
		fail(c, 400, "站点配置无效")
		return
	}
	result, err := a.store.DB.Exec(`UPDATE support_sites SET name=?,allowed_origin=?,welcome_message=?,enabled=? WHERE id=?`, in.Name, in.AllowedOrigin, in.WelcomeMessage, in.Enabled, id)
	if err != nil {
		fail(c, 500, "保存站点失败")
		return
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		var count int
		_ = a.store.DB.QueryRow(`SELECT COUNT(*) FROM support_sites WHERE id=?`, id).Scan(&count)
		if count == 0 {
			fail(c, 404, "站点不存在")
			return
		}
	}
	c.Status(http.StatusNoContent)
}
