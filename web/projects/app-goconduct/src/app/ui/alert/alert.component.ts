import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { TuiIcon } from '@taiga-ui/core';

/** What the message reports, which decides its color and its icon. */
export type AlertTone = 'info' | 'success' | 'warning' | 'error';

/** Default icon of each tone. */
const TONE_ICONS: Record<AlertTone, string> = {
  info: '@tui.info',
  success: '@tui.circle-check',
  warning: '@tui.triangle-alert',
  error: '@tui.circle-alert',
};

/**
 * Contextual message placed beside the section it is about.
 *
 * Taiga UI has no inline page alert in this version: `tui-notification-alert`
 * is the portal of the toast service, and `tuiMessage` is an inline chat
 * bubble that sizes to its content and never wraps. Neither fits a full-width
 * banner with a heading, a description and a recovery action, so the component
 * exists here rather than being pulled out of the library.
 *
 * An error message announces itself, because it reports something that already
 * failed. Every other tone stays out of the announcement queue.
 */
@Component({
  selector: 'app-alert',
  imports: [TuiIcon],
  templateUrl: './alert.component.html',
  styleUrl: './alert.component.less',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class AlertComponent {
  /** What the message reports. */
  readonly tone = input<AlertTone>('info');

  /** What happened, in a few words. */
  readonly heading = input.required<string>();

  /** Icon name, defaulted from the tone. */
  readonly icon = input('');

  /** The icon actually rendered. */
  protected readonly resolvedIcon = computed(() => this.icon() || TONE_ICONS[this.tone()]);

  /** ARIA role, so a failure reaches a screen reader without a navigation. */
  protected readonly role = computed(() => (this.tone() === 'error' ? 'alert' : 'status'));
}
