// Package jsonsafe keeps a response's JSON readable exactly by a JavaScript
// client.
//
// A browser's JSON.parse reads every number as a double, which holds integers
// exactly only up to 2^53. A 64-bit ID sent as a JSON number is rounded
// without any error, so every integer in a response either travels as a
// string, through the ",string" option, or is an Int, which refuses to encode
// a value a client would misread.
package jsonsafe

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
)

var (
	marshaler = reflect.TypeFor[json.Marshaler]()
	intType   = reflect.TypeFor[Int]()
)

// checked caches what Check concluded about each type, since a type's verdict
// never changes and servers encode the same few types on every request.
var checked sync.Map // reflect.Type -> error

// Check reports an error naming every field of v's type whose encoded value a
// JavaScript client could misread, or nil if there are none.
func Check(v any) error {
	t := reflect.TypeOf(v)
	if t == nil {
		return nil
	}
	if err, ok := checked.Load(t); ok {
		if err == nil {
			return nil
		}
		return err.(error)
	}
	var err error
	if fields := ImpreciseFields(t); len(fields) > 0 {
		err = fmt.Errorf("%s holds numbers a JavaScript client could misread: %s; "+
			"send each as a string (`json:\",string\"`), or as a jsonsafe.Int if it has to stay a number",
			t, strings.Join(fields, ", "))
	}
	checked.Store(t, err)
	return err
}

// ImpreciseFields returns the fields reachable from t whose encoded values a
// JavaScript client could misread, each as "Type.Field". These are:
//
//   - integers of any width, unless tagged ",string" or of type Int;
//   - interfaces, whose dynamic values cannot be known from the type;
//   - types other than Int that encode themselves with MarshalJSON.
func ImpreciseFields(t reflect.Type) []string {
	w := walker{seen: map[reflect.Type]bool{}}
	w.value(t, t.String())
	slices.Sort(w.found)
	return slices.Compact(w.found)
}

type walker struct {
	seen  map[reflect.Type]bool
	found []string
}

// value checks a value of type t, reached at path.
func (w *walker) value(t reflect.Type, path string) {
	if t == intType {
		return
	}
	if t.Implements(marshaler) || reflect.PointerTo(t).Implements(marshaler) {
		w.found = append(w.found, path)
		return
	}
	switch {
	case isInteger(t), t.Kind() == reflect.Interface:
		w.found = append(w.found, path)
	case t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8:
		// Bytes are encoded as a base64 string.
	case t.Kind() == reflect.Pointer, t.Kind() == reflect.Slice, t.Kind() == reflect.Array:
		w.value(t.Elem(), path)
	case t.Kind() == reflect.Map:
		// Keys are always encoded as strings.
		w.value(t.Elem(), path)
	case t.Kind() == reflect.Struct:
		w.fields(t)
	default:
		// Floats are doubles on both ends; strings and booleans are exact.
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
		// The option quotes only scalars, and only those held directly.
		_, opts, _ := strings.Cut(tag, ",")
		if isInteger(f.Type) && slices.Contains(strings.Split(opts, ","), "string") {
			continue
		}
		w.value(f.Type, t.Name()+"."+f.Name)
	}
}

func isInteger(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return true
	default:
		return false
	}
}
