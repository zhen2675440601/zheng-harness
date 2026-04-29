package llm

import (
	"context"
	"fmt"

	"zheng-harness/internal/domain"
)

type StreamingEvent = domain.StreamingEvent

// StreamFallback wraps Generate into a minimal streaming sequence.
func StreamFallback(ctx context.Context, generate func(context.Context, Request) (Response, error), request Request, emit func(domain.StreamingEvent) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	response, err := generate(ctx, request)
	if err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	tokenEvent, err := domain.TokenDelta(0, response.Output)
	if err != nil {
		return fmt.Errorf("create token delta event: %w", err)
	}
	if err := emit(*tokenEvent); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	completeEvent, err := domain.SessionComplete("", "success")
	if err != nil {
		return fmt.Errorf("create session complete event: %w", err)
	}
	if err := emit(*completeEvent); err != nil {
		return err
	}

	return nil
}
