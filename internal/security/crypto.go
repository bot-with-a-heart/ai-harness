package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

const (
	EnvelopeVersion = 1
	AlgorithmAESGCM = "AES-256-GCM"
	KeySize         = 32
)

type Envelope struct {
	Version    int       `json:"version"`
	Algorithm  string    `json:"algorithm"`
	KeyID      string    `json:"keyId"`
	Nonce      string    `json:"nonce"`
	Ciphertext string    `json:"ciphertext"`
	CreatedAt  time.Time `json:"createdAt"`
}

func Encrypt(key []byte, keyID string, plaintext []byte, aad []byte) ([]byte, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("AES-256-GCM key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, aad)
	envelope := Envelope{
		Version:    EnvelopeVersion,
		Algorithm:  AlgorithmAESGCM,
		KeyID:      keyID,
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
		CreatedAt:  time.Now().UTC(),
	}
	contents, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode encrypted envelope: %w", err)
	}
	return append(contents, '\n'), nil
}

func Decrypt(key []byte, contents []byte, aad []byte) ([]byte, Envelope, error) {
	if len(key) != KeySize {
		return nil, Envelope{}, fmt.Errorf("AES-256-GCM key must be 32 bytes")
	}
	var envelope Envelope
	if err := json.Unmarshal(contents, &envelope); err != nil {
		return nil, Envelope{}, fmt.Errorf("decode encrypted envelope: %w", err)
	}
	if envelope.Version != EnvelopeVersion {
		return nil, Envelope{}, fmt.Errorf("unsupported encrypted envelope version %d", envelope.Version)
	}
	if envelope.Algorithm != AlgorithmAESGCM {
		return nil, Envelope{}, fmt.Errorf("unsupported encrypted envelope algorithm %q", envelope.Algorithm)
	}

	nonce, err := base64.StdEncoding.DecodeString(envelope.Nonce)
	if err != nil {
		return nil, Envelope{}, fmt.Errorf("decode nonce: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(envelope.Ciphertext)
	if err != nil {
		return nil, Envelope{}, fmt.Errorf("decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, Envelope{}, fmt.Errorf("create AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, Envelope{}, fmt.Errorf("create GCM: %w", err)
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, Envelope{}, fmt.Errorf("decrypt envelope: %w", err)
	}
	return plaintext, envelope, nil
}
