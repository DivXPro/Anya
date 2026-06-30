package appupdate

import "testing"

// TestDefaultVerifierHasKey confirms the embedded trusted update-signing key is
// present and parseable, so the production updater isn't disabled.
func TestDefaultVerifierHasKey(t *testing.T) {
	v, err := DefaultVerifier()
	if err != nil {
		t.Fatalf("DefaultVerifier: %v", err)
	}
	if len(v.TrustedKeys) == 0 {
		t.Fatal("expected at least one embedded trusted key")
	}
}
