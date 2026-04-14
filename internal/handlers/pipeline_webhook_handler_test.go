package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestVerifyHMAC_Valid(t *testing.T) {
	secret := "my-webhook-secret"
	body := []byte(`{"ref":"refs/heads/main"}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	if !verifyHMAC(body, sig, secret) {
		t.Error("expected valid HMAC to pass")
	}
}

func TestVerifyHMAC_ValidWithPrefix(t *testing.T) {
	secret := "my-webhook-secret"
	body := []byte(`{"ref":"refs/heads/main"}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if !verifyHMAC(body, sig, secret) {
		t.Error("expected sha256= prefixed HMAC to pass")
	}
}

func TestVerifyHMAC_InvalidSignature(t *testing.T) {
	if verifyHMAC([]byte("body"), "deadbeef", "secret") {
		t.Error("expected invalid signature to fail")
	}
}

func TestVerifyHMAC_WrongSecret(t *testing.T) {
	secret := "correct-secret"
	body := []byte(`{"data":"test"}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	if verifyHMAC(body, sig, "wrong-secret") {
		t.Error("expected wrong secret to fail")
	}
}

func TestVerifyHMAC_EmptySignature(t *testing.T) {
	if verifyHMAC([]byte("body"), "", "secret") {
		t.Error("expected empty signature to fail")
	}
}

func TestVerifyHMAC_BadHex(t *testing.T) {
	if verifyHMAC([]byte("body"), "not-valid-hex!!", "secret") {
		t.Error("expected bad hex to fail")
	}
}

func TestVerifyHMAC_EmptyBody(t *testing.T) {
	secret := "my-secret"
	body := []byte{}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	if !verifyHMAC(body, sig, secret) {
		t.Error("expected empty body HMAC to pass")
	}
}
