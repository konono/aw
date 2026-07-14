package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/konono/aw/v4/internal/config"
	"github.com/konono/aw/v4/internal/image"
	"github.com/konono/aw/v4/internal/profile"
	"github.com/konono/aw/v4/internal/toolinfo"
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
	cmData := collectToolConfigData(srcDir, *spec, tool)
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

func collectToolConfigData(srcDir string, spec config.ToolSyncSpec, tool string) map[string]string {
	data := make(map[string]string)
	for _, f := range spec.Files {
		path := filepath.Join(srcDir, f)
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if patcher, ok := spec.Patch[f]; ok {
			patched, err := patcher(content)
			if err == nil {
				content = patched
			}
		}
		if f == "settings.json" {
			content = stripHooksFromSettings(content)
		}
		data[f] = string(content)
	}
	return data
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
