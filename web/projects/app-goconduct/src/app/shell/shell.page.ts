import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { TuiButton, TuiIcon } from '@taiga-ui/core';
import { TuiBadge } from '@taiga-ui/kit';
import { GraphStore } from '../kernel/graph/graph.store';
import { navTo } from '../kernel/routing/app-navigation';
import { buildShellNavigation } from './shell.navigation';
import { ShellStore } from './shell.store';

/** Where the reader finds the source of the console. */
const REPOSITORY_URL = 'https://github.com/cgardev/goconduct';

/**
 * The application frame: a full-height sidebar and the routed content beside
 * it. There is deliberately no top bar. The sidebar footer carries the analysis
 * state, the refresh control, and the link to the source.
 *
 * The sidebar collapses to an icon rail, and the choice is persisted, so a
 * reader who works on a wide map keeps the extra width across reloads.
 */
@Component({
  selector: 'app-shell-page',
  imports: [RouterLink, RouterLinkActive, RouterOutlet, TuiBadge, TuiButton, TuiIcon],
  templateUrl: './shell.page.html',
  styleUrl: './shell.page.less',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ShellPage {
  protected readonly shell = inject(ShellStore);
  protected readonly graph = inject(GraphStore);

  protected readonly navigationGroups = buildShellNavigation();
  protected readonly homeLink = navTo.overview();
  protected readonly repositoryUrl = REPOSITORY_URL;
}
