import {
  Component,
  provideBrowserGlobalErrorListeners,
  provideZonelessChangeDetection,
} from '@angular/core';
import { bootstrapApplication } from '@angular/platform-browser';
import { provideTaiga, TuiRoot } from '@taiga-ui/core';
import { App } from './app/app';
import { createAppConfig } from './app/app.config';
import { loadRuntimeEnvironment, RuntimeEnvironment } from './app/kernel/runtime-environment';
import {
  ConfigurationErrorPage,
  provideConfigurationError,
} from './app/shell/configuration-error/configuration-error.page';
import { environment } from './environments/environment';

/**
 * Root of the failure mode of the console: the whole application when the
 * runtime environment could not be resolved.
 *
 * It carries the `app-root` selector because index.html offers exactly one host
 * element, and it deliberately provides no router: the routes read the very
 * document that is missing, so there is nothing here to navigate to.
 */
@Component({
  selector: 'app-root',
  imports: [ConfigurationErrorPage, TuiRoot],
  template: '<tui-root><app-configuration-error-page /></tui-root>',
  styles: `
    :host {
      display: flex;
      width: 100%;
      min-height: 100%;
    }
  `,
})
class ConfigurationErrorApp {}

/**
 * Resolves the runtime environment before bootstrapping, so the Connect RPC
 * transport is configured by the first provider rather than reconfigured once
 * some later asynchronous step settles.
 *
 * A rejection is not left as a console message behind a blank page: the console
 * boots into the configuration error page instead, which names the offending
 * field. A reader facing a console that will not start has to be told why
 * without opening developer tools.
 */
async function bootstrap(): Promise<void> {
  let runtimeEnvironment: RuntimeEnvironment;
  try {
    runtimeEnvironment = await loadRuntimeEnvironment(environment.runtimeEnvironment);
  } catch (error: unknown) {
    await bootstrapConfigurationError(error);
    return;
  }

  await bootstrapApplication(App, createAppConfig(runtimeEnvironment));
}

function bootstrapConfigurationError(error: unknown): Promise<unknown> {
  return bootstrapApplication(ConfigurationErrorApp, {
    providers: [
      provideBrowserGlobalErrorListeners(),
      provideZonelessChangeDetection(),
      provideTaiga({ mode: 'light' }),
      ...provideConfigurationError(error),
    ],
  });
}

// A failure of the fallback itself leaves no surface to report on, so the
// browser console is the genuine last resort here.
void bootstrap().catch((error: unknown) => console.error(error));
