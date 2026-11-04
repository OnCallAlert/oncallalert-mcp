package tools

import (
	"fmt"
	"time"
)

const (
	defaultFirst = 15
	maxFirst     = 100
)

// PageInfo is the paging part of every list result.
type PageInfo struct {
	EndCursor   string `json:"end_cursor"`
	HasNextPage bool   `json:"has_next_page"`
}

// gqlPage is GraphQL's PageInfo.
type gqlPage struct {
	EndCursor   *string `json:"endCursor"`
	HasNextPage bool    `json:"hasNextPage"`
}

func (p gqlPage) convert() PageInfo {
	pi := PageInfo{HasNextPage: p.HasNextPage}
	if p.EndCursor != nil {
		pi.EndCursor = *p.EndCursor
	}
	return pi
}

// clampFirst applies the page size default (15) and cap (100).
func clampFirst(n int) int {
	if n <= 0 {
		return defaultFirst
	}
	return min(n, maxFirst)
}

// parseTime parses RFC 3339 or a YYYY-MM-DD date (UTC).
func parseTime(field, s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.DateOnly, s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("%s: %q is not RFC 3339 (2026-11-01T09:00:00Z) or a date (2026-11-01)", field, s)
}

// optTime parses an optional time and returns it as an RFC 3339 string ("" if empty).
func optTime(field, s string) (string, error) {
	if s == "" {
		return "", nil
	}
	t, err := parseTime(field, s)
	if err != nil {
		return "", err
	}
	return t.UTC().Format(time.RFC3339), nil
}

// notFound is the error for a lookup that returned null.
func notFound(kind, id string) error { return fmt.Errorf("%s not found: %s", kind, id) }

func errRequired(field string) error { return fmt.Errorf("%s is required", field) }
