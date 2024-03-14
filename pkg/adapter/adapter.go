package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/akyriako/opentelekomcloud/auth"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/hashicorp/go-multierror"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"net/url"
	"strings"
	"time"
)

// SinkBindingConfig The SinkBinding object supports decoupling event production from delivery addressing.
// You can use sink binding to direct a subject to a sink. A subject is a Kubernetes resource that embeds a PodSpec
// template and produces events. A sink is an addressable Kubernetes object that can receive events.
// The SinkBinding object injects environment variables into the PodTemplateSpec of the sink. Because of this, the
// application code does not need to interact directly with the Kubernetes API to locate the event destination.
type SinkBindingConfig struct {
	// The URL of the resolved sink (K_SINK)
	SinkUrl string

	// A JSON object that specifies overrides to the outbound event (K_CE_OVERRIDES)
	CeOverrides string
}

type Adapter struct {
	ctsQuerier
	ceClient    cloudevents.Client
	sinkUrl     *url.URL
	ceOverrides *duckv1.CloudEventOverrides
}

func NewAdapter(c *auth.OpenTelekomCloudClient, cqc CtsQuerierConfig, sbc SinkBindingConfig) (*Adapter, error) {
	qry, err := newCtsQuerier(cqc, c)
	if err != nil {
		return nil, err
	}

	sinkUrl, err := url.ParseRequestURI(sbc.SinkUrl)
	if err != nil {
		return nil, err
	}

	var ceOverrides *duckv1.CloudEventOverrides
	if len(sbc.CeOverrides) > 0 {
		overrides := duckv1.CloudEventOverrides{}
		err := json.Unmarshal([]byte(sbc.CeOverrides), &overrides)
		if err != nil {
			return nil, fmt.Errorf("parsing cloud event overrides failed: %w", err)
		}
		ceOverrides = &overrides
	}

	ceProtocol, err := cloudevents.NewHTTP(cloudevents.WithTarget(sinkUrl.String()))
	if err != nil {
		return nil, fmt.Errorf("failed to create http protocol: %w", err)
	}

	ceClient, err := cloudevents.NewClient(ceProtocol, cloudevents.WithUUIDs(), cloudevents.WithTimeNow())
	if err != nil {
		return nil, fmt.Errorf("failed to create http client: %w", err)
	}

	adapter := Adapter{*qry, ceClient, sinkUrl, ceOverrides}
	return &adapter, nil
}

func (a *Adapter) GetEvents() ([]cloudevents.Event, error) {
	ltr, err := a.getTraces()
	if err != nil {
		return nil, err
	}

	if ltr.MetaData.Count <= 0 {
		return nil, err
	}

	events := make([]cloudevents.Event, 0, ltr.MetaData.Count)

	for _, trace := range ltr.Traces {
		event := cloudevents.NewEvent()
		event.SetID(trace.TraceId)

		event.SetSource(a.ctsServiceClient.Endpoint)

		evtType := strings.ToLower(fmt.Sprintf(
			"%s.%s.%s.%s",
			trace.ServiceType,
			trace.TraceType,
			trace.ResourceType,
			trace.TraceName,
		))
		evtType = strings.TrimRight(evtType, ".")
		event.SetType(evtType)

		subject := trace.ResourceId
		if strings.TrimSpace(trace.ResourceName) != "" {
			subject = trace.ResourceName
		}
		event.SetSubject(subject)

		event.SetTime(time.UnixMilli(trace.Time))

		err := event.SetData(cloudevents.ApplicationJSON, trace)
		if err != nil {
			return nil, err
		}

		event.SetExtension("status", trace.TraceStatus)
		event.SetExtension("code", trace.Code)
		event.SetExtension("resourceid", trace.ResourceId)
		event.SetExtension("region", a.ctsServiceClient.RegionID)
		event.SetExtension("domain", a.ctsServiceClient.DomainID)
		event.SetExtension("tenant", a.ctsServiceClient.ProjectID)

		if a.ceOverrides != nil && a.ceOverrides.Extensions != nil {
			for n, v := range a.ceOverrides.Extensions {
				event.SetExtension(n, v)
			}
		}

		events = append(events, event)
	}

	return events, nil
}

func (a *Adapter) SendEvents(events []cloudevents.Event) (int, error) {
	var result *multierror.Error
	sent := len(events)

	if events != nil && len(events) > 0 {
		for _, event := range events {
			if res := a.ceClient.Send(context.Background(), event); !cloudevents.IsACK(res) {
				err := fmt.Errorf("sending event %s failed: %w", event.ID(), res)
				result = multierror.Append(result, err)

				sent -= 1
			}
		}
	}

	return sent, result.ErrorOrNil()
}
