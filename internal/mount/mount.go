package mount

import (
	"os"
	"path/filepath"

	"github.com/konono/aw/internal/docker"
)

// MountOptions contains the parameters needed to construct Docker mounts.
type MountOptions struct {
	HomeDir          string         // host user home directory
	WorkDir          string         // host working directory (workspace)
	ToolStageDir     string         // host staging dir for tool config (e.g. ~/.agent-workspace/claude)
	ToolContainerDir string         // container target for tool config (e.g. /home/agent/.claude)
	MountSSH         bool           // whether to mount host ~/.ssh into the container
	ExtraMounts      []docker.Mount // user-defined custom mounts
}

// Builder constructs Docker mount arguments.
type Builder interface {
	BuildMounts(opts MountOptions) ([]docker.Mount, error)
}

// DefaultBuilder is the default mount builder that checks the real filesystem.
type DefaultBuilder struct{}

// NewBuilder creates a new DefaultBuilder.
func NewBuilder() *DefaultBuilder {
	return &DefaultBuilder{}
}

// BuildMounts constructs the full list of Docker mounts for the container.
func (b *DefaultBuilder) BuildMounts(opts MountOptions) ([]docker.Mount, error) {
	var mounts []docker.Mount

	if opts.ToolStageDir != "" && opts.ToolContainerDir != "" {
		mounts = append(mounts, docker.Mount{
			Source: opts.ToolStageDir,
			Target: opts.ToolContainerDir,
		})
	}

	mounts = append(mounts, docker.Mount{
		Source: opts.WorkDir,
		Target: opts.WorkDir,
	})

	mounts = append(mounts, optionalMounts(opts.HomeDir, opts.MountSSH)...)

	worktreeMount, err := worktreeMount(opts.WorkDir)
	if err != nil {
		return nil, err
	}
	if worktreeMount != nil {
		mounts = append(mounts, *worktreeMount)
	}

	mounts = append(mounts, opts.ExtraMounts...)

	return mounts, nil
}

func optionalMounts(homeDir string, mountSSH bool) []docker.Mount {
	var mounts []docker.Mount

	gitconfig := filepath.Join(homeDir, ".gitconfig")
	if fileExists(gitconfig) {
		mounts = append(mounts, docker.Mount{
			Source: gitconfig,
			Target: "/home/agent/.gitconfig",
		})
	}

	ghConfig := filepath.Join(homeDir, ".config", "gh")
	if dirExists(ghConfig) {
		mounts = append(mounts, docker.Mount{
			Source: ghConfig,
			Target: "/home/agent/.config/gh",
		})
	}

	sshDir := filepath.Join(homeDir, ".ssh")
	if mountSSH && dirExists(sshDir) {
		mounts = append(mounts, docker.Mount{
			Source:   sshDir,
			Target:   "/home/agent/.ssh-host",
			ReadOnly: true,
		})
	}

	return mounts
}

func worktreeMount(workDir string) (*docker.Mount, error) {
	mainGitDir, err := DetectWorktree(workDir)
	if err != nil {
		return nil, err
	}
	if mainGitDir == "" {
		return nil, nil
	}

	if IsSubpath(workDir, mainGitDir) {
		return nil, nil
	}

	return &docker.Mount{
		Source: mainGitDir,
		Target: mainGitDir,
	}, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
