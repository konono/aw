package cmd

import (
	"os"

	"github.com/konono/aw/v4/internal/pipeline"
	"github.com/konono/aw/v4/internal/profile"
)

func buildExecutionContext(profileName string, p profile.Profile) (*pipeline.ExecutionContext, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	workDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	return &pipeline.ExecutionContext{
		Profile:     p,
		ProfileName: profileName,
		HomeDir:     homeDir,
		OrigWorkDir: workDir,
		WorkDir:     workDir,
	}, nil
}
