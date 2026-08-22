import { DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { TuiIcon, TuiLoader } from '@taiga-ui/core';
import { ComponentSelectionStore } from '../../../../kernel/graph/component-selection.store';
import { GRAPH_LAYOUT_LIMIT } from '../../../../kernel/graph/graph-layout';
import { GraphStore } from '../../../../kernel/graph/graph.store';
import { ComponentDetailComponent } from '../../component-detail/component-detail.component';

/** Summary of the analysis and dependency map of the repository. */
@Component({
  selector: 'app-overview-page',
  imports: [ComponentDetailComponent, DecimalPipe, TuiIcon, TuiLoader],
  templateUrl: './overview.page.html',
  styleUrl: './overview.page.less',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class OverviewPage {
  protected readonly graph = inject(GraphStore);
  protected readonly selection = inject(ComponentSelectionStore);

  /** How many components the map places, stated in the legend. */
  protected readonly layoutLimit = GRAPH_LAYOUT_LIMIT;
}
