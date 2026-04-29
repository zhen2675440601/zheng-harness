package llm

import (
	"bufio"
	"context"
	"io"
	"strings"
)

const sseDoneMarker = "[DONE]"

// ParseSSE extracts complete SSE data events from a stream.
func ParseSSE(ctx context.Context, reader io.Reader, emit func(string) error) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	dataLines := make([]string, 0, 4)
	flush := func() error {
		if len(dataLines) == 0 {
			return nil
		}

		chunk := strings.Join(dataLines, "\n")
		dataLines = dataLines[:0]

		if chunk == sseDoneMarker {
			return io.EOF
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		return emit(chunk)
	}

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}

		line := scanner.Text()
		if line == "" {
			if err := flush(); err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			}
			continue
		}

		if strings.HasPrefix(line, ":") {
			continue
		}

		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimPrefix(line, "data:")
		data = strings.TrimPrefix(data, " ")
		dataLines = append(dataLines, data)
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	if err := flush(); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}

	return ctx.Err()
}
