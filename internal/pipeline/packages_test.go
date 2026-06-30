package pipeline

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCollectPackages(t *testing.T) {
	t.Run("profile only", func(t *testing.T) {
		got := CollectPackages([]string{"ripgrep", "fd-find"})
		want := []string{"ripgrep", "fd-find"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("workspace packages.txt", func(t *testing.T) {
		workDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(workDir, "packages.txt"), []byte("jq\ntree\ncurl\n"), 0644); err != nil {
			t.Fatal(err)
		}
		got := CollectPackages(nil, workDir)
		want := []string{"jq", "tree", "curl"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("workspace dedup with profile", func(t *testing.T) {
		workDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(workDir, "packages.txt"), []byte("jq\ntree\n"), 0644); err != nil {
			t.Fatal(err)
		}
		got := CollectPackages([]string{"tree", "ripgrep"}, workDir)
		want := []string{"tree", "ripgrep", "jq"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("comments and blank lines ignored", func(t *testing.T) {
		workDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(workDir, "packages.txt"), []byte("# comment\njq\n\n# another\ntree\n"), 0644); err != nil {
			t.Fatal(err)
		}
		got := CollectPackages(nil, workDir)
		want := []string{"jq", "tree"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("no packages.txt files", func(t *testing.T) {
		got := CollectPackages(nil)
		if len(got) != 0 {
			t.Errorf("expected empty, got %v", got)
		}
	})

	t.Run("multiple workDirs", func(t *testing.T) {
		workDir1 := t.TempDir()
		workDir2 := t.TempDir()
		if err := os.WriteFile(filepath.Join(workDir1, "packages.txt"), []byte("jq\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(workDir2, "packages.txt"), []byte("tree\n"), 0644); err != nil {
			t.Fatal(err)
		}
		got := CollectPackages(nil, workDir1, workDir2)
		want := []string{"jq", "tree"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}
