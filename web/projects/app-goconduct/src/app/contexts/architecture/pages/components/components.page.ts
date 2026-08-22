import { DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { TuiIcon, TuiLoader } from '@taiga-ui/core';
import { ComponentSelectionStore } from '../../../../kernel/graph/component-selection.store';
import { filterComponents } from '../../../../kernel/graph/graph-layout';
import { GraphStore } from '../../../../kernel/graph/graph.store';
import { ComponentDetailComponent } from '../../component-detail/component-detail.component';

/** Value of the role filter that keeps every role. */
const EVERY_ROLE = 'all';

/**
 * Coupling metrics of every analyzed component.
 *
 * The text query and the role filter stay in this page rather than in the graph
 * store: they narrow one table, and no other page reads them.
 */
@Component({
  selector: 'app-components-page',
  imports: [ComponentDetailComponent, DecimalPipe, TuiIcon, TuiLoader],
  templateUrl: './components.page.html',
  styleUrl: './components.page.less',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ComponentsPage {
  protected readonly graph = inject(GraphStore);
  protected readonly selection = inject(ComponentSelectionStore);

  protected readonly query = signal('');
  protected readonly role = signal(EVERY_ROLE);

  /** The components that satisfy the query and the role filter. */
  protected readonly components = computed(() =>
    filterComponents(this.graph.components(), this.role(), this.query()),
  );

  protected updateQuery(event: Event): void {
    this.query.set((event.target as HTMLInputElement).value);
  }

  protected updateRole(event: Event): void {
    this.role.set((event.target as HTMLSelectElement).value);
  }

  /** Converts a ratio of the report into the width percentage of its bar. */
  protected percentage(value: number): number {
    return Math.round(value * 100);
  }
}
