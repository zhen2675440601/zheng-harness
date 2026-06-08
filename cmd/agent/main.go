package main

import (
	"context"
	"os"
)

func main() {
	os.Exit(runMain(context.Background(), os.Args[1:]))
}

func runMain(ctx context.Context, args []string) int {
	return runCLI(ctx, args, os.Stdout, os.Stderr)
}
