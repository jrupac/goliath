package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

const (
	// checkpointCollection is the backup collection every checkpoint is written
	// into.
	//
	// A single shared collection is what keeps this readable over SQL. `SHOW
	// BACKUPS IN` enumerates one collection, while the `nodelocal://1/` root
	// above it enumerates as empty, so a collection per migration could only be
	// listed by reaching into the database container's filesystem. That path is
	// an implementation detail of where a node puts its external storage,
	// whereas the URI scheme is a documented interface.
	checkpointCollection = "nodelocal://1/goliath-checkpoints"

	// schemaDatabase is the database the migrations in backend/schema target.
	schemaDatabase = "goliath"

	// checkpointAppUser is the SQL user the backend connects as. RESTORE
	// creates the database owned by root and carries over none of the original
	// database's grants, so this has to be granted again afterwards. Without
	// it the backend starts, connects successfully, and then dies on its first
	// query complaining about a missing SELECT privilege — which reads like a
	// corrupt restore but is only a missing grant.
	checkpointAppUser = "goliath"
)

// checkpointIdSafe admits the characters a collection path is built from.
//
// This is deliberately not a description of how CockroachDB names a backup.
// That naming is documented by example rather than guaranteed, so pinning its
// exact shape here would turn any future change to it into a refusal to
// restore anything. What has to hold of a checkpoint name is only that it
// cannot end the string literal it is placed in; which names exist is settled
// by asking the database, not by matching a pattern.
var checkpointIdSafe = regexp.MustCompile(`^[0-9A-Za-z/_.\-]+$`)

// Statements are assembled by interpolation, because the SQL shell reached
// through `docker exec` has no way to bind parameters. Everything interpolated
// is therefore either a compile-time constant or a checkpoint ID, and a
// checkpoint ID is only ever a value the database itself just listed —
// resolveCheckpoint turns what was asked for into one of those or into
// nothing. Nothing from the command line reaches a statement as text.

// crdbExec runs statements against the database container, streaming their
// output to the terminal.
func crdbExec(dbContainer, query string) error {
	cmd := exec.Command("docker", "exec", "-i", dbContainer, "./cockroach", "sql", "--insecure")
	cmd.Stdin = strings.NewReader(query)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// crdbQuery runs a query against the database container and returns its rows,
// one string per row, with the header dropped.
func crdbQuery(dbContainer, query string) ([]string, error) {
	cmd := exec.Command("docker", "exec", "-i", dbContainer,
		"./cockroach", "sql", "--insecure", "--format=csv")
	cmd.Stdin = strings.NewReader(query)
	cmd.Stderr = os.Stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseCsvRows(out), nil
}

// parseCsvRows turns the SQL shell's CSV output into one string per row.
//
// The queries here select a single column, so a row is its whole value. The
// header is always present, even for an empty result, and is dropped.
func parseCsvRows(out []byte) []string {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) <= 1 {
		return nil
	}

	var rows []string
	for _, line := range lines[1:] {
		if row := strings.TrimSpace(line); row != "" {
			rows = append(rows, row)
		}
	}
	return rows
}

// listCheckpoints returns the checkpoints the database holds, newest first.
// Their names begin with a zero-padded date, so ordering them lexically orders
// them by age.
//
// A collection that has never been written to lists as empty rather than as an
// error, so a first run needs no special case.
func listCheckpoints(dbContainer string) ([]string, error) {
	rows, err := crdbQuery(dbContainer, fmt.Sprintf("SHOW BACKUPS IN '%s';", checkpointCollection))
	if err != nil {
		return nil, err
	}
	sort.Sort(sort.Reverse(sort.StringSlice(rows)))
	return rows, nil
}

// resolveCheckpoint matches what was asked for against what the database holds,
// returning the database's own spelling of it.
//
// Selecting from that list, rather than checking the request for dangerous
// characters, is what keeps a checkpoint ID from being a way to write SQL: the
// value that reaches a statement was produced by the database, and the request
// is only ever compared for equality.
func resolveCheckpoint(dbContainer, requested string) (string, bool, error) {
	names, err := listCheckpoints(dbContainer)
	if err != nil {
		return "", false, err
	}
	for _, name := range names {
		if name == requested {
			return name, true, nil
		}
	}
	return "", false, nil
}

// createCheckpoint backs the schema database up and returns the checkpoint's
// name.
//
// Callers stop the application first, so there is no concurrent writer and the
// backup needs no AS OF SYSTEM TIME. Taking it as of now rather than a moment
// in the past is what makes a rollback restore everything written right up to
// the shutdown.
func createCheckpoint(dbContainer string) (string, error) {
	// The backup statement reports a job, not a location, so the new
	// checkpoint is identified by what the collection gained.
	before, err := listCheckpoints(dbContainer)
	if err != nil {
		return "", fmt.Errorf("listing existing checkpoints: %w", err)
	}
	seen := make(map[string]bool, len(before))
	for _, name := range before {
		seen[name] = true
	}

	if err = crdbExec(dbContainer, fmt.Sprintf(
		"BACKUP DATABASE %s INTO '%s';", schemaDatabase, checkpointCollection)); err != nil {
		return "", err
	}

	after, err := listCheckpoints(dbContainer)
	if err != nil {
		return "", fmt.Errorf("locating the new checkpoint: %w", err)
	}
	for _, name := range after {
		if !seen[name] {
			return name, nil
		}
	}
	return "", errors.New("backup reported success but added no checkpoint")
}

// restoreCheckpoint replaces the schema database with the named checkpoint.
//
// The statements are issued separately because RESTORE cannot run inside a
// transaction, and a multi-statement batch would put it in one.
func restoreCheckpoint(dbContainer, id string) error {
	// A backstop: callers pass a name the database listed, so a value failing
	// here means that guarantee was lost somewhere rather than that a user
	// mistyped something.
	if !checkpointIdSafe.MatchString(id) {
		return fmt.Errorf("refusing to restore from unusable checkpoint name %q", id)
	}

	if err := crdbExec(dbContainer, fmt.Sprintf(
		"DROP DATABASE IF EXISTS %s CASCADE;", schemaDatabase)); err != nil {
		return fmt.Errorf("dropping the existing database: %w", err)
	}

	if err := crdbExec(dbContainer, fmt.Sprintf(
		"RESTORE DATABASE %s FROM '%s' IN '%s';",
		schemaDatabase, id, checkpointCollection)); err != nil {
		return fmt.Errorf("restoring the checkpoint: %w", err)
	}

	return crdbExec(dbContainer, fmt.Sprintf(`
		GRANT ALL ON DATABASE %[1]s TO %[2]s;
		GRANT ALL ON SCHEMA %[1]s.public TO %[2]s;
		GRANT ALL ON ALL TABLES IN SCHEMA %[1]s.public TO %[2]s;`,
		schemaDatabase, checkpointAppUser))
}

// printAvailableCheckpoints lists the checkpoints the database holds, each with
// the command that restores it.
func printAvailableCheckpoints(env, dbContainer string) {
	names, err := listCheckpoints(dbContainer)
	if err != nil {
		fmt.Printf("Could not list checkpoints on %s: %v\n", dbContainer, err)
		return
	}
	if len(names) == 0 {
		fmt.Println("No checkpoints found. One is taken automatically by migrate-schema.")
		return
	}

	fmt.Println("Available checkpoints, newest first:")
	for _, name := range names {
		fmt.Printf("\n  %s\n", name)
		fmt.Printf("    goliath-cli rollback-schema --env %s --checkpoint %s\n", env, name)
	}
}
