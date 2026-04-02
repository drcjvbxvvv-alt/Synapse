// Package crypto provides AES-256-GCM field-level encryption for sensitive
// database columns (kubeconfig, CA cert, SA token, etc.).
//
// Usage:
//
//	// At application startup:
//	crypto.Init(os.Getenv("ENCRYPTION_KEY"))
//
//	// Encrypt / Decrypt are safe to call even when no key is configured:
//	// they transparently pass the value through unchanged (dev-mode fallback).
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"sync"
)

var (
	mu        sync.RWMutex
	globalKey []byte // 32-byte AES-256 key; nil means encryption disabled
	initOnce  sync.Once
)

// Init derives a 32-byte AES-256 key from rawKey via SHA-256 and stores it
// globally.  Must be called once at startup before any Encrypt/Decrypt calls.
// If rawKey is empty, encryption is silently disabled (plaintext passthrough).
func Init(rawKey string) {
	initOnce.Do(func() {
		if rawKey == "" {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		h := sha256.Sum256([]byte(rawKey))
		globalKey = h[:]
	})
}

// IsEnabled reports whether field encryption is active.
func IsEnabled() bool {
	mu.RLock()
	defer mu.RUnlock()
	return len(globalKey) == 32
}

// Encrypt encrypts plaintext using AES-256-GCM and returns a base64-encoded
// string suitable for database storage.
// Returns plaintext unchanged when encryption is disabled or plaintext is empty.
func Encrypt(plaintext string) (string, error) {
	if !IsEnabled() || plaintext == "" {
		return plaintext, nil
	}

	mu.RLock()
	key := globalKey
	mu.RUnlock()

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt decrypts a base64-encoded AES-256-GCM ciphertext produced by Encrypt.
// Returns the input unchanged when:
//   - encryption is disabled
//   - the input is empty
//   - the input cannot be decoded/decrypted (graceful fallback for legacy
//     unencrypted values that pre-date encryption enablement)
func Decrypt(ciphertextB64 string) (string, error) {
	if !IsEnabled() || ciphertextB64 == "" {
		return ciphertextB64, nil
	}

	data, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		// Not valid base64 → treat as legacy unencrypted value
		return ciphertextB64, nil
	}

	mu.RLock()
	key := globalKey
	mu.RUnlock()

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		// Too short → legacy unencrypted value
		return ciphertextB64, nil
	}
	plaintext, err := gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		// Authentication tag mismatch → likely legacy unencrypted value
		return ciphertextB64, nil
	}
	return string(plaintext), nil
}
