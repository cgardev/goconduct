package architecture

import (
	"slices"
	"strings"
	"testing"

	"github.com/cgardev/goconduct/pkg/report"
)

func newTypeAnalysisFixture(t *testing.T) string {
	t.Helper()
	repositoryRoot := t.TempDir()
	writeFixtureFile(t, repositoryRoot, "go.mod", "module example.com/types\n\ngo 1.26\n")
	writeFixtureFile(
		t,
		repositoryRoot,
		"internal/library/telemetry/telemetry.go",
		`package telemetry

type Writer interface {
	Write(payload string) error
}

type Recorder struct {
	name string
}

func (recorder *Recorder) Write(payload string) error { return nil }
`,
	)
	writeFixtureFile(
		t,
		repositoryRoot,
		"internal/module/orders/orders.go",
		`package orders

import "example.com/types/internal/library/telemetry"

type Order struct {
	telemetry.Recorder
	Sink  telemetry.Writer
	Lines []Line
}

func (order Order) Total() int { return len(order.Lines) }

type Line struct {
	Quantity Quantity
}

type Quantity int

type Log = telemetry.Recorder

type Sink interface {
	telemetry.Writer
	Flush() error
}
`,
	)
	writeFixtureFile(
		t,
		repositoryRoot,
		"internal/module/orders/orders_test.go",
		`package orders

type recordedOrder struct {
	Order Order
}
`,
	)
	return repositoryRoot
}

func typeWithIdentifier(t *testing.T, graph Graph, identifier string) TypeDeclaration {
	t.Helper()
	for _, declaration := range graph.Types {
		if declaration.Identifier == identifier {
			return declaration
		}
	}
	t.Fatalf("the graph contains no type %q", identifier)
	return TypeDeclaration{}
}

func TestTypeAnalysis_ExtractDeclaredTypes(t *testing.T) {
	t.Run("Scenario: Two components declare structs, interfaces, aliases, and basic types", func(t *testing.T) {
		var sourceAnalyzer *analyzer
		var graph Graph
		var analysisError error

		t.Run("Given a repository with cross-component type relations", func(step *testing.T) {
			repositoryRoot := newTypeAnalysisFixture(t)
			var err error
			sourceAnalyzer, err = newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
			if err != nil {
				step.Fatalf("newAnalyzer fails: %v", err)
			}
		})

		t.Run("When the analyzer calculates the dependency graph", func(*testing.T) {
			graph, analysisError = sourceAnalyzer.analyze(t.Context())
		})

		if !t.Run("Then the graph reports every declared type with its kind", func(t *testing.T) {
			if analysisError != nil {
				t.Fatalf("type analysis fails: %v", analysisError)
			}
			kinds := map[string]report.TypeKind{
				"internal/library/telemetry.Writer":   report.TypeKindInterface,
				"internal/library/telemetry.Recorder": report.TypeKindStruct,
				"internal/module/orders.Order":        report.TypeKindStruct,
				"internal/module/orders.Line":         report.TypeKindStruct,
				"internal/module/orders.Quantity":     report.TypeKindBasic,
				"internal/module/orders.Log":          report.TypeKindAlias,
				"internal/module/orders.Sink":         report.TypeKindInterface,
			}
			for identifier, kind := range kinds {
				declaration := typeWithIdentifier(t, graph, identifier)
				if declaration.Kind != kind {
					t.Errorf("type %s has kind %q, want %q", identifier, declaration.Kind, kind)
				}
			}
		}) {
			return
		}

		t.Run("And a pointer receiver method satisfies the implemented interface", func(t *testing.T) {
			recorder := typeWithIdentifier(t, graph, "internal/library/telemetry.Recorder")
			want := []TypeReference{{
				Identifier: "internal/library/telemetry.Writer",
				Component:  "internal/library/telemetry",
			}}
			if !slices.Equal(recorder.Implements, want) {
				t.Errorf("Recorder implements %+v, want %+v", recorder.Implements, want)
			}
			if len(recorder.Methods) != 1 || !recorder.Methods[0].PointerReceiver {
				t.Errorf("Recorder methods are %+v, want one pointer receiver method", recorder.Methods)
			}
			if recorder.Methods[0].Signature != "(payload string) error" {
				t.Errorf("Recorder Write signature is %q", recorder.Methods[0].Signature)
			}
		})

		t.Run("And an embedded struct carries the declaring component identifier", func(t *testing.T) {
			order := typeWithIdentifier(t, graph, "internal/module/orders.Order")
			wantEmbeds := []TypeReference{{
				Identifier: "internal/library/telemetry.Recorder",
				Component:  "internal/library/telemetry",
			}}
			if !slices.Equal(order.Embeds, wantEmbeds) {
				t.Errorf("Order embeds %+v, want %+v", order.Embeds, wantEmbeds)
			}
			wantReferences := []TypeReference{
				{
					Identifier: "internal/library/telemetry.Writer",
					Component:  "internal/library/telemetry",
				},
				{Identifier: "internal/module/orders.Line", Component: "internal/module/orders"},
			}
			if !slices.Equal(order.References, wantReferences) {
				t.Errorf("Order references %+v, want %+v", order.References, wantReferences)
			}
			if len(order.Fields) != 3 || !order.Fields[0].Embedded || order.Fields[1].Name != "Sink" {
				t.Errorf("Order fields are %+v", order.Fields)
			}
		})

		t.Run("And the promoted pointer method set implements the interface", func(t *testing.T) {
			order := typeWithIdentifier(t, graph, "internal/module/orders.Order")
			want := []TypeReference{{
				Identifier: "internal/library/telemetry.Writer",
				Component:  "internal/library/telemetry",
			}}
			if !slices.Equal(order.Implements, want) {
				t.Errorf("Order implements %+v, want %+v", order.Implements, want)
			}
		})

		t.Run("And an alias reports its target and its named references", func(t *testing.T) {
			alias := typeWithIdentifier(t, graph, "internal/module/orders.Log")
			if alias.Underlying != "telemetry.Recorder" {
				t.Errorf("Log underlying is %q, want telemetry.Recorder", alias.Underlying)
			}
			want := []TypeReference{{
				Identifier: "internal/library/telemetry.Recorder",
				Component:  "internal/library/telemetry",
			}}
			if !slices.Equal(alias.References, want) {
				t.Errorf("Log references %+v, want %+v", alias.References, want)
			}
			if len(alias.Implements) != 0 || len(alias.Methods) != 0 {
				t.Errorf("Log carries implements=%v methods=%v", alias.Implements, alias.Methods)
			}
		})

		t.Run("And an embedded interface extends the embedded contract", func(t *testing.T) {
			sink := typeWithIdentifier(t, graph, "internal/module/orders.Sink")
			want := []TypeReference{{
				Identifier: "internal/library/telemetry.Writer",
				Component:  "internal/library/telemetry",
			}}
			if !slices.Equal(sink.Embeds, want) {
				t.Errorf("Sink embeds %+v, want %+v", sink.Embeds, want)
			}
			if !slices.Equal(sink.Implements, want) {
				t.Errorf("Sink implements %+v, want %+v", sink.Implements, want)
			}
			if len(sink.Methods) != 1 || sink.Methods[0].Name != "Flush" {
				t.Errorf("Sink methods are %+v, want the explicit Flush method", sink.Methods)
			}
		})

		t.Run("And a basic type reports its underlying type", func(t *testing.T) {
			quantity := typeWithIdentifier(t, graph, "internal/module/orders.Quantity")
			if quantity.Underlying != "int" || quantity.Exported != true {
				t.Errorf("Quantity declaration is %+v", quantity)
			}
		})

		t.Run("And a type declared in a test file carries the test marker", func(t *testing.T) {
			recorded := typeWithIdentifier(t, graph, "internal/module/orders.recordedOrder")
			if !recorded.Test || recorded.Exported {
				t.Errorf("recordedOrder declaration is %+v", recorded)
			}
			if !strings.HasSuffix(recorded.Path, "orders_test.go") {
				t.Errorf("recordedOrder path is %q", recorded.Path)
			}
		})

		t.Run("And the graph sorts types by stable identifiers", func(t *testing.T) {
			identifiers := make([]string, 0, len(graph.Types))
			for _, declaration := range graph.Types {
				identifiers = append(identifiers, declaration.Identifier)
			}
			if !slices.IsSorted(identifiers) {
				t.Errorf("type identifiers are not sorted: %v", identifiers)
			}
		})
	})
}

func TestTypeAnalysis_RepeatDeterministicResults(t *testing.T) {
	t.Run("Scenario: Two analyses of the same repository report identical types", func(t *testing.T) {
		var sourceAnalyzer *analyzer
		var first Graph
		var second Graph
		var analysisError error

		t.Run("Given an analyzer over a fixed repository", func(step *testing.T) {
			repositoryRoot := newTypeAnalysisFixture(t)
			var err error
			sourceAnalyzer, err = newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
			if err != nil {
				step.Fatalf("newAnalyzer fails: %v", err)
			}
		})

		t.Run("When the analyzer runs twice", func(t *testing.T) {
			first, analysisError = sourceAnalyzer.analyze(t.Context())
			if analysisError != nil {
				t.Fatalf("first analysis fails: %v", analysisError)
			}
			second, analysisError = sourceAnalyzer.analyze(t.Context())
			if analysisError != nil {
				t.Fatalf("second analysis fails: %v", analysisError)
			}
		})

		t.Run("Then both analyses report the same type inventory", func(t *testing.T) {
			if len(first.Types) == 0 {
				t.Fatal("the analysis reports no types")
			}
			if !slices.EqualFunc(first.Types, second.Types, func(a, b TypeDeclaration) bool {
				return a.Identifier == b.Identifier && a.Kind == b.Kind &&
					slices.Equal(a.Implements, b.Implements) &&
					slices.Equal(a.References, b.References)
			}) {
				t.Errorf("type inventories differ:\n%+v\n%+v", first.Types, second.Types)
			}
		})
	})
}

// Go satisfies a generic interface through an instantiation, which
// `types.Implements` cannot check without arguments. The analyzer infers the
// arguments from the candidate's methods, so an implicit implementation of a
// generic interface still reaches the report.
func TestTypeAnalysis_GenericInterfaceImplementation(t *testing.T) {
	t.Run("Scenario: A struct satisfies a generic interface of another component", func(t *testing.T) {
		var graph Graph
		var analysisError error

		t.Run("Given a generic interface and an implementer with a pointer receiver", func(step *testing.T) {
			repositoryRoot := t.TempDir()
			writeFixtureFile(t, repositoryRoot, "go.mod", "module example.com/generic\n\ngo 1.26\n")
			writeFixtureFile(
				t,
				repositoryRoot,
				"internal/library/contracts/contracts.go",
				`package contracts

type Snapshotter[T any] interface {
	ToSnapshot() T
}

type Collector[T any] interface {
	Collect() []T
}
`,
			)
			writeFixtureFile(
				t,
				repositoryRoot,
				"internal/module/store/store.go",
				`package store

type WidgetSnapshot struct {
	Total int
}

type Widget struct {
	total int
}

func (widget *Widget) ToSnapshot() WidgetSnapshot {
	return WidgetSnapshot{Total: widget.total}
}

func (widget *Widget) Collect() []WidgetSnapshot {
	return nil
}

type Mismatch struct{}

func (mismatch Mismatch) ToSnapshot() (WidgetSnapshot, error) {
	return WidgetSnapshot{}, nil
}
`,
			)
			sourceAnalyzer, err := newAnalyzer(fixtureAnalysisConfiguration(repositoryRoot))
			if err != nil {
				step.Fatalf("newAnalyzer fails: %v", err)
			}
			graph, analysisError = sourceAnalyzer.analyze(t.Context())
		})

		t.Run("Then the implementer reports both generic interfaces", func(t *testing.T) {
			if analysisError != nil {
				t.Fatalf("type analysis fails: %v", analysisError)
			}
			widget := typeWithIdentifier(t, graph, "internal/module/store.Widget")
			want := []TypeReference{
				{
					Identifier: "internal/library/contracts.Collector",
					Component:  "internal/library/contracts",
				},
				{
					Identifier: "internal/library/contracts.Snapshotter",
					Component:  "internal/library/contracts",
				},
			}
			if !slices.Equal(widget.Implements, want) {
				t.Errorf("Widget implements %+v, want %+v", widget.Implements, want)
			}
		})

		t.Run("And a signature that does not unify reports no implementation", func(t *testing.T) {
			mismatch := typeWithIdentifier(t, graph, "internal/module/store.Mismatch")
			if len(mismatch.Implements) != 0 {
				t.Errorf("Mismatch implements %+v, want none", mismatch.Implements)
			}
		})
	})
}
