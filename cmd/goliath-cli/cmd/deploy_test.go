package cmd

import (
	"strings"
	"testing"

	"github.com/jrupac/goliath/schema"
)

func TestLastRecordIn(t *testing.T) {
	log := `{"kind":"upgrade","env":"prod","database":"goliath","hash":"a"}

{"kind":"upgrade","env":"dev","database":"upgrade_test","hash":"b"}
{"kind":"rollback","env":"prod","database":"goliath","hash":"c"}
{"kind":"upgrade","env":"dev","database":"goliath","hash":"d"}
`
	for _, c := range []struct {
		env, db string
		want    string
		found   bool
	}{
		{"prod", "goliath", "c", true},
		{"dev", "upgrade_test", "b", true},
		{"dev", "goliath", "d", true},
		{"debug", "goliath", "", false},
	} {
		rec, found, err := lastRecordIn(strings.NewReader(log), c.env, c.db)
		if err != nil {
			t.Fatal(err)
		}
		if found != c.found || rec.Hash != c.want {
			t.Errorf("%s/%s: got %q (found %v), want %q (found %v)", c.env, c.db, rec.Hash, found, c.want, c.found)
		}
	}

	if _, _, err := lastRecordIn(strings.NewReader("{\"kind\":\n"), "prod", "goliath"); err == nil {
		t.Error("a malformed line was not reported")
	}
}

func TestExpectedVersionCheck(t *testing.T) {
	v := func(n int) *int { return &n }
	up := versionInfo{BuildHash: "abc", SchemaVersion: v(29), DBSchemaVersion: v(29)}
	old := versionInfo{BuildHash: "old"}

	for _, c := range []struct {
		name string
		want expectedVersion
		got  versionInfo
		ok   bool
	}{
		{"everything matches", expectedVersion{"abc", 29, 29}, up, true},
		{"wrong build", expectedVersion{"def", 29, 29}, up, false},
		{"database behind", expectedVersion{"abc", 29, 30}, up, false},
		{"binary without schema fields", expectedVersion{"old", 29, 0}, old, false},
		// Rolling back to a binary that predates the schema fields asks only
		// about the build.
		{"only the build asked about", expectedVersion{Hash: "old"}, old, true},
		{"nothing asked", expectedVersion{}, old, true},
	} {
		if err := c.want.check(c.got); (err == nil) != c.ok {
			t.Errorf("%s: check = %v, want ok=%v", c.name, err, c.ok)
		}
	}
}

func TestCheckpointPath(t *testing.T) {
	got, err := checkpointPath("/2026/09/13-215449.91")
	if err != nil {
		t.Fatal(err)
	}
	if want := "/cockroach/cockroach-data/extern/goliath-checkpoints/2026/09/13-215449.91"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	for _, name := range []string{"", "2026/09/13-215449.91", "/../../..", "/2026/09/13 x", "/2026/'x"} {
		if _, err := checkpointPath(name); err == nil {
			t.Errorf("located %q", name)
		}
	}
}

func TestChooseRollback(t *testing.T) {
	rec := deployRecord{SchemaFrom: 25}
	baseline := schema.Applied{Version: 25, Name: "baseline", BreaksOlderBinaries: true}
	compatible := func(v int) schema.Applied { return schema.Applied{Version: v, Name: "v.sql"} }
	breaking := func(v int) schema.Applied { return schema.Applied{Version: v, Name: "v.sql", BreaksOlderBinaries: true} }

	for _, c := range []struct {
		name    string
		applied []schema.Applied
		force   bool
		full    bool
	}{
		{"every migration compatible", []schema.Applied{baseline, compatible(26), compatible(27)}, false, false},
		{"one breaks older binaries", []schema.Applied{baseline, compatible(26), breaking(27)}, false, true},
		{"asked for", []schema.Applied{baseline, compatible(26)}, true, true},
		// An upgrade that failed before recording anything leaves a database
		// whose state is known only from the checkpoint.
		{"nothing recorded", nil, false, true},
	} {
		if full, _ := chooseRollback(rec, c.applied, c.force); full != c.full {
			t.Errorf("%s: full = %v, want %v", c.name, full, c.full)
		}
	}
}
