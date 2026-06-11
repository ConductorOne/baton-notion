package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
)

const (
	defaultBaseURL    = "https://www.notion.so/scim/v2"
	DefaultUserSchema = "urn:ietf:params:scim:schemas:core:2.0:User"

	// SCIM resource path segments, joined onto baseURL via url.JoinPath.
	usersPath  = "Users"
	groupsPath = "Groups"
)

type NotionClient struct {
	client    *uhttp.BaseHttpClient
	scimToken string
	baseURL   string
}

// GetUsers paginates `GET /scim/v2/Users` using Notion's startIndex/count
// convention (startIndex is 1-indexed, count caps at 100).
//
// Doc: https://www.notion.com/help/provision-users-and-groups-with-scim
// (section "Users" → `GET /Users`).
func (c *NotionClient) GetUsers(ctx context.Context, pageOps PaginationOptions) ([]User, string, error) {
	var nextPage string
	requestURL, err := url.JoinPath(c.baseURL, usersPath)
	if err != nil {
		return nil, "", fmt.Errorf("baton-notion: build users URL: %w", err)
	}

	var res UsersResponse
	_, err = c.doRequest(
		ctx,
		http.MethodGet,
		requestURL,
		&res,
		nil,
		WithPageSize(pageOps.PerPage),
		WithStartIndex(pageOps.StartIndex),
	)
	if err != nil {
		return nil, "", err
	}

	if (int64(pageOps.StartIndex) + res.ItemsPerPage) < res.TotalResults {
		nextPage = strconv.FormatInt(int64(pageOps.StartIndex)+res.ItemsPerPage, 10)
	}

	return res.Resources, nextPage, nil
}

// GetGroups paginates `GET /scim/v2/Groups`. Notion caps an unpaginated
// request at 100 results — we always send startIndex/count for safety.
//
// Doc: https://www.notion.com/help/provision-users-and-groups-with-scim
// (section "Groups" → `GET /Groups`).
func (c *NotionClient) GetGroups(ctx context.Context, pageOps PaginationOptions) ([]Group, string, error) {
	var nextPage string
	requestURL, err := url.JoinPath(c.baseURL, groupsPath)
	if err != nil {
		return nil, "", fmt.Errorf("baton-notion: build groups URL: %w", err)
	}

	var res GroupsResponse
	_, err = c.doRequest(
		ctx,
		http.MethodGet,
		requestURL,
		&res,
		nil,
		WithPageSize(pageOps.PerPage),
		WithStartIndex(pageOps.StartIndex),
	)
	if err != nil {
		return nil, "", err
	}

	if (int64(pageOps.StartIndex) + res.ItemsPerPage) < res.TotalResults {
		nextPage = strconv.FormatInt(int64(pageOps.StartIndex)+res.ItemsPerPage, 10)
	}

	return res.Resources, nextPage, nil
}

// GetGroup fetches `GET /scim/v2/Groups/{id}`. The id format documented by
// Notion is a 32-char UUID with hyphens (00000000-0000-0000-0000-000000000000).
//
// Doc: https://www.notion.com/help/provision-users-and-groups-with-scim
// (section "Groups" → `GET /Groups/<id>`).
func (c *NotionClient) GetGroup(ctx context.Context, groupId string) (Group, error) {
	requestURL, err := url.JoinPath(c.baseURL, groupsPath, groupId)
	if err != nil {
		return Group{}, fmt.Errorf("baton-notion: build group URL for %s: %w", groupId, err)
	}

	var groupResponse Group
	_, err = c.doRequest(ctx, http.MethodGet, requestURL, &groupResponse, nil)
	if err != nil {
		return Group{}, err
	}

	return groupResponse, nil
}

// GetUser fetches `GET /scim/v2/Users/{id}`. The Notion help center notes
// that meta.created and meta.lastModified do not reflect meaningful
// timestamps, which is why this connector does not surface them.
//
// Doc: https://www.notion.com/help/provision-users-and-groups-with-scim
// (section "Users" → `GET /Users/<id>`).
func (c *NotionClient) GetUser(ctx context.Context, userID string) (*User, error) {
	var userData *User
	requestURL, err := url.JoinPath(c.baseURL, usersPath, userID)
	if err != nil {
		return nil, fmt.Errorf("baton-notion: build user URL for %s: %w", userID, err)
	}

	_, err = c.doRequest(ctx, http.MethodGet, requestURL, &userData, nil)
	if err != nil {
		return nil, err
	}

	return userData, nil
}

// CreateUser provisions a workspace member via `POST /scim/v2/Users`. If the
// email already maps to an existing Notion user account, the call adds that
// account to the workspace; otherwise it creates a fresh Notion user.
//
// The Notion help center also notes that the profile-photo property is read
// only on creation, never on updates.
//
// Doc: https://www.notion.com/help/provision-users-and-groups-with-scim
// (section "Users" → `POST /Users`).
func (c *NotionClient) CreateUser(ctx context.Context, user *User) (*User, error) {
	var newUser *User
	requestURL, err := url.JoinPath(c.baseURL, usersPath)
	if err != nil {
		return nil, fmt.Errorf("baton-notion: build create-user URL: %w", err)
	}

	_, err = c.doRequest(ctx, http.MethodPost, requestURL, &newUser, user)
	if err != nil {
		return nil, err
	}

	return newUser, nil
}

// PatchUserRole updates the workspace role of a user via SCIM PATCH on the
// Notion role extension. The body uses the value-as-object form (no `path`)
// because extension paths with `:` separators are inconsistently supported
// across SCIM implementations.
//
// Doc: https://www.notion.com/help/provision-users-and-groups-with-scim
// (section "Users" → `PATCH /Users/<id>`).
func (c *NotionClient) PatchUserRole(ctx context.Context, userID, role string) (*User, error) {
	requestURL, err := url.JoinPath(c.baseURL, usersPath, userID)
	if err != nil {
		return nil, fmt.Errorf("baton-notion: build PATCH url for user %s: %w", userID, err)
	}

	body := SCIMPatchRequest{
		Schemas: []string{SCIMPatchOpSchema},
		Operations: []SCIMPatchOperation{{
			Op: "replace",
			Value: map[string]any{
				NotionUserExtensionSchema: map[string]string{"role": role},
			},
		}},
	}

	var updated *User
	if _, err := c.doRequest(ctx, http.MethodPatch, requestURL, &updated, body); err != nil {
		return nil, err
	}
	return updated, nil
}

// DeleteUser removes a workspace member via `DELETE /scim/v2/Users/{id}`.
// Per the Notion help center, this removes the user from the workspace and
// logs them out of active sessions — it does NOT delete the underlying
// Notion user account (that must be done manually). Additionally, the
// workspace owner that issued the SCIM bot token cannot be removed via the API.
//
// Doc: https://www.notion.com/help/provision-users-and-groups-with-scim
// (section "Users" → `DELETE /Users/<id>`).
func (c *NotionClient) DeleteUser(ctx context.Context, userID string) error {
	requestURL, err := url.JoinPath(c.baseURL, usersPath, userID)
	if err != nil {
		return fmt.Errorf("baton-notion: build delete-user URL for %s: %w", userID, err)
	}

	_, err = c.doRequest(ctx, http.MethodDelete, requestURL, nil, nil)
	if err != nil {
		return err
	}

	return nil
}

func (c *NotionClient) doRequest(
	ctx context.Context,
	method string,
	endpointUrl string,
	res any,
	body any,
	reqOpts ...ReqOpt,
) (http.Header, error) {
	var resp *http.Response

	urlAddress, err := url.Parse(endpointUrl)
	if err != nil {
		return nil, err
	}

	for _, o := range reqOpts {
		o(urlAddress)
	}

	opts := []uhttp.RequestOption{
		uhttp.WithBearerToken(c.scimToken),
		uhttp.WithAcceptJSONHeader(),
	}
	if body != nil {
		opts = append(opts, uhttp.WithContentTypeJSONHeader(), uhttp.WithJSONBody(body))
	}

	req, err := c.client.NewRequest(
		ctx,
		method,
		urlAddress,
		opts...,
	)
	if err != nil {
		return nil, err
	}

	switch method {
	case http.MethodGet, http.MethodPut, http.MethodPost, http.MethodPatch:
		var doOptions []uhttp.DoOption
		if res != nil {
			doOptions = append(doOptions, uhttp.WithResponse(&res))
		}
		resp, err = c.client.Do(req, doOptions...)
		if resp != nil {
			defer resp.Body.Close()
		}

	case http.MethodDelete:
		resp, err = c.client.Do(req)
		if resp != nil {
			defer resp.Body.Close()
		}
	}
	if err != nil {
		return nil, err
	}

	return resp.Header, nil
}

func New(ctx context.Context, scimToken string, baseURL string) (*NotionClient, error) {
	httpClient, err := uhttp.NewClient(ctx, uhttp.WithLogger(true, ctxzap.Extract(ctx)))
	if err != nil {
		return nil, err
	}

	cli, err := uhttp.NewBaseHttpClientWithContext(ctx, httpClient)
	if err != nil {
		return nil, err
	}

	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	notionClient := NotionClient{
		client:    cli,
		scimToken: scimToken,
		baseURL:   baseURL,
	}

	return &notionClient, nil
}
