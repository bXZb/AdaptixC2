package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"axtool/internal/install"
	"axtool/internal/source"
	"axtool/internal/spec"
)

var (
	installName          string
	installForce         bool
	installIgnoreVersion bool
	installFromFile      string
	installPackages      bool
	installInstallDeps   bool
)

var installCmd = &cobra.Command{
	Use:   "install [source]",
	Short: "Install plugin(s) or axscript kit(s)",
	Long: `Install from a local directory, a plugin archive, a git URL, or adaptix.spec packages:.

Archives (.zip, .tar, .tar.gz, .tgz) may contain one plugin (axtool.spec at
the root) or several plugin directories. A single wrapper folder is unwrapped.

  axtool adaptix.spec ext install ./plugins.tgz -f
  axtool adaptix.spec ext install ./beacon_agent -f
  axtool adaptix.spec ext install github.com/org/repo@v1 --name beacon_agent`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInstall,
}

func init() {
	installCmd.Flags().StringVar(&installName, "name", "", "install only this name from a multi-item package axtool.spec")
	installCmd.Flags().StringVar(&installPath, "path", "", "subdir inside the repo that holds the package axtool.spec")
	installCmd.Flags().BoolVarP(&installForce, "force", "f", false, "overwrite existing install")
	installCmd.Flags().BoolVar(&installIgnoreVersion, "ignore-version", false, "ignore min_server_version check")
	installCmd.Flags().StringVar(&installFromFile, "from", "", "install all sources listed in a packages YAML file")
	installCmd.Flags().BoolVar(&installPackages, "packages", false, "install all packages: from adaptix.spec")
	installCmd.Flags().BoolVarP(&installInstallDeps, "install-deps", "d", false, "install host apt deps from axtool.spec deps: before build")
	extCmd.AddCommand(installCmd)
}

var installPath string

func runInstall(c *cobra.Command, args []string) error {
	projectRoot, serverDir, srv, err := loadProject()
	if err != nil {
		return err
	}
	out := colorOut(c.OutOrStderr())

	var refs []spec.PackageRef
	switch {
	case installFromFile != "" && installPackages:
		return fmt.Errorf("use either --from or --packages, not both")
	case installFromFile != "" && len(args) > 0:
		return fmt.Errorf("do not pass a source argument with --from")
	case installPackages && len(args) > 0:
		return fmt.Errorf("do not pass a source argument with --packages")
	case installFromFile != "":
		path, err := absCLIPath(installFromFile)
		if err != nil {
			return err
		}
		refs, err = spec.LoadPackagesFile(path)
		if err != nil {
			return err
		}
	case installPackages:
		if len(srv.Packages) == 0 {
			return fmt.Errorf("adaptix.spec has no packages: list")
		}
		refs = srv.Packages
	case len(args) == 1:
		refs = []spec.PackageRef{{
			Source: args[0],
			Path:   installPath,
			Name:   installName,
		}}
	default:
		return fmt.Errorf("provide a source, or --from <file>, or --packages")
	}

	return installPackageRefs(c, projectRoot, serverDir, refs, installOptions{
		Force:           installForce,
		IgnoreVersion:   installIgnoreVersion,
		OnlyName:        installName,
		Path:            installPath,
		InstallHostDeps: installInstallDeps,
		Out:             out,
	})
}

type installOptions struct {
	Force           bool
	IgnoreVersion   bool
	OnlyName        string
	Path            string
	InstallHostDeps bool

	NoProfile bool
	Out       io.Writer
}

func installPackageRefs(c *cobra.Command, projectRoot, serverDir string, refs []spec.PackageRef, opts installOptions) error {
	out := opts.Out
	if out == nil {
		out = colorOut(c.OutOrStderr())
	}
	for i, ref := range refs {
		src := ref.Source
		only := ref.Name
		if only == "" {
			only = opts.OnlyName
		}
		subPath := ref.Path
		if subPath == "" {
			subPath = opts.Path
		}

		inst, err := install.New(install.Options{
			ProjectRoot:       projectRoot,
			ServerDir:         serverDir,
			PluginDirOverride: ref.PluginDir,
			Force:             opts.Force,
			IgnoreVersion:     opts.IgnoreVersion,
			OnlyName:          only,
			InstallHostDeps:   opts.InstallHostDeps,
			NoProfile:         opts.NoProfile,
			Out:               out,
		})
		if err != nil {
			return err
		}

		resolver, err := source.New(src)
		if err != nil {
			return fmt.Errorf("packages[%d] %s: %w", i, src, err)
		}

		resolved, cleanup, err := resolver.Resolve(c.Context())
		if err != nil {
			return fmt.Errorf("packages[%d] %s: %w", i, src, err)
		}
		if cleanup != nil {

			defer cleanup()
		}

		for _, r := range resolved {
			pkgDir := r.LocalDir
			origin := r.Origin
			if subPath != "" {
				pkgDir = filepath.Join(r.LocalDir, filepath.FromSlash(subPath))
				info, err := os.Stat(pkgDir)
				if err != nil || !info.IsDir() {
					return fmt.Errorf("%s: path %q not found in package", origin, subPath)
				}
				origin = origin + "#" + subPath
			}
			pl, err := spec.LoadPlugin(pkgDir)
			if err != nil {
				return fmt.Errorf("%s: %w", origin, err)
			}
			fmt.Fprintf(out, "[source] %s\n", origin)
			if err := inst.Install(c.Context(), pkgDir, origin, r.Commit, pl); err != nil {
				return err
			}
		}
	}
	return nil
}

func installSpecPackages(c *cobra.Command, force bool, noProfile bool) error {
	projectRoot, serverDir, srv, err := loadProject()
	if err != nil {
		return err
	}
	if len(srv.Packages) == 0 {
		fmt.Fprintln(colorOut(c.OutOrStderr()), "[packages] none listed in adaptix.spec (skip)")
		return nil
	}
	out := colorOut(c.OutOrStderr())
	fmt.Fprintf(out, "[packages] installing %d package(s) from adaptix.spec\n", len(srv.Packages))
	return installPackageRefs(c, projectRoot, serverDir, srv.Packages, installOptions{
		Force:     force,
		NoProfile: noProfile,
		Out:       out,
	})
}
