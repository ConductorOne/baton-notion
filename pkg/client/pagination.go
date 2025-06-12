package client

import (
	"net/url"
	"strconv"
)

const maxItemsPerPage = 100

type ReqOpt func(reqURL *url.URL)

type PaginationOptions struct {
	StartIndex int `url:"startIndex,omitempty"`
	PerPage    int `url:"count,omitempty"`
}

func WithPageSize(count int) ReqOpt {
	if count <= 0 || count > maxItemsPerPage {
		count = maxItemsPerPage
	}
	return WithQueryParam("count", strconv.Itoa(count))
}

func WithStartIndex(index int) ReqOpt {
	if index <= 0 {
		index = 1
	}
	return WithQueryParam("startIndex", strconv.Itoa(index))
}

func WithQueryParam(key string, value string) ReqOpt {
	return func(reqURL *url.URL) {
		q := reqURL.Query()
		q.Set(key, value)
		reqURL.RawQuery = q.Encode()
	}
}
