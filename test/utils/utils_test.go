package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetProjectDirStripsTestSubdirs(t *testing.T) {
	base := t.TempDir()
	// Resolve symlinks if any (e.g. /tmp on some systems)
	if resolved, err := filepath.EvalSymlinks(base); err == nil {
		base = resolved
	}

	for _, sub := range []string{"", filepath.Join("test", "e2e"), filepath.Join("test", "utils")} {
		dir := filepath.Join(base, sub)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		t.Chdir(dir)

		got, err := GetProjectDir()
		if err != nil {
			t.Fatalf("GetProjectDir(%s) failed: %v", sub, err)
		}
		if got != base {
			t.Errorf("GetProjectDir() from %q = %q, want %q", dir, got, base)
		}
	}
}
