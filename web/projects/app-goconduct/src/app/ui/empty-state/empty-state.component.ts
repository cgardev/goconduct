import { Component, input } from '@angular/core';
import { TuiIcon } from '@taiga-ui/core';

/** What the absence of content means, which decides the icon color. */
export type EmptyStateTone = 'neutral' | 'positive' | 'negative';

/**
 * States that a collection has nothing to show, and offers the one way out.
 *
 * The three cases a reader can reach are deliberately distinct, because the
 * recovery differs: an empty collection has no resources yet, a zero-results
 * state has a filter to clear, and a failure has an operation to retry. The
 * caller states which one it is through the heading, the description and the
 * projected action.
 */
@Component({
  selector: 'app-empty-state',
  imports: [TuiIcon],
  templateUrl: './empty-state.component.html',
  styleUrl: './empty-state.component.less',
})
export class EmptyStateComponent {
  /** Taiga icon name shown above the heading. */
  readonly icon = input.required<string>();

  /** What the state is, in three words or so. */
  readonly heading = input.required<string>();

  /** Why the collection is empty, and what to do next. */
  readonly description = input('');

  /** What the absence means, which decides the color of the icon. */
  readonly tone = input<EmptyStateTone>('neutral');
}
