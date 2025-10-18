package cmd

import (
	"context"
	"fmt"

	"github.com/jrupac/goliath/admin"
	"github.com/spf13/cobra"
)

var listFeedsCmd = &cobra.Command{
	Use:     "list-feeds",
	Short:   "List all feeds for a user",
	GroupID: "admin",
	Run: func(cmd *cobra.Command, args []string) {
		client, conn := getAdminClient(cmd)
		defer conn.Close()

		user := getUser(cmd)
		if user == "" {
			fmt.Println("Command aborted. User is required.")
			return
		}

		req := &admin.GetFeedsRequest{
			Username: user,
		}

		res, err := client.GetFeeds(context.Background(), req)
		if err != nil {
			fmt.Printf("Error calling GetFeeds: %v\n", err)
			return
		}

		if len(res.Feeds) == 0 {
			fmt.Println("No feeds found for user:", user)
			return
		}

		fmt.Println("Feeds for", user, ":")
		for _, feed := range res.Feeds {
			fmt.Printf("  ID: %d, Title: %s\n", feed.Id, feed.Title)
		}
	},
}

func init() {
	rootCmd.AddCommand(listFeedsCmd)
	addGrpcAddressFlag(listFeedsCmd)
	addUserFlag(listFeedsCmd)
}
