package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:     "logs",
	Short:   "View the application logs in a pager",
	GroupID: "debug",
	Run: func(cmd *cobra.Command, args []string) {
		logfile, _ := cmd.Flags().GetString("logfile")

		// Check if logfile exists
		if _, err := os.Stat(logfile); os.IsNotExist(err) {
			fmt.Printf("Error: log file not found at %s\n", logfile)
			return
		}

		binary, err := exec.LookPath("less")
		if err != nil {
			fmt.Printf("Error: less command not found: %v\n", err)
			return
		}

		lessArgs := []string{"less", logfile}
		if err := syscall.Exec(binary, lessArgs, os.Environ()); err != nil {
			fmt.Printf("Error executing command: %v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)
	logsCmd.Flags().String("logfile", "/tmp/goliath.INFO", "Path to the log file to view")
}
