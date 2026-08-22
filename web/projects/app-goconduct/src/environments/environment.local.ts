import { AppEnvironment } from './interface';

/**
 * Local environment, for a run that reaches the `goconduct` server directly
 * instead of through the proxy of the development server. The port is the one
 * the server binds by default.
 *
 * The server must allow the origin of the development server, or the browser
 * refuses every cross-origin request before it reaches the transport.
 */
export const environment: AppEnvironment = {
  production: false,
  runtimeEnvironment: {
    kind: 'inline',
    values: { apiBaseUrl: 'http://127.0.0.1:6062' },
  },
};
