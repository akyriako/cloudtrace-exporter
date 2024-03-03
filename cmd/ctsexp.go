package main

import (
	"flag"
	"fmt"
	"github.com/akyriako/cloudtrace-exporter/pkg/adapter"
	"github.com/akyriako/opentelekomcloud/auth"
	"github.com/caarlos0/env/v10"
	"github.com/davecgh/go-spew/spew"
	"log/slog"
	"os"
	"strings"
	"time"
)

type environment struct {
	Cloud       string `env:"OS_CLOUD"`
	Debug       bool   `env:"OS_DEBUG" envDefault:"false"`
	Tracker     string `env:"CTS_TRACKER" envDefault:"system"`
	From        uint   `env:"CTS_FROM" envDefault:"5"`
	PullAndPush bool   `env:"CTS_X_PNP" envDefault:"false"`
	SinkUrl     string `env:"K_SINK"`
	CeOverrides string `env:"K_CE_OVERRIDES"`
}

var (
	config environment
	logger *slog.Logger
	from   uint
)

const (
	exitCodeConfigurationError          int  = 1
	exitCodeOpenTelekomCloudClientError int  = 2
	exitCodeDeliveringCloudEventsError  int  = 3
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

func fromInRange(from uint) error {
	if from < minFrom || from > maxFrom {
		return fmt.Errorf("envvar 'from' out of range: %d and %d", minFrom, maxFrom)
	}

	return nil
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

	ctsAdapter, err := adapter.NewAdapter(client, cqc, sbc)
	if err != nil {
		slog.Error(fmt.Sprintf("creating an cloud trace adapter failed: %s", err))
		os.Exit(exitCodeOpenTelekomCloudClientError)
	}

	interval := time.Duration(config.From) * time.Minute
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		events, err := ctsAdapter.GetEvents()
		if err != nil {
			slog.Error(fmt.Sprintf("querying cloud trace service failed: %s", err))
		}

		if config.PullAndPush {
			err := ctsAdapter.SendEvents(events)
			if err != nil {
				slog.Error(fmt.Sprintf("delivering cloud events failed: %s", err))
			}
		} else {
			spew.Dump(events)
		}

		<-ticker.C
	}
}
