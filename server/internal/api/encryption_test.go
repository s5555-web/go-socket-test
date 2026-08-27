package api

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func validEnvelopeJSON(t *testing.T) string {
	t.Helper()
	value, err := json.Marshal(encryptedEnvelope{
		V:   1,
		Alg: "P256-HKDF-A256GCM",
		Recipients: map[string]encryptedRecipientBox{
			"7": {
				IV: base64.StdEncoding.EncodeToString(make([]byte, 12)),
				CT: base64.StdEncoding.EncodeToString(make([]byte, 32)),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(value)
}

func TestValidateEncryptedEnvelope(t *testing.T) {
	if _, err := validateEncryptedEnvelope(validEnvelopeJSON(t)); err != nil {
		t.Fatalf("valid envelope rejected: %v", err)
	}
}

func TestValidateEncryptedEnvelopeRejectsPlaintext(t *testing.T) {
	for _, value := range []string{"hello", `{"v":1,"alg":"none","recipients":{"7":{"iv":"aGVsbG8=","ct":"d29ybGQ="}}}`} {
		if _, err := validateEncryptedEnvelope(value); err == nil {
			t.Fatalf("unencrypted value accepted: %q", value)
		}
	}
}

func TestValidateEncryptedEnvelopeRejectsMalformedGCMBox(t *testing.T) {
	value := strings.Replace(validEnvelopeJSON(t), base64.StdEncoding.EncodeToString(make([]byte, 12)), "not-base64", 1)
	if _, err := validateEncryptedEnvelope(value); err == nil {
		t.Fatal("malformed AES-GCM box accepted")
	}
}
