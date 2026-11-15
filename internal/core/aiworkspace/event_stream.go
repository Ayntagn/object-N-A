package aiworkspace

import (
	"bufio"
	"context"
	"io"
	"strings"
)

// readEvents delivers full SSE data fields in wire order. The callback may end
// the stream when it receives the protocol's terminal event.
func readEvents(ctx context.Context, source io.Reader, receive func(string) (bool, error)) error {
	limited := &io.LimitedReader{R: source, N: maxResponseBytes + 1}
	scanner := bufio.NewScanner(limited)
	scanner.Buffer(make([]byte, 4096), maxResponseBytes+1)
	var data strings.Builder
	first := true
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		if limited.N == 0 {
			return errResponseTooLarge
		}
		line := scanner.Text()
		if first {
			line = strings.TrimPrefix(line, "\ufeff")
			first = false
		}
		if line == "" {
			if data.Len() == 0 {
				continue
			}
			stop, err := receive(strings.TrimSuffix(data.String(), "\n"))
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			if stop || err != nil {
				return err
			}
			data.Reset()
		} else if value, ok := strings.CutPrefix(line, "data:"); ok {
			data.WriteString(strings.TrimPrefix(value, " "))
			data.WriteByte('\n')
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if limited.N == 0 {
		return errResponseTooLarge
	}
	if err := scanner.Err(); err != nil {
		return transportError(ctx, err)
	}
	return nil
}
