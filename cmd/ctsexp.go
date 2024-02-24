package main

import (
	"fmt"
	"github.com/akyriako/cloudtrace-exporter/pkg/adapter"
	otccommon "github.com/akyriako/opentelekomcloud/common"
	"github.com/caarlos0/env/v10"
	"github.com/davecgh/go-spew/spew"
	"log/slog"
	"os"
	"strings"
)

type environment struct {
	Cloud          string `env:"OS_CLOUD"`
	Debug          bool   `env:"OS_DEBUG" envDefault:"true"`
	Tracker        string `env:"CTS_TRACKER" envDefault:"system"`
	From           uint   `env:"CTS_FROM" envDefault:"5"`
	PullAndPush    bool   `env:"CTS_X_PNP" envDefault:"false"`
	K_SINK         string `env:"K_SINK"`
	K_CE_OVERRIDES string `env:"K_CE_OVERRIDES"`
}

var (
	config environment
	logger *slog.Logger
)

const (
	exitCodeConfigurationError          int = 1
	exitCodeOpenTelekomCloudClientError int = 2
	exitCodeDeliveringCloudEventsError  int = 3
)

func init() {
	err := env.Parse(&config)
	if err != nil {
		slog.Error(fmt.Sprintf("parsing env variables failed: %s", err.Error()))
		os.Exit(exitCodeConfigurationError)
	}

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
	client, err := otccommon.NewOpenTelekomCloudClient(config.Cloud)
	if err != nil {
		slog.Error(fmt.Sprintf("acquiring an opentelekomcloud client failed: %s", strings.ToLower(err.Error())))
		os.Exit(exitCodeConfigurationError)
	}

	cqc := adapter.CtsQuerierConfig{
		ProjectId:   client.ProjectClient.ProjectID,
		TrackerName: config.Tracker,
		From:        config.From,
	}

	sbc := adapter.SinkBindingConfig{
		K_SINK:         config.K_SINK,
		K_CE_OVERRIDES: config.K_CE_OVERRIDES,
	}

	ctsAdapter, err := adapter.NewAdapter(client, cqc, sbc)
	if err != nil {
		slog.Error(fmt.Sprintf("creating an cloud trace adapter failed: %s", err))
		os.Exit(exitCodeOpenTelekomCloudClientError)
	}

	events, err := ctsAdapter.GetEvents()
	if err != nil {
		slog.Error(fmt.Sprintf("querying cloud trace service failed: %s", err))
		os.Exit(exitCodeOpenTelekomCloudClientError)
	}

	if config.PullAndPush {
		err := ctsAdapter.SendEvents(events)
		if err != nil {
			slog.Error(fmt.Sprintf("delivering cloud events failed: %s", err))
			os.Exit(exitCodeDeliveringCloudEventsError)
		}
	} else {
		spew.Dump(events)
	}
}
