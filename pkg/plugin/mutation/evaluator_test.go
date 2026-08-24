package mutation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cgardev/goconduct/pkg/failure"
	"github.com/cgardev/goconduct/pkg/plugin"
)

// errSuiteFails stands for one test suite that reports a failure.
// A mutation run that answers with this error is one detected mutation.
var errSuiteFails = errors.New("the fixture test suite fails")

// gradeSource holds two mutation sites, both inside one function.
// The sites are the comparison of line 4 and the comparison of line 7.
const gradeSource = `package fixture

func Grade(score int) string {
	if score > 90 {
		return "A"
	}
	if score > 50 {
		return "B"
	}
	return "C"
}
`

// untestedSource holds one mutation site, the multiplication of line 4.
const untestedSource = `package fixture

func Untested(value int) int {
	return value * 2
}
`

// singleSiteSource holds one covered mutation site, the comparison of line 4.
const singleSiteSource = `package fixture

func Grade(score int) string {
	if score > 90 {
		return "A"
	}
	return "B"
}
`

// brokenSource holds a Go file that the parser rejects.
const brokenSource = "package fixture\n\nfunc Broken( {\n"

// gradeProfile marks every line of Grade as reached by the test suite.
const gradeProfile = "mode: atomic\nexample.com/fixture/grade.go:3.30,10.11 3 1\n"

// untestedProfile marks the only line of Untested as never reached.
const untestedProfile = "example.com/fixture/untested.go:3.30,4.18 1 0\n"

// realSuiteSource holds one covered function and one function without a test.
const realSuiteSource = `package fixture

func Grade(score int) string {
	if score > 90 {
		return "A"
	}
	return "B"
}

func Untested(value int) int {
	if value == 0 {
		return 1
	}
	return value * 2
}
`

// realSuiteTestSource reaches Grade only, so Untested stays uncovered.
const realSuiteTestSource = `package fixture

import "testing"

func TestGrade(t *testing.T) {
	if Grade(95) != "A" {
		t.Fatal("A")
	}
	if Grade(10) != "B" {
		t.Fatal("B")
	}
}
`

// mutationCall records one command that the evaluator asked the runner to run.
type mutationCall struct {
	command   plugin.Command
	bounded   bool
	remaining time.Duration
}

// mutationRunner answers every command of one evaluation without a child
// process. The coverage command writes the configured profile, and each
// mutation command answers with the next scripted outcome.
type mutationRunner struct {
	profile        string
	baseline       time.Duration
	coverageResult error
	protect        string
	outcomes       []error
	defaultOutcome error
	cancelAt       int
	cancel         context.CancelFunc
	calls          []mutationCall
	mutations      int
}

var _ plugin.CommandRunner = (*mutationRunner)(nil)

// Run records one command and answers it with the scripted outcome.
func (runner *mutationRunner) Run(
	ctx context.Context,
	command plugin.Command,
) (plugin.CommandResult, error) {
	call := mutationCall{command: command}
	if deadline, bounded := ctx.Deadline(); bounded {
		call.bounded = true
		call.remaining = time.Until(deadline)
	}
	runner.calls = append(runner.calls, call)
	for _, argument := range command.Arguments {
		profilePath, isProfile := strings.CutPrefix(argument, "-coverprofile=")
		if !isProfile {
			continue
		}
		time.Sleep(runner.baseline)
		return runner.answerCoverage(profilePath)
	}
	runner.mutations++
	if runner.mutations == runner.cancelAt {
		runner.cancel()
	}
	if runner.mutations <= len(runner.outcomes) {
		return plugin.CommandResult{}, runner.outcomes[runner.mutations-1]
	}
	return plugin.CommandResult{}, runner.defaultOutcome
}

// answerCoverage writes the profile of the unchanged suite.
func (runner *mutationRunner) answerCoverage(profilePath string) (plugin.CommandResult, error) {
	if runner.coverageResult != nil {
		return plugin.CommandResult{
			StandardError: []byte("the fixture build fails"),
		}, runner.coverageResult
	}
	if err := os.WriteFile(profilePath, []byte(runner.profile), 0o600); err != nil {
		return plugin.CommandResult{}, err
	}
	if runner.protect == "" {
		return plugin.CommandResult{}, nil
	}
	return plugin.CommandResult{}, os.Chmod(runner.protect, 0o500)
}

// mutationCommands lists the commands that ran one mutation.
func (runner *mutationRunner) mutationCommands() []mutationCall {
	return runner.calls[1:]
}

// newMutationFixture writes one Go module with the given files.
func newMutationFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	writeFixtureFile(t, root, "go.mod", "module example.com/fixture\n\ngo 1.26.3\n")
	for name, content := range files {
		writeFixtureFile(t, root, name, content)
	}
	return root
}

func writeFixtureFile(t *testing.T, root string, name string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("create the fixture directory of %s: %v", name, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write the fixture file %s: %v", name, err)
	}
}

// runEvaluation evaluates one fixture repository with a scripted runner.
func runEvaluation(
	t *testing.T,
	runner plugin.CommandRunner,
	configuration Configuration,
	root string,
) (plugin.Report, error) {
	t.Helper()
	evaluator, err := NewEvaluator(runner, configuration)
	if err != nil {
		t.Fatalf("create evaluator: %v", err)
	}
	return evaluator.Evaluate(t.Context(), plugin.Request{RepositoryRoot: root})
}

// evaluateFixture evaluates one fixture repository that must report no failure.
func evaluateFixture(
	t *testing.T,
	runner plugin.CommandRunner,
	configuration Configuration,
	root string,
) plugin.Report {
	t.Helper()
	report, err := runEvaluation(t, runner, configuration, root)
	if err != nil {
		t.Fatalf("evaluate mutation: %v", err)
	}
	return report
}

// executeConfiguration returns the default configuration with the run enabled.
func executeConfiguration() Configuration {
	configuration := DefaultConfiguration()
	configuration.Execute = true
	return configuration
}

// assertMetrics compares every measurement of one file with the expected set.
func assertMetrics(t *testing.T, report plugin.Report, path string, want map[string]float64) {
	t.Helper()
	measured := make(map[string]float64, len(report.Metrics))
	for _, metric := range report.Metrics {
		if metric.Path != path {
			continue
		}
		measured[metric.Name] = metric.Value
	}
	for name, expected := range want {
		value, found := measured[name]
		if !found {
			t.Errorf("the report holds no %s of %s", name, path)
			continue
		}
		if value != expected {
			t.Errorf("%s of %s is %v, want %v", name, path, value, expected)
		}
	}
	for name, value := range measured {
		if _, expected := want[name]; !expected {
			t.Errorf("%s of %s reports %v, want no such measurement", name, path, value)
		}
	}
}

// reportPaths lists every file that the report measures, without repetition.
func reportPaths(report plugin.Report) []string {
	paths := make([]string, 0, len(report.Metrics))
	for _, metric := range report.Metrics {
		if !slices.Contains(paths, metric.Path) {
			paths = append(paths, metric.Path)
		}
	}
	slices.Sort(paths)
	return paths
}

// survivorIdentifiers lists the surviving mutations that the report names.
func survivorIdentifiers(t *testing.T, report plugin.Report) []string {
	t.Helper()
	identifiers := make([]string, 0, len(report.Findings))
	for _, finding := range report.Findings {
		if finding.Rule != "surviving-mutation" {
			continue
		}
		if finding.Severity != plugin.SeverityNotice {
			t.Errorf("the surviving mutation %q reports severity %q", finding.ID, finding.Severity)
		}
		if finding.Actual != nil || finding.Limit != nil {
			t.Errorf("the surviving mutation %q carries a threshold", finding.ID)
		}
		identifiers = append(identifiers, finding.ID)
	}
	return identifiers
}

// limitFinding holds the comparison that one threshold finding reports.
type limitFinding struct {
	rule   string
	path   string
	actual float64
	limit  float64
}

// limitFindings collects every threshold finding of one report.
func limitFindings(t *testing.T, report plugin.Report) []limitFinding {
	t.Helper()
	collected := make([]limitFinding, 0, len(report.Findings))
	for _, finding := range report.Findings {
		if finding.Rule == "surviving-mutation" {
			continue
		}
		if finding.Severity != plugin.SeverityError {
			t.Errorf("the threshold finding %q reports severity %q", finding.ID, finding.Severity)
		}
		if finding.Actual == nil || finding.Limit == nil {
			t.Fatalf("the threshold finding %q reports no count or no limit", finding.ID)
		}
		collected = append(collected, limitFinding{
			rule: finding.Rule, path: finding.Path,
			actual: *finding.Actual, limit: *finding.Limit,
		})
	}
	return collected
}

// assertSourceIsRestored compares every fixture file with its original content.
func assertSourceIsRestored(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		restored, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatalf("read the fixture file %s: %v", name, err)
		}
		if string(restored) != content {
			t.Errorf("the evaluation leaves %s changed", name)
		}
	}
}

// packagePatterns reports the package selection of one command.
func packagePatterns(command plugin.Command) []string {
	patterns := make([]string, 0, len(command.Arguments))
	for _, argument := range command.Arguments {
		if argument == "test" || strings.HasPrefix(argument, "-") {
			continue
		}
		patterns = append(patterns, argument)
	}
	return patterns
}

func TestDefaultConfigurationReportsSitesWithoutRunningThem(t *testing.T) {
	configuration := DefaultConfiguration()

	if configuration.Command != "go" {
		t.Errorf("the default command is %q, want go", configuration.Command)
	}
	if !slices.Equal(configuration.Packages, []string{"./..."}) {
		t.Errorf("the default packages are %v, want the whole module", configuration.Packages)
	}
	if configuration.Paths != nil {
		t.Errorf("the default configuration selects the paths %v, want the whole repository",
			configuration.Paths)
	}
	if configuration.Execute {
		t.Error("the default configuration runs every mutation")
	}
	if configuration.TimeoutFactor != 10 {
		t.Errorf("the default timeout factor is %d, want 10", configuration.TimeoutFactor)
	}
	if configuration.MaximumSurvivors != 0 {
		t.Errorf("the default survivor limit is %d, want 0", configuration.MaximumSurvivors)
	}
	if configuration.MaximumUncovered != 0 {
		t.Errorf("the default uncovered limit is %d, want 0", configuration.MaximumUncovered)
	}
}

func TestNewEvaluatorValidatesItsConfiguration(t *testing.T) {
	testCases := []struct {
		name      string
		nilRunner bool
		change    func(configuration *Configuration)
		wantValid bool
	}{
		{name: "a nil command runner is rejected", nilRunner: true},
		{
			name:   "an empty command is rejected",
			change: func(configuration *Configuration) { configuration.Command = "   " },
		},
		{
			name:   "a timeout factor of zero is rejected",
			change: func(configuration *Configuration) { configuration.TimeoutFactor = 0 },
		},
		{
			name:   "a negative timeout factor is rejected",
			change: func(configuration *Configuration) { configuration.TimeoutFactor = -1 },
		},
		{
			name:      "a timeout factor of one is accepted",
			change:    func(configuration *Configuration) { configuration.TimeoutFactor = 1 },
			wantValid: true,
		},
		{
			name:   "a negative survivor limit is rejected",
			change: func(configuration *Configuration) { configuration.MaximumSurvivors = -1 },
		},
		{
			name:   "a negative uncovered limit is rejected",
			change: func(configuration *Configuration) { configuration.MaximumUncovered = -1 },
		},
		{
			name:      "a limit of zero is accepted",
			change:    func(configuration *Configuration) { configuration.MaximumSurvivors = 0 },
			wantValid: true,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			configuration := DefaultConfiguration()
			if testCase.change != nil {
				testCase.change(&configuration)
			}
			runner := plugin.NewCommandRunner()
			if testCase.nilRunner {
				runner = nil
			}

			evaluator, err := NewEvaluator(runner, configuration)

			if !testCase.wantValid {
				if !errors.Is(err, failure.ErrValidation) {
					t.Fatalf("the rejected configuration reports %v, want a validation failure", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("create evaluator: %v", err)
			}
			if name := evaluator.Name(); name != "mutation" {
				t.Errorf("the evaluator name is %q, want mutation", name)
			}
		})
	}
}

func TestNewEvaluatorCopiesTheConfiguredSlices(t *testing.T) {
	root := newMutationFixture(t, map[string]string{
		"alpha/grade.go": gradeSource,
		"beta/grade.go":  gradeSource,
	})
	configuration := DefaultConfiguration()
	configuration.Paths = []string{"alpha"}
	configuration.Packages = []string{"./alpha/..."}
	evaluator, err := NewEvaluator(&mutationRunner{
		profile: "mode: atomic\nexample.com/fixture/alpha/grade.go:3.30,10.11 3 1\n",
	}, configuration)
	if err != nil {
		t.Fatalf("create evaluator: %v", err)
	}
	configuration.Paths[0] = "beta"
	configuration.Packages[0] = "./beta/..."

	report, err := evaluator.Evaluate(t.Context(), plugin.Request{RepositoryRoot: root})

	if err != nil {
		t.Fatalf("evaluate mutation: %v", err)
	}
	if paths := reportPaths(report); !slices.Equal(paths, []string{"alpha/grade.go"}) {
		t.Errorf("the evaluation measures %v, want the files of the configured path", paths)
	}
}

func TestEvaluatorSelectsThePathsOfTheRequestBeforeTheConfiguredPaths(t *testing.T) {
	testCases := []struct {
		name              string
		configuredPaths   []string
		requestPaths      []string
		wantMeasuredPaths []string
	}{
		{
			name:              "the request replaces the configured paths",
			configuredPaths:   []string{"beta"},
			requestPaths:      []string{"alpha"},
			wantMeasuredPaths: []string{"alpha/grade.go"},
		},
		{
			name:              "an empty request keeps the configured paths",
			configuredPaths:   []string{"beta"},
			wantMeasuredPaths: []string{"beta/grade.go"},
		},
		{
			name:              "an empty selection measures the whole repository",
			wantMeasuredPaths: []string{"alpha/grade.go", "beta/grade.go"},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			root := newMutationFixture(t, map[string]string{
				"alpha/grade.go": gradeSource,
				"beta/grade.go":  gradeSource,
			})
			configuration := DefaultConfiguration()
			configuration.Paths = testCase.configuredPaths
			evaluator, err := NewEvaluator(&mutationRunner{profile: "mode: atomic\n"}, configuration)
			if err != nil {
				t.Fatalf("create evaluator: %v", err)
			}

			report, err := evaluator.Evaluate(t.Context(), plugin.Request{
				RepositoryRoot: root, Paths: testCase.requestPaths,
			})

			if err != nil {
				t.Fatalf("evaluate mutation: %v", err)
			}
			if paths := reportPaths(report); !slices.Equal(paths, testCase.wantMeasuredPaths) {
				t.Errorf("the evaluation measures %v, want %v", paths, testCase.wantMeasuredPaths)
			}
		})
	}
}

func TestEvaluatorSeparatesCoveredSitesFromUncoveredSitesInOneScan(t *testing.T) {
	root := newMutationFixture(t, map[string]string{
		"grade.go":    gradeSource,
		"untested.go": untestedSource,
	})
	runner := &mutationRunner{profile: gradeProfile + untestedProfile}

	report := evaluateFixture(t, runner, DefaultConfiguration(), root)

	assertMetrics(t, report, "grade.go", map[string]float64{
		"mutation.total": 2, "mutation.covered": 2, "mutation.uncovered": 0,
	})
	assertMetrics(t, report, "untested.go", map[string]float64{
		"mutation.total": 1, "mutation.covered": 0, "mutation.uncovered": 1,
	})
	if paths := reportPaths(report); !slices.Equal(paths, []string{"grade.go", "untested.go"}) {
		t.Errorf("the scan measures %v, want both production files", paths)
	}
	if identifiers := survivorIdentifiers(t, report); len(identifiers) != 0 {
		t.Errorf("the scan names the survivors %v, and it measures none", identifiers)
	}
	if len(runner.calls) != 1 {
		t.Errorf("the scan runs %d commands, want only the unchanged suite", len(runner.calls))
	}
}

func TestEvaluatorRunsTheCoveredMutationsOnly(t *testing.T) {
	files := map[string]string{"grade.go": gradeSource, "untested.go": untestedSource}
	root := newMutationFixture(t, files)
	runner := &mutationRunner{
		profile:  gradeProfile + untestedProfile,
		outcomes: []error{errSuiteFails, nil},
	}

	report := evaluateFixture(t, runner, executeConfiguration(), root)

	assertMetrics(t, report, "grade.go", map[string]float64{
		"mutation.total": 2, "mutation.covered": 2, "mutation.uncovered": 0,
		"mutation.killed": 1, "mutation.survived": 1, "mutation.killed.percent": 50,
	})
	assertMetrics(t, report, "untested.go", map[string]float64{
		"mutation.total": 1, "mutation.covered": 0, "mutation.uncovered": 1,
		"mutation.killed": 0, "mutation.survived": 0,
	})
	if commands := runner.mutationCommands(); len(commands) != 2 {
		t.Fatalf("the run executes %d mutations, want one for each covered site", len(commands))
	}
	identifiers := survivorIdentifiers(t, report)
	if !slices.Equal(identifiers, []string{"mutation:survivor:grade.go:7:1"}) {
		t.Errorf("the run names the survivors %v, want the second site of grade.go", identifiers)
	}
	for _, call := range runner.mutationCommands() {
		if call.command.Path != "go" || call.command.Directory != root {
			t.Errorf("one mutation runs %q in %q", call.command.Path, call.command.Directory)
		}
		if patterns := packagePatterns(call.command); !slices.Equal(patterns, []string{"./..."}) {
			t.Errorf("one mutation runs the packages %v, want the configured selection", patterns)
		}
	}
	assertSourceIsRestored(t, root, files)
}

func TestEvaluatorReportsThePercentageOfDetectedMutations(t *testing.T) {
	testCases := []struct {
		name         string
		outcomes     []error
		wantKilled   float64
		wantSurvived float64
		wantPercent  float64
	}{
		{
			name:       "every detected mutation reports one hundred percent",
			outcomes:   []error{errSuiteFails, errSuiteFails},
			wantKilled: 2, wantPercent: 100,
		},
		{
			name:       "one detected mutation of two reports fifty percent",
			outcomes:   []error{errSuiteFails, nil},
			wantKilled: 1, wantSurvived: 1, wantPercent: 50,
		},
		{
			name:         "no detected mutation reports zero percent",
			outcomes:     []error{nil, nil},
			wantSurvived: 2, wantPercent: 0,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			root := newMutationFixture(t, map[string]string{"grade.go": gradeSource})
			runner := &mutationRunner{profile: gradeProfile, outcomes: testCase.outcomes}
			configuration := executeConfiguration()
			configuration.MaximumSurvivors = 2

			report := evaluateFixture(t, runner, configuration, root)

			assertMetrics(t, report, "grade.go", map[string]float64{
				"mutation.total": 2, "mutation.covered": 2, "mutation.uncovered": 0,
				"mutation.killed": testCase.wantKilled, "mutation.survived": testCase.wantSurvived,
				"mutation.killed.percent": testCase.wantPercent,
			})
			if identifiers := survivorIdentifiers(t, report); len(identifiers) != int(testCase.wantSurvived) {
				t.Errorf("the run names %d survivors, want %v", len(identifiers), testCase.wantSurvived)
			}
		})
	}
}

func TestEvaluatorOmitsThePercentageWhenNoMutationRuns(t *testing.T) {
	files := map[string]string{"untested.go": untestedSource}
	root := newMutationFixture(t, files)
	runner := &mutationRunner{profile: "mode: atomic\n" + untestedProfile}
	configuration := executeConfiguration()
	configuration.MaximumUncovered = 1

	report := evaluateFixture(t, runner, configuration, root)

	assertMetrics(t, report, "untested.go", map[string]float64{
		"mutation.total": 1, "mutation.covered": 0, "mutation.uncovered": 1,
		"mutation.killed": 0, "mutation.survived": 0,
	})
	if commands := runner.mutationCommands(); len(commands) != 0 {
		t.Errorf("the run executes %d mutations over uncovered sites", len(commands))
	}
	if findings := limitFindings(t, report); len(findings) != 0 {
		t.Errorf("the run reports the threshold findings %v, want none", findings)
	}
	assertSourceIsRestored(t, root, files)
}

func TestEvaluatorAppliesTheConfiguredMutationLimits(t *testing.T) {
	testCases := []struct {
		name             string
		execute          bool
		outcomes         []error
		maximumSurvivors int
		maximumUncovered int
		wantFindings     []limitFinding
	}{
		{
			name:             "a scan within the uncovered limit reports no finding",
			maximumUncovered: 1,
		},
		{
			name: "a scan above the uncovered limit reports the count and the limit",
			wantFindings: []limitFinding{
				{rule: "maximum-uncovered-mutations", path: "untested.go", actual: 1, limit: 0},
			},
		},
		{
			name:             "a run without survivors reports no survivor finding",
			execute:          true,
			outcomes:         []error{errSuiteFails, errSuiteFails},
			maximumUncovered: 1,
		},
		{
			name:             "a run above the survivor limit reports the count and the limit",
			execute:          true,
			outcomes:         []error{nil, nil},
			maximumUncovered: 1,
			wantFindings: []limitFinding{
				{rule: "maximum-surviving-mutations", path: "grade.go", actual: 2, limit: 0},
			},
		},
		{
			name:             "a run within the survivor limit reports no finding",
			execute:          true,
			outcomes:         []error{nil, nil},
			maximumSurvivors: 2,
			maximumUncovered: 1,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			root := newMutationFixture(t, map[string]string{
				"grade.go": gradeSource, "untested.go": untestedSource,
			})
			runner := &mutationRunner{
				profile: gradeProfile + untestedProfile, outcomes: testCase.outcomes,
			}
			configuration := DefaultConfiguration()
			configuration.Execute = testCase.execute
			configuration.MaximumSurvivors = testCase.maximumSurvivors
			configuration.MaximumUncovered = testCase.maximumUncovered

			report := evaluateFixture(t, runner, configuration, root)

			findings := limitFindings(t, report)
			if !slices.Equal(findings, testCase.wantFindings) {
				t.Errorf("the report holds the threshold findings %v, want %v",
					findings, testCase.wantFindings)
			}
		})
	}
}

func TestEvaluatorBoundsEveryMutationWithADeadline(t *testing.T) {
	testCases := []struct {
		name        string
		baseline    time.Duration
		factor      int
		wantAtLeast time.Duration
		wantAtMost  time.Duration
	}{
		{
			name:        "a suite that ends at once still bounds every mutation",
			factor:      1,
			wantAtLeast: minimumMutationTimeout - time.Second,
			wantAtMost:  minimumMutationTimeout,
		},
		{
			name:        "a slow suite bounds every mutation with its own duration",
			baseline:    20 * time.Millisecond,
			factor:      1000,
			wantAtLeast: 3 * minimumMutationTimeout,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			root := newMutationFixture(t, map[string]string{"grade.go": gradeSource})
			runner := &mutationRunner{
				profile: gradeProfile, baseline: testCase.baseline,
				defaultOutcome: errSuiteFails,
			}
			configuration := executeConfiguration()
			configuration.TimeoutFactor = testCase.factor

			evaluateFixture(t, runner, configuration, root)

			if runner.calls[0].bounded {
				t.Error("the unchanged suite runs with a deadline")
			}
			for index, call := range runner.mutationCommands() {
				if !call.bounded {
					t.Fatalf("mutation %d runs without a deadline", index)
				}
				if call.remaining < testCase.wantAtLeast {
					t.Errorf("mutation %d gets %v, want at least %v",
						index, call.remaining, testCase.wantAtLeast)
				}
				if testCase.wantAtMost != 0 && call.remaining > testCase.wantAtMost {
					t.Errorf("mutation %d gets %v, want at most %v",
						index, call.remaining, testCase.wantAtMost)
				}
			}
		})
	}
}

func TestEvaluatorCountsATimedOutMutationAsDetected(t *testing.T) {
	root := newMutationFixture(t, map[string]string{"grade.go": gradeSource})
	runner := &mutationRunner{profile: gradeProfile, outcomes: []error{
		failure.Unavailable(`run command "go"`, context.DeadlineExceeded),
		errSuiteFails,
	}}
	configuration := executeConfiguration()

	report := evaluateFixture(t, runner, configuration, root)

	assertMetrics(t, report, "grade.go", map[string]float64{
		"mutation.total": 2, "mutation.covered": 2, "mutation.uncovered": 0,
		"mutation.killed": 2, "mutation.survived": 0, "mutation.killed.percent": 100,
	})
}

func TestEvaluatorStopsAndRestoresTheSourceOnCancellation(t *testing.T) {
	testCases := []struct {
		name     string
		files    map[string]string
		profile  string
		cancelAt int
		outcomes []error
	}{
		{
			name:     "the run stops before the next mutation of one file",
			files:    map[string]string{"grade.go": gradeSource},
			profile:  gradeProfile,
			cancelAt: 1,
			outcomes: []error{nil, nil},
		},
		{
			name:     "the run stops when the cancelled mutation reports a failure",
			files:    map[string]string{"grade.go": gradeSource},
			profile:  gradeProfile,
			cancelAt: 2,
			outcomes: []error{nil, errSuiteFails},
		},
		{
			name: "the run stops before the next file",
			files: map[string]string{
				"alpha/grade.go": singleSiteSource, "beta/grade.go": singleSiteSource,
			},
			profile: "mode: atomic\n" +
				"example.com/fixture/alpha/grade.go:3.30,4.16 1 1\n" +
				"example.com/fixture/beta/grade.go:3.30,4.16 1 1\n",
			cancelAt: 1,
			outcomes: []error{nil, nil},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			root := newMutationFixture(t, testCase.files)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			runner := &mutationRunner{
				profile: testCase.profile, outcomes: testCase.outcomes,
				cancelAt: testCase.cancelAt, cancel: cancel,
			}
			evaluator, err := NewEvaluator(runner, executeConfiguration())
			if err != nil {
				t.Fatalf("create evaluator: %v", err)
			}

			_, err = evaluator.Evaluate(ctx, plugin.Request{RepositoryRoot: root})

			if !errors.Is(err, context.Canceled) {
				t.Fatalf("the cancelled evaluation reports %v, want a cancellation", err)
			}
			assertSourceIsRestored(t, root, testCase.files)
		})
	}
}

func TestEvaluatorRestoresTheSourceWhenOneMutationCannotBeWritten(t *testing.T) {
	files := map[string]string{"grade.go": gradeSource}
	root := newMutationFixture(t, files)
	source := filepath.Join(root, "grade.go")
	if err := os.Chmod(source, 0o400); err != nil {
		t.Fatalf("protect the fixture file: %v", err)
	}
	runner := &mutationRunner{profile: gradeProfile, defaultOutcome: errSuiteFails}

	_, err := runEvaluation(t, runner, executeConfiguration(), root)

	if !errors.Is(err, failure.ErrUnavailable) {
		t.Fatalf("a protected source reports %v, want an unavailable failure", err)
	}
	assertSourceIsRestored(t, root, files)
}

func TestEvaluatorReportsAFailureOfTheUnchangedSuite(t *testing.T) {
	root := newMutationFixture(t, map[string]string{"grade.go": gradeSource})
	runner := &mutationRunner{coverageResult: errSuiteFails}

	_, err := runEvaluation(t, runner, DefaultConfiguration(), root)

	if !errors.Is(err, errSuiteFails) {
		t.Fatalf("a failing unchanged suite reports %v, want the failure of the runner", err)
	}
	if !strings.Contains(err.Error(), "the fixture build fails") {
		t.Errorf("the failure message is %q, want the captured output", err.Error())
	}
}

func TestEvaluatorRejectsUnusableInput(t *testing.T) {
	testCases := []struct {
		name         string
		files        map[string]string
		profile      string
		requestPaths []string
		wantCategory error
	}{
		{
			name:         "a repository without a production Go file",
			files:        map[string]string{"grade_test.go": realSuiteTestSource},
			wantCategory: failure.ErrValidation,
		},
		{
			name:         "a selected path outside the repository",
			files:        map[string]string{"grade.go": gradeSource},
			requestPaths: []string{"../outside"},
			wantCategory: failure.ErrValidation,
		},
		{
			name:         "a Go file that the parser rejects",
			files:        map[string]string{"broken.go": brokenSource},
			profile:      "mode: atomic\n",
			wantCategory: failure.ErrDataIntegrity,
		},
		{
			name:         "a coverage profile that the parser rejects",
			files:        map[string]string{"grade.go": gradeSource},
			profile:      "this is no coverage profile\n",
			wantCategory: failure.ErrDataIntegrity,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			root := newMutationFixture(t, testCase.files)
			evaluator, err := NewEvaluator(
				&mutationRunner{profile: testCase.profile},
				DefaultConfiguration(),
			)
			if err != nil {
				t.Fatalf("create evaluator: %v", err)
			}

			_, err = evaluator.Evaluate(t.Context(), plugin.Request{
				RepositoryRoot: root, Paths: testCase.requestPaths,
			})

			if !errors.Is(err, testCase.wantCategory) {
				t.Fatalf("the evaluation reports %v, want the category %v", err, testCase.wantCategory)
			}
		})
	}
}

func TestEvaluatorReportsAFailureOfTheCoverageProfileFile(t *testing.T) {
	testCases := []struct {
		name    string
		prepare func(t *testing.T, directory string)
	}{
		{
			name: "the evaluation cannot create the profile",
			prepare: func(t *testing.T, directory string) {
				t.Helper()
				t.Setenv("TMPDIR", filepath.Join(directory, "missing"))
			},
		},
		{
			name: "the evaluation cannot remove the profile",
			prepare: func(t *testing.T, directory string) {
				t.Helper()
				t.Setenv("TMPDIR", directory)
				t.Cleanup(func() {
					if err := os.Chmod(directory, 0o700); err != nil {
						t.Errorf("restore the temporary directory: %v", err)
					}
				})
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			directory := t.TempDir()
			root := newMutationFixture(t, map[string]string{"grade.go": gradeSource})
			testCase.prepare(t, directory)
			runner := &mutationRunner{profile: gradeProfile, protect: directory}

			_, err := runEvaluation(t, runner, DefaultConfiguration(), root)

			if !errors.Is(err, failure.ErrUnavailable) {
				t.Fatalf("the profile failure reports %v, want an unavailable failure", err)
			}
		})
	}
}

func TestEvaluatorRunsTheRealSuiteForEveryCoveredMutation(t *testing.T) {
	files := map[string]string{
		"grade.go": realSuiteSource, "grade_test.go": realSuiteTestSource,
	}
	root := newMutationFixture(t, files)
	configuration := executeConfiguration()
	configuration.MaximumSurvivors = 1
	configuration.MaximumUncovered = 4

	report := evaluateFixture(t, plugin.NewCommandRunner(), configuration, root)

	assertMetrics(t, report, "grade.go", map[string]float64{
		"mutation.total": 5, "mutation.covered": 1, "mutation.uncovered": 4,
		"mutation.killed": 0, "mutation.survived": 1, "mutation.killed.percent": 0,
	})
	identifiers := survivorIdentifiers(t, report)
	if !slices.Equal(identifiers, []string{"mutation:survivor:grade.go:4:0"}) {
		t.Errorf("the real run names the survivors %v, want the comparison of line 4", identifiers)
	}
	if findings := limitFindings(t, report); len(findings) != 0 {
		t.Errorf("the real run reports the threshold findings %v, want none", findings)
	}
	assertSourceIsRestored(t, root, files)
}

func TestEvaluatorScansTheRealRepositoryWithoutChangingIt(t *testing.T) {
	files := map[string]string{
		"grade.go": realSuiteSource, "grade_test.go": realSuiteTestSource,
	}
	root := newMutationFixture(t, files)

	report := evaluateFixture(t, plugin.NewCommandRunner(), DefaultConfiguration(), root)

	assertMetrics(t, report, "grade.go", map[string]float64{
		"mutation.total": 5, "mutation.covered": 1, "mutation.uncovered": 4,
	})
	assertSourceIsRestored(t, root, files)
}
