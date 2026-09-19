package spec

import (
	"os"
	"path/filepath"
	"strings"
)

type HostDeps struct {
	Apt []string `yaml:"apt,omitempty"`
}

type ProjectDeps struct {
	Common HostDeps `yaml:"common,omitempty"`
	Server HostDeps `yaml:"server,omitempty"`
	Client HostDeps `yaml:"client,omitempty"`
}

func (d ProjectDeps) AptPackages(server, client bool) []string {
	var out []string
	out = append(out, d.Common.Apt...)
	if server {
		out = append(out, d.Server.Apt...)
	}
	if client {
		out = append(out, d.Client.Apt...)
	}
	return uniqueNonEmpty(out)
}

func (h HostDeps) AptPackages() []string {
	return uniqueNonEmpty(h.Apt)
}

func (p PluginSpec) CollectPluginAptDeps() []string {
	var out []string
	for _, e := range p.Extenders {
		out = append(out, e.Deps.Apt...)
	}

	return uniqueNonEmpty(out)
}

// CollectInstallAptDeps returns host apt packages for a server or client install:
// adaptix.spec deps.common + deps.server|client, plus deps.apt from local packages[]
// that already have an axtool.spec on disk. Git/URL package sources are skipped
// until they exist locally (server build -d picks them up after clone).
func CollectInstallAptDeps(projectRoot string, srv ServerSpec, server, client bool) []string {
	out := srv.Deps.AptPackages(server, client)
	if server {
		out = append(out, CollectPackageAptDeps(projectRoot, srv.Packages)...)
	}
	return uniqueNonEmpty(out)
}

func CollectPackageAptDeps(projectRoot string, refs []PackageRef) []string {
	var out []string
	for _, ref := range refs {
		path, ok := localPackagePath(projectRoot, ref)
		if !ok {
			continue
		}
		pl, err := LoadPlugin(path)
		if err != nil {
			continue
		}
		if ref.Name != "" {
			for _, e := range pl.Extenders {
				if e.Name == ref.Name {
					out = append(out, e.Deps.Apt...)
				}
			}
			continue
		}
		out = append(out, pl.CollectPluginAptDeps()...)
	}
	return uniqueNonEmpty(out)
}

func localPackagePath(projectRoot string, ref PackageRef) (string, bool) {
	src := strings.TrimSpace(ref.Source)
	if src == "" || IsRemoteSource(src) {
		return "", false
	}
	path := src
	if !filepath.IsAbs(path) {
		path = filepath.Join(projectRoot, path)
	}
	if ref.Path != "" {
		path = filepath.Join(path, filepath.FromSlash(ref.Path))
	}
	if _, err := os.Stat(path); err != nil {
		return "", false
	}
	return path, true
}

func IsRemoteSource(src string) bool {
	s := strings.ToLower(strings.TrimSpace(src))
	switch {
	case strings.HasPrefix(s, "http://"), strings.HasPrefix(s, "https://"),
		strings.HasPrefix(s, "git@"), strings.HasPrefix(s, "ssh://"):
		return true
	}
	if strings.HasPrefix(src, ".") || strings.HasPrefix(src, "/") || filepath.IsAbs(src) {
		return false
	}
	return strings.Contains(s, "github.com/") || strings.Contains(s, "gitlab.com/") ||
		strings.Contains(s, "bitbucket.org/")
}

func uniqueNonEmpty(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
