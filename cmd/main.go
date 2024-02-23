package main

import (
	"fmt"
	"github.com/akyriako/cloudtrace-exporter/pkg/adapter"
	"github.com/akyriako/cloudtrace-exporter/pkg/provider"
	"github.com/caarlos0/env/v10"
	"log/slog"
	"os"
	"strings"
)

type environment struct {
	CloudsConfig string `env:"OS_CLOUD" envDefault:"./clouds.yaml"`
	Debug        bool   `env:"OS_DEBUG" envDefault:"true"`
	Tracker      string `env:"CTS_TRACKER"`
	From         uint   `env:"CTS_FROM" envDefault:"5"`
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
	pConfig, err := provider.GetConfigFromFile(config.CloudsConfig)
	if err != nil {
		wd, wderr := os.Getwd()
		if wderr != nil {
			slog.Error(fmt.Sprintf("parsing cloud config failed: %s", wderr.Error()))
			os.Exit(exitCodeConfigurationError)
		}

		slog.Error(fmt.Sprintf("parsing cloud config at %s%s failed: %s", wd, strings.Trim(config.CloudsConfig, "."), err.Error()))
		os.Exit(exitCodeConfigurationError)
	}

	pClient, err := provider.NewOpenTelekomCloudClient(pConfig)
	if err != nil {
		slog.Error(fmt.Sprintf("acquiring an opentelekomcloud client failed: %s", err))
		os.Exit(exitCodeOpenTelekomCloudClientError)
	}

	ctsAdapter, err := adapter.NewAdapter(pClient, config.Tracker)
	if err != nil {
		slog.Error(fmt.Sprintf("creating an cloud trace adapter failed: %s", err))
		os.Exit(exitCodeOpenTelekomCloudClientError)
	}

	events, err := ctsAdapter.GetEvents(config.From)
	if err != nil {
		slog.Error(fmt.Sprintf("querying cloud trace service failed: %s", err))
		os.Exit(exitCodeOpenTelekomCloudClientError)
	}

	fmt.Println(len(events))
}
