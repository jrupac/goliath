// Package jsonsafe checks that a type's JSON encoding reads back exactly in a
// JavaScript client.
//
// A browser's JSON.parse reads every number as a double, which holds integers
// exactly only up to 2^53. A 64-bit ID sent as a JSON number is rounded
// without any error, so such values have to travel as strings.
package jsonsafe

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
)

var marshaler = reflect.TypeFor[json.Marshaler]()

// ImpreciseFields returns the fields reachable from t whose encoded values a
// JavaScript client could misread, each as "Type.Field". These are:
//
//   - integers wider than 32 bits, unless tagged ",string";
//   - interfaces, whose dynamic values cannot be known from the type;
//   - types that encode themselves with MarshalJSON.
//
// Fields named in exempt are skipped: those whose values are known to stay
// far below 2^53, such as timestamps in seconds or counts.
func ImpreciseFields(t reflect.Type, exempt map[string]bool) []string {
	w := walker{exempt: exempt, seen: map[reflect.Type]bool{}}
	w.value(t, t.String())
	slices.Sort(w.found)
	return slices.Compact(w.found)
}

type walker struct {
	exempt map[string]bool
	seen   map[reflect.Type]bool
	found  []string
}

// value checks a value of type t, reached at path.
func (w *walker) value(t reflect.Type, path string) {
	if t.Implements(marshaler) || reflect.PointerTo(t).Implements(marshaler) {
		w.found = append(w.found, path)
		return
	}
	switch t.Kind() {
	case reflect.Int, reflect.Int64, reflect.Uint, reflect.Uint64, reflect.Uintptr,
		reflect.Interface:
		w.found = append(w.found, path)
	case reflect.Pointer, reflect.Slice, reflect.Array:
		w.value(t.Elem(), path)
	case reflect.Map:
		// Keys are always encoded as strings.
		w.value(t.Elem(), path)
	case reflect.Struct:
		w.fields(t)
	default:
		// Narrower integers, floats, strings and booleans read back exactly.
	}
}

// fields checks each field of the struct t that is encoded.
func (w *walker) fields(t reflect.Type) {
	if w.seen[t] {
		return
	}
	w.seen[t] = true
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() && !f.Anonymous {
			continue
		}
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		path := t.Name() + "." + f.Name
		if w.exempt[path] {
			continue
		}
		_, opts, _ := strings.Cut(tag, ",")
		if slices.Contains(strings.Split(opts, ","), "string") {
			// The option quotes only scalars, and only those held directly.
			switch f.Type.Kind() {
			case reflect.Int, reflect.Int64, reflect.Uint, reflect.Uint64, reflect.Uintptr:
				continue
			default:
			}
		}
		w.value(f.Type, path)
	}
}
