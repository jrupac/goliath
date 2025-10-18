package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

var reloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "Reloads the Goliath environment (down, build, and up)",
	Long: `Reloads the Goliath environment by running 'down', then 'up --build'.

Any arguments passed after a '--' separator will be passed directly to the 'docker compose down' and 'docker compose up' commands.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if cmd.ArgsLenAtDash() > 0 {
			return errors.New("arguments can only be passed after --")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		env, _ := cmd.Flags().GetString("env")
		attached, _ := cmd.Flags().GetBool("attached")

		var extraArgs []string
		if cmd.ArgsLenAtDash() != -1 {
			extraArgs = args
		}

		runDockerDown(env, extraArgs)
		runDockerUp(env, attached, true, extraArgs) // build is true
	},
}

func init() {
	reloadCmd.GroupID = "lifecycle"
	rootCmd.AddCommand(reloadCmd)
	addEnvFlag(reloadCmd)
	addAttachedFlag(reloadCmd)
}
