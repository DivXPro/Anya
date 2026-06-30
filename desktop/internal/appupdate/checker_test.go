package appupdate

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckerLatestSelectsPlatformAsset(t *testing.T) {
	want := assetName()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/o/r/releases/latest" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Fprintf(w, `{
		  "tag_name": "v1.4.0",
		  "body": "release notes",
		  "assets": [
		    {"name": %q, "browser_download_url": "https://dl/asset", "size": 123},
		    {"name": "checksums.txt", "browser_download_url": "https://dl/checksums.txt", "size": 10},
		    {"name": "checksums.txt.sig", "browser_download_url": "https://dl/checksums.txt.sig", "size": 5}
		  ]
		}`, want)
	}))
	defer srv.Close()

	c := NewChecker("o", "r")
	c.BaseURL = srv.URL
	info, err := c.Latest(context.Background())
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if info.Version != "v1.4.0" || info.AssetName != want {
		t.Fatalf("got version=%q asset=%q", info.Version, info.AssetName)
	}
	if info.AssetURL == "" || info.ChecksumsURL == "" || info.SignatureURL == "" {
		t.Fatalf("missing urls: %+v", info)
	}
}

func TestCheckerLatestMissingAsset(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"v1.0.0","assets":[]}`)
	}))
	defer srv.Close()
	c := NewChecker("o", "r")
	c.BaseURL = srv.URL
	if _, err := c.Latest(context.Background()); err == nil {
		t.Fatal("expected error when platform asset missing")
	}
}
