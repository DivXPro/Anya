package version

import "testing"

func TestDefaults(t *testing.T) {
	if Version == "" {
		t.Fatal("Version must have a non-empty default")
	}
	if RepoOwner == "" || RepoName == "" {
		t.Fatal("RepoOwner/RepoName must have defaults")
	}
}
