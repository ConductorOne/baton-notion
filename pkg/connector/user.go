package connector

import (
	"context"
	"fmt"

	"github.com/conductorone/baton-notion/pkg/client"
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type userBuilder struct {
	client *client.NotionClient
}

var _ connectorbuilder.AccountManager = &userBuilder{}

func (b *userBuilder) ResourceType(_ context.Context) *v2.ResourceType {
	return userResourceType
}

func (b *userBuilder) List(ctx context.Context, _ *v2.ResourceId, token *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	var userResources []*v2.Resource

	bag, pageToken, err := getToken(token, groupResourceType)
	if err != nil {
		return nil, "", nil, err
	}

	users, nextPageToken, err := b.client.GetUsers(
		ctx,
		client.PaginationOptions{
			StartIndex: pageToken,
			PerPage:    100,
		},
	)
	if err != nil {
		return nil, "", nil, fmt.Errorf("baton-notion: failed to list groups: %w", err)
	}

	for _, user := range users {
		newUserResource, err := parseIntoUserResource(user)
		if err != nil {
			return nil, "", nil, err
		}
		userResources = append(userResources, newUserResource)
	}

	err = bag.Next(nextPageToken)
	if err != nil {
		return nil, "", nil, err
	}

	nextPageToken, err = bag.Marshal()
	if err != nil {
		return nil, "", nil, err
	}

	return userResources, nextPageToken, nil, nil
}

func (b *userBuilder) Entitlements(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func (b *userBuilder) Grants(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
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
	profile := map[string]interface{}{
		"first_name": user.Name.GivenName,
		"last_name":  user.Name.FamilyName,
		"email":      user.UserName,
	}

	if user.Active {
		userStatus = v2.UserTrait_Status_STATUS_ENABLED
	}

	userTraitOptions := []rs.UserTraitOption{
		rs.WithStatus(userStatus),
		rs.WithUserProfile(profile),
		rs.WithUserLogin(user.UserName),
		rs.WithEmail(user.UserName, true),
	}

	// Since the name is different
	ret, err := rs.NewUserResource(
		user.Name.Formatted,
		userResourceType,
		user.ID,
		userTraitOptions,
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
