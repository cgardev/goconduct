package loc

import (
	"fmt"
	"path"

	querymodel "github.com/cgardev/goconduct/internal/query"
	"github.com/cgardev/goconduct/pkg/failure"
	"github.com/cgardev/goconduct/pkg/plugin"
)

// SourceKind selects LOC resources by their source classification.
type SourceKind string

const (
	// SourceKindAll selects every source classification.
	SourceKindAll SourceKind = "all"
	// SourceKindHandwritten selects sources that are neither tests nor generated.
	SourceKindHandwritten SourceKind = "handwritten"
	// SourceKindTest selects test sources.
	SourceKindTest SourceKind = "test"
	// SourceKindGenerated selects generated sources.
	SourceKindGenerated SourceKind = "generated"
)

// AggregateSort selects the metric that orders LOC aggregate resources.
type AggregateSort string

const (
	// AggregateSortPath orders aggregates by repository path.
	AggregateSortPath AggregateSort = "path"
	// AggregateSortTotal orders aggregates by total physical lines.
	AggregateSortTotal AggregateSort = "total"
	// AggregateSortHandwritten orders aggregates by handwritten non-test lines.
	AggregateSortHandwritten AggregateSort = "handwritten"
	// AggregateSortTest orders aggregates by test lines.
	AggregateSortTest AggregateSort = "test"
	// AggregateSortGenerated orders aggregates by generated lines.
	AggregateSortGenerated AggregateSort = "generated"
	// AggregateSortCode orders aggregates by code lines.
	AggregateSortCode AggregateSort = "code"
	// AggregateSortComment orders aggregates by comment lines.
	AggregateSortComment AggregateSort = "comment"
	// AggregateSortBlank orders aggregates by blank lines.
	AggregateSortBlank AggregateSort = "blank"
	// AggregateSortFunctions orders aggregates by declared functions.
	AggregateSortFunctions AggregateSort = "functions"
	// AggregateSortAverageFunction orders aggregates by average function code lines.
	AggregateSortAverageFunction AggregateSort = "average-function"
	// AggregateSortP95Function orders aggregates by their function line percentile.
	AggregateSortP95Function AggregateSort = "p95-function"
	// AggregateSortMaximumFunction orders aggregates by their largest function.
	AggregateSortMaximumFunction AggregateSort = "maximum-function"
)

// FileSort selects the metric that orders LOC file resources.
type FileSort string

const (
	// FileSortPath orders files by repository path.
	FileSortPath FileSort = "path"
	// FileSortTotal orders files by total physical lines.
	FileSortTotal FileSort = "total"
	// FileSortCode orders files by code lines.
	FileSortCode FileSort = "code"
	// FileSortComment orders files by comment lines.
	FileSortComment FileSort = "comment"
	// FileSortBlank orders files by blank lines.
	FileSortBlank FileSort = "blank"
	// FileSortFunctions orders files by declared functions.
	FileSortFunctions FileSort = "functions"
)

// FunctionSort selects the metric that orders LOC function resources.
type FunctionSort string

const (
	// FunctionSortIdentifier orders functions by their stable LOC identifier.
	FunctionSortIdentifier FunctionSort = "identifier"
	// FunctionSortTotal orders functions by total physical lines.
	FunctionSortTotal FunctionSort = "total"
	// FunctionSortCode orders functions by code lines.
	FunctionSortCode FunctionSort = "code"
	// FunctionSortComment orders functions by comment lines.
	FunctionSortComment FunctionSort = "comment"
	// FunctionSortBlank orders functions by blank lines.
	FunctionSortBlank FunctionSort = "blank"
)

// CountBreakdown separates total, handwritten, test, and generated counts.
type CountBreakdown struct {
	Total       int `json:"total"`
	Handwritten int `json:"handwritten"`
	Test        int `json:"test"`
	Generated   int `json:"generated"`
}

// LineBreakdown contains physical line measurements and source classifications.
type LineBreakdown struct {
	Total              int     `json:"total"`
	Code               int     `json:"code"`
	Comment            int     `json:"comment"`
	Blank              int     `json:"blank"`
	Handwritten        int     `json:"handwritten"`
	Test               int     `json:"test"`
	Generated          int     `json:"generated"`
	HandwrittenPercent float64 `json:"handwrittenPercent"`
	TestPercent        float64 `json:"testPercent"`
	GeneratedPercent   float64 `json:"generatedPercent"`
}

// FunctionLineBreakdown contains aggregate function code-line statistics.
type FunctionLineBreakdown struct {
	Average float64 `json:"average"`
	P95     int     `json:"p95"`
	Maximum int     `json:"maximum"`
}

// AggregateOverview contains grouped LOC evidence for one source scope.
type AggregateOverview struct {
	Path          string                `json:"path,omitempty"`
	Files         CountBreakdown        `json:"files"`
	Lines         LineBreakdown         `json:"lines"`
	Functions     CountBreakdown        `json:"functions"`
	FunctionLines FunctionLineBreakdown `json:"functionLines"`
}

// FileOverview contains grouped LOC evidence for one Go file.
type FileOverview struct {
	Path      string        `json:"path"`
	Test      bool          `json:"test"`
	Generated bool          `json:"generated"`
	Lines     LineBreakdown `json:"lines"`
	Functions int           `json:"functions"`
}

// FunctionOverview contains grouped LOC evidence for one declared Go function.
type FunctionOverview struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Path      string        `json:"path"`
	StartLine int           `json:"startLine"`
	EndLine   int           `json:"endLine"`
	Test      bool          `json:"test"`
	Generated bool          `json:"generated"`
	Lines     LineBreakdown `json:"lines"`
}

// QueryAnalysis identifies the source LOC report and its policy findings.
type QueryAnalysis struct {
	SchemaVersion   int    `json:"schemaVersion"`
	Plugin          string `json:"plugin"`
	Findings        int    `json:"findings"`
	FailingFindings int    `json:"failingFindings"`
}

// SummaryResult contains repository-wide LOC evidence.
type SummaryResult struct {
	Analysis QueryAnalysis     `json:"analysis"`
	Summary  AggregateOverview `json:"summary"`
}

// PackagesParams defines package ordering and the result limit.
type PackagesParams struct {
	Sort  AggregateSort
	Limit int
}

// PackagesResult contains package LOC summaries that match one query.
type PackagesResult struct {
	Analysis QueryAnalysis       `json:"analysis"`
	Matched  int                 `json:"matched"`
	Returned int                 `json:"returned"`
	Packages []AggregateOverview `json:"packages"`
}

// FilesParams defines file filters, ordering, and the result limit.
type FilesParams struct {
	Package string
	Kind    SourceKind
	Sort    FileSort
	Limit   int
}

// FilesResult contains file LOC summaries that match one query.
type FilesResult struct {
	Analysis QueryAnalysis  `json:"analysis"`
	Matched  int            `json:"matched"`
	Returned int            `json:"returned"`
	Files    []FileOverview `json:"files"`
}

// FunctionsParams defines function filters, ordering, and the result limit.
type FunctionsParams struct {
	Package string
	File    string
	Kind    SourceKind
	Sort    FunctionSort
	Limit   int
}

// FunctionsResult contains function LOC summaries that match one query.
type FunctionsResult struct {
	Analysis  QueryAnalysis      `json:"analysis"`
	Matched   int                `json:"matched"`
	Returned  int                `json:"returned"`
	Functions []FunctionOverview `json:"functions"`
}

// Summary returns repository-wide LOC evidence from one normalized report.
func Summary(report plugin.Report) (SummaryResult, error) {
	index, err := newMetricIndex(report)
	if err != nil {
		return SummaryResult{}, err
	}
	return SummaryResult{
		Analysis: queryAnalysis(report),
		Summary:  index.aggregate("repository", ""),
	}, nil
}

// Packages returns filtered and ordered package LOC evidence.
func Packages(report plugin.Report, params PackagesParams) (PackagesResult, error) {
	if err := querymodel.ValidateLimit(params.Limit); err != nil {
		return PackagesResult{}, err
	}
	index, err := newMetricIndex(report)
	if err != nil {
		return PackagesResult{}, err
	}
	packages := make([]AggregateOverview, 0)
	for _, metric := range report.Metrics {
		if metric.Name != "loc.package.files.total" {
			continue
		}
		packages = append(packages, index.aggregate("package", metric.Path))
	}
	selection := querymodel.Select(
		packages,
		nil,
		aggregateComparison(params.Sort),
		params.Limit,
	)
	return PackagesResult{
		Analysis: queryAnalysis(report),
		Matched:  selection.Matched,
		Returned: len(selection.Values),
		Packages: selection.Values,
	}, nil
}

// Files returns filtered and ordered file LOC evidence.
func Files(report plugin.Report, params FilesParams) (FilesResult, error) {
	if err := querymodel.ValidateLimit(params.Limit); err != nil {
		return FilesResult{}, err
	}
	index, err := newMetricIndex(report)
	if err != nil {
		return FilesResult{}, err
	}
	selection := querymodel.Select(index.files, func(file FileOverview) bool {
		return (params.Package == "" || path.Dir(file.Path) == params.Package) &&
			matchesSourceKind(file.Test, file.Generated, params.Kind)
	}, fileComparison(params.Sort), params.Limit)
	return FilesResult{
		Analysis: queryAnalysis(report),
		Matched:  selection.Matched,
		Returned: len(selection.Values),
		Files:    selection.Values,
	}, nil
}

// Functions returns filtered and ordered function LOC evidence.
func Functions(report plugin.Report, params FunctionsParams) (FunctionsResult, error) {
	if err := querymodel.ValidateLimit(params.Limit); err != nil {
		return FunctionsResult{}, err
	}
	index, err := newMetricIndex(report)
	if err != nil {
		return FunctionsResult{}, err
	}
	selection := querymodel.Select(index.functions, func(function FunctionOverview) bool {
		return (params.Package == "" || path.Dir(function.Path) == params.Package) &&
			(params.File == "" || function.Path == params.File) &&
			matchesSourceKind(function.Test, function.Generated, params.Kind)
	}, functionComparison(params.Sort), params.Limit)
	return FunctionsResult{
		Analysis:  queryAnalysis(report),
		Matched:   selection.Matched,
		Returned:  len(selection.Values),
		Functions: selection.Values,
	}, nil
}

// ParseSourceKind validates one source classification filter.
func ParseSourceKind(value string) (SourceKind, error) {
	kind := SourceKind(value)
	switch kind {
	case SourceKindAll, SourceKindHandwritten, SourceKindTest, SourceKindGenerated:
		return kind, nil
	default:
		return "", failure.Validation(
			fmt.Sprintf("source kind %q must be all, handwritten, test, or generated", value),
			nil,
		)
	}
}

// ParseAggregateSort validates one aggregate sort value.
func ParseAggregateSort(value string) (AggregateSort, error) {
	sortOrder := AggregateSort(value)
	switch sortOrder {
	case AggregateSortPath,
		AggregateSortTotal,
		AggregateSortHandwritten,
		AggregateSortTest,
		AggregateSortGenerated,
		AggregateSortCode,
		AggregateSortComment,
		AggregateSortBlank,
		AggregateSortFunctions,
		AggregateSortAverageFunction,
		AggregateSortP95Function,
		AggregateSortMaximumFunction:
		return sortOrder, nil
	default:
		return "", failure.Validation(
			fmt.Sprintf(
				"LOC aggregate sort %q must be path, total, handwritten, test, generated, code, "+
					"comment, blank, functions, average-function, p95-function, or maximum-function",
				value,
			),
			nil,
		)
	}
}

// ParseFileSort validates one file sort value.
func ParseFileSort(value string) (FileSort, error) {
	sortOrder := FileSort(value)
	switch sortOrder {
	case FileSortPath, FileSortTotal, FileSortCode, FileSortComment, FileSortBlank, FileSortFunctions:
		return sortOrder, nil
	default:
		return "", failure.Validation(
			fmt.Sprintf(
				"LOC file sort %q must be path, total, code, comment, blank, or functions",
				value,
			),
			nil,
		)
	}
}

// ParseFunctionSort validates one function sort value.
func ParseFunctionSort(value string) (FunctionSort, error) {
	sortOrder := FunctionSort(value)
	switch sortOrder {
	case FunctionSortIdentifier,
		FunctionSortTotal,
		FunctionSortCode,
		FunctionSortComment,
		FunctionSortBlank:
		return sortOrder, nil
	default:
		return "", failure.Validation(
			fmt.Sprintf(
				"LOC function sort %q must be identifier, total, code, comment, or blank",
				value,
			),
			nil,
		)
	}
}

func queryAnalysis(report plugin.Report) QueryAnalysis {
	return QueryAnalysis{
		SchemaVersion: report.SchemaVersion, Plugin: report.Plugin,
		Findings: len(report.Findings), FailingFindings: plugin.FailingFindings(report.Findings),
	}
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-23T21:34:41Z","module_hash":"27be54046c505060e4544c4a4ab3b09fe6e543c49dfb52a6251d5c9a80a301ec","functions":[{"id":"func/Summary","name":"Summary","line":211,"end_line":220,"hash":"7e6eeea487722dcc9c97e0cbc43569e9289c89753dad6a9b879b172ae4c5661d"},{"id":"func/Packages","name":"Packages","line":223,"end_line":250,"hash":"86bff8e373f4fc89e89aa7f946f9a1cff01c61792628fff63b047ea1b29e8ee7"},{"id":"func/Files","name":"Files","line":253,"end_line":271,"hash":"e9653bb18830972c57233b15d17d3f4fd7f60f677f253c76208e606b73d12fae"},{"id":"func/Functions","name":"Functions","line":274,"end_line":293,"hash":"f0f913220bd5a7c6730655dea2b9ba847514a4de6a73d7683dc977a59d99254c"},{"id":"func/ParseSourceKind","name":"ParseSourceKind","line":296,"end_line":307,"hash":"b70bc4701726d0769a3b7dcfdbe64facb16586f0aa33c2d84fefc67b80fc5262"},{"id":"func/ParseAggregateSort","name":"ParseAggregateSort","line":310,"end_line":336,"hash":"bfbbd2a7e955604decad5dcfbe5c992a7e272df0f80c4c48ef73241d129680ef"},{"id":"func/ParseFileSort","name":"ParseFileSort","line":339,"end_line":353,"hash":"8afd4968d6a25c3e33c7fcd62d5a46739f35e986697378e51abf55a77a1dad99"},{"id":"func/ParseFunctionSort","name":"ParseFunctionSort","line":356,"end_line":374,"hash":"381e58044e0a59e27e75752e43a192bd21b954d2da78a2a6d48c8f41d0b4e803"},{"id":"func/queryAnalysis","name":"queryAnalysis","line":376,"end_line":381,"hash":"22ca3d5c47676965e388276df15b6288168d3de03dc388d0b1778396b362f779"}]}
// mutate4go-manifest-end
