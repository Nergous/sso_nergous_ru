package jwt

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

func LoadPrivateKeyPEM(path string) (ed25519.PrivateKey, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(file)
	if block == nil {
		return nil, fmt.Errorf("pem.Decode: %q: no PEM block found", path)
	}

	if block.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("unexpected PEM block type: %q", block.Type)
	}

	pk, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("x509.ParsePKCS8PrivateKey: %w", err)
	}
	ed, ok := pk.(ed25519.PrivateKey)

	if !ok {
		return nil, fmt.Errorf("not an Ed25519 private key, got %T", pk)
	}
	return ed, nil
}

func LoadPublicKeyPEM(path string) (ed25519.PublicKey, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(file)
	if block == nil {
		return nil, fmt.Errorf("pem.Decode: %q: no PEM block found", path)
	}

	if block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("unexpected PEM block type: %q", block.Type)
	}

	pk, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("x509.ParsePKIXPublicKey: %w", err)
	}

	ed, ok := pk.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an Ed25519 public key, got %T", pk)
	}
	return ed, nil
}

func GenerateKeyPair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return ed25519.GenerateKey(rand.Reader)
}
