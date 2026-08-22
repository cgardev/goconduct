import { DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { RouterLink } from '@angular/router';
import { TuiButton, TuiIcon, TuiLoader } from '@taiga-ui/core';
import { ComponentSelectionStore } from '../../../../kernel/graph/component-selection.store';
import { GRAPH_LAYOUT_LIMIT } from '../../../../kernel/graph/graph-layout';
import { GraphStore } from '../../../../kernel/graph/graph.store';
import { navTo } from '../../../../kernel/routing/app-navigation';
import { AlertComponent } from '../../../../ui/alert/alert.component';
import { EmptyStateComponent } from '../../../../ui/empty-state/empty-state.component';
import { PageHeaderComponent } from '../../../../ui/page-header/page-header.component';
import { ComponentDetailComponent } from '../../component-detail/component-detail.component';

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
    EmptyStateComponent,
    PageHeaderComponent,
    RouterLink,
    TuiButton,
    TuiIcon,
    TuiLoader,
  ],
  templateUrl: './overview.page.html',
  styleUrl: './overview.page.less',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class OverviewPage {
  protected readonly graph = inject(GraphStore);
  protected readonly selection = inject(ComponentSelectionStore);

  /** How many components the map places, stated in the legend. */
  protected readonly layoutLimit = GRAPH_LAYOUT_LIMIT;

  protected readonly componentsLink = navTo.components();
  protected readonly findingsLink = navTo.findings();
}
