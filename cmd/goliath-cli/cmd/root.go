package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	lifecycleGroup = &cobra.Group{
		ID:    "lifecycle",
		Title: "Application Lifecycle:",
	}
	setupGroup = &cobra.Group{
		ID:    "setup",
		Title: "Application Setup:",
	}
	userFeedGroup = &cobra.Group{
		ID:    "user_feed",
		Title: "User Feed Management:",
	}
	userPrefGroup = &cobra.Group{
		ID:    "user_pref",
		Title: "User Preference Management:",
	}
)

var rootCmd = &cobra.Command{
	Use:   "goliath-cli",
	Short: "Goliath CLI",
	Long:  `A CLI for interacting with Goliath.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddGroup(lifecycleGroup)
	rootCmd.AddGroup(setupGroup)
	rootCmd.AddGroup(userFeedGroup)
	rootCmd.AddGroup(userPrefGroup)
}
