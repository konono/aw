package profile

import "strings"

// Describe returns a short human-readable summary of a profile.
func Describe(p Profile) string {
	parts := []string{}
	if p.Worktree != nil {
		parts = append(parts, "worktree")
	}
	parts = append(parts, string(p.Environment))
	parts = append(parts, string(p.Launch))
	if p.OS != "" {
		parts = append(parts, "os:"+string(p.OS))
	}
	if p.Image != "" {
		parts = append(parts, "image:"+p.Image)
	}
	if p.Dockerfile != "" {
		parts = append(parts, "dockerfile:"+p.Dockerfile)
	}
	return strings.Join(parts, " + ")
}

// FeatureFlags returns enabled optional profile features for verbose display.
func FeatureFlags(p Profile) string {
	var features []string
	if p.EffectiveGhToken() {
		features = append(features, "gh_token")
	}
	if p.EffectiveMountGH() {
		features = append(features, "mount_gh")
	}
	if p.EffectiveMountSSH() {
		features = append(features, "mount_ssh")
	}
	if p.EffectiveSSHAgentForwarding() {
		features = append(features, "ssh_agent_forwarding")
	}
	if p.EffectiveMountContainerSock() {
		features = append(features, "mount_container_sock")
	}
	return strings.Join(features, " + ")
}
