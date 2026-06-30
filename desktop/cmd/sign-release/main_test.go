package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
)

func TestSignDataRoundTrip(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	privB64 := base64.StdEncoding.EncodeToString(priv)
	data := []byte("checksums-file-contents\n")

	sigB64, err := signData(privB64, data)
	if err != nil {
		t.Fatalf("signData: %v", err)
	}
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		t.Fatalf("decode sig: %v", err)
	}
	if !ed25519.Verify(pub, data, sig) {
		t.Fatal("signature did not verify with the generated public key")
	}
}
