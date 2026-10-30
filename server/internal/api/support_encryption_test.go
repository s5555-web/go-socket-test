package api

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"golang.org/x/crypto/hkdf"
	"io"
	"testing"
)

func TestSupportMigrationAndValidation(t *testing.T) {
	private, _ := ecdh.P256().GenerateKey(rand.Reader)
	pub := private.PublicKey().Bytes()
	jwk, _ := json.Marshal(map[string]string{"kty": "EC", "crv": "P-256", "x": base64.RawURLEncoding.EncodeToString(pub[1:33]), "y": base64.RawURLEncoding.EncodeToString(pub[33:])})
	members := []supportMember{{ID: "agent:7", PublicKey: string(jwk)}}
	value, err := encryptSupportForMigration("private legacy content", 33, members)
	if err != nil {
		t.Fatal(err)
	}
	if err = validateSupportEnvelope(value, members); err != nil {
		t.Fatal(err)
	}
	var env supportEnvelope
	_ = json.Unmarshal([]byte(value), &env)
	x, _ := base64.RawURLEncoding.DecodeString(env.EPK["x"])
	y, _ := base64.RawURLEncoding.DecodeString(env.EPK["y"])
	epk, _ := ecdh.P256().NewPublicKey(append(append([]byte{4}, x...), y...))
	secret, _ := private.ECDH(epk)
	raw := make([]byte, 32)
	_, _ = io.ReadFull(hkdf.New(sha256.New, secret, []byte("signal-support-v1"), []byte("33:agent:7")), raw)
	block, _ := aes.NewCipher(raw)
	gcm, _ := cipher.NewGCM(block)
	iv, _ := base64.StdEncoding.DecodeString(env.Recipients["agent:7"].IV)
	ct, _ := base64.StdEncoding.DecodeString(env.Recipients["agent:7"].CT)
	clear, err := gcm.Open(nil, iv, ct, []byte("33:agent:7"))
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]string
	_ = json.Unmarshal(clear, &payload)
	if payload["body"] != "private legacy content" {
		t.Fatal("migration lost history")
	}
	for _, bad := range []string{"plaintext", fmt.Sprintf(`{"body":"plain",%s`, value[1:]), `{"v":1,"alg":"SUPPORT-P256-HKDF-A256GCM"}`} {
		if validateSupportEnvelope(bad, members) == nil {
			t.Fatal("accepted malformed/unencrypted content")
		}
	}
	if validateSupportEnvelope(value, []supportMember{{ID: "agent:8"}}) == nil {
		t.Fatal("accepted incorrect recipients")
	}
}
func TestRejectPlaintextAlongsideEncryptedEnvelope(t *testing.T) {
	value := validEnvelopeJSON(t)
	var env map[string]interface{}
	_ = json.Unmarshal([]byte(value), &env)
	env["body"] = "plaintext"
	bad, _ := json.Marshal(env)
	if _, err := validateEncryptedEnvelope(string(bad)); err == nil {
		t.Fatal("accepted plaintext sidecar")
	}
}
