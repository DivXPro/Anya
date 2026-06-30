package appupdate

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

type fakeApplier struct{ applied, relaunched bool }

func (f *fakeApplier) Apply(string) error { f.applied = true; return nil }
func (f *fakeApplier) Relaunch() error    { f.relaunched = true; return nil }

type fakeEmitter struct {
	mu     sync.Mutex
	events []string
}

func (e *fakeEmitter) Emit(name string, _ any) {
	e.mu.Lock()
	e.events = append(e.events, name)
	e.mu.Unlock()
}

// newTestServer serves release JSON + asset/checksums/sig signed by priv.
func newTestServer(t *testing.T, priv ed25519.PrivateKey, asset []byte) *httptest.Server {
	t.Helper()
	name := assetName()
	sum := sha256.Sum256(asset)
	checksums := []byte(fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), name))
	sig := []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(priv, checksums)))

	mux := http.NewServeMux()
	var base string
	mux.HandleFunc("/repos/o/r/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"tag_name":"v9.9.9","body":"notes","assets":[
		  {"name":%q,"browser_download_url":"%s/a","size":%d},
		  {"name":"checksums.txt","browser_download_url":"%s/c","size":%d},
		  {"name":"checksums.txt.sig","browser_download_url":"%s/s","size":%d}]}`,
			name, base, len(asset), base, len(checksums), base, len(sig))
	})
	mux.HandleFunc("/a", func(w http.ResponseWriter, r *http.Request) { w.Write(asset) })
	mux.HandleFunc("/c", func(w http.ResponseWriter, r *http.Request) { w.Write(checksums) })
	mux.HandleFunc("/s", func(w http.ResponseWriter, r *http.Request) { w.Write(sig) })
	srv := httptest.NewServer(mux)
	base = srv.URL
	return srv
}

func TestManagerCheckAndApply(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	asset := []byte("the-new-app-bytes")
	srv := newTestServer(t, priv, asset)
	defer srv.Close()

	checker := NewChecker("o", "r")
	checker.BaseURL = srv.URL
	app := &fakeApplier{}
	emit := &fakeEmitter{}
	m := NewManager("v1.0.0", checker, &Verifier{TrustedKeys: []ed25519.PublicKey{pub}}, app, emit)

	info, err := m.CheckForUpdate(context.Background())
	if err != nil {
		t.Fatalf("CheckForUpdate: %v", err)
	}
	if info == nil || info.Version != "v9.9.9" {
		t.Fatalf("info=%+v", info)
	}
	if err := m.DownloadAndApply(context.Background()); err != nil {
		t.Fatalf("DownloadAndApply: %v", err)
	}
	if !app.applied || !app.relaunched {
		t.Fatal("applier not invoked")
	}
}

func TestManagerRejectsTamperedAsset(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	srv := newTestServer(t, priv, []byte("legit-bytes"))
	defer srv.Close()

	// Trust a DIFFERENT key so the (valid) signature fails verification.
	otherPub, _, _ := ed25519.GenerateKey(nil)
	_ = pub
	checker := NewChecker("o", "r")
	checker.BaseURL = srv.URL
	app := &fakeApplier{}
	m := NewManager("v1.0.0", checker, &Verifier{TrustedKeys: []ed25519.PublicKey{otherPub}}, app, &fakeEmitter{})

	if _, err := m.CheckForUpdate(context.Background()); err != nil {
		t.Fatalf("CheckForUpdate: %v", err)
	}
	if err := m.DownloadAndApply(context.Background()); err == nil {
		t.Fatal("expected verification failure, got nil")
	}
	if app.applied {
		t.Fatal("must NOT apply when signature is untrusted")
	}
}

func TestManagerUpToDate(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)
	srv := newTestServer(t, priv, []byte("x"))
	defer srv.Close()
	checker := NewChecker("o", "r")
	checker.BaseURL = srv.URL
	m := NewManager("v99.0.0", checker, &Verifier{}, &fakeApplier{}, &fakeEmitter{})
	info, err := m.CheckForUpdate(context.Background())
	if err != nil {
		t.Fatalf("CheckForUpdate: %v", err)
	}
	if info != nil {
		t.Fatalf("expected up-to-date (nil), got %+v", info)
	}
}

func TestManagerRejectsConcurrentApply(t *testing.T) {
	m := NewManager("v1.0.0", NewChecker("o", "r"), &Verifier{}, &fakeApplier{}, &fakeEmitter{})
	m.available = &UpdateInfo{Version: "v9.9.9", AssetName: "x"}
	m.inProgress.Store(true) // simulate an apply already running
	if err := m.DownloadAndApply(context.Background()); err == nil {
		t.Fatal("expected 'already in progress' error")
	}
}
