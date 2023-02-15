package connector

import (
	"context"
	"fmt"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"github.com/dstotijn/go-notion"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
)

var (
	resourceTypeUser = &v2.ResourceType{
		Id:          "user",
		DisplayName: "User",
		Traits: []v2.ResourceType_Trait{
			v2.ResourceType_TRAIT_USER,
		},
	}
)

type Notion struct {
	client *notion.Client
	apiKey string
}

func (nt *Notion) ResourceSyncers(ctx context.Context) []connectorbuilder.ResourceSyncer {
	return []connectorbuilder.ResourceSyncer{
		userBuilder(nt.client),
	}
}

// Metadata returns metadata about the connector.
func (nt *Notion) Metadata(ctx context.Context) (*v2.ConnectorMetadata, error) {
	return &v2.ConnectorMetadata{
		DisplayName: "Notion",
	}, nil
}

// Validate hits the Notion API to validate that the API key passed works.
func (nt *Notion) Validate(ctx context.Context) (annotations.Annotations, error) {
	_, err := nt.client.FindUserByID(ctx, "me")
	if err != nil {
		return nil, fmt.Errorf("notion-connector: failed to authenticate. Error: %w", err)
	}

	return nil, nil
}

// New returns the Notion connector.
func New(ctx context.Context, apiKey string) (*Notion, error) {
	httpClient, err := uhttp.NewClient(ctx, uhttp.WithLogger(true, ctxzap.Extract(ctx)))
	if err != nil {
		return nil, err
	}

	return &Notion{
		client: notion.NewClient(apiKey, notion.WithHTTPClient(httpClient)),
		apiKey: apiKey,
	}, nil
}
