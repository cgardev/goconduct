package gosimilarity

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cgardev/goconduct/pkg/failure"
)

// alphaSource and betaSource come from the published description of the method.
// Both functions differ in their names, their local names, their predicate, and
// their literal values, and the method still scores them at 1.
const alphaSource = `package sample

func Alpha(xs []int) []int {
	var ys []int
	for _, x := range xs {
		if x%2 == 1 {
			ys = append(ys, x+1)
		}
	}
	return ys
}
`

const betaSource = `package sample

func Beta(items []int) []int {
	var kept []int
	for _, item := range items {
		if item%2 == 0 {
			kept = append(kept, item+1)
		}
	}
	return kept
}
`

// gammaSource adds one statement to the same shape, so it shares most of the
// structure without reaching a score of 1.
const gammaSource = `package sample

func Gamma(items []int) []int {
	var kept []int
	for _, item := range items {
		if item%2 == 0 {
			kept = append(kept, item+1)
		}
	}
	kept = append(kept, 7)
	return kept
}
`

// declarationSource holds one type declaration and one function without a body.
// The scan skips both and still reads the function that follows them.
const declarationSource = `package sample

type Collection struct{}

func Declared(values []int) []int

func Delta(items []int) []int {
	var kept []int
	for _, item := range items {
		if item%2 == 0 {
			kept = append(kept, item+1)
		}
	}
	return kept
}
`

// shortSource holds 3 source lines and 12 normalized nodes.
const shortSource = `package sample

func Short(a int) int {
	return a + 1
}
`

// alphaLines and alphaNodes are the measured size of alphaSource.
// The boundary tests select exactly these two values.
const (
	alphaLines = 9
	alphaNodes = 39
)

// alphaGammaScore is the measured similarity of alphaSource and gammaSource.
// Both functions share 23 fingerprints of the 30 that they hold together.
const alphaGammaScore = 23.0 / 30.0

func writeSource(t *testing.T, directory string, name string, content string) string {
	t.Helper()
	path := filepath.Join(directory, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create the directory of %q: %v", name, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %q: %v", name, err)
	}
	return path
}

// sampleTree writes one file for each name of sources and returns the root.
func sampleTree(t *testing.T, sources map[string]string) string {
	t.Helper()
	directory := t.TempDir()
	for name, content := range sources {
		writeSource(t, directory, name, content)
	}
	return directory
}

// summary names one candidate by its two locations and its score.
func summary(candidate Candidate) string {
	return fmt.Sprintf(
		"%s:%d-%d %s:%d-%d %.4f",
		filepath.Base(candidate.Left.File),
		candidate.Left.StartLine,
		candidate.Left.EndLine,
		filepath.Base(candidate.Right.File),
		candidate.Right.StartLine,
		candidate.Right.EndLine,
		candidate.Score,
	)
}

func summaries(candidates []Candidate) []string {
	reported := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		reported = append(reported, summary(candidate))
	}
	return reported
}

// renderShape writes one normalized tree as nested parentheses.
// The test compares that text, so a wrong tag or a wrong child order fails.
func renderShape(current shape) string {
	var builder strings.Builder
	builder.WriteString("(")
	builder.WriteString(current.tag)
	for _, child := range current.children {
		builder.WriteString(" ")
		builder.WriteString(renderShape(child))
	}
	builder.WriteString(")")
	return builder.String()
}

// firstFunction parses one sample source and returns its first declaration.
func firstFunction(t *testing.T, source string) *ast.FuncDecl {
	t.Helper()
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "sample.go", source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse the sample source: %v", err)
	}
	for _, declaration := range file.Decls {
		if declared, isFunction := declaration.(*ast.FuncDecl); isFunction {
			return declared
		}
	}
	t.Fatalf("the sample source declares no function")
	return nil
}

// bodyShape renders the normalized body of one statement sample.
func bodyShape(t *testing.T, body string) string {
	t.Helper()
	declared := firstFunction(t, "package sample\n\nfunc sample() {\n"+body+"\n}\n")
	return renderShape(normalize(declared.Body))
}

// fingerprintSet builds one sorted fingerprint set from leaf tags.
func fingerprintSet(t *testing.T, tags ...string) []fingerprint {
	t.Helper()
	set := make([]fingerprint, 0, len(tags))
	for _, tag := range tags {
		digest, _ := fingerprintTree(newShape(tag), nil)
		set = append(set, digest)
	}
	slices.SortFunc(set, compareFingerprint)
	return slices.Compact(set)
}

func TestCandidatesScoresOneWhenOnlyTheNamesAndTheValuesDiffer(t *testing.T) {
	directory := sampleTree(t, map[string]string{"alpha.go": alphaSource, "beta.go": betaSource})

	candidates, err := Candidates(t.Context(), []string{directory}, 0.82, 4, 20)
	if err != nil {
		t.Fatalf("read candidates: %v", err)
	}

	want := []string{"alpha.go:3-11 beta.go:3-11 1.0000"}
	if reported := summaries(candidates); !slices.Equal(reported, want) {
		t.Fatalf("the report holds %v, want %v", reported, want)
	}
	if candidates[0].LeftNodes != alphaNodes || candidates[0].RightNodes != alphaNodes {
		t.Errorf(
			"the pair counts %d and %d nodes, want %d for both",
			candidates[0].LeftNodes,
			candidates[0].RightNodes,
			alphaNodes,
		)
	}
}

func TestCandidatesRanksTheMostSimilarPairFirst(t *testing.T) {
	directory := sampleTree(t, map[string]string{
		"alpha.go": alphaSource,
		"beta.go":  betaSource,
		"gamma.go": gammaSource,
	})

	candidates, err := Candidates(t.Context(), []string{directory}, 0.7, 4, 20)
	if err != nil {
		t.Fatalf("read candidates: %v", err)
	}

	want := []string{
		"alpha.go:3-11 beta.go:3-11 1.0000",
		"alpha.go:3-11 gamma.go:3-12 0.7667",
		"beta.go:3-11 gamma.go:3-12 0.7667",
	}
	if reported := summaries(candidates); !slices.Equal(reported, want) {
		t.Fatalf("the report holds %v, want %v", reported, want)
	}
}

func TestCandidatesRespectsTheThresholdBoundary(t *testing.T) {
	directory := sampleTree(t, map[string]string{
		"alpha.go": alphaSource,
		"beta.go":  betaSource,
		"gamma.go": gammaSource,
	})
	testCases := []struct {
		name      string
		threshold float64
		want      int
	}{
		{name: "a threshold at the score of the pair reports it", threshold: alphaGammaScore, want: 3},
		{
			name:      "a threshold above the score of the pair drops it",
			threshold: math.Nextafter(alphaGammaScore, 1),
			want:      1,
		},
		{name: "a threshold of 1 reports the identical pair", threshold: 1, want: 1},
		{name: "a threshold of 0 reports every pair", threshold: 0, want: 3},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			candidates, err := Candidates(t.Context(), []string{directory}, testCase.threshold, 4, 20)
			if err != nil {
				t.Fatalf("read candidates: %v", err)
			}

			if len(candidates) != testCase.want {
				t.Errorf("the report holds %v, want %d candidates", summaries(candidates), testCase.want)
			}
		})
	}
}

func TestCandidatesExcludesAFunctionBelowOneMinimum(t *testing.T) {
	directory := sampleTree(t, map[string]string{"alpha.go": alphaSource, "beta.go": betaSource})
	testCases := []struct {
		name         string
		minimumLines int
		minimumNodes int
		want         int
	}{
		{
			name:         "a function with exactly the minimum lines joins the comparison",
			minimumLines: alphaLines,
			minimumNodes: 1,
			want:         1,
		},
		{
			name:         "a function below the minimum lines leaves the comparison",
			minimumLines: alphaLines + 1,
			minimumNodes: 1,
			want:         0,
		},
		{
			name:         "a function with exactly the minimum nodes joins the comparison",
			minimumLines: 1,
			minimumNodes: alphaNodes,
			want:         1,
		},
		{
			name:         "a function below the minimum nodes leaves the comparison",
			minimumLines: 1,
			minimumNodes: alphaNodes + 1,
			want:         0,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			candidates, err := Candidates(
				t.Context(),
				[]string{directory},
				0.82,
				testCase.minimumLines,
				testCase.minimumNodes,
			)
			if err != nil {
				t.Fatalf("read candidates: %v", err)
			}

			if len(candidates) != testCase.want {
				t.Errorf("the report holds %v, want %d candidates", summaries(candidates), testCase.want)
			}
		})
	}
}

func TestCandidatesComparesEachPairOnceAndNeverComparesAFunctionWithItself(t *testing.T) {
	directory := sampleTree(t, map[string]string{
		"alpha.go": alphaSource,
		"beta.go":  betaSource,
		"copy.go":  strings.ReplaceAll(alphaSource, "Alpha", "Copy"),
	})

	candidates, err := Candidates(t.Context(), []string{directory}, 1, 4, 20)
	if err != nil {
		t.Fatalf("read candidates: %v", err)
	}

	want := []string{
		"alpha.go:3-11 beta.go:3-11 1.0000",
		"alpha.go:3-11 copy.go:3-11 1.0000",
		"beta.go:3-11 copy.go:3-11 1.0000",
	}
	if reported := summaries(candidates); !slices.Equal(reported, want) {
		t.Fatalf("the report holds %v, want %v", reported, want)
	}
}

func TestCandidatesSkipsEveryDeclarationThatDeclaresNoBody(t *testing.T) {
	directory := sampleTree(t, map[string]string{
		"alpha.go": alphaSource,
		"delta.go": declarationSource,
	})

	candidates, err := Candidates(t.Context(), []string{directory}, 0.82, 4, 20)
	if err != nil {
		t.Fatalf("read candidates: %v", err)
	}

	want := []string{"alpha.go:3-11 delta.go:7-15 1.0000"}
	if reported := summaries(candidates); !slices.Equal(reported, want) {
		t.Fatalf("the report holds %v, want %v", reported, want)
	}
}

func TestCandidatesWalksEveryDirectoryAndSkipsTheGeneratedOnes(t *testing.T) {
	directory := sampleTree(t, map[string]string{
		"alpha.go":                  alphaSource,
		"nested/deep/beta_test.go":  betaSource,
		".git/hidden.go":            alphaSource,
		"vendor/other/vendored.go":  alphaSource,
		"target/generated/built.go": alphaSource,
		"nested/notes.txt":          "not Go source",
	})

	candidates, err := Candidates(t.Context(), []string{directory}, 0.82, 4, 20)
	if err != nil {
		t.Fatalf("read candidates: %v", err)
	}

	want := []string{"alpha.go:3-11 beta_test.go:3-11 1.0000"}
	if reported := summaries(candidates); !slices.Equal(reported, want) {
		t.Fatalf("the report holds %v, want %v", reported, want)
	}
}

func TestCandidatesSkipsASkippedDirectoryThatTheCallerNames(t *testing.T) {
	directory := sampleTree(t, map[string]string{
		"vendor/alpha.go": alphaSource,
		"vendor/beta.go":  betaSource,
	})

	candidates, err := Candidates(
		t.Context(),
		[]string{filepath.Join(directory, "vendor")},
		0.82,
		4,
		20,
	)
	if err != nil {
		t.Fatalf("read candidates: %v", err)
	}

	if len(candidates) != 0 {
		t.Errorf("the report holds %v, want no candidate", summaries(candidates))
	}
}

func TestCandidatesReadsEveryPathOfTheSelectionExactlyOnce(t *testing.T) {
	directory := sampleTree(t, map[string]string{"alpha.go": alphaSource, "beta.go": betaSource})
	alpha := filepath.Join(directory, "alpha.go")
	testCases := []struct {
		name  string
		paths []string
		want  []string
	}{
		{
			name:  "one file alone has no pair",
			paths: []string{alpha},
			want:  []string{},
		},
		{
			name:  "the same file twice still has no pair",
			paths: []string{alpha, alpha},
			want:  []string{},
		},
		{
			name:  "one file and one directory report the pair once",
			paths: []string{alpha, directory},
			want:  []string{"alpha.go:3-11 beta.go:3-11 1.0000"},
		},
		{
			name:  "a path that names another kind of file adds nothing",
			paths: []string{directory, writeSource(t, t.TempDir(), "notes.txt", "not Go source")},
			want:  []string{"alpha.go:3-11 beta.go:3-11 1.0000"},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			candidates, err := Candidates(t.Context(), testCase.paths, 0.82, 4, 20)
			if err != nil {
				t.Fatalf("read candidates: %v", err)
			}

			if reported := summaries(candidates); !slices.Equal(reported, testCase.want) {
				t.Errorf("the report holds %v, want %v", reported, testCase.want)
			}
		})
	}
}

func TestCandidatesReportsTheSameResultInEveryRun(t *testing.T) {
	directory := sampleTree(t, map[string]string{
		"alpha.go": alphaSource,
		"beta.go":  betaSource,
		"gamma.go": gammaSource,
		"short.go": shortSource,
	})

	first, err := Candidates(t.Context(), []string{directory}, 0, 1, 1)
	if err != nil {
		t.Fatalf("read the first report: %v", err)
	}
	second, err := Candidates(t.Context(), []string{directory}, 0, 1, 1)
	if err != nil {
		t.Fatalf("read the second report: %v", err)
	}

	if len(first) != 6 {
		t.Fatalf("the report holds %v, want 6 candidates", summaries(first))
	}
	if !slices.Equal(first, second) {
		t.Errorf("the second report holds %v, want %v", summaries(second), summaries(first))
	}
}

func TestCandidatesRejectsAnArgumentThatNoComparisonAccepts(t *testing.T) {
	directory := sampleTree(t, map[string]string{"alpha.go": alphaSource})
	testCases := []struct {
		name         string
		paths        []string
		threshold    float64
		minimumLines int
		minimumNodes int
	}{
		{name: "an empty path list", paths: nil, threshold: 0.82, minimumLines: 4, minimumNodes: 20},
		{
			name:         "an empty path",
			paths:        []string{"   "},
			threshold:    0.82,
			minimumLines: 4,
			minimumNodes: 20,
		},
		{
			name:         "a path that no file system entry holds",
			paths:        []string{filepath.Join(directory, "absent")},
			threshold:    0.82,
			minimumLines: 4,
			minimumNodes: 20,
		},
		{
			name:         "a threshold below 0",
			paths:        []string{directory},
			threshold:    -0.001,
			minimumLines: 4,
			minimumNodes: 20,
		},
		{
			name:         "a threshold above 1",
			paths:        []string{directory},
			threshold:    1.001,
			minimumLines: 4,
			minimumNodes: 20,
		},
		{
			name:         "a minimum of 0 source lines",
			paths:        []string{directory},
			threshold:    0.82,
			minimumLines: 0,
			minimumNodes: 20,
		},
		{
			name:         "a minimum of 0 normalized nodes",
			paths:        []string{directory},
			threshold:    0.82,
			minimumLines: 4,
			minimumNodes: 0,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := Candidates(
				t.Context(),
				testCase.paths,
				testCase.threshold,
				testCase.minimumLines,
				testCase.minimumNodes,
			)

			if !errors.Is(err, failure.ErrValidation) {
				t.Errorf("the error is %v, want a validation failure", err)
			}
		})
	}
}

func TestCandidatesAcceptsEveryLimitAtItsBoundary(t *testing.T) {
	directory := sampleTree(t, map[string]string{"alpha.go": alphaSource, "short.go": shortSource})
	testCases := []struct {
		name         string
		threshold    float64
		minimumLines int
		minimumNodes int
	}{
		{name: "a threshold of 0", threshold: 0, minimumLines: 1, minimumNodes: 1},
		{name: "a threshold of 1", threshold: 1, minimumLines: 1, minimumNodes: 1},
		{name: "a minimum of 1 source line", threshold: 0.82, minimumLines: 1, minimumNodes: 1},
		{name: "a minimum of 1 normalized node", threshold: 0.82, minimumLines: 1, minimumNodes: 1},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := Candidates(
				t.Context(),
				[]string{directory},
				testCase.threshold,
				testCase.minimumLines,
				testCase.minimumNodes,
			); err != nil {
				t.Errorf("the limit is rejected with %v", err)
			}
		})
	}
}

func TestCandidatesComparesEveryFunctionWhenTheThresholdIsZero(t *testing.T) {
	directory := sampleTree(t, map[string]string{
		"alpha.go": alphaSource,
		"beta.go":  betaSource,
		"short.go": shortSource,
	})

	candidates, err := Candidates(t.Context(), []string{directory}, 0, 1, 1)
	if err != nil {
		t.Fatalf("read candidates: %v", err)
	}

	want := []string{
		"alpha.go:3-11 beta.go:3-11 1.0000",
		"alpha.go:3-11 short.go:3-5 0.0968",
		"beta.go:3-11 short.go:3-5 0.0968",
	}
	if reported := summaries(candidates); !slices.Equal(reported, want) {
		t.Fatalf("the report holds %v, want %v", reported, want)
	}
}

func TestCandidatesClassifiesUnreadableSource(t *testing.T) {
	directory := sampleTree(t, map[string]string{"broken.go": "package sample\n\nfunc Broken( {\n"})

	_, err := Candidates(t.Context(), []string{directory}, 0.82, 4, 20)

	if !errors.Is(err, failure.ErrDataIntegrity) {
		t.Errorf("the error is %v, want a data integrity failure", err)
	}
}

func TestCandidatesClassifiesAFailedDirectoryWalk(t *testing.T) {
	directory := sampleTree(t, map[string]string{"nested/alpha.go": alphaSource})
	nested := filepath.Join(directory, "nested")
	if err := os.Chmod(nested, 0o000); err != nil {
		t.Fatalf("close the nested directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(nested, 0o700); err != nil {
			t.Errorf("open the nested directory again: %v", err)
		}
	})
	if _, err := os.ReadDir(nested); err == nil {
		t.Skip("the current user reads a directory without permission")
	}

	_, err := Candidates(t.Context(), []string{directory}, 0.82, 4, 20)

	if !errors.Is(err, failure.ErrUnavailable) {
		t.Errorf("the error is %v, want an unavailable dependency failure", err)
	}
}

func TestCandidatesStopsWhenTheCallerCancelsTheScan(t *testing.T) {
	directory := sampleTree(t, map[string]string{"alpha.go": alphaSource, "beta.go": betaSource})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := Candidates(ctx, []string{directory}, 0.82, 4, 20)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("the error is %v, want a cancelled context", err)
	}
}

func TestPairsStopsWhenTheCallerCancelsTheComparison(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	compared := []function{
		{location: Location{File: "left.go"}, fingerprints: fingerprintSet(t, "a", "b")},
		{location: Location{File: "right.go"}, fingerprints: fingerprintSet(t, "a", "b")},
	}

	_, err := pairs(ctx, compared, 0.82)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("the error is %v, want a cancelled context", err)
	}
}

func TestSimilarityAppliesTheJaccardIndex(t *testing.T) {
	testCases := []struct {
		name  string
		left  []string
		right []string
		want  float64
	}{
		{name: "two equal sets share everything", left: []string{"a", "b", "c"},
			right: []string{"a", "b", "c"}, want: 1},
		{name: "two disjoint sets share nothing", left: []string{"a", "b"},
			right: []string{"c", "d"}, want: 0},
		{name: "two sets that overlap by half", left: []string{"a", "b", "c"},
			right: []string{"b", "c", "d"}, want: 0.5},
		{name: "one set inside a larger set", left: []string{"a"},
			right: []string{"a", "b", "c"}, want: 1.0 / 3.0},
		{name: "one set that ends before the other", left: []string{"a", "b", "c", "d"},
			right: []string{"a"}, want: 0.25},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			left := fingerprintSet(t, testCase.left...)
			right := fingerprintSet(t, testCase.right...)

			score := similarity(left, right)

			if math.Abs(score-testCase.want) > 1e-12 {
				t.Errorf("the score is %v, want %v", score, testCase.want)
			}
			if mirrored := similarity(right, left); mirrored != score {
				t.Errorf("the mirrored score is %v, want %v", mirrored, score)
			}
		})
	}
}

func TestCompareCandidateOrdersEveryField(t *testing.T) {
	reference := Candidate{
		Score: 0.9,
		Left:  Location{File: "a.go", StartLine: 10},
		Right: Location{File: "b.go", StartLine: 20},
	}
	testCases := []struct {
		name  string
		right Candidate
		want  int
	}{
		{name: "one pair with a lower score comes later", right: withScore(reference, 0.8), want: -1},
		{name: "one pair with a higher score comes first", right: withScore(reference, 0.95), want: 1},
		{name: "one equal pair keeps its place", right: reference, want: 0},
		{
			name:  "one pair with a later left file comes later",
			right: withLeft(reference, "z.go", 10),
			want:  -1,
		},
		{
			name:  "one pair with a later left line comes later",
			right: withLeft(reference, "a.go", 11),
			want:  -1,
		},
		{
			name:  "one pair with a later right file comes later",
			right: withRight(reference, "z.go", 20),
			want:  -1,
		},
		{
			name:  "one pair with a later right line comes later",
			right: withRight(reference, "b.go", 21),
			want:  -1,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			comparison := compareCandidate(reference, testCase.right)

			if sign(comparison) != testCase.want {
				t.Errorf("the comparison is %d, want the sign %d", comparison, testCase.want)
			}
		})
	}
}

func withScore(candidate Candidate, score float64) Candidate {
	candidate.Score = score
	return candidate
}

func withLeft(candidate Candidate, file string, startLine int) Candidate {
	candidate.Left = Location{File: file, StartLine: startLine}
	return candidate
}

func withRight(candidate Candidate, file string, startLine int) Candidate {
	candidate.Right = Location{File: file, StartLine: startLine}
	return candidate
}

func sign(comparison int) int {
	if comparison < 0 {
		return -1
	}
	if comparison > 0 {
		return 1
	}
	return 0
}

func TestNormalizeKeepsTheStructureOfEveryStatement(t *testing.T) {
	testCases := []struct {
		name string
		body string
		want string
	}{
		{name: "a nested block", body: "{\nf()\n}", want: "(block (block (expr-stmt (call (callee)))))"},
		{
			name: "an if statement with every clause",
			body: "if a := 1; a > 0 {\nb()\n} else {\nc()\n}",
			want: "(block (if (assign/:= (lhs (ident)) (rhs (literal/INT)))" +
				" (binary/> (ident) (literal/INT)) (block (expr-stmt (call (callee))))" +
				" (block (expr-stmt (call (callee))))))",
		},
		{
			name: "an if statement without an init and without an else",
			body: "if a {\nb()\n}",
			want: "(block (if (nil) (ident) (block (expr-stmt (call (callee)))) (nil)))",
		},
		{
			name: "a for statement with every clause",
			body: "for i := 0; i < 3; i++ {\nf()\n}",
			want: "(block (for (assign/:= (lhs (ident)) (rhs (literal/INT)))" +
				" (binary/< (ident) (literal/INT)) (incdec/++ (ident))" +
				" (block (expr-stmt (call (callee))))))",
		},
		{
			name: "a range statement drops the key and the value",
			body: "for k, v := range m {\nf()\n}",
			want: "(block (range (ident) (block (expr-stmt (call (callee))))))",
		},
		{
			name: "a switch statement with one case and one default",
			body: "switch a {\ncase 1, 2:\nf()\ndefault:\ng()\n}",
			want: "(block (switch (nil) (ident) (block" +
				" (case (case-list (literal/INT) (literal/INT))" +
				" (case-body (expr-stmt (call (callee)))))" +
				" (case (case-list) (case-body (expr-stmt (call (callee))))))))",
		},
		{
			name: "a type switch statement",
			body: "switch a.(type) {\ncase int:\n}",
			want: "(block (type-switch (nil) (expr-stmt (type-assert (ident) (nil)))" +
				" (block (case (case-list (ident)) (case-body)))))",
		},
		{
			name: "a select statement with one communication",
			body: "select {\ncase <-c:\nf()\n}",
			want: "(block (select (block (comm (expr-stmt (unary/<- (ident)))" +
				" (comm-body (expr-stmt (call (callee))))))))",
		},
		{
			name: "a select statement with only a default",
			body: "select {\ndefault:\n}",
			want: "(block (select (block (comm (nil) (comm-body)))))",
		},
		{
			name: "an assignment keeps its operator and both sides",
			body: "a, b = c, d",
			want: "(block (assign/= (lhs (ident) (ident)) (rhs (ident) (ident))))",
		},
		{
			name: "a variable declaration",
			body: "var a int = 1",
			want: "(block (decl (gen-decl/var (value-spec (ident) (literal/INT)))))",
		},
		{
			name: "a type declaration",
			body: "type T struct{}",
			want: "(block (decl (gen-decl/type (type-spec (struct-type)))))",
		},
		{name: "a return statement", body: "return 1, a", want: "(block (return (literal/INT) (ident)))"},
		{
			name: "a branch statement keeps its keyword",
			body: "for {\nbreak\n}",
			want: "(block (for (nil) (nil) (nil) (block (branch/break))))",
		},
		{name: "a go statement", body: "go f()", want: "(block (go (call (callee))))"},
		{name: "a defer statement", body: "defer f()", want: "(block (defer (call (callee))))"},
		{name: "a send statement", body: "c <- 1", want: "(block (send (ident) (literal/INT)))"},
		{name: "an increment statement", body: "a++", want: "(block (incdec/++ (ident)))"},
		{
			name: "a labeled statement",
			body: "loop:\nfor {\n}",
			want: "(block (label (for (nil) (nil) (nil) (block))))",
		},
		{name: "an empty statement", body: ";", want: "(block (empty))"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if tree := bodyShape(t, testCase.body); tree != testCase.want {
				t.Errorf("the tree is\n%s\nwant\n%s", tree, testCase.want)
			}
		})
	}
}

func TestNormalizeKeepsTheStructureOfEveryExpression(t *testing.T) {
	testCases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "a binary expression keeps its operator",
			body: "a = b && c || d",
			want: "(block (assign/= (lhs (ident)) (rhs (binary/|| (binary/&& (ident) (ident)) (ident)))))",
		},
		{
			name: "a unary expression keeps its operator",
			body: "a = -b",
			want: "(block (assign/= (lhs (ident)) (rhs (unary/- (ident)))))",
		},
		{
			name: "a method call keeps the shape of its receiver",
			body: "a = x.y()",
			want: "(block (assign/= (lhs (ident)) (rhs (call (selector-callee (ident) (member))))))",
		},
		{
			name: "a call of a function literal",
			body: "a = func() {}()",
			want: "(block (assign/= (lhs (ident)) (rhs (call (func-lit (params) (results) (block))))))",
		},
		{
			name: "a selector drops the member name",
			body: "a = b.c",
			want: "(block (assign/= (lhs (ident)) (rhs (selector (ident) (member)))))",
		},
		{
			name: "an index expression",
			body: "a = b[c]",
			want: "(block (assign/= (lhs (ident)) (rhs (index (ident) (ident)))))",
		},
		{
			name: "an index expression with two type arguments",
			body: "a = f[int, string]",
			want: "(block (assign/= (lhs (ident)) (rhs (index-list (ident) (ident) (ident)))))",
		},
		{
			name: "a slice expression keeps its three bounds",
			body: "a = b[1:2:3]",
			want: "(block (assign/= (lhs (ident))" +
				" (rhs (slice (ident) (literal/INT) (literal/INT) (literal/INT)))))",
		},
		{
			name: "a pointer expression",
			body: "a = *b",
			want: "(block (assign/= (lhs (ident)) (rhs (star (ident)))))",
		},
		{
			name: "a parenthesized expression",
			body: "a = (b)",
			want: "(block (assign/= (lhs (ident)) (rhs (paren (ident)))))",
		},
		{
			name: "a composite literal keeps its type and its elements",
			body: "a = T{1}",
			want: "(block (assign/= (lhs (ident)) (rhs (composite (ident) (literal/INT)))))",
		},
		{
			name: "a composite literal with keys",
			body: "a = map[string]int{\"k\": 1}",
			want: "(block (assign/= (lhs (ident)) (rhs (composite (map-type (ident) (ident))" +
				" (key-value (literal/STRING) (literal/INT))))))",
		},
		{
			name: "a function literal keeps its signature",
			body: "a = func(x int) int {\nreturn x\n}",
			want: "(block (assign/= (lhs (ident)) (rhs (func-lit (params (field (ident)))" +
				" (results (field (ident))) (block (return (ident)))))))",
		},
		{
			name: "a type assertion",
			body: "a = b.(int)",
			want: "(block (assign/= (lhs (ident)) (rhs (type-assert (ident) (ident)))))",
		},
		{
			name: "a decimal literal keeps only its kind",
			body: "a = 1.5",
			want: "(block (assign/= (lhs (ident)) (rhs (literal/FLOAT))))",
		},
		{
			name: "a character literal keeps only its kind",
			body: "a = 'c'",
			want: "(block (assign/= (lhs (ident)) (rhs (literal/CHAR))))",
		},
		{
			name: "a text literal keeps only its kind",
			body: "a = \"s\"",
			want: "(block (assign/= (lhs (ident)) (rhs (literal/STRING))))",
		},
		{
			name: "an imaginary literal keeps only its kind",
			body: "a = 1i",
			want: "(block (assign/= (lhs (ident)) (rhs (literal/IMAG))))",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if tree := bodyShape(t, testCase.body); tree != testCase.want {
				t.Errorf("the tree is\n%s\nwant\n%s", tree, testCase.want)
			}
		})
	}
}

func TestNormalizeKeepsTheStructureOfEveryType(t *testing.T) {
	testCases := []struct {
		name string
		body string
		want string
	}{
		{name: "a slice type", body: "var a []int", want: "(array-type (ident))"},
		{name: "an array type states the same shape", body: "var a [3]int", want: "(array-type (ident))"},
		{name: "a map type", body: "var a map[string]int", want: "(map-type (ident) (ident))"},
		{name: "a struct type", body: "var a struct{ x int }", want: "(struct-type (field (ident)))"},
		{
			name: "an interface type",
			body: "var a interface{ M() }",
			want: "(interface-type (field (func-type (params) (results))))",
		},
		{name: "a channel type", body: "var a chan int", want: "(chan-type (ident))"},
		{
			name: "a directed channel states the same shape",
			body: "var a <-chan int",
			want: "(chan-type (ident))",
		},
		{
			name: "a function type",
			body: "var a func(int) error",
			want: "(func-type (params (field (ident))) (results (field (ident))))",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			want := "(block (decl (gen-decl/var (value-spec " + testCase.want + "))))"

			if tree := bodyShape(t, testCase.body); tree != want {
				t.Errorf("the tree is\n%s\nwant\n%s", tree, want)
			}
		})
	}
}

func TestNormalizeKeepsTheNameOfEveryPredeclaredConstant(t *testing.T) {
	testCases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "the nil constant", body: "a = nil",
			want: "(block (assign/= (lhs (ident)) (rhs (literal/nil))))",
		},
		{
			name: "the true constant", body: "a = true",
			want: "(block (assign/= (lhs (ident)) (rhs (literal/true))))",
		},
		{
			name: "the false constant", body: "a = false",
			want: "(block (assign/= (lhs (ident)) (rhs (literal/false))))",
		},
		{
			name: "a plain identifier", body: "a = b",
			want: "(block (assign/= (lhs (ident)) (rhs (ident))))",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if tree := bodyShape(t, testCase.body); tree != testCase.want {
				t.Errorf("the tree is\n%s\nwant\n%s", tree, testCase.want)
			}
		})
	}
}

// TestNormalizeSeparatesAPredeclaredConstantFromAValue pins the rule that
// decides whether a test against nil resembles a test against a value.
func TestNormalizeSeparatesAPredeclaredConstantFromAValue(t *testing.T) {
	against := bodyShape(t, "c = a != nil")
	between := bodyShape(t, "c = a != b")

	if against == between {
		t.Errorf("a test against nil shares the shape of a test between values: %s", against)
	}
	if bodyShape(t, "c = a != true") == between {
		t.Error("a test against true shares the shape of a test between values")
	}
}

func TestNormalizeFunctionKeepsTheSignatureShape(t *testing.T) {
	testCases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "a function without a parameter and without a result",
			source: "package sample\n\nfunc sample() {\n}\n",
			want:   "(func (params) (results) (block))",
		},
		{
			name:   "a function with grouped names, a variadic parameter, and two results",
			source: "package sample\n\nfunc sample(a, b int, c ...string) (int, error) {\nreturn 0, nil\n}\n",
			want: "(func (params (field (ident)) (field (ident)) (field (ellipsis (ident))))" +
				" (results (field (ident)) (field (ident))) (block (return (literal/INT) (literal/nil))))",
		},
		{
			name:   "a method keeps its receiver after the results",
			source: "package sample\n\nfunc (w Widget) sample() {\n}\n",
			want:   "(func (params) (results) (receiver (field (ident))) (block))",
		},
		{
			name:   "an unnamed parameter still counts as one field",
			source: "package sample\n\nfunc sample(int) {\n}\n",
			want:   "(func (params (field (ident))) (results) (block))",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			tree := renderShape(normalizeFunction(firstFunction(t, testCase.source)))

			if tree != testCase.want {
				t.Errorf("the tree is\n%s\nwant\n%s", tree, testCase.want)
			}
		})
	}
}

func TestNormalizeNamesAnUnexpectedNode(t *testing.T) {
	testCases := []struct {
		name  string
		shape shape
		want  string
	}{
		{name: "an absent node", shape: normalize(nil), want: "(nil)"},
		{name: "a node that is neither a statement nor an expression",
			shape: normalize(&ast.Field{}), want: "(*ast.Field)"},
		{name: "an unknown statement", shape: normalizeStatement(&ast.BadStmt{}), want: "(*ast.BadStmt)"},
		{name: "an unknown expression", shape: normalizeExpression(&ast.BadExpr{}), want: "(*ast.BadExpr)"},
		{name: "a declaration that is not general",
			shape: normalizeDeclaration(&ast.FuncDecl{}), want: "(decl)"},
		{name: "a specification that declares no value and no type",
			shape: normalizeSpecification(&ast.ImportSpec{}), want: "(spec)"},
		{name: "a callee that is neither a name nor a member",
			shape: normalizeCallee(&ast.ParenExpr{X: &ast.Ident{Name: "f"}}), want: "(paren (ident))"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if tree := renderShape(testCase.shape); tree != testCase.want {
				t.Errorf("the tree is %s, want %s", tree, testCase.want)
			}
		})
	}
}

func TestNormalizeFieldListMarksAnAbsentList(t *testing.T) {
	if tree := renderShape(normalizeFieldList("results", nil)); tree != "(results)" {
		t.Errorf("the tree is %s, want (results)", tree)
	}
}

func TestCountShapesCountsTheRootAndEveryChild(t *testing.T) {
	testCases := []struct {
		name string
		root shape
		want int
	}{
		{name: "one leaf", root: newShape("a"), want: 1},
		{name: "one root with two leaves", root: newShape("a", newShape("b"), newShape("c")), want: 3},
		{
			name: "one root with a nested branch",
			root: newShape("a", newShape("b", newShape("c", newShape("d")))),
			want: 4,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if total := countShapes(testCase.root); total != testCase.want {
				t.Errorf("the tree counts %d nodes, want %d", total, testCase.want)
			}
		})
	}
}

func TestFingerprintsSortTheSetAndRemoveEveryDuplicate(t *testing.T) {
	root := newShape("a", newShape("b"), newShape("b"), newShape("c"))

	collected := fingerprints(root)

	if countShapes(root) != 4 {
		t.Fatalf("the tree counts %d nodes, want 4", countShapes(root))
	}
	if len(collected) != 3 {
		t.Errorf("the set holds %d fingerprints, want 3", len(collected))
	}
	if !slices.IsSortedFunc(collected, compareFingerprint) {
		t.Errorf("the set is not sorted")
	}
}

func TestFingerprintTreeSeparatesTheTagFromTheChildren(t *testing.T) {
	testCases := []struct {
		name  string
		left  shape
		right shape
		equal bool
	}{
		{name: "two equal trees", left: newShape("a", newShape("b")),
			right: newShape("a", newShape("b")), equal: true},
		{name: "two trees with a different tag", left: newShape("a"), right: newShape("b")},
		{name: "one tag against one child with that tag", left: newShape("ab"),
			right: newShape("a", newShape("b"))},
		{name: "two trees with the children in another order",
			left:  newShape("a", newShape("b"), newShape("c")),
			right: newShape("a", newShape("c"), newShape("b"))},
		{name: "one tree with one child more", left: newShape("a", newShape("b")),
			right: newShape("a", newShape("b"), newShape("b"))},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			left, _ := fingerprintTree(testCase.left, nil)
			right, _ := fingerprintTree(testCase.right, nil)

			if equal := left == right; equal != testCase.equal {
				t.Errorf("the trees share one fingerprint: %v, want %v", equal, testCase.equal)
			}
		})
	}
}
