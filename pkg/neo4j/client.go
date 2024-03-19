package neo4j

import (
	"context"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

const (
	cypher string = `MERGE (region:REGION {name: $region})
MERGE (tenant:TENANT {tenantId: $tenantId, domainId: $domainId})-[:LOCATED_AT]->(region) 
MERGE (resource:RESOURCE {id: $resourceId})-[:MEMBER_OF]->(tenant)
MERGE (action:ACTION {id: $actionId, timestamp: $timestamp, source: $source, type: $type, status: $status})-[:APPLIED_ON]->(resource)
MERGE (subject:SUBJECT {id: $status})<-[:PERFORMED_BY]-(action)
RETURN action,resource,subject`
)

type Client struct {
	neo4j.DriverWithContext
}

type ClientConfig struct {
	Uri      string `env:"NEO4J_URI" envDefault:"neo4j://localhost:7687"`
	User     string `env:"NEO4J_USER" envDefault:"neo4j"`
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

func (c *Client) WriteEventGraph(ctx context.Context, event cloudevents.Event) error {
	session := c.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(transaction neo4j.ManagedTransaction) (any, error) {
		result, err := transaction.Run(ctx,
			cypher,
			map[string]any{
				"region":     event.Extensions()["region"],
				"tenantId":   event.Extensions()["tenant"],
				"domainId":   event.Extensions()["domain"],
				"resourceId": event.Extensions()["resourceid"],
				"actionId":   event.ID(),
				"timestamp":  event.Time(),
				"source":     event.Source(),
				"type":       event.Type(),
				"status":     event.Extensions()["status"],
			})
		if err != nil {
			return nil, err
		}

		if result.Next(ctx) {
			return result.Record().Values[0], nil
		}

		return nil, result.Err()
	})
	if err != nil {
		return err
	}

	return nil
}
