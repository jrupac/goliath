package cmd

import (
	"context"
	"fmt"

	"github.com/jrupac/goliath/admin"
	"github.com/spf13/cobra"
)

var listMuteWordsCmd = &cobra.Command{
	Use:     "list-mute-words",
	Short:   "List all mute words for a user",
	GroupID: "admin",
	Run: func(cmd *cobra.Command, args []string) {
		client, conn := getAdminClient(cmd)
		defer conn.Close()

		user := getUser(cmd)
		if user == "" {
			fmt.Println("Command aborted. User is required.")
			return
		}

		req := &admin.GetMuteWordsRequest{
			Username: user,
		}

		res, err := client.GetMuteWords(context.Background(), req)
		if err != nil {
			fmt.Printf("Error calling GetMuteWords: %v\n", err)
			return
		}

		if len(res.MuteWord) == 0 {
			fmt.Println("No mute words found for user:", user)
			return
		}

		fmt.Println("Mute words for", user, ":")
		for _, word := range res.MuteWord {
			fmt.Printf("  - %s\n", word)
		}
	},
}

func init() {
	rootCmd.AddCommand(listMuteWordsCmd)
	addGrpcAddressFlag(listMuteWordsCmd)
	addUserFlag(listMuteWordsCmd)
}
