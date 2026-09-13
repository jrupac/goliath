package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// upTimeout bounds how long a started application has to report the expected
// build and schema on /version.
const upTimeout = 2 * time.Minute

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Pull, migrate, rebuild and restart, keeping a way back",
	Long: `Bring a deployment up to date with its upstream branch, in one run:

1. Fast-forward the checkout (refused while tracked files are modified)
2. Build goliath-cli from the new checkout and continue in it, so that every
   later step is done by the code just pulled
3. Build the new application image while the old one keeps running
4. Stop the application, checkpoint the database, apply pending migrations
5. Start the new image, and wait for /version to report its build and schema
6. Prune what earlier deploys left: images no tag refers to, and this
   database's older checkpoints

The image that was running stays tagged :previous, and with the checkpoint is
what 'goliath-cli rollback' returns to. One deploy back is kept.

Examples:
  goliath-cli upgrade
  goliath-cli upgrade --env dev --database upgrade_test --no-pull`,
	GroupID: "lifecycle",
	Run: func(cmd *cobra.Command, args []string) {
		o := upgradeOptions{}
		o.env, _ = cmd.Flags().GetString("env")
		o.noPull, _ = cmd.Flags().GetBool("no-pull")
		o.yes, _ = cmd.Flags().GetBool("yes")
		o.pruneBuildCache, _ = cmd.Flags().GetBool("prune-build-cache")
		o.versionURL, _ = cmd.Flags().GetString("version-url")
		o.resumedFrom, _ = cmd.Flags().GetString("resumed-from")
		useDatabase(cmd)
		getServiceNames(o.env)

		if o.resumedFrom == "" {
			prepareUpgrade(o.noPull)
			return
		}
		runUpgrade(o)
	},
}

type upgradeOptions struct {
	env             string
	noPull          bool
	yes             bool
	pruneBuildCache bool
	versionURL      string
	resumedFrom     string
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
	addEnvFlag(upgradeCmd)
	addDatabaseFlag(upgradeCmd)
	upgradeCmd.Flags().Bool("no-pull", false, "Deploy the checkout as it is, without fetching")
	upgradeCmd.Flags().Bool("yes", false, "Skip the confirmation prompt")
	upgradeCmd.Flags().Bool("prune-build-cache", false, "Also prune Docker's build cache, which is shared by everything on the host")
	upgradeCmd.Flags().String("version-url", "http://127.0.0.1:9999/version", "Where the application reports its build and schema")
	// Set by the first stage when it continues in the freshly built CLI, to
	// the commit the checkout was at before pulling.
	upgradeCmd.Flags().String("resumed-from", "", "")
	_ = upgradeCmd.Flags().MarkHidden("resumed-from")
}

func exitWith(format string, args ...any) {
	fmt.Printf("Error: "+format+"\n", args...)
	os.Exit(1)
}

// prepareUpgrade updates the checkout, builds the CLI from it, and continues
// the upgrade in that CLI. Doing the rest in the new build is what lets a
// release change how it is itself deployed.
func prepareUpgrade(noPull bool) {
	head, err := commandOutput("git", "rev-parse", "HEAD")
	if err != nil {
		exitWith("%v; run this from the repository root", err)
	}

	if noPull {
		fmt.Println("Not pulling (--no-pull); deploying the checkout as it is.")
	} else {
		pullCheckout()
	}

	fmt.Println("Building goliath-cli from the checkout...")
	if err = runCommand(nil, "docker", "build", "--target", "cli", "--output", "type=local,dest=./dist", "."); err != nil {
		exitWith("building goliath-cli: %v. The checkout is updated; nothing else has changed.", err)
	}
	exe, err := filepath.Abs(filepath.Join("dist", "goliath-cli"))
	if err != nil {
		exitWith("%v", err)
	}

	args := append([]string{exe}, os.Args[1:]...)
	args = append(args, "--resumed-from="+head)
	fmt.Printf("Continuing in %s\n\n", exe)
	err = syscall.Exec(exe, args, os.Environ())
	// Exec returns only if it failed.
	exitWith("continuing in %s: %v", exe, err)
}

// pullCheckout fast-forwards the checkout to its upstream.
func pullCheckout() {
	status, err := commandOutput("git", "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		exitWith("%v", err)
	}
	if status != "" {
		fmt.Println("Tracked files are modified:")
		fmt.Println(status)
		exitWith("refusing to pull over local changes; commit, stash or revert them first")
	}
	upstream, err := commandOutput("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		exitWith("the current branch has no upstream to pull from: %v", err)
	}

	fmt.Printf("Fetching %s...\n", upstream)
	if err = runCommand(nil, "git", "fetch", "--quiet"); err != nil {
		exitWith("fetching: %v", err)
	}
	incoming, err := commandOutput("git", "log", "--oneline", "--no-decorate", "HEAD..@{u}")
	if err != nil {
		exitWith("%v", err)
	}
	if incoming == "" {
		fmt.Printf("Already up to date with %s.\n", upstream)
		return
	}
	if err = runCommand(nil, "git", "merge", "--ff-only", "--quiet", "@{u}"); err != nil {
		exitWith("the checkout cannot be fast-forwarded to %s: %v", upstream, err)
	}
}

func runUpgrade(o upgradeOptions) {
	appService, dbService, dbContainer := getServiceNames(o.env)

	hash, err := deployHash()
	if err != nil {
		exitWith("%v", err)
	}
	if pulled, err := commandOutput("git", "log", "--oneline", "--no-decorate", o.resumedFrom+"..HEAD"); err == nil && pulled != "" {
		fmt.Printf("Pulled:\n%s\n\n", pulled)
	}

	migrations, err := loadMigrations()
	if err != nil {
		exitWith("%v", err)
	}
	latest := migrations[len(migrations)-1].Version

	fmt.Printf("Ensuring %s is running...\n", dbService)
	startDatabase(o.env, dbService, dbContainer)

	applied, versioned, err := readApplied(dbContainer)
	if err != nil {
		exitWith("%v", err)
	}
	plan, err := makePlan(migrations, applied, versioned, 0)
	if err != nil {
		exitWith("%v", err)
	}

	repo, err := serviceImage(o.env, appService)
	if err != nil {
		exitWith("%v", err)
	}
	previousImage, err := runningImage(o.env, appService)
	if err != nil {
		exitWith("%v", err)
	}
	previousHash := ""
	if v, err := fetchVersion(o.versionURL); err == nil {
		previousHash = v.BuildHash
	}

	keepsData := previousImage != ""
	for _, m := range plan.Pending {
		keepsData = keepsData && !m.Compat.BreaksOlderBinaries()
	}

	fmt.Printf("\nUpgrading %s (environment: %s, database: %s):\n", appService, o.env, schemaDatabase)
	fmt.Printf("  build:  %s -> %s\n", orUnknown(previousHash), hash)
	fmt.Printf("  schema: v%d -> v%d\n", plan.From, latest)
	if len(plan.Pending) > 0 {
		fmt.Print(describePending(plan.Pending))
	}
	fmt.Println()
	switch {
	case previousImage == "":
		fmt.Println("No container exists for the application, so there is no image to roll back to.")
	case keepsData:
		fmt.Println("Rolling back afterwards can keep the data: every migration here lets older")
		fmt.Println("binaries run.")
	default:
		fmt.Println("Rolling back afterwards means restoring the checkpoint, and losing what is")
		fmt.Println("written after it.")
	}
	fmt.Println("The image is built first; the application is down from the checkpoint until")
	fmt.Println("the new image is up.")
	fmt.Println()
	confirmOrExit(o.yes, "upgrade")

	const steps = 7
	step := 0
	next := func(format string, args ...any) {
		step++
		fmt.Printf("[%d/%d] %s\n", step, steps, fmt.Sprintf(format, args...))
	}

	next("Building %s at %s...", repo, hash)
	if previousImage != "" {
		// Tagged before building, so that the build cannot leave it untagged
		// and in the way of the pruning at the end.
		if err = tagImage(previousImage, repo+":previous"); err != nil {
			exitWith("tagging the running image: %v", err)
		}
	}
	buildEnv := []string{"BUILD_HASH=" + hash, "BUILD_TIMESTAMP=" + strconv.FormatInt(time.Now().Unix(), 10)}
	if err = runCommand(buildEnv, "docker", "compose", "--profile", o.env, "build", appService); err != nil {
		exitWith("building the image: %v. Nothing else has changed; the application is still running.", err)
	}
	newImage, err := imageID(repo + ":latest")
	if err != nil {
		exitWith("%v", err)
	}

	next("Stopping %s service...", appService)
	stopService(o.env, appService)

	next("Checkpointing %s...", schemaDatabase)
	checkpoint, err := createCheckpoint(dbContainer)
	if err != nil {
		fmt.Printf("Error creating checkpoint: %v\n", err)
		fmt.Println("Nothing has been migrated, but the new image is built and tagged latest.")
		fmt.Printf("Start the previous one again with: docker tag %s %s:latest && goliath-cli up --env %s\n",
			previousImage, repo, o.env)
		os.Exit(1)
	}
	fmt.Printf("        Checkpointed as %s\n", checkpoint)

	// Recorded before anything is changed, so that every failure from here on
	// can be undone by rolling back.
	if err = appendDeployRecord(deployRecord{
		Kind: deployUpgrade, Time: time.Now().UTC(), Env: o.env, Database: schemaDatabase,
		Hash: hash, Image: newImage, PreviousHash: previousHash, PreviousImage: previousImage,
		SchemaFrom: plan.From, SchemaTo: latest, Checkpoint: checkpoint,
	}); err != nil {
		fmt.Printf("Error recording the deploy in %s: %v\n", deployLogPath, err)
		fmt.Println("Nothing has been migrated. Start the application again with:")
		fmt.Printf("  goliath-cli up --env %s\n", o.env)
		os.Exit(1)
	}

	if len(plan.Pending) == 0 {
		next("No migrations to apply.")
	} else {
		next("Applying %d migration(s)...", len(plan.Pending))
	}
	if err = applyPlan(dbContainer, plan); err != nil {
		failedUpgrade(o.env)
	}

	next("Starting %s service...", appService)
	startApp(o.env, appService)

	next("Waiting for %s to report build %s and schema v%d...", o.versionURL, hash, latest)
	want := expectedVersion{Hash: hash, Schema: latest, DBSchema: latest}
	if o.env == "debug" {
		// The debug image takes no build arguments.
		want.Hash = ""
	}
	if err = waitForVersion(o.versionURL, upTimeout, want); err != nil {
		fmt.Printf("Error: the application did not come up as expected: %v\n", err)
		fmt.Printf("See: docker compose --profile %s logs %s\n", o.env, appService)
		fmt.Println("Nothing has been pruned.")
		failedUpgrade(o.env)
	}

	next("Pruning what earlier deploys left...")
	if err = pruneImages(appService, repo); err != nil {
		fmt.Printf("        Warning: pruning images: %v\n", err)
	}
	deleted, err := pruneCheckpoints(dbContainer, checkpoint)
	for _, name := range deleted {
		fmt.Printf("        Deleted checkpoint %s\n", name)
	}
	if err != nil {
		fmt.Printf("        Warning: pruning checkpoints: %v\n", err)
	}
	if o.pruneBuildCache {
		if err = runCommand(nil, "docker", "builder", "prune", "--force"); err != nil {
			fmt.Printf("        Warning: pruning the build cache: %v\n", err)
		}
	}

	fmt.Println()
	fmt.Printf("Upgrade complete: %s is running %s on schema v%d.\n", appService, hash, latest)
	if previousImage != "" {
		fmt.Println()
		fmt.Println("To return to the previous deploy:")
		fmt.Printf("  goliath-cli rollback --env %s%s\n", o.env, databaseFlagSuffix())
	}
	if installed, differs := installedCLIDiffers(); differs {
		fmt.Println()
		if installed == "" {
			fmt.Println("goliath-cli is not on the PATH. Install this build with:")
		} else {
			fmt.Printf("%s is not this build. Install it with:\n", installed)
		}
		fmt.Println("  sudo make install")
	}
}

func orUnknown(s string) string {
	if s == "" {
		return "(unknown)"
	}
	return s
}

// failedUpgrade says how to get back from an upgrade that failed after the
// application was stopped.
func failedUpgrade(env string) {
	fmt.Println()
	fmt.Println("The upgrade did not complete. To return to the previous deploy, restoring the")
	fmt.Println("database from the checkpoint taken before it:")
	fmt.Printf("  goliath-cli rollback --env %s%s --full\n", env, databaseFlagSuffix())
	os.Exit(1)
}
