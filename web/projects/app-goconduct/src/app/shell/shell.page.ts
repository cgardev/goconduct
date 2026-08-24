import { Component, inject } from '@angular/core';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { TuiButton, TuiIcon } from '@taiga-ui/core';
import { buildShellNavigation } from './shell.navigation';
import { ShellStore } from './shell.store';
import { TopNavigationComponent } from './top-navigation/top-navigation.component';

/**
 * The application frame: a product bar above, a structural sidebar beside the
 * routed content.
 *
 * The two navigations answer different questions and are kept apart on purpose.
 * The sidebar carries destinations only, so every entry in it navigates. What
 * acts on the console as a whole — the analysis state, the refresh control, the
 * link to the source — lives in the product bar instead.
 *
 * The sidebar collapses to an icon rail, and the choice is persisted, so a
 * reader who works on a wide table keeps the extra width across reloads.
 */
@Component({
  selector: 'app-shell-page',
  imports: [
    RouterLink,
    RouterLinkActive,
    RouterOutlet,
    TopNavigationComponent,
    TuiButton,
    TuiIcon,
  ],
  templateUrl: './shell.page.html',
  styleUrl: './shell.page.less',
})
export class ShellPage {
  protected readonly shell = inject(ShellStore);

  protected readonly navigationGroups = buildShellNavigation();
}
