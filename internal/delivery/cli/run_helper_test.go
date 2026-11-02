package cli

import (
	"context"
	"io"
)

// Run executes a command without cancellation, for tests; the binary calls
// RunContext with the context of its process.
func Run(args []string, stdout, stderr io.Writer) int {
	return RunContext(context.Background(), args, stdout, stderr)
}
