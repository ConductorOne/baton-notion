package connector

import (
	"context"
	"fmt"

	"github.com/conductorone/baton-notion/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	ent "github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
)

const memberEntitlement = "member"

type groupBuilder struct {
	client *client.NotionClient
}

func (b *groupBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return groupResourceType
}

// Create a new connector resource for a Notion group.
func groupResource(group *client.Group) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"group_id":   group.ID,
		"group_name": group.DisplayName,
	}

	groupTraitOptions := []rs.GroupTraitOption{rs.WithGroupProfile(profile)}

	ret, err := rs.NewGroupResource(
		group.DisplayName,
		groupResourceType,
		group.ID,
		groupTraitOptions,
	)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (b *groupBuilder) List(ctx context.Context, _ *v2.ResourceId, attrs rs.SyncOpAttrs) ([]*v2.Resource, *rs.SyncOpResults, error) {
	bag, pageToken, err := getToken(attrs.PageToken.Token, groupResourceType)
	if err != nil {
		return nil, nil, err
	}

	groups, nextPageToken, err := b.client.GetGroups(
		ctx,
		client.PaginationOptions{
			StartIndex: pageToken,
			PerPage:    100,
		},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("baton-notion: failed to list groups: %w", err)
	}

	var rv []*v2.Resource
	for _, group := range groups {
		groupCopy := group
		ur, err := groupResource(&groupCopy)
		if err != nil {
			return nil, nil, err
		}
		rv = append(rv, ur)
	}

	err = bag.Next(nextPageToken)
	if err != nil {
		return nil, nil, err
	}

	nextPageToken, err = bag.Marshal()
	if err != nil {
		return nil, nil, err
	}

	return rv, &rs.SyncOpResults{NextPageToken: nextPageToken}, nil
}

func (b *groupBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	var rv []*v2.Entitlement

	assigmentOptions := []ent.EntitlementOption{
		ent.WithGrantableTo(userResourceType),
		ent.WithDescription(fmt.Sprintf("Member of %s Group in Notion", resource.DisplayName)),
		ent.WithDisplayName(fmt.Sprintf("%s Group %s", resource.DisplayName, memberEntitlement)),
	}

	en := ent.NewAssignmentEntitlement(resource, memberEntitlement, assigmentOptions...)
	rv = append(rv, en)

	return rv, nil, nil
}

func (b *groupBuilder) Grants(ctx context.Context, resource *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Grant, *rs.SyncOpResults, error) {
	var groupGrants []*v2.Grant

	group, err := b.client.GetGroup(ctx, resource.Id.Resource)
	if err != nil {
		return nil, nil, err
	}

	for _, member := range group.Members {
		userId := &v2.ResourceId{
			ResourceType: userResourceType.Id,
			Resource:     member.Value,
		}

		groupGrants = append(groupGrants, grant.NewGrant(resource, memberEntitlement, userId))
	}

	return groupGrants, nil, nil
}

func newGroupBuilder(scimClient *client.NotionClient) *groupBuilder {
	return &groupBuilder{
		client: scimClient,
	}
}
