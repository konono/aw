package manifest

import (
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/konono/aw/internal/containerenv"
	"github.com/konono/aw/internal/launcher"
	"github.com/konono/aw/internal/profile"
	"github.com/konono/aw/internal/toolinfo"
)

func renderDeployment(name, namespace, imageName string, p profile.Profile, cenv containerenv.Config, sc *profile.SecretsConfig, hasToolConfig bool, labels map[string]string) (Resource, error) {
	k := p.Kubernetes
	mode := k.EffectiveMode()
	tool := p.EffectiveTool()
	toolDir := cenv.ToolDir(tool)
	hasSecretFiles := HasSecretFiles(sc)

	sa := name
	if k != nil && k.ServiceAccount != "" {
		sa = k.ServiceAccount
	}

	selectorLabels := map[string]string{
		labelAppInstance: labels[labelAppInstance],
	}

	container := buildMainContainer(imageName, name, tool, toolDir, mode, cenv, p, hasSecretFiles, hasToolConfig, sc, k)

	podSpec := map[string]interface{}{
		"serviceAccountName": sa,
		"securityContext":    podSecurityContext(),
		"containers":        []interface{}{container},
		"volumes":           buildVolumes(name, k, hasSecretFiles, hasToolConfig),
	}

	if k != nil && len(k.NodeSelector) > 0 {
		podSpec["nodeSelector"] = k.NodeSelector
	}
	if k != nil && len(k.Tolerations) > 0 {
		podSpec["tolerations"] = buildTolerations(k.Tolerations)
	}
	if k != nil && len(k.ImagePullSecrets) > 0 {
		secrets := make([]map[string]string, len(k.ImagePullSecrets))
		for i, s := range k.ImagePullSecrets {
			secrets[i] = map[string]string{"name": s}
		}
		podSpec["imagePullSecrets"] = secrets
	}

	if hasToolConfig {
		podSpec["initContainers"] = []interface{}{buildInitContainer(imageName, toolDir)}
	}

	templateMeta := map[string]interface{}{
		"labels": labels,
	}
	if k != nil && len(k.PodAnnotations) > 0 {
		templateMeta["annotations"] = k.PodAnnotations
	}

	deployment := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
			"labels":    labels,
		},
		"spec": map[string]interface{}{
			"replicas": 1,
			"selector": map[string]interface{}{
				"matchLabels": selectorLabels,
			},
			"template": map[string]interface{}{
				"metadata": templateMeta,
				"spec":     podSpec,
			},
		},
	}

	data, err := yaml.Marshal(deployment)
	if err != nil {
		return Resource{}, err
	}

	return Resource{Kind: "Deployment", Name: name, YAML: data}, nil
}

func buildMainContainer(imageName, name, tool, toolDir string, mode profile.KubernetesMode, cenv containerenv.Config, p profile.Profile, hasSecretFiles, hasToolConfig bool, sc *profile.SecretsConfig, k *profile.KubernetesConfig) map[string]interface{} {
	pullPolicy := "IfNotPresent"
	if strings.HasSuffix(imageName, ":latest") {
		pullPolicy = "Always"
	}

	container := map[string]interface{}{
		"name":            "agent",
		"image":           imageName,
		"imagePullPolicy": pullPolicy,
		"command":         []string{"/entrypoint.sh"},
	}

	if mode == profile.KubernetesModeChat {
		container["args"] = []string{"sleep", "infinity"}
	} else {
		cmd := launcher.ToolContainerCommand(tool)
		if cmd == nil {
			cmd = []string{"/bin/bash"}
		}
		container["args"] = cmd
		container["stdin"] = true
		container["tty"] = true
	}

	// Env vars: core + tool-specific + profile env + secret file env
	envVars := []map[string]string{
		{"name": "AW_USER", "value": cenv.User},
		{"name": "AW_HOME", "value": cenv.Home},
		{"name": "HOST_WORKSPACE", "value": cenv.Workspace},
	}

	// Tool-specific env vars (AW_CONTAINER_CONFIG_DIR, AW_DATA_SYMLINKS)
	toolEnv := toolinfo.ContainerEnvVarsFor(nil, tool, cenv)
	keys := make([]string, 0, len(toolEnv))
	for k := range toolEnv {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		envVars = append(envVars, map[string]string{"name": k, "value": toolEnv[k]})
	}

	// Profile-level env vars
	if len(p.Env) > 0 {
		profileKeys := make([]string, 0, len(p.Env))
		for k := range p.Env {
			profileKeys = append(profileKeys, k)
		}
		sort.Strings(profileKeys)
		for _, k := range profileKeys {
			envVars = append(envVars, map[string]string{"name": k, "value": p.Env[k]})
		}
	}

	// Secret file env vars (e.g. GOOGLE_APPLICATION_CREDENTIALS -> mountPath)
	envVars = append(envVars, secretFileEnvVars(sc)...)
	container["env"] = envVars

	// envFrom: only env secrets (not file secrets)
	container["envFrom"] = []map[string]interface{}{
		{
			"secretRef": map[string]interface{}{
				"name":     name + "-env-secrets",
				"optional": true,
			},
		},
	}

	container["volumeMounts"] = buildVolumeMounts(toolDir, hasSecretFiles, hasToolConfig, sc)
	container["securityContext"] = containerSecurityContext()

	if k != nil && k.Resources != nil {
		res := map[string]interface{}{}
		if len(k.Resources.Requests) > 0 {
			res["requests"] = k.Resources.Requests
		}
		if len(k.Resources.Limits) > 0 {
			res["limits"] = k.Resources.Limits
		}
		container["resources"] = res
	}

	return container
}

func buildInitContainer(imageName, toolDir string) map[string]interface{} {
	pullPolicy := "IfNotPresent"
	if strings.HasSuffix(imageName, ":latest") {
		pullPolicy = "Always"
	}
	return map[string]interface{}{
		"name":            "init-tool-config",
		"image":           imageName,
		"imagePullPolicy": pullPolicy,
		"command":         []string{"cp", "-rL", "/tool-config-ro/.", toolDir + "/"},
		"volumeMounts": []map[string]interface{}{
			{
				"name":      "tool-config-ro",
				"mountPath": "/tool-config-ro",
				"readOnly":  true,
			},
			{
				"name":      "tool-config",
				"mountPath": toolDir,
			},
		},
		"securityContext": containerSecurityContext(),
	}
}

func buildVolumeMounts(toolDir string, hasSecretFiles, hasToolConfig bool, sc *profile.SecretsConfig) []map[string]interface{} {
	mounts := []map[string]interface{}{
		{
			"name":      "init-script",
			"mountPath": "/aw-init.sh",
			"subPath":   "aw-init.sh",
			"readOnly":  true,
		},
	}

	if hasToolConfig {
		mounts = append(mounts, map[string]interface{}{
			"name":      "tool-config",
			"mountPath": toolDir,
		})
	}

	if hasSecretFiles {
		for _, m := range secretFileVolumeMounts(sc) {
			mounts = append(mounts, map[string]interface{}{
				"name":      m.name,
				"mountPath": m.mountPath,
				"subPath":   m.subPath,
				"readOnly":  true,
			})
		}
	}

	mounts = append(mounts, map[string]interface{}{
		"name":      "workspace",
		"mountPath": "/workspace",
	})

	return mounts
}

func buildVolumes(name string, k *profile.KubernetesConfig, hasSecretFiles bool, hasToolConfig bool) []map[string]interface{} {
	volumes := []map[string]interface{}{
		{
			"name": "init-script",
			"configMap": map[string]interface{}{
				"name":        name + "-init",
				"defaultMode": 0o755,
			},
		},
	}

	if hasToolConfig {
		volumes = append(volumes,
			map[string]interface{}{
				"name": "tool-config-ro",
				"configMap": map[string]interface{}{
					"name":     name + "-tool-config",
					"optional": true,
				},
			},
			map[string]interface{}{
				"name":     "tool-config",
				"emptyDir": map[string]interface{}{},
			},
		)
	}

	if hasSecretFiles {
		volumes = append(volumes, map[string]interface{}{
			"name": "secret-files",
			"secret": map[string]interface{}{
				"secretName": name + "-file-secrets",
				"optional":   true,
			},
		})
	}

	hasPVC := k != nil && k.WorkspaceSize != ""
	if hasPVC {
		volumes = append(volumes, map[string]interface{}{
			"name": "workspace",
			"persistentVolumeClaim": map[string]interface{}{
				"claimName": name + "-workspace",
			},
		})
	} else {
		volumes = append(volumes, map[string]interface{}{
			"name":     "workspace",
			"emptyDir": map[string]interface{}{},
		})
	}

	return volumes
}

func buildTolerations(tolerations []profile.Toleration) []map[string]string {
	result := make([]map[string]string, len(tolerations))
	for i, t := range tolerations {
		tol := map[string]string{
			"key": t.Key,
		}
		if t.Operator != "" {
			tol["operator"] = t.Operator
		}
		if t.Value != "" {
			tol["value"] = t.Value
		}
		if t.Effect != "" {
			tol["effect"] = t.Effect
		}
		result[i] = tol
	}
	return result
}
