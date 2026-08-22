import {
  ApplicationConfig,
  provideBrowserGlobalErrorListeners,
  provideZonelessChangeDetection,
} from '@angular/core';
import { provideRouter, withComponentInputBinding, withInMemoryScrolling } from '@angular/router';
import { provideTaiga } from '@taiga-ui/core';
import { appRoutes } from './app.routing';
import { provideApi } from './kernel/client/api';
import { RUNTIME_ENVIRONMENT, RuntimeEnvironment } from './kernel/runtime-environment';

/**
 * Builds the root providers from the runtime environment resolved before
 * bootstrap. Taking it as a parameter rather than resolving it from an
 * initializer is what lets the Connect RPC transport be configured by the very
 * first provider, instead of leaving it half-configured until some later
 * asynchronous step completes.
 *
 * @param runtimeEnvironment - Configuration resolved before bootstrap.
 */
export function createAppConfig(runtimeEnvironment: RuntimeEnvironment): ApplicationConfig {
  return {
    providers: [
      { provide: RUNTIME_ENVIRONMENT, useValue: runtimeEnvironment },
      provideBrowserGlobalErrorListeners(),
      provideZonelessChangeDetection(),
      // Taiga UI reads its options from an injection token that has no default
      // factory, so <tui-root> — and therefore every dialog and dropdown
      // portalled through it — fails to construct without this. It also brings
      // in the event plugins the templates of Taiga bind against.
      provideTaiga(),
      provideRouter(
        appRoutes,
        withComponentInputBinding(),
        withInMemoryScrolling({ scrollPositionRestoration: 'enabled' }),
      ),
      ...provideApi(runtimeEnvironment.apiBaseUrl),
    ],
  };
}
