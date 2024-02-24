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
	Cloud   string `env:"OS_CLOUD"`
	Debug   bool   `env:"OS_DEBUG" envDefault:"true"`
	Tracker string `env:"CTS_TRACKER"`
	From    uint   `env:"CTS_FROM" envDefault:"5"`
}

var (
	config environment
	logger *slog.Logger
)

const (
	exitCodeConfigurationError          int = 1
	exitCodeOpenTelekomCloudClientError int = 2
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

	ctsAdapter, err := adapter.NewAdapter(client, config.Tracker)
	if err != nil {
		slog.Error(fmt.Sprintf("creating an cloud trace adapter failed: %s", err))
		os.Exit(exitCodeOpenTelekomCloudClientError)
	}

	events, err := ctsAdapter.GetEvents(config.From)
	if err != nil {
		slog.Error(fmt.Sprintf("querying cloud trace service failed: %s", err))
		os.Exit(exitCodeOpenTelekomCloudClientError)
	}

	spew.Dump(events)
}
