package stage

import (
	"context"

	awauth "github.com/konono/aw/internal/auth"
	"github.com/konono/aw/internal/pipeline"
)

// AuthStage runs an explicit `aw auth ...` action.
type AuthStage struct {
	Action awauth.Action
}

func (s *AuthStage) Name() string { return "auth" }

func (s *AuthStage) Run(ctx context.Context, ec *pipeline.ExecutionContext) error {
	return awauth.Run(ctx, ec, s.Action)
}
