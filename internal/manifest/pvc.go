package manifest

import "gopkg.in/yaml.v3"

func renderPVC(name, namespace, size, storageClass string) (*Resource, error) {
	if size == "" {
		return nil, nil
	}

	pvc := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "PersistentVolumeClaim",
		"metadata": map[string]interface{}{
			"name":      name + "-workspace",
			"namespace": namespace,
			"labels": map[string]string{
				labelManagedBy: "aw",
			},
		},
		"spec": map[string]interface{}{
			"accessModes": []string{"ReadWriteOnce"},
			"resources": map[string]interface{}{
				"requests": map[string]string{
					"storage": size,
				},
			},
		},
	}

	if storageClass != "" {
		pvc["spec"].(map[string]interface{})["storageClassName"] = storageClass
	}

	data, err := yaml.Marshal(pvc)
	if err != nil {
		return nil, err
	}

	r := Resource{Kind: "PersistentVolumeClaim", Name: name + "-workspace", YAML: data}
	return &r, nil
}
