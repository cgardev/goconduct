import {
  ChangeDetectionStrategy,
  Component,
  inject,
  InjectionToken,
  type Provider,
} from '@angular/core';
import { TuiIcon } from '@taiga-ui/core';

/**
 * The failure that stopped the console from starting. The bootstrap binds it
 * when `loadRuntimeEnvironment` rejects, and leaves it null otherwise, so the
 * page also renders without one.
 */
export const CONFIGURATION_ERROR = new InjectionToken<unknown>('CONFIGURATION_ERROR', {
  factory: () => null,
});

/** Binds the failure the configuration error page reports. */
export function provideConfigurationError(error: unknown): Provider[] {
  return [{ provide: CONFIGURATION_ERROR, useValue: error }];
}

/**
 * The visible half of the fail-loud runtime environment contract: when the
 * environment document of the deployment is missing or incomplete, this page is
 * the whole application.
 *
 * It deliberately offers no way onward. The one field it validates decides where
 * every request goes, so a console without it has nothing to render. What it
 * does offer is the name of the offending field, which turns this screen into a
 * fix.
 */
@Component({
  selector: 'app-configuration-error-page',
  imports: [TuiIcon],
  templateUrl: './configuration-error.page.html',
  styleUrl: './configuration-error.page.less',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ConfigurationErrorPage {
  /** What exactly is wrong, as reported by the runtime environment loader. */
  protected readonly detail = describe(inject(CONFIGURATION_ERROR));
}

// describe extracts the message of the loader, which names the offending field.
// An unrecognized value is reported as such rather than stringified into noise,
// because a misleading detail here is worse than none.
function describe(error: unknown): string {
  if (error instanceof Error && error.message !== '') {
    return error.message;
  }
  if (typeof error === 'string' && error !== '') {
    return error;
  }
  return 'The runtime environment document could not be validated.';
}
