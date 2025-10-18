package cmd

import (
	"context"
	"fmt"

	"github.com/jrupac/goliath/admin"
	"github.com/spf13/cobra"
)

var deleteFeedsCmd = &cobra.Command{
	Use:     "delete-feeds",
	Short:   "Delete feeds for a user",
	GroupID: "user_feed",
	Run: func(cmd *cobra.Command, args []string) {
		client, conn := getAdminClient(cmd)
		defer conn.Close()

		user := getUser(cmd)
		if user == "" {
			fmt.Println("Command aborted. User is required.")
			return
		}

		// Get existing feeds
		getRequest := &admin.GetFeedsRequest{Username: user}
		getResponse, err := client.GetFeeds(context.Background(), getRequest)
		if err != nil {
			fmt.Printf("Error fetching feeds: %v\n", err)
			return
		}

		if len(getResponse.Feeds) == 0 {
			fmt.Println("No feeds found for user:", user)
			return
		}

		// Prepare for checklist prompt
		var feedTitles []string
		feedTitleToID := make(map[string]int64)
		for _, feed := range getResponse.Feeds {
			feedTitles = append(feedTitles, feed.Title)
			feedTitleToID[feed.Title] = feed.Id
		}

		// Prompt user to select feeds to delete
		feedsToDelete := promptForChecklist("Select feeds to delete:", feedTitles)

		if len(feedsToDelete) == 0 {
			fmt.Println("No feeds selected. Aborting.")
			return
		}

		// Delete selected feeds
		var deletedCount int
		for _, title := range feedsToDelete {
			id := feedTitleToID[title]
			deleteRequest := &admin.RemoveFeedRequest{
				Username: user,
				Id:       id,
			}
			_, err := client.RemoveFeed(context.Background(), deleteRequest)
			if err != nil {
				fmt.Printf("Error deleting feed '%s' (ID: %d): %v\n", title, id, err)
			} else {
				deletedCount++
			}
		}

		fmt.Printf("Successfully deleted %d feeds for user: %s\n", deletedCount, user)
	},
}

func init() {
	rootCmd.AddCommand(deleteFeedsCmd)
	addGrpcAddressFlag(deleteFeedsCmd)
	addUserFlag(deleteFeedsCmd)
}
