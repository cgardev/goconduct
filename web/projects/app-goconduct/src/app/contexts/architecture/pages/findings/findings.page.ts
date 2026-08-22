import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { TuiIcon, TuiLoader } from '@taiga-ui/core';
import { TuiBadge } from '@taiga-ui/kit';
import { GraphStore } from '../../../../kernel/graph/graph.store';

/** Results of the deterministic architecture rules. */
@Component({
  selector: 'app-findings-page',
  imports: [TuiBadge, TuiIcon, TuiLoader],
  templateUrl: './findings.page.html',
  styleUrl: './findings.page.less',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class FindingsPage {
  protected readonly graph = inject(GraphStore);

  /** Icon that states the severity of one finding. */
  protected severityIcon(severity: string): string {
    return severity === 'error' ? '@tui.circle-x' : '@tui.triangle-alert';
  }
}
