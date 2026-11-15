package main

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/jrupac/goliath/jsonsafe"
)

// The web client parses /version with JSON.parse, which rounds any integer past
// 2^53. Schema versions are migration numbers, far below that.
func TestVersionHoldsNoNumberAClientCouldRound(t *testing.T) {
	small := map[string]bool{
		"versionResponse.SchemaVersion":   true,
		"versionResponse.DBSchemaVersion": true,
	}
	for _, field := range jsonsafe.ImpreciseFields(reflect.TypeFor[versionResponse](), small) {
		t.Errorf("%s may not read back exactly; send it as a string (`json:\",string\"`)", field)
	}
}

func TestVersionReportsBuildAndSchema(t *testing.T) {
	w := httptest.NewRecorder()
	handleVersion(w, httptest.NewRequest("GET", "/version", nil))

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, key := range []string{"build_timestamp", "build_hash", "schema_version", "db_schema_version"} {
		if _, ok := got[key]; !ok {
			t.Errorf("response %s has no %q", w.Body, key)
		}
	}
}
