package connector

import (
	"context"
	"fmt"

	notionScim "github.com/conductorone/baton-notion/pkg/client"
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
			newUserBuilder(d.client, d.scimClient),
			newGroupBuilder(d.client, d.scimClient),
		}
	}

	return []connectorbuilder.ResourceSyncer{
		newUserBuilder(d.client, nil),
	}
}

// Metadata returns metadata about the connector.
func (d *Connector) Metadata(_ context.Context) (*v2.ConnectorMetadata, error) {
	return &v2.ConnectorMetadata{
		DisplayName: "Notion",
		Description: "Connector syncing users and groups from Notion",
		AccountCreationSchema: &v2.ConnectorAccountCreationSchema{
			FieldMap: map[string]*v2.ConnectorAccountCreationSchema_Field{
				"first_name": {
					DisplayName: "First Name",
					Required:    true,
					Description: "First name of the person who will own the user.",
					Field: &v2.ConnectorAccountCreationSchema_Field_StringField{
						StringField: &v2.ConnectorAccountCreationSchema_StringField{},
					},
					Placeholder: "John",
					Order:       1,
				},
				"last_name": {
					DisplayName: "Last Name",
					Required:    true,
					Description: "Last name of the person who will own the user.",
					Field: &v2.ConnectorAccountCreationSchema_Field_StringField{
						StringField: &v2.ConnectorAccountCreationSchema_StringField{},
					},
					Placeholder: "Doe",
					Order:       2,
				},
				"email": {
					DisplayName: "Email",
					Required:    true,
					Description: "This email will be used as the login for the user.",
					Field: &v2.ConnectorAccountCreationSchema_Field_StringField{
						StringField: &v2.ConnectorAccountCreationSchema_StringField{},
					},
					Placeholder: "john.doe@example.com",
					Order:       3,
				},
			},
		},
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
		scimClient, err = notionScim.New(ctx, scimToken)
		if err != nil {
			return nil, err
		}
	}

	return &Connector{
		client:     notion.NewClient(apiKey, notion.WithHTTPClient(httpClient)),
		scimClient: scimClient,
	}, nil
}
