package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

var (
	ErrInvalidPayload = errors.New("invalid encrypted payload")
	ErrDecrypt        = errors.New("decrypt encrypted payload")
)

const sealVersion = "v1"

// Box encrypts secrets at rest with AES-GCM.
type Box struct {
	gcm cipher.AEAD
}

// NewBox derives a 32-byte key from secret material via SHA-256.
func NewBox(secret string) (*Box, error) {
	if secret == "" {
		return nil, errors.New("empty secret")
	}
	sum := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, fmt.Errorf("aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	return &Box{gcm: gcm}, nil
}

// Seal encrypts plaintext and returns v1.base64(nonce|ciphertext).
func (b *Box) Seal(plaintext string) (string, error) {
	nonce := make([]byte, b.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}
	out := b.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return sealVersion + "." + base64.StdEncoding.EncodeToString(out), nil
}

// Open decrypts a Seal payload.
func (b *Box) Open(payload string) (string, error) {
	version, encoded, ok := strings.Cut(payload, ".")
	if !ok || version != sealVersion || encoded == "" {
		return "", fmt.Errorf("%w: unsupported version", ErrInvalidPayload)
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode: %w: %w", ErrInvalidPayload, err)
	}
	ns := b.gcm.NonceSize()
	if len(raw) < ns {
		return "", fmt.Errorf("%w: ciphertext too short", ErrInvalidPayload)
	}
	plain, err := b.gcm.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", fmt.Errorf("open: %w: %w", ErrDecrypt, err)
	}
	return string(plain), nil
}

// MaskCookie returns a redacted cookie for API responses.
func MaskCookie(cookie string) string {
	c := stringsTrim(cookie)
	if c == "" {
		return ""
	}
	if len(c) <= 8 {
		return "***"
	}
	return c[:4] + "***" + c[len(c)-2:]
}

// MaskKey shows a short prefix of an API key.
func MaskKey(key string) string {
	k := stringsTrim(key)
	if len(k) <= 10 {
		return "***"
	}
	return k[:6] + "..." + k[len(k)-4:]
}

func stringsTrim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
