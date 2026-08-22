// Package gosource lists the production Go files of one repository scope.
// Several quality plugins analyze the same file set, so they share this walk.
package gosource

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"

	"github.com/cgardev/goconduct/failure"
)

// ignoredDirectories hold generated output, dependencies, or version control
// data. No analysis of this repository reads them.
var ignoredDirectories = []string{".git", "node_modules", "target", "testdata", "vendor"}

// Files lists the production Go files under root, relative to root and with
// forward slashes. An empty selection lists every production file.
// Each selected path must exist inside the repository.
func Files(root string, selected []string) ([]string, error) {
	scopes, err := Scopes(root, selected)
	if err != nil {
		return nil, err
	}
	files := make([]string, 0)
	walkError := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkError error) error {
		if walkError != nil {
			return walkError
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if entry.IsDir() {
			if relative != "." && ignoredDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if productionFile(relative) && withinScopes(relative, scopes) {
			files = append(files, relative)
		}
		return nil
	})
	if walkError != nil {
		return nil, failure.Unavailable("discover Go source files", walkError)
	}
	slices.Sort(files)
	return files, nil
}

// Scopes normalizes each selected path against the repository root.
// It returns no scope when the selection covers the complete repository.
func Scopes(root string, selected []string) ([]string, error) {
	scopes := make([]string, 0, len(selected))
	for _, candidate := range selected {
		if strings.TrimSpace(candidate) == "" {
			return nil, failure.Validation("analysis path is empty", nil)
		}
		if filepath.IsAbs(candidate) {
			return nil, failure.Validation(fmt.Sprintf(
				"analysis path %q is not repository-relative",
				candidate,
			), nil)
		}
		cleaned := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(candidate)), "./")
		if cleaned == "." {
			return nil, nil
		}
		if strings.HasPrefix(cleaned, "..") {
			return nil, failure.Validation(fmt.Sprintf(
				"analysis path %q is outside the repository",
				candidate,
			), nil)
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(cleaned))); err != nil {
			return nil, failure.Validation(fmt.Sprintf("inspect analysis path %q", candidate), err)
		}
		scopes = append(scopes, cleaned)
	}
	return scopes, nil
}

func withinScopes(relative string, scopes []string) bool {
	if len(scopes) == 0 {
		return true
	}
	for _, scope := range scopes {
		if relative == scope || strings.HasPrefix(relative, scope+"/") {
			return true
		}
	}
	return false
}

func productionFile(relative string) bool {
	return strings.HasSuffix(relative, ".go") && !strings.HasSuffix(relative, "_test.go")
}

func ignoredDirectory(name string) bool {
	return strings.HasPrefix(name, ".") || slices.Contains(ignoredDirectories, name)
}

// ModulePath reads the module declaration of one repository.
// A Go coverage profile names each file with that path, so a reader needs the
// declaration to recover the repository-relative path.
func ModulePath(root string) (string, error) {
	payload, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", failure.Validation("read the repository module file", err)
	}
	modulePath := modfile.ModulePath(payload)
	if modulePath == "" {
		return "", failure.DataIntegrity("the module file declares no module path", nil)
	}
	return modulePath, nil
}
