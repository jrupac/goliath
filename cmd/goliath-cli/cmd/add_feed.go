package cmd

import (
	"context"
	"fmt"

	"github.com/jrupac/goliath/admin"
	"github.com/spf13/cobra"
)

var addFeedCmd = &cobra.Command{
	Use:     "add-feed",
	Short:   "Add a new feed",
	GroupID: "admin",
	Run: func(cmd *cobra.Command, args []string) {
		client, conn := getAdminClient(cmd)
		defer conn.Close()

		user := getUser(cmd)
		title, _ := cmd.Flags().GetString("title")
		description, _ := cmd.Flags().GetString("description")
		url, _ := cmd.Flags().GetString("url")
		link, _ := cmd.Flags().GetString("link")
		folder, _ := cmd.Flags().GetString("folder")

		if title == "" {
			title = promptForInput("Enter Title:")
		}
		if url == "" {
			url = promptForInput("Enter Feed URL:")
		}
		if link == "" {
			link = promptForInput("Enter Homepage URL:")
		}

		// After prompting, if they are still empty, it means the user quit the prompt.
		if user == "" || title == "" || url == "" || link == "" {
			fmt.Println("Command aborted. All required fields must be provided.")
			return
		}

		req := &admin.AddFeedRequest{
			Username:    user,
			Title:       title,
			Description: description,
			URL:         url,
			Link:        link,
			Folder:      folder,
		}

		res, err := client.AddFeed(context.Background(), req)
		if err != nil {
			fmt.Printf("Error calling AddFeed: %v\n", err)
			return
		}

		fmt.Printf("Successfully added feed with ID: %d\n", res.Id)
	},
}

func init() {
	rootCmd.AddCommand(addFeedCmd)
	addGrpcAddressFlag(addFeedCmd)
	addUserFlag(addFeedCmd)
	addFeedCmd.Flags().String("title", "", "Logical title of feed")
	addFeedCmd.Flags().String("description", "", "Description of feed")
	addFeedCmd.Flags().String("url", "", "URL should point to the fetch URL for the feed")
	addFeedCmd.Flags().String("link", "", "URL should point to the logical homepage for the feed")
	addFeedCmd.Flags().String("folder", "", "If set, this new feed will be placed under the folder of the supplied name")
}
