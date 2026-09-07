package cmd

import (
	"sort"
	"testing"
)

// The backstop guards the one property that matters: a checkpoint name is
// interpolated into a string literal, so it must not be able to end one. It
// deliberately does not encode CockroachDB's naming, which is documented by
// example rather than guaranteed.
func TestCheckpointIdSafeAcceptsNamesThatCannotEscape(t *testing.T) {
	for _, id := range []string{
		"/2026/09/07-041622.99",
		"/2026/01/01-000000.00",
		// Shapes CockroachDB does not currently produce, admitted so that a
		// change to its naming does not become a refusal to restore.
		"/2026/09/07-041622.999999",
		"/2026/09/07T041622Z",
		"backup_2026-09-07",
	} {
		if !checkpointIdSafe.MatchString(id) {
			t.Errorf("rejected the usable name %q", id)
		}
	}

	for _, id := range []string{
		"",
		"'; DROP DATABASE goliath; --",
		"/2026/09/07-041622.99'",
		"/2026/09/07-041622.99'; DROP DATABASE goliath; SELECT '",
		`/2026/09/07-041622.99\`,
		"/2026/09/07-041622.99;",
		"/2026/09/07-041622.99 ",
		"/2026/09/07-041622.99\n/etc",
	} {
		if checkpointIdSafe.MatchString(id) {
			t.Errorf("accepted %q", id)
		}
	}
}

// A name carrying a quote must never reach a statement, whatever else happens.
func TestRestoreCheckpointRejectsUnrecognizedNames(t *testing.T) {
	// The container name is deliberately unusable: a rejected name must fail
	// before anything is run, so nothing here should reach docker.
	err := restoreCheckpoint("", "x'; DROP DATABASE goliath CASCADE; SELECT '")
	if err == nil {
		t.Fatal("restoreCheckpoint accepted a name that is not a generated checkpoint")
	}
}

// The header row is present even when a query matches nothing, so an empty
// result must not be read as a row.
func TestParseCsvRowsDropsHeader(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want []string
	}{
		{"empty result", "path\n", nil},
		{"no output at all", "", nil},
		{"one row", "path\n/2026/09/07-041622.99\n", []string{"/2026/09/07-041622.99"}},
		{
			"several rows",
			"path\n/2026/09/07-041622.99\n/2026/09/06-120000.00\n",
			[]string{"/2026/09/07-041622.99", "/2026/09/06-120000.00"},
		},
		{
			"carriage returns are not part of the value",
			"path\r\n/2026/09/07-041622.99\r\n",
			[]string{"/2026/09/07-041622.99"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseCsvRows([]byte(tc.out))
			if len(got) != len(tc.want) {
				t.Fatalf("parseCsvRows(%q) = %v, want %v", tc.out, got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("row %d = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// Checkpoints are offered newest first, and their names are ordered by age only
// because the date is zero-padded and most-significant first.
func TestCheckpointNamesSortByAge(t *testing.T) {
	got := []string{
		"/2026/01/02-000000.00",
		"/2026/09/07-041622.99",
		"/2025/12/31-235959.00",
		"/2026/09/07-041622.10",
	}
	sort.Sort(sort.Reverse(sort.StringSlice(got)))

	want := []string{
		"/2026/09/07-041622.99",
		"/2026/09/07-041622.10",
		"/2026/01/02-000000.00",
		"/2025/12/31-235959.00",
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sorted = %v, want %v", got, want)
		}
	}
}
