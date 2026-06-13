package rsa

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"testing"

	"github.com/Eric033/x-mate/engine/internal/context"
	"github.com/Eric033/x-mate/engine/internal/handler"
)

// generateTestRSAKeyPair generates an RSA key pair for testing.
func generateTestRSAKeyPair(t *testing.T) (privateKey *rsa.PrivateKey, publicKeyPEM string) {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("failed to marshal public key: %v", err)
	}

	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})

	return priv, string(pubPEM)
}

func TestRSAHandler_Execute_PEMKey(t *testing.T) {
	_, pubPEM := generateTestRSAKeyPair(t)

	ctx := context.New()

	data := &handler.StepData{
		StepType: "rsa",
		Attrs: map[string]string{
			"key":   pubPEM,
			"value": "test_data_to_encrypt",
		},
		Saves: []handler.SaveEntry{
			{Name: "encrypted_result", Locator: ""},
		},
	}

	h := &RSAHandler{}
	result := h.Execute(data, ctx)

	if !result.Success {
		t.Fatalf("expected success, got failure: %s", result.FailureMessage)
	}

	if result.ResponseData == "" {
		t.Fatal("expected non-empty ResponseData")
	}

	// Check that result was saved to context
	if val, ok := ctx.Get("encrypted_result"); !ok {
		t.Error("expected 'encrypted_result' to be saved")
	} else if val != result.ResponseData {
		t.Errorf("saved value mismatch: %q vs %q", val, result.ResponseData)
	}

	// Verify that the encrypted data is valid base64
	decoded, err := base64.StdEncoding.DecodeString(result.ResponseData)
	if err != nil {
		t.Fatalf("response is not valid base64: %v", err)
	}

	if len(decoded) == 0 {
		t.Fatal("decoded encrypted data is empty")
	}
}

func TestRSAHandler_Execute_Base64Key(t *testing.T) {
	_, pubPEM := generateTestRSAKeyPair(t)

	// Decode PEM to get raw DER bytes, then base64-encode
	block, _ := pem.Decode([]byte(pubPEM))
	if block == nil {
		t.Fatal("failed to decode PEM")
	}
	b64Key := base64.StdEncoding.EncodeToString(block.Bytes)

	ctx := context.New()

	data := &handler.StepData{
		StepType: "rsa",
		Attrs: map[string]string{
			"key":   b64Key,
			"value": "data_for_base64_key",
		},
	}

	h := &RSAHandler{}
	result := h.Execute(data, ctx)

	if !result.Success {
		t.Fatalf("expected success, got failure: %s", result.FailureMessage)
	}

	if result.ResponseData == "" {
		t.Fatal("expected non-empty ResponseData")
	}

	// Verify valid base64
	_, err := base64.StdEncoding.DecodeString(result.ResponseData)
	if err != nil {
		t.Fatalf("response is not valid base64: %v", err)
	}
}

func TestRSAHandler_Execute_LongData(t *testing.T) {
	_, pubPEM := generateTestRSAKeyPair(t)

	// Create data longer than 117 bytes to test segment encryption
	longData := ""
	for i := 0; i < 300; i++ {
		longData += "A"
	}

	ctx := context.New()

	data := &handler.StepData{
		StepType: "rsa",
		Attrs: map[string]string{
			"key":   pubPEM,
			"value": longData,
		},
	}

	h := &RSAHandler{}
	result := h.Execute(data, ctx)

	if !result.Success {
		t.Fatalf("expected success, got failure: %s", result.FailureMessage)
	}

	if result.ResponseData == "" {
		t.Fatal("expected non-empty ResponseData")
	}

	// For long data, encrypted output should be longer than a single block
	decoded, err := base64.StdEncoding.DecodeString(result.ResponseData)
	if err != nil {
		t.Fatalf("response is not valid base64: %v", err)
	}

	// With 1024-bit RSA, each block is 128 bytes encrypted
	// 300 bytes -> ceil(300/117) = 3 blocks -> 384 bytes
	if len(decoded) < 128 {
		t.Errorf("expected encrypted data to be at least 128 bytes (multi-block), got %d", len(decoded))
	}
}

func TestRSAHandler_Execute_EmptyValue(t *testing.T) {
	_, pubPEM := generateTestRSAKeyPair(t)

	ctx := context.New()

	data := &handler.StepData{
		StepType: "rsa",
		Attrs: map[string]string{
			"key":   pubPEM,
			"value": "",
		},
	}

	h := &RSAHandler{}
	result := h.Execute(data, ctx)

	if !result.Success {
		t.Fatalf("expected success for empty value, got: %s", result.FailureMessage)
	}
}

func TestRSAHandler_Execute_MissingKey(t *testing.T) {
	ctx := context.New()

	data := &handler.StepData{
		StepType: "rsa",
		Attrs: map[string]string{
			"value": "test_data",
		},
	}

	h := &RSAHandler{}
	result := h.Execute(data, ctx)

	if result.Success {
		t.Fatal("expected failure for missing key")
	}
	if result.FailureMessage == "" {
		t.Error("expected non-empty failure message")
	}
}

func TestRSAHandler_Execute_InvalidKey(t *testing.T) {
	ctx := context.New()

	data := &handler.StepData{
		StepType: "rsa",
		Attrs: map[string]string{
			"key":   "not-a-valid-key",
			"value": "test_data",
		},
	}

	h := &RSAHandler{}
	result := h.Execute(data, ctx)

	if result.Success {
		t.Fatal("expected failure for invalid key")
	}
	if result.FailureMessage == "" {
		t.Error("expected non-empty failure message")
	}
}

func TestParseRSAPublicKey_PEM(t *testing.T) {
	_, pubPEM := generateTestRSAKeyPair(t)

	pub, err := parseRSAPublicKey(pubPEM)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if pub == nil {
		t.Fatal("expected non-nil public key")
	}
}

func TestParseRSAPublicKey_Base64(t *testing.T) {
	_, pubPEM := generateTestRSAKeyPair(t)

	block, _ := pem.Decode([]byte(pubPEM))
	if block == nil {
		t.Fatal("failed to decode PEM")
	}

	b64Key := base64.StdEncoding.EncodeToString(block.Bytes)

	pub, err := parseRSAPublicKey(b64Key)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if pub == nil {
		t.Fatal("expected non-nil public key")
	}
}

func TestParseRSAPublicKey_Invalid(t *testing.T) {
	_, err := parseRSAPublicKey("invalid-key-data")
	if err == nil {
		t.Fatal("expected error for invalid key")
	}
}

func TestEncryptWithPublicKey(t *testing.T) {
	// Generate a key pair and keep the private key for decryption verification
	priv, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	data := []byte("test data for encryption")

	encrypted, err := encryptWithPublicKey(data, &priv.PublicKey)
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}

	if len(encrypted) == 0 {
		t.Fatal("encrypted data is empty")
	}

	// Verify we can decrypt it with the same private key
	decrypted, err := rsa.DecryptPKCS1v15(rand.Reader, priv, encrypted)
	if err != nil {
		t.Fatalf("decryption failed: %v", err)
	}

	if string(decrypted) != string(data) {
		t.Errorf("decrypted data mismatch: got %q, want %q", string(decrypted), string(data))
	}
}

func TestEncryptWithPublicKey_EmptyData(t *testing.T) {
	_, pubPEM := generateTestRSAKeyPair(t)

	block, _ := pem.Decode([]byte(pubPEM))
	if block == nil {
		t.Fatal("failed to decode PEM")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse public key: %v", err)
	}

	rsaPub := pub.(*rsa.PublicKey)
	encrypted, err := encryptWithPublicKey([]byte{}, rsaPub)
	if err != nil {
		t.Fatalf("encryption of empty data should not fail: %v", err)
	}

	if len(encrypted) != 0 {
		t.Errorf("expected empty encrypted data, got %d bytes", len(encrypted))
	}
}