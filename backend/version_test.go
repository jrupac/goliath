package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVersionReportsBuildAndSchema(t *testing.T) {
	w := httptest.NewRecorder()
	handleVersion(w, httptest.NewRequest("GET", "/version", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	dec := json.NewDecoder(bytes.NewReader(w.Body.Bytes()))
	dec.UseNumber()
	var got map[string]any
	if err := dec.Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, key := range []string{"build_timestamp", "build_hash"} {
		if _, ok := got[key].(string); !ok {
			t.Errorf("%s = %#v, want a string", key, got[key])
		}
	}
	for _, key := range []string{"schema_version", "db_schema_version"} {
		if _, ok := got[key].(json.Number); !ok {
			t.Errorf("%s = %#v, want a number", key, got[key])
		}
	}
}
