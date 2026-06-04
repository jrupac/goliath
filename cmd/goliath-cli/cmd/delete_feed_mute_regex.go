package cmd

import (
	"context"
	"fmt"

	"github.com/jrupac/goliath/admin"
	"github.com/spf13/cobra"
)

var deleteFeedMuteRegexCmd = &cobra.Command{
	Use:     "delete-feed-mute-regex",
	Short:   "Delete one or more feed-specific mute regexes",
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

		// If both feed-id and regex are provided, perform direct deletion
		if feedID != 0 && regex != "" {
			req := &admin.DeleteFeedMuteRegexRequest{
				Username: user,
				FeedId:   feedID,
				Regex:    regex,
			}

			_, err := client.DeleteFeedMuteRegex(context.Background(), req)
			if err != nil {
				fmt.Printf("Error deleting feed mute regex: %v\n", err)
				return
			}

			fmt.Printf("Successfully deleted mute regex %q for feed %d for user: %s\n", regex, feedID, user)
			return
		}

		// Otherwise, retrieve all existing rules and present checklist
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

		var choices []string
		choiceToRule := make(map[string]*admin.FeedMuteRegexRule)

		for _, rule := range rulesRes.Rules {
			title, ok := feedIDToTitle[rule.FeedId]
			if !ok {
				title = "Unknown Feed"
			}
			choice := fmt.Sprintf("%s (ID: %d): %s", title, rule.FeedId, rule.Regex)
			choices = append(choices, choice)
			choiceToRule[choice] = rule
		}

		selectedChoices := promptForChecklist("Select feed mute regexes to delete:", choices)
		if len(selectedChoices) == 0 {
			fmt.Println("No regexes selected. Aborting.")
			return
		}

		successCount := 0
		for _, choice := range selectedChoices {
			rule := choiceToRule[choice]
			req := &admin.DeleteFeedMuteRegexRequest{
				Username: user,
				FeedId:   rule.FeedId,
				Regex:    rule.Regex,
			}

			_, err := client.DeleteFeedMuteRegex(context.Background(), req)
			if err != nil {
				fmt.Printf("Error deleting regex %q for feed ID %d: %v\n", rule.Regex, rule.FeedId, err)
				continue
			}
			successCount++
		}

		fmt.Printf("Successfully deleted %d feed mute regexes for user: %s\n", successCount, user)
	},
}

func init() {
	rootCmd.AddCommand(deleteFeedMuteRegexCmd)
	addGrpcAddressFlag(deleteFeedMuteRegexCmd)
	addUserFlag(deleteFeedMuteRegexCmd)
	deleteFeedMuteRegexCmd.Flags().Int64("feed-id", 0, "Feed ID to delete regex from")
	deleteFeedMuteRegexCmd.Flags().String("regex", "", "Regex pattern to delete")
}
