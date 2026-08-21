package architecture

import (
	"reflect"
	"testing"
)

func TestComponent_ExcludePresentationCategory(t *testing.T) {
	t.Run("Scenario: The architecture model stays independent of presentation categories", func(t *testing.T) {
		var componentType reflect.Type
		var categoryExists bool

		t.Run("Given the pure architecture component type", func(*testing.T) {
			componentType = reflect.TypeOf(Component{})
		})

		t.Run("When the test searches for the presentation category field", func(*testing.T) {
			_, categoryExists = componentType.FieldByName("Category")
		})

		t.Run("Then the pure component excludes the presentation field", func(t *testing.T) {
			if categoryExists {
				t.Error("the pure component contains the presentation category")
			}
		})
	})
}
