package workspace

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const DefaultGoVersion = "1.26.5"

func DetectGoVersion(projectRoot, serverDir string) string {
	candidates := []string{
		filepath.Join(serverDir, GoWorkFileName),
		filepath.Join(projectRoot, "AdaptixTools", "go.mod"),
		filepath.Join(serverDir, "go.mod"),
		filepath.Join(projectRoot, "go.mod"),
	}
	for _, p := range candidates {
		v, err := GoVersionFile(p)
		if err != nil || v == "" {
			continue
		}
		return NormalizeGoVersion(v)
	}
	return DefaultGoVersion
}

func GoVersionFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if i := strings.Index(line, "//"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if !strings.HasPrefix(line, "go ") {
			continue
		}
		v := strings.TrimSpace(strings.TrimPrefix(line, "go"))
		if v == "" {
			continue
		}
		return v, nil
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("no go version directive in %s", path)
}

func NormalizeGoVersion(v string) string {
	v = strings.TrimSpace(strings.TrimPrefix(v, "go"))
	if v == "" {
		return DefaultGoVersion
	}
	parts := strings.Split(v, ".")
	if len(parts) == 2 && parts[0] == "1" && parts[1] == "26" {
		return DefaultGoVersion
	}
	if len(parts) == 2 {
		return v + ".0"
	}
	return v
}
