package cmd

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jrupac/goliath/schema"
)

func TestParseVersion(t *testing.T) {
	for in, want := range map[string]int{
		"v19_add_saved_to_article.sql": 19,
		"v25":                          25,
		"25":                           25,
		"v28.sql":                      28,
	} {
		if got, err := parseVersion(in); err != nil || got != want {
			t.Errorf("parseVersion(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", "v", "vx", "0", "-3", "latest.sql"} {
		if _, err := parseVersion(in); err == nil {
			t.Errorf("parseVersion(%q) accepted", in)
		}
	}
}

func migrationsUpTo(versions ...int) []schema.Migration {
	var ms []schema.Migration
	for _, v := range versions {
		ms = append(ms, schema.Migration{
			Version: v,
			Name:    "v" + strconv.Itoa(v) + "_x.sql",
			SQL:     "SET DATABASE TO Goliath;\n",
		})
	}
	return ms
}

// withTrackingTable makes the migration numbered v the one that creates the
// tracking table.
func withTrackingTable(ms []schema.Migration, v int) []schema.Migration {
	for i := range ms {
		if ms[i].Version == v {
			ms[i].SQL += "CREATE TABLE IF NOT EXISTS SchemaVersion (version INT PRIMARY KEY);\n"
		}
	}
	return ms
}

func versions(ms []schema.Migration) []int {
	var vs []int
	for _, m := range ms {
		vs = append(vs, m.Version)
	}
	return vs
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestPlanPending(t *testing.T) {
	onDisk := migrationsUpTo(20, 25, 26, 27, 28)
	at := func(v int) []schema.Applied { return []schema.Applied{{Version: v, Name: baselineName}} }

	cases := []struct {
		name    string
		disk    []schema.Migration
		applied []schema.Applied
		target  int
		want    []int
		wantErr string
	}{
		{name: "everything pending", disk: onDisk, applied: at(25), want: []int{26, 27, 28}},
		{name: "up to a target", disk: onDisk, applied: at(25), target: 27, want: []int{26, 27}},
		{name: "nothing pending", disk: onDisk, applied: at(28)},
		{name: "target already applied", disk: onDisk, applied: at(28), target: 26},
		{name: "database ahead of checkout", disk: onDisk, applied: at(29), wantErr: "behind"},
		{name: "target beyond checkout", disk: onDisk, applied: at(25), target: 30, wantErr: "no migration v30"},
		{name: "gap in the numbering", disk: migrationsUpTo(25, 26, 28), applied: at(25), wantErr: "no migration v27"},
		// A baseline at a version with no file of its own is fine: what
		// matters is that the next file follows it.
		{name: "baseline between files", disk: migrationsUpTo(20, 26), applied: at(25), want: []int{26}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := planPending(c.disk, c.applied, c.target)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("err = %v, want one mentioning %q", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !equalInts(versions(got), c.want) {
				t.Fatalf("got %v, want %v", versions(got), c.want)
			}
		})
	}
}

func TestMakePlan(t *testing.T) {
	onDisk := withTrackingTable(migrationsUpTo(25, 26, 27, 28, 29), 29)
	recorded := func(vs ...int) []schema.Applied {
		var as []schema.Applied
		for _, v := range vs {
			as = append(as, schema.Applied{Version: v, Name: "v" + strconv.Itoa(v) + "_x.sql"})
		}
		return as
	}

	cases := []struct {
		name      string
		applied   []schema.Applied
		versioned bool
		baseline  int
		target    int
		want      []int
		bootstrap bool
		wantErr   string
	}{
		{name: "unversioned, no baseline", wantErr: "--baseline"},
		{name: "unversioned, with a baseline", baseline: 25, want: []int{26, 27, 28, 29}, bootstrap: true},
		{name: "unversioned, stopping before the table", baseline: 25, target: 28, wantErr: "creates the SchemaVersion"},
		{name: "unversioned, baseline past the table", baseline: 29, wantErr: "creates the SchemaVersion"},
		{name: "versioned, with a baseline", applied: recorded(29), versioned: true, baseline: 25, wantErr: "already records"},
		{name: "versioned but empty", versioned: true, wantErr: "empty"},
		{name: "versioned", applied: recorded(27), versioned: true, want: []int{28, 29}},
		{name: "versioned, current", applied: recorded(29), versioned: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan, err := makePlan(onDisk, c.applied, c.versioned, c.baseline, c.target)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("err = %v, want one mentioning %q", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !equalInts(versions(plan.Pending), c.want) {
				t.Errorf("pending %v, want %v", versions(plan.Pending), c.want)
			}
			if (plan.Baseline != nil) != c.bootstrap {
				t.Errorf("baseline %v, want bootstrap=%v", plan.Baseline, c.bootstrap)
			}
		})
	}
}

// The migrations in the checkout, not only the fixtures above: the bootstrap
// has to find the real file that creates the table, and every file it may
// apply has to be retargetable.
func TestCheckoutMigrations(t *testing.T) {
	migrations, err := schema.Load(os.DirFS(filepath.Join("..", "..", "..", "backend", "schema")))
	if err != nil {
		t.Fatal(err)
	}

	plan, err := makePlan(migrations, nil, false, 25, 0)
	if err != nil {
		t.Fatalf("bootstrapping from the production baseline: %v", err)
	}
	for _, m := range plan.Pending {
		if _, err := forDatabase(m.SQL, "upgrade_test"); err != nil {
			t.Errorf("%s: %v", m.Name, err)
		}
	}
}

func TestForDatabase(t *testing.T) {
	got, err := forDatabase("-- Summary.\n\nSET DATABASE TO Goliath;\n\nALTER TABLE Feed ADD COLUMN x INT;\n", "upgrade_test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "SET DATABASE TO upgrade_test;") || strings.Contains(got, "Goliath") {
		t.Errorf("not retargeted:\n%s", got)
	}

	for name, sql := range map[string]string{
		"no statement":  "ALTER TABLE Feed ADD COLUMN x INT;\n",
		"two":           "SET DATABASE TO Goliath;\nSET DATABASE TO goliath;\n",
		"other databse": "SET DATABASE TO defaultdb;\n",
	} {
		if _, err := forDatabase(sql, "upgrade_test"); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestParseApplied(t *testing.T) {
	// The SQL shell has printed booleans both ways across versions.
	out := "version,name,breaks_older_binaries\n25,baseline,true\n26,v26_file_orphan_folders_under_root.sql,f\n"
	records, err := parseCsvRecords([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	got, err := parseApplied(records)
	if err != nil {
		t.Fatal(err)
	}
	want := []schema.Applied{
		{Version: 25, Name: "baseline", BreaksOlderBinaries: true},
		{Version: 26, Name: "v26_file_orphan_folders_under_root.sql", BreaksOlderBinaries: false},
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d = %v, want %v", i, got[i], want[i])
		}
	}

	if records, _ := parseCsvRecords([]byte("version,name,breaks_older_binaries\n")); records != nil {
		t.Errorf("header alone parsed as %v", records)
	}
}

func TestRecordStatementRefusesUnsafeNames(t *testing.T) {
	if _, err := recordStatement(26, "v26_x.sql", false); err != nil {
		t.Errorf("refused a migration file name: %v", err)
	}
	if _, err := recordStatement(1, baselineName, true); err != nil {
		t.Errorf("refused the baseline name: %v", err)
	}
	for _, name := range []string{"x'); DROP DATABASE goliath; --", "v26 x.sql", ""} {
		if _, err := recordStatement(26, name, false); err == nil {
			t.Errorf("accepted %q", name)
		}
	}
}
