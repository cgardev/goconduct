import { AppEnvironment } from './interface';

/**
 * Default (development) environment, replaced at build time by
 * environment.prod.ts or environment.local.ts, depending on the build
 * configuration.
 *
 * The values are inlined rather than fetched, so a development run needs no
 * second process that serves a document. The development server proxies every
 * Connect call to the local `goconduct` server, which is why the base URL is the
 * origin of that development server.
 */
export const environment: AppEnvironment = {
  production: false,
  runtimeEnvironment: {
    kind: 'inline',
    values: { apiBaseUrl: '/' },
  },
};
