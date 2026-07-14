package manifest

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/konono/aw/internal/containerenv"
	"github.com/konono/aw/internal/pathutil"
	"github.com/konono/aw/internal/profile"
	"github.com/konono/aw/internal/stage"
)

func renderEnvSecret(name, namespace string, sc *profile.SecretsConfig, p profile.Profile) (Resource, error) {
	envData := collectEnvSecrets(sc, p)

	secret := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]interface{}{
			"name":      name + "-env-secrets",
			"namespace": namespace,
			"labels": map[string]string{
				labelManagedBy: "aw",
			},
		},
		"type": "Opaque",
	}

	if len(envData) > 0 {
		secret["stringData"] = envData
	} else {
		secret["stringData"] = map[string]string{}
	}

	data, err := yaml.Marshal(secret)
	if err != nil {
		return Resource{}, err
	}
	return Resource{Kind: "Secret", Name: name + "-env-secrets", YAML: data}, nil
}

// renderFileSecretFromData renders a file Secret from pre-collected data.
func renderFileSecretFromData(name, namespace string, fileData map[string]string) (*Resource, error) {
	if len(fileData) == 0 {
		return nil, nil
	}

	secret := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]interface{}{
			"name":      name + "-file-secrets",
			"namespace": namespace,
			"labels": map[string]string{
				labelManagedBy: "aw",
			},
		},
		"type": "Opaque",
	}

	bd := make(map[string]string, len(fileData))
	for k, v := range fileData {
		bd[k] = base64.StdEncoding.EncodeToString([]byte(v))
	}
	secret["data"] = bd

	data, err := yaml.Marshal(secret)
	if err != nil {
		return nil, err
	}
	r := Resource{Kind: "Secret", Name: name + "-file-secrets", YAML: data}
	return &r, nil
}

// effectiveSecretsForDeployment returns a SecretsConfig that only includes
// files that were actually collected, so the Deployment doesn't reference
// mounts/envs for files that couldn't be read.
func effectiveSecretsForDeployment(sc *profile.SecretsConfig, collectedData map[string]string) *profile.SecretsConfig {
	if sc == nil {
		return nil
	}
	var files []profile.SecretFile
	for _, f := range sc.Files {
		key := secretKeyForFile(f)
		if _, ok := collectedData[key]; ok {
			files = append(files, f)
		}
	}
	if len(sc.Env) == 0 && len(files) == 0 {
		return nil
	}
	return &profile.SecretsConfig{
		Env:   sc.Env,
		Files: files,
	}
}

func collectEnvSecrets(sc *profile.SecretsConfig, p profile.Profile) map[string]string {
	var result map[string]string

	if p.EffectiveGhToken() {
		if token := detectGhToken(); token != "" {
			if result == nil {
				result = make(map[string]string)
			}
			result["GITHUB_TOKEN"] = token
		}
	}

	if sc != nil {
		for _, entry := range sc.Env {
			key, val, hasValue := strings.Cut(entry, "=")
			if !hasValue {
				val = os.Getenv(key)
			}
			if val != "" {
				if result == nil {
					result = make(map[string]string)
				}
				result[key] = val
			} else {
				fmt.Fprintf(os.Stderr, "Warning: secrets.env: %s is not set\n", key)
			}
		}
	}

	return result
}

func collectFileSecrets(sc *profile.SecretsConfig, homeDir string, inMemoryFiles map[string][]byte) map[string]string {
	if sc == nil || len(sc.Files) == 0 {
		return nil
	}

	result := make(map[string]string)
	for _, f := range sc.Files {
		key := secretKeyForFile(f)
		if mem, ok := inMemoryFiles[f.MountPath]; ok {
			result[key] = string(mem)
			continue
		}
		src := pathutil.ExpandTilde(f.Source, homeDir)
		data, err := os.ReadFile(src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: secrets.files: %s: %v\n", f.Source, err)
			continue
		}
		result[key] = string(data)
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

var safeKeyRe = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// secretKeyForFile generates a unique Secret data key from a SecretFile.
// Derives the key from mountPath (guaranteed unique per volume mount) to
// avoid collisions between files with the same parent-dir/filename.
func secretKeyForFile(f profile.SecretFile) string {
	key := strings.TrimPrefix(f.MountPath, "/")
	key = safeKeyRe.ReplaceAllString(key, "-")
	if len(key) > 253 {
		key = key[:253]
	}
	return key
}

func detectGhToken() string {
	token, err := stage.DetectGhToken()
	if err != nil {
		return ""
	}
	return token
}

// secretFileVolumeMounts returns volume mounts for files in the file Secret.
func secretFileVolumeMounts(sc *profile.SecretsConfig) []volumeMount {
	if sc == nil {
		return nil
	}
	var mounts []volumeMount
	for _, f := range sc.Files {
		mounts = append(mounts, volumeMount{
			name:      "secret-files",
			mountPath: f.MountPath,
			subPath:   secretKeyForFile(f),
			readOnly:  true,
		})
	}
	return mounts
}

// secretFileEnvVars returns env vars that point to mounted secret files.
func secretFileEnvVars(sc *profile.SecretsConfig) []map[string]string {
	if sc == nil {
		return nil
	}
	var envs []map[string]string
	for _, f := range sc.Files {
		if f.Env != "" {
			envs = append(envs, map[string]string{
				"name":  f.Env,
				"value": f.MountPath,
			})
		}
	}
	return envs
}

type volumeMount struct {
	name      string
	mountPath string
	subPath   string
	readOnly  bool
}

type toolAuthResult struct {
	path    string // file path containing auth data (empty when inMemory is set)
	inMemory []byte // in-memory auth data (avoids temp file for Keychain data)
}

// detectToolAuth tries to find auth credentials for the given tool.
// For cursor: macOS Keychain → ~/.config/cursor/auth.json file fallback.
// For codex: ~/.codex/auth.json.
// Returns nil if no auth found. Keychain data is returned in-memory to avoid
// leaking credentials to temp files.
func detectToolAuth(tool, homeDir string) *toolAuthResult {
	switch tool {
	case "cursor":
		if data := cursorAuthFromKeychain(); data != nil {
			return &toolAuthResult{inMemory: data}
		}
		path := filepath.Join(homeDir, ".config", "cursor", "auth.json")
		if _, err := os.Stat(path); err == nil {
			return &toolAuthResult{path: path}
		}
	case "codex":
		path := filepath.Join(homeDir, ".codex", "auth.json")
		if _, err := os.Stat(path); err == nil {
			return &toolAuthResult{path: path}
		}
	}
	return nil
}

func cursorAuthFromKeychain() []byte {
	return stage.CursorAuthFromKeychain()
}

// HasSecretFiles returns true if the secrets config has files to mount.
func HasSecretFiles(sc *profile.SecretsConfig) bool {
	return sc != nil && len(sc.Files) > 0
}

// effectiveSecretsConfig returns the secrets config and any in-memory file data
// (e.g. from Keychain). Falls back to auto-detection when no explicit config is provided.
func effectiveSecretsConfig(p profile.Profile, homeDir string) (*profile.SecretsConfig, map[string][]byte) {
	if p.Kubernetes != nil && p.Kubernetes.Secrets != nil {
		return p.Kubernetes.Secrets, nil
	}
	return autoDetectSecrets(p, homeDir)
}

func autoDetectSecrets(p profile.Profile, homeDir string) (*profile.SecretsConfig, map[string][]byte) {
	var envVars []string
	var files []profile.SecretFile

	knownEnvVars := []string{
		"ANTHROPIC_API_KEY",
		"CLAUDE_CODE_USE_VERTEX", "ANTHROPIC_VERTEX_PROJECT_ID",
		"CLOUD_ML_PROJECT_ID", "GCP_PROJECT_ID",
		"ANTHROPIC_VERTEX_REGION", "CLOUD_ML_REGION", "GCP_REGION",
		"AWS_REGION", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY",
		"AWS_SESSION_TOKEN", "ANTHROPIC_BEDROCK_MODEL_ID",
		"OPENAI_API_KEY", "OPENAI_ORG_ID",
	}
	for _, v := range knownEnvVars {
		if os.Getenv(v) != "" {
			envVars = append(envVars, v)
		}
	}

	adcPath := filepath.Join(homeDir, ".config", "gcloud", "application_default_credentials.json")
	if _, err := os.Stat(adcPath); err == nil {
		files = append(files, profile.SecretFile{
			Source:    adcPath,
			MountPath: "/run/secrets/aw/gcloud/application_default_credentials.json",
			Env:       "GOOGLE_APPLICATION_CREDENTIALS",
		})
	}

	tool := p.EffectiveTool()
	cenv := containerenv.FromUser(p.EffectiveContainerUser())
	var inMemoryFiles map[string][]byte
	if authData := detectToolAuth(tool, homeDir); authData != nil {
		mountPath := cenv.ToolDir(tool) + "/auth.json"
		if authData.inMemory != nil {
			inMemoryFiles = map[string][]byte{mountPath: authData.inMemory}
			files = append(files, profile.SecretFile{
				Source:    "(keychain)",
				MountPath: mountPath,
			})
		} else {
			files = append(files, profile.SecretFile{
				Source:    authData.path,
				MountPath: mountPath,
			})
		}
	}

	if len(envVars) == 0 && len(files) == 0 {
		return nil, nil
	}
	return &profile.SecretsConfig{Env: envVars, Files: files}, inMemoryFiles
}
