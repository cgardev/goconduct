package gocoverage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cgardev/goconduct/pkg/failure"
)

const sampleProfile = `mode: atomic
example.com/module/internal/order/order.go:10.20,14.2 3 2
example.com/module/internal/order/order.go:16.30,18.2 2 0
example.com/module/internal/empty/empty.go:5.10,5.11 0 0
`

const fixtureModulePath = "example.com/module"

func newProfile(t *testing.T, payload string) *Profile {
	t.Helper()
	path := filepath.Join(t.TempDir(), "coverage.out")
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatalf("write coverage profile: %v", err)
	}
	profile, err := Load(path, fixtureModulePath)
	if err != nil {
		t.Fatalf("load coverage profile: %v", err)
	}
	return profile
}

func TestFractionWeightsEveryStatementOfTheRange(t *testing.T) {
	profile := newProfile(t, sampleProfile)

	testCases := []struct {
		name      string
		file      string
		startLine int
		endLine   int
		want      float64
		measured  bool
	}{
		{
			name: "a range with covered statements only", file: "internal/order/order.go",
			startLine: 10, endLine: 14, want: 100, measured: true,
		},
		{
			name: "a range with uncovered statements only", file: "internal/order/order.go",
			startLine: 16, endLine: 18, want: 0, measured: true,
		},
		{
			name: "a range over both blocks", file: "internal/order/order.go",
			startLine: 1, endLine: 100, want: 60, measured: true,
		},
		{
			name: "a range outside every block", file: "internal/order/order.go",
			startLine: 200, endLine: 210, want: 0, measured: true,
		},
		{
			name: "a file the profile does not describe", file: "internal/absent/absent.go",
			startLine: 1, endLine: 10, want: 0, measured: false,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fraction, measured := profile.Fraction(testCase.file, testCase.startLine, testCase.endLine)
			if measured != testCase.measured {
				t.Fatalf("measured is %t, want %t", measured, testCase.measured)
			}
			if fraction != testCase.want {
				t.Errorf("fraction is %v, want %v", fraction, testCase.want)
			}
		})
	}
}

func TestFractionKeysEveryFileByItsRepositoryPath(t *testing.T) {
	profile := newProfile(t, sampleProfile)

	testCases := []struct {
		name     string
		file     string
		measured bool
	}{
		{name: "the repository-relative path", file: "internal/order/order.go", measured: true},
		{name: "the same path with a leading dot slash", file: "./internal/order/order.go", measured: true},
		{
			name: "the module-qualified name the profile carries",
			file: "example.com/module/internal/order/order.go", measured: false,
		},
		{name: "a path suffix", file: "order/order.go", measured: false},
		{name: "a partial segment", file: "der/order.go", measured: false},
		{name: "an empty name", file: "", measured: false},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, measured := profile.Fraction(testCase.file, 10, 14); measured != testCase.measured {
				t.Errorf("measured is %t, want %t", measured, testCase.measured)
			}
		})
	}
}

func TestLoadKeepsTheNameOfAnotherModule(t *testing.T) {
	profile := newProfile(t, sampleProfile)

	if _, measured := profile.Fraction("internal/order/order.go", 10, 14); !measured {
		t.Fatal("the profile does not describe its own module")
	}
	other, err := Load(writeProfile(t, sampleProfile), "other.example/module")
	if err != nil {
		t.Fatalf("load coverage profile: %v", err)
	}
	if _, measured := other.Fraction("internal/order/order.go", 10, 14); measured {
		t.Error("a file of another module answers a repository-relative lookup")
	}
}

func writeProfile(t *testing.T, payload string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "coverage.out")
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatalf("write coverage profile: %v", err)
	}
	return path
}

func TestCoversLineReportsTheReachedLines(t *testing.T) {
	profile := newProfile(t, sampleProfile)

	testCases := []struct {
		name string
		line int
		want bool
	}{
		{name: "the first line of a covered block", line: 10, want: true},
		{name: "the last line of a covered block", line: 14, want: true},
		{name: "a line of an uncovered block", line: 17, want: false},
		{name: "a line outside every block", line: 500, want: false},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			covered := profile.CoversLine("internal/order/order.go", testCase.line)
			if covered != testCase.want {
				t.Errorf("covered is %t, want %t", covered, testCase.want)
			}
		})
	}
}

func TestCoversLineRejectsAnUnknownFile(t *testing.T) {
	profile := newProfile(t, sampleProfile)

	if profile.CoversLine("internal/absent/absent.go", 10) {
		t.Error("an unknown file reports a covered line")
	}
}

func TestFilesListsEveryDescribedFile(t *testing.T) {
	profile := newProfile(t, sampleProfile)

	if len(profile.Files()) != 2 {
		t.Errorf("the profile lists %v, want two files", profile.Files())
	}
}

func TestLoadClassifiesAnUnreadableProfile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "absent.out"), fixtureModulePath)

	if !errors.Is(err, failure.ErrDataIntegrity) {
		t.Errorf("load error is %v, want a data integrity failure", err)
	}
}

func TestLoadClassifiesAMalformedProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "coverage.out")
	if err := os.WriteFile(path, []byte("mode: atomic\nnot-a-block\n"), 0o600); err != nil {
		t.Fatalf("write coverage profile: %v", err)
	}

	if _, err := Load(path, fixtureModulePath); !errors.Is(err, failure.ErrDataIntegrity) {
		t.Errorf("load error is %v, want a data integrity failure", err)
	}
}

// boundaryProfile places one block that ends exactly where the next begins, and
// one block reached exactly once, so a limit that moves by one becomes visible.
const boundaryProfile = `mode: atomic
example.com/module/internal/order/order.go:10.20,20.2 4 1
example.com/module/internal/order/order.go:21.20,30.2 6 0
`

func TestFractionIncludesTheBlocksThatTouchTheRangeBounds(t *testing.T) {
	profile := newProfile(t, boundaryProfile)

	testCases := []struct {
		name      string
		startLine int
		endLine   int
		want      float64
	}{
		{
			name:      "a range that starts on the last line of the covered block",
			startLine: 20, endLine: 30, want: 40,
		},
		{
			name:      "a range that ends on the first line of the uncovered block",
			startLine: 10, endLine: 21, want: 40,
		},
		{name: "a range over the covered block only", startLine: 10, endLine: 20, want: 100},
		{name: "a range over the uncovered block only", startLine: 21, endLine: 30, want: 0},
		{name: "a range before every block", startLine: 1, endLine: 9, want: 0},
		{name: "a range after every block", startLine: 31, endLine: 40, want: 0},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fraction, measured := profile.Fraction(
				"internal/order/order.go",
				testCase.startLine,
				testCase.endLine,
			)
			if !measured {
				t.Fatal("the profile describes the file but reports no measurement")
			}
			if fraction != testCase.want {
				t.Errorf("fraction is %v, want %v", fraction, testCase.want)
			}
		})
	}
}

func TestFractionCountsABlockReachedExactlyOnce(t *testing.T) {
	profile := newProfile(t, boundaryProfile)

	fraction, measured := profile.Fraction("internal/order/order.go", 10, 20)

	if !measured || fraction != 100 {
		t.Errorf("a block with one hit reports %v, want 100", fraction)
	}
}

func TestCoversLineAcceptsALineReachedExactlyOnce(t *testing.T) {
	profile := newProfile(t, boundaryProfile)

	if !profile.CoversLine("internal/order/order.go", 15) {
		t.Error("a block with one hit reports an unreached line")
	}
}
