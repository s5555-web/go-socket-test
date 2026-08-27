package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"sket/internal/auth"
	"sket/internal/store"
)

func fail(c *gin.Context, code int, msg string) { c.AbortWithStatusJSON(code, gin.H{"error": msg}) }
func (a *API) requireUser(admin bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		cl, err := a.auth.Parse(auth.Bearer(c.GetHeader("Authorization")))
		if err != nil || cl.UserID == 0 {
			fail(c, 401, "请先登录")
			return
		}
		if admin && !cl.Admin {
			fail(c, 403, "需要管理员权限")
			return
		}
		c.Set("uid", cl.UserID)
		c.Next()
	}
}
func uid(c *gin.Context) int64 { return c.MustGet("uid").(int64) }

func (a *API) register(c *gin.Context) {
	var in struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
		About       string `json:"about"`
	}
	if c.ShouldBindJSON(&in) != nil {
		fail(c, 400, "参数错误")
		return
	}
	in.Username = strings.ToLower(strings.TrimSpace(in.Username))
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	in.About = strings.TrimSpace(in.About)
	if len(in.Username) < 3 || len(in.Username) > 40 || len(in.Password) < 6 || in.DisplayName == "" || len([]rune(in.DisplayName)) > 80 || len([]rune(in.About)) > 160 {
		fail(c, 400, "用户名至少3位，密码至少6位，昵称不能为空")
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	res, err := a.store.DB.Exec(`INSERT INTO users(username,password_hash,display_name,about) VALUES(?,?,?,?)`, in.Username, string(hash), in.DisplayName, in.About)
	if err != nil {
		fail(c, 409, "用户名已存在")
		return
	}
	id, _ := res.LastInsertId()
	c.JSON(201, gin.H{"token": a.auth.Sign(id, false)})
}
func (a *API) login(c *gin.Context) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if c.ShouldBindJSON(&in) != nil {
		fail(c, 400, "参数错误")
		return
	}
	var id int64
	var hash string
	var admin bool
	err := a.store.DB.QueryRow(`SELECT id,password_hash,is_admin FROM users WHERE username=?`, strings.ToLower(strings.TrimSpace(in.Username))).Scan(&id, &hash, &admin)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil {
		fail(c, 401, "用户名或密码错误")
		return
	}
	c.JSON(200, gin.H{"token": a.auth.Sign(id, admin), "is_admin": admin})
}
func (a *API) me(c *gin.Context) {
	var u store.User
	err := a.store.DB.QueryRow(`SELECT id,username,display_name,COALESCE(about,''),avatar,COALESCE(public_key,''),COALESCE(encrypted_private_key,''),is_admin,created_at FROM users WHERE id=?`, uid(c)).Scan(&u.ID, &u.Username, &u.DisplayName, &u.About, &u.Avatar, &u.PublicKey, &u.KeyBackup, &u.IsAdmin, &u.CreatedAt)
	if err != nil {
		fail(c, 404, "用户不存在")
		return
	}
	c.JSON(200, u)
}

func (a *API) updateProfile(c *gin.Context) {
	var in struct {
		DisplayName string `json:"display_name"`
		About       string `json:"about"`
	}
	if c.ShouldBindJSON(&in) != nil {
		fail(c, 400, "资料格式错误")
		return
	}
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	in.About = strings.TrimSpace(in.About)
	if in.DisplayName == "" || len([]rune(in.DisplayName)) > 80 || len([]rune(in.About)) > 160 {
		fail(c, 400, "昵称不能为空，个人简介不能超过160字")
		return
	}
	if _, err := a.store.DB.Exec(`UPDATE users SET display_name=?,about=? WHERE id=?`, in.DisplayName, in.About, uid(c)); err != nil {
		fail(c, 500, "保存资料失败")
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *API) setPublicKey(c *gin.Context) {
	var in struct {
		PublicKey string `json:"public_key"`
		KeyBackup string `json:"key_backup"`
		Password  string `json:"password"`
		Replace   bool   `json:"replace"`
	}
	if c.ShouldBindJSON(&in) != nil || len(in.PublicKey) < 40 || len(in.PublicKey) > 4096 || len(in.KeyBackup) < 80 || len(in.KeyBackup) > 20000 {
		fail(c, 400, "公钥格式错误")
		return
	}
	if in.Replace {
		var hash string
		if a.store.DB.QueryRow(`SELECT password_hash FROM users WHERE id=?`, uid(c)).Scan(&hash) != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil {
			fail(c, 401, "密码验证失败")
			return
		}
	}
	query := `UPDATE users SET public_key=?,encrypted_private_key=? WHERE id=? AND (public_key IS NULL OR public_key='' OR public_key=?)`
	args := []interface{}{in.PublicKey, in.KeyBackup, uid(c), in.PublicKey}
	if in.Replace {
		query = `UPDATE users SET public_key=?,encrypted_private_key=? WHERE id=?`
		args = []interface{}{in.PublicKey, in.KeyBackup, uid(c)}
	}
	res, err := a.store.DB.Exec(query, args...)
	if err != nil {
		fail(c, 500, "保存公钥失败")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		fail(c, 409, "账号已绑定另一把加密密钥，拒绝覆盖")
		return
	}
	c.Status(204)
}
func (a *API) users(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	if query == "" {
		c.JSON(200, []store.User{})
		return
	}
	q := "%" + query + "%"
	rows, err := a.store.DB.Query(`SELECT u.id,u.username,u.display_name,COALESCE(u.about,''),u.avatar,u.is_admin,u.created_at,CASE WHEN f.status='accepted' THEN 'friend' WHEN f.status='pending' AND f.requested_by=? THEN 'outgoing' WHEN f.status='pending' THEN 'incoming' ELSE 'none' END FROM users u LEFT JOIN friendships f ON f.user_low_id=LEAST(u.id,?) AND f.user_high_id=GREATEST(u.id,?) WHERE u.id<>? AND (u.username LIKE ? OR u.display_name LIKE ?) ORDER BY u.display_name LIMIT 50`, uid(c), uid(c), uid(c), uid(c), q, q)
	if err != nil {
		fail(c, 500, "查询失败")
		return
	}
	defer rows.Close()
	out := []store.User{}
	for rows.Next() {
		var u store.User
		_ = rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.About, &u.Avatar, &u.IsAdmin, &u.CreatedAt, &u.Relationship)
		out = append(out, u)
	}
	c.JSON(200, out)
}

func friendPair(a, b int64) (int64, int64) {
	if a < b {
		return a, b
	}
	return b, a
}

func (a *API) areFriends(first, second int64) bool {
	low, high := friendPair(first, second)
	var n int
	return a.store.DB.QueryRow(`SELECT COUNT(*) FROM friendships WHERE user_low_id=? AND user_high_id=? AND status='accepted'`, low, high).Scan(&n) == nil && n == 1
}

func (a *API) friends(c *gin.Context) {
	rows, err := a.store.DB.Query(`SELECT u.id,u.username,u.display_name,COALESCE(u.about,''),u.avatar,u.is_admin,u.created_at FROM friendships f JOIN users u ON u.id=IF(f.user_low_id=?,f.user_high_id,f.user_low_id) WHERE (f.user_low_id=? OR f.user_high_id=?) AND f.status='accepted' ORDER BY u.display_name`, uid(c), uid(c), uid(c))
	if err != nil {
		fail(c, 500, "读取好友失败")
		return
	}
	defer rows.Close()
	out := []store.User{}
	for rows.Next() {
		var u store.User
		if rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.About, &u.Avatar, &u.IsAdmin, &u.CreatedAt) == nil {
			u.Relationship = "friend"
			out = append(out, u)
		}
	}
	c.JSON(200, out)
}

func (a *API) friendRequests(c *gin.Context) {
	rows, err := a.store.DB.Query(`SELECT u.id,u.username,u.display_name,COALESCE(u.about,''),u.avatar,u.is_admin,u.created_at FROM friendships f JOIN users u ON u.id=f.requested_by WHERE (f.user_low_id=? OR f.user_high_id=?) AND f.status='pending' AND f.requested_by<>? ORDER BY f.created_at DESC`, uid(c), uid(c), uid(c))
	if err != nil {
		fail(c, 500, "读取好友申请失败")
		return
	}
	defer rows.Close()
	out := []store.User{}
	for rows.Next() {
		var u store.User
		if rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.About, &u.Avatar, &u.IsAdmin, &u.CreatedAt) == nil {
			u.Relationship = "incoming"
			out = append(out, u)
		}
	}
	c.JSON(200, out)
}

func (a *API) requestFriend(c *gin.Context) {
	var in struct {
		UserID int64 `json:"user_id"`
	}
	if c.ShouldBindJSON(&in) != nil || in.UserID <= 0 || in.UserID == uid(c) {
		fail(c, 400, "好友账号无效")
		return
	}
	var exists int
	if a.store.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE id=?`, in.UserID).Scan(&exists) != nil || exists != 1 {
		fail(c, 404, "用户不存在")
		return
	}
	low, high := friendPair(uid(c), in.UserID)
	_, err := a.store.DB.Exec(`INSERT INTO friendships(user_low_id,user_high_id,requested_by,status) VALUES(?,?,?,'pending')`, low, high, uid(c))
	if err != nil {
		var status string
		var requestedBy int64
		if a.store.DB.QueryRow(`SELECT status,requested_by FROM friendships WHERE user_low_id=? AND user_high_id=?`, low, high).Scan(&status, &requestedBy) == nil {
			if status == "accepted" {
				fail(c, 409, "对方已经是你的好友")
			} else if requestedBy == in.UserID {
				fail(c, 409, "对方已向你发送申请，请在好友申请中确认")
			} else {
				fail(c, 409, "好友申请已发送")
			}
			return
		}
		fail(c, 500, "发送好友申请失败")
		return
	}
	a.hub.SendTo([]int64{in.UserID}, gin.H{"type": "friendship", "action": "requested", "user_id": uid(c)})
	c.Status(http.StatusCreated)
}

func (a *API) acceptFriend(c *gin.Context) {
	otherID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	low, high := friendPair(uid(c), otherID)
	if err != nil || otherID <= 0 || otherID == uid(c) {
		fail(c, 400, "好友申请无效")
		return
	}
	result, err := a.store.DB.Exec(`UPDATE friendships SET status='accepted' WHERE user_low_id=? AND user_high_id=? AND requested_by=? AND status='pending'`, low, high, otherID)
	if err != nil {
		fail(c, 500, "接受好友失败")
		return
	}
	updated, _ := result.RowsAffected()
	if updated != 1 {
		fail(c, 404, "好友申请不存在")
		return
	}
	a.hub.SendTo([]int64{otherID}, gin.H{"type": "friendship", "action": "accepted", "user_id": uid(c)})
	c.Status(http.StatusNoContent)
}

func (a *API) removeFriend(c *gin.Context) {
	otherID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	low, high := friendPair(uid(c), otherID)
	if err != nil || otherID <= 0 || otherID == uid(c) {
		fail(c, 400, "好友账号无效")
		return
	}
	result, err := a.store.DB.Exec(`DELETE FROM friendships WHERE user_low_id=? AND user_high_id=?`, low, high)
	if err != nil {
		fail(c, 500, "处理好友关系失败")
		return
	}
	deleted, _ := result.RowsAffected()
	if deleted != 1 {
		fail(c, 404, "好友关系不存在")
		return
	}
	a.hub.SendTo([]int64{otherID}, gin.H{"type": "friendship", "action": "removed", "user_id": uid(c)})
	c.Status(http.StatusNoContent)
}

func (a *API) conversations(c *gin.Context) {
	rows, err := a.store.DB.Query(`SELECT x.id,IF(x.is_group,x.name,COALESCE(other.display_name,x.name)),x.is_group,COALESCE(m.body,''),m.created_at,GREATEST((SELECT COUNT(*) FROM messages um WHERE um.conversation_id=x.id AND um.id>cm.last_read_message_id AND um.sender_id<>? AND NOT EXISTS(SELECT 1 FROM message_deletions md WHERE md.message_id=um.id AND md.user_id=?)),IF(cm.manual_unread,1,0)),cm.manual_unread,cm.pinned,cm.archived,cm.muted_until FROM conversation_members cm JOIN conversations x ON x.id=cm.conversation_id LEFT JOIN conversation_members ocm ON ocm.conversation_id=x.id AND ocm.user_id<>? AND x.is_group=0 LEFT JOIN users other ON other.id=ocm.user_id LEFT JOIN messages m ON m.id=(SELECT MAX(mm.id) FROM messages mm WHERE mm.conversation_id=x.id AND NOT EXISTS(SELECT 1 FROM message_deletions md2 WHERE md2.message_id=mm.id AND md2.user_id=?)) WHERE cm.user_id=? AND cm.hidden=FALSE ORDER BY cm.pinned DESC,COALESCE(m.created_at,x.created_at) DESC`, uid(c), uid(c), uid(c), uid(c), uid(c))
	if err != nil {
		fail(c, 500, "查询失败")
		return
	}
	defer rows.Close()
	out := []store.Conversation{}
	for rows.Next() {
		var x store.Conversation
		_ = rows.Scan(&x.ID, &x.Name, &x.IsGroup, &x.LastMessage, &x.LastAt, &x.Unread, &x.ManualUnread, &x.Pinned, &x.Archived, &x.MutedUntil)
		x.Muted = x.MutedUntil != nil && x.MutedUntil.After(time.Now())
		out = append(out, x)
	}
	c.JSON(200, out)
}
func (a *API) createConversation(c *gin.Context) {
	var in struct {
		Name      string  `json:"name"`
		MemberIDs []int64 `json:"member_ids"`
	}
	if c.ShouldBindJSON(&in) != nil || len(in.MemberIDs) == 0 {
		fail(c, 400, "请选择联系人")
		return
	}
	members := []int64{uid(c)}
	seen := map[int64]bool{uid(c): true}
	for _, id := range in.MemberIDs {
		if id > 0 && !seen[id] {
			members = append(members, id)
			seen[id] = true
		}
	}
	for _, memberID := range members[1:] {
		if !a.areFriends(uid(c), memberID) {
			fail(c, 403, "只能与好友创建对话")
			return
		}
	}
	isGroup := len(members) > 2
	if isGroup && strings.TrimSpace(in.Name) == "" {
		fail(c, 400, "群聊需要名称")
		return
	}
	tx, err := a.store.DB.Begin()
	if err != nil {
		fail(c, 500, "创建失败")
		return
	}
	if !isGroup {
		var existing int64
		err = tx.QueryRow(`SELECT cm.conversation_id FROM conversation_members cm JOIN conversations x ON x.id=cm.conversation_id AND x.is_group=0 WHERE cm.user_id IN (?,?) GROUP BY cm.conversation_id HAVING COUNT(*)=2 LIMIT 1`, members[0], members[1]).Scan(&existing)
		if err == nil {
			_, _ = tx.Exec(`UPDATE conversation_members SET hidden=FALSE,archived=FALSE WHERE conversation_id=? AND user_id=?`, existing, uid(c))
			_ = tx.Commit()
			c.JSON(200, gin.H{"id": existing})
			return
		}
	}
	res, err := tx.Exec(`INSERT INTO conversations(name,is_group,created_by) VALUES(?,?,?)`, strings.TrimSpace(in.Name), isGroup, uid(c))
	if err != nil {
		_ = tx.Rollback()
		fail(c, 500, "创建失败")
		return
	}
	id, _ := res.LastInsertId()
	for _, m := range members {
		if _, err = tx.Exec(`INSERT INTO conversation_members(conversation_id,user_id) VALUES(?,?)`, id, m); err != nil {
			_ = tx.Rollback()
			fail(c, 400, "成员无效")
			return
		}
	}
	if tx.Commit() != nil {
		fail(c, 500, "创建失败")
		return
	}
	c.JSON(201, gin.H{"id": id})
}

func (a *API) deleteConversation(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || !a.member(id, uid(c)) {
		fail(c, 404, "会话不存在")
		return
	}
	if _, err = a.store.DB.Exec(`UPDATE conversation_members SET hidden=TRUE WHERE conversation_id=? AND user_id=?`, id, uid(c)); err != nil {
		fail(c, 500, "删除失败")
		return
	}
	c.Status(204)
}

func (a *API) updateConversationState(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	var in struct {
		Action      string `json:"action"`
		MuteSeconds int64  `json:"mute_seconds"`
	}
	if err != nil || c.ShouldBindJSON(&in) != nil || !a.member(id, uid(c)) {
		fail(c, 404, "会话不存在")
		return
	}
	var query string
	var args []interface{}
	switch in.Action {
	case "mark_unread":
		query = `UPDATE conversation_members SET manual_unread=TRUE WHERE conversation_id=? AND user_id=?`
	case "pin":
		query = `UPDATE conversation_members SET pinned=TRUE,archived=FALSE WHERE conversation_id=? AND user_id=?`
	case "unpin":
		query = `UPDATE conversation_members SET pinned=FALSE WHERE conversation_id=? AND user_id=?`
	case "archive":
		query = `UPDATE conversation_members SET archived=TRUE,pinned=FALSE WHERE conversation_id=? AND user_id=?`
	case "unarchive":
		query = `UPDATE conversation_members SET archived=FALSE WHERE conversation_id=? AND user_id=?`
	case "unmute":
		query = `UPDATE conversation_members SET muted_until=NULL WHERE conversation_id=? AND user_id=?`
	case "mute":
		var muteUntil time.Time
		if in.MuteSeconds == -1 {
			muteUntil = time.Date(2099, time.December, 31, 23, 59, 59, 0, time.Local)
		} else if in.MuteSeconds == 3600 || in.MuteSeconds == 28800 || in.MuteSeconds == 604800 {
			muteUntil = time.Now().Add(time.Duration(in.MuteSeconds) * time.Second)
		} else {
			fail(c, 400, "静音时长无效")
			return
		}
		query = `UPDATE conversation_members SET muted_until=? WHERE conversation_id=? AND user_id=?`
		args = append(args, muteUntil)
	default:
		fail(c, 400, "会话操作无效")
		return
	}
	args = append(args, id, uid(c))
	if _, err = a.store.DB.Exec(query, args...); err != nil {
		fail(c, 500, "更新会话失败")
		return
	}
	a.hub.SendTo([]int64{uid(c)}, gin.H{"type": "conversation", "action": "state", "conversation_id": id})
	c.Status(http.StatusNoContent)
}

func (a *API) clearConversationMessages(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || !a.member(id, uid(c)) {
		fail(c, 404, "会话不存在")
		return
	}
	tx, err := a.store.DB.Begin()
	if err != nil {
		fail(c, 500, "清除失败")
		return
	}
	rollback := func() {
		_ = tx.Rollback()
	}
	memberRows, err := tx.Query(`SELECT user_id FROM conversation_members WHERE conversation_id=? FOR UPDATE`, id)
	if err != nil {
		rollback()
		fail(c, 500, "清除失败")
		return
	}
	members := []int64{}
	for memberRows.Next() {
		var memberID int64
		if memberRows.Scan(&memberID) == nil {
			members = append(members, memberID)
		}
	}
	_ = memberRows.Close()
	attachmentRows, err := tx.Query(`SELECT id FROM encrypted_attachments WHERE conversation_id=? AND message_id IS NOT NULL FOR UPDATE`, id)
	if err != nil {
		rollback()
		fail(c, 500, "清除失败")
		return
	}
	attachmentIDs := []string{}
	for attachmentRows.Next() {
		var attachmentID string
		if attachmentRows.Scan(&attachmentID) == nil {
			attachmentIDs = append(attachmentIDs, attachmentID)
		}
	}
	_ = attachmentRows.Close()
	if _, err = tx.Exec(`DELETE md FROM message_deletions md JOIN messages m ON m.id=md.message_id WHERE m.conversation_id=?`, id); err == nil {
		_, err = tx.Exec(`DELETE FROM encrypted_attachments WHERE conversation_id=? AND message_id IS NOT NULL`, id)
	}
	if err == nil {
		_, err = tx.Exec(`DELETE FROM messages WHERE conversation_id=?`, id)
	}
	if err == nil {
		_, err = tx.Exec(`UPDATE conversation_members SET last_read_message_id=0,manual_unread=FALSE WHERE conversation_id=?`, id)
	}
	if err != nil || tx.Commit() != nil {
		rollback()
		fail(c, 500, "清除失败")
		return
	}
	for _, attachmentID := range attachmentIDs {
		_ = os.Remove(filepath.Join(a.attachmentDir, attachmentID+".bin"))
	}
	a.hub.SendTo(members, gin.H{"type": "conversation", "action": "cleared", "conversation_id": id, "actor_id": uid(c)})
	c.Status(http.StatusNoContent)
}
func (a *API) member(conversation, user int64) bool {
	var n int
	return a.store.DB.QueryRow(`SELECT COUNT(*) FROM conversation_members WHERE conversation_id=? AND user_id=?`, conversation, user).Scan(&n) == nil && n > 0
}
func (a *API) messages(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || !a.member(id, uid(c)) {
		fail(c, 403, "无权访问")
		return
	}
	rows, err := a.store.DB.Query(`SELECT m.id,m.conversation_id,m.sender_id,u.display_name,m.body,m.created_at FROM messages m JOIN users u ON u.id=m.sender_id WHERE m.conversation_id=? AND NOT EXISTS(SELECT 1 FROM message_deletions md WHERE md.message_id=m.id AND md.user_id=?) ORDER BY m.id DESC LIMIT 100`, id, uid(c))
	if err != nil {
		fail(c, 500, "查询失败")
		return
	}
	defer rows.Close()
	rev := []store.Message{}
	for rows.Next() {
		var m store.Message
		_ = rows.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.SenderName, &m.Body, &m.CreatedAt)
		rev = append(rev, m)
	}
	out := make([]store.Message, len(rev))
	for i := range rev {
		out[len(rev)-1-i] = rev[i]
	}
	c.JSON(200, out)
}

func (a *API) deleteMessage(c *gin.Context) {
	conversationID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	messageID, messageErr := strconv.ParseInt(c.Param("message"), 10, 64)
	if err != nil || messageErr != nil || messageID <= 0 || !a.member(conversationID, uid(c)) {
		fail(c, 404, "消息不存在")
		return
	}
	var senderID int64
	var attachmentID string
	err = a.store.DB.QueryRow(`SELECT m.sender_id,COALESCE(ea.id,'') FROM messages m LEFT JOIN encrypted_attachments ea ON ea.message_id=m.id WHERE m.id=? AND m.conversation_id=? LIMIT 1`, messageID, conversationID).Scan(&senderID, &attachmentID)
	if err != nil {
		fail(c, 404, "消息不存在")
		return
	}
	scope := c.DefaultQuery("scope", "self")
	if scope == "self" {
		if _, err = a.store.DB.Exec(`INSERT IGNORE INTO message_deletions(message_id,user_id) VALUES(?,?)`, messageID, uid(c)); err != nil {
			fail(c, 500, "删除消息失败")
			return
		}
		a.hub.SendTo([]int64{uid(c)}, gin.H{"type": "conversation", "action": "message_deleted", "scope": "self", "conversation_id": conversationID, "message_id": messageID, "actor_id": uid(c)})
		c.Status(http.StatusNoContent)
		return
	}
	if scope != "all" {
		fail(c, 400, "删除范围无效")
		return
	}
	if senderID != uid(c) {
		fail(c, 403, "只能撤回自己发送的消息")
		return
	}
	tx, err := a.store.DB.Begin()
	if err != nil {
		fail(c, 500, "撤回消息失败")
		return
	}
	memberRows, err := tx.Query(`SELECT user_id FROM conversation_members WHERE conversation_id=? FOR UPDATE`, conversationID)
	if err != nil {
		_ = tx.Rollback()
		fail(c, 500, "撤回消息失败")
		return
	}
	members := []int64{}
	for memberRows.Next() {
		var memberID int64
		if memberRows.Scan(&memberID) == nil {
			members = append(members, memberID)
		}
	}
	_ = memberRows.Close()
	if _, err = tx.Exec(`DELETE FROM message_deletions WHERE message_id=?`, messageID); err == nil {
		_, err = tx.Exec(`DELETE FROM encrypted_attachments WHERE message_id=?`, messageID)
	}
	var deleted int64
	if err == nil {
		var result sql.Result
		result, err = tx.Exec(`DELETE FROM messages WHERE id=? AND conversation_id=? AND sender_id=?`, messageID, conversationID, uid(c))
		if err == nil {
			deleted, _ = result.RowsAffected()
		}
	}
	if err != nil || deleted != 1 || tx.Commit() != nil {
		_ = tx.Rollback()
		fail(c, 500, "撤回消息失败")
		return
	}
	if attachmentID != "" {
		_ = os.Remove(filepath.Join(a.attachmentDir, attachmentID+".bin"))
	}
	a.hub.SendTo(members, gin.H{"type": "conversation", "action": "message_deleted", "scope": "all", "conversation_id": conversationID, "message_id": messageID, "actor_id": uid(c)})
	c.Status(http.StatusNoContent)
}

func (a *API) conversationMembers(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || !a.member(id, uid(c)) {
		fail(c, 403, "无权访问")
		return
	}
	rows, err := a.store.DB.Query(`SELECT u.id,u.display_name,COALESCE(u.public_key,'') FROM conversation_members cm JOIN users u ON u.id=cm.user_id WHERE cm.conversation_id=? ORDER BY u.id`, id)
	if err != nil {
		fail(c, 500, "查询失败")
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var memberID int64
		var name, key string
		_ = rows.Scan(&memberID, &name, &key)
		out = append(out, gin.H{"id": memberID, "display_name": name, "public_key": key})
	}
	c.JSON(200, out)
}

const maxEncryptedAttachmentSize = 8*1024*1024 + 64

func (a *API) uploadAttachment(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || !a.member(id, uid(c)) {
		fail(c, 403, "无权上传图片")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxEncryptedAttachmentSize)
	token := make([]byte, 16)
	if _, err = rand.Read(token); err != nil {
		fail(c, 500, "生成附件编号失败")
		return
	}
	attachmentID := hex.EncodeToString(token)
	path := filepath.Join(a.attachmentDir, attachmentID+".bin")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		fail(c, 500, "保存加密图片失败")
		return
	}
	size, copyErr := io.Copy(file, c.Request.Body)
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil || size < 17 || size > maxEncryptedAttachmentSize {
		_ = os.Remove(path)
		fail(c, 400, "加密图片无效或超过 8MB")
		return
	}
	if _, err = a.store.DB.Exec(`INSERT INTO encrypted_attachments(id,conversation_id,uploader_id,cipher_size) VALUES(?,?,?,?)`, attachmentID, id, uid(c), size); err != nil {
		_ = os.Remove(path)
		fail(c, 500, "记录加密图片失败")
		return
	}
	c.JSON(201, gin.H{"id": attachmentID, "cipher_size": size})
}

func (a *API) downloadAttachment(c *gin.Context) {
	conversationID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	attachmentID := c.Param("attachment")
	if err != nil || len(attachmentID) != 32 || !a.member(conversationID, uid(c)) {
		fail(c, 403, "无权查看图片")
		return
	}
	if _, err = hex.DecodeString(attachmentID); err != nil {
		fail(c, 404, "图片不存在")
		return
	}
	var size int64
	err = a.store.DB.QueryRow(`SELECT cipher_size FROM encrypted_attachments WHERE id=? AND conversation_id=? AND message_id IS NOT NULL`, attachmentID, conversationID).Scan(&size)
	if err != nil {
		fail(c, 404, "图片不存在")
		return
	}
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Length", strconv.FormatInt(size, 10))
	c.File(filepath.Join(a.attachmentDir, attachmentID+".bin"))
}

func (a *API) deleteAttachment(c *gin.Context) {
	conversationID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	attachmentID := c.Param("attachment")
	if err != nil || len(attachmentID) != 32 || !a.member(conversationID, uid(c)) {
		fail(c, 403, "无权删除图片")
		return
	}
	if _, err = hex.DecodeString(attachmentID); err != nil {
		fail(c, 404, "图片不存在")
		return
	}
	result, err := a.store.DB.Exec(`DELETE FROM encrypted_attachments WHERE id=? AND conversation_id=? AND uploader_id=? AND message_id IS NULL`, attachmentID, conversationID, uid(c))
	if err != nil {
		fail(c, 500, "删除图片失败")
		return
	}
	deleted, _ := result.RowsAffected()
	if deleted != 1 {
		fail(c, 404, "图片不存在或已经发送")
		return
	}
	_ = os.Remove(filepath.Join(a.attachmentDir, attachmentID+".bin"))
	c.Status(http.StatusNoContent)
}

type encryptedRecipientBox struct {
	IV string `json:"iv"`
	CT string `json:"ct"`
}

type encryptedEnvelope struct {
	V            int                              `json:"v"`
	Alg          string                           `json:"alg"`
	AttachmentID string                           `json:"attachment_id"`
	Recipients   map[string]encryptedRecipientBox `json:"recipients"`
}

func validateEncryptedEnvelope(value string) (encryptedEnvelope, error) {
	var envelope encryptedEnvelope
	if value == "" || len(value) > 131072 || json.Unmarshal([]byte(value), &envelope) != nil {
		return envelope, fmt.Errorf("invalid encrypted envelope")
	}
	if envelope.V != 1 || envelope.Alg != "P256-HKDF-A256GCM" || len(envelope.Recipients) == 0 || len(envelope.Recipients) > 256 {
		return envelope, fmt.Errorf("unsupported encrypted envelope")
	}
	for recipientID, box := range envelope.Recipients {
		id, err := strconv.ParseInt(recipientID, 10, 64)
		if err != nil || id <= 0 {
			return envelope, fmt.Errorf("invalid recipient")
		}
		iv, ivErr := base64.StdEncoding.DecodeString(box.IV)
		ciphertext, cipherErr := base64.StdEncoding.DecodeString(box.CT)
		if ivErr != nil || cipherErr != nil || len(iv) != 12 || len(ciphertext) < 17 {
			return envelope, fmt.Errorf("invalid AES-GCM box")
		}
	}
	if envelope.AttachmentID != "" {
		decoded, err := hex.DecodeString(envelope.AttachmentID)
		if err != nil || len(decoded) != 16 {
			return envelope, fmt.Errorf("invalid attachment id")
		}
	}
	return envelope, nil
}

func (a *API) sendMessage(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	var in struct {
		Body string `json:"body"`
	}
	if err != nil || c.ShouldBindJSON(&in) != nil || !a.member(id, uid(c)) {
		fail(c, 403, "无法发送")
		return
	}
	in.Body = strings.TrimSpace(in.Body)
	envelope, envelopeErr := validateEncryptedEnvelope(in.Body)
	if envelopeErr != nil {
		fail(c, 400, "加密消息无效或过大")
		return
	}
	rows, err := a.store.DB.Query(`SELECT user_id,COALESCE(muted_until>NOW(),FALSE) FROM conversation_members WHERE conversation_id=?`, id)
	if err != nil {
		fail(c, 500, "校验收件人失败")
		return
	}
	defer rows.Close()
	expected := 0
	ids := []int64{}
	pushIDs := []int64{}
	for rows.Next() {
		var memberID int64
		var muted bool
		_ = rows.Scan(&memberID, &muted)
		ids = append(ids, memberID)
		if !muted {
			pushIDs = append(pushIDs, memberID)
		}
		expected++
		if _, ok := envelope.Recipients[strconv.FormatInt(memberID, 10)]; !ok {
			fail(c, 400, "加密消息未覆盖全部会话成员")
			return
		}
	}
	if len(envelope.Recipients) != expected {
		fail(c, 400, "加密消息收件人不匹配")
		return
	}
	if envelope.AttachmentID != "" {
		var n int
		if a.store.DB.QueryRow(`SELECT COUNT(*) FROM encrypted_attachments WHERE id=? AND conversation_id=? AND uploader_id=? AND message_id IS NULL`, envelope.AttachmentID, id, uid(c)).Scan(&n) != nil || n != 1 {
			fail(c, 400, "加密图片不存在或已发送")
			return
		}
	}
	res, err := a.store.DB.Exec(`INSERT INTO messages(conversation_id,sender_id,body) VALUES(?,?,?)`, id, uid(c), in.Body)
	if err != nil {
		fail(c, 500, "发送失败")
		return
	}
	mid, _ := res.LastInsertId()
	if envelope.AttachmentID != "" {
		result, updateErr := a.store.DB.Exec(`UPDATE encrypted_attachments SET message_id=? WHERE id=? AND conversation_id=? AND uploader_id=? AND message_id IS NULL`, mid, envelope.AttachmentID, id, uid(c))
		var updated int64
		if updateErr == nil {
			updated, _ = result.RowsAffected()
		}
		if updateErr != nil || updated != 1 {
			_, _ = a.store.DB.Exec(`DELETE FROM messages WHERE id=?`, mid)
			fail(c, 409, "加密图片发送冲突，请重试")
			return
		}
	}
	_, _ = a.store.DB.Exec(`UPDATE conversation_members SET hidden=FALSE,archived=FALSE WHERE conversation_id=?`, id)
	_, _ = a.store.DB.Exec(`UPDATE conversation_members SET last_read_message_id=?,manual_unread=FALSE WHERE conversation_id=? AND user_id=?`, mid, id, uid(c))
	var m store.Message
	_ = a.store.DB.QueryRow(`SELECT m.id,m.conversation_id,m.sender_id,u.display_name,m.body,m.created_at FROM messages m JOIN users u ON u.id=m.sender_id WHERE m.id=?`, mid).Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.SenderName, &m.Body, &m.CreatedAt)
	a.hub.SendTo(ids, gin.H{"type": "message", "data": m})
	go a.sendPush(pushIDs, uid(c), m)
	c.JSON(201, m)
}

type pushSubscriptionInput struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

func (a *API) pushConfig(c *gin.Context) {
	c.JSON(200, gin.H{"enabled": a.push.PublicKey != "" && a.push.PrivateKey != "", "public_key": a.push.PublicKey})
}

func (a *API) savePushSubscription(c *gin.Context) {
	var in pushSubscriptionInput
	if c.ShouldBindJSON(&in) != nil || !strings.HasPrefix(in.Endpoint, "https://") || len(in.Endpoint) > 8192 || len(in.Keys.P256dh) < 40 || len(in.Keys.Auth) < 8 {
		fail(c, 400, "推送订阅格式错误")
		return
	}
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(in.Endpoint)))
	_, err := a.store.DB.Exec(`INSERT INTO push_subscriptions(endpoint_hash,user_id,endpoint,p256dh,auth) VALUES(?,?,?,?,?) ON DUPLICATE KEY UPDATE user_id=VALUES(user_id),endpoint=VALUES(endpoint),p256dh=VALUES(p256dh),auth=VALUES(auth)`, hash, uid(c), in.Endpoint, in.Keys.P256dh, in.Keys.Auth)
	if err != nil {
		fail(c, 500, "保存推送订阅失败")
		return
	}
	c.Status(204)
}

func (a *API) deletePushSubscription(c *gin.Context) {
	var in struct {
		Endpoint string `json:"endpoint"`
	}
	if c.ShouldBindJSON(&in) != nil || in.Endpoint == "" {
		fail(c, 400, "推送订阅格式错误")
		return
	}
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(in.Endpoint)))
	_, _ = a.store.DB.Exec(`DELETE FROM push_subscriptions WHERE endpoint_hash=? AND user_id=?`, hash, uid(c))
	c.Status(204)
}

func (a *API) sendPush(userIDs []int64, senderID int64, m store.Message) {
	if a.push.PublicKey == "" || a.push.PrivateKey == "" {
		return
	}
	ids := make([]interface{}, 0, len(userIDs))
	marks := make([]string, 0, len(userIDs))
	for _, id := range userIDs {
		if id != senderID {
			ids = append(ids, id)
			marks = append(marks, "?")
		}
	}
	if len(ids) == 0 {
		return
	}
	rows, err := a.store.DB.Query(`SELECT endpoint_hash,endpoint,p256dh,auth FROM push_subscriptions WHERE user_id IN (`+strings.Join(marks, ",")+`)`, ids...)
	if err != nil {
		return
	}
	defer rows.Close()
	payload, _ := json.Marshal(gin.H{"title": m.SenderName, "body": "发来一条端到端加密消息", "conversation_id": m.ConversationID, "url": fmt.Sprintf("/?conversation=%d", m.ConversationID)})
	for rows.Next() {
		var hash, endpoint, p256dh, authKey string
		if rows.Scan(&hash, &endpoint, &p256dh, &authKey) != nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		resp, err := webpush.SendNotificationWithContext(ctx, payload, &webpush.Subscription{Endpoint: endpoint, Keys: webpush.Keys{P256dh: p256dh, Auth: authKey}}, &webpush.Options{Subscriber: a.push.Subject, VAPIDPublicKey: a.push.PublicKey, VAPIDPrivateKey: a.push.PrivateKey, TTL: 60})
		cancel()
		if resp != nil {
			_ = resp.Body.Close()
		}
		if resp != nil && (resp.StatusCode == 404 || resp.StatusCode == 410) {
			_, _ = a.store.DB.Exec(`DELETE FROM push_subscriptions WHERE endpoint_hash=?`, hash)
		}
		_ = err
	}
}
func (a *API) readConversation(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if !a.member(id, uid(c)) {
		fail(c, 403, "无权访问")
		return
	}
	_, _ = a.store.DB.Exec(`UPDATE conversation_members SET last_read_message_id=COALESCE((SELECT MAX(id) FROM messages WHERE conversation_id=?),0),manual_unread=FALSE WHERE conversation_id=? AND user_id=?`, id, id, uid(c))
	c.Status(204)
}

func (a *API) adminStats(c *gin.Context) {
	var users, convs, msgs int64
	_ = a.store.DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&users)
	_ = a.store.DB.QueryRow(`SELECT COUNT(*) FROM conversations`).Scan(&convs)
	_ = a.store.DB.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&msgs)
	c.JSON(200, gin.H{"users": users, "conversations": convs, "messages": msgs})
}
func (a *API) adminUsers(c *gin.Context) {
	rows, err := a.store.DB.Query(`SELECT id,username,display_name,COALESCE(about,''),avatar,is_admin,created_at FROM users ORDER BY id DESC LIMIT 200`)
	if err != nil {
		fail(c, 500, "查询失败")
		return
	}
	defer rows.Close()
	out := []store.User{}
	for rows.Next() {
		var u store.User
		_ = rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.About, &u.Avatar, &u.IsAdmin, &u.CreatedAt)
		out = append(out, u)
	}
	c.JSON(200, out)
}
func (a *API) deleteUser(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if id == uid(c) {
		fail(c, 400, "不能删除自己")
		return
	}
	tx, err := a.store.DB.Begin()
	if err != nil {
		fail(c, 500, "删除失败")
		return
	}
	_, _ = tx.Exec(`DELETE md FROM message_deletions md JOIN messages m ON m.id=md.message_id WHERE m.sender_id=?`, id)
	_, _ = tx.Exec(`DELETE FROM message_deletions WHERE user_id=?`, id)
	_, _ = tx.Exec(`DELETE FROM messages WHERE sender_id=?`, id)
	_, _ = tx.Exec(`DELETE FROM conversation_members WHERE user_id=?`, id)
	_, _ = tx.Exec(`DELETE FROM push_subscriptions WHERE user_id=?`, id)
	_, _ = tx.Exec(`DELETE FROM friendships WHERE user_low_id=? OR user_high_id=?`, id, id)
	res, err := tx.Exec(`DELETE FROM users WHERE id=?`, id)
	if err != nil {
		_ = tx.Rollback()
		fail(c, 500, "删除失败")
		return
	}
	_ = tx.Commit()
	n, _ := res.RowsAffected()
	if n == 0 {
		fail(c, 404, "用户不存在")
		return
	}
	c.Status(204)
}
