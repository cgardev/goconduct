package duplication

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/cgardev/goconduct/plugin"
)

type sourceRange struct {
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

type candidate struct {
	Score      float64     `json:"score"`
	Left       sourceRange `json:"left"`
	Right      sourceRange `json:"right"`
	LeftNodes  int         `json:"left_nodes"`
	RightNodes int         `json:"right_nodes"`
}

type dryReport struct {
	Candidates []candidate `json:"candidates"`
}

// Evaluator executes dry4go and normalizes duplicate candidates.
type Evaluator struct {
	runner        plugin.CommandRunner
	configuration Configuration
}

var _ plugin.Evaluator = (*Evaluator)(nil)

// NewEvaluator validates configuration and creates a duplication evaluator.
func NewEvaluator(runner plugin.CommandRunner, configuration Configuration) (*Evaluator, error) {
	if runner == nil {
		return nil, fmt.Errorf("duplication command runner is nil")
	}
	if strings.TrimSpace(configuration.Command) == "" {
		return nil, fmt.Errorf("duplication command is empty")
	}
	if configuration.Similarity < 0 || configuration.Similarity > 1 {
		return nil, fmt.Errorf("duplication similarity %.3f is outside 0 through 1", configuration.Similarity)
	}
	if configuration.MinimumLines <= 0 || configuration.MinimumNodes <= 0 {
		return nil, fmt.Errorf("duplication minimum lines and nodes must be positive")
	}
	if configuration.MaximumCandidates < 0 {
		return nil, fmt.Errorf("maximum duplication candidates is negative")
	}
	return &Evaluator{runner: runner, configuration: configuration}, nil
}

// Name returns the stable evaluator identifier.
func (*Evaluator) Name() string { return "duplication" }

// Evaluate runs dry4go for the selected repository paths.
func (evaluator *Evaluator) Evaluate(
	ctx context.Context,
	request plugin.Request,
) (plugin.Report, error) {
	root := request.RepositoryRoot
	if root == "" {
		root = "."
	}
	arguments := []string{
		"--json",
		"--threshold", strconv.FormatFloat(evaluator.configuration.Similarity, 'f', -1, 64),
		"--min-lines", strconv.Itoa(evaluator.configuration.MinimumLines),
		"--min-nodes", strconv.Itoa(evaluator.configuration.MinimumNodes),
	}
	arguments = append(arguments, slices.Clone(request.Paths)...)
	result, err := evaluator.runner.Run(ctx, plugin.Command{
		Path: evaluator.configuration.Command, Arguments: arguments, Directory: root,
	})
	if err != nil {
		return plugin.Report{}, fmt.Errorf(
			"run dry4go: %w; stderr: %s",
			err,
			strings.TrimSpace(string(result.StandardError)),
		)
	}
	report, err := parseDryReport(result.StandardOutput)
	if err != nil {
		return plugin.Report{}, err
	}
	return evaluator.report(report.Candidates)
}

func parseDryReport(payload []byte) (dryReport, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var report dryReport
	if err := decoder.Decode(&report); err != nil {
		return dryReport{}, fmt.Errorf("decode dry4go report: %w", err)
	}
	slices.SortFunc(report.Candidates, compareCandidate)
	return report, nil
}

func compareCandidate(left, right candidate) int {
	if comparison := strings.Compare(left.Left.File, right.Left.File); comparison != 0 {
		return comparison
	}
	if comparison := left.Left.StartLine - right.Left.StartLine; comparison != 0 {
		return comparison
	}
	if comparison := strings.Compare(left.Right.File, right.Right.File); comparison != 0 {
		return comparison
	}
	return left.Right.StartLine - right.Right.StartLine
}

func (evaluator *Evaluator) report(candidates []candidate) (plugin.Report, error) {
	metrics := make([]plugin.Metric, 0, len(candidates)+1)
	findings := make([]plugin.Finding, 0)
	metrics = append(metrics, plugin.Metric{
		ID: "duplication:candidates", Name: "duplication.candidates",
		Value: float64(len(candidates)), Unit: "count",
	})
	for index, candidate := range candidates {
		ordinal := strconv.Itoa(index)
		metrics = append(metrics, plugin.Metric{
			ID: "duplication:similarity:" + ordinal, Path: candidate.Left.File,
			Name: "duplication.similarity", Value: candidate.Score,
		})
		if index < evaluator.configuration.MaximumCandidates {
			continue
		}
		actual := candidate.Score
		limit := evaluator.configuration.Similarity
		findings = append(findings, plugin.Finding{
			ID: "duplication:" + ordinal, Rule: "structural-duplication",
			Path: candidate.Left.File, Severity: plugin.SeverityError,
			Message: fmt.Sprintf(
				"%s:%d-%d resembles %s:%d-%d with %.3f similarity",
				candidate.Left.File,
				candidate.Left.StartLine,
				candidate.Left.EndLine,
				candidate.Right.File,
				candidate.Right.StartLine,
				candidate.Right.EndLine,
				candidate.Score,
			),
			Actual: &actual, Limit: &limit,
		})
	}
	return plugin.NewReport("duplication", metrics, findings)
}
