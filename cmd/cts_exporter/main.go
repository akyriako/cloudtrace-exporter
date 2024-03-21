package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/akyriako/cloudtrace-exporter/pkg/adapter"
	"github.com/akyriako/opentelekomcloud/auth"
	"github.com/caarlos0/env/v10"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/hashicorp/go-multierror"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
)

type environment struct {
	Cloud          string `env:"OS_CLOUD"`
	Debug          bool   `env:"OS_DEBUG" envDefault:"false"`
	Tracker        string `env:"CTS_TRACKER" envDefault:"system"`
	From           uint   `env:"CTS_FROM" envDefault:"5"`
	PullAndPush    bool   `env:"CTS_X_PNP" envDefault:"false"`
	ProcessStreams bool   `env:"CTS_STREAMS" envDefault:"true"`
	SinkUrl        string `env:"K_SINK"`
	CeOverrides    string `env:"K_CE_OVERRIDES"`
}

var (
	config     environment
	logger     *slog.Logger
	from       uint
	ctsAdapter *adapter.Adapter
)

const (
	exitCodeConfigurationError          int  = 78
	exitCodeOpenTelekomCloudClientError int  = 70
	minFrom                             uint = 1
	maxFrom                             uint = 10800
)

func init() {
	err := env.Parse(&config)
	if err != nil {
		slog.Error(fmt.Sprintf("parsing env variables failed: %s", err.Error()))
		os.Exit(exitCodeConfigurationError)
	}

	flag.UintVar(&from, "from", 0, "the number of minutes between queries")

	levelInfo := slog.LevelInfo
	if config.Debug {
		levelInfo = slog.LevelDebug
	}

	logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: levelInfo,
	}))

	slog.SetDefault(logger)
}

func main() {
	flag.Parse()

	client, err := auth.NewOpenTelekomCloudClient(config.Cloud)
	if err != nil {
		slog.Error(fmt.Sprintf("acquiring an opentelekomcloud client failed: %s", strings.ToLower(err.Error())))
		os.Exit(exitCodeConfigurationError)
	}

	if from != 0 {
		err = fromInRange(from)
		if err != nil {
			slog.Error(err.Error())
			os.Exit(exitCodeConfigurationError)
		}

		config.From = from
	}

	cqc := adapter.CtsQuerierConfig{
		ProjectId:   client.ProjectClient.ProjectID,
		TrackerName: config.Tracker,
		From:        config.From,
	}

	sbc := adapter.SinkBindingConfig{
		SinkUrl:     config.SinkUrl,
		CeOverrides: config.CeOverrides,
	}

	ctsAdapter, err = adapter.NewAdapter(client, cqc, sbc)
	if err != nil {
		slog.Error(fmt.Sprintf("creating a cloud trace adapter failed: %s", err))
		os.Exit(exitCodeOpenTelekomCloudClientError)
	}

	slog.Info("started cloud trace adapter",
		"domain", client.ProjectClient.DomainID, "region", client.ProjectClient.RegionID, "project",
		client.ProjectClient.ProjectID, "tracker", config.Tracker, "interval", fmt.Sprintf("%vm", config.From), "sink", config.SinkUrl)

	interval := time.Duration(config.From) * time.Minute
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if config.ProcessStreams {
			go func() {
				receiveStream := make(chan cloudevents.Event)
				defer close(receiveStream)

				done := make(chan interface{})
				defer close(done)

				wg := sync.WaitGroup{}
				wg.Add(1)

				go func(receiveStream chan cloudevents.Event, done chan interface{}) {
					defer wg.Done()
					processEventStream(receiveStream, done)
				}(receiveStream, done)

				go func(receiveStream chan cloudevents.Event, done chan interface{}) {
					ctsAdapter.GetEventsStream(receiveStream, done)
				}(receiveStream, done)

				wg.Wait()
			}()
		} else {
			processEvents()
		}

		<-ticker.C
	}
}

func processEventStream(receiveStream <-chan cloudevents.Event, done <-chan interface{}) {
	sendStream := make(chan cloudevents.Event)
	defer close(sendStream)

	go func(sendStream <-chan cloudevents.Event) {
		ctsAdapter.SendEventsStream(sendStream)
	}(sendStream)

process:
	for {
		select {
		case event, ok := <-receiveStream:
			if !ok {
				break process
			}

			if config.PullAndPush {
				sendStream <- event
			}

			slog.Debug("processed event", "id", event.ID(), "status", event.Extensions()["status"], "type", event.Type(), "source", event.Source(), "subject", event.Subject())
		case <-done:
			break process
		}
	}
}

func processEvents() {
	events, err := ctsAdapter.GetEvents()
	if err != nil {
		slog.Error(fmt.Sprintf("querying cloud trace service failed: %s", err))
	}

	if config.Debug {
		for _, event := range events {
			slog.Debug("collected event", "id", event.ID(), "status", event.Extensions()["status"], "type", event.Type(), "source", event.Source(), "subject", event.Subject())
		}
	}

	if config.PullAndPush {
		if len(events) > 0 {
			sent, err := ctsAdapter.SendEvents(events)
			if err != nil {
				var merr *multierror.Error
				if errors.As(err, &merr) {
					for _, err := range merr.Errors {
						slog.Error(fmt.Sprintf("delivering cloud event failed: %s", err))
					}
				} else {
					slog.Error(fmt.Sprintf("delivering cloud events failed: %s", err))
				}
			}
			slog.Info(fmt.Sprintf("delivered %d/%d cloud events", sent, len(events)), "sink", config.SinkUrl)
		}
	}
}

func fromInRange(from uint) error {
	if from < minFrom || from > maxFrom {
		return fmt.Errorf("envvar 'from' out of range: %d and %d", minFrom, maxFrom)
	}

	return nil
}
