// Package gocoverage answers coverage questions about one Go source range.
// The Go tool names each file of a coverage profile with the module path, so
// this package also matches a repository-relative path against that name.
package gocoverage

import (
	"fmt"
	"strings"

	"golang.org/x/tools/cover"

	"github.com/cgardev/goconduct/pkg/failure"
)

// Profile reports which statements of a repository the test run reached.
type Profile struct {
	blocks map[string][]cover.ProfileBlock
}

// Load reads one Go coverage profile and keys it by repository-relative path.
// The Go tool names each file with the module path, so Load removes that
// prefix. A lookup is then an exact match, which keeps the answer independent
// of the order the profile lists its files in.
func Load(path string, modulePath string) (*Profile, error) {
	profiles, err := cover.ParseProfiles(path)
	if err != nil {
		return nil, failure.DataIntegrity(fmt.Sprintf("parse coverage profile %q", path), err)
	}
	blocks := make(map[string][]cover.ProfileBlock, len(profiles))
	for _, entry := range profiles {
		blocks[repositoryPath(entry.FileName, modulePath)] = entry.Blocks
	}
	return &Profile{blocks: blocks}, nil
}

// repositoryPath removes the module path from one profile file name.
// A file of another module keeps its name, so it never matches a lookup.
func repositoryPath(file string, modulePath string) string {
	normalized := normalizePath(file)
	if modulePath == "" {
		return normalized
	}
	if relative, trimmed := strings.CutPrefix(normalized, modulePath+"/"); trimmed {
		return relative
	}
	return normalized
}

// Files lists the profile file names, as the Go tool wrote them.
func (profile *Profile) Files() []string {
	files := make([]string, 0, len(profile.blocks))
	for file := range profile.blocks {
		files = append(files, file)
	}
	return files
}

// Fraction reports the percentage of covered statements between two lines of
// one file. It reports false when the profile describes no statement of the
// file, which happens when the selected build never compiled it.
func (profile *Profile) Fraction(file string, startLine, endLine int) (float64, bool) {
	blocks := profile.blocksFor(file)
	if len(blocks) == 0 {
		return 0, false
	}
	statements := 0
	covered := 0
	for _, block := range blocks {
		if block.EndLine < startLine || block.StartLine > endLine {
			continue
		}
		statements += block.NumStmt
		if block.Count > 0 {
			covered += block.NumStmt
		}
	}
	if statements == 0 {
		return 0, true
	}
	return 100 * float64(covered) / float64(statements), true
}

// CoversLine reports whether the test run reached one line of one file.
func (profile *Profile) CoversLine(file string, line int) bool {
	for _, block := range profile.blocksFor(file) {
		if line >= block.StartLine && line <= block.EndLine && block.Count > 0 {
			return true
		}
	}
	return false
}

// blocksFor selects the blocks of one file, by its repository-relative path.
func (profile *Profile) blocksFor(file string) []cover.ProfileBlock {
	return profile.blocks[normalizePath(file)]
}

func normalizePath(path string) string {
	return strings.TrimPrefix(strings.ReplaceAll(path, `\`, "/"), "./")
}
