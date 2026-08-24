import { DecimalPipe } from '@angular/common';
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
import {
  bucketDistances,
  rankOffenders,
  RANKING_LIMIT,
  summarizeZones,
} from '../../../../kernel/graph/balance-report';
import { ComponentSelectionStore } from '../../../../kernel/graph/component-selection.store';
import { GraphStore } from '../../../../kernel/graph/graph.store';
import { AlertComponent } from '../../../../ui/alert/alert.component';
import { EmptyStateComponent } from '../../../../ui/empty-state/empty-state.component';
import { PageHeaderComponent } from '../../../../ui/page-header/page-header.component';
import { ComponentDetailComponent } from '../../component-detail/component-detail.component';
import {
  createBalanceDistributionChart,
  type BalanceDistributionChart,
} from './balance-distribution.chart';
import { createBalanceRankingChart, type BalanceRankingChart } from './balance-ranking.chart';

/**
 * The balance of the repository, read three ways.
 *
 * A scatter chart of every component stops reading once the repository grows,
 * so this page answers the three questions behind it directly: how many
 * components sit in each zone, which ones to act on first, and how the whole
 * repository spreads around its balance.
 */
@Component({
  selector: 'app-balance-page',
  imports: [
    AlertComponent,
    ComponentDetailComponent,
    DecimalPipe,
    EmptyStateComponent,
    PageHeaderComponent,
    TuiButton,
    TuiLoader,
  ],
  templateUrl: './balance.page.html',
  styleUrl: './balance.page.less',
})
export class BalancePage {
  protected readonly graph = inject(GraphStore);
  private readonly selection = inject(ComponentSelectionStore);
  private readonly rankingSurface = viewChild<ElementRef<HTMLElement>>('rankingSurface');
  private readonly distributionSurface = viewChild<ElementRef<HTMLElement>>('distributionSurface');

  /** How many components the ranking shows, stated in the panel description. */
  protected readonly rankingLimit = RANKING_LIMIT;

  protected readonly zones = computed(() => summarizeZones(this.graph.components()));
  protected readonly offenders = computed(() => rankOffenders(this.graph.components()));
  protected readonly buckets = computed(() => bucketDistances(this.graph.components()));

  // The engines load on demand, so each chart exists only after the first
  // render. The signals let the effects below wait for them.
  private readonly ranking = signal<BalanceRankingChart | undefined>(undefined);
  private readonly distribution = signal<BalanceDistributionChart | undefined>(undefined);
  private destroyed = false;

  constructor() {
    inject(DestroyRef).onDestroy(() => {
      this.destroyed = true;
      this.ranking()?.dispose();
      this.distribution()?.dispose();
    });

    // The surfaces sit behind loading and empty states. The render effect
    // reads the view-child signals, so it runs again when a surface appears
    // and mounts the missing chart.
    afterRenderEffect(() => this.mountCharts());

    effect(() => {
      this.ranking()?.render(this.offenders());
    });
    effect(() => {
      this.distribution()?.render(this.buckets());
    });
  }

  private mountCharts(): void {
    if (this.destroyed || !supportsCanvasRendering()) {
      return;
    }
    const rankingElement = this.rankingSurface()?.nativeElement;
    if (rankingElement !== undefined && this.ranking() === undefined) {
      void createBalanceRankingChart(rankingElement, (id) => this.selection.select(id)).then(
        (chart) => this.adopt(chart, this.ranking),
      );
    }
    const distributionElement = this.distributionSurface()?.nativeElement;
    if (distributionElement !== undefined && this.distribution() === undefined) {
      void createBalanceDistributionChart(distributionElement).then((chart) =>
        this.adopt(chart, this.distribution),
      );
    }
  }

  private adopt<T extends { dispose(): void }>(
    chart: T,
    holder: ReturnType<typeof signal<T | undefined>>,
  ): void {
    if (this.destroyed || holder() !== undefined) {
      chart.dispose();
      return;
    }
    holder.set(chart);
  }
}
