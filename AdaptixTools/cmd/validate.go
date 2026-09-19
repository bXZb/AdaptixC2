package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"axtool/internal/spec"
)

var validateCmd = &cobra.Command{
	Use:   "validate <path>",
	Short: "Validate an axtool.spec file",
	Long: `Validate the plugin spec (axtool.spec) found in the given directory.

Reports any schema or consistency errors without writing anything.`,
	Args: cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		abs, err := filepath.Abs(args[0])
		if err != nil {
			return err
		}
		pl, err := spec.LoadPlugin(abs)
		if err != nil {
			return err
		}
		out := c.OutOrStdout()
		fmt.Fprintf(out, "valid: %d extender(s)\n", len(pl.Extenders))
		for _, e := range pl.Extenders {
			fmt.Fprintf(out, "  - %s %s (%s)\n", e.Name, e.Version, e.Type)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
