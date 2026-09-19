package source

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Resolved struct {
	LocalDir string
	Origin   string
	Commit   string
}

type Resolver interface {
	Resolve(ctx context.Context) (resolved []Resolved, cleanup func(), err error)
}

func New(source string) (Resolver, error) {
	if source == "" {
		return nil, errors.New("empty source")
	}
	abs, absErr := filepath.Abs(source)
	if absErr == nil {
		if info, err := os.Stat(abs); err == nil {
			if !info.IsDir() {
				if isArchiveName(abs) {
					return &ArchiveResolver{Path: abs}, nil
				}
				return nil, fmt.Errorf("local source %q is not a directory or plugin archive (.zip/.tar/.tar.gz/.tgz)", abs)
			}
			if hasSpec(abs) {
				return &LocalResolver{Path: abs}, nil
			}
			return &BulkResolver{Root: abs}, nil
		}
	}
	if isGitSource(source) {
		return &GitResolver{Source: source}, nil
	}
	if absErr != nil {
		return nil, fmt.Errorf("resolve local path %q: %w", source, absErr)
	}
	return nil, fmt.Errorf("stat local source %q: %w", source, os.ErrNotExist)
}

func isGitSource(s string) bool {

	base, _ := splitRef(s)

	if strings.HasPrefix(base, "github.com/") {
		return true
	}
	if strings.HasPrefix(base, "https://") || strings.HasPrefix(base, "http://") || strings.HasPrefix(base, "file://") {
		return strings.HasSuffix(base, ".git") || strings.Contains(base, "/-/") || strings.Contains(base, "gitlab.") || strings.Contains(base, "github.") || strings.HasPrefix(base, "file://")
	}
	if strings.HasPrefix(base, "git@") {
		return true
	}
	if strings.HasSuffix(base, ".git") {
		return true
	}
	return false
}

const specFileName = "axtool.spec"
