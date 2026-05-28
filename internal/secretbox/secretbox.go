package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

type Box struct{ key [32]byte }

func New(secret string) (Box, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return Box{}, errors.New("encryption key is required")
	}
	return Box{key: sha256.Sum256([]byte(secret))}, nil
}

func (b Box) Seal(plain []byte) (string, error) {
	block, err := aes.NewCipher(b.key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := append(nonce, gcm.Seal(nil, nonce, plain, nil)...)
	return base64.RawURLEncoding.EncodeToString(out), nil
}

func (b Box) Open(encoded string) ([]byte, error) {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return nil, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(b.key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(data) < gcm.NonceSize() {
		return nil, errors.New("invalid secret payload")
	}
	nonce, cipherText := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	return gcm.Open(nil, nonce, cipherText, nil)
}
