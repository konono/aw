package manifest

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/konono/aw/internal/config"
	"github.com/konono/aw/internal/image"
	"github.com/konono/aw/internal/profile"
	"github.com/konono/aw/internal/toolinfo"
)

func renderInitConfigMap(name, namespace string) (Resource, error) {
	initScript := string(image.InitScript())

	cm := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      name + "-init",
			"namespace": namespace,
			"labels": map[string]string{
				labelManagedBy: "aw",
			},
		},
		"data": map[string]string{
			"aw-init.sh": initScript,
		},
	}

	data, err := yaml.Marshal(cm)
	if err != nil {
		return Resource{}, err
	}

	return Resource{Kind: "ConfigMap", Name: name + "-init", YAML: data}, nil
}

func renderToolConfigMap(name, namespace, tool, homeDir string, p profile.Profile) (*Resource, error) {
	if tool == "" {
		return nil, nil
	}

	spec := toolSyncSpec(tool, p)
	if spec == nil {
		return nil, nil
	}

	srcDir := toolinfo.HomePath(tool, homeDir)
	cmData := collectToolConfigData(srcDir, *spec)
	if len(cmData) == 0 {
		return nil, nil
	}

	cm := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      name + "-tool-config",
			"namespace": namespace,
			"labels": map[string]string{
				labelManagedBy: "aw",
			},
		},
		"data": cmData,
	}

	data, err := yaml.Marshal(cm)
	if err != nil {
		return nil, err
	}

	r := Resource{Kind: "ConfigMap", Name: name + "-tool-config", YAML: data}
	return &r, nil
}

func toolSyncSpec(tool string, p profile.Profile) *config.ToolSyncSpec {
	switch tool {
	case "claude":
		return &config.ClaudeSyncSpec
	case "codex":
		spec := config.CodexSyncSpecWithOptions("file", "if_missing")
		return &spec
	case "opencode":
		return &config.OpenCodeSyncSpec
	case "cursor":
		return &config.CursorSyncSpec
	default:
		return nil
	}
}

func collectToolConfigData(srcDir string, spec config.ToolSyncSpec) map[string]string {
	data := make(map[string]string)

	for _, f := range spec.Files {
		if content := readToolConfigFile(srcDir, f, spec); content != nil {
			data[f] = string(content)
		}
	}

	for _, f := range spec.SeedFiles {
		// auth.json is injected via Secret in K8s, not ConfigMap.
		if filepath.Base(f) == "auth.json" {
			continue
		}
		if content := readToolConfigFile(srcDir, f, spec); content != nil {
			data[f] = string(content)
		}
	}

	for _, d := range spec.Dirs {
		collectToolConfigDir(data, filepath.Join(srcDir, d), d, spec)
	}

	return data
}

func readToolConfigFile(srcDir, relPath string, spec config.ToolSyncSpec) []byte {
	path := filepath.Join(srcDir, relPath)
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	if patcher, ok := spec.Patch[relPath]; ok {
		if patched, err := patcher(content); err == nil {
			content = patched
		}
	}
	if relPath == "settings.json" {
		content = stripHooksFromSettings(content)
	}
	return content
}

func collectToolConfigDir(data map[string]string, dirPath, dirKey string, spec config.ToolSyncSpec) {
	info, err := os.Stat(dirPath)
	if err != nil || !info.IsDir() {
		return
	}

	_ = filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dirPath, path)
		if err != nil {
			return nil
		}
		key := dirKey + "/" + filepath.ToSlash(rel)
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if patcher, ok := spec.Patch[key]; ok {
			if patched, err := patcher(content); err == nil {
				content = patched
			}
		}
		data[key] = string(content)
		return nil
	})
}

// stripHooksFromSettings removes hooks from settings.json for K8s.
// Hook scripts reference host paths that don't exist in the pod.
func stripHooksFromSettings(data []byte) []byte {
	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return data
	}
	delete(settings, "hooks")
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return data
	}
	return append(out, '\n')
}
