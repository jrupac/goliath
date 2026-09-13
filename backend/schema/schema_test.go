package schema

import (
	"io/fs"
	"regexp"
	"strconv"
	"testing"
	"testing/fstest"
)

// Latest panics if the embedded files do not load, so this is what keeps a bad
// migration file from reaching a binary.
func TestEmbeddedMigrationsLoad(t *testing.T) {
	if embeddedErr != nil {
		t.Fatalf("loading embedded migrations: %v", embeddedErr)
	}
	for i := 1; i < len(embeddedMigrations); i++ {
		if embeddedMigrations[i].Version <= embeddedMigrations[i-1].Version {
			t.Errorf("%s is not after %s", embeddedMigrations[i].Name, embeddedMigrations[i-1].Name)
		}
	}
	if Latest() != embeddedMigrations[len(embeddedMigrations)-1].Version {
		t.Errorf("Latest() = %d, want the newest file's version", Latest())
	}
}

var latestStamp = regexp.MustCompile(`INSERT INTO SchemaVersion[^;]*?SELECT\s+([0-9]+)\s*,`)

// A database created from latest.sql has to be recorded at the version the
// file describes, or the binary built beside it refuses to start on it.
func TestLatestSQLIsStampedWithLatest(t *testing.T) {
	content, err := fs.ReadFile(embedded, "latest.sql")
	if err != nil {
		t.Fatal(err)
	}
	m := latestStamp.FindStringSubmatch(string(content))
	if m == nil {
		t.Fatal("latest.sql records no schema version")
	}
	if v, _ := strconv.Atoi(m[1]); v != Latest() {
		t.Errorf("latest.sql records v%d, but the newest migration is v%d", v, Latest())
	}
}

func TestParseCompat(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    Compat
		wantErr bool
	}{
		{"compatible", "-- Summary.\n--\n-- older-binaries: compatible\n\nSET DATABASE TO Goliath;\n", CompatOlderBinariesRun, false},
		{"incompatible", "-- older-binaries: incompatible\nALTER TABLE x DROP COLUMN y;\n", CompatOlderBinariesBreak, false},
		{"absent", "-- Summary.\nSET DATABASE TO Goliath;\n", CompatUnknown, false},
		// A declaration after the first statement is part of the body, where
		// it would be easy to leave behind in a copied file.
		{"after the leading comments", "SET DATABASE TO Goliath;\n-- older-binaries: compatible\n", CompatUnknown, false},
		{"misspelled value", "-- older-binaries: yes\n", CompatUnknown, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseCompat(c.content)
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, c.wantErr)
			}
			if got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	declared := &fstest.MapFile{Data: []byte("-- older-binaries: compatible\n")}
	undeclared := &fstest.MapFile{Data: []byte("-- Nothing said.\n")}

	t.Run("orders by version, not by name", func(t *testing.T) {
		got, err := Load(fstest.MapFS{
			"v9.sql":         undeclared,
			"v10.sql":        undeclared,
			"v27_thing.sql":  declared,
			"latest.sql":     undeclared,
			"base.sql":       undeclared,
			"v2_readme.txt":  undeclared,
			"v26_other.sql":  declared,
			"schema.go":      undeclared,
			"v25_before.sql": undeclared,
		})
		if err != nil {
			t.Fatal(err)
		}
		var versions []int
		for _, m := range got {
			versions = append(versions, m.Version)
		}
		want := []int{9, 10, 25, 26, 27}
		if len(versions) != len(want) {
			t.Fatalf("got %v, want %v", versions, want)
		}
		for i := range want {
			if versions[i] != want[i] {
				t.Fatalf("got %v, want %v", versions, want)
			}
		}
	})

	for name, fsys := range map[string]fstest.MapFS{
		"shared version":            {"v26_a.sql": declared, "v26_b.sql": declared},
		"undeclared recent version": {"v26_a.sql": undeclared},
		"unreadable declaration":    {"v26_a.sql": {Data: []byte("-- older-binaries: maybe\n")}},
	} {
		t.Run("refuses "+name, func(t *testing.T) {
			if _, err := Load(fsys); err == nil {
				t.Error("loaded without error")
			}
		})
	}
}

func TestCheck(t *testing.T) {
	baseline := Applied{Version: 25, Name: "baseline", BreaksOlderBinaries: true}
	additive := func(v int) Applied {
		return Applied{Version: v, Name: "v" + strconv.Itoa(v) + ".sql"}
	}
	breaking := func(v int) Applied {
		return Applied{Version: v, Name: "v" + strconv.Itoa(v) + ".sql", BreaksOlderBinaries: true}
	}

	cases := []struct {
		name    string
		binary  int
		applied []Applied
		ok      bool
	}{
		{"unversioned", 28, nil, false},
		{"matching", 28, []Applied{baseline, additive(26), additive(27), additive(28)}, true},
		{"behind", 28, []Applied{baseline, additive(26)}, false},
		{"ahead, compatible", 26, []Applied{baseline, additive(26), additive(27), additive(28)}, true},
		{"ahead, breaking", 26, []Applied{baseline, additive(26), breaking(27), additive(28)}, false},
		// What the binary already knows about is not its concern, however it
		// was declared.
		{"breaking, but not ahead", 27, []Applied{baseline, additive(26), breaking(27)}, true},
		// A baseline is conservative: nothing is known about what it covers.
		{"older than a baseline", 20, []Applied{baseline}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := Check(c.binary, c.applied)
			if (err == nil) != c.ok {
				t.Errorf("Check = %v, want ok=%v", err, c.ok)
			}
		})
	}
}
