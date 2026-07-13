package manifest

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/konono/aw/internal/pathutil"
	"github.com/konono/aw/internal/profile"
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

func renderFileSecret(name, namespace, homeDir string, sc *profile.SecretsConfig) (*Resource, error) {
	fileData := collectFileSecrets(sc, homeDir)

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
		for _, envVar := range sc.Env {
			if val := os.Getenv(envVar); val != "" {
				if result == nil {
					result = make(map[string]string)
				}
				result[envVar] = val
			} else {
				fmt.Fprintf(os.Stderr, "Warning: secrets.env: %s is not set\n", envVar)
			}
		}
	}

	return result
}

func collectFileSecrets(sc *profile.SecretsConfig, homeDir string) map[string]string {
	if sc == nil || len(sc.Files) == 0 {
		return nil
	}

	result := make(map[string]string)
	for _, f := range sc.Files {
		src := pathutil.ExpandTilde(f.Source, homeDir)
		data, err := os.ReadFile(src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: secrets.files: %s: %v\n", f.Source, err)
			continue
		}
		key := secretKeyForFile(f)
		result[key] = string(data)
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

var safeKeyRe = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// secretKeyForFile generates a unique Secret data key from a SecretFile.
// Uses the parent directory name as a prefix to avoid basename collisions.
func secretKeyForFile(f profile.SecretFile) string {
	dir := filepath.Base(filepath.Dir(f.Source))
	base := filepath.Base(f.Source)
	key := fmt.Sprintf("%s--%s", dir, base)
	return safeKeyRe.ReplaceAllString(key, "-")
}

func detectGhToken() string {
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
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

// HasSecretFiles returns true if the secrets config has files to mount.
func HasSecretFiles(sc *profile.SecretsConfig) bool {
	return sc != nil && len(sc.Files) > 0
}

// effectiveSecretsConfig returns the secrets config, falling back to auto-detection
// for known tools when no explicit config is provided.
func effectiveSecretsConfig(p profile.Profile, homeDir string) *profile.SecretsConfig {
	if p.Kubernetes != nil && p.Kubernetes.Secrets != nil {
		return p.Kubernetes.Secrets
	}
	return autoDetectSecrets(p, homeDir)
}

func autoDetectSecrets(p profile.Profile, homeDir string) *profile.SecretsConfig {
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

	if len(envVars) == 0 && len(files) == 0 {
		return nil
	}
	return &profile.SecretsConfig{Env: envVars, Files: files}
}
