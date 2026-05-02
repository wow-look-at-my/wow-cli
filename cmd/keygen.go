package cmd

import (
	"fmt"

	"filippo.io/age"
	"github.com/spf13/cobra"
)

var keygenCmd = &cobra.Command{
	Use:   "keygen",
	Short: "Generate an age X25519 keypair for signing manifests",
	Long: `Generate a fresh age X25519 keypair and print both halves to stdout.

Use the recipient (age1...) as the WOW_MANIFEST_RECIPIENT secret in your CI
that publishes the manifest. Share the identity (AGE-SECRET-KEY-...) with
users who should be able to decrypt the manifest; they pass it to
"wow add-src".`,
	Args: cobra.NoArgs,
	RunE: runKeygen,
}

func init() {
	rootCmd.AddCommand(keygenCmd)
}

func runKeygen(cmd *cobra.Command, _ []string) error {
	id, err := age.GenerateX25519Identity()
	if err != nil {
		return fmt.Errorf("generate keypair: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "# recipient (publish to CI as WOW_MANIFEST_RECIPIENT):\n%s\n", id.Recipient())
	fmt.Fprintf(cmd.OutOrStdout(), "# identity (share with users for `wow add-src`):\n%s\n", id)
	return nil
}
