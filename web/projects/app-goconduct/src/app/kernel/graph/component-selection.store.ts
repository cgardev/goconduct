import { computed, inject, Service, signal } from '@angular/core';
import { GraphStore } from './graph.store';
import { readMetrics } from './metric-reading';

/** One component on either side of the selected one. */
export interface Neighbor {
  readonly id: string;
  readonly name: string;
  readonly role: string;
  /** Whether only test files create the dependency, so it does not count toward coupling. */
  readonly testOnly: boolean;
}

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

  /** Plain-language reading of the selected component's metrics, undefined when nothing is selected. */
  readonly selectedReading = computed(() => {
    const component = this.selectedComponent();
    return component === undefined ? undefined : readMetrics(component);
  });

  /** The components that import the selected one, the source of its afferent coupling. */
  readonly importers = computed(() =>
    this.selectedRelationships()
      .filter((relationship) => relationship.target === this.selectedId())
      .map((relationship) => this.neighbor(relationship.source, relationship.testOnly)),
  );

  /** The components the selected one imports, the source of its efferent coupling. */
  readonly dependencies = computed(() =>
    this.selectedRelationships()
      .filter((relationship) => relationship.source === this.selectedId())
      .map((relationship) => this.neighbor(relationship.target, relationship.testOnly)),
  );

  /** Selects the component with the given identifier. */
  select(identifier: string): void {
    this.selectedId.set(identifier);
  }

  /** Clears the selection and closes the detail drawer. */
  close(): void {
    this.selectedId.set('');
  }

  private neighbor(identifier: string, testOnly: boolean): Neighbor {
    const component = this.graph.components().find((candidate) => candidate.id === identifier);
    return {
      id: identifier,
      name: component?.name ?? (identifier.split('/').at(-1) ?? identifier),
      role: component?.role ?? '',
      testOnly,
    };
  }
}
