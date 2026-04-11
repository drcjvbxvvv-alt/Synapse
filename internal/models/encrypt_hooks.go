package models

import "github.com/shaia/Synapse/pkg/crypto"

// encryptFields encrypts each pointed-to string in place using AES-256-GCM.
// Called by BeforeSave hooks. No-op when encryption is disabled or the value is empty.
func encryptFields(fields ...*string) error {
	if !crypto.IsEnabled() {
		return nil
	}
	for _, f := range fields {
		enc, err := crypto.Encrypt(*f)
		if err != nil {
			return err
		}
		*f = enc
	}
	return nil
}

// decryptFields decrypts each pointed-to string in place.
// Called by AfterFind / AfterCreate / AfterUpdate hooks.
// Legacy plaintext values (pre-encryption) pass through unchanged thanks to
// the graceful fallback in crypto.Decrypt.
func decryptFields(fields ...*string) error {
	if !crypto.IsEnabled() {
		return nil
	}
	for _, f := range fields {
		dec, err := crypto.Decrypt(*f)
		if err != nil {
			return err
		}
		*f = dec
	}
	return nil
}
