package launcher

import (
	"context"

	"github.com/konono/aw/v4/internal/pipeline"
)

// Launcher executes the final "run something" step of the pipeline.
type Launcher interface {
	Launch(ctx context.Context, ec *pipeline.ExecutionContext) error
}
