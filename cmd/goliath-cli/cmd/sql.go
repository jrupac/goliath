package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"
)

var sqlCmd = &cobra.Command{
	Use:     "sql",
	Short:   "Open an interactive CRDB SQL shell",
	GroupID: "debug",
	Run: func(cmd *cobra.Command, args []string) {
		env, _ := cmd.Flags().GetString("env")
		database, _ := cmd.Flags().GetString("database")

		var containerName string
		switch env {
		case "prod":
			containerName = "crdb-service"
		case "dev":
			containerName = "crdb-dev"
		case "debug":
			containerName = "crdb-debug"
		default:
			fmt.Printf("Invalid environment: %s. Must be one of 'prod', 'dev', or 'debug'.\n", env)
			return
		}

		binary, err := exec.LookPath("docker")
		if err != nil {
			fmt.Printf("Error: docker command not found: %v\n", err)
			return
		}

		dockerArgs := []string{"docker", "exec", "-it", containerName, "./cockroach", "sql", "--insecure", "--database=" + database}

		fmt.Println("Executing:", dockerArgs)

		if err := syscall.Exec(binary, dockerArgs, os.Environ()); err != nil {
			fmt.Printf("Error executing command: %v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(sqlCmd)
	addEnvFlag(sqlCmd)
	sqlCmd.Flags().String("database", "goliath", "The database to connect to")
}
