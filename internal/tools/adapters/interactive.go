package adapters

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"zheng-harness/internal/domain"
)

const maxInteractiveAttempts = 3

// InteractiveAdapter prompts the CLI user for input through injected streams.
type InteractiveAdapter struct {
	in  io.Reader
	out io.Writer
}

// NewInteractiveAdapter constructs an ask_user adapter with injectable I/O.
func NewInteractiveAdapter(in io.Reader, out io.Writer) InteractiveAdapter {
	return InteractiveAdapter{in: in, out: out}
}

type askUserInput struct {
	Question string   `json:"question"`
	Options  []string `json:"options"`
}

func (a InteractiveAdapter) AskUser(ctx context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	start := time.Now()
	input, err := parseAskUserInput(call.Input)
	if err != nil {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
	}
	reader := bufio.NewReader(a.in)

	for attempt := 1; attempt <= maxInteractiveAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, userPromptTimeoutError(err)
		}

		if err := a.printPrompt(input); err != nil {
			return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
		}

		response, err := readLine(ctx, reader)
		if err != nil {
			return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
		}

		if len(input.Options) == 0 {
			return domain.ToolResult{ToolName: call.Name, Output: response, Duration: time.Since(start)}, nil
		}

		selection, ok := validateOptionSelection(response, input.Options)
		if ok {
			return domain.ToolResult{ToolName: call.Name, Output: selection, Duration: time.Since(start)}, nil
		}

		if _, err := fmt.Fprintf(a.out, "Invalid choice. Enter a number between 1 and %d.\n", len(input.Options)); err != nil {
			return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
		}
	}

	return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, fmt.Errorf("invalid choice after %d attempts", maxInteractiveAttempts)
}

func parseAskUserInput(raw string) (askUserInput, error) {
	var input askUserInput
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		return askUserInput{}, fmt.Errorf("ask_user input must be valid JSON: %w", err)
	}
	input.Question = strings.TrimSpace(input.Question)
	if input.Question == "" {
		return askUserInput{}, fmt.Errorf("ask_user question must not be empty")
	}
	for i, option := range input.Options {
		trimmed := strings.TrimSpace(option)
		if trimmed == "" {
			return askUserInput{}, fmt.Errorf("ask_user options[%d] must not be empty", i)
		}
		input.Options[i] = trimmed
	}
	return input, nil
}

func (a InteractiveAdapter) printPrompt(input askUserInput) error {
	if _, err := fmt.Fprintln(a.out, input.Question); err != nil {
		return err
	}
	for i, option := range input.Options {
		if _, err := fmt.Fprintf(a.out, "%d. %s\n", i+1, option); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprint(a.out, "> "); err != nil {
		return err
	}
	return nil
}

func readLine(ctx context.Context, reader *bufio.Reader) (string, error) {
	type readResult struct {
		line string
		err  error
	}

	resultCh := make(chan readResult, 1)
	go func() {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			resultCh <- readResult{err: err}
			return
		}
		resultCh <- readResult{line: strings.TrimSpace(line)}
	}()

	select {
	case <-ctx.Done():
		return "", userPromptTimeoutError(ctx.Err())
	case result := <-resultCh:
		if result.err != nil {
			return "", result.err
		}
		return result.line, nil
	}
}

func validateOptionSelection(raw string, options []string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false
	}
	for i, option := range options {
		if trimmed == fmt.Sprintf("%d", i+1) {
			return option, true
		}
	}
	return "", false
}

func userPromptTimeoutError(err error) error {
	if err == nil {
		return nil
	}
	if err == context.Canceled || err == context.DeadlineExceeded {
		return fmt.Errorf("user did not respond in time")
	}
	return err
}
