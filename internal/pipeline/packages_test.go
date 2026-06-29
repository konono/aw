package pipeline

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCollectPackages(t *testing.T) {
	t.Run("global only", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "packages.txt"), []byte("jq\ntree\n"), 0644); err != nil {
			t.Fatal(err)
		}
		got := CollectPackages(dir, nil)
		want := []string{"jq", "tree"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("profile only", func(t *testing.T) {
		dir := t.TempDir()
		got := CollectPackages(dir, []string{"ripgrep", "fd-find"})
		want := []string{"ripgrep", "fd-find"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("global and profile dedup", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "packages.txt"), []byte("jq\ntree\n"), 0644); err != nil {
			t.Fatal(err)
		}
		got := CollectPackages(dir, []string{"tree", "ripgrep"})
		want := []string{"jq", "tree", "ripgrep"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("workspace packages.txt", func(t *testing.T) {
		globalDir := t.TempDir()
		workDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(globalDir, "packages.txt"), []byte("jq\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(workDir, "packages.txt"), []byte("tree\ncurl\n"), 0644); err != nil {
			t.Fatal(err)
		}
		got := CollectPackages(globalDir, nil, workDir)
		want := []string{"jq", "tree", "curl"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("workspace dedup with global and profile", func(t *testing.T) {
		globalDir := t.TempDir()
		workDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(globalDir, "packages.txt"), []byte("jq\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(workDir, "packages.txt"), []byte("jq\ntree\n"), 0644); err != nil {
			t.Fatal(err)
		}
		got := CollectPackages(globalDir, []string{"tree"}, workDir)
		want := []string{"jq", "tree"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("comments and blank lines ignored", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "packages.txt"), []byte("# comment\njq\n\n# another\ntree\n"), 0644); err != nil {
			t.Fatal(err)
		}
		got := CollectPackages(dir, nil)
		want := []string{"jq", "tree"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("no packages.txt files", func(t *testing.T) {
		dir := t.TempDir()
		got := CollectPackages(dir, nil)
		if len(got) != 0 {
			t.Errorf("expected empty, got %v", got)
		}
	})

	t.Run("multiple workDirs", func(t *testing.T) {
		globalDir := t.TempDir()
		workDir1 := t.TempDir()
		workDir2 := t.TempDir()
		if err := os.WriteFile(filepath.Join(workDir1, "packages.txt"), []byte("jq\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(workDir2, "packages.txt"), []byte("tree\n"), 0644); err != nil {
			t.Fatal(err)
		}
		got := CollectPackages(globalDir, nil, workDir1, workDir2)
		want := []string{"jq", "tree"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}
