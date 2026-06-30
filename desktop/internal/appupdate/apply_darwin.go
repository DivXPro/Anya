//go:build darwin

package appupdate

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// NewApplier returns the macOS applier.
func NewApplier() Applier { return &darwinApplier{} }

type darwinApplier struct{ relaunchPath string }

// bundlePathFromExe derives the .app bundle dir from the running executable path,
// e.g. /Applications/Anya.app/Contents/MacOS/anya -> /Applications/Anya.app.
func bundlePathFromExe(exe string) (string, error) {
	marker := ".app/Contents/MacOS/"
	i := strings.Index(exe, marker)
	if i < 0 {
		return "", fmt.Errorf("executable %q is not inside a .app bundle", exe)
	}
	return exe[:i+len(".app")], nil
}

// swapDir atomically replaces dst with src: move dst aside, move src into place,
// remove the old copy. On failure it restores dst.
func swapDir(src, dst string) error {
	backup := dst + ".bak"
	_ = os.RemoveAll(backup)
	if err := os.Rename(dst, backup); err != nil {
		return fmt.Errorf("move current aside: %w", err)
	}
	if err := os.Rename(src, dst); err != nil {
		_ = os.Rename(backup, dst) // rollback
		return fmt.Errorf("move new into place: %w", err)
	}
	_ = os.RemoveAll(backup)
	return nil
}

func (a *darwinApplier) Apply(assetPath string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	bundle, err := bundlePathFromExe(exe)
	if err != nil {
		return err
	}
	if !writable(bundle) {
		return fmt.Errorf("cannot update: %s is not writable (move the app to /Applications and retry)", bundle)
	}

	work, err := os.MkdirTemp("", "anya-apply-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)

	if err := unzip(assetPath, work); err != nil {
		return fmt.Errorf("unzip: %w", err)
	}
	newBundle, err := findDotApp(work)
	if err != nil {
		return err
	}
	// Strip the quarantine attribute inherited from the download so the
	// relaunched (unsigned) app isn't blocked by Gatekeeper.
	_ = exec.Command("xattr", "-dr", "com.apple.quarantine", newBundle).Run()

	if err := swapDir(newBundle, bundle); err != nil {
		return err
	}
	a.relaunchPath = bundle
	return nil
}

func (a *darwinApplier) Relaunch() error {
	if a.relaunchPath == "" {
		return fmt.Errorf("nothing to relaunch")
	}
	if err := exec.Command("open", a.relaunchPath).Start(); err != nil {
		return err
	}
	go func() { os.Exit(0) }()
	return nil
}

func writable(path string) bool {
	probe := filepath.Join(path, ".anya-write-probe")
	if err := os.WriteFile(probe, []byte("x"), 0o644); err != nil {
		return false
	}
	_ = os.Remove(probe)
	return true
}

func findDotApp(root string) (string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasSuffix(e.Name(), ".app") {
			return filepath.Join(root, e.Name()), nil
		}
	}
	return "", fmt.Errorf("no .app found in %s", root)
}

func unzip(src, dst string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		target := filepath.Join(dst, f.Name)
		if !strings.HasPrefix(target, filepath.Clean(dst)+string(os.PathSeparator)) {
			return fmt.Errorf("zip slip: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, f.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}
		_, cErr := io.Copy(out, rc)
		rc.Close()
		out.Close()
		if cErr != nil {
			return cErr
		}
	}
	return nil
}
