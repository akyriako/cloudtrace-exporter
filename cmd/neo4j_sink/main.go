package main

import (
	"context"
	"fmt"
	"github.com/akyriako/cloudtrace-exporter/pkg/neo4j"
	"github.com/caarlos0/env/v10"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"log/slog"
	"os"
)

var (
	logger       *slog.Logger
	eventsStream chan cloudevents.Event
	client       *neo4j.Client
	config       neo4j.ClientConfig
)

const (
	exitCodeConfigurationError int = 78
	exitCodeInternalError      int = 70
)

func init() {
	err := env.Parse(&config)
	if err != nil {
		slog.Error(fmt.Sprintf("parsing env variables failed: %s", err.Error()))
		os.Exit(exitCodeConfigurationError)
	}

	levelInfo := slog.LevelInfo
	logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: levelInfo,
	}))

	slog.SetDefault(logger)
}

func main() {
	ctx := context.TODO()

	nc, err := neo4j.NewClient(ctx, config)
	if err != nil {
		slog.Error(fmt.Sprintf("failed to create neo4j client: %s", err.Error()))
		os.Exit(exitCodeConfigurationError)
	}
	defer nc.Close(ctx)
	client = nc

	slog.Info("connected to neo4j instance", "uri", config.Uri)

	c, err := cloudevents.NewClientHTTP()
	if err != nil {
		slog.Error(fmt.Sprintf("failed to create cloudevents client: %s", err.Error()))
		os.Exit(exitCodeInternalError)
	}

	slog.Info("listening for cloudevents", "port", "8080")

	eventsStream = make(chan cloudevents.Event)
	defer close(eventsStream)

	go func(ctx context.Context) {
		processEventsStream(ctx)
	}(ctx)

	if err := c.StartReceiver(ctx, receiveEvent); err != nil {
		slog.Error(fmt.Sprintf("failed to start receiver: %s", err.Error()))
		os.Exit(-1)
	}

	<-ctx.Done()
}

func receiveEvent(event cloudevents.Event) {
	eventsStream <- event
}

func processEventsStream(ctx context.Context) {
	for event := range eventsStream {
		err := client.WriteEventGraph(ctx, event)
		if err != nil {
			slog.Error(fmt.Sprintf("processing event failed: %s", err.Error()), "id", event.ID(), "source", event.Source())
		}

		slog.Info("processed event", "id", event.ID(), "source", event.Source())
	}
}
