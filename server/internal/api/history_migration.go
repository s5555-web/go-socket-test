package api

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"golang.org/x/crypto/hkdf"
	"io"
	"strconv"
)

// EncryptLegacySupport is a one-way maintenance operation: only recipients hold
// decryption keys. Never write a plaintext dump while performing the migration.
func EncryptLegacySupport(db *sql.DB, apply bool) (int, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	rows, err := tx.Query(`SELECT id,thread_id,body FROM support_messages ORDER BY id FOR UPDATE`)
	if err != nil {
		return 0, err
	}
	type legacy struct {
		id, thread int64
		body       string
	}
	pending := []legacy{}
	for rows.Next() {
		var m legacy
		if err = rows.Scan(&m.id, &m.thread, &m.body); err != nil {
			rows.Close()
			return 0, err
		}
		var env supportEnvelope
		if json.Unmarshal([]byte(m.body), &env) == nil && env.Alg == "SUPPORT-P256-HKDF-A256GCM" && env.V == 1 {
			continue
		}
		pending = append(pending, m)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	for _, m := range pending {
		members := []supportMember{}
		agents, err := tx.Query(`SELECT u.id,u.public_key FROM users u JOIN support_user_sites x ON x.user_id=u.id JOIN support_threads t ON t.site_id=x.site_id WHERE t.id=? AND u.is_support=TRUE`, m.thread)
		if err != nil {
			return 0, err
		}
		for agents.Next() {
			var id int64
			var pub sql.NullString
			if err = agents.Scan(&id, &pub); err != nil {
				agents.Close()
				return 0, err
			}
			if !validSupportPublicKey(pub.String) {
				agents.Close()
				return 0, fmt.Errorf("agent %d has no valid key", id)
			}
			members = append(members, supportMember{ID: "agent:" + strconv.FormatInt(id, 10), PublicKey: pub.String})
		}
		err = agents.Err()
		agents.Close()
		if err != nil {
			return 0, err
		}
		if len(members) == 0 {
			return 0, fmt.Errorf("thread %d has no encryption recipients", m.thread)
		}
		// Old visitors had no encryption identity. Preserve their history for agents;
		// no server-held fallback decryption key is created.
		body, err := encryptSupportForMigration(m.body, m.thread, members)
		if err != nil {
			return 0, err
		}
		if apply {
			if _, err = tx.Exec(`UPDATE support_messages SET body=? WHERE id=?`, body, m.id); err != nil {
				return 0, err
			}
		}
	}
	if apply {
		// Recover existing offline ciphertext into permanent history without ever
		// accepting legacy plaintext into the new history path.
		rows, err := tx.Query(`SELECT id,conversation_id,sender_id,body,created_at FROM pending_messages`)
		if err != nil {
			return 0, err
		}
		type record struct {
			id, conversation, sender int64
			body                     string
			created                  interface{}
		}
		records := []record{}
		for rows.Next() {
			var m record
			if err = rows.Scan(&m.id, &m.conversation, &m.sender, &m.body, &m.created); err != nil {
				rows.Close()
				return 0, err
			}
			if _, err = validateEncryptedEnvelope(m.body); err != nil {
				rows.Close()
				return 0, fmt.Errorf("pending message %d is not encrypted", m.id)
			}
			records = append(records, m)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return 0, err
		}
		for _, m := range records {
			if _, err = tx.Exec(`INSERT IGNORE INTO messages(id,conversation_id,sender_id,body,created_at) VALUES(?,?,?,?,?)`, m.id, m.conversation, m.sender, m.body, m.created); err != nil {
				return 0, err
			}
		}
		if err = tx.Commit(); err != nil {
			return 0, err
		}
	}
	return len(pending), nil
}
func encryptSupportForMigration(body string, threadID int64, members []supportMember) (string, error) {
	pair, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return "", err
	}
	pub := pair.PublicKey().Bytes()
	env := supportEnvelope{V: 1, Alg: "SUPPORT-P256-HKDF-A256GCM", EPK: map[string]string{"kty": "EC", "crv": "P-256", "x": base64.RawURLEncoding.EncodeToString(pub[1:33]), "y": base64.RawURLEncoding.EncodeToString(pub[33:])}, Recipients: map[string]encryptedRecipientBox{}}
	pad := make([]byte, 512-len([]byte(body))%512)
	if _, err = rand.Read(pad); err != nil {
		return "", err
	}
	clear, err := json.Marshal(map[string]string{"body": body, "pad": base64.StdEncoding.EncodeToString(pad)})
	if err != nil {
		return "", err
	}
	defer func() {
		for i := range clear {
			clear[i] = 0
		}
	}()
	for _, member := range members {
		var jwk map[string]string
		if err = json.Unmarshal([]byte(member.PublicKey), &jwk); err != nil {
			// Public JWKs can carry booleans/key_ops alongside the coordinates.
			var full map[string]interface{}
			if err = json.Unmarshal([]byte(member.PublicKey), &full); err != nil {
				return "", err
			}
			jwk = map[string]string{}
			for _, k := range []string{"x", "y"} {
				jwk[k], _ = full[k].(string)
			}
		}
		xb, _ := base64.RawURLEncoding.DecodeString(jwk["x"])
		yb, _ := base64.RawURLEncoding.DecodeString(jwk["y"])
		public, err := ecdh.P256().NewPublicKey(append(append([]byte{4}, xb...), yb...))
		if err != nil {
			return "", err
		}
		secret, err := pair.ECDH(public)
		if err != nil {
			return "", err
		}
		context := fmt.Sprintf("%d:%s", threadID, member.ID)
		raw := make([]byte, 32)
		_, err = io.ReadFull(hkdf.New(sha256.New, secret, []byte("signal-support-v1"), []byte(context)), raw)
		for i := range secret {
			secret[i] = 0
		}
		if err != nil {
			return "", err
		}
		block, err := aes.NewCipher(raw)
		for i := range raw {
			raw[i] = 0
		}
		if err != nil {
			return "", err
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return "", err
		}
		iv := make([]byte, 12)
		if _, err = rand.Read(iv); err != nil {
			return "", err
		}
		env.Recipients[member.ID] = encryptedRecipientBox{IV: base64.StdEncoding.EncodeToString(iv), CT: base64.StdEncoding.EncodeToString(gcm.Seal(nil, iv, clear, []byte(context)))}
	}
	data, err := json.Marshal(env)
	return string(data), err
}
