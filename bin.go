package main

import (
	"context"
	"github.com/ZenLiuCN/go-pkg/commands"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cc := withCancel()
	defer cc()
	if err := commands.Commands().Run(ctx, os.Args); err != nil {
		panic(err)
	}
}
func withCancel() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()
	return ctx, cancel
}
