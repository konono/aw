package manifest

// podSecurityContext returns OpenShift restricted-v2/v3-compatible pod security context.
//
// Design decisions:
//   - runAsUser/runAsGroup: omitted — OpenShift assigns UID from project range;
//     aw-init.sh injects /etc/passwd dynamically. Standard K8s uses Dockerfile's USER (1001).
//   - fsGroup/supplementalGroups: omitted — restricted-v2/v3 rejects GID 0.
//     OpenShift assigns from the namespace range automatically.
//   - hostUsers: false — required by restricted-v3.
func podSecurityContext() map[string]interface{} {
	return map[string]interface{}{
		"hostUsers": false,
	}
}

// containerSecurityContext returns per-container security context
// compliant with the Kubernetes restricted Pod Security Standard.
func containerSecurityContext() map[string]interface{} {
	return map[string]interface{}{
		"allowPrivilegeEscalation": false,
		"runAsNonRoot":             true,
		"capabilities": map[string]interface{}{
			"drop": []string{"ALL"},
		},
		"seccompProfile": map[string]string{
			"type": "RuntimeDefault",
		},
	}
}
