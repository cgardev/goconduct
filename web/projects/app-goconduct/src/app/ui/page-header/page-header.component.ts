import { ChangeDetectionStrategy, Component, input } from '@angular/core';

/**
 * Heading of a routed page, with its counter, its description and its actions.
 *
 * Every page states what it shows in the same shape, so a reader who moves
 * between two of them finds the title, the size of the collection and the
 * controls in the same place. The counter sits beside the heading rather than
 * inside it, so a screen reader announces the title without the number.
 */
@Component({
  selector: 'app-page-header',
  templateUrl: './page-header.component.html',
  styleUrl: './page-header.component.less',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class PageHeaderComponent {
  /** Title of the page. */
  readonly heading = input.required<string>();

  /** One sentence that states what the page answers. */
  readonly description = input('');

  /** Size of the collection the page shows, such as `(42)`. */
  readonly counter = input('');
}
