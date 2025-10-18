package cmd

import (
	"context"
	"fmt"

	"github.com/jrupac/goliath/admin"
	"github.com/spf13/cobra"
)

var deleteMuteWordsCmd = &cobra.Command{
	Use:     "delete-mute-words",
	Short:   "Delete mute words for a user",
	GroupID: "admin",
	Run: func(cmd *cobra.Command, args []string) {
		client, conn := getAdminClient(cmd)
		defer conn.Close()

		user := getUser(cmd)
		if user == "" {
			fmt.Println("Command aborted. User is required.")
			return
		}

		// Get existing mute words
		getRequest := &admin.GetMuteWordsRequest{Username: user}
		getResponse, err := client.GetMuteWords(context.Background(), getRequest)
		if err != nil {
			fmt.Printf("Error fetching mute words: %v\n", err)
			return
		}

		if len(getResponse.MuteWord) == 0 {
			fmt.Println("No mute words found for user:", user)
			return
		}

		// Prompt user to select words to delete
		wordsToDelete := promptForChecklist("Select mute words to delete:", getResponse.MuteWord)

		if len(wordsToDelete) == 0 {
			fmt.Println("No words selected. Aborting.")
			return
		}

		// Delete selected words
		deleteRequest := &admin.DeleteMuteWordRequest{
			Username: user,
			MuteWord: wordsToDelete,
		}

		_, err = client.DeleteMuteWord(context.Background(), deleteRequest)
		if err != nil {
			fmt.Printf("Error calling DeleteMuteWord: %v\n", err)
			return
		}

		fmt.Printf("Successfully deleted %d mute words for user: %s\n", len(wordsToDelete), user)
	},
}

func init() {
	rootCmd.AddCommand(deleteMuteWordsCmd)
	addGrpcAddressFlag(deleteMuteWordsCmd)
	addUserFlag(deleteMuteWordsCmd)
}
