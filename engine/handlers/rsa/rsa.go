package rsa

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"

	"github.com/Eric033/x-mate/engine/internal/context"
	"github.com/Eric033/x-mate/engine/internal/handler"
)

// RSAHandler performs RSA encryption.
type RSAHandler struct{}

func (h *RSAHandler) Execute(data *handler.StepData, ctx *context.TestContext) *handler.StepResult {
	keyStr := data.Attrs["key"]
	value := data.Attrs["value"]

	if keyStr == "" {
		return &handler.StepResult{Success: false, FailureMessage: "RSA: missing 'key' attribute"}
	}

	// Parse public key
	pubKey, err := parseRSAPublicKey(keyStr)
	if err != nil {
		return &handler.StepResult{Success: false, FailureMessage: fmt.Sprintf("RSA key parse: %v", err)}
	}

	// Encrypt
	encrypted, err := encryptWithPublicKey([]byte(value), pubKey)
	if err != nil {
		return &handler.StepResult{Success: false, FailureMessage: fmt.Sprintf("RSA encrypt: %v", err)}
	}

	encoded := base64.StdEncoding.EncodeToString(encrypted)

	// Store result
	for _, s := range data.Saves {
		ctx.Set(s.Name, encoded)
	}

	return &handler.StepResult{
		Success:      true,
		ResponseData: encoded,
	}
}

// parseRSAPublicKey parses a Base64-encoded or PEM-encoded RSA public key.
func parseRSAPublicKey(keyStr string) (*rsa.PublicKey, error) {
	// Try PEM format first
	block, _ := pem.Decode([]byte(keyStr))
	if block != nil {
		pub, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rsaPub, ok := pub.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("not an RSA public key")
		}
		return rsaPub, nil
	}

	// Try raw Base64
	decoded, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		return nil, fmt.Errorf("invalid base64 key: %w", err)
	}

	pub, err := x509.ParsePKIXPublicKey(decoded)
	if err != nil {
		// Try PKCS1
		pubPKCS1, err2 := x509.ParsePKCS1PublicKey(decoded)
		if err2 != nil {
			return nil, fmt.Errorf("parse key: %w / %w", err, err2)
		}
		return pubPKCS1, nil
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}
	return rsaPub, nil
}

// encryptWithPublicKey encrypts data using RSA/ECB/PKCS1Padding with segment encryption.
func encryptWithPublicKey(data []byte, pub *rsa.PublicKey) ([]byte, error) {
	keySize := pub.Size()
	maxBlock := keySize - 11 // PKCS1 padding overhead
	if maxBlock > 117 {
		maxBlock = 117 // SRS: max 117 bytes per block
	}

	var result []byte
	for offset := 0; offset < len(data); offset += maxBlock {
		end := offset + maxBlock
		if end > len(data) {
			end = len(data)
		}
		block := data[offset:end]

		encrypted, err := rsa.EncryptPKCS1v15(rand.Reader, pub, block)
		if err != nil {
			return nil, fmt.Errorf("encrypt block %d: %w", offset, err)
		}
		result = append(result, encrypted...)
	}

	return result, nil
}
