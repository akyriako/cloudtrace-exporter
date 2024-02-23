package adapter

import (
	"fmt"
	otccommon "github.com/akyriako/opentelekomcloud/common"
	cloudevents "github.com/cloudevents/sdk-go"
	"strings"
	"time"
)

type Adapter struct {
	ctsQuerier
}

func NewAdapter(c *otccommon.OpenTelekomCloudClient, tracker string) (*Adapter, error) {
	ctsQuerierConfig := ctsQuerierConfig{
		ProjectId:   c.ProjectClient.ProjectID,
		TrackerName: tracker,
	}

	qry, err := newCtsQuerier(ctsQuerierConfig, c)
	if err != nil {
		return nil, err
	}

	adapter := Adapter{*qry}
	return &adapter, nil
}

func (a *Adapter) GetEvents(from uint) ([]cloudevents.Event, error) {
	ltr, err := a.getTraces(from)
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
