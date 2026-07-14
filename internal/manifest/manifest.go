package manifest

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"maps"
	"regexp"
	"strings"

	"github.com/konono/aw/v4/internal/containerenv"
	"github.com/konono/aw/v4/internal/profile"
)

// Options configures manifest generation.
type Options struct {
	Profile      profile.Profile
	ProfileName  string
	InstanceName string // --name flag; empty = random suffix
	ImageName    string
	HomeDir      string
}

// Resource is a single Kubernetes resource with its YAML representation.
type Resource struct {
	Kind string
	Name string
	YAML []byte
}

// Generate produces all Kubernetes resources for the given profile.
func Generate(opts Options) ([]Resource, error) {
	name, err := resourceName(opts.ProfileName, opts.InstanceName)
	if err != nil {
		return nil, err
	}
	k := opts.Profile.Kubernetes
	namespace := k.EffectiveNamespace()
	mode := k.EffectiveMode()

	tool := opts.Profile.EffectiveTool()
	cenv := containerenv.FromUser(opts.Profile.EffectiveContainerUser())
	if k != nil && k.SessionLog {
		cenv.SessionLog = true
	}
	sc, inMemoryFiles := effectiveSecretsConfig(opts.Profile, opts.HomeDir)

	labels := standardLabels(name, opts.ProfileName, tool, string(mode))
	if k != nil && len(k.PodLabels) > 0 {
		maps.Copy(labels, k.PodLabels)
		// Restore reserved labels that must not be overridden by pod_labels.
		// app.kubernetes.io/instance is used as the Deployment selector.
		labels[labelAppInstance] = name
	}

	imageName := opts.ImageName
	if imageName == "" {
		imageName = "aw-container:latest"
	}

	var resources []Resource

	nsYAML, err := renderNamespace(namespace)
	if err != nil {
		return nil, fmt.Errorf("rendering namespace: %w", err)
	}
	resources = append(resources, Resource{
		Kind: "Namespace",
		Name: namespace,
		YAML: nsYAML,
	})

	if k == nil || k.ServiceAccount == "" {
		saYAML, err := renderServiceAccount(name, namespace)
		if err != nil {
			return nil, fmt.Errorf("rendering service account: %w", err)
		}
		resources = append(resources, Resource{
			Kind: "ServiceAccount",
			Name: name,
			YAML: saYAML,
		})
	}

	initCM, err := renderInitConfigMap(name, namespace)
	if err != nil {
		return nil, fmt.Errorf("rendering init configmap: %w", err)
	}
	resources = append(resources, initCM)

	toolCM, err := renderToolConfigMap(name, namespace, tool, opts.HomeDir, opts.Profile)
	if err != nil {
		return nil, fmt.Errorf("rendering tool configmap: %w", err)
	}
	hasToolConfig := toolCM != nil
	if hasToolConfig {
		resources = append(resources, *toolCM)
	}

	envSecret, err := renderEnvSecret(name, namespace, sc, opts.Profile)
	if err != nil {
		return nil, fmt.Errorf("rendering env secret: %w", err)
	}
	resources = append(resources, envSecret)

	fileSecretData := collectFileSecrets(sc, opts.HomeDir, inMemoryFiles)
	fileSecret, err := renderFileSecretFromData(name, namespace, fileSecretData)
	if err != nil {
		return nil, fmt.Errorf("rendering file secret: %w", err)
	}
	if fileSecret != nil {
		resources = append(resources, *fileSecret)
	}

	// Build the effective SecretsConfig based on actually collected data
	// so the Deployment only references files that actually exist.
	effectiveSC := effectiveSecretsForDeployment(sc, fileSecretData)

	if k != nil && k.WorkspaceSize != "" {
		pvc, err := renderPVC(name, namespace, k.WorkspaceSize, k.StorageClass)
		if err != nil {
			return nil, fmt.Errorf("rendering pvc: %w", err)
		}
		if pvc != nil {
			resources = append(resources, *pvc)
		}
	}

	dep, err := renderDeployment(name, namespace, imageName, opts.Profile, cenv, effectiveSC, hasToolConfig, labels)
	if err != nil {
		return nil, fmt.Errorf("rendering deployment: %w", err)
	}
	resources = append(resources, dep)

	return resources, nil
}

// RenderAll joins all resources into a single multi-document YAML.
func RenderAll(resources []Resource) []byte {
	var buf bytes.Buffer
	for i, r := range resources {
		if i > 0 {
			buf.WriteString("---\n")
		}
		buf.Write(r.YAML)
	}
	return buf.Bytes()
}

var resourceNameRe = regexp.MustCompile(`[^a-z0-9-]`)

func resourceName(profileName, instanceName string) (string, error) {
	sanitized := resourceNameRe.ReplaceAllString(strings.ToLower(profileName), "-")
	sanitized = strings.Trim(sanitized, "-")
	if sanitized == "" {
		sanitized = "default"
	}
	base := "aw-" + sanitized
	var name string
	if instanceName != "" {
		name = base + "-" + instanceName
	} else {
		name = base + "-" + randomSuffix()
	}
	if len(name) > 63 {
		return "", fmt.Errorf("generated resource name %q exceeds 63 characters (profile %q + name %q); use shorter names", name, profileName, instanceName)
	}
	return name, nil
}

func randomSuffix() string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}
