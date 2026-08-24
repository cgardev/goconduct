import { computed, DestroyRef, inject, Service, signal } from '@angular/core';
import { ConnectError } from '@connectrpc/connect';
import type { IncomingTypeRelation, TypeDeclaration } from 'lib-api-gen/gen/v1/graph_pb';
import { GRAPH_CLIENT } from '../client/api';

/**
 * The declared types of one component, read through `GetComponentTypes`.
 *
 * The store is application-scoped, like the graph store, so returning to the
 * types page keeps the last inventory while a navigation-triggered load
 * replaces it. The selection and the collapse state live beside the types
 * because the diagram and the detail panel both read them.
 */
@Service()
export class ComponentTypesStore {
  private readonly client = inject(GRAPH_CLIENT);
  private readonly abortController = new AbortController();
  // Identifies the most recent load, so a slow earlier response never
  // overwrites the inventory a later one already published.
  private loadSequence = 0;

  private readonly typesState = signal<readonly TypeDeclaration[]>([]);
  private readonly incomingState = signal<readonly IncomingTypeRelation[]>([]);
  private readonly selectedId = signal('');
  private readonly collapsedState = signal<ReadonlySet<string>>(new Set());

  /** Identifier of the loaded component, empty before the first load. */
  readonly componentId = signal('');

  /** Whether a load is in flight. */
  readonly loading = signal(false);

  /** Message of the last failure, empty when the last load succeeded. */
  readonly error = signal('');

  /** The declared types of the loaded component, in identifier order. */
  readonly types = this.typesState.asReadonly();

  /**
   * Relations that reach the loaded component from other components. Go
   * satisfies interfaces implicitly, so only this inverse view names the
   * implementers and the users of the component's types.
   */
  readonly incoming = this.incomingState.asReadonly();

  /** Identifier of the selected type, empty when nothing is selected. */
  readonly selectedTypeId = this.selectedId.asReadonly();

  /** Identifiers of the collapsed types. */
  readonly collapsedIds = this.collapsedState.asReadonly();

  /** The selected type, undefined when nothing is selected. */
  readonly selectedType = computed(() => {
    const identifier = this.selectedId();
    return this.typesState().find((declaration) => declaration.id === identifier);
  });

  /** The incoming relations that reach the selected type. */
  readonly selectedIncoming = computed(() => {
    const identifier = this.selectedId();
    if (identifier === '') {
      return [];
    }
    return this.incomingState().filter((relation) => relation.targetId === identifier);
  });

  constructor() {
    inject(DestroyRef).onDestroy(() => this.abortController.abort());
  }

  /** Loads the types of one component and resets the reading state. */
  async load(componentId: string): Promise<void> {
    if (componentId === '' || componentId === this.componentId()) {
      return;
    }
    const sequence = ++this.loadSequence;
    this.componentId.set(componentId);
    this.selectedId.set('');
    this.collapsedState.set(new Set());
    this.loading.set(true);
    try {
      const response = await this.client.getComponentTypes(
        { componentId },
        { signal: this.abortController.signal },
      );
      if (sequence !== this.loadSequence) {
        return;
      }
      this.typesState.set(response.types);
      this.incomingState.set(response.incoming);
      // Every type starts collapsed: the first reading is the relation map,
      // and a reader expands one type to study its members.
      this.collapsedState.set(new Set(response.types.map((declaration) => declaration.id)));
      this.error.set('');
    } catch (error: unknown) {
      if (this.abortController.signal.aborted || sequence !== this.loadSequence) {
        return;
      }
      this.typesState.set([]);
      this.incomingState.set([]);
      this.error.set(
        ConnectError.from(error).rawMessage || 'The component types are unavailable.',
      );
    } finally {
      if (sequence === this.loadSequence) {
        this.loading.set(false);
      }
    }
  }

  /** Selects one type, so the detail panel opens on it. */
  selectType(identifier: string): void {
    this.selectedId.set(identifier);
  }

  /** Closes the detail panel. */
  closeDetail(): void {
    this.selectedId.set('');
  }

  /** Collapses one expanded type, or expands one collapsed type. */
  toggleCollapsed(identifier: string): void {
    this.collapsedState.update((current) => {
      const next = new Set(current);
      if (!next.delete(identifier)) {
        next.add(identifier);
      }
      return next;
    });
  }
}
