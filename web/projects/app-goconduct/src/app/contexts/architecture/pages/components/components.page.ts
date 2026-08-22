import { DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, input, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { TuiButton, TuiDropdown, TuiIcon, TuiLoader, TuiTextfield } from '@taiga-ui/core';
import { TuiChevron, TuiDataListWrapper, TuiSelect } from '@taiga-ui/kit';
import { ComponentSelectionStore } from '../../../../kernel/graph/component-selection.store';
import {
  DEFAULT_COMPONENT_SORT,
  EVERY_ROLE,
  filterComponents,
  sortComponents,
  type ComponentSort,
  type ComponentSortColumn,
} from '../../../../kernel/graph/graph-layout';
import { GraphStore } from '../../../../kernel/graph/graph.store';
import { EmptyStateComponent } from '../../../../ui/empty-state/empty-state.component';
import { PageHeaderComponent } from '../../../../ui/page-header/page-header.component';
import { ComponentDetailComponent } from '../../component-detail/component-detail.component';

/** One sortable column of the table, with the label of its header. */
interface ColumnDefinition {
  readonly column: ComponentSortColumn;
  readonly label: string;
  readonly description: string;
  readonly numeric: boolean;
}

/** Columns of the table, ordered from the most identifying to the most derived. */
const COLUMNS: readonly ColumnDefinition[] = [
  { column: 'name', label: 'Component', description: 'Name and import path', numeric: false },
  { column: 'role', label: 'Role', description: 'Architectural role', numeric: false },
  { column: 'afferentCoupling', label: 'Ca', description: 'Afferent coupling', numeric: true },
  { column: 'efferentCoupling', label: 'Ce', description: 'Efferent coupling', numeric: true },
  { column: 'instability', label: 'Instability', description: 'Ce / (Ca + Ce)', numeric: true },
  {
    column: 'mainSequenceDistance',
    label: 'Distance',
    description: 'Distance from the main sequence',
    numeric: true,
  },
];

/**
 * Coupling metrics of every analyzed component.
 *
 * The text query and the role travel in the query string, so a reader can share
 * the exact table they are looking at, and a reload keeps it. The URL is the
 * single source of truth: the controls write to it, and the rows are computed
 * from what it holds, never from a second copy of the same state.
 *
 * The sort stays in the component instead, because it orders one reading rather
 * than selecting which components the reading is about.
 */
@Component({
  selector: 'app-components-page',
  imports: [
    ComponentDetailComponent,
    DecimalPipe,
    EmptyStateComponent,
    FormsModule,
    PageHeaderComponent,
    TuiButton,
    TuiChevron,
    TuiDataListWrapper,
    TuiDropdown,
    TuiIcon,
    TuiLoader,
    TuiSelect,
    TuiTextfield,
  ],
  templateUrl: './components.page.html',
  styleUrl: './components.page.less',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ComponentsPage {
  private readonly router = inject(Router);
  private readonly route = inject(ActivatedRoute);

  protected readonly graph = inject(GraphStore);
  protected readonly selection = inject(ComponentSelectionStore);

  /**
   * Text query, bound from the `q` query parameter.
   *
   * The router writes `undefined` when the parameter is absent, which overrides
   * the declared default, so both inputs are normalized below rather than read
   * directly.
   */
  readonly q = input<string | undefined>('');

  /** Selected role, bound from the `role` query parameter. */
  readonly role = input<string | undefined>(EVERY_ROLE);

  protected readonly columns = COLUMNS;
  protected readonly sort = signal<ComponentSort>(DEFAULT_COMPONENT_SORT);

  /** The text query, with an absent parameter read as an empty one. */
  protected readonly query = computed(() => this.q() ?? '');

  // An absent parameter and an empty one mean the same thing to a reader, so
  // both resolve to the value that keeps every role.
  protected readonly selectedRole = computed(() => this.role() || EVERY_ROLE);

  /** Roles offered by the filter, with the catch-all value first. */
  protected readonly roleOptions = computed(() => [EVERY_ROLE, ...this.graph.roles()]);

  /** The components that satisfy the query and the role. */
  protected readonly matches = computed(() =>
    filterComponents(this.graph.components(), this.selectedRole(), this.query()),
  );

  /** The matching components, in the order of the selected column. */
  protected readonly rows = computed(() => sortComponents(this.matches(), this.sort()));

  /** Whether the reader has narrowed the table at all. */
  protected readonly filtered = computed(
    () => this.query().trim() !== '' || this.selectedRole() !== EVERY_ROLE,
  );

  /** How many components match, stated beside the heading. */
  protected readonly counter = computed(() => {
    const total = this.graph.components().length;
    return this.filtered() ? `(${this.matches().length} of ${total})` : `(${total})`;
  });

  /** Names the selected role in the trigger and in the dropdown. */
  protected readonly roleLabel = (value: string): string =>
    value === EVERY_ROLE ? 'All roles' : value;

  protected setQuery(value: string): void {
    void this.writeFilters({ q: value.trim() === '' ? null : value });
  }

  protected setRole(value: string | null): void {
    void this.writeFilters({ role: value === null || value === EVERY_ROLE ? null : value });
  }

  protected clearFilters(): void {
    void this.writeFilters({ q: null, role: null });
  }

  /** Sorts by a column, and reverses the order when it is already the sorted one. */
  protected toggleSort(column: ComponentSortColumn): void {
    this.sort.update((current) => {
      if (current.column !== column) {
        // A numeric column is most useful highest-first, a text column A to Z.
        const numeric = COLUMNS.find((entry) => entry.column === column)?.numeric ?? false;
        return { column, direction: numeric ? 'descending' : 'ascending' };
      }
      return {
        column,
        direction: current.direction === 'ascending' ? 'descending' : 'ascending',
      };
    });
  }

  /** Value of `aria-sort` for one column header. */
  protected ariaSort(column: ComponentSortColumn): string {
    return this.sort().column === column ? this.sort().direction : 'none';
  }

  /** Converts a ratio of the report into the width percentage of its bar. */
  protected percentage(value: number): number {
    return Math.round(value * 100);
  }

  // writeFilters replaces the current history entry rather than pushing one, so
  // typing a query does not fill the back button with one entry per keystroke.
  private writeFilters(queryParams: Record<string, string | null>): Promise<boolean> {
    return this.router.navigate([], {
      relativeTo: this.route,
      queryParams,
      queryParamsHandling: 'merge',
      replaceUrl: true,
    });
  }
}
