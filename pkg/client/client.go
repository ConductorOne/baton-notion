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
	baseUrl           = "https://www.notion.so/scim/v2"
	defaultUserSchema = "urn:ietf:params:scim:schemas:core:2.0:User"
)

type ScimClient struct {
	client    *uhttp.BaseHttpClient
	scimToken string
}

// GetGroups returns all Notion groups.
func (c *ScimClient) GetGroups(ctx context.Context, pageOps PaginationOptions) ([]Group, string, error) {
	var nextPage string
	requestURL := fmt.Sprint(baseUrl, "/Groups")

	var res GroupsResponse
	_, err := c.doRequest(
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

// GetGroup returns group details by group ID.
func (c *ScimClient) GetGroup(ctx context.Context, groupId string) (Group, error) {
	requestURL := fmt.Sprint(baseUrl, "/Groups/", groupId)

	var groupResponse Group
	_, err := c.doRequest(ctx, http.MethodGet, requestURL, &groupResponse, nil)
	if err != nil {
		return Group{}, err
	}

	return groupResponse, nil
}

func (c *ScimClient) GetUser(ctx context.Context, userID string) (*User, error) {
	var userData *User
	requestURL := fmt.Sprint(baseUrl, "/Users/", userID)

	_, err := c.doRequest(ctx, http.MethodGet, requestURL, &userData, nil)
	if err != nil {
		return nil, err
	}

	return userData, nil
}

func (c *ScimClient) DeleteUser(ctx context.Context, userID string) error {
	// DELETE <https://api.notion.com/scim/v2/Users/><id>
	requestURL := fmt.Sprint(baseUrl, "/Users/", userID)

	_, err := c.doRequest(ctx, http.MethodDelete, requestURL, nil, nil)
	if err != nil {
		return err
	}

	return nil
}

//func (c *ScimClient) doRequest(req *http.Request, resType interface{}) error {
//	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.scimToken))
//	req.Header.Add("accept", "application/json")
//	resp, err := c.httpClient.Do(req)
//	if err != nil {
//		return err
//	}
//
//	defer resp.Body.Close()
//
//	if err := json.NewDecoder(resp.Body).Decode(&resType); err != nil {
//		return err
//	}
//
//	return nil
//}

func (c *ScimClient) doRequest(
	ctx context.Context,
	method string,
	endpointUrl string,
	res interface{},
	body interface{},
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

	opts := []uhttp.RequestOption{uhttp.WithBearerToken(c.scimToken)}
	if body != nil {
		opts = append(opts, uhttp.WithAcceptJSONHeader(), uhttp.WithContentTypeJSONHeader(), uhttp.WithJSONBody(body))
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

func New(ctx context.Context, scimToken string) (*ScimClient, error) {
	httpClient, err := uhttp.NewClient(ctx, uhttp.WithLogger(true, ctxzap.Extract(ctx)))
	if err != nil {
		return nil, err
	}

	cli, err := uhttp.NewBaseHttpClientWithContext(ctx, httpClient)
	if err != nil {
		return nil, err
	}

	notionClient := ScimClient{
		client:    cli,
		scimToken: scimToken,
	}

	return &notionClient, nil
}
