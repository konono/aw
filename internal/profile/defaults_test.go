package profile

import (
	"bytes"
	"testing"
)

func TestDefaultConfigYAML_ExplicitlyDisablesMountSSH(t *testing.T) {
	if !bytes.Contains(DefaultConfigYAML(), []byte("mount_ssh: false")) {
		t.Fatal("DefaultConfigYAML() should explicitly include mount_ssh: false")
	}
}
