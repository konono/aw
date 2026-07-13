package manifest

const (
	labelAppName      = "app.kubernetes.io/name"
	labelAppInstance   = "app.kubernetes.io/instance"
	labelAppComponent = "app.kubernetes.io/component"
	labelManagedBy    = "app.kubernetes.io/managed-by"
	labelProfile      = "aw.dev/profile"
	labelTool         = "aw.dev/tool"
	labelMode         = "aw.dev/mode"
)

func standardLabels(resourceName, profileName, tool, mode string) map[string]string {
	return map[string]string{
		labelAppName:      "aw",
		labelAppInstance:   resourceName,
		labelAppComponent: "agent",
		labelManagedBy:    "aw",
		labelProfile:      profileName,
		labelTool:         tool,
		labelMode:         mode,
	}
}
