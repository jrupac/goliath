package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var migrateSchemaCmd = &cobra.Command{
	Use:   "migrate-schema",
	Short: "Apply a database schema migration",
	Long: `Safely apply a database schema migration by:
1. Stopping the application service (while keeping the database running)
2. Checkpointing the database so the migration can be undone
3. Applying the specified migration
4. Restarting the application service

The checkpoint is a full backup taken while the application is stopped, so
rolling back to it restores both the schema and the data as they were. Its name
is printed on completion, along with the command that restores it.

Example:
  goliath-cli migrate-schema --version v19_add_saved_to_article.sql`,
	GroupID: "lifecycle",
	Run: func(cmd *cobra.Command, args []string) {
		env, _ := cmd.Flags().GetString("env")
		version, _ := cmd.Flags().GetString("version")
		skipCheckpoint, _ := cmd.Flags().GetBool("skip-checkpoint")

		if version == "" {
			fmt.Println("Error: --version flag is required")
			fmt.Println("Example: goliath-cli migrate-schema --version v19_add_saved_to_article.sql")
			os.Exit(1)
		}

		runMigration(env, version, skipCheckpoint)
	},
}

func init() {
	rootCmd.AddCommand(migrateSchemaCmd)
	addEnvFlag(migrateSchemaCmd)
	migrateSchemaCmd.Flags().String("version", "", "Migration version to apply (e.g., v19_add_saved_to_article.sql)")
	migrateSchemaCmd.Flags().Bool("skip-checkpoint", false, "Apply the migration without taking a checkpoint first")
}

func getServiceNames(env string) (appService, dbService, dbContainer string) {
	switch env {
	case "prod":
		return "goliath", "crdb", "crdb-service"
	case "dev":
		return "backend-dev", "crdb-dev", "crdb-dev"
	case "debug":
		return "backend-debug", "crdb-debug", "crdb-debug"
	default:
		fmt.Printf("Unknown environment: %s\n", env)
		os.Exit(1)
		return "", "", ""
	}
}

func runMigration(env, version string, skipCheckpoint bool) {
	appService, dbService, dbContainer := getServiceNames(env)

	// Verify migration file exists
	migrationPath := filepath.Join("backend", "schema", version)
	if _, err := os.Stat(migrationPath); os.IsNotExist(err) {
		fmt.Printf("Migration file not found: %s\n", migrationPath)
		os.Exit(1)
	}

	steps := 5
	if skipCheckpoint {
		steps = 4
	}
	step := 0
	next := func(format string, args ...any) {
		step++
		fmt.Printf("[%d/%d] %s\n", step, steps, fmt.Sprintf(format, args...))
	}

	fmt.Printf("Applying migration: %s (environment: %s)\n", version, env)
	fmt.Println()

	// Stopping the application first is what lets the checkpoint be taken as of
	// now rather than as of a moment in the past: with no writer, there is
	// nothing to be inconsistent with.
	next("Stopping %s service...", appService)
	stopService(env, appService)

	next("Ensuring %s is running...", dbService)
	startService(env, dbService)

	checkpoint := ""
	if skipCheckpoint {
		fmt.Println("Skipping checkpoint; this migration will not be reversible.")
	} else {
		next("Checkpointing %s...", schemaDatabase)
		var err error
		if checkpoint, err = createCheckpoint(dbContainer); err != nil {
			fmt.Printf("Error creating checkpoint: %v\n", err)
			fmt.Println("Nothing has been migrated. Restart the application with:")
			fmt.Printf("  goliath-cli up --env %s\n", env)
			os.Exit(1)
		}
		// The checkpoint is named by the database, so the migration it belongs
		// to is recorded here rather than in the name.
		fmt.Printf("        Checkpointed as %s (before %s)\n", checkpoint, version)
	}

	next("Applying migration %s...", version)
	applyMigration(dbContainer, version, env, checkpoint)

	next("Starting %s service...", appService)
	startService(env, appService)

	fmt.Println()
	fmt.Println("Migration completed successfully!")
	if checkpoint != "" {
		fmt.Println()
		fmt.Println("To undo this migration, restoring both the schema and the data as they")
		fmt.Println("were before it ran:")
		fmt.Printf("  goliath-cli rollback-schema --env %s --checkpoint %s\n", env, checkpoint)
	}
}

func applyMigration(dbContainer, version, env, checkpoint string) {
	// Read the migration file
	migrationPath := filepath.Join("backend", "schema", version)
	migrationSQL, err := os.ReadFile(migrationPath)
	if err != nil {
		fmt.Printf("Error reading migration file: %v\n", err)
		os.Exit(1)
	}

	// Execute the migration via docker exec
	// Using stdin to pass the SQL avoids shell escaping issues
	args := []string{"exec", "-i", dbContainer, "./cockroach", "sql", "--insecure"}
	cmd := exec.Command("docker", args...)
	cmd.Stdin = strings.NewReader(string(migrationSQL))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("Error applying migration: %v\n", err)
		fmt.Println()
		fmt.Println("The application service was stopped but not restarted due to migration failure.")
		if checkpoint != "" {
			fmt.Println("The migration may have applied in part. To return the database to its")
			fmt.Println("state from before it ran:")
			fmt.Printf("  goliath-cli rollback-schema --env %s --checkpoint %s\n", env, checkpoint)
			fmt.Println()
		}
		fmt.Printf("Otherwise, fix the issue and restart manually with: goliath-cli up --env %s\n", env)
		os.Exit(1)
	}
}
