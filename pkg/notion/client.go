package notion

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

const baseUrl = "https://www.notion.so/scim/v2"

const (
	// 1-based not zero based.
	defaultStartIndex = 1
	defaultCount      = 100

	defaultUserSchema = "urn:ietf:params:scim:schemas:core:2.0:User"
)

type ScimClient struct {
	httpClient *http.Client
	scimToken  string
}

func NewScimClient(scimToken string, httpClient *http.Client) *ScimClient {
	return &ScimClient{
		httpClient: httpClient,
		scimToken:  scimToken,
	}
}

type GroupsResponse struct {
	TotalResults int64   `json:"totalResults"`
	Resources    []Group `json:"Resources"`
	StartIndex   int64   `json:"startIndex"`
	ItemsPerPage int64   `json:"itemsPerPage"`
}

// GetGroups returns all Notion groups.
func (c *ScimClient) GetGroups(ctx context.Context, count int, startIndex int) (GroupsResponse, error) {
	groupsUrl := fmt.Sprint(baseUrl, "/Groups")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, groupsUrl, nil)
	if err != nil {
		return GroupsResponse{}, err
	}

	q := url.Values{}
	q.Add("count", strconv.Itoa(count))
	q.Add("startIndex", strconv.Itoa(startIndex))
	req.URL.RawQuery = q.Encode()

	var res GroupsResponse
	groupsErr := c.doRequest(req, &res)
	if groupsErr != nil {
		return GroupsResponse{}, groupsErr
	}
	return res, nil
}

// GetPaginatedGroups returns all groups - paginated.
func (c *ScimClient) GetPaginatedGroups(ctx context.Context) ([]Group, error) {
	var allGroups []Group
	count := defaultCount
	totalReturned := 0
	pageIndex := defaultStartIndex

	for {
		resp, err := c.GetGroups(ctx, count, pageIndex)
		if err != nil {
			return nil, fmt.Errorf("notion-connector: failed to list groups: %w", err)
		}

		totalReturned += int(resp.ItemsPerPage)

		allGroups = append(resp.Resources, allGroups...)

		if totalReturned >= int(resp.TotalResults) {
			break
		}
		pageIndex += 1
	}

	return allGroups, nil
}

// GetGroup returns group details by group ID.
func (c *ScimClient) GetGroup(ctx context.Context, groupId string) (Group, error) {
	url := fmt.Sprint(baseUrl, "/Groups/", groupId)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Group{}, err
	}

	var res Group
	groupErr := c.doRequest(req, &res)
	if groupErr != nil {
		return Group{}, groupErr
	}
	return res, nil
}

func (c *ScimClient) GetUser(ctx context.Context, userID string) (*User, error) {
	var userData *User
	requestURL := fmt.Sprint(baseUrl, "/Users/", userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}

	err = c.doRequest(req, &userData)
	if err != nil {
		return nil, err
	}

	return userData, nil
}

func (c *ScimClient) DeleteUser(ctx context.Context, userID string) error {
	// DELETE <https://api.notion.com/scim/v2/Users/><id>
	requestURL := fmt.Sprint(baseUrl, "/Users/", userID)
	_, err := http.NewRequestWithContext(ctx, http.MethodDelete, requestURL, nil)
	if err != nil {
		return err
	}

	return nil
}

func (c *ScimClient) doRequest(req *http.Request, resType interface{}) error {
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.scimToken))
	req.Header.Add("accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&resType); err != nil {
		return err
	}

	return nil
}
