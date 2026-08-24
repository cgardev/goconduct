package loc

import (
	"errors"
	"testing"

	"github.com/cgardev/goconduct/pkg/failure"
	"github.com/cgardev/goconduct/pkg/plugin"
)

func evaluateLOCFixture(t *testing.T) plugin.Report {
	t.Helper()
	evaluator, err := NewEvaluator(fixtureConfiguration())
	if err != nil {
		t.Fatalf("create LOC evaluator: %v", err)
	}
	report, err := evaluator.Evaluate(t.Context(), plugin.Request{RepositoryRoot: locFixtureRoot(t)})
	if err != nil {
		t.Fatalf("evaluate LOC fixture: %v", err)
	}
	return report
}

func TestLOCQueriesReturnFocusedAndOrderedEvidence(t *testing.T) {
	report := evaluateLOCFixture(t)

	summary, err := Summary(report)
	if err != nil {
		t.Fatalf("query LOC summary: %v", err)
	}
	if summary.Summary.Files.Total != 7 || summary.Summary.Lines.Total != 25 {
		t.Errorf("the LOC summary is %+v", summary.Summary)
	}

	packages, err := Packages(report, PackagesParams{Sort: AggregateSortTotal, Limit: 1})
	if err != nil {
		t.Fatalf("query LOC packages: %v", err)
	}
	if packages.Matched != 4 || packages.Returned != 1 || packages.Packages[0].Path != "." {
		t.Errorf("the LOC package query is %+v", packages)
	}

	files, err := Files(report, FilesParams{
		Kind: SourceKindHandwritten, Sort: FileSortTotal, Limit: 1,
	})
	if err != nil {
		t.Fatalf("query LOC files: %v", err)
	}
	if files.Matched != 3 || files.Returned != 1 || files.Files[0].Path != "main.go" {
		t.Errorf("the LOC file query is %+v", files)
	}

	functions, err := Functions(report, FunctionsParams{
		Kind: SourceKindHandwritten, Sort: FunctionSortCode, Limit: 1,
	})
	if err != nil {
		t.Fatalf("query LOC functions: %v", err)
	}
	if functions.Matched != 3 || functions.Returned != 1 || functions.Functions[0].Name != "Main" {
		t.Errorf("the LOC function query is %+v", functions)
	}
}

func TestLOCQueriesApplySourceAndPathFilters(t *testing.T) {
	report := evaluateLOCFixture(t)

	files, err := Files(report, FilesParams{
		Package: "internal/protogen", Kind: SourceKindGenerated,
		Sort: FileSortPath,
	})
	if err != nil {
		t.Fatalf("query generated package files: %v", err)
	}
	if files.Matched != 1 || files.Files[0].Path != "internal/protogen/message.pb.go" {
		t.Errorf("the generated file query is %+v", files)
	}

	functions, err := Functions(report, FunctionsParams{
		File: "main_test.go", Kind: SourceKindTest, Sort: FunctionSortIdentifier,
	})
	if err != nil {
		t.Fatalf("query test file functions: %v", err)
	}
	if functions.Matched != 1 || functions.Functions[0].Name != "TestMain" {
		t.Errorf("the test function query is %+v", functions)
	}
}

func TestLOCSummaryHandlesGeneratedTestOverlap(t *testing.T) {
	report, err := plugin.NewReport("loc", []plugin.Metric{
		{ID: "repository-files-total", Name: "loc.repository.files.total", Value: 1},
		{ID: "repository-files-test", Name: "loc.repository.files.test", Value: 1},
		{ID: "repository-files-generated", Name: "loc.repository.files.generated", Value: 1},
		{ID: "repository-lines-total", Name: "loc.repository.lines.total", Value: 5},
		{ID: "repository-lines-test", Name: "loc.repository.lines.test", Value: 5},
		{ID: "repository-lines-generated", Name: "loc.repository.lines.generated", Value: 5},
		{ID: "repository-functions-total", Name: "loc.repository.functions.total", Value: 1},
		{ID: "repository-functions-test", Name: "loc.repository.functions.test", Value: 1},
		{ID: "repository-functions-generated", Name: "loc.repository.functions.generated", Value: 1},
		{ID: "file-files-total", Path: "generated_test.go", Name: "loc.file.files.total", Value: 1},
		{ID: "file-files-test", Path: "generated_test.go", Name: "loc.file.files.test", Value: 1},
		{ID: "file-files-generated", Path: "generated_test.go", Name: "loc.file.files.generated", Value: 1},
		{ID: "file-lines-total", Path: "generated_test.go", Name: "loc.file.lines.total", Value: 5},
		{ID: "file-functions-total", Path: "generated_test.go", Name: "loc.file.functions.total", Value: 1},
	}, nil)
	if err != nil {
		t.Fatalf("create overlap report: %v", err)
	}

	summary, err := Summary(report)
	if err != nil {
		t.Fatalf("query overlap summary: %v", err)
	}
	if summary.Summary.Files.Handwritten != 0 ||
		summary.Summary.Lines.Handwritten != 0 ||
		summary.Summary.Functions.Handwritten != 0 {
		t.Errorf("the overlap summary counts handwritten evidence: %+v", summary.Summary)
	}
}

func TestLOCQueryParsersRejectValuesOutsideTheirClosedVocabularies(t *testing.T) {
	tests := []struct {
		name  string
		parse func() error
	}{
		{name: "source kind", parse: func() error { _, err := ParseSourceKind("other"); return err }},
		{name: "aggregate sort", parse: func() error { _, err := ParseAggregateSort("other"); return err }},
		{name: "file sort", parse: func() error { _, err := ParseFileSort("other"); return err }},
		{name: "function sort", parse: func() error { _, err := ParseFunctionSort("other"); return err }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.parse(); !errors.Is(err, failure.ErrValidation) {
				t.Errorf("the parser reports %v, want a validation failure", err)
			}
		})
	}
}

func TestLOCQueriesRejectNegativeLimits(t *testing.T) {
	report := plugin.Report{Plugin: "loc", SchemaVersion: 1}
	queries := []func() error{
		func() error { _, err := Packages(report, PackagesParams{Limit: -1}); return err },
		func() error { _, err := Files(report, FilesParams{Limit: -1}); return err },
		func() error { _, err := Functions(report, FunctionsParams{Limit: -1}); return err },
	}
	for _, query := range queries {
		if err := query(); !errors.Is(err, failure.ErrValidation) {
			t.Errorf("the negative limit reports %v, want a validation failure", err)
		}
	}
}
