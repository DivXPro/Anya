package appupdate

import (
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifierSignatureAndChecksum(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	v := &Verifier{TrustedKeys: []ed25519.PublicKey{pub}}

	checksums := []byte("abc123  Anya-darwin-arm64.zip\n")
	sigB64 := []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(priv, checksums)))

	if err := v.VerifySignature(checksums, sigB64); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	if err := v.VerifySignature([]byte("tampered\n"), sigB64); err == nil {
		t.Fatal("tampered data accepted")
	}

	sum, err := ChecksumFor(checksums, "Anya-darwin-arm64.zip")
	if err != nil || sum != "abc123" {
		t.Fatalf("ChecksumFor=%q err=%v", sum, err)
	}
	if _, err := ChecksumFor(checksums, "missing"); err == nil {
		t.Fatal("expected error for missing asset")
	}
}

func TestVerifyFileSHA256(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(p, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	// sha256("hello")
	const sum = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if err := VerifyFileSHA256(p, sum); err != nil {
		t.Fatalf("valid sha rejected: %v", err)
	}
	if err := VerifyFileSHA256(p, "deadbeef"); err == nil {
		t.Fatal("bad sha accepted")
	}
}
