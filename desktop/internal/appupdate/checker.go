package appupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"time"
)

const checksumsName = "checksums.txt"

// Checker queries a GitHub-Releases-compatible API and selects the asset for the
// running platform.
type Checker struct {
	HTTPClient *http.Client
	BaseURL    string
	Owner      string
	Repo       string
}

func NewChecker(owner, repo string) *Checker {
	return &Checker{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		BaseURL:    "https://api.github.com",
		Owner:      owner,
		Repo:       repo,
	}
}

type ghRelease struct {
	TagName string `json:"tag_name"`
	Body    string `json:"body"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
		Size int64  `json:"size"`
	} `json:"assets"`
}

// assetName returns this platform's asset file name, e.g. "Anya-darwin-arm64.zip".
func assetName() string {
	ext := "zip"
	if runtime.GOOS == "windows" {
		ext = "exe"
	}
	return fmt.Sprintf("Anya-%s-%s.%s", runtime.GOOS, runtime.GOARCH, ext)
}

// Latest fetches the newest release and returns UpdateInfo for this platform.
func (c *Checker) Latest(ctx context.Context) (*UpdateInfo, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", c.BaseURL, c.Owner, c.Repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github releases: status %d", resp.StatusCode)
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	want := assetName()
	info := &UpdateInfo{Version: rel.TagName, Notes: rel.Body, AssetName: want}
	for _, a := range rel.Assets {
		switch a.Name {
		case want:
			info.AssetURL = a.URL
			info.Size = a.Size
		case checksumsName:
			info.ChecksumsURL = a.URL
		case checksumsName + ".sig":
			info.SignatureURL = a.URL
		}
	}
	if info.AssetURL == "" {
		return nil, fmt.Errorf("no asset %q in release %s", want, rel.TagName)
	}
	if info.ChecksumsURL == "" || info.SignatureURL == "" {
		return nil, fmt.Errorf("release %s missing checksums/signature", rel.TagName)
	}
	return info, nil
}
