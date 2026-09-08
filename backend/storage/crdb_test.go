package storage

import (
	"strings"
	"testing"

	"github.com/jrupac/goliath/models"
)

// The time bound is an optional clause with its own parameter, so the two
// shapes the template renders are worth pinning: a stray bound would silently
// filter a stream that asked for none, and a missing one would ignore the
// caller's request.
func TestArticleMetaQueryShapes(t *testing.T) {
	for _, tc := range []struct {
		name      string
		fragments articleMetaQueryFragments
		want      []string
		notWant   []string
	}{
		{
			name:      "unbounded",
			fragments: articleMetaQueryFragments{Filter: "NOT read"},
			want:      []string{"WHERE userid = $1 AND id > $2 AND NOT read", "ORDER BY id LIMIT $3"},
			notWant:   []string{"$4"},
		},
		{
			name:      "bounded by read time",
			fragments: articleMetaQueryFragments{Filter: "read", SinceColumn: "readat"},
			want:      []string{"AND read", "AND readat > $4", "ORDER BY id LIMIT $3"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var q strings.Builder
			if err := articleMetaQuery.Execute(&q, tc.fragments); err != nil {
				t.Fatalf("Execute: %v", err)
			}
			got := q.String()
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Errorf("query missing %q:\n%s", want, got)
				}
			}
			for _, notWant := range tc.notWant {
				if strings.Contains(got, notWant) {
					t.Errorf("query contains %q, which it should not:\n%s", notWant, got)
				}
			}
		})
	}
}

// Every stream filter must select a predicate, and the read stream must be the
// only one whose time bound is on read time rather than publication time.
func TestArticleMetaFragmentsCoverEveryFilter(t *testing.T) {
	for _, tc := range []struct {
		filter      models.StreamFilter
		sinceColumn string
	}{
		{models.StreamFilterRead, "readat"},
		{models.StreamFilterUnread, "date"},
		{models.StreamFilterSaved, "date"},
		{models.StreamFilterUnsaved, "date"},
	} {
		got, err := articleMetaFragments(tc.filter)
		if err != nil {
			t.Errorf("filter %v: %v", tc.filter, err)
			continue
		}
		if got.Filter == "" {
			t.Errorf("filter %v has no predicate", tc.filter)
		}
		if got.SinceColumn != tc.sinceColumn {
			t.Errorf("filter %v bounds on %q, want %q", tc.filter, got.SinceColumn, tc.sinceColumn)
		}
	}

	if _, err := articleMetaFragments(models.StreamFilterUnknown); err == nil {
		t.Error("an unknown filter was accepted")
	}
}
