package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/jrupac/goliath/admin"
	"github.com/spf13/cobra"
)

var listSessionsCmd = &cobra.Command{
	Use:   "list-sessions",
	Short: "List the active login sessions for a user",
	Long: `List the sessions a user currently has, most recently used first.

A session is one client's login. It expires only after going unused for the
configured idle window, so a client that syncs regularly keeps its session
indefinitely.

The user agent is whatever the client called itself when it signed in. It is
client-supplied, so treat it as a hint about which device a session belongs to
rather than as proof.`,
	GroupID: "user_pref",
	Run: func(cmd *cobra.Command, args []string) {
		client, conn := getAdminClient(cmd)
		defer conn.Close()

		user := getUser(cmd)
		if user == "" {
			fmt.Println("Command aborted. User is required.")
			return
		}

		res, err := client.ListSessions(context.Background(), &admin.ListSessionsRequest{Username: user})
		if err != nil {
			fmt.Printf("Error calling ListSessions: %v\n", err)
			return
		}

		if len(res.Session) == 0 {
			fmt.Println("No active sessions for user:", user)
			return
		}

		fmt.Printf("Sessions for %s:\n", user)
		for _, s := range res.Session {
			fmt.Printf("\n  %s\n", s.SessionId)
			fmt.Printf("    scheme:     %s\n", formatAuthScheme(s.Scheme))
			fmt.Printf("    created:    %s\n", formatSessionTime(s.CreatedUnixSec))
			fmt.Printf("    last seen:  %s\n", formatSessionTime(s.LastSeenUnixSec))
			fmt.Printf("    user agent: %s\n", orPlaceholder(s.UserAgent, "(none given)"))
			fmt.Printf("    revoke:     goliath-cli revoke-session --user %s --session %s\n", user, s.SessionId)
		}
	},
}

var revokeSessionCmd = &cobra.Command{
	Use:   "revoke-session",
	Short: "Revoke one or all login sessions for a user",
	Long: `Revoke a session, which takes effect immediately: the next request made with
it is refused and the client has to sign in again.

Pass --session with an identifier from list-sessions, or --all to revoke every
session the user has.`,
	GroupID: "user_pref",
	Run: func(cmd *cobra.Command, args []string) {
		client, conn := getAdminClient(cmd)
		defer conn.Close()

		user := getUser(cmd)
		if user == "" {
			fmt.Println("Command aborted. User is required.")
			return
		}

		session, _ := cmd.Flags().GetString("session")
		all, _ := cmd.Flags().GetBool("all")

		if session == "" && !all {
			fmt.Println("Command aborted. Pass --session <id> or --all.")
			return
		}
		if session != "" && all {
			fmt.Println("Command aborted. Pass either --session or --all, not both.")
			return
		}

		req := &admin.RevokeSessionsRequest{Username: user}
		if all {
			req.Target = &admin.RevokeSessionsRequest_All{All: &admin.AllSessions{}}
		} else {
			req.Target = &admin.RevokeSessionsRequest_SessionId{SessionId: session}
		}

		res, err := client.RevokeSessions(context.Background(), req)
		if err != nil {
			fmt.Printf("Error calling RevokeSessions: %v\n", err)
			return
		}

		if res.RevokedCount == 0 {
			fmt.Println("No sessions were revoked; nothing matched.")
			return
		}
		fmt.Printf("Revoked %d session(s) for %s.\n", res.RevokedCount, user)
	},
}

var changePasswordCmd = &cobra.Command{
	Use:   "change-password",
	Short: "Change a user's password",
	Long: `Change a user's password, prompting for the new one without echoing it.

This resets everything derived from the password, so every client has to be
signed in again:

  - The Fever API key changes, because it is defined as md5(username:password).
    Fever clients store that key and will need reconfiguring, not just a new
    password.
  - The web session cookie carries the same value and stops working.
  - GReader tokens issued before sessions existed are derived from the
    password hash and stop working.
  - Every session is revoked.

There is no way to change the password without this happening: the Fever
protocol fixes how its key is derived, so the key cannot outlive the password
it is built from.`,
	GroupID: "user_pref",
	Run: func(cmd *cobra.Command, args []string) {
		client, conn := getAdminClient(cmd)
		defer conn.Close()

		user := getUser(cmd)
		if user == "" {
			fmt.Println("Command aborted. User is required.")
			return
		}

		fmt.Printf("Changing the password for %s will sign out every client and\n", user)
		fmt.Println("change the Fever API key, which Fever clients store and will need again.")
		fmt.Println()

		password := promptForPassword("New password:")
		if password == "" {
			fmt.Println("Command aborted. Password is required.")
			return
		}
		if confirm := promptForPassword("Confirm new password:"); confirm != password {
			fmt.Println("Command aborted. The passwords did not match.")
			return
		}

		res, err := client.ChangePassword(context.Background(), &admin.ChangePasswordRequest{
			Username: user,
			Password: password,
		})
		if err != nil {
			fmt.Printf("Error calling ChangePassword: %v\n", err)
			return
		}

		fmt.Printf("\nPassword changed for %s; revoked %d session(s).\n", user, res.RevokedSessionCount)
		fmt.Println("Sign in again on each client. Fever clients need the new API key,")
		fmt.Println("which is md5 of \"username:password\" for the new password.")
	},
}

func init() {
	rootCmd.AddCommand(listSessionsCmd)
	addGrpcAddressFlag(listSessionsCmd)
	addUserFlag(listSessionsCmd)

	rootCmd.AddCommand(revokeSessionCmd)
	addGrpcAddressFlag(revokeSessionCmd)
	addUserFlag(revokeSessionCmd)
	revokeSessionCmd.Flags().String("session", "", "Identifier of the session to revoke (see list-sessions)")
	revokeSessionCmd.Flags().Bool("all", false, "Revoke every session belonging to the user")

	rootCmd.AddCommand(changePasswordCmd)
	addGrpcAddressFlag(changePasswordCmd)
	addUserFlag(changePasswordCmd)
}

func formatSessionTime(unixSec int64) string {
	t := time.Unix(unixSec, 0)
	return fmt.Sprintf("%s (%s ago)", t.Format(time.RFC3339), time.Since(t).Round(time.Minute))
}

// formatAuthScheme renders a scheme for display, trading the wire enum's
// spelling for the one the rest of the system uses.
func formatAuthScheme(scheme admin.AuthScheme) string {
	switch scheme {
	case admin.AuthScheme_AUTH_SCHEME_GREADER:
		return "greader"
	case admin.AuthScheme_AUTH_SCHEME_WEB:
		return "web"
	default:
		return "(unrecognized)"
	}
}

func orPlaceholder(value, placeholder string) string {
	if value == "" {
		return placeholder
	}
	return value
}
