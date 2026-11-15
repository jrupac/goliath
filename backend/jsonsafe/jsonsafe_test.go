package jsonsafe

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
	"time"
)

type leaf struct {
	ID    int64 `json:"id"`
	Small int32 `json:"small"`
}

type node struct {
	Quoted   int64  `json:"quoted,string"`
	Omitted  uint64 `json:"quotedToo,omitempty,string"`
	Skipped  int64  `json:"-"`
	hidden   int64
	Ratio    float64          `json:"ratio"`
	Name     string           `json:"name"`
	Leaves   []leaf           `json:"leaves"`
	ByName   map[int64]*leaf  `json:"byName"`
	Counts   [2]uint          `json:"counts"`
	Anything any              `json:"anything"`
	When     time.Time        `json:"when"`
	Raw      json.RawMessage  `json:"raw"`
	Quoted2  []int64          `json:"quotedSlice,string"`
	Self     *node            `json:"self"`
	Nested   map[string][]int `json:"nested"`
	Untagged int
}

func TestImpreciseFields(t *testing.T) {
	got := ImpreciseFields(reflect.TypeFor[node](), nil)
	want := []string{
		"leaf.ID",
		"node.Anything",
		"node.Counts",
		"node.Nested",
		"node.Quoted2",
		"node.Raw",
		"node.Untagged",
		"node.When",
	}
	if !slices.Equal(got, want) {
		t.Errorf("ImpreciseFields =\n  %q\nwant\n  %q", got, want)
	}
}

func TestImpreciseFieldsSkipsExemptions(t *testing.T) {
	got := ImpreciseFields(reflect.TypeFor[[]leaf](), map[string]bool{"leaf.ID": true})
	if len(got) != 0 {
		t.Errorf("ImpreciseFields = %q, want none", got)
	}
}

func TestImpreciseFieldsOfAScalar(t *testing.T) {
	if got := ImpreciseFields(reflect.TypeFor[int64](), nil); !slices.Equal(got, []string{"int64"}) {
		t.Errorf("ImpreciseFields(int64) = %q", got)
	}
	if got := ImpreciseFields(reflect.TypeFor[string](), nil); len(got) != 0 {
		t.Errorf("ImpreciseFields(string) = %q, want none", got)
	}
}
