package signr

import (
	"github.com/spf13/cobra"
)

// importCmd represents the import command
var importCmd = &cobra.Command{
	Use:   "import <secret key> <name>",
	Short: "Import a secret key",
	Long: `Import a secret key provided in hexadecimal or nostr nsec format.
`,
	Run: func(cmd *cobra.Command, args []string) {

		argLen := len(args)
		if argLen < 2 {
			cmd.Help()
			s.Fatal("error: two parameters required\n")
		}
		if err := s.Import(args[0], args[1]); err != nil {
			s.Fatal("ERROR: while importing: '%s'\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(importCmd)
}
