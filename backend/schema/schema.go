// Package schema holds the database migrations and decides whether a binary
// can run against a database, given the migrations that database has had.
//
// A database records what it has had in the SchemaVersion table, one row per
// migration, written by goliath-cli after the migration succeeds. The
// migrations themselves do not write it: the version is already in the file
// name, and a row each file had to remember to insert would sooner or later be
// forgotten or copied with the wrong number.
package schema

import (
	"bufio"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

//go:embed *.sql
var embedded embed.FS

// Compat is what a migration does to binaries built before it.
type Compat int

const (
	// CompatUnknown is a migration that does not say. It is treated as
	// breaking, since the cost of being wrong the other way is a binary
	// running against a schema it cannot use.
	CompatUnknown Compat = iota
	// CompatOlderBinariesRun is a migration binaries built before it keep
	// running on, such as one adding a column they never read.
	CompatOlderBinariesRun
	// CompatOlderBinariesBreak is a migration binaries built before it cannot
	// run on, such as one dropping or renaming something they use.
	CompatOlderBinariesBreak
)

// BreaksOlderBinaries reports whether a binary built before the migration must
// refuse to run once it is applied.
func (c Compat) BreaksOlderBinaries() bool {
	return c != CompatOlderBinariesRun
}

// Migration is one numbered migration file.
type Migration struct {
	Version int
	// Name is the file name, which is also what the tracking table records.
	Name   string
	Compat Compat
	SQL    string
}

// firstAnnotated is the first version whose file must declare what it does to
// older binaries. Earlier ones predate the declaration, and are never applied
// one by one: every database that records its version is already past them.
const firstAnnotated = 26

// compatHeader is the declaration a migration carries in its leading comment
// block, e.g. `-- older-binaries: compatible`.
const compatHeader = "-- older-binaries:"

var migrationName = regexp.MustCompile(`^v([0-9]+)(_[a-z0-9_]+)?\.sql$`)

// Load reads the migrations in a directory, ordered by version.
//
// It fails on anything that would make the order ambiguous or a declaration
// unreadable, so that a mistake in a new file stops a release before it starts
// rather than halfway through one.
func Load(fsys fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}

	var migrations []Migration
	seen := map[int]string{}
	for _, e := range entries {
		m := migrationName.FindStringSubmatch(e.Name())
		if e.IsDir() || m == nil {
			continue
		}
		version, err := strconv.Atoi(m[1])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		if other, ok := seen[version]; ok {
			return nil, fmt.Errorf("%s and %s share version %d", other, e.Name(), version)
		}
		seen[version] = e.Name()

		content, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return nil, err
		}
		compat, err := parseCompat(string(content))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		if compat == CompatUnknown && version >= firstAnnotated {
			return nil, fmt.Errorf("%s: missing %q declaration in its leading comments",
				e.Name(), compatHeader+" compatible|incompatible")
		}

		migrations = append(migrations, Migration{
			Version: version,
			Name:    e.Name(),
			Compat:  compat,
			SQL:     string(content),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})
	return migrations, nil
}

// parseCompat reads the declaration from a file's leading comment block, the
// run of comment and blank lines before the first statement.
func parseCompat(content string) (Compat, error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "--") {
			break
		}
		value, ok := strings.CutPrefix(line, compatHeader)
		if !ok {
			continue
		}
		switch strings.TrimSpace(value) {
		case "compatible":
			return CompatOlderBinariesRun, nil
		case "incompatible":
			return CompatOlderBinariesBreak, nil
		default:
			return CompatUnknown, fmt.Errorf("unrecognized declaration %q", line)
		}
	}
	return CompatUnknown, scanner.Err()
}

var (
	embeddedMigrations []Migration
	embeddedErr        error
)

func init() {
	embeddedMigrations, embeddedErr = Load(embedded)
}

// Latest is the newest migration this binary was built with, and so the
// oldest schema it can run against.
//
// Every migration in the tree is taken to be one the code beside it depends
// on. A binary able to run on a schema older than its own tree would need code
// written to work both with and without a migration, and would run the one of
// those paths nobody tested.
func Latest() int {
	if embeddedErr != nil {
		// The same files are loaded by a unit test, so this is unreachable in
		// anything that passed its tests.
		panic(fmt.Sprintf("schema: reading embedded migrations: %s", embeddedErr))
	}
	if len(embeddedMigrations) == 0 {
		return 0
	}
	return embeddedMigrations[len(embeddedMigrations)-1].Version
}

// Applied is a row of the tracking table.
type Applied struct {
	Version             int
	Name                string
	BreaksOlderBinaries bool
}

// ErrUnversioned is a database with no record of its migrations.
var ErrUnversioned = errors.New(
	"database records no schema version, so which migrations it has had is unknown; " +
		"check that the database named is the right one")

// Current is the version a database is at: the newest migration it has had.
func Current(applied []Applied) int {
	current := 0
	for _, a := range applied {
		current = max(current, a.Version)
	}
	return current
}

// Check decides whether a binary built with migrations up to binary can run
// against a database that has had applied.
//
// A database behind the binary is refused: the code depends on every migration
// in its tree. A database ahead of it is accepted unless one of the migrations
// the binary has never seen declares that it breaks older binaries, which is
// what makes it possible to roll back a binary without rolling back the data.
func Check(binary int, applied []Applied) error {
	if len(applied) == 0 {
		return ErrUnversioned
	}

	current := Current(applied)
	if current < binary {
		return fmt.Errorf(
			"database is at schema v%d but this binary needs v%d; "+
				"apply the pending migrations with `goliath-cli upgrade`", current, binary)
	}

	var breaking []string
	for _, a := range applied {
		if a.Version > binary && a.BreaksOlderBinaries {
			breaking = append(breaking, a.Name)
		}
	}
	if len(breaking) > 0 {
		sort.Strings(breaking)
		return fmt.Errorf(
			"database is at schema v%d, past this binary's v%d, and %s cannot "+
				"run on binaries built before %s", current, binary,
			strings.Join(breaking, ", "), pluralize(len(breaking), "it", "them"))
	}
	return nil
}

func pluralize(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
