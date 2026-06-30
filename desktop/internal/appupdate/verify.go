package appupdate

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// trustedPublicKeysB64 holds std-base64 ed25519 public keys the app trusts for
// update signatures. Populated by the keygen step (Task 3.3). Multiple keys
// allow rotation (current + next).
var trustedPublicKeysB64 = []string{
	"gJtXGULfORxXOrSqlYvwX1JZVBCmahh5bN9aFUIW3Nk=", // current update-signing key
}

// Verifier verifies update artifacts. Production: DefaultVerifier(); tests inject keys.
type Verifier struct {
	TrustedKeys []ed25519.PublicKey
}

// DefaultVerifier builds a Verifier from the embedded trusted keys.
func DefaultVerifier() (*Verifier, error) {
	v := &Verifier{}
	for _, b := range trustedPublicKeysB64 {
		raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b))
		if err != nil {
			return nil, fmt.Errorf("decode trusted key: %w", err)
		}
		if len(raw) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("bad trusted key size %d", len(raw))
		}
		v.TrustedKeys = append(v.TrustedKeys, ed25519.PublicKey(raw))
	}
	if len(v.TrustedKeys) == 0 {
		return nil, fmt.Errorf("no trusted update keys embedded")
	}
	return v, nil
}

// VerifySignature checks sigB64 (std-base64) is a valid ed25519 signature over
// data by any trusted key.
func (v *Verifier) VerifySignature(data, sigB64 []byte) error {
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sigB64)))
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	for _, k := range v.TrustedKeys {
		if ed25519.Verify(k, data, sig) {
			return nil
		}
	}
	return fmt.Errorf("signature not valid for any trusted key")
}

// ChecksumFor parses sha256sum-style lines ("<hex>  <name>") and returns the hex
// digest for assetName.
func ChecksumFor(checksums []byte, assetName string) (string, error) {
	sc := bufio.NewScanner(bytes.NewReader(checksums))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 2 && f[1] == assetName {
			return f[0], nil
		}
	}
	return "", fmt.Errorf("no checksum for %q", assetName)
}

// VerifyFileSHA256 checks the file at path hashes to expected (hex).
func VerifyFileSHA256(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, strings.TrimSpace(expected)) {
		return fmt.Errorf("sha256 mismatch: got %s want %s", got, expected)
	}
	return nil
}
