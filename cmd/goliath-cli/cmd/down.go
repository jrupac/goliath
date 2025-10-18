package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stops the Goliath environment",
	Long: `Stops the Goliath environment using Docker Compose.

Any arguments passed after a '--' separator will be passed directly to the 'docker compose down' command.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if cmd.ArgsLenAtDash() > 0 {
			return errors.New("arguments can only be passed after --")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		env, _ := cmd.Flags().GetString("env")

		var extraArgs []string
		if cmd.ArgsLenAtDash() != -1 {
			extraArgs = args
		}

		runDockerDown(env, extraArgs)
	},
}

func init() {
	downCmd.GroupID = "lifecycle"
	rootCmd.AddCommand(downCmd)
	addEnvFlag(downCmd)
}
