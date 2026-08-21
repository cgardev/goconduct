package report

import (
	"encoding/json"
	"testing"
)

func TestGraph_MarshalJSONContract(t *testing.T) {
	t.Run("Scenario: The report uses schema version 8 and the current metric field names", func(t *testing.T) {
		var graph Graph
		var payload []byte
		var marshalError error
		var document map[string]any
		var relationshipDocument map[string]any
		var functionDocument map[string]any

		t.Run("Given a report with relationship and function metrics", func(*testing.T) {
			graph = Graph{
				SchemaVersion: SchemaVersion,
				Relationships: []Relationship{{
					ProductionReferencingFiles: 1,
					TestReferencingFiles:       2,
					CallerFunctions:            3,
					CalleeFunctions:            4,
				}},
				Functions: []Function{{
					CrossComponentCalleeFunctions: 5,
					TransitiveCalleeFunctions:     6,
				}},
			}
		})

		t.Run("When the report is encoded as JSON", func(*testing.T) {
			payload, marshalError = json.Marshal(graph)
		})

		if !t.Run("Then the JSON report uses schema version 8", func(t *testing.T) {
			if marshalError != nil {
				t.Fatalf("encode report: %v", marshalError)
			}
			if SchemaVersion != 8 {
				t.Fatalf("schema version is %d, want 8", SchemaVersion)
			}
			if err := json.Unmarshal(payload, &document); err != nil {
				t.Fatalf("decode report: %v", err)
			}
			if document["schemaVersion"] != float64(8) {
				t.Fatalf("encoded schema version is %v, want 8", document["schemaVersion"])
			}

			relationships, ok := document["relationships"].([]any)
			if !ok || len(relationships) != 1 {
				t.Fatalf(
					"encoded relationships are %T with length %d, want one relationship",
					relationships,
					len(relationships),
				)
			}
			relationshipDocument, ok = relationships[0].(map[string]any)
			if !ok {
				t.Fatalf("encoded relationship has type %T, want an object", relationships[0])
			}

			functions, ok := document["functions"].([]any)
			if !ok || len(functions) != 1 {
				t.Fatalf("encoded functions are %T with length %d, want one function", functions, len(functions))
			}
			functionDocument, ok = functions[0].(map[string]any)
			if !ok {
				t.Fatalf("encoded function has type %T, want an object", functions[0])
			}
		}) {
			return
		}

		t.Run("And the JSON relationship uses the current metric field names", func(t *testing.T) {
			want := map[string]float64{
				"productionReferencingFiles": 1,
				"testReferencingFiles":       2,
				"callerFunctions":            3,
				"calleeFunctions":            4,
			}
			for field, value := range want {
				if relationshipDocument[field] != value {
					t.Errorf("relationship field %s is %v, want %v", field, relationshipDocument[field], value)
				}
			}
		})

		t.Run("And the JSON function uses the current callee field names", func(t *testing.T) {
			want := map[string]float64{
				"crossComponentCalleeFunctions": 5,
				"transitiveCalleeFunctions":     6,
			}
			for field, value := range want {
				if functionDocument[field] != value {
					t.Errorf("function field %s is %v, want %v", field, functionDocument[field], value)
				}
			}
		})

		t.Run("And the JSON report excludes the replaced field names", func(t *testing.T) {
			for _, field := range []string{
				"productionReferences",
				"testReferences",
				"callingFunctions",
				"calledFunctions",
			} {
				if _, exists := relationshipDocument[field]; exists {
					t.Errorf("relationship contains replaced field %s", field)
				}
			}
			for _, field := range []string{
				"crossComponentCalledFunctions",
				"transitiveCalledFunctions",
			} {
				if _, exists := functionDocument[field]; exists {
					t.Errorf("function contains replaced field %s", field)
				}
			}
		})
	})
}
