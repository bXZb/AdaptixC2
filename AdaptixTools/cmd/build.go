package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"axtool/internal/build"
	"axtool/internal/spec"
)

var buildCmd = &cobra.Command{
	Use:   "build <path>",
	Short: "Build a plugin without installing it",
	Long: `Build one or all plugins declared in the axtool.spec at <path>.

Artifacts are written wherever the plugin's build commands put them; axtool does
not modify the source tree or any server files. Use this to test that a plugin
spec compiles end-to-end before publishing.`,
	Args: cobra.ExactArgs(1),
	RunE: runBuild,
}

func init() {
	rootCmd.AddCommand(buildCmd)
}

func runBuild(c *cobra.Command, args []string) error {
	abs, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}
	out := c.OutOrStderr()
	pl, err := spec.LoadPlugin(abs)
	if err != nil {
		return err
	}
	ctx := context.Background()
	for _, e := range pl.Extenders {
		srcDir := abs
		if e.Source != "" {
			srcDir = filepath.Join(abs, e.Source)
		}
		fmt.Fprintf(out, "[build] %s %s in %s\n", e.Name, e.Version, srcDir)
		runner := &build.Runner{
			Dir: srcDir,
			Env: []string{"GOEXPERIMENT=jsonv2,greenteagc"},
			Out: out,
		}
		if err := runner.Run(ctx, e.Build); err != nil {
			return err
		}
		// Show where the release would be collected from.
		files, err := build.CollectRelease(srcDir, e.Release)
		if err != nil {
			return fmt.Errorf("%s: collect release: %w", e.Name, err)
		}
		fmt.Fprintf(out, "[ok] %s: %d artifact(s)\n", e.Name, len(files))
		for _, f := range files {
			rel, _ := filepath.Rel(srcDir, f)
			fmt.Fprintf(out, "    %s\n", rel)
		}
	}
	_ = os.Stdout
	return nil
}
