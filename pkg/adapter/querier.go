package adapter

import (
	"fmt"
	otccommon "github.com/akyriako/opentelekomcloud/common"
	golangsdk "github.com/opentelekomcloud/gophertelekomcloud"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/cts/v2/traces"
	"strconv"
	"strings"
	"time"
)

const (
	defaultTrackerName string = "system"
)

type ctsQuerierConfig struct {
	ProjectId   string
	TrackerName string
}

type ctsQuerier struct {
	config           ctsQuerierConfig
	ctsServiceClient *golangsdk.ServiceClient
}

func (q *ctsQuerier) getTraces(from uint) (*traces.ListTracesResponse, error) {
	fromInMilliSeconds := time.Now().Add(time.Duration(-from)*time.Minute).UTC().UnixNano() / 1e6
	toInMilliSeconds := time.Now().UTC().UnixNano() / 1e6

	ltr, err := traces.List(q.ctsServiceClient, q.config.TrackerName, traces.ListTracesOpts{
		From: strconv.FormatInt(fromInMilliSeconds, 10),
		To:   strconv.FormatInt(toInMilliSeconds, 10),
	})
	if err != nil {
		return nil, err
	}

	return ltr, nil
}

func newCtsQuerier(config ctsQuerierConfig, client *otccommon.OpenTelekomCloudClient) (*ctsQuerier, error) {
	if strings.TrimSpace(config.TrackerName) == "" {
		config.TrackerName = defaultTrackerName
	}

	ctsServiceClient, err := getCtsClient(client)
	if err != nil {
		return nil, err
	}

	querier := &ctsQuerier{
		config:           config,
		ctsServiceClient: ctsServiceClient,
	}

	return querier, nil
}

func getCtsClient(c *otccommon.OpenTelekomCloudClient) (*golangsdk.ServiceClient, error) {
	client, err := openstack.NewCTSV2(c.ProjectClient, golangsdk.EndpointOpts{
		Region: c.ProjectClient.RegionID,
	})
	if err != nil {
		err = fmt.Errorf(fmt.Sprintf(
			"acquiring a cloud trace service client failed, %s",
			strings.ToLower(err.Error()),
		))
		return nil, err
	}

	return client, nil
}
