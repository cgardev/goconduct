import { ChangeDetectionStrategy, Component } from '@angular/core';
import { TuiRoot } from '@taiga-ui/core';
import { DashboardPage } from './dashboard/dashboard.page';

/** Application root and Taiga UI portal host. */
@Component({
  selector: 'app-root',
  imports: [DashboardPage, TuiRoot],
  template: '<tui-root><app-dashboard-page /></tui-root>',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class App {}
