package cmd

import (
	"context"
	"fmt"

	"github.com/jrupac/goliath/admin"
	"github.com/spf13/cobra"
)

var listUnmutedFeedsCmd = &cobra.Command{
	Use:     "list-unmuted-feeds",
	Short:   "List all unmuted feeds for a user",
	GroupID: "user_pref",
	Run: func(cmd *cobra.Command, args []string) {
		client, conn := getAdminClient(cmd)
		defer conn.Close()

		user := getUser(cmd)
		if user == "" {
			fmt.Println("Command aborted. User is required.")
			return
		}

		// Get unmuted feed IDs
		unmutedRes, err := client.GetUnmutedFeeds(context.Background(), &admin.GetUnmutedFeedsRequest{Username: user})
		if err != nil {
			fmt.Printf("Error fetching unmuted feeds: %v\n", err)
			return
		}

		if len(unmutedRes.UnmutedFeedId) == 0 {
			fmt.Println("No unmuted feeds found for user:", user)
			return
		}

		// Get all feeds to map IDs to titles
		allFeedsRes, err := client.GetFeeds(context.Background(), &admin.GetFeedsRequest{Username: user})
		if err != nil {
			fmt.Printf("Error fetching all feeds: %v\n", err)
			return
		}

		idToTitle := make(map[int64]string)
		for _, feed := range allFeedsRes.Feeds {
			idToTitle[feed.Id] = feed.Title
		}

		fmt.Println("Unmuted feeds for", user, ":")
		for _, id := range unmutedRes.UnmutedFeedId {
			title, ok := idToTitle[id]
			if !ok {
				title = "Unknown Title"
			}
			fmt.Printf("  ID: %d, Title: %s\n", id, title)
		}
	},
}

func init() {
	rootCmd.AddCommand(listUnmutedFeedsCmd)
	addGrpcAddressFlag(listUnmutedFeedsCmd)
	addUserFlag(listUnmutedFeedsCmd)
}
