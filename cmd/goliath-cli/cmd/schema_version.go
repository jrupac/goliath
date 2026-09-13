package cmd

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/jrupac/goliath/schema"
	"github.com/spf13/cobra"
)

// migrationsDir is where the migrations are read from, relative to the
// repository root every lifecycle command runs from.
//
// They are read from disk rather than from what was compiled into this binary,
// so that after a pull the files the checkout now has are the ones applied,
// whichever build of the CLI is doing it.
var migrationsDir = filepath.Join("backend", "schema")

func loadMigrations() ([]schema.Migration, error) {
	migrations, err := schema.Load(os.DirFS(migrationsDir))
	if err != nil {
		return nil, fmt.Errorf("reading migrations from %s: %w", migrationsDir, err)
	}
	if len(migrations) == 0 {
		return nil, fmt.Errorf("no migrations found in %s; run this from the repository root", migrationsDir)
	}
	return migrations, nil
}

// databaseName admits what --database may name. The name is interpolated into
// statements, so this is what keeps the flag from being a way to write SQL.
var databaseName = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

func addDatabaseFlag(cmd *cobra.Command) {
	cmd.Flags().String("database", schemaDatabase,
		"Database to act on; another name rehearses the command against a copy")
}

// useDatabase points the command at the database its --database flag names.
func useDatabase(cmd *cobra.Command) {
	db, _ := cmd.Flags().GetString("database")
	if !databaseName.MatchString(db) {
		fmt.Printf("Error: --database: not a usable database name: %q\n", db)
		os.Exit(1)
	}
	schemaDatabase = db
}

// setDatabase matches the statement a migration uses to name the database it
// acts on.
var setDatabase = regexp.MustCompile(`(?im)^[ \t]*SET[ \t]+DATABASE[ \t]+(?:TO|=)[ \t]+goliath[ \t]*;`)

// forDatabase points a migration at the database being acted on. A migration
// without the statement would act on whichever database the session happens
// to default to, and one with two is not saying anything clear, so both are
// refused rather than applied.
func forDatabase(sql, db string) (string, error) {
	if n := len(setDatabase.FindAllStringIndex(sql, -1)); n != 1 {
		return "", fmt.Errorf("expected exactly one `SET DATABASE TO Goliath;`, found %d", n)
	}
	return setDatabase.ReplaceAllLiteralString(sql, "SET DATABASE TO "+db+";"), nil
}

// parseVersion accepts a migration's file name, `vNN`, or a bare number.
func parseVersion(s string) (int, error) {
	s = strings.TrimSuffix(strings.TrimSpace(s), ".sql")
	s = strings.TrimPrefix(s, "v")
	if i := strings.IndexByte(s, '_'); i >= 0 {
		s = s[:i]
	}
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("not a migration version: %q", s)
	}
	return v, nil
}

// crdbRecords runs a query against the database container and returns its
// rows split into columns, with the header dropped.
func crdbRecords(dbContainer, query string) ([][]string, error) {
	cmd := exec.Command("docker", "exec", "-i", dbContainer,
		"./cockroach", "sql", "--insecure", "--format=csv")
	cmd.Stdin = strings.NewReader(query)
	cmd.Stderr = os.Stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseCsvRecords(out)
}

func parseCsvRecords(out []byte) ([][]string, error) {
	records, err := csv.NewReader(strings.NewReader(string(out))).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) <= 1 {
		return nil, nil
	}
	return records[1:], nil
}

// readApplied returns the migrations the database records having had, and
// whether it has the table that records them at all.
func readApplied(dbContainer string) ([]schema.Applied, bool, error) {
	exists, err := crdbQuery(dbContainer, fmt.Sprintf(
		`SELECT count(*) FROM %s.information_schema.tables
		 WHERE table_schema = 'public' AND table_name = 'schemaversion';`, schemaDatabase))
	if err != nil {
		return nil, false, fmt.Errorf("looking for the tracking table: %w", err)
	}
	if len(exists) != 1 || exists[0] == "0" {
		return nil, false, nil
	}

	records, err := crdbRecords(dbContainer, fmt.Sprintf(
		`SELECT version, name, breaks_older_binaries FROM %s.public.schemaversion
		 ORDER BY version;`, schemaDatabase))
	if err != nil {
		return nil, true, fmt.Errorf("reading the tracking table: %w", err)
	}
	applied, err := parseApplied(records)
	return applied, true, err
}

func parseApplied(records [][]string) ([]schema.Applied, error) {
	var applied []schema.Applied
	for _, r := range records {
		if len(r) != 3 {
			return nil, fmt.Errorf("unexpected tracking table row %q", r)
		}
		version, err := strconv.Atoi(r[0])
		if err != nil {
			return nil, fmt.Errorf("unexpected version in tracking table row %q", r)
		}
		breaks, err := strconv.ParseBool(r[2])
		if err != nil {
			return nil, fmt.Errorf("unexpected flag in tracking table row %q", r)
		}
		applied = append(applied, schema.Applied{
			Version: version, Name: r[1], BreaksOlderBinaries: breaks,
		})
	}
	return applied, nil
}

// Plan is what a run will do to a database.
type Plan struct {
	// From is the version the database is at.
	From    int
	Pending []schema.Migration
}

// makePlan decides what a run applies.
//
// A database that records nothing is refused, whether it lacks the tracking
// table or has an empty one: which migrations it has had is then a guess, and
// guessing is how a migration gets run twice.
func makePlan(migrations []schema.Migration, applied []schema.Applied, versioned bool, target int) (Plan, error) {
	if !versioned {
		return Plan{}, fmt.Errorf("%s has no SchemaVersion table, so which migrations it has had "+
			"is unknown; check that --database names the right one", schemaDatabase)
	}
	if len(applied) == 0 {
		// The table exists but is empty: something went wrong, and guessing
		// where the database stands is how a migration gets run twice.
		return Plan{}, errors.New("the SchemaVersion table is empty; record the version by hand " +
			"before migrating")
	}
	pending, err := planPending(migrations, applied, target)
	if err != nil {
		return Plan{}, err
	}
	return Plan{From: schema.Current(applied), Pending: pending}, nil
}

// planPending returns the migrations that take a database from what it has
// had to target, in order, or to the newest in the checkout if target is 0.
//
// It refuses rather than guesses whenever the checkout and the database
// disagree about history: a database ahead of the checkout and a gap in the
// numbering are each more likely a mistake than something to apply around.
func planPending(migrations []schema.Migration, applied []schema.Applied, target int) ([]schema.Migration, error) {
	current := schema.Current(applied)
	latest := migrations[len(migrations)-1].Version
	if current > latest {
		return nil, fmt.Errorf("the database is at v%d, newer than this checkout's newest "+
			"migration v%d; the checkout is behind", current, latest)
	}
	if target == 0 {
		target = latest
	}
	if target > latest {
		return nil, fmt.Errorf("no migration v%d in this checkout; the newest is v%d", target, latest)
	}

	var pending []schema.Migration
	expected := current + 1
	for _, m := range migrations {
		if m.Version <= current || m.Version > target {
			continue
		}
		if m.Version != expected {
			return nil, fmt.Errorf("the database is at v%d but there is no migration v%d "+
				"before %s", expected-1, expected, m.Name)
		}
		pending = append(pending, m)
		expected++
	}
	return pending, nil
}

// safeRecordName admits what the tracking table's name column is given: a
// migration's file name, which schema.Load has already matched. It is a
// backstop for interpolation, as checkpointIdSafe is.
var safeRecordName = regexp.MustCompile(`^[a-z0-9_.]+$`)

// recordStatement is the statement that records a migration as applied.
func recordStatement(version int, name string, breaks bool) (string, error) {
	if !safeRecordName.MatchString(name) {
		return "", fmt.Errorf("refusing to record unusable migration name %q", name)
	}
	return fmt.Sprintf(
		"INSERT INTO %s.public.schemaversion (version, name, breaks_older_binaries) VALUES (%d, '%s', %t);",
		schemaDatabase, version, name, breaks), nil
}

// apply applies a migration to the database being acted on and records it in
// the same session, so that the row is written only if every statement before
// it succeeded: the SQL shell stops at the first error when reading from a
// pipe.
func apply(dbContainer string, m schema.Migration) error {
	sql, err := forDatabase(m.SQL, schemaDatabase)
	if err != nil {
		return fmt.Errorf("%s: %w", m.Name, err)
	}
	stmt, err := recordStatement(m.Version, m.Name, m.Compat.BreaksOlderBinaries())
	if err != nil {
		return err
	}
	return crdbExec(dbContainer, sql+"\n"+stmt+"\n")
}

func describePending(pending []schema.Migration) string {
	var b strings.Builder
	for _, m := range pending {
		effect := "older binaries keep running"
		if m.Compat.BreaksOlderBinaries() {
			effect = "older binaries cannot run on it"
		}
		fmt.Fprintf(&b, "  %s (%s)\n", m.Name, effect)
	}
	return b.String()
}
