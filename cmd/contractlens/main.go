package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/devenes/ContractLens/internal/cli"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	exitCode := cli.Execute(ctx, os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}
