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
2. Applying the specified migration
3. Restarting the application service

Example:
  goliath-cli migrate-schema --version v19_add_saved_to_article.sql`,
	GroupID: "lifecycle",
	Run: func(cmd *cobra.Command, args []string) {
		env, _ := cmd.Flags().GetString("env")
		version, _ := cmd.Flags().GetString("version")

		if version == "" {
			fmt.Println("Error: --version flag is required")
			fmt.Println("Example: goliath-cli migrate-schema --version v19_add_saved_to_article.sql")
			os.Exit(1)
		}

		runMigration(env, version)
	},
}

func init() {
	rootCmd.AddCommand(migrateSchemaCmd)
	addEnvFlag(migrateSchemaCmd)
	migrateSchemaCmd.Flags().String("version", "", "Migration version to apply (e.g., v19_add_saved_to_article.sql)")
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

func runMigration(env, version string) {
	appService, dbService, dbContainer := getServiceNames(env)

	// Verify migration file exists
	migrationPath := filepath.Join("backend", "schema", version)
	if _, err := os.Stat(migrationPath); os.IsNotExist(err) {
		fmt.Printf("Migration file not found: %s\n", migrationPath)
		os.Exit(1)
	}

	fmt.Printf("Applying migration: %s (environment: %s)\n", version, env)
	fmt.Println()

	// Step 1: Stop the application service
	fmt.Printf("[1/4] Stopping %s service...\n", appService)
	stopService(env, appService)

	// Step 2: Ensure database is running
	fmt.Printf("[2/4] Ensuring %s is running...\n", dbService)
	startService(env, dbService)

	// Step 3: Apply the migration
	fmt.Printf("[3/4] Applying migration %s...\n", version)
	applyMigration(dbContainer, version)

	// Step 4: Restart the application service
	fmt.Printf("[4/4] Starting %s service...\n", appService)
	startService(env, appService)

	fmt.Println()
	fmt.Println("Migration completed successfully!")
}

func applyMigration(dbContainer, version string) {
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
		fmt.Println("The application service was stopped but not restarted due to migration failure.")
		fmt.Println("Please fix the issue and restart manually with: goliath-cli up --env " + dbContainer)
		os.Exit(1)
	}
}
