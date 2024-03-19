package neo4j

import (
	"context"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type Client struct {
	neo4j.DriverWithContext
}

type ClientConfig struct {
	Uri      string `env:"NEO4J_URI"`
	User     string `env:"NEO4J_USER"`
	Password string `env:"NEO4J_PASSWORD"`
}

func NewClient(ctx context.Context, config ClientConfig) (*Client, error) {
	driver, err := neo4j.NewDriverWithContext(
		config.Uri,
		neo4j.BasicAuth(config.User, config.Password, ""))
	if err != nil {
		return nil, err
	}

	err = driver.VerifyConnectivity(ctx)
	if err != nil {
		return nil, err
	}

	client := &Client{driver}
	return client, nil
}
