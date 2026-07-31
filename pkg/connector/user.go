package connector

import (
	"context"
	"fmt"

	"github.com/conductorone/baton-notion/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	sdkGrant "github.com/conductorone/baton-sdk/pkg/types/grant"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// profileKeyWorkspaceRole stashes the Notion workspace role on the user
// resource during List so Grants can emit role grants without an extra
// GetUser call per user.
const profileKeyWorkspaceRole = "workspace_role"

type userBuilder struct {
	client *client.NotionClient
}

var _ connectorbuilder.AccountManagerV2 = &userBuilder{}

func (b *userBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return userResourceType
}

func (b *userBuilder) List(ctx context.Context, _ *v2.ResourceId, attrs rs.SyncOpAttrs) ([]*v2.Resource, *rs.SyncOpResults, error) {
	var userResources []*v2.Resource

	bag, pageToken, err := getToken(attrs.PageToken.Token, userResourceType)
	if err != nil {
		return nil, nil, err
	}

	users, nextPageToken, err := b.client.GetUsers(
		ctx,
		client.PaginationOptions{
			StartIndex: pageToken,
			PerPage:    100,
		},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("baton-notion: failed to list users: %w", err)
	}

	for _, user := range users {
		newUserResource, err := parseIntoUserResource(user)
		if err != nil {
			return nil, nil, err
		}
		userResources = append(userResources, newUserResource)
	}

	err = bag.Next(nextPageToken)
	if err != nil {
		return nil, nil, err
	}

	nextPageToken, err = bag.Marshal()
	if err != nil {
		return nil, nil, err
	}

	return userResources, &rs.SyncOpResults{NextPageToken: nextPageToken}, nil
}

func (b *userBuilder) Entitlements(_ context.Context, _ *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	return nil, nil, nil
}

// Grants emits one role grant per user, read from the stashed profile field.
// A role value that is not one of the four documented Notion roles is logged
// at Warn and the grant is skipped — this surfaces silent drift (e.g. Notion
// introducing a new tier) instead of dropping users invisibly.
func (b *userBuilder) Grants(ctx context.Context, resource *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Grant, *rs.SyncOpResults, error) {
	role := workspaceRoleFromResource(resource)
	if role == "" {
		return nil, nil, nil
	}
	if !isSupportedRole(role) {
		ctxzap.Extract(ctx).Warn(
			"baton-notion: skipping role grant for unknown role value",
			zap.String("user_id", resource.Id.Resource),
			zap.String("unknown_role", role),
		)
		return nil, nil, nil
	}

	roleRes := &v2.Resource{Id: &v2.ResourceId{
		ResourceType: roleResourceType.Id,
		Resource:     role,
	}}
	return []*v2.Grant{sdkGrant.NewGrant(roleRes, assignedEntitlement, resource.Id)}, nil, nil
}

func workspaceRoleFromResource(resource *v2.Resource) string {
	profile := resource.GetProfile()
	if profile == nil {
		return ""
	}
	fields := profile.GetFields()
	if fields == nil {
		return ""
	}
	roleVal, ok := fields[profileKeyWorkspaceRole]
	if !ok {
		return ""
	}
	return roleVal.GetStringValue()
}

func (b *userBuilder) CreateAccountCapabilityDetails(_ context.Context) (*v2.CredentialDetailsAccountProvisioning, annotations.Annotations, error) {
	return &v2.CredentialDetailsAccountProvisioning{
		SupportedCredentialOptions: []v2.CapabilityDetailCredentialOption{
			v2.CapabilityDetailCredentialOption_CAPABILITY_DETAIL_CREDENTIAL_OPTION_NO_PASSWORD,
		},
		PreferredCredentialOption: v2.CapabilityDetailCredentialOption_CAPABILITY_DETAIL_CREDENTIAL_OPTION_NO_PASSWORD,
	}, nil, nil
}

func (b *userBuilder) CreateAccount(
	ctx context.Context,
	accountInfo *v2.AccountInfo,
	_ *v2.LocalCredentialOptions,
) (connectorbuilder.CreateAccountResponse, []*v2.PlaintextData, annotations.Annotations, error) {
	newUserInfo, err := createNewUserData(accountInfo)
	if err != nil {
		return nil, nil, nil, err
	}

	newUser, err := b.client.CreateUser(ctx, newUserInfo)
	if err != nil {
		return nil, nil, nil, err
	}

	userResource, err := parseIntoUserResource(*newUser)
	if err != nil {
		return nil, nil, nil, err
	}

	caResponse := &v2.CreateAccountResponse_SuccessResult{
		Resource: userResource,
	}

	return caResponse, []*v2.PlaintextData{}, nil, nil
}

func createNewUserData(accountInfo *v2.AccountInfo) (*client.User, error) {
	pMap := accountInfo.Profile.AsMap()

	firstName, ok := pMap["first_name"].(string)
	if !ok || firstName == "" {
		return nil, fmt.Errorf("first name is required")
	}

	lastName, ok := pMap["last_name"].(string)
	if !ok || lastName == "" {
		return nil, fmt.Errorf("last name is required")
	}

	email, ok := pMap["email"].(string)
	if !ok || email == "" {
		return nil, fmt.Errorf("email is required")
	}

	newUser := &client.User{
		Schemas:  []string{client.DefaultUserSchema},
		UserName: email,
		Name: struct {
			GivenName  string `json:"givenName"`
			FamilyName string `json:"familyName"`
			Formatted  string `json:"formatted"`
		}{
			GivenName:  firstName,
			FamilyName: lastName,
			Formatted:  fmt.Sprintf("%s %s", firstName, lastName),
		},
		Active: true,
		Emails: []struct {
			Primary bool   `json:"primary"`
			Value   string `json:"value"`
			Type    string `json:"type"`
		}{
			{
				Primary: true,
				Value:   email,
				Type:    "Primary",
			},
		},
	}

	return newUser, nil
}

func (b *userBuilder) Delete(ctx context.Context, principal *v2.ResourceId) (annotations.Annotations, error) {
	userID := principal.Resource

	err := b.client.DeleteUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	deletedUser, err := b.client.GetUser(ctx, userID)
	if err == nil || status.Code(err) != codes.NotFound || deletedUser != nil {
		return nil, fmt.Errorf("error deleting user. User %s still exists", userID)
	}

	return nil, nil
}

func parseIntoUserResource(user client.User) (*v2.Resource, error) {
	userStatus := v2.UserTrait_Status_STATUS_DISABLED
	profile := map[string]any{
		"first_name": user.Name.GivenName,
		"last_name":  user.Name.FamilyName,
		"email":      user.UserName,
	}

	if role := user.Role(); role != "" {
		profile[profileKeyWorkspaceRole] = role
	}

	if user.Active {
		userStatus = v2.UserTrait_Status_STATUS_ENABLED
	}

	userTraitOptions := []rs.UserTraitOption{
		rs.WithUserLogin(user.UserName),
		rs.WithEmail(user.UserName, true),
	}

	// Since the name is different
	ret, err := rs.NewUserResource(
		user.Name.Formatted,
		userResourceType,
		user.ID,
		userTraitOptions,
		rs.WithResourceProfile(profile),
		rs.WithResourceStatus(v2.Status_ResourceStatus(userStatus), ""),
	)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func newUserBuilder(scimClient *client.NotionClient) *userBuilder {
	return &userBuilder{
		client: scimClient,
	}
}
