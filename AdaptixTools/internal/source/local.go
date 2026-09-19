package source

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type LocalResolver struct {
	Path string
}

func (r *LocalResolver) Resolve(_ context.Context) ([]Resolved, func(), error) {
	if _, err := os.Stat(filepath.Join(r.Path, specFileName)); err != nil {
		return nil, nil, fmt.Errorf("local source %q has no %s: %w", r.Path, specFileName, err)
	}
	return []Resolved{{
		LocalDir: r.Path,
		Origin:   r.Path,
	}}, nil, nil
}

type BulkResolver struct {
	Root string
}

func (r *BulkResolver) Resolve(_ context.Context) ([]Resolved, func(), error) {
	dirs, err := findPluginDirs(r.Root)
	if err != nil {
		return nil, nil, err
	}
	out := make([]Resolved, 0, len(dirs))
	for _, d := range dirs {
		out = append(out, Resolved{LocalDir: d, Origin: d})
	}
	return out, nil, nil
}

func hasSpec(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, specFileName))
	return err == nil && !st.IsDir()
}

func findPluginDirs(root string) ([]string, error) {
	return findPluginDirsLimited(root, 3)
}

func findPluginDirsLimited(root string, unwrap int) ([]string, error) {
	if hasSpec(root) {
		return []string{root}, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("scan %s: %w", root, err)
	}
	var dirs []string
	var only string
	nDirs := 0
	for _, e := range entries {
		if !e.IsDir() || skipDirName(e.Name()) {
			continue
		}
		child := filepath.Join(root, e.Name())
		nDirs++
		only = child
		if hasSpec(child) {
			dirs = append(dirs, child)
		}
	}
	if len(dirs) > 0 {
		return dirs, nil
	}
	if nDirs == 1 && unwrap > 0 {
		return findPluginDirsLimited(only, unwrap-1)
	}
	return nil, fmt.Errorf("directory %s contains no %s (expected a plugin dir or archive of plugin dirs)", root, specFileName)
}

func skipDirName(name string) bool {
	return name == "__MACOSX" || name == ".DS_Store" || (len(name) > 0 && name[0] == '.')
}
