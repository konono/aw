//go:build unix

package platform

import "syscall"

// ExecReplace replaces the current process with the given binary.
// On Unix, this uses syscall.Exec which never returns on success.
func ExecReplace(binPath string, argv []string, env []string) error {
	return syscall.Exec(binPath, argv, env)
}
