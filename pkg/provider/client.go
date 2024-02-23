package provider

import (
	"errors"
	"fmt"
	golangsdk "github.com/opentelekomcloud/gophertelekomcloud"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack"
	"log/slog"
	"net/http"
)

type ClientConfig struct {
	AccessKey        string
	SecretKey        string
	DomainID         string
	DomainName       string
	EndpointType     string
	IdentityEndpoint string
	Insecure         bool
	Password         string
	Region           string
	TenantID         string
	TenantName       string
	Token            string
	Username         string
	UserID           string
}

type OpenTelekomCloudClient struct {
	OtcClient *golangsdk.ProviderClient
	Config    ClientConfig
}

func NewOpenTelekomCloudClient(config *CloudConfig) (*OpenTelekomCloudClient, error) {
	auth := config.Auth
	clientConfig := ClientConfig{
		IdentityEndpoint: auth.AuthURL,
		TenantName:       auth.ProjectName,
		AccessKey:        auth.AccessKey,
		SecretKey:        auth.SecretKey,
		DomainName:       auth.DomainName,
		Username:         auth.UserName,
		Region:           auth.Region,
		Password:         auth.Password,
		Insecure:         true,
	}

	client, err := buildClient(&clientConfig)
	if err != nil {
		slog.Error(fmt.Sprintf("acquiring an openstack client failed: %s", err.Error()))
		return nil, err
	}

	config.Auth.ProjectID = clientConfig.TenantID
	return client, err
}

func buildClient(c *ClientConfig) (*OpenTelekomCloudClient, error) {
	if c.AccessKey != "" && c.SecretKey != "" {
		return buildClientByAKSK(c)
	} else if c.Password != "" && (c.Username != "" || c.UserID != "") {
		return buildClientByPassword(c)
	}

	return nil, errors.New("a config token or an ak/sk pair or username/password credentials required")
}

func buildClientByPassword(c *ClientConfig) (*OpenTelekomCloudClient, error) {
	var pao, dao golangsdk.AuthOptions

	pao = golangsdk.AuthOptions{
		DomainID:   c.DomainID,
		DomainName: c.DomainName,
		TenantID:   c.TenantID,
		TenantName: c.TenantName,
	}

	dao = golangsdk.AuthOptions{
		DomainID:   c.DomainID,
		DomainName: c.DomainName,
	}

	for _, ao := range []*golangsdk.AuthOptions{&pao, &dao} {
		ao.IdentityEndpoint = c.IdentityEndpoint
		ao.Password = c.Password
		ao.Username = c.Username
		ao.UserID = c.UserID
	}

	return newOpenTelekomCloudClient(c, pao, dao)
}

func buildClientByAKSK(c *ClientConfig) (*OpenTelekomCloudClient, error) {
	var pao, dao golangsdk.AKSKAuthOptions

	pao = golangsdk.AKSKAuthOptions{
		ProjectName: c.TenantName,
		ProjectId:   c.TenantID,
	}

	dao = golangsdk.AKSKAuthOptions{
		DomainID: c.DomainID,
		Domain:   c.DomainName,
	}

	for _, ao := range []*golangsdk.AKSKAuthOptions{&pao, &dao} {
		ao.IdentityEndpoint = c.IdentityEndpoint
		ao.AccessKey = c.AccessKey
		ao.SecretKey = c.SecretKey
	}
	return newOpenTelekomCloudClient(c, pao, dao)
}

func newOpenTelekomCloudClient(c *ClientConfig, pao, dao golangsdk.AuthOptionsProvider) (*OpenTelekomCloudClient, error) {
	openstackClient, err := newOpenStackClient(c, pao)
	if err != nil {
		return nil, err
	}

	client := &OpenTelekomCloudClient{
		OtcClient: openstackClient,
		Config:    *c,
	}

	return client, err
}

func newOpenStackClient(c *ClientConfig, ao golangsdk.AuthOptionsProvider) (*golangsdk.ProviderClient, error) {
	client, err := openstack.NewClient(ao.GetIdentityEndpoint())
	if err != nil {
		return nil, err
	}

	client.HTTPClient = http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if client.AKSKAuthOptions.AccessKey != "" {
				golangsdk.ReSign(req, golangsdk.SignOptions{
					AccessKey: client.AKSKAuthOptions.AccessKey,
					SecretKey: client.AKSKAuthOptions.SecretKey,
				})
			}
			return nil
		},
	}

	err = openstack.Authenticate(client, ao)
	if err != nil {
		return nil, err
	}

	//c.TenantID = client.ProjectID
	return client, nil
}
