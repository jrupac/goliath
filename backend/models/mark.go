package models

import "fmt"

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
