//go:build windows

package appupdate

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

// NewApplier returns the Windows applier.
func NewApplier() Applier { return &windowsApplier{} }

type windowsApplier struct{ relaunchPath string }

// replaceExe replaces the (possibly running) exe at dst with src. Windows allows
// renaming a running exe, so we move the running one aside (.old) then copy the
// new one into its path. The leftover .old is removed at the start of the next
// update (the os.Remove below).
func replaceExe(src, dst string) error {
	old := dst + ".old"
	_ = os.Remove(old)
	if err := os.Rename(dst, old); err != nil {
		return fmt.Errorf("move running exe aside: %w", err)
	}
	if err := copyFile(src, dst); err != nil {
		_ = os.Rename(old, dst) // rollback
		return fmt.Errorf("write new exe: %w", err)
	}
	return nil
}

func (a *windowsApplier) Apply(assetPath string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := replaceExe(assetPath, exe); err != nil {
		return err
	}
	a.relaunchPath = exe
	return nil
}

func (a *windowsApplier) Relaunch() error {
	if a.relaunchPath == "" {
		return fmt.Errorf("nothing to relaunch")
	}
	if err := exec.Command(a.relaunchPath).Start(); err != nil {
		return err
	}
	go func() { os.Exit(0) }()
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
