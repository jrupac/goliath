package models

import (
	"fmt"
	"time"
)

type MarkType int

const (
	MarkTypeUnknown MarkType = iota
	MarkTypeRead
	MarkTypeSaved
)

type MarkAction int

const (
	MarkActionUnknown MarkAction = iota
	MarkActionRead
	MarkActionUnread
	MarkActionSaved
	MarkActionUnsaved
)

// Parse returns the database column name and boolean value for the action.
func (a MarkAction) Parse() (markType MarkType, value bool, err error) {
	switch a {
	case MarkActionRead:
		return MarkTypeRead, true, nil
	case MarkActionUnread:
		return MarkTypeRead, false, nil
	case MarkActionSaved:
		return MarkTypeSaved, true, nil
	case MarkActionUnsaved:
		return MarkTypeSaved, false, nil
	default:
		return MarkTypeUnknown, false, fmt.Errorf("unknown MarkAction: %d", a)
	}
}

type StreamFilter int

const (
	StreamFilterUnknown StreamFilter = iota
	StreamFilterRead
	StreamFilterUnread
	StreamFilterSaved
	StreamFilterUnsaved
)

// StreamCursor bounds a query over a stream of articles.
//
// The two bounds answer different questions and compose: SinceID pages through
// a result set in ID order, while Since narrows that set to articles that
// entered the stream recently. Zero values leave the corresponding bound off.
type StreamCursor struct {
	// SinceID is an exclusive lower bound on article ID.
	SinceID int64
	// Since is an exclusive lower bound on the time an article entered the
	// stream: when it was marked read for a read-filtered stream, and when it
	// was published otherwise.
	Since time.Time
}
