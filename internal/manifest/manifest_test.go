package manifest

import (
	"strings"
	"testing"

	"github.com/konono/aw/v4/internal/profile"
)

func boolPtr(v bool) *bool { return &v }

func TestGenerate_InteractiveMode(t *testing.T) {
	p := profile.Profile{
		Environment: profile.EnvironmentContainer,
		Launch:      profile.LaunchClaude,
		Kubernetes: &profile.KubernetesConfig{
			Mode:      profile.KubernetesModeInteractive,
			Namespace: "test-ns",
		},
	}

	resources, err := Generate(Options{
		Profile:      p,
		ProfileName:  "claude",
		InstanceName: "dev1",
		ImageName:    "ghcr.io/test/aw-claude:latest",
		HomeDir:      "/tmp/test-home",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	yaml := string(RenderAll(resources))

	assertContains(t, yaml, "kind: Namespace")
	assertContains(t, yaml, "kind: ServiceAccount")
	assertContains(t, yaml, "kind: ConfigMap")
	assertContains(t, yaml, "kind: Secret")
	assertContains(t, yaml, "kind: Deployment")

	assertContains(t, yaml, "name: aw-claude-dev1")
	assertContains(t, yaml, "namespace: test-ns")
	assertContains(t, yaml, "image: ghcr.io/test/aw-claude:latest")
	assertContains(t, yaml, "- claude\n")
	assertContains(t, yaml, "- --permission-mode\n")
	assertContains(t, yaml, "- bypassPermissions\n")
	assertContains(t, yaml, "stdin: true")
	assertContains(t, yaml, "tty: true")
	assertContains(t, yaml, "aw.dev/mode: interactive")
}

func TestGenerate_ChatMode(t *testing.T) {
	p := profile.Profile{
		Environment: profile.EnvironmentContainer,
		Launch:      profile.LaunchClaude,
		Kubernetes: &profile.KubernetesConfig{
			Mode:      profile.KubernetesModeChat,
			Namespace: "chat-ns",
		},
	}

	resources, err := Generate(Options{
		Profile:      p,
		ProfileName:  "claude",
		InstanceName: "bot1",
		ImageName:    "ghcr.io/test/aw-claude:v1",
		HomeDir:      "/tmp/test-home",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	yaml := string(RenderAll(resources))

	assertContains(t, yaml, "- sleep\n")
	assertContains(t, yaml, "- infinity\n")
	assertNotContains(t, yaml, "stdin: true")
	assertNotContains(t, yaml, "tty: true")
	assertContains(t, yaml, "aw.dev/mode: chat")
}

func TestGenerate_RandomSuffix(t *testing.T) {
	p := profile.Profile{
		Environment: profile.EnvironmentContainer,
		Launch:      profile.LaunchClaude,
	}

	r1, _ := Generate(Options{Profile: p, ProfileName: "claude", ImageName: "img:1", HomeDir: "/tmp"})
	r2, _ := Generate(Options{Profile: p, ProfileName: "claude", ImageName: "img:1", HomeDir: "/tmp"})

	// Find the Deployment resource to check random name
	var dep1, dep2 string
	for _, r := range r1 {
		if r.Kind == "Deployment" {
			dep1 = r.Name
		}
	}
	for _, r := range r2 {
		if r.Kind == "Deployment" {
			dep2 = r.Name
		}
	}

	if dep1 == dep2 {
		t.Errorf("expected different random deployment names, got both %q", dep1)
	}
}

func TestGenerate_NamedInstance(t *testing.T) {
	p := profile.Profile{
		Environment: profile.EnvironmentContainer,
		Launch:      profile.LaunchClaude,
	}

	resources, _ := Generate(Options{
		Profile:      p,
		ProfileName:  "claude",
		InstanceName: "alice",
		ImageName:    "img:1",
		HomeDir:      "/tmp",
	})

	yaml := string(RenderAll(resources))
	assertContains(t, yaml, "name: aw-claude-alice")
	assertContains(t, yaml, "name: aw-claude-alice-init")
	assertContains(t, yaml, "name: aw-claude-alice-env-secrets")
}

func TestGenerate_SecurityContext(t *testing.T) {
	p := profile.Profile{
		Environment: profile.EnvironmentContainer,
		Launch:      profile.LaunchClaude,
	}

	resources, _ := Generate(Options{
		Profile:      p,
		ProfileName:  "claude",
		InstanceName: "sec",
		ImageName:    "img:1",
		HomeDir:      "/tmp",
	})

	yaml := string(RenderAll(resources))

	assertContains(t, yaml, "runAsNonRoot: true")
	assertContains(t, yaml, "allowPrivilegeEscalation: false")
	assertContains(t, yaml, "hostUsers: false")
	assertNotContains(t, yaml, "runAsUser:")
	assertNotContains(t, yaml, "runAsGroup:")
	assertNotContains(t, yaml, "fsGroup:")
	assertNotContains(t, yaml, "supplementalGroups:")
	assertNotContains(t, yaml, "AUDIT_WRITE")
}

func TestGenerate_WithResources(t *testing.T) {
	p := profile.Profile{
		Environment: profile.EnvironmentContainer,
		Launch:      profile.LaunchClaude,
		Kubernetes: &profile.KubernetesConfig{
			Resources: &profile.ResourceConfig{
				Requests: map[string]string{"cpu": "1", "memory": "2Gi"},
				Limits:   map[string]string{"cpu": "4", "memory": "8Gi"},
			},
		},
	}

	resources, _ := Generate(Options{
		Profile:      p,
		ProfileName:  "claude",
		InstanceName: "res",
		ImageName:    "img:1",
		HomeDir:      "/tmp",
	})

	yaml := string(RenderAll(resources))
	assertContains(t, yaml, "requests:")
	assertContains(t, yaml, `cpu: "1"`)
	assertContains(t, yaml, "memory: 2Gi")
	assertContains(t, yaml, "limits:")
	assertContains(t, yaml, `cpu: "4"`)
	assertContains(t, yaml, "memory: 8Gi")
}

func TestGenerate_WithPVC(t *testing.T) {
	p := profile.Profile{
		Environment: profile.EnvironmentContainer,
		Launch:      profile.LaunchClaude,
		Kubernetes: &profile.KubernetesConfig{
			WorkspaceSize: "10Gi",
			StorageClass:  "gp3",
		},
	}

	resources, _ := Generate(Options{
		Profile:      p,
		ProfileName:  "claude",
		InstanceName: "pvc",
		ImageName:    "img:1",
		HomeDir:      "/tmp",
	})

	yaml := string(RenderAll(resources))
	assertContains(t, yaml, "kind: PersistentVolumeClaim")
	assertContains(t, yaml, "storage: 10Gi")
	assertContains(t, yaml, "storageClassName: gp3")
	assertContains(t, yaml, "claimName: aw-claude-pvc-workspace")
}

func TestGenerate_WithTolerations(t *testing.T) {
	p := profile.Profile{
		Environment: profile.EnvironmentContainer,
		Launch:      profile.LaunchClaude,
		Kubernetes: &profile.KubernetesConfig{
			Tolerations: []profile.Toleration{
				{Key: "dedicated", Value: "ai", Effect: "NoSchedule"},
			},
			NodeSelector: map[string]string{"kubernetes.io/arch": "amd64"},
		},
	}

	resources, _ := Generate(Options{
		Profile:      p,
		ProfileName:  "claude",
		InstanceName: "tol",
		ImageName:    "img:1",
		HomeDir:      "/tmp",
	})

	yaml := string(RenderAll(resources))
	assertContains(t, yaml, "tolerations:")
	assertContains(t, yaml, "key: dedicated")
	assertContains(t, yaml, "value: ai")
	assertContains(t, yaml, "nodeSelector:")
	assertContains(t, yaml, "kubernetes.io/arch: amd64")
}

func TestGenerate_GhTokenSecret(t *testing.T) {
	p := profile.Profile{
		Environment: profile.EnvironmentContainer,
		Launch:      profile.LaunchClaude,
		GhToken:     boolPtr(true),
	}

	resources, _ := Generate(Options{
		Profile:      p,
		ProfileName:  "claude",
		InstanceName: "gh",
		ImageName:    "img:1",
		HomeDir:      "/tmp",
	})

	yaml := string(RenderAll(resources))
	assertContains(t, yaml, "GITHUB_TOKEN")
}

func TestGenerate_DefaultNamespace(t *testing.T) {
	p := profile.Profile{
		Environment: profile.EnvironmentContainer,
		Launch:      profile.LaunchClaude,
	}

	resources, _ := Generate(Options{
		Profile:      p,
		ProfileName:  "claude",
		InstanceName: "ns",
		ImageName:    "img:1",
		HomeDir:      "/tmp",
	})

	yaml := string(RenderAll(resources))
	assertContains(t, yaml, "namespace: aw")
}

func TestGenerate_CursorTool(t *testing.T) {
	p := profile.Profile{
		Environment: profile.EnvironmentContainer,
		Launch:      profile.LaunchCursor,
	}

	resources, _ := Generate(Options{
		Profile:      p,
		ProfileName:  "cursor",
		InstanceName: "cur",
		ImageName:    "img:1",
		HomeDir:      "/tmp",
	})

	yaml := string(RenderAll(resources))
	assertContains(t, yaml, "- agent\n")
	assertContains(t, yaml, "aw.dev/tool: cursor")
}

func TestGenerate_SelectorIsInstanceScoped(t *testing.T) {
	p := profile.Profile{
		Environment: profile.EnvironmentContainer,
		Launch:      profile.LaunchClaude,
	}

	r1, _ := Generate(Options{Profile: p, ProfileName: "claude", InstanceName: "alice", ImageName: "img:1", HomeDir: "/tmp"})
	r2, _ := Generate(Options{Profile: p, ProfileName: "claude", InstanceName: "bob", ImageName: "img:1", HomeDir: "/tmp"})

	y1 := string(RenderAll(r1))
	y2 := string(RenderAll(r2))

	assertContains(t, y1, "app.kubernetes.io/instance: aw-claude-alice")
	assertContains(t, y2, "app.kubernetes.io/instance: aw-claude-bob")
	assertNotContains(t, y1, "aw-claude-bob")
	assertNotContains(t, y2, "aw-claude-alice")
}

func TestGenerate_CustomSASkipsGeneration(t *testing.T) {
	p := profile.Profile{
		Environment: profile.EnvironmentContainer,
		Launch:      profile.LaunchClaude,
		Kubernetes: &profile.KubernetesConfig{
			ServiceAccount: "existing-sa",
		},
	}

	resources, _ := Generate(Options{Profile: p, ProfileName: "claude", InstanceName: "sa", ImageName: "img:1", HomeDir: "/tmp"})

	for _, r := range resources {
		if r.Kind == "ServiceAccount" {
			t.Error("expected no ServiceAccount resource when custom SA is specified")
		}
	}

	yaml := string(RenderAll(resources))
	assertContains(t, yaml, "serviceAccountName: existing-sa")
}

func TestGenerate_SessionLog(t *testing.T) {
	p := profile.Profile{
		Environment: profile.EnvironmentContainer,
		Launch:      profile.LaunchClaude,
		Kubernetes: &profile.KubernetesConfig{
			Mode:       profile.KubernetesModeInteractive,
			Namespace:  "test-ns",
			SessionLog: true,
		},
	}

	resources, err := Generate(Options{
		Profile:      p,
		ProfileName:  "claude",
		InstanceName: "slog",
		ImageName:    "ghcr.io/test/aw-claude:latest",
		HomeDir:      "/tmp/test-home",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	yaml := string(RenderAll(resources))

	assertContains(t, yaml, "name: AW_SESSION_LOG")
	assertContains(t, yaml, "stdin: false")
	assertContains(t, yaml, "tty: false")
}

func TestGenerate_SessionLogDisabled(t *testing.T) {
	p := profile.Profile{
		Environment: profile.EnvironmentContainer,
		Launch:      profile.LaunchClaude,
		Kubernetes: &profile.KubernetesConfig{
			Mode:      profile.KubernetesModeInteractive,
			Namespace: "test-ns",
		},
	}

	resources, err := Generate(Options{
		Profile:      p,
		ProfileName:  "claude",
		InstanceName: "no-slog",
		ImageName:    "ghcr.io/test/aw-claude:latest",
		HomeDir:      "/tmp/test-home",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	yaml := string(RenderAll(resources))

	assertNotContains(t, yaml, "name: AW_SESSION_LOG")
	assertContains(t, yaml, "stdin: true")
	assertContains(t, yaml, "tty: true")
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected output to contain %q, but it didn't.\nOutput (first 500 chars):\n%s", needle, truncate(haystack, 500))
	}
}

func assertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Errorf("expected output NOT to contain %q, but it did", needle)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
