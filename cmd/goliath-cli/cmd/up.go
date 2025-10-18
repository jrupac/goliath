package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Starts the Goliath environment",
	Long: `Starts the Goliath environment using Docker Compose.

Any arguments passed after a '--' separator will be passed directly to the 'docker compose up' command.
For example: goliath-cli up -- --build --force-recreate`,
	Args: func(cmd *cobra.Command, args []string) error {
		if cmd.ArgsLenAtDash() > 0 {
			return errors.New("arguments can only be passed after --")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		env, _ := cmd.Flags().GetString("env")
		attached, _ := cmd.Flags().GetBool("attached")
		build, _ := cmd.Flags().GetBool("build")

		var extraArgs []string
		if cmd.ArgsLenAtDash() != -1 {
			extraArgs = args
		}

		runDockerUp(env, attached, build, extraArgs)
	},
}

func init() {
	upCmd.GroupID = "lifecycle"
	rootCmd.AddCommand(upCmd)
	addEnvFlag(upCmd)
	addAttachedFlag(upCmd)
	addBuildFlag(upCmd)
}
