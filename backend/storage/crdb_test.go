package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/jrupac/goliath/models"
)

// Each condition is optional and binds its own parameter, so the shapes the
// query renders are worth pinning: a stray condition would silently filter a
// stream that asked for none, a missing one would ignore the caller's request,
// and a misnumbered placeholder would bind one value where another belongs.
func TestArticleMetaQueryShapes(t *testing.T) {
	since := time.Unix(1786146087, 0)
	for _, tc := range []struct {
		name    string
		stream  models.Stream
		cursor  models.StreamCursor
		want    []string
		notWant []string
		binds   int
	}{
		{
			name:    "unbounded",
			stream:  models.Stream{Filter: models.StreamFilterUnread},
			want:    []string{"WHERE userid = $1 AND id > $2", "AND NOT read", "ORDER BY id LIMIT $3"},
			notWant: []string{"$4"},
		},
		{
			name:   "bounded by read time",
			stream: models.Stream{Filter: models.StreamFilterRead},
			cursor: models.StreamCursor{Since: since},
			want:   []string{"AND read", "AND readat > $4", "ORDER BY id LIMIT $3"},
			binds:  1,
		},
		{
			name:    "the unread part of one feed, bounded",
			stream:  models.Stream{Filter: models.StreamFilterAll, FeedID: 7, ExcludeRead: true},
			cursor:  models.StreamCursor{Since: since},
			want:    []string{"AND TRUE", "AND NOT read", "AND feed = $4", "AND date > $5"},
			notWant: []string{"folder = "},
			binds:   2,
		},
		{
			name:    "one folder",
			stream:  models.Stream{Filter: models.StreamFilterAll, FolderID: 3},
			want:    []string{"AND folder = $4"},
			notWant: []string{"$5", "NOT read", "feed = "},
			binds:   1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conditions, binds, err := articleMetaConditions(tc.stream, tc.cursor)
			if err != nil {
				t.Fatalf("articleMetaConditions: %v", err)
			}
			if len(binds) != tc.binds {
				t.Errorf("%d values bound, want %d", len(binds), tc.binds)
			}
			var q strings.Builder
			if err := articleMetaQuery.Execute(&q, conditions); err != nil {
				t.Fatalf("Execute: %v", err)
			}
			got := q.String()
			// Every read a client makes leaves unsubscribed feeds out.
			for _, want := range append(tc.want, liveArticles) {
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
		{models.StreamFilterAll, "date"},
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
