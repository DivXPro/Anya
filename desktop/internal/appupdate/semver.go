package appupdate

import (
	"fmt"
	"strconv"
	"strings"
)

// IsNewer reports whether release version `latest` is strictly newer than the
// running `current`. Both accept an optional leading "v" and ignore any
// pre-release/build suffix. A current version of "dev" or "" is never older, so
// un-stamped local builds do not auto-update.
func IsNewer(latest, current string) (bool, error) {
	if current == "dev" || current == "" {
		return false, nil
	}
	lh, err := parseSemver(latest)
	if err != nil {
		return false, err
	}
	ch, err := parseSemver(current)
	if err != nil {
		return false, err
	}
	for i := 0; i < 3; i++ {
		if lh[i] != ch[i] {
			return lh[i] > ch[i], nil
		}
	}
	return false, nil
}

func parseSemver(v string) ([3]int, error) {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, fmt.Errorf("invalid semver %q", v)
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return out, fmt.Errorf("invalid semver %q: %w", v, err)
		}
		out[i] = n
	}
	return out, nil
}
