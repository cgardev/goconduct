import {
  afterRenderEffect,
  Component,
  computed,
  DestroyRef,
  effect,
  ElementRef,
  inject,
  signal,
  viewChild,
} from '@angular/core';
import { TuiButton, TuiLoader } from '@taiga-ui/core';
import { supportsCanvasRendering } from '../../../../kernel/diagram/rendering-support';
import { GraphStore } from '../../../../kernel/graph/graph.store';
import {
  buildStrategyReport,
  groupOf,
  THIN_EDGE_LIMIT,
  type StrategyEdge,
} from '../../../../kernel/graph/strategy-report';
import { EmptyStateComponent } from '../../../../ui/empty-state/empty-state.component';
import { PageHeaderComponent } from '../../../../ui/page-header/page-header.component';
import { StrategyMapComponent } from '../../strategy-map/strategy-map.component';
import { createStrategyMatrix, type StrategyMatrixChart } from './strategy-matrix.chart';

/** What the reader inspects: one group, or one ordered pair of groups. */
interface StrategyFocus {
  readonly source: string;
  /** Empty when the reader inspects every dependency of the source group. */
  readonly target: string;
}

/**
 * The strategy the repository already follows, mined from its dependencies.
 *
 * The page answers three strategic questions without any declared intent:
 * which layers the groups form de facto, how strongly each group leans on
 * each other one, and where the current strategy contradicts itself — cycles
 * and near-empty edges a reader may want to cut. Selecting a group or a
 * matrix cell lists the exact component dependencies behind it, which is the
 * lever for varying the strategy.
 */
@Component({
  selector: 'app-strategy-page',
  imports: [
    EmptyStateComponent,
    PageHeaderComponent,
    StrategyMapComponent,
    TuiButton,
    TuiLoader,
  ],
  templateUrl: './strategy.page.html',
  styleUrl: './strategy.page.less',
})
export class StrategyPage {
  protected readonly graph = inject(GraphStore);
  private readonly matrixSurface = viewChild<ElementRef<HTMLElement>>('matrixSurface');

  protected readonly thinLimit = THIN_EDGE_LIMIT;

  protected readonly report = computed(() =>
    buildStrategyReport(this.graph.components(), this.graph.relationships()),
  );

  /** One sentence per cycle between two groups, the strategy contradicting itself. */
  protected readonly cycles = computed(() => {
    const seen = new Set<string>();
    const sentences: string[] = [];
    for (const edge of this.report().edges) {
      if (!edge.cyclic) {
        continue;
      }
      const key = [edge.source, edge.target].sort().join(' ');
      if (seen.has(key)) {
        continue;
      }
      seen.add(key);
      sentences.push(`${edge.source} and ${edge.target} depend on each other.`);
    }
    return sentences;
  });

  /** The near-empty edges: candidates to cut when varying the strategy. */
  protected readonly thinEdges = computed(() =>
    this.report()
      .edges.filter((edge) => !edge.cyclic && edge.weight <= THIN_EDGE_LIMIT)
      .sort((first, second) => first.weight - second.weight),
  );

  protected readonly focus = signal<StrategyFocus | undefined>(undefined);

  /** The component dependencies behind the focused group or pair. */
  protected readonly focusedRelationships = computed(() => {
    const focus = this.focus();
    if (focus === undefined) {
      return [];
    }
    return this.graph
      .relationships()
      .filter(
        (relationship) =>
          !relationship.testOnly &&
          groupOf(relationship.source) === focus.source &&
          groupOf(relationship.target) !== focus.source &&
          (focus.target === '' || groupOf(relationship.target) === focus.target),
      )
      .sort(
        (first, second) =>
          first.source.localeCompare(second.source) || first.target.localeCompare(second.target),
      );
  });

  private readonly matrix = signal<StrategyMatrixChart | undefined>(undefined);
  private destroyed = false;

  constructor() {
    inject(DestroyRef).onDestroy(() => {
      this.destroyed = true;
      this.matrix()?.dispose();
    });

    // The surface sits behind loading states. The render effect reads the
    // view-child signal, so it runs again when the surface appears.
    afterRenderEffect(() => this.mountMatrix());

    effect(() => {
      this.matrix()?.render(this.report());
    });
  }

  protected inspect(source: string, target = ''): void {
    this.focus.set({ source, target });
  }

  protected inspectEdge(edge: StrategyEdge): void {
    this.inspect(edge.source, edge.target);
  }

  protected clearFocus(): void {
    this.focus.set(undefined);
  }

  private mountMatrix(): void {
    if (this.destroyed || !supportsCanvasRendering() || this.matrix() !== undefined) {
      return;
    }
    const element = this.matrixSurface()?.nativeElement;
    if (element === undefined) {
      return;
    }
    void createStrategyMatrix(element, (source, target) => this.inspect(source, target)).then(
      (chart) => {
        if (this.destroyed || this.matrix() !== undefined) {
          chart.dispose();
          return;
        }
        this.matrix.set(chart);
      },
    );
  }
}
