package jsonsafe

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

type leaf struct {
	ID    int64 `json:"id"`
	Small int32 `json:"small"`
	Safe  Int   `json:"safe"`
}

type node struct {
	Quoted   int64  `json:"quoted,string"`
	Omitted  uint64 `json:"quotedToo,omitempty,string"`
	Narrow   int8   `json:"narrow,string"`
	Skipped  int64  `json:"-"`
	hidden   int64
	Count    Int              `json:"count"`
	Counts   []Int            `json:"counts"`
	Ratio    float64          `json:"ratio"`
	Name     string           `json:"name"`
	Bytes    []byte           `json:"bytes"`
	Leaves   []leaf           `json:"leaves"`
	ByName   map[int64]*leaf  `json:"byName"`
	Unsigned [2]uint          `json:"unsigned"`
	Anything any              `json:"anything"`
	When     time.Time        `json:"when"`
	Raw      json.RawMessage  `json:"raw"`
	Quoted2  []int64          `json:"quotedSlice,string"`
	Self     *node            `json:"self"`
	Nested   map[string][]int `json:"nested"`
	Untagged int
}

func TestImpreciseFields(t *testing.T) {
	got := ImpreciseFields(reflect.TypeFor[node]())
	want := []string{
		"leaf.ID",
		"leaf.Small",
		"node.Anything",
		"node.Nested",
		"node.Quoted2",
		"node.Raw",
		"node.Unsigned",
		"node.Untagged",
		"node.When",
	}
	if !slices.Equal(got, want) {
		t.Errorf("ImpreciseFields =\n  %q\nwant\n  %q", got, want)
	}
}

func TestImpreciseFieldsOfAScalar(t *testing.T) {
	if got := ImpreciseFields(reflect.TypeFor[int64]()); !slices.Equal(got, []string{"int64"}) {
		t.Errorf("ImpreciseFields(int64) = %q", got)
	}
	for _, typ := range []reflect.Type{reflect.TypeFor[string](), reflect.TypeFor[Int]()} {
		if got := ImpreciseFields(typ); len(got) != 0 {
			t.Errorf("ImpreciseFields(%s) = %q, want none", typ, got)
		}
	}
}

type safe struct {
	ID    int64 `json:"id,string"`
	Count Int   `json:"count"`
}

type unsafe struct {
	ID    int64 `json:"id"`
	Count int   `json:"count"`
}

func TestCheck(t *testing.T) {
	for _, v := range []any{nil, safe{}, &safe{}, []safe{}, "text"} {
		if err := Check(v); err != nil {
			t.Errorf("Check(%T) = %v, want nil", v, err)
		}
	}
	// Asked twice, so that the second answer comes from the cache.
	for range 2 {
		err := Check(unsafe{})
		if err == nil {
			t.Fatal("Check(unsafe) = nil, want an error")
		}
		for _, field := range []string{"unsafe.ID", "unsafe.Count"} {
			if !strings.Contains(err.Error(), field) {
				t.Errorf("Check(unsafe) = %q, want it to name %s", err, field)
			}
		}
	}
}
