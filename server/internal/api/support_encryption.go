package api

import (
	"crypto/ecdh"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"strconv"
)

type supportMember struct {
	ID        string `json:"id"`
	PublicKey string `json:"public_key"`
}
type supportEnvelope struct {
	V          int                              `json:"v"`
	Alg        string                           `json:"alg"`
	EPK        map[string]string                `json:"epk"`
	Recipients map[string]encryptedRecipientBox `json:"recipients"`
}

func validSupportPublicKey(value string) bool {
	var jwk map[string]interface{}
	if json.Unmarshal([]byte(value), &jwk) != nil || jwk["kty"] != "EC" || jwk["crv"] != "P-256" || jwk["d"] != nil {
		return false
	}
	x, xok := jwk["x"].(string)
	y, yok := jwk["y"].(string)
	xb, e1 := base64.RawURLEncoding.DecodeString(x)
	yb, e2 := base64.RawURLEncoding.DecodeString(y)
	if !xok || !yok || e1 != nil || e2 != nil || len(xb) != 32 || len(yb) != 32 {
		return false
	}
	_, err := ecdh.P256().NewPublicKey(append(append([]byte{4}, xb...), yb...))
	return err == nil
}
func validateSupportEnvelope(value string, members []supportMember) error {
	var env supportEnvelope
	if len(value) > 524288 || json.Unmarshal([]byte(value), &env) != nil || env.V != 1 || env.Alg != "SUPPORT-P256-HKDF-A256GCM" || len(env.Recipients) != len(members) || len(members) == 0 || len(members) > 256 || len(env.EPK) != 4 {
		return fmt.Errorf("invalid support envelope")
	}
	epk, _ := json.Marshal(env.EPK)
	if !validSupportPublicKey(string(epk)) {
		return fmt.Errorf("invalid ephemeral public key")
	}
	// Reject unknown fields so callers cannot smuggle plaintext alongside ciphertext.
	var top map[string]json.RawMessage
	_ = json.Unmarshal([]byte(value), &top)
	if len(top) != 4 {
		return fmt.Errorf("unexpected fields")
	}
	for _, member := range members {
		box, ok := env.Recipients[member.ID]
		iv, e1 := base64.StdEncoding.DecodeString(box.IV)
		ct, e2 := base64.StdEncoding.DecodeString(box.CT)
		if !ok || e1 != nil || e2 != nil || len(iv) != 12 || len(ct) < 17 || box.PQCT != "" {
			return fmt.Errorf("invalid recipient box")
		}
	}
	var raw struct {
		Recipients map[string]map[string]json.RawMessage `json:"recipients"`
	}
	_ = json.Unmarshal([]byte(value), &raw)
	for _, box := range raw.Recipients {
		if len(box) != 2 {
			return fmt.Errorf("unexpected box fields")
		}
	}
	return nil
}
func (a *API) supportEncryptionMembers(threadID int64) ([]supportMember, error) {
	var visitorID, key string
	if err := a.store.DB.QueryRow(`SELECT v.id,COALESCE(v.public_key,'') FROM support_threads t JOIN support_visitors v ON v.id=t.visitor_id WHERE t.id=?`, threadID).Scan(&visitorID, &key); err != nil {
		return nil, err
	}
	members := []supportMember{{ID: "visitor:" + visitorID, PublicKey: key}}
	rows, err := a.store.DB.Query(`SELECT u.id,COALESCE(u.public_key,'') FROM users u JOIN support_user_sites x ON x.user_id=u.id JOIN support_threads t ON t.site_id=x.site_id WHERE t.id=? AND u.is_support=TRUE ORDER BY u.id`, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var pub string
		if err := rows.Scan(&id, &pub); err != nil {
			return nil, err
		}
		members = append(members, supportMember{ID: "agent:" + strconv.FormatInt(id, 10), PublicKey: pub})
	}
	return members, rows.Err()
}
func (a *API) setVisitorKey(c *gin.Context) {
	thread, err := a.visitorThread(c)
	if err != nil {
		fail(c, 401, "访客会话失效")
		return
	}
	site, err := a.siteByKeyForID(thread.SiteID)
	if err != nil || !site.Enabled || !originAllowed(site.AllowedOrigin, c.GetHeader("Origin")) {
		fail(c, 403, "无权访问")
		return
	}
	var in struct {
		PublicKey string `json:"public_key"`
	}
	if c.ShouldBindJSON(&in) != nil || !validSupportPublicKey(in.PublicKey) {
		fail(c, 400, "加密公钥无效")
		return
	}
	result, err := a.store.DB.Exec(`UPDATE support_visitors SET public_key=? WHERE id=? AND (public_key IS NULL OR public_key='' OR public_key=?)`, in.PublicKey, thread.VisitorID, in.PublicKey)
	if err != nil {
		fail(c, 500, "保存公钥失败")
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		var old string
		_ = a.store.DB.QueryRow(`SELECT public_key FROM support_visitors WHERE id=?`, thread.VisitorID).Scan(&old)
		if old != in.PublicKey {
			fail(c, 409, "访客密钥已改变，请发起新咨询")
			return
		}
	}
	c.JSON(200, gin.H{"recipient_id": "visitor:" + thread.VisitorID})
}
func (a *API) widgetMembers(c *gin.Context) {
	thread, err := a.visitorThread(c)
	if err != nil {
		fail(c, 401, "访客会话失效")
		return
	}
	site, err := a.siteByKeyForID(thread.SiteID)
	if err != nil || !site.Enabled || !originAllowed(site.AllowedOrigin, c.GetHeader("Origin")) {
		fail(c, 403, "无权访问")
		return
	}
	a.writeSupportMembers(c, thread.ID)
}
func (a *API) supportMembers(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || !a.supportThreadAllowed(uid(c), id) {
		fail(c, 404, "会话不存在")
		return
	}
	a.writeSupportMembers(c, id)
}
func (a *API) writeSupportMembers(c *gin.Context, id int64) {
	members, err := a.supportEncryptionMembers(id)
	if err != nil {
		fail(c, 500, "读取加密成员失败")
		return
	}
	c.JSON(200, members)
}
