package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAgentCWDValidation(t *testing.T) {
	tmp := t.TempDir()

	cases := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"empty is valid", "", false},
		{"existing dir", tmp, false},
		{"non-existent", filepath.Join(tmp, "missing"), true},
		{"file not dir", filepath.Join(tmp, "file"), true},
		{"relative path", "./relative", true},
	}

	// create a file for the "file not dir" case
	if f, err := os.Create(cases[3].path); err == nil {
		f.Close()
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateWorkingDirectory(c.path)
			if (err != nil) != c.wantErr {
				t.Fatalf("validateWorkingDirectory(%q) error = %v, wantErr %v", c.path, err, c.wantErr)
			}
		})
	}
}
