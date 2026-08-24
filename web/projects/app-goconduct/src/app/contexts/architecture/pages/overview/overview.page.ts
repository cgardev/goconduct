import { DecimalPipe } from '@angular/common';
import { Component, computed, inject } from '@angular/core';
import { RouterLink } from '@angular/router';
import { TuiButton, TuiIcon, TuiLoader } from '@taiga-ui/core';
import { summarizeZones } from '../../../../kernel/graph/balance-report';
import { GRAPH_LAYOUT_LIMIT } from '../../../../kernel/graph/graph-layout';
import { GraphStore } from '../../../../kernel/graph/graph.store';
import { navTo } from '../../../../kernel/routing/app-navigation';
import { AlertComponent } from '../../../../ui/alert/alert.component';
import { EmptyStateComponent } from '../../../../ui/empty-state/empty-state.component';
import { PageHeaderComponent } from '../../../../ui/page-header/page-header.component';
import { ComponentDetailComponent } from '../../component-detail/component-detail.component';
import { DependencyMapComponent } from '../../dependency-map/dependency-map.component';

/**
 * Landing page of the console: what the analysis found, and the map of the
 * repository.
 *
 * The page reports state rather than selling the product, so it opens with a
 * page heading instead of a hero. A reader arrives here to see whether anything
 * changed, and a full-height banner would push that answer below the fold.
 *
 * The alpha notice stays on this page and is not dismissible. The state applies
 * to the whole console, and this is the page a reader lands on.
 */
@Component({
  selector: 'app-overview-page',
  imports: [
    AlertComponent,
    ComponentDetailComponent,
    DecimalPipe,
    DependencyMapComponent,
    EmptyStateComponent,
    PageHeaderComponent,
    RouterLink,
    TuiButton,
    TuiIcon,
    TuiLoader,
  ],
  templateUrl: './overview.page.html',
  styleUrl: './overview.page.less',
})
export class OverviewPage {
  protected readonly graph = inject(GraphStore);

  /** How many components the map places, stated in the legend. */
  protected readonly layoutLimit = GRAPH_LAYOUT_LIMIT;

  /** Components per balance zone, summarized for the panel that links to the balance page. */
  protected readonly zones = computed(() => summarizeZones(this.graph.components()));

  protected readonly componentsLink = navTo.components();
  protected readonly balanceLink = navTo.balance();
  protected readonly findingsLink = navTo.findings();
}
