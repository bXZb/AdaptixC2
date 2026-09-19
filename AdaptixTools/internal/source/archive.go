package source

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxArchiveFiles = 50_000
	maxArchiveBytes = 2 << 30
)

type ArchiveResolver struct {
	Path string
}

func isArchiveName(path string) bool {
	n := strings.ToLower(path)
	return strings.HasSuffix(n, ".zip") ||
		strings.HasSuffix(n, ".tar") ||
		strings.HasSuffix(n, ".tar.gz") ||
		strings.HasSuffix(n, ".tgz")
}

func (r *ArchiveResolver) Resolve(_ context.Context) ([]Resolved, func(), error) {
	tempDir, err := os.MkdirTemp("", "axtool-archive-*")
	if err != nil {
		return nil, nil, fmt.Errorf("create temp dir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }

	if err := extractArchive(r.Path, tempDir); err != nil {
		cleanup()
		return nil, nil, err
	}
	dirs, err := findPluginDirs(tempDir)
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("archive %s: %w", r.Path, err)
	}
	out := make([]Resolved, 0, len(dirs))
	for _, d := range dirs {
		origin := r.Path
		if rel, rerr := filepath.Rel(tempDir, d); rerr == nil && rel != "." {
			origin = r.Path + "#" + filepath.ToSlash(rel)
		}
		out = append(out, Resolved{LocalDir: d, Origin: origin})
	}
	return out, cleanup, nil
}

func extractArchive(src, dest string) error {
	n := strings.ToLower(src)
	switch {
	case strings.HasSuffix(n, ".zip"):
		return extractZip(src, dest)
	case strings.HasSuffix(n, ".tar.gz"), strings.HasSuffix(n, ".tgz"):
		return extractTar(src, dest, true)
	case strings.HasSuffix(n, ".tar"):
		return extractTar(src, dest, false)
	default:
		return fmt.Errorf("unsupported archive %s (use .zip, .tar, .tar.gz, .tgz)", src)
	}
}

func extractZip(src, dest string) error {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("open zip %s: %w", src, err)
	}
	defer zr.Close()

	var files int
	var bytes int64
	for _, f := range zr.File {
		files++
		if files > maxArchiveFiles {
			return fmt.Errorf("zip %s: too many files", src)
		}
		target, err := safeArchivePath(dest, f.Name)
		if err != nil {
			return err
		}
		if strings.HasSuffix(f.Name, "/") || f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		n, err := writeLimitedFile(target, rc, f.Mode(), &bytes)
		_ = rc.Close()
		if err != nil {
			return fmt.Errorf("zip %s: %w", f.Name, err)
		}
		_ = n
	}
	return nil
}

func extractTar(src, dest string, gz bool) error {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open tar %s: %w", src, err)
	}
	defer f.Close()

	var r io.Reader = f
	if gz {
		gr, err := gzip.NewReader(f)
		if err != nil {
			return fmt.Errorf("gzip %s: %w", src, err)
		}
		defer gr.Close()
		r = gr
	}
	tr := tar.NewReader(r)
	var files int
	var bytes int64
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("tar %s: %w", src, err)
		}
		files++
		if files > maxArchiveFiles {
			return fmt.Errorf("tar %s: too many files", src)
		}
		name := hdr.Name
		if hdr.Typeflag == tar.TypeDir || strings.HasSuffix(name, "/") {
			target, err := safeArchivePath(dest, name)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		target, err := safeArchivePath(dest, name)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if _, err := writeLimitedFile(target, tr, hdr.FileInfo().Mode(), &bytes); err != nil {
			return fmt.Errorf("tar %s: %w", name, err)
		}
	}
}

func writeLimitedFile(path string, r io.Reader, mode os.FileMode, total *int64) (int64, error) {
	if mode == 0 {
		mode = 0o644
	}
	out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return 0, err
	}
	defer out.Close()
	n, err := io.Copy(out, io.LimitReader(r, maxArchiveBytes-*total+1))
	*total += n
	if *total > maxArchiveBytes {
		return n, fmt.Errorf("uncompressed size exceeds %d bytes", maxArchiveBytes)
	}
	return n, err
}

func safeArchivePath(dest, name string) (string, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.TrimPrefix(name, "/")
	if name == "" || name == "." {
		return dest, nil
	}
	target := filepath.Join(dest, filepath.FromSlash(name))
	rel, err := filepath.Rel(dest, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive path escapes destination: %q", name)
	}
	return target, nil
}
