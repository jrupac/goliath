package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rollbackSchemaCmd = &cobra.Command{
	Use:   "rollback-schema",
	Short: "Restore the database from a checkpoint taken before a migration",
	Long: `Undo a schema migration by:
1. Stopping the application service (while keeping the database running)
2. Checkpointing the current state, so the rollback is itself reversible
3. Replacing the database with a checkpoint taken before the migration
4. Restarting the application service

This DISCARDS everything written since that checkpoint was taken, not only the
schema change: articles fetched, marks made, and sessions established in the
meantime are all lost. The checkpoint taken in step 2 is what makes that
recoverable if it turns out to be the wrong call.

List what is available with:
  goliath-cli list-checkpoints --env prod

Example:
  goliath-cli rollback-schema --checkpoint /2026/01/01-000000.00`,
	GroupID: "lifecycle",
	Run: func(cmd *cobra.Command, args []string) {
		env, _ := cmd.Flags().GetString("env")
		checkpoint, _ := cmd.Flags().GetString("checkpoint")
		force, _ := cmd.Flags().GetBool("force")

		if checkpoint == "" {
			_, dbService, dbContainer := getServiceNames(env)
			fmt.Println("Error: --checkpoint flag is required")
			fmt.Println()
			startService(env, dbService)
			printAvailableCheckpoints(env, dbContainer)
			os.Exit(1)
		}

		runRollback(env, checkpoint, force)
	},
}

var listCheckpointsCmd = &cobra.Command{
	Use:   "list-checkpoints",
	Short: "List database checkpoints available to roll back to",
	Long: `List the checkpoints migrate-schema and rollback-schema have taken, newest
first, each with the command that restores it.

Snapshots taken by tools/dev-db/snapshot.sh are kept separately and are not
listed here; restore those with tools/dev-db/restore.sh.`,
	GroupID: "lifecycle",
	Run: func(cmd *cobra.Command, args []string) {
		env, _ := cmd.Flags().GetString("env")
		_, dbService, dbContainer := getServiceNames(env)

		fmt.Printf("Ensuring %s is running...\n", dbService)
		startService(env, dbService)
		fmt.Println()

		printAvailableCheckpoints(env, dbContainer)
	},
}

func init() {
	rootCmd.AddCommand(rollbackSchemaCmd)
	addEnvFlag(rollbackSchemaCmd)
	rollbackSchemaCmd.Flags().String("checkpoint", "", "Checkpoint to restore (see list-checkpoints)")
	rollbackSchemaCmd.Flags().Bool("force", false, "Skip the confirmation prompt")

	rootCmd.AddCommand(listCheckpointsCmd)
	addEnvFlag(listCheckpointsCmd)
}

func runRollback(env, requested string, force bool) {
	appService, dbService, dbContainer := getServiceNames(env)

	fmt.Printf("Rolling back to checkpoint: %s (environment: %s)\n", requested, env)
	fmt.Println()

	// The database has to be up before checkpoints can be listed, since it is
	// the database that holds them.
	fmt.Printf("[1/6] Ensuring %s is running...\n", dbService)
	startService(env, dbService)

	fmt.Printf("[2/6] Locating checkpoint %s...\n", requested)
	checkpoint, found, err := resolveCheckpoint(dbContainer, requested)
	if err != nil {
		fmt.Printf("Could not list checkpoints: %v\n", err)
		os.Exit(1)
	}
	if !found {
		fmt.Printf("No checkpoint named '%s' on container '%s'.\n\n", requested, dbContainer)
		printAvailableCheckpoints(env, dbContainer)
		os.Exit(1)
	}

	if !force {
		fmt.Println()
		fmt.Printf("WARNING: this DROPS database '%s' in the '%s' environment and replaces\n", schemaDatabase, env)
		fmt.Println("         it with the checkpoint. Everything written since the checkpoint")
		fmt.Println("         was taken is lost, including articles fetched and marks made.")
		fmt.Println("         The current state is checkpointed first, so this is undoable.")
		if promptForInput(fmt.Sprintf("Type '%s' to confirm:", schemaDatabase)) != schemaDatabase {
			fmt.Println("Confirmation failed. Aborting; nothing changed.")
			os.Exit(1)
		}
		fmt.Println()
	}

	fmt.Printf("[3/6] Stopping %s service...\n", appService)
	stopService(env, appService)

	// Rolling back is the most destructive thing this tool does, and until this
	// checkpoint exists it is the only one with nothing to return to.
	fmt.Printf("[4/6] Checkpointing the current state of %s...\n", schemaDatabase)
	undo, err := createCheckpoint(dbContainer)
	if err != nil {
		fmt.Printf("Error checkpointing the current state: %v\n", err)
		fmt.Println("Nothing has been rolled back. Restart the application with:")
		fmt.Printf("  goliath-cli up --env %s\n", env)
		os.Exit(1)
	}

	fmt.Printf("[5/6] Restoring %s from %s...\n", schemaDatabase, checkpoint)
	if err = restoreCheckpoint(dbContainer, checkpoint); err != nil {
		fmt.Printf("Error restoring checkpoint: %v\n", err)
		fmt.Printf("The database may be in a partial state and %s is stopped.\n", appService)
		fmt.Println("Both checkpoints are unharmed. Retry with:")
		fmt.Printf("  goliath-cli rollback-schema --env %s --checkpoint %s\n", env, checkpoint)
		fmt.Println("Or return to the state from just before this attempt with:")
		fmt.Printf("  goliath-cli rollback-schema --env %s --checkpoint %s\n", env, undo)
		os.Exit(1)
	}

	fmt.Printf("[6/6] Starting %s service...\n", appService)
	startService(env, appService)

	fmt.Println()
	fmt.Println("Rollback completed successfully!")
	fmt.Println()
	fmt.Println("To undo this rollback, returning to the state from just before it ran:")
	fmt.Printf("  goliath-cli rollback-schema --env %s --checkpoint %s\n", env, undo)
}
