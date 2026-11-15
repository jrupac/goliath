package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/jrupac/goliath/jsonsafe"
)

// greaderResponses are the types the GReader API encodes as JSON.
var greaderResponses = map[string]reflect.Type{
	"greaderTokenType":           reflect.TypeFor[greaderTokenType](),
	"greaderHandlelogin":         reflect.TypeFor[greaderHandlelogin](),
	"greaderUserInfo":            reflect.TypeFor[greaderUserInfo](),
	"greaderCategory":            reflect.TypeFor[greaderCategory](),
	"greaderSubscription":        reflect.TypeFor[greaderSubscription](),
	"greaderSubscriptionList":    reflect.TypeFor[greaderSubscriptionList](),
	"greaderTag":                 reflect.TypeFor[greaderTag](),
	"greaderTagList":             reflect.TypeFor[greaderTagList](),
	"greaderCanonical":           reflect.TypeFor[greaderCanonical](),
	"greaderContent":             reflect.TypeFor[greaderContent](),
	"greaderOrigin":              reflect.TypeFor[greaderOrigin](),
	"greaderItemContent":         reflect.TypeFor[greaderItemContent](),
	"greaderItemRef":             reflect.TypeFor[greaderItemRef](),
	"greaderStreamItemIds":       reflect.TypeFor[greaderStreamItemIds](),
	"greaderStreamItemsContents": reflect.TypeFor[greaderStreamItemsContents](),
	"greaderQuickAddResponse":    reflect.TypeFor[greaderQuickAddResponse](),
	"parseFullArticleResponse":   reflect.TypeFor[parseFullArticleResponse](),
}

// smallGReaderNumbers are the numeric fields whose values stay far below 2^53.
var smallGReaderNumbers = map[string]bool{
	// Unix times in seconds.
	"greaderItemContent.Published":       true,
	"greaderStreamItemsContents.Updated": true,
	// How many feeds one quick-add subscribed to.
	"greaderQuickAddResponse.NumResults": true,
}

// The web client parses responses with JSON.parse, which rounds any integer
// past 2^53, and IDs are 64-bit. Every such value has to be sent as a string.
func TestGReaderResponsesHoldNoNumberAClientCouldRound(t *testing.T) {
	found := map[string]bool{}
	for _, typ := range greaderResponses {
		for _, field := range jsonsafe.ImpreciseFields(typ, smallGReaderNumbers) {
			found[field] = true
		}
	}
	for _, field := range slices.Sorted(maps.Keys(found)) {
		t.Errorf("%s may not read back exactly; send it as a string (`json:\",string\"`)", field)
	}
}

// A response type the check above does not know about would go unchecked, so
// every struct in the package that is encoded as JSON has to be listed.
func TestEveryGReaderResponseIsChecked(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, path := range files {
		// Fever's protocol sends IDs as numbers by design.
		if path == "fever.go" || strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			spec, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			st, ok := spec.Type.(*ast.StructType)
			if !ok || !hasJSONTag(st) {
				return true
			}
			if _, ok := greaderResponses[spec.Name.Name]; !ok {
				t.Errorf("%s: %s is encoded as JSON but missing from greaderResponses", fset.Position(spec.Pos()), spec.Name.Name)
			}
			return true
		})
	}
}

func hasJSONTag(st *ast.StructType) bool {
	for _, field := range st.Fields.List {
		if field.Tag == nil {
			continue
		}
		tag, err := strconv.Unquote(field.Tag.Value)
		if err != nil {
			continue
		}
		if _, ok := reflect.StructTag(tag).Lookup("json"); ok {
			return true
		}
	}
	return false
}
