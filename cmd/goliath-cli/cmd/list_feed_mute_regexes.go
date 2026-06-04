package cmd

import (
	"context"
	"fmt"

	"github.com/jrupac/goliath/admin"
	"github.com/spf13/cobra"
)

var listFeedMuteRegexesCmd = &cobra.Command{
	Use:     "list-feed-mute-regexes",
	Short:   "List feed-specific mute regexes for a user",
	GroupID: "user_pref",
	Run: func(cmd *cobra.Command, args []string) {
		client, conn := getAdminClient(cmd)
		defer conn.Close()

		user := getUser(cmd)
		if user == "" {
			fmt.Println("Command aborted. User is required.")
			return
		}

		rulesRes, err := client.GetFeedMuteRegexes(context.Background(), &admin.GetFeedMuteRegexesRequest{Username: user})
		if err != nil {
			fmt.Printf("Error fetching feed mute regexes: %v\n", err)
			return
		}

		if len(rulesRes.Rules) == 0 {
			fmt.Println("No feed mute regexes found for user:", user)
			return
		}

		feedsRes, err := client.GetFeeds(context.Background(), &admin.GetFeedsRequest{Username: user})
		if err != nil {
			fmt.Printf("Error fetching feeds: %v\n", err)
			return
		}

		feedIDToTitle := make(map[int64]string)
		for _, f := range feedsRes.Feeds {
			feedIDToTitle[f.Id] = f.Title
		}

		// Group regexes by feed ID
		groupedRegexes := make(map[int64][]string)
		for _, rule := range rulesRes.Rules {
			groupedRegexes[rule.FeedId] = append(groupedRegexes[rule.FeedId], rule.Regex)
		}

		fmt.Printf("Feed-specific mute regexes for user: %s\n\n", user)
		for feedID, regexes := range groupedRegexes {
			title, ok := feedIDToTitle[feedID]
			if !ok {
				title = "Unknown Feed"
			}
			fmt.Printf("%s (ID: %d):\n", title, feedID)
			for _, r := range regexes {
				fmt.Printf("  - %s\n", r)
			}
			fmt.Println()
		}
	},
}

func init() {
	rootCmd.AddCommand(listFeedMuteRegexesCmd)
	addGrpcAddressFlag(listFeedMuteRegexesCmd)
	addUserFlag(listFeedMuteRegexesCmd)
}
