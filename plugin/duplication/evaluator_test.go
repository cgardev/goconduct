package duplication

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/cgardev/goconduct/failure"
	"github.com/cgardev/goconduct/internal/library/gosimilarity"
	"github.com/cgardev/goconduct/plugin"
)

// fixtureDirectory holds the fixture sources inside the repository. The name
// carries one ampersand, so a report proves that no writer escapes the path.
const fixtureDirectory = "a&b"

// rankingSimilarity accepts both fixture pairs. The exact pair scores 1.000 and
// the near pair scores less, so a budget must forgive the near pair first.
const rankingSimilarity = 0.5

const exactDuplicateSource = `package fixture

func Left(values []int) []int {
	kept := make([]int, 0, len(values))
	for _, value := range values {
		if value%2 == 0 {
			kept = append(kept, value+1)
		}
	}
	return kept
}

func Right(items []int) []int {
	chosen := make([]int, 0, len(items))
	for _, item := range items {
		if item%2 == 0 {
			chosen = append(chosen, item+1)
		}
	}
	return chosen
}
`

const nearDuplicateSource = `package fixture

func Near(values []string) []string {
	kept := make([]string, 0, len(values))
	for _, value := range values {
		if len(value) > 2 {
			kept = append(kept, value)
		}
	}
	if len(kept) == 0 {
		return nil
	}
	return kept
}

func Alike(items []string) []string {
	chosen := make([]string, 0, len(items))
	for _, item := range items {
		if len(item) > 2 {
			chosen = append(chosen, item)
		}
	}
	return chosen
}
`

const singleFunctionSource = `package fixture

func Only(values []int) int {
	total := 0
	for _, value := range values {
		total += value * 3
	}
	return total
}
`

// newDuplicationFixture builds one Go module below the temporary directory.
// The module is not the temporary directory itself, so a test also names a
// sibling path that exists outside the repository.
func newDuplicationFixture(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "repository")
	writeFixtureFile(t, filepath.Join(root, "go.mod"), "module example.com/duplicationfixture\n\ngo 1.26.3\n")
	writeFixtureFile(t, fixturePath(root, "z_exact.go"), exactDuplicateSource)
	writeFixtureFile(t, fixturePath(root, "a_near.go"), nearDuplicateSource)
	writeFixtureFile(t, filepath.Join(base, "outside", "outside.go"), exactDuplicateSource)
	return root
}

// fixturePath names one fixture source inside the analyzed directory.
func fixturePath(root string, name string) string {
	return filepath.Join(root, fixtureDirectory, name)
}

// reportedPath names one fixture source as the report names it.
func reportedPath(name string) string {
	return fixtureDirectory + "/" + name
}

func writeFixtureFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create directory of %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// newFixtureEvaluator creates one evaluator from the default limits.
func newFixtureEvaluator(t *testing.T, change func(Configuration) Configuration) *Evaluator {
	t.Helper()
	configuration := DefaultConfiguration()
	if change != nil {
		configuration = change(configuration)
	}
	evaluator, err := NewEvaluator(configuration)
	if err != nil {
		t.Fatalf("create evaluator: %v", err)
	}
	return evaluator
}

// rankingConfiguration accepts both fixture pairs and applies one budget.
func rankingConfiguration(budget int) func(Configuration) Configuration {
	return func(configuration Configuration) Configuration {
		configuration.Similarity = rankingSimilarity
		configuration.MaximumCandidates = budget
		return configuration
	}
}

func findingIdentifiers(findings []plugin.Finding) []string {
	identifiers := make([]string, 0, len(findings))
	for _, finding := range findings {
		identifiers = append(identifiers, finding.ID)
	}
	slices.Sort(identifiers)
	return identifiers
}

func metricValue(t *testing.T, report plugin.Report, identifier string) float64 {
	t.Helper()
	for _, metric := range report.Metrics {
		if metric.ID == identifier {
			return metric.Value
		}
	}
	t.Fatalf("the report holds no metric %q", identifier)
	return 0
}

func TestEvaluatorReportsStructuralDuplicates(t *testing.T) {
	evaluator := newFixtureEvaluator(t, nil)

	report, err := evaluator.Evaluate(t.Context(), plugin.Request{
		RepositoryRoot: newDuplicationFixture(t),
	})
	if err != nil {
		t.Fatalf("evaluate duplication: %v", err)
	}

	if report.Plugin != "duplication" {
		t.Fatalf("report plugin is %q", report.Plugin)
	}
	if report.SchemaVersion != plugin.ReportSchemaVersion {
		t.Errorf("report schema version is %d", report.SchemaVersion)
	}
	if value := metricValue(t, report, "duplication:candidates"); value != 1 {
		t.Errorf("the fixture reports %v candidates, want 1", value)
	}
	identity := reportedPath("z_exact.go") + ":3:" + reportedPath("z_exact.go") + ":13"
	want := []string{"duplication:" + identity}
	if identifiers := findingIdentifiers(report.Findings); !slices.Equal(identifiers, want) {
		t.Errorf("the findings are %v, want %v", identifiers, want)
	}
	if value := metricValue(t, report, "duplication:similarity:"+identity); value != 1 {
		t.Errorf("the exact pair scores %v, want 1", value)
	}
	if message := report.Findings[0].Message; !strings.Contains(message, "1.000 similarity") {
		t.Errorf("the message %q states no score", message)
	}
}

func TestEvaluatorReportsEveryPathRelativeToTheRepository(t *testing.T) {
	evaluator := newFixtureEvaluator(t, rankingConfiguration(0))

	report, err := evaluator.Evaluate(t.Context(), plugin.Request{
		RepositoryRoot: newDuplicationFixture(t),
	})
	if err != nil {
		t.Fatalf("evaluate duplication: %v", err)
	}

	paths := make([]string, 0, len(report.Metrics)+len(report.Findings))
	for _, metric := range report.Metrics {
		paths = append(paths, metric.Path)
	}
	for _, finding := range report.Findings {
		paths = append(paths, finding.Path)
		paths = append(paths, finding.Message)
	}
	for _, path := range paths {
		if filepath.IsAbs(path) {
			t.Errorf("path %q is not repository-relative", path)
		}
		if strings.ContainsAny(path, `\`) {
			t.Errorf("path %q holds a backward slash", path)
		}
		if strings.Contains(path, string(filepath.Separator)+"..") {
			t.Errorf("path %q leaves the repository", path)
		}
	}
	if paths[0] != "" {
		t.Errorf("the candidate count metric names path %q, want no path", paths[0])
	}
}

func TestEvaluatorForgivesTheLeastSimilarCandidatesFirst(t *testing.T) {
	evaluator := newFixtureEvaluator(t, rankingConfiguration(1))
	exact := reportedPath("z_exact.go") + ":3:" + reportedPath("z_exact.go") + ":13"
	near := reportedPath("a_near.go") + ":3:" + reportedPath("a_near.go") + ":16"

	report, err := evaluator.Evaluate(t.Context(), plugin.Request{
		RepositoryRoot: newDuplicationFixture(t),
	})
	if err != nil {
		t.Fatalf("evaluate duplication: %v", err)
	}

	want := []string{"duplication:" + exact}
	if identifiers := findingIdentifiers(report.Findings); !slices.Equal(identifiers, want) {
		t.Fatalf("the budget of one keeps %v, want %v", identifiers, want)
	}
	if value := metricValue(t, report, "duplication:similarity:"+exact); value != 1 {
		t.Errorf("the reported pair scores %v, want the highest score 1", value)
	}
	forgiven := metricValue(t, report, "duplication:similarity:"+near)
	if forgiven >= 1 {
		t.Errorf("the forgiven pair scores %v, want less than the reported pair", forgiven)
	}
}

func TestEvaluatorReportsEveryCandidateOverTheBudget(t *testing.T) {
	testCases := []struct {
		name     string
		budget   int
		findings int
	}{
		{name: "a budget of zero reports both candidates", budget: 0, findings: 2},
		{name: "a budget of one reports the most similar candidate", budget: 1, findings: 1},
		{name: "a budget of two forgives both candidates", budget: 2, findings: 0},
		{name: "a budget over the candidate count forgives both candidates", budget: 5, findings: 0},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			evaluator := newFixtureEvaluator(t, rankingConfiguration(testCase.budget))

			report, err := evaluator.Evaluate(t.Context(), plugin.Request{
				RepositoryRoot: newDuplicationFixture(t),
			})
			if err != nil {
				t.Fatalf("evaluate duplication: %v", err)
			}

			if value := metricValue(t, report, "duplication:candidates"); value != 2 {
				t.Fatalf("the fixture reports %v candidates, want 2", value)
			}
			if len(report.Findings) != testCase.findings {
				t.Errorf("the report holds %d findings, want %d", len(report.Findings), testCase.findings)
			}
		})
	}
}

func TestEvaluatorReportsNoFindingWhenNoStructureRepeats(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repository")
	writeFixtureFile(t, filepath.Join(root, "go.mod"), "module example.com/single\n\ngo 1.26.3\n")
	writeFixtureFile(t, fixturePath(root, "only.go"), singleFunctionSource)
	evaluator := newFixtureEvaluator(t, nil)

	report, err := evaluator.Evaluate(t.Context(), plugin.Request{RepositoryRoot: root})
	if err != nil {
		t.Fatalf("evaluate duplication: %v", err)
	}

	if value := metricValue(t, report, "duplication:candidates"); value != 0 {
		t.Errorf("the fixture reports %v candidates, want 0", value)
	}
	if len(report.Metrics) != 1 || len(report.Findings) != 0 {
		t.Errorf("the report holds %d metrics and %d findings", len(report.Metrics), len(report.Findings))
	}
}

// newSyntheticCandidate builds one pair of coordinates inside the repository.
// The analysis never sees these sources, so one test states every score.
func newSyntheticCandidate(root string, score float64, leftLine int, rightLine int) gosimilarity.Candidate {
	file := filepath.ToSlash(filepath.Join(root, fixtureDirectory, "synthetic.go"))
	return gosimilarity.Candidate{
		Score: score,
		Left:  gosimilarity.Location{File: file, StartLine: leftLine, EndLine: leftLine + 4},
		Right: gosimilarity.Location{File: file, StartLine: rightLine, EndLine: rightLine + 4},
	}
}

func syntheticIdentifier(leftLine int, rightLine int) string {
	return "duplication:" + reportedPath("synthetic.go") + ":" + strconv.Itoa(leftLine) +
		":" + reportedPath("synthetic.go") + ":" + strconv.Itoa(rightLine)
}

func TestReportForgivesTheLeastSimilarCandidatesInEveryInputOrder(t *testing.T) {
	root := t.TempDir()
	worst := newSyntheticCandidate(root, 1, 10, 20)
	middle := newSyntheticCandidate(root, 0.9, 30, 40)
	least := newSyntheticCandidate(root, 0.85, 50, 60)
	orders := map[string][]gosimilarity.Candidate{
		"the ranked order":   {worst, middle, least},
		"the reversed order": {least, middle, worst},
		"a rotated order":    {middle, worst, least},
	}
	testCases := []struct {
		name   string
		budget int
		want   []string
	}{
		{
			name:   "a budget of zero reports every candidate",
			budget: 0,
			want: []string{
				syntheticIdentifier(10, 20),
				syntheticIdentifier(30, 40),
				syntheticIdentifier(50, 60),
			},
		},
		{
			name:   "a budget of one forgives the least similar candidate",
			budget: 1,
			want:   []string{syntheticIdentifier(10, 20), syntheticIdentifier(30, 40)},
		},
		{
			name:   "a budget of two keeps the most similar candidate",
			budget: 2,
			want:   []string{syntheticIdentifier(10, 20)},
		},
		{
			name:   "a budget over the candidate count forgives every candidate",
			budget: 5,
			want:   []string{},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			for name, candidates := range orders {
				evaluator := newFixtureEvaluator(t, rankingConfiguration(testCase.budget))

				report, err := evaluator.report(root, candidates)
				if err != nil {
					t.Fatalf("report %s: %v", name, err)
				}

				identifiers := findingIdentifiers(report.Findings)
				slices.Sort(testCase.want)
				if !slices.Equal(identifiers, testCase.want) {
					t.Errorf("%s reports %v, want %v", name, identifiers, testCase.want)
				}
				if value := metricValue(t, report, "duplication:candidates"); value != 3 {
					t.Errorf("%s counts %v candidates, want 3", name, value)
				}
			}
		})
	}
}

func TestReportKeepsTheCandidateSliceOfTheCaller(t *testing.T) {
	root := t.TempDir()
	candidates := []gosimilarity.Candidate{
		newSyntheticCandidate(root, 0.85, 50, 60),
		newSyntheticCandidate(root, 1, 10, 20),
	}
	evaluator := newFixtureEvaluator(t, nil)

	if _, err := evaluator.report(root, candidates); err != nil {
		t.Fatalf("report candidates: %v", err)
	}

	if candidates[0].Score != 0.85 {
		t.Errorf("the report sorted the slice of the caller into %v", candidates)
	}
}

func TestCompareCandidateOrdersEveryField(t *testing.T) {
	reference := gosimilarity.Candidate{
		Score: 0.9,
		Left:  gosimilarity.Location{File: "a.go", StartLine: 10},
		Right: gosimilarity.Location{File: "b.go", StartLine: 20},
	}
	testCases := []struct {
		name  string
		right gosimilarity.Candidate
		want  int
	}{
		{name: "an equal pair keeps its place", right: reference, want: 0},
		{
			name:  "a less similar pair comes later",
			right: withScore(reference, 0.8),
			want:  -1,
		},
		{
			name:  "a more similar pair comes first",
			right: withScore(reference, 0.95),
			want:  1,
		},
		{
			name:  "a later left file comes later",
			right: withLeft(reference, "z.go", 10),
			want:  -1,
		},
		{
			name:  "an earlier left file comes first",
			right: withLeft(reference, "A.go", 10),
			want:  1,
		},
		{
			name:  "a later left line comes later",
			right: withLeft(reference, "a.go", 11),
			want:  -1,
		},
		{
			name:  "an earlier left line comes first",
			right: withLeft(reference, "a.go", 9),
			want:  1,
		},
		{
			name:  "a later right file comes later",
			right: withRight(reference, "z.go", 20),
			want:  -1,
		},
		{
			name:  "an earlier right file comes first",
			right: withRight(reference, "A.go", 20),
			want:  1,
		},
		{
			name:  "a later right line comes later",
			right: withRight(reference, "b.go", 21),
			want:  -1,
		},
		{
			name:  "an earlier right line comes first",
			right: withRight(reference, "b.go", 19),
			want:  1,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			comparison := compareCandidate(reference, testCase.right)

			if comparisonSign(comparison) != testCase.want {
				t.Errorf("the comparison is %d, want the sign %d", comparison, testCase.want)
			}
		})
	}
}

func withScore(candidate gosimilarity.Candidate, score float64) gosimilarity.Candidate {
	candidate.Score = score
	return candidate
}

func withLeft(candidate gosimilarity.Candidate, file string, startLine int) gosimilarity.Candidate {
	candidate.Left = gosimilarity.Location{File: file, StartLine: startLine}
	return candidate
}

func withRight(candidate gosimilarity.Candidate, file string, startLine int) gosimilarity.Candidate {
	candidate.Right = gosimilarity.Location{File: file, StartLine: startLine}
	return candidate
}

func comparisonSign(comparison int) int {
	if comparison < 0 {
		return -1
	}
	if comparison > 0 {
		return 1
	}
	return 0
}

func TestEvaluatorAnalyzesEverySelectedPath(t *testing.T) {
	testCases := []struct {
		name  string
		paths []string
	}{
		{name: "no selection analyzes the whole repository", paths: nil},
		{name: "one directory of the repository", paths: []string{fixtureDirectory}},
		{name: "the repository root itself", paths: []string{"."}},
		{
			name:  "one repeated directory",
			paths: []string{fixtureDirectory, fixtureDirectory},
		},
		{
			name:  "one file inside one selected directory",
			paths: []string{reportedPath("z_exact.go"), fixtureDirectory},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			evaluator := newFixtureEvaluator(t, nil)

			report, err := evaluator.Evaluate(t.Context(), plugin.Request{
				RepositoryRoot: newDuplicationFixture(t), Paths: testCase.paths,
			})
			if err != nil {
				t.Fatalf("evaluate duplication: %v", err)
			}

			identity := reportedPath("z_exact.go") + ":3:" + reportedPath("z_exact.go") + ":13"
			want := []string{"duplication:" + identity}
			if identifiers := findingIdentifiers(report.Findings); !slices.Equal(identifiers, want) {
				t.Errorf("the selection reports %v, want %v", identifiers, want)
			}
			if value := metricValue(t, report, "duplication:candidates"); value != 1 {
				t.Errorf("the selection reports %v candidates, want 1", value)
			}
		})
	}
}

func TestEvaluatorRejectsAnUnusablePath(t *testing.T) {
	testCases := []struct {
		name string
		path string
	}{
		{name: "a path that does not exist", path: "does-not-exist"},
		{name: "an empty path", path: "  "},
		{name: "the parent of the repository", path: ".."},
		{name: "a sibling directory of the repository", path: "../outside"},
		{name: "a path that leaves and returns", path: "../outside/../outside"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			evaluator := newFixtureEvaluator(t, nil)

			_, err := evaluator.Evaluate(t.Context(), plugin.Request{
				RepositoryRoot: newDuplicationFixture(t), Paths: []string{testCase.path},
			})

			if !errors.Is(err, failure.ErrValidation) {
				t.Errorf("path %q reports the error %v, want a validation failure", testCase.path, err)
			}
		})
	}
}

func TestEvaluatorKeepsIdentifiersStableWhenAnotherDuplicateAppears(t *testing.T) {
	evaluator := newFixtureEvaluator(t, nil)
	root := newDuplicationFixture(t)
	added := fixturePath(root, "m_added.go")

	before, err := evaluator.Evaluate(t.Context(), plugin.Request{RepositoryRoot: root})
	if err != nil {
		t.Fatalf("evaluate duplication: %v", err)
	}
	writeFixtureFile(t, added, strings.ReplaceAll(exactDuplicateSource, "Left", "Added"))
	during, err := evaluator.Evaluate(t.Context(), plugin.Request{RepositoryRoot: root})
	if err != nil {
		t.Fatalf("evaluate duplication: %v", err)
	}
	if err := os.Remove(added); err != nil {
		t.Fatalf("remove added duplicate: %v", err)
	}
	after, err := evaluator.Evaluate(t.Context(), plugin.Request{RepositoryRoot: root})
	if err != nil {
		t.Fatalf("evaluate duplication: %v", err)
	}

	if len(during.Findings) <= len(before.Findings) {
		t.Fatalf("the added duplicate changes %d findings into %d", len(before.Findings), len(during.Findings))
	}
	original := findingIdentifiers(before.Findings)
	grown := findingIdentifiers(during.Findings)
	for _, identifier := range original {
		if !slices.Contains(grown, identifier) {
			t.Errorf("identifier %q changed when another duplicate appeared", identifier)
		}
	}
	if shrunk := findingIdentifiers(after.Findings); !slices.Equal(shrunk, original) {
		t.Errorf("the identifiers are %v after the removal, want %v", shrunk, original)
	}
}

func TestEvaluatorStopsWhenTheCallerCancelsTheScan(t *testing.T) {
	evaluator := newFixtureEvaluator(t, nil)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := evaluator.Evaluate(ctx, plugin.Request{RepositoryRoot: newDuplicationFixture(t)})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("the cancelled scan reports %v, want a cancelled context", err)
	}
}

func TestEvaluatorClassifiesUnparsableSource(t *testing.T) {
	root := newDuplicationFixture(t)
	writeFixtureFile(t, fixturePath(root, "broken.go"), "package fixture\n\nfunc Broken( {\n")
	evaluator := newFixtureEvaluator(t, nil)

	_, err := evaluator.Evaluate(t.Context(), plugin.Request{RepositoryRoot: root})

	if !errors.Is(err, failure.ErrDataIntegrity) {
		t.Errorf("the unparsable source reports %v, want a data integrity failure", err)
	}
}

func TestRepositoryPathKeepsAFileItCannotRelate(t *testing.T) {
	testCases := []struct {
		name string
		root string
		file string
		want string
	}{
		{
			name: "one file inside the repository",
			root: filepath.Join(string(filepath.Separator), "repository"),
			file: filepath.ToSlash(filepath.Join(string(filepath.Separator), "repository", "a&b", "x.go")),
			want: "a&b/x.go",
		},
		{
			name: "one relative file that no absolute root relates",
			root: filepath.Join(string(filepath.Separator), "repository"),
			file: "elsewhere/x.go",
			want: "elsewhere/x.go",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if path := repositoryPath(testCase.root, testCase.file); path != testCase.want {
				t.Errorf("the reported path is %q, want %q", path, testCase.want)
			}
		})
	}
}

func TestCandidateIdentityNamesTheSourceCoordinates(t *testing.T) {
	identity := candidateIdentity("a&b/left.go", 3, "a&b/right.go", 13)

	if want := "a&b/left.go:3:a&b/right.go:13"; identity != want {
		t.Errorf("the identity is %q, want %q", identity, want)
	}
}

func TestNewEvaluatorRejectsInvalidConfiguration(t *testing.T) {
	testCases := []struct {
		name          string
		configuration func(Configuration) Configuration
	}{
		{
			name:          "a similarity below zero",
			configuration: func(c Configuration) Configuration { c.Similarity = -0.1; return c },
		},
		{
			name:          "a similarity above one",
			configuration: func(c Configuration) Configuration { c.Similarity = 1.5; return c },
		},
		{
			name:          "a minimum line count of zero",
			configuration: func(c Configuration) Configuration { c.MinimumLines = 0; return c },
		},
		{
			name:          "a negative minimum line count",
			configuration: func(c Configuration) Configuration { c.MinimumLines = -1; return c },
		},
		{
			name:          "a minimum node count of zero",
			configuration: func(c Configuration) Configuration { c.MinimumNodes = 0; return c },
		},
		{
			name:          "a negative minimum node count",
			configuration: func(c Configuration) Configuration { c.MinimumNodes = -1; return c },
		},
		{
			name:          "a negative candidate budget",
			configuration: func(c Configuration) Configuration { c.MaximumCandidates = -1; return c },
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			evaluator, err := NewEvaluator(testCase.configuration(DefaultConfiguration()))

			if evaluator != nil || !errors.Is(err, failure.ErrValidation) {
				t.Errorf("the configuration creates %v with the error %v", evaluator, err)
			}
		})
	}
}

func TestNewEvaluatorAcceptsEveryLimitAtItsBoundary(t *testing.T) {
	testCases := []struct {
		name          string
		configuration func(Configuration) Configuration
	}{
		{
			name:          "a similarity of zero",
			configuration: func(c Configuration) Configuration { c.Similarity = 0; return c },
		},
		{
			name:          "a similarity of one",
			configuration: func(c Configuration) Configuration { c.Similarity = 1; return c },
		},
		{
			name:          "a minimum line count of one",
			configuration: func(c Configuration) Configuration { c.MinimumLines = 1; return c },
		},
		{
			name:          "a minimum node count of one",
			configuration: func(c Configuration) Configuration { c.MinimumNodes = 1; return c },
		},
		{
			name:          "a candidate budget of zero",
			configuration: func(c Configuration) Configuration { c.MaximumCandidates = 0; return c },
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			evaluator, err := NewEvaluator(testCase.configuration(DefaultConfiguration()))

			if err != nil || evaluator == nil {
				t.Errorf("the boundary configuration reports the error %v", err)
			}
		})
	}
}

func TestDefaultConfigurationRejectsEveryStructuralDuplicate(t *testing.T) {
	configuration := DefaultConfiguration()

	if configuration.MaximumCandidates != 0 {
		t.Errorf("the default budget is %d, want 0", configuration.MaximumCandidates)
	}
	if configuration.Similarity <= 0 || configuration.Similarity > 1 {
		t.Errorf("the default similarity is %v, want a threshold inside 0 through 1", configuration.Similarity)
	}
	if configuration.MinimumLines <= 0 || configuration.MinimumNodes <= 0 {
		t.Errorf(
			"the default minimums are %d lines and %d nodes, want positive minimums",
			configuration.MinimumLines,
			configuration.MinimumNodes,
		)
	}
}

func TestEvaluatorNameIsStable(t *testing.T) {
	evaluator := newFixtureEvaluator(t, nil)

	if name := evaluator.Name(); name != "duplication" {
		t.Errorf("the evaluator name is %q, want %q", name, "duplication")
	}
}
