package crap

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cgardev/goconduct/pkg/failure"
	"github.com/cgardev/goconduct/pkg/plugin"
	"github.com/cgardev/goconduct/pkg/policy"
)

// fixtureModule names the module of every prepared coverage profile. The Go
// tool writes the module path in front of each file, so the tests reproduce it.
const fixtureModule = "example.com/crapfixture"

// newModuleRoot creates one temporary repository with its module file.
// The coverage profile names each file with the module path, so the analysis
// needs that declaration to recover the repository-relative path.
func newModuleRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	module := "module " + fixtureModule + "\n\ngo 1.26.3\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(module), 0o600); err != nil {
		t.Fatalf("write module file: %v", err)
	}
	return root
}

const riskySource = `package fixture

func Risky(value int) int {
	if value > 10 {
		return value * 2
	}
	if value > 5 {
		return value + 2
	}
	if value > 0 {
		return value + 1
	}
	if value < -5 {
		return value - 1
	}
	return 0
}

func Simple(value int) int {
	return value
}
`

const windowsSource = `//go:build windows

package fixture

func OnlyOnWindows(value int) int {
	if value > 0 {
		return value
	}
	return -value
}
`

// scoredSource declares two functions with a fixed complexity and fixed lines.
// Simple starts at line 3 and has a complexity of 1. Risky starts at line 7,
// ends at line 18, and has a complexity of 4.
const scoredSource = `package fixture

func Simple(value int) int {
	return value
}

func Risky(value int) int {
	if value > 10 {
		return value * 2
	}
	if value > 5 {
		return value + 2
	}
	if value > 0 {
		return value + 1
	}
	return 0
}
`

// profileRunner writes a prepared coverage profile instead of starting the Go
// tool, so one test fixes the coverage of every function of a fixture.
type profileRunner struct {
	profile string
	failure error
	result  plugin.CommandResult
	command plugin.Command
}

var _ plugin.CommandRunner = (*profileRunner)(nil)

func (runner *profileRunner) Run(
	_ context.Context,
	command plugin.Command,
) (plugin.CommandResult, error) {
	runner.command = command
	if runner.failure != nil {
		return runner.result, runner.failure
	}
	for _, argument := range command.Arguments {
		path, selected := strings.CutPrefix(argument, "-coverprofile=")
		if !selected {
			continue
		}
		if err := os.WriteFile(path, []byte(runner.profile), 0o600); err != nil {
			return plugin.CommandResult{}, err
		}
	}
	return runner.result, nil
}

func writeFixture(t *testing.T, root, name, source string) {
	t.Helper()
	fullPath := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
		t.Fatalf("create fixture directory of %q: %v", name, err)
	}
	if err := os.WriteFile(fullPath, []byte(source), 0o600); err != nil {
		t.Fatalf("write fixture %q: %v", name, err)
	}
}

// scoredProfile writes one coverage profile for scoredSource. Simple keeps
// every statement covered. Risky covers the requested count of its four
// statements, so its coverage is 25 percent for each covered statement.
func scoredProfile(file string, coveredStatements int) string {
	lines := []string{
		"mode: atomic",
		fmt.Sprintf("%s/%s:3.28,5.2 1 1", fixtureModule, file),
	}
	for index, startLine := range []int{8, 11, 14, 17} {
		count := 0
		if index < coveredStatements {
			count = 1
		}
		lines = append(lines, fmt.Sprintf(
			"%s/%s:%d.16,%d.20 1 %d", fixtureModule, file, startLine, startLine+1, count,
		))
	}
	return strings.Join(lines, "\n") + "\n"
}

// permissiveConfiguration accepts every score, so a test reads measurements
// without the findings of the default limit.
func permissiveConfiguration() Configuration {
	configuration := DefaultConfiguration()
	configuration.MaximumScore = 1000
	return configuration
}

func scoreThreshold(comparison policy.Comparison, limit float64) policy.Threshold {
	return policy.Threshold{
		Metric:     metricCRAPScore,
		Comparison: comparison,
		Value:      limit,
		Severity:   plugin.SeverityError,
	}
}

// runProfile evaluates one fixture repository and returns the failure unchanged.
func runProfile(
	t *testing.T,
	root string,
	profile string,
	configuration Configuration,
	paths []string,
) (plugin.Report, error) {
	t.Helper()
	evaluator, err := NewEvaluator(&profileRunner{profile: profile}, configuration)
	if err != nil {
		t.Fatalf("create evaluator: %v", err)
	}
	return evaluator.Evaluate(t.Context(), plugin.Request{RepositoryRoot: root, Paths: paths})
}

func evaluateProfile(t *testing.T, root, profile string, configuration Configuration) plugin.Report {
	t.Helper()
	report, err := runProfile(t, root, profile, configuration, nil)
	if err != nil {
		t.Fatalf("evaluate CRAP: %v", err)
	}
	return report
}

// scoredFixture writes scoredSource and evaluates it with a prepared profile.
func scoredFixture(t *testing.T, coveredStatements int, configuration Configuration) plugin.Report {
	t.Helper()
	root := newModuleRoot(t)
	writeFixture(t, root, "scored.go", scoredSource)
	return evaluateProfile(t, root, scoredProfile("scored.go", coveredStatements), configuration)
}

func findMetric(t *testing.T, report plugin.Report, id string) plugin.Metric {
	t.Helper()
	for _, metric := range report.Metrics {
		if metric.ID == id {
			return metric
		}
	}
	t.Fatalf("the report holds no metric %q: %+v", id, report.Metrics)
	return plugin.Metric{}
}

func hasMetric(report plugin.Report, id string) bool {
	for _, metric := range report.Metrics {
		if metric.ID == id {
			return true
		}
	}
	return false
}

func findFinding(t *testing.T, report plugin.Report, id string) plugin.Finding {
	t.Helper()
	for _, finding := range report.Findings {
		if finding.ID == id {
			return finding
		}
	}
	t.Fatalf("the report holds no finding %q: %+v", id, report.Findings)
	return plugin.Finding{}
}

func hasFinding(report plugin.Report, id string) bool {
	for _, finding := range report.Findings {
		if finding.ID == id {
			return true
		}
	}
	return false
}

func newCRAPFixture(t *testing.T) string {
	t.Helper()
	root := newModuleRoot(t)
	writeFixture(t, root, "go.mod", "module example.com/crapfixture\n\ngo 1.26.3\n")
	writeFixture(t, root, "risk.go", riskySource)
	writeFixture(t, root, "risk_windows.go", windowsSource)
	writeFixture(t, root, "risk_test.go", "package fixture\n\nimport \"testing\"\n\n"+
		"func TestSimple(t *testing.T) {\n\tif Simple(3) != 3 {\n\t\tt.Fatal(\"simple\")\n\t}\n}\n")
	return root
}

func evaluateFixture(t *testing.T, configuration Configuration) plugin.Report {
	t.Helper()
	evaluator, err := NewEvaluator(plugin.NewCommandRunner(), configuration)
	if err != nil {
		t.Fatalf("create evaluator: %v", err)
	}
	report, err := evaluator.Evaluate(t.Context(), plugin.Request{
		RepositoryRoot: newCRAPFixture(t),
	})
	if err != nil {
		t.Fatalf("evaluate CRAP: %v", err)
	}
	return report
}

func TestEvaluatorMeasuresEveryProductionFunction(t *testing.T) {
	report := evaluateFixture(t, DefaultConfiguration())

	if report.Plugin != "crap" {
		t.Fatalf("report plugin is %q", report.Plugin)
	}
	complexities := make(map[string]float64)
	for _, metric := range report.Metrics {
		if metric.Name == "complexity.cyclomatic" {
			complexities[metric.Path] += metric.Value
		}
	}
	if complexities["risk.go"] != 6 {
		t.Errorf("risk.go reports a total complexity of %v, want 6", complexities["risk.go"])
	}
	if _, measured := complexities["risk_windows.go"]; !measured {
		t.Errorf("the build-tagged file has no metric: %v", complexities)
	}
}

func TestEvaluatorReportsFunctionsWithoutACRAPScore(t *testing.T) {
	report := evaluateFixture(t, DefaultConfiguration())

	unmeasured := make([]string, 0)
	for _, finding := range report.Findings {
		if finding.Rule != "indeterminate-crap-score" {
			continue
		}
		unmeasured = append(unmeasured, finding.Path)
		if finding.Severity != plugin.SeverityWarning {
			t.Errorf("finding severity is %q, want a warning", finding.Severity)
		}
	}
	if len(unmeasured) == 0 {
		t.Fatalf("the build-tagged file has no coverage and no warning: %+v", report.Findings)
	}
	for _, path := range unmeasured {
		if path != "risk_windows.go" {
			t.Errorf("unmeasured path is %q, want the build-tagged file", path)
		}
	}
}

func TestEvaluatorResolvesPathPoliciesWithARepositoryPath(t *testing.T) {
	configuration := permissiveConfiguration()
	configuration.Policies = []policy.PathPolicy{{
		ID:         "risky-sources",
		Include:    []string{"risk.go"},
		Thresholds: []policy.Threshold{scoreThreshold(policy.ComparisonMaximum, 1)},
	}}

	report := evaluateFixture(t, configuration)

	matched := false
	for _, finding := range report.Findings {
		if !strings.Contains(finding.ID, "risky-sources") {
			continue
		}
		matched = true
		if finding.Path != "risk.go" {
			t.Errorf("finding path is %q, want the repository-relative file", finding.Path)
		}
	}
	if !matched {
		t.Fatalf("a policy on a repository path matched nothing: %+v", report.Findings)
	}
}

// TestEvaluatorAppliesTheChangeRiskAntiPatternFormula checks the definition of
// Alberto Savoia and Bob Evans end to end. The library takes a percentage, so
// the test also proves that no layer converts the coverage twice.
func TestEvaluatorAppliesTheChangeRiskAntiPatternFormula(t *testing.T) {
	testCases := []struct {
		name              string
		coveredStatements int
		wantCoverage      float64
		wantScore         float64
	}{
		{
			name:              "a fully covered function scores exactly its own complexity",
			coveredStatements: 4,
			wantCoverage:      100,
			wantScore:         4,
		},
		{
			name:              "a function that covers three of its four statements",
			coveredStatements: 3,
			wantCoverage:      75,
			wantScore:         4.25,
		},
		{
			name:              "a function that covers half of its statements",
			coveredStatements: 2,
			wantCoverage:      50,
			wantScore:         6,
		},
		{
			name:              "a function that covers one of its four statements",
			coveredStatements: 1,
			wantCoverage:      25,
			wantScore:         10.75,
		},
		{
			name:              "an uncovered function scores its squared complexity plus its complexity",
			coveredStatements: 0,
			wantCoverage:      0,
			wantScore:         20,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			report := scoredFixture(t, testCase.coveredStatements, permissiveConfiguration())

			complexity := findMetric(t, report, "crap:complexity:scored.go:Risky:0")
			if complexity.Value != 4 {
				t.Fatalf("Risky reports a complexity of %v, want 4", complexity.Value)
			}
			coverage := findMetric(t, report, "crap:coverage:scored.go:Risky:0")
			if coverage.Value != testCase.wantCoverage {
				t.Errorf("Risky covers %v percent, want %v", coverage.Value, testCase.wantCoverage)
			}
			if coverage.Unit != "percent" {
				t.Errorf("the coverage metric unit is %q, want percent", coverage.Unit)
			}
			score := findMetric(t, report, "crap:score:scored.go:Risky:0")
			if score.Name != metricCRAPScore {
				t.Errorf("the score metric name is %q, want %q", score.Name, metricCRAPScore)
			}
			if score.Value != testCase.wantScore {
				t.Errorf("Risky scores %v, want %v", score.Value, testCase.wantScore)
			}
			simple := findMetric(t, report, "crap:score:scored.go:Simple:1")
			if simple.Value != 1 {
				t.Errorf("the fully covered Simple scores %v, want its complexity 1", simple.Value)
			}
		})
	}
}

func TestEvaluatorRaisesTheCRAPScoreAsCoverageFalls(t *testing.T) {
	previous := 0.0
	for coveredStatements := 4; coveredStatements >= 0; coveredStatements-- {
		report := scoredFixture(t, coveredStatements, permissiveConfiguration())

		score := findMetric(t, report, "crap:score:scored.go:Risky:0").Value
		if score <= previous {
			t.Fatalf(
				"Risky scores %v with %d covered statements, want more than %v",
				score,
				coveredStatements,
				previous,
			)
		}
		previous = score
	}
}

func TestEvaluatorReportsAnIndeterminateScoreOutsideTheCoverageProfile(t *testing.T) {
	root := newModuleRoot(t)
	writeFixture(t, root, "scored.go", scoredSource)
	profile := "mode: atomic\n" + fixtureModule + "/other.go:1.1,2.1 1 1\n"

	report := evaluateProfile(t, root, profile, permissiveConfiguration())

	findMetric(t, report, "crap:complexity:scored.go:Risky:0")
	if hasMetric(report, "crap:coverage:scored.go:Risky:0") {
		t.Errorf("an unmeasured function reports coverage: %+v", report.Metrics)
	}
	if hasMetric(report, "crap:score:scored.go:Risky:0") {
		t.Errorf("an unmeasured function reports a score: %+v", report.Metrics)
	}
	finding := findFinding(t, report, "crap:unmeasured:scored.go:Risky:0")
	if finding.Rule != "indeterminate-crap-score" {
		t.Errorf("the finding rule is %q, want indeterminate-crap-score", finding.Rule)
	}
	if finding.Severity != plugin.SeverityWarning {
		t.Errorf("the finding severity is %q, want a warning", finding.Severity)
	}
	if finding.Actual != nil || finding.Limit != nil {
		t.Errorf("an indeterminate finding carries a limit: %+v", finding)
	}
	if !strings.Contains(finding.Message, "fixture.Risky") {
		t.Errorf("the finding message is %q, want the package and the function", finding.Message)
	}
}

// TestEvaluatorIdentifiesEachFunctionByFileBeforeName pins the order of the
// analysis. The identity of every metric holds the position of its function, so
// a different order changes every identifier of the report.
func TestEvaluatorIdentifiesEachFunctionByFileBeforeName(t *testing.T) {
	root := newModuleRoot(t)
	writeFixture(t, root, "alpha.go", "package fixture\n\nfunc Zebra() int {\n\treturn 1\n}\n")
	writeFixture(t, root, "zebra.go", "package fixture\n\nfunc Alpha() int {\n\treturn 1\n}\n")
	profile := strings.Join([]string{
		"mode: atomic",
		fixtureModule + "/alpha.go:3.18,5.2 1 1",
		fixtureModule + "/zebra.go:3.18,5.2 1 1",
	}, "\n") + "\n"

	report := evaluateProfile(t, root, profile, permissiveConfiguration())

	findMetric(t, report, "crap:score:alpha.go:Zebra:0")
	findMetric(t, report, "crap:score:zebra.go:Alpha:1")
	if len(report.Findings) != 0 {
		t.Errorf("two covered simple functions report findings: %+v", report.Findings)
	}
}

// TestEvaluatorHonoursTheThresholdComparison proves that a minimum limit is not
// evaluated as a maximum. Risky keeps full coverage, so it scores exactly 4.
func TestEvaluatorHonoursTheThresholdComparison(t *testing.T) {
	testCases := []struct {
		name        string
		comparison  policy.Comparison
		limit       float64
		wantFinding bool
	}{
		{
			name:        "a maximum limit under the score reports a finding",
			comparison:  policy.ComparisonMaximum,
			limit:       1,
			wantFinding: true,
		},
		{
			name:        "a maximum limit over the score reports no finding",
			comparison:  policy.ComparisonMaximum,
			limit:       10,
			wantFinding: false,
		},
		{
			name:        "a minimum limit over the score reports a finding",
			comparison:  policy.ComparisonMinimum,
			limit:       10,
			wantFinding: true,
		},
		{
			name:        "a minimum limit under the score reports no finding",
			comparison:  policy.ComparisonMinimum,
			limit:       1,
			wantFinding: false,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			configuration := permissiveConfiguration()
			configuration.Policies = []policy.PathPolicy{{
				ID:         "score-limit",
				Include:    []string{"scored.go"},
				Thresholds: []policy.Threshold{scoreThreshold(testCase.comparison, testCase.limit)},
			}}

			report := scoredFixture(t, 4, configuration)

			identity := "crap:score-limit:scored.go:Risky:0"
			if !testCase.wantFinding {
				if hasFinding(report, identity) {
					t.Fatalf("a satisfied limit reports a finding: %+v", report.Findings)
				}
				return
			}
			finding := findFinding(t, report, identity)
			if finding.Rule != "maximum-crap-score" {
				t.Errorf("the finding rule is %q, want maximum-crap-score", finding.Rule)
			}
			if finding.Severity != plugin.SeverityError {
				t.Errorf("the finding severity is %q, want an error", finding.Severity)
			}
			if finding.Actual == nil || *finding.Actual != 4 {
				t.Errorf("the finding reports the actual value %v, want 4", finding.Actual)
			}
			if finding.Limit == nil || *finding.Limit != testCase.limit {
				t.Errorf("the finding reports the limit %v, want %v", finding.Limit, testCase.limit)
			}
			wanted := fmt.Sprintf("outside the %s limit", testCase.comparison)
			if !strings.Contains(finding.Message, wanted) {
				t.Errorf("the finding message is %q, want %q", finding.Message, wanted)
			}
		})
	}
}

// TestEvaluatorMatchesAPolicyOnADirectoryPattern checks that the path offered
// to the resolver is repository-relative, so a pattern over a directory selects
// every file under it.
func TestEvaluatorMatchesAPolicyOnADirectoryPattern(t *testing.T) {
	root := newModuleRoot(t)
	writeFixture(t, root, "plugin/crap/scored.go", scoredSource)
	configuration := permissiveConfiguration()
	configuration.Policies = []policy.PathPolicy{{
		ID:         "plugin-sources",
		Include:    []string{"plugin/**"},
		Thresholds: []policy.Threshold{scoreThreshold(policy.ComparisonMaximum, 1)},
	}}

	report := evaluateProfile(
		t,
		root,
		scoredProfile("plugin/crap/scored.go", 4),
		configuration,
	)

	finding := findFinding(t, report, "crap:plugin-sources:plugin/crap/scored.go:Risky:0")
	if finding.Path != "plugin/crap/scored.go" {
		t.Errorf("the finding path is %q, want the repository-relative file", finding.Path)
	}
	if !strings.Contains(finding.Message, `policy "plugin-sources"`) {
		t.Errorf("the finding message is %q, want the matched policy", finding.Message)
	}
}

func TestEvaluatorAppliesTheConfiguredLimitWithoutAPolicy(t *testing.T) {
	configuration := DefaultConfiguration()
	configuration.MaximumScore = 3

	report := scoredFixture(t, 4, configuration)

	finding := findFinding(t, report, "crap:default:scored.go:Risky:0")
	if finding.Severity != plugin.SeverityError {
		t.Errorf("the default finding severity is %q, want an error", finding.Severity)
	}
	if finding.Limit == nil || *finding.Limit != 3 {
		t.Errorf("the default finding reports the limit %v, want 3", finding.Limit)
	}
	if hasFinding(report, "crap:default:scored.go:Simple:1") {
		t.Errorf("the fully covered Simple fails the default limit: %+v", report.Findings)
	}
}

func TestEvaluateRunsTheConfiguredCoverageCommand(t *testing.T) {
	root := newModuleRoot(t)
	writeFixture(t, root, "scored.go", scoredSource)
	runner := &profileRunner{profile: scoredProfile("scored.go", 4)}
	evaluator, err := NewEvaluator(runner, permissiveConfiguration())
	if err != nil {
		t.Fatalf("create evaluator: %v", err)
	}

	if _, err := evaluator.Evaluate(t.Context(), plugin.Request{RepositoryRoot: root}); err != nil {
		t.Fatalf("evaluate CRAP: %v", err)
	}

	if runner.command.Path != "go" {
		t.Errorf("the coverage command is %q, want go", runner.command.Path)
	}
	if runner.command.Directory != root {
		t.Errorf("the coverage command runs in %q, want %q", runner.command.Directory, root)
	}
	wanted := []string{"test", "-covermode=atomic"}
	for index, argument := range wanted {
		if runner.command.Arguments[index] != argument {
			t.Errorf("argument %d is %q, want %q", index, runner.command.Arguments[index], argument)
		}
	}
	last := runner.command.Arguments[len(runner.command.Arguments)-1]
	if last != "./..." {
		t.Errorf("the last argument is %q, want the configured package pattern", last)
	}
}

func TestEvaluateReportsTheOutputOfAFailedCoverageRun(t *testing.T) {
	root := newModuleRoot(t)
	writeFixture(t, root, "scored.go", scoredSource)
	runner := &profileRunner{
		failure: failure.Unavailable(`run command "go"`, nil),
		result: plugin.CommandResult{
			StandardOutput: []byte("FAIL example.com/crapfixture\n"),
			StandardError:  []byte("no required module provides package\n"),
		},
	}
	evaluator, err := NewEvaluator(runner, DefaultConfiguration())
	if err != nil {
		t.Fatalf("create evaluator: %v", err)
	}

	_, err = evaluator.Evaluate(t.Context(), plugin.Request{RepositoryRoot: root})

	if !errors.Is(err, failure.ErrUnavailable) {
		t.Fatalf("a failed coverage run reports %v, want an unavailable failure", err)
	}
	for _, wanted := range []string{"FAIL example.com/crapfixture", "no required module provides package"} {
		if !strings.Contains(err.Error(), wanted) {
			t.Errorf("the failure message is %q, want %q", err.Error(), wanted)
		}
	}
}

func TestEvaluateReturnsAClassifiedFailure(t *testing.T) {
	testCases := []struct {
		name         string
		source       string
		profile      string
		paths        []string
		policies     []policy.PathPolicy
		wantCategory error
	}{
		{
			name:         "the coverage profile is malformed",
			source:       scoredSource,
			profile:      "not a coverage profile\n",
			wantCategory: failure.ErrDataIntegrity,
		},
		{
			name:         "the repository holds an unparsable Go file",
			source:       "package fixture\n\nfunc Broken( {\n",
			profile:      "mode: atomic\n",
			wantCategory: failure.ErrDataIntegrity,
		},
		{
			name:         "the request names a path outside the repository",
			source:       scoredSource,
			profile:      scoredProfile("scored.go", 4),
			paths:        []string{filepath.Join("..", "outside")},
			wantCategory: failure.ErrValidation,
		},
		{
			name:    "two policies limit the same metric of the same path",
			source:  scoredSource,
			profile: scoredProfile("scored.go", 4),
			policies: []policy.PathPolicy{
				{
					ID:         "everything",
					Include:    []string{"**"},
					Thresholds: []policy.Threshold{scoreThreshold(policy.ComparisonMaximum, 8)},
				},
				{
					ID:         "scored-file",
					Include:    []string{"scored.go"},
					Thresholds: []policy.Threshold{scoreThreshold(policy.ComparisonMaximum, 4)},
				},
			},
			wantCategory: failure.ErrValidation,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			root := newModuleRoot(t)
			writeFixture(t, root, "scored.go", testCase.source)
			configuration := permissiveConfiguration()
			configuration.Policies = testCase.policies

			_, err := runProfile(t, root, testCase.profile, configuration, testCase.paths)

			if !errors.Is(err, testCase.wantCategory) {
				t.Fatalf("the evaluation reports %v, want the category %v", err, testCase.wantCategory)
			}
		})
	}
}

func TestNewEvaluatorAcceptsAZeroMaximumScore(t *testing.T) {
	configuration := DefaultConfiguration()
	configuration.MaximumScore = 0

	evaluator, err := NewEvaluator(&profileRunner{}, configuration)

	if err != nil {
		t.Fatalf("a zero maximum score reports %v, want an evaluator", err)
	}
	if evaluator.configuration.MaximumScore != 0 {
		t.Errorf("the evaluator keeps the limit %v, want 0", evaluator.configuration.MaximumScore)
	}
}

func TestNewEvaluatorRejectsInvalidConfiguration(t *testing.T) {
	testCases := []struct {
		name    string
		runner  plugin.CommandRunner
		prepare func(configuration *Configuration)
	}{
		{
			name:    "a nil command runner",
			runner:  nil,
			prepare: func(*Configuration) {},
		},
		{
			name:    "a command of only spaces",
			runner:  &profileRunner{},
			prepare: func(configuration *Configuration) { configuration.Command = " " },
		},
		{
			name:    "a negative maximum score",
			runner:  &profileRunner{},
			prepare: func(configuration *Configuration) { configuration.MaximumScore = -1 },
		},
		{
			name:   "a path policy without an identifier",
			runner: &profileRunner{},
			prepare: func(configuration *Configuration) {
				configuration.Policies = []policy.PathPolicy{{
					Include:    []string{"scored.go"},
					Thresholds: []policy.Threshold{scoreThreshold(policy.ComparisonMaximum, 8)},
				}}
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			configuration := DefaultConfiguration()
			testCase.prepare(&configuration)

			evaluator, err := NewEvaluator(testCase.runner, configuration)

			if !errors.Is(err, failure.ErrValidation) {
				t.Fatalf("the constructor reports %v, want a validation failure", err)
			}
			if evaluator != nil {
				t.Errorf("a rejected configuration creates the evaluator %+v", evaluator)
			}
		})
	}
}

func TestEvaluatorReportsItsStableName(t *testing.T) {
	evaluator, err := NewEvaluator(&profileRunner{}, DefaultConfiguration())
	if err != nil {
		t.Fatalf("create evaluator: %v", err)
	}
	if evaluator.Name() != "crap" {
		t.Errorf("the evaluator name is %q, want crap", evaluator.Name())
	}
}

func TestNewCoverageProfileCreatesAndRemovesOneTemporaryFile(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())

	path, remove, err := newCoverageProfile()

	if err != nil {
		t.Fatalf("create the coverage profile: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("inspect the coverage profile: %v", err)
	}
	if err := remove(); err != nil {
		t.Fatalf("remove the coverage profile: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the coverage profile still exists: %v", err)
	}
	if err := remove(); err != nil {
		t.Errorf("a second removal reports %v, want no failure", err)
	}
}

func TestNewCoverageProfileReportsAnUnavailableTemporaryDirectory(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "absent"))

	_, remove, err := newCoverageProfile()

	if !errors.Is(err, failure.ErrUnavailable) {
		t.Fatalf("an absent temporary directory reports %v, want an unavailable failure", err)
	}
	if remove != nil {
		t.Errorf("a failed creation returns a removal function")
	}
}

func TestNewCoverageProfileReportsAFailedRemoval(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("TMPDIR", directory)
	_, remove, err := newCoverageProfile()
	if err != nil {
		t.Fatalf("create the coverage profile: %v", err)
	}
	if err := os.Chmod(directory, 0o500); err != nil {
		t.Fatalf("restrict the temporary directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(directory, 0o700); err != nil {
			t.Errorf("restore the temporary directory: %v", err)
		}
	})

	if err := remove(); !errors.Is(err, failure.ErrUnavailable) {
		t.Errorf("a read-only directory reports %v, want an unavailable failure", err)
	}
}
