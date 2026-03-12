package connector

import (
	"strconv"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/pagination"
)

func getToken(tokenStr string, resourceType *v2.ResourceType) (*pagination.Bag, int, error) {
	var pageToken int
	bag := &pagination.Bag{}

	if tokenStr != "" {
		err := bag.Unmarshal(tokenStr)
		if err != nil {
			return bag, 0, err
		}
	}

	if bag.Current() == nil {
		bag.Push(pagination.PageState{
			ResourceTypeID: resourceType.Id,
		})
	}

	if bag.PageToken() != "" {
		var err error
		pageToken, err = strconv.Atoi(bag.PageToken())
		if err != nil {
			return bag, 0, err
		}
	}

	return bag, pageToken, nil
}
