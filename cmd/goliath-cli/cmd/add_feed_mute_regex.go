package cmd

import (
	"context"
	"fmt"

	"github.com/jrupac/goliath/admin"
	"github.com/spf13/cobra"
)

var addFeedMuteRegexCmd = &cobra.Command{
	Use:     "add-feed-mute-regex",
	Short:   "Add a mute regex for one or more feeds",
	GroupID: "user_pref",
	Run: func(cmd *cobra.Command, args []string) {
		client, conn := getAdminClient(cmd)
		defer conn.Close()

		user := getUser(cmd)
		if user == "" {
			fmt.Println("Command aborted. User is required.")
			return
		}

		feedID, _ := cmd.Flags().GetInt64("feed-id")
		regex, _ := cmd.Flags().GetString("regex")

		var selectedFeedIDs []int64

		if feedID != 0 {
			selectedFeedIDs = []int64{feedID}
		} else {
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

			selectedTitles := promptForChecklist("Select feeds to apply regex to:", feedTitles)
			if len(selectedTitles) == 0 {
				fmt.Println("No feeds selected. Aborting.")
				return
			}

			for _, title := range selectedTitles {
				selectedFeedIDs = append(selectedFeedIDs, feedTitleToID[title])
			}
		}

		if regex == "" {
			regex = promptForInput("Enter regex:")
			if regex == "" {
				fmt.Println("No regex provided. Aborting.")
				return
			}
		}

		successCount := 0
		for _, id := range selectedFeedIDs {
			req := &admin.AddFeedMuteRegexRequest{
				Username: user,
				FeedId:   id,
				Regex:    regex,
			}

			_, err := client.AddFeedMuteRegex(context.Background(), req)
			if err != nil {
				fmt.Printf("Error calling AddFeedMuteRegex for feed ID %d: %v\n", id, err)
				continue
			}
			successCount++
		}

		fmt.Printf("Successfully added regex to %d feed(s) for user: %s\n", successCount, user)
	},
}

func init() {
	rootCmd.AddCommand(addFeedMuteRegexCmd)
	addGrpcAddressFlag(addFeedMuteRegexCmd)
	addUserFlag(addFeedMuteRegexCmd)
	addFeedMuteRegexCmd.Flags().Int64("feed-id", 0, "Feed ID to apply the regex to")
	addFeedMuteRegexCmd.Flags().String("regex", "", "Mute regex pattern to add")
}
