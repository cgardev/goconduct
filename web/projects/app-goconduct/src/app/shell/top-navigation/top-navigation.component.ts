import { ChangeDetectionStrategy, Component, computed, DestroyRef, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { TuiButton, TuiLink } from '@taiga-ui/core';
import { TuiButtonLoading } from '@taiga-ui/kit';
import { formatRelativeTime } from '../../kernel/format/relative-time';
import { GraphStore } from '../../kernel/graph/graph.store';
import { navTo } from '../../kernel/routing/app-navigation';

/** Where the reader finds the source of the console. */
const REPOSITORY_URL = 'https://github.com/cgardev/goconduct';

/** How often the relative timestamp is recomputed, in milliseconds. */
const CLOCK_INTERVAL = 30_000;

/**
 * Product-level bar: the service identity and the controls that act on the
 * whole console.
 *
 * The analysis state, the refresh control and the link to the source live here
 * rather than in the sidebar, because none of them navigates. The sidebar
 * answers "where can I go"; this bar answers "what is the state of the data I
 * am looking at, and how do I renew it".
 */
@Component({
  selector: 'app-top-navigation',
  imports: [RouterLink, TuiButton, TuiButtonLoading, TuiLink],
  templateUrl: './top-navigation.component.html',
  styleUrl: './top-navigation.component.less',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TopNavigationComponent {
  protected readonly graph = inject(GraphStore);

  protected readonly homeLink = navTo.overview();
  protected readonly repositoryUrl = REPOSITORY_URL;

  // The relative timestamp is a function of the current instant, so it needs a
  // clock of its own. Half a minute is the coarsest tick that still turns "Now"
  // into "1 minute ago" without a visible delay.
  private readonly now = signal(new Date());

  /** State of the analysis, as one word for the reader. */
  protected readonly statusLabel = computed(() => {
    if (this.graph.refreshing()) {
      return 'Analyzing';
    }
    return { connecting: 'Connecting', live: 'Live', disconnected: 'Offline' }[
      this.graph.liveState()
    ];
  });

  /** The same state, as a value the stylesheet colors the indicator by. */
  protected readonly statusState = computed(() =>
    this.graph.refreshing() ? 'pending' : this.graph.liveState(),
  );

  /** How long ago the held graph arrived, in words. */
  protected readonly updatedLabel = computed(() => {
    const instant = this.graph.lastUpdatedAt();
    return instant === undefined ? '' : formatRelativeTime(instant, this.now());
  });

  constructor() {
    const timer = setInterval(() => this.now.set(new Date()), CLOCK_INTERVAL);
    inject(DestroyRef).onDestroy(() => clearInterval(timer));
  }

  /** The exact instant, for the tooltip of the relative timestamp. */
  protected absolute(instant: Date): string {
    return instant.toLocaleString();
  }
}
