import { DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { TuiButton, TuiIcon, TuiLoader } from '@taiga-ui/core';
import { TuiBadge } from '@taiga-ui/kit';
import { DashboardStore } from './dashboard.store';

/** Displays the deterministic architecture report. */
@Component({
  selector: 'app-dashboard-page',
  imports: [DecimalPipe, TuiBadge, TuiButton, TuiIcon, TuiLoader],
  providers: [DashboardStore],
  templateUrl: './dashboard.page.html',
  styleUrl: './dashboard.page.less',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class DashboardPage {
  protected readonly store = inject(DashboardStore);

  protected updateQuery(event: Event): void {
    this.store.setQuery((event.target as HTMLInputElement).value);
  }

  protected updateRole(event: Event): void {
    this.store.setRole((event.target as HTMLSelectElement).value);
  }

  protected percentage(value: number): number {
    return Math.round(value * 100);
  }
}
