package manifest

import "gopkg.in/yaml.v3"

func renderNamespace(namespace string) ([]byte, error) {
	ns := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata": map[string]interface{}{
			"name": namespace,
			"labels": map[string]string{
				labelManagedBy: "aw",
			},
		},
	}
	return yaml.Marshal(ns)
}

func renderServiceAccount(name, namespace string) ([]byte, error) {
	sa := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ServiceAccount",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
			"labels": map[string]string{
				labelManagedBy: "aw",
			},
		},
	}
	return yaml.Marshal(sa)
}
