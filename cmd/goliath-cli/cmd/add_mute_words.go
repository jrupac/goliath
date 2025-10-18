package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/jrupac/goliath/admin"
	"github.com/spf13/cobra"
)

var addMuteWordsCmd = &cobra.Command{
	Use:     "add-mute-words",
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

		fmt.Printf("Successfully added %d mute words for user: %s\n", len(words), user)
	},
}

func init() {
	rootCmd.AddCommand(addMuteWordsCmd)
	addGrpcAddressFlag(addMuteWordsCmd)
	addUserFlag(addMuteWordsCmd)
	addMuteWordsCmd.Flags().String("words", "", "Comma-separated list of words to add")
}
