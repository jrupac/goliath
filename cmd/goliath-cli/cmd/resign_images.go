package cmd

import (
	"context"
	"fmt"

	"github.com/jrupac/goliath/admin"
	"github.com/spf13/cobra"
)

var resignImagesCmd = &cobra.Command{
	Use:   "resign-images",
	Short: "Re-sign image proxy URLs in stored articles",
	Long: `Bring image proxy URLs in already-stored articles up to date with the current
signing key.

The image proxy only fetches URLs Goliath signed, which is what keeps it from
fetching anything it is handed. Articles stored before signing existed, or under
a different key, name targets the proxy now refuses, so their images appear
broken. This signs those URLs in place.

Signing depends only on the target and the key, so running this more than once
changes nothing further. Run it after enabling image proxying on a database that
already holds articles, and after rotating the key.

Use --dry-run first to see how much would change.

Example:
  goliath-cli resign-images --user someone --dry-run`,
	GroupID: "user_pref",
	Run: func(cmd *cobra.Command, args []string) {
		client, conn := getAdminClient(cmd)
		defer conn.Close()

		user := getUser(cmd)
		if user == "" {
			fmt.Println("Command aborted. User is required.")
			return
		}

		dryRun, _ := cmd.Flags().GetBool("dry-run")

		res, err := client.ResignProxiedImages(context.Background(), &admin.ResignProxiedImagesRequest{
			Username: user,
			DryRun:   dryRun,
		})
		if err != nil {
			fmt.Printf("Error calling ResignProxiedImages: %v\n", err)
			return
		}

		verb := "Signed"
		if dryRun {
			verb = "Would sign"
		}
		fmt.Printf("%s %d image URL(s) across %d of %d article(s) for %s.\n",
			verb, res.UrlsSigned, res.ArticlesUpdated, res.ArticlesScanned, user)

		if dryRun && res.UrlsSigned > 0 {
			fmt.Println("\nRe-run without --dry-run to apply.")
		}
	},
}

func init() {
	rootCmd.AddCommand(resignImagesCmd)
	addGrpcAddressFlag(resignImagesCmd)
	addUserFlag(resignImagesCmd)
	resignImagesCmd.Flags().Bool("dry-run", false, "Report what would change without writing")
}
