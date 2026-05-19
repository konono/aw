package stage

import (
	"context"
	"fmt"
	"os"

	awauth "github.com/konono/aw/internal/auth"
	"github.com/konono/aw/internal/pipeline"
	"github.com/konono/aw/internal/profile"
)

// AuthCheckStage optionally checks auth state before a normal launch.
type AuthCheckStage struct{}

func (s *AuthCheckStage) Name() string { return "auth-check" }

func (s *AuthCheckStage) Run(ctx context.Context, ec *pipeline.ExecutionContext) error {
	mode := ec.Profile.EffectiveAuthOnLaunchCheck()
	if mode == "" || mode == profile.AuthOnLaunchCheckNone {
		return nil
	}

	result, err := awauth.CheckLaunch(ctx, ec)
	if err != nil {
		return err
	}

	if !result.Supported {
		return s.handle(mode, fmt.Sprintf("auth.on_launch.check をスキップしました: %s", result.Message))
	}
	if !result.LoggedIn {
		msg := result.Message
		if msg == "" {
			msg = fmt.Sprintf("%s はまだ認証されていないようです", ec.Profile.EffectiveTool())
		}
		return s.handle(mode, msg)
	}

	return nil
}

func (s *AuthCheckStage) handle(mode profile.AuthOnLaunchCheck, message string) error {
	switch mode {
	case profile.AuthOnLaunchCheckRequire:
		return fmt.Errorf("%s", message)
	case profile.AuthOnLaunchCheckWarn:
		fmt.Fprintf(os.Stderr, "Warning: %s\n", message)
	}
	return nil
}
