package main

import (
	"context"
	"os"
)

func main() {
	os.Exit(runServer(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}
