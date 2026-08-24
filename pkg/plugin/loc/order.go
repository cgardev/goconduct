package loc

import (
	"cmp"
	"strings"
)

func matchesSourceKind(test, generated bool, kind SourceKind) bool {
	switch kind {
	case SourceKindAll:
		return true
	case SourceKindHandwritten:
		return !test && !generated
	case SourceKindTest:
		return test
	case SourceKindGenerated:
		return generated
	default:
		return false
	}
}

func aggregateComparison(sortOrder AggregateSort) func(AggregateOverview, AggregateOverview) int {
	return func(left, right AggregateOverview) int {
		comparison := 0
		switch sortOrder {
		case AggregateSortPath:
		case AggregateSortTotal:
			comparison = cmp.Compare(right.Lines.Total, left.Lines.Total)
		case AggregateSortHandwritten:
			comparison = cmp.Compare(right.Lines.Handwritten, left.Lines.Handwritten)
		case AggregateSortTest:
			comparison = cmp.Compare(right.Lines.Test, left.Lines.Test)
		case AggregateSortGenerated:
			comparison = cmp.Compare(right.Lines.Generated, left.Lines.Generated)
		case AggregateSortCode:
			comparison = cmp.Compare(right.Lines.Code, left.Lines.Code)
		case AggregateSortComment:
			comparison = cmp.Compare(right.Lines.Comment, left.Lines.Comment)
		case AggregateSortBlank:
			comparison = cmp.Compare(right.Lines.Blank, left.Lines.Blank)
		case AggregateSortFunctions:
			comparison = cmp.Compare(right.Functions.Total, left.Functions.Total)
		case AggregateSortAverageFunction:
			comparison = cmp.Compare(right.FunctionLines.Average, left.FunctionLines.Average)
		case AggregateSortP95Function:
			comparison = cmp.Compare(right.FunctionLines.P95, left.FunctionLines.P95)
		case AggregateSortMaximumFunction:
			comparison = cmp.Compare(right.FunctionLines.Maximum, left.FunctionLines.Maximum)
		}
		return cmp.Or(comparison, strings.Compare(left.Path, right.Path))
	}
}

func fileComparison(sortOrder FileSort) func(FileOverview, FileOverview) int {
	return func(left, right FileOverview) int {
		comparison := 0
		switch sortOrder {
		case FileSortPath:
		case FileSortTotal:
			comparison = cmp.Compare(right.Lines.Total, left.Lines.Total)
		case FileSortCode:
			comparison = cmp.Compare(right.Lines.Code, left.Lines.Code)
		case FileSortComment:
			comparison = cmp.Compare(right.Lines.Comment, left.Lines.Comment)
		case FileSortBlank:
			comparison = cmp.Compare(right.Lines.Blank, left.Lines.Blank)
		case FileSortFunctions:
			comparison = cmp.Compare(right.Functions, left.Functions)
		}
		return cmp.Or(comparison, strings.Compare(left.Path, right.Path))
	}
}

func functionComparison(sortOrder FunctionSort) func(FunctionOverview, FunctionOverview) int {
	return func(left, right FunctionOverview) int {
		comparison := 0
		switch sortOrder {
		case FunctionSortIdentifier:
		case FunctionSortTotal:
			comparison = cmp.Compare(right.Lines.Total, left.Lines.Total)
		case FunctionSortCode:
			comparison = cmp.Compare(right.Lines.Code, left.Lines.Code)
		case FunctionSortComment:
			comparison = cmp.Compare(right.Lines.Comment, left.Lines.Comment)
		case FunctionSortBlank:
			comparison = cmp.Compare(right.Lines.Blank, left.Lines.Blank)
		}
		return cmp.Or(comparison, strings.Compare(left.ID, right.ID))
	}
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-08-23T21:34:41Z","module_hash":"d7e8139a5bdb1cc389f7b0cfa29245d95254880f43faacc880d38a652c7752c1","functions":[{"id":"func/matchesSourceKind","name":"matchesSourceKind","line":8,"end_line":21,"hash":"f12f8b1189a301d3c990552041bc2a91a01c1dee80cf312f33bdfccf4b4038bc"},{"id":"func/aggregateComparison","name":"aggregateComparison","line":23,"end_line":53,"hash":"214d3074972e3152e859cce5ab6c5cdc055bce28ec29e40b530ecf6ca29af8fd"},{"id":"func/fileComparison","name":"fileComparison","line":55,"end_line":73,"hash":"546c2d36036d310c20d62841e3fcc5007d3f5cb824e1ef647a1c468b131e6564"},{"id":"func/functionComparison","name":"functionComparison","line":75,"end_line":91,"hash":"e042bbf5bc6c21571071f899c09b9676d11311aa66a68cc2549defee46fce3ee"}]}
// mutate4go-manifest-end
