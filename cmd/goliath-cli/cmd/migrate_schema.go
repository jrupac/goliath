package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var migrateSchemaCmd = &cobra.Command{
	Use:   "migrate-schema",
	Short: "Apply pending database schema migrations",
	Long: `Apply all pending migrations to the database, in order, by:
1. Stopping the application service (while keeping the database running)
2. Checkpointing the database so the migrations can be undone
3. Applying each pending migration, recording it in the SchemaVersion table
4. Restarting the application service

Which migrations are pending is decided by the database's SchemaVersion table
and the files in backend/schema. With --version, migrations are applied up to
and including that one; without it, up to the newest.

A database that predates the SchemaVersion table has to be told which version
it is already at. It is recorded at that version, and with every migration
applied after it, once the migration that creates the table has run:
  goliath-cli migrate-schema --baseline v25

The checkpoint is a full backup taken while the application is stopped, so
rolling back to it restores both the schema and the data as they were. Its name
is printed on completion, along with the command that restores it.

Examples:
  goliath-cli migrate-schema
  goliath-cli migrate-schema --version v28
  goliath-cli migrate-schema --env dev --database upgrade_test`,
	GroupID: "lifecycle",
	Run: func(cmd *cobra.Command, args []string) {
		env, _ := cmd.Flags().GetString("env")
		version, _ := cmd.Flags().GetString("version")
		baselineFlag, _ := cmd.Flags().GetString("baseline")
		skipCheckpoint, _ := cmd.Flags().GetBool("skip-checkpoint")
		useDatabase(cmd)

		var target, baseline int
		var err error
		if version != "" {
			if target, err = parseVersion(version); err != nil {
				fmt.Printf("Error: --version: %v\n", err)
				os.Exit(1)
			}
		}
		if baselineFlag != "" {
			if baseline, err = parseVersion(baselineFlag); err != nil {
				fmt.Printf("Error: --baseline: %v\n", err)
				os.Exit(1)
			}
		}
		runMigration(env, target, baseline, skipCheckpoint)
	},
}

func init() {
	rootCmd.AddCommand(migrateSchemaCmd)
	addEnvFlag(migrateSchemaCmd)
	addDatabaseFlag(migrateSchemaCmd)
	migrateSchemaCmd.Flags().String("version", "", "Apply migrations up to and including this one (e.g., v28); the newest if unset")
	migrateSchemaCmd.Flags().String("baseline", "", "For a database without the SchemaVersion table: the newest migration it has had (e.g., v25)")
	migrateSchemaCmd.Flags().Bool("skip-checkpoint", false, "Apply the migrations without taking a checkpoint first")
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

func runMigration(env string, target, baseline int, skipCheckpoint bool) {
	appService, dbService, dbContainer := getServiceNames(env)

	migrations, err := loadMigrations()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// The database has to be up to say what it has had, and deciding that
	// first means a run with nothing to do never stops the application.
	fmt.Printf("Ensuring %s is running...\n", dbService)
	startService(env, dbService)

	applied, versioned, err := readApplied(dbContainer)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	plan, err := makePlan(migrations, applied, versioned, baseline, target)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	if len(plan.Pending) == 0 {
		fmt.Printf("%s is at v%d; nothing to apply.\n", schemaDatabase, plan.From)
		return
	}

	steps := 4
	if skipCheckpoint {
		steps = 3
	}
	step := 0
	next := func(format string, args ...any) {
		step++
		fmt.Printf("[%d/%d] %s\n", step, steps, fmt.Sprintf(format, args...))
	}

	fmt.Printf("\nMigrating %s (environment: %s) from v%d to v%d:\n%s\n",
		schemaDatabase, env, plan.From, plan.Pending[len(plan.Pending)-1].Version,
		describePending(plan.Pending))
	if plan.Baseline != nil {
		fmt.Printf("%s records no schema version. It is taken to be at v%d, and is recorded\n", schemaDatabase, plan.From)
		fmt.Println("as such, with each migration below, once they have all applied.")
		fmt.Println()
	}

	// Stopping the application first is what lets the checkpoint be taken as of
	// now rather than as of a moment in the past: with no writer, there is
	// nothing to be inconsistent with.
	next("Stopping %s service...", appService)
	stopService(env, appService)

	checkpoint := ""
	if skipCheckpoint {
		fmt.Println("Skipping checkpoint; these migrations will not be reversible.")
	} else {
		next("Checkpointing %s...", schemaDatabase)
		if checkpoint, err = createCheckpoint(dbContainer); err != nil {
			fmt.Printf("Error creating checkpoint: %v\n", err)
			fmt.Println("Nothing has been migrated. Restart the application with:")
			fmt.Printf("  goliath-cli up --env %s\n", env)
			os.Exit(1)
		}
		// The checkpoint is named by the database, so the migration it belongs
		// to is recorded here rather than in the name.
		fmt.Printf("        Checkpointed as %s (before %s)\n", checkpoint, plan.Pending[0].Name)
	}

	next("Applying %d migration(s)...", len(plan.Pending))
	if err = applyPlan(dbContainer, plan); err != nil {
		failedMigration(env, checkpoint)
	}

	next("Starting %s service...", appService)
	startService(env, appService)

	fmt.Println()
	fmt.Println("Migration completed successfully!")
	if checkpoint != "" {
		fmt.Println()
		fmt.Println("To undo it, restoring both the schema and the data as they were before")
		fmt.Println("it ran:")
		fmt.Printf("  %s\n", rollbackCommand(env, checkpoint))
	}
}

// rollbackCommand is the command that restores a checkpoint of the database
// being acted on.
func rollbackCommand(env, checkpoint string) string {
	cmd := fmt.Sprintf("goliath-cli rollback-schema --env %s", env)
	if schemaDatabase != "goliath" {
		cmd += " --database " + schemaDatabase
	}
	return cmd + " --checkpoint " + checkpoint
}

// applyPlan applies each migration in turn, stopping at the first failure,
// and says how far it got. What to do about a failure is the caller's to say;
// the application stays stopped either way, since what it would start against
// is a schema nobody chose.
func applyPlan(dbContainer string, plan Plan) error {
	bootstrap := plan.Baseline != nil
	for i, m := range plan.Pending {
		fmt.Printf("        %s\n", m.Name)
		if err := apply(dbContainer, m, !bootstrap); err != nil {
			fmt.Printf("Error applying %s: %v\n", m.Name, err)
			fmt.Println()
			if i > 0 {
				done := fmt.Sprintf("%s through %s", plan.Pending[0].Name, plan.Pending[i-1].Name)
				if bootstrap {
					fmt.Printf("Applied before it, and not recorded: %s.\n", done)
				} else {
					fmt.Printf("Applied and recorded before it: %s.\n", done)
				}
			}
			fmt.Printf("%s may have applied in part, and is not recorded.\n", m.Name)
			return err
		}
	}

	if bootstrap {
		if err := recordBootstrap(dbContainer, *plan.Baseline, plan.Pending); err != nil {
			fmt.Printf("Error recording the migrations: %v\n", err)
			fmt.Println()
			fmt.Println("Every migration applied, but none is recorded.")
			return err
		}
	}
	return nil
}

func failedMigration(env, checkpoint string) {
	fmt.Println("The application service was stopped and has not been restarted.")
	if checkpoint != "" {
		fmt.Println("To return the database to its state from before this run:")
		fmt.Printf("  %s\n", rollbackCommand(env, checkpoint))
		fmt.Println()
	}
	fmt.Printf("Otherwise, fix the issue and restart manually with: goliath-cli up --env %s\n", env)
	os.Exit(1)
}
