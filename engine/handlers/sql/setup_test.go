package sql

import (
	"github.com/Eric033/x-mate/engine/internal/context"
	"github.com/Eric033/x-mate/engine/internal/handler"
	"github.com/Eric033/x-mate/engine/internal/sampler"
)

func setupTestCtx() *context.TestContext {
	ctx := context.New()
	ctx.Set("user_id", "12345")
	ctx.Set("col", "name")
	ctx.Set("val_a", "100")
	ctx.Set("val_b", "hello")
	return ctx
}

// Re-export types for test convenience
var _ = handler.StepData{}
var _ = sampler.QueryResult{}
