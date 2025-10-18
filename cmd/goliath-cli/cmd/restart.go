package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restarts the Goliath environment",
	Long: `Restarts the Goliath environment by running 'docker compose restart'.

Any arguments passed after a '--' separator will be passed directly to the 'docker compose restart' command.`,
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

		dockerArgs := []string{"compose", "--profile", env, "restart"}
		dockerArgs = append(dockerArgs, extraArgs...)
		executeDockerCompose(dockerArgs)
	},
}

func init() {
	restartCmd.GroupID = "lifecycle"
	rootCmd.AddCommand(restartCmd)
	addEnvFlag(restartCmd)
}
