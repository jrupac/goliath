package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/jrupac/goliath/admin"
	"github.com/spf13/cobra"
)

var addUserCmd = &cobra.Command{
	Use:   "add-user",
	Short: "Add a user",
	Long: `Add a user, prompting for their password without echoing it.

The new user starts with no feeds. They can sign in to the web client and to
GReader clients with the username and password; Fever clients take the API
key, which is md5 of "username:password".

A username is ASCII letters and digits, with '.', '_' and '-' allowed after
the first character, and may not differ from an existing one only in case --
including a deleted user's, until they are purged.`,
	GroupID: "user_pref",
	Run: func(cmd *cobra.Command, args []string) {
		client, conn := getAdminClient(cmd)
		defer conn.Close()

		user := getUser(cmd)
		if user == "" {
			fmt.Println("Command aborted. User is required.")
			return
		}

		password := promptForPassword("Password:")
		if password == "" {
			fmt.Println("Command aborted. Password is required.")
			return
		}
		if confirm := promptForPassword("Confirm password:"); confirm != password {
			fmt.Println("Command aborted. The passwords did not match.")
			return
		}

		res, err := client.AddUser(context.Background(), &admin.AddUserRequest{
			Username: user,
			Password: password,
		})
		if err != nil {
			fmt.Printf("Error calling AddUser: %v\n", err)
			return
		}
		fmt.Printf("Added %s (%s).\n", user, res.UserId)
	},
}

var listUsersCmd = &cobra.Command{
	Use:     "list-users",
	Short:   "List every user",
	Long:    `List every user with how many feeds, folders, articles and sessions each has.`,
	GroupID: "user_pref",
	Run: func(cmd *cobra.Command, args []string) {
		client, conn := getAdminClient(cmd)
		defer conn.Close()

		res, err := client.ListUsers(context.Background(), &admin.ListUsersRequest{})
		if err != nil {
			fmt.Printf("Error calling ListUsers: %v\n", err)
			return
		}
		if len(res.User) == 0 {
			fmt.Println("No users.")
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.AlignRight)
		fmt.Fprintln(w, "USERNAME\tFEEDS\tFOLDERS\tARTICLES\tUNREAD\tSESSIONS\tDELETED\tID\t")
		for _, u := range res.User {
			deleted := "-"
			if u.DeletedUnixSec != 0 {
				deleted = time.Unix(u.DeletedUnixSec, 0).Format(time.DateTime)
			}
			fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\t%d\t%s\t%s\t\n", u.Username, u.FeedCount, u.FolderCount,
				u.ArticleCount, u.UnreadCount, u.SessionCount, deleted, u.UserId)
		}
		w.Flush()
	},
}

var deleteUserCmd = &cobra.Command{
	Use:   "delete-user",
	Short: "Delete a user, restorable until purged",
	Long: `Delete a user. They are signed out everywhere and refused from their next
request, and their feeds stop being fetched, but their feeds, folders and
articles are kept until the garbage collector purges them once the server's
deletedUserRetention has passed. Until then restore-user brings them back.

Asks for the username to be typed again.`,
	GroupID: "user_pref",
	Run: func(cmd *cobra.Command, args []string) {
		client, conn := getAdminClient(cmd)
		defer conn.Close()

		user := getUser(cmd)
		if user == "" {
			fmt.Println("Command aborted. User is required.")
			return
		}

		fmt.Printf("This signs %s out and deletes them; their data is purged after the retention window.\n", user)
		if confirm := promptForInput("Type the username again to confirm:"); confirm != user {
			fmt.Println("Command aborted. The username did not match.")
			return
		}

		res, err := client.DeleteUser(context.Background(), &admin.DeleteUserRequest{Username: user})
		if err != nil {
			fmt.Printf("Error calling DeleteUser: %v\n", err)
			return
		}
		fmt.Printf("Deleted %s, holding %d feed(s) and %d article(s); revoked %d session(s).\n",
			user, res.FeedCount, res.ArticleCount, res.SessionCount)
		fmt.Printf("Purged after %s. Until then:\n  goliath-cli restore-user --user %s\n",
			time.Unix(res.PurgeAfterUnixSec, 0).Format(time.DateTime), user)
	},
}

var restoreUserCmd = &cobra.Command{
	Use:   "restore-user",
	Short: "Restore a deleted user who has not yet been purged",
	Long: `Restore a deleted user with everything they had, and resume fetching their
feeds. Their sessions were revoked when they were deleted, so each client has to
sign in again.`,
	GroupID: "user_pref",
	Run: func(cmd *cobra.Command, args []string) {
		client, conn := getAdminClient(cmd)
		defer conn.Close()

		user := getUser(cmd)
		if user == "" {
			fmt.Println("Command aborted. User is required.")
			return
		}
		if _, err := client.RestoreUser(context.Background(), &admin.RestoreUserRequest{Username: user}); err != nil {
			fmt.Printf("Error calling RestoreUser: %v\n", err)
			return
		}
		fmt.Printf("Restored %s. Each client has to sign in again.\n", user)
	},
}

func init() {
	rootCmd.AddCommand(addUserCmd)
	addGrpcAddressFlag(addUserCmd)
	addUserFlag(addUserCmd)

	rootCmd.AddCommand(listUsersCmd)
	addGrpcAddressFlag(listUsersCmd)

	rootCmd.AddCommand(deleteUserCmd)
	addGrpcAddressFlag(deleteUserCmd)
	addUserFlag(deleteUserCmd)

	rootCmd.AddCommand(restoreUserCmd)
	addGrpcAddressFlag(restoreUserCmd)
	addUserFlag(restoreUserCmd)
}
