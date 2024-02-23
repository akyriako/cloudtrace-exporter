package provider

import (
	"errors"
	"fmt"
	golangsdk "github.com/opentelekomcloud/gophertelekomcloud"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack"
	"log/slog"
	"net/http"
)

type AuthOptionsProviderConfig struct {
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
	*golangsdk.ProviderClient
	Config AuthOptionsProviderConfig
}

func NewOpenTelekomCloudClient(config *CloudConfig) (*OpenTelekomCloudClient, error) {
	auth := config.Auth
	authOptionsProviderConfig := AuthOptionsProviderConfig{
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

	client, err := buildOpenTelekomCloudClient(authOptionsProviderConfig)
	if err != nil {
		slog.Error(fmt.Sprintf("acquiring an openstack client failed: %s", err.Error()))
		return nil, err
	}

	return client, err
}

func buildOpenTelekomCloudClient(c AuthOptionsProviderConfig) (*OpenTelekomCloudClient, error) {
	var authOptionsProvider golangsdk.AuthOptionsProvider

	if c.AccessKey != "" && c.SecretKey != "" {
		authOptionsProvider = golangsdk.AKSKAuthOptions{
			DomainID:         c.DomainID,
			Domain:           c.DomainName,
			ProjectId:        c.TenantID,
			ProjectName:      c.TenantName,
			IdentityEndpoint: c.IdentityEndpoint,
			AccessKey:        c.AccessKey,
			SecretKey:        c.SecretKey,
			Region:           c.Region,
		}
	} else if c.Password != "" && (c.Username != "" || c.UserID != "") {
		authOptionsProvider = golangsdk.AuthOptions{
			DomainID:         c.DomainID,
			DomainName:       c.DomainName,
			TenantID:         c.TenantID,
			TenantName:       c.TenantName,
			IdentityEndpoint: c.IdentityEndpoint,
			Password:         c.Password,
			Username:         c.Username,
			UserID:           c.UserID,
		}
	} else {
		return nil, errors.New("a config token or an ak/sk pair or credentials required")
	}

	openstackClient, err := buildOpenStackClient(authOptionsProvider)
	if err != nil {
		return nil, err
	}

	client := &OpenTelekomCloudClient{
		ProviderClient: openstackClient,
		Config:         c,
	}

	return client, err
}

func buildOpenStackClient(aop golangsdk.AuthOptionsProvider) (*golangsdk.ProviderClient, error) {
	client, err := openstack.NewClient(aop.GetIdentityEndpoint())
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

	err = openstack.Authenticate(client, aop)
	if err != nil {
		return nil, err
	}

	return client, nil
}
