package api

import (
	"errors"
	"net/url"
	"strconv"
)

// Page sizes for list endpoints. Without a limit, a list returns one page.
const (
	defaultPageSize = 100
	maxPageSize     = 500
)

var errBadPaging = errors.New("limit must be between 1 and 500, offset must be 0 or more")

// parsePaging reads the optional limit and offset query parameters.
func parsePaging(q url.Values) (limit, offset int, err error) {
	limit = defaultPageSize
	if v := q.Get("limit"); v != "" {
		limit, err = strconv.Atoi(v)
		if err != nil || limit < 1 || limit > maxPageSize {
			return 0, 0, errBadPaging
		}
	}
	if v := q.Get("offset"); v != "" {
		offset, err = strconv.Atoi(v)
		if err != nil || offset < 0 {
			return 0, 0, errBadPaging
		}
	}
	return limit, offset, nil
}
