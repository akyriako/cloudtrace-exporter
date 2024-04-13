package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/akyriako/opentelekomcloud/auth"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/hashicorp/go-multierror"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/cts/v2/traces"
	duckv1 "knative.dev/pkg/apis/duck/v1"
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
	ceClient cloudevents.Client
	debug    bool

	sinkUrl     *url.URL
	ceOverrides *duckv1.CloudEventOverrides

	deltaLock sync.RWMutex
	delta     time.Duration
}

func NewAdapter(c *auth.OpenTelekomCloudClient, cqc CtsQuerierConfig, sbc SinkBindingConfig, debug bool) (*Adapter, error) {
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

	adapter := Adapter{
		ctsQuerier:  *qry,
		ceClient:    ceClient,
		sinkUrl:     sinkUrl,
		ceOverrides: ceOverrides,
		debug:       debug,
		delta:       0,
	}

	return &adapter, nil
}

func (a *Adapter) GetEvents() ([]cloudevents.Event, error) {
	now := time.Now()
	defer a.trackDelta(now)

	fromTime := now.Add(time.Duration(-a.ctsQuerier.config.From) * time.Minute).Add(-a.delta).UTC()
	toTime := now.UTC()
	fromInMilliSeconds := fromTime.UnixNano() / 1e6
	toInMilliSeconds := toTime.UnixNano() / 1e6

	listTracesOpts := traces.ListTracesOpts{
		From:  strconv.FormatInt(fromInMilliSeconds, 10),
		To:    strconv.FormatInt(toInMilliSeconds, 10),
		Limit: strconv.Itoa(tracesLowerBound),
	}

	events := make([]cloudevents.Event, 0)

	for {
		ltr, err := a.getTraces(listTracesOpts)
		if err != nil {
			return nil, err
		}

		if ltr.MetaData.Count <= 0 {
			return nil, fmt.Errorf("no traces collected")
		}

		for _, trace := range ltr.Traces {
			event, err := a.TraceToCloudEvent(trace)
			if err != nil {
				return nil, err
			}

			events = append(events, *event)
		}

		if ltr.MetaData.Marker == "" {
			break
		}

		listTracesOpts.Next = ltr.MetaData.Marker
	}

	if len(events) > 0 || a.debug {
		slog.Info(fmt.Sprintf("collected %d events", len(events)),
			"project", a.config.ProjectId, "tracker", a.config.TrackerName, "from", fromTime, "to", toTime)
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

func (a *Adapter) GetEventsStream(eventsStream chan<- cloudevents.Event, done chan<- interface{}) {
	now := time.Now()
	defer a.trackDelta(now)

	fromTime := now.Add(time.Duration(-a.ctsQuerier.config.From) * time.Minute).Add(-a.delta).UTC()
	toTime := now.UTC()
	fromInMilliSeconds := fromTime.UnixNano() / 1e6
	toInMilliSeconds := toTime.UnixNano() / 1e6

	listTracesOpts := traces.ListTracesOpts{
		From:  strconv.FormatInt(fromInMilliSeconds, 10),
		To:    strconv.FormatInt(toInMilliSeconds, 10),
		Limit: strconv.Itoa(tracesLowerBound),
	}

	var collected int

	for {
		ltr, err := a.getTraces(listTracesOpts)
		if err != nil {
			slog.Error(fmt.Sprintf("querying cloud trace service failed: %s", err))
		}

		if ltr.MetaData.Count <= 0 {
			break
		}

		collected += ltr.MetaData.Count

		for _, trace := range ltr.Traces {
			event, err := a.TraceToCloudEvent(trace)
			if err != nil {
				collected -= 1
				slog.Error(fmt.Sprintf("transforming trace to cloudevent failed: %s", err))
			}

			eventsStream <- *event
		}

		if ltr.MetaData.Marker == "" {
			break
		}

		listTracesOpts.Next = ltr.MetaData.Marker
	}

	if collected > 0 || a.debug {
		slog.Info(fmt.Sprintf("collected %d events", collected),
			"project", a.config.ProjectId, "tracker", a.config.TrackerName, "from", fromTime, "to", toTime)
	}

	done <- struct{}{}
}

func (a *Adapter) SendEventsStream(eventStream <-chan cloudevents.Event) {
	for event := range eventStream {
		if res := a.ceClient.Send(context.Background(), event); !cloudevents.IsACK(res) {
			slog.Error(fmt.Sprintf("sending event %s failed: %s", event.ID(), res))
		}
	}
}

func (a *Adapter) TraceToCloudEvent(trace traces.Traces) (*cloudevents.Event, error) {
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
		extensions := a.ceOverrides.Extensions
		for n, v := range extensions {
			event.SetExtension(n, v)
		}
	}

	return &event, nil
}

func (a *Adapter) trackDelta(start time.Time) {
	defer a.deltaLock.Unlock()

	a.deltaLock.Lock()
	a.delta = time.Since(start)
}
