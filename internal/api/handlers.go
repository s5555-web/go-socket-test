package api

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"sket/internal/auth"
	"sket/internal/store"
	"strconv"
	"strings"
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
	}
	if c.ShouldBindJSON(&in) != nil {
		fail(c, 400, "参数错误")
		return
	}
	in.Username = strings.ToLower(strings.TrimSpace(in.Username))
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	if len(in.Username) < 3 || len(in.Password) < 6 || in.DisplayName == "" {
		fail(c, 400, "用户名至少3位，密码至少6位，昵称不能为空")
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	res, err := a.store.DB.Exec(`INSERT INTO users(username,password_hash,display_name) VALUES(?,?,?)`, in.Username, string(hash), in.DisplayName)
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
	err := a.store.DB.QueryRow(`SELECT id,username,display_name,avatar,COALESCE(public_key,''),is_admin,created_at FROM users WHERE id=?`, uid(c)).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Avatar, &u.PublicKey, &u.IsAdmin, &u.CreatedAt)
	if err != nil {
		fail(c, 404, "用户不存在")
		return
	}
	c.JSON(200, u)
}

func (a *API) setPublicKey(c *gin.Context) {
	var in struct {
		PublicKey string `json:"public_key"`
	}
	if c.ShouldBindJSON(&in) != nil || len(in.PublicKey) < 40 || len(in.PublicKey) > 4096 {
		fail(c, 400, "公钥格式错误")
		return
	}
	res, err := a.store.DB.Exec(`UPDATE users SET public_key=? WHERE id=? AND (public_key IS NULL OR public_key='' OR public_key=?)`, in.PublicKey, uid(c), in.PublicKey)
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
	q := "%" + strings.TrimSpace(c.Query("q")) + "%"
	rows, err := a.store.DB.Query(`SELECT id,username,display_name,avatar,is_admin,created_at FROM users WHERE id<>? AND (username LIKE ? OR display_name LIKE ?) ORDER BY display_name LIMIT 50`, uid(c), q, q)
	if err != nil {
		fail(c, 500, "查询失败")
		return
	}
	defer rows.Close()
	out := []store.User{}
	for rows.Next() {
		var u store.User
		_ = rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Avatar, &u.IsAdmin, &u.CreatedAt)
		out = append(out, u)
	}
	c.JSON(200, out)
}

func (a *API) conversations(c *gin.Context) {
	rows, err := a.store.DB.Query(`SELECT x.id,IF(x.is_group,x.name,COALESCE(other.display_name,x.name)),x.is_group,COALESCE(m.body,''),m.created_at,(SELECT COUNT(*) FROM messages um WHERE um.conversation_id=x.id AND um.id>cm.last_read_message_id AND um.sender_id<>?) FROM conversation_members cm JOIN conversations x ON x.id=cm.conversation_id LEFT JOIN conversation_members ocm ON ocm.conversation_id=x.id AND ocm.user_id<>? AND x.is_group=0 LEFT JOIN users other ON other.id=ocm.user_id LEFT JOIN messages m ON m.id=(SELECT MAX(id) FROM messages WHERE conversation_id=x.id) WHERE cm.user_id=? ORDER BY COALESCE(m.created_at,x.created_at) DESC`, uid(c), uid(c), uid(c))
	if err != nil {
		fail(c, 500, "查询失败")
		return
	}
	defer rows.Close()
	out := []store.Conversation{}
	for rows.Next() {
		var x store.Conversation
		_ = rows.Scan(&x.ID, &x.Name, &x.IsGroup, &x.LastMessage, &x.LastAt, &x.Unread)
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
			_ = tx.Rollback()
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
	rows, err := a.store.DB.Query(`SELECT m.id,m.conversation_id,m.sender_id,u.display_name,m.body,m.created_at FROM messages m JOIN users u ON u.id=m.sender_id WHERE m.conversation_id=? ORDER BY m.id DESC LIMIT 100`, id)
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
	var envelope struct {
		V          int    `json:"v"`
		Alg        string `json:"alg"`
		Recipients map[string]struct {
			IV string `json:"iv"`
			CT string `json:"ct"`
		} `json:"recipients"`
	}
	if in.Body == "" || len(in.Body) > 131072 || json.Unmarshal([]byte(in.Body), &envelope) != nil || envelope.V != 1 || envelope.Alg != "P256-HKDF-A256GCM" || len(envelope.Recipients) == 0 {
		fail(c, 400, "加密消息无效或过大")
		return
	}
	for _, box := range envelope.Recipients {
		if len(box.IV) < 16 || len(box.CT) < 24 {
			fail(c, 400, "加密消息封装无效")
			return
		}
	}
	rows, err := a.store.DB.Query(`SELECT user_id FROM conversation_members WHERE conversation_id=?`, id)
	if err != nil {
		fail(c, 500, "校验收件人失败")
		return
	}
	defer rows.Close()
	expected := 0
	for rows.Next() {
		var memberID int64
		_ = rows.Scan(&memberID)
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
	res, err := a.store.DB.Exec(`INSERT INTO messages(conversation_id,sender_id,body) VALUES(?,?,?)`, id, uid(c), in.Body)
	if err != nil {
		fail(c, 500, "发送失败")
		return
	}
	mid, _ := res.LastInsertId()
	var m store.Message
	_ = a.store.DB.QueryRow(`SELECT m.id,m.conversation_id,m.sender_id,u.display_name,m.body,m.created_at FROM messages m JOIN users u ON u.id=m.sender_id WHERE m.id=?`, mid).Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.SenderName, &m.Body, &m.CreatedAt)
	rows, _ = a.store.DB.Query(`SELECT user_id FROM conversation_members WHERE conversation_id=?`, id)
	ids := []int64{}
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var x int64
			_ = rows.Scan(&x)
			ids = append(ids, x)
		}
	}
	a.hub.SendTo(ids, gin.H{"type": "message", "data": m})
	c.JSON(201, m)
}
func (a *API) readConversation(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if !a.member(id, uid(c)) {
		fail(c, 403, "无权访问")
		return
	}
	_, _ = a.store.DB.Exec(`UPDATE conversation_members SET last_read_message_id=COALESCE((SELECT MAX(id) FROM messages WHERE conversation_id=?),0) WHERE conversation_id=? AND user_id=?`, id, id, uid(c))
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
	rows, err := a.store.DB.Query(`SELECT id,username,display_name,avatar,is_admin,created_at FROM users ORDER BY id DESC LIMIT 200`)
	if err != nil {
		fail(c, 500, "查询失败")
		return
	}
	defer rows.Close()
	out := []store.User{}
	for rows.Next() {
		var u store.User
		_ = rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Avatar, &u.IsAdmin, &u.CreatedAt)
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
	_, _ = tx.Exec(`DELETE FROM messages WHERE sender_id=?`, id)
	_, _ = tx.Exec(`DELETE FROM conversation_members WHERE user_id=?`, id)
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
