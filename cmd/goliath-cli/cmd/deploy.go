package cmd

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jrupac/goliath/schema"
)

// deployLogPath is where upgrade and rollback record what they deployed,
// relative to the repository root. It lives on the host rather than in the
// database because a full rollback restores the database to before the
// upgrade it undoes, which would take the record of that upgrade with it.
var deployLogPath = filepath.Join(".goliath", "deploys.jsonl")

const (
	deployUpgrade  = "upgrade"
	deployRollback = "rollback"
)

// deployRecord is one line of the deploy log.
type deployRecord struct {
	Kind     string    `json:"kind"`
	Time     time.Time `json:"time"`
	Env      string    `json:"env"`
	Database string    `json:"database"`
	// Hash and Image are what was started, PreviousHash and PreviousImage what
	// was running before. A hash is empty where it could not be learned.
	Hash          string `json:"hash"`
	Image         string `json:"image"`
	PreviousHash  string `json:"previous_hash"`
	PreviousImage string `json:"previous_image"`
	SchemaFrom    int    `json:"schema_from"`
	SchemaTo      int    `json:"schema_to"`
	// Checkpoint was taken before the change, with the application stopped.
	Checkpoint string `json:"checkpoint"`
}

func appendDeployRecord(r deployRecord) error {
	if err := os.MkdirAll(filepath.Dir(deployLogPath), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(deployLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	line, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = f.Write(append(line, '\n'))
	return err
}

// lastDeployRecord returns the most recent record for an environment and
// database.
func lastDeployRecord(env, database string) (deployRecord, bool, error) {
	f, err := os.Open(deployLogPath)
	if errors.Is(err, os.ErrNotExist) {
		return deployRecord{}, false, nil
	}
	if err != nil {
		return deployRecord{}, false, err
	}
	defer f.Close()
	return lastRecordIn(f, env, database)
}

func lastRecordIn(r io.Reader, env, database string) (deployRecord, bool, error) {
	var last deployRecord
	found := false
	scanner := bufio.NewScanner(r)
	for n := 1; scanner.Scan(); n++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var rec deployRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			return deployRecord{}, false, fmt.Errorf("%s line %d: %w", deployLogPath, n, err)
		}
		if rec.Env == env && rec.Database == database {
			last, found = rec, true
		}
	}
	return last, found, scanner.Err()
}

// commandOutput runs a command and returns its trimmed output. What it writes
// to stderr is kept out of the terminal and returned with a failure, since
// compose warns about unset variables on every call.
func commandOutput(name string, args ...string) (string, error) {
	var stderr bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err,
			strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(out)), nil
}

// runCommand runs a command with its output going to the terminal.
func runCommand(env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), env...)
	return cmd.Run()
}

// serviceImage is the repository compose tags a service's built image with:
// the service's own image name if it sets one, otherwise one made from the
// project and service names.
func serviceImage(env, service string) (string, error) {
	out, err := commandOutput("docker", "compose", "--profile", env, "config", "--format", "json")
	if err != nil {
		return "", err
	}
	var cfg struct {
		Name     string `json:"name"`
		Services map[string]struct {
			Image string `json:"image"`
		} `json:"services"`
	}
	if err = json.Unmarshal([]byte(out), &cfg); err != nil {
		return "", fmt.Errorf("reading the compose configuration: %w", err)
	}
	s, ok := cfg.Services[service]
	if !ok {
		return "", fmt.Errorf("no service %s in the %s profile", service, env)
	}
	image := s.Image
	if image == "" {
		image = cfg.Name + "-" + service
	}
	// Tags are dropped: upgrade and rollback manage their own.
	if i := strings.LastIndex(image, ":"); i > strings.LastIndex(image, "/") {
		image = image[:i]
	}
	return image, nil
}

// runningImage is the ID of the image the service's container was created
// from, running or not, or empty if it has no container.
func runningImage(env, service string) (string, error) {
	ids, err := commandOutput("docker", "compose", "--profile", env, "ps", "--all", "--quiet", service)
	if err != nil || ids == "" {
		return "", err
	}
	id, _, _ := strings.Cut(ids, "\n")
	return commandOutput("docker", "inspect", "--format", "{{.Image}}", id)
}

func imageID(ref string) (string, error) {
	return commandOutput("docker", "image", "inspect", "--format", "{{.Id}}", ref)
}

func tagImage(id, ref string) error {
	_, err := commandOutput("docker", "tag", id, ref)
	return err
}

// deployHash names what is being built: the commit, marked when tracked files
// differ from it, since then the build is not that commit.
func deployHash() (string, error) {
	head, err := commandOutput("git", "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	status, err := commandOutput("git", "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return "", err
	}
	if status != "" {
		head += "-dirty"
	}
	return head, nil
}

// versionInfo is what the application reports on /version. The schema fields
// are absent from binaries that predate them.
type versionInfo struct {
	BuildHash       string `json:"build_hash"`
	SchemaVersion   *int   `json:"schema_version"`
	DBSchemaVersion *int   `json:"db_schema_version"`
}

func fetchVersion(url string) (versionInfo, error) {
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return versionInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return versionInfo{}, fmt.Errorf("%s answered %s", url, resp.Status)
	}
	var v versionInfo
	if err = json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return versionInfo{}, fmt.Errorf("reading %s: %w", url, err)
	}
	return v, nil
}

// expectedVersion is what the application should report once a deploy is up.
// A zero field accepts anything.
type expectedVersion struct {
	Hash     string
	Schema   int
	DBSchema int
}

func (e expectedVersion) check(v versionInfo) error {
	if e.Hash != "" && v.BuildHash != e.Hash {
		return fmt.Errorf("it reports build %q, not %q", v.BuildHash, e.Hash)
	}
	if e.Schema != 0 && (v.SchemaVersion == nil || *v.SchemaVersion != e.Schema) {
		return fmt.Errorf("it reports being built for schema %s, not v%d", describeVersion(v.SchemaVersion), e.Schema)
	}
	if e.DBSchema != 0 && (v.DBSchemaVersion == nil || *v.DBSchemaVersion != e.DBSchema) {
		return fmt.Errorf("it reports the database at schema %s, not v%d", describeVersion(v.DBSchemaVersion), e.DBSchema)
	}
	return nil
}

func describeVersion(v *int) string {
	if v == nil {
		return "(none)"
	}
	return fmt.Sprintf("v%d", *v)
}

// waitForVersion polls /version until it reports what is expected. A wrong
// answer is polled past like no answer, since the container being replaced
// can answer for a moment before its successor does.
func waitForVersion(url string, timeout time.Duration, want expectedVersion) error {
	deadline := time.Now().Add(timeout)
	for {
		v, err := fetchVersion(url)
		if err == nil {
			if err = want.check(v); err == nil {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return err
		}
		time.Sleep(time.Second)
	}
}

// nodelocalRoot is where the node keeps what nodelocal://1/ names.
const nodelocalRoot = "/cockroach/cockroach-data/extern"

// checkpointPath is where the node keeps a checkpoint.
func checkpointPath(name string) (string, error) {
	if !checkpointIdSafe.MatchString(name) || !strings.HasPrefix(name, "/") || strings.Contains(name, "..") {
		return "", fmt.Errorf("refusing to locate unusable checkpoint name %q", name)
	}
	return nodelocalRoot + strings.TrimPrefix(checkpointCollection, "nodelocal://1") + name, nil
}

// checkpointDatabase returns the database a checkpoint holds.
func checkpointDatabase(dbContainer, name string) (string, error) {
	if !checkpointIdSafe.MatchString(name) {
		return "", fmt.Errorf("refusing to inspect unusable checkpoint name %q", name)
	}
	rows, err := crdbQuery(dbContainer, fmt.Sprintf(
		`SELECT DISTINCT database_name FROM [SHOW BACKUP FROM '%s' IN '%s'] WHERE database_name IS NOT NULL;`,
		name, checkpointCollection))
	if err != nil {
		return "", err
	}
	if len(rows) != 1 {
		return "", fmt.Errorf("checkpoint %s holds %d databases", name, len(rows))
	}
	return rows[0], nil
}

// pruneCheckpoints deletes every checkpoint of the database being acted on
// except keep, and returns what it deleted. Checkpoints of other databases
// share the collection and are left alone.
//
// CockroachDB has no statement that deletes a backup, so a checkpoint is
// removed from where the node keeps nodelocal storage. Only names the database
// itself listed are removed, and each is checked again before it becomes part
// of a path.
func pruneCheckpoints(dbContainer, keep string) ([]string, error) {
	names, err := listCheckpoints(dbContainer)
	if err != nil {
		return nil, err
	}
	var deleted []string
	for _, name := range names {
		if name == keep {
			continue
		}
		db, err := checkpointDatabase(dbContainer, name)
		if err != nil {
			return deleted, err
		}
		if db != schemaDatabase {
			continue
		}
		path, err := checkpointPath(name)
		if err != nil {
			return deleted, err
		}
		if _, err = commandOutput("docker", "exec", dbContainer, "rm", "-rf", "--", path); err != nil {
			return deleted, err
		}
		deleted = append(deleted, name)
	}
	return deleted, nil
}

// pruneImages removes the service's images that no tag refers to any more,
// which is what each build leaves of the one before. Compose labels an image
// with its project and service, which keeps this from touching anything else
// on the host.
func pruneImages(service, repo string) error {
	project, err := commandOutput("docker", "image", "inspect", "--format",
		`{{index .Config.Labels "com.docker.compose.project"}}`, repo+":latest")
	if err != nil {
		return err
	}
	if project == "" {
		return fmt.Errorf("%s:latest carries no compose project label", repo)
	}
	return runCommand(nil, "docker", "image", "prune", "--force",
		"--filter", "label=com.docker.compose.project="+project,
		"--filter", "label=com.docker.compose.service="+service)
}

// chooseRollback decides whether rolling back an upgrade has to restore the
// database too, and why. The binary being returned to was built for the
// schema the upgrade started from, so the question is the one that binary
// would ask of the database at startup.
func chooseRollback(rec deployRecord, applied []schema.Applied, forceFull bool) (bool, string) {
	if forceFull {
		return true, "asked for with --full"
	}
	if err := schema.Check(rec.SchemaFrom, applied); err != nil {
		return true, err.Error()
	}
	return false, ""
}

func fileDigest(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

// installedCLIDiffers reports whether the goliath-cli on the PATH is missing
// or differs from the one running, and where it is.
func installedCLIDiffers() (string, bool) {
	self, err := os.Executable()
	if err != nil {
		return "", false
	}
	installed, err := exec.LookPath("goliath-cli")
	if err != nil {
		return "", true
	}
	a, errA := fileDigest(self)
	b, errB := fileDigest(installed)
	if errA != nil || errB != nil {
		return installed, false
	}
	return installed, !bytes.Equal(a, b)
}

// confirmOrExit asks for a word to be typed before going on, unless told not
// to ask.
func confirmOrExit(yes bool, word string) {
	if yes {
		return
	}
	if promptForInput(fmt.Sprintf("Type '%s' to proceed:", word)) != word {
		fmt.Println("Aborted; nothing changed.")
		os.Exit(1)
	}
	fmt.Println()
}

// databaseFlagSuffix is what a printed command needs to act on the same
// database as this one.
func databaseFlagSuffix() string {
	if schemaDatabase == "goliath" {
		return ""
	}
	return " --database " + schemaDatabase
}
