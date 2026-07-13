package manifest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/konono/aw/internal/config"
)

func TestCollectToolConfigData_FilesAndDirs(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "settings.json"), []byte(`{"hooks":{}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "CLAUDE.md"), []byte("# notes"), 0644); err != nil {
		t.Fatal(err)
	}
	hooksDir := filepath.Join(srcDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "pre.sh"), []byte("#!/bin/sh"), 0755); err != nil {
		t.Fatal(err)
	}

	data := collectToolConfigData(srcDir, config.ClaudeSyncSpec)

	if got := data["settings.json"]; got == "" {
		t.Fatal("expected settings.json in config data")
	}
	if got := data["CLAUDE.md"]; got != "# notes" {
		t.Fatalf("CLAUDE.md = %q, want %q", got, "# notes")
	}
	if got := data["hooks/pre.sh"]; got != "#!/bin/sh" {
		t.Fatalf("hooks/pre.sh = %q, want hook script", got)
	}
	if string(data["settings.json"]) == `{"hooks":{}}` {
		t.Fatal("expected hooks to be stripped from settings.json")
	}
}

func TestCollectToolConfigData_SkipsAuthSeedFile(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "cli-config.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "auth.json"), []byte(`{"token":"secret"}`), 0600); err != nil {
		t.Fatal(err)
	}

	data := collectToolConfigData(srcDir, config.CursorSyncSpec)

	if _, ok := data["auth.json"]; ok {
		t.Fatal("auth.json must not be included in tool ConfigMap")
	}
	if got := data["cli-config.json"]; got != "{}" {
		t.Fatalf("cli-config.json = %q, want {}", got)
	}
}

func TestCollectToolConfigData_MissingDirIgnored(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	data := collectToolConfigData(srcDir, config.ClaudeSyncSpec)
	if len(data) != 0 {
		t.Fatalf("expected empty data, got %v", data)
	}
}
