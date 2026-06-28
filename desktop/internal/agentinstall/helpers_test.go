package agentinstall

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func scriptExt() string {
	if runtime.GOOS == "windows" {
		return ".cmd"
	}
	return ""
}

func writeScript(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name+scriptExt())
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatalf("write script %s: %v", name, err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, 0755); err != nil {
			t.Fatalf("chmod %s: %v", name, err)
		}
	}
	return path
}

func writeFakeBinary(t *testing.T, dir, name, output string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		content := "@echo off\n" + output + "\n"
		return writeScript(t, dir, name, content)
	}
	content := "#!/bin/sh\necho \"" + output + "\"\n"
	return writeScript(t, dir, name, content)
}

func writeFakeNPM(t *testing.T, dir string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		content := fmt.Sprintf(`@echo off
if "%%1"=="prefix" if "%%2"=="-g" (
  echo %s
  exit /b 0
)
if "%%1"=="install" if "%%2"=="-g" (
  (
    echo @echo off
    echo echo Claude Code 0.1.0
  ) > "%s\claude.cmd"
  exit /b 0
)
exit /b 1
`, dir, dir)
		return writeScript(t, dir, "npm", content)
	}
	content := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "prefix" ] && [ "$2" = "-g" ]; then
  echo "%s"
  exit 0
fi
if [ "$1" = "install" ] && [ "$2" = "-g" ]; then
  cat > "%s/claude" <<'EOF'
#!/bin/sh
echo "Claude Code 0.1.0"
EOF
  chmod +x "%s/claude"
  exit 0
fi
exit 1
`, dir, dir, dir)
	return writeScript(t, dir, "npm", content)
}
