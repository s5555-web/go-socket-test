package api

import (
	"crypto/ecdh"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"sket/internal/api/socket"
	"sket/internal/auth"
	"sket/internal/config"
	"sket/internal/store"
	"strings"
	"testing"
	"time"
)

// Opt in against this project's database only. Fixtures use unique names and
// are removed with exact primary-key predicates. No real accounts are used.
func TestEncryptedPersistenceIntegration(t *testing.T) {
	dsn := os.Getenv("SIGNAL_ENCRYPTION_TEST_DSN")
	if dsn == "" {
		t.Skip("set SIGNAL_ENCRYPTION_TEST_DSN for MySQL integration")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	must := func(query string, args ...interface{}) int64 {
		t.Helper()
		result, err := db.Exec(query, args...)
		if err != nil {
			t.Fatal(err)
		}
		id, _ := result.LastInsertId()
		return id
	}
	name := fmt.Sprintf("verify_%d", time.Now().UnixNano())
	user := must(`INSERT INTO users(username,password_hash,display_name,is_support) VALUES(?,'disabled','Encryption verification',TRUE)`, name)
	defer db.Exec(`DELETE FROM users WHERE id=?`, user)
	other := must(`INSERT INTO users(username,password_hash,display_name) VALUES(?,'disabled','Encryption verification')`, name+"b")
	defer db.Exec(`DELETE FROM users WHERE id=?`, other)
	conversation := must(`INSERT INTO conversations(name,created_by) VALUES('encryption verification',?)`, user)
	defer db.Exec(`DELETE FROM conversations WHERE id=?`, conversation)
	defer func() {
		_, _ = db.Exec(`DELETE FROM pending_message_recipients WHERE message_id IN(SELECT id FROM pending_messages WHERE conversation_id=?)`, conversation)
		_, _ = db.Exec(`DELETE FROM pending_messages WHERE conversation_id=?`, conversation)
		_, _ = db.Exec(`DELETE FROM message_deletions WHERE message_id IN(SELECT id FROM messages WHERE conversation_id=?)`, conversation)
		_, _ = db.Exec(`DELETE FROM messages WHERE conversation_id=?`, conversation)
		_, _ = db.Exec(`DELETE FROM conversation_members WHERE conversation_id=?`, conversation)
	}()

	must(`INSERT INTO conversation_members(conversation_id,user_id) VALUES(?,?),(?,?)`, conversation, user, conversation, other)
	hub := socket.NewHub()
	go hub.Run()
	defer hub.Stop()
	secret := "verification-secret-never-used-for-real-tokens"
	app := &API{store: &store.Store{DB: db}, auth: auth.New(secret), hub: hub, clientDir: t.TempDir()}
	engine := app.ClientEngine(&config.Config{})
	call := func(method, path, body string, account int64, expect int) []byte {
		t.Helper()
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+app.auth.Sign(account, false))
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, request)
		if rec.Code != expect {
			t.Fatalf("%s %s: status %d, expected %d", method, path, rec.Code, expect)
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("sensitive API response can be cached")
		}
		return rec.Body.Bytes()
	}
	path := fmt.Sprintf("/api/conversations/%d/messages", conversation)
	call("POST", path, `{"body":"must never be stored"}`, user, 400)
	var env encryptedEnvelope
	_ = json.Unmarshal([]byte(validEnvelopeJSON(t)), &env)
	box := env.Recipients["7"]
	env.Recipients = map[string]encryptedRecipientBox{fmt.Sprint(user): box, fmt.Sprint(other): box}
	envelope, _ := json.Marshal(env)
	payload, _ := json.Marshal(map[string]string{"body": string(envelope)})
	created := call("POST", path, string(payload), user, 201)
	var message store.Message
	_ = json.Unmarshal(created, &message)
	var stored string
	if err = db.QueryRow(`SELECT body FROM messages WHERE id=?`, message.ID).Scan(&stored); err != nil || stored != string(envelope) {
		t.Fatal("server did not persist exact ciphertext")
	}
	call("POST", fmt.Sprintf("/api/messages/%d/ack", message.ID), "", other, 204)
	if err = db.QueryRow(`SELECT body FROM messages WHERE id=?`, message.ID).Scan(&stored); err != nil {
		t.Fatal("ack removed durable history")
	}
	var history []store.Message
	_ = json.Unmarshal(call("GET", path, "", other, 200), &history)
	if len(history) != 1 || history[0].Body != stored {
		t.Fatal("recipient history unavailable")
	}

	call("DELETE", fmt.Sprintf("%s/%d?scope=self", path, message.ID), "", other, 204)
	history = nil
	_ = json.Unmarshal(call("GET", path, "", other, 200), &history)
	if len(history) != 0 {
		t.Fatal("self deletion did not hide server history")
	}
	_ = json.Unmarshal(call("GET", path, "", user, 200), &history)
	if len(history) != 1 {
		t.Fatal("self deletion affected sender")
	}
	call("DELETE", fmt.Sprintf("%s/%d?scope=all", path, message.ID), "", user, 204)
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM messages WHERE id=?`, message.ID).Scan(&count)
	if count != 0 {
		t.Fatal("recall retained history")
	}
	// Exercise visitor/agent storage and strict plaintext rejection on both routes.
	pair, _ := ecdh.P256().GenerateKey(rand.Reader)
	pub := pair.PublicKey().Bytes()
	jwk, _ := json.Marshal(map[string]string{"kty": "EC", "crv": "P-256", "x": base64.RawURLEncoding.EncodeToString(pub[1:33]), "y": base64.RawURLEncoding.EncodeToString(pub[33:])})
	must(`UPDATE users SET public_key=? WHERE id=?`, string(jwk), user)
	key, _ := randomHex(16)
	site := must(`INSERT INTO support_sites(name,site_key,allowed_origin) VALUES('encryption verification',?,'*')`, key)
	defer db.Exec(`DELETE FROM support_sites WHERE id=?`, site)
	must(`INSERT INTO support_user_sites(user_id,site_id) VALUES(?,?)`, user, site)
	defer db.Exec(`DELETE FROM support_user_sites WHERE site_id=?`, site)
	visitor, _ := randomHex(16)
	token, _ := randomHex(32)
	must(`INSERT INTO support_visitors(id,site_id,token_hash,public_key) VALUES(?,?,?,?)`, visitor, site, tokenHash(token), string(jwk))
	defer db.Exec(`DELETE FROM support_visitors WHERE id=?`, visitor)
	thread := must(`INSERT INTO support_threads(site_id,visitor_id) VALUES(?,?)`, site, visitor)
	defer db.Exec(`DELETE FROM support_threads WHERE id=?`, thread)
	defer db.Exec(`DELETE FROM support_messages WHERE thread_id=?`, thread)
	supportPath := fmt.Sprintf("/api/support/threads/%d/messages", thread)
	call("POST", supportPath, `{"body":"must never be stored"}`, user, 400)
	members := []supportMember{{ID: "agent:" + fmt.Sprint(user), PublicKey: string(jwk)}, {ID: "visitor:" + visitor, PublicKey: string(jwk)}}
	body, err := encryptSupportForMigration("integration verification", thread, members)
	if err != nil {
		t.Fatal(err)
	}
	encryptedPayload, _ := json.Marshal(map[string]string{"body": body})
	call("POST", supportPath, string(encryptedPayload), user, 201)
	for _, test := range []struct {
		body   string
		status int
	}{{`{"body":"must never be stored"}`, 400}, {string(encryptedPayload), 201}} {
		req := httptest.NewRequest("POST", "/api/widget/messages", strings.NewReader(test.body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if rec.Code != test.status {
			t.Fatalf("visitor send status %d expected %d", rec.Code, test.status)
		}
	}
	rows, err := db.Query(`SELECT body FROM support_messages WHERE thread_id=?`, thread)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	count = 0
	for rows.Next() {
		if err = rows.Scan(&stored); err != nil {
			t.Fatal(err)
		}
		if stored != body {
			t.Fatal("support persisted plaintext")
		}
		count++
	}
	if count != 2 {
		t.Fatal("support messages missing")
	}
}
