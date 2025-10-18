package cmd

import (
	"context"
	"fmt"

	"github.com/jrupac/goliath/admin"
	"github.com/spf13/cobra"
)

var deleteUnmutedFeedsCmd = &cobra.Command{
	Use:     "delete-unmuted-feeds",
	Short:   "Remove feeds from the unmute list for a user",
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

		var unmutedFeedTitles []string
		unmutedFeedTitleToID := make(map[string]int64)
		for _, id := range unmutedRes.UnmutedFeedId {
			title, ok := idToTitle[id]
			if !ok {
				title = fmt.Sprintf("Unknown Title (ID: %d)", id)
			}
			unmutedFeedTitles = append(unmutedFeedTitles, title)
			unmutedFeedTitleToID[title] = id
		}

		selectedTitles := promptForChecklist("Select unmuted feeds to remove:", unmutedFeedTitles)

		if len(selectedTitles) == 0 {
			fmt.Println("No feeds selected. Aborting.")
			return
		}

		var idsToDelete []int64
		for _, title := range selectedTitles {
			idsToDelete = append(idsToDelete, unmutedFeedTitleToID[title])
		}

		req := &admin.DeleteUnmutedFeedRequest{
			Username:      user,
			UnmutedFeedId: idsToDelete,
		}

		_, err = client.DeleteUnmutedFeed(context.Background(), req)
		if err != nil {
			fmt.Printf("Error calling DeleteUnmutedFeed: %v\n", err)
			return
		}

		fmt.Printf("Successfully removed %d feeds from the unmute list for user: %s\n", len(idsToDelete), user)
	},
}

func init() {
	rootCmd.AddCommand(deleteUnmutedFeedsCmd)
	addGrpcAddressFlag(deleteUnmutedFeedsCmd)
	addUserFlag(deleteUnmutedFeedsCmd)
}
