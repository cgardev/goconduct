import { computed, inject, Service, signal } from '@angular/core';
import { GraphStore } from './graph.store';

/**
 * The component the reader inspects, held for the whole console.
 *
 * The map and the components table both select, and both pages open the same
 * detail drawer. The selection therefore lives beside the graph rather than in
 * either page, so a reader who selects a node on the map still sees that
 * component when the components table opens.
 */
@Service()
export class ComponentSelectionStore {
  private readonly graph = inject(GraphStore);

  private readonly selectedId = signal('');

  /** Identifier of the selected component, empty when nothing is selected. */
  readonly selectedComponentId = this.selectedId.asReadonly();

  /** The selected component, undefined when nothing is selected. */
  readonly selectedComponent = computed(() => {
    const identifier = this.selectedId();
    return this.graph.components().find((component) => component.id === identifier);
  });

  /** Every dependency the selected component takes part in. */
  readonly selectedRelationships = computed(() => {
    const identifier = this.selectedId();
    if (identifier === '') {
      return [];
    }
    return this.graph
      .relationships()
      .filter(
        (relationship) => relationship.source === identifier || relationship.target === identifier,
      );
  });

  /** Selects the component with the given identifier. */
  select(identifier: string): void {
    this.selectedId.set(identifier);
  }

  /** Clears the selection and closes the detail drawer. */
  close(): void {
    this.selectedId.set('');
  }
}
