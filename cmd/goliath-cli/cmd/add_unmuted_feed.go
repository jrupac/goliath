package cmd

import (
	"context"
	"fmt"

	"github.com/jrupac/goliath/admin"
	"github.com/spf13/cobra"
)

var addUnmutedFeedCmd = &cobra.Command{
	Use:     "add-unmuted-feed",
	Short:   "Add feeds to the unmute list for a user",
	GroupID: "user_pref",
	Run: func(cmd *cobra.Command, args []string) {
		client, conn := getAdminClient(cmd)
		defer conn.Close()

		user := getUser(cmd)
		if user == "" {
			fmt.Println("Command aborted. User is required.")
			return
		}

		// Get all feeds to present a selection
		allFeedsRes, err := client.GetFeeds(context.Background(), &admin.GetFeedsRequest{Username: user})
		if err != nil {
			fmt.Printf("Error fetching feeds: %v\n", err)
			return
		}

		if len(allFeedsRes.Feeds) == 0 {
			fmt.Println("No feeds found for user:", user)
			return
		}

		var feedTitles []string
		feedTitleToID := make(map[string]int64)
		for _, feed := range allFeedsRes.Feeds {
			feedTitles = append(feedTitles, feed.Title)
			feedTitleToID[feed.Title] = feed.Id
		}

		selectedTitles := promptForChecklist("Select feeds to unmute:", feedTitles)

		if len(selectedTitles) == 0 {
			fmt.Println("No feeds selected. Aborting.")
			return
		}

		var idsToAdd []int64
		for _, title := range selectedTitles {
			idsToAdd = append(idsToAdd, feedTitleToID[title])
		}

		req := &admin.AddUnmutedFeedRequest{
			Username:      user,
			UnmutedFeedId: idsToAdd,
		}

		_, err = client.AddUnmutedFeed(context.Background(), req)
		if err != nil {
			fmt.Printf("Error calling AddUnmutedFeed: %v\n", err)
			return
		}

		fmt.Printf("Successfully added %d feeds to the unmute list for user: %s\n", len(idsToAdd), user)
	},
}

func init() {
	rootCmd.AddCommand(addUnmutedFeedCmd)
	addGrpcAddressFlag(addUnmutedFeedCmd)
	addUserFlag(addUnmutedFeedCmd)
}
