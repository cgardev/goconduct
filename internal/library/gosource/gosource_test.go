package gosource

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/cgardev/goconduct/pkg/failure"
)

func newRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"root.go":                       "package sample\n",
		"root_test.go":                  "package sample\n",
		"internal/order/order.go":       "package order\n",
		"internal/order/order_test.go":  "package order\n",
		"internal/order/notes.md":       "notes\n",
		"vendor/other/other.go":         "package other\n",
		"target/generated/generated.go": "package generated\n",
		"testdata/fixture/fixture.go":   "package fixture\n",
		".hidden/hidden.go":             "package hidden\n",
	}
	for name, content := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return root
}

func TestFilesListEveryProductionSource(t *testing.T) {
	root := newRepository(t)

	files, err := Files(root, nil)
	if err != nil {
		t.Fatalf("list Go source files: %v", err)
	}

	want := []string{"internal/order/order.go", "root.go"}
	if !slices.Equal(files, want) {
		t.Errorf("the walk lists %v, want %v", files, want)
	}
}

func TestAllFilesIncludesTestsAndPolicyOwnedDirectories(t *testing.T) {
	root := newRepository(t)

	files, err := AllFiles(root, nil)
	if err != nil {
		t.Fatalf("list every Go source file: %v", err)
	}

	want := []string{
		".hidden/hidden.go",
		"internal/order/order.go",
		"internal/order/order_test.go",
		"root.go",
		"root_test.go",
		"target/generated/generated.go",
		"testdata/fixture/fixture.go",
		"vendor/other/other.go",
	}
	if !slices.Equal(files, want) {
		t.Errorf("the complete walk lists %v, want %v", files, want)
	}
}

func TestAllFilesDeduplicatesOverlappingScopes(t *testing.T) {
	root := newRepository(t)

	files, err := AllFiles(root, []string{"internal", "internal/order", "root_test.go"})
	if err != nil {
		t.Fatalf("list selected Go source files: %v", err)
	}

	want := []string{
		"internal/order/order.go",
		"internal/order/order_test.go",
		"root_test.go",
	}
	if !slices.Equal(files, want) {
		t.Errorf("the selected walk lists %v, want %v", files, want)
	}
}

func TestAllFilesRejectsASelectedNonGoFile(t *testing.T) {
	_, err := AllFiles(newRepository(t), []string{"internal/order/notes.md"})

	if !errors.Is(err, failure.ErrValidation) {
		t.Errorf("the walk error is %v, want a validation failure", err)
	}
}

func TestFilesRestrictTheWalkToTheSelectedScope(t *testing.T) {
	root := newRepository(t)

	testCases := []struct {
		name     string
		selected []string
		want     []string
	}{
		{name: "one directory", selected: []string{"internal/order"}, want: []string{"internal/order/order.go"}},
		{name: "one file", selected: []string{"root.go"}, want: []string{"root.go"}},
		{
			name:     "the repository itself",
			selected: []string{"."},
			want:     []string{"internal/order/order.go", "root.go"},
		},
		{
			name:     "a path with a leading dot slash",
			selected: []string{"./internal/order"},
			want:     []string{"internal/order/order.go"},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			files, err := Files(root, testCase.selected)
			if err != nil {
				t.Fatalf("list Go source files: %v", err)
			}
			if !slices.Equal(files, testCase.want) {
				t.Errorf("the walk lists %v, want %v", files, testCase.want)
			}
		})
	}
}

func TestFilesRejectAnUnusablePath(t *testing.T) {
	root := newRepository(t)

	testCases := []struct {
		name     string
		selected string
	}{
		{name: "a path that does not exist", selected: "absent"},
		{name: "an empty path", selected: "   "},
		{name: "a path outside the repository", selected: "../escape"},
		{name: "an absolute path", selected: "/etc"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := Files(root, []string{testCase.selected})

			if !errors.Is(err, failure.ErrValidation) {
				t.Errorf("the walk error is %v, want a validation failure", err)
			}
		})
	}
}

func TestFilesClassifyAnUnreadableRepository(t *testing.T) {
	_, err := Files(filepath.Join(t.TempDir(), "absent"), nil)

	if !errors.Is(err, failure.ErrUnavailable) {
		t.Errorf("the walk error is %v, want an unavailable failure", err)
	}
}

func TestScopesReportNoScopeForTheCompleteRepository(t *testing.T) {
	scopes, err := Scopes(newRepository(t), []string{"internal/order", "."})
	if err != nil {
		t.Fatalf("read scopes: %v", err)
	}

	if len(scopes) != 0 {
		t.Errorf("the scopes are %v, want none", scopes)
	}
}

func TestModulePathReadsTheModuleDeclaration(t *testing.T) {
	root := t.TempDir()
	module := "module example.com/sample\n\ngo 1.26.3\n\nrequire golang.org/x/tools v0.47.0\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(module), 0o600); err != nil {
		t.Fatalf("write module file: %v", err)
	}

	path, err := ModulePath(root)
	if err != nil {
		t.Fatalf("read module path: %v", err)
	}

	if path != "example.com/sample" {
		t.Errorf("module path is %q, want example.com/sample", path)
	}
}

func TestModulePathRejectsARepositoryWithoutAModuleFile(t *testing.T) {
	_, err := ModulePath(t.TempDir())

	if !errors.Is(err, failure.ErrValidation) {
		t.Errorf("the read error is %v, want a validation failure", err)
	}
}

func TestModulePathRejectsAModuleFileWithoutADeclaration(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("go 1.26.3\n"), 0o600); err != nil {
		t.Fatalf("write module file: %v", err)
	}

	_, err := ModulePath(root)

	if !errors.Is(err, failure.ErrDataIntegrity) {
		t.Errorf("the read error is %v, want a data integrity failure", err)
	}
}
