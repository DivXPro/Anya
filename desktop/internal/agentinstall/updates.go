package agentinstall

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"desktop/internal/store"
)

var npmHTTP = &http.Client{}

var semverRe = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)

// latestNpmVersion fetches the latest published version of an npm package from
// the public registry. Scoped names like "@scope/name" are handled directly by
// the registry's "/<pkg>/latest" route.
func latestNpmVersion(ctx context.Context, pkg string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://registry.npmjs.org/"+pkg+"/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := npmHTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("registry status %d", resp.StatusCode)
	}
	var body struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return "", err
	}
	return body.Version, nil
}

// latestFromURL fetches a vendor endpoint that returns the latest version as
// plain text (redirects are followed). Used for agents with an official version
// endpoint instead of / in addition to npm (e.g. claude-code, kimi).
func latestFromURL(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := npmHTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// parseSemver extracts the first x.y.z triple found in s (agent --version output
// is free-form, e.g. "Claude Code 1.2.3" or "1.2.3 (abc)").
func parseSemver(s string) ([3]int, bool) {
	m := semverRe.FindStringSubmatch(s)
	if m == nil {
		return [3]int{}, false
	}
	var v [3]int
	for i := 0; i < 3; i++ {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return [3]int{}, false
		}
		v[i] = n
	}
	return v, true
}

// updateAvailable reports whether latest is a strictly newer semver than
// installed. If either version can't be parsed, it returns false (no update).
func updateAvailable(latest, installed string) bool {
	lv, ok := parseSemver(latest)
	if !ok {
		return false
	}
	iv, ok := parseSemver(installed)
	if !ok {
		return false
	}
	for i := 0; i < 3; i++ {
		if lv[i] != iv[i] {
			return lv[i] > iv[i]
		}
	}
	return false
}

// CheckUpdates queries the npm registry for every installed agent and returns a
// map of agent ID -> latest version for those whose installed version is older.
// Queries run concurrently; registry/network failures are ignored (treated as
// "no update available") so a slow or offline registry never blocks the UI
// beyond the caller's context deadline.
func (i *Installer) CheckUpdates(ctx context.Context) map[string]string {
	agents, err := store.ListAgents(i.db)
	if err != nil {
		return map[string]string{}
	}
	result := make(map[string]string)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, ag := range agents {
		info, ok := Registry[ag.ID]
		if !ok || !ag.Installed || ag.Version == nil {
			continue
		}
		if info.LatestVersionURL == "" && info.Packages["npm"] == "" {
			continue
		}
		wg.Add(1)
		go func(id string, info AgentInfo, installed string) {
			defer wg.Done()
			var (
				latest string
				err    error
			)
			if info.LatestVersionURL != "" {
				latest, err = latestFromURL(ctx, info.LatestVersionURL)
			} else {
				latest, err = latestNpmVersion(ctx, info.Packages["npm"])
			}
			if err != nil {
				return
			}
			if updateAvailable(latest, installed) {
				mu.Lock()
				result[id] = latest
				mu.Unlock()
			}
		}(ag.ID, info, *ag.Version)
	}
	wg.Wait()
	return result
}
