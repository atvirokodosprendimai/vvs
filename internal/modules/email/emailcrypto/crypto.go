package emailcrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
)

// EncryptPassword encrypts plaintext with AES-256-GCM. Returns plaintext unchanged when key is empty (dev mode).
func EncryptPassword(key, plaintext []byte) ([]byte, error) {
	if len(key) == 0 {
		return plaintext, nil
	}
	if len(key) != 32 {
		return nil, errors.New("email: AES key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptPassword decrypts an AES-256-GCM ciphertext (nonce prepended). Returns ciphertext unchanged when key is empty.
func DecryptPassword(key, ciphertext []byte) ([]byte, error) {
	if len(key) == 0 {
		return ciphertext, nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("email: ciphertext too short")
	}
	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ct, nil)
}
