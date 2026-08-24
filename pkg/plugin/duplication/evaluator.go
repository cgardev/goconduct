package duplication

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/cgardev/goconduct/internal/library/gosimilarity"
	"github.com/cgardev/goconduct/pkg/failure"
	"github.com/cgardev/goconduct/pkg/plugin"
)

// Evaluator reports structurally duplicate Go functions and methods.
type Evaluator struct {
	configuration Configuration
}

var _ plugin.Evaluator = (*Evaluator)(nil)

// NewEvaluator validates configuration and creates a duplication evaluator.
func NewEvaluator(configuration Configuration) (*Evaluator, error) {
	if configuration.Similarity < 0 || configuration.Similarity > 1 {
		return nil, failure.Validation(fmt.Sprintf(
			"duplication similarity %.3f is outside 0 through 1",
			configuration.Similarity,
		), nil)
	}
	if configuration.MinimumLines <= 0 || configuration.MinimumNodes <= 0 {
		return nil, failure.Validation("duplication minimum lines and nodes must be positive", nil)
	}
	if configuration.MaximumCandidates < 0 {
		return nil, failure.Validation("maximum duplication candidates is negative", nil)
	}
	return &Evaluator{configuration: configuration}, nil
}

// Name returns the stable evaluator identifier.
func (*Evaluator) Name() string { return "duplication" }

// Evaluate compares every selected Go function and reports duplicate candidates.
func (evaluator *Evaluator) Evaluate(
	ctx context.Context,
	request plugin.Request,
) (plugin.Report, error) {
	if err := ctx.Err(); err != nil {
		return plugin.Report{}, err
	}
	root, err := filepath.Abs(cmp.Or(request.RepositoryRoot, "."))
	if err != nil {
		return plugin.Report{}, failure.Internal("resolve duplication repository root", err)
	}
	paths, err := analysisPaths(root, request.Paths)
	if err != nil {
		return plugin.Report{}, err
	}
	candidates, err := gosimilarity.Candidates(
		ctx,
		paths,
		evaluator.configuration.Similarity,
		evaluator.configuration.MinimumLines,
		evaluator.configuration.MinimumNodes,
	)
	if err != nil {
		return plugin.Report{}, err
	}
	return evaluator.report(root, candidates)
}

// analysisPaths resolves each selected path inside the repository.
// dry4go skips a path it cannot read, so the evaluator rejects it instead.
func analysisPaths(root string, selected []string) ([]string, error) {
	if len(selected) == 0 {
		return []string{root}, nil
	}
	paths := make([]string, 0, len(selected))
	for _, candidate := range selected {
		full, err := repositoryEntry(root, candidate)
		if err != nil {
			return nil, err
		}
		paths = append(paths, full)
	}
	slices.Sort(paths)
	return slices.Compact(paths), nil
}

func repositoryEntry(root, candidate string) (string, error) {
	if strings.TrimSpace(candidate) == "" {
		return "", failure.Validation("duplication path is empty", nil)
	}
	full := filepath.Join(root, filepath.Clean(candidate))
	relative, err := filepath.Rel(root, full)
	escapes := err != nil || relative == ".." ||
		strings.HasPrefix(relative, ".."+string(filepath.Separator))
	if escapes {
		return "", failure.Validation(fmt.Sprintf(
			"duplication path %q is outside the repository",
			candidate,
		), nil)
	}
	if _, err := os.Stat(full); err != nil {
		return "", failure.Validation(fmt.Sprintf("inspect duplication path %q", candidate), err)
	}
	return full, nil
}

// compareCandidate ranks the most similar candidate first, as dry4go does.
// The candidate budget forgives the least similar candidates, so this order
// decides which duplicates the report keeps.
func compareCandidate(left, right gosimilarity.Candidate) int {
	return cmp.Or(
		cmp.Compare(right.Score, left.Score),
		compareLocation(left.Left, right.Left),
		compareLocation(left.Right, right.Right),
	)
}

// compareLocation orders two source coordinates by file and then by line.
// Two candidates with one score then keep one order in every run.
func compareLocation(left, right gosimilarity.Location) int {
	return cmp.Or(
		strings.Compare(left.File, right.File),
		cmp.Compare(left.StartLine, right.StartLine),
	)
}

func (evaluator *Evaluator) report(
	root string,
	candidates []gosimilarity.Candidate,
) (plugin.Report, error) {
	candidates = slices.Clone(candidates)
	slices.SortFunc(candidates, compareCandidate)
	metrics := []plugin.Metric{{
		ID: "duplication:candidates", Name: "duplication.candidates",
		Value: float64(len(candidates)), Unit: "count",
	}}
	findings := make([]plugin.Finding, 0)
	reported := max(len(candidates)-evaluator.configuration.MaximumCandidates, 0)
	for index, candidate := range candidates {
		left := repositoryPath(root, candidate.Left.File)
		right := repositoryPath(root, candidate.Right.File)
		identity := candidateIdentity(left, candidate.Left.StartLine, right, candidate.Right.StartLine)
		metrics = append(metrics, plugin.Metric{
			ID: "duplication:similarity:" + identity, Path: left,
			Name: "duplication.similarity", Value: candidate.Score,
		})
		if index >= reported {
			continue
		}
		actual := candidate.Score
		limit := evaluator.configuration.Similarity
		findings = append(findings, plugin.Finding{
			ID: "duplication:" + identity, Rule: "structural-duplication",
			Path: left, Severity: plugin.SeverityError,
			Message: fmt.Sprintf(
				"%s:%d-%d resembles %s:%d-%d with %.3f similarity",
				left,
				candidate.Left.StartLine,
				candidate.Left.EndLine,
				right,
				candidate.Right.StartLine,
				candidate.Right.EndLine,
				candidate.Score,
			),
			Actual: &actual, Limit: &limit,
		})
	}
	return plugin.NewReport("duplication", metrics, findings)
}

// repositoryPath reports one analyzed file relative to the repository root.
func repositoryPath(root, file string) string {
	relative, err := filepath.Rel(root, filepath.FromSlash(file))
	if err != nil {
		return filepath.ToSlash(file)
	}
	return filepath.ToSlash(relative)
}

// candidateIdentity names one duplicate pair by its source coordinates.
// The identifier stays stable when another duplicate appears or disappears.
func candidateIdentity(leftFile string, leftLine int, rightFile string, rightLine int) string {
	return leftFile + ":" + strconv.Itoa(leftLine) +
		":" + rightFile + ":" + strconv.Itoa(rightLine)
}
