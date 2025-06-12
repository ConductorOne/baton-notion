package connector

import (
	"context"
	"fmt"

	notionScim "github.com/conductorone/baton-notion/pkg/notion"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"github.com/dstotijn/go-notion"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
)

type Connector struct {
	client     *notion.Client
	scimClient *notionScim.ScimClient
}

func (d *Connector) ResourceSyncers(_ context.Context) []connectorbuilder.ResourceSyncer {
	if d.scimClient != nil {
		return []connectorbuilder.ResourceSyncer{
			newUserBuilder(d.client),
			newGroupBuilder(d.client, d.scimClient),
		}
	}

	return []connectorbuilder.ResourceSyncer{
		newUserBuilder(d.client),
	}
}

// Metadata returns metadata about the connector.
func (d *Connector) Metadata(_ context.Context) (*v2.ConnectorMetadata, error) {
	return &v2.ConnectorMetadata{
		DisplayName: "Notion",
		Description: "Connector syncing users and groups from Notion",
	}, nil
}

// Validate hits the Notion API to validate that the API key passed works.
func (d *Connector) Validate(ctx context.Context) (annotations.Annotations, error) {
	_, err := d.client.FindUserByID(ctx, "me")
	if err != nil {
		return nil, fmt.Errorf("notion-connector: failed to authenticate. Error: %w", err)
	}

	return nil, nil
}

// New returns the Notion connector.
func New(ctx context.Context, apiKey string, scimToken string) (*Connector, error) {
	var scimClient *notionScim.ScimClient
	httpClient, err := uhttp.NewClient(ctx, uhttp.WithLogger(true, ctxzap.Extract(ctx)))
	if err != nil {
		return nil, err
	}

	if scimToken != "" {
		scimClient = notionScim.NewScimClient(scimToken, httpClient)
	}

	return &Connector{
		client:     notion.NewClient(apiKey, notion.WithHTTPClient(httpClient)),
		scimClient: scimClient,
	}, nil
}
