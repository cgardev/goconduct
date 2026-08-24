import { Component, computed, effect, inject, input } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { TuiLoader } from '@taiga-ui/core';
import { ComponentTypesStore } from '../../../../kernel/graph/component-types.store';
import { GraphStore } from '../../../../kernel/graph/graph.store';
import { EmptyStateComponent } from '../../../../ui/empty-state/empty-state.component';
import { PageHeaderComponent } from '../../../../ui/page-header/page-header.component';
import { ComponentTreeComponent } from '../../component-tree/component-tree.component';
import { TypeDetailComponent } from '../../type-detail/type-detail.component';
import { TypeDiagramComponent } from '../../type-diagram/type-diagram.component';
import { buildTypeDiagramModel } from '../../type-diagram/type-diagram.model';

/**
 * UML-style diagram of the Go types of one analyzed component.
 *
 * The selected component travels in the `component` query parameter, so a
 * reader can share the exact diagram they are looking at, and a reload keeps
 * it. The URL is the single source of truth: the tree writes to it, and the
 * diagram is loaded from what it holds. Following a cross-component relation
 * writes the other component's identifier to the same parameter.
 */
@Component({
  selector: 'app-types-page',
  imports: [
    ComponentTreeComponent,
    EmptyStateComponent,
    PageHeaderComponent,
    TuiLoader,
    TypeDetailComponent,
    TypeDiagramComponent,
  ],
  templateUrl: './types.page.html',
  styleUrl: './types.page.less',
})
export class TypesPage {
  private readonly router = inject(Router);
  private readonly route = inject(ActivatedRoute);

  protected readonly graph = inject(GraphStore);
  protected readonly store = inject(ComponentTypesStore);

  /** Identifier of the diagrammed component, bound from the query parameter. */
  readonly component = input<string | undefined>('');

  constructor() {
    // The address selects the component; the store loads what it names.
    effect(() => {
      const componentId = this.component() ?? '';
      if (componentId !== '') {
        void this.store.load(componentId);
      }
    });
  }

  /** Identifiers of every analyzed component, for the picker. */
  protected readonly componentOptions = computed(() =>
    this.graph
      .components()
      .map((candidate) => candidate.id)
      .sort((first, second) => first.localeCompare(second)),
  );

  /** The positioned nodes and links of the loaded component. */
  protected readonly model = computed(() =>
    buildTypeDiagramModel(
      this.store.types(),
      this.store.incoming(),
      this.store.collapsedIds(),
      this.store.selectedTypeId(),
    ),
  );

  /** How many types the diagram shows, stated beside the heading. */
  protected readonly counter = computed(() => {
    if (this.selectedComponent() === '' || this.store.loading()) {
      return '';
    }
    return `(${this.store.types().length})`;
  });

  /** The component the address names, empty before the first pick. */
  protected readonly selectedComponent = computed(() => this.component() ?? '');

  /** Writes the picked component to the address, which triggers the load. */
  protected setComponent(componentId: string | null): void {
    void this.router.navigate([], {
      relativeTo: this.route,
      queryParams: { component: componentId === '' ? null : componentId },
      queryParamsHandling: 'merge',
    });
  }
}
