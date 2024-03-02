package adapter

import (
	"fmt"
	"github.com/akyriako/opentelekomcloud/auth"
	cloudevents "github.com/cloudevents/sdk-go"
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
	SinkBinding SinkBindingConfig
}

func NewAdapter(c *auth.OpenTelekomCloudClient, cqc CtsQuerierConfig, sbc SinkBindingConfig) (*Adapter, error) {
	qry, err := newCtsQuerier(cqc, c)
	if err != nil {
		return nil, err
	}

	adapter := Adapter{*qry, sbc}
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

		event.SetDataContentType(cloudevents.ApplicationJSON)
		err := event.SetData(trace)
		if err != nil {
			return nil, err
		}

		event.SetExtension("code", trace.Code)
		event.SetExtension("resourceid", trace.ResourceId)
		event.SetExtension("region", a.ctsServiceClient.RegionID)
		event.SetExtension("domain", a.ctsServiceClient.DomainID)
		event.SetExtension("tenant", a.ctsServiceClient.ProjectID)

		events = append(events, event)
	}

	return events, nil
}

func (a *Adapter) SendEvents(events []cloudevents.Event) error {

	return nil
}
