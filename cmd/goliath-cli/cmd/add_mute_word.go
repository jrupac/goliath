package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/jrupac/goliath/admin"
	"github.com/spf13/cobra"
)

var addMuteWordCmd = &cobra.Command{
	Use:     "add-mute-word",
	Short:   "Add mute words for a user",
	GroupID: "admin",
	Run: func(cmd *cobra.Command, args []string) {
		client, conn := getAdminClient(cmd)
		defer conn.Close()

		user := getUser(cmd)
		if user == "" {
			fmt.Println("Command aborted. User is required.")
			return
		}

		wordsStr, _ := cmd.Flags().GetString("words")
		var words []string
		if wordsStr != "" {
			words = strings.Split(wordsStr, ",")
		} else {
			words = promptForMultiline()
		}

		if len(words) == 0 {
			fmt.Println("No words provided. Aborting.")
			return
		}

		req := &admin.AddMuteWordRequest{
			Username: user,
			MuteWord: words,
		}

		_, err := client.AddMuteWord(context.Background(), req)
		if err != nil {
			fmt.Printf("Error calling AddMuteWord: %v\n", err)
			return
		}

		fmt.Println("Successfully added mute words for user:", user)
	},
}

func init() {
	rootCmd.AddCommand(addMuteWordCmd)
	addGrpcAddressFlag(addMuteWordCmd)
	addUserFlag(addMuteWordCmd)
	addMuteWordCmd.Flags().String("words", "", "Comma-separated list of words to add")
}
