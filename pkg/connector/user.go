package connector

import (
	"context"
	"fmt"
	"strings"

	notionScim "github.com/conductorone/baton-notion/pkg/notion"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/dstotijn/go-notion"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type userBuilder struct {
	scimClient *notionScim.ScimClient
	client     *notion.Client
}

func (b *userBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return userResourceType
}

// Create a new connector resource for a Notion user.
func userResource(user notion.User) (*v2.Resource, error) {
	names := strings.SplitN(user.Name, " ", 2)
	var firstName, lastName, email string

	switch len(names) {
	case 1:
		firstName = names[0]
	case 2:
		firstName = names[0]
		lastName = names[1]
	}

	if user.Person != nil {
		email = user.Person.Email
	}

	profile := map[string]interface{}{
		"first_name": firstName,
		"last_name":  lastName,
		"login":      email,
		"user_id":    user.ID,
	}

	userTraitOptions := []rs.UserTraitOption{
		rs.WithUserProfile(profile),
		rs.WithEmail(email, true),
		rs.WithStatus(v2.UserTrait_Status_STATUS_ENABLED),
	}

	ret, err := rs.NewUserResource(
		user.Name,
		userResourceType,
		user.ID,
		userTraitOptions,
	)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (b *userBuilder) List(ctx context.Context, _ *v2.ResourceId, token *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	var pageToken string
	bag, err := parsePageToken(token.Token, &v2.ResourceId{ResourceType: userResourceType.Id})
	if err != nil {
		return nil, "", nil, err
	}

	usersResponse, err := b.client.ListUsers(ctx, &notion.PaginationQuery{PageSize: resourcePageSize, StartCursor: bag.PageToken()})
	if err != nil {
		return nil, "", nil, fmt.Errorf("notion-connector: failed to list users: %w", err)
	}

	if usersResponse.HasMore {
		pageToken, err = bag.NextToken(*usersResponse.NextCursor)
		if err != nil {
			return nil, "", nil, err
		}
	}

	var rv []*v2.Resource
	for _, user := range usersResponse.Results {
		ur, err := userResource(user)
		if err != nil {
			return nil, "", nil, err
		}
		rv = append(rv, ur)
	}

	return rv, pageToken, nil, nil
}

func (b *userBuilder) Entitlements(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func (b *userBuilder) Grants(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func (b *userBuilder) Delete(ctx context.Context, principal *v2.ResourceId) (annotations.Annotations, error) {
	if b.scimClient == nil {
		return nil, fmt.Errorf("baton-notion: scim client not initialized")
	}

	userID := principal.Resource

	err := b.scimClient.DeleteUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	deletedUser, err := b.scimClient.GetUser(ctx, userID)
	if err == nil || status.Code(err) != codes.NotFound || deletedUser != nil {
		return nil, fmt.Errorf("error deleting user. User %s still exists", userID)
	}

	return nil, nil
}

func newUserBuilder(client *notion.Client, scimClient *notionScim.ScimClient) *userBuilder {
	return &userBuilder{
		scimClient: scimClient,
		client:     client,
	}
}
