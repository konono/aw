package mise

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestFingerprint(t *testing.T) {
	t.Run("no mise config", func(t *testing.T) {
		if _, ok := Fingerprint(t.TempDir()); ok {
			t.Error("ok should be false without a mise config: there is nothing to skip")
		}
	})

	t.Run("mise.toml is covered", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "mise.toml", "[tools]\njq = \"latest\"\n")
		fp, ok := Fingerprint(dir)
		if !ok {
			t.Fatal("ok should be true for a plain mise.toml")
		}
		if fp == "" {
			t.Error("fingerprint should not be empty when ok is true")
		}
	})

	t.Run("stable across calls", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "mise.toml", "[tools]\njq = \"latest\"\n")
		first, _ := Fingerprint(dir)
		second, _ := Fingerprint(dir)
		if first != second {
			t.Errorf("fingerprint is not stable: %q then %q", first, second)
		}
	})

	t.Run("changes when a tool is added", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "mise.toml", "[tools]\njq = \"latest\"\n")
		before, _ := Fingerprint(dir)

		writeFile(t, dir, "mise.toml", "[tools]\njq = \"latest\"\nfd = \"latest\"\n")
		after, ok := Fingerprint(dir)
		if !ok {
			t.Fatal("ok should stay true after editing mise.toml")
		}
		if before == after {
			t.Error("appending a tool must change the fingerprint, otherwise the install is skipped")
		}
	})

	t.Run("changes when a tool is removed", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "mise.toml", "[tools]\njq = \"latest\"\nfd = \"latest\"\n")
		before, _ := Fingerprint(dir)

		writeFile(t, dir, "mise.toml", "[tools]\njq = \"latest\"\n")
		after, _ := Fingerprint(dir)
		if before == after {
			t.Error("removing a tool must change the fingerprint")
		}
	})

	t.Run("mise.toml and .mise.toml differ", func(t *testing.T) {
		body := "[tools]\njq = \"latest\"\n"

		dotted := t.TempDir()
		writeFile(t, dotted, ".mise.toml", body)
		dottedFP, ok := Fingerprint(dotted)
		if !ok {
			t.Fatal(".mise.toml should be covered too")
		}

		plain := t.TempDir()
		writeFile(t, plain, "mise.toml", body)
		plainFP, _ := Fingerprint(plain)

		if dottedFP == plainFP {
			t.Error("the same body in a different file must not collide: mise resolves them differently")
		}
	})

	t.Run("both files are covered together", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "mise.toml", "[tools]\njq = \"latest\"\n")
		onlyOne, _ := Fingerprint(dir)

		writeFile(t, dir, ".mise.toml", "[tools]\nfd = \"latest\"\n")
		both, ok := Fingerprint(dir)
		if !ok {
			t.Fatal("ok should be true with both snapshot files present")
		}
		if onlyOne == both {
			t.Error("adding .mise.toml must change the fingerprint")
		}
	})

	// Anything the snapshot does not copy cannot be described by the
	// fingerprint, so these must fall back to running mise install.
	t.Run("uncovered inputs disqualify the workspace", func(t *testing.T) {
		for _, name := range []string{
			"mise.lock",
			".tool-versions",
			"mise.ci.toml",
			".mise.ci.toml",
			"mise/config.toml",
			".mise/config.toml",
			".config/mise.toml",
		} {
			t.Run(name, func(t *testing.T) {
				dir := t.TempDir()
				writeFile(t, dir, "mise.toml", "[tools]\njq = \"latest\"\n")
				writeFile(t, dir, name, "")
				if _, ok := Fingerprint(dir); ok {
					t.Errorf("%s is not baked into the snapshot, so ok must be false", name)
				}
			})
		}
	})

	t.Run("include directive disqualifies the workspace", func(t *testing.T) {
		for _, body := range []string{
			"[[include]]\npath = \"other.toml\"\n",
			"[include]\npaths = [\"other.toml\"]\n",
			"include = [\"other.toml\"]\n",
		} {
			dir := t.TempDir()
			writeFile(t, dir, "mise.toml", body)
			if _, ok := Fingerprint(dir); ok {
				t.Errorf("an included file is not baked in, so ok must be false for:\n%s", body)
			}
		}
	})

	t.Run("the word include elsewhere is not a directive", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "mise.toml", "# include everything we need\n[tools]\nincludecpp = \"latest\"\n")
		if _, ok := Fingerprint(dir); !ok {
			t.Error("a comment and a tool name that merely contain 'include' should stay covered")
		}
	})
}
