package main

import (
	"context"
	"fmt"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"log/slog"
	"os"
)

var (
	logger       *slog.Logger
	eventsStream chan cloudevents.Event
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

	eventsStream = make(chan cloudevents.Event)
	defer close(eventsStream)

	go func() {
		processEventsStream()
	}()

	if err := c.StartReceiver(ctx, receiveEvent); err != nil {
		slog.Error("failed to start receiver: %s", err.Error())
	}

	<-ctx.Done()
}

func receiveEvent(event cloudevents.Event) {
	eventsStream <- event
}

func processEventsStream() {
	for event := range eventsStream {
		err := writeGraph(event)
		if err != nil {
			slog.Error(fmt.Sprintf("processing event failed: %s", err.Error()), "id", event.ID(), "source", event.Source())
		}

		slog.Info("processed event", "id", event.ID(), "source", event.Source())
	}
}

func writeGraph(event cloudevents.Event) error {
	return nil
}
