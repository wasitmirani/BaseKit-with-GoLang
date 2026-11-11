package messaging

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

// SecretBox opens provider credentials sealed by NestJS
// (backend-nestjs/src/common/crypto/secret-box.ts): AES-256-GCM, stored as
// `v1:<iv>:<tag>:<ciphertext>` in base64url. Both services must use the same key.
type SecretBox struct{ key []byte }

// NewSecretBox derives the key exactly like NestJS: SECRETS_ENCRYPTION_KEY (32+
// characters) hashed with SHA-256; outside production a development key derived
// from JWT_SECRET.
func NewSecretBox(encryptionKey, jwtSecret string, production bool) (*SecretBox, error) {
	if len(encryptionKey) >= 32 {
		k := sha256.Sum256([]byte(encryptionKey))
		return &SecretBox{key: k[:]}, nil
	}
	if production {
		return nil, errors.New("SECRETS_ENCRYPTION_KEY must be set (32+ characters) in production")
	}
	k := sha256.Sum256([]byte("linqflow-dev-secrets:" + jwtSecret))
	return &SecretBox{key: k[:]}, nil
}

var errUnreadable = errors.New("unreadable secret (tampered, or SECRETS_ENCRYPTION_KEY differs from NestJS)")

// Open decrypts a sealed JSON value into out.
func (b *SecretBox) Open(sealed string, out any) error {
	parts := strings.Split(sealed, ":")
	if len(parts) != 4 || parts[0] != "v1" {
		return errUnreadable
	}
	dec := base64.RawURLEncoding
	iv, err1 := dec.DecodeString(parts[1])
	tag, err2 := dec.DecodeString(parts[2])
	body, err3 := dec.DecodeString(parts[3])
	if err1 != nil || err2 != nil || err3 != nil || len(iv) != 12 {
		return errUnreadable
	}
	block, err := aes.NewCipher(b.key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	plain, err := gcm.Open(nil, iv, append(body, tag...), nil)
	if err != nil {
		return errUnreadable
	}
	return json.Unmarshal(plain, out)
}
