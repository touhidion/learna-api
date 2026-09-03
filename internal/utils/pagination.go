package utils

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	defaultPage     = 1
	defaultPageSize = 20
	maxPageSize     = 100
)

// Pagination is an offset-based page request. Phase 1 uses offset paging
// everywhere; the architecture doc reserves cursor paging for public listings
// once catalogue size makes it worthwhile.
type Pagination struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

// Limit and Offset translate the page request into SQL terms.
func (p Pagination) Limit() int  { return p.PageSize }
func (p Pagination) Offset() int { return (p.Page - 1) * p.PageSize }

// ParsePagination reads ?page and ?page_size, clamping both into range.
// Malformed values fall back to the defaults rather than erroring, so a bad
// bookmark still returns the first page.
func ParsePagination(c *gin.Context) Pagination {
	p := Pagination{Page: defaultPage, PageSize: defaultPageSize}

	if v, err := strconv.Atoi(c.Query("page")); err == nil && v > 0 {
		p.Page = v
	}
	if v, err := strconv.Atoi(c.Query("page_size")); err == nil && v > 0 {
		p.PageSize = min(v, maxPageSize)
	}
	return p
}

// PageMeta accompanies a paginated payload.
type PageMeta struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}

// Page is the envelope for every list endpoint.
type Page[T any] struct {
	Items []T      `json:"items"`
	Meta  PageMeta `json:"meta"`
}

// NewPage wraps items with the metadata derived from p and the total count.
// A nil slice is normalised to an empty one so the JSON is [] and not null.
func NewPage[T any](items []T, total int64, p Pagination) Page[T] {
	if items == nil {
		items = []T{}
	}

	totalPages := 0
	if p.PageSize > 0 {
		totalPages = int((total + int64(p.PageSize) - 1) / int64(p.PageSize))
	}

	return Page[T]{
		Items: items,
		Meta: PageMeta{
			Page:       p.Page,
			PageSize:   p.PageSize,
			TotalItems: total,
			TotalPages: totalPages,
			HasNext:    p.Page < totalPages,
			HasPrev:    p.Page > 1,
		},
	}
}
