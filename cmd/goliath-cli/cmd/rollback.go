package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/jrupac/goliath/schema"
	"github.com/spf13/cobra"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Return to the deploy before the last upgrade",
	Long: `Undo the last 'goliath-cli upgrade' by starting the image it replaced, which
upgrade kept tagged :previous.

When every migration the upgrade applied lets older binaries run, only the
image changes and everything written since is kept. Otherwise, or with --full,
the database is also restored from the checkpoint the upgrade took, which
loses everything written since; the current state is checkpointed first.

The checkout is left alone. 'goliath-cli up --build' or 'reload' would rebuild
what was rolled back; roll forward with 'goliath-cli upgrade' once a fix is
upstream. Only one deploy back is kept, so a rollback cannot be repeated.`,
	GroupID: "lifecycle",
	Run: func(cmd *cobra.Command, args []string) {
		env, _ := cmd.Flags().GetString("env")
		full, _ := cmd.Flags().GetBool("full")
		yes, _ := cmd.Flags().GetBool("yes")
		versionURL, _ := cmd.Flags().GetString("version-url")
		useDatabase(cmd)
		runDeployRollback(env, full, yes, versionURL)
	},
}

func init() {
	rootCmd.AddCommand(rollbackCmd)
	addEnvFlag(rollbackCmd)
	addDatabaseFlag(rollbackCmd)
	rollbackCmd.Flags().Bool("full", false, "Also restore the database from the upgrade's checkpoint, even if the previous image could run on it")
	rollbackCmd.Flags().Bool("yes", false, "Skip the confirmation prompt")
	rollbackCmd.Flags().String("version-url", "http://127.0.0.1:9999/version", "Where the application reports its build")
}

func runDeployRollback(env string, forceFull, yes bool, versionURL string) {
	appService, dbService, dbContainer := getServiceNames(env)

	rec, found, err := lastDeployRecord(env, schemaDatabase)
	if err != nil {
		exitWith("%v", err)
	}
	if !found || rec.Kind != deployUpgrade {
		exitWith("nothing to roll back: the last deploy recorded for %s in %s is not an upgrade, "+
			"and only one deploy back is kept", schemaDatabase, env)
	}
	if rec.PreviousImage == "" {
		exitWith("the last upgrade found no running image to keep, so there is none to return to")
	}

	repo, err := serviceImage(env, appService)
	if err != nil {
		exitWith("%v", err)
	}
	if id, err := imageID(repo + ":previous"); err != nil || id != rec.PreviousImage {
		exitWith("%s:previous is no longer the image the last upgrade replaced (%s)", repo, rec.PreviousImage)
	}

	fmt.Printf("Ensuring %s is running...\n", dbService)
	startService(env, dbService)
	applied, _, err := readApplied(dbContainer)
	if err != nil {
		exitWith("%v", err)
	}
	full, reason := chooseRollback(rec, applied, forceFull)
	if full && rec.Checkpoint == "" {
		exitWith("the database has to be restored (%s), but the upgrade recorded no checkpoint", reason)
	}

	fmt.Printf("\nRolling back %s (environment: %s, database: %s):\n", appService, env, schemaDatabase)
	fmt.Printf("  build:    %s -> %s\n", orUnknown(rec.Hash), orUnknown(rec.PreviousHash))
	if full {
		fmt.Printf("  database: restored from %s, taken before the upgrade (%s).\n", rec.Checkpoint, reason)
		fmt.Println("            Everything written since then is LOST; the current state is")
		fmt.Println("            checkpointed first, so this is undoable.")
	} else {
		fmt.Printf("  database: kept as it is, at v%d; the previous image can run on it.\n", schema.Current(applied))
	}
	fmt.Println()
	confirmOrExit(yes, "rollback")

	undo := ""
	if full {
		fmt.Printf("Stopping %s service...\n", appService)
		stopService(env, appService)

		fmt.Printf("Checkpointing the current state of %s...\n", schemaDatabase)
		if undo, err = createCheckpoint(dbContainer); err != nil {
			exitWith("checkpointing the current state: %v. Nothing has been rolled back; restart with "+
				"goliath-cli up --env %s", err, env)
		}
		checkpoint, found, err := resolveCheckpoint(dbContainer, rec.Checkpoint)
		if err != nil || !found {
			exitWith("the upgrade's checkpoint %s is not available (%v); nothing has been restored, "+
				"and %s is stopped", rec.Checkpoint, err, appService)
		}
		fmt.Printf("Restoring %s from %s...\n", schemaDatabase, checkpoint)
		if err = restoreCheckpoint(dbContainer, checkpoint); err != nil {
			fmt.Printf("Error restoring the checkpoint: %v\n", err)
			fmt.Printf("The database may be in a partial state and %s is stopped. Return to the state\n", appService)
			fmt.Println("from just before this attempt with:")
			fmt.Printf("  %s\n", rollbackCommand(env, undo))
			os.Exit(1)
		}
	}

	fmt.Printf("Starting the previous image...\n")
	if err = tagImage(rec.PreviousImage, repo+":latest"); err != nil {
		exitWith("tagging the previous image: %v", err)
	}
	startService(env, appService)

	if rec.PreviousHash == "" {
		fmt.Printf("Waiting for %s to answer...\n", versionURL)
	} else {
		fmt.Printf("Waiting for %s to report build %s...\n", versionURL, rec.PreviousHash)
	}
	upErr := waitForVersion(versionURL, upTimeout, expectedVersion{Hash: rec.PreviousHash})

	current := schema.Current(applied)
	if full {
		current = rec.SchemaFrom
	}
	if err = appendDeployRecord(deployRecord{
		Kind: deployRollback, Time: time.Now().UTC(), Env: env, Database: schemaDatabase,
		Hash: rec.PreviousHash, Image: rec.PreviousImage, PreviousHash: rec.Hash, PreviousImage: rec.Image,
		SchemaFrom: schema.Current(applied), SchemaTo: current, Checkpoint: undo,
	}); err != nil {
		fmt.Printf("Warning: recording the rollback in %s: %v\n", deployLogPath, err)
	}

	if upErr != nil {
		fmt.Printf("Error: the previous image did not come up as expected: %v\n", upErr)
		fmt.Printf("See: docker compose --profile %s logs %s\n", env, appService)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("Rollback complete.")
	if undo != "" {
		fmt.Println("The database as it was before this rollback is checkpointed as:")
		fmt.Printf("  %s\n", undo)
	}
	fmt.Println()
	fmt.Println("The checkout is unchanged, so 'goliath-cli up --build' or 'reload' would rebuild")
	fmt.Println("what was rolled back. Roll forward with 'goliath-cli upgrade' once a fix is")
	fmt.Println("upstream.")
}
