package connector

import (
	"strconv"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/pagination"
)

func getToken(pToken *pagination.Token, resourceType *v2.ResourceType) (*pagination.Bag, int, error) {
	var pageToken int
	_, bag, err := unmarshalSkipToken(pToken)
	if err != nil {
		return bag, 0, err
	}

	if bag.Current() == nil {
		bag.Push(pagination.PageState{
			ResourceTypeID: resourceType.Id,
		})
	}

	if bag.Current().Token != "" {
		pageToken, err = strconv.Atoi(bag.Current().Token)
		if err != nil {
			return bag, 0, err
		}
	}

	return bag, pageToken, nil
}

func unmarshalSkipToken(token *pagination.Token) (int32, *pagination.Bag, error) {
	b := &pagination.Bag{}
	err := b.Unmarshal(token.Token)
	if err != nil {
		return 0, nil, err
	}
	current := b.Current()

	ret := int32(0)
	if current != nil && current.Token != "" {
		currentToken, err := strconv.ParseInt(current.Token, 10, 32)
		if err != nil {
			return 0, nil, err
		}
		ret = int32(currentToken)
	}

	return ret, b, nil
}
