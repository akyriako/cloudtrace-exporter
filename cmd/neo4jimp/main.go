package main

import (
	"context"
	"fmt"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"log/slog"
	"os"
)

var (
	logger *slog.Logger
)

func init() {
	levelInfo := slog.LevelInfo
	logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: levelInfo,
	}))

	slog.SetDefault(logger)
}

func main() {
	ctx := context.TODO()

	c, err := cloudevents.NewClientHTTP()
	if err != nil {
		slog.Error("failed to create client: %s", err.Error())
		os.Exit(-1)
	}

	slog.Info(fmt.Sprintf("listening on port %d", 8080))

	if err := c.StartReceiver(ctx, receiveEvent); err != nil {
		slog.Error("failed to start receiver: %s", err.Error())
	}

	<-ctx.Done()
}

func receiveEvent(event cloudevents.Event) {
	slog.Info("received event", "id", event.ID(), "status", event.Extensions()["status"], "type", event.Type(), "source", event.Source(), "subject", event.Subject())
}
