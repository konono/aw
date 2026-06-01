package profile

import (
	"bytes"
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

// Migrate produces an updated config YAML by merging user customizations into
// the current default template. The returned bytes preserve the template's
// comments and formatting while reflecting the user's settings.
func Migrate(userCfg *Config) ([]byte, error) {
	var templateDoc yaml.Node
	if err := yaml.Unmarshal(DefaultConfigYAML(), &templateDoc); err != nil {
		return nil, fmt.Errorf("parsing template: %w", err)
	}

	templateCfg, err := Parse(DefaultConfigYAML())
	if err != nil {
		return nil, fmt.Errorf("parsing template config: %w", err)
	}

	root := docRoot(&templateDoc)
	if root == nil {
		return nil, fmt.Errorf("template has no root mapping")
	}

	applyTopLevelScalars(root, userCfg, templateCfg)
	applyTopLevelEnv(root, userCfg, templateCfg)
	applyTopLevelAuth(root, userCfg, templateCfg)
	applyTopLevelWorktree(root, userCfg, templateCfg)
	applyTopLevelZellij(root, userCfg, templateCfg)
	applyTopLevelMounts(root, userCfg, templateCfg)

	profilesNode := findMappingValue(root, "profiles")
	if profilesNode != nil {
		applyProfileChanges(profilesNode, userCfg, templateCfg)
		addUserProfiles(profilesNode, userCfg, templateCfg)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&templateDoc); err != nil {
		return nil, fmt.Errorf("encoding migrated config: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("closing encoder: %w", err)
	}
	return buf.Bytes(), nil
}

func docRoot(doc *yaml.Node) *yaml.Node {
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return doc.Content[0]
	}
	return doc
}

func applyTopLevelScalars(root *yaml.Node, user, template *Config) {
	defs := user.Defaults.AsProfile()
	tmplDefs := template.Defaults.AsProfile()

	if user.Default != "" && user.Default != template.Default {
		setMappingScalar(root, "default", user.Default)
	}
	if defs.Environment != "" && defs.Environment != tmplDefs.Environment {
		setMappingScalar(root, "environment", string(defs.Environment))
	}
	if defs.OS != "" && defs.OS != tmplDefs.OS {
		setMappingScalar(root, "os", string(defs.OS))
	}
	if defs.Dockerfile != "" && defs.Dockerfile != tmplDefs.Dockerfile {
		setMappingScalar(root, "dockerfile", defs.Dockerfile)
	}
	if defs.ContainerRuntime != "" && defs.ContainerRuntime != tmplDefs.ContainerRuntime {
		setMappingScalar(root, "container_runtime", string(defs.ContainerRuntime))
	}
	if defs.MountSSH != nil && !equalBoolPtr(defs.MountSSH, tmplDefs.MountSSH) {
		setMappingScalar(root, "mount_ssh", fmt.Sprintf("%v", *defs.MountSSH))
	}
	if defs.MountGH != nil && !equalBoolPtr(defs.MountGH, tmplDefs.MountGH) {
		setMappingScalar(root, "mount_gh", fmt.Sprintf("%v", *defs.MountGH))
	}
	if defs.SSHAgentForwarding != nil && !equalBoolPtr(defs.SSHAgentForwarding, tmplDefs.SSHAgentForwarding) {
		setMappingScalar(root, "ssh_agent_forwarding", fmt.Sprintf("%v", *defs.SSHAgentForwarding))
	}
}

func applyTopLevelEnv(root *yaml.Node, user, template *Config) {
	userEnv := user.Defaults.Env
	tmplEnv := template.Defaults.Env
	if userEnv == nil || equalEnv(userEnv, tmplEnv) {
		return
	}
	envNode := findMappingValue(root, "env")
	if envNode == nil {
		appendMappingNode(root, "env", marshalValue(userEnv))
	} else {
		replaceNode(envNode, marshalValue(userEnv))
	}
}

func applyTopLevelAuth(root *yaml.Node, user, template *Config) {
	if user.Defaults.Auth == nil || equalAuth(user.Defaults.Auth, template.Defaults.Auth) {
		return
	}
	authNode := findMappingValue(root, "auth")
	if authNode == nil {
		appendMappingNode(root, "auth", marshalValue(user.Defaults.Auth))
	} else {
		replaceNode(authNode, marshalValue(user.Defaults.Auth))
	}
}

func applyTopLevelWorktree(root *yaml.Node, user, template *Config) {
	if user.Defaults.Worktree == nil {
		return
	}
	rel := relativeWorktree(template.Defaults.Worktree, user.Defaults.Worktree)
	if rel == nil {
		return
	}
	wtNode := findMappingValue(root, "worktree")
	if wtNode == nil {
		appendMappingNode(root, "worktree", marshalValue(user.Defaults.Worktree))
	} else {
		replaceNode(wtNode, marshalValue(user.Defaults.Worktree))
	}
}

func applyTopLevelZellij(root *yaml.Node, user, template *Config) {
	if user.Defaults.Zellij == nil {
		return
	}
	rel := relativeZellij(template.Defaults.Zellij, user.Defaults.Zellij)
	if rel == nil {
		return
	}
	zjNode := findMappingValue(root, "zellij")
	if zjNode == nil {
		appendMappingNode(root, "zellij", marshalValue(user.Defaults.Zellij))
	} else {
		replaceNode(zjNode, marshalValue(user.Defaults.Zellij))
	}
}

func applyTopLevelMounts(root *yaml.Node, user, template *Config) {
	if user.Defaults.Mounts == nil || equalMounts(user.Defaults.Mounts, template.Defaults.Mounts) {
		return
	}
	mnNode := findMappingValue(root, "mounts")
	if mnNode == nil {
		appendMappingNode(root, "mounts", marshalValue(user.Defaults.Mounts))
	} else {
		replaceNode(mnNode, marshalValue(user.Defaults.Mounts))
	}
}

func applyProfileChanges(profilesNode *yaml.Node, user, template *Config) {
	for name, userProfile := range user.Profiles {
		tmplProfile, isBuiltin := template.Profiles[name]
		if !isBuiltin {
			continue
		}
		relative := RelativeProfile(tmplProfile, MergeProfile(tmplProfile, userProfile))
		if isEmptyProfile(relative) {
			continue
		}
		profNode := findMappingValue(profilesNode, name)
		if profNode == nil {
			continue
		}
		merged := marshalValue(MergeProfile(tmplProfile, userProfile))
		replaceNode(profNode, merged)
	}
}

func addUserProfiles(profilesNode *yaml.Node, user, template *Config) {
	names := make([]string, 0)
	for name := range user.Profiles {
		if _, isBuiltin := template.Profiles[name]; !isBuiltin {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	for _, name := range names {
		p := user.Profiles[name]
		appendMappingNode(profilesNode, name, marshalValue(p))
	}
}

func isEmptyProfile(p Profile) bool {
	return p.Environment == "" &&
		p.Launch == "" &&
		p.Worktree == nil &&
		p.Zellij == nil &&
		p.Auth == nil &&
		p.Env == nil &&
		p.OS == "" &&
		p.Dockerfile == "" &&
		p.ContainerRuntime == "" &&
		p.MountGH == nil &&
		p.MountSSH == nil &&
		p.SSHAgentForwarding == nil &&
		p.Mounts == nil
}

// findMappingValue finds the value node for a given key in a mapping node.
func findMappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

// setMappingScalar updates a scalar value in a mapping node,
// or adds the key-value pair if the key is not found.
func setMappingScalar(mapping *yaml.Node, key, value string) {
	if mapping.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1].Value = value
			mapping.Content[i+1].Tag = ""
			mapping.Content[i+1].Style = 0
			return
		}
	}
	appendMappingNode(mapping, key, &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: value,
	})
}

func appendMappingNode(mapping *yaml.Node, key string, value *yaml.Node) {
	keyNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: key,
	}
	mapping.Content = append(mapping.Content, keyNode, value)
}

func replaceNode(dst *yaml.Node, src *yaml.Node) {
	*dst = *src
}

func marshalValue(v interface{}) *yaml.Node {
	var node yaml.Node
	data, err := yaml.Marshal(v)
	if err != nil {
		return &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%v", v)}
	}
	if err := yaml.Unmarshal(data, &node); err != nil {
		return &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%v", v)}
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return node.Content[0]
	}
	return &node
}

func equalEnv(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}
