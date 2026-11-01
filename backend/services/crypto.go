// Package services contains the application's business logic.
package services

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// NewID returns a new UUID string.
func NewID() string { return uuid.NewString() }

// RandomToken returns n random bytes as an unpadded base64url string.
func RandomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// HashToken returns the hex SHA-256 of a high-entropy token (refresh, one-time, recovery).
func HashToken(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// PasswordHasher hashes and verifies passwords.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(hash, password string) bool
}

// BcryptHasher implements PasswordHasher. Passwords are SHA-256 pre-hashed so that
// inputs longer than bcrypt's 72-byte limit are fully significant.
type BcryptHasher struct{ Cost int }

func prehash(p string) []byte {
	sum := sha256.Sum256([]byte(p))
	return []byte(base64.StdEncoding.EncodeToString(sum[:]))
}

func (h BcryptHasher) Hash(password string) (string, error) {
	cost := h.Cost
	if cost < bcrypt.MinCost {
		cost = bcrypt.DefaultCost
	}
	b, err := bcrypt.GenerateFromPassword(prehash(password), cost)
	return string(b), err
}

func (h BcryptHasher) Verify(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), prehash(password)) == nil
}

// SecretBox encrypts small secrets (TOTP seeds) at rest with AES-256-GCM.
type SecretBox struct{ aead cipher.AEAD }

// NewSecretBox derives a key from the given secret.
func NewSecretBox(secret string) (*SecretBox, error) {
	key := sha256.Sum256([]byte("secretbox:" + secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &SecretBox{aead: aead}, nil
}

// Seal encrypts plaintext into base64.
func (b *SecretBox) Seal(plain string) (string, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b.aead.Seal(nonce, nonce, []byte(plain), nil)), nil
}

// Open decrypts the output of Seal.
func (b *SecretBox) Open(sealed string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		return "", err
	}
	ns := b.aead.NonceSize()
	if len(raw) < ns {
		return "", errors.New("ciphertext too short")
	}
	plain, err := b.aead.Open(nil, raw[:ns], raw[ns:], nil)
	return string(plain), err
}
